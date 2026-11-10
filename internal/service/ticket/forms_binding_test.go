package ticket

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// 验证 gin 绑定 json.RawMessage 时批量 form 的 items 完整保留
// （问题排查：审批详情显示空白，先实证后端数据链路）
func TestSubmitRequestBindingPreservesItems(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var captured string
	r.POST("/tickets", func(c *gin.Context) {
		var req SubmitRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			t.Fatalf("bind failed: %v", err)
		}
		captured = string(req.Form)
		c.Status(http.StatusOK)
	})

	body := `{"type":"topic_create","cluster_id":1,"form":{"items":[` +
		`{"topic_name":"trade.ods.order-created","partitions":6,"replication_factor":3,"retention_days":3,"business_desc":"订单事件流"},` +
		`{"topic_name":"trade.ods.order-paid","partitions":6,"replication_factor":3,"retention_days":0,"business_desc":"支付事件流"}]}}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tickets", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// 模拟存储与读回：Form 原样存 text，前端 JSON.parse 后应拿到 items
	var stored map[string]json.RawMessage
	if err := json.Unmarshal([]byte(captured), &stored); err != nil {
		t.Fatalf("stored form is not valid JSON: %v\nform=%s", err, captured)
	}
	rawItems, ok := stored["items"]
	if !ok {
		t.Fatalf("items key missing in stored form: %s", captured)
	}
	var items []map[string]any
	if err := json.Unmarshal(rawItems, &items); err != nil {
		t.Fatalf("items is not an array: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0]["topic_name"] != "trade.ods.order-created" || items[1]["business_desc"] != "支付事件流" {
		t.Fatalf("item content corrupted: %+v", items)
	}

	// 再验证 Parse + IsBatch
	form, err := ParseTopicCreateForm(captured)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !form.IsBatch() || len(form.Items) != 2 {
		t.Fatalf("IsBatch=%v items=%d", form.IsBatch(), len(form.Items))
	}
}

// 验证规范化 Marshal 往返：Parse 后重新 Marshal 的 JSON 与原始语义一致
func TestBatchFormNormalizedMarshal(t *testing.T) {
	raw := `{"items":[{"topic_name":"a.b","partitions":6,"replication_factor":3,"retention_days":3,"business_desc":"测试"}]}`
	form, err := ParseTopicCreateForm(raw)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	out, err := json.Marshal(form)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	round, err := ParseTopicCreateForm(string(out))
	if err != nil {
		t.Fatalf("re-parse failed: %v\njson=%s", err, out)
	}
	if !round.IsBatch() || round.Items[0].TopicName != "a.b" || round.Items[0].BusinessDesc != "测试" {
		t.Fatalf("round trip corrupted: %s", out)
	}
}
