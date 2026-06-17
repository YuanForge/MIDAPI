package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"fanapi/internal/billing"
	"fanapi/internal/db"
	"fanapi/internal/model"
)

// WriteTx 写入一条计费流水，并在需要时同步余额。
// poolKeyID 为本次请求使用的号池 Key ID（0 表示未使用号池）。
// cost 为支付给上游的进价成本（若暂不记录可传 0）。
// modelCreditCharged 为本次从专属模型积分中扣除的数量（0 表示全部来自通用余额）。
// DB 仅更新通用余额（users.balance），模型积分存储在 user_model_credits 表中。
//
// 余额同步策略：
//   - "hold"/"settle"/"charge"/"refund"：调用方必须先成功操作 Redis，WriteTx 只记录流水；
//     PostgreSQL users.balance 由 Redis->DB 后台同步器按最终 Redis 余额写回。
//   - "recharge"/"adjust"：先在 DB 原子更新余额，再增量同步到 Redis。
func WriteTx(ctx context.Context, userID, channelID, apiKeyID, poolKeyID int64, corrID, txType string, credits, cost, modelCreditCharged int64, metrics model.JSON) error {
	taskID := metricInt64(metrics, "task_id")
	llmLogID := metricInt64(metrics, "llm_log_id")
	skipRedisSync := metricBool(metrics, "skip_redis_sync")
	tx := &model.BillingTransaction{
		UserID:             userID,
		ChannelID:          channelID,
		APIKeyID:           apiKeyID,
		PoolKeyID:          poolKeyID,
		CorrID:             corrID,
		Type:               txType,
		Credits:            credits,
		ModelCreditCharged: modelCreditCharged,
		Cost:               cost,
		Metrics:            metrics,
		LLMLogID:           llmLogID,
		TaskID:             taskID,
	}

	// DB 仅反映通用余额变化；专属模型积分变化记录在 user_model_credits 表。
	generalCredits := credits - modelCreditCharged
	if generalCredits < 0 {
		generalCredits = 0
	}

	var delta int64
	redisPreApplied := false
	switch txType {
	case "charge", "settle", "hold":
		delta = -generalCredits
		redisPreApplied = true
	case "refund", "recharge":
		delta = generalCredits
		redisPreApplied = txType == "refund"
	case "adjust":
		delta = credits
	}
	var balanceSyncJob *model.BalanceSyncJob

	compensateRedis := func(reason string) {
		if skipRedisSync || !redisPreApplied || delta == 0 {
			return
		}
		if err := billing.ApplyBalanceDelta(context.Background(), userID, -delta); err != nil {
			log.Printf("[billing] redis compensation failed user=%d type=%s corr_id=%s delta=%d reason=%s err=%v",
				userID, txType, corrID, -delta, reason, err)
			if markErr := billing.MarkBalanceDirty(context.Background(), userID); markErr != nil {
				log.Printf("[billing] mark dirty after compensation failure failed user=%d type=%s corr_id=%s err=%v",
					userID, txType, corrID, markErr)
			}
		}
	}

	// 将余额更新与流水插入包在同一事务，避免余额已改但流水缺失
	sess := db.Engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		compensateRedis("begin")
		return err
	}
	if !redisPreApplied && !skipRedisSync && delta != 0 {
		if _, err := sess.Exec("SELECT pg_advisory_xact_lock($1, $2)", int64(20260617), userID); err != nil {
			if rbErr := sess.Rollback(); rbErr != nil {
				log.Printf("[billing] rollback failed: %v", rbErr)
			}
			return err
		}
	}

	if redisPreApplied {
		if bal, found, err := billing.CachedBalance(ctx, userID); err == nil && found {
			tx.BalanceAfter = bal
		}
	} else if delta != 0 {
		// 未预先操作 Redis 的余额变更在 DB 内原子更新；消费/退款类由 Redis→PG 同步器负责写回 DB。
		rows, err := sess.QueryString(
			"UPDATE users SET balance = balance + $1 WHERE id = $2 AND balance + $1 >= 0 RETURNING balance",
			delta, userID,
		)
		if err != nil {
			if rbErr := sess.Rollback(); rbErr != nil {
				log.Printf("[billing] rollback failed: %v", rbErr)
			}
			compensateRedis("update_error")
			return err
		}
		if len(rows) == 0 {
			if rbErr := sess.Rollback(); rbErr != nil {
				log.Printf("[billing] rollback failed: %v", rbErr)
			}
			compensateRedis("update_no_rows")
			return fmt.Errorf("用户余额不足或用户不存在")
		}
		if balStr, ok := rows[0]["balance"]; ok {
			tx.BalanceAfter, _ = strconv.ParseInt(balStr, 10, 64)
		}
	}

	if _, err := sess.Insert(tx); err != nil {
		if rbErr := sess.Rollback(); rbErr != nil {
			log.Printf("[billing] rollback failed: %v", rbErr)
		}
		compensateRedis("insert_tx")
		return err
	}

	if !skipRedisSync && !redisPreApplied && delta != 0 {
		balanceSyncJob = &model.BalanceSyncJob{
			UserID: userID,
			Delta:  delta,
			Reason: txType,
			CorrID: corrID,
			Status: "pending",
		}
		if _, err := sess.Insert(balanceSyncJob); err != nil {
			if rbErr := sess.Rollback(); rbErr != nil {
				log.Printf("[billing] rollback failed: %v", rbErr)
			}
			return err
		}
	}

	if err := sess.Commit(); err != nil {
		compensateRedis("commit")
		return err
	}

	// 未预先操作 Redis 的 DB 余额变更（如充值、手动调账）在事务成功后用增量同步到 Redis。
	if !skipRedisSync && redisPreApplied {
		if err := billing.MarkBalanceDirty(context.Background(), userID); err != nil {
			log.Printf("[billing] mark dirty balance failed user=%d type=%s corr_id=%s err=%v",
				userID, txType, corrID, err)
		}
	}
	if !skipRedisSync && !redisPreApplied && delta != 0 {
		if balanceSyncJob == nil {
			log.Printf("[billing] missing balance sync job user=%d type=%s corr_id=%s delta=%d",
				userID, txType, corrID, delta)
			return nil
		}
		if err := billing.ApplyBalanceSyncJob(context.Background(), *balanceSyncJob); err != nil {
			log.Printf("[billing] apply db delta to redis failed user=%d type=%s corr_id=%s delta=%d err=%v",
				userID, txType, corrID, delta, err)
		} else if _, err := db.Engine.ID(balanceSyncJob.ID).Cols("status").Update(&model.BalanceSyncJob{Status: "done"}); err != nil {
			log.Printf("[billing] mark db->redis balance sync done failed user=%d type=%s corr_id=%s job_id=%d err=%v",
				userID, txType, corrID, balanceSyncJob.ID, err)
		}
	}

	// 消费类交易触发邀请返佣和号商收益：
	//   hold    — 输入费预扣（input_from_response=false 时精确，=true 时为估算）
	//   settle  — 输出费或差额补扣
	//   charge  — 图片/视频/音频一次性扣费
	//   refund  — 退款时反向扣回已发放的返佣/收益（传负值）
	switch txType {
	case "charge", "settle", "hold":
		go applyPostBillingHooks(userID, poolKeyID, credits, cost)
	case "refund":
		go applyPostBillingHooks(userID, poolKeyID, -credits, -cost)
	}
	return nil
}

