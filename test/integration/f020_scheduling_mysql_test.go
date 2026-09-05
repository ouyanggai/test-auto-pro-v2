package integration_test

import (
	"context"
	"testing"
	"time"

	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
)

// f020Store 是 F-020 调度存储测试的真实 MySQL 仓储。
type f020Store struct {
	runs repository.RunStore
	db   *planmysql.Database
}

// openF020Store 建立临时计划数据库并完成全部迁移；用例之间互不共享数据。
func openF020Store(t *testing.T) f020Store {
	t.Helper()
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-020 MySQL 集成测试缺少配置名：%v", missing)
	}
	cfg.Name = temporaryPlanDatabaseName(t)
	t.Cleanup(func() { dropTemporaryPlanDatabase(t, cfg) })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("临时计划数据库迁移失败：%v", err)
	}
	t.Cleanup(func() { _ = database.DB.Close() })
	return f020Store{runs: planmysql.NewRunRepository(database.DB), db: database}
}

// f020MaxConcurrency 返回并发上限的指针形态。
func f020MaxConcurrency(value int) *int {
	return &value
}

// TestF020CreateRunWithPathsIdempotent 锁定启动幂等：同幂等键重试返回同一次运行与同一组路径运行，
// 绝不创建第二个运行或第二份路径实例；不同键才是新运行。
func TestF020CreateRunWithPathsIdempotent(t *testing.T) {
	store := openF020Store(t)
	ctx := context.Background()

	first, pathRuns, err := store.runs.CreateRunWithPaths(ctx, repository.CreateRunInput{
		PlanID: 1, ExecutionPathIDs: []uint64{11, 12, 13},
		Mode: model.RunModeAuto, Trigger: model.RunTriggerManual,
		MaxConcurrency: f020MaxConcurrency(2), IdempotencyKey: "launch-1", PresetBreakpoints: `[{"type":"first_write"}]`,
	})
	if err != nil {
		t.Fatalf("首次创建失败：%v", err)
	}
	if len(pathRuns) != 3 {
		t.Fatalf("应按勾选集合创建 3 条路径运行，实际 %d", len(pathRuns))
	}
	retried, retriedPathRuns, err := store.runs.CreateRunWithPaths(ctx, repository.CreateRunInput{
		PlanID: 1, ExecutionPathIDs: []uint64{11, 12, 13},
		Mode: model.RunModeAuto, Trigger: model.RunTriggerManual,
		MaxConcurrency: f020MaxConcurrency(2), IdempotencyKey: "launch-1",
	})
	if err != nil {
		t.Fatalf("幂等重试失败：%v", err)
	}
	if retried.ID != first.ID || len(retriedPathRuns) != 3 {
		t.Fatalf("同键重试必须返回同一次运行：%d vs %d，路径数 %d", retried.ID, first.ID, len(retriedPathRuns))
	}
	other, _, err := store.runs.CreateRunWithPaths(ctx, repository.CreateRunInput{
		PlanID: 1, ExecutionPathIDs: []uint64{11},
		Mode: model.RunModeSingleStep, Trigger: model.RunTriggerManual,
		MaxConcurrency: f020MaxConcurrency(1), IdempotencyKey: "launch-2",
	})
	if err != nil {
		t.Fatalf("不同键创建失败：%v", err)
	}
	if other.ID == first.ID {
		t.Fatal("不同幂等键必须创建新运行")
	}
}

