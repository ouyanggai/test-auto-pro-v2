package contracts_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/api"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// f010TemplateCatalogStub 提供设置页规则目录 API 的固定业务结果。
type f010TemplateCatalogStub struct{}

// Summary 返回 FormMaking、Vue 页面和待识别模板汇总。
func (f010TemplateCatalogStub) Summary(context.Context) (model.TemplateRuleCatalogSummary, error) {
	return model.TemplateRuleCatalogSummary{CatalogTotal: 73, FormMaking: 60, VueCustom: 12, Unknown: 1, Complete: 67, NeedsAttention: 4, Blocked: 1, Failed: 1, Components: map[string]int{"input": 60, "vue:ContractReview": 1}, RegisteredComponents: 20}, nil
}

// List 返回不含规则正文的目录摘要。
func (f010TemplateCatalogStub) List(context.Context, string, int, int) ([]model.TemplateRuleCatalogItem, int, error) {
	return []model.TemplateRuleCatalogItem{{ID: 1, SourceTemplateID: "private-template", FlowCode: "contract_review", FlowName: "合同评审", RenderType: model.TemplateRuleRenderVueCustom, Status: "blocked", SourceFingerprint: "private-digest", RuleData: map[string]any{"private": true}, Coverage: map[string]any{"fieldCount": 12, "components": map[string]any{"vue:ContractReview": float64(1)}}, Issues: []string{"auditWay 缺失"}}}, 73, nil
}

// CreateJob 返回已排队的全量目录任务。
func (f010TemplateCatalogStub) CreateJob(_ context.Context, account, mode string) (model.TemplateRuleAnalysisJob, error) {
	return f010CatalogJob(account, mode, "queued"), nil
}

// GetJob 返回任务的真实进度计数。
func (f010TemplateCatalogStub) GetJob(context.Context, string) (model.TemplateRuleAnalysisJob, error) {
	return f010CatalogJob("欧阳改", "full", "running"), nil
}

// LatestJob 返回页面刷新后可恢复的最近任务。
func (f010TemplateCatalogStub) LatestJob(_ context.Context, account string) (model.TemplateRuleAnalysisJob, bool, error) {
	return f010CatalogJob(account, "incremental", "finished"), true, nil
}

// f010CatalogJob 构造规则分析任务的稳定协议样本。
func f010CatalogJob(account, mode, state string) model.TemplateRuleAnalysisJob {
	now := time.Date(2026, 8, 19, 16, 0, 0, 0, time.UTC)
	job := model.TemplateRuleAnalysisJob{ID: "f010-job", Account: account, Mode: mode, State: state, Failures: []model.TemplateRuleAnalysisFailure{}, UpdatedAt: now}
	if state == "running" {
		job.Total, job.Listed, job.Accounted, job.Complete, job.NeedsAttention, job.Blocked, job.Failed = 73, 40, 40, 35, 3, 1, 1
	}
	if state == "finished" {
		job.Total, job.Listed, job.Accounted, job.Complete, job.NeedsAttention, job.Blocked, job.Failed = 73, 73, 73, 67, 4, 1, 1
		job.Outcome = "with_issues"
	}
	return job
}

// f010TemplateCatalogHandler 组装包含设置页规则目录的真实 HTTP 路由。
func f010TemplateCatalogHandler() http.Handler {
	return api.NewHandlerWithTemplateCatalogServices(
		&stubTargetReader{}, service.NewPlanService(&contractPlanRepository{}), &stubFlowGraphService{},
		&stubExecutionPathService{}, &stubPathRequirementService{}, &stubPathConfigurationService{},
		&stubFormRuntimeMaintenance{}, &f009PathPreparationStub{}, f010TemplateCatalogStub{},
	)
}

// TestF010TemplateCatalogAPIContract 验证汇总、分页、任务创建和刷新协议且规则正文不下发。
func TestF010TemplateCatalogAPIContract(t *testing.T) {
	handler := f010TemplateCatalogHandler()
	for _, test := range []struct{ method, path, body, want string }{
		{http.MethodGet, "/api/settings/template-rules/summary", "", `"catalogTotal":73`},
		{http.MethodGet, "/api/settings/template-rules?page=1&size=50", "", `"vue_custom"`},
		{http.MethodGet, "/api/settings/template-rules/jobs/latest?account=欧阳改", "", `"state":"finished"`},
		{http.MethodGet, "/api/settings/template-rules/jobs/f010-job", "", `"accounted":40`},
		{http.MethodPost, "/api/settings/template-rules/jobs", `{"account":"欧阳改","mode":"full"}`, `"state":"queued"`},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(test.method, test.path, strings.NewReader(test.body)))
		if response.Code < 200 || response.Code >= 300 || !strings.Contains(response.Body.String(), test.want) {
			t.Fatalf("%s %s 契约不正确：status=%d body=%s", test.method, test.path, response.Code, response.Body.String())
		}
		if strings.Contains(response.Body.String(), `"private":true`) {
			t.Fatalf("设置页泄露了规则正文：%s", response.Body.String())
		}
		for _, privateField := range []string{"sourceTemplateId", "sourceFingerprint", "targetDigest", "ruleData", "coverage"} {
			if strings.Contains(response.Body.String(), `"`+privateField+`"`) {
				t.Fatalf("设置页泄露内部字段 %s：%s", privateField, response.Body.String())
			}
		}
	}
}
