package errors

import (
	"fmt"
	"net/http"

	"gorm.io/gorm"
)

// 错误码定义
const (
	// 通用错误
	ErrCodeSuccess           = 0
	ErrCodeInternalError     = 10001
	ErrCodeInvalidParam      = 10002
	ErrCodeUnauthorized      = 10003
	ErrCodeForbidden         = 10004
	ErrCodeNotFound          = 10005
	ErrCodeConflict          = 10006

	// 认证错误
	ErrCodeInvalidCredentials = 20001
	ErrCodeTokenExpired       = 20002
	ErrCodeTokenInvalid       = 20003
	ErrCodeUserDisabled       = 20004

	// 集群错误
	ErrCodeClusterNotFound     = 30001
	ErrCodeClusterExists       = 30002
	ErrCodeClusterConnection   = 30003
	ErrCodeClusterAuthFailed   = 30004

	// Topic错误
	ErrCodeTopicNotFound       = 40001
	ErrCodeTopicExists         = 40002
	ErrCodeTopicInvalidName    = 40003
	ErrCodeTopicInvalidConfig  = 40004

	// ACL错误
	ErrCodeACLNotFound         = 50001
	ErrCodeACLExists           = 50002

	// 监控错误
	ErrCodePrometheusError     = 60001
	ErrCodeMetricsNotFound     = 60002

	// 权限错误
	ErrCodePermissionDenied    = 70001
	ErrCodeClusterAccessDenied = 70002
)

// AppError 应用错误结构
type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// Error 实现 error 接口
func (e *AppError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("[%d] %s: %s", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// WithDetail 添加详细错误信息
func (e *AppError) WithDetail(detail string) *AppError {
	return &AppError{
		Code:    e.Code,
		Message: e.Message,
		Detail:  detail,
	}
}

// NewAppError 创建新的应用错误
func NewAppError(code int, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// HTTPStatus 返回 HTTP 状态码
func (e *AppError) HTTPStatus() int {
	switch e.Code {
	case ErrCodeSuccess:
		return http.StatusOK
	case ErrCodeInternalError:
		return http.StatusInternalServerError
	case ErrCodeInvalidParam:
		return http.StatusBadRequest
	case ErrCodeUnauthorized:
		return http.StatusUnauthorized
	case ErrCodeForbidden:
		return http.StatusForbidden
	case ErrCodeNotFound:
		return http.StatusNotFound
	case ErrCodeConflict:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// 预定义错误
var (
	ErrInternal     = NewAppError(ErrCodeInternalError, "内部错误")
	ErrInvalidParam = NewAppError(ErrCodeInvalidParam, "参数错误")
	ErrUnauthorized = NewAppError(ErrCodeUnauthorized, "未授权")
	ErrForbidden    = NewAppError(ErrCodeForbidden, "禁止访问")
	ErrNotFound     = NewAppError(ErrCodeNotFound, "资源不存在")
	ErrConflict     = NewAppError(ErrCodeConflict, "资源冲突")

	// 认证相关
	ErrInvalidCredentials = NewAppError(ErrCodeInvalidCredentials, "用户名或密码错误")
	ErrTokenExpired       = NewAppError(ErrCodeTokenExpired, "Token已过期")
	ErrTokenInvalid       = NewAppError(ErrCodeTokenInvalid, "无效的Token")
	ErrUserDisabled       = NewAppError(ErrCodeUserDisabled, "用户已被禁用")

	// 集群相关
	ErrClusterNotFound   = NewAppError(ErrCodeClusterNotFound, "集群不存在")
	ErrClusterExists     = NewAppError(ErrCodeClusterExists, "集群已存在")
	ErrClusterConnection = NewAppError(ErrCodeClusterConnection, "集群连接失败")
	ErrClusterAuthFailed = NewAppError(ErrCodeClusterAuthFailed, "集群认证失败")

	// Topic相关
	ErrTopicNotFound      = NewAppError(ErrCodeTopicNotFound, "Topic不存在")
	ErrTopicExists        = NewAppError(ErrCodeTopicExists, "Topic已存在")
	ErrTopicInvalidName   = NewAppError(ErrCodeTopicInvalidName, "Topic名称无效")
	ErrTopicInvalidConfig = NewAppError(ErrCodeTopicInvalidConfig, "Topic配置无效")

	// ACL相关
	ErrACLNotFound = NewAppError(ErrCodeACLNotFound, "ACL不存在")
	ErrACLExists   = NewAppError(ErrCodeACLExists, "ACL已存在")

	// 监控相关
	ErrPrometheusError = NewAppError(ErrCodePrometheusError, "Prometheus查询错误")
	ErrMetricsNotFound = NewAppError(ErrCodeMetricsNotFound, "指标不存在")

	// 权限相关
	ErrPermissionDenied    = NewAppError(ErrCodePermissionDenied, "权限不足")
	ErrClusterAccessDenied = NewAppError(ErrCodeClusterAccessDenied, "无集群访问权限")
)

// IsRecordNotFound 判断是否为记录未找到错误
func IsRecordNotFound(err error) bool {
	return gorm.ErrRecordNotFound.Error() == err.Error()
}