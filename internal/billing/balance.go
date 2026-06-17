package billing

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"fanapi/internal/cache"
	"fanapi/internal/db"
	"fanapi/internal/model"

	"github.com/redis/go-redis/v9"
)

const balanceKeyFmt = "user:balance:%d"
const dirtyBalanceSetKey = "billing:dirty_balances"
const balanceSyncAppliedSetKey = "billing:balance_sync_jobs:applied"

func balanceKey(userID int64) string {
	return fmt.Sprintf(balanceKeyFmt, userID)
}

// SyncBalanceToRedis 将用户的 DB 余额加载到 Redis（在启动时或缓存错过时调用）。
func SyncBalanceToRedis(ctx context.Context, userID int64) (int64, error) {
	sess := db.Engine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return 0, err
	}
	if _, err := sess.Exec("SELECT pg_advisory_xact_lock($1, $2)", int64(20260617), userID); err != nil {
		_ = sess.Rollback()
		return 0, err
	}

	var result struct{ Balance int64 }
	found, err := sess.SQL("SELECT balance FROM users WHERE id = ?", userID).Get(&result)
	if err != nil {
		_ = sess.Rollback()
		return 0, err
	}
	if !found {
		_ = sess.Rollback()
		return 0, fmt.Errorf("用户不存在")
	}

	var jobs []model.BalanceSyncJob
	if err := sess.Where("user_id = ? AND status = ?", userID, "pending").
		Cols("id").
		Find(&jobs); err != nil {
		_ = sess.Rollback()
		return 0, err
	}
	if err := setBalanceAndMarkSyncJobsApplied(ctx, userID, result.Balance, jobs); err != nil {
		_ = sess.Rollback()
		return 0, err
	}
	if len(jobs) > 0 {
		ids := make([]int64, 0, len(jobs))
		for _, job := range jobs {
			ids = append(ids, job.ID)
		}
		if _, err := sess.In("id", ids).Cols("status").Update(&model.BalanceSyncJob{Status: "done"}); err != nil {
			_ = sess.Rollback()
			return 0, err
		}
	}
	if err := sess.Commit(); err != nil {
		return 0, err
	}
	return result.Balance, nil
}

// CachedBalance 返回 Redis 中已有的余额；键不存在时返回 found=false，不会从 DB 回填。
func CachedBalance(ctx context.Context, userID int64) (balance int64, found bool, err error) {
	val, err := cache.Client.Get(ctx, balanceKey(userID)).Int64()
	if err == redis.Nil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return val, true, nil
}

// SyncAllBalancesToRedis 将 users.balance 全量刷新到 Redis。
// 仅应在明确需要以 PostgreSQL 为准重建缓存的维护窗口使用。
func SyncAllBalancesToRedis(ctx context.Context) (int, error) {
	var rows []struct {
		ID      int64 `xorm:"id"`
		Balance int64 `xorm:"balance"`
	}
	if err := db.Engine.SQL("SELECT id, balance FROM users").Find(&rows); err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	pipe := cache.Client.Pipeline()
	for _, row := range rows {
		pipe.Set(ctx, balanceKey(row.ID), row.Balance, 0)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return len(rows), nil
}

// ApplyBalanceDelta 对 Redis 余额做增量更新，用于充值/调账同步或失败补偿。
func ApplyBalanceDelta(ctx context.Context, userID, delta int64) error {
	if delta == 0 {
		return nil
	}
	key := balanceKey(userID)
	exists, err := cache.Client.Exists(ctx, key).Result()
	if err != nil {
		return err
	}
	if exists == 0 {
		_, err = SyncBalanceToRedis(ctx, userID)
		return err
	}
	if err := cache.Client.IncrBy(ctx, key, delta).Err(); err != nil {
		return err
	}
	_ = MarkBalanceDirty(ctx, userID)
	return nil
}

// MarkBalanceDirty 记录该用户 Redis 余额需要同步到 PostgreSQL。
func MarkBalanceDirty(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return nil
	}
	return cache.Client.SAdd(ctx, dirtyBalanceSetKey, strconv.FormatInt(userID, 10)).Err()
}

