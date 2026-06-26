package topic

import (
	"context"
	"errors"
	"fmt"

	"kafka-management-platform/internal/logger"
	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/repository"
	"kafka-management-platform/pkg/encryption"
	"kafka-management-platform/pkg/kafka"

	"github.com/IBM/sarama"
)

var (
	// ErrTopicAlreadyExists Topic 已存在
	ErrTopicAlreadyExists = errors.New("topic already exists")
	// ErrTopicNotFound Topic 不存在
	ErrTopicNotFound = errors.New("topic not found")
	// ErrInvalidTopicName 无效的 Topic 名称
	ErrInvalidTopicName = errors.New("invalid topic name")
	// ErrInvalidPartitions 无效的分区数
	ErrInvalidPartitions = errors.New("invalid partitions")
	// ErrInvalidReplicationFactor 无效的副本数
	ErrInvalidReplicationFactor = errors.New("invalid replication factor")
	// ErrFeatureNotImplemented 功能未实现
	ErrFeatureNotImplemented = errors.New("feature not implemented")
)

// Service Topic 管理服务
type Service struct {
	topicRepo       repository.TopicRepository
	clusterRepo     repository.ClusterRepository
	encryptionSvc   *encryption.Service
	kerberosBaseDir string
}

// NewService 创建 Topic 管理服务实例
func NewService(
	topicRepo repository.TopicRepository,
	clusterRepo repository.ClusterRepository,
	encryptionSvc *encryption.Service,
	kerberosBaseDir string,
) *Service {
	return &Service{
		topicRepo:       topicRepo,
		clusterRepo:     clusterRepo,
		encryptionSvc:   encryptionSvc,
		kerberosBaseDir: kerberosBaseDir,
	}
}

// CreateTopicRequest 创建 Topic 请求
type CreateTopicRequest struct {
	ClusterID         int64             `json:"cluster_id" binding:"required"`
	TopicName         string            `json:"topic_name" binding:"required"`
	Description       string            `json:"description"`
	Partitions        int32             `json:"partitions" binding:"required,min=1"`
	ReplicationFactor int16             `json:"replication_factor" binding:"required,min=1"`
	Config            map[string]string `json:"config"`
}

// UpdateTopicConfigRequest 更新 Topic 配置请求
type UpdateTopicConfigRequest struct {
	ClusterID int64             `json:"cluster_id" binding:"required"`
	TopicName string            `json:"topic_name" binding:"required"`
	Config    map[string]string `json:"config" binding:"required"`
}

// ListTopicsRequest 列出 Topic 请求
type ListTopicsRequest struct {
	ClusterID     int64    `json:"cluster_id"`
	Search        string   `json:"search"`
	Offset        int      `json:"offset"`
	Limit         int      `json:"limit"`
	AllowedTopics []string `json:"-"` // 普通用户有权限的 Topic 列表，为空表示无限制
}

// ListTopicsResponse 列出 Topic 响应
type ListTopicsResponse struct {
	Data             []*models.Topic `json:"data"`
	Total            int64           `json:"total"`
	TotalPartitions  int64           `json:"total_partitions"`
	TotalReplicas    int64           `json:"total_replicas"`
}

