package monitor

import (
	"context"
	"fmt"
	"log"

	"kafka-management-platform/internal/repository"
	"kafka-management-platform/pkg/encryption"
	"kafka-management-platform/pkg/kafka"

	"github.com/IBM/sarama"
)

// ConsumerGroupInfo 消费者组信息
type ConsumerGroupInfo struct {
	GroupID  string         `json:"group_id"`
	State    string         `json:"state"`
	Members  int            `json:"members"`
	TotalLag int64          `json:"total_lag"`
	Topics   []TopicLagInfo `json:"topics"`
}

// TopicLagInfo Topic 级别的 Lag 信息
type TopicLagInfo struct {
	Topic         string             `json:"topic"`
	Partitions    []PartitionLagInfo `json:"partitions"`
	CurrentOffset int64              `json:"current_offset"`
	EndOffset     int64              `json:"end_offset"`
	Lag           int64              `json:"lag"`
}

// PartitionLagInfo 分区级别的 Lag 信息
type PartitionLagInfo struct {
	Partition     int32 `json:"partition"`
	CurrentOffset int64 `json:"current_offset"`
	EndOffset     int64 `json:"end_offset"`
	Lag           int64 `json:"lag"`
}

// TopicOffsetInfo Topic Offset 信息
type TopicOffsetInfo struct {
	Topic       string            `json:"topic"`
	Partitions  int32             `json:"partitions"`
	Replication int               `json:"replication"`
	Offsets     []PartitionOffset `json:"offsets"`
}

// PartitionOffset 分区 Offset
type PartitionOffset struct {
	Partition int32 `json:"partition"`
	Offset    int64 `json:"offset"`
}

// ClusterMetadataInfo 集群元数据信息
type ClusterMetadataInfo struct {
	Brokers    []BrokerInfo `json:"brokers"`
	Controller int32        `json:"controller"`
	TopicCount int          `json:"topic_count"`
}

// BrokerInfo Broker 信息
type BrokerInfo struct {
	ID   int32  `json:"id"`
	Host string `json:"host"`
	Port int32  `json:"port"`
}

// KafkaExporterService 内置 Kafka Exporter 服务
type KafkaExporterService struct {
	clusterRepo     repository.ClusterRepository
	encryptionSvc   *encryption.Service
	kerberosBaseDir string
}

// NewKafkaExporterService 创建内置 Kafka Exporter 服务
func NewKafkaExporterService(
	clusterRepo repository.ClusterRepository,
	encryptionSvc *encryption.Service,
	kerberosBaseDir string,
) *KafkaExporterService {
	return &KafkaExporterService{
		clusterRepo:     clusterRepo,
		encryptionSvc:   encryptionSvc,
		kerberosBaseDir: kerberosBaseDir,
	}
}

// getAdminClient 获取 AdminClient
func (s *KafkaExporterService) getAdminClient(ctx context.Context, clusterID int64) (*kafka.AdminClient, error) {
	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	var authConfigJSON string
	if cluster.AuthConfig != "" {
		decrypted, err := s.encryptionSvc.DecryptString(cluster.AuthConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt auth config: %w", err)
		}
		authConfigJSON = decrypted
	}

	return kafka.NewAdminClientWithKerberos(cluster, authConfigJSON, s.kerberosBaseDir)
}

// GetClusterMetadata 获取集群元数据
func (s *KafkaExporterService) GetClusterMetadata(ctx context.Context, clusterID int64) (*ClusterMetadataInfo, error) {
	adminClient, err := s.getAdminClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	defer adminClient.Close()

	brokers, controllerID, err := adminClient.DescribeCluster()
	if err != nil {
		return nil, fmt.Errorf("failed to describe cluster: %w", err)
	}

	// 获取 Topic 列表
	topics, err := adminClient.ListTopics()
	if err != nil {
		return nil, fmt.Errorf("failed to list topics: %w", err)
	}

	var brokerInfos []BrokerInfo
	for _, b := range brokers {
		brokerInfos = append(brokerInfos, BrokerInfo{
			ID:   b.ID(),
			Host: b.Addr(),
			Port: 0, // sarama 不直接返回端口
		})
	}

	return &ClusterMetadataInfo{
		Brokers:    brokerInfos,
		Controller: controllerID,
		TopicCount: len(topics),
	}, nil
}

// ListConsumerGroups 列出所有消费者组
func (s *KafkaExporterService) ListConsumerGroups(ctx context.Context, clusterID int64) ([]string, error) {
	adminClient, err := s.getAdminClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	defer adminClient.Close()

	groups, err := adminClient.ListConsumerGroups()
	if err != nil {
		return nil, fmt.Errorf("failed to list consumer groups: %w", err)
	}

	var groupIDs []string
	for id := range groups {
		groupIDs = append(groupIDs, id)
	}
	return groupIDs, nil
}

