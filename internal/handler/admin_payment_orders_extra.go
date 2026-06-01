package handler

import (
	"fanapi/internal/db"
	"fanapi/internal/model"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

// GET /admin/payments  管理员查看所有支付订单
func AdminListPaymentOrders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	engine := db.Engine
	sess := engine.Table("payment_orders").
		Select("payment_orders.*, users.email AS user_email").
		Join("LEFT", "users", "users.id = payment_orders.user_id").
		OrderBy("payment_orders.created_at DESC")

	if s := c.Query("status"); s != "" {
		sess = sess.Where("payment_orders.status=?", s)
	}
	if uid := c.Query("user_id"); uid != "" {
		sess = sess.Where("payment_orders.user_id=?", uid)
	}
	if email := c.Query("email"); email != "" {
		sess = sess.Where("users.email ILIKE ?", "%"+email+"%")
	}
	if pf := c.Query("pay_flat"); pf != "" {
		sess = sess.Where("payment_orders.pay_flat=?", pf)
	}
	if pc := c.Query("pay_channel"); pc != "" {
		sess = sess.Where("payment_orders.pay_channel=?", pc)
	}
	startAt := c.Query("start_at")
	endAt := c.Query("end_at")
	if startAt != "" {
		t, _ := parseDateTime(startAt, false)
		if !t.IsZero() {
			sess = sess.Where("payment_orders.created_at>=?", t)
		}
	}
	if endAt != "" {
		t, _ := parseDateTime(endAt, true)
		if !t.IsZero() {
			sess = sess.Where("payment_orders.created_at<=?", t)
		}
	}

	type orderRow struct {
		model.PaymentOrder `xorm:"extends"`
		UserEmail          string `json:"user_email" xorm:"user_email"`
	}
	countSess := engine.Table("payment_orders").Join("LEFT", "users", "users.id = payment_orders.user_id")
	if s := c.Query("status"); s != "" {
		countSess = countSess.Where("payment_orders.status=?", s)
	}
	if uid := c.Query("user_id"); uid != "" {
		countSess = countSess.Where("payment_orders.user_id=?", uid)
	}
	if email := c.Query("email"); email != "" {
		countSess = countSess.Where("users.email ILIKE ?", "%"+email+"%")
	}
	if pf := c.Query("pay_flat"); pf != "" {
		countSess = countSess.Where("payment_orders.pay_flat=?", pf)
	}
	if pc := c.Query("pay_channel"); pc != "" {
		countSess = countSess.Where("payment_orders.pay_channel=?", pc)
	}
	if startAt != "" {
		if t, _ := parseDateTime(startAt, false); !t.IsZero() {
			countSess = countSess.Where("payment_orders.created_at>=?", t)
		}
	}
	if endAt != "" {
		if t, _ := parseDateTime(endAt, true); !t.IsZero() {
			countSess = countSess.Where("payment_orders.created_at<=?", t)
		}
	}
	total, _ := countSess.Count(&model.PaymentOrder{})
	var rows []orderRow
	sess.Limit(size, (page-1)*size).Find(&rows)
	c.JSON(http.StatusOK, gin.H{"orders": rows, "total": total})
}
