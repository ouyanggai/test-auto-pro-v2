package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/model"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
	"test-auto-pro-v2/internal/service"
)

// TestF009PathPreparationMySQLCheckpoints 验证独立任务幂等、单活动任务、取消恢复、分页和真实计数。
func TestF009PathPreparationMySQLCheckpoints(t *testing.T) {
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-009 MySQL 集成测试缺少配置名：%v", missing)
	}
	cfg.Name = temporaryPlanDatabaseName(t)
	t.Cleanup(func() { dropTemporaryPlanDatabase(t, cfg) })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("F-009 临时数据库迁移失败：%v", err)
	}
	defer database.Close()
	plans := service.NewPlanService(planmysql.NewPlanRepository(database.DB))
	plan := createPathTestPlan(t, ctx, plans, "new", "123e4567-e89b-12d3-a456-426614174701")
	paths := planmysql.NewExecutionPathRepository(database.DB)
	configs := planmysql.NewPathConfigurationRepository(database.DB)
	created := make([]model.ExecutionPath, 0, 3)
	for index := 0; index < 3; index++ {
		path, _, createErr := paths.Create(ctx, plan.ID, []string{
			"123e4567-e89b-12d3-a456-426614174711", "123e4567-e89b-12d3-a456-426614174712", "123e4567-e89b-12d3-a456-426614174713",
		}[index], "", []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch"}}, time.Now().UTC())
		if createErr != nil {
			t.Fatalf("建立任务路径失败：%v", createErr)
		}
		created = append(created, path)
		actions := map[string]string{}
		if index < 2 {
			actions["f008:test-included"] = "true"
		}
		_, saveErr := configs.Save(ctx, model.StoredPathConfig{
			PathID: path.ID, Revision: 1, NodeRevision: 1, Status: "pending", ConfigVersion: 4,
			FieldValues: map[string]map[string]string{}, ActionValues: actions, ConfirmedNodeKeys: []string{},
			FormValues: map[string]any{}, FormStatus: "empty", DataStatus: "not_generated",
			GeneratedFieldPaths: []string{}, ManualOverridePaths: []string{},
		}, 0, time.Now().UTC())
		if saveErr != nil {
			t.Fatalf("保存路径勾选事实失败：%v", saveErr)
		}
	}
	repository := planmysql.NewPathPreparationRepository(database.DB)
	job, createdJob, err := repository.Create(ctx, plan.ID, "123e4567-e89b-12d3-a456-426614174720", time.Now().UTC())
	if err != nil || !createdJob || job.Total != 2 {
		t.Fatalf("任务没有只快照两条勾选路径：created=%v job=%+v err=%v", createdJob, job, err)
	}
	retried, createdJob, err := repository.Create(ctx, plan.ID, job.ID, time.Now().UTC())
	if err != nil || createdJob || retried.ID != job.ID {
		t.Fatalf("相同幂等键没有返回原任务：created=%v job=%+v err=%v", createdJob, retried, err)
	}
	active, createdJob, err := repository.Create(ctx, plan.ID, "123e4567-e89b-12d3-a456-426614174721", time.Now().UTC())
	if err != nil || createdJob || active.ID != job.ID {
		t.Fatalf("同计划建立了第二个活动任务：created=%v job=%+v err=%v", createdJob, active, err)
	}
	if err := repository.Start(ctx, plan.ID, job.ID, time.Now().UTC()); err != nil {
		t.Fatalf("启动任务失败：%v", err)
	}
	claimed, err := repository.ClaimBatch(ctx, plan.ID, job.ID, 1, time.Now().UTC())
	if err != nil || len(claimed) != 1 {
		t.Fatalf("领取首批检查点失败：items=%+v err=%v", claimed, err)
	}
	cancelled, err := repository.Cancel(ctx, plan.ID, job.ID, time.Now().UTC())
	if err != nil || cancelled.Status != "cancelled" {
		t.Fatalf("取消任务失败：job=%+v err=%v", cancelled, err)
	}
	resumed, err := repository.Resume(ctx, plan.ID, job.ID, time.Now().UTC())
	if err != nil || resumed.Status != "queued" {
		t.Fatalf("恢复任务失败：job=%+v err=%v", resumed, err)
	}
	if err := repository.Start(ctx, plan.ID, job.ID, time.Now().UTC()); err != nil {
		t.Fatalf("恢复后启动失败：%v", err)
	}
	claimed, err = repository.ClaimBatch(ctx, plan.ID, job.ID, 25, time.Now().UTC())
	if err != nil || len(claimed) != 2 {
		t.Fatalf("恢复没有从原检查点领取全部未完成路径：items=%+v err=%v", claimed, err)
	}
	if err := repository.CompleteItem(ctx, plan.ID, job.ID, claimed[0].ID, model.PathPreparationItemResult{Status: "completed", Reason: "准备完成", NodeConfigured: true, DataGenerated: true}, time.Now().UTC()); err != nil {
		t.Fatalf("提交成功明细失败：%v", err)
	}
	if err := repository.CompleteItem(ctx, plan.ID, job.ID, claimed[1].ID, model.PathPreparationItemResult{Status: "needs_attention", Reason: "条件需要人工核对", NeedsAttention: true, PreservedManual: true}, time.Now().UTC()); err != nil {
		t.Fatalf("提交需处理明细失败：%v", err)
	}
	finished, err := repository.Finish(ctx, plan.ID, job.ID, time.Now().UTC())
	if err != nil || finished.Status != "completed" || finished.Processed != 2 || finished.DataGenerated != 1 || finished.NeedsAttention != 1 || finished.PreservedManual != 1 {
		t.Fatalf("任务真实计数不正确：job=%+v err=%v", finished, err)
	}
	firstPage, err := repository.ListItems(ctx, plan.ID, job.ID, 0, 1)
	if err != nil || len(firstPage.Items) != 1 || firstPage.NextCursor == 0 {
		t.Fatalf("首个游标页不正确：page=%+v err=%v", firstPage, err)
	}
	secondPage, err := repository.ListItems(ctx, plan.ID, job.ID, firstPage.NextCursor, 1)
	if err != nil || len(secondPage.Items) != 1 || secondPage.Items[0].PathID == firstPage.Items[0].PathID {
		t.Fatalf("第二个游标页重复或缺失：page=%+v err=%v", secondPage, err)
	}
	bulk, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("建立数百路径事务失败：%v", err)
	}
	for index := 0; index < 200; index++ {
		createdAt := time.Now().UTC()
		result, insertErr := bulk.ExecContext(ctx, `INSERT INTO test_execution_paths
(plan_id, sequence_no, create_key, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
			plan.ID, index+4, fmt.Sprintf("123e4567-e89b-12d3-a456-%012d", index+800), fmt.Sprintf("批量路径 %d", index+1), createdAt, createdAt)
		if insertErr != nil {
			_ = bulk.Rollback()
			t.Fatalf("插入数百路径失败：%v", insertErr)
		}
		pathID, insertErr := result.LastInsertId()
		if insertErr == nil {
			_, insertErr = bulk.ExecContext(ctx, `INSERT INTO test_execution_path_configs
(path_id, config_status, field_values, action_values, form_status, data_status, created_at, updated_at)
VALUES (?, 'pending', JSON_OBJECT(), JSON_OBJECT('f008:test-included', 'true'), 'empty', 'not_generated', ?, ?)`, pathID, createdAt, createdAt)
		}
		if insertErr != nil {
			_ = bulk.Rollback()
			t.Fatalf("插入数百路径勾选配置失败：%v", insertErr)
		}
	}
	if err := bulk.Commit(); err != nil {
		t.Fatalf("提交数百路径事务失败：%v", err)
	}
	largeJob, createdLarge, err := repository.Create(ctx, plan.ID, "123e4567-e89b-12d3-a456-426614174722", time.Now().UTC())
	if err != nil || !createdLarge || largeJob.Total != 202 {
		t.Fatalf("数百路径没有使用数据库快照建立完整任务：created=%v job=%+v err=%v", createdLarge, largeJob, err)
	}
	if err := repository.Start(ctx, plan.ID, largeJob.ID, time.Now().UTC()); err != nil {
		t.Fatalf("启动数百路径任务失败：%v", err)
	}
	largeBatch, err := repository.ClaimBatch(ctx, plan.ID, largeJob.ID, 25, time.Now().UTC())
	if err != nil || len(largeBatch) != 25 {
		t.Fatalf("数百路径没有按资源边界领取：items=%d err=%v", len(largeBatch), err)
	}
	if _, err := repository.Cancel(ctx, plan.ID, largeJob.ID, time.Now().UTC()); err != nil {
		t.Fatalf("数百路径任务取消失败：%v", err)
	}
	_ = created
}
