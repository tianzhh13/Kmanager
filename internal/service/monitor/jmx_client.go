package monitor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// promMetricPattern 预编译的 Prometheus 格式匹配正则
var promMetricPattern = regexp.MustCompile(`^([a-zA-Z_:][a-zA-Z0-9_:]*)((?:\{[^}]*\})?)?\s+([0-9eE.+-]+|NaN|\+Inf|-Inf)$`)

// JMXMetric JMX 指标
type JMXMetric struct {
	Name   string            `json:"name"`
	Value  float64           `json:"value"`
	Labels map[string]string `json:"labels"`
}

// BrokerMetrics Broker 指标汇总（集群维度，聚合所有 Broker）
type BrokerMetrics struct {
	// 吞吐量（集群汇总）
	MessagesInPerSec float64 `json:"messages_in_per_sec"`
	BytesInPerSec    float64 `json:"bytes_in_per_sec"`
	BytesOutPerSec   float64 `json:"bytes_out_per_sec"`

	// 副本状态（取第一个 Broker 的值，因为是集群级别的）
	UnderReplicatedPartitions float64 `json:"under_replicated_partitions"`
	ISRShrinksPerSec          float64 `json:"isr_shrinks_per_sec"`
	ISRExpandsPerSec          float64 `json:"isr_expands_per_sec"`

	// Controller（集群只有一个 Active Controller）
	ActiveControllerCount  float64 `json:"active_controller_count"`
	OfflinePartitionsCount float64 `json:"offline_partitions_count"`

	// 请求（集群汇总）
	TotalProduceRequestsPerSec float64 `json:"total_produce_requests_per_sec"`
	TotalFetchRequestsPerSec   float64 `json:"total_fetch_requests_per_sec"`
	RequestQueueSize           float64 `json:"request_queue_size"`

	// JVM（不适用于集群汇总，保留字段）
	HeapMemoryUsed float64 `json:"heap_memory_used"`
	HeapMemoryMax  float64 `json:"heap_memory_max"`
	GCTimeSeconds  float64 `json:"gc_time_seconds"`
	GCCount        float64 `json:"gc_count"`

	// Broker 数量
	BrokerCount int `json:"broker_count"`
}

// BrokerMetricDetail 单个 Broker 的指标详情
type BrokerMetricDetail struct {
	BrokerID   int            `json:"broker_id"`
	BrokerHost string         `json:"broker_host"`
	Metrics    *BrokerMetrics `json:"metrics"`
}

