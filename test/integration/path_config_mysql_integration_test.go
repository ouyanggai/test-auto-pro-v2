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

// TestPathConfigurationMySQLMigrationAndCascade 验证 008/009 迁移、节点与表单分域存储、幂等与删除级联。
func TestPathConfigurationMySQLMigrationAndCascade(t *testing.T) {
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-007 本机计划数据库缺少配置名：%v", missing)
	}
	cfg.Name = temporaryPlanDatabaseName(t)
	t.Cleanup(func() { dropTemporaryPlanDatabase(t, cfg) })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	database, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("F-007 临时数据库迁移失败：%v", err)
	}
	defer database.Close()
	assertF007Tables(t, database.DB)

	plans := service.NewPlanService(planmysql.NewPlanRepository(database.DB))
	plan := createPathTestPlan(t, ctx, plans, "new", "123e4567-e89b-12d3-a456-426614174701")
	paths := planmysql.NewExecutionPathRepository(database.DB)
	firstPath, _, err := paths.Create(ctx, plan.ID, "123e4567-e89b-12d3-a456-426614174711", "配置路径一", []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "a"}}, time.Now().UTC())
	if err != nil {
		t.Fatalf("准备路径失败：%v", err)
	}
	secondPath, _, err := paths.Create(ctx, plan.ID, "123e4567-e89b-12d3-a456-426614174712", "配置路径二", []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "b"}}, time.Now().UTC())
	if err != nil {
		t.Fatalf("准备第二条路径失败：%v", err)
	}
	configs := planmysql.NewPathConfigurationRepository(database.DB)
	now := time.Now().UTC()
	firstRecord := model.StoredPathConfig{
		PathID: firstPath.ID, Revision: 1, NodeRevision: 1, FormRevision: 1,
		IdempotencyKey: "123e4567-e89b-12d3-a456-426614174721", Status: "configured", ConfigVersion: 2,
		FieldValues: map[string]map[string]string{"node-a": {"amount": "2500"}}, ActionValues: map[string]string{"node-a": "agree"},
		ConfirmedNodeKeys: []string{"node-token"}, FormValues: map[string]any{"amount": float64(2500), "__condition": "large"},
		FormStatus: "valid", FormValidated: true, FormSeed: 73,
		GeneratedFieldPaths: []string{"amount"}, ManualOverridePaths: []string{"note"},
		SampleSummary: model.PathFormSampleSummary{Saved: true, Recent: 2}, FormTemplateVersion: "template-v2",
	}
	saved, err := configs.Save(ctx, firstRecord, 0, now)
	if err != nil || saved.Revision != 1 {
		t.Fatalf("首次配置保存失败：saved=%+v err=%v", saved, err)
	}
	loaded, found, err := configs.FindByPath(ctx, firstPath.ID)
	if err != nil || !found || loaded.Revision != 1 || loaded.NodeRevision != 1 || loaded.FormRevision != 1 || loaded.FieldValues["node-a"]["amount"] != "2500" || loaded.ActionValues["node-a"] != "agree" {
		t.Fatalf("配置读取不正确：loaded=%+v found=%v err=%v", loaded, found, err)
	}
	if loaded.FormStatus != "valid" || !loaded.FormValidated || loaded.FormValues["amount"] != float64(2500) || loaded.SampleSummary.Recent != 2 || loaded.FormTemplateVersion != "template-v2" {
		t.Fatalf("表单 values 与生成元数据没有同事务恢复：%+v", loaded)
	}
	byKey, found, err := configs.FindByPathAndKey(ctx, firstPath.ID, "123e4567-e89b-12d3-a456-426614174721")
	if err != nil || !found || byKey.Revision != 1 {
		t.Fatalf("配置幂等记录无法读取：byKey=%+v found=%v err=%v", byKey, found, err)
	}
	if _, found, err := configs.FindByPathAndKey(ctx, secondPath.ID, "123e4567-e89b-12d3-a456-426614174721"); err != nil || found {
		t.Fatalf("配置幂等记录泄露到其他路径：found=%v err=%v", found, err)
	}
	if _, err := configs.Save(ctx, model.StoredPathConfig{PathID: firstPath.ID, Revision: 2, IdempotencyKey: "123e4567-e89b-12d3-a456-426614174722", Status: "configured", FieldValues: map[string]map[string]string{"node-a": {"amount": "3000"}}}, 0, now); !errors.Is(err, repository.ErrPathConfigConflict) {
		t.Fatalf("过期修订号没有被事务内拒绝：%v", err)
	}
	secondSave, err := configs.Save(ctx, model.StoredPathConfig{PathID: firstPath.ID, Revision: 2, IdempotencyKey: "123e4567-e89b-12d3-a456-426614174722", Status: "configured", FieldValues: map[string]map[string]string{"node-a": {"amount": "3000"}}, ActionValues: map[string]string{"node-a": "disagree"}}, 1, now)
	if err != nil || secondSave.Revision != 2 || secondSave.FieldValues["node-a"]["amount"] != "3000" {
		t.Fatalf("修订号推进保存失败：saved=%+v err=%v", secondSave, err)
	}
	other, err := configs.Save(ctx, model.StoredPathConfig{PathID: secondPath.ID, Revision: 1, IdempotencyKey: "123e4567-e89b-12d3-a456-426614174723", Status: "configured", FieldValues: map[string]map[string]string{"node-b": {"name": "\"乙\""}}}, 0, now)
	if err != nil || other.Revision != 1 || other.FieldValues["node-b"]["name"] == "" {
		t.Fatalf("第二条路径配置保存失败：saved=%+v err=%v", other, err)
	}
	firstReload, _, err := configs.FindByPath(ctx, firstPath.ID)
	if err != nil || firstReload.FieldValues["node-a"]["amount"] != "3000" || firstReload.ActionValues["node-a"] != "disagree" {
		t.Fatalf("路径配置互相覆盖：%+v err=%v", firstReload, err)
	}

	if err := database.Close(); err != nil {
		t.Fatal("关闭首次连接池失败")
	}
	reopened, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("F-007 重复迁移或重连失败：%v", err)
	}
	defer reopened.Close()
	reloaded, found, err := planmysql.NewPathConfigurationRepository(reopened.DB).FindByPath(ctx, firstPath.ID)
	if err != nil || !found || reloaded.Revision != 2 {
		t.Fatalf("重启后配置没有恢复：reloaded=%+v found=%v err=%v", reloaded, found, err)
	}
	if err := planmysql.NewExecutionPathRepository(reopened.DB).Delete(ctx, plan.ID, firstPath.ID, time.Now().UTC()); err != nil {
		t.Fatalf("删除路径失败：%v", err)
	}
	var remaining int
	if err := reopened.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_execution_path_configs WHERE path_id = ?", firstPath.ID).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("删除路径没有级联清理配置：count=%d err=%v", remaining, err)
	}
}

