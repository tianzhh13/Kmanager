package monitor

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"kafka-management-platform/internal/logger"
	"kafka-management-platform/internal/repository"
	"kafka-management-platform/pkg/encryption"
	"kafka-management-platform/pkg/kafka"

	"github.com/IBM/sarama"
)

// ConsumerGroupInfo 消费者组信息
type ConsumerGroupInfo struct {
	GroupID  string         `json:"group_id"`
	State    string         `json:"state"`
	Members  int            `json:"member_count"`
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
	LagSeconds    int64 `json:"lag_seconds"` // Lag 时间（秒），-1 表示无法计算
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
	Rack string `json:"rack"` // Broker 机架信息
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
		// 解析 Addr() 获取 host 和 port
		// Addr() 返回格式为 "host:port"
		host, port := parseBrokerAddr(b.Addr())
		brokerInfos = append(brokerInfos, BrokerInfo{
			ID:   b.ID(),
			Host: host,
			Port: port,
			Rack: b.Rack(),
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

	// 3. 收集需要查询 EndOffset 的 Topic 分区
	topicPartitions := make(map[string][]int32)
	consumerOffsets := make(map[string]map[int32]int64) // topic -> partition -> offset

	for topic, partitions := range offsetResp.Blocks {
		consumerOffsets[topic] = make(map[int32]int64)
		for partition, block := range partitions {
			if block.Offset >= 0 {
				consumerOffsets[topic][partition] = block.Offset
				topicPartitions[topic] = append(topicPartitions[topic], partition)
			}
		}
	}

	// 4. 批量获取 LogEndOffset
	endOffsets, err := adminClient.GetTopicPartitionOffsets(topicPartitions)
	if err != nil {
		logger.Warn("Error getting topic end offsets", "error", err)
	}

	// 5. 计算每个 Topic 的 Lag
	topicLags := make(map[string]*TopicLagInfo)
	var totalLag int64

	for topic, partitions := range consumerOffsets {
		topicLag := &TopicLagInfo{
			Topic: topic,
		}

		for partition, currentOffset := range partitions {
			var endOffset int64
			if endOffsets[topic] != nil {
				endOffset = endOffsets[topic][partition]
			}

			lag := endOffset - currentOffset
			if lag < 0 {
				lag = 0
			}

			partitionLag := PartitionLagInfo{
				Partition:     partition,
				CurrentOffset: currentOffset,
				EndOffset:     endOffset,
				Lag:           lag,
			}

			topicLag.Partitions = append(topicLag.Partitions, partitionLag)
			topicLag.CurrentOffset += currentOffset
			topicLag.EndOffset += endOffset
			topicLag.Lag += lag
		}

		if len(topicLag.Partitions) > 0 {
			topicLags[topic] = topicLag
		}
	}

	// 转换为切片
	var topics []TopicLagInfo
	for _, tl := range topicLags {
		topics = append(topics, *tl)
		totalLag += tl.Lag
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
	logger.Info("Getting all consumer group lags", "cluster_id", clusterID)

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

	logger.Info("Found consumer groups", "count", len(groups))

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

	// 3. 收集所有需要查询 EndOffset 的 Topic 分区
	topicPartitions := make(map[string][]int32)
	groupOffsets := make(map[string]map[string]map[int32]int64) // group -> topic -> partition -> offset

	for _, desc := range descs {
		if desc.ErrorCode != 0 {
			continue
		}

		offsetResp, err := adminClient.ListConsumerGroupOffsets(desc.GroupId, nil)
		if err != nil {
			logger.Warn("Error getting offsets for group", "group", desc.GroupId, "error", err)
			continue
		}

		groupOffsets[desc.GroupId] = make(map[string]map[int32]int64)
		for topic, partitions := range offsetResp.Blocks {
			groupOffsets[desc.GroupId][topic] = make(map[int32]int64)
			for partition, block := range partitions {
				if block.Offset >= 0 {
					groupOffsets[desc.GroupId][topic][partition] = block.Offset
					topicPartitions[topic] = append(topicPartitions[topic], partition)
				}
			}
		}
	}

	// 4. 批量获取所有 Topic 分区的 LogEndOffset
	endOffsets, err := adminClient.GetTopicPartitionOffsets(topicPartitions)
	if err != nil {
		logger.Warn("Error getting topic end offsets", "error", err)
	}

	// 4.1 批量获取所有 Topic 分区的 LogStartOffset（用于判断消费 offset 是否已过期）
	oldestOffsets := make(map[string]map[int32]int64)
	if oldestOffsetsRaw, err := adminClient.GetTopicPartitionStartOffsets(topicPartitions); err == nil {
		oldestOffsets = oldestOffsetsRaw
	}

	// 5. 计算每个消费者组的 Lag
	var result []*ConsumerGroupInfo
	for _, desc := range descs {
		if desc.ErrorCode != 0 {
			logger.Warn("Error describing group", "group", desc.GroupId, "error_code", desc.ErrorCode)
			continue
		}

		info := &ConsumerGroupInfo{
			GroupID: desc.GroupId,
			State:   desc.State,
			Members: len(desc.Members),
		}

		offsets, ok := groupOffsets[desc.GroupId]
		if !ok {
			result = append(result, info)
			continue
		}

		// 计算每个 Topic 的 Lag
		topicLags := make(map[string]*TopicLagInfo)
		var totalLag int64

		for topic, partitions := range offsets {
			topicLag := &TopicLagInfo{
				Topic: topic,
			}

			for partition, currentOffset := range partitions {
				// 获取 EndOffset
				var endOffset int64
				if endOffsets[topic] != nil {
					endOffset = endOffsets[topic][partition]
				}

				// 计算 Lag
				lag := endOffset - currentOffset
				if lag < 0 {
					lag = 0
				}

				// 计算 LagSeconds（仅对有 Lag 的分区计算）
				lagSeconds := int64(-1) // -1 表示未计算或无法计算
				if lag > 0 && currentOffset >= 0 {
					// 检查 offset 是否在有效范围内
					oldest := int64(-1)
					if oldestOffsets[topic] != nil {
						oldest = oldestOffsets[topic][partition]
					}
					if oldest >= 0 && currentOffset < oldest {
						// offset 已过期（被日志清理删除），跳过计算
						lagSeconds = -1
					} else {
						ls, err := adminClient.CalculateConsumerGroupLagSeconds(topic, partition, currentOffset, endOffset)
						if err != nil {
							logger.Warn("Failed to calculate lag seconds", "topic", topic, "partition", partition, "error", err)
							lagSeconds = -1
						} else {
							lagSeconds = ls
						}
					}
				}

				partitionLag := PartitionLagInfo{
					Partition:     partition,
					CurrentOffset: currentOffset,
					EndOffset:     endOffset,
					Lag:           lag,
					LagSeconds:    lagSeconds,
				}

				topicLag.Partitions = append(topicLag.Partitions, partitionLag)
				topicLag.CurrentOffset += currentOffset
				topicLag.EndOffset += endOffset
				topicLag.Lag += lag
			}

			if len(topicLag.Partitions) > 0 {
				topicLags[topic] = topicLag
				totalLag += topicLag.Lag
			}
		}

		// 转换为切片
		for _, tl := range topicLags {
			info.Topics = append(info.Topics, *tl)
		}
		info.TotalLag = totalLag

		result = append(result, info)
	}

	logger.Info("Processed consumer groups", "count", len(result))
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

// GetTopicPartitionDetails 获取 Topic 分区详情（包括 Offset）
func (s *KafkaExporterService) GetTopicPartitionDetails(ctx context.Context, clusterID int64) ([]kafka.TopicPartitionInfo, error) {
	adminClient, err := s.getAdminClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	defer adminClient.Close()

	return adminClient.GetTopicPartitionDetails()
}

// GetTopicPartitionOffsets 获取 Topic 分区的 LogEndOffset
func (s *KafkaExporterService) GetTopicPartitionOffsets(ctx context.Context, clusterID int64, topicPartitions map[string][]int32) (map[string]map[int32]int64, error) {
	adminClient, err := s.getAdminClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	defer adminClient.Close()

	return adminClient.GetTopicPartitionOffsets(topicPartitions)
}

// GetTopicPartitionStartOffsets 获取 Topic 分区的 LogStartOffset（最旧偏移量）
func (s *KafkaExporterService) GetTopicPartitionStartOffsets(ctx context.Context, clusterID int64, topicPartitions map[string][]int32) (map[string]map[int32]int64, error) {
	adminClient, err := s.getAdminClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	defer adminClient.Close()

	return adminClient.GetTopicPartitionStartOffsets(topicPartitions)
}

// 内部使用的 sarama 类型别名（避免导出）
type _ = sarama.ConsumerGroup // 确保导入 sarama

// parseBrokerAddr 解析 Broker 地址，返回 host 和 port
// 输入格式: "host:port" 或 "host"
func parseBrokerAddr(addr string) (host string, port int32) {
	parts := strings.Split(addr, ":")
	if len(parts) == 2 {
		host = parts[0]
		if p, err := strconv.ParseInt(parts[1], 10, 32); err == nil {
			port = int32(p)
		}
	} else if len(parts) == 1 {
		host = parts[0]
		port = 9092 // 默认端口
	}
	return
}
