package middleware

import (
	"strings"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/pkg/jwt"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware JWT 认证中间件
func AuthMiddleware(jwtSvc *jwt.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Header 获取 Token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(401, gin.H{
				"error": "missing authorization header",
			})
			c.Abort()
			return
		}

		// 解析 Bearer Token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(401, gin.H{
				"error": "invalid authorization header format",
			})
			c.Abort()
			return
		}

		// 验证 Token
		claims, err := jwtSvc.ValidateToken(parts[1])
		if err != nil {
			c.JSON(401, gin.H{
				"error": "invalid or expired token",
			})
			c.Abort()
			return
		}

		// 将用户信息存入 Context
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// GetUserID 从 Context 获取用户 ID
func GetUserID(c *gin.Context) int64 {
	if userID, exists := c.Get("user_id"); exists {
		return userID.(int64)
	}
	return 0
}

// GetUsername 从 Context 获取用户名
func GetUsername(c *gin.Context) string {
	if username, exists := c.Get("username"); exists {
		return username.(string)
	}
	return ""
}

// GetUserRole 从 Context 获取用户角色
func GetUserRole(c *gin.Context) models.UserRole {
	if role, exists := c.Get("role"); exists {
		return role.(models.UserRole)
	}
	return ""
}
