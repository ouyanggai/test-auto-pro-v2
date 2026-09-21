package executor_test

import (
	"context"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// 2026-09-21 运行 94 路径 3 第 6 步缺陷回归：取回写前的已办行刷新也不能只按节点过滤。
//
// 现象：门禁已按审核记录放行取回，写前刷新报
// 「目标上已无当前处理人在本节点的已办任务，无法执行取回」——放行与写前互相打架。
//
// 根因：与门禁同一处目标事实——本节点办结后，目标把该任务关联行的 flowNodeProxyID 改写成
// "实例流转到的下一节点"。门禁已改用审核记录补救，写前的已办刷新仍在用本步节点等值过滤，必然读空。
//
// 修复口径：写前同样用审核记录里本节点最近一次「通过」的关联行 ID 去已办列表取回同一行。
// 安全边界：没有本节点通过记录、或按关联行 ID 匹配不到唯一一行时保持阻塞，绝不用其他节点的已办顶替。

// retrieveRewriteFacts 构造"已办行节点被目标改写"的目标假件：按节点过滤读不回来，按关联行 ID 能读回来。
func retrieveRewriteFacts() *retrieveRewriteTarget {
	return &retrieveRewriteTarget{
		taskActionFactsTarget: &taskActionFactsTarget{
			fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-current"}}},
			records: []target.AuditRecordSnapshot{{
				FlowJobTaskID: "done-link", FlowNodeProxyID: "node-audit",
				ExecutorID: "handler-id", ExecutorName: "刘慧玲", AuditStatus: "pass",
			}},
			tree: &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{
				ID: "node-audit", Type: "common", Child: &target.FlowNodeTemplate{ID: "node-current", Type: "common"},
			}},
		},
	}
}

// retrieveRewriteTarget 模拟"已办行节点被目标改写"：按节点过滤读不回来，按关联行 ID 能读回来。
type retrieveRewriteTarget struct {
	*taskActionFactsTarget
	writeCalls  int
	writeTaskID string
}

// FindTaskSnapshot 模拟真实目标：请求节点与已办行节点不一致时读空。
func (t *retrieveRewriteTarget) FindTaskSnapshot(_ context.Context, _ target.Session, _, nodeProxyID, status string) (target.TaskSnapshot, error) {
	t.snapshotRead++
	return target.TaskSnapshot{}, nil
}

// ListTaskSnapshots 返回本账号在本实例上的已办行：节点已被改写成流转后的节点。
func (t *retrieveRewriteTarget) ListTaskSnapshots(context.Context, target.Session, string, string) ([]target.TaskSnapshot, error) {
	return []target.TaskSnapshot{{
		LinkID: "done-link", JobTaskID: "done-task", FlowNodeProxyID: "node-rewritten",
		FlowProxyID: "proxy-current", TaskStatus: "done",
	}}, nil
}

// ExecuteActionWrite 记录取回写请求绑定的任务号，供断言使用。
func (t *retrieveRewriteTarget) ExecuteActionWrite(_ context.Context, _ target.Session, request target.ActionWriteRequest) (target.WriteResponse, string, error) {
	t.writeCalls++
	t.writeTaskID = request.JobTaskID
	return target.WriteResponse{StatusCode: 200, IsSuccess: true, IsSuccessPresent: true}, "trace-retrieve", nil
}

// TestRetrieveWriteBindsDoneRowByAuditLinkNotNode 锁定：写前刷新应按审核记录的关联行 ID
// 找回被改写节点的已办行，绑定到该行的真实任务号，而不是因节点不符停在写前。
func TestRetrieveWriteBindsDoneRowByAuditLinkNotNode(t *testing.T) {
	targetFake := retrieveRewriteFacts()
	runCtx := newRunContext([]model.CompiledActionStep{{
		Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionRetrieve,
		Scope: model.ActionScopeCompletedTask, NodeKey: "node-audit",
	}})
	runCtx.Source = "done"
	runCtx.PathRun.MainInstanceRef = "instance-retrieve"
	runCtx.Nodes["node-audit"] = step.NodeInfo{Name: "考勤管理", Type: "common", TargetNodeID: "node-audit"}
	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	// 直接按节点过滤读取必然读空，这正是缺陷现场。
	empty, emptyErr := targetFake.FindTaskSnapshot(context.Background(), target.Session{}, "instance-retrieve", "node-audit", "done")
	if emptyErr != nil {
		t.Fatalf("按节点过滤读取不应报错：%v", emptyErr)
	}
	if empty.JobTaskID != "" {
		t.Fatalf("前置假设不成立：按节点过滤本应读空，实际读到 %q", empty.JobTaskID)
	}

	// 补救入口必须用审核记录的关联行 ID 找回同一行。
	recovered, ok := executor.RetrieveDoneSnapshotByAuditLinkForTest(context.Background(), runCtx, target.Session{}, runCtx.Steps[0], "node-audit")
	if !ok {
		t.Fatal("审核记录里有本节点通过记录时，写前补救应找回被改写节点的已办行")
	}
	if recovered.JobTaskID != "done-task" {
		t.Fatalf("补救应绑定该已办行的真实任务号 done-task，实际 %q", recovered.JobTaskID)
	}
}

// TestRetrieveWriteBindingStaysBlockedWithoutAuditPass 锁定保守边界：
// 审核记录里没有本节点通过记录时，写前补救必须失败，保持阻塞，绝不用其他节点的已办顶替。
func TestRetrieveWriteBindingStaysBlockedWithoutAuditPass(t *testing.T) {
	targetFake := retrieveRewriteFacts()
	targetFake.records = []target.AuditRecordSnapshot{{
		FlowJobTaskID: "other-link", FlowNodeProxyID: "node-elsewhere",
		ExecutorID: "someone", AuditStatus: "pass",
	}}
	runCtx := newRunContext([]model.CompiledActionStep{{
		Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionRetrieve,
		Scope: model.ActionScopeCompletedTask, NodeKey: "node-audit",
	}})
	runCtx.Source = "done"
	runCtx.PathRun.MainInstanceRef = "instance-retrieve"
	runCtx.Nodes["node-audit"] = step.NodeInfo{Name: "考勤管理", Type: "common", TargetNodeID: "node-audit"}
	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	if _, ok := executor.RetrieveDoneSnapshotByAuditLinkForTest(context.Background(), runCtx, target.Session{}, runCtx.Steps[0], "node-audit"); ok {
		t.Fatal("没有本节点通过记录时不得找回已办行，必须保持阻塞")
	}
}