// SyncDirtyBalancesToDB 将 Redis 中已变更的余额批量写回 PostgreSQL。
// Redis 是消费热路径的权威余额；只有当 Redis 在写库后仍保持同一余额时，才清理 dirty 标记。
func SyncDirtyBalancesToDB(ctx context.Context, batchSize int64) (int, error) {
	if batchSize <= 0 {
		batchSize = 1000
	}
	members, err := cache.Client.SRandMemberN(ctx, dirtyBalanceSetKey, batchSize).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	synced := 0
	for _, member := range members {
		userID, parseErr := strconv.ParseInt(member, 10, 64)
		if parseErr != nil || userID <= 0 {
			_ = cache.Client.SRem(ctx, dirtyBalanceSetKey, member).Err()
			continue
		}
		bal, found, err := CachedBalance(ctx, userID)
		if err != nil {
			return synced, err
		}
		if !found {
			_ = cache.Client.SRem(ctx, dirtyBalanceSetKey, member).Err()
			continue
		}
		if _, err := db.Engine.Exec("UPDATE users SET balance = $1 WHERE id = $2", bal, userID); err != nil {
			return synced, err
		}
		if err := clearDirtyIfUnchanged(ctx, userID, member, bal); err != nil {
			return synced, err
		}
		synced++
	}
	return synced, nil
}

var luaClearDirtyIfUnchanged = redis.NewScript(`
local bal = tonumber(redis.call("GET", KEYS[1]))
if not bal then
  return redis.call("SREM", KEYS[2], ARGV[1])
end
if bal == tonumber(ARGV[2]) then
  return redis.call("SREM", KEYS[2], ARGV[1])
end
return 0
`)

func clearDirtyIfUnchanged(ctx context.Context, userID int64, member string, balance int64) error {
	return luaClearDirtyIfUnchanged.Run(ctx, cache.Client, []string{balanceKey(userID), dirtyBalanceSetKey}, member, balance).Err()
}

var luaApplyBalanceSyncJob = redis.NewScript(`
if redis.call("SISMEMBER", KEYS[2], ARGV[2]) == 1 then
  return 2
end
local bal = tonumber(redis.call("GET", KEYS[1]))
if not bal then
  return -2
end
redis.call("INCRBY", KEYS[1], ARGV[1])
redis.call("SADD", KEYS[2], ARGV[2])
redis.call("SADD", KEYS[3], ARGV[3])
return 1
`)

func ApplyBalanceSyncJob(ctx context.Context, job model.BalanceSyncJob) error {
	if job.ID <= 0 || job.UserID <= 0 || job.Delta == 0 {
		return nil
	}
	jobID := strconv.FormatInt(job.ID, 10)
	userID := strconv.FormatInt(job.UserID, 10)
	result, err := luaApplyBalanceSyncJob.Run(
		ctx,
		cache.Client,
		[]string{balanceKey(job.UserID), balanceSyncAppliedSetKey, dirtyBalanceSetKey},
		job.Delta,
		jobID,
		userID,
	).Int64()
	if err != nil {
		return err
	}
	if result == -2 {
		_, err = SyncBalanceToRedis(ctx, job.UserID)
		return err
	}
	return nil
}

var luaSetBalanceAndMarkSyncJobsApplied = redis.NewScript(`
redis.call("SET", KEYS[1], ARGV[1])
for i = 2, #ARGV do
  redis.call("SADD", KEYS[2], ARGV[i])
end
return 1
`)

func setBalanceAndMarkSyncJobsApplied(ctx context.Context, userID, balance int64, jobs []model.BalanceSyncJob) error {
	args := make([]interface{}, 0, len(jobs)+1)
	args = append(args, balance)
	for _, job := range jobs {
		args = append(args, strconv.FormatInt(job.ID, 10))
	}
	return luaSetBalanceAndMarkSyncJobsApplied.Run(ctx, cache.Client, []string{balanceKey(userID), balanceSyncAppliedSetKey}, args...).Err()
}

