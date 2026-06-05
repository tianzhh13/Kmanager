package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"kafka-management-platform/internal/cache"
	"kafka-management-platform/internal/logger"
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
	jmxClients    sync.Map              // clusterID -> *JMXClient（并发安全）
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
		jmxClients:    sync.Map{},
		kafkaExporter: NewKafkaExporterService(clusterRepo, encryptionSvc, kerberosBaseDir),
		vmClient:      vmClient,
	}
}

// getJMXClient 获取或创建 JMX 客户端
func (s *Service) getJMXClient(clusterID int64, jmxURL string) *JMXClient {
	if val, ok := s.jmxClients.Load(clusterID); ok {
		return val.(*JMXClient)
	}

	client := NewJMXClient(jmxURL)
	actual, _ := s.jmxClients.LoadOrStore(clusterID, client)
	return actual.(*JMXClient)
}

// GetClusterMetrics 获取集群监控指标（整合 JMX + Kafka Exporter）
func (s *Service) GetClusterMetrics(ctx context.Context, clusterID int64) (*ClusterMetricsResponse, error) {
	logger.Debug("Getting cluster metrics", "cluster_id", clusterID)

	cluster, err := s.clusterRepo.FindByID(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	response := &ClusterMetricsResponse{
		ClusterID: clusterID,
	}

	// 1. 从 JMX Exporter 获取 Broker 指标（支持多个 Broker）
	// 使用独立超时 context，避免 JMX 不通时拖住后续 AdminClient 调用
	if cluster.JMXExporterURLs != "" {
		urls := ParseJMXExporterURLs(cluster.JMXExporterURLs)
		if len(urls) > 0 {
			jmxCtx, jmxCancel := context.WithTimeout(ctx, 10*time.Second)
			multiClient := NewMultiJMXClient(urls)
			brokerMetrics, err := multiClient.GetAggregatedMetrics(jmxCtx)
			jmxCancel()
			if err != nil {
				logger.Warn("Failed to get JMX metrics (10s timeout)", "cluster_id", clusterID, "error", err)
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
		logger.Warn("Failed to get consumer group lags", "cluster_id", clusterID, "error", err)
		response.KafkaExporterAvailable = false
	} else {
		response.ConsumerGroups = consumerGroups
		response.KafkaExporterAvailable = true
	}

	// 3. 获取元数据
	metadata, err := s.kafkaExporter.GetClusterMetadata(ctx, clusterID)
	if err != nil {
		logger.Warn("Failed to get cluster metadata", "cluster_id", clusterID, "error", err)
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

// GetClusterMetadata 获取集群元数据（Broker 列表、Controller、Topic 数量）
func (s *Service) GetClusterMetadata(ctx context.Context, clusterID int64) (*ClusterMetadataInfo, error) {
	return s.kafkaExporter.GetClusterMetadata(ctx, clusterID)
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

// QueryMetricsInstant 从 VictoriaMetrics 查询即时指标
func (s *Service) QueryMetricsInstant(ctx context.Context, query string) ([]byte, error) {
	if s.vmClient == nil || !s.vmClient.IsEnabled() {
		return nil, fmt.Errorf("victoriametrics is not configured")
	}
	return s.vmClient.QueryInstant(ctx, query)
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

// BrokerOverviewItem Broker 总览信息
type BrokerOverviewItem struct {
	BrokerID      int     `json:"broker_id"`
	BrokerHost    string  `json:"broker_host"`
	LeaderCount   int     `json:"leader_count"`
	ReplicaCount  int     `json:"replica_count"`
	LeaderPercent float64 `json:"leader_percent"`
	IsController  bool    `json:"is_controller"`
}

// BrokerOverviewResponse Broker 总览响应
type BrokerOverviewResponse struct {
	Data []BrokerOverviewItem `json:"data"`
}

// vmQueryResult VM 查询结果
type vmQueryResult struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Metric map[string]string `json:"metric"`
			Value  [2]interface{}    `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// GetBrokerOverview 获取 Broker 总览数据
func (h *Handler) GetBrokerOverview(c *gin.Context) {
	clusterID, err := parseInt64Param(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid cluster id"})
		return
	}

	clusterIDStr := strconv.FormatInt(clusterID, 10)

	// 查询 broker_info、leader_count、replica_count、active_controller
	queries := map[string]string{
		"info":       fmt.Sprintf(`kafka_broker_info{cluster_id="%s"}`, clusterIDStr),
		"leader":     fmt.Sprintf(`kafka_broker_leader_count{cluster_id="%s"}`, clusterIDStr),
		"replica":    fmt.Sprintf(`kafka_broker_replica_count{cluster_id="%s"}`, clusterIDStr),
		"controller": fmt.Sprintf(`kafka_controller_kafkacontroller_activecontrollercount{cluster_id="%s"}`, clusterIDStr),
	}

	// 并行查询
	type queryResult struct {
		name   string
		result *vmQueryResult
	}

	resultCh := make(chan queryResult, len(queries))
	for name, query := range queries {
		go func(n, q string) {
			data, err := h.svc.QueryMetricsInstant(c.Request.Context(), q)
			if err != nil {
				resultCh <- queryResult{name: n, result: nil}
				return
			}
			var res vmQueryResult
			if err := json.Unmarshal(data, &res); err != nil {
				resultCh <- queryResult{name: n, result: nil}
				return
			}
			resultCh <- queryResult{name: n, result: &res}
		}(name, query)
	}

	// 收集结果
	results := make(map[string]*vmQueryResult)
	for i := 0; i < len(queries); i++ {
		r := <-resultCh
		results[r.name] = r.result
	}

	// 解析 broker 信息
	type brokerData struct {
		host       string
		leader     int
		replica    int
		controller bool
	}
	brokers := make(map[string]*brokerData)

	// 从 broker_info 获取 broker 列表
	if info := results["info"]; info != nil {
		for _, r := range info.Data.Result {
			id := r.Metric["broker_id"]
			if id == "" {
				continue
			}
			if _, exists := brokers[id]; !exists {
				brokers[id] = &brokerData{
					host: r.Metric["broker_host"],
				}
			}
		}
	}

	// leader count
	if leader := results["leader"]; leader != nil {
		for _, r := range leader.Data.Result {
			id := r.Metric["broker_id"]
			if b, ok := brokers[id]; ok {
				if v, err := parseFloat(r.Value[1]); err == nil {
					b.leader = int(v)
				}
			}
		}
	}

	// replica count
	if replica := results["replica"]; replica != nil {
		for _, r := range replica.Data.Result {
			id := r.Metric["broker_id"]
			if b, ok := brokers[id]; ok {
				if v, err := parseFloat(r.Value[1]); err == nil {
					b.replica = int(v)
				}
			}
		}
	}

	// controller
	if controller := results["controller"]; controller != nil {
		for _, r := range controller.Data.Result {
			id := r.Metric["broker_id"]
			if b, ok := brokers[id]; ok {
				if v, err := parseFloat(r.Value[1]); err == nil {
					b.controller = v > 0
				}
			}
		}
	}

	// 构建响应
	var items []BrokerOverviewItem
	// 计算集群 Leader 总数，用于每个 Broker 的 Leader Percent
	totalLeader := 0
	for _, b := range brokers {
		totalLeader += b.leader
	}
	for id, b := range brokers {
		brokerID, _ := strconv.Atoi(id)
		leaderPercent := 0.0
		if totalLeader > 0 {
			leaderPercent = float64(b.leader) / float64(totalLeader) * 100
		}
		items = append(items, BrokerOverviewItem{
			BrokerID:      brokerID,
			BrokerHost:    b.host,
			LeaderCount:   b.leader,
			ReplicaCount:  b.replica,
			LeaderPercent: leaderPercent,
			IsController:  b.controller,
		})
	}

	// 按 broker_id 排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].BrokerID < items[j].BrokerID
	})

	c.JSON(200, BrokerOverviewResponse{Data: items})
}

// parseFloat 解析 VM value 字段
func parseFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case string:
		return strconv.ParseFloat(val, 64)
	case float64:
		return val, nil
	default:
		return 0, fmt.Errorf("cannot parse %v", v)
	}
}

// parseInt64Param 解析 int64 参数
func parseInt64Param(param string) (int64, error) {
	var result int64
	_, err := fmt.Sscanf(param, "%d", &result)
	return result, err
}

// ============================================================
// Batch Query（批量查询 + 内存缓存）
// ============================================================

// batchQueryItem 单个查询请求
type batchQueryItem struct {
	ID    string `json:"id"`
	Query string `json:"query"`
	Start int64  `json:"start"`
	End   int64  `json:"end"`
	Step  string `json:"step"`
}

// batchQueryRequest 批量查询请求
type batchQueryRequest struct {
	Queries []batchQueryItem `json:"queries"`
}

// dedupedQuery 去重后的查询
type dedupedQuery struct {
	Key         string
	Query       string
	Start       time.Time
	End         time.Time
	Step        string
	OriginalIDs []string
}

// metricsCache 全局 VM 查询缓存（30 秒 TTL）
var metricsCache = cache.NewMemoryCache(30 * time.Second)

// init 确保缓存初始化
func init() {
	if metricsCache == nil {
		metricsCache = cache.NewMemoryCache(30 * time.Second)
	}
}

// BatchQueryMetrics 批量查询指标（去重 + 缓存 + 并发查 VM）
// 将前端 N 个独立请求合并为 1 个 POST，后端去重后只查 VM 一次，结果缓存 30 秒
func (h *Handler) BatchQueryMetrics(c *gin.Context) {
	var req batchQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request body"})
		return
	}

	if len(req.Queries) == 0 {
		c.JSON(400, gin.H{"error": "queries is empty"})
		return
	}

	// 1. 去重：相同 query+start+end+step 只查一次
	uniqueMap := make(map[string]int)
	deduped := make([]dedupedQuery, 0, len(req.Queries))
	for _, q := range req.Queries {
		key := fmt.Sprintf("%s|%d|%d|%s", q.Query, q.Start, q.End, q.Step)
		if idx, ok := uniqueMap[key]; ok {
			deduped[idx].OriginalIDs = append(deduped[idx].OriginalIDs, q.ID)
		} else {
			uniqueMap[key] = len(deduped)
			deduped = append(deduped, dedupedQuery{
				Key:         key,
				Query:       q.Query,
				Start:       time.Unix(q.Start, 0),
				End:         time.Unix(q.End, 0),
				Step:        q.Step,
				OriginalIDs: []string{q.ID},
			})
		}
	}

	// 2. 并发查询（缓存 + VM）
	ctx := c.Request.Context()
	results := make(map[string]json.RawMessage, len(req.Queries))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := range deduped {
		dq := &deduped[i]
		wg.Add(1)
		go func() {
			defer wg.Done()

			// 检查缓存
			if cached, err := metricsCache.Get(ctx, dq.Key); err == nil && cached != nil {
				mu.Lock()
				if data, ok := cached.([]byte); ok {
					for _, id := range dq.OriginalIDs {
						results[id] = json.RawMessage(data)
					}
				}
				mu.Unlock()
				return
			}

			// 查询 VM
			result, err := h.svc.QueryMetricsRange(ctx, dq.Query, dq.Start, dq.End, dq.Step)
			if err != nil {
				result = []byte(`{"status":"error","data":{"result":[]}}`)
			}

			// 写入缓存
			_ = metricsCache.Set(ctx, dq.Key, result, 0)

			mu.Lock()
			for _, id := range dq.OriginalIDs {
				results[id] = json.RawMessage(result)
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	c.JSON(200, gin.H{"results": results})
}

// ============================================================
// Dashboard VM Data（Dashboard / Cluster 扩展复用）
// ============================================================

// DashboardVMData Dashboard 所需的 VM 聚合数据
// 注意：此类型与 dashboard 包中的同名类型字段一致，但分属不同包，
// dashboard 包通过 VMAggregator 接口引用此类型
type DashboardVMData struct {
	TotalBrokers          int
	TotalPartitions       int
	CGTotal               int
	CGLag                 int64
	BrokerCountByCluster  map[int64]int     // cluster_id -> current broker_count
	BrokerMaxByCluster    map[int64]int     // cluster_id -> max broker_count in 7d
	HealthStatusByCluster map[int64]string  // cluster_id -> health_status
}

// dashboardVMCache Dashboard VM 聚合数据缓存（30s TTL）
var dashboardVMCache = cache.NewMemoryCache(30 * time.Second)

const dashboardVMCacheKey = "monitor:dashboard_vm_data"

// GetDashboardVMData 获取 Dashboard 所需的 VM 聚合数据
// 一次调用内并发查 6 组 PromQL，30s 内存缓存
func (s *Service) GetDashboardVMData(ctx context.Context, clusterIDs []int64) *DashboardVMData {
	if s.vmClient == nil || !s.vmClient.IsEnabled() {
		return nil
	}

	// 检查缓存
	if cached, err := dashboardVMCache.Get(ctx, dashboardVMCacheKey); err == nil && cached != nil {
		if data, ok := cached.([]byte); ok {
			var d DashboardVMData
			if err := json.Unmarshal(data, &d); err == nil {
				return &d
			}
		}
	}

	// 构建 cluster_id 正则选择器
	idStrs := make([]string, len(clusterIDs))
	for i, id := range clusterIDs {
		idStrs[i] = strconv.FormatInt(id, 10)
	}
	clusterSelector := strings.Join(idStrs, "|")

	// 6 组并发 PromQL 查询
	type queryResult struct {
		name string
		data []byte
		err  error
	}

	queries := map[string]string{
		"brokers_total":    fmt.Sprintf(`sum(kafka_broker_info{app="kmanager",cluster_id=~"%s"}) by (cluster_id)`, clusterSelector),
		"broker_max_7d":    fmt.Sprintf(`max by (cluster_id) (max_over_time(kafka_broker_count{app="kmanager",cluster_id=~"%s"}[7d]))`, clusterSelector),
		"broker_current":   fmt.Sprintf(`max by (cluster_id) (kafka_broker_count{app="kmanager",cluster_id=~"%s"})`, clusterSelector),
		"partitions_total": fmt.Sprintf(`sum(kafka_topic_partition_replicas{app="kmanager",cluster_id=~"%s"})`, clusterSelector),
		"cg_total":         `count(count(kafka_consumergroup_lag{app="kmanager"}) by (group,cluster_id))`,
		"cg_lag":           fmt.Sprintf(`sum(kafka_consumergroup_lag{app="kmanager",cluster_id=~"%s"})`, clusterSelector),
	}

	resultCh := make(chan queryResult, len(queries))
	for name, query := range queries {
		go func(n, q string) {
			data, err := s.vmClient.QueryInstant(ctx, q)
			resultCh <- queryResult{name: n, data: data, err: err}
		}(name, query)
	}

	results := make(map[string][]byte)
	for i := 0; i < len(queries); i++ {
		r := <-resultCh
		if r.err != nil {
			logger.Warn("VM query failed", "name", r.name, "error", r.err)
			continue
		}
		results[r.name] = r.data
	}

	// 解析结果
	brokersByCluster := parseVMValuesByClusterInt64(results["brokers_total"])
	brokerMaxByCluster := parseVMValuesByClusterInt64(results["broker_max_7d"])
	brokerCurrentByCluster := parseVMValuesByClusterInt64(results["broker_current"])

	totalBrokers := 0
	for _, bc := range brokersByCluster {
		totalBrokers += bc
	}
	totalPartitions := 0
	if data, ok := results["partitions_total"]; ok {
		totalPartitions = sumVMInstantValues(data)
	}
	cgTotal := 0
	if data, ok := results["cg_total"]; ok {
		cgTotal = sumVMInstantValues(data)
	}
	var cgLag int64
	if data, ok := results["cg_lag"]; ok {
		cgLag = int64(sumVMInstantValues(data))
	}

	// 逐集群判定健康状态
	// 逻辑：7天内 broker 数最大值 vs 当前值，差值为0→healthy，差值>0→error（有 broker 掉线）
	// 不依赖 JMX 指标（URP/offline），因为不是所有集群都配 JMX
	healthByCluster := make(map[int64]string, len(clusterIDs))
	for _, id := range clusterIDs {
		maxBroker, hasMax := brokerMaxByCluster[id]
		currentBroker, hasCurrent := brokerCurrentByCluster[id]
		if hasMax && hasCurrent {
			if currentBroker < maxBroker {
				healthByCluster[id] = "error" // 有 broker 掉线
			} else {
				healthByCluster[id] = "healthy"
			}
		} else if hasCurrent {
			// 有当前值但无7d历史（新集群），无法比较，用当前值有数据即认为可达
			healthByCluster[id] = "healthy"
		} else if _, hasBrokerInfo := brokersByCluster[id]; hasBrokerInfo {
			// kafka_broker_info 有数据但 kafka_broker_count 无，说明有旧指标残留
			healthByCluster[id] = "healthy"
		} else {
			healthByCluster[id] = "unknown"
		}
	}

	vmData := &DashboardVMData{
		TotalBrokers:           totalBrokers,
		TotalPartitions:        totalPartitions,
		CGTotal:                cgTotal,
		CGLag:                  cgLag,
		BrokerCountByCluster:   brokersByCluster,
		BrokerMaxByCluster:     brokerMaxByCluster,
		HealthStatusByCluster:  healthByCluster,
	}

	// 写入缓存
	if data, err := json.Marshal(vmData); err == nil {
		_ = dashboardVMCache.Set(ctx, dashboardVMCacheKey, data, 0)
	}

	return vmData
}

// GetClustersHealthStatus 批量获取集群健康状态（供 Cluster Handler 复用）
func (s *Service) GetClustersHealthStatus(ctx context.Context, clusterIDs []int64) map[int64]string {
	result := make(map[int64]string, len(clusterIDs))

	if s.vmClient != nil && s.vmClient.IsEnabled() {
		vmData := s.GetDashboardVMData(ctx, clusterIDs)
		if vmData != nil {
			for _, id := range clusterIDs {
				if status, ok := vmData.HealthStatusByCluster[id]; ok {
					result[id] = status
				} else {
					result[id] = "unknown"
				}
			}
		}
	}

	// VM 未覆盖或返回 unknown 的集群，用 AdminClient 补充
	// 如果 AdminClient 能连上说明集群可达 → healthy
	needFallback := false
	for _, id := range clusterIDs {
		if result[id] == "" || result[id] == "unknown" {
			needFallback = true
			break
		}
	}
	if needFallback {
		adminCounts := s.GetAdminClientBrokerCounts(ctx, clusterIDs)
		for _, id := range clusterIDs {
			if result[id] == "" || result[id] == "unknown" {
				if _, ok := adminCounts[id]; ok {
					result[id] = "healthy"
				} else {
					result[id] = "unknown"
				}
			}
		}
	}

	// 填充默认值
	for _, id := range clusterIDs {
		if result[id] == "" {
			result[id] = "unknown"
		}
	}

	return result
}

// GetAdminClientBrokerCounts 通过 AdminClient 直接查询各集群的 broker 数量
// 不依赖 VM，当 VM 不可达时作为兜底数据源
func (s *Service) GetAdminClientBrokerCounts(ctx context.Context, clusterIDs []int64) map[int64]int {
	result := make(map[int64]int, len(clusterIDs))

	type brokerResult struct {
		clusterID int64
		count     int
	}
	ch := make(chan brokerResult, len(clusterIDs))

	for _, id := range clusterIDs {
		go func(clusterID int64) {
			meta, err := s.kafkaExporter.GetClusterMetadata(ctx, clusterID)
			if err == nil && meta != nil {
				ch <- brokerResult{clusterID: clusterID, count: len(meta.Brokers)}
			} else {
				ch <- brokerResult{clusterID: clusterID, count: 0}
			}
		}(id)
	}

	for i := 0; i < len(clusterIDs); i++ {
		r := <-ch
		if r.count > 0 {
			result[r.clusterID] = r.count
		}
	}
	return result
}

// GetAdminClientPartitionCounts 通过 AdminClient 直接查询各集群的分区总数
// 不依赖 VM，当 VM 不可达时作为兜底数据源
func (s *Service) GetAdminClientPartitionCounts(ctx context.Context, clusterIDs []int64) int {
	total := 0
	ch := make(chan int, len(clusterIDs))

	for _, id := range clusterIDs {
		go func(clusterID int64) {
			partitionCount, err := s.GetTopicPartitionCount(ctx, clusterID)
			if err != nil {
				ch <- 0
				return
			}
			count := 0
			for _, pc := range partitionCount {
				count += int(pc)
			}
			ch <- count
		}(id)
	}

	for i := 0; i < len(clusterIDs); i++ {
		total += <-ch
	}
	return total
}

// GetAdminClientConsumerGroupStats 通过 AdminClient 直接查询消费者组统计
// 不依赖 VM，当 VM 不可达时作为兜底数据源
func (s *Service) GetAdminClientConsumerGroupStats(ctx context.Context, clusterIDs []int64) (totalGroups int, totalLag int64) {
	type cgResult struct {
		groups int
		lag    int64
	}
	ch := make(chan cgResult, len(clusterIDs))

	for _, id := range clusterIDs {
		go func(clusterID int64) {
			groups, err := s.GetConsumerGroupLags(ctx, clusterID)
			if err != nil {
				ch <- cgResult{}
				return
			}
			var lag int64
			for _, g := range groups {
				lag += g.TotalLag
			}
			ch <- cgResult{groups: len(groups), lag: lag}
		}(id)
	}

	for i := 0; i < len(clusterIDs); i++ {
		r := <-ch
		totalGroups += r.groups
		totalLag += r.lag
	}
	return
}

// GetBrokerCountByCluster 批量获取集群 broker 数量
func (s *Service) GetBrokerCountByCluster(ctx context.Context, clusterIDs []int64) map[int64]int {
	result := make(map[int64]int, len(clusterIDs))

	if s.vmClient != nil && s.vmClient.IsEnabled() {
		vmData := s.GetDashboardVMData(ctx, clusterIDs)
		if vmData != nil {
			for _, id := range clusterIDs {
				if bc, ok := vmData.BrokerCountByCluster[id]; ok {
					result[id] = bc
				}
			}
		}
	}

	// VM 没覆盖到的集群，用 AdminClient 直连补充
	if len(result) < len(clusterIDs) {
		adminCounts := s.GetAdminClientBrokerCounts(ctx, clusterIDs)
		for _, id := range clusterIDs {
			if _, ok := result[id]; !ok {
				if ac, ok2 := adminCounts[id]; ok2 {
					result[id] = ac
				}
			}
		}
	}

	return result
}

// sumVMInstantValues 解析 VM instant query返回值并求和
func sumVMInstantValues(data []byte) int {
	if data == nil {
		return 0
	}
	var result vmQueryResult
	if err := json.Unmarshal(data, &result); err != nil {
		return 0
	}
	total := 0
	for _, r := range result.Data.Result {
		if len(r.Value) >= 2 {
			if v, err := parseFloatValue(r.Value[1]); err == nil {
				total += int(v)
			}
		}
	}
	return total
}

// parseVMValuesByClusterStr 解析 VM instant query，按 cluster_id(string) 返回 map
func parseVMValuesByClusterStr(data []byte) map[string]float64 {
	result := make(map[string]float64)
	if data == nil {
		return result
	}
	var vmResult vmQueryResult
	if err := json.Unmarshal(data, &vmResult); err != nil {
		return result
	}
	for _, r := range vmResult.Data.Result {
		cid := r.Metric["cluster_id"]
		if cid == "" {
			continue
		}
		if len(r.Value) >= 2 {
			if v, err := parseFloatValue(r.Value[1]); err == nil {
				result[cid] = v
			}
		}
	}
	return result
}

// parseVMValuesByClusterInt64 解析 VM instant query，按 cluster_id(int64) 返回 map
func parseVMValuesByClusterInt64(data []byte) map[int64]int {
	result := make(map[int64]int)
	if data == nil {
		return result
	}
	var vmResult vmQueryResult
	if err := json.Unmarshal(data, &vmResult); err != nil {
		return result
	}
	for _, r := range vmResult.Data.Result {
		cidStr := r.Metric["cluster_id"]
		if cidStr == "" {
			continue
		}
		cid, err := strconv.ParseInt(cidStr, 10, 64)
		if err != nil {
			continue
		}
		if len(r.Value) >= 2 {
			if v, err := parseFloatValue(r.Value[1]); err == nil {
				result[cid] = int(v)
			}
		}
	}
	return result
}

// parseFloatValue 解析 VM value 字段为 float64
func parseFloatValue(v interface{}) (float64, error) {
	switch val := v.(type) {
	case string:
		return strconv.ParseFloat(val, 64)
	case float64:
		return val, nil
	default:
		return 0, fmt.Errorf("cannot parse %v", v)
	}
}