// JMXClient JMX Exporter HTTP 客户端
type JMXClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewJMXClient 创建 JMX 客户端
func NewJMXClient(url string) *JMXClient {
	return &JMXClient{
		baseURL: url,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// FetchMetrics 获取所有指标
func (c *JMXClient) FetchMetrics(ctx context.Context) ([]JMXMetric, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("jmx exporter url is empty")
	}

	// 确保 URL 以 /metrics 结尾
	metricsURL := c.baseURL
	if !strings.HasSuffix(metricsURL, "/metrics") {
		metricsURL = strings.TrimSuffix(metricsURL, "/") + "/metrics"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", metricsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch metrics failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 限制 10MB
	if err != nil {
		return nil, fmt.Errorf("read response body failed: %w", err)
	}

	return parsePrometheusMetrics(string(body)), nil
}

// ParseJMXExporterURLs 解析 JMX Exporter URL 列表（逗号分隔）
func ParseJMXExporterURLs(urls string) []string {
	if urls == "" {
		return nil
	}
	parts := strings.Split(urls, ",")
	var result []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// MultiJMXClient 多 Broker JMX 客户端
type MultiJMXClient struct {
	urls       []string
	httpClient *http.Client
}

// NewMultiJMXClient 创建多 Broker JMX 客户端
func NewMultiJMXClient(urls []string) *MultiJMXClient {
	return &MultiJMXClient{
		urls: urls,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// FetchAllBrokerMetrics 并行获取所有 Broker 的指标
func (m *MultiJMXClient) FetchAllBrokerMetrics(ctx context.Context) ([]BrokerMetricDetail, error) {
	if len(m.urls) == 0 {
		return nil, fmt.Errorf("no jmx exporter urls configured")
	}

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		result []BrokerMetricDetail
		errors []error
	)

	for i, url := range m.urls {
		wg.Add(1)
		go func(index int, jmxURL string) {
			defer wg.Done()

			client := NewJMXClient(jmxURL)
			metrics, err := client.GetBrokerMetrics(ctx)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("broker %d (%s): %w", index, jmxURL, err))
				mu.Unlock()
				return
			}

			// 从 URL 中提取 host 作为标识
			host := extractHostFromURL(jmxURL)

			mu.Lock()
			result = append(result, BrokerMetricDetail{
				BrokerID:   index + 1,
				BrokerHost: host,
				Metrics:    metrics,
			})
			mu.Unlock()
		}(i, url)
	}

	wg.Wait()

	// 如果全部失败，返回错误
	if len(result) == 0 && len(errors) > 0 {
		return nil, fmt.Errorf("all jmx exporters failed: %v", errors)
	}

	return result, nil
}

// GetAggregatedMetrics 获取聚合后的集群级别指标
func (m *MultiJMXClient) GetAggregatedMetrics(ctx context.Context) (*BrokerMetrics, error) {
	details, err := m.FetchAllBrokerMetrics(ctx)
	if err != nil {
		return nil, err
	}

	if len(details) == 0 {
		return nil, fmt.Errorf("no broker metrics available")
	}

	aggregated := &BrokerMetrics{
		BrokerCount: len(details),
	}

	for _, detail := range details {
		if detail.Metrics == nil {
			continue
		}

		// 吞吐量：累加所有 Broker
		aggregated.MessagesInPerSec += detail.Metrics.MessagesInPerSec
		aggregated.BytesInPerSec += detail.Metrics.BytesInPerSec
		aggregated.BytesOutPerSec += detail.Metrics.BytesOutPerSec
		aggregated.TotalProduceRequestsPerSec += detail.Metrics.TotalProduceRequestsPerSec
		aggregated.TotalFetchRequestsPerSec += detail.Metrics.TotalFetchRequestsPerSec
		aggregated.RequestQueueSize += detail.Metrics.RequestQueueSize

		// Controller 指标：取第一个有效值（集群级别只有一个 Active Controller）
		if aggregated.ActiveControllerCount == 0 && detail.Metrics.ActiveControllerCount > 0 {
			aggregated.ActiveControllerCount = detail.Metrics.ActiveControllerCount
		}
		if aggregated.OfflinePartitionsCount == 0 {
			aggregated.OfflinePartitionsCount = detail.Metrics.OfflinePartitionsCount
		}
		if aggregated.UnderReplicatedPartitions == 0 {
			aggregated.UnderReplicatedPartitions = detail.Metrics.UnderReplicatedPartitions
		}
	}

	return aggregated, nil
}

// extractHostFromURL 从 URL 中提取 host
func extractHostFromURL(urlStr string) string {
	// 简单提取：去掉 http:// 前缀和端口
	urlStr = strings.TrimPrefix(urlStr, "http://")
	urlStr = strings.TrimPrefix(urlStr, "https://")
	if idx := strings.Index(urlStr, ":"); idx > 0 {
		return urlStr[:idx]
	}
	if idx := strings.Index(urlStr, "/"); idx > 0 {
		return urlStr[:idx]
	}
	return urlStr
}

// parsePrometheusMetrics 解析 Prometheus 格式的 metrics
func parsePrometheusMetrics(data string) []JMXMetric {
	var metrics []JMXMetric
	lines := strings.Split(data, "\n")

	// 匹配 Prometheus 格式: metric_name{label1="value1",label2="value2"} value
	// 使用预编译的包级正则
	_ = promMetricPattern

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		matches := promMetricPattern.FindStringSubmatch(line)
		if len(matches) != 4 {
			continue
		}

		name := matches[1]
		labelsStr := matches[2]
		valueStr := matches[3]

		// 处理特殊值
		var value float64
		switch valueStr {
		case "NaN":
			value = 0
		case "+Inf":
			value = float64(1e308)
		case "-Inf":
			value = float64(-1e308)
		default:
			var err error
			value, err = strconv.ParseFloat(valueStr, 64)
			if err != nil {
				continue
			}
		}

		labels := parseLabels(labelsStr)

		metrics = append(metrics, JMXMetric{
			Name:   name,
			Value:  value,
			Labels: labels,
		})
	}

	return metrics
}

// parseLabels 解析标签
func parseLabels(labelsStr string) map[string]string {
	labels := make(map[string]string)
	if labelsStr == "" {
		return labels
	}

	// 去掉外层花括号
	labelsStr = strings.Trim(labelsStr, "{}")
	if labelsStr == "" {
		return labels
	}

	// 分割标签
	parts := strings.Split(labelsStr, ",")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		value := strings.Trim(kv[1], `"`)
		labels[key] = value
	}

	return labels
}

