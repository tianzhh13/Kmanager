package middleware

import (
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

var (
	cachedOrigins   []string
	cachedOriginsOnce sync.Once
)

// getCachedOrigins 获取缓存的允许跨域来源列表（初始化时读取一次）
func getCachedOrigins() []string {
	cachedOriginsOnce.Do(func() {
		cachedOrigins = viper.GetStringSlice("cors.allowed_origins")
	})
	return cachedOrigins
}

// isOriginAllowed 检查origin是否在允许列表中
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	if len(allowedOrigins) == 0 {
		return false
	}
	for _, allowed := range allowedOrigins {
		if allowed == origin {
			return true
		}
	}
	return false
}

// CORSMiddleware CORS 中间件
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// 从缓存获取允许的跨域来源列表（只在首次调用时读取 viper）
		allowedOrigins := getCachedOrigins()

		// 根据gin模式决定CORS策略
		isReleaseMode := gin.Mode() == gin.ReleaseMode

		var allowOrigin string
		if isReleaseMode {
			// 生产模式：使用白名单
			if origin != "" && isOriginAllowed(origin, allowedOrigins) {
				allowOrigin = origin
			} else {
				// 生产模式下不允许跨域
				c.Header("Access-Control-Allow-Origin", "")
			}
		} else {
			// 开发模式：允许所有跨域
			if len(allowedOrigins) > 0 && origin != "" && isOriginAllowed(origin, allowedOrigins) {
				// 如果配置了白名单且origin匹配，使用具体origin
				allowOrigin = origin
			} else {
				// 否则使用通配符（开发环境）
				allowOrigin = "*"
			}
		}

		c.Header("Access-Control-Allow-Origin", allowOrigin)

		// 只有在设置了具体origin时才允许credentials
		if allowOrigin != "" && allowOrigin != "*" {
			c.Header("Access-Control-Allow-Credentials", "true")
		} else {
			c.Header("Access-Control-Allow-Credentials", "false")
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Requested-With")
		c.Header("Access-Control-Expose-Headers", "Content-Length")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
