package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"strconv"
	"strings"
	"time"

	"kafka-management-platform/internal/service/audit"

	"github.com/gin-gonic/gin"
)

// AuditMiddleware 审计日志中间件
type AuditMiddleware struct {
	auditSvc *audit.Service
}

// NewAuditMiddleware 创建审计日志中间件
func NewAuditMiddleware(auditSvc *audit.Service) *AuditMiddleware {
	return &AuditMiddleware{
		auditSvc: auditSvc,
	}
}

// AuditLogConfig 审计日志配置
type AuditLogConfig struct {
	// 需要记录的操作类型
	Actions []string
	// 需要忽略的路径
	ExcludePaths []string
	// 是否记录请求体
	RecordRequestBody bool
	// 是否记录响应体
	RecordResponseBody bool
}

// DefaultAuditLogConfig 默认审计日志配置
var DefaultAuditLogConfig = AuditLogConfig{
	Actions: []string{
		"POST", "PUT", "PATCH", "DELETE",
	},
	ExcludePaths: []string{
		"/health",
		"/api/v1/auth/login",
		"/api/v1/auth/refresh",
	},
	RecordRequestBody:  false,
	RecordResponseBody: false,
}

// Audit 审计日志中间件处理函数
func (m *AuditMiddleware) Audit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否需要记录
		if !m.shouldAudit(c) {
			c.Next()
			return
		}

		// 记录请求开始时间
		startTime := time.Now()

		// 记录请求体（可选）
		var requestBody string
		if DefaultAuditLogConfig.RecordRequestBody && c.Request.Body != nil {
			bodyBytes, _ := io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			requestBody = string(bodyBytes)
		}

		// 处理请求
		c.Next()

		// 记录响应时间
		duration := time.Since(startTime)

		// 获取操作结果状态
		status := "success"
		if c.Writer.Status() >= 400 {
			status = "failed"
		}

		// 提取操作类型和资源信息
		action := m.extractAction(c)
		resourceType := m.extractResourceType(c)
		resourceID := m.extractResourceID(c)
		clusterID := m.extractClusterID(c)

		// 构建详情
		details := m.buildDetails(c, requestBody, duration)

		// 获取用户信息
		userID := GetUserID(c)
		username := GetUsername(c)

		// 记录审计日志
		err := m.auditSvc.LogWithDetails(
			userID,
			username,
			action,
			resourceType,
			resourceID,
			details,
			m.getClientIP(c),
			c.Request.UserAgent(),
			status,
			clusterID,
		)

		if err != nil {
			// 审计日志记录失败不应影响正常请求
			fmt.Printf("failed to write audit log: %v\n", err)
		}
	}
}

// shouldAudit 检查是否需要记录审计日志
func (m *AuditMiddleware) shouldAudit(c *gin.Context) bool {
	// 检查是否在排除路径中
	path := c.Request.URL.Path
	for _, excludePath := range DefaultAuditLogConfig.ExcludePaths {
		if strings.HasPrefix(path, excludePath) {
			return false
		}
	}

	// 检查操作类型
	for _, action := range DefaultAuditLogConfig.Actions {
		if c.Request.Method == action {
			return true
		}
	}

	return false
}

// extractAction 提取操作类型
func (m *AuditMiddleware) extractAction(c *gin.Context) string {
	method := c.Request.Method
	path := c.Request.URL.Path

	// 根据路径和方法确定操作
	if strings.HasSuffix(path, "/clusters") {
		if method == "POST" {
			return "create_cluster"
		}
		if method == "GET" {
			return "list_clusters"
		}
	}

	if strings.Contains(path, "/clusters/") {
		if method == "GET" {
			return "get_cluster"
		}
		if method == "PUT" || method == "PATCH" {
			return "update_cluster"
		}
		if method == "DELETE" {
			return "delete_cluster"
		}
		if strings.HasSuffix(path, "/test") {
			return "test_connection"
		}
		if strings.HasSuffix(path, "/grant") {
			return "grant_cluster_access"
		}
		if strings.HasSuffix(path, "/revoke") {
			return "revoke_cluster_access"
		}
	}

	if strings.HasSuffix(path, "/topics") {
		if method == "POST" {
			return "create_topic"
		}
		if method == "GET" {
			return "list_topics"
		}
	}

	if strings.Contains(path, "/topics/") && !strings.HasSuffix(path, "/sync") {
		if method == "GET" {
			return "get_topic"
		}
		if method == "DELETE" {
			return "delete_topic"
		}
		if strings.HasSuffix(path, "/config") {
			return "update_topic_config"
		}
	}

	if strings.HasSuffix(path, "/acls") {
		if method == "POST" {
			return "create_acl"
		}
		if method == "GET" {
			return "list_acls"
		}
	}

	if strings.Contains(path, "/acls/") {
		if method == "DELETE" && !strings.HasSuffix(path, "/batch-delete") {
			return "delete_acl"
		}
		if strings.HasSuffix(path, "/batch-delete") {
			return "batch_delete_acl"
		}
	}

	if strings.HasSuffix(path, "/users") {
		if method == "POST" {
			return "create_user"
		}
		if method == "GET" {
			return "list_users"
		}
	}

	if strings.Contains(path, "/users/") {
		if method == "GET" {
			return "get_user"
		}
		if method == "PUT" || method == "PATCH" {
			return "update_user"
		}
		if method == "DELETE" {
			return "delete_user"
		}
	}

	if strings.HasSuffix(path, "/audit-logs") {
		if method == "GET" {
			return "list_audit_logs"
		}
	}

	// 默认使用 HTTP 方法
	return strings.ToLower(method) + "_operation"
}

