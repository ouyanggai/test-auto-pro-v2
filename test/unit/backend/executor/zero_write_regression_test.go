package executor_test

import (
	"context"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// reviewTarget 是零写入复核用目标假件：同意步骤的待办新鲜读取可配置为失败或空结果，
// 写方法一旦被调用即记账，用于证明「没有发出写请求」这一事实。
type reviewTarget struct {
	dueTaskID   string
	dueTaskErr  error
	submitCalls int
	auditCalls  int
	actionCalls int
	actionErr   error
}

// FindSubmittedFlow 返回已发起且在运行中的实例事实。
func (r *reviewTarget) FindSubmittedFlow(context.Context, target.Session, string) (string, []string, string, []string, bool, error) {
	return "flow-proxy-1", []string{"node-audit"}, "run", nil, true, nil
}

// FindDueFlow 返回本节点仍有待办的事实。
func (r *reviewTarget) FindDueFlow(context.Context, target.Session, string) (string, []string, []string, bool, error) {
	// F-035：返回非空 DueNodes，保持「有待办但任务读取失败/缺失」的原语义；
	// 空待办属于“处理人未生成”分型，不再与零写入回归混用。
	return "flow-proxy-1", []string{"node-audit"}, []string{"node-audit"}, true, nil
}

// ReadInstanceCurrentData 复核用假件不预设实例表单数据。
func (r *reviewTarget) ReadInstanceCurrentData(context.Context, target.Session, string) (map[string]any, error) {
	return nil, nil
}

// FindDueTaskID 按预设返回待办任务 ID 或读取错误（只读，发生在写请求之前）。
func (r *reviewTarget) FindDueTaskID(context.Context, target.Session, string, string) (string, error) {
	return r.dueTaskID, r.dueTaskErr
}

// SubmitFlowInstance 记账发起写调用。
func (r *reviewTarget) SubmitFlowInstance(context.Context, target.Session, target.SubmitFlowInstanceRequest) (*target.SubmitFlowInstanceResult, target.WriteResponse, string, error) {
	r.submitCalls++
	return &target.SubmitFlowInstanceResult{InstanceID: "i-1", Status: "run"}, target.WriteResponse{StatusCode: 200, IsSuccess: true, IsSuccessPresent: true}, "t-submit", nil
}

// AuditCurrentTask 记账同意写调用。
func (r *reviewTarget) AuditCurrentTask(context.Context, target.Session, target.AuditCurrentTaskRequest) (*target.AuditCurrentTaskResult, target.WriteResponse, string, error) {
	r.auditCalls++
	return &target.AuditCurrentTaskResult{InstanceID: "i-1"}, target.WriteResponse{StatusCode: 200, IsSuccess: true, IsSuccessPresent: true}, "t-audit", nil
}

// ExecuteActionWrite 记录统一动作写出口调用，按用例返回目标假件结果。
func (r *reviewTarget) ExecuteActionWrite(context.Context, target.Session, target.ActionWriteRequest) (target.WriteResponse, string, error) {
	r.actionCalls++
	return target.WriteResponse{}, "", r.actionErr
}

// runReviewApprove 跑一次同意步骤的放行，返回状态机假件、事实假件与目标假件。
func runReviewApprove(t *testing.T, targetFake *reviewTarget) (*fakeRunState, *fakeFacts, *reviewTarget) {
	t.Helper()
	state := &fakeRunState{}
	facts := &fakeFacts{}
	executor := step.NewExecutor(targetFake, &fakeSessions{}, state, facts, fixedRunConfig(), func() time.Time { return time.Unix(0, 0).UTC() })
	runCtx := newRunContext([]model.CompiledActionStep{approveStep()})
	runCtx.PathRun.MainInstanceRef = "i-1"
	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished {
		t.Fatalf("预览构造失败：err=%v finished=%v", err, finished)
	}
	if preview.BlockReason != "" {
		t.Fatalf("门禁应通过，实际被阻塞：%s", preview.BlockReason)
	}
	if _, _, err := executor.RunApprovedStep(context.Background(), step.ApprovedStep{RunCtx: runCtx, Preview: preview, NextIndex: 0}); err != nil {
		t.Fatalf("放行失败：%v", err)
	}
	return state, facts, targetFake
}

// TestF016PreWriteReadFailureSettlesAsZeroWriteFailure 复核评审缺陷 3 的修复回归：
// 同意步骤在写请求之前的待办新鲜读取失败时，写请求从未发出，
// 路径运行必须按「演员不可解析」确定失败，绝不进入待对账（零写入不存在写结果不确定）。
func TestF016PreWriteReadFailureSettlesAsZeroWriteFailure(t *testing.T) {
	state, facts, fake := runReviewApprove(t, &reviewTarget{dueTaskErr: target.NewError(target.ErrorTimeout, nil)})
	// 门禁结论快照必须随步骤落账（评审缺陷 10）：侧栏据此还原当时的门禁判定。
	if len(facts.steps) != 1 || facts.steps[0].GateSnapshot == "" {
		t.Fatalf("步骤记录应携带门禁快照，实际 %+v", facts.steps)
	}
	if fake.auditCalls != 0 {
		t.Fatalf("本用例前提是写请求从未发出，实际调用 %d 次", fake.auditCalls)
	}
	if len(state.finishedTo) == 0 || state.finishedTo[len(state.finishedTo)-1] != model.PathRunStatusFailed {
		t.Fatalf("零写入必须确定失败，实际终态 %v", state.finishedTo)
	}
	for _, class := range state.finishClasses {
		if class != model.FailureClassActorUnresolved {
			t.Fatalf("零写入失败分类应为演员/待办解析失败，实际 %v", state.finishClasses)
		}
	}
}

// TestF016MissingDueTaskSettlesAsZeroWriteFailure 复核：目标上已无本节点待办时
// 执行器正确地不发写请求，路径运行同样按零写入确定失败而不是待对账。
func TestF016MissingDueTaskSettlesAsZeroWriteFailure(t *testing.T) {
	state, _, fake := runReviewApprove(t, &reviewTarget{dueTaskID: ""})
	if fake.auditCalls != 0 {
		t.Fatalf("没有待办时绝不能发写请求，实际调用 %d 次", fake.auditCalls)
	}
	if len(state.finishedTo) == 0 || state.finishedTo[len(state.finishedTo)-1] != model.PathRunStatusFailed {
		t.Fatalf("零写入必须确定失败，实际终态 %v", state.finishedTo)
	}
	for _, class := range state.finishClasses {
		if class != model.FailureClassActorUnresolved {
			t.Fatalf("零写入失败分类应为演员/待办解析失败，实际 %v", state.finishClasses)
		}
	}
}

// TestF019LocalActionValidationSettlesAsZeroWriteFailure 验证统一动作出口的本地载荷校验失败
// 不会被执行器误记为已发出写请求，也不会把路径运行推进到待对账。
func TestF019LocalActionValidationSettlesAsZeroWriteFailure(t *testing.T) {
	fake := &reviewTarget{actionErr: &target.RequestValidationError{Message: "流程实例 id不能为空，拒绝发送"}}
	state := &fakeRunState{}
	facts := &fakeFacts{}
	executor := step.NewExecutor(fake, &fakeSessions{}, state, facts, fixedRunConfig(), func() time.Time { return time.Unix(0, 0).UTC() })
	runCtx := newRunContext([]model.CompiledActionStep{{
		Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionFollow,
		Scope: model.ActionScopeInstance, NodeKey: "node-audit",
	}})
	runCtx.PathRun.MainInstanceRef = "instance-1"
	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil || !preview.GateAllowed {
		t.Fatalf("关注动作预览应通过：finished=%v err=%v preview=%+v", finished, err, preview)
	}
	if _, _, err := executor.RunApprovedStep(context.Background(), step.ApprovedStep{RunCtx: runCtx, Preview: preview, NextIndex: 0}); err != nil {
		t.Fatalf("执行本地校验失败动作不应返回控制错误：%v", err)
	}
	if fake.actionCalls != 1 {
		t.Fatalf("统一动作出口应只被调用一次，实际 %d 次", fake.actionCalls)
	}
	if len(state.finishedTo) == 0 || state.finishedTo[len(state.finishedTo)-1] != model.PathRunStatusFailed {
		t.Fatalf("本地校验失败必须确定置败，实际终态 %v", state.finishedTo)
	}
	for _, class := range state.finishClasses {
		if class != model.FailureClassToolBug {
			t.Fatalf("本地校验失败应归为工具缺陷，实际 %v", state.finishClasses)
		}
	}
	if len(facts.attempts) != 1 || facts.attempts[0].SideEffect != "none" {
		t.Fatalf("本地校验失败应记录零副作用尝试，实际 %+v", facts.attempts)
	}
	if facts.attempts[0].Verdict != "confirmed_failure" {
		t.Fatalf("本地校验失败不得记录为待确认，实际 %+v", facts.attempts[0])
	}
}
