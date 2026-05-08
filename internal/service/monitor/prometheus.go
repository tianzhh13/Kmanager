package monitor

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"kafka-management-platform/internal/repository"
	"kafka-management-platform/pkg/encryption"
	"kafka-management-platform/pkg/kafka"
	"kafka-management-platform/pkg/victoriametrics"

	"github.com/gin-gonic/gin"
)

var (
	// ErrNoJMXExporterURL 集群未配置 JMX Exporter URL
	ErrNoJMXExporterURL = fmt.Errorf("cluster does not have JMX Exporter URL configured")
)

// TopicPartitionInfo Topic 分区信息（别名）
type TopicPartitionInfo = kafka.TopicPartitionInfo

// ClusterMetricsResponse 集群指标响应
type ClusterMetricsResponse struct {
	ClusterID int64 `json:"cluster_id"`

	// 来自 JMX Exporter
	BrokerMetrics *BrokerMetrics `json:"broker_metrics"`

	// 来自内置 Kafka Exporter
	ConsumerGroups []*ConsumerGroupInfo `json:"consumer_groups"`

	// 元数据
	Brokers     []BrokerInfo `json:"brokers"`
	BrokerCount int          `json:"broker_count"`
	TopicCount  int          `json:"topic_count"`

	// 监控状态
	JMXExporterAvailable   bool `json:"jmx_exporter_available"`
	KafkaExporterAvailable bool `json:"kafka_exporter_available"`
}

// Service 监控服务（整合 JMX Exporter + 内置 Kafka Exporter）
type Service struct {
	clusterRepo   repository.ClusterRepository
	encryptionSvc *encryption.Service
	jmxClients    map[int64]*JMXClient  // JMX Exporter 客户端缓存
	kafkaExporter *KafkaExporterService // 内置 Kafka Exporter
	vmClient      *victoriametrics.Client
}

// NewService 创建监控服务
func NewService(
	clusterRepo repository.ClusterRepository,
	encryptionSvc *encryption.Service,
	vmClient *victoriametrics.Client,
	kerberosBaseDir string,
) *Service {
	return &Service{
		clusterRepo:   clusterRepo,
		encryptionSvc: encryptionSvc,
		jmxClients:    make(map[int64]*JMXClient),
		kafkaExporter: NewKafkaExporterService(clusterRepo, encryptionSvc, kerberosBaseDir),
		vmClient:      vmClient,
	}
}

// getJMXClient 获取或创建 JMX 客户端
func (s *Service) getJMXClient(clusterID int64, jmxURL string) *JMXClient {
	if client, exists := s.jmxClients[clusterID]; exists {
		return client
	}

	client := NewJMXClient(jmxURL)
	s.jmxClients[clusterID] = client
	return client
}

// GetClusterMetrics 获取集群监控指标（整合 JMX + Kafka Exporter）
func (s *Service) GetClusterMetrics(ctx context.Context, clusterID int64) (*ClusterMetricsResponse, error) {
	log.Printf("[Monitor] Getting cluster metrics for cluster %d", clusterID)

	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	response := &ClusterMetricsResponse{
		ClusterID: clusterID,
	}

	// 1. 从 JMX Exporter 获取 Broker 指标（支持多个 Broker）
	if cluster.JMXExporterURLs != "" {
		urls := ParseJMXExporterURLs(cluster.JMXExporterURLs)
		if len(urls) > 0 {
			multiClient := NewMultiJMXClient(urls)
			brokerMetrics, err := multiClient.GetAggregatedMetrics(ctx)
			if err != nil {
				log.Printf("[Monitor] Failed to get JMX metrics: %v", err)
				response.JMXExporterAvailable = false
			} else {
				response.BrokerMetrics = brokerMetrics
				response.JMXExporterAvailable = true
			}
		}
	}

	// 2. 从内置 Kafka Exporter 获取消费者组 Lag
	consumerGroups, err := s.kafkaExporter.GetAllConsumerGroupLags(ctx, clusterID)
	if err != nil {
		log.Printf("[Monitor] Failed to get consumer group lags: %v", err)
		response.KafkaExporterAvailable = false
	} else {
		response.ConsumerGroups = consumerGroups
		response.KafkaExporterAvailable = true
	}

	// 3. 获取元数据
	metadata, err := s.kafkaExporter.GetClusterMetadata(ctx, clusterID)
	if err != nil {
		log.Printf("[Monitor] Failed to get cluster metadata: %v", err)
	} else {
		response.Brokers = metadata.Brokers
		response.BrokerCount = len(metadata.Brokers)
		response.TopicCount = metadata.TopicCount
	}

	return response, nil
}