// extractResourceType 提取资源类型
func (m *AuditMiddleware) extractResourceType(c *gin.Context) string {
	path := c.Request.URL.Path

	if strings.Contains(path, "/clusters") {
		return "cluster"
	}
	if strings.Contains(path, "/topics") {
		return "topic"
	}
	if strings.Contains(path, "/acls") {
		return "acl"
	}
	if strings.Contains(path, "/users") {
		return "user"
	}
	if strings.Contains(path, "/metrics") {
		return "metrics"
	}
	if strings.Contains(path, "/audit-logs") {
		return "audit_log"
	}

	return "unknown"
}

// extractResourceID 提取资源 ID
func (m *AuditMiddleware) extractResourceID(c *gin.Context) string {
	path := c.Request.URL.Path

	// 尝试从路径中提取 ID
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "clusters" && i+1 < len(parts) {
			if _, err := strconv.ParseInt(parts[i+1], 10, 64); err == nil {
				return parts[i+1]
			}
		}
		if part == "topics" && i+1 < len(parts) {
			// Topic 名��可能包含特殊字符
			if i+1 < len(parts) && parts[i+1] != "sync" {
				return parts[i+1]
			}
		}
		if part == "acls" && i+1 < len(parts) {
			if _, err := strconv.ParseInt(parts[i+1], 10, 64); err == nil {
				return parts[i+1]
			}
		}
		if part == "users" && i+1 < len(parts) {
			if _, err := strconv.ParseInt(parts[i+1], 10, 64); err == nil {
				return parts[i+1]
			}
		}
	}

	return ""
}

// extractClusterID 提取集群 ID
func (m *AuditMiddleware) extractClusterID(c *gin.Context) *int64 {
	// 从路径参数获取
	clusterIDStr := c.Param("id")
	if clusterIDStr == "" {
		clusterIDStr = c.Query("cluster_id")
	}

	if clusterIDStr != "" {
		if clusterID, err := strconv.ParseInt(clusterIDStr, 10, 64); err == nil {
			return &clusterID
		}
	}

	// 从请求体获取（仅支持 JSON）
	if c.Request.Body != nil {
		var body map[string]interface{}
		if data, err := io.ReadAll(c.Request.Body); err == nil {
			if err := json.Unmarshal(data, &body); err == nil {
				if clusterID, ok := body["cluster_id"].(float64); ok {
					id := int64(clusterID)
					return &id
				}
			}
		}
	}

	return nil
}

// buildDetails 构建详情
func (m *AuditMiddleware) buildDetails(c *gin.Context, requestBody string, duration time.Duration) string {
	var details strings.Builder

	details.WriteString(fmt.Sprintf("Method: %s, ", c.Request.Method))
	details.WriteString(fmt.Sprintf("Path: %s, ", c.Request.URL.Path))
	details.WriteString(fmt.Sprintf("Status: %d, ", c.Writer.Status()))
	details.WriteString(fmt.Sprintf("Duration: %v", duration))

	if requestBody != "" && len(requestBody) < 1000 {
		details.WriteString(fmt.Sprintf(", Body: %s", requestBody))
	}

	// 添加查询参数
	if query := c.Request.URL.RawQuery; query != "" {
		params, _ := url.ParseQuery(query)
		if clusterID := params.Get("cluster_id"); clusterID != "" {
			details.WriteString(fmt.Sprintf(", ClusterID: %s", clusterID))
		}
	}

	return details.String()
}

