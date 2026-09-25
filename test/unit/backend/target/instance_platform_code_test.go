package target_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
)

// TestFindSubmittedFlowFactsCapturesPlatformCode 锁定按实例精确读取事实时必须带回实例平台码：
// 双平台（200001/999999）下，实例重建类装载据此让运行时页面按实例真实平台发目标请求，
// 修复默认 200001 查 999999 实例查空、详情弹窗不打开、装载 60s 超时的问题（欧阳改测试09 实测）。
func TestFindSubmittedFlowFactsCapturesPlatformCode(t *testing.T) {
	const (
		instanceID   = "0d7f17a2ebb24fbdbbe4579cf0b4182a"
		customerCode = "d4036cc6581141a8b13cf39387c8d6f2"
	)
	// 目标 flowInstanceApi/list 返回该实例，平台码为 999999，代理非空以满足唯一命中判据。
	body := `{"isSuccess":true,"data":[{"id":"` + instanceID +
		`","name":"欧阳改测试09-路径_1","flowProxyId":"proxy-1","formProxyId":"form-1",` +
		`"status":"draft","currentNodeProxyId":"node-1","platformCode":"999999",` +
		`"flowInstanceBizRelevanceList":[]}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/web/flowInstanceApi/list") {
			_, _ = w.Write([]byte(body))
			return
		}
		_, _ = w.Write([]byte(`{"isSuccess":true,"data":[]}`))
	}))
	defer server.Close()

	client, err := target.NewClient(target.ClientConfig{
		BaseURL:      server.URL,
		CustomerCode: customerCode,
		PlatformCode: "200001",
		Timeout:      5 * time.Second,
	})
	if err != nil {
		t.Fatalf("构造目标客户端失败：%v", err)
	}

	facts, err := client.FindSubmittedFlowFacts(context.Background(), target.Session{
		SID: "session-a", CustomerCode: customerCode, PlatformCode: "200001",
	}, instanceID)
	if err != nil {
		t.Fatalf("读取实例事实失败：%v", err)
	}
	if !facts.Found {
		t.Fatalf("应命中目标实例：%+v", facts)
	}
	if facts.PlatformCode != "999999" {
		t.Fatalf("实例事实必须带回实例平台码 999999，实际 %q", facts.PlatformCode)
	}
}
