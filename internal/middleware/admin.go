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