// getClientIP 获取客户端 IP
func (m *AuditMiddleware) getClientIP(c *gin.Context) string {
	// 优先从 X-Forwarded-For 获取
	if ip := c.GetHeader("X-Forwarded-For"); ip != "" {
		// 取第一个 IP
		ips := strings.Split(ip, ",")
		return strings.TrimSpace(ips[0])
	}
	// 然后从 X-Real-IP 获取
	if ip := c.GetHeader("X-Real-IP"); ip != "" {
		return ip
	}
	// 最后从 RemoteAddr 获取
	return c.ClientIP()
}

// AuditLogHandler 审计日志处理器
type AuditLogHandler struct {
	auditSvc *audit.Service
}

// NewAuditLogHandler 创建审计日志处理器
func NewAuditLogHandler(auditSvc *audit.Service) *AuditLogHandler {
	return &AuditLogHandler{
		auditSvc: auditSvc,
	}
}

// ListAuditLogs 获取审计日志列表
func (h *AuditLogHandler) ListAuditLogs(c *gin.Context) {
	// 解析查询参数
	var userID *int64
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		id, err := strconv.ParseInt(userIDStr, 10, 64)
		if err == nil {
			userID = &id
		}
	}

	username := c.Query("username")
	operation := c.Query("operation")
	resourceType := c.Query("resource_type")
	status := c.Query("status")

	var startTime, endTime *time.Time
	if startStr := c.Query("start_time"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			startTime = &t
		}
	}
	if endStr := c.Query("end_time"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			endTime = &t
		}
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	logs, total, err := h.auditSvc.QueryLogs(
		c.Request.Context(),
		userID,
		&username,
		&operation,
		&resourceType,
		&status,
		startTime,
		endTime,
		page,
		pageSize,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"data":  logs,
		"total": total,
		"page":  page,
		"size":  pageSize,
	})
}

// GetAuditLog 获取单条审计日志
func (h *AuditLogHandler) GetAuditLog(c *gin.Context) {
	logID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid log id"})
		return
	}

	log, err := h.auditSvc.GetAuditLog(c.Request.Context(), logID)
	if err != nil {
		c.JSON(404, gin.H{"error": "audit log not found"})
		return
	}

	c.JSON(200, log)
}

// ExportAuditLogs 导出审计日志
func (h *AuditLogHandler) ExportAuditLogs(c *gin.Context) {
	format := c.DefaultQuery("format", "json")

	// 解析查询参数（与 ListAuditLogs 相同）
	var userID *int64
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		id, err := strconv.ParseInt(userIDStr, 10, 64)
		if err == nil {
			userID = &id
		}
	}

	username := c.Query("username")
	operation := c.Query("operation")
	resourceType := c.Query("resource_type")
	status := c.Query("status")

	var startTime, endTime *time.Time
	if startStr := c.Query("start_time"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			startTime = &t
		}
	}
	if endStr := c.Query("end_time"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			endTime = &t
		}
	}

	// 获取所有匹配的日志（不分页）
	logs, _, err := h.auditSvc.QueryLogs(
		c.Request.Context(),
		userID,
		&username,
		&operation,
		&resourceType,
		&status,
		startTime,
		endTime,
		1,
		10000, // 最多导出 10000 条
	)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	if format == "csv" {
		// 导出为 CSV
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename=audit_logs.csv")

		// 写入 CSV 头
		c.String(200, "log_id,user_id,username,action,resource,resource_id,cluster_id,status,ip_address,created_at\n")

		// 写入数据
		for _, log := range logs {
			clusterID := ""
			if log.ClusterID != nil {
				clusterID = strconv.FormatInt(*log.ClusterID, 10)
			}
			c.String(200, "%d,%d,%s,%s,%s,%s,%s,%s,%s,%s\n",
				log.LogID,
				log.UserID,
				log.Username,
				log.Action,
				log.Resource,
				log.ResourceID,
				clusterID,
				log.Status,
				log.IPAddress,
				log.CreatedAt.Format(time.RFC3339),
			)
		}
		return
	}

	// 默认导出为 JSON
	c.JSON(200, gin.H{
		"data":       logs,
		"exported":   len(logs),
		"format":     "json",
		"timestamp":  time.Now().Format(time.RFC3339),
	})
}

// IsMultipartForm 检查请求是否为 multipart/form-data
func IsMultipartForm(c *gin.Context) bool {
	return strings.Contains(c.ContentType(), "multipart/form-data")
}

// GetFormValue 获取表单值
func GetFormValue(c *gin.Context, key string) string {
	if IsMultipartForm(c) {
		if form, err := c.MultipartForm(); err == nil {
			if values := form.Value[key]; len(values) > 0 {
				return values[0]
			}
		}
	}
	return c.PostForm(key)
}

// GetFormFile 获取上传的文件
func GetFormFile(c *gin.Context, key string) (*multipart.FileHeader, error) {
	return c.FormFile(key)
}