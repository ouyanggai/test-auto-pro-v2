package executor_test

import (
	"context"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// 2026-09-21 运行 93 路径 3 第 6 步缺陷回归：取回门禁不能只信按节点过滤的已办行。
//
// 现象：第 5 步「取回准备同意」以本节点真实处理人身份同意成功，第 6 步取回却停在写前
// 「当前用户没有可取回的已完成任务」，重跑结果相同。
//
// 根因：目标在本节点办结后，把任务关联行的 flowNodeProxyId 改写成"实例流转到的下一节点"。
// 实测同一条 jobTaskId 858ded51… 在 pending 时是发起人 a408500c…，
// 03:10:19 办结后变成省区经理 b9fc434e…；而 08e604bf…（计划账号）的已办查询
// target_rows=1/matched_rows=1 明明命中，只是节点对不上本步节点，被等值过滤丢弃。
// 门禁因此把"确实已办结"误判成"没有已办"，把可取回的实例判成不可取。
//
// 修复口径：审核记录是本节点「确实已被办结」的权威正面分型，节点字段不被改写。
// 已办行读空但审核记录确有本节点 pass 时，采用审核记录作为已办事实（含办结执行人）。
// 安全边界：只有确实读到本节点 pass 记录才补；读不到审核记录时保持读空并按既有门禁阻塞。

// retrieveNodePassRunContext 构造一条位于「node-audit」的取回步骤运行上下文。
// 目标节点 ID 固定为 node-audit：真实场景里本步配置节点的目标 ID 不随实例流转改写。
func retrieveNodePassRunContext() step.RunContext {
	runCtx := newRunContext([]model.CompiledActionStep{{
		Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionRetrieve,
		Scope: model.ActionScopeCompletedTask, NodeKey: "node-audit",
	}})
	runCtx.Source = "done"
	runCtx.PathRun.MainInstanceRef = "instance-retrieve"
	runCtx.Nodes["node-audit"] = step.NodeInfo{Name: "考勤管理", Type: "common", TargetNodeID: "node-audit"}
	return runCtx
}

// TestRetrieveGateAcceptsAuditRecordWhenDoneRowNodeWasRewritten 锁定主修复路径：
// 已办行按节点过滤读空（目标把节点改写成了下一节点），但审核记录证明本节点确已办结且给出了执行人，
// 取回门禁必须放行，并以审核记录里的执行人作为取回身份（取回由该办结人本人发出）。
func TestRetrieveGateAcceptsAuditRecordWhenDoneRowNodeWasRewritten(t *testing.T) {
	targetFake := &taskActionFactsTarget{
		fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-current"}}},
		// 已办行读空：真实场景里节点过滤把命中行丢掉了。
		snapshot: target.TaskSnapshot{},
		records: []target.AuditRecordSnapshot{{
			FlowJobTaskID: "done-link", FlowNodeProxyID: "node-audit",
			ExecutorID: "handler-id", ExecutorName: "刘慧玲", AuditStatus: "pass",
		}},
		tree: &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{
			ID: "node-audit", Type: "common", Child: &target.FlowNodeTemplate{ID: "node-current", Type: "common"},
		}},
	}
	runCtx := retrieveNodePassRunContext()
	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil {
		t.Fatalf("取回预览失败：finished=%v err=%v preview=%+v", finished, err, preview)
	}
	if !preview.GateAllowed {
		t.Fatalf("审核记录证明本节点已办结时取回应放行，实际被拒：%+v", preview.GateItems)
	}
	if preview.Facts.CompletedTaskBasis != step.CompletedTaskBasisOfForTest(true, true, false) {
		t.Fatalf("已办依据应为「确实核对并命中」，实际：%q", preview.Facts.CompletedTaskBasis)
	}
	if preview.Facts.CompletedTaskNodeID != "node-audit" {
		t.Fatalf("已办节点应按审核记录锁定为本步节点，实际：%q", preview.Facts.CompletedTaskNodeID)
	}
	if preview.Facts.CompletedTaskExecutorID != "handler-id" {
		t.Fatalf("取回身份应为办结该节点的真实执行人，实际：%q", preview.Facts.CompletedTaskExecutorID)
	}
}

// TestRetrieveGateStaysBlockedWithoutAuditEvidence 锁定保守边界：
// 已办行读空且审核记录里没有本节点的通过记录时，仍按「没有可取回的已完成任务」阻塞，
// 绝不用其他节点的已办或未知事实顶替。
func TestRetrieveGateStaysBlockedWithoutAuditEvidence(t *testing.T) {
	targetFake := &taskActionFactsTarget{
		fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-current"}}},
		snapshot:   target.TaskSnapshot{},
		records: []target.AuditRecordSnapshot{{
			// 通过记录属于另一个节点：不能用来证明本节点已办结。
			FlowJobTaskID: "other-link", FlowNodeProxyID: "node-elsewhere",
			ExecutorID: "handler-id", ExecutorName: "某人", AuditStatus: "pass",
		}},
		tree: &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{
			ID: "node-audit", Type: "common", Child: &target.FlowNodeTemplate{ID: "node-current", Type: "common"},
		}},
	}
	runCtx := retrieveNodePassRunContext()
	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil {
		t.Fatalf("取回预览失败：finished=%v err=%v preview=%+v", finished, err, preview)
	}
	if preview.GateAllowed {
		t.Fatalf("没有本节点已办证据时不得放行取回：%+v", preview)
	}
}
