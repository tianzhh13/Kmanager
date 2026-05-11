package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"sync"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/service/auth"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// PermissionMiddlewareWrapper 权限中间件包装器，支持链式调用
type PermissionMiddlewareWrapper struct {
	permissionSvc *auth.PermissionService
}

// NewPermissionMiddleware 创建权限中间件包装器
func NewPermissionMiddleware(permissionSvc *auth.PermissionService) *PermissionMiddlewareWrapper {
	return &PermissionMiddlewareWrapper{
		permissionSvc: permissionSvc,
	}
}

// RequireSuperAdmin 要求超级管理员权限
func (p *PermissionMiddlewareWrapper) RequireSuperAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := GetUserRole(c)
		if userRole != string(models.RoleSuperAdmin) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "super admin permission required",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// ClusterPermissionMiddlewareWrapper 集群权限中间件包装器
type ClusterPermissionMiddlewareWrapper struct {
	permissionSvc *auth.PermissionService
}

// NewClusterPermissionMiddleware 创建集群权限中间件包装器
func NewClusterPermissionMiddleware(permissionSvc *auth.PermissionService) *ClusterPermissionMiddlewareWrapper {
	return &ClusterPermissionMiddlewareWrapper{
		permissionSvc: permissionSvc,
	}
}

// RequireClusterAccess 要求集群访问权限
func (c *ClusterPermissionMiddlewareWrapper) RequireClusterAccess() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		userID := GetUserID(ctx)
		userRole := GetUserRole(ctx)
		var clusterIDStr string

		// 1. 先从 URL 路径参数获取
		clusterIDStr = ctx.Param("id")

		// 2. 尝试从查询参数获取
		if clusterIDStr == "" {
			clusterIDStr = ctx.Query("cluster_id")
		}

		// 如果没有 cluster_id，直接放行（由 handler 处理）
		if clusterIDStr == "" {
			ctx.Next()
			return
		}

		// 超级管理员拥有所有权限
		if userRole == string(models.RoleSuperAdmin) {
			ctx.Next()
			return
		}

		// 解析集群ID
		clusterID, err := strconv.ParseInt(clusterIDStr, 10, 64)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster id"})
			ctx.Abort()
			return
		}

		// 普通用户使用读权限检查（检查 cluster_user_relation）
		var hasPermission bool
		if userRole == string(models.RoleNormalUser) {
			hasPermission, err = c.permissionSvc.CheckClusterReadPermission(ctx.Request.Context(), userID, clusterID)
		} else {
			hasPermission, err = c.permissionSvc.CheckClusterPermission(ctx.Request.Context(), userID, clusterID)
		}
		if err != nil || !hasPermission {
			ctx.JSON(http.StatusForbidden, gin.H{
				"error": "no permission for this cluster",
			})
			ctx.Abort()
			return
		}

		ctx.Next()
	}
}

// RequireClusterWriteAccess 要求集群写权限
func (c *ClusterPermissionMiddlewareWrapper) RequireClusterWriteAccess() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		userID := GetUserID(ctx)
		userRole := GetUserRole(ctx)
		var clusterIDStr string

		// 1. 先从 URL 路径参数获取
		clusterIDStr = ctx.Param("id")

		// 2. 尝试从查询参数获取
		if clusterIDStr == "" {
			clusterIDStr = ctx.Query("cluster_id")
		}

		// 3. 尝试从请求体获取 cluster_id
		if clusterIDStr == "" && ctx.Request.Body != nil {
			// 读取请求体
			bodyBytes, err := io.ReadAll(ctx.Request.Body)
			if err == nil && len(bodyBytes) > 0 {
				// 恢复请求体供后续使用
				ctx.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

				// 解析 JSON 获取 cluster_id
				var body struct {
					ClusterID int64 `json:"cluster_id"`
				}
				if err := json.Unmarshal(bodyBytes, &body); err == nil && body.ClusterID > 0 {
					clusterIDStr = strconv.FormatInt(body.ClusterID, 10)
				}
			}
		}

		if clusterIDStr == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "cluster id required"})
			ctx.Abort()
			return
		}

		// 超级管理员拥有所有权限
		if userRole == string(models.RoleSuperAdmin) {
			ctx.Next()
			return
		}

		// 解析集群ID
		clusterID, err := strconv.ParseInt(clusterIDStr, 10, 64)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster id"})
			ctx.Abort()
			return
		}

		// 检查集群权限
		hasPermission, err := c.permissionSvc.CheckClusterPermission(ctx.Request.Context(), userID, clusterID)
		if err != nil || !hasPermission {
			ctx.JSON(http.StatusForbidden, gin.H{
				"error": "no permission for this cluster",
			})
			ctx.Abort()
			return
		}

		ctx.Next()
	}
}

// AuditMiddlewareWrapper 审计中间件包装器
type AuditMiddlewareWrapper struct {
	auditSvc any
}

// NewAuditMiddleware 创建审计中间件包装器
func NewAuditMiddleware(auditSvc any) *AuditMiddlewareWrapper {
	return &AuditMiddlewareWrapper{
		auditSvc: auditSvc,
	}
}

// Audit 返回审计中间件
func (a *AuditMiddlewareWrapper) Audit() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// IPRateLimiter IP 级别限流器
type IPRateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

// NewIPRateLimiter 创建 IP 限流器
func NewIPRateLimiter(requestsPerMinute int) *IPRateLimiter {
	return &IPRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(float64(requestsPerMinute) / 60.0),
		burst:    requestsPerMinute,
	}
}

// getLimiter 获取或创建 IP 的限流器
func (r *IPRateLimiter) getLimiter(ip string) *rate.Limiter {
	r.mu.RLock()
	limiter, exists := r.limiters[ip]
	r.mu.RUnlock()

	if exists {
		return limiter
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if limiter, exists = r.limiters[ip]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(r.rate, r.burst)
	r.limiters[ip] = limiter
	return limiter
}

// IPRateLimitMiddleware IP 级别限流中间件
func IPRateLimitMiddleware(requestsPerMinute int) gin.HandlerFunc {
	limiter := NewIPRateLimiter(requestsPerMinute)

	return func(c *gin.Context) {
		ip := c.ClientIP()

		if !limiter.getLimiter(ip).Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded, please try again later",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
