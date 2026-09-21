package executor_test

import (
	"testing"

	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// 2026-09-21 夜间全矩阵真实运行缺陷回归：路径 3/7 等在「发起人」节点执行取回时，
// 实例待办已经流转到后继节点（省区经理 / 发起人部门领导），执行器把这一正常前提
// 误判成"目标自动跳过"，进而在写前阻塞，整条路径停在取回步骤。
//
// 根因：目标跳过判定（targetSkippedStepReason）描述的是"推进型动作要办的节点已被目标越过"，
// 而取回/回退/撤回这类恢复型动作的成立前提恰恰就是实例已越过目标节点。
// 修复后恢复型动作不参与跳过判定，改由动作自身的已办/前驱门禁裁决。

// recoverySkipFacts 构造"实例待办已越过本步节点"的事实：待办落在路径后方节点。
func recoverySkipFacts() step.InstanceFacts {
	return step.InstanceFacts{
		Found:        true,
		Status:       "run",
		CurrentNodes: []string{"late-target"},
		DueNodes:     []string{"late-target"},
	}
}

// recoverySkipContext 构造两步场景：本步在 early 节点（位置 0），路径后方 late 节点（位置 1）。
func recoverySkipContext() step.RunContext {
	return step.RunContext{
		Nodes: map[string]step.NodeInfo{
			"early": {Name: "发起人", Type: "start", TargetNodeID: "early-target"},
			"late":  {Name: "省区经理", Type: "common", TargetNodeID: "late-target"},
		},
		Steps: []model.CompiledActionStep{
			{Source: model.ActionStepSourceUser, Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: "early"},
			{Source: model.ActionStepSourceUser, Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: "late"},
		},
	}
}

// TestRecoveryEffectStepsExcludedFromSkipGuard 锁定：取回/回退/撤回归入恢复型动作，
// 推进型的同意/不同意/加签等不得被误纳入，否则会拆掉真实的写前阻塞门禁。
func TestRecoveryEffectStepsExcludedFromSkipGuard(t *testing.T) {
	for _, action := range []model.ActionKey{model.ActionRetrieve, model.ActionRollback, model.ActionWithdraw} {
		if !step.IsRecoveryEffectStepForTest(model.CompiledActionStep{Action: action}) {
			t.Fatalf("%s 必须归入恢复型动作", action)
		}
	}
	for _, action := range []model.ActionKey{model.ActionApprove, model.ActionReject, model.ActionAddSign, model.ActionTransfer, model.ActionUrge} {
		if step.IsRecoveryEffectStepForTest(model.CompiledActionStep{Action: action}) {
			t.Fatalf("%s 不是恢复型动作，必须继续受目标跳过判定约束", action)
		}
	}
}

// TestRetrievePastNodeIsNotSkipped 锁定：取回步骤在"待办已越过本节点"时必须能继续，
// 不再被解析成目标跳过（skip）或写前阻塞（block）。这正是夜间路径 3/7 的卡点。
func TestRetrievePastNodeIsNotSkipped(t *testing.T) {
	runCtx := recoverySkipContext()
	retrieve := model.CompiledActionStep{
		Source: model.ActionStepSourceUser, Action: model.ActionRetrieve,
		Scope: model.ActionScopeCompletedTask, NodeKey: "early",
	}
	if step.IsRecoveryEffectStepForTest(retrieve) != true {
		t.Fatal("取回步骤必须被识别为恢复型动作，从而跳过目标跳过判定")
	}
	// 反证：同样的事实下，推进型的同意动作仍然必须被判定为阻塞，门禁没有被整体削弱。
	approve := model.CompiledActionStep{
		Source: model.ActionStepSourceUser, Action: model.ActionApprove,
		Scope: model.ActionScopeTask, NodeKey: "early",
	}
	action, _ := step.TargetSkippedClassificationForTest(runCtx, approve, recoverySkipFacts())
	if action != "block" {
		t.Fatalf("审批动作在待办越过后仍必须阻塞，实际 %s", action)
	}
}
