package ticket

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"kafka-management-platform/internal/logger"
	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/service/acl"
	"kafka-management-platform/internal/service/scram"
	"kafka-management-platform/internal/service/topic"
	"kafka-management-platform/pkg/encryption"
)

const passwordCharset = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// executor 工单执行器：审批通过后调用既有 service 完成实际资源操作，幂等可重试
type executor struct {
	topicSvc   *topic.Service
	scramSvc   *scram.Service
	aclSvc     *acl.Service
	encryptSvc *encryption.Service
}

func newExecutor(topicSvc *topic.Service, scramSvc *scram.Service, aclSvc *acl.Service, encryptSvc *encryption.Service) *executor {
	return &executor{
		topicSvc:   topicSvc,
		scramSvc:   scramSvc,
		aclSvc:     aclSvc,
		encryptSvc: encryptSvc,
	}
}

// executeResult 执行结果摘要（不含凭证）
type executeResult struct {
	Created []string `json:"created,omitempty"`
	Skipped []string `json:"skipped,omitempty"`
}

func (r *executeResult) addCreated(item string) { r.Created = append(r.Created, item) }
func (r *executeResult) addSkipped(item string) { r.Skipped = append(r.Skipped, item) }
func (r *executeResult) toJSON() string {
	data, _ := json.Marshal(r)
	return string(data)
}

// credentialData 新建 SCRAM 用户的凭证（加密存储，一次性查看）
type credentialData struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Mechanism string `json:"mechanism"`
}

// Execute 执行工单。返回执行结果 JSON 与加密后的凭证（仅有新建用户时非空）。
// 幂等：topic 已存在 / SCRAM 用户已存在 / ACL 已存在均跳过而不报错，失败工单可直接重试。
func (e *executor) Execute(ctx context.Context, ticket *models.Ticket, applicantUsername string) (resultJSON string, encryptedCredential string, err error) {
	switch ticket.Type {
	case models.TicketTypeTopicCreate:
		resultJSON, err = e.executeTopicCreate(ctx, ticket, applicantUsername)
	case models.TicketTypePermissionGrant:
		resultJSON, encryptedCredential, err = e.executePermissionGrant(ctx, ticket)
	default:
		err = fmt.Errorf("unknown ticket type: %s", ticket.Type)
	}
	return resultJSON, encryptedCredential, err
}

// executeTopicCreate 执行申请 Topic 工单（单个 / 批量共用：逐个创建，幂等可重试）
func (e *executor) executeTopicCreate(ctx context.Context, ticket *models.Ticket, applicantUsername string) (string, error) {
	form, err := ParseTopicCreateForm(ticket.Form)
	if err != nil {
		return "", err
	}

	items := form.Items
	if !form.IsBatch() {
		items = []TopicCreateItem{{
			TopicName:         form.TopicName,
			Partitions:        form.Partitions,
			ReplicationFactor: form.ReplicationFactor,
			RetentionDays:     form.RetentionDays,
			BusinessDesc:      form.BusinessDesc,
		}}
	}

	result := &executeResult{}
	var failed []string

	// 提交与执行之间集群可能有变化，执行前再查一次 Kafka 实际 topic 列表
	// （已存在 → 幂等跳过；存在仅 '.' 与 '_' 差异的同名冲突 → 失败，防止 Kafka 指标命名冲突）
	// 批量工单一次拉取列表完成全部名称预检，避免逐个建立 Admin 客户端
	precheck := make(map[string]*topic.TopicNameCheckResult, len(items))
	if len(items) == 1 {
		if check, checkErr := e.topicSvc.CheckTopicNameAvailable(ctx, ticket.ClusterID, items[0].TopicName); checkErr == nil {
			precheck[items[0].TopicName] = check
		}
	} else {
		checks, checkErr := e.topicSvc.CheckTopicNamesAvailable(ctx, ticket.ClusterID, form.TopicNames())
		if checkErr == nil {
			precheck = checks
		}
	}

	for i := range items {
		item := &items[i]
		if check, ok := precheck[item.TopicName]; ok {
			if check.Exists {
				result.addSkipped(fmt.Sprintf("topic %s 已存在，跳过创建", item.TopicName))
				continue
			}
			if check.ConflictWith != "" {
				failed = append(failed, fmt.Sprintf("%s（与 %s 命名冲突）", item.TopicName, check.ConflictWith))
				continue
			}
		}

		if err := e.createTopic(ctx, ticket, applicantUsername, item, result); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", item.TopicName, err))
		}
	}

	// 批量允许部分成功：失败项记入错误信息，工单标记失败后可直接重试（已创建的自动跳过）
	if len(failed) > 0 {
		return result.toJSON(), fmt.Errorf("%d/%d 个 topic 创建失败: %s", len(failed), len(items), strings.Join(failed, "；"))
	}
	return result.toJSON(), nil
}

