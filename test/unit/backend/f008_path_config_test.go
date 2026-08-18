package backend_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

type pathConfigSnapshotReader struct {
	snapshot target.PathConfigurationSnapshot
}

// PathConfigurationSnapshot 为路径配置服务返回固定的当前真实流程快照。
func (r pathConfigSnapshotReader) PathConfigurationSnapshot(context.Context, string, string, string) (target.PathConfigurationSnapshot, error) {
	return r.snapshot, nil
}

type emptyPathConfigRepository struct{}

// FindByPath 表示当前路径还没有保存过任何节点或表单配置。
func (emptyPathConfigRepository) FindByPath(context.Context, uint64) (model.StoredPathConfig, bool, error) {
	return model.StoredPathConfig{}, false, nil
}

// FindByPathAndKey 表示当前测试没有可复用的幂等保存结果。
func (emptyPathConfigRepository) FindByPathAndKey(context.Context, uint64, string) (model.StoredPathConfig, bool, error) {
	return model.StoredPathConfig{}, false, nil
}

// Save 返回传入记录，满足只读配置测试的仓储接口边界。
func (emptyPathConfigRepository) Save(_ context.Context, record model.StoredPathConfig, _ uint64, _ time.Time) (model.StoredPathConfig, error) {
	return record, nil
}

// TestF008PathConfigReadsDetailChoices 验证节点尚未配置时仍会使用单条完整路径进入待配置态。
func TestF008PathConfigReadsDetailChoices(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Account: "account", FlowSource: "new", TargetObjectID: "template", TargetObjectName: "测试流程", Status: model.PlanStatusPendingConfiguration}}
	paths := &memoryExecutionPathRepository{listSummaryOnly: true, paths: []model.ExecutionPath{{
		ID: 32, PlanID: 7, SequenceNo: 1, Name: "路径 1", Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}},
	}}}
	serviceUnderTest := service.NewPathConfigService(
		service.NewPlanService(plans),
		pathConfigSnapshotReader{snapshot: target.PathConfigurationSnapshot{Tree: requirementConditionTree(), EntryNodeIDs: []string{"start"}}},
		analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathConfigAnalyzer(), paths, emptyPathConfigRepository{},
	)
	configuration, err := serviceUnderTest.Get(context.Background(), 7, 32)
	if err != nil {
		t.Fatalf("路径尚未配置节点时不应被误报为路径失效：%v", err)
	}
	if configuration.Status != "pending" || configuration.Progress.Pending == 0 {
		t.Fatalf("未配置节点应返回待配置状态：%+v", configuration)
	}
	if paths.getCalls != 1 {
		t.Fatalf("路径配置没有按需读取单条完整 choices：calls=%d", paths.getCalls)
	}
}

// TestF008ActionConfigurationUsesOneArrivalPerAction 验证可配置动作按到达顺序保存，不包含系统基础动作。
func TestF008ActionConfigurationUsesOneArrivalPerAction(t *testing.T) {
	target := analyzer.PathConfigNodeTarget{NodeID: "approval-a", Name: "审批", ActionKinds: map[string]bool{"reject_no_pass": true}}
	encoded, count, reason := analyzer.EncodePathConfigActions(target, []model.PathConfigConfiguredActionInput{{Key: "action-1", Kind: "reject_no_pass", Count: 2}})
	if reason != "" || count != 2 {
		t.Fatalf("动作配置编码失败：count=%d reason=%s", count, reason)
	}
	if !strings.Contains(encoded, `"version":1`) || strings.Contains(encoded, "arrivals") || strings.Contains(encoded, "target") {
		t.Fatalf("动作配置仍混入旧结构：%s", encoded)
	}
}

// TestF008ActionConfigurationRejectsUnsupportedInput 验证动作目录、次数和单节点上限由服务端约束。
func TestF008ActionConfigurationRejectsUnsupportedInput(t *testing.T) {
	target := analyzer.PathConfigNodeTarget{ActionKinds: map[string]bool{"reject_no_pass": true}}
	if encoded, count, reason := analyzer.EncodePathConfigActions(target, nil); reason != "" || count != 0 || !strings.Contains(encoded, `"actions":[]`) {
		t.Fatalf("默认空动作配置不应失败：count=%d reason=%s encoded=%s", count, reason, encoded)
	}
	if _, _, reason := analyzer.EncodePathConfigActions(target, []model.PathConfigConfiguredActionInput{{Kind: "draft_save", Count: 1}}); reason == "" {
		t.Fatal("不允许的动作没有被拒绝")
	}
	if _, _, reason := analyzer.EncodePathConfigActions(target, []model.PathConfigConfiguredActionInput{{Kind: "reject_no_pass", Count: 1}, {Kind: "reject_no_pass", Count: 1}}); reason == "" {
		t.Fatal("重复动作没有被拒绝")
	}
	for _, kind := range []string{"approve_pass", "submit", "transfer_approver", "transpond"} {
		if _, _, reason := analyzer.EncodePathConfigActions(target, []model.PathConfigConfiguredActionInput{{Kind: kind, Count: 1}}); reason == "" {
			t.Fatalf("系统基础或错误动作没有被拒绝：%s", kind)
		}
	}
}

// TestF008ActionExecutionCountIgnoresLegacyKeys 验证旧 action-plan 键不会进入新动作执行量统计。
func TestF008ActionExecutionCountIgnoresLegacyKeys(t *testing.T) {
	values := map[string]string{"action-plan:approval-a": `{"version":1,"arrivals":[]}`}
	if count, valid := analyzer.CountStoredPathConfigActionExecutions(values); count != 0 || !valid {
		t.Fatalf("旧动作键错误影响新统计：count=%d valid=%v", count, valid)
	}
	encoded, _, _ := analyzer.EncodePathConfigActions(analyzer.PathConfigNodeTarget{ActionKinds: map[string]bool{"reject_no_pass": true}}, []model.PathConfigConfiguredActionInput{{Kind: "reject_no_pass", Count: 1}})
	values[analyzer.PathConfigActionConfigurationStorageKey("approval-a")] = encoded
	if count, valid := analyzer.CountStoredPathConfigActionExecutions(values); count != 1 || !valid {
		t.Fatalf("新动作执行量统计错误：count=%d valid=%v", count, valid)
	}
}
