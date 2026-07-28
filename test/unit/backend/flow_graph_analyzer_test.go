package backend_test

import (
	"errors"
	"fmt"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
)

func TestFlowGraphAnalyzerStraightUnknownAndDeduplication(t *testing.T) {
	end := &target.FlowNodeTemplate{ID: "end", Name: "结束", Type: "end"}
	unknown := &target.FlowNodeTemplate{ID: "unknown", Name: "扩展节点", Type: "future", Child: end}
	root := &target.FlowNodeTemplate{ID: "start", Name: "发起人", Type: "start", Child: unknown}
	nodes, edges, warnings, err := analyzer.NewFlowGraphAnalyzer().Analyze(root)
	if err != nil || len(nodes) != 3 || len(edges) != 2 || len(warnings) != 1 {
		t.Fatalf("直线或未知普通节点解析不正确：nodes=%d edges=%d warnings=%d err=%v", len(nodes), len(edges), len(warnings), err)
	}
	if nodes[1].Type != "unknown" || nodes[1].TypeName != "未知类型" {
		t.Fatal("未知普通节点没有保留并标注")
	}
}

func TestFlowGraphAnalyzerDeduplicatesSharedRealNode(t *testing.T) {
	shared := &target.FlowNodeTemplate{ID: "shared", Name: "共享审批", Type: "common"}
	root := &target.FlowNodeTemplate{
		ID: "parallel", Name: "并行", Type: "parallel",
		ParallelNodes: []target.FlowBranchTemplate{
			{ID: "branch-a", Name: "甲", Child: shared},
			{ID: "branch-b", Name: "乙", Child: &target.FlowNodeTemplate{ID: "shared", Name: "共享审批", Type: "common"}},
		},
	}
	nodes, edges, _, err := analyzer.NewFlowGraphAnalyzer().Analyze(root)
	if err != nil || len(nodes) != 2 || len(edges) != 2 {
		t.Fatalf("共享真实节点未正确去重：nodes=%d edges=%d err=%v", len(nodes), len(edges), err)
	}
	if nodes[0].MergeTargetID != "" {
		t.Fatalf("无主线后继的路由不应设置汇合提示：%q", nodes[0].MergeTargetID)
	}
}

func TestFlowGraphAnalyzerConditionManualParallelNestedAndMerge(t *testing.T) {
	merge := &target.FlowNodeTemplate{ID: "merge", Name: "汇合审批", Type: "common", Child: &target.FlowNodeTemplate{ID: "end", Name: "结束", Type: "end"}}
	nested := &target.FlowNodeTemplate{
		ID: "nested", Name: "并行处理", Type: "parallel", Child: merge,
		ParallelNodes: []target.FlowBranchTemplate{
			{ID: "parallel-b", Name: "并行乙", Sort: 2, Child: &target.FlowNodeTemplate{ID: "parallel-node-b", Name: "协同乙", Type: "synergy"}},
			{ID: "parallel-a", Name: "并行甲", Sort: 1, Child: &target.FlowNodeTemplate{ID: "parallel-node-a", Name: "审批甲", Type: "common"}},
		},
	}
	manual := &target.FlowNodeTemplate{
		ID: "manual", Name: "人工选择", Type: "condition", BranchExecuteType: "custom_choose", Child: nested,
		ConditionNodes: []target.FlowBranchTemplate{
			{ID: "manual-a", Name: "人工分支", Child: &target.FlowNodeTemplate{ID: "manual-node", Name: "人工审批", Type: "common"}},
		},
	}
	condition := &target.FlowNodeTemplate{
		ID: "condition", Name: "金额判断", Type: "condition", Child: manual,
		ConditionNodes: []target.FlowBranchTemplate{
			{ID: "condition-a", Name: "小额", Child: &target.FlowNodeTemplate{ID: "small", Name: "小额审批", Type: "common"}},
			{ID: "condition-b", Name: "大额", Child: &target.FlowNodeTemplate{ID: "large", Name: "大额审批", Type: "common"}},
		},
	}
	root := &target.FlowNodeTemplate{ID: "start", Name: "发起", Type: "start", Child: condition}
	nodes, edges, warnings, err := analyzer.NewFlowGraphAnalyzer().Analyze(root)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("嵌套分支解析失败：%v", err)
	}
	wantKinds := map[string]bool{"condition": false, "manual": false, "parallel": false, "sequence": false}
	mergeIncoming := 0
	seenNode := map[string]bool{}
	mergeTargets := map[string]string{}
	for _, node := range nodes {
		if seenNode[node.ID] {
			t.Fatal("节点未按真实 ID 去重")
		}
		seenNode[node.ID] = true
		mergeTargets[node.ID] = node.MergeTargetID
	}
	for _, edge := range edges {
		wantKinds[edge.Kind] = true
		if edge.Target == "merge" {
			mergeIncoming++
		}
		if edge.Kind != "sequence" && edge.BranchID == "" {
			t.Fatal("分支边缺少真实策略 ID")
		}
	}
	for kind, found := range wantKinds {
		if !found {
			t.Fatalf("缺少 %s 边", kind)
		}
	}
	if mergeIncoming != 2 {
		t.Fatalf("并行分支没有汇合到同一真实后继：%d", mergeIncoming)
	}
	if len(nodes) != 11 {
		t.Fatalf("嵌套图节点数量异常：%d", len(nodes))
	}
	if mergeTargets["condition"] != "manual" || mergeTargets["manual"] != "nested" || mergeTargets["nested"] != "merge" {
		t.Fatalf("嵌套路由汇合提示不正确：%v", mergeTargets)
	}
}

