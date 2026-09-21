package executor_test

import (
	"context"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// prepConsentRunContext 构造只含一条取回准备同意步骤的运行上下文。
func prepConsentRunContext() step.RunContext {
	runCtx := newRunContext([]model.CompiledActionStep{{
		Sequence: 1, Source: model.ActionStepSourceRecovery, Action: model.ActionApprove,
		Scope: model.ActionScopeTask, NodeKey: "node-audit", RetrievePrepareConsent: true,
	}})
	runCtx.Source = "done"
	runCtx.PathRun.MainInstanceRef = "instance-retrieve"
	runCtx.Nodes["node-audit"] = step.NodeInfo{Name: "已办审批", Type: "common", TargetNodeID: "node-done"}
	return runCtx
}

// TestRetrievePrepareConsentSkippedWhenDoneExists 验证「已有已办直接取回」：当前用户在本节点已有
// 已办任务时，取回准备同意在预览阶段判为跳过，不发出任何写请求。
func TestRetrievePrepareConsentSkippedWhenDoneExists(t *testing.T) {
	targetFake := &taskActionFactsTarget{
		fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-done"}, DueNodes: []string{"node-done"}}},
		snapshot: target.TaskSnapshot{
			LinkID: "done-link", JobTaskID: "done-task", FlowNodeProxyID: "node-done",
			FlowProxyID: "proxy-current", TaskStatus: "done",
		},
		tree: &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{ID: "node-done", Type: "common"}},
	}
	runCtx := prepConsentRunContext()
	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil {
		t.Fatalf("取回准备同意预览失败：finished=%v err=%v preview=%+v", finished, err, preview)
	}
	if !preview.TargetSkipped {
		t.Fatalf("已有已办时取回准备同意应跳过（不发写请求）：%+v", preview)
	}
	if !strings.Contains(preview.SkipReason, "已办") {
		t.Fatalf("跳过原因应说明当前用户在本节点已有已办任务：%q", preview.SkipReason)
	}
}

// TestRetrievePrepareConsentNotSkippedWhenNoDone 验证「先同意再取回」：当前用户在本节点没有已办
// 任务时，取回准备同意不会被跳过，而是落到普通同意门禁按实时待办判定（无可同意待办则阻塞）。
func TestRetrievePrepareConsentNotSkippedWhenNoDone(t *testing.T) {
	targetFake := &taskActionFactsTarget{
		fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-done"}, DueNodes: []string{"node-done"}}},
		snapshot:   target.TaskSnapshot{}, // 无已办任务
		tree:       &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{ID: "node-done", Type: "common"}},
	}
	runCtx := prepConsentRunContext()
	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil {
		t.Fatalf("取回准备同意预览失败：finished=%v err=%v preview=%+v", finished, err, preview)
	}
	if preview.TargetSkipped {
		t.Fatalf("无已办时取回准备同意不应被跳过，应落到普通同意门禁：%+v", preview)
	}
}
