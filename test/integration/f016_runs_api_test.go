package integration_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/api"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/engine/control"
	"test-auto-pro-v2/internal/engine/run"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/logging"
	"test-auto-pro-v2/internal/model"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
	"test-auto-pro-v2/internal/service"
)

// TestF016RunsAPIGuards 用真实 MySQL 按服务端装配路径组装运行主线 API：
// 未通过运行前检查的启动被 409 拒绝且中文原因与运行前检查同源；缺失运行 404 中文；列表端点可用。
func TestF016RunsAPIGuards(t *testing.T) {
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-016 API 集成测试缺少配置名：%v", missing)
	}
	cfg.Name = temporaryPlanDatabaseName(t)
	t.Cleanup(func() { dropTemporaryPlanDatabase(t, cfg) })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("临时计划数据库迁移失败：%v", err)
	}
	defer database.Close()

	planService := service.NewPlanService(planmysql.NewPlanRepository(database.DB))
	pathRepository := planmysql.NewExecutionPathRepository(database.DB)
	// 复用 F-015 集成测试的结构投影桩：拓扑复验只需要确定输入，不读真实目标。
	graphReader := f015StubGraphReader{}
	historyWorkspaceStore := planmysql.NewHistoryReplayRepository(database.DB)
	readiness := service.NewRunReadinessService(
		planService, pathRepository, graphReader, historyWorkspaceStore,
		analyzer.NewExecutionPathAnalyzer(), time.Now,
	)

	stubTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer stubTarget.Close()
	engineClient, err := target.NewClient(target.ClientConfig{BaseURL: stubTarget.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("构造引擎目标客户端失败：%v", err)
	}
	runConfig := config.RunConfig{
		LeaseDuration: time.Minute, ReadOnlyRetryAttempts: 2,
		ReadOnlyRetryBaseDelay: time.Millisecond, ReadOnlyRetryMaxDelay: 2 * time.Millisecond,
		StatusPollInterval: time.Second,
	}
	logRouter := logging.NewRouter(t.TempDir(), time.Now)
	runStore := planmysql.NewRunRepository(database.DB)
	runState := run.NewService(runStore, "worker-api-test", time.Minute, time.Now)
	executor := step.NewExecutor(engineClient, stubSessions{}, runState, runStore, runConfig, nil)
	controlService := control.NewService(runState, executor, runStore, time.Now)
	orchestrator := service.NewRunOrchestrationService(
		planService, pathRepository, graphReader, historyWorkspaceStore,
		readiness, controlService, runStore, logRouter, runConfig, time.Now,
	)
	handler := api.NewHandlerWithRunControl(http.NewServeMux(), orchestrator)

	// 建立真实计划与路径：未配置的路径必然被运行前检查阻塞。
	plan := createPathTestPlan(t, ctx, planService, "new", "123e4567-e89b-12d3-a456-4266141750f1")
	if _, created, err := pathRepository.Create(ctx, plan.ID, "123e4567-e89b-12d3-a456-4266141750f2", "路径 1",
		[]model.ExecutionPathChoice{{RouteNodeID: "route-a", BranchID: "branch-a"}}, time.Now().UTC()); err != nil || !created {
		t.Fatalf("路径创建失败：created=%v err=%v", created, err)
	}

	// FlowGraphService 依赖真实目标读取，这里为 nil 会 panic：详情/启动走不到图读取即被阻塞，安全。
	body := []byte(`{"executionPathId":1}`)
	request := httptest.NewRequest(http.MethodPost, "/api/plans/"+strconvUint(plan.ID)+"/runs", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("未通过运行前检查的启动应被 409 拒绝，实际 %d：%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "运行前检查未通过") {
		t.Fatalf("拒绝原因应与运行前检查同源：%s", recorder.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/runs/999999", nil)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound || !strings.Contains(recorder.Body.String(), "运行记录不存在") {
		t.Fatalf("缺失运行应 404 且给中文：%d %s", recorder.Code, recorder.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/plans/"+strconvUint(plan.ID)+"/runs", nil)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("运行列表应可读：%d %s", recorder.Code, recorder.Body.String())
	}
}

// stubSessions 是 API 测试里的会话面占位：流程到不了会话获取即被运行前检查挡住。
type stubSessions struct{}

// Current 不会被调用；被调用即视为装配错误。
func (stubSessions) Current(context.Context, string) (target.Session, error) {
	return target.Session{}, nil
}

// Refresh 不会被调用；被调用即视为装配错误。
func (stubSessions) Refresh(context.Context, string) (target.Session, error) {
	return target.Session{}, nil
}

// strconvUint 输出无符号整数文本。
func strconvUint(value uint64) string {
	return strconv.FormatUint(value, 10)
}
