package handler

import (
	"fanapi/internal/db"
	"fanapi/internal/model"
	"fmt"
	"github.com/gin-gonic/gin"
	"math"
	"net/http"
	"strings"
	"time"
)

// ValidateCoupon 验证优惠券并返回折扣信息。
// GET /user/coupons/validate?code=xxx&amount=xxx
func ValidateCoupon(c *gin.Context) {
	code := strings.TrimSpace(c.Query("code"))
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入优惠券码"})
		return
	}
	var amountYuan float64
	fmt.Sscanf(c.DefaultQuery("amount", "0"), "%f", &amountYuan)

	userID := c.MustGet("user_id").(int64)

	now := time.Now()
	var cp model.Coupon
	found, err := db.Engine.Where("code = ?", code).Get(&cp)
	if err != nil || !found {
		c.JSON(http.StatusBadRequest, gin.H{"error": "优惠券不存在"})
		return
	}
	if cp.ValidFrom != nil && now.Before(*cp.ValidFrom) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "优惠券尚未生效"})
		return
	}
	if cp.ValidUntil != nil && now.After(*cp.ValidUntil) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "优惠券已过期"})
		return
	}
	if cp.TotalCount > 0 && cp.UsedCount >= cp.TotalCount {
		c.JSON(http.StatusBadRequest, gin.H{"error": "优惠券已达使用上限"})
		return
	}
	// 检查每人使用次数
	if cp.PerUserLimit > 0 {
		cnt, _ := db.Engine.Where("coupon_id=? AND user_id=?", cp.ID, userID).Count(&model.CouponUse{})
		if int(cnt) >= cp.PerUserLimit {
			c.JSON(http.StatusBadRequest, gin.H{"error": "您已达到该优惠券使用次数上限"})
			return
		}
	}
	// 最低消费检查（单位：分 → 元）
	minAmountYuan := float64(cp.MinAmount) / 100.0
	if amountYuan > 0 && minAmountYuan > 0 && amountYuan < minAmountYuan {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("该优惠券最低消费 ¥%.2f", minAmountYuan),
		})
		return
	}

	// 计算折扣（percent: discount_value 为折扣比例的百分比×100存储，如 500=5%；fixed: 分为单位）
	var discountYuan float64
	if cp.DiscountType == "percent" {
		discountYuan = amountYuan * float64(cp.DiscountValue) / 10000.0
	} else {
		discountYuan = float64(cp.DiscountValue) / 100.0
	}
	if cp.MaxDiscount > 0 {
		maxYuan := float64(cp.MaxDiscount) / 100.0
		if discountYuan > maxYuan {
			discountYuan = maxYuan
		}
	}
	if discountYuan < 0 {
		discountYuan = 0
	}
	finalAmount := amountYuan - discountYuan
	if finalAmount < 0.01 {
		finalAmount = 0.01
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":          true,
		"coupon_id":      cp.ID,
		"discount_yuan":  math.Round(discountYuan*100) / 100,
		"final_amount":   math.Round(finalAmount*100) / 100,
		"discount_type":  cp.DiscountType,
		"discount_value": cp.DiscountValue,
	})
}