// applyPostBillingHooks 在消费发生后异步处理：
//  1. 邀请返佣：若用户有邀请人，按比例将 credits 加入邀请人的冻结余额
//  2. 号商收益：若本次请求使用了号商的 Key，按比例计入号商可提现余额
//
// credits/cost 可为负值（refund 场景），表示回扣已发放的返佣/收益，不会使余额低于 0。
func metricInt64(metrics model.JSON, key string) int64 {
	if metrics == nil {
		return 0
	}
	switch v := metrics[key].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case json.Number:
		n, _ := v.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	default:
		return 0
	}
}

func metricBool(metrics model.JSON, key string) bool {
	if metrics == nil {
		return false
	}
	switch v := metrics[key].(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1"
	default:
		return false
	}
}

func applyPostBillingHooks(userID, poolKeyID, credits, cost int64) {
	ctx := context.Background()

	// ── 邀请返佣 ─────────────────────────────────────────────────────────
	if credits != 0 {
		var inviterID int64
		var rebateRatio *float64
		rows, err := db.Engine.QueryString(
			"SELECT inviter_id, rebate_ratio FROM users WHERE id = $1", userID,
		)
		if err == nil && len(rows) > 0 {
			if s := rows[0]["inviter_id"]; s != "" {
				inviterID, _ = strconv.ParseInt(s, 10, 64)
			}
			if s := rows[0]["rebate_ratio"]; s != "" {
				var r float64
				if _, err2 := fmt.Sscanf(s, "%f", &r); err2 == nil {
					rebateRatio = &r
				}
			}
		}

		if inviterID > 0 {
			ratio := getRebateRatio(ctx, rebateRatio)
			if ratio > 0 {
				rebateCredits := int64(float64(credits) * ratio)
				if rebateCredits != 0 {
					var sql string
					if rebateCredits > 0 {
						sql = "UPDATE users SET frozen_balance = frozen_balance + $1 WHERE id = $2"
					} else {
						// 回扣：floor 0，不允许透支冻结余额
						sql = "UPDATE users SET frozen_balance = GREATEST(0, frozen_balance + $1) WHERE id = $2"
					}
					if _, err := db.Engine.Exec(sql, rebateCredits, inviterID); err != nil {
						log.Printf("[billing] apply inviter rebate failed user=%d inviter=%d err=%v", userID, inviterID, err)
					}
				}
			}
		}
	}

	// ── 号商收益 ──────────────────────────────────────────────────────────
	if poolKeyID > 0 && cost != 0 {
		rows, err := db.Engine.QueryString(
			"SELECT vendor_id FROM pool_keys WHERE id = $1", poolKeyID,
		)
		if err == nil && len(rows) > 0 {
			vendorIDStr := rows[0]["vendor_id"]
			if vendorIDStr != "" {
				vendorID, _ := strconv.ParseInt(vendorIDStr, 10, 64)
				if vendorID > 0 {
					commission := getVendorCommission(ctx, vendorID)
					// 号商到手 = cost * (1 - commission)；负值时回扣
					vendorEarns := int64(float64(cost) * (1 - commission))
					if vendorEarns != 0 {
						var sql string
						if vendorEarns > 0 {
							sql = "UPDATE vendors SET balance = balance + $1 WHERE id = $2"
						} else {
							// 回扣：floor 0
							sql = "UPDATE vendors SET balance = GREATEST(0, balance + $1) WHERE id = $2"
						}
						if _, err2 := db.Engine.Exec(sql, vendorEarns, vendorID); err2 != nil {
							log.Printf("[billing] apply vendor earning failed vendor=%d err=%v", vendorID, err2)
						}
					}
				}
			}
		}
	}
}

