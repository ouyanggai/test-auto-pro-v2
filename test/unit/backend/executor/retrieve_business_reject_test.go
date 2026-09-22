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
		return target.TaskSnapshot{
			LinkID: "done-link", JobTaskID: "done-task", FlowNodeProxyID: "node-done",
			FlowProxyID: "proxy-current", TaskStatus: "done", ExecutorID: "user-handler",
		}, nil
	}
	return target.TaskSnapshot{}, nil
}

func (t *retrieveRejectTarget) ExecuteActionWrite(context.Context, target.Session, target.ActionWriteRequest) (target.WriteResponse, string, error) {
	t.writeCalls++
	// 目标返回完整响应且明确拒绝：适配层以 BusinessRejection 上抛，同时带回 IsSuccessPresent 响应。
	return target.WriteResponse{StatusCode: 200, IsSuccessPresent: true, IsSuccess: false, Code: "FLOW_409", Message: t.rejectMessage},
		"trace-retrieve-reject", &target.BusinessRejection{Code: "FLOW_409", Message: t.rejectMessage}
}

// TestRetrieveBusinessRejectionFailsWithoutSkip 锁定用户裁决：目标对取回的明确业务拒绝
// （如「下一节点已被处理,不支持取回」）属于恢复链动作的真实业务失败，绝不能伪装为「警告并跳过」。
// 必须如实判为确定失败（OutcomeFailed），保留目标原话，并把路径运行置为失败终态。
func TestRetrieveBusinessRejectionFailsWithoutSkip(t *testing.T) {
	rejectMsg := "下一节点已被处理,不支持取回"
	targetFake := &retrieveRejectTarget{
		taskActionFactsTarget: &taskActionFactsTarget{
			fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-done"}, DueNodes: []string{"node-done"}}},
			records: []target.AuditRecordSnapshot{{
				FlowJobTaskID: "done-link", FlowNodeProxyID: "node-done",
				ExecutorID: "user-handler", AuditStatus: "pass",
			}},
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
	// 取回失败必须如实判定为失败，不得伪装跳过并声称成功
	if outcome.Verdict != string(verdict.OutcomeFailed) {
		t.Fatalf("取回被目标明确拒绝时必须判确定失败，实际 verdict=%s", outcome.Verdict)
	}
	if len(facts.attempts) != 1 {
		t.Fatalf("应恰好记录一次尝试，实际 %+v", facts.attempts)
	}
	if facts.attempts[0].Verdict != string(verdict.OutcomeFailed) {
		t.Fatalf("尝试应记为确定失败，实际 verdict=%s", facts.attempts[0].Verdict)
	}
	if !strings.Contains(facts.attempts[0].Reason, rejectMsg) {
		t.Fatalf("失败原因必须带目标原话「%s」，实际「%s」", rejectMsg, facts.attempts[0].Reason)
	}
	// 失败后路径运行必须置为失败终态
	if len(runState.finishedTo) != 1 || runState.finishedTo[0] != model.PathRunStatusFailed {
		t.Fatalf("失败分支必须将路径运行置为失败终态，实际 finishedTo=%v", runState.finishedTo)
	}
}
