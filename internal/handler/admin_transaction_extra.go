package handler

import (
	"fanapi/internal/billing"
	"fanapi/internal/db"
	"fanapi/internal/model"
	"fanapi/internal/service"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

// GET /admin/transactions/aggregate  多维聚合
func GetTransactionAggregate(c *gin.Context) {
	dim := c.DefaultQuery("dim", "day") // day/user/channel/model
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	startAt := c.Query("start_at")
	endAt := c.Query("end_at")

	engine := db.Engine
	args := []interface{}{}
	where := "type IN ('charge','settle')"
	if startAt != "" {
		t, _ := parseDateTime(startAt, false)
		if !t.IsZero() {
			where += fmt.Sprintf(" AND created_at >= $%d", len(args)+1)
			args = append(args, t)
		}
	}
	if endAt != "" {
		t, _ := parseDateTime(endAt, true)
		if !t.IsZero() {
			where += fmt.Sprintf(" AND created_at <= $%d", len(args)+1)
			args = append(args, t)
		}
	}

	type aggRow struct {
		Key     string  `json:"key" xorm:"key"`
		Revenue float64 `json:"revenue" xorm:"revenue"`
		Cost    float64 `json:"cost" xorm:"cost"`
		Profit  float64 `json:"profit" xorm:"profit"`
		Calls   int64   `json:"calls" xorm:"calls"`
	}

	var selectExpr, groupExpr string
	switch dim {
	case "user":
		selectExpr = "user_id::text AS key"
		groupExpr = "user_id"
	case "channel":
		selectExpr = "channel_id::text AS key"
		groupExpr = "channel_id"
	case "model":
		// join with llm_logs by corr_id – too expensive; use metrics->>'model'
		selectExpr = "COALESCE(metrics->>'model', 'unknown') AS key"
		groupExpr = "COALESCE(metrics->>'model', 'unknown')"
	default: // day
		selectExpr = "TO_CHAR(DATE_TRUNC('day', created_at AT TIME ZONE 'Asia/Shanghai'), 'YYYY-MM-DD') AS key"
		groupExpr = "DATE_TRUNC('day', created_at AT TIME ZONE 'Asia/Shanghai')"
	}

	whereExpr := where
	if whereExpr != "" {
		whereExpr = "WHERE " + whereExpr
	}
	sql := fmt.Sprintf(
		`SELECT %s,
		        SUM(credits)::float8 AS revenue,
		        SUM(cost)::float8 AS cost,
		        SUM(credits-cost)::float8 AS profit,
		        COUNT(*) AS calls
		 FROM billing_transactions %s
		 GROUP BY %s ORDER BY revenue DESC LIMIT %d OFFSET %d`,
		selectExpr, whereExpr, groupExpr, size, (page-1)*size,
	)
	var rows []aggRow
	if err := engine.SQL(sql, args...).Find(&rows); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for i := range rows {
		rows[i].Revenue /= creditsPerCNY
		rows[i].Cost /= creditsPerCNY
		rows[i].Profit /= creditsPerCNY
	}
	c.JSON(http.StatusOK, gin.H{"rows": rows, "dim": dim})
}

// POST /admin/transactions/adjust  手动调账
func AdjustTransaction(c *gin.Context) {
	var req struct {
		UserID  int64  `json:"user_id"`
		Type    string `json:"type"`    // adjust/recharge/refund
		Credits int64  `json:"credits"` // 正负均可
		Reason  string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.UserID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id 不能为空"})
		return
	}
	if len(req.Reason) < 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reason 至少 5 个字符"})
		return
	}
	if req.Type == "" {
		req.Type = "adjust"
	}

	if err := service.WriteTx(c.Request.Context(), req.UserID, 0, 0, 0, "", "adjust", req.Credits, 0, 0, model.JSON{
		"reason":   req.Reason,
		"admin_id": getAdminID(c),
		"type":     req.Type,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	newBalance, _ := service.GetBalance(c.Request.Context(), req.UserID)
	c.JSON(http.StatusOK, gin.H{"ok": true, "balance_after": newBalance})
}

// POST /admin/transactions/sync-user-balance  按 Redis 余额修正 DB 并写调账流水
func SyncUserBalanceFromRedis(c *gin.Context) {
	var req struct {
		UserID int64  `json:"user_id" binding:"required"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Reason == "" {
		req.Reason = "sync balance from redis"
	}

	redisBalance, found, err := billing.CachedBalance(c.Request.Context(), req.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Redis 余额不存在"})
		return
	}

	dbBalance, err := service.GetDBBalance(c.Request.Context(), req.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	delta := redisBalance - dbBalance
	if delta != 0 {
		if err := service.WriteTx(c.Request.Context(), req.UserID, 0, 0, 0, "", "adjust", delta, 0, 0, model.JSON{
			"reason":          req.Reason,
			"admin_id":        getAdminID(c),
			"source":          "redis",
			"redis_balance":   redisBalance,
			"db_balance":      dbBalance,
			"skip_redis_sync": true,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":            true,
		"db_before":     dbBalance,
		"redis_balance": redisBalance,
		"delta":         delta,
		"balance_after": redisBalance,
		"changed":       delta != 0,
	})
}
