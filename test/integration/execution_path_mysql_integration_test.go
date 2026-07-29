package integration_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
	"test-auto-pro-v2/internal/service"
)

// TestExecutionPathMySQLConfiguredDatabaseHasF005Schema 验证本机独立开发库已安全应用 F-005 向前迁移。
func TestExecutionPathMySQLConfiguredDatabaseHasF005Schema(t *testing.T) {
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-005 本机计划数据库缺少配置名：%v", missing)
	}
	if cfg.Name != "test_auto_pro_v2" {
		t.Fatal("F-005 拒绝在批准范围外的数据库核对开发迁移")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	database, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("F-005 本机独立计划数据库迁移失败：%v", err)
	}
	defer database.Close()
	assertF005Tables(t, database.DB)
}

// TestExecutionPathMySQLMigrationTransactionsAndCounts 验证真实迁移、事务、幂等、计数和重连读取。
func TestExecutionPathMySQLMigrationTransactionsAndCounts(t *testing.T) {
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-005 MySQL 集成测试缺少配置名：%v", missing)
	}
	cfg.Name = temporaryPlanDatabaseName(t)
	t.Cleanup(func() { dropTemporaryPlanDatabase(t, cfg) })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	database, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("F-005 临时数据库迁移失败：%v", err)
	}
	defer database.Close()
	assertF005Tables(t, database.DB)

	plans := service.NewPlanService(planmysql.NewPlanRepository(database.DB))
	newPlan := createPathTestPlan(t, ctx, plans, "new", "123e4567-e89b-12d3-a456-426614174101")
	startedPlan := createPathTestPlan(t, ctx, plans, "started", "123e4567-e89b-12d3-a456-426614174102")
	paths := planmysql.NewExecutionPathRepository(database.DB)
	firstChoices := []model.ExecutionPathChoice{{RouteNodeID: "route-a", BranchID: "branch-a"}}
	first, created, err := paths.Create(ctx, newPlan.ID, "123e4567-e89b-12d3-a456-426614174201", "  财务重点路径  ", firstChoices, time.Now().UTC())
	if err != nil || !created || first.SequenceNo != 1 {
		t.Fatalf("首次路径创建失败：created=%v path=%+v err=%v", created, first, err)
	}
	if first.Name != "财务重点路径" {
		t.Fatalf("自定义路径名称未持久化：%q", first.Name)
	}
	found, exists, err := paths.FindByCreateKey(ctx, newPlan.ID, "123e4567-e89b-12d3-a456-426614174201")
	if err != nil || !exists || found.ID != first.ID || len(found.Choices) != 1 {
		t.Fatalf("计划内幂等记录无法直接读取：found=%+v exists=%v err=%v", found, exists, err)
	}
	if leaked, exists, err := paths.FindByCreateKey(ctx, startedPlan.ID, "123e4567-e89b-12d3-a456-426614174201"); err != nil || exists || leaked.ID != 0 {
		t.Fatalf("幂等记录泄露到其他计划：found=%+v exists=%v err=%v", leaked, exists, err)
	}
	retried, created, err := paths.Create(ctx, newPlan.ID, "123e4567-e89b-12d3-a456-426614174201", "重试名称不能覆盖", firstChoices, time.Now().UTC())
	if err != nil || created || retried.ID != first.ID {
		t.Fatal("相同创建键没有返回同一路径")
	}
	second, created, err := paths.Create(ctx, newPlan.ID, "123e4567-e89b-12d3-a456-426614174202", "", firstChoices, time.Now().UTC())
	if err != nil || !created || second.SequenceNo != 2 {
		t.Fatal("新发起计划没有分配稳定递增序号")
	}
	if second.Name != "路径 2" {
		t.Fatalf("默认名称没有使用实际稳定序号：%q", second.Name)
	}
	updatedChoices := []model.ExecutionPathChoice{{RouteNodeID: "route-a", BranchID: "branch-b"}}
	updated, err := paths.Update(ctx, newPlan.ID, first.ID, "改名后路径", updatedChoices, time.Now().UTC())
	if err != nil || updated.SequenceNo != 1 || updated.Name != "改名后路径" || updated.Choices[0].BranchID != "branch-b" {
		t.Fatal("路径原位更新没有保留序号或替换选择")
	}
	if _, err := paths.Update(ctx, startedPlan.ID, first.ID, "跨计划", updatedChoices, time.Now().UTC()); !errors.Is(err, repository.ErrExecutionPathNotFound) {
		t.Fatalf("跨计划路径更新没有被归属校验拒绝：%v", err)
	}
	storedPlan, err := plans.Get(ctx, newPlan.ID)
	if err != nil || storedPlan.PathCount != 2 || storedPlan.Status != model.PlanStatusPendingConfiguration {
		t.Fatal("计划详情没有返回真实路径数量或状态被提前改变")
	}
	if err := paths.Delete(ctx, newPlan.ID, second.ID, time.Now().UTC()); err != nil {
		t.Fatalf("删除路径失败：%v", err)
	}
	var deletedChoices int
	if err := database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_execution_path_choices WHERE path_id = ?", second.ID).Scan(&deletedChoices); err != nil || deletedChoices != 0 {
		t.Fatalf("删除路径没有级联清理选择：count=%d err=%v", deletedChoices, err)
	}
	remaining, err := paths.List(ctx, newPlan.ID)
	if err != nil || len(remaining) != 1 || remaining[0].ID != first.ID || remaining[0].SequenceNo != 1 {
		t.Fatal("删除路径后错误重排了剩余序号")
	}
	third, created, err := paths.Create(ctx, newPlan.ID, "123e4567-e89b-12d3-a456-426614174205", "", firstChoices, time.Now().UTC())
	if err != nil || !created || third.SequenceNo != 3 {
		t.Fatalf("删除最高序号后复用了历史编号：created=%v path=%+v err=%v", created, third, err)
	}

	concurrentPlan := createPathTestPlan(t, ctx, plans, "new", "123e4567-e89b-12d3-a456-426614174103")
	type concurrentCreateResult struct {
		path    model.ExecutionPath
		created bool
		err     error
	}
	start := make(chan struct{})
	results := make(chan concurrentCreateResult, 2)
	for index := 0; index < 2; index++ {
		go func() {
			<-start
			path, created, err := paths.Create(ctx, concurrentPlan.ID, "123e4567-e89b-12d3-a456-426614174206", "并发幂等路径", firstChoices, time.Now().UTC())
			results <- concurrentCreateResult{path: path, created: created, err: err}
		}()
	}
	// 两次同键请求同时越过客户端边界时，计划行锁与事务内幂等检查必须只允许一个真实写入。
	close(start)
	left, right := <-results, <-results
	if left.err != nil || right.err != nil || left.path.ID == 0 || left.path.ID != right.path.ID || left.created == right.created {
		t.Fatalf("并发同键创建没有收敛为一条路径：left=%+v right=%+v", left, right)
	}
	var concurrentCount int
	if err := database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_execution_paths WHERE plan_id = ?", concurrentPlan.ID).Scan(&concurrentCount); err != nil || concurrentCount != 1 {
		t.Fatalf("并发同键创建产生重复路径：count=%d err=%v", concurrentCount, err)
	}

	started, _, err := paths.Create(ctx, startedPlan.ID, "123e4567-e89b-12d3-a456-426614174203", "", nil, time.Now().UTC())
	if err != nil || started.ID == 0 {
		t.Fatalf("已发计划首条路径创建失败：%v", err)
	}
	_, _, err = paths.Create(ctx, startedPlan.ID, "123e4567-e89b-12d3-a456-426614174204", "", nil, time.Now().UTC())
	if !errors.Is(err, repository.ErrExecutionPathLimit) {
		t.Fatalf("已发计划没有限制为一条路径：%v", err)
	}

	if err := database.Close(); err != nil {
		t.Fatal("关闭首次连接池失败")
	}
	reopened, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("F-005 重复迁移或重连失败：%v", err)
	}
	defer reopened.Close()
	reloaded, err := planmysql.NewExecutionPathRepository(reopened.DB).List(ctx, newPlan.ID)
	if err != nil || len(reloaded) != 2 || reloaded[0].SequenceNo != 1 || reloaded[1].SequenceNo != 3 {
		t.Fatal("重启语义下没有读取到相同路径")
	}
}

