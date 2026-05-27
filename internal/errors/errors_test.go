package errors

import (
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestAppError(t *testing.T) {
	// 测试创建错误
	err := NewAppError(10001, "测试错误")
	if err.Code != 10001 {
		t.Errorf("expected code 10001, got %d", err.Code)
	}
	if err.Message != "测试错误" {
		t.Errorf("expected message '测试错误', got %s", err.Message)
	}
}

func TestAppErrorWithDetail(t *testing.T) {
	err := NewAppError(10001, "测试错误").WithDetail("详细信息")
	if err.Detail != "详细信息" {
		t.Errorf("expected detail '详细信息', got %s", err.Detail)
	}
}

func TestAppErrorWithError(t *testing.T) {
	originalErr := errors.New("原始错误")
	err := NewAppError(10001, "测试错误").WithError(originalErr)
	if err.Err != originalErr {
		t.Errorf("expected original error")
	}
}

func TestAppErrorError(t *testing.T) {
	err := NewAppError(10001, "测试错误").WithDetail("详细信息")
	expected := "[10001] 测试错误: 详细信息"
	if err.Error() != expected {
		t.Errorf("expected %s, got %s", expected, err.Error())
	}
}

func TestAppErrorHTTPStatus(t *testing.T) {
	tests := []struct {
		code     int
		expected int
	}{
		{ErrCodeSuccess, 200},
		{ErrCodeInternalError, 500},
		{ErrCodeInvalidParam, 400},
		{ErrCodeUnauthorized, 401},
		{ErrCodeForbidden, 403},
		{ErrCodeNotFound, 404},
		{ErrCodeConflict, 409},
		{ErrCodeServiceUnavailable, 503},
	}

	for _, tt := range tests {
		err := NewAppError(tt.code, "test")
		if err.HTTPStatus() != tt.expected {
			t.Errorf("code %d: expected %d, got %d", tt.code, tt.expected, err.HTTPStatus())
		}
	}
}

func TestIsRecordNotFound(t *testing.T) {
	// 测试 gorm.ErrRecordNotFound
	if !IsRecordNotFound(gorm.ErrRecordNotFound) {
		t.Error("expected gorm.ErrRecordNotFound to be recognized")
	}

	// 测试普通错误
	if IsRecordNotFound(errors.New("not found")) {
		t.Error("expected regular error to not be recognized as record not found")
	}
}

func TestIsDatabaseError(t *testing.T) {
	// 测试数据库错误
	if !IsDatabaseError(errors.New("database connection failed")) {
		t.Error("expected database error to be recognized")
	}
	if !IsDatabaseError(errors.New("sql syntax error")) {
		t.Error("expected sql error to be recognized")
	}

	// 测试非数据库错误
	if IsDatabaseError(errors.New("some other error")) {
		t.Error("expected non-database error to not be recognized")
	}
}

func TestIsKafkaError(t *testing.T) {
	// 测试 Kafka 错误
	if !IsKafkaError(errors.New("kafka connection failed")) {
		t.Error("expected kafka error to be recognized")
	}
	if !IsKafkaError(errors.New("broker not available")) {
		t.Error("expected broker error to be recognized")
	}

	// 测试非 Kafka 错误
	if IsKafkaError(errors.New("some other error")) {
		t.Error("expected non-kafka error to not be recognized")
	}
}

func TestIsAuthError(t *testing.T) {
	// 测试认证错误
	if !IsAuthError(ErrInvalidCredentials) {
		t.Error("expected ErrInvalidCredentials to be auth error")
	}
	if !IsAuthError(ErrTokenExpired) {
		t.Error("expected ErrTokenExpired to be auth error")
	}
	if !IsAuthError(ErrTokenInvalid) {
		t.Error("expected ErrTokenInvalid to be auth error")
	}
	if !IsAuthError(ErrUserDisabled) {
		t.Error("expected ErrUserDisabled to be auth error")
	}

	// 测试非认证错误
	if IsAuthError(ErrNotFound) {
		t.Error("expected ErrNotFound to not be auth error")
	}
}

func TestIsPermissionError(t *testing.T) {
	// 测试权限错误
	if !IsPermissionError(ErrPermissionDenied) {
		t.Error("expected ErrPermissionDenied to be permission error")
	}
	if !IsPermissionError(ErrClusterAccessDenied) {
		t.Error("expected ErrClusterAccessDenied to be permission error")
	}

	// 测试非权限错误
	if IsPermissionError(ErrNotFound) {
		t.Error("expected ErrNotFound to not be permission error")
	}
}

func TestIsRetryable(t *testing.T) {
	// 测试可重试错误
	if !IsRetryable(ErrServiceUnavil) {
		t.Error("expected ErrServiceUnavil to be retryable")
	}
	if !IsRetryable(ErrDatabaseConnection) {
		t.Error("expected ErrDatabaseConnection to be retryable")
	}
	if !IsRetryable(ErrKafkaConnectionFailed) {
		t.Error("expected ErrKafkaConnectionFailed to be retryable")
	}
	if !IsRetryable(errors.New("database connection failed")) {
		t.Error("expected database connection error to be retryable")
	}

	// 测试不可重试错误
	if IsRetryable(ErrNotFound) {
		t.Error("expected ErrNotFound to not be retryable")
	}
	if IsRetryable(ErrPermissionDenied) {
		t.Error("expected ErrPermissionDenied to not be retryable")
	}
}

func TestFilterSensitive(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"normal message", "normal message"},
		{"password=secret123", "password=***"},
		{"password=secret&token=abc", "password=***&token=***"},
		{"username=test&password=pass", "username=test&password=***"},
		{"key=abc", "key=***"},
		{"credential=xyz", "credential=***"},
		// JSON 格式测试（简化）
		{`password=secret`, `password=***`},
		{`token=abc`, `token=***`},
	}

	for _, tt := range tests {
		result := FilterSensitive(tt.input)
		if result != tt.expected {
			t.Errorf("FilterSensitive(%s): expected %s, got %s", tt.input, tt.expected, result)
		}
	}
}

