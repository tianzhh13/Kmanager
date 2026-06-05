package middleware

import (
	"context"
	"fmt"
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

// userStatusCacheRef 用户状态缓存引用（用于外部失效）
var userStatusCacheRef cache.Cache

// AuthMiddleware JWT 认证中间件
func AuthMiddleware(jwtSvc *jwt.Service, blacklistCache *cache.TokenBlacklistCache, userRepo repository.UserRepository, userStatusCache cache.Cache) gin.HandlerFunc {
	userStatusCacheRef = userStatusCache

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

		// 检查用户状态（禁用用户无法访问），带缓存减少数据库查询
		if userRepo != nil {
			userID := claims.UserID
			disabled := false

			// 先查缓存
			if userStatusCache != nil {
				cacheKey := fmt.Sprintf("user_status:%d", userID)
				if cached, err := userStatusCache.Get(c.Request.Context(), cacheKey); err == nil && cached != nil {
					if active, ok := cached.(bool); ok {
						disabled = !active
					}
				}
			}

			// 缓存未命中或已失效，查数据库
			if !disabled {
				user, err := userRepo.FindByID(c.Request.Context(), userID)
				active := err == nil && user.Status == models.UserStatusActive
				disabled = !active

				// 写入缓存（TTL 0 使用缓存默认值 30 秒）
				if userStatusCache != nil {
					cacheKey := fmt.Sprintf("user_status:%d", userID)
					_ = userStatusCache.Set(c.Request.Context(), cacheKey, active, 0)
				}
			}

			if disabled {
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

// InvalidateUserStatusCache 失效指定用户的认证状态缓存（用于禁用/启用用户后立即生效）
func InvalidateUserStatusCache(userID int64) {
	if userStatusCacheRef != nil {
		key := fmt.Sprintf("user_status:%d", userID)
		_ = userStatusCacheRef.Delete(context.Background(), key)
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
