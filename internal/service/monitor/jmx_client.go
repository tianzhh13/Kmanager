package monitor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// JMXMetric JMX 指标
type JMXMetric struct {
	Name   string            `json:"name"`
	Value  float64           `json:"value"`
	Labels map[string]string `json:"labels"`
}

// BrokerMetrics Broker 指标汇总
type BrokerMetrics struct {
	// 吞吐量
	MessagesInPerSec float64 `json:"messages_in_per_sec"`
	BytesInPerSec    float64 `json:"bytes_in_per_sec"`
	BytesOutPerSec   float64 `json:"bytes_out_per_sec"`

	// 副本状态
	UnderReplicatedPartitions float64 `json:"under_replicated_partitions"`
	ISRShrinksPerSec          float64 `json:"isr_shrinks_per_sec"`
	ISRExpandsPerSec          float64 `json:"isr_expands_per_sec"`

	// Controller
	ActiveControllerCount  float64 `json:"active_controller_count"`
	OfflinePartitionsCount float64 `json:"offline_partitions_count"`

	// 请求
	TotalProduceRequestsPerSec float64 `json:"total_produce_requests_per_sec"`
	TotalFetchRequestsPerSec   float64 `json:"total_fetch_requests_per_sec"`
	RequestQueueSize           float64 `json:"request_queue_size"`

	// JVM
	HeapMemoryUsed float64 `json:"heap_memory_used"`
	HeapMemoryMax  float64 `json:"heap_memory_max"`
	GCTimeSeconds  float64 `json:"gc_time_seconds"`
	GCCount        float64 `json:"gc_count"`
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

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL, nil)
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body failed: %w", err)
	}

	return parsePrometheusMetrics(string(body)), nil
}

// parsePrometheusMetrics 解析 Prometheus 格式的 metrics
func parsePrometheusMetrics(data string) []JMXMetric {
	var metrics []JMXMetric
	lines := strings.Split(data, "\n")

	// 匹配 Prometheus 格式: metric_name{label1="value1",label2="value2"} value
	metricPattern := regexp.MustCompile(`^([a-zA-Z_:][a-zA-Z0-9_:]*)((?:\{[^}]*\})?)?\s+([0-9eE.+-]+|NaN|\+Inf|-Inf)$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		matches := metricPattern.FindStringSubmatch(line)
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
	// 吞吐量 - 尝试多种匹配方式
	result.MessagesInPerSec = findMetricValue(metricMap, "kafka_server_BrokerTopicMetrics_MessagesInPerSec", "kafka_server_BrokerTopicMetrics_OneMinuteRate")
	result.BytesInPerSec = findMetricValue(metricMap, "kafka_server_BrokerTopicMetrics_BytesInPerSec", "kafka_server_BrokerTopicMetrics_OneMinuteRate")
	result.BytesOutPerSec = findMetricValue(metricMap, "kafka_server_BrokerTopicMetrics_BytesOutPerSec", "kafka_server_BrokerTopicMetrics_OneMinuteRate")

	// 副本状态
	result.UnderReplicatedPartitions = getFirstValue(metricMap, "kafka_server_ReplicaManager_UnderReplicatedPartitions")
	result.ISRShrinksPerSec = getFirstValue(metricMap, "kafka_server_ReplicaManager_IsrShrinksPerSec")
	result.ISRExpandsPerSec = getFirstValue(metricMap, "kafka_server_ReplicaManager_IsrExpandsPerSec")

	// Controller
	result.ActiveControllerCount = getFirstValue(metricMap, "kafka_controller_KafkaController_ActiveControllerCount")
	result.OfflinePartitionsCount = getFirstValue(metricMap, "kafka_controller_KafkaController_OfflinePartitionsCount")

	// 请求
	result.TotalProduceRequestsPerSec = getFirstValue(metricMap, "kafka_server_BrokerTopicMetrics_TotalProduceRequestsPerSec")
	result.TotalFetchRequestsPerSec = getFirstValue(metricMap, "kafka_server_BrokerTopicMetrics_TotalFetchRequestsPerSec")
	result.RequestQueueSize = getFirstValue(metricMap, "kafka_network_RequestChannel_RequestQueueSize")

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
