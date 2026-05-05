package middleware

import (
	"net/http"

	"fanapi/internal/db"
	"fanapi/internal/model"

	"github.com/gin-gonic/gin"
)

// Admin checks that the authenticated user has the "admin" role.
func Admin() gin.HandlerFunc {
	return func(c *gin.Context) {
		// If JWT, role is already in context
		if role, exists := c.Get("role"); exists && role == "admin" {
			c.Next()
			return
		}

		// If API key auth, load role from DB
		userID, exists := c.Get("user_id")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "无访问权限"})
			return
		}

		user := &model.User{}
		found, err := db.Engine.Where("id = ?", userID).Cols("role").Get(user)
		if err != nil || !found || user.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
			return
		}
		c.Set("role", "admin")
		c.Next()
	}
}

// AdminOrOperator allows both "admin" and "operator" roles.
func AdminOrOperator() gin.HandlerFunc {
	return func(c *gin.Context) {
		if role, exists := c.Get("role"); exists {
			if role == "admin" || role == "operator" {
				c.Next()
				return
			}
		}

		userID, exists := c.Get("user_id")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "无访问权限"})
			return
		}

		user := &model.User{}
		found, err := db.Engine.Where("id = ?", userID).Cols("role").Get(user)
		if err != nil || !found || (user.Role != "admin" && user.Role != "operator") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "需要管理员或运营权限"})
			return
		}
		c.Set("role", user.Role)
		c.Next()
	}
}