// GetConsumerGroupInfo 获取消费者组详情
func (s *KafkaExporterService) GetConsumerGroupInfo(ctx context.Context, clusterID int64, groupID string) (*ConsumerGroupInfo, error) {
	adminClient, err := s.getAdminClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	defer adminClient.Close()

	// 1. 描述消费者组
	descs, err := adminClient.DescribeConsumerGroups([]string{groupID})
	if err != nil {
		return nil, fmt.Errorf("failed to describe consumer group: %w", err)
	}

	if len(descs) == 0 {
		return nil, fmt.Errorf("consumer group not found: %s", groupID)
	}

	desc := descs[0]
	if desc.ErrorCode != 0 {
		return nil, fmt.Errorf("error describing consumer group: code %d", desc.ErrorCode)
	}

	// 2. 获取消费者组 Offset
	offsetResp, err := adminClient.ListConsumerGroupOffsets(groupID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list consumer group offsets: %w", err)
	}

	// 3. 计算每个 Topic 的 Lag
	topicLags := make(map[string]*TopicLagInfo)
	var totalLag int64

	for topic, partitions := range offsetResp.Blocks {
		topicLag := &TopicLagInfo{
			Topic: topic,
		}

		for partition, block := range partitions {
			// 如果 Offset 是 -1，表示没有消费过
			if block.Offset >= 0 {
				partitionLag := PartitionLagInfo{
					Partition:     partition,
					CurrentOffset: block.Offset,
					EndOffset:     0,
					Lag:           0,
				}
				topicLag.Partitions = append(topicLag.Partitions, partitionLag)
				topicLag.CurrentOffset += block.Offset
			}
		}

		if len(topicLag.Partitions) > 0 {
			topicLags[topic] = topicLag
		}
	}

	// 转换为切片
	var topics []TopicLagInfo
	for _, tl := range topicLags {
		topics = append(topics, *tl)
	}

	return &ConsumerGroupInfo{
		GroupID:  groupID,
		State:    desc.State,
		Members:  len(desc.Members),
		TotalLag: totalLag,
		Topics:   topics,
	}, nil
}

// GetAllConsumerGroupLags 获取所有消费者组的 Lag（内置 Kafka Exporter 核心功能）
func (s *KafkaExporterService) GetAllConsumerGroupLags(ctx context.Context, clusterID int64) ([]*ConsumerGroupInfo, error) {
	log.Printf("[KafkaExporter] Getting all consumer group lags for cluster %d", clusterID)

	adminClient, err := s.getAdminClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	defer adminClient.Close()

	// 1. 列出所有消费者组
	groups, err := adminClient.ListConsumerGroups()
	if err != nil {
		return nil, fmt.Errorf("failed to list consumer groups: %w", err)
	}

	log.Printf("[KafkaExporter] Found %d consumer groups", len(groups))

	// 2. 批量描述消费者组
	groupIDs := make([]string, 0, len(groups))
	for id := range groups {
		groupIDs = append(groupIDs, id)
	}

	if len(groupIDs) == 0 {
		return []*ConsumerGroupInfo{}, nil
	}

	descs, err := adminClient.DescribeConsumerGroups(groupIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to describe consumer groups: %w", err)
	}

	// 3. 获取每个消费者组的 Offset 和计算 Lag
	var result []*ConsumerGroupInfo
	for _, desc := range descs {
		if desc.ErrorCode != 0 {
			log.Printf("[KafkaExporter] Error describing group %s: code %d", desc.GroupId, desc.ErrorCode)
			continue
		}

		info := &ConsumerGroupInfo{
			GroupID: desc.GroupId,
			State:   desc.State,
			Members: len(desc.Members),
		}

		// 获取该消费者组的 Offset
		offsetResp, err := adminClient.ListConsumerGroupOffsets(desc.GroupId, nil)
		if err != nil {
			log.Printf("[KafkaExporter] Error getting offsets for group %s: %v", desc.GroupId, err)
			result = append(result, info)
			continue
		}

		// 计算每个 Topic 的 Lag
		topicLags := make(map[string]*TopicLagInfo)
		var totalLag int64

		for topic, partitions := range offsetResp.Blocks {
			topicLag := &TopicLagInfo{
				Topic: topic,
			}

			for partition, block := range partitions {
				if block.Offset < 0 {
					continue // 没有消费过
				}

				partitionLag := PartitionLagInfo{
					Partition:     partition,
					CurrentOffset: block.Offset,
					EndOffset:     0, // TODO: 需要查询 Topic EndOffset
					Lag:           0,
				}

				topicLag.Partitions = append(topicLag.Partitions, partitionLag)
				topicLag.CurrentOffset += block.Offset
			}

			if len(topicLag.Partitions) > 0 {
				topicLags[topic] = topicLag
			}
		}

		// 转换为切片
		for _, tl := range topicLags {
			info.Topics = append(info.Topics, *tl)
		}
		info.TotalLag = totalLag

		result = append(result, info)
	}

	log.Printf("[KafkaExporter] Processed %d consumer groups", len(result))
	return result, nil
}

// GetTopicOffsets 获取 Topic 的 Offset 信息
func (s *KafkaExporterService) GetTopicOffsets(ctx context.Context, clusterID int64, topicName string) (*TopicOffsetInfo, error) {
	adminClient, err := s.getAdminClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	defer adminClient.Close()

	// 获取 Topic 详情
	topics, err := adminClient.ListTopics()
	if err != nil {
		return nil, fmt.Errorf("failed to list topics: %w", err)
	}

	detail, ok := topics[topicName]
	if !ok {
		return nil, fmt.Errorf("topic not found: %s", topicName)
	}

	info := &TopicOffsetInfo{
		Topic:       topicName,
		Partitions:  detail.NumPartitions,
		Replication: int(detail.ReplicationFactor),
	}

	// TODO: 获取每个分区的 Offset
	// 需要使用 sarama.SyncProducer 或其他方式获取 high watermark

	return info, nil
}

// GetTopicPartitionCount 获取 Topic 分区数
func (s *KafkaExporterService) GetTopicPartitionCount(ctx context.Context, clusterID int64) (map[string]int32, error) {
	adminClient, err := s.getAdminClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	defer adminClient.Close()

	topics, err := adminClient.ListTopics()
	if err != nil {
		return nil, fmt.Errorf("failed to list topics: %w", err)
	}

	result := make(map[string]int32)
	for name, detail := range topics {
		result[name] = detail.NumPartitions
	}

	return result, nil
}

// 内部使用的 sarama 类型别名（避免导出）
type _ = sarama.ConsumerGroup // 确保导入 sarama
