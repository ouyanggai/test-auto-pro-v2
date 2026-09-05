package target_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
)

// recordedRequest 是动态录制的一条请求事实：方法、路径与完整正文。
type recordedRequest struct {
	method string
	path   string
	body   string
}

// requestRecorder 是动态录制的目标假件：把打到它的每个请求原样记账（评审缺陷 19）。
type requestRecorder struct {
	mu       sync.Mutex
	requests []recordedRequest
}

// ServeHTTP 记账后返回通用成功包，业务语义不影响白名单断言。
func (r *requestRecorder) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	body := make([]byte, 0, 1024)
	buf := make([]byte, 4096)
	for {
		n, err := request.Body.Read(buf)
		body = append(body, buf[:n]...)
		if err != nil {
			break
		}
	}
	r.mu.Lock()
	r.requests = append(r.requests, recordedRequest{method: request.Method, path: request.URL.Path, body: string(body)})
	r.mu.Unlock()
	if request.URL.Path == "/web/flowJobTaskLink/list" {
		// 待办列表端点按数组解析 data；空数组表示无待办，探针语义仍成立。
		_, _ = w.Write([]byte(`{"isSuccess":true,"data":[]}`))
		return
	}
	_, _ = w.Write([]byte(`{"isSuccess":true,"data":{}}`))
}

// TestF016WriteEndpointsDynamicWhitelist 是写端点白名单的动态录制断言（静态 grep 之外的守卫）：
// 驱动真实客户端发出写请求，录制实际出网的请求事实，断言：
// ① POST 只落在白名单的两个写端点上；② 任何写请求正文都不携带 batchCode（幂等禁令）。
func TestF016WriteEndpointsDynamicWhitelist(t *testing.T) {
	recorder := &requestRecorder{}
	server := httptest.NewServer(recorder)
	defer server.Close()

	client := newF016Client(t, server.URL, 5*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	session := target.Session{SID: "sid", CompanyID: "company"}

	// 只读探针（允许的非写端点）+ 两个白名单写端点全部真实发出。
	if _, err := client.FindDueTaskID(ctx, session, "instance-1", "node-1"); err != nil {
		t.Fatalf("待办读取探针失败：%v", err)
	}
	if _, _, _, err := client.SubmitFlowInstance(ctx, session, target.SubmitFlowInstanceRequest{Name: "白名单", FlowProxyID: "proxy-1"}); err != nil {
		t.Fatalf("发起探针失败：%v", err)
	}
	if _, _, _, err := client.AuditCurrentTask(ctx, session, target.AuditCurrentTaskRequest{InstanceID: "i-1", JobTaskID: "t-1", AuditStatus: "pass"}); err != nil {
		t.Fatalf("同意探针失败：%v", err)
	}

	// 本次探针预期触达的全部端点：一个只读端点 + 白名单内的两个写端点。
	// 客户端任何新增写行为都会以未知路径出现在录制里，本断言即失败。
	allowedReads := map[string]bool{"/web/flowJobTaskLink/list": true}
	allowedWrites := map[string]bool{target.WriteEndpointSubmit: true, target.WriteEndpointAudit: true}

	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	writes := map[string]int{}
	for _, request := range recorder.requests {
		if request.method != http.MethodPost {
			continue
		}
		if strings.Contains(request.body, "batchCode") {
			t.Fatalf("写请求正文携带禁用的 batchCode：%s %s", request.path, request.body)
		}
		if allowedReads[request.path] {
			continue
		}
		if !allowedWrites[request.path] {
			t.Fatalf("出现了白名单之外的写端点：%s", request.path)
		}
		writes[request.path]++
	}
	if writes[target.WriteEndpointSubmit] != 1 || writes[target.WriteEndpointAudit] != 1 {
		t.Fatalf("两个写端点应各被真实命中一次，实际分布 %v", writes)
	}
}
