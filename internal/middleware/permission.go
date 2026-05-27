package middleware

import (
	"net/http"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/service/auth"

	"github.com/gin-gonic/gin"
)

// PermissionMiddleware 权限验证中间件
func PermissionMiddleware(permissionSvc *auth.PermissionService, requiredRole ...models.UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := GetUserRole(c)

		// 检查用户角色权限
		hasPermission := false
		for _, role := range requiredRole {
			if userRole == string(role) {
				hasPermission = true
				break
			}
		}

		// 超级管理员拥有所有权限
		if userRole == string(models.RoleSuperAdmin) {
			hasPermission = true
		}

		if !hasPermission {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "insufficient permissions",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
