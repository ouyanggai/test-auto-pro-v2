package integration_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/engine/control"
	"test-auto-pro-v2/internal/engine/run"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/logging"
	"test-auto-pro-v2/internal/model"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
	"test-auto-pro-v2/internal/service"
)

// TestF016RunControlResolvesRunIDToPathRun 复核评审缺陷 6 的修复回归：
// 控制端点以运行 ID 寻址，服务层必须解析成该运行自己的路径运行 ID。
// 两个自增序列错位时（本用例给第一次运行补了一条额外路径运行），
// 旧实现把运行 ID 直接当路径运行 ID 用，控制动作会作用到另一条路径运行上。
func TestF016RunControlResolvesRunIDToPathRun(t *testing.T) {
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-016 控制寻址集成测试缺少配置名：%v", missing)
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
	runStore := planmysql.NewRunRepository(database.DB)
	runState := run.NewService(runStore, "worker-ids-test", time.Minute, time.Now)
	executor := step.NewExecutor(engineClient, stubSessions{}, runState, runStore, runConfig, nil)
	controlService := control.NewService(runState, executor, runStore, time.Now)
	orchestrator := service.NewRunOrchestrationService(
		planService, pathRepository, graphReader, historyWorkspaceStore,
		readiness, controlService, runStore, logging.NewRouter(t.TempDir(), time.Now), runConfig, nil, time.Now,
	)

	// 第一次运行（run 1 / path_run 1），再补一条同运行下的额外路径运行（path_run 2）制造序列错位。
	firstRun, _, err := runStore.CreateRun(ctx, 1, 101, model.RunModeSingleStep, model.RunTriggerManual, nil, time.Now())
	if err != nil {
		t.Fatalf("首次运行创建失败：%v", err)
	}
	if _, err := database.DB.ExecContext(ctx,
		`INSERT INTO path_runs (run_id, execution_path_id, status, created_at, updated_at) VALUES (?, 102, 'waiting', ?, ?)`,
		firstRun.ID, time.Now().UTC(), time.Now().UTC()); err != nil {
		t.Fatalf("补充路径运行失败：%v", err)
	}
	// 第二次运行：run 2 / path_run 3。旧缺陷下 ListBreakpoints(run 2) 会按 path_run 2 回放事实。
	secondRun, secondPathRun, err := runStore.CreateRun(ctx, 1, 103, model.RunModeSingleStep, model.RunTriggerManual, nil, time.Now())
	if err != nil {
		t.Fatalf("第二次运行创建失败：%v", err)
	}
	if secondRun.ID == secondPathRun.ID {
		t.Fatalf("本用例前提是两个序列错位：run_id=%d path_run_id=%d", secondRun.ID, secondPathRun.ID)
	}

	// 给第二次运行自己的路径运行预置一个步骤断点事实（运行前预置不经会话，直接落事实）。
	own := model.RunControl{
		RunID: secondRun.ID, PathRunID: secondPathRun.ID, Kind: model.ControlFactBreakpointSet,
		BreakpointType: model.BreakpointStep, ObjectKind: "step", ObjectKey: "2",
		Source: model.RunControlSourceUI, CreatedAt: time.Now().UTC(),
	}
	if err := runStore.AppendRunControl(ctx, own, time.Now()); err != nil {
		t.Fatalf("预置断点事实失败：%v", err)
	}
	// 给错位目标（第一次运行的额外路径运行）预置一个可区分的断点事实。
	other := model.RunControl{
		RunID: firstRun.ID, PathRunID: secondPathRun.ID - 1, Kind: model.ControlFactBreakpointSet,
		BreakpointType: model.BreakpointStep, ObjectKind: "step", ObjectKey: "9",
		Source: model.RunControlSourceUI, CreatedAt: time.Now().UTC(),
	}
	if err := runStore.AppendRunControl(ctx, other, time.Now()); err != nil {
		t.Fatalf("预置错位断点事实失败：%v", err)
	}

	breakpoints, err := orchestrator.ListBreakpoints(ctx, secondRun.ID)
	if err != nil {
		t.Fatalf("按运行 ID 读取断点失败：%v", err)
	}
	// 回放结果 = 两个默认断点（路径偏离强制开、首次写默认开）+ 运行自己的预置断点（步骤 2）。
	if len(breakpoints) != 3 {
		t.Fatalf("断点回放应为两个默认断点加本运行预置断点，实际 %+v", breakpoints)
	}
	for _, bp := range breakpoints {
		if bp.Type == model.BreakpointStep && bp.StepNo != 2 {
			// 步骤 9 的断点事实挂在错位目标的路径运行上，绝不能出现在这里。
			t.Fatalf("控制端点解析到了别的路径运行的断点：%+v", breakpoints)
		}
	}
	foundOwn := false
	for _, bp := range breakpoints {
		if bp.Type == model.BreakpointStep && bp.StepNo == 2 {
			foundOwn = true
		}
	}
	if !foundOwn {
		t.Fatalf("未读到运行自己路径运行上的预置断点：%+v", breakpoints)
	}
}
