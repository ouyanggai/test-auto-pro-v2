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
// 已办任务且审核记录证实通过时，取回准备同意在预览阶段判为跳过，不发出任何写请求。
func TestRetrievePrepareConsentSkippedWhenDoneExists(t *testing.T) {
	targetFake := &taskActionFactsTarget{
		fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-done"}, DueNodes: []string{"node-done"}}},
		snapshot: target.TaskSnapshot{
			LinkID: "done-link", JobTaskID: "done-task", FlowNodeProxyID: "node-done",
			FlowProxyID: "proxy-current", TaskStatus: "done",
		},
		records: []target.AuditRecordSnapshot{{
			FlowJobTaskID: "done-link", FlowNodeProxyID: "node-done",
			ExecutorID: "user-handler", AuditStatus: "pass",
		}},
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
// TestRetrievePrepareConsentNotMistakenlySkippedByTargetSkippedStep 验证：
// 即使当前节点配置了 isSkip=true 且实例当前节点显示在后续节点，取回准备同意步骤
// 也绝不能被 targetSkippedStepReason 误跳过，保证准备同意能被正常裁决与执行。
func TestRetrievePrepareConsentNotMistakenlySkippedByTargetSkippedStep(t *testing.T) {
	isSkip := true
	runCtx := newRunContext([]model.CompiledActionStep{
		{
			Sequence: 1, Source: model.ActionStepSourceRecovery, Action: model.ActionApprove,
			Scope: model.ActionScopeTask, NodeKey: "node-audit", RetrievePrepareConsent: true,
		},
		{
			Sequence: 2, Source: model.ActionStepSourceUser, Action: model.ActionRetrieve,
			Scope: model.ActionScopeCompletedTask, NodeKey: "node-audit",
		},
	})
	runCtx.Source = "submitted"
	runCtx.PathRun.MainInstanceRef = "instance-retrieve-skip"
	runCtx.Nodes["node-audit"] = step.NodeInfo{
		Name: "审核人1", Type: "common", TargetNodeID: "node-1", IsSkip: &isSkip,
	}

	targetFake := &taskActionFactsTarget{
		fakeTarget: &fakeTarget{instance: fakeTargetView{
			Found: true, Status: "run", CurrentNodes: []string{"node-2"}, DueNodes: []string{"node-2"},
		}},
		snapshot: target.TaskSnapshot{}, // 无已办任务
	}
	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil {
		t.Fatalf("预览生成失败：%v", err)
	}
	// 准备同意绝不能被判为“实例待办已在后续节点，本节点已被目标自动跳过”
	if preview.TargetSkipped && strings.Contains(preview.SkipReason, "自动跳过") {
		t.Fatalf("取回准备同意绝不能被 targetSkippedStepReason 误判为目标自动跳过，实际 preview=%+v", preview)
	}
}

type followTrackingTarget struct {
	*taskActionFactsTarget
	isTracking bool
}

func (f *followTrackingTarget) ReadFlowTracking(context.Context, target.Session, string) (bool, bool, error) {
	return f.isTracking, true, nil
}

// TestFollowActionIdempotentSkipWhenAlreadyFollowed 验证：
// 当主实例当前已经处于跟踪状态时，执行“设为跟踪”（follow）动作判定为放行并幂等跳过（TargetSkipped=true），
// 记录明确原因，不发出重复写请求，绝不阻断停机。
func TestFollowActionIdempotentSkipWhenAlreadyFollowed(t *testing.T) {
	runCtx := newRunContext([]model.CompiledActionStep{
		{
			Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionFollow,
			Scope: model.ActionScopeInstance, NodeKey: "node-1",
		},
	})
	runCtx.Source = "submitted"
	runCtx.PathRun.MainInstanceRef = "instance-follow"
	runCtx.Nodes["node-1"] = step.NodeInfo{Name: "审核节点", Type: "common", TargetNodeID: "node-1"}

	targetFake := &followTrackingTarget{
		taskActionFactsTarget: &taskActionFactsTarget{
			fakeTarget: &fakeTarget{
				instance: fakeTargetView{
					Found: true, Status: "run", CurrentNodes: []string{"node-1"}, DueNodes: []string{"node-1"},
				},
			},
		},
		isTracking: true, // 已经处于跟踪状态
	}
	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil {
		t.Fatalf("设为跟踪预览构建失败：err=%v finished=%v preview=%+v", err, finished, preview)
	}
	if !preview.GateAllowed {
		t.Fatalf("已处于跟踪状态时应放行并幂等跳过，而不是拦截阻断：blockReason=%s", preview.BlockReason)
	}
	if !preview.TargetSkipped {
		t.Fatalf("已处于跟踪状态时应标记 TargetSkipped=true 跳过重复写入：preview=%+v", preview)
	}
	if !strings.Contains(preview.SkipReason, "跟踪") {
		t.Fatalf("跳过原因应说明已处于跟踪状态：%s", preview.SkipReason)
	}
}
// TestUrgeActionAllowedWhenInstanceHasCurrentHandlers 验证：
// 当发起人查询自己的待办为空（DueNodes 为空），但已发列表明确返回了当前待办人（CurrentHandlers）
// 且实例处于 run 状态时，催办动作的 HasPendingRecipient 必须判定为满足，顺利放行催办。
func TestUrgeActionAllowedWhenInstanceHasCurrentHandlers(t *testing.T) {
	runCtx := newRunContext([]model.CompiledActionStep{
		{
			Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionUrge,
			Scope: model.ActionScopeInstance, NodeKey: "node-1",
		},
	})
	runCtx.Source = "submitted"
	runCtx.PathRun.MainInstanceRef = "instance-urge"
	runCtx.Nodes["node-1"] = step.NodeInfo{Name: "审核节点", Type: "common", TargetNodeID: "node-1"}
	targetFake := &taskActionFactsTarget{
		fakeTarget: &fakeTarget{
			instance: fakeTargetView{
				Found: true, Status: "run", CurrentNodes: []string{"node-1"},
				DueNodes: []string{}, // 发起人自己无待办，但实例处于 run 状态且停留在 node-1
			},
		},
	}
	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil {
		t.Fatalf("催办预览构建失败：err=%v finished=%v preview=%+v", err, finished, preview)
	}
	if !preview.GateAllowed {
		t.Fatalf("有当前待办接收人时催办应顺利放行，实际被拦截：blockReason=%s", preview.BlockReason)
	}
}
