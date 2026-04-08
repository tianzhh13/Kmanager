package middleware

import (
	"net/http"
	"strconv"

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

// ClusterPermissionMiddleware 集群级别权限检查中间件
func ClusterPermissionMiddleware(permissionSvc *auth.PermissionService, requiredPermission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GetUserID(c)
		userRole := GetUserRole(c)
		clusterIDStr := c.Param("id")

		if clusterIDStr == "" {
			c.Next()
			return
		}

		// 超级管理员拥有所有权限
		if userRole == string(models.RoleSuperAdmin) {
			c.Next()
			return
		}

		// 解析集群ID
		clusterID, err := strconv.ParseInt(clusterIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster id"})
			c.Abort()
			return
		}

		// 检查集群权限
		hasPermission, err := permissionSvc.CheckClusterPermission(c.Request.Context(), userID, clusterID)
		if err != nil || !hasPermission {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "no permission for this cluster",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}