func TestAppErrorIsSensitive(t *testing.T) {
	// 测试敏感错误
	err := NewAppError(ErrCodeInternalError, "password validation failed").WithDetail("password=123456")
	if !err.IsSensitive() {
		t.Error("expected error with password to be sensitive")
	}

	// 测试非敏感错误
	err2 := NewAppError(ErrCodeNotFound, "resource not found")
	if err2.IsSensitive() {
		t.Error("expected error without sensitive info to not be sensitive")
	}
}

func TestWrapError(t *testing.T) {
	originalErr := errors.New("original error")
	wrapped := WrapError(originalErr, ErrCodeInternalError, "包装错误")
	if wrapped == nil {
		t.Fatal("expected wrapped error to not be nil")
	}
	if wrapped.Code != ErrCodeInternalError {
		t.Errorf("expected code %d, got %d", ErrCodeInternalError, wrapped.Code)
	}
	if wrapped.Err != originalErr {
		t.Error("expected original error to be preserved")
	}
}

func TestWrapErrorNil(t *testing.T) {
	wrapped := WrapError(nil, ErrCodeInternalError, "test")
	if wrapped != nil {
		t.Error("expected nil error to return nil")
	}
}

func TestWrapErrorAlreadyAppError(t *testing.T) {
	original := NewAppError(ErrCodeNotFound, "not found")
	wrapped := WrapError(original, ErrCodeInternalError, "test")
	// 应该保持原有的 AppError
	if wrapped.Code != ErrCodeNotFound {
		t.Errorf("expected code %d, got %d", ErrCodeNotFound, wrapped.Code)
	}
}

func TestErrorUnwrap(t *testing.T) {
	originalErr := errors.New("original error")
	err := NewAppError(10001, "test").WithError(originalErr)
	if !errors.Is(err, originalErr) {
		t.Error("expected errors.Is to return true for original error")
	}
}

func TestErrorAs(t *testing.T) {
	originalErr := NewAppError(10001, "test")
	var target *AppError
	if !errors.As(originalErr, &target) {
		t.Error("expected errors.As to return true for AppError")
	}
	if target.Code != 10001 {
		t.Errorf("expected code 10001, got %d", target.Code)
	}
}

func TestErrorMessageContains(t *testing.T) {
	err := NewAppError(ErrCodeInvalidCredentials, "用户名或密码错误")
	msg := err.Error()
	if !strings.Contains(msg, "用户名或密码错误") {
		t.Errorf("expected error message to contain '用户名或密码错误', got %s", msg)
	}
}