package middleware

import (
	"net/http"
	"strconv"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/service/auth"

	"github.com/gin-gonic/gin"
)

// TopicPermissionMiddleware Topic 权限检查中间件
func TopicPermissionMiddleware(permissionSvc *auth.PermissionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GetUserID(c)
		userRole := GetUserRole(c)

		// 超级管理员直接通过
		if userRole == string(models.RoleSuperAdmin) {
			c.Next()
			return
		}

		// 获取集群 ID
		clusterIDStr := c.Param("cluster_id")
		if clusterIDStr == "" {
			clusterIDStr = c.Query("cluster_id")
		}
		if clusterIDStr == "" {
			c.Next()
			return
		}

		clusterID, err := strconv.ParseInt(clusterIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster id"})
			c.Abort()
			return
		}

		// 集群管理员检查集群权限
		if userRole == string(models.RoleClusterAdmin) {
			hasPermission, err := permissionSvc.CheckClusterPermission(c.Request.Context(), userID, clusterID)
			if err != nil || !hasPermission {
				c.JSON(http.StatusForbidden, gin.H{"error": "no permission for this cluster"})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		// 普通用户检查集群读权限
		if userRole == string(models.RoleNormalUser) {
			hasPermission, err := permissionSvc.CheckClusterReadPermission(c.Request.Context(), userID, clusterID)
			if err != nil || !hasPermission {
				c.JSON(http.StatusForbidden, gin.H{"error": "no permission for this cluster"})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		c.Abort()
	}
}

// RequireSuperAdminOrClusterAdmin 要求超级管理员或集群管理员权限
func RequireSuperAdminOrClusterAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := GetUserRole(c)

		if userRole == string(models.RoleSuperAdmin) || userRole == string(models.RoleClusterAdmin) {
			c.Next()
			return
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		c.Abort()
	}
}

// RequireNormalUser 要求普通用户权限（仅普通用户可访问）
func RequireNormalUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := GetUserRole(c)

		if userRole == string(models.RoleNormalUser) {
			c.Next()
			return
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "this endpoint is for normal users only"})
		c.Abort()
	}
}
