package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/api"
	"test-auto-pro-v2/internal/logging"
)

// loggingHarness 把日志根指向临时目录，返回目录路由与日志器，避免污染仓库 logs/。
func loggingHarness(t *testing.T) (*logging.Logger, *logging.Router, string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv(logging.LogRootEnv, root)
	if resolved := logging.Root("/should-not-be-used"); resolved != root {
		t.Fatalf("日志根没有按环境变量覆盖：%s", resolved)
	}
	router := logging.NewRouter(root, time.Now)
	return logging.NewLogger(router, time.Now), router, root
}

// readConfigBucket 读取当天配置桶内的日志文件内容，缺失时返回空串而不是失败，
// 便于同一个用例同时断言"应该出现"和"不应该出现"。
func readConfigBucket(t *testing.T, root, name string) string {
	t.Helper()
	path := filepath.Join(root, "config", time.Now().Format("2006-01-02"), name)
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(content)
}

// TestTargetRequestLoggingRecordsFailureAndReplayableCurl 用不可达目标地址触发一次真实失败，
// 断言网络日志分流、trace_id 与 curl_trace_id 双向可查，且命令与实际请求逐字一致。
func TestTargetRequestLoggingRecordsFailureAndReplayableCurl(t *testing.T) {
	logger, _, root := loggingHarness(t)
	// 127.0.0.1:1 是保留端口，本机必然连不上，用它稳定触发传输层失败。
	client, err := target.NewClient(target.ClientConfig{
		BaseURL: "http://127.0.0.1:1", LoginPassword: "password", LoginAESKey: "0123456789abcdef",
		LoginCode: "code", PlatformCode: "invest", CustomerCode: "customer", Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("创建目标客户端失败：%v", err)
	}
	client.SetNetworkLogger(logger)
	ctx := logging.WithScope(context.Background(), logging.Scope{RequestID: "req-unreachable"})
	if _, err := client.Login(ctx, "account-a"); err == nil {
		t.Fatal("不可达目标地址必须返回错误")
	}
	network := readConfigBucket(t, root, "network.log")
	failure := readConfigBucket(t, root, "network-error.log")
	curl := readConfigBucket(t, root, "curl.log")
	if network == "" || failure == "" || curl == "" {
		t.Fatalf("三个网络日志文件都必须落盘：network=%d error=%d curl=%d", len(network), len(failure), len(curl))
	}
	traceID := fieldValue(failure, "trace_id")
	if traceID == "" || traceID == "-" {
		t.Fatalf("失败记录缺少 trace_id：%s", failure)
	}
	if !strings.Contains(network, "trace_id="+traceID) {
		t.Fatalf("network.log 里查不到同一次请求：%s", network)
	}
	if !strings.Contains(curl, "--- begin curl trace_id="+traceID+" ---") {
		t.Fatalf("curl.log 里查不到同一次请求：%s", curl)
	}
	if !strings.Contains(failure, "request_id=req-unreachable") {
		t.Fatalf("失败记录没有带上 context 作用域：%s", failure)
	}
	if !strings.Contains(failure, "result=failure") || fieldValue(failure, "error_type") == "-" {
		t.Fatalf("失败记录缺少结果或错误分类：%s", failure)
	}
	if !strings.Contains(curl, "curl -sS -X POST 'http://127.0.0.1:1/web/user/api/login/user/login") {
		t.Fatalf("curl 命令与实际请求不一致：%s", curl)
	}
	if !strings.Contains(curl, "--data-raw ") {
		t.Fatalf("curl 命令缺少实际请求正文：%s", curl)
	}
	// 本切片写端点白名单为空：所有请求都必须记为只读。
	for _, line := range strings.Split(strings.TrimSpace(network), "\n") {
		if strings.Contains(line, "request_class=") && !strings.Contains(line, "request_class=read") {
			t.Fatalf("出现了非只读的目标请求：%s", line)
		}
	}
}

// TestAPIFailureLogMatchesResponseMessage 断言程序错误日志里的 user_message 与响应体中的
// error.message 完全一致，用户报"页面提示 XXX"时可以直接 grep 到那一行。
func TestAPIFailureLogMatchesResponseMessage(t *testing.T) {
	logger, _, root := loggingHarness(t)
	const message = "读取流程超时，请重试"
	handler := api.WithRequestLogging(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json; charset=utf-8")
		response.WriteHeader(http.StatusGatewayTimeout)
		_ = json.NewEncoder(response).Encode(map[string]any{
			"success": false,
			"error":   map[string]any{"code": "TARGET_TIMEOUT", "message": message, "retryable": true},
		})
	}), logger)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/plans/1/flow-graph", nil))
	if recorder.Code != http.StatusGatewayTimeout {
		t.Fatalf("失败响应状态码被改写：%d", recorder.Code)
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应体不是统一失败包络：%v", err)
	}
	content := readConfigBucket(t, root, "program-error.log")
	if content == "" {
		t.Fatal("失败响应没有写入程序错误日志")
	}
	if !strings.Contains(content, "user_message="+logging.SanitizeValue(body.Error.Message)) {
		t.Fatalf("日志里的界面提示与响应体不一致：body=%s log=%s", body.Error.Message, content)
	}
	if !strings.Contains(content, "error_code=TARGET_TIMEOUT") || !strings.Contains(content, "error_class=network") {
		t.Fatalf("失败日志缺少稳定错误码或错误分类：%s", content)
	}
	if program := readConfigBucket(t, root, "program.log"); !strings.Contains(program, "route=/api/plans/1/flow-graph") {
		t.Fatalf("请求日志没有记录路由：%s", program)
	}
}

// TestAPIPanicReturnsStableChineseErrorWithoutStack 断言 panic 被恢复为稳定中文 500，
// 栈只进日志不进响应体。
func TestAPIPanicReturnsStableChineseErrorWithoutStack(t *testing.T) {
	logger, _, root := loggingHarness(t)
	handler := api.WithRequestLogging(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("配置投影出现空指针")
	}), logger)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/plans/1/execution-paths", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("panic 没有收敛为 500：%d", recorder.Code)
	}
	body := recorder.Body.String()
	if strings.Contains(body, "goroutine") || strings.Contains(body, ".go:") {
		t.Fatalf("响应体泄漏了调用栈：%s", body)
	}
	if !strings.Contains(body, "工具内部发生错误") {
		t.Fatalf("panic 响应缺少稳定中文提示：%s", body)
	}
	content := readConfigBucket(t, root, "program-error.log")
	if !strings.Contains(content, "--- begin stack trace_id=") || !strings.Contains(content, "goroutine") {
		t.Fatalf("panic 栈没有写入程序错误日志：%s", content)
	}
	if !strings.Contains(content, "error_class=tool_bug") {
		t.Fatalf("panic 没有归类为工具缺陷：%s", content)
	}
}

// fieldValue 从日志内容第一行里取出指定字段值，便于跨文件核对同一次请求。
func fieldValue(content, key string) string {
	line := strings.TrimSpace(strings.Split(strings.TrimSpace(content), "\n")[0])
	for _, token := range strings.Fields(line) {
		if strings.HasPrefix(token, key+"=") {
			return strings.TrimPrefix(token, key+"=")
		}
	}
	return ""
}
