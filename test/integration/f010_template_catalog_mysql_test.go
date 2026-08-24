package integration_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/model"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
)

// TestF010TemplateCatalogMySQLPersistence 验证规则目录迁移、覆盖写入、分页汇总和任务恢复的真实 MySQL 行为。
func TestF010TemplateCatalogMySQLPersistence(t *testing.T) {
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-010 MySQL 集成测试缺少配置名：%v", missing)
	}
	cfg.Name = temporaryPlanDatabaseName(t)
	t.Cleanup(func() { dropTemporaryPlanDatabase(t, cfg) })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("F-010 临时数据库迁移失败：%v", err)
	}
	defer database.Close()
	repository := planmysql.NewTemplateCatalogRepository(database.DB)
	now := time.Now().UTC().Truncate(time.Millisecond)
	item := model.TemplateRuleCatalogItem{
		SourceTemplateID: "template-vue", FlowCode: "flow-vue", FlowName: "Vue 流程", TemplateType: "业务",
		FormExist: "noForm", RenderType: model.TemplateRuleRenderVueCustom, SourceAccount: "欧阳改",
		SourceVersion: "v1", SourceFingerprint: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", AnalyzerVersion: "f010-v1",
		Status: "complete", RuleData: map[string]any{"page": map[string]any{"pageKey": "flow-vue", "fields": []any{map[string]any{"path": "title"}}}},
		Coverage: map[string]any{"customComponents": map[string]any{"custome-info-select": 1}}, Issues: []string{},
		AnalyzedAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	stored, err := repository.Upsert(ctx, item)
	if err != nil || stored.FlowCode != item.FlowCode || stored.RuleData["page"] == nil || stored.Issues == nil {
		t.Fatalf("规则快照首次写入或 JSON 回读失败：item=%+v err=%v", stored, err)
	}
	item.SourceVersion, item.Status, item.Issues, item.UpdatedAt = "v2", "needs_attention", []string{"动态协议需要核对"}, now.Add(time.Second)
	updated, err := repository.Upsert(ctx, item)
	if err != nil || updated.ID != stored.ID || updated.SourceVersion != "v2" || len(updated.Issues) != 1 {
		t.Fatalf("同源模板没有原位覆盖更新：item=%+v err=%v", updated, err)
	}
	items, total, err := repository.List(ctx, "Vue", 0, 1)
	if err != nil || total != 1 || len(items) != 1 || items[0].RuleData["page"] == nil {
		t.Fatalf("规则目录分页不正确：total=%d items=%+v err=%v", total, items, err)
	}
	summary, err := repository.Summary(ctx)
	if err != nil || summary.CatalogTotal != 1 || summary.VueCustom != 1 || summary.NeedsAttention != 1 || summary.Components["custom:custome-info-select"] != 1 {
		t.Fatalf("规则目录汇总不正确：summary=%+v err=%v", summary, err)
	}
	planResult, err := database.DB.ExecContext(ctx, `INSERT INTO test_plans
(create_key, name, account, account_display_name, flow_source, target_object_id, target_object_name, run_mode, status, created_at, updated_at)
VALUES (?, ?, ?, ?, 'new', ?, ?, 'all', 'not_started', ?, ?)`, "ebc731b8-8bcc-48c6-ab16-3ba93a64dd11", "stale 计划", "发起人", "发起人", item.SourceTemplateID, item.FlowName, now, now)
	if err != nil {
		t.Fatalf("创建 stale 关联计划失败：%v", err)
	}
	planID, _ := planResult.LastInsertId()
	pathResult, err := database.DB.ExecContext(ctx, `INSERT INTO test_execution_paths
(plan_id, sequence_no, create_key, name, created_at, updated_at) VALUES (?, 1, ?, '路径 1', ?, ?)`, planID, "30525943-57f2-469b-b7bd-b0eeb81ed3c3", now, now)
	if err != nil {
		t.Fatalf("创建 stale 关联路径失败：%v", err)
	}
	pathID, _ := pathResult.LastInsertId()
	_, err = database.DB.ExecContext(ctx, `INSERT INTO test_execution_path_configs
(path_id, revision, field_values, action_values, form_values, form_status, data_status, form_validated, created_at, updated_at)
VALUES (?, 1, JSON_OBJECT(), JSON_OBJECT(), JSON_OBJECT('title', '保留值'), 'valid', 'confirmed', 1, ?, ?)`, pathID, now, now)
	if err != nil {
		t.Fatalf("创建 stale 关联表单配置失败：%v", err)
	}
	if err := repository.MarkStale(ctx, item.SourceTemplateID); err != nil {
		t.Fatalf("标记目录 stale 失败：%v", err)
	}
	marked, found, err := repository.GetBySourceTemplateID(ctx, item.SourceTemplateID)
	if err != nil || !found || !marked.Stale {
		t.Fatalf("stale 标记没有持久化：item=%+v found=%v err=%v", marked, found, err)
	}
	var formStatus, dataStatus, formValues string
	var validated bool
	if err := database.DB.QueryRowContext(ctx, "SELECT form_status, data_status, form_validated, form_values FROM test_execution_path_configs WHERE path_id = ?", pathID).Scan(&formStatus, &dataStatus, &validated, &formValues); err != nil || formStatus != "affected" || dataStatus != "needs_attention" || validated || !strings.Contains(formValues, "保留值") {
		t.Fatalf("stale 没有保留 values 并传播需处理状态：form=%s data=%s validated=%v values=%s err=%v", formStatus, dataStatus, validated, formValues, err)
	}
	summary, err = repository.Summary(ctx)
	if err != nil || summary.Stale != 1 {
		t.Fatalf("stale 汇总不正确：summary=%+v err=%v", summary, err)
	}
	item.Stale = false
	item.UpdatedAt = now.Add(2 * time.Second)
	refreshed, err := repository.Upsert(ctx, item)
	if err != nil || refreshed.Stale {
		t.Fatalf("模板更新成功后 stale 没有清除：item=%+v err=%v", refreshed, err)
	}
	if err := database.DB.QueryRowContext(ctx, "SELECT form_status, data_status, form_values FROM test_execution_path_configs WHERE path_id = ?", pathID).Scan(&formStatus, &dataStatus, &formValues); err != nil || formStatus != "affected" || dataStatus != "needs_attention" || !strings.Contains(formValues, "保留值") {
		t.Fatalf("目录刷新后旧值被冒充为已复验：form=%s data=%s values=%s err=%v", formStatus, dataStatus, formValues, err)
	}
	job := model.TemplateRuleAnalysisJob{ID: "f010-mysql-job", Mode: "full", Account: "欧阳改", State: "running", Total: 73, Listed: 8, Accounted: 8, Complete: 8, Failures: []model.TemplateRuleAnalysisFailure{}, CreatedAt: now, UpdatedAt: now}
	if _, err := repository.CreateJob(ctx, job); err != nil {
		t.Fatalf("创建规则分析任务失败：%v", err)
	}
	if err := repository.MarkInterruptedJobs(ctx, "服务重启后重试"); err != nil {
		t.Fatalf("恢复中断任务失败：%v", err)
	}
	recovered, err := repository.GetJob(ctx, job.ID)
	if err != nil || recovered.State != "finished" || recovered.Outcome != "failed" || recovered.Accounted != recovered.Total || recovered.Unlisted != 65 || recovered.FinishedAt == nil || len(recovered.Failures) != 1 || recovered.Failures[0].Stage != "service_recovery" {
		t.Fatalf("中断任务没有收敛为可重试失败态：job=%+v err=%v", recovered, err)
	}
}

