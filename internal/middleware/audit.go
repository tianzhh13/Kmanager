package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"kafka-management-platform/internal/service/audit"

	"github.com/gin-gonic/gin"
)

const auditChannelSize = 1000

// auditTask 审计日志写入任务
type auditTask struct {
	ctx     context.Context
	request *audit.LogRequest
}

// AuditMiddleware 审计日志中间件
func AuditMiddleware(auditSvc *audit.Service) gin.HandlerFunc {
	// 创建带缓冲的 channel
	ch := make(chan *auditTask, auditChannelSize)

	// 启动后台 worker
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go auditWorker(ch, auditSvc, &wg)
	}

	return func(c *gin.Context) {
		// 跳过监控查询接口（会产生大量日志）
		path := c.Request.URL.Path
		if strings.Contains(path, "/metrics") && c.Request.Method == "GET" {
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

		// 构建审计日志详情
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

		// 非阻塞发送到 channel，满则丢弃（防止阻塞请求）
		task := &auditTask{
			ctx: context.Background(), // 使用独立 context，不随请求取消
			request: &audit.LogRequest{
				UserID:       userID,
				Username:     username,
				Action:       action,
				ResourceType: resourceType,
				ResourceID:   c.Param("id"),
				Details:      details,
				IPAddress:    c.ClientIP(),
				UserAgent:    c.Request.UserAgent(),
				Status:       status,
			},
		}
		select {
		case ch <- task:
		default:
			log.Printf("[AuditMiddleware] Channel full, audit log dropped: action=%s", action)
		}
	}
}

// auditWorker 后台审计日志写入 worker
func auditWorker(ch <-chan *auditTask, auditSvc *audit.Service, wg *sync.WaitGroup) {
	defer wg.Done()
	for task := range ch {
		if err := auditSvc.Log(task.ctx, task.request); err != nil {
			log.Printf("[AuditMiddleware] Failed to log: %v, action=%s, resource=%s", err, task.request.Action, task.request.ResourceType)
		}
	}
}

// getResourceType 根据路径确定资源类型
func getResourceType(path string) string {
	switch {
	case strings.Contains(path, "/auth/"):
		return "auth"
	case strings.Contains(path, "/users"):
		return "user"
	case strings.Contains(path, "/clusters"):
		return "cluster"
	case strings.Contains(path, "/topics"):
		return "topic"
	case strings.Contains(path, "/acls"):
		return "acl"
	case strings.Contains(path, "/scram-users"):
		return "scram_user"
	case strings.Contains(path, "/metrics"):
		return "monitor"
	case strings.Contains(path, "/audit-logs"):
		return "audit_log"
	case strings.Contains(path, "/topic-permissions"):
		return "topic_permission"
	default:
		return "system"
	}
}

// sanitizeRequestBody replaces sensitive fields with "******" for logging
func sanitizeRequestBody(path string, body []byte) []byte {
	if len(body) == 0 {
		return body
	}

	// Check if path matches sensitive patterns
	isSensitive := strings.Contains(path, "/api/v1/auth/login") ||
		strings.Contains(path, "/api/v1/auth/change-password") ||
		strings.Contains(path, "/api/v1/clusters") ||
		strings.Contains(path, "/api/v1/scram-users")

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
