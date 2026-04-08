package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/repository"

	"github.com/gin-gonic/gin"
)

var (
	// ErrInvalidUserID 无效的用户ID
	ErrInvalidUserID = errors.New("invalid user_id")
	// ErrInvalidUsername 无效的用户名
	ErrInvalidUsername = errors.New("invalid username")
	// ErrInvalidAction 无效的操作类型
	ErrInvalidAction = errors.New("invalid action")
	// ErrInvalidResourceType 无效的资源类型
	ErrInvalidResourceType = errors.New("invalid resource_type")
	// ErrInvalidStatus 无效的状态
	ErrInvalidStatus = errors.New("invalid status")
)

// ValidActions 有效的操作类型
var ValidActions = map[string]bool{
	"login":                true,
	"logout":               true,
	"create_cluster":       true,
	"update_cluster":       true,
	"delete_cluster":       true,
	"create_topic":         true,
	"update_topic":         true,
	"delete_topic":         true,
	"create_acl":           true,
	"delete_acl":           true,
	"batch_delete_acl":     true,
	"create_user":          true,
	"update_user":          true,
	"delete_user":          true,
	"disable_user":         true,
	"grant_cluster_access": true,
	"revoke_cluster_access": true,
	"test_connection":      true,
	"sync_topics":          true,
	"sync_acls":            true,
	"export_logs":          true,
}

// ValidResourceTypes 有效的资源类型
var ValidResourceTypes = map[string]bool{
	"user":    true,
	"cluster": true,
	"topic":   true,
	"acl":     true,
	"system":  true,
}

// ValidStatuses 有效的状态
var ValidStatuses = map[string]bool{
	"success": true,
	"failed":  true,
}

// LogRequest 审计日志请求
type LogRequest struct {
	UserID       int64                  `json:"user_id"`
	Username     string                 `json:"username"`
	Action       string                 `json:"action"`
	ResourceType string                 `json:"resource_type"`
	ResourceID   string                 `json:"resource_id"`
	ClusterID    *int64                 `json:"cluster_id,omitempty"`
	Details      interface{}            `json:"details,omitempty"`
	IPAddress    string                 `json:"ip_address"`
	UserAgent    string                 `json:"user_agent"`
	Status       string                 `json:"status"`
}

// Service 审计服务
type Service struct {
	auditLogRepo repository.AuditLogRepository
}

// NewService 创建审计服务
func NewService(auditLogRepo repository.AuditLogRepository) *Service {
	return &Service{
		auditLogRepo: auditLogRepo,
	}
}

// Log 记录审计日志（支持 context.Context 和 LogRequest）
func (s *Service) Log(ctx context.Context, req *LogRequest) error {
	// 验证字段完整性
	if err := s.validateAuditLogFields(req.UserID, req.Username, req.Action, req.ResourceType, req.Status); err != nil {
		return err
	}

	// 将 details 转换为 JSON 字符串
	var detailsStr string
	if req.Details != nil {
		if b, err := json.Marshal(req.Details); err == nil {
			detailsStr = string(b)
		}
	}

	auditLog := &models.AuditLog{
		UserID:     req.UserID,
		Username:   req.Username,
		Action:     req.Action,
		Resource:   req.ResourceType,
		ResourceID: req.ResourceID,
		ClusterID:  req.ClusterID,
		Details:    detailsStr,
		IPAddress:  req.IPAddress,
		UserAgent:  req.UserAgent,
		Status:     models.AuditStatus(req.Status),
	}

	return s.auditLogRepo.Create(nil, auditLog)
}

// LogGin 记录审计日志（使用 gin.Context）
func (s *Service) LogGin(ctx *gin.Context, operation string, resourceType string, resourceID string, details string) error {
	// 验证字段完整性
	if err := s.validateAuditLogFields(0, "", operation, resourceType, "success"); err != nil {
		return err
	}

	userID := int64(0)
	username := ""
	
	if ctx != nil {
		if v, exists := ctx.Get("user_id"); exists {
			userID = v.(int64)
		}
		if v, exists := ctx.Get("username"); exists {
			username = v.(string)
		}
	}

	auditLog := &models.AuditLog{
		UserID:       userID,
		Username:     username,
		Action:       operation,
		Resource:     resourceType,
		ResourceID:   resourceID,
		Details:      details,
		IPAddress:    getClientIP(ctx),
		UserAgent:    getUserAgent(ctx),
		Status:       models.AuditStatusSuccess,
	}

	return s.auditLogRepo.Create(nil, auditLog)
}

