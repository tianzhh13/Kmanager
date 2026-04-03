package monitor

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"kafka-management-platform/internal/models"
	"kafka-management-platform/internal/repository"
	"kafka-management-platform/pkg/prometheus"

	"github.com/gin-gonic/gin"
)

var (
	// ErrNoPrometheusURL 集群未配置 Prometheus URL
	ErrNoPrometheusURL = fmt.Errorf("cluster does not have Prometheus URL configured")
	// ErrTimeRangeExceeded 时间范围超过最大限制
	ErrTimeRangeExceeded = fmt.Errorf("time range exceeds maximum of 30 days")
	// ErrInvalidTimeRange 无效的时间范围
	ErrInvalidTimeRange = fmt.Errorf("end time must be after start time")
)

// Service 监控服务
type Service struct {
	clusterRepo repository.ClusterRepository
	clients     map[int64]*prometheus.Client // 集群ID到Prometheus客户端的映射
}

// NewService 创建监控服务
func NewService(clusterRepo repository.ClusterRepository) *Service {
	return &Service{
		clusterRepo: clusterRepo,
		clients:     make(map[int64]*prometheus.Client),
	}
}

// getClient 获取或创建 Prometheus 客户端
func (s *Service) getClient(clusterID int64, prometheusURL string) (*prometheus.Client, error) {
	// 缓存客户端
	if client, exists := s.clients[clusterID]; exists {
		return client, nil
	}

	client := prometheus.NewClient(prometheusURL)
	s.clients[clusterID] = client
	return client, nil
}

// ValidateTimeRange 验证时间范围（最长30天）
func ValidateTimeRange(start, end time.Time) error {
	maxDuration := 30 * 24 * time.Hour
	if end.Sub(start) > maxDuration {
		return ErrTimeRangeExceeded
	}
	if end.Before(start) {
		return ErrInvalidTimeRange
	}
	return nil
}

// GetClusterMetrics 获取集群概览指标
// 需求: 6.2
func (s *Service) GetClusterMetrics(ctx context.Context, clusterID int64, start, end time.Time) (*models.ClusterMetrics, error) {
	// 验证时间范围
	if err := ValidateTimeRange(start, end); err != nil {
		return nil, err
	}

	// 获取集群配置
	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	if cluster.PrometheusURL == "" {
		return nil, ErrNoPrometheusURL
	}

	client, err := s.getClient(clusterID, cluster.PrometheusURL)
	if err != nil {
		return nil, err
	}

	metrics := &models.ClusterMetrics{
		ClusterID: clusterID,
		StartTime: start,
		EndTime:   end,
	}

	// 查询 Broker 数量
	brokerCount, err := s.querySingleValue(ctx, client, "count(kafka_broker_info)", end)
	if err == nil {
		metrics.BrokerCount = int(brokerCount)
	}

	// 查询 Topic 数量
	topicCount, err := s.querySingleValue(ctx, client, "count(kafka_topic_partition_count)", end)
	if err == nil {
		metrics.TopicCount = int(topicCount)
	}

	// 查询消息速率（每秒消息数）
	msgRate, err := s.querySingleValue(ctx, client, "sum(rate(kafka_server_brokertopicmetrics_messagesin_total[5m]))", end)
	if err == nil {
		metrics.MessageRate = msgRate
	}

	// 查询字节速率
	bytesInRate, err := s.querySingleValue(ctx, client, "sum(rate(kafka_server_brokertopicmetrics_bytesin_total[5m]))", end)
	if err == nil {
		metrics.BytesInRate = bytesInRate
	}

	bytesOutRate, err := s.querySingleValue(ctx, client, "sum(rate(kafka_server_brokertopicmetrics_bytesout_total[5m]))", end)
	if err == nil {
		metrics.BytesOutRate = bytesOutRate
	}

	return metrics, nil
}