// GetConsumerGroupLags 获取消费者组 Lag（内置 Kafka Exporter）
func (s *Service) GetConsumerGroupLags(ctx context.Context, clusterID int64) ([]*ConsumerGroupInfo, error) {
	return s.kafkaExporter.GetAllConsumerGroupLags(ctx, clusterID)
}

// GetConsumerGroupInfo 获取单个消费者组详情
func (s *Service) GetConsumerGroupInfo(ctx context.Context, clusterID int64, groupID string) (*ConsumerGroupInfo, error) {
	return s.kafkaExporter.GetConsumerGroupInfo(ctx, clusterID, groupID)
}

// GetTopicPartitionCount 获取 Topic 分区数
func (s *Service) GetTopicPartitionCount(ctx context.Context, clusterID int64) (map[string]int32, error) {
	return s.kafkaExporter.GetTopicPartitionCount(ctx, clusterID)
}

// GetTopicPartitionDetails 获取 Topic 分区详情
func (s *Service) GetTopicPartitionDetails(ctx context.Context, clusterID int64) ([]TopicPartitionInfo, error) {
	return s.kafkaExporter.GetTopicPartitionDetails(ctx, clusterID)
}

// GetTopicPartitionOffsets 获取 Topic 分区的 LogEndOffset
func (s *Service) GetTopicPartitionOffsets(ctx context.Context, clusterID int64, topicPartitions map[string][]int32) (map[string]map[int32]int64, error) {
	return s.kafkaExporter.GetTopicPartitionOffsets(ctx, clusterID, topicPartitions)
}

// GetTopicPartitionStartOffsets 获取 Topic 分区的 LogStartOffset（最旧偏移量）
func (s *Service) GetTopicPartitionStartOffsets(ctx context.Context, clusterID int64, topicPartitions map[string][]int32) (map[string]map[int32]int64, error) {
	return s.kafkaExporter.GetTopicPartitionStartOffsets(ctx, clusterID, topicPartitions)
}

// GetBrokerMetrics 从 JMX Exporter 获取 Broker 指标（集群聚合）
func (s *Service) GetBrokerMetrics(ctx context.Context, clusterID int64) (*BrokerMetrics, error) {
	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	if cluster.JMXExporterURLs == "" {
		return nil, ErrNoJMXExporterURL
	}

	urls := ParseJMXExporterURLs(cluster.JMXExporterURLs)
	if len(urls) == 0 {
		return nil, ErrNoJMXExporterURL
	}

	multiClient := NewMultiJMXClient(urls)
	return multiClient.GetAggregatedMetrics(ctx)
}

// TestJMXExporter 测试 JMX Exporter 连接
func (s *Service) TestJMXExporter(ctx context.Context, clusterID int64) error {
	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	if cluster.JMXExporterURLs == "" {
		return ErrNoJMXExporterURL
	}

	urls := ParseJMXExporterURLs(cluster.JMXExporterURLs)
	if len(urls) == 0 {
		return ErrNoJMXExporterURL
	}

	// 测试第一个 URL 的连接
	client := NewJMXClient(urls[0])
	return client.HealthCheck(ctx)
}

