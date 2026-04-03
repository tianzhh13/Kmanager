package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client Prometheus HTTP API 客户端
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient 创建 Prometheus 客户端
func NewClient(prometheusURL string) *Client {
	return &Client{
		baseURL: prometheusURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// QueryResult Prometheus 查询结果
type QueryResult struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// QueryRangeResult 范围查询结果
type QueryRangeResult struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Values [][]interface{}   `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

// Query 执行即时查询
func (c *Client) Query(ctx context.Context, query string, timestamp time.Time) (*QueryResult, error) {
	params := url.Values{}
	params.Set("query", query)
	if !timestamp.IsZero() {
		params.Set("time", timestamp.Format(time.RFC3339))
	}

	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/api/v1/query?%s", c.baseURL, params.Encode()), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prometheus query failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result QueryResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("prometheus query returned error status: %s", result.Status)
	}

	return &result, nil
}

// QueryRange 执行范围查询
func (c *Client) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*QueryRangeResult, error) {
	params := url.Values{}
	params.Set("query", query)
	params.Set("start", start.Format(time.RFC3339))
	params.Set("end", end.Format(time.RFC3339))
	params.Set("step", step.String())

	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/api/v1/query_range?%s", c.baseURL, params.Encode()), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prometheus query failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result QueryRangeResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("prometheus query returned error status: %s", result.Status)
	}

	return &result, nil
}

// ParseValue 从查询结果中解析单个值
func ParseValue(result *QueryResult) (float64, error) {
	if result == nil || len(result.Data.Result) == 0 {
		return 0, fmt.Errorf("no data returned")
	}

	value := result.Data.Result[0].Value
	if len(value) < 2 {
		return 0, fmt.Errorf("invalid value format")
	}

	return parseFloat(value[1])
}

// ParseValues 从范围查询结果中解析多个值
func ParseValues(result *QueryRangeResult) ([]TimeSeriesPoint, error) {
	if result == nil || len(result.Data.Result) == 0 {
		return nil, fmt.Errorf("no data returned")
	}

	var points []TimeSeriesPoint
	for _, series := range result.Data.Result {
		for _, v := range series.Values {
			if len(v) < 2 {
				continue
			}
			timestamp, ok := v[0].(float64)
			if !ok {
				continue
			}
			value, err := parseFloat(v[1])
			if err != nil {
				continue
			}
			points = append(points, TimeSeriesPoint{
				Timestamp: int64(timestamp),
				Value:     value,
			})
		}
	}

	return points, nil
}

// TimeSeriesPoint 时间序列数据点
type TimeSeriesPoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

// parseFloat 解析浮点数
func parseFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case string:
		var f float64
		_, err := fmt.Sscanf(val, "%f", &f)
		if err != nil {
			return 0, fmt.Errorf("failed to parse value: %w", err)
		}
		return f, nil
	default:
		return 0, fmt.Errorf("unsupported value type: %T", v)
	}
}