package middleware

import (
	"net/http"
	"strings"

	"kafka-management-platform/internal/cache"
	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/repository"
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
	// ContextKeyToken 原始 Token 键名
	ContextKeyToken = "raw_token"
)

// AuthMiddleware JWT 认证中间件
func AuthMiddleware(jwtSvc *jwt.Service, blacklistCache *cache.TokenBlacklistCache, userRepo repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenString string

		// 优先从 Authorization Header 获取 Token
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenString = parts[1]
			}
		}

		// Header 中没有 Token，尝试从 httpOnly Cookie 读取
		if tokenString == "" {
			if cookie, err := c.Cookie("access_token"); err == nil && cookie != "" {
				tokenString = cookie
			}
		}

		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
			c.Abort()
			return
		}

		// 检查 Token 是否在黑名单中
		if blacklistCache != nil {
			isBlacklisted, err := blacklistCache.IsBlacklisted(c.Request.Context(), tokenString)
			if err == nil && isBlacklisted {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked"})
				c.Abort()
				return
			}
		}

		// 验证 Token
		claims, err := jwtSvc.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		// 检查用户状态（禁用用户无法访问）
		if userRepo != nil {
			user, err := userRepo.FindByID(c.Request.Context(), claims.UserID)
			if err != nil || user.Status != models.UserStatusActive {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "account is disabled"})
				c.Abort()
				return
			}
		}

		// 将用户信息注入 Context
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyUsername, claims.Username)
		c.Set(ContextKeyUserRole, claims.Role)
		c.Set(ContextKeyToken, tokenString) // 保存原始 Token，用于退出登录时加入黑名单

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

// GetRawToken 获取当前请求的原始 Token
func GetRawToken(c *gin.Context) string {
	if v, exists := c.Get(ContextKeyToken); exists {
		return v.(string)
	}
	return ""
}