// TestF020RunTerminalAggregatesAfterAllPathsFinish 锁定聚合收尾：
// 一条路径终态不收尾运行；全部路径终态后按 失败 > 已停止 > 已完成 的优先级聚合。
func TestF020RunTerminalAggregatesAfterAllPathsFinish(t *testing.T) {
	store := openF020Store(t)
	ctx := context.Background()

	runRow, pathRuns, err := store.runs.CreateRunWithPaths(ctx, repository.CreateRunInput{
		PlanID: 2, ExecutionPathIDs: []uint64{21, 22},
		Mode: model.RunModeAuto, Trigger: model.RunTriggerManual, MaxConcurrency: f020MaxConcurrency(1),
	})
	if err != nil {
		t.Fatalf("创建运行失败：%v", err)
	}
	if _, err := store.runs.AdvanceRunStatus(ctx, runRow.ID, model.RunStatusPending, model.RunStatusRunning,
		model.RunEvent{Kind: "run_started", Label: "测试"}, time.Now().UTC()); err != nil {
		t.Fatalf("推进运行失败：%v", err)
	}
	first, second := pathRuns[0], pathRuns[1]

	failed := model.RunResultFailed
	if _, err := store.runs.FinishPathRun(ctx, first.ID, model.PathRunStatusFailed, &failed, nil,
		model.RunEvent{Kind: "path_run_finished", Label: "第一步失败"}, time.Now().UTC()); err != nil {
		t.Fatalf("第一条路径置失败失败：%v", err)
	}
	afterFirst, err := store.runs.GetRun(ctx, runRow.ID)
	if err != nil {
		t.Fatalf("读取运行失败：%v", err)
	}
	if afterFirst.Status != model.RunStatusRunning {
		t.Fatalf("还有等待路径时运行必须保持运行中，实际 %s", model.RunStatusName(afterFirst.Status))
	}
	if _, err := store.runs.AdvancePathRunStatus(ctx, second.ID,
		model.PathRunStatusWaiting, model.PathRunStatusRunning,
		model.RunEvent{Kind: "path_run_started", Label: "调度启动第二条"}, time.Now().UTC()); err != nil {
		t.Fatalf("第二条路径应可被调度启动：%v", err)
	}
	if _, err := store.runs.FinishPathRun(ctx, second.ID, model.PathRunStatusStopped, nil, nil,
		model.RunEvent{Kind: "path_run_finished", Label: "第二条停止"}, time.Now().UTC()); err != nil {
		t.Fatalf("第二条路径置停止失败：%v", err)
	}
	afterAll, err := store.runs.GetRun(ctx, runRow.ID)
	if err != nil {
		t.Fatalf("读取运行失败：%v", err)
	}
	if afterAll.Status != model.RunStatusFailed {
		t.Fatalf("全部终态且存在失败时应聚合为失败，实际 %s", model.RunStatusName(afterAll.Status))
	}
}

// TestF020WaitingPathCanBeCancelledForRunStop 锁定运行级停止的存储语义：
// 未启动的等待路径可以直接置为已取消，聚合在全部路径终态后收尾。
func TestF020WaitingPathCanBeCancelledForRunStop(t *testing.T) {
	store := openF020Store(t)
	ctx := context.Background()

	runRow, pathRuns, err := store.runs.CreateRunWithPaths(ctx, repository.CreateRunInput{
		PlanID: 3, ExecutionPathIDs: []uint64{31, 32},
		Mode: model.RunModeSingleStep, Trigger: model.RunTriggerManual, MaxConcurrency: f020MaxConcurrency(1),
	})
	if err != nil {
		t.Fatalf("创建运行失败：%v", err)
	}
	if _, err := store.runs.AdvanceRunStatus(ctx, runRow.ID, model.RunStatusPending, model.RunStatusRunning,
		model.RunEvent{Kind: "run_started", Label: "测试"}, time.Now().UTC()); err != nil {
		t.Fatalf("推进运行失败：%v", err)
	}
	if _, err := store.runs.AdvancePathRunStatus(ctx, pathRuns[1].ID,
		model.PathRunStatusWaiting, model.PathRunStatusCancelled,
		model.RunEvent{Kind: "path_run_cancelled", Label: "运行停止时取消等待路径"}, time.Now().UTC()); err != nil {
		t.Fatalf("等待路径应可被取消：%v", err)
	}
	if _, err := store.runs.AdvancePathRunStatus(ctx, pathRuns[0].ID,
		model.PathRunStatusWaiting, model.PathRunStatusRunning,
		model.RunEvent{Kind: "path_run_started", Label: "开始"}, time.Now().UTC()); err != nil {
		t.Fatalf("等待路径应可启动：%v", err)
	}
	if _, err := store.runs.FinishPathRun(ctx, pathRuns[0].ID, model.PathRunStatusStopped, nil, nil,
		model.RunEvent{Kind: "path_run_finished", Label: "用户停止"}, time.Now().UTC()); err != nil {
		t.Fatalf("活动路径停止失败：%v", err)
	}
	final, err := store.runs.GetRun(ctx, runRow.ID)
	if err != nil {
		t.Fatalf("读取运行失败：%v", err)
	}
	if final.Status != model.RunStatusStopped {
		t.Fatalf("失败不存在时停止应聚合为已停止，实际 %s", model.RunStatusName(final.Status))
	}
}

