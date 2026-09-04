package executor_test

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

	"test-auto-pro-v2/internal/engine/control"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/run"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/logging"
	"test-auto-pro-v2/internal/model"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
)

// newF016TargetStub 提供一个假目标 HTTP 服务：发起成功、实例已运行、无待办。
// 表单数据响应按原样返回，用于核对 curl.log 与实际请求逐字一致。
func newF016TargetStub(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/web/flowInstanceApi/submit":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"isSuccess": true,
				"data":      map[string]any{"id": "instance-66", "status": "run", "currentNodeProxyId": "node-audit"},
			})
		case "/web/flowInstanceApi/list":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"isSuccess": true,
				"data":      []map[string]any{{"id": "instance-66", "status": "run", "currentNodeProxyId": "node-audit"}},
			})
		case "/web/flowJobTaskLink/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"isSuccess": true, "data": []map[string]any{}})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"isSuccess": true, "data": map[string]any{}})
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// TestF016StepLogBidirectionalReachability 验证记录与日志双向可达（纲领第 7.3 节）：
// 尝试记录能落到具体日志行（相对路径+行号），日志行能查回运行记录（run_id/path_run_id/step_id），
// 尝试的 trace_id 与 curl.log 的关联键一致，写请求在日志里分类为 write。
func TestF016StepLogBidirectionalReachability(t *testing.T) {
	database := newF016ControlDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	store := planmysql.NewRunRepository(database.DB)
	runService := run.NewService(store, "worker-test", time.Minute, time.Now)

	logRoot := t.TempDir()
	router := logging.NewRouter(logRoot, time.Now)
	appLogger := logging.NewLogger(router, time.Now)

	server := newF016TargetStub(t)
	client, err := target.NewClient(target.ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("构造目标客户端失败：%v", err)
	}
	client.SetNetworkLogger(appLogger)

	executor := step.NewExecutor(client, &fakeSessions{}, runService, store, fixedRunConfig(), nil)
	executor.SetLogFactory(step.NewRouterStepLogFactory(router))

	runCtx := step.RunContext{
		Run:               model.Run{PlanID: 21},
		PathRun:           model.PathRun{ExecutionPathID: 22, Status: model.PathRunStatusRunning},
		PlanName:          "日志测试计划",
		PathName:          "日志路径",
		PlanAccount:       "oyg-test",
		FlowProxyID:       "flow-proxy-1",
		Source:            "new",
		GraphNodes:        []model.FlowGraphNode{{ID: "node-start", Name: "发起人", Type: "start"}},
		Steps:             []model.CompiledActionStep{submitStep()},
		EffectiveFormData: []byte(`{"amount":"12.30"}`),
	}

	// 控制服务启动并放行一步；请求上下文携带运行作用域，传输层日志按作用域落进运行目录。
	controller := control.NewService(runService, executor, store, time.Now)
	started, err := controller.Start(ctx, runCtx)
	if err != nil {
		t.Fatalf("启动失败：%v", err)
	}
	runScope := logging.Scope{
		PlanID:            strconv.FormatUint(started.Run.PlanID, 10),
		PlanName:          runCtx.PlanName,
		ExecutionPathID:   strconv.FormatUint(started.PathRun.ExecutionPathID, 10),
		ExecutionPathName: runCtx.PathName,
		RunID:             strconv.FormatUint(started.Run.ID, 10),
		RunSeq:            strconv.FormatUint(started.Run.RunNo, 10),
		PathRunID:         strconv.FormatUint(started.PathRun.ID, 10),
	}
	if _, err := controller.Approve(logging.WithScope(ctx, runScope), started.PathRun.ID); err != nil {
		t.Fatalf("放行失败：%v", err)
	}

	// 记录 -> 日志：尝试记录的相对日志路径与行号必须能打开并命中同一行。
	var attempt model.RunStepAttempt
	if err := database.DB.QueryRowContext(ctx,
		"SELECT path_run_id, verdict, trace_id, curl_trace_id, log_path, log_line FROM run_step_attempts WHERE path_run_id = ? ORDER BY id DESC LIMIT 1",
		started.PathRun.ID).Scan(&attempt.PathRunID, &attempt.Verdict, &attempt.TraceID, &attempt.CurlTraceID, &attempt.LogPath, &attempt.LogLine); err != nil {
		t.Fatalf("读取尝试记录失败：%v", err)
	}
	if attempt.TraceID == "" || attempt.CurlTraceID == "" || attempt.TraceID != attempt.CurlTraceID {
		t.Fatalf("尝试记录应携带一致的 trace 双键：%+v", attempt)
	}
	logPath := filepath.Join(logRoot, filepath.FromSlash(attempt.LogPath))
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("按尝试记录的相对日志路径打开 step.log 失败：%v", err)
	}
	lines := strings.Split(string(content), "\n")
	if int(attempt.LogLine) > len(lines) || attempt.LogLine < 1 {
		t.Fatalf("行号越界：line=%d total=%d", attempt.LogLine, len(lines))
	}
	settleLine := lines[attempt.LogLine-1]
	for _, required := range []string{
		"run_id=" + strconv.FormatUint(started.Run.ID, 10),
		"path_run_id=" + strconv.FormatUint(started.PathRun.ID, 10),
		"step_id=1", "phase=settle",
	} {
		if !strings.Contains(settleLine, required) {
			t.Fatalf("落账行缺少关联键 %s：%s", required, settleLine)
		}
	}

	// 日志 -> 记录：用日志行上的 path_run_id 必须能查回步骤与尝试事实。
	var stepCount int
	if err := database.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM run_steps WHERE path_run_id = ?", started.PathRun.ID).Scan(&stepCount); err != nil || stepCount != 1 {
		t.Fatalf("按日志行查回运行记录失败：count=%d err=%v", stepCount, err)
	}

	// curl.log：写请求必须以 write 分类记录，且与尝试记录的 trace_id 一致。
	curlPath := filepath.Join(filepath.Dir(logPath), "curl.log")
	curlContent, err := os.ReadFile(curlPath)
	if err != nil {
		t.Fatalf("读取 curl.log 失败：%v", err)
	}
	if !strings.Contains(string(curlContent), "trace_id="+attempt.TraceID) {
		t.Fatalf("curl.log 应包含尝试记录的 trace_id=%s", attempt.TraceID)
	}
	networkPath := filepath.Join(filepath.Dir(logPath), "network.log")
	networkContent, err := os.ReadFile(networkPath)
	if err != nil {
		t.Fatalf("读取 network.log 失败：%v", err)
	}
	if !strings.Contains(string(networkContent), "class=write") && !strings.Contains(string(networkContent), "request_class=write") {
		t.Fatalf("network.log 应把写请求分类为 write：%s", networkContent)
	}
}
