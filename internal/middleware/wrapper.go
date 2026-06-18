package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/service/audit"
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

// resolveClusterID 从请求中解析集群 ID
// fromBody: 是否尝试从请求体中读取（写权限场景）
func resolveClusterID(ctx *gin.Context, fromBody bool) string {
	// 1. URL 路径参数
	if id := ctx.Param("id"); id != "" {
		return id
	}
	// 2. 查询参数
	if id := ctx.Query("cluster_id"); id != "" {
		return id
	}
	// 3. 请求体（仅写权限场景）
	if fromBody && ctx.Request.Body != nil {
		bodyBytes, err := io.ReadAll(ctx.Request.Body)
		if err == nil && len(bodyBytes) > 0 {
			ctx.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			var body struct {
				ClusterID int64 `json:"cluster_id"`
			}
			if err := json.Unmarshal(bodyBytes, &body); err == nil && body.ClusterID > 0 {
				return strconv.FormatInt(body.ClusterID, 10)
			}
		}
	}
	return ""
}

// checkClusterAccess 检查集群访问权限
func (c *ClusterPermissionMiddlewareWrapper) checkClusterAccess(ctx *gin.Context, userID int64, clusterID int64, userRole string, writeMode bool) bool {
	var hasPermission bool
	var err error
	if writeMode {
		hasPermission, err = c.permissionSvc.CheckClusterPermission(ctx.Request.Context(), userID, clusterID, userRole)
	} else if userRole == string(models.RoleNormalUser) {
		hasPermission, err = c.permissionSvc.CheckClusterReadPermission(ctx.Request.Context(), userID, clusterID, userRole)
	} else {
		hasPermission, err = c.permissionSvc.CheckClusterPermission(ctx.Request.Context(), userID, clusterID, userRole)
	}
	return err == nil && hasPermission
}

// clusterAccessMiddleware 集群权限中间件的通用实现
func (c *ClusterPermissionMiddlewareWrapper) clusterAccessMiddleware(writeMode bool, allowMissing bool) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		clusterIDStr := resolveClusterID(ctx, writeMode)

		if clusterIDStr == "" {
			if allowMissing {
				ctx.Next()
				return
			}
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "cluster id required"})
			ctx.Abort()
			return
		}

		userRole := GetUserRole(ctx)
		if userRole == string(models.RoleSuperAdmin) {
			ctx.Next()
			return
		}

		clusterID, err := strconv.ParseInt(clusterIDStr, 10, 64)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid cluster id"})
			ctx.Abort()
			return
		}

		userID := GetUserID(ctx)
		if !c.checkClusterAccess(ctx, userID, clusterID, userRole, writeMode) {
			ctx.JSON(http.StatusForbidden, gin.H{"error": "no permission for this cluster"})
			ctx.Abort()
			return
		}

		ctx.Next()
	}
}

// RequireClusterAccess 要求集群访问权限
func (c *ClusterPermissionMiddlewareWrapper) RequireClusterAccess() gin.HandlerFunc {
	return c.clusterAccessMiddleware(false, true)
}

// RequireClusterWriteAccess 要求集群写权限
func (c *ClusterPermissionMiddlewareWrapper) RequireClusterWriteAccess() gin.HandlerFunc {
	return c.clusterAccessMiddleware(true, false)
}

// AuditMiddlewareWrapper 审计中间件包装器
type AuditMiddlewareWrapper struct {
	auditSvc *audit.Service
}

// NewAuditMiddleware 创建审计中间件包装器
func NewAuditMiddleware(auditSvc *audit.Service) *AuditMiddlewareWrapper {
	return &AuditMiddlewareWrapper{
		auditSvc: auditSvc,
	}
}

// Audit 返回审计中间件
func (a *AuditMiddlewareWrapper) Audit() gin.HandlerFunc {
	return AuditMiddleware(a.auditSvc)
}

// IPRateLimiter IP 级别限流器
type IPRateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
	stopCh   chan struct{}
}

// NewIPRateLimiter 创建 IP 限流器
func NewIPRateLimiter(requestsPerMinute int) *IPRateLimiter {
	rl := &IPRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(float64(requestsPerMinute) / 60.0),
		burst:    requestsPerMinute,
		stopCh:   make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

// Stop 停止清理 goroutine
func (r *IPRateLimiter) Stop() {
	close(r.stopCh)
}

// cleanupLoop 定期清理过期 IP 的限流器，防止 map 无限增长
func (r *IPRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.mu.Lock()
			now := time.Now()
			for ip, limiter := range r.limiters {
				// 如果限流器的预留令牌已满（说明该 IP 长时间无请求），则删除
				if limiter.AllowN(now, 0) && limiter.Tokens() == float64(r.burst) {
					delete(r.limiters, ip)
				}
			}
			r.mu.Unlock()
		case <-r.stopCh:
			return
		}
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
