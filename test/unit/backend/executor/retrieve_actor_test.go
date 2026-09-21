package executor_test

import (
	"context"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// TestRetrieveActorResolvesToNodeApprover 锁定用户裁决与目标 saveRetrieveProcess 语义：
// 取回必须以「该办结节点的真实审批人」（审核记录里最近一次 pass 的 executorId）身份发出，
// 绝不能用计划账号/发起人顶替——目标按 executorId 校验归属，否则报「当前办结节点不是该用户审批」。
func TestRetrieveActorResolvesToNodeApprover(t *testing.T) {
	targetFake := &taskActionFactsTarget{
		fakeTarget: &fakeTarget{instance: fakeTargetView{
			Found: true, Status: "run", CurrentNodes: []string{"node-current"}, DueNodes: []string{"node-current"},
		}},
		// 该已办节点存在 pending/done 任务快照，取回读到它作为完成事实。
		snapshot: target.TaskSnapshot{
			LinkID: "done-link", JobTaskID: "done-task", FlowNodeProxyID: "node-done",
			FlowProxyID: "proxy-current", TaskStatus: "done",
		},
		// 审核记录证明 node-done 由处理人 user-handler 审批通过（不是计划账号）。
		records: []target.AuditRecordSnapshot{
			{FlowJobTaskID: "done-link", FlowInstanceID: "instance-retrieve", FlowNodeProxyID: "node-done", ExecutorID: "user-handler", AuditStatus: "pass"},
		},
		tree: &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{
			ID: "node-done", Type: "common", Child: &target.FlowNodeTemplate{ID: "node-current", Type: "common"},
		}},
	}
	runCtx := newRunContext([]model.CompiledActionStep{{
		Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionRetrieve,
		Scope: model.ActionScopeCompletedTask, NodeKey: "node-audit",
	}})
	runCtx.Source = "done"
	runCtx.PathRun.MainInstanceRef = "instance-retrieve"
	runCtx.Nodes["node-audit"] = step.NodeInfo{Name: "已办审批", Type: "common", TargetNodeID: "node-done"}
	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil {
		t.Fatalf("取回预览失败：finished=%v err=%v preview=%+v", finished, err, preview)
	}
	if preview.Facts.CompletedTaskExecutorID != "user-handler" {
		t.Fatalf("取回节点审批人应解析为 user-handler，实际 %q", preview.Facts.CompletedTaskExecutorID)
	}
	// 取回执行人必须交给写路径的处理人切换逻辑：CurrentTaskAssigneeID = 该节点真实审批人。
	if preview.Facts.CurrentTaskAssigneeID != "user-handler" {
		t.Fatalf("取回应以节点审批人身份发出（CurrentTaskAssigneeID=user-handler），实际 %q（计划账号顶替是错的）", preview.Facts.CurrentTaskAssigneeID)
	}
	if preview.ActorName != "user-handler" {
		t.Fatalf("目标只返回实际处理人 ID 时，取回预览也应保留执行人信息，实际 %q", preview.ActorName)
	}
}