// GetBrokerMetrics 获取 Broker 级别指标
// 需求: 6.3
func (s *Service) GetBrokerMetrics(ctx context.Context, clusterID int64, brokerHost string, start, end time.Time) (*models.BrokerMetrics, error) {
	if err := ValidateTimeRange(start, end); err != nil {
		return nil, err
	}

	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	if cluster.PrometheusURL == "" {
		return nil, ErrNoPrometheusURL
	}

	client, err := s.getClient(clusterID, cluster.PrometheusURL)
	if err != nil {
		return nil, err
	}

	metrics := &models.BrokerMetrics{
		ClusterID:  clusterID,
		BrokerHost: brokerHost,
		StartTime:  start,
		EndTime:    end,
	}

	// 查询 CPU 使用率
	cpuUsage, err := s.querySingleValue(ctx, client,
		fmt.Sprintf("avg(rate(process_cpu_seconds_total{instance=\"%s\"}[5m])) * 100", brokerHost), end)
	if err == nil {
		metrics.CPUUsage = cpuUsage
	}

	// 查询内存使用
	memUsage, err := s.querySingleValue(ctx, client,
		fmt.Sprintf("process_resident_memory_bytes{instance=\"%s\"}", brokerHost), end)
	if err == nil {
		metrics.MemoryUsage = memUsage
	}

	// 查询网络流入速率
	netIn, err := s.querySingleValue(ctx, client,
		fmt.Sprintf("sum(rate(kafka_network_socketserver_networkprocessor_avgidle_percent{instance=\"%s\"}[5m]))", brokerHost), end)
	if err == nil {
		metrics.NetworkInRate = netIn
	}

	// 查询网络流出速率
	netOut, err := s.querySingleValue(ctx, client,
		fmt.Sprintf("sum(rate(kafka_network_socketserver_networkprocessor_avgidle_percent{instance=\"%s\"}[5m]))", brokerHost), end)
	if err == nil {
		metrics.NetworkOutRate = netOut
	}

	return metrics, nil
}

// GetTopicMetrics 获取 Topic 级别指标
// 需求: 6.4
func (s *Service) GetTopicMetrics(ctx context.Context, clusterID int64, topicName string, start, end time.Time) (*models.TopicMetrics, error) {
	if err := ValidateTimeRange(start, end); err != nil {
		return nil, err
	}

	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	if cluster.PrometheusURL == "" {
		return nil, ErrNoPrometheusURL
	}

	client, err := s.getClient(clusterID, cluster.PrometheusURL)
	if err != nil {
		return nil, err
	}

	metrics := &models.TopicMetrics{
		ClusterID: clusterID,
		TopicName: topicName,
		StartTime: start,
		EndTime:   end,
	}

	// 查询消息流入速率
	msgRate, err := s.querySingleValue(ctx, client,
		fmt.Sprintf("sum(rate(kafka_server_brokertopicmetrics_messagesin_total{topic=\"%s\"}[5m]))", topicName), end)
	if err == nil {
		metrics.MessageRateIn = msgRate
	}

	// 查询字节流入速率
	bytesInRate, err := s.querySingleValue(ctx, client,
		fmt.Sprintf("sum(rate(kafka_server_brokertopicmetrics_bytesin_total{topic=\"%s\"}[5m]))", topicName), end)
	if err == nil {
		metrics.BytesRateIn = bytesInRate
	}

	// 查询字节流出速率
	bytesOutRate, err := s.querySingleValue(ctx, client,
		fmt.Sprintf("sum(rate(kafka_server_brokertopicmetrics_bytesout_total{topic=\"%s\"}[5m]))", topicName), end)
	if err == nil {
		metrics.BytesRateOut = bytesOutRate
	}

	// 查询分区数量
	partitions, err := s.querySingleValue(ctx, client,
		fmt.Sprintf("kafka_topic_partition_count{topic=\"%s\"}", topicName), end)
	if err == nil {
		metrics.PartitionCount = int(partitions)
	}

	return metrics, nil
}

// GetConsumerGroupMetrics 获取消费组指标
// 需求: 6.5
func (s *Service) GetConsumerGroupMetrics(ctx context.Context, clusterID int64, consumerGroup string, start, end time.Time) (*models.ConsumerGroupMetrics, error) {
	if err := ValidateTimeRange(start, end); err != nil {
		return nil, err
	}

	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	if cluster.PrometheusURL == "" {
		return nil, ErrNoPrometheusURL
	}

	client, err := s.getClient(clusterID, cluster.PrometheusURL)
	if err != nil {
		return nil, err
	}

	metrics := &models.ConsumerGroupMetrics{
		ClusterID:     clusterID,
		ConsumerGroup: consumerGroup,
		StartTime:     start,
		EndTime:       end,
	}

	// 查询消费延迟
	lag, err := s.querySingleValue(ctx, client,
		fmt.Sprintf("sum(kafka_consumergroup_group_topic_partition_lag{group=\"%s\"})", consumerGroup), end)
	if err == nil {
		metrics.Lag = lag
	}

	// 查询消费速率
	consumeRate, err := s.querySingleValue(ctx, client,
		fmt.Sprintf("sum(rate(kafka_consumergroup_group_topic_records_consumed_total{group=\"%s\"}[5m]))", consumerGroup), end)
	if err == nil {
		metrics.ConsumeRate = consumeRate
	}

	// 查询成员数量
	members, err := s.querySingleValue(ctx, client,
		fmt.Sprintf("kafka_consumergroup_group_member_count{group=\"%s\"}", consumerGroup), end)
	if err == nil {
		metrics.MemberCount = int(members)
	}

	return metrics, nil
}

