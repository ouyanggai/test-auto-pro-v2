package target_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/logging"
)

type networkRecordCollector struct {
	records []logging.NetworkRecord
}

// Network 收集目标网络日志，供大正文回归用例检查日志层没有破坏请求结果。
func (c *networkRecordCollector) Network(_ logging.Scope, record logging.NetworkRecord) {
	c.records = append(c.records, record)
}

// TestNetworkLoggerPreservesLargeRequestAndResponse 验证日志大小上限只作用于日志字段，
// 不截断真实请求正文，也不截断交给目标客户端解析的响应正文。
func TestNetworkLoggerPreservesLargeRequestAndResponse(t *testing.T) {
	largeValue := strings.Repeat("x", 2<<20)
	var receivedRequest []byte
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		var err error
		receivedRequest, err = io.ReadAll(request.Body)
		if err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		payload, _ := json.Marshal(map[string]any{
			"isSuccess": true,
			"data":      map[string]any{"id": "instance-large", "padding": largeValue},
		})
		_, _ = response.Write(payload)
	}))
	defer server.Close()

	client, err := target.NewClient(target.ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("创建目标客户端失败：%v", err)
	}
	collector := &networkRecordCollector{}
	client.SetNetworkLogger(collector)
	formData := json.RawMessage(`{"padding":"` + largeValue + `"}`)
	result, _, _, err := client.SubmitFlowInstance(context.Background(), target.Session{SID: "sid-large"}, target.SubmitFlowInstanceRequest{
		Name: "large-request", CompanyID: "company-large", BatchCode: "batch-large",
		FormData: formData,
	})
	if err != nil {
		t.Fatalf("大正文请求不应因日志层失败：%v", err)
	}
	if result == nil || result.InstanceID != "instance-large" {
		t.Fatalf("大响应未完整解析：%+v", result)
	}
	if len(receivedRequest) <= 1<<20 {
		t.Fatalf("真实请求正文被日志层截断：%d", len(receivedRequest))
	}
	if len(collector.records) != 1 {
		t.Fatalf("网络日志记录次数错误：%d", len(collector.records))
	}
	if len(collector.records[0].ResponseBody) >= len(largeValue) {
		t.Fatalf("日志正文未受大小上限约束：%d", len(collector.records[0].ResponseBody))
	}
}