// ProcessBalanceSyncJobs retries committed DB->Redis balance deltas.
func ProcessBalanceSyncJobs(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	var jobs []model.BalanceSyncJob
	if err := db.Engine.Context(ctx).
		Where("status = ?", "pending").
		Asc("id").
		Limit(limit).
		Find(&jobs); err != nil {
		return 0, err
	}

	processed := 0
	for _, job := range jobs {
		if job.UserID <= 0 || job.Delta == 0 {
			_, _ = db.Engine.Context(ctx).ID(job.ID).Cols("status").Update(&model.BalanceSyncJob{Status: "done"})
			processed++
			continue
		}
		if err := ApplyBalanceSyncJob(ctx, job); err != nil {
			if _, updateErr := db.Engine.Context(ctx).
				ID(job.ID).
				Cols("attempts", "last_error", "updated_at").
				Update(&model.BalanceSyncJob{
					Attempts:  job.Attempts + 1,
					LastError: err.Error(),
				}); updateErr != nil {
				return processed, updateErr
			}
			return processed, err
		}
		if _, err := db.Engine.Context(ctx).
			ID(job.ID).
			Cols("status", "attempts", "last_error", "updated_at").
			Update(&model.BalanceSyncJob{
				Status:    "done",
				Attempts:  job.Attempts + 1,
				LastError: "",
			}); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

// StartBalanceSyncer 后台把 Redis 余额变更同步到 PostgreSQL。
func StartBalanceSyncer(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		log.Println("[billing-sync] redis balance syncer started")
		syncDirtyBalances(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				syncDirtyBalances(ctx)
			}
		}
	}()
}

func syncDirtyBalances(ctx context.Context) {
	if n, err := ProcessBalanceSyncJobs(ctx, 100); err != nil {
		log.Printf("[billing-sync] process db->redis balance jobs failed after %d jobs: %v", n, err)
	}
	for {
		n, err := SyncDirtyBalancesToDB(ctx, 1000)
		if err != nil {
			log.Printf("[billing-sync] sync dirty balances failed: %v", err)
			return
		}
		if n < 1000 {
			return
		}
	}
}

// GetBalance 返回 Redis 缓存的余额，缓存未命中时自动从 DB 同步。
func GetBalance(ctx context.Context, userID int64) (int64, error) {
	val, err := cache.Client.Get(ctx, balanceKey(userID)).Int64()
	if err == nil {
		return val, nil
	}
	return SyncBalanceToRedis(ctx, userID)
}

// luaCharge 原子地扣减 credits，余额不足时返回失败。
var luaCharge = redis.NewScript(`
local bal = tonumber(redis.call("GET", KEYS[1]))
if not bal then return -2 end
if bal < tonumber(ARGV[1]) then return -1 end
return redis.call("DECRBY", KEYS[1], ARGV[1])
`)

// Charge 原子扣减 credits。余额不足时返回错误。
// Lua 脚本在键不存在时返回 -2；此时从 DB 同步余额后重试一次，
// 避免在热路径上额外发起一次 GET。
func Charge(ctx context.Context, userID, credits int64) error {
	if credits <= 0 {
		return nil
	}
	key := balanceKey(userID)
	result, err := luaCharge.Run(ctx, cache.Client, []string{key}, credits).Int64()
	if err != nil {
		return err
	}
	if result == -2 {
		// Redis 键不存在：从 DB 同步后重试
		if _, syncErr := SyncBalanceToRedis(ctx, userID); syncErr != nil {
			return syncErr
		}
		result, err = luaCharge.Run(ctx, cache.Client, []string{key}, credits).Int64()
		if err != nil {
			return err
		}
	}
	if result == -1 {
		return fmt.Errorf("余额不足")
	}
	if result == -2 {
		return fmt.Errorf("余额记录异常，请联系管理员")
	}
	_ = MarkBalanceDirty(ctx, userID)
	return nil
}

// Refund 退还 credits（用于 LLM 输出实际量少于预扣时的差额退款）。
func Refund(ctx context.Context, userID, credits int64) error {
	if credits <= 0 {
		return nil
	}
	key := balanceKey(userID)
	// 确保 Redis 键存在，避免 IncrBy 在键不存在时创建只含退款金额的新键
	// （正确行为应为：实际余额 + 退款金额）。
	if _, err := cache.Client.Get(ctx, key).Int64(); err != nil {
		if _, syncErr := SyncBalanceToRedis(ctx, userID); syncErr != nil {
			return syncErr
		}
	}
	if err := cache.Client.IncrBy(ctx, key, credits).Err(); err != nil {
		return err
	}
	_ = MarkBalanceDirty(ctx, userID)
	return nil
}
