package errors

import (
	"fmt"
	"net/http"

	"kafka-management-platform/internal/logger"
)

// ErrorCode 错误码类型
type ErrorCode string

// 业务错误码定义
const (
	// 通用错误码
	ErrCodeInternalServerError ErrorCode = "ERR_INTERNAL_SERVER_ERROR"
	ErrCodeInvalidParams       ErrorCode = "ERR_INVALID_PARAMS"
	ErrCodeUnauthorized        ErrorCode = "ERR_UNAUTHORIZED"
	ErrCodeForbidden           ErrorCode = "ERR_FORBIDDEN"
	ErrCodeNotFound            ErrorCode = "ERR_NOT_FOUND"
	ErrCodeConflict            ErrorCode = "ERR_CONFLICT"
	ErrCodeTooManyRequests     ErrorCode = "ERR_TOO_MANY_REQUESTS"

	// 认证相关错误码
	ErrCodeInvalidCredentials ErrorCode = "ERR_INVALID_CREDENTIALS"
	ErrCodeTokenExpired       ErrorCode = "ERR_TOKEN_EXPIRED"
	ErrCodeTokenInvalid       ErrorCode = "ERR_TOKEN_INVALID"
	ErrCodeUserInactive       ErrorCode = "ERR_USER_INACTIVE"

	// 集群相关错误码
	ErrCodeClusterNotFound        ErrorCode = "ERR_CLUSTER_NOT_FOUND"
	ErrCodeClusterNameExists      ErrorCode = "ERR_CLUSTER_NAME_EXISTS"
	ErrCodeClusterConnectionFailed ErrorCode = "ERR_CLUSTER_CONNECTION_FAILED"
	ErrCodeClusterAccessDenied    ErrorCode = "ERR_CLUSTER_ACCESS_DENIED"

	// Topic 相关错误码
	ErrCodeTopicNotFound           ErrorCode = "ERR_TOPIC_NOT_FOUND"
	ErrCodeTopicAlreadyExists      ErrorCode = "ERR_TOPIC_ALREADY_EXISTS"
	ErrCodeInvalidTopicName        ErrorCode = "ERR_INVALID_TOPIC_NAME"
	ErrCodeInvalidPartitions       ErrorCode = "ERR_INVALID_PARTITIONS"
	ErrCodeInvalidReplicationFactor ErrorCode = "ERR_INVALID_REPLICATION_FACTOR"

	// ACL 相关错误码
	ErrCodeACLNotFound      ErrorCode = "ERR_ACL_NOT_FOUND"
	ErrCodeACLAlreadyExists ErrorCode = "ERR_ACL_ALREADY_EXISTS"
	ErrCodeInvalidACLParams ErrorCode = "ERR_INVALID_ACL_PARAMS"

	// 用户相关错误码
	ErrCodeUserNotFound         ErrorCode = "ERR_USER_NOT_FOUND"
	ErrCodeUsernameExists       ErrorCode = "ERR_USERNAME_EXISTS"
	ErrCodeInvalidPassword      ErrorCode = "ERR_INVALID_PASSWORD"
	ErrCodeCannotDisableSelf    ErrorCode = "ERR_CANNOT_DISABLE_SELF"

	// 监控相关错误码
	ErrCodeNoPrometheusURL    ErrorCode = "ERR_NO_PROMETHEUS_URL"
	ErrCodeTimeRangeExceeded  ErrorCode = "ERR_TIME_RANGE_EXCEEDED"
	ErrCodeInvalidTimeRange   ErrorCode = "ERR_INVALID_TIME_RANGE"
	ErrCodePrometheusQueryFailed ErrorCode = "ERR_PROMETHEUS_QUERY_FAILED"

	// Kafka 相关错误码
	ErrCodeKafkaConnectionFailed ErrorCode = "ERR_KAFKA_CONNECTION_FAILED"
	ErrCodeKafkaOperationFailed  ErrorCode = "ERR_KAFKA_OPERATION_FAILED"

	// 加密相关错误码
	ErrCodeEncryptionFailed    ErrorCode = "ERR_ENCRYPTION_FAILED"
	ErrCodeDecryptionFailed    ErrorCode = "ERR_DECRYPTION_FAILED"
	ErrCodeInvalidEncryptionKey ErrorCode = "ERR_INVALID_ENCRYPTION_KEY"

	// 数据库相关错误码
	ErrCodeDatabaseError    ErrorCode = "ERR_DATABASE_ERROR"
	ErrCodeDatabaseNotFound ErrorCode = "ERR_DATABASE_NOT_FOUND"
)

