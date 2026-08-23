package backend_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

type f010SlowOptionalReader struct {
	snapshot target.PathConfigurationSnapshot
}

// PathConfigurationSnapshot 立即返回规则快照，把测试等待精确限制在可选数据源阶段。
func (r f010SlowOptionalReader) PathConfigurationSnapshot(context.Context, string, string, string) (target.PathConfigurationSnapshot, error) {
	return r.snapshot, nil
}

// RecentFormSamplesForRule 等待子预算取消，模拟目标样本接口无响应。
func (r f010SlowOptionalReader) RecentFormSamplesForRule(ctx context.Context, _, _, _, _, _ string, _ int) ([]map[string]any, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// FormIdentityContext 等待子预算取消，模拟身份目录无响应。
func (r f010SlowOptionalReader) FormIdentityContext(ctx context.Context, _ string) (target.FormIdentityContext, error) {
	<-ctx.Done()
	return target.FormIdentityContext{}, ctx.Err()
}

type f010SlowCandidateProvider struct{}

// waitF010CandidateContext 统一等待候选子预算取消，避免测试为不存在的候选制造返回值。
func waitF010CandidateContext(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

// ComponentCandidates 模拟模板实际使用的项目候选超时。
func (f010SlowCandidateProvider) ComponentCandidates(ctx context.Context, _, _, _ string) ([]any, error) {
	return nil, waitF010CandidateContext(ctx)
}

// TestF010GenerationOptionalReadsShareThreeSecondBudget 验证三类可选读取并行超时并形成可展示 partial，而非串行等待或技术错误。
func TestF010GenerationOptionalReadsShareThreeSecondBudget(t *testing.T) {
	serviceUnderTest := newF010SlowGenerationService()
	startedAt := time.Now()
	result, err := serviceUnderTest.GenerateForm(context.Background(), 110, 111, 1, map[string]any{}, nil, false)
	elapsed := time.Since(startedAt)
	if err != nil {
		t.Fatalf("可选读取超时不应吞成技术错误：%v", err)
	}
	if elapsed < 2800*time.Millisecond || elapsed > 4500*time.Millisecond {
		t.Fatalf("样本、身份和候选没有共享三秒预算：%s", elapsed)
	}
	if result.GenerationState != "partial" {
		t.Fatalf("可选读取超时应返回 partial：%+v", result)
	}
	issueText := f010GenerationIssueText(result.Issues)
	for _, expected := range []string{"近期样本读取超时", "发起人身份读取超时", "组件候选读取超时"} {
		if !strings.Contains(issueText, expected) {
			t.Fatalf("缺少分阶段超时问题 %q：%s", expected, issueText)
		}
	}
}

// TestF010GenerationStopsOnCallerCancellation 验证调用方取消会立即穿透可选读取并退出服务。
func TestF010GenerationStopsOnCallerCancellation(t *testing.T) {
	serviceUnderTest := newF010SlowGenerationService()
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	startedAt := time.Now()
	_, err := serviceUnderTest.GenerateForm(ctx, 110, 111, 1, map[string]any{}, nil, false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("调用方取消没有原样退出：%v", err)
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("调用方取消退出过慢：%s", elapsed)
	}
}

// newF010SlowGenerationService 创建只有可选数据源会阻塞的路径生成服务。
func newF010SlowGenerationService() *service.PathConfigService {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 110, Account: "account", FlowSource: "new", TargetObjectID: "template", TargetObjectName: "测试流程", Status: model.PlanStatusNotStarted}}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 111, PlanID: 110, SequenceNo: 1, Name: "直达路径"}}}
	tree := &target.FlowNodeTemplate{ID: "start", Name: "发起", Type: "start", Child: &target.FlowNodeTemplate{ID: "end", Name: "结束", Type: "end"}}
	template := `{"list":[{"type":"input","model":"title","name":"标题","options":{"defaultValue":"安全值","required":true}},{"type":"custom","el":"custome-select-project","model":"project","name":"项目","options":{"required":true}}]}`
	reader := f010SlowOptionalReader{snapshot: target.PathConfigurationSnapshot{
		Tree: tree, EntryNodeIDs: []string{"start"}, FlowCode: "flow", TemplateID: "template", RuleVersion: "rule-v1",
		Forms: []target.FormRuntimeTemplate{{Name: "申请表", TemplateData: template}},
	}}
	configured := service.NewPathConfigService(service.NewPlanService(plans), reader, analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathConfigAnalyzer(), paths, emptyPathConfigRepository{})
	configured.SetComponentCandidateCache(service.NewComponentCandidateCache(f010SlowCandidateProvider{}, 10, time.Minute))
	return configured
}

// f010GenerationIssueText 合并生成问题，便于核对固定阶段原因。
func f010GenerationIssueText(issues []model.PathFormGenerationIssue) string {
	parts := make([]string, 0, len(issues))
	for _, issue := range issues {
		parts = append(parts, issue.Field+"："+issue.Reason)
	}
	return strings.Join(parts, "|")
}
