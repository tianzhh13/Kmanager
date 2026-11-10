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
	ticketRepo      repository.TicketRepository
	clusterRepo     repository.ClusterRepository
	scramUserRepo   repository.ScramUserRepository
	clusterUserRepo repository.ClusterUserRepository
	userRepo        repository.UserRepository
	permissionSvc   *auth.PermissionService
	auditSvc        *audit.Service
	encryptSvc      *encryption.Service
	topicSvc        *topic.Service
	exec            *executor
}

// NewService 创建工单服务实例
func NewService(
	ticketRepo repository.TicketRepository,
	clusterRepo repository.ClusterRepository,
	scramUserRepo repository.ScramUserRepository,
	clusterUserRepo repository.ClusterUserRepository,
	userRepo repository.UserRepository,
	permissionSvc *auth.PermissionService,
	auditSvc *audit.Service,
	encryptSvc *encryption.Service,
	topicSvc *topic.Service,
	scramSvc *scram.Service,
	aclSvc *acl.Service,
) *Service {
	return &Service{
		ticketRepo:      ticketRepo,
		clusterRepo:     clusterRepo,
		scramUserRepo:   scramUserRepo,
		clusterUserRepo: clusterUserRepo,
		userRepo:        userRepo,
		permissionSvc:   permissionSvc,
		auditSvc:        auditSvc,
		encryptSvc:      encryptSvc,
		topicSvc:        topicSvc,
		exec:            newExecutor(topicSvc, scramSvc, aclSvc, encryptSvc),
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

	// 任何登录用户均可为任意集群提交工单（是否放行由审批人决定，这正是工单流程的意义）
	// 集群基础信息对所有用户可见（GET /clusters/options），无需先被授权

	var title string
	switch req.Type {
	case models.TicketTypeTopicCreate:
		form, err := ParseTopicCreateForm(string(req.Form))
		if err != nil {
			return nil, err
		}
		// 查询 Kafka 集群实际 topic 列表：存在性 + '.' 与 '_' 命名冲突校验
		// （Kafka 指标名把 '.' 替换为 '_'，仅这两种字符差异的 topic 会产生指标冲突）
		if form.IsBatch() {
			checks, err := s.topicSvc.CheckTopicNamesAvailable(ctx, req.ClusterID, form.TopicNames())
			if err != nil {
				return nil, fmt.Errorf("检查集群 topic 列表失败: %w", err)
			}
			for _, name := range form.TopicNames() {
				check := checks[name]
				if check == nil {
					continue
				}
				if check.Exists {
					return nil, fmt.Errorf("topic %s 已存在于该集群", name)
				}
				if check.ConflictWith != "" {
					return nil, fmt.Errorf("topic 名称 %s 与既有 topic %s 仅存在 '.' 与 '_' 的差异（Kafka 指标命名冲突），请更换名称", name, check.ConflictWith)
				}
			}
			title = fmt.Sprintf("批量申请创建 %d 个 Topic（%s）", len(form.Items), truncateTopicNames(form.TopicNames()))
		} else {
			check, err := s.topicSvc.CheckTopicNameAvailable(ctx, req.ClusterID, form.TopicName)
			if err != nil {
				return nil, fmt.Errorf("检查集群 topic 列表失败: %w", err)
			}
			if check.Exists {
				return nil, fmt.Errorf("topic %s 已存在于该集群", form.TopicName)
			}
			if check.ConflictWith != "" {
				return nil, fmt.Errorf("topic 名称 %s 与既有 topic %s 仅存在 '.' 与 '_' 的差异（Kafka 指标命名冲突），请更换名称", form.TopicName, check.ConflictWith)
			}
			title = fmt.Sprintf("申请创建 Topic %s", form.TopicName)
		}

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

	// 固定审批人规则：优先该集群被授权的集群管理员（user_id 最小），否则超级管理员（user_id 最小）
	designated, err := s.selectDesignatedApprover(ctx, req.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("确定审批人失败: %w", err)
	}
	if designated == nil {
		return nil, fmt.Errorf("系统缺少可用的审批人（无活跃的集群管理员/超级管理员），请联系平台管理员")
	}

	ticket := &models.Ticket{
		TicketNo:             ticketNo,
		Type:                 req.Type,
		ClusterID:            req.ClusterID,
		ApplicantID:          userID,
		Title:                title,
		Form:                 string(req.Form),
		Status:               models.TicketStatusPending,
		DesignatedApproverID: designated,
		AppliedAt:            now,
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
		// 固定审批人规则：集群管理员的待办 = 指定给自己的工单；超级管理员兜底可见全部待审批
		if role == string(models.RoleClusterAdmin) {
			filter.DesignatedApproverID = &userID
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
	s.enrichTicketNames(ctx, tickets)
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
	s.enrichTicketNames(ctx, []*models.Ticket{ticket})
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
	if err := s.checkApprover(userID, role, ticket); err != nil {
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
	if err := s.checkApprover(userID, role, ticket); err != nil {
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
	if err := s.checkApprover(userID, role, ticket); err != nil {
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
		// 批量执行可能部分成功，保留已创建/跳过明细供审批人查看（重试时会再次汇总）
		if resultJSON != "" {
			ticket.Result = resultJSON
		}
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

// checkApprover 校验审批人权限：超级管理员兜底可审任意工单，集群管理员仅可审指定给自己的工单
func (s *Service) checkApprover(userID int64, role string, ticket *models.Ticket) error {
	if role == string(models.RoleSuperAdmin) {
		return nil
	}
	if ticket.DesignatedApproverID != nil && *ticket.DesignatedApproverID == userID {
		return nil
	}
	return ErrPermissionDenied
}

// selectDesignatedApprover 选定指定审批人（固定规则）：
// 优先该集群被授权的活跃 cluster_admin（user_id 最小），没有则取 user_id 最小的活跃 super_admin
func (s *Service) selectDesignatedApprover(ctx context.Context, clusterID int64) (*int64, error) {
	admins, err := s.clusterUserRepo.ListAdminsByCluster(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	if len(admins) > 0 {
		id := admins[0].UserID
		return &id, nil
	}
	superAdmins, err := s.userRepo.ListByRole(ctx, models.RoleSuperAdmin)
	if err != nil {
		return nil, err
	}
	if len(superAdmins) > 0 {
		id := superAdmins[0].UserID
		return &id, nil
	}
	return nil, nil
}

// enrichTicketNames 填充申请人/审批人/指定审批人用户名（查询失败仅记录日志，不影响主流程）
func (s *Service) enrichTicketNames(ctx context.Context, tickets []*models.Ticket) {
	if len(tickets) == 0 {
		return
	}
	ids := make([]int64, 0, len(tickets)*3)
	for _, t := range tickets {
		ids = append(ids, t.ApplicantID)
		if t.ApproverID != nil {
			ids = append(ids, *t.ApproverID)
		}
		if t.DesignatedApproverID != nil {
			ids = append(ids, *t.DesignatedApproverID)
		}
	}
	users, err := s.userRepo.FindByIDs(ctx, ids)
	if err != nil {
		logger.Error("ticket: failed to load user names", "error", err)
		return
	}
	for _, t := range tickets {
		if u, ok := users[t.ApplicantID]; ok {
			t.ApplicantName = u.Username
		}
		if t.ApproverID != nil {
			if u, ok := users[*t.ApproverID]; ok {
				t.ApproverName = u.Username
			}
		}
		if t.DesignatedApproverID != nil {
			if u, ok := users[*t.DesignatedApproverID]; ok {
				t.DesignatedApproverName = u.Username
			}
		}
	}
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

// truncateTopicNames 批量工单标题里的 topic 名列表，超长截断（title 列为 varchar(128)）
func truncateTopicNames(names []string) string {
	joined := strings.Join(names, "、")
	if runes := []rune(joined); len(runes) > 80 {
		joined = string(runes[:80]) + "..."
	}
	return joined
}
