package target_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/logging"
)

// bodyTimingRecorder 捕获唯一传输层的计时事实，不写实际日志文件。
type bodyTimingRecorder struct{ record logging.NetworkRecord }

// Network 记录最近一次目标传输事实，供测试核对正文慢传输也计入接口耗时。
func (r *bodyTimingRecorder) Network(_ logging.Scope, record logging.NetworkRecord) {
	r.record = record
}

// TestForwardTimingIncludesResponseBody 验证响应头很快但正文延迟时，接口耗时覆盖完整传输且正文原样回填。
func TestForwardTimingIncludesResponseBody(t *testing.T) {
	const original = `{"isSuccess":true,"data":{"id":"原始响应"}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		time.Sleep(80 * time.Millisecond)
		_, _ = io.WriteString(w, original)
	}))
	t.Cleanup(server.Close)
	client, err := target.NewClient(target.ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	recorder := &bodyTimingRecorder{}
	client.SetNetworkLogger(recorder)
	response, err := client.ForwardRuntimeRequest(context.Background(), target.Session{}, target.PageForwardRequest{Method: "POST", Path: "/web/formProxy/findById", RequestClass: "read", RequestSeq: 7, RetryAttempt: 1})
	if err != nil {
		t.Fatal(err)
	}
	if recorder.record.Duration < 60*time.Millisecond || recorder.record.RequestAt.IsZero() {
		t.Fatalf("计时不得在收到响应头时提前结束：%v", recorder.record.Duration)
	}
	if string(response.Body) != original || recorder.record.ResponseBody != original {
		t.Fatal("计时与日志不能改变响应正文")
	}
}
