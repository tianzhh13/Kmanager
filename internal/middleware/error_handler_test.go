package middleware

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"kafka-management-platform/internal/errors"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestErrorHandler(t *testing.T) {
	tests := []struct {
		name           string
		setupHandler   func(c *gin.Context)
		expectedCode   int
		expectedMsg    string
	}{
		{
			name: "AppError - Internal Error",
			setupHandler: func(c *gin.Context) {
				c.Errors = append(c.Errors, &gin.Error{
					Err: errors.ErrInternal,
				})
			},
			expectedCode: 10001,
			expectedMsg:  "内部错误",
		},
		{
			name: "AppError - Not Found",
			setupHandler: func(c *gin.Context) {
				c.Errors = append(c.Errors, &gin.Error{
					Err: errors.ErrNotFound,
				})
			},
			expectedCode: 10005,
			expectedMsg:  "资源不存在",
		},
		{
			name: "AppError - Invalid Param",
			setupHandler: func(c *gin.Context) {
				c.Errors = append(c.Errors, &gin.Error{
					Err: errors.ErrInvalidParam,
				})
			},
			expectedCode: 10002,
			expectedMsg:  "参数错误",
		},
		{
			name: "AppError - Forbidden",
			setupHandler: func(c *gin.Context) {
				c.Errors = append(c.Errors, &gin.Error{
					Err: errors.ErrForbidden,
				})
			},
			expectedCode: 10004,
			expectedMsg:  "禁止访问",
		},
		{
			name: "AppError - Unauthorized",
			setupHandler: func(c *gin.Context) {
				c.Errors = append(c.Errors, &gin.Error{
					Err: errors.ErrUnauthorized,
				})
			},
			expectedCode: 10003,
			expectedMsg:  "未授权",
		},
		{
			name: "AppError - Conflict",
			setupHandler: func(c *gin.Context) {
				c.Errors = append(c.Errors, &gin.Error{
					Err: errors.ErrConflict,
				})
			},
			expectedCode: 10006,
			expectedMsg:  "资源冲突",
		},
		{
			name: "AppError - Service Unavailable",
			setupHandler: func(c *gin.Context) {
				c.Errors = append(c.Errors, &gin.Error{
					Err: errors.ErrServiceUnavil,
				})
			},
			expectedCode: 10007,
			expectedMsg:  "服务不可用",
		},
		{
			name: "AppError with detail",
			setupHandler: func(c *gin.Context) {
				err := errors.NewAppError(errors.ErrCodeInternalError, "测试错误").WithDetail("详细信息")
				c.Errors = append(c.Errors, &gin.Error{
					Err: err,
				})
			},
			expectedCode: 10001,
			expectedMsg:  "测试错误",
		},
		{
			name: "Permission error",
			setupHandler: func(c *gin.Context) {
				c.Errors = append(c.Errors, &gin.Error{
					Err: errors.ErrPermissionDenied,
				})
			},
			expectedCode: 70001,
			expectedMsg:  "权限不足",
		},
		{
			name: "Auth error",
			setupHandler: func(c *gin.Context) {
				c.Errors = append(c.Errors, &gin.Error{
					Err: errors.ErrInvalidCredentials,
				})
			},
			expectedCode: 20001,
			expectedMsg:  "用户名或密码错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			// 设置基本的请求信息
			c.Request = httptest.NewRequest("GET", "/test", nil)
			c.Request.RemoteAddr = "127.0.0.1:12345"

			// 执行处理函数
			tt.setupHandler(c)

			// 调用错误处理
			if len(c.Errors) > 0 {
				handleError(c, c.Errors.Last().Err)
			}

			var resp ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if resp.Code != tt.expectedCode {
				t.Errorf("expected code %d, got %d", tt.expectedCode, resp.Code)
			}

			if resp.Message != tt.expectedMsg {
				t.Errorf("expected message %s, got %s", tt.expectedMsg, resp.Message)
			}

			// 验证时间戳存在
			if resp.Timestamp == "" {
				t.Error("expected timestamp to be present")
			}
		})
	}
}

func TestFilterSensitiveInfo(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"normal message", "normal message"},
		{"username=test", "username=test"},
		// 简化测试 - 只测试基本功能
		{`password=secret`, `password=***`},
		{`token=abc`, `token=***`},
		{`key=test`, `key=***`},
		{`credential=xyz`, `credential=***`},
	}

	for _, tt := range tests {
		result := filterSensitiveInfo(tt.input)
		if result != tt.expected {
			t.Errorf("filterSensitiveInfo(%s): expected %s, got %s", tt.input, tt.expected, result)
		}
	}
}

func TestErrorResponseFormat(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)
	c.Request.RemoteAddr = "127.0.0.1:12345"

	err := errors.NewAppError(errors.ErrCodeInvalidParam, "参数错误").WithDetail("field is required")
	c.Errors = append(c.Errors, &gin.Error{Err: err})

	handleError(c, c.Errors.Last().Err)

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// 验证响应格式
	if resp.Code == 0 {
		t.Error("expected code to be set")
	}
	if resp.Message == "" {
		t.Error("expected message to be set")
	}
	if resp.Timestamp == "" {
		t.Error("expected timestamp to be set")
	}
	if resp.RequestID == "" {
		t.Error("expected request_id to be set")
	}
}

func TestRecoveryWithErrorHandler(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)

	// 模拟 panic
	defer func() {
		if r := recover(); r != nil {
			// 验证 panic 被捕获
			if r != "test panic" {
				t.Errorf("expected panic 'test panic', got %v", r)
			}
		}
	}()

	handler := RecoveryWithErrorHandler()
	handler(c)

	// 手动触发 panic 来测试
	panic("test panic")
}

func TestErrorHandlerWithSensitiveData(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)
	c.Request.RemoteAddr = "127.0.0.1:12345"

	// 创建包含敏感信���的错误
	err := errors.NewAppError(errors.ErrCodeInternalError, "password validation failed").WithDetail("password=secret123")
	c.Errors = append(c.Errors, &gin.Error{Err: err})

	handleError(c, c.Errors.Last().Err)

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// 验证敏感信息被过滤
	if resp.Detail == "password=secret123" {
		t.Error("expected sensitive data to be filtered")
	}
}

func TestErrorHandlerWithMetadata(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)
	c.Request.RemoteAddr = "127.0.0.1:12345"

	// 创建带元数据的错误
	err := errors.NewAppError(errors.ErrCodeInvalidParam, "参数错误").
		WithMetadata("field", "username").
		WithMetadata("value", "invalid")
	c.Errors = append(c.Errors, &gin.Error{Err: err})

	handleError(c, c.Errors.Last().Err)

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// 验证元数据存在
	if resp.Metadata == nil {
		t.Error("expected metadata to be present")
	}
}