// QueryMetricsRange 从 VictoriaMetrics 查询历史指标
func (s *Service) QueryMetricsRange(ctx context.Context, query string, start, end time.Time, step string) ([]byte, error) {
	if s.vmClient == nil || !s.vmClient.IsEnabled() {
		return nil, fmt.Errorf("victoriametrics is not configured")
	}
	return s.vmClient.QueryRange(ctx, query, start, end, step)
}

// ============================================================
// HTTP Handlers
// ============================================================

// Handler HTTP 处理器
type Handler struct {
	svc *Service
}

// NewHandler 创建监控处理器
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GetClusterMetrics 处理获取集群指标请求
func (h *Handler) GetClusterMetrics(c *gin.Context) {
	clusterID, err := parseInt64Param(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid cluster id"})
		return
	}

	metrics, err := h.svc.GetClusterMetrics(c.Request.Context(), clusterID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, metrics)
}

// GetConsumerGroupLags 处理获取消费者组 Lag 请求
func (h *Handler) GetConsumerGroupLags(c *gin.Context) {
	clusterID, err := parseInt64Param(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid cluster id"})
		return
	}

	groups, err := h.svc.GetConsumerGroupLags(c.Request.Context(), clusterID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"data": groups,
	})
}

// GetConsumerGroupInfo 处理获取单个消费者组详情请求
func (h *Handler) GetConsumerGroupInfo(c *gin.Context) {
	clusterID, err := parseInt64Param(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid cluster id"})
		return
	}

	groupID := c.Query("group")
	if groupID == "" {
		c.JSON(400, gin.H{"error": "group parameter is required"})
		return
	}

	info, err := h.svc.GetConsumerGroupInfo(c.Request.Context(), clusterID, groupID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, info)
}

// GetBrokerMetrics 处理获取 Broker 指标请求
func (h *Handler) GetBrokerMetrics(c *gin.Context) {
	clusterID, err := parseInt64Param(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid cluster id"})
		return
	}

	metrics, err := h.svc.GetBrokerMetrics(c.Request.Context(), clusterID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, metrics)
}

// GetMetricsHistory 处理获取历史指标请求（代理 VictoriaMetrics 查询）
func (h *Handler) GetMetricsHistory(c *gin.Context) {
	// 获取查询参数
	query := c.Query("query")
	if query == "" {
		c.JSON(400, gin.H{"error": "query parameter is required"})
		return
	}

	var start, end time.Time
	var step string

	// 优先使用 start/end 参数
	startStr := c.Query("start")
	endStr := c.Query("end")

	if startStr != "" && endStr != "" {
		// 使用精确的 start/end 时间戳
		startUnix, err1 := strconv.ParseInt(startStr, 10, 64)
		endUnix, err2 := strconv.ParseInt(endStr, 10, 64)
		if err1 != nil || err2 != nil {
			c.JSON(400, gin.H{"error": "invalid start/end timestamp"})
			return
		}
		start = time.Unix(startUnix, 0)
		end = time.Unix(endUnix, 0)
	} else {
		// 兼容旧的 duration 参数
		durationStr := c.DefaultQuery("duration", "1h")
		var duration time.Duration
		switch durationStr {
		case "6h", "6hour":
			duration = 6 * time.Hour
		case "24h", "1d", "1day":
			duration = 24 * time.Hour
		case "7d", "7day", "1w":
			duration = 7 * 24 * time.Hour
		default:
			var err error
			duration, err = time.ParseDuration(durationStr)
			if err != nil {
				duration = time.Hour
			}
		}
		end = time.Now()
		start = end.Add(-duration)
	}

	// 步长参数
	step = c.DefaultQuery("step", "30s")

	// 查询 VictoriaMetrics
	result, err := h.svc.QueryMetricsRange(c.Request.Context(), query, start, end, step)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.Data(200, "application/json", result)
}

// parseInt64Param 解析 int64 参数
func parseInt64Param(param string) (int64, error) {
	var result int64
	_, err := fmt.Sscanf(param, "%d", &result)
	return result, err
}
