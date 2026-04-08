package middleware

import (
	"net/http"
	"runtime/debug"

	"kafka-management-platform/internal/errors"
	"kafka-management-platform/internal/logger"

	"github.com/gin-gonic/gin"
)

// ErrorHandlerMiddleware 统一错误处理中间件
func ErrorHandlerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// 检查是否有错误
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			handleError(c, err)
		}
	}
}

// RecoveryMiddleware 恢复中间件，捕获 panic
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 记录堆栈信息
				logger.Error("Panic recovered",
					"error", err,
					"stack", string(debug.Stack()),
				)

				// 返回内部服务器错误
				appErr := errors.NewInternalError("Internal server error")
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code":    appErr.Code,
					"message": appErr.Message,
				})
			}
		}()
		c.Next()
	}
}

// handleError 处理错误
func handleError(c *gin.Context, err error) {
	// 避免重复响应
	if c.Writer.Status() != http.StatusOK {
		return
	}

	// 记录错误日志
	errors.LogError(err, "HTTP Error")

	// 获取 HTTP 状态码
	httpStatus := errors.GetHTTPStatus(err)

	// 如果是应用错误，返回结构化响应
	if appErr, ok := err.(*errors.AppError); ok {
		// 过滤敏感信息
		response := filterSensitiveData(appErr)
		c.AbortWithStatusJSON(httpStatus, response)
		return
	}

	// 其他错误返回通用响应
	c.AbortWithStatusJSON(httpStatus, gin.H{
		"code":    errors.ErrCodeInternalServerError,
		"message": "Internal server error",
	})
}

// filterSensitiveData 过滤敏感信息
func filterSensitiveData(err *errors.AppError) gin.H {
	response := gin.H{
		"code":    err.Code,
		"message": err.Message,
	}

	// 不在生产环境显示详细信息
	if err.Details != "" {
		response["details"] = err.Details
	}

	return response
}

// NotFoundHandler 404 处理
func NotFoundHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    errors.ErrCodeNotFound,
			"message": "Resource not found",
			"path":    c.Request.URL.Path,
		})
	}
}

// MethodNotAllowedHandler 405 处理
func MethodNotAllowedHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"code":    errors.ErrCodeInvalidParams,
			"message": "Method not allowed",
		})
	}
}