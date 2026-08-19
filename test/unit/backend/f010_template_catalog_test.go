package backend_test

import (
	"context"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	"test-auto-pro-v2/internal/service"
)

// TestF010VuePageRuleExtractsNoFormConfiguration 验证配置式 Vue 页面不会因缺少 FormMaking JSON 被当作无需数据。
func TestF010VuePageRuleExtractsNoFormConfiguration(t *testing.T) {
	page := service.AnalyzeVueCustomPageRule(f010ProjectRoot(t), "contract_review", "noForm")
	if page.PageName != "合同评审" || len(page.Fields) == 0 {
		t.Fatalf("合同评审 Vue 页面规则未识别：%+v", page)
	}
	found := false
	for _, field := range page.Fields {
		if field.Path == "contractNameId" && field.ValueType == "select" && field.Required {
			found = true
		}
	}
	if !found || len(page.Issues) > 0 {
		t.Fatalf("Vue 配置字段或发起入口分析不完整：fields=%+v issues=%+v", page.Fields, page.Issues)
	}
}

// TestF010TemplateCatalogCachesRulesAndIncrementallySkips 验证规则目录持久化后增量分析只读取变化模板，不在计划操作时重新扫描源码。
func TestF010TemplateCatalogCachesRulesAndIncrementallySkips(t *testing.T) {
	repository := newF010CatalogRepository()
	reader := &f010CatalogTarget{templates: []target.FlowTemplate{
		{ID: "fm-1", Code: "leave", FlowName: "请假", TypeName: "行政", FormExist: "form", UpdateDate: "2026-08-19"},
		{ID: "vue-1", Code: "contract_review", FlowName: "合同评审", TypeName: "合同", FormExist: "noForm", UpdateDate: "2026-08-19"},
	}}
	reader.configurations = map[string]target.PathConfigurationSnapshot{
		"fm-1":  {FlowCode: "leave", RenderType: target.FormRenderTypeFormMaking, Forms: []target.FormRuntimeTemplate{{TemplateData: `{"list":[{"type":"input","model":"reason","options":{"required":true}}]}`}}},
		"vue-1": {FlowCode: "contract_review", RenderType: target.FormRenderTypeVueCustom},
	}
	catalog := service.NewTemplateCatalogService(reader, repository, f010ProjectRoot(t))
	job, err := catalog.CreateJob(context.Background(), "欧阳改", "full")
	if err != nil {
		t.Fatalf("创建全量规则分析任务失败：%v", err)
	}
	finished := waitF010CatalogJob(t, catalog, job.ID)
	if finished.Status != "completed" || finished.Total != 2 || finished.Completed != 2 {
		t.Fatalf("全量规则目录分析未完成：%+v", finished)
	}
	if reader.configurationReads() != 2 {
		t.Fatalf("全量分析应逐模板读取一次详情，实际 %d 次", reader.configurationReads())
	}
	summary, err := catalog.Summary(context.Background())
	if err != nil || summary.Total != 2 || summary.FormMaking != 1 || summary.VueCustom != 1 || summary.NeedsAttention != 0 {
		t.Fatalf("规则目录汇总不正确：summary=%+v err=%v", summary, err)
	}
	second, err := catalog.CreateJob(context.Background(), "欧阳改", "incremental")
	if err != nil {
		t.Fatalf("创建增量规则分析任务失败：%v", err)
	}
	if finished = waitF010CatalogJob(t, catalog, second.ID); finished.Status != "completed" || finished.Processed != 2 {
		t.Fatalf("增量规则目录分析未完成：%+v", finished)
	}
	if reader.configurationReads() != 2 {
		t.Fatalf("未变化模板不应重新读取详情，实际 %d 次", reader.configurationReads())
	}
}

