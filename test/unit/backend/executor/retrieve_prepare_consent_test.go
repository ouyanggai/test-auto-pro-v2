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
// 已办任务时，取回准备同意在预览阶段判为跳过，不发出任何写请求。
func TestRetrievePrepareConsentSkippedWhenDoneExists(t *testing.T) {
	targetFake := &taskActionFactsTarget{
		fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-done"}, DueNodes: []string{"node-done"}}},
		snapshot: target.TaskSnapshot{
			LinkID: "done-link", JobTaskID: "done-task", FlowNodeProxyID: "node-done",
			FlowProxyID: "proxy-current", TaskStatus: "done",
		},
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
// 任务、且读不到本节点处理人事实时，取回准备同意不会被跳过，而是落到普通同意门禁按实时待办判定
// （无可同意待办则阻塞）。事实不全时不提前阻塞——读不到不等于做不到。
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

// 2026-09-21 运行 90/92 路径 2 第 3-4 步缺陷回归：取回准备同意的前提判定。
//
// 现象：路径 2 第 3 步「取回准备同意」成功执行，第 4 步取回却停在写前
// 「当前用户没有可取回的已完成任务」，重跑同样结果。
//
// 根因：准备同意的唯一目的是让**取回人本人**（计划账号）在本节点获得一条已办任务——
// 取回门禁的 owned_completed_task 按当前会话账号核对已办（原生「取回」入口只出现在
// 已办列表里，作用对象就是当前用户自己的已办）。而普通同意门禁会把执行者切换成该节点
// 目标裁决的真实处理人：实测路径 2 该节点处理人是「集团考勤管理员」刘慧玲，
// 计划账号骆蒙恩不在候选之列。于是准备同意产生的已办落在刘慧玲名下，取回人依旧没有已办，
// 该步永远无法满足取回前提——与目标状态无关，属编排自身无法达成的组合。
//
// 修复口径：读不到本账号已办、且本节点当前处理人不含本账号时，在准备同意写前
// 按「取回前提无法满足」阻塞，并如实说明是哪条节点。
// 安全边界：读不到处理人事实或拿不到用户 ID 时不阻塞（读不到不等于做不到）。

// retrieveBlockerFacts 构造含本节点处理人用户 ID 的事实。
func retrieveBlockerFacts(handlerIDs ...string) step.InstanceFacts {
	return step.InstanceFacts{
		CurrentHandlers: []target.NodeCurrentHandler{
			{NodeID: "node-eeccefe5", BizIDs: handlerIDs},
		},
	}
}

// TestRetrievePrepareConsentBlocksWhenHandlerIsNotRetriever 锁定：本节点处理人不含取回人时，
// 准备同意无法给取回人留下已办，必须在写前阻塞并说明是哪条节点。
func TestRetrievePrepareConsentBlocksWhenHandlerIsNotRetriever(t *testing.T) {
	facts := retrieveBlockerFacts("handler-liuhuiling-id")
	reason := step.RetrievePrepareConsentBlockerForTest(facts, "plan-luo-id", "骆蒙恩", "node-eeccefe5")
	if strings.TrimSpace(reason) == "" {
		t.Fatal("本节点处理人不是取回人本人时，准备同意无法达成目的，必须阻塞")
	}
	if !strings.Contains(reason, "node-eeccefe5") {
		t.Fatalf("阻塞原因必须指明节点，实际：%s", reason)
	}
	if !strings.Contains(reason, "骆蒙恩") {
		t.Fatalf("阻塞原因必须指明取回人账号，实际：%s", reason)
	}
}

// TestRetrievePrepareConsentAllowedWhenRetrieverIsHandler 锁定：本节点处理人就是取回人本人时
// 不阻塞——准备同意会给他自己留下可核对的已办，这正是路径 1 能走通的形态。
func TestRetrievePrepareConsentAllowedWhenRetrieverIsHandler(t *testing.T) {
	facts := retrieveBlockerFacts("plan-luo-id")
	if reason := step.RetrievePrepareConsentBlockerForTest(facts, "plan-luo-id", "骆蒙恩", "node-eeccefe5"); reason != "" {
		t.Fatalf("处理人就是取回人本人时不应阻塞，实际原因：%s", reason)
	}
}

// TestRetrievePrepareConsentAllowedWhenRetrieverAmongCountersigners 锁定竞签边界：
// 本节点有多个候选处理人（竞签）且取回人位列其中时不阻塞——任一可用处理人推进即可。
func TestRetrievePrepareConsentAllowedWhenRetrieverAmongCountersigners(t *testing.T) {
	facts := retrieveBlockerFacts("handler-a-id", "plan-luo-id", "handler-b-id")
	if reason := step.RetrievePrepareConsentBlockerForTest(facts, "plan-luo-id", "骆蒙恩", "node-eeccefe5"); reason != "" {
		t.Fatalf("取回人在候选处理人之中时不应阻塞，实际原因：%s", reason)
	}
}

// TestRetrievePrepareConsentDoesNotBlockOnMissingFacts 锁定保守边界：
// 读不到本节点处理人事实，或拿不到处理人/取回人的用户 ID 时都不阻塞——
// 读不到不等于做不到，绝不把「没读到」当成「不可能」。
func TestRetrievePrepareConsentDoesNotBlockOnMissingFacts(t *testing.T) {
	cases := map[string]struct {
		facts      step.InstanceFacts
		planUserID string
		nodeID     string
	}{
		"没有处理人事实":     {step.InstanceFacts{}, "plan-luo-id", "node-eeccefe5"},
		"处理人缺少用户 ID":  {retrieveBlockerFacts(""), "plan-luo-id", "node-eeccefe5"},
		"拿不到取回人用户 ID": {retrieveBlockerFacts("handler-id"), "", "node-eeccefe5"},
		"节点标识不匹配":     {retrieveBlockerFacts("handler-id"), "plan-luo-id", "other-node"},
	}
	for name, item := range cases {
		if reason := step.RetrievePrepareConsentBlockerForTest(item.facts, item.planUserID, "骆蒙恩", item.nodeID); reason != "" {
			t.Fatalf("%s：事实不全时不得阻塞，实际原因：%s", name, reason)
		}
	}
}
