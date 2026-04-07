package middleware

import (
	"fmt"
	"net/http"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/service/auth"

	"github.com/gin-gonic/gin"
)

// PermissionMiddleware 权限验证中间件
func PermissionMiddleware(authSvc *auth.Service, requiredRole ...models.UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := GetUserRole(c)
		userID := GetUserID(c)

		// 检查用户角色权限
		hasPermission := false
		for _, role := range requiredRole {
			if auth.CheckPermission(userRole, role) {
				hasPermission = true
				break
			}
		}

		// 超级管理员拥有所有权限
		if userRole == string(models.UserRoleSuperAdmin) {
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
func ClusterPermissionMiddleware(authSvc *auth.Service, requiredPermission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GetUserID(c)
		userRole := GetUserRole(c)
		clusterID := c.Param("id")

		if clusterID == "" {
			c.Next()
			return
		}

		// 超级管理员拥有所有权限
		if userRole == string(models.UserRoleSuperAdmin) {
			c.Next()
			return
		}

		// 解析集群ID
		var clusterIDInt int64
		_, err := fmt.Sscanf(clusterID, "%d", &clusterIDInt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster id"})
			c.Abort()
			return
		}

		// 检查集群权限
		hasPermission, err := authSvc.CheckClusterPermission(c.Request.Context(), userID, clusterIDInt, requiredPermission)
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