// TestExecutionPathMySQLGenerateAllIsPersistentAndAtomic 验证批量重复过滤、连续序号、持久幂等和失败零写入。
func TestExecutionPathMySQLGenerateAllIsPersistentAndAtomic(t *testing.T) {
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-005 MySQL 集成测试缺少配置名：%v", missing)
	}
	cfg.Name = temporaryPlanDatabaseName(t)
	t.Cleanup(func() { dropTemporaryPlanDatabase(t, cfg) })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	database, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("F-005 临时数据库迁移失败：%v", err)
	}
	defer database.Close()
	plans := service.NewPlanService(planmysql.NewPlanRepository(database.DB))
	paths := planmysql.NewExecutionPathRepository(database.DB)
	plan := createPathTestPlan(t, ctx, plans, "new", "123e4567-e89b-12d3-a456-426614174111")
	choiceA := []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "a"}}
	choiceB := []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "b"}}
	if _, _, err := paths.Create(ctx, plan.ID, "123e4567-e89b-12d3-a456-426614174211", "", choiceA, time.Now().UTC()); err != nil {
		t.Fatalf("准备已存在路径失败：%v", err)
	}
	key := "123e4567-e89b-12d3-a456-426614174299"
	result, created, err := paths.GenerateAll(ctx, plan.ID, key, [][]model.ExecutionPathChoice{choiceA, choiceB}, time.Now().UTC())
	if err != nil || !created || result.TotalCount != 2 || result.ExistingCount != 1 || result.CreatedCount != 1 || len(result.Paths) != 1 {
		t.Fatalf("批量重复过滤不正确：result=%+v created=%v err=%v", result, created, err)
	}
	if result.Paths[0].SequenceNo != 2 || result.Paths[0].Name != "路径 2" {
		t.Fatalf("批量路径序号或默认名不正确：%+v", result.Paths[0])
	}
	retried, created, err := paths.GenerateAll(ctx, plan.ID, key, [][]model.ExecutionPathChoice{{}}, time.Now().UTC())
	if err != nil || created || retried.CreatedCount != 1 || len(retried.Paths) != 1 || retried.Paths[0].ID != result.Paths[0].ID {
		t.Fatalf("持久批量幂等结果不稳定：result=%+v created=%v err=%v", retried, created, err)
	}
	found, exists, err := paths.FindBatchByCreateKey(ctx, plan.ID, key)
	if err != nil || !exists || found.Paths[0].ID != result.Paths[0].ID {
		t.Fatalf("批次无法在目标读取前恢复：result=%+v exists=%v err=%v", found, exists, err)
	}

	failurePlan := createPathTestPlan(t, ctx, plans, "new", "123e4567-e89b-12d3-a456-426614174112")
	invalid := []model.ExecutionPathChoice{{RouteNodeID: "same", BranchID: "a"}, {RouteNodeID: "same", BranchID: "b"}}
	_, _, err = paths.GenerateAll(ctx, failurePlan.ID, "123e4567-e89b-12d3-a456-426614174298", [][]model.ExecutionPathChoice{choiceA, invalid}, time.Now().UTC())
	if err == nil {
		t.Fatal("批量中途写入错误没有回滚")
	}
	remaining, listErr := paths.List(ctx, failurePlan.ID)
	if listErr != nil || len(remaining) != 0 {
		t.Fatalf("批量失败留下部分路径：paths=%+v err=%v", remaining, listErr)
	}
	var batches int
	if err := database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_execution_path_batches WHERE plan_id = ?", failurePlan.ID).Scan(&batches); err != nil || batches != 0 {
		t.Fatalf("批量失败留下幂等批次：count=%d err=%v", batches, err)
	}
}

