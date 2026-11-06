package ticket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"kafka-management-platform/internal/logger"
	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/repository"
	"kafka-management-platform/internal/service/acl"
	"kafka-management-platform/internal/service/audit"
	"kafka-management-platform/internal/service/auth"
	"kafka-management-platform/internal/service/scram"
	"kafka-management-platform/internal/service/topic"
	"kafka-management-platform/pkg/encryption"

	"gorm.io/gorm"
)

var (
	// ErrTicketNotFound 工单不存在
	ErrTicketNotFound = errors.New("工单不存在")
	// ErrInvalidStatus 无效的工单状态
	ErrInvalidStatus = errors.New("当前工单状态不允许该操作")
	// ErrPermissionDenied 无权限
	ErrPermissionDenied = errors.New("无权限执行该操作")
	// ErrRemarkRequired 驳回必须填写原因
	ErrRemarkRequired = errors.New("请填写审批备注")
	// ErrCredentialUnavailable 凭证不可用
	ErrCredentialUnavailable = errors.New("凭证不存在或已查看过")
)

// Scope 常量
const (
	ScopeMine    = "mine"
	ScopePending = "pending"
	ScopeAll     = "all"
)

// Service 工单服务
type Service struct {
	ticketRepo    repository.TicketRepository
	clusterRepo   repository.ClusterRepository
	topicRepo     repository.TopicRepository
	scramUserRepo repository.ScramUserRepository
	userRepo      repository.UserRepository
	permissionSvc *auth.PermissionService
	auditSvc      *audit.Service
	encryptSvc    *encryption.Service
	exec          *executor
}

// NewService 创建工单服务实例
func NewService(
	ticketRepo repository.TicketRepository,
	clusterRepo repository.ClusterRepository,
	topicRepo repository.TopicRepository,
	scramUserRepo repository.ScramUserRepository,
	userRepo repository.UserRepository,
	permissionSvc *auth.PermissionService,
	auditSvc *audit.Service,
	encryptSvc *encryption.Service,
	topicSvc *topic.Service,
	scramSvc *scram.Service,
	aclSvc *acl.Service,
) *Service {
	return &Service{
		ticketRepo:    ticketRepo,
		clusterRepo:   clusterRepo,
		topicRepo:     topicRepo,
		scramUserRepo: scramUserRepo,
		userRepo:      userRepo,
		permissionSvc: permissionSvc,
		auditSvc:      auditSvc,
		encryptSvc:    encryptSvc,
		exec:          newExecutor(topicSvc, scramSvc, aclSvc, encryptSvc),
	}
}

// SubmitRequest 提交工单请求
type SubmitRequest struct {
	Type      models.TicketType `json:"type" binding:"required"`
	ClusterID int64             `json:"cluster_id" binding:"required"`
	Form      json.RawMessage   `json:"form" binding:"required"`
}

// ListRequest 工单列表请求
type ListRequest struct {
	Scope     string `form:"scope"`
	Status    string `form:"status"`
	Type      string `form:"type"`
	ClusterID *int64 `form:"cluster_id"`
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
}

// ListResponse 工单列表响应
type ListResponse struct {
	Data  []*models.Ticket `json:"data"`
	Total int64            `json:"total"`
}