// TestF020ManualConclusionClosesRunAggregation 锁定 F-018×F-020 交界：
// 待对账路径登记人工结论后即视为闭合；全部闭合时运行才收尾，未闭合时运行保持运行中。
func TestF020ManualConclusionClosesRunAggregation(t *testing.T) {
	store := openF020Store(t)
	ctx := context.Background()
	now := time.Now().UTC()

	runRow, pathRuns, err := store.runs.CreateRunWithPaths(ctx, repository.CreateRunInput{
		PlanID: 4, ExecutionPathIDs: []uint64{41, 42},
		Mode: model.RunModeSingleStep, Trigger: model.RunTriggerManual, MaxConcurrency: f020MaxConcurrency(1),
	})
	if err != nil {
		t.Fatalf("创建运行失败：%v", err)
	}
	if _, err := store.runs.AdvanceRunStatus(ctx, runRow.ID, model.RunStatusPending, model.RunStatusRunning,
		model.RunEvent{Kind: "run_started", Label: "测试"}, now); err != nil {
		t.Fatalf("推进运行失败：%v", err)
	}
	for _, pathRun := range pathRuns {
		if _, err := store.runs.AdvancePathRunStatus(ctx, pathRun.ID,
			model.PathRunStatusWaiting, model.PathRunStatusRunning,
			model.RunEvent{Kind: "path_run_started", Label: "开始"}, now); err != nil {
			t.Fatalf("启动路径失败：%v", err)
		}
		if _, err := store.runs.AdvancePathRunStatus(ctx, pathRun.ID,
			model.PathRunStatusRunning, model.PathRunStatusAwaitingReconciliation,
			model.RunEvent{Kind: "path_run_awaiting", Label: "写结果不确定"}, now); err != nil {
			t.Fatalf("置待对账失败：%v", err)
		}
	}
	// 只登记第一条路径的结论：另一条仍未闭合，运行不得收尾。
	if err := store.runs.AppendManualConclusion(ctx, model.RunManualConclusion{
		RunID: runRow.ID, PathRunID: pathRuns[0].ID,
		InstanceStatus: "none", CurrentNode: "无", Note: "验证", Reporter: "自动验证",
	}, now); err != nil {
		t.Fatalf("登记结论失败：%v", err)
	}
	closed, err := store.runs.FinishRunIfAllPathsClosed(ctx, runRow.ID, now)
	if err != nil {
		t.Fatalf("收尾检查失败：%v", err)
	}
	if closed {
		t.Fatal("还有未闭合路径时运行不得收尾")
	}
	if err := store.runs.AppendManualConclusion(ctx, model.RunManualConclusion{
		RunID: runRow.ID, PathRunID: pathRuns[1].ID,
		InstanceStatus: "none", CurrentNode: "无", Note: "验证", Reporter: "自动验证",
	}, now); err != nil {
		t.Fatalf("登记第二条结论失败：%v", err)
	}
	closed, err = store.runs.FinishRunIfAllPathsClosed(ctx, runRow.ID, now)
	if err != nil {
		t.Fatalf("收尾检查失败：%v", err)
	}
	if !closed {
		t.Fatal("全部闭合后运行应收尾")
	}
	final, err := store.runs.GetRun(ctx, runRow.ID)
	if err != nil {
		t.Fatalf("读取运行失败：%v", err)
	}
	if final.Status != model.RunStatusStopped {
		t.Fatalf("人工结论闭合的运行应聚合为已停止，实际 %s", model.RunStatusName(final.Status))
	}
}

