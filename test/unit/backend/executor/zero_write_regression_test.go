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
}

// FindSubmittedFlow 返回已发起且在运行中的实例事实。
func (r *reviewTarget) FindSubmittedFlow(context.Context, target.Session, string) (string, []string, string, []string, bool, error) {
	return "flow-proxy-1", []string{"node-audit"}, "run", nil, true, nil
}

// FindDueFlow 返回本节点仍有待办的事实。
func (r *reviewTarget) FindDueFlow(context.Context, target.Session, string) (string, []string, []string, bool, error) {
	return "flow-proxy-1", []string{"node-audit"}, nil, true, nil
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

// FindDueFlowActions 满足 TargetClient 其余方法的空实现（本复核不涉及）。
func (r *reviewTarget) ExecuteActionWrite(context.Context, target.Session, target.ActionWriteRequest) (target.WriteResponse, string, error) {
	return target.WriteResponse{}, "", nil
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