// LogError 记录错误操作的审计日志
func (s *Service) LogError(ctx *gin.Context, operation string, resourceType string, resourceID string, details string, err error) error {
	// 验证字段完整性
	if err := s.validateAuditLogFields(0, "", operation, resourceType, "failed"); err != nil {
		return err
	}

	userID := int64(0)
	username := ""
	
	if ctx != nil {
		if v, exists := ctx.Get("user_id"); exists {
			userID = v.(int64)
		}
		if v, exists := ctx.Get("username"); exists {
			username = v.(string)
		}
	}

	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	auditLog := &models.AuditLog{
		UserID:       userID,
		Username:     username,
		Action:       operation,
		Resource:     resourceType,
		ResourceID:   resourceID,
		Details:      details,
		IPAddress:    getClientIP(ctx),
		UserAgent:    getUserAgent(ctx),
		Status:       models.AuditStatusFailed,
		ErrorMsg:     errMsg,
	}

	return s.auditLogRepo.Create(nil, auditLog)
}

// LogWithDetails 记录带完整详情的审计日志
func (s *Service) LogWithDetails(userID int64, username, operation, resourceType, resourceID, details, ipAddress, userAgent, status string, clusterID *int64) error {
	// 验证字段完整性
	if err := s.validateAuditLogFields(userID, username, operation, resourceType, status); err != nil {
		return err
	}

	auditLog := &models.AuditLog{
		UserID:       userID,
		Username:     username,
		Action:       operation,
		Resource:     resourceType,
		ResourceID:   resourceID,
		ClusterID:    clusterID,
		Details:      details,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		Status:       models.AuditStatus(status),
	}

	return s.auditLogRepo.Create(nil, auditLog)
}

// validateAuditLogFields 验证审计日志字段完整性
func (s *Service) validateAuditLogFields(userID int64, username, action, resourceType, status string) error {
	// 验证操作类型
	if action != "" && !ValidActions[action] {
		return fmt.Errorf("%w: %s", ErrInvalidAction, action)
	}

	// 验证资源类型
	if resourceType != "" && !ValidResourceTypes[resourceType] {
		return fmt.Errorf("%w: %s", ErrInvalidResourceType, resourceType)
	}

	// 验证状态
	if status != "" && !ValidStatuses[status] {
		return fmt.Errorf("%w: %s", ErrInvalidStatus, status)
	}

	return nil
}

// QueryLogs 查询审计日志
func (s *Service) QueryLogs(ctx context.Context, userID *int64, username, operation, resourceType, status *string, startTime, endTime *time.Time, page, pageSize int) ([]models.AuditLog, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize

	return s.auditLogRepo.Query(ctx, userID, username, operation, resourceType, status, startTime, endTime, pageSize, offset)
}

// CleanExpiredLogs 清理过期日志（默认180天）
func (s *Service) CleanExpiredLogs(ctx context.Context, days int) (int64, error) {
	if days <= 0 {
		days = 180
	}
	
	cutoffTime := time.Now().AddDate(0, 0, -days)
	return s.auditLogRepo.DeleteBefore(ctx, cutoffTime)
}

// GetAuditLog 获取单条审计日志
func (s *Service) GetAuditLog(ctx context.Context, logID int64) (*models.AuditLog, error) {
	return s.auditLogRepo.FindByID(ctx, logID)
}

// getClientIP 获取客户端IP
func getClientIP(ctx *gin.Context) string {
	if ctx == nil {
		return ""
	}
	
	// 优先从 X-Forwarded-For 获取
	if ip := ctx.GetHeader("X-Forwarded-For"); ip != "" {
		return ip
	}
	// 然后从 X-Real-IP 获取
	if ip := ctx.GetHeader("X-Real-IP"); ip != "" {
		return ip
	}
	// 最后从 RemoteAddr 获取
	return ctx.ClientIP()
}

// getUserAgent 获取用户代理
func getUserAgent(ctx *gin.Context) string {
	if ctx == nil {
		return ""
	}
	return ctx.GetHeader("User-Agent")
}

// Handler HTTP 处理器
type Handler struct {
	svc *Service
}

// NewHandler 创建审计处理器
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ListAuditLogs 处理获取审计日志列表请求
func (h *Handler) ListAuditLogs(c *gin.Context) {
	// 解析查询参数
	var userID *int64
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		id, err := fmt.Sscanf(userIDStr, "%d", &userID)
		if err != nil || id == 0 {
			userID = nil
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

	logs, total, err := h.svc.QueryLogs(
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

// CleanLogs 处理清理过期日志请求
func (h *Handler) CleanLogs(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "180"))

	deleted, err := h.svc.CleanExpiredLogs(c.Request.Context(), days)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"message":       "logs cleaned successfully",
		"deleted_count": deleted,
	})
}