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
