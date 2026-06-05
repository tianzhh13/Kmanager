package victoriametrics

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client VictoriaMetrics 客户端
type Client struct {
	writeURL   string
	queryURL   string
	httpClient *http.Client
	enabled    bool
}

// NewClient 创建 VictoriaMetrics 客户端
func NewClient(writeURL, queryURL string, enabled bool) *Client {
	return &Client{
		writeURL: writeURL,
		queryURL: queryURL,
		enabled:  enabled,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Metric 指标
type Metric struct {
	Name   string
	Value  float64
	Labels map[string]string
}

const (
	writeBatchSize = 5000 // 每批最大写入指标数
	appLabel       = "app"
	appName        = "kmanager"
)

// Write 写入指标（Prometheus remote write 格式，自动分片）
func (c *Client) Write(ctx context.Context, metrics []Metric) error {
	if !c.enabled {
		return nil
	}

	// 分片写入，每批最多 writeBatchSize 条
	for i := 0; i < len(metrics); i += writeBatchSize {
		end := i + writeBatchSize
		if end > len(metrics) {
			end = len(metrics)
		}
		batch := metrics[i:end]
		if err := c.writeBatch(ctx, batch); err != nil {
			return fmt.Errorf("write metrics batch [%d:%d] failed: %w", i, end, err)
		}
	}

	return nil
}

// writeBatch 写入单批指标
func (c *Client) writeBatch(ctx context.Context, metrics []Metric) error {
	// 构建 Prometheus 格式数据
	var buf bytes.Buffer
	for _, m := range metrics {
		buf.WriteString(m.Name)
		// 始终注入 app="kmanager" 标签
		buf.WriteString("{")
		buf.WriteString(appLabel)
		buf.WriteString("=\"")
		buf.WriteString(appName)
		buf.WriteString("\"")
		for k, v := range m.Labels {
			buf.WriteString(",")
			buf.WriteString(k)
			buf.WriteString("=\"")
			buf.WriteString(escapeLabelValue(v))
			buf.WriteString("\"")
		}
		buf.WriteString("}")
		buf.WriteString(" ")
		buf.WriteString(strconv.FormatFloat(m.Value, 'f', -1, 64))
		buf.WriteString("\n")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.writeURL, &buf)
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("write metrics failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("write metrics failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// WriteBatch 批量写入指标
func (c *Client) WriteBatch(ctx context.Context, metrics []Metric) error {
	return c.Write(ctx, metrics)
}

// Query 查询指标（PromQL/MetricsQL）
func (c *Client) Query(ctx context.Context, query string) ([]byte, error) {
	if !c.enabled {
		return nil, fmt.Errorf("victoriametrics is not enabled")
	}

	queryURL := fmt.Sprintf("%s/api/v1/query?query=%s", c.queryURL, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query metrics failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("query metrics failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024)) // 限制 50MB
}

// QueryRange 范围查询
func (c *Client) QueryRange(ctx context.Context, query string, start, end time.Time, step string) ([]byte, error) {
	if !c.enabled {
		return nil, fmt.Errorf("victoriametrics is not enabled")
	}

	queryURL := fmt.Sprintf("%s/api/v1/query_range?query=%s&start=%d&end=%d&step=%s",
		c.queryURL,
		url.QueryEscape(query),
		start.Unix(),
		end.Unix(),
		step,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query range failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("query range failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024)) // 限制 50MB
}

// QueryInstant 即时查询
func (c *Client) QueryInstant(ctx context.Context, query string) ([]byte, error) {
	if !c.enabled {
		return nil, fmt.Errorf("victoriametrics is not enabled")
	}

	queryURL := fmt.Sprintf("%s/api/v1/query?query=%s",
		c.queryURL,
		url.QueryEscape(query),
	)

	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query instant failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("query instant failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024)) // 限制 50MB
}

// HealthCheck 检查连接
func (c *Client) HealthCheck(ctx context.Context) error {
	if !c.enabled {
		return nil
	}

	// 尝试查询一个简单的指标
	req, err := http.NewRequestWithContext(ctx, "GET", c.queryURL+"/api/v1/query?query=up", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: status=%d", resp.StatusCode)
	}

	return nil
}

// IsEnabled 是否启用
func (c *Client) IsEnabled() bool {
	return c.enabled
}

// escapeLabelValue 转义标签值
func escapeLabelValue(v string) string {
	v = strings.ReplaceAll(v, "\\", "\\\\")
	v = strings.ReplaceAll(v, "\"", "\\\"")
	v = strings.ReplaceAll(v, "\n", "\\n")
	return v
}
