package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
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

// planBucketDir 返回某个计划在配置阶段的日志目录；pathDir 传 "_plan" 表示只知道计划的操作。
func planBucketDir(root, planDir, pathDir string) string {
	return filepath.Join(root, "plans", planDir, "configuration", pathDir, time.Now().Format("2006-01-02"))
}

// readLogFile 读取日志文件内容，缺失时返回空串而不是失败，
// 便于同一个用例同时断言"应该出现"和"不应该出现"。
func readLogFile(dir, name string) string {
	content, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return ""
	}
	return string(content)
}

// stubLogScopeResolver 用固定显示名模拟从真实业务记录解析归属的结果，
// 让集成用例可以只验证中间件的注入与落盘规则，不依赖真实数据库。
type stubLogScopeResolver struct{}

// ResolveLogScope 返回计划与执行路径的显示名；路径 ID 为 0 表示计划级操作。
func (stubLogScopeResolver) ResolveLogScope(_ context.Context, planID, pathID uint64) logging.Scope {
	scope := logging.Scope{PlanID: strconv.FormatUint(planID, 10), PlanName: "计划" + strconv.FormatUint(planID, 10)}
	if pathID != 0 {
		scope.ExecutionPathID = strconv.FormatUint(pathID, 10)
		scope.ExecutionPathName = "执行路径 " + strconv.FormatUint(pathID, 10)
	}
	return scope
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
	ctx := logging.WithScope(context.Background(), logging.Scope{
		RequestID: "req-unreachable", PlanID: "7", PlanName: "员工请假单（集团）",
		ExecutionPathID: "13", ExecutionPathName: "执行路径 1",
	})
	if _, err := client.Login(ctx, "account-a"); err == nil {
		t.Fatal("不可达目标地址必须返回错误")
	}
	// 目标请求即使由 HTTP 客户端底层发出，只要属于某个计划就必须落进该计划目录。
	bucket := planBucketDir(root, "员工请假单（集团）__plan-7", "执行路径 1__path-13")
	network := readLogFile(bucket, "network.log")
	failure := readLogFile(bucket, "network-error.log")
	curl := readLogFile(bucket, "curl.log")
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
	}), logger, stubLogScopeResolver{})
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
	// 流程图接口只带计划 ID，日志进该计划的计划级目录，不进所有计划共用的目录。
	bucket := planBucketDir(root, "计划1__plan-1", "_plan")
	content := readLogFile(bucket, "operation-error.log")
	if content == "" {
		t.Fatal("失败响应没有写入该计划的错误日志")
	}
	if !strings.Contains(content, "user_message="+logging.SanitizeValue(body.Error.Message)) {
		t.Fatalf("日志里的界面提示与响应体不一致：body=%s log=%s", body.Error.Message, content)
	}
	if !strings.Contains(content, "error_code=TARGET_TIMEOUT") || !strings.Contains(content, "error_class=network") {
		t.Fatalf("失败日志缺少稳定错误码或错误分类：%s", content)
	}
	if operation := readLogFile(bucket, "operation.log"); !strings.Contains(operation, "route=/api/plans/1/flow-graph") {
		t.Fatalf("请求日志没有记录路由：%s", operation)
	}
	if !strings.Contains(content, "plan_id=1") || !strings.Contains(content, "plan_name=计划1") {
		t.Fatalf("失败日志没有带上业务归属：%s", content)
	}
	// 已经归属到计划的业务异常不能改写进应用程序错误日志。
	applicationDir := filepath.Join(root, "application", time.Now().Format("2006-01-02"))
	for _, name := range []string{"application.log", "application-error.log"} {
		if leaked := readLogFile(applicationDir, name); strings.Contains(leaked, "/api/plans/1/flow-graph") {
			t.Fatalf("业务日志泄漏到 %s：%s", name, leaked)
		}
	}
}

// TestAPIPanicReturnsStableChineseErrorWithoutStack 断言 panic 被恢复为稳定中文 500，
// 栈只进日志不进响应体。
func TestAPIPanicReturnsStableChineseErrorWithoutStack(t *testing.T) {
	logger, _, root := loggingHarness(t)
	handler := api.WithRequestLogging(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("配置投影出现空指针")
	}), logger, stubLogScopeResolver{})
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
	content := readLogFile(planBucketDir(root, "计划1__plan-1", "_plan"), "operation-error.log")
	if !strings.Contains(content, "--- begin stack trace_id=") || !strings.Contains(content, "goroutine") {
		t.Fatalf("panic 栈没有写入程序错误日志：%s", content)
	}
	if !strings.Contains(content, "error_class=tool_bug") {
		t.Fatalf("panic 没有归类为工具缺陷：%s", content)
	}
}

// TestRequestScopeKeepsPlansAndPathsSeparated 分别访问两个不同计划与执行路径的配置接口，
// 断言日志各自落在自己的目录里不串目录，业务日志也不会只出现在共享目录或应用程序日志里。
func TestRequestScopeKeepsPlansAndPathsSeparated(t *testing.T) {
	logger, _, root := loggingHarness(t)
	handler := api.WithRequestLogging(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		// 业务处理器直接复用 context 里的作用域，等价于调用目标站点时携带同一份归属。
		logger.Info(logging.ScopeFrom(request.Context()), "读取路径配置")
		response.WriteHeader(http.StatusOK)
	}), logger, stubLogScopeResolver{})
	for _, route := range []string{
		"/api/plans/1/execution-paths/11/configuration",
		"/api/plans/2/execution-paths/22/configuration/data",
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, route, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("请求 %s 失败：%d", route, recorder.Code)
		}
	}
	firstDir := planBucketDir(root, "计划1__plan-1", "执行路径 11__path-11")
	secondDir := planBucketDir(root, "计划2__plan-2", "执行路径 22__path-22")
	first, second := readLogFile(firstDir, "operation.log"), readLogFile(secondDir, "operation.log")
	if first == "" || second == "" {
		t.Fatalf("两个计划的日志都必须落进各自目录：first=%d second=%d", len(first), len(second))
	}
	if strings.Contains(first, "plan_id=2") || strings.Contains(second, "plan_id=1") {
		t.Fatalf("两个计划的日志串目录了：\nfirst=%s\nsecond=%s", first, second)
	}
	if strings.Contains(first, "execution_path_id=22") || strings.Contains(second, "execution_path_id=11") {
		t.Fatalf("两条执行路径的日志串目录了：\nfirst=%s\nsecond=%s", first, second)
	}
	for _, dir := range []string{firstDir, secondDir} {
		if readLogFile(dir, "meta.json") == "" {
			t.Fatalf("目录 %s 缺少 meta.json", dir)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "config")); err == nil {
		t.Fatal("不应再出现所有计划共用的 logs/config 目录")
	}
	applicationDir := filepath.Join(root, "application", time.Now().Format("2006-01-02"))
	if leaked := readLogFile(applicationDir, "application.log"); strings.Contains(leaked, "读取路径配置") {
		t.Fatalf("业务日志重复写进了应用程序日志：%s", leaked)
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
