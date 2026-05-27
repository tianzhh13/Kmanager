package middleware

import (
	"net/http"

	"kafka-management-platform/internal/models"

	"github.com/gin-gonic/gin"
)

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