// querySingleValue 从查询结果中提取单个值
func (s *Service) querySingleValue(ctx context.Context, client *prometheus.Client, query string, timestamp time.Time) (float64, error) {
	result, err := client.Query(ctx, query, timestamp)
	if err != nil {
		return 0, err
	}

	return prometheus.ParseValue(result)
}

// QueryPrometheus 执行自定义 PromQL 查询
// 需求: 6.1, 6.6, 6.7
func (s *Service) QueryPrometheus(ctx context.Context, clusterID int64, query string, start, end time.Time, step time.Duration) ([]prometheus.TimeSeriesPoint, error) {
	if err := ValidateTimeRange(start, end); err != nil {
		return nil, err
	}

	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	if cluster.PrometheusURL == "" {
		return nil, ErrNoPrometheusURL
	}

	client, err := s.getClient(clusterID, cluster.PrometheusURL)
	if err != nil {
		return nil, err
	}

	result, err := client.QueryRange(ctx, query, start, end, step)
	if err != nil {
		return nil, err
	}

	return prometheus.ParseValues(result)
}

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
	clusterID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid cluster id"})
		return
	}

	// 解析时间参数
	startStr := c.DefaultQuery("start", time.Now().Add(-1*time.Hour).Format(time.RFC3339))
	endStr := c.DefaultQuery("end", time.Now().Format(time.RFC3339))

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid start time format"})
		return
	}

	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid end time format"})
		return
	}

	metrics, err := h.svc.GetClusterMetrics(c.Request.Context(), clusterID, start, end)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, metrics)
}

// GetBrokerMetrics 处理获取 Broker 指标请求
func (h *Handler) GetBrokerMetrics(c *gin.Context) {
	clusterID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid cluster id"})
		return
	}

	brokerHost := c.Query("host")
	if brokerHost == "" {
		c.JSON(400, gin.H{"error": "broker host is required"})
		return
	}

	startStr := c.DefaultQuery("start", time.Now().Add(-1*time.Hour).Format(time.RFC3339))
	endStr := c.DefaultQuery("end", time.Now().Format(time.RFC3339))

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid start time format"})
		return
	}

	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid end time format"})
		return
	}

	metrics, err := h.svc.GetBrokerMetrics(c.Request.Context(), clusterID, brokerHost, start, end)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, metrics)
}

// GetTopicMetrics 处理获取 Topic 指标请求
func (h *Handler) GetTopicMetrics(c *gin.Context) {
	clusterID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid cluster id"})
		return
	}

	topicName := c.Param("topic")
	if topicName == "" {
		c.JSON(400, gin.H{"error": "topic name is required"})
		return
	}

	startStr := c.DefaultQuery("start", time.Now().Add(-1*time.Hour).Format(time.RFC3339))
	endStr := c.DefaultQuery("end", time.Now().Format(time.RFC3339))

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid start time format"})
		return
	}

	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid end time format"})
		return
	}

	metrics, err := h.svc.GetTopicMetrics(c.Request.Context(), clusterID, topicName, start, end)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, metrics)
}

// GetConsumerGroupMetrics 处理获取消费组指标请求
func (h *Handler) GetConsumerGroupMetrics(c *gin.Context) {
	clusterID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid cluster id"})
		return
	}

	group := c.Query("group")
	if group == "" {
		c.JSON(400, gin.H{"error": "consumer group is required"})
		return
	}

	startStr := c.DefaultQuery("start", time.Now().Add(-1*time.Hour).Format(time.RFC3339))
	endStr := c.DefaultQuery("end", time.Now().Format(time.RFC3339))

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid start time format"})
		return
	}

	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid end time format"})
		return
	}

	metrics, err := h.svc.GetConsumerGroupMetrics(c.Request.Context(), clusterID, group, start, end)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, metrics)
}

// QueryPrometheus 处理自定义 PromQL 查询请求
func (h *Handler) QueryPrometheus(c *gin.Context) {
	clusterID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid cluster id"})
		return
	}

	query := c.Query("query")
	if query == "" {
		c.JSON(400, gin.H{"error": "query is required"})
		return
	}

	startStr := c.DefaultQuery("start", time.Now().Add(-1*time.Hour).Format(time.RFC3339))
	endStr := c.DefaultQuery("end", time.Now().Format(time.RFC3339))
	stepStr := c.DefaultQuery("step", "1m")

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid start time format"})
		return
	}

	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid end time format"})
		return
	}

	step, err := time.ParseDuration(stepStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid step format"})
		return
	}

	points, err := h.svc.QueryPrometheus(c.Request.Context(), clusterID, query, start, end, step)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"query": query,
		"data":  points,
	})
}