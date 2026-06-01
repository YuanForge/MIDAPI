package handler

import (
	"fanapi/internal/db"
	"fanapi/internal/model"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

// GET /admin/audit  审计日志列表
func ListAuditLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	engine := db.Engine
	sess := engine.Table("admin_audit_logs").OrderBy("created_at DESC")
	if v := c.Query("admin_id"); v != "" {
		sess = sess.Where("admin_id=?", v)
	}
	if v := c.Query("resource_type"); v != "" {
		sess = sess.Where("resource_type=?", v)
	}
	if v := c.Query("action"); v != "" {
		sess = sess.Where("action=?", v)
	}
	countSess := engine.Table("admin_audit_logs")
	if v := c.Query("admin_id"); v != "" {
		countSess = countSess.Where("admin_id=?", v)
	}
	if v := c.Query("resource_type"); v != "" {
		countSess = countSess.Where("resource_type=?", v)
	}
	if v := c.Query("action"); v != "" {
		countSess = countSess.Where("action=?", v)
	}
	total, _ := countSess.Count(&model.AdminAuditLog{})
	var logs []model.AdminAuditLog
	sess.Limit(size, (page-1)*size).Find(&logs)
	c.JSON(http.StatusOK, gin.H{"logs": logs, "total": total})
}

// helper：从 context 取管理员 ID（middleware 注入 "user_id"）
func getAdminID(c *gin.Context) int64 {
	if v, ok := c.Get("user_id"); ok {
		if id, ok := v.(int64); ok {
			return id
		}
	}
	return 0
}

// GET /admin/settings/logs
func ListSettingLogs(c *gin.Context) {
	var logs []model.AdminAuditLog
	db.Engine.Where("resource_type='settings'").OrderBy("created_at DESC").Limit(100).Find(&logs)
	c.JSON(http.StatusOK, gin.H{"logs": logs})
}
