package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/gin-gonic/gin"
)

// SecurityHeadersMiddleware 安全响应头中间件
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 防止点击劫持
		c.Header("X-Frame-Options", "DENY")
		// 防止 MIME 类型嗅探
		c.Header("X-Content-Type-Options", "nosniff")
		// XSS 保护
		c.Header("X-XSS-Protection", "1; mode=block")
		// 禁用缓存（敏感数据）
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
		c.Header("Pragma", "no-cache")
		// 内容安全策略
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'")

		c.Next()
	}
}

// RequestBodySizeLimitMiddleware 请求体大小限制中间件
func RequestBodySizeLimitMiddleware(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 默认最大 10MB
		if maxSize <= 0 {
			maxSize = 10 * 1024 * 1024
		}

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)

		c.Next()
	}
}

// RateLimiter 限流器
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

// NewRateLimiter 创建限流器
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(rps),
		burst:    burst,
	}
}

// getLimiter 获取或创建用户的限流器
func (r *RateLimiter) getLimiter(key string) *rate.Limiter {
	r.mu.RLock()
	limiter, exists := r.limiters[key]
	r.mu.RUnlock()

	if exists {
		return limiter
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 双重检查
	if limiter, exists = r.limiters[key]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(r.rate, r.burst)
	r.limiters[key] = limiter

	return limiter
}

// Allow 检查是否允许请求
func (r *RateLimiter) Allow(key string) bool {
	limiter := r.getLimiter(key)
	return limiter.Allow()
}

// RateLimitMiddleware 限流中间件
// 每用户每分钟 100 请求 = 每秒约 1.67 请求，burst 设置为 20
func RateLimitMiddleware() gin.HandlerFunc {
	// 每分钟 100 请求 = 每秒 1.67 请求，burst 设为 20 允许突发
	limiter := NewRateLimiter(1.67, 20)

	return func(c *gin.Context) {
		// 使用用户 ID 作为限流 key，如果没有则使用 IP
		key := c.ClientIP()
		if userID, exists := c.Get("user_id"); exists {
			key = fmt.Sprintf("user:%d", userID)
		}

		if !limiter.Allow(key) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"message":     "too many requests, please try again later",
				"retry_after": 60,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// IPRateLimitMiddleware IP 限流中间件（用于登录等公共端点）
func IPRateLimitMiddleware(requestsPerMinute int) gin.HandlerFunc {
	// 转换为每秒请求数
	rps := float64(requestsPerMinute) / 60.0
	limiter := NewRateLimiter(rps, requestsPerMinute/10)

	return func(c *gin.Context) {
		key := c.ClientIP()

		if !limiter.Allow(key) {
			retryAfter := 60
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "too many requests",
				"message":     "too many requests from this IP, please try again later",
				"retry_after": retryAfter,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequestIDMiddleware 请求 ID 中间件
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)

		c.Next()
	}
}

// generateRequestID 生成请求 ID
func generateRequestID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

// TimeoutMiddleware 请求超时中间件
func TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Writer.Header().Get("X-Content-Type-Options") == "" {
			c.Header("X-Content-Type-Options", "nosniff")
		}

		c.Next()
	}
}

// HSTSMiddleware HSTS 中间件（生产环境使用）
func HSTSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 仅在 HTTPS 请求时设置
		if c.Request.TLS != nil {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

// RefererPolicyMiddleware Referer 策略中间件
func RefererPolicyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	}
}

// PermissionsPolicyMiddleware 权限策略中间件
func PermissionsPolicyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		c.Next()
	}
}