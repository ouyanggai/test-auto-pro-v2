package runtimeexec

import (
	"context"
	"encoding/json"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// TestForwardRuntimeUsesCurrentTaskAndExactMessage 验证实例级转发按当前任务装载，保留空附言且不执行审批字段补丁。
func TestForwardRuntimeUsesCurrentTaskAndExactMessage(t *testing.T) {
	for _, message := range []string{"", " 本次转发附言 "} {
		view := &recordingTarget{taskID: "live-forward-task"}
		runner := &fakeRuntimeRunner{status: "success", forwardID: "forward-created"}
		executor := newExecutorForRouting(t, view, runner)
		runCtx := runtimeRunCtx()
		runCtx.PathRun.MainInstanceRef = "original-main"
		runCtx.ActionPersonIDs = map[string][]string{step.ActionPersonIndex("", model.ActionForward): {"receiver-1"}}
		runCtx.FormPersonFields = []target.NodeFormPersonField{{NodeID: "other-node", Field: "nextPerson__formPersonId"}}
		runCtx.BranchSelections = map[string]string{"other-branch": "wrong-entry"}
		runCtx.GraphEdges = map[string][]step.GraphEdgeInfo{"other-branch": {{Target: "different-entry"}}}
		compiled := model.CompiledActionStep{Action: model.ActionForward, Scope: model.ActionScopeInstance, Parameters: map[string]any{"approveMessage": message, "forwardCheckList": []string{"fuyan"}}}
		preview := &step.StepPreview{Action: model.ActionForward, ActorAccount: "actor-acct", Facts: step.InstanceFacts{CurrentTaskNodeID: "live-node"}, FormOverlaid: []string{"nextPerson"}}
		executor.RunViaRuntimeForTest(context.Background(), runCtx, compiled, target.Session{}, preview, step.ApprovedStep{Attempt: 1})
		request := runner.lastRequest()
		if runner.callCount() != 1 || request.JobTaskID != "live-forward-task" || request.ReceiverID != "receiver-1" || request.InstanceID != "original-main" || request.AuditDesc != message {
			t.Fatalf("转发输入串号或空附言被替换：%+v", request)
		}
		if len(request.ChangedFields) != 0 || len(request.ReadonlyFields) != 0 || len(request.BranchSelections) != 0 || len(request.NextNodeAuditors) != 0 {
			t.Fatalf("转发不得触发审批补丁或下一节点选人：%+v", request)
		}
		if len(view.writeCalls()) != 0 || runCtx.PathRun.MainInstanceRef != "original-main" {
			t.Fatal("转发不能调用直连写或改掉原主实例")
		}
		if got := step.ForwardAuxiliaryInstanceIDForTest(preview.WriteResponseForTest().Data); got != "forward-created" {
			t.Fatalf("转发新实例必须回传：%s", got)
		}
	}
}

// TestForwardRuntimeRetainsCreatedInstanceAfterPostscriptFailure 验证后置保存失败或未知时仍保留第一笔写入的关联实例。
func TestForwardRuntimeRetainsCreatedInstanceAfterPostscriptFailure(t *testing.T) {
	for _, status := range []string{"business_failed", "uncertain", "timeout"} {
		view := &recordingTarget{taskID: "live-forward-task"}
		runner := &fakeRuntimeRunner{status: status, forwardID: "already-created", summary: json.RawMessage(`{"forwardInstanceId":"already-created"}`)}
		executor := newExecutorForRouting(t, view, runner)
		runCtx := runtimeRunCtx()
		runCtx.PathRun.MainInstanceRef = "original-main"
		runCtx.ActionPersonIDs = map[string][]string{step.ActionPersonIndex("node-1", model.ActionForward): {"receiver-1"}}
		preview := &step.StepPreview{Action: model.ActionForward, ActorAccount: "actor-acct"}
		executor.RunViaRuntimeForTest(context.Background(), runCtx, runtimeStep(model.ActionForward), target.Session{}, preview, step.ApprovedStep{Attempt: 1})
		if !preview.WriteSent() || preview.WriteErrForTest() == nil || step.ForwardAuxiliaryInstanceIDForTest(preview.WriteResponseForTest().Data) != "already-created" {
			t.Fatalf("%s 不得丢掉已写入实例或报零写入", status)
		}
	}
}
