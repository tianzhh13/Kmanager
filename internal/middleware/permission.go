package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/service/auth"

	"github.com/gin-gonic/gin"
)

// PermissionMiddleware 权限验证中间件
type PermissionMiddleware struct {
	permissionSvc *auth.PermissionService
}

// NewPermissionMiddleware 创建权限验证中间件
func NewPermissionMiddleware(permissionSvc *auth.PermissionService) *PermissionMiddleware {
	return &PermissionMiddleware{
		permissionSvc: permissionSvc,
	}
}

// RequireRole 创建角色验证中间件
func (m *PermissionMiddleware) RequireRole(roles ...models.UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := GetUserRole(c)
		
		for _, role := range roles {
			if userRole == role {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"error": "insufficient permissions",
		})
		c.Abort()
	}
}

// RequireSuperAdmin 超级管理员权限中间件
func (m *PermissionMiddleware) RequireSuperAdmin() gin.HandlerFunc {
	return m.RequireRole(models.RoleSuperAdmin)
}

// RequireClusterAdmin 集群管理员权限中间件
func (m *PermissionMiddleware) RequireClusterAdmin() gin.HandlerFunc {
	return m.RequireRole(models.RoleSuperAdmin, models.RoleClusterAdmin)
}

// ClusterPermissionMiddleware 集群权限验证中间件
type ClusterPermissionMiddleware struct {
	permissionSvc *auth.PermissionService
}

// NewClusterPermissionMiddleware 创建集群权限验证中间件
func NewClusterPermissionMiddleware(permissionSvc *auth.PermissionService) *ClusterPermissionMiddleware {
	return &ClusterPermissionMiddleware{
		permissionSvc: permissionSvc,
	}
}

// RequireClusterAccess 验证用户是否有集群访问权限
func (m *ClusterPermissionMiddleware) RequireClusterAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterIDStr := c.Param("id")
		if clusterIDStr == "" {
			clusterIDStr = c.Query("cluster_id")
		}

		if clusterIDStr == "" {
			c.Next()
			return
		}

		var clusterID int64
		if _, err := fmt.Sscanf(clusterIDStr, "%d", &clusterID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid cluster id",
			})
			c.Abort()
			return
		}

		userID := GetUserID(c)
		userRole := GetUserRole(c)

		// 超级管理员拥有所有权限
		if userRole == models.RoleSuperAdmin {
			c.Next()
			return
		}

		// 只读用户可以访问
		if userRole == models.RoleReadOnly {
			hasAccess, err := m.permissionSvc.CheckClusterReadPermission(c.Request.Context(), userID, clusterID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "failed to check permission",
				})
				c.Abort()
				return
			}
			if !hasAccess {
				c.JSON(http.StatusForbidden, gin.H{
					"error": "no access to this cluster",
				})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		// 集群管理员需要检查是否被授权
		hasAccess, err := m.permissionSvc.CheckClusterPermission(c.Request.Context(), userID, clusterID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to check permission",
			})
			c.Abort()
			return
		}

		if !hasAccess {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "no access to this cluster",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireClusterWriteAccess 验证用户是否有集群写权限
func (m *ClusterPermissionMiddleware) RequireClusterWriteAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterIDStr := c.Param("id")
		if clusterIDStr == "" {
			clusterIDStr = c.Query("cluster_id")
		}

		if clusterIDStr == "" {
			c.Next()
			return
		}

		var clusterID int64
		if _, err := fmt.Sscanf(clusterIDStr, "%d", &clusterID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid cluster id",
			})
			c.Abort()
			return
		}

		userID := GetUserID(c)
		userRole := GetUserRole(c)

		// 超级管理员拥有所有权限
		if userRole == models.RoleSuperAdmin {
			c.Next()
			return
		}

		// 只读用户没有写权限
		if userRole == models.RoleReadOnly {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "read-only user cannot perform this operation",
			})
			c.Abort()
			return
		}

		// 集群管理员需要检查是否被授权
		hasAccess, err := m.permissionSvc.CheckClusterPermission(c.Request.Context(), userID, clusterID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to check permission",
			})
			c.Abort()
			return
		}

		if !hasAccess {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "no access to this cluster",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// ActionPermissionMiddleware 操作权限验证中间件
type ActionPermissionMiddleware struct {
	permissionSvc *auth.PermissionService
}

// NewActionPermissionMiddleware 创建操作权限验证中间件
func NewActionPermissionMiddleware(permissionSvc *auth.PermissionService) *ActionPermissionMiddleware {
	return &ActionPermissionMiddleware{
		permissionSvc: permissionSvc,
	}
}

// RequireAction 验证用户是否有指定操作的权限
func (m *ActionPermissionMiddleware) RequireAction(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GetUserID(c)
		userRole := GetUserRole(c)

		// 超级管理员拥有所有权限
		if userRole == models.RoleSuperAdmin {
			c.Next()
			return
		}

		// 解析资源类型
		resource := extractResourceFromPath(c.Request.URL.Path)

		hasPermission, err := m.permissionSvc.CheckPermission(c.Request.Context(), userID, resource, action)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to check permission",
			})
			c.Abort()
			return
		}

		if !hasPermission {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "no permission to perform this action",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// extractResourceFromPath 从请求路径中提取资源类型
func extractResourceFromPath(path string) string {
	// 移除前缀
	path = strings.TrimPrefix(path, "/api/v1/")

	// 提取资源类型
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		switch parts[0] {
		case "clusters":
			return "cluster"
		case "topics":
			return "topic"
		case "acls":
			return "acl"
		case "users":
			return "user"
		case "metrics":
			return "metrics"
		case "audit-logs":
			return "audit_log"
		}
	}

	return ""
}