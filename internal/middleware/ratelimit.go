package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/gin-gonic/gin"
)

// limiterEntry 存储限流器及其最后访问时间
type limiterEntry struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// RateLimiter 限流器
type RateLimiter struct {
	limiters    map[string]*limiterEntry
	mu          sync.RWMutex
	rate        rate.Limit
	burst       int
	cleanupDone chan struct{}
}

// NewRateLimiter 创建限流器
// rate: 每秒请求数
// burst: 突发流量上限
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		limiters:    make(map[string]*limiterEntry),
		rate:        rate.Limit(rps),
		burst:       burst,
		cleanupDone: make(chan struct{}),
	}

	// 启动后台清理goroutine，每2分钟清理一次
	go rl.cleanupLoop()

	return rl
}

// Stop 停止限流器后台清理 goroutine
func (r *RateLimiter) Stop() {
	select {
	case <-r.cleanupDone:
		// 已关闭
	default:
		close(r.cleanupDone)
	}
}

// cleanupLoop 后台清理循环
func (r *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.cleanupStaleEntries()
		case <-r.cleanupDone:
			return
		}
	}
}

// cleanupStaleEntries 清理超过10分钟未访问的条目
func (r *RateLimiter) cleanupStaleEntries() {
	r.mu.Lock()
	defer r.mu.Unlock()

	threshold := time.Now().Add(-10 * time.Minute)
	for key, entry := range r.limiters {
		if entry.lastAccess.Before(threshold) {
			delete(r.limiters, key)
		}
	}
}

// getLimiter 获取或创建用户的限流器（双重检查锁）
func (r *RateLimiter) getLimiter(key string) *rate.Limiter {
	r.mu.RLock()
	if entry, exists := r.limiters[key]; exists {
		entry.lastAccess = time.Now()
		r.mu.RUnlock()
		return entry.limiter
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	// 再次检查，避免并发创建
	if entry, exists := r.limiters[key]; exists {
		entry.lastAccess = time.Now()
		return entry.limiter
	}

	limiter := rate.NewLimiter(r.rate, r.burst)
	r.limiters[key] = &limiterEntry{
		limiter:    limiter,
		lastAccess: time.Now(),
	}
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
			key = "user:" + strconv.FormatInt(userID, 10)
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
