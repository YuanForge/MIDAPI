package handler

import (
	"fanapi/internal/db"
	"fanapi/internal/model"
	"fmt"
	"github.com/gin-gonic/gin"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// GET /admin/coupons
func ListCoupons(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	engine := db.Engine
	total, _ := engine.Count(&model.Coupon{})
	var items []model.Coupon
	engine.OrderBy("created_at DESC").Limit(size, (page-1)*size).Find(&items)
	c.JSON(http.StatusOK, gin.H{"coupons": items, "total": total})
}

// POST /admin/coupons
func CreateCoupon(c *gin.Context) {
	var req struct {
		Code          string `json:"code"`
		Type          string `json:"type"`
		Title         string `json:"title"`
		DiscountType  string `json:"discount_type"`
		DiscountValue int64  `json:"discount_value"`
		MinAmount     int64  `json:"min_amount"`
		MaxDiscount   int64  `json:"max_discount"`
		TotalCount    int    `json:"total_count"`
		PerUserLimit  int    `json:"per_user_limit"`
		ValidFrom     string `json:"valid_from"`
		ValidUntil    string `json:"valid_until"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var validFrom *time.Time
	if t, err := parseDateTime(req.ValidFrom, false); err == nil && !t.IsZero() {
		validFrom = &t
	}
	var validUntil *time.Time
	if t, err := parseDateTime(req.ValidUntil, true); err == nil && !t.IsZero() {
		validUntil = &t
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		code = fmt.Sprintf("CPN%d%d", time.Now().Unix(), rand.Intn(10000))
	}
	cp := &model.Coupon{
		Code:          code,
		Type:          req.Type,
		Title:         req.Title,
		DiscountType:  req.DiscountType,
		DiscountValue: req.DiscountValue,
		MinAmount:     req.MinAmount,
		MaxDiscount:   req.MaxDiscount,
		TotalCount:    req.TotalCount,
		PerUserLimit:  req.PerUserLimit,
		ValidFrom:     validFrom,
		ValidUntil:    validUntil,
		CreatedBy:     getAdminID(c),
	}
	if _, err := db.Engine.Insert(cp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, cp)
}

// DELETE /admin/coupons/:id  作废整批优惠券
func VoidCoupon(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID 格式错误"})
		return
	}
	// 将有效期设为过去，达到作废效果
	db.Engine.Exec("UPDATE coupons SET valid_until=NOW()-INTERVAL '1 second' WHERE id=$1", id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GET /admin/coupons/:id/uses  优惠券使用记录
func ListCouponUses(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID 格式错误"})
		return
	}
	var uses []model.CouponUse
	db.Engine.Where("coupon_id=?", id).OrderBy("created_at DESC").Limit(100).Find(&uses)
	c.JSON(http.StatusOK, gin.H{"uses": uses})
}
