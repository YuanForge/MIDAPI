package handler

import (
	"net/http"

	"fanapi/internal/config"
	"fanapi/internal/service"

	"github.com/gin-gonic/gin"
)

func abortIfResellerSiteChannelWrite(c *gin.Context) bool {
	raw, ok := c.Get("app_config")
	if !ok {
		return false
	}
	cfg, ok := raw.(*config.Config)
	if !ok || !service.IsResellerSiteMode(cfg) {
		return false
	}
	c.JSON(http.StatusForbidden, gin.H{"error": "代理站渠道只能从主站同步，不能新增、编辑、启停或删除"})
	return true
}
