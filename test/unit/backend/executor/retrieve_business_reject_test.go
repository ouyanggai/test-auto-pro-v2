package executor_test

import (
	"context"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/engine/verdict"
	"test-auto-pro-v2/internal/model"
)

// retrieveRejectTarget 模拟目标对取回返回明确业务拒绝（如「下一节点已被处理,不支持取回」）。
type retrieveRejectTarget struct {
	*taskActionFactsTarget
	rejectMessage string
	writeCalls    int
}

func (t *retrieveRejectTarget) FindTaskSnapshot(_ context.Context, _ target.Session, _ string, _ string, status string) (target.TaskSnapshot, error) {
	if status == "done" {
		return target.TaskSnapshot{LinkID: "done-link", JobTaskID: "done-task", FlowNodeProxyID: "node-done", FlowProxyID: "proxy-current", TaskStatus: "done"}, nil
	}
	return target.TaskSnapshot{}, nil
}

func (t *retrieveRejectTarget) ExecuteActionWrite(context.Context, target.Session, target.ActionWriteRequest) (target.WriteResponse, string, error) {
	t.writeCalls++
	// 目标返回完整响应且明确拒绝：适配层以 BusinessRejection 上抛，同时带回 IsSuccessPresent 响应。
	return target.WriteResponse{StatusCode: 200, IsSuccessPresent: true, IsSuccess: false, Code: "FLOW_409", Message: t.rejectMessage},
		"trace-retrieve-reject", &target.BusinessRejection{Code: "FLOW_409", Message: t.rejectMessage}
}

// TestRetrieveBusinessRejectionSkipsAndContinues 锁定用户裁决：目标对取回的明确业务拒绝
// （如「下一节点已被处理,不支持取回」）是静态可预期的编排冲突、且确定无副作用——不阻断整条运行，
// 按「警告并跳过该动作」处理并继续，尝试记录里保留目标原话。绝不判「结果待确认」，也不整条失败中断。
func TestRetrieveBusinessRejectionSkipsAndContinues(t *testing.T) {
	rejectMsg := "下一节点已被处理,不支持取回"
	targetFake := &retrieveRejectTarget{
		taskActionFactsTarget: &taskActionFactsTarget{
			fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-done"}, DueNodes: []string{"node-done"}}},
			tree: &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{
				ID: "node-done", Type: "common", Child: &target.FlowNodeTemplate{ID: "node-next", Type: "common"},
			}},
		},
		rejectMessage: rejectMsg,
	}
	runCtx := newRunContext([]model.CompiledActionStep{{
		Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionRetrieve,
		Scope: model.ActionScopeCompletedTask, NodeKey: "node-audit",
	}})
	runCtx.Source = "done"
	runCtx.PathRun.MainInstanceRef = "instance-retrieve"
	runCtx.Nodes["node-audit"] = step.NodeInfo{Name: "已办审批", Type: "common", TargetNodeID: "node-done"}
	facts := &fakeFacts{}
	runState := &fakeRunState{}
	executor := step.NewExecutor(targetFake, &fakeSessions{}, runState, facts, fixedRunConfig(), nil)

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil || !preview.GateAllowed {
		t.Fatalf("取回前置事实应通过：finished=%v err=%v preview=%+v", finished, err, preview)
	}
	outcome, _, err := executor.RunApprovedStep(context.Background(), step.ApprovedStep{RunCtx: runCtx, Preview: preview, NextIndex: 0})
	if err != nil {
		t.Fatalf("取回执行返回错误：%v", err)
	}
	// 预期内失败：不整条中断，标记跳过并继续（本例只有一步 → 跳过后收尾）。
	if outcome.Verdict != string(verdict.OutcomeSucceeded) {
		t.Fatalf("预期内取回拒绝应跳过并继续（不阻断），实际 verdict=%s", outcome.Verdict)
	}
	if len(facts.attempts) != 1 {
		t.Fatalf("应恰好记录一次尝试，实际 %+v", facts.attempts)
	}
	if facts.attempts[0].Verdict != step.StepVerdictSkippedWarning {
		t.Fatalf("尝试应记为「警告并跳过」，实际 verdict=%s", facts.attempts[0].Verdict)
	}
	if facts.attempts[0].SideEffect != string(verdict.SideEffectNone) {
		t.Fatalf("确定无副作用才允许跳过，实际 sideEffect=%s", facts.attempts[0].SideEffect)
	}
	if !strings.Contains(facts.attempts[0].Reason, rejectMsg) {
		t.Fatalf("跳过原因必须带目标原话「%s」，实际：%s", rejectMsg, facts.attempts[0].Reason)
	}
	// 跳过后必须释放本步推进权，否则下一步 ClaimExecution 会拿到「路径运行已被其他执行者占用」而卡住。
	if runState.released < 1 {
		t.Fatalf("跳过分支必须释放执行租约，实际 released=%d", runState.released)
	}
}
