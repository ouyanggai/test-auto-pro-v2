package backend_test

import (
	"context"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

type requirementSnapshotReader struct {
	account  string
	source   string
	targetID string
	snapshot target.FlowRequirementSnapshot
	err      error
}

// FlowRequirementSnapshot 记录服务传入的持久化身份并返回预设快照。
func (r *requirementSnapshotReader) FlowRequirementSnapshot(_ context.Context, account, source, targetID string) (target.FlowRequirementSnapshot, error) {
	r.account, r.source, r.targetID = account, source, targetID
	return r.snapshot, r.err
}

// TestPathRequirementServiceUsesPersistedIdentityAndPlanScopedPath 验证要求读取只使用计划身份且不泄露其他计划路径。
func TestPathRequirementServiceUsesPersistedIdentityAndPlanScopedPath(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Account: "saved-account", FlowSource: "new", TargetObjectID: "saved-template", TargetObjectName: "流程"}}
	tree := requirementConditionTree()
	reader := &requirementSnapshotReader{snapshot: target.FlowRequirementSnapshot{Tree: tree, EntryNodeIDs: []string{"start"}}}
	repo := &memoryExecutionPathRepository{listSummaryOnly: true, paths: []model.ExecutionPath{
		{ID: 31, PlanID: 8, SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}},
		{ID: 32, PlanID: 7, SequenceNo: 2, Name: "本计划路径", Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}},
	}}
	serviceUnderTest := service.NewPathRequirementService(
		service.NewPlanService(plans), reader, analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(),
		analyzer.NewPathRequirementAnalyzer(), repo,
	)
	result, err := serviceUnderTest.Get(context.Background(), 7, 32)
	if err != nil || result.Path.SequenceNo != 2 || result.Path.Name != "本计划路径" {
		t.Fatalf("读取本计划要求失败：result=%+v err=%v", result, err)
	}
	if reader.account != "saved-account" || reader.source != "new" || reader.targetID != "saved-template" {
		t.Fatalf("要求读取没有只使用计划身份：%+v", reader)
	}
	if repo.getCalls != 1 {
		t.Fatalf("要求读取没有按需读取完整路径 choices：calls=%d", repo.getCalls)
	}
	_, err = serviceUnderTest.Get(context.Background(), 7, 31)
	if !service.IsExecutionPathErrorKind(err, service.ExecutionPathErrorNotFound) {
		t.Fatalf("其他计划路径归属没有被隔离：%v", err)
	}
}

// TestPathRequirementServiceRejectsPathAfterCurrentGraphChanges 验证保存选择无法对应当前真实图时返回路径失效。
func TestPathRequirementServiceRejectsPathAfterCurrentGraphChanges(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Account: "account", FlowSource: "new", TargetObjectID: "template"}}
	tree := requirementConditionTree()
	repo := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{
		ID: 32, PlanID: 7, SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "removed-branch"}},
	}}}
	serviceUnderTest := service.NewPathRequirementService(
		service.NewPlanService(plans), &requirementSnapshotReader{snapshot: target.FlowRequirementSnapshot{Tree: tree, EntryNodeIDs: []string{"start"}}},
		analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathRequirementAnalyzer(), repo,
	)
	_, err := serviceUnderTest.Get(context.Background(), 7, 32)
	if !service.IsExecutionPathErrorKind(err, service.ExecutionPathErrorInvalid) {
		t.Fatalf("当前图变化后旧路径仍生成要求：%v", err)
	}
}