// Submit 提交工单
func (s *Service) Submit(ctx context.Context, userID int64, username, role string, req *SubmitRequest) (*models.Ticket, error) {
	// 校验集群
	cluster, err := s.clusterRepo.FindByID(ctx, req.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("集群不存在")
	}
	if cluster.Status == models.ClusterStatusInactive {
		return nil, fmt.Errorf("集群已停用，无法提交工单")
	}

	// 非超级管理员只能对被授权的集群提交工单
	if role != string(models.RoleSuperAdmin) {
		if err := s.checkClusterAccess(ctx, userID, req.ClusterID); err != nil {
			return nil, err
		}
	}

	var title string
	switch req.Type {
	case models.TicketTypeTopicCreate:
		form, err := ParseTopicCreateForm(string(req.Form))
		if err != nil {
			return nil, err
		}
		exists, err := s.topicRepo.Exists(ctx, req.ClusterID, form.TopicName)
		if err != nil {
			return nil, fmt.Errorf("检查 topic 是否存在失败: %w", err)
		}
		if exists {
			return nil, fmt.Errorf("topic %s 在该集群已存在", form.TopicName)
		}
		title = fmt.Sprintf("申请创建 Topic %s", form.TopicName)

	case models.TicketTypePermissionGrant:
		// 权限工单仅适用于 SCRAM 集群（PLAINTEXT 无认证、Kerberos 用户在 KDC 管理）
		if cluster.AuthType != models.AuthTypeSCRAM {
			return nil, fmt.Errorf("仅 SCRAM 认证的集群支持申请权限（该集群认证方式为 %s）", cluster.AuthType)
		}
		form, err := ParsePermissionGrantForm(string(req.Form))
		if err != nil {
			return nil, err
		}
		exists, err := s.scramUserRepo.Exists(ctx, req.ClusterID, form.Username)
		if err != nil {
			return nil, fmt.Errorf("检查 SCRAM 用户失败: %w", err)
		}
		if form.Mode == ModeNewUser && exists {
			return nil, fmt.Errorf("SCRAM 用户 %s 在该集群已存在，请选择已有用户或更换用户名", form.Username)
		}
		if form.Mode == ModeExistingUser && !exists {
			return nil, fmt.Errorf("SCRAM 用户 %s 在该集群不存在", form.Username)
		}
		title = fmt.Sprintf("申请权限: %s %s -> %s", form.Username, permissionLabel(form.Permission), strings.Join(form.Topics, ", "))

	default:
		return nil, fmt.Errorf("不支持的工单类型: %s", req.Type)
	}

	now := time.Now()
	ticketNo, err := s.nextTicketNo(ctx, now)
	if err != nil {
		return nil, fmt.Errorf("生成工单号失败: %w", err)
	}

	ticket := &models.Ticket{
		TicketNo:    ticketNo,
		Type:        req.Type,
		ClusterID:   req.ClusterID,
		ApplicantID: userID,
		Title:       title,
		Form:        string(req.Form),
		Status:      models.TicketStatusPending,
		AppliedAt:   now,
	}
	if err := s.ticketRepo.Create(ctx, ticket); err != nil {
		return nil, fmt.Errorf("保存工单失败: %w", err)
	}

	s.audit(ctx, userID, username, "submit_ticket", ticket.TicketNo, fmt.Sprintf("提交工单: %s（集群 %s）", title, cluster.ClusterName), "success", &req.ClusterID)
	logger.Info("ticket: submitted", "ticket_no", ticketNo, "type", req.Type, "cluster_id", req.ClusterID, "applicant", username)
	return ticket, nil
}

// List 工单列表（scope: mine 本人 / pending 待我审批 / all 全部）
func (s *Service) List(ctx context.Context, userID int64, role string, req *ListRequest) (*ListResponse, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 20
	}

	filter := &repository.TicketListFilter{
		Status:    req.Status,
		Type:      req.Type,
		ClusterID: req.ClusterID,
		Offset:    (req.Page - 1) * req.PageSize,
		Limit:     req.PageSize,
	}

	isAdmin := role == string(models.RoleSuperAdmin) || role == string(models.RoleClusterAdmin)
	switch {
	case req.Scope == ScopePending && isAdmin:
		filter.Status = string(models.TicketStatusPending)
		// 集群管理员只能看到自己名下集群的待审批工单
		if role == string(models.RoleClusterAdmin) {
			clusterIDs, err := s.permissionSvc.GetAllowedClusters(ctx, userID)
			if err != nil {
				return nil, fmt.Errorf("获取授权集群失败: %w", err)
			}
			if clusterIDs == nil {
				clusterIDs = []int64{}
			}
			filter.ClusterIDs = clusterIDs
		}
	case req.Scope == ScopeAll && isAdmin:
		if role == string(models.RoleClusterAdmin) {
			clusterIDs, err := s.permissionSvc.GetAllowedClusters(ctx, userID)
			if err != nil {
				return nil, fmt.Errorf("获取授权集群失败: %w", err)
			}
			if clusterIDs == nil {
				clusterIDs = []int64{}
			}
			filter.ClusterIDs = clusterIDs
		}
	default:
		// 普通用户（或未指定 scope）只看自己的工单
		filter.ApplicantID = &userID
	}

	tickets, total, err := s.ticketRepo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("查询工单失败: %w", err)
	}
	return &ListResponse{Data: tickets, Total: total}, nil
}