// TestF010PathConfigurationConsumesStoredVueRules 验证计划配置只消费本地目录中的 Vue 页面规则，不在请求链中重新扫描宿主源码。
func TestF010PathConfigurationConsumesStoredVueRules(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 71, Account: "欧阳改", FlowSource: "new", TargetObjectID: "vue-1", TargetObjectName: "合同评审", Status: model.PlanStatusNotStarted}}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 72, PlanID: 71, SequenceNo: 1, Name: "路径 1", Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-last"}}}}}
	reader := pathConfigSnapshotReader{snapshot: target.PathConfigurationSnapshot{Tree: requirementConditionTree(), EntryNodeIDs: []string{"start"}, FlowCode: "contract_review", RenderType: target.FormRenderTypeVueCustom}}
	catalog := &f010StoredRuleReader{item: model.TemplateRuleCatalogItem{FlowCode: "contract_review", RenderType: model.TemplateRuleRenderVueCustom, RuleData: map[string]any{"page": map[string]any{"pageKey": "contract_review", "pageName": "合同评审", "componentName": "ContractReview", "route": "contract_review", "fields": []map[string]any{{"path": "contractNameId", "name": "合同名称", "valueType": "select", "required": true}}, "issues": []string{}}}}}
	serviceUnderTest := service.NewPathConfigService(service.NewPlanService(plans), reader, analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathConfigAnalyzer(), paths, emptyPathConfigRepository{})
	serviceUnderTest.SetTemplateRuleCatalog(catalog)
	configuration, err := serviceUnderTest.Get(context.Background(), 71, 72)
	if err != nil {
		t.Fatalf("读取本地 Vue 页面规则失败：%v", err)
	}
	if catalog.calls != 1 || configuration.Form.RenderType != string(target.FormRenderTypeVueCustom) || configuration.Form.VuePage == nil || len(configuration.Form.VuePage.Fields) != 1 || configuration.Form.VuePage.Issues != nil && len(configuration.Form.VuePage.Issues) > 0 {
		t.Fatalf("路径配置没有消费稳定 Vue 页面规则：calls=%d form=%+v", catalog.calls, configuration.Form)
	}
}

// f010StoredRuleReader 提供路径配置读取已持久化规则的最小接口，并记录调用边界。
type f010StoredRuleReader struct {
	item  model.TemplateRuleCatalogItem
	calls int
}

// GetByFlowCode 返回已保存的流程规则，不触发目标平台访问。
func (r *f010StoredRuleReader) GetByFlowCode(_ context.Context, _ string) (model.TemplateRuleCatalogItem, bool, error) {
	r.calls++
	return r.item, true, nil
}

// waitF010CatalogJob 等待内存任务落到终态，超时直接暴露后台任务无法收口的问题。
func waitF010CatalogJob(t *testing.T, catalog *service.TemplateCatalogService, jobID string) model.TemplateRuleAnalysisJob {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err := catalog.GetJob(context.Background(), jobID)
		if err != nil {
			t.Fatalf("读取规则分析任务失败：%v", err)
		}
		if job.Status == "completed" || job.Status == "failed" {
			return job
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("规则分析任务未在时限内结束：%s", jobID)
	return model.TemplateRuleAnalysisJob{}
}

// f010CatalogTarget 模拟目标平台只读模板接口，并记录是否发生不必要的详情读取。
type f010CatalogTarget struct {
	mu             sync.Mutex
	templates      []target.FlowTemplate
	configurations map[string]target.PathConfigurationSnapshot
	reads          int
}

// Templates 返回一次完整但分页的账号可见模板列表。
func (r *f010CatalogTarget) Templates(_ context.Context, _ string, _ string, page, pageSize int) (target.Page[target.FlowTemplate], error) {
	start := (page - 1) * pageSize
	if start >= len(r.templates) {
		return target.Page[target.FlowTemplate]{Items: []target.FlowTemplate{}, Page: page, PageSize: pageSize, Total: len(r.templates)}, nil
	}
	end := start + pageSize
	if end > len(r.templates) {
		end = len(r.templates)
	}
	return target.Page[target.FlowTemplate]{Items: append([]target.FlowTemplate(nil), r.templates[start:end]...), Page: page, PageSize: pageSize, Total: len(r.templates), HasMore: end < len(r.templates)}, nil
}

// TemplateConfiguration 返回单模板快照并记录读取次数。
func (r *f010CatalogTarget) TemplateConfiguration(_ context.Context, _ string, templateID string) (target.PathConfigurationSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reads++
	return r.configurations[templateID], nil
}

// configurationReads 返回同步期间发生的目标详情读取次数。
func (r *f010CatalogTarget) configurationReads() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reads
}

// f010CatalogRepository 是后台目录任务的并发安全内存仓储，覆盖幂等和检查点行为。
type f010CatalogRepository struct {
	mu    sync.Mutex
	items map[string]model.TemplateRuleCatalogItem
	jobs  map[string]model.TemplateRuleAnalysisJob
}