// TestF021RunEventsIncrementalAndStatusFilter 锁定 F-021 增量语义：
// 事件按自增键升序且游标只取其后；状态筛选只返回匹配的运行。
func TestF021RunEventsIncrementalAndStatusFilter(t *testing.T) {
	store := openF020Store(t)
	ctx := context.Background()
	now := time.Now().UTC()

	runRow, _, err := store.runs.CreateRunWithPaths(ctx, repository.CreateRunInput{
		PlanID: 5, ExecutionPathIDs: []uint64{51},
		Mode: model.RunModeSingleStep, Trigger: model.RunTriggerManual, MaxConcurrency: f020MaxConcurrency(1),
	})
	if err != nil {
		t.Fatalf("创建运行失败：%v", err)
	}
	if _, err := store.runs.AdvanceRunStatus(ctx, runRow.ID, model.RunStatusPending, model.RunStatusRunning,
		model.RunEvent{Kind: "run_started", Label: "开始"}, now); err != nil {
		t.Fatalf("推进运行失败：%v", err)
	}
	all, err := store.runs.ListRunEvents(ctx, runRow.ID, 0, 100, 0)
	if err != nil {
		t.Fatalf("读取事件失败：%v", err)
	}
	if len(all) < 2 {
		t.Fatalf("至少应有创建与开始两条事件，实际 %d", len(all))
	}
	cursor := all[0].ID
	rest, err := store.runs.ListRunEvents(ctx, runRow.ID, cursor, 100, 0)
	if err != nil {
		t.Fatalf("增量读取失败：%v", err)
	}
	for _, event := range rest {
		if event.ID <= cursor {
			t.Fatalf("增量读取返回了游标之前的事件 %d", event.ID)
		}
	}
	filtered, err := store.runs.ListRunsFiltered(ctx, 5, string(model.RunStatusRunning), 0, 100)
	if err != nil || len(filtered) != 1 {
		t.Fatalf("状态筛选应返回运行中的 1 条：%v %v", filtered, err)
	}
	none, err := store.runs.ListRunsFiltered(ctx, 5, string(model.RunStatusCompleted), 0, 100)
	if err != nil || len(none) != 0 {
		t.Fatalf("已完成筛选应为空：%v %v", none, err)
	}
}

// TestF020ScheduledClaimConsumedOnce 锁定定时一次性消费：并发/重复领取只有一个赢家。
func TestF020ScheduledClaimConsumedOnce(t *testing.T) {
	store := openF020Store(t)
	ctx := context.Background()
	now := time.Now().UTC().Add(-time.Minute)

	if _, err := store.db.DB.ExecContext(ctx,
		"INSERT INTO test_plans (create_key, name, account, account_display_name, flow_source, target_object_id, target_object_name, run_mode, scheduled_at, status, created_at, updated_at) VALUES ('k-f020', '定时计划', 'a', '账号', 'new', 'obj', '流程', 'serial', ?, 'not_started', NOW(3), NOW(3))", now); err != nil {
		t.Fatalf("准备计划失败：%v", err)
	}
	due, err := store.runs.ListDueScheduledPlans(ctx, time.Now().UTC())
	if err != nil {
		t.Fatalf("扫描到点计划失败：%v", err)
	}
	if len(due) != 1 {
		t.Fatalf("应扫到 1 条到点计划，实际 %d", len(due))
	}
	first, err := store.runs.ClaimScheduledPlan(ctx, due[0].ID, time.Now().UTC())
	if err != nil || !first {
		t.Fatalf("首次领取应成功：%v %v", first, err)
	}
	second, err := store.runs.ClaimScheduledPlan(ctx, due[0].ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("重复领取出错：%v", err)
	}
	if second {
		t.Fatal("重复领取必须落空：定时启动只消费一次")
	}
	again, err := store.runs.ListDueScheduledPlans(ctx, time.Now().UTC())
	if err != nil {
		t.Fatalf("重复扫描失败：%v", err)
	}
	if len(again) != 0 {
		t.Fatalf("已消费的计划不得再被扫到：%d", len(again))
	}
}
