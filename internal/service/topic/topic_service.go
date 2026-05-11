package topic

import (
	"context"
	"errors"
	"fmt"
	"log"

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
	Data  []*models.Topic `json:"data"`
	Total int64           `json:"total"`
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

	// 从数据库删除 Topic
	topic, err := s.topicRepo.FindByName(ctx, clusterID, topicName)
	if err != nil {
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

	return nil
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

	return &ListTopicsResponse{
		Data:  topics,
		Total: total,
	}, nil
}

// SyncTopics 同步集群的所有 Topic 数据
func (s *Service) SyncTopics(ctx context.Context, clusterID int64) error {
	log.Printf("[SyncTopics] Starting sync for cluster %d", clusterID)

	// 获取集群配置
	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}
	log.Printf("[SyncTopics] Cluster found: %s, bootstrap: %s", cluster.ClusterName, cluster.BootstrapServers)

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
	log.Printf("[SyncTopics] Kafka admin client created successfully")

	// 从 Kafka 获取所有 Topic 列表
	kafkaTopics, err := adminClient.ListTopics()
	if err != nil {
		return fmt.Errorf("failed to list topics from kafka: %w", err)
	}
	log.Printf("[SyncTopics] Found %d topics from Kafka: %v", len(kafkaTopics), getTopicNames(kafkaTopics))

	// 从数据库获取当前 Topic 列表
	dbTopics, err := s.topicRepo.ListByCluster(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("failed to list topics from database: %w", err)
	}
	log.Printf("[SyncTopics] Found %d topics in database", len(dbTopics))

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
				log.Printf("[SyncTopics] Failed to update topic %s: %v", topicName, err)
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
				log.Printf("[SyncTopics] Failed to create topic %s: %v", topicName, err)
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
				log.Printf("[SyncTopics] Failed to delete topic %s: %v", topicName, err)
				continue
			}
			deleteCount++
		}
	}

	log.Printf("[SyncTopics] Sync completed for cluster %d: new=%d, updated=%d, deleted=%d", clusterID, newCount, updateCount, deleteCount)
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
