package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/gin-gonic/gin"
)

// RateLimiter 限流器
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

// NewRateLimiter 创建限流器
// rate: 每秒请求数
// burst: 突发流量上限
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

// RateLimitMiddleware 限流中间件
// 默认每用户每分钟 100 请求
func RateLimitMiddleware() gin.HandlerFunc {
	limiter := NewRateLimiter(100.0/60, 100) // 100 requests per minute

	return func(c *gin.Context) {
		// 使用用户ID作为限流key，如果是匿名请求则使用IP
		key := c.ClientIP()
		if userID := GetUserID(c); userID > 0 {
			key = "user:" + string(rune(userID))
		}

		if !limiter.getLimiter(key).Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded, please try again later",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequestSizeLimitMiddleware 请求体大小限制中间件
func RequestSizeLimitMiddleware(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)
		c.Next()
	}
}

// TimeoutMiddleware 请求超时中间件
func TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Gin 默认支持 context 超时
		c.Set("request_timeout", timeout)
		c.Next()
	}
}