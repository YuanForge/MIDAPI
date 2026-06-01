package handler

import (
	"fanapi/internal/db"
	"fanapi/internal/model"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"time"
)

// GET /admin/notifications
func ListNotifications(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	engine := db.Engine
	sess := engine.Table("notifications").OrderBy("created_at DESC")
	if s := c.Query("status"); s != "" {
		sess = sess.Where("status=?", s)
	}
	countSess := engine.Table("notifications")
	if s := c.Query("status"); s != "" {
		countSess = countSess.Where("status=?", s)
	}
	total, _ := countSess.Count(&model.Notification{})
	var items []model.Notification
	sess.Limit(size, (page-1)*size).Find(&items)
	c.JSON(http.StatusOK, gin.H{"notifications": items, "total": total})
}

// POST /admin/notifications
func CreateNotification(c *gin.Context) {
	var n model.Notification
	if err := c.ShouldBindJSON(&n); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	n.CreatedBy = getAdminID(c)
	if n.Status == "" {
		n.Status = "draft"
	}
	if _, err := db.Engine.Insert(&n); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, n)
}

// POST /admin/notifications/:id/send  立即发送
func SendNotification(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID 格式错误"})
		return
	}
	now := time.Now()
	db.Engine.Exec(
		"UPDATE notifications SET status='sent', sent_at=$1 WHERE id=$2 AND status='draft'",
		now, id,
	)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DELETE /admin/notifications/:id
func DeleteNotification(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID 格式错误"})
		return
	}
	db.Engine.Delete(&model.Notification{ID: id})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GET /admin/alerts
func ListAlerts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	engine := db.Engine
	sess := engine.Table("alerts").OrderBy("created_at DESC")
	if s := c.Query("status"); s != "" {
		sess = sess.Where("status=?", s)
	}
	if t := c.Query("type"); t != "" {
		sess = sess.Where("type=?", t)
	}
	countSess := engine.Table("alerts")
	if s := c.Query("status"); s != "" {
		countSess = countSess.Where("status=?", s)
	}
	if t := c.Query("type"); t != "" {
		countSess = countSess.Where("type=?", t)
	}
	total, _ := countSess.Count(&model.Alert{})
	var items []model.Alert
	sess.Limit(size, (page-1)*size).Find(&items)
	c.JSON(http.StatusOK, gin.H{"alerts": items, "total": total})
}

// PATCH /admin/alerts/:id/ack  确认告警
func AckAlert(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID 格式错误"})
		return
	}
	adminID := getAdminID(c)
	now := time.Now()
	db.Engine.Exec(
		"UPDATE alerts SET status='acked', acked_by=$1, acked_at=$2 WHERE id=$3 AND status='open'",
		adminID, now, id,
	)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// PATCH /admin/alerts/:id/resolve  解决告警
func ResolveAlert(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID 格式错误"})
		return
	}
	now := time.Now()
	db.Engine.Exec(
		"UPDATE alerts SET status='resolved', resolved_at=$1 WHERE id=$2",
		now, id,
	)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