// CreateTopic 创建 Topic
func (s *Service) CreateTopic(ctx context.Context, req *CreateTopicRequest) error {
	// 验证请求参数
	if err := s.validateCreateTopicRequest(req); err != nil {
		return err
	}

	// 检查 Topic 是否已存在
	exists, err := s.topicRepo.Exists(ctx, req.ClusterID, req.TopicName)
	if err != nil {
		return fmt.Errorf("failed to check topic existence: %w", err)
	}
	if exists {
		return ErrTopicAlreadyExists
	}

	// 获取集群配置
	cluster, err := s.clusterRepo.FindByID(ctx, req.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	// 解密认证配置
	var authConfigJSON string
	if cluster.AuthConfig != "" {
		decrypted, err := s.encryptionSvc.DecryptString(cluster.AuthConfig)
		if err != nil {
			return fmt.Errorf("failed to decrypt auth config: %w", err)
		}
		authConfigJSON = decrypted
	}

	// 创建 Kafka Admin 客户端（支持 Kerberos）
	adminClient, err := kafka.NewAdminClientWithKerberos(cluster, authConfigJSON, s.kerberosBaseDir)
	if err != nil {
		return fmt.Errorf("failed to create kafka admin client: %w", err)
	}
	defer adminClient.Close()

	// 调用 Kafka API 创建 Topic
	// 转换 Config 类型：map[string]string -> map[string]*string
	configEntries := make(map[string]*string)
	for k, v := range req.Config {
		value := v
		configEntries[k] = &value
	}

	topicDetail := &sarama.TopicDetail{
		NumPartitions:     req.Partitions,
		ReplicationFactor: req.ReplicationFactor,
		ConfigEntries:     convertConfigEntries(req.Config),
	}

	if err := adminClient.CreateTopic(req.TopicName, topicDetail, false); err != nil {
		return fmt.Errorf("failed to create topic in kafka: %w", err)
	}

	// 保存 Topic 元数据到数据库
	topic := &models.Topic{
		ClusterID:         req.ClusterID,
		TopicName:         req.TopicName,
		Description:       req.Description,
		Partitions:        req.Partitions,
		ReplicationFactor: req.ReplicationFactor,
		SyncStatus:        "synced",
	}

	if err := s.topicRepo.Create(ctx, topic); err != nil {
		return fmt.Errorf("failed to save topic to database: %w", err)
	}

	return nil
}

// DeleteTopic 删除 Topic
func (s *Service) DeleteTopic(ctx context.Context, clusterID int64, topicName string) error {
	// 获取集群配置
	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	// 解密认证配置
	var authConfigJSON string
	if cluster.AuthConfig != "" {
		decrypted, err := s.encryptionSvc.DecryptString(cluster.AuthConfig)
		if err != nil {
			return fmt.Errorf("failed to decrypt auth config: %w", err)
		}
		authConfigJSON = decrypted
	}

	// 创建 Kafka Admin 客户端（支持 Kerberos）
	adminClient, err := kafka.NewAdminClientWithKerberos(cluster, authConfigJSON, s.kerberosBaseDir)
	if err != nil {
		return fmt.Errorf("failed to create kafka admin client: %w", err)
	}
	defer adminClient.Close()

	// 从 Kafka 删除 Topic
	if err := adminClient.DeleteTopic(topicName); err != nil {
		return fmt.Errorf("failed to delete topic from kafka: %w", err)
	}

	// 从数据库删除 Topic（幂等：如果已被 SyncWorker 清理则视为成功）
	topic, err := s.topicRepo.FindByName(ctx, clusterID, topicName)
	if err != nil {
		if errors.Is(err, repository.ErrTopicNotFound) {
			return nil
		}
		return fmt.Errorf("failed to find topic: %w", err)
	}
	if err := s.topicRepo.Delete(ctx, topic.TopicID); err != nil {
		return fmt.Errorf("failed to delete topic from database: %w", err)
	}

	return nil
}

// UpdateTopicConfig 更新 Topic 配置
func (s *Service) UpdateTopicConfig(ctx context.Context, req *UpdateTopicConfigRequest) error {
	// 获取集群配置
	cluster, err := s.clusterRepo.FindByID(ctx, req.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	// 解密认证配置
	var authConfigJSON string
	if cluster.AuthConfig != "" {
		decrypted, err := s.encryptionSvc.DecryptString(cluster.AuthConfig)
		if err != nil {
			return fmt.Errorf("failed to decrypt auth config: %w", err)
		}
		authConfigJSON = decrypted
	}

	// 创建 Kafka Admin 客户端（支持 Kerberos）
	adminClient, err := kafka.NewAdminClientWithKerberos(cluster, authConfigJSON, s.kerberosBaseDir)
	if err != nil {
		return fmt.Errorf("failed to create kafka admin client: %w", err)
	}
	defer adminClient.Close()

	// TODO: 实现 Topic 配置更新逻辑
	// Sarama 的 AlterConfig API 需要额外实现
	// 当前返回明确错误，避免前端误以为操作成功
	return ErrFeatureNotImplemented
}

// TopicConfigEntry Topic 配置项
type TopicConfigEntry struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Source    string `json:"source"`
	ReadOnly  bool   `json:"read_only"`
	IsDefault bool   `json:"is_default"`
}

// GetTopicConfig 获取 Topic 配置
func (s *Service) GetTopicConfig(ctx context.Context, clusterID int64, topicName string) ([]TopicConfigEntry, error) {
	cluster, authConfigJSON, err := s.getClusterWithAuth(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	adminClient, err := kafka.NewAdminClientWithKerberos(cluster, authConfigJSON, s.kerberosBaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka admin client: %w", err)
	}
	defer adminClient.Close()

	entries, err := adminClient.DescribeTopicConfig(topicName)
	if err != nil {
		return nil, fmt.Errorf("failed to describe topic config: %w", err)
	}

	result := make([]TopicConfigEntry, 0, len(entries))
	for _, e := range entries {
		result = append(result, TopicConfigEntry{
			Name:      e.Name,
			Value:     e.Value,
			Source:    e.Source.String(),
			ReadOnly:  e.ReadOnly,
			IsDefault: e.Default,
		})
	}
	return result, nil
}

// TopicConsumerGroupInfo Topic 消费者组信息
type TopicConsumerGroupInfo struct {
	GroupID     string `json:"group_id"`
	State       string `json:"state"`
	MemberCount int    `json:"member_count"`
}

// GetTopicConsumerGroups 获取 Topic 的消费组列表
func (s *Service) GetTopicConsumerGroups(ctx context.Context, clusterID int64, topicName string) ([]TopicConsumerGroupInfo, error) {
	cluster, authConfigJSON, err := s.getClusterWithAuth(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	adminClient, err := kafka.NewAdminClientWithKerberos(cluster, authConfigJSON, s.kerberosBaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka admin client: %w", err)
	}
	defer adminClient.Close()

	// 1. 获取所有消费组 (map[groupID]protocolType)
	groups, err := adminClient.ListConsumerGroups()
	if err != nil {
		return nil, fmt.Errorf("failed to list consumer groups: %w", err)
	}

	// 2. 过滤系统消费组，收集需要 Describe 的 groupID
	var result []TopicConsumerGroupInfo
	var groupIDs []string
	for groupID := range groups {
		if len(groupID) > 0 && groupID[0] == '_' {
			continue
		}
		groupIDs = append(groupIDs, groupID)
	}

	if len(groupIDs) == 0 {
		return result, nil
	}

	// 3. 批量 Describe（一次 RPC 调用代替 N 次）
	descriptions, err := adminClient.DescribeConsumerGroups(groupIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to describe consumer groups: %w", err)
	}

	// 4. 筛选订阅了目标 Topic 的消费组
	for _, desc := range descriptions {
		memberTopicSet := make(map[string]bool)
		for _, member := range desc.Members {
			// 从 MemberAssignment 中解析订阅的 Topic
			assignment, _ := member.GetMemberAssignment()
			if assignment != nil {
				for topic := range assignment.Topics {
					memberTopicSet[topic] = true
				}
			}
			// 兼容：也从 MemberMetadata 中解析
			metadata, _ := member.GetMemberMetadata()
			if metadata != nil && len(metadata.Topics) > 0 {
				for _, t := range metadata.Topics {
					memberTopicSet[t] = true
				}
			}
		}

		if !memberTopicSet[topicName] {
			continue
		}

		result = append(result, TopicConsumerGroupInfo{
			GroupID:     desc.GroupId,
			State:       string(desc.State),
			MemberCount: len(desc.Members),
		})
	}

	return result, nil
}

// getClusterWithAuth 获取集群配置并解密认证信息
func (s *Service) getClusterWithAuth(ctx context.Context, clusterID int64) (*models.Cluster, string, error) {
	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get cluster: %w", err)
	}

	var authConfigJSON string
	if cluster.AuthConfig != "" {
		decrypted, err := s.encryptionSvc.DecryptString(cluster.AuthConfig)
		if err != nil {
			return nil, "", fmt.Errorf("failed to decrypt auth config: %w", err)
		}
		authConfigJSON = decrypted
	}

	return cluster, authConfigJSON, nil
}

// GetTopic 获取 Topic 详情
func (s *Service) GetTopic(ctx context.Context, clusterID int64, topicName string) (*models.Topic, error) {
	topic, err := s.topicRepo.FindByName(ctx, clusterID, topicName)
	if err != nil {
		return nil, err
	}
	return topic, nil
}

// ListTopics 列出 Topic
func (s *Service) ListTopics(ctx context.Context, req *ListTopicsRequest) (*ListTopicsResponse, error) {
	var topics []*models.Topic
	var total int64
	var err error

	// 如果有 Topic 权限限制（普通用户），使用过滤查询
	if len(req.AllowedTopics) > 0 {
		topics, total, err = s.topicRepo.ListByNames(ctx, req.ClusterID, req.AllowedTopics, req.Offset, req.Limit)
	} else if req.Search != "" {
		// 根据是否有搜索条件选择合适的查询方法
		topics, total, err = s.topicRepo.Search(ctx, req.ClusterID, req.Search, req.Offset, req.Limit)
	} else {
		topics, total, err = s.topicRepo.List(ctx, req.ClusterID, req.Offset, req.Limit)
	}

	if err != nil {
		return nil, err
	}

	// 获取集群级别的全量统计
	totalPartitions, totalReplicas, err := s.topicRepo.GetClusterTopicStats(ctx, req.ClusterID)
	if err != nil {
		return nil, err
	}

	return &ListTopicsResponse{
		Data:            topics,
		Total:           total,
		TotalPartitions: totalPartitions,
		TotalReplicas:   totalReplicas,
	}, nil
}

// SyncTopics 同步集群的所有 Topic 数据
func (s *Service) SyncTopics(ctx context.Context, clusterID int64) error {
	logger.Info("SyncTopics: starting sync", "cluster_id", clusterID)

	// 获取集群配置
	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}
	logger.Info("SyncTopics: cluster found", "name", cluster.ClusterName, "bootstrap", cluster.BootstrapServers)

	// 解密认证配置
	var authConfigJSON string
	if cluster.AuthConfig != "" {
		decrypted, err := s.encryptionSvc.DecryptString(cluster.AuthConfig)
		if err != nil {
			return fmt.Errorf("failed to decrypt auth config: %w", err)
		}
		authConfigJSON = decrypted
	}

	// 创建 Kafka Admin 客户端（支持 Kerberos）
	adminClient, err := kafka.NewAdminClientWithKerberos(cluster, authConfigJSON, s.kerberosBaseDir)
	if err != nil {
		return fmt.Errorf("failed to create kafka admin client: %w", err)
	}
	defer adminClient.Close()
	logger.Info("SyncTopics: kafka admin client created")

	// 从 Kafka 获取所有 Topic 列表
	kafkaTopics, err := adminClient.ListTopics()
	if err != nil {
		return fmt.Errorf("failed to list topics from kafka: %w", err)
	}
	logger.Info("SyncTopics: found topics from kafka", "count", len(kafkaTopics))

	// 从数据库获取当前 Topic 列表
	dbTopics, err := s.topicRepo.ListByCluster(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("failed to list topics from database: %w", err)
	}
	logger.Info("SyncTopics: found topics in db", "count", len(dbTopics))

	// 构建 Topic 名称映射
	kafkaTopicMap := make(map[string]sarama.TopicDetail)
	for name, detail := range kafkaTopics {
		kafkaTopicMap[name] = detail
	}

	dbTopicMap := make(map[string]*models.Topic)
	for _, topic := range dbTopics {
		dbTopicMap[topic.TopicName] = topic
	}

	// 处理新增和更新的 Topic
	newCount := 0
	updateCount := 0
	for topicName, kafkaTopic := range kafkaTopicMap {
		if dbTopic, exists := dbTopicMap[topicName]; exists {
			// Topic 已存在，更新元数据
			dbTopic.Partitions = kafkaTopic.NumPartitions
			dbTopic.ReplicationFactor = kafkaTopic.ReplicationFactor
			dbTopic.SyncStatus = "synced"
			if err := s.topicRepo.Update(ctx, dbTopic); err != nil {
				logger.Error("SyncTopics: failed to update topic", "topic", topicName, "error", err)
				continue
			}
			updateCount++
		} else {
			// 新 Topic，插入数据库
			newTopic := &models.Topic{
				ClusterID:         clusterID,
				TopicName:         topicName,
				Partitions:        kafkaTopic.NumPartitions,
				ReplicationFactor: kafkaTopic.ReplicationFactor,
				SyncStatus:        "synced",
			}
			if err := s.topicRepo.Create(ctx, newTopic); err != nil {
				logger.Error("SyncTopics: failed to create topic", "topic", topicName, "error", err)
				continue
			}
			newCount++
		}
	}

	// 处理已删除的 Topic
	deleteCount := 0
	for topicName, dbTopic := range dbTopicMap {
		if _, exists := kafkaTopicMap[topicName]; !exists {
			// Topic 在 Kafka 中不存在，从数据库删除
			if err := s.topicRepo.Delete(ctx, dbTopic.TopicID); err != nil {
				logger.Error("SyncTopics: failed to delete topic", "topic", topicName, "error", err)
				continue
			}
			deleteCount++
		}
	}

	logger.Info("SyncTopics: completed", "cluster_id", clusterID, "new", newCount, "updated", updateCount, "deleted", deleteCount)
	return nil
}

// UpdateTopicDescriptionRequest 更新 Topic 描述请求
type UpdateTopicDescriptionRequest struct {
	Description string `json:"description"`
}

// UpdateTopicDescription 更新 Topic 描述
func (s *Service) UpdateTopicDescription(ctx context.Context, clusterID int64, topicName string, req *UpdateTopicDescriptionRequest) error {
	topic, err := s.topicRepo.FindByName(ctx, clusterID, topicName)
	if err != nil {
		return fmt.Errorf("failed to find topic: %w", err)
	}
	if topic == nil {
		return ErrTopicNotFound
	}

	topic.Description = req.Description
	if err := s.topicRepo.Update(ctx, topic); err != nil {
		return fmt.Errorf("failed to update topic description: %w", err)
	}

	return nil
}

// validateCreateTopicRequest 验证创建 Topic 请求
func (s *Service) validateCreateTopicRequest(req *CreateTopicRequest) error {
	if req.ClusterID <= 0 {
		return fmt.Errorf("invalid cluster_id")
	}
	if req.TopicName == "" {
		return ErrInvalidTopicName
	}
	if req.Partitions <= 0 {
		return ErrInvalidPartitions
	}
	if req.ReplicationFactor <= 0 {
		return ErrInvalidReplicationFactor
	}
	return nil
}

// convertConfigEntries 转换配置项格式
func convertConfigEntries(config map[string]string) map[string]*string {
	if config == nil {
		return nil
	}
	result := make(map[string]*string)
	for k, v := range config {
		// 需要复制值，避免循环变量问题
		value := v
		result[k] = &value
	}
	return result
}

// getTopicNames 获取 Topic 名称列表（用于日志）
func getTopicNames(topics map[string]sarama.TopicDetail) []string {
	names := make([]string, 0, len(topics))
	for name := range topics {
		names = append(names, name)
	}
	return names
}
