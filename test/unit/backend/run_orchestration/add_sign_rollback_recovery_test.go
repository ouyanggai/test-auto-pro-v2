package run_orchestration_test

import (
	"testing"

	"test-auto-pro-v2/internal/engine/control"
	"test-auto-pro-v2/internal/model"
)

// 2026-09-21 运行 75 路径 1 第 28 步缺陷回归：回退后的恢复链必须先经过加签插入节点。
//
// 现象：锚点节点「发起人综合部负责人」上加签后，插入节点 F 出现在锚点之后。
// 随后回退（rollBackThePreviousLevel）把实例退回到锚点之后的位置——实例实际停在 F 上。
// 恢复链里那条指向锚点的恢复同意先撞上"实例待办已越过锚点"，被判无可用处理人，
// 在写前阻塞并停止整条路径：
//
//	目标跳过判定：实例待办在「加签审批节点」，模板未声明 isSkip，必须阻塞
//
// 修复纪律：识别"回退引出的锚点同意"，在其之前先补插入节点同意，把实例沿真实路径推回锚点。
// 判定只看结构与动作语义，不匹配中文节点名、路径编号或固定步骤序号。

// rollbackRecoverySteps 构造「加签 → 锚点同意 → 导航 → 回退 → 锚点恢复同意」的步骤表。
func rollbackRecoverySteps() []model.CompiledActionStep {
	return []model.CompiledActionStep{
		{Source: model.ActionStepSourceUser, Action: model.ActionAddSign, NodeKey: "anchor"},
		{Source: model.ActionStepSourceSystemDefault, Action: model.ActionApprove, NodeKey: "anchor"},
		{Source: model.ActionStepSourceNavigation, Action: model.ActionSystemAutomatic, NodeKey: "cond"},
		{Source: model.ActionStepSourceUser, Action: model.ActionRollback, NodeKey: "next"},
		{Source: model.ActionStepSourceRecovery, Action: model.ActionApprove, NodeKey: "anchor"},
	}
}

// TestRollbackBeforeAnchorApproveIsRecognized 锁定：回退之后那条指向锚点的恢复同意必须被识别，
// 从而用插入节点同意替换它；这正是运行 75/76/77 卡住的那一步。
//
// 为什么是"替换"而不是"在它之前追加"：回退把实例退到锚点之后的插入节点上，实例已经越过锚点。
// 那条锚点恢复同意是多余的——运行 77 第 29 步实测它撞上"待办已越过锚点"而在写前阻塞。
// 真正需要的是把实例从插入节点推回，补一次插入节点同意即等价于完成这次恢复。
func TestRollbackBeforeAnchorApproveIsRecognized(t *testing.T) {
	steps := rollbackRecoverySteps()
	// 下标 4 是回退引出的锚点恢复同意。
	if !control.RollbackPrecedesForTest(steps, 4, "anchor") {
		t.Fatal("回退之后的锚点恢复同意必须被识别，否则实例会停在加签节点上被误判为无处理人")
	}
	// 下标 1 是正向流转的锚点同意，前面没有回退，不能被误判。
	if control.RollbackPrecedesForTest(steps, 1, "anchor") {
		t.Fatal("正向流转的锚点同意不得被误判为回退引出，否则会打乱正常加签注入顺序")
	}
}

// TestRollbackPrecedesIgnoresNavigationAndOtherApproves 锁定判定的边界：
// 只读导航不改变"谁把实例退回来"的事实；遇到更近的一次锚点同意则不再回溯判定。
func TestRollbackPrecedesIgnoresNavigationAndOtherApproves(t *testing.T) {
	// 回退与锚点同意之间隔着只读导航，仍应识别出来。
	withNavigation := []model.CompiledActionStep{
		{Source: model.ActionStepSourceUser, Action: model.ActionRollback, NodeKey: "next"},
		{Source: model.ActionStepSourceNavigation, Action: model.ActionSystemAutomatic, NodeKey: "cond"},
		{Source: model.ActionStepSourceNavigation, Action: model.ActionSystemAutomatic, NodeKey: "cond"},
		{Source: model.ActionStepSourceRecovery, Action: model.ActionApprove, NodeKey: "anchor"},
	}
	if !control.RollbackPrecedesForTest(withNavigation, 3, "anchor") {
		t.Fatal("只读导航不应阻断回退判定")
	}
	// 回退之后已经先出现过一次锚点同意，说明本次同意属正向流转，不再按回退处理。
	afterAnchorApprove := []model.CompiledActionStep{
		{Source: model.ActionStepSourceUser, Action: model.ActionRollback, NodeKey: "next"},
		{Source: model.ActionStepSourceRecovery, Action: model.ActionApprove, NodeKey: "anchor"},
		{Source: model.ActionStepSourceRecovery, Action: model.ActionApprove, NodeKey: "anchor"},
	}
	if control.RollbackPrecedesForTest(afterAnchorApprove, 2, "anchor") {
		t.Fatal("更近的一次锚点同意之后不得再按回退判定")
	}
	// 前面是别的节点动作（不是回退）时不得判定。
	plain := []model.CompiledActionStep{
		{Source: model.ActionStepSourceSystemDefault, Action: model.ActionApprove, NodeKey: "other"},
		{Source: model.ActionStepSourceRecovery, Action: model.ActionApprove, NodeKey: "anchor"},
	}
	if control.RollbackPrecedesForTest(plain, 1, "anchor") {
		t.Fatal("前一人工步骤不是回退时不得判定为回退引出")
	}
}
