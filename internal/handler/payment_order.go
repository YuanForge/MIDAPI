package handler

import (
	"fanapi/internal/db"
	"fanapi/internal/model"
	"github.com/gin-gonic/gin"
	"net/http"
)

// GetPaymentOrderStatus 查询单笔订单支付状态（仅订单所有者可访问）。
// GET /pay/order/status?out_trade_no=xxx
func GetPaymentOrderStatus(c *gin.Context) {
	outTradeNo := c.Query("out_trade_no")
	if outTradeNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 out_trade_no 参数"})
		return
	}
	userID := c.MustGet("user_id").(int64)

	order := &model.PaymentOrder{}
	found, err := db.Engine.Where("out_trade_no = ? AND user_id = ?", outTradeNo, userID).Get(order)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询订单失败"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "订单不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": order.Status})
}
