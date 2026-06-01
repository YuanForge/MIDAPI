package handler

import (
	"fanapi/internal/db"
	"fanapi/internal/model"
	"fanapi/internal/service"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

// POST /user/apikeys  (requires auth)
func (h *AuthHandler) CreateAPIKey(c *gin.Context) {
	var req struct {
		Name    string `json:"name" binding:"required,max=64"`
		KeyType string `json:"key_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.KeyType != "stable" {
		req.KeyType = "low_price"
	}
	userID := c.MustGet("user_id").(int64)
	rawKey, err := service.GenerateAPIKey(c.Request.Context(), userID, req.Name, req.KeyType, h.cfg.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"key": rawKey, "note": "store this key safely, it will not be shown again"})
}

// GET /user/apikeys
func (h *AuthHandler) ListAPIKeys(c *gin.Context) {
	userID := c.MustGet("user_id").(int64)
	var keys []model.APIKey
	if err := db.Engine.Where("user_id = ?", userID).
		Cols("id", "name", "key_hash", "raw_key_enc", "key_type", "is_active", "last_used_at", "created_at").
		Desc("id").
		Find(&keys); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type apiKeyItem struct {
		ID         int64       `json:"id"`
		Name       string      `json:"name"`
		KeyType    string      `json:"key_type"`
		KeyPrefix  string      `json:"key_prefix"`
		RawKey     string      `json:"raw_key"`
		Viewable   bool        `json:"viewable"`
		IsActive   bool        `json:"is_active"`
		LastUsedAt interface{} `json:"last_used_at"`
		CreatedAt  interface{} `json:"created_at"`
	}

	items := make([]apiKeyItem, 0, len(keys))
	for _, k := range keys {
		rawKey := ""
		viewable := false
		if k.RawKeyEnc != "" {
			if decrypted, err := service.DecryptAPIKey(k.RawKeyEnc, h.cfg.JWTSecret); err == nil {
				rawKey = decrypted
				viewable = true
			}
		}
		prefix := ""
		if len(k.KeyHash) >= 12 {
			prefix = k.KeyHash[:12]
		} else {
			prefix = k.KeyHash
		}
		keyType := k.KeyType
		if keyType == "" {
			keyType = "low_price"
		}
		items = append(items, apiKeyItem{
			ID:         k.ID,
			Name:       k.Name,
			KeyType:    keyType,
			KeyPrefix:  prefix,
			RawKey:     rawKey,
			Viewable:   viewable,
			IsActive:   k.IsActive,
			LastUsedAt: k.LastUsedAt,
			CreatedAt:  k.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"api_keys": items})
}

// DELETE /user/apikeys/:id
func (h *AuthHandler) DeleteAPIKey(c *gin.Context) {
	userID := c.MustGet("user_id").(int64)
	keyID := strings.TrimSpace(c.Param("id"))
	affected, err := db.Engine.Where("id = ? AND user_id = ?", keyID, userID).
		Delete(&model.APIKey{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "API Key 不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "API Key 已删除"})
}
