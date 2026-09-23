package target_test

import (
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
)

// TestAlignProxyTreeBranchesParallelReorder 验证加签导致并行分支数组顺序颠倒时，能够按语义特征精确恢复原始顺序。
func TestAlignProxyTreeBranchesParallelReorder(t *testing.T) {
	// 静态旧树：[0] 资金副主任, [1] 张政
	refTree := &target.FlowNodeTemplate{
		ID: "root", Type: "start",
		Child: &target.FlowNodeTemplate{
			ID: "p-gate", Type: "parallel",
			ParallelNodes: []target.FlowBranchTemplate{
				{
					ID:   "branch-2",
					Name: "节点2",
					Sort: 1,
					Child: &target.FlowNodeTemplate{
						ID:   "node-zj",
						Name: "财资部-资金副主任",
						Type: "common",
					},
				},
				{
					ID:   "branch-1",
					Name: "节点1",
					Sort: 1,
					Child: &target.FlowNodeTemplate{
						ID:   "node-zz",
						Name: "张政",
						Type: "common",
					},
				},
			},
		},
	}

	// 动态新树（张政加签后排到了首位，且张政后挂了新插入的加签审批人）
	targetTree := &target.FlowNodeTemplate{
		ID: "root-new", Type: "start",
		Child: &target.FlowNodeTemplate{
			ID: "p-gate-new", Type: "parallel",
			ParallelNodes: []target.FlowBranchTemplate{
				{
					ID:   "branch-1-new",
					Name: "节点1",
					Sort: 1,
					Child: &target.FlowNodeTemplate{
						ID:   "node-zz-new",
						Name: "张政",
						Type: "common",
						Child: &target.FlowNodeTemplate{
							ID:   "node-sign",
							Name: "宋恺",
							Type: "common",
						},
					},
				},
				{
					ID:   "branch-2-new",
					Name: "节点2",
					Sort: 1,
					Child: &target.FlowNodeTemplate{
						ID:   "node-zj-new",
						Name: "财资部-资金副主任",
						Type: "common",
					},
				},
			},
		},
	}

	// 执行对齐
	target.AlignProxyTreeBranches(refTree, targetTree)

	// 断言：对齐后 targetTree 的第 0 个分支必须是资金副主任，第 1 个必须是张政
	if len(targetTree.Child.ParallelNodes) != 2 {
		t.Fatalf("分支数量应为 2，实际为 %d", len(targetTree.Child.ParallelNodes))
	}
	if got := targetTree.Child.ParallelNodes[0].Child.Name; got != "财资部-资金副主任" {
		t.Errorf("第 0 分支首节点应为资金副主任，实际为 %q", got)
	}
	if got := targetTree.Child.ParallelNodes[1].Child.Name; got != "张政" {
		t.Errorf("第 1 分支首节点应为张政，实际为 %q", got)
	}
	if got := targetTree.Child.ParallelNodes[1].Child.Child.Name; got != "宋恺" {
		t.Errorf("第 1 分支张政之后应保留加签节点宋恺，实际为 %q", got)
	}
}
