package handler

import (
	"fanapi/internal/service"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

// POST /admin/users/:id/recharge — 为用户手动充值（直接填写 credits 数量）
func Recharge(c *gin.Context) {
	targetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID 格式错误"})
		return
	}
	adminID := c.MustGet("user_id").(int64)

	var req struct {
		Amount int64 `json:"amount" binding:"required,gt=0"` // credits 数量
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := service.Recharge(c.Request.Context(), targetID, adminID, req.Amount); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"credited_credits": req.Amount,
		"credited_cny":     float64(req.Amount) / 1_000_000,
	})
}

// POST /admin/users/:id/model-credits — 为用户赠送专属模型积分
func GrantModelCredit(c *gin.Context) {
	targetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID 格式错误"})
		return
	}
	var req struct {
		ModelName string `json:"model_name" binding:"required"`
		Credits   int64  `json:"credits" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := service.GrantModelCredit(c.Request.Context(), targetID, req.ModelName, req.Credits); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"model_name":       req.ModelName,
		"credited_credits": req.Credits,
		"credited_cny":     float64(req.Credits) / 1_000_000,
	})
}

// GET /admin/users/:id/model-credits — 查询用户的专属模型积分列表
func AdminListModelCredits(c *gin.Context) {
	targetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID 格式错误"})
		return
	}
	records, err := service.ListModelCredits(c.Request.Context(), targetID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"model_credits": records})
}