func TestFlowGraphAnalyzerRejectsInvalidStructuresAndLimits(t *testing.T) {
	cycle := &target.FlowNodeTemplate{ID: "cycle", Name: "循环", Type: "common"}
	cycle.Child = cycle
	missingID := &target.FlowNodeTemplate{Name: "缺少 ID", Type: "common"}
	emptyBranch := &target.FlowNodeTemplate{ID: "condition", Name: "条件", Type: "condition"}
	missingBranchID := &target.FlowNodeTemplate{ID: "condition", Name: "条件", Type: "condition", ConditionNodes: []target.FlowBranchTemplate{{Child: &target.FlowNodeTemplate{ID: "child", Type: "common"}}}}
	unknownBranch := &target.FlowNodeTemplate{ID: "unknown", Name: "未知", Type: "future", ConditionNodes: []target.FlowBranchTemplate{{ID: "branch", Child: &target.FlowNodeTemplate{ID: "child", Type: "common"}}}}

	tooDeep := &target.FlowNodeTemplate{ID: "node-0", Type: "start"}
	cursor := tooDeep
	for index := 1; index <= 200; index++ {
		cursor.Child = &target.FlowNodeTemplate{ID: fmt.Sprintf("node-%d", index), Type: "common"}
		cursor = cursor.Child
	}
	tooMany := &target.FlowNodeTemplate{ID: "parallel-root", Name: "并行", Type: "parallel"}
	for index := 0; index < 500; index++ {
		tooMany.ParallelNodes = append(tooMany.ParallelNodes, target.FlowBranchTemplate{
			ID: fmt.Sprintf("branch-%d", index), Child: &target.FlowNodeTemplate{ID: fmt.Sprintf("wide-%d", index), Type: "common"},
		})
	}
	for name, root := range map[string]*target.FlowNodeTemplate{
		"循环": cycle, "关键 ID 缺失": missingID, "空分支": emptyBranch,
		"策略 ID 缺失": missingBranchID, "未知分支节点": unknownBranch,
		"超过节点上限": tooMany, "超过深度上限": tooDeep,
	} {
		t.Run(name, func(t *testing.T) {
			_, _, _, err := analyzer.NewFlowGraphAnalyzer().Analyze(root)
			if !errors.Is(err, analyzer.ErrFlowStructureInvalid) {
				t.Fatalf("异常结构未被拒绝：%v", err)
			}
		})
	}
}
