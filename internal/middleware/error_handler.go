package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"kafka-management-platform/internal/errors"
	"kafka-management-platform/internal/logger"
)

// ErrorResponse 错误响应结构
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// ErrorHandler 错误处理中间件
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// 检查是否有错误
		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			handleError(c, err.Err)
		}
	}
}

// handleError 处理错误
func handleError(c *gin.Context, err error) {
	// 记录错误日志
	logger.Error("Request error",
		"path", c.Request.URL.Path,
		"method", c.Request.Method,
		"error", err.Error(),
	)

	// 判断错误类型
	var appErr *errors.AppError
	if errors.As(err, &appErr) {
		// 过滤敏感信息
		detail := filterSensitiveInfo(appErr.Detail)
		
		c.JSON(appErr.HTTPStatus(), ErrorResponse{
			Code:    appErr.Code,
			Message: appErr.Message,
			Detail:  detail,
		})
		return
	}

	// 数据库记录未找到
	if errors.IsRecordNotFound(err) {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    errors.ErrCodeNotFound,
			Message: "资源不存在",
		})
		return
	}

	// 其他内部错误
	c.JSON(http.StatusInternalServerError, ErrorResponse{
		Code:    errors.ErrCodeInternalError,
		Message: "内部错误",
	})
}

// filterSensitiveInfo 过滤敏感信息
func filterSensitiveInfo(detail string) string {
	if detail == "" {
		return ""
	}

	// 过滤密码、密钥等敏感信息
	sensitiveKeys := []string{"password", "secret", "token", "key", "credential"}
	filtered := detail

	for _, key := range sensitiveKeys {
		// 替换 password=xxx 为 password=***
		filtered = strings.ReplaceAll(filtered, key+"=", key+"=***")
	}

	return filtered
}

// RecoveryWithErrorHandler 带错误恢复的中间件
func RecoveryWithErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("Panic recovered",
					"path", c.Request.URL.Path,
					"error", err,
				)
				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Code:    errors.ErrCodeInternalError,
					Message: "系统内部错误",
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}