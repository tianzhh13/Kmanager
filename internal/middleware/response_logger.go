package middleware

import (
	"bytes"
	"time"

	"kafka-management-platform/internal/logger"

	"github.com/gin-gonic/gin"
)

// responseWriter 包装 gin.ResponseWriter 以捕获响应体
type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// ResponseLoggerMiddleware 记录所有非 2xx 响应的详细信息
// 用于排查生产环境"操作失败但不知道原因"的问题
func ResponseLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 跳过静态资源和健康检查
		path := c.Request.URL.Path
		if len(path) >= 4 && path[:4] != "/api" {
			c.Next()
			return
		}
		if path == "/health" || path == "/api/v1/system/config" {
			c.Next()
			return
		}

		// 包装 writer 捕获响应
		w := &responseWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBuffer(nil),
		}
		c.Writer = w

		start := time.Now()
		c.Next()
		latency := time.Since(start)

		status := c.Writer.Status()
		if status >= 400 {
			logger.Warn("API error response",
				"method", c.Request.Method,
				"path", path,
				"status", status,
				"latency", latency.String(),
				"client_ip", c.ClientIP(),
				"user_id", GetUserID(c),
				"body", w.body.String(),
			)
		}
	}
}