// GetBrokerMetrics 获取 Broker 关键指标汇总
func (c *JMXClient) GetBrokerMetrics(ctx context.Context) (*BrokerMetrics, error) {
	metrics, err := c.FetchMetrics(ctx)
	if err != nil {
		return nil, err
	}

	result := &BrokerMetrics{}
	metricMap := make(map[string]map[string]float64) // name -> labels_hash -> value

	for _, m := range metrics {
		if _, ok := metricMap[m.Name]; !ok {
			metricMap[m.Name] = make(map[string]float64)
		}
		// 简化处理：只取第一个匹配的值
		if len(metricMap[m.Name]) == 0 {
			metricMap[m.Name][""] = m.Value
		}
	}

	// 解析关键指标
	// 吞吐量 - JMX Exporter 使用小写 + _total 后缀
	result.MessagesInPerSec = findMetricValue(metricMap, "kafka_server_brokertopicmetrics_messagesin_total", "kafka_server_BrokerTopicMetrics_MessagesInPersec")
	result.BytesInPerSec = findMetricValue(metricMap, "kafka_server_brokertopicmetrics_bytesin_total", "kafka_server_BrokerTopicMetrics_BytesInPersec")
	result.BytesOutPerSec = findMetricValue(metricMap, "kafka_server_brokertopicmetrics_bytesout_total", "kafka_server_BrokerTopicMetrics_BytesOutPersec")

	// 副本状态
	result.UnderReplicatedPartitions = findMetricValue(metricMap, "kafka_server_replicamanager_underreplicatedpartitions", "kafka_server_ReplicaManager_UnderReplicatedPartitions")
	result.ISRShrinksPerSec = findMetricValue(metricMap, "kafka_server_replicamanager_isrshrinkspersec", "kafka_server_ReplicaManager_IsrShrinksPerSec")
	result.ISRExpandsPerSec = findMetricValue(metricMap, "kafka_server_replicamanager_isrexpandspersec", "kafka_server_ReplicaManager_IsrExpandsPerSec")

	// Controller
	result.ActiveControllerCount = findMetricValue(metricMap, "kafka_controller_kafkacontroller_activecontrollercount", "kafka_controller_KafkaController_ActiveControllerCount")
	result.OfflinePartitionsCount = findMetricValue(metricMap, "kafka_controller_kafkacontroller_offlinepartitionscount", "kafka_controller_KafkaController_OfflinePartitionsCount")

	// 请求
	result.TotalProduceRequestsPerSec = findMetricValue(metricMap, "kafka_server_brokertopicmetrics_totalproducerequests_total", "kafka_server_BrokerTopicMetrics_TotalProduceRequestsPersec")
	result.TotalFetchRequestsPerSec = findMetricValue(metricMap, "kafka_server_brokertopicmetrics_totalfetchrequests_total", "kafka_server_BrokerTopicMetrics_TotalFetchRequestsPersec")
	result.RequestQueueSize = findMetricValue(metricMap, "kafka_network_requestchannel_requestqueuesize", "kafka_network_RequestChannel_RequestQueueSize")

	// JVM
	result.HeapMemoryUsed = getFirstValue(metricMap, "jvm_memory_bytes_used")
	result.HeapMemoryMax = getFirstValue(metricMap, "jvm_memory_bytes_max")
	result.GCTimeSeconds = getFirstValue(metricMap, "jvm_gc_collection_seconds_sum")
	result.GCCount = getFirstValue(metricMap, "jvm_gc_collection_seconds_count")

	return result, nil
}

// findMetricValue 尝试多个指标名查找值
func findMetricValue(metricMap map[string]map[string]float64, names ...string) float64 {
	for _, name := range names {
		if v, ok := metricMap[name]; ok {
			for _, val := range v {
				return val
			}
		}
	}
	return 0
}

// getFirstValue 获取第一个值
func getFirstValue(metricMap map[string]map[string]float64, name string) float64 {
	if v, ok := metricMap[name]; ok {
		for _, val := range v {
			return val
		}
	}
	return 0
}

// BrokerRawMetrics 单个 Broker 的原始 JMX 指标集合
type BrokerRawMetrics struct {
	BrokerID   int
	BrokerHost string
	Metrics    []JMXMetric
}

// FetchAllBrokerRawMetrics 并行获取所有 Broker 的原始指标
func (m *MultiJMXClient) FetchAllBrokerRawMetrics(ctx context.Context) ([]BrokerRawMetrics, error) {
	if len(m.urls) == 0 {
		return nil, fmt.Errorf("no jmx exporter urls configured")
	}

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		result []BrokerRawMetrics
	)

	for i, url := range m.urls {
		wg.Add(1)
		go func(index int, jmxURL string) {
			defer wg.Done()

			client := NewJMXClient(jmxURL)
			metrics, err := client.FetchMetrics(ctx)
			if err != nil {
				return
			}

			host := extractHostFromURL(jmxURL)

			mu.Lock()
			result = append(result, BrokerRawMetrics{
				BrokerID:   index + 1,
				BrokerHost: host,
				Metrics:    metrics,
			})
			mu.Unlock()
		}(i, url)
	}

	wg.Wait()

	if len(result) == 0 {
		return nil, fmt.Errorf("all jmx exporters failed")
	}

	return result, nil
}

// HealthCheck 检查 JMX Exporter 是否可用
func (c *JMXClient) HealthCheck(ctx context.Context) error {
	if c.baseURL == "" {
		return fmt.Errorf("jmx exporter url not configured")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status code: %d", resp.StatusCode)
	}

	return nil
}
