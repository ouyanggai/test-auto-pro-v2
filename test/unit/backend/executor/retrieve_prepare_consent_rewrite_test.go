package executor_test

import (
	"context"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
)

// 2026-09-21 运行 100/101 路径 5 第 12 步缺陷回归：取回准备同意的跳过判据不能只看"读到了已办"。
//
// 现象：第 12 步（取回准备同意）以「当前用户在本节点已有已办任务」为理由跳过，
// 紧接着第 13 步取回目标指向一个从未被审批过的节点（省区经理），写请求虽然返回成功，
// 实例却停在原地，第 14 步报「当前待办已经处理，不能重复执行」而停止整条路径。
//
// 根因：跳过判据只要求 done 读取命中，而 done 读取按本步节点等值过滤的同时，
// 目标会在本节点办结后把该任务关联行的 flowNodeProxyID 改写成"实例流转到的下一节点"。
// 于是命中的那一行可能根本不是本节点的已办，"读到了已办"被误当成"本节点办结过"。
//
// 修复口径：跳过必须有明确证明——要么该行就留在本步节点上（目标未改写节点），
// 要么审核记录正面证明本节点存在通过记录。两者都不成立时不得跳过，
// 落到普通同意门禁按实时待办裁决。

// prepConsentRewriteRunContext 构造一条取回准备同意步骤，节点目标 ID 为 node-audit。
func prepConsentRewriteRunContext() step.RunContext {
	runCtx := prepConsentRunContext()
	runCtx.Nodes["node-audit"] = step.NodeInfo{Name: "省区经理", Type: "common", TargetNodeID: "node-audit"}
	return runCtx
}

// TestRetrievePrepareConsentNotSkippedWhenDoneRowWasRewritten 锁定主修复路径：
// 已办行读到了，但该行的节点已被目标改写、且审核记录里没有本节点的通过记录，
// 此时不得跳过准备同意（否则后续取回会指向没有已办任务的节点）。
func TestRetrievePrepareConsentNotSkippedWhenDoneRowWasRewritten(t *testing.T) {
	targetFake := &taskActionFactsTarget{
		fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-current"}, DueNodes: []string{"node-current"}}},
		// 行读到了，但节点是"流转后的下一节点"，不是本步节点。
		snapshot: target.TaskSnapshot{
			LinkID: "done-link", JobTaskID: "done-task", FlowNodeProxyID: "node-current",
			FlowProxyID: "proxy-current", TaskStatus: "done",
		},
		// 审核记录里只有别的节点的通过记录，没有任何本步节点 node-audit 的记录。
		records: []target.AuditRecordSnapshot{{
			FlowJobTaskID: "other-link", FlowNodeProxyID: "node-elsewhere",
			ExecutorID: "someone", AuditStatus: "pass",
		}},
		tree: &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{
			ID: "node-audit", Type: "common", Child: &target.FlowNodeTemplate{ID: "node-current", Type: "common"},
		}},
	}
	runCtx := prepConsentRewriteRunContext()
	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil {
		t.Fatalf("取回准备同意预览失败：finished=%v err=%v preview=%+v", finished, err, preview)
	}
	if preview.TargetSkipped {
		t.Fatalf("已办行节点被改写且审核记录无本节点通过时不得跳过准备同意：%+v", preview)
	}
}

// TestRetrievePrepareConsentSkippedWhenAuditProvesNodeHandled 锁定互补路径：
// 已办行节点虽被改写，但审核记录确实有本步节点的通过记录时，仍应跳过准备同意（不多发写请求）。
func TestRetrievePrepareConsentSkippedWhenAuditProvesNodeHandled(t *testing.T) {
	targetFake := &taskActionFactsTarget{
		fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-current"}, DueNodes: []string{"node-current"}}},
		snapshot: target.TaskSnapshot{
			LinkID: "done-link", JobTaskID: "done-task", FlowNodeProxyID: "node-current",
			FlowProxyID: "proxy-current", TaskStatus: "done",
		},
		records: []target.AuditRecordSnapshot{{
			FlowJobTaskID: "done-link", FlowNodeProxyID: "node-audit",
			ExecutorID: "handler-id", ExecutorName: "刘慧玲", AuditStatus: "pass",
		}},
		tree: &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{
			ID: "node-audit", Type: "common", Child: &target.FlowNodeTemplate{ID: "node-current", Type: "common"},
		}},
	}
	runCtx := prepConsentRewriteRunContext()
	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil {
		t.Fatalf("取回准备同意预览失败：finished=%v err=%v preview=%+v", finished, err, preview)
	}
	if !preview.TargetSkipped {
		t.Fatalf("审核记录证明本节点已办结时应跳过准备同意：%+v", preview)
	}
}