// createPathTestPlan 在随机临时库中创建指定来源的待配置计划。
func createPathTestPlan(t *testing.T, ctx context.Context, plans *service.PlanService, source, key string) model.Plan {
	t.Helper()
	plan, created, err := plans.Create(ctx, key, service.CreatePlanInput{
		Name: "路径集成计划", Account: "integration-account", FlowSource: source,
		TargetObjectID: "target-id", TargetObjectName: "集成流程", RunMode: "serial",
	})
	if err != nil || !created {
		t.Fatalf("准备 %s 计划失败：created=%v err=%v", source, created, err)
	}
	return plan
}

// assertF005Tables 精确核对 F-005 临时库表、迁移和内部序号计数器。
func assertF005Tables(t *testing.T, db *sql.DB) {
	t.Helper()
	want := map[string]bool{
		"schema_migrations": true, "test_plans": true,
		"test_execution_paths": true, "test_execution_path_choices": true,
		"test_execution_path_batches": true, "test_execution_path_batch_items": true,
	}
	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		t.Fatal("无法核对 F-005 表")
	}
	defer rows.Close()
	found := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal("无法读取 F-005 表名")
		}
		found[name] = true
	}
	if len(found) != len(want) {
		t.Fatalf("F-005 临时库表集合不正确：%v", found)
	}
	for name := range want {
		if !found[name] {
			t.Fatalf("F-005 临时库缺少表 %s", name)
		}
	}
	var migrations int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&migrations); err != nil || migrations != 7 {
		t.Fatalf("F-005 迁移版本数量不正确：%d err=%v", migrations, err)
	}
	var counterColumns int
	if err := db.QueryRow(`
SELECT COUNT(*) FROM information_schema.columns
WHERE table_schema = DATABASE() AND table_name = 'test_plans' AND column_name = 'next_path_sequence_no'`).Scan(&counterColumns); err != nil || counterColumns != 1 {
		t.Fatalf("F-005 稳定序号计数器不可用：count=%d err=%v", counterColumns, err)
	}
	var nameColumns int
	if err := db.QueryRow(`
SELECT COUNT(*) FROM information_schema.columns
WHERE table_schema = DATABASE() AND table_name = 'test_execution_paths' AND column_name = 'name'`).Scan(&nameColumns); err != nil || nameColumns != 1 {
		t.Fatalf("F-005 路径名称列不可用：count=%d err=%v", nameColumns, err)
	}
}
