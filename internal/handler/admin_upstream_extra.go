package handler

import (
	"encoding/json"
	"fanapi/internal/db"
	"fanapi/internal/model"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"time"
)

// GET /admin/upstream-platforms
func ListUpstreamPlatforms(c *gin.Context) {
	var items []model.UpstreamPlatform
	db.Engine.OrderBy("created_at DESC").Find(&items)
	c.JSON(http.StatusOK, gin.H{"platforms": items})
}

// POST /admin/upstream-platforms
func CreateUpstreamPlatform(c *gin.Context) {
	var p model.UpstreamPlatform
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 简单存储（生产中 APIKey 应加密）
	if _, err := db.Engine.Insert(&p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	p.APIKeyEnc = "" // 不返回
	c.JSON(http.StatusCreated, p)
}

// PUT /admin/upstream-platforms/:id
func UpdateUpstreamPlatform(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID 格式错误"})
		return
	}
	var p model.UpstreamPlatform
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p.ID = id
	db.Engine.ID(id).AllCols().Update(&p)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DELETE /admin/upstream-platforms/:id
func DeleteUpstreamPlatform(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID 格式错误"})
		return
	}
	db.Engine.Delete(&model.UpstreamPlatform{ID: id})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GET /admin/upstream-platforms/:id/models  拉取上游可用模型列表
func GetUpstreamModels(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID 格式错误"})
		return
	}
	var p model.UpstreamPlatform
	if found, _ := db.Engine.ID(id).Get(&p); !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "平台不存在"})
		return
	}
	baseURL := p.BaseURL
	if baseURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "平台未配置 Base URL"})
		return
	}
	apiKey := p.APIKeyEnc // stored as plaintext for now

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", baseURL+"/v1/models", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "请求上游失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("上游响应 %d", resp.StatusCode)})
		return
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "解析上游响应失败: " + err.Error()})
		return
	}

	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		if m.ID != "" {
			models = append(models, m.ID)
		}
	}
	c.JSON(http.StatusOK, gin.H{"models": models})
}

// POST /admin/channels/batch-from-upstream  从上游平台一键批量创建渠道
func BatchCreateChannelsFromUpstream(c *gin.Context) {
	var req struct {
		PlatformID int64    `json:"platform_id"`
		Models     []string `json:"models"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.PlatformID == 0 || len(req.Models) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platform_id 和 models 为必填"})
		return
	}
	var p model.UpstreamPlatform
	if found, _ := db.Engine.ID(req.PlatformID).Get(&p); !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "平台不存在"})
		return
	}

	created := 0
	for _, modelName := range req.Models {
		ch := &model.Channel{
			Name:        p.Name + " - " + modelName,
			Model:       modelName,
			Type:        "llm",
			BaseURL:     p.BaseURL + "/v1/chat/completions",
			Method:      "POST",
			BillingType: "token",
			Protocol:    "openai",
			IsActive:    true,
			Weight:      1,
		}
		// Set Authorization header
		if p.APIKeyEnc != "" {
			ch.Headers = model.JSON{"Authorization": "Bearer " + p.APIKeyEnc}
		}
		if _, err := db.Engine.Insert(ch); err == nil {
			created++
		}
	}
	c.JSON(http.StatusCreated, gin.H{"created": created})
}