// TestF010TemplateCatalogMySQLPaginatesWideRules 验证目录分页先排序窄字段，规则正文较大时不会耗尽 MySQL 排序缓冲。
func TestF010TemplateCatalogMySQLPaginatesWideRules(t *testing.T) {
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-010 MySQL 集成测试缺少配置名：%v", missing)
	}
	cfg.Name = temporaryPlanDatabaseName(t)
	t.Cleanup(func() { dropTemporaryPlanDatabase(t, cfg) })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("F-010 宽规则临时数据库迁移失败：%v", err)
	}
	defer database.Close()
	repository := planmysql.NewTemplateCatalogRepository(database.DB)
	now := time.Now().UTC().Truncate(time.Millisecond)
	wideValue := strings.Repeat("规则", 16*1024)
	for index := 0; index < 80; index++ {
		code := fmt.Sprintf("wide-flow-%03d", index)
		_, err := repository.Upsert(ctx, model.TemplateRuleCatalogItem{
			SourceTemplateID: code, FlowCode: code, FlowName: "宽规则", RenderType: model.TemplateRuleRenderFormMaking,
			SourceAccount: "欧阳改", SourceFingerprint: strings.Repeat("a", 64), AnalyzerVersion: "f010-v2", Status: "complete",
			RuleData: map[string]any{"wide": wideValue}, Coverage: map[string]any{"fieldCount": 1}, Issues: []string{},
			AnalyzedAt: &now, CreatedAt: now, UpdatedAt: now.Add(time.Duration(index) * time.Millisecond),
		})
		if err != nil {
			t.Fatalf("写入宽规则 %d 失败：%v", index, err)
		}
	}
	items, total, err := repository.List(ctx, "", 0, 20)
	if err != nil || total != 80 || len(items) != 20 {
		t.Fatalf("宽规则目录分页失败：total=%d items=%d err=%v", total, len(items), err)
	}
}