// createTopic 创建单个 topic（幂等：已存在则跳过），申请人信息由平台自动写入 topic 描述
func (e *executor) createTopic(ctx context.Context, ticket *models.Ticket, applicantUsername string, item *TopicCreateItem, result *executeResult) error {
	config := make(map[string]string)
	if item.RetentionDays > 0 {
		config["retention.ms"] = fmt.Sprintf("%d", item.RetentionDays*24*int64(time.Hour)/int64(time.Millisecond))
	}

	err := e.topicSvc.CreateTopic(ctx, &topic.CreateTopicRequest{
		ClusterID:         ticket.ClusterID,
		TopicName:         item.TopicName,
		Description:       BuildTopicDescription(applicantUsername, ticket.AppliedAt, item.BusinessDesc),
		Partitions:        item.Partitions,
		ReplicationFactor: item.ReplicationFactor,
		Config:            config,
	})
	if err != nil {
		if errors.Is(err, topic.ErrTopicAlreadyExists) {
			result.addSkipped(fmt.Sprintf("topic %s 已存在，跳过创建", item.TopicName))
			return nil
		}
		return err
	}
	result.addCreated(fmt.Sprintf("topic %s（%d 分区 %d 副本）", item.TopicName, item.Partitions, item.ReplicationFactor))
	return nil
}

// executePermissionGrant 执行申请权限工单
func (e *executor) executePermissionGrant(ctx context.Context, ticket *models.Ticket) (string, string, error) {
	form, err := ParsePermissionGrantForm(ticket.Form)
	if err != nil {
		return "", "", err
	}

	result := &executeResult{}
	encryptedCredential := ""

	// 1. 新建用户（幂等：已存在则跳过，不覆盖密码）
	if form.Mode == ModeNewUser {
		password, genErr := generatePassword(24)
		if genErr != nil {
			return "", "", fmt.Errorf("failed to generate password: %w", genErr)
		}
		err = e.scramSvc.CreateUser(ctx, &scram.CreateUserRequest{
			ClusterID: ticket.ClusterID,
			Username:  form.Username,
			Password:  password,
			Mechanism: form.Mechanism,
		})
		if err != nil {
			if errors.Is(err, scram.ErrUserAlreadyExists) {
				result.addSkipped(fmt.Sprintf("SCRAM 用户 %s 已存在，跳过创建", form.Username))
			} else {
				return "", "", err
			}
		} else {
			result.addCreated(fmt.Sprintf("SCRAM 用户 %s（%s）", form.Username, form.Mechanism))
			credData, _ := json.Marshal(&credentialData{
				Username:  form.Username,
				Password:  password,
				Mechanism: form.Mechanism,
			})
			encryptedCredential, err = e.encryptSvc.EncryptString(string(credData))
			if err != nil {
				return "", "", fmt.Errorf("failed to encrypt credential: %w", err)
			}
		}
	}

	// 2. 展开并创建 ACL（幂等：按 principal 比对既有 ACL 去重）
	rules := form.ExpandACLs()
	principal := form.Principal()

	resp, err := e.aclSvc.ListACLs(ctx, &acl.ListACLsRequest{
		ClusterID: ticket.ClusterID,
		Principal: principal,
		Offset:    0,
		Limit:     1000,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to list existing acls: %w", err)
	}
	existing := make(map[string]bool, len(resp.Data))
	for _, a := range resp.Data {
		key := fmt.Sprintf("%s|%s|%s|%s", a.ResourceType, a.ResourceName, a.ResourcePattern, a.Operation)
		existing[key] = true
	}

	for _, rule := range rules {
		key := fmt.Sprintf("%s|%s|%s|%s", rule.ResourceType, rule.ResourceName, rule.ResourcePattern, rule.Operation)
		if existing[key] {
			result.addSkipped(fmt.Sprintf("ACL 已存在: %s %s %s %s",
				strings.ToUpper(rule.ResourceType), rule.ResourceName, rule.ResourcePattern, rule.Operation))
			continue
		}
		err = e.aclSvc.CreateACL(ctx, &acl.CreateACLRequest{
			ClusterID:       ticket.ClusterID,
			ResourceType:    models.ResourceType(rule.ResourceType),
			ResourceName:    rule.ResourceName,
			ResourcePattern: models.PatternType(rule.ResourcePattern),
			Principal:       principal,
			Operation:       models.OperationType(rule.Operation),
			PermissionType:  models.PermissionTypeAllow,
		})
		if err != nil {
			return "", "", fmt.Errorf("failed to create acl %s %s %s: %w",
				rule.ResourceType, rule.ResourceName, rule.Operation, err)
		}
		result.addCreated(fmt.Sprintf("ACL: %s %s [%s] %s -> %s",
			strings.ToUpper(rule.ResourceType), rule.ResourceName, rule.ResourcePattern, rule.Operation, principal))
	}

	return result.toJSON(), encryptedCredential, nil
}

// generatePassword 生成随机密码（排除易混淆字符）
func generatePassword(length int) (string, error) {
	var sb strings.Builder
	max := big.NewInt(int64(len(passwordCharset)))
	for i := 0; i < length; i++ {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		sb.WriteByte(passwordCharset[idx.Int64()])
	}
	return sb.String(), nil
}

// logExecuteFailure 记录执行失败日志
func logExecuteFailure(ticket *models.Ticket, err error) {
	logger.Error("ticket executor: execute failed",
		"ticket_no", ticket.TicketNo, "type", ticket.Type, "error", err)
}