// 预定义的错误变量
var (
	// ErrDatabaseConnection 数据库连接错误
	ErrDatabaseConnection = NewAppError(ErrCodeDatabaseError, "database connection failed", http.StatusInternalServerError)
)

// AppError 应用错误
type AppError struct {
	Code       ErrorCode   `json:"code"`
	Message    string      `json:"message"`
	Details    string      `json:"details,omitempty"`
	HTTPStatus int         `json:"-"`
	Err        error       `json:"-"`
}

// Error 实现 error 接口
func (e *AppError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 获取底层错误
func (e *AppError) Unwrap() error {
	return e.Err
}

// WithDetails 添加错误详情
func (e *AppError) WithDetails(details string) *AppError {
	e.Details = details
	return e
}

// WithErr 添加底层错误
func (e *AppError) WithErr(err error) *AppError {
	e.Err = err
	return e
}

// NewAppError 创建新的应用错误
func NewAppError(code ErrorCode, message string, httpStatus int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
	}
}

// NewInternalError 创建内部服务器错误
func NewInternalError(message string) *AppError {
	return NewAppError(ErrCodeInternalServerError, message, http.StatusInternalServerError)
}

// NewInvalidParamsError 创建参数错误
func NewInvalidParamsError(message string) *AppError {
	return NewAppError(ErrCodeInvalidParams, message, http.StatusBadRequest)
}

// NewUnauthorizedError 创建未授权错误
func NewUnauthorizedError(message string) *AppError {
	return NewAppError(ErrCodeUnauthorized, message, http.StatusUnauthorized)
}

// NewForbiddenError 创建禁止访问错误
func NewForbiddenError(message string) *AppError {
	return NewAppError(ErrCodeForbidden, message, http.StatusForbidden)
}

// NewNotFoundError 创建资源不存在错误
func NewNotFoundError(message string) *AppError {
	return NewAppError(ErrCodeNotFound, message, http.StatusNotFound)
}

// NewConflictError 创建冲突错误
func NewConflictError(message string) *AppError {
	return NewAppError(ErrCodeConflict, message, http.StatusConflict)
}

// NewTooManyRequestsError 创建请求过多错误
func NewTooManyRequestsError(message string) *AppError {
	return NewAppError(ErrCodeTooManyRequests, message, http.StatusTooManyRequests)
}

// WrapError 包装错误
func WrapError(err error, code ErrorCode, message string) *AppError {
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	return &AppError{
		Code:       code,
		Message:    message,
		Err:        err,
		HTTPStatus: http.StatusInternalServerError,
	}
}

// IsAppError 检查是否为应用错误
func IsAppError(err error) bool {
	_, ok := err.(*AppError)
	return ok
}

// GetHTTPStatus 获取 HTTP 状态码
func GetHTTPStatus(err error) int {
	if appErr, ok := err.(*AppError); ok {
		return appErr.HTTPStatus
	}
	return http.StatusInternalServerError
}

// LogError 记录错误日志
func LogError(err error, context string) {
	if appErr, ok := err.(*AppError); ok {
		logger.Error(context,
			"code", appErr.Code,
			"message", appErr.Message,
			"details", appErr.Details,
			"error", appErr.Err,
		)
	} else {
		logger.Error(context,
			"error", err,
		)
	}
}

// IsRetryable 检查错误是否可重试
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	
	// 检查是否为 AppError
	if appErr, ok := err.(*AppError); ok {
		switch appErr.Code {
		case ErrCodeKafkaConnectionFailed,
			ErrCodeClusterConnectionFailed,
			ErrCodeDatabaseError,
			ErrCodePrometheusQueryFailed:
			return true
		default:
			return false
		}
	}
	
	// 对于其他错误类型，默认不可重试
	return false
}