// Get 工单详情
func (s *Service) Get(ctx context.Context, userID int64, role string, ticketID int64) (*models.Ticket, error) {
	ticket, err := s.getTicket(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket.ApplicantID != userID {
		ok, err := s.permissionSvc.CheckClusterPermission(ctx, userID, ticket.ClusterID, role)
		if err != nil || !ok {
			return nil, ErrPermissionDenied
		}
	}
	return ticket, nil
}

// Approve 审批通过并触发执行
func (s *Service) Approve(ctx context.Context, userID int64, username, role string, ticketID int64, remark string) (*models.Ticket, error) {
	ticket, err := s.getTicket(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket.Status != models.TicketStatusPending {
		return nil, ErrInvalidStatus
	}
	if err := s.checkApprover(ctx, userID, ticket.ClusterID, role); err != nil {
		return nil, err
	}

	now := time.Now()
	ticket.Status = models.TicketStatusExecuting
	ticket.ApproverID = &userID
	ticket.ApproveRemark = remark
	ticket.ApprovedAt = &now
	if err := s.ticketRepo.Update(ctx, ticket); err != nil {
		return nil, fmt.Errorf("更新工单失败: %w", err)
	}

	s.audit(ctx, userID, username, "approve_ticket", ticket.TicketNo, "审批通过: "+ticket.Title, "success", &ticket.ClusterID)

	s.execute(ctx, ticket, username, "approve_ticket")
	return ticket, nil
}

// Reject 驳回工单
func (s *Service) Reject(ctx context.Context, userID int64, username, role string, ticketID int64, remark string) (*models.Ticket, error) {
	if strings.TrimSpace(remark) == "" {
		return nil, ErrRemarkRequired
	}
	ticket, err := s.getTicket(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket.Status != models.TicketStatusPending {
		return nil, ErrInvalidStatus
	}
	if err := s.checkApprover(ctx, userID, ticket.ClusterID, role); err != nil {
		return nil, err
	}

	ticket.Status = models.TicketStatusRejected
	ticket.ApproverID = &userID
	ticket.ApproveRemark = remark
	now := time.Now()
	ticket.ApprovedAt = &now
	ticket.CompletedAt = &now
	if err := s.ticketRepo.Update(ctx, ticket); err != nil {
		return nil, fmt.Errorf("更新工单失败: %w", err)
	}

	s.audit(ctx, userID, username, "reject_ticket", ticket.TicketNo, "驳回工单: "+ticket.Title+"（原因: "+remark+"）", "success", &ticket.ClusterID)
	logger.Info("ticket: rejected", "ticket_no", ticket.TicketNo, "approver", username)
	return ticket, nil
}

// Cancel 申请人撤回工单（仅待审批状态）
func (s *Service) Cancel(ctx context.Context, userID int64, username string, ticketID int64) (*models.Ticket, error) {
	ticket, err := s.getTicket(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket.Status != models.TicketStatusPending {
		return nil, ErrInvalidStatus
	}
	if ticket.ApplicantID != userID {
		return nil, ErrPermissionDenied
	}

	ticket.Status = models.TicketStatusCancelled
	now := time.Now()
	ticket.CompletedAt = &now
	if err := s.ticketRepo.Update(ctx, ticket); err != nil {
		return nil, fmt.Errorf("更新工单失败: %w", err)
	}

	s.audit(ctx, userID, username, "cancel_ticket", ticket.TicketNo, "撤回工单: "+ticket.Title, "success", &ticket.ClusterID)
	logger.Info("ticket: cancelled", "ticket_no", ticket.TicketNo, "applicant", username)
	return ticket, nil
}

// Retry 重试失败的工单（执行器幂等，已创建的资源自动跳过）
func (s *Service) Retry(ctx context.Context, userID int64, username, role string, ticketID int64) (*models.Ticket, error) {
	ticket, err := s.getTicket(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket.Status != models.TicketStatusFailed {
		return nil, ErrInvalidStatus
	}
	if err := s.checkApprover(ctx, userID, ticket.ClusterID, role); err != nil {
		return nil, err
	}

	ticket.Status = models.TicketStatusExecuting
	ticket.ErrorMsg = ""
	if err := s.ticketRepo.Update(ctx, ticket); err != nil {
		return nil, fmt.Errorf("更新工单失败: %w", err)
	}

	s.audit(ctx, userID, username, "retry_ticket", ticket.TicketNo, "重试执行工单: "+ticket.Title, "success", &ticket.ClusterID)

	s.execute(ctx, ticket, username, "retry_ticket")
	return ticket, nil
}

// TakeCredential 申请人一次性查看新建 SCRAM 用户的凭证
func (s *Service) TakeCredential(ctx context.Context, userID int64, username string, ticketID int64) (map[string]string, error) {
	ticket, err := s.getTicket(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket.ApplicantID != userID {
		return nil, ErrPermissionDenied
	}
	if ticket.Status != models.TicketStatusSucceeded || ticket.Credential == "" {
		return nil, ErrCredentialUnavailable
	}

	decrypted, err := s.encryptSvc.DecryptString(ticket.Credential)
	if err != nil {
		return nil, fmt.Errorf("解密凭证失败: %w", err)
	}
	var cred credentialData
	if err := json.Unmarshal([]byte(decrypted), &cred); err != nil {
		return nil, fmt.Errorf("解析凭证失败: %w", err)
	}

	// 一次性查看：返回后立即清除
	ticket.Credential = ""
	now := time.Now()
	ticket.CredentialViewedAt = &now
	if err := s.ticketRepo.Update(ctx, ticket); err != nil {
		return nil, fmt.Errorf("更新工单失败: %w", err)
	}

	s.audit(ctx, userID, username, "view_ticket_credential", ticket.TicketNo, "查看工单凭证", "success", &ticket.ClusterID)
	return map[string]string{
		"username":  cred.Username,
		"password":  cred.Password,
		"mechanism": cred.Mechanism,
	}, nil
}

// Stats 工单状态统计（普通用户统计自己的，管理员统计全部）
func (s *Service) Stats(ctx context.Context, userID int64, role string) (map[string]int64, error) {
	isAdmin := role == string(models.RoleSuperAdmin) || role == string(models.RoleClusterAdmin)
	var applicantID *int64
	if !isAdmin {
		applicantID = &userID
	}
	return s.ticketRepo.CountByStatus(ctx, applicantID)
}

// execute 同步执行工单并落最终状态
func (s *Service) execute(ctx context.Context, ticket *models.Ticket, operatorUsername, action string) {
	applicantUsername := operatorUsername
	if user, err := s.userRepo.FindByID(ctx, ticket.ApplicantID); err == nil {
		applicantUsername = user.Username
	}

	resultJSON, encryptedCredential, err := s.exec.Execute(ctx, ticket, applicantUsername)
	now := time.Now()
	ticket.CompletedAt = &now
	if err != nil {
		logExecuteFailure(ticket, err)
		ticket.Status = models.TicketStatusFailed
		ticket.ErrorMsg = err.Error()
	} else {
		ticket.Status = models.TicketStatusSucceeded
		ticket.Result = resultJSON
		ticket.ErrorMsg = ""
	}
	// 执行失败但新建用户可能已成功，保留凭证避免重试时丢失密码
	if encryptedCredential != "" {
		ticket.Credential = encryptedCredential
	}

	if err := s.ticketRepo.Update(ctx, ticket); err != nil {
		logger.Error("ticket: failed to persist execution result", "ticket_no", ticket.TicketNo, "error", err)
		return
	}

	status := "success"
	if ticket.Status == models.TicketStatusFailed {
		status = "failed"
	}
	details := fmt.Sprintf("执行工单 %s: %s", ticket.Title, status)
	if ticket.ErrorMsg != "" {
		details += "（" + ticket.ErrorMsg + "）"
	}
	var approverID int64
	if ticket.ApproverID != nil {
		approverID = *ticket.ApproverID
	}
	s.audit(ctx, approverID, operatorUsername, action, ticket.TicketNo, details, status, &ticket.ClusterID)
}

// checkApprover 校验审批人权限：超级管理员全部，集群管理员限其名下集群
func (s *Service) checkApprover(ctx context.Context, userID, clusterID int64, role string) error {
	ok, err := s.permissionSvc.CheckClusterPermission(ctx, userID, clusterID, role)
	if err != nil {
		return fmt.Errorf("校验审批权限失败: %w", err)
	}
	if !ok {
		return ErrPermissionDenied
	}
	return nil
}

// checkClusterAccess 校验用户对集群的访问权限
func (s *Service) checkClusterAccess(ctx context.Context, userID, clusterID int64) error {
	clusterIDs, err := s.permissionSvc.GetAllowedClusters(ctx, userID)
	if err != nil {
		return fmt.Errorf("获取授权集群失败: %w", err)
	}
	if clusterIDs == nil {
		return nil
	}
	for _, id := range clusterIDs {
		if id == clusterID {
			return nil
		}
	}
	return ErrPermissionDenied
}

// getTicket 查询工单
func (s *Service) getTicket(ctx context.Context, ticketID int64) (*models.Ticket, error) {
	ticket, err := s.ticketRepo.FindByID(ctx, ticketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, fmt.Errorf("查询工单失败: %w", err)
	}
	return ticket, nil
}

// nextTicketNo 生成工单号 TK-YYYYMMDD-NNNN（按天递增）
func (s *Service) nextTicketNo(ctx context.Context, now time.Time) (string, error) {
	prefix := fmt.Sprintf("TK-%s-", now.Format("20060102"))
	maxNo, err := s.ticketRepo.MaxTicketNoByPrefix(ctx, prefix)
	if err != nil {
		return "", err
	}
	seq := 1
	if maxNo != "" {
		parts := strings.Split(maxNo, "-")
		if len(parts) == 3 {
			if v, parseErr := strconv.Atoi(parts[2]); parseErr == nil {
				seq = v + 1
			}
		}
	}
	return fmt.Sprintf("%s%04d", prefix, seq), nil
}

// audit 记录审计日志（失败仅记录日志，不影响主流程）
func (s *Service) audit(ctx context.Context, userID int64, username, action, resourceID, details, status string, clusterID *int64) {
	if err := s.auditSvc.Log(ctx, &audit.LogRequest{
		UserID:       userID,
		Username:     username,
		Action:       action,
		ResourceType: "ticket",
		ResourceID:   resourceID,
		ClusterID:    clusterID,
		Details:      details,
		Status:       status,
	}); err != nil {
		logger.Error("ticket: failed to write audit log", "action", action, "resource_id", resourceID, "error", err)
	}
}

// permissionLabel 权限模板中文名
func permissionLabel(permission string) string {
	switch permission {
	case PermissionRead:
		return "只读"
	case PermissionWrite:
		return "只写"
	case PermissionReadWrite:
		return "读写"
	default:
		return permission
	}
}