// getRebateRatio 返回有效的返佣比例：优先使用用户个人设置，否则读取系统默认值。
func getRebateRatio(_ context.Context, userRatio *float64) float64 {
	if userRatio != nil {
		return *userRatio
	}
	s := &model.SystemSetting{}
	if found, _ := db.Engine.Where("key = ?", "default_rebate_ratio").Get(s); found && s.Value != "" {
		var r float64
		if _, err := fmt.Sscanf(s.Value, "%f", &r); err == nil {
			return r
		}
	}
	return 0
}

// getVendorCommission 返回有效的平台手续费比例：优先使用号商个人设置，否则读取系统默认值。
func getVendorCommission(_ context.Context, vendorID int64) float64 {
	rows, _ := db.Engine.QueryString("SELECT commission_ratio FROM vendors WHERE id = $1", vendorID)
	if len(rows) > 0 && rows[0]["commission_ratio"] != "" {
		var r float64
		if _, err := fmt.Sscanf(rows[0]["commission_ratio"], "%f", &r); err == nil {
			return r
		}
	}
	s := &model.SystemSetting{}
	if found, _ := db.Engine.Where("key = ?", "default_vendor_commission").Get(s); found && s.Value != "" {
		var r float64
		if _, err := fmt.Sscanf(s.Value, "%f", &r); err == nil {
			return r
		}
	}
	return 0
}

