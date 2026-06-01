package handler

import (
	"github.com/gin-gonic/gin"
	"strings"
)

// clientIP 从请求头获取真实客户端 IP。
func clientIP(c *gin.Context) string {
	ip := c.GetHeader("X-Forwarded-For")
	if idx := strings.Index(ip, ","); idx != -1 {
		ip = ip[:idx]
	}
	ip = strings.TrimSpace(ip)
	if ip == "" {
		ip = c.ClientIP()
	}
	return ip
}