// newF010CatalogRepository 创建空的规则目录内存仓储。
func newF010CatalogRepository() *f010CatalogRepository {
	return &f010CatalogRepository{items: map[string]model.TemplateRuleCatalogItem{}, jobs: map[string]model.TemplateRuleAnalysisJob{}}
}

// Upsert 覆盖同一源模板的规则快照。
func (r *f010CatalogRepository) Upsert(_ context.Context, item model.TemplateRuleCatalogItem) (model.TemplateRuleCatalogItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item.ID = uint64(len(r.items) + 1)
	r.items[item.SourceTemplateID] = item
	return item, nil
}

// GetByFlowCode 按流程编码返回最新本地规则。
func (r *f010CatalogRepository) GetByFlowCode(_ context.Context, flowCode string) (model.TemplateRuleCatalogItem, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.items {
		if item.FlowCode == flowCode {
			return item, true, nil
		}
	}
	return model.TemplateRuleCatalogItem{}, false, nil
}

// GetBySourceTemplateID 按源模板标识返回规则快照。
func (r *f010CatalogRepository) GetBySourceTemplateID(_ context.Context, templateID string) (model.TemplateRuleCatalogItem, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, found := r.items[templateID]
	return item, found, nil
}

// List 提供稳定排序的目录分页结果。
func (r *f010CatalogRepository) List(_ context.Context, _ string, offset, limit int) ([]model.TemplateRuleCatalogItem, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]model.TemplateRuleCatalogItem, 0, len(r.items))
	for _, item := range r.items {
		items = append(items, item)
	}
	sort.Slice(items, func(left, right int) bool { return items[left].SourceTemplateID < items[right].SourceTemplateID })
	if offset >= len(items) {
		return []model.TemplateRuleCatalogItem{}, len(items), nil
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end], len(items), nil
}

// Summary 汇总内存目录的渲染类型和问题状态。
func (r *f010CatalogRepository) Summary(_ context.Context) (model.TemplateRuleCatalogSummary, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := model.TemplateRuleCatalogSummary{Components: map[string]int{}}
	for _, item := range r.items {
		result.Total++
		switch item.RenderType {
		case model.TemplateRuleRenderFormMaking:
			result.FormMaking++
		case model.TemplateRuleRenderVueCustom:
			result.VueCustom++
		default:
			result.Unknown++
		}
		if item.Status == "complete" {
			result.Complete++
		} else if item.Status == "needs_attention" {
			result.NeedsAttention++
		}
	}
	return result, nil
}

// CreateJob 建立账号唯一活动任务。
func (r *f010CatalogRepository) CreateJob(_ context.Context, job model.TemplateRuleAnalysisJob) (model.TemplateRuleAnalysisJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, current := range r.jobs {
		if current.Account == job.Account && (current.Status == "queued" || current.Status == "running") {
			return model.TemplateRuleAnalysisJob{}, repository.ErrTemplateCatalogActive
		}
	}
	r.jobs[job.ID] = job
	return job, nil
}

// GetJob 返回任务检查点。
func (r *f010CatalogRepository) GetJob(_ context.Context, jobID string) (model.TemplateRuleAnalysisJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, found := r.jobs[jobID]
	if !found {
		return model.TemplateRuleAnalysisJob{}, repository.ErrTemplateCatalogNotFound
	}
	return job, nil
}

// LatestJob 返回账号最近建的任务。
func (r *f010CatalogRepository) LatestJob(_ context.Context, account string) (model.TemplateRuleAnalysisJob, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result model.TemplateRuleAnalysisJob
	found := false
	for _, job := range r.jobs {
		if job.Account == account && (!found || job.CreatedAt.After(result.CreatedAt)) {
			result, found = job, true
		}
	}
	return result, found, nil
}

// UpdateJob 覆盖任务实时进度。
func (r *f010CatalogRepository) UpdateJob(_ context.Context, job model.TemplateRuleAnalysisJob) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[job.ID] = job
	return nil
}

// MarkInterruptedJobs 把内存中的活动任务收敛为失败。
func (r *f010CatalogRepository) MarkInterruptedJobs(_ context.Context, message string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, job := range r.jobs {
		if job.Status == "queued" || job.Status == "running" {
			job.Status, job.Message = "failed", message
			r.jobs[id] = job
		}
	}
	return nil
}

// f010ProjectRoot 从当前测试文件定位项目根目录，避免依赖执行命令的工作目录。
func f010ProjectRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位 F-010 测试文件")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