// TestPathConfigurationMySQLConcurrentSameKeyFirstSave 验证同一幂等键并发首次保存只产生一条配置且修订号一致。
func TestPathConfigurationMySQLConcurrentSameKeyFirstSave(t *testing.T) {
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-007 并发测试缺少本机计划数据库配置名：%v", missing)
	}
	cfg.Name = temporaryPlanDatabaseName(t)
	t.Cleanup(func() { dropTemporaryPlanDatabase(t, cfg) })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	database, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("F-007 并发测试临时数据库迁移失败：%v", err)
	}
	defer database.Close()
	plans := service.NewPlanService(planmysql.NewPlanRepository(database.DB))
	plan := createPathTestPlan(t, ctx, plans, "new", "123e4567-e89b-12d3-a456-426614174709")
	paths := planmysql.NewExecutionPathRepository(database.DB)
	path, _, err := paths.Create(ctx, plan.ID, "123e4567-e89b-12d3-a456-426614174719", "并发配置路径", []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "a"}}, time.Now().UTC())
	if err != nil {
		t.Fatalf("准备并发测试路径失败：%v", err)
	}
	configs := planmysql.NewPathConfigurationRepository(database.DB)
	key := "123e4567-e89b-12d3-a456-426614174729"
	record := model.StoredPathConfig{
		PathID: path.ID, Revision: 1, IdempotencyKey: key, Status: "configured",
		FieldValues:  map[string]map[string]string{"node-a": {"amount": "2500"}},
		ActionValues: map[string]string{"node-a": "agree"},
	}
	type concurrentSaveResult struct {
		record model.StoredPathConfig
		err    error
	}
	start := make(chan struct{})
	results := make(chan concurrentSaveResult, 2)
	for index := 0; index < 2; index++ {
		go func() {
			<-start
			saved, saveErr := configs.Save(ctx, record, 0, time.Now().UTC())
			results <- concurrentSaveResult{record: saved, err: saveErr}
		}()
	}
	// 两个同键请求同时越过服务层幂等检查时，仓储必须在唯一键冲突后返回同一胜出记录，而不是存储错误。
	close(start)
	left, right := <-results, <-results
	if left.err != nil || right.err != nil || left.record.Revision != 1 || right.record.Revision != 1 || left.record.IdempotencyKey != key || right.record.IdempotencyKey != key {
		t.Fatalf("并发同键首次保存没有收敛为同一结果：left=%+v right=%+v", left, right)
	}
	var count int
	if err := database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_execution_path_configs WHERE path_id = ?", path.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("并发同键首次保存产生重复配置：count=%d err=%v", count, err)
	}
}

// assertF007Tables 精确核对 F-007 临时库表与迁移版本数量。
func assertF007Tables(t *testing.T, db *sql.DB) {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'test_execution_path_configs'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("F-007 临时库缺少配置表：count=%d err=%v", count, err)
	}
	var migrations int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&migrations); err != nil || migrations != 10 {
		t.Fatalf("F-007 迁移版本数量不正确：%d err=%v", migrations, err)
	}
	var columns int
	if err := db.QueryRow("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'test_execution_path_configs' AND column_name IN ('node_revision','form_revision','form_values','form_status','generated_field_paths','manual_override_paths')").Scan(&columns); err != nil || columns != 6 {
		t.Fatalf("F-007 节点与表单分域列不完整：count=%d err=%v", columns, err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'test_form_runtime_sync_jobs'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("F-007 临时库缺少表单运行时维护任务表：count=%d err=%v", count, err)
	}
}
