package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"time"

	"kafka-management-platform/internal/service/audit"

	"github.com/gin-gonic/gin"
)

// AuditMiddleware 审计日志中间件
func AuditMiddleware(auditSvc *audit.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 跳过监控查询接口（会产生大量日志）
		path := c.Request.URL.Path
		if contains(path, "/metrics") && c.Request.Method == "GET" {
			c.Next()
			return
		}

		// 记录请求开始时间
		startTime := time.Now()

		// 记录请求体（用于后续可能的审计）
		var requestBody []byte
		if c.Request.Body != nil {
			requestBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// 处理请求
		c.Next()

		// 记录请求完成时间
		duration := time.Since(startTime)

		// 获取用户信息
		userID := GetUserID(c)
		username := GetUsername(c)

		// 确定操作类型
		action := c.Request.Method + " " + c.FullPath()
		if c.FullPath() == "" {
			action = c.Request.Method + " " + c.Request.URL.Path
		}

		// 确定资源类型
		resourceType := getResourceType(c.FullPath())

		// 记录审计日志
		go func() {
			details := map[string]interface{}{
				"method":     c.Request.Method,
				"path":       c.Request.URL.Path,
				"query":      c.Request.URL.Query(),
				"duration":   duration.Milliseconds(),
				"status":     c.Writer.Status(),
				"client_ip":  c.ClientIP(),
				"user_agent": c.Request.UserAgent(),
			}

			// 添加请求体（如果存在），并对敏感路由进行脱敏处理
			if len(requestBody) > 0 {
				sanitizedBody := sanitizeRequestBody(c.Request.URL.Path, requestBody)
				details["request_body"] = string(sanitizedBody)
			}

			status := "success"
			if c.Writer.Status() >= 400 {
				status = "failed"
			}

			if err := auditSvc.Log(c.Request.Context(), &audit.LogRequest{
				UserID:       userID,
				Username:     username,
				Action:       action,
				ResourceType: resourceType,
				ResourceID:   c.Param("id"),
				Details:      details,
				IPAddress:    c.ClientIP(),
				UserAgent:    c.Request.UserAgent(),
				Status:       status,
			}); err != nil {
				log.Printf("[AuditMiddleware] Failed to log: %v, action=%s, resource=%s", err, action, resourceType)
			}
		}()
	}
}

// getResourceType 根据路径确定资源类型
func getResourceType(path string) string {
	switch {
	case contains(path, "/auth/"):
		return "auth"
	case contains(path, "/users"):
		return "user"
	case contains(path, "/clusters"):
		return "cluster"
	case contains(path, "/topics"):
		return "topic"
	case contains(path, "/acls"):
		return "acl"
	case contains(path, "/scram-users"):
		return "scram_user"
	case contains(path, "/metrics"):
		return "monitor"
	case contains(path, "/audit-logs"):
		return "audit_log"
	case contains(path, "/topic-permissions"):
		return "topic_permission"
	case contains(path, "/dashboard"):
		return "dashboard"
	default:
		return "system"
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// isSensitiveRoute checks if the route may contain passwords
func isSensitiveRoute(method, path string) bool {
	switch {
	// POST /api/v1/auth/login
	case method == "POST" && contains(path, "/api/v1/auth/login"):
		return true
	// POST /api/v1/auth/change-password
	case method == "POST" && contains(path, "/api/v1/auth/change-password"):
		return true
	// POST /api/v1/clusters (may contain SCRAM password)
	case method == "POST" && contains(path, "/api/v1/clusters"):
		return true
	// PUT /api/v1/clusters/:id
	case method == "PUT" && contains(path, "/api/v1/clusters/"):
		return true
	// POST /api/v1/scram-users
	case method == "POST" && contains(path, "/api/v1/scram-users"):
		return true
	// PUT /api/v1/scram-users/:id
	case method == "PUT" && contains(path, "/api/v1/scram-users/"):
		return true
	default:
		return false
	}
}

// sanitizeRequestBody replaces sensitive fields with "******" for logging
func sanitizeRequestBody(path string, body []byte) []byte {
	if len(body) == 0 {
		return body
	}

	// Check if path matches sensitive patterns
	isSensitive := contains(path, "/api/v1/auth/login") ||
		contains(path, "/api/v1/auth/change-password") ||
		contains(path, "/api/v1/clusters") ||
		contains(path, "/api/v1/scram-users")

	if !isSensitive {
		return body
	}

	// Try to parse as JSON
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		// Not valid JSON, return as-is
		return body
	}

	// Fields to sanitize
	sensitiveFields := []string{"password", "scram_password", "new_password", "old_password"}

	// Replace sensitive fields
	for _, field := range sensitiveFields {
		if _, ok := data[field]; ok {
			data[field] = "******"
		}
	}

	// Re-serialize
	sanitized, err := json.Marshal(data)
	if err != nil {
		// If marshaling fails, return original
		return body
	}

	return sanitized
}
