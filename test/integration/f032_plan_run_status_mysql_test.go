package integration_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/model"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
)

// F-032 计划状态与最近运行结果修复：计划公开状态由 runs 运行事实派生（读侧自愈），
// 运行事务内同步计划存储列（写侧锁定判断一致）；最近运行结果按「第 N 次：结果」口径返回。

// openF032Store 建立临时计划数据库并完成全部迁移；用例之间互不共享数据。
func openF032Store(t *testing.T) *planmysql.Database {
	t.Helper()
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-032 MySQL 集成测试缺少配置名：%v", missing)
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
	return database
}

// insertF032Plan 直接写入一条未运行计划，返回计划 ID。
func insertF032Plan(t *testing.T, db *sql.DB, name string) uint64 {
	t.Helper()
	result, err := db.ExecContext(context.Background(),
		"INSERT INTO test_plans (create_key, name, account, account_display_name, flow_source, target_object_id, target_object_name, run_mode, status, created_at, updated_at) VALUES (?, ?, 'a', '账号', 'new', 'obj', '流程', 'serial', 'not_started', NOW(3), NOW(3))",
		"k-"+name, name)
	if err != nil {
		t.Fatalf("写入计划失败：%v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("读取计划主键失败：%v", err)
	}
	return uint64(id)
}

// TestF032PlanStatusFollowsRunFacts 验证计划状态随运行事实流转且永不回退：
// 未运行 -> 创建运行即运行中 -> 全部路径闭合后已运行 -> 再次运行重新运行中 -> 已运行。
func TestF032PlanStatusFollowsRunFacts(t *testing.T) {
	db := openF032Store(t)
	ctx := context.Background()
	runs := planmysql.NewRunRepository(db.DB)
	plans := planmysql.NewPlanRepository(db.DB)
	planID := insertF032Plan(t, db.DB, "f032-lifecycle")

	// 未运行：没有任何运行记录。
	plan, err := plans.Get(ctx, planID)
	if err != nil || plan.Status != model.PlanStatusNotStarted {
		t.Fatalf("新计划应为未运行：status=%s err=%v", plan.Status, err)
	}

	// 创建运行：状态立即变为运行中（等待运行也算活跃运行事实）。
	run, pathRun, err := runs.CreateRun(ctx, planID, 11, model.RunModeAuto, model.RunTriggerManual, nil, time.Now().UTC())
	if err != nil {
		t.Fatalf("创建运行失败：%v", err)
	}
	if _, err := runs.AdvanceRunStatus(ctx, run.ID, model.RunStatusPending, model.RunStatusRunning,
		model.RunEvent{Kind: "run_started", Label: "测试启动"}, time.Now().UTC()); err != nil {
		t.Fatalf("推进运行聚合失败：%v", err)
	}
	plan, err = plans.Get(ctx, planID)
	if err != nil || plan.Status != model.PlanStatusRunning || !plan.HasActiveRun {
		t.Fatalf("创建运行后计划应为运行中：status=%s active=%v err=%v", plan.Status, plan.HasActiveRun, err)
	}
	if plan.LastRunNo == nil || *plan.LastRunNo != 1 {
		t.Fatalf("最近运行号应为 1：%v", plan.LastRunNo)
	}

	// 路径运行成功闭合：计划变为已运行，最近运行结果为成功。
	if _, err := runs.AdvancePathRunStatus(ctx, pathRun.ID, model.PathRunStatusWaiting, model.PathRunStatusRunning,
		model.RunEvent{Kind: "path_run_started", Label: "测试启动"}, time.Now().UTC()); err != nil {
		t.Fatalf("推进路径运行失败：%v", err)
	}
	succeeded := model.RunResultSucceeded
	_, err = runs.FinishPathRun(ctx, pathRun.ID, model.PathRunStatusCompleted, &succeeded, nil,
		model.RunEvent{Kind: "path_run_finished", Label: "测试收尾"}, time.Now().UTC())
	if err != nil {
		t.Fatalf("路径运行收尾失败：%v", err)
	}
	closed, err := runs.FinishRunIfAllPathsClosed(ctx, run.ID, time.Now().UTC())
	if err != nil || !closed {
		t.Fatalf("运行应收尾：closed=%v err=%v", closed, err)
	}
	plan, err = plans.Get(ctx, planID)
	if err != nil || plan.Status != model.PlanStatusCompleted || plan.HasActiveRun {
		t.Fatalf("运行结束后计划应为已运行：status=%s active=%v err=%v", plan.Status, plan.HasActiveRun, err)
	}
	if plan.LastRunStatus != model.RunStatusCompleted || plan.LastRunResult == nil || *plan.LastRunResult != model.RunResultSucceeded {
		t.Fatalf("最近运行应为成功：status=%s result=%v", plan.LastRunStatus, plan.LastRunResult)
	}
	if plan.LastRunFinishedAt == nil {
		t.Fatal("最近运行结束时间不应为空")
	}

	// 再次运行：重新变为运行中，运行号递增；结束回到已运行且永不回到未运行。
	run2, pathRun2, err := runs.CreateRun(ctx, planID, 11, model.RunModeAuto, model.RunTriggerManual, nil, time.Now().UTC())
	if err != nil {
		t.Fatalf("再次运行创建失败：%v", err)
	}
	if _, err := runs.AdvanceRunStatus(ctx, run2.ID, model.RunStatusPending, model.RunStatusRunning,
		model.RunEvent{Kind: "run_started", Label: "测试启动"}, time.Now().UTC()); err != nil {
		t.Fatalf("推进运行聚合失败：%v", err)
	}
	plan, err = plans.Get(ctx, planID)
	if err != nil || plan.Status != model.PlanStatusRunning {
		t.Fatalf("再次运行后计划应为运行中：status=%s err=%v", plan.Status, err)
	}
	if plan.LastRunNo == nil || *plan.LastRunNo != 2 {
		t.Fatalf("最近运行号应为 2：%v", plan.LastRunNo)
	}
	if _, err := runs.AdvancePathRunStatus(ctx, pathRun2.ID, model.PathRunStatusWaiting, model.PathRunStatusRunning,
		model.RunEvent{Kind: "path_run_started", Label: "测试启动"}, time.Now().UTC()); err != nil {
		t.Fatalf("推进第二次路径运行失败：%v", err)
	}
	_, err = runs.FinishPathRun(ctx, pathRun2.ID, model.PathRunStatusFailed, func() *model.RunResult { v := model.RunResultFailed; return &v }(), nil,
		model.RunEvent{Kind: "path_run_finished", Label: "测试收尾"}, time.Now().UTC())
	if err != nil {
		t.Fatalf("第二次路径收尾失败：%v", err)
	}
	if _, err := runs.FinishRunIfAllPathsClosed(ctx, run2.ID, time.Now().UTC()); err != nil {
		t.Fatalf("第二次运行收尾失败：%v", err)
	}
	plan, err = plans.Get(ctx, planID)
	if err != nil || plan.Status != model.PlanStatusCompleted {
		t.Fatalf("第二次运行结束后计划应为已运行：status=%s err=%v", plan.Status, err)
	}
	if plan.LastRunStatus != model.RunStatusFailed {
		t.Fatalf("最近运行应为失败：status=%s", plan.LastRunStatus)
	}
}

// TestF032PlanLockUsesSyncedStatus 验证运行后计划存储列已同步，路径配置锁定随之生效。
func TestF032PlanLockUsesSyncedStatus(t *testing.T) {
	db := openF032Store(t)
	ctx := context.Background()
	runs := planmysql.NewRunRepository(db.DB)
	planID := insertF032Plan(t, db.DB, "f032-lock")

	// 创建运行后存储列应为运行中，此时新增路径配置被锁定拒绝。
	run, _, err := runs.CreateRun(ctx, planID, 11, model.RunModeAuto, model.RunTriggerManual, nil, time.Now().UTC())
	if err != nil {
		t.Fatalf("创建运行失败：%v", err)
	}
	if _, err := runs.AdvanceRunStatus(ctx, run.ID, model.RunStatusPending, model.RunStatusRunning,
		model.RunEvent{Kind: "run_started", Label: "测试启动"}, time.Now().UTC()); err != nil {
		t.Fatalf("推进运行聚合失败：%v", err)
	}
	var storedStatus string
	if err := db.DB.QueryRowContext(ctx, "SELECT status FROM test_plans WHERE id = ?", planID).Scan(&storedStatus); err != nil || storedStatus != string(model.PlanStatusRunning) {
		t.Fatalf("存储列应为 running：status=%s err=%v", storedStatus, err)
	}
	paths := planmysql.NewExecutionPathRepository(db.DB)
	if _, _, err := paths.Create(ctx, planID, "k-f032-new-path", "锁定路径", nil, time.Now().UTC()); err == nil {
		t.Fatal("运行中计划应拒绝新增路径配置")
	}

	// 收尾后存储列应为已运行（锁定持续生效），读侧派生状态一致。
	run, pathRun, err := runs.CreateRun(ctx, planID, 11, model.RunModeAuto, model.RunTriggerManual, nil, time.Now().UTC())
	if err != nil {
		t.Fatalf("第二次创建运行失败：%v", err)
	}
	if _, err := runs.AdvanceRunStatus(ctx, run.ID, model.RunStatusPending, model.RunStatusRunning,
		model.RunEvent{Kind: "run_started", Label: "测试启动"}, time.Now().UTC()); err != nil {
		t.Fatalf("推进运行聚合失败：%v", err)
	}
	if _, err := runs.AdvancePathRunStatus(ctx, pathRun.ID, model.PathRunStatusWaiting, model.PathRunStatusRunning,
		model.RunEvent{Kind: "path_run_started", Label: "测试启动"}, time.Now().UTC()); err != nil {
		t.Fatalf("推进路径运行失败：%v", err)
	}
	succeeded := model.RunResultSucceeded
	if _, err := runs.FinishPathRun(ctx, pathRun.ID, model.PathRunStatusCompleted, &succeeded, nil,
		model.RunEvent{Kind: "path_run_finished", Label: "测试收尾"}, time.Now().UTC()); err != nil {
		t.Fatalf("路径收尾失败：%v", err)
	}
	if _, err := runs.FinishRunIfAllPathsClosed(ctx, run.ID, time.Now().UTC()); err != nil {
		t.Fatalf("运行收尾失败：%v", err)
	}
	if err := db.DB.QueryRowContext(ctx, "SELECT status FROM test_plans WHERE id = ?", planID).Scan(&storedStatus); err != nil || storedStatus != string(model.PlanStatusCompleted) {
		t.Fatalf("存储列应为 completed：status=%s err=%v", storedStatus, err)
	}
	if _, _, err := paths.Create(ctx, planID, "k-f032-new-path-2", "锁定路径", nil, time.Now().UTC()); err == nil {
		t.Fatal("已运行计划应拒绝新增路径配置")
	}
}

// TestF032PlanListAggregatesLastRun 验证计划列表一次 SQL 聚合最近运行事实，无 N+1 查询。
func TestF032PlanListAggregatesLastRun(t *testing.T) {
	db := openF032Store(t)
	ctx := context.Background()
	runs := planmysql.NewRunRepository(db.DB)
	plans := planmysql.NewPlanRepository(db.DB)
	neverRan := insertF032Plan(t, db.DB, "f032-list-never")
	ranOnce := insertF032Plan(t, db.DB, "f032-list-ran")

	list, err := plans.List(ctx, model.PlanListFilter{Limit: 200})
	if err != nil || len(list) != 2 {
		t.Fatalf("列表应返回两条计划：n=%d err=%v", len(list), err)
	}
	byID := map[uint64]model.Plan{}
	for _, plan := range list {
		byID[plan.ID] = plan
	}
	if item := byID[neverRan]; item.LastRunNo != nil || item.Status != model.PlanStatusNotStarted {
		t.Fatalf("未运行计划不应有运行摘要：%+v", item)
	}

	run, pathRun, err := runs.CreateRun(ctx, ranOnce, 21, model.RunModeAuto, model.RunTriggerManual, nil, time.Now().UTC())
	if err != nil {
		t.Fatalf("创建运行失败：%v", err)
	}
	if _, err := runs.AdvanceRunStatus(ctx, run.ID, model.RunStatusPending, model.RunStatusRunning,
		model.RunEvent{Kind: "run_started", Label: "测试启动"}, time.Now().UTC()); err != nil {
		t.Fatalf("推进运行聚合失败：%v", err)
	}
	if _, err := runs.AdvancePathRunStatus(ctx, pathRun.ID, model.PathRunStatusWaiting, model.PathRunStatusRunning,
		model.RunEvent{Kind: "path_run_started", Label: "测试启动"}, time.Now().UTC()); err != nil {
		t.Fatalf("推进路径运行失败：%v", err)
	}
	succeeded := model.RunResultSucceeded
	if _, err := runs.FinishPathRun(ctx, pathRun.ID, model.PathRunStatusCompleted, &succeeded, nil,
		model.RunEvent{Kind: "path_run_finished", Label: "测试收尾"}, time.Now().UTC()); err != nil {
		t.Fatalf("路径收尾失败：%v", err)
	}
	if _, err := runs.FinishRunIfAllPathsClosed(ctx, run.ID, time.Now().UTC()); err != nil {
		t.Fatalf("运行收尾失败：%v", err)
	}

	list, err = plans.List(ctx, model.PlanListFilter{Name: "f032-list", Limit: 200})
	if err != nil || len(list) != 2 {
		t.Fatalf("名称筛选应返回两条计划：%d err=%v", len(list), err)
	}
	byID = map[uint64]model.Plan{}
	for _, plan := range list {
		byID[plan.ID] = plan
	}
	item := byID[ranOnce]
	if item.Status != model.PlanStatusCompleted || item.LastRunNo == nil || *item.LastRunNo != 1 ||
		item.LastRunStatus != model.RunStatusCompleted || item.LastRunResult == nil ||
		*item.LastRunResult != model.RunResultSucceeded || item.HasActiveRun {
		t.Fatalf("已运行计划聚合摘要不正确：%+v", item)
	}
	if item := byID[neverRan]; item.LastRunNo != nil {
		t.Fatalf("未运行计划运行摘要应为空：%+v", item)
	}

	// 状态筛选使用派生状态：只筛已运行时未运行计划不出现。
	filtered, err := plans.List(ctx, model.PlanListFilter{Status: model.PlanStatusCompleted, Limit: 200})
	if err != nil || len(filtered) != 1 || filtered[0].ID != ranOnce {
		t.Fatalf("按已运行筛选结果不正确：n=%d err=%v", len(filtered), err)
	}
}