// GetBalance 从 DB 返回用户的当前余额。
func GetBalance(ctx context.Context, userID int64) (int64, error) {
	if balance, err := billing.GetBalance(ctx, userID); err == nil {
		return balance, nil
	}
	return GetDBBalance(ctx, userID)
}

// GetDBBalance 从 PostgreSQL 返回用户的当前余额快照。
func GetDBBalance(ctx context.Context, userID int64) (int64, error) {
	user := &model.User{}
	found, err := db.Engine.Where("id = ?", userID).Cols("balance").Get(user)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, fmt.Errorf("用户不存在")
	}
	return user.Balance, nil
}

// Recharge 为用户增加 credits（管理员操作）。
// 余额更新已在 WriteTx 内完成，请勿在此处重复更新 DB。
func Recharge(ctx context.Context, userID, adminID, credits int64) error {
	return WriteTx(ctx, userID, 0, 0, 0, "", "recharge", credits, 0, 0, nil)
}

// GrantModelCredit 为用户赠送指定模型的专属积分（管理员操作）。
// modelName 为渠道的路由键（display_name 非空时为 display_name，否则为 model）。
func GrantModelCredit(ctx context.Context, userID int64, modelName string, credits int64) error {
	return billing.AddModelCredit(ctx, userID, modelName, credits)
}

// ListModelCredits 返回用户所有模型专属积分记录。
func ListModelCredits(ctx context.Context, userID int64) ([]model.UserModelCredit, error) {
	var records []model.UserModelCredit
	err := db.Engine.Where("user_id = ? AND credits > 0", userID).
		OrderBy("model_name").Find(&records)
	return records, err
}

// ListTransactions 返回用户的分页计费历史。corrID/taskID 非空时分别按对应字段过滤。
func ListTransactions(ctx context.Context, userID int64, page, pageSize int, corrID, taskID string) ([]model.BillingTransaction, error) {
	var txs []model.BillingTransaction
	sess := db.Engine.Where("user_id = ?", userID)
	if corrID != "" {
		sess.And("corr_id = ?", corrID)
	}
	if taskID != "" {
		if id, err := strconv.ParseInt(taskID, 10, 64); err == nil {
			sess.And("task_id = ?", id)
		} else {
			sess.And("1 = 0")
		}
	}
	err := sess.Desc("created_at").
		Limit(pageSize, (page-1)*pageSize).
		Find(&txs)
	return txs, err
}

// CountTransactions 返回用户的计费记录总数。corrID/taskID 非空时分别按对应字段过滤。
func CountTransactions(ctx context.Context, userID int64, corrID, taskID string) (int64, error) {
	sess := db.Engine.Where("user_id = ?", userID)
	if corrID != "" {
		sess.And("corr_id = ?", corrID)
	}
	if taskID != "" {
		if id, err := strconv.ParseInt(taskID, 10, 64); err == nil {
			sess.And("task_id = ?", id)
		} else {
			sess.And("1 = 0")
		}
	}
	count, err := sess.Count(&model.BillingTransaction{})
	return count, err
}
