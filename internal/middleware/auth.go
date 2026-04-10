package middleware

import (
	"net/http"
	"strings"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/pkg/jwt"

	"github.com/gin-gonic/gin"
)

const (
	// ContextKeyUserID 用户ID键名
	ContextKeyUserID = "user_id"
	// ContextKeyUsername 用户名键名
	ContextKeyUsername = "username"
	// ContextKeyUserRole 用户角色键名
	ContextKeyUserRole = "user_role"
)

// AuthMiddleware JWT 认证中间件
func AuthMiddleware(jwtSvc *jwt.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Header 获取 Token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			c.Abort()
			return
		}

		// 解析 Bearer Token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// 验证 Token
		claims, err := jwtSvc.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		// 将用户信息注入 Context
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyUsername, claims.Username)
		c.Set(ContextKeyUserRole, claims.Role)

		c.Next()
	}
}

// GetUserID 获取当前用户ID
func GetUserID(c *gin.Context) int64 {
	if v, exists := c.Get(ContextKeyUserID); exists {
		return v.(int64)
	}
	return 0
}

// GetUsername 获取当前用户名
func GetUsername(c *gin.Context) string {
	if v, exists := c.Get(ContextKeyUsername); exists {
		return v.(string)
	}
	return ""
}

// GetUserRole 获取当前用户角色
func GetUserRole(c *gin.Context) string {
	if v, exists := c.Get(ContextKeyUserRole); exists {
		// 处理 models.UserRole 类型
		if role, ok := v.(models.UserRole); ok {
			return string(role)
		}
		// 兼容 string 类型
		if role, ok := v.(string); ok {
			return role
		}
	}
	return ""
}