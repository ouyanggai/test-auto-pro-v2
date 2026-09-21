package executor_test

import (
	"testing"

	"test-auto-pro-v2/internal/engine/step"
)

// 2026-09-21 运行 76 路径 1 第 7 步缺陷回归：加签后的恢复链同意不得被判成路径不匹配。
//
// 现象：锚点节点加签后插入节点 F，随后回退把实例退到 F 上。恢复链里紧跟 F 的那一步同意，
// 其"下一业务节点"是 F 的前驱锚点 A——正向边永远走不到，门禁报：
//
//	构造写请求失败：当前路径没有找到从"分支"到下一节点的入口，请重新选择路径
//
// 修复口径：正向走不到但"目标可以正向到达当前节点"时，本次流转是恢复/回退语义，
// 正向分支入口为空即正确结果，不阻塞。真正的路径选择不一致仍然照旧阻塞。

// reverseTransitionRunCtx 构造「锚点 A → 插入节点 F → 后继 B」的真实图，回退后实例停在 F。
// 恢复链的下一节点是 A（F 的前驱），因此 A 能正向到达 F，而 F 走不到 A。
func reverseTransitionRunCtx() step.RunContext {
	const (
		anchor   = "anchor-node"
		inserted = "inserted-node"
		next     = "next-node"
	)
	return step.RunContext{
		Nodes: map[string]step.NodeInfo{
			"anchor-action":   {Name: "锚点", Type: "common", TargetNodeID: anchor},
			"inserted-action": {Name: "加签审批节点", Type: "common", TargetNodeID: inserted},
		},
		GraphEdges: map[string][]step.GraphEdgeInfo{
			anchor:   {{Target: inserted}},
			inserted: {{Target: next}},
		},
		GraphNodeTypes: map[string]string{anchor: "common", inserted: "common", next: "common"},
	}
}

// TestReverseRecoveryTransitionDoesNotBlock 锁定：从插入节点恢复到其前驱锚点时，
// 不得因为"正向走不到目标"而阻塞——这正是运行 76 卡住的那一步。
func TestReverseRecoveryTransitionDoesNotBlock(t *testing.T) {
	runCtx := reverseTransitionRunCtx()
	entries, blockReason := step.BranchEntriesForTransitionForTest("inserted-action", "anchor-action", runCtx)
	if blockReason != "" {
		t.Fatalf("插入节点恢复到前驱锚点属于反向流转，不得阻塞，实际原因：%s", blockReason)
	}
	if len(entries) != 0 {
		t.Fatalf("反向流转不应携带正向分支入口，实际：%+v", entries)
	}
}

// TestForwardBranchMismatchStillBlocks 反证：真正的路径选择不一致（正向该到的节点到不了，
// 且目标也不是当前节点的上游）必须继续阻塞，修复不能把安全门禁放过去。
func TestForwardBranchMismatchStillBlocks(t *testing.T) {
	runCtx := reverseTransitionRunCtx()
	// unrelated 与现有图完全不相连：既不是下游也不是上游。
	runCtx.Nodes["unrelated-action"] = step.NodeInfo{Name: "无关节点", Type: "common", TargetNodeID: "unrelated-node"}
	entries, blockReason := step.BranchEntriesForTransitionForTest("inserted-action", "unrelated-action", runCtx)
	if blockReason == "" {
		t.Fatal("与当前节点无上下游关系的目标必须继续阻塞，否则会把路径选择错误放过去")
	}
	if len(entries) != 0 {
		t.Fatalf("阻塞时不应返回分支入口，实际：%+v", entries)
	}
}

// TestManualBranchWithoutSelectionStillBlocks 反证：手动分支没有已保存选择时仍然阻塞，
// 反向流转豁免不得绕过 F-034 的手动分支选择纪律。
func TestManualBranchWithoutSelectionStillBlocks(t *testing.T) {
	runCtx := reverseTransitionRunCtx()
	runCtx.GraphNodeTypes["manual-route"] = "manual"
	runCtx.GraphEdges["manual-route"] = []step.GraphEdgeInfo{{Target: "branch-a"}, {Target: "branch-b"}}
	runCtx.Nodes["manual-action"] = step.NodeInfo{Name: "手动分支", Type: "manual", TargetNodeID: "manual-route"}
	runCtx.Nodes["branch-a-action"] = step.NodeInfo{Name: "分支甲", Type: "common", TargetNodeID: "branch-a"}

	_, blockReason := step.BranchEntriesForTransitionForTest("manual-action", "branch-a-action", runCtx)
	if blockReason == "" {
		t.Fatal("手动分支没有已保存选择时必须阻塞，不得用第一条出边兜底")
	}
}
