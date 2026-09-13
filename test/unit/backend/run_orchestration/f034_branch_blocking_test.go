package run_orchestration

import (
	"strings"
	"testing"

	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/engine/verdict"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// F-034 T01/T04：复现路径 4217（路径 8）的手动分支缺陷——分支已保存但入口是空节点，
// 下一业务动作不是分支入口。旧代码 chosenBranchEntryForNode 只在“下一业务节点 == 分支目标”时命中，
// 导致 /flowInstanceApi/audit 缺少入口被目标「手动条件分支,请选择」拒绝。

// f034GraphEdges 构造与真实路径 8 相同的图形状（目标节点 ID 空间）：
// 发起(startX) → 审批(b037 分支路由) → 分支1入口 b9fc（省区经理）/ 分支2入口 8b676（空节点）
// → 8b676 空节点 → c359 条件审批 → 下一个业务动作 approverX。
const (
	branchRouteNode   = "b037ae8937574810a458c44d06eb9237"
	branchEntryB9FC   = "b9fcaaaaaaaagaaaa4fd2b7e54f94a59"
	branchEntryEmpty  = "8b676f8c25b34fd2b7e54f94a59f2e3c"
	emptyNodeID       = "8b676f8c25b34fd2b7e54f94a59f2e3c"
	approverNode      = "c359ccccccccccccccccc44d06eb9237"
	businessApprover  = "d001aaaaaaaabbbbbbbbbbbbbbbbbb"
	startNode         = "start-0000-0000-0000-000000000000"
	selectedBranchTwo = "2c26ec387e2f46fca04886d22fc1271e"
)

// f034RunContext 构造路径 8 的运行上下文：分支 2 已保存，入口是空节点。
func f034RunContext(currentNodeKey, nextNodeKey string) step.RunContext {
	return step.RunContext{
		Nodes: map[string]step.NodeInfo{
			currentNodeKey: {Name: "分支节点", Type: "manual", TargetNodeID: branchRouteNode},
			nextNodeKey:    {Name: "审核人", Type: "common", TargetNodeID: businessApprover},
		},
		BranchSelections: map[string]string{branchRouteNode: branchEntryEmpty},
		GraphEdges: map[string][]step.GraphEdgeInfo{
			startNode:       {{Target: branchRouteNode}},
			branchRouteNode: {{Target: branchEntryB9FC, BranchID: "1"}, {Target: branchEntryEmpty, BranchID: selectedBranchTwo}},
			emptyNodeID:     {{Target: approverNode}},
			approverNode:    {{Target: businessApprover}},
		},
		GraphNodeTypes: map[string]string{
			startNode: "start", branchRouteNode: "manual", branchEntryB9FC: "common",
			emptyNodeID: "empty", approverNode: "common", businessApprover: "common",
		},
	}
}

// TestF034BranchEntryResolvedAcrossEmptyNode 验证跨空节点的手动分支入口解析：
// 当前步在分支路由节点、下一业务动作在空节点后方时，解析结果必须携带已保存的空入口 8b676。
func TestF034BranchEntryResolvedAcrossEmptyNode(t *testing.T) {
	runCtx := f034RunContext("route", "approver")
	runCtx.Nodes["approver"] = step.NodeInfo{Name: "审核人", Type: "common", TargetNodeID: businessApprover}
	entries, blockReason := step.BranchEntriesForTransitionForTest("route", "approver", runCtx)
	if blockReason != "" {
		t.Fatalf("分支已保存时不应阻塞：%s", blockReason)
	}
	if len(entries) != 1 || entries[0] != branchEntryEmpty {
		t.Fatalf("应携带空节点分支入口 8b676：%v", entries)
	}
}

// TestF034BranchMissingSelectionBlocksBeforeSend 验证手动分支没有已保存选择时写前阻塞，
// 不再发送缺少手动分支入口的 /flowInstanceApi/audit 请求。
func TestF034BranchMissingSelectionBlocksBeforeSend(t *testing.T) {
	runCtx := f034RunContext("route", "approver")
	delete(runCtx.BranchSelections, branchRouteNode)
	_, blockReason := step.BranchEntriesForTransitionForTest("route", "approver", runCtx)
	if blockReason == "" {
		t.Fatal("手动分支无选择时应在发送前阻塞")
	}
	if !strings.Contains(blockReason, "入口") {
		t.Fatalf("阻塞原因应说明找不到入口：%s", blockReason)
	}
}

// TestF034BranchNoCrossingReturnsEmpty 验证不跨分支的普通流转返回空入口、不阻塞、不带多余条目。
func TestF034BranchNoCrossingReturnsEmpty(t *testing.T) {
	runCtx := f034RunContext("approver", "next")
	runCtx.Nodes["approver"] = step.NodeInfo{Name: "审核人", Type: "common", TargetNodeID: approverNode}
	runCtx.Nodes["next"] = step.NodeInfo{Name: "下一审核", Type: "common", TargetNodeID: businessApprover}
	entries, blockReason := step.BranchEntriesForTransitionForTest("approver", "next", runCtx)
	if blockReason != "" || len(entries) != 0 {
		t.Fatalf("不跨分支的流转不应携带入口或阻塞：entries=%v reason=%s", entries, blockReason)
	}
}

// TestF034CustomChooseRejectionIsBlocked 验证「手动条件分支,请选择」与「未设置审批人」
// 命中前置拒绝清单：HTTP 200 + isSuccess=false + 重读未变 = 确定失败（阻塞语义），不再落结果待确认。
func TestF034CustomChooseRejectionIsBlocked(t *testing.T) {
	observation := verdict.Observation{
		Endpoint: "/flowInstanceApi/audit", StatusCode: 200, Transport: verdict.TransportResponded,
		Response: &verdict.Response{IsSuccessPresent: true, IsSuccess: false, Message: "手动条件分支,请选择"},
		Reread:   verdict.RereadUnchanged,
	}
	result := verdict.Evaluate(observation)
	if result.Outcome != verdict.OutcomeFailed || result.Initial != verdict.InitialPreRejected {
		t.Fatalf("手动分支拒绝应判为前置阻塞失败：%+v", result)
	}
	if result.SideEffect != verdict.SideEffectNone {
		t.Fatalf("前置拒绝必须无副作用：%+v", result)
	}

	unauthorized := verdict.Observation{
		Endpoint: "/flowInstanceApi/audit", StatusCode: 200, Transport: verdict.TransportResponded,
		Response: &verdict.Response{IsSuccessPresent: true, IsSuccess: false, Message: "未设置审批人"},
		Reread:   verdict.RereadUnchanged,
	}
	result = verdict.Evaluate(unauthorized)
	if result.Outcome != verdict.OutcomeFailed || result.Initial != verdict.InitialPreRejected {
		t.Fatalf("未设置审批人应判为前置阻塞失败：%+v", result)
	}
}

// TestF034RereadAdvancedKeepsUncertain 验证前置拒绝与重读推进冲突时仍判不确定，
// 不因新增清单条目放宽矩阵兜底。
func TestF034RereadAdvancedKeepsUncertain(t *testing.T) {
	observation := verdict.Observation{
		Endpoint: "/flowInstanceApi/audit", StatusCode: 200, Transport: verdict.TransportResponded,
		Response: &verdict.Response{IsSuccessPresent: true, IsSuccess: false, Message: "手动条件分支,请选择"},
		Reread:   verdict.RereadAdvanced,
	}
	result := verdict.Evaluate(observation)
	if result.Outcome != verdict.OutcomeUncertain {
		t.Fatalf("拒绝与推进冲突时必须保持不确定：%+v", result)
	}
}

// TestF034LastAttemptPreRejected 验证阻塞投影判据：只有最后一次尝试初判为 pre_rejected 才算阻塞。
func TestF034LastAttemptPreRejected(t *testing.T) {
	pre := model.RunStepAttempt{AttemptNo: 1, Initial: "pre_rejected"}
	unexplained := model.RunStepAttempt{AttemptNo: 1, Initial: "unexplained"}
	if !service.LastAttemptWasPreRejectedForTest([]model.RunStepAttempt{pre}) {
		t.Fatal("pre_rejected 应判为阻塞")
	}
	if service.LastAttemptWasPreRejectedForTest([]model.RunStepAttempt{unexplained}) {
		t.Fatal("不可解释失败不应判为阻塞")
	}
	if service.LastAttemptWasPreRejectedForTest(nil) {
		t.Fatal("没有尝试记录时不应判为阻塞")
	}
}

// TestF034PreRejectedStepUsesPersistedOrder 锁定跨步骤尝试的最后记录判定：
// AttemptNo 是单步编号，不能拿第 1 步的第 3 次尝试覆盖第 3 步的第 1 次阻塞。
func TestF034PreRejectedStepUsesPersistedOrder(t *testing.T) {
	attempts := []model.RunStepAttempt{
		{ID: 11, StepID: 1, AttemptNo: 1, Initial: "unexplained"},
		{ID: 12, StepID: 1, AttemptNo: 3, Initial: "unexplained"},
		{ID: 13, StepID: 3, AttemptNo: 1, Initial: "pre_rejected"},
	}
	if !service.LastAttemptWasPreRejectedForTest(attempts) {
		t.Fatal("落账最后一条为 pre_rejected 时应判为阻塞")
	}
	if got := service.LastPreRejectedStepNoForTest(attempts); got != 3 {
		t.Fatalf("应定位到第 3 步，实际是第 %d 步", got)
	}

	attempts = append(attempts, model.RunStepAttempt{ID: 14, StepID: 4, AttemptNo: 1, Initial: "unexplained"})
	if service.LastAttemptWasPreRejectedForTest(attempts) {
		t.Fatal("最后一条不是 pre_rejected 时不应判为阻塞")
	}
	if got := service.LastPreRejectedStepNoForTest(attempts); got != 3 {
		t.Fatalf("最后一次前置拒绝仍应定位到第 3 步，实际是第 %d 步", got)
	}
}

// TestF034StepDTOCarriesStableActionKey 锁定已落账步骤同时返回稳定动作键与中文名称，
// 使历史动作名称缺失时前端仍可通过 action 键显示中文。
func TestF034StepDTOCarriesStableActionKey(t *testing.T) {
	dtos := service.BuildStepDTOsForTest([]model.RunStep{{
		ID: 1, StepNo: 1, Action: "approve", NodeKey: "node-1", Status: model.RunStepSucceeded,
	}}, nil)
	if len(dtos) != 1 {
		t.Fatalf("应返回 1 条步骤事实，实际 %d 条", len(dtos))
	}
	if dtos[0].Action != "approve" || dtos[0].ActionName != "同意" {
		t.Fatalf("步骤 DTO 应同时返回 action=approve 与中文名称同意：%+v", dtos[0])
	}
}

// TestF034SelectedManualBranchNeverSwitchesToOtherBranches 锁定 F-034 评审 #1：
// 手动分支已有保存选择时只允许沿该选择继续；选中路径无法到达下一业务节点时直接阻塞，
// 绝不遍历其他出边改走另一条分支——否则会把请求发到与路径选择不一致的路径。
func TestF034SelectedManualBranchNeverSwitchesToOtherBranches(t *testing.T) {
	// 图形：分支路由 route 有两个出边：selectedA（已选）与 otherB（未选）。
	// selectedA 走到死胡同，otherB 才能到达目标节点。
	runCtx := step.RunContext{
		Nodes: map[string]step.NodeInfo{
			"current": {Name: "分支节点", Type: "manual", TargetNodeID: "route-target"},
			"next":    {Name: "审核人", Type: "common", TargetNodeID: "next-target"},
		},
		BranchSelections: map[string]string{"route-target": "selected-a"},
		GraphEdges: map[string][]step.GraphEdgeInfo{
			"route-target": {{Target: "selected-a"}, {Target: "other-b"}},
			"selected-a":   {},
			"other-b":      {{Target: "next-target"}},
		},
		GraphNodeTypes: map[string]string{"route-target": "manual", "selected-a": "common", "other-b": "common"},
	}
	_, blockReason := step.BranchEntriesForTransitionForTest("current", "next", runCtx)
	if blockReason == "" {
		t.Fatal("已选分支走不通时必须阻塞，不得改走未选分支")
	}
	if !strings.Contains(blockReason, "入口") {
		t.Fatalf("阻塞原因应说明找不到入口：%s", blockReason)
	}
}

// TestF034ConditionBranchMayTraverseAllEdgesWhenUnresolved 锁定评审 #1 的另一面：
// 条件分支没有保存选择时（目标运行时自动求值），允许沿任意出边尝试到达目标，不阻塞。
func TestF034ConditionBranchMayTraverseAllEdgesWhenUnresolved(t *testing.T) {
	runCtx := step.RunContext{
		Nodes: map[string]step.NodeInfo{
			"current": {Name: "条件节点", Type: "condition", TargetNodeID: "cond-target"},
			"next":    {Name: "审核人", Type: "common", TargetNodeID: "next-target"},
		},
		BranchSelections: map[string]string{},
		GraphEdges: map[string][]step.GraphEdgeInfo{
			"cond-target": {{Target: "dead-end"}, {Target: "next-target"}},
			"dead-end":    {},
		},
		GraphNodeTypes: map[string]string{"cond-target": "condition", "dead-end": "end"},
	}
	entries, blockReason := step.BranchEntriesForTransitionForTest("current", "next", runCtx)
	if blockReason != "" {
		t.Fatalf("条件分支无选择时不应阻塞：%s", blockReason)
	}
	if len(entries) != 0 {
		t.Fatalf("条件分支入口不进手动分支条目：%v", entries)
	}
}
