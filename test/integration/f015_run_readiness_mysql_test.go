package integration_test

import (
	"context"
	"testing"
	"time"

	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/model"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
	"test-auto-pro-v2/internal/service"
)

// f015StubGraphReader 只为拓扑复验提供一张固定的真实结构投影。
// 运行前检查对真实结构的用途只有拓扑复验一项，目标读取本身由 target 只读用例覆盖，
// 这里不重复伪装成"真实目标"，避免测试名与实际覆盖不一致。
type f015StubGraphReader struct{}

// Get 返回一张最小可用结构，保证拓扑复验有确定输入。
func (f015StubGraphReader) Get(context.Context, uint64) (model.FlowGraph, error) {
	return model.FlowGraph{EntryNodeIDs: []string{"start"}, Nodes: []model.FlowGraphNode{{ID: "start", Name: "发起人", Type: "start"}}}, nil
}

// newF015Readiness 用真实 MySQL 组装运行前检查服务，只把真实结构读取替换为固定投影。
func newF015Readiness(t *testing.T, database *planmysql.Database) *service.RunReadinessService {
	t.Helper()
	return service.NewRunReadinessService(
		service.NewPlanService(planmysql.NewPlanRepository(database.DB)),
		planmysql.NewExecutionPathRepository(database.DB),
		f015StubGraphReader{},
		planmysql.NewHistoryReplayRepository(database.DB),
		analyzer.NewExecutionPathAnalyzer(),
		time.Now,
	)
}

// TestF015ReadinessReadsRealDatabaseFacts 用真实 MySQL 验证运行前检查读的是数据库真实事实：
// 刚建立的路径没有节点配置也没有基础表单数据，必须被这两类阻塞挡住，不能判为可运行。
func TestF015ReadinessReadsRealDatabaseFacts(t *testing.T) {
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-015 MySQL 集成测试缺少配置名：%v", missing)
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

	plans := service.NewPlanService(planmysql.NewPlanRepository(database.DB))
	plan := createPathTestPlan(t, ctx, plans, "new", "123e4567-e89b-12d3-a456-4266141750f1")
	paths := planmysql.NewExecutionPathRepository(database.DB)
	if _, created, err := paths.Create(ctx, plan.ID, "123e4567-e89b-12d3-a456-4266141750f2", "路径 1",
		[]model.ExecutionPathChoice{{RouteNodeID: "route-a", BranchID: "branch-a"}}, time.Now().UTC()); err != nil || !created {
		t.Fatalf("路径创建失败：created=%v err=%v", created, err)
	}

	readiness, err := newF015Readiness(t, database).PlanReadiness(ctx, plan.ID, nil)
	if err != nil {
		t.Fatalf("运行前检查失败：%v", err)
	}
	if readiness.RunnableCount != 0 || readiness.BlockedCount != 1 || len(readiness.Paths) != 1 {
		t.Fatalf("未配置路径不得判为可运行：%+v", readiness)
	}
	kinds := map[string]bool{}
	for _, block := range readiness.Paths[0].Blocks {
		kinds[block.Kind] = true
		if block.Reason == "" {
			t.Fatalf("阻塞缺少中文原因：%+v", block)
		}
	}
	for _, required := range []string{model.RunReadinessNodeConfiguration, model.RunReadinessFormData} {
		if !kinds[required] {
			t.Fatalf("缺少必需的阻塞类别 %s：%+v", required, readiness.Paths[0].Blocks)
		}
	}
}

// TestF015ReadinessStorageFailureIsNotReportedAsMissingPlan 锁定人工复审的 P2：
// 存储故障必须给可重试结论，绝不能说成"计划不存在"——那是不可重试的说法，用户看到就不会再试。
func TestF015ReadinessStorageFailureIsNotReportedAsMissingPlan(t *testing.T) {
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-015 MySQL 集成测试缺少配置名：%v", missing)
	}
	cfg.Name = temporaryPlanDatabaseName(t)
	t.Cleanup(func() { dropTemporaryPlanDatabase(t, cfg) })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("临时计划数据库迁移失败：%v", err)
	}
	plans := service.NewPlanService(planmysql.NewPlanRepository(database.DB))
	plan := createPathTestPlan(t, ctx, plans, "new", "123e4567-e89b-12d3-a456-4266141750f3")
	readiness := newF015Readiness(t, database)

	// 先确认计划确实存在，再切断连接，这样后续失败只可能来自存储不可用。
	if _, err := readiness.PlanReadiness(ctx, plan.ID, nil); err != nil {
		t.Fatalf("连接正常时运行前检查不应失败：%v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("关闭数据库失败：%v", err)
	}

	_, err = readiness.PlanReadiness(ctx, plan.ID, nil)
	if err == nil {
		t.Fatal("存储不可用时必须报错，不能返回结论")
	}
	if service.IsRunReadinessErrorKind(err, service.RunReadinessErrorNotFound) {
		t.Fatalf("存储故障被误报为计划不存在：%v", err)
	}
	if !service.IsRunReadinessErrorKind(err, service.RunReadinessErrorStorage) {
		t.Fatalf("存储故障应归为存储类错误：%v", err)
	}
}

// TestF015ReadinessMissingPlanStillReportsNotFound 保证修复 P2 后真正不存在的计划仍然是 404 语义。
func TestF015ReadinessMissingPlanStillReportsNotFound(t *testing.T) {
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-015 MySQL 集成测试缺少配置名：%v", missing)
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

	_, err = newF015Readiness(t, database).PlanReadiness(ctx, 999999, nil)
	if !service.IsRunReadinessErrorKind(err, service.RunReadinessErrorNotFound) {
		t.Fatalf("不存在的计划应报计划不存在：%v", err)
	}
}
