package backend_test

import (
	"errors"
	"fmt"
	"testing"

	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
)

// pathNode 创建路径分析测试使用的最小节点。
func pathNode(id, nodeType string) model.FlowGraphNode {
	return model.FlowGraphNode{ID: id, Name: id, Type: nodeType, TypeName: nodeType}
}

// pathEdge 创建路径分析测试使用的最小真实边。
func pathEdge(id, source, target, kind, branchID string) model.FlowGraphEdge {
	return model.FlowGraphEdge{ID: id, Source: source, Target: target, Kind: kind, BranchID: branchID}
}

// TestExecutionPathAnalyzerStraightFlowNeedsNoChoice 验证直线图零选择即可完整。
func TestExecutionPathAnalyzerStraightFlowNeedsNoChoice(t *testing.T) {
	graph := model.FlowGraph{
		EntryNodeIDs: []string{"start"},
		Nodes:        []model.FlowGraphNode{pathNode("start", "start"), pathNode("end", "end")},
		Edges:        []model.FlowGraphEdge{pathEdge("start-end", "start", "end", "sequence", "")},
	}
	result, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, nil)
	if err != nil || !result.Complete || len(result.ReachableNodeIDs) != 2 {
		t.Fatalf("直线路径分析失败：result=%+v err=%v", result, err)
	}
}

// TestExecutionPathAnalyzerConditionAndManualRequireOneReachableChoice 验证单选路由只要求可达分支。
func TestExecutionPathAnalyzerConditionAndManualRequireOneReachableChoice(t *testing.T) {
	graph := model.FlowGraph{
		EntryNodeIDs: []string{"condition"},
		Nodes: []model.FlowGraphNode{
			pathNode("condition", "condition"), pathNode("left", "common"), pathNode("right", "common"),
			pathNode("manual", "manual"), pathNode("manual-a", "common"), pathNode("manual-b", "common"), pathNode("end", "end"),
		},
		Edges: []model.FlowGraphEdge{
			pathEdge("condition-left", "condition", "left", "condition", "left-branch"),
			pathEdge("condition-right", "condition", "right", "condition", "right-branch"),
			pathEdge("left-manual", "left", "manual", "sequence", ""),
			pathEdge("right-end", "right", "end", "sequence", ""),
			pathEdge("manual-a", "manual", "manual-a", "manual", "manual-a-branch"),
			pathEdge("manual-b", "manual", "manual-b", "manual", "manual-b-branch"),
			pathEdge("manual-a-end", "manual-a", "end", "sequence", ""),
			pathEdge("manual-b-end", "manual-b", "end", "sequence", ""),
		},
	}
	pathAnalyzer := analyzer.NewExecutionPathAnalyzer()
	incomplete, err := pathAnalyzer.Analyze(graph, []model.ExecutionPathChoice{{RouteNodeID: "condition", BranchID: "left-branch"}})
	if err != nil || incomplete.Complete || len(incomplete.MissingRouteNodeIDs) != 1 || incomplete.MissingRouteNodeIDs[0] != "manual" {
		t.Fatalf("可达手动分支没有成为唯一待选项：result=%+v err=%v", incomplete, err)
	}
	complete, err := pathAnalyzer.Analyze(graph, []model.ExecutionPathChoice{
		{RouteNodeID: "condition", BranchID: "left-branch"},
		{RouteNodeID: "manual", BranchID: "manual-b-branch"},
	})
	if err != nil || !complete.Complete {
		t.Fatalf("条件与手动分支完整选择失败：result=%+v err=%v", complete, err)
	}
	_, err = pathAnalyzer.Analyze(graph, []model.ExecutionPathChoice{
		{RouteNodeID: "condition", BranchID: "right-branch"},
		{RouteNodeID: "manual", BranchID: "manual-a-branch"},
	})
	if !errors.Is(err, analyzer.ErrExecutionPathInvalid) {
		t.Fatalf("不可达手动选择未被拒绝：%v", err)
	}
}

// TestExecutionPathAnalyzerParallelIncludesAllBranchesAndNestedChoices 验证并行全包含及嵌套待选点。
func TestExecutionPathAnalyzerParallelIncludesAllBranchesAndNestedChoices(t *testing.T) {
	graph := model.FlowGraph{
		EntryNodeIDs: []string{"parallel"},
		Nodes: []model.FlowGraphNode{
			pathNode("parallel", "parallel"), pathNode("left-route", "condition"), pathNode("right-route", "manual"),
			pathNode("left-a", "common"), pathNode("left-b", "common"), pathNode("right-a", "common"), pathNode("right-b", "common"),
			pathNode("merge", "common"), pathNode("end", "end"),
		},
		Edges: []model.FlowGraphEdge{
			pathEdge("parallel-left", "parallel", "left-route", "parallel", "parallel-left"),
			pathEdge("parallel-right", "parallel", "right-route", "parallel", "parallel-right"),
			pathEdge("left-a", "left-route", "left-a", "condition", "left-a-branch"),
			pathEdge("left-b", "left-route", "left-b", "condition", "left-b-branch"),
			pathEdge("right-a", "right-route", "right-a", "manual", "right-a-branch"),
			pathEdge("right-b", "right-route", "right-b", "manual", "right-b-branch"),
			pathEdge("left-a-merge", "left-a", "merge", "sequence", ""), pathEdge("left-b-merge", "left-b", "merge", "sequence", ""),
			pathEdge("right-a-merge", "right-a", "merge", "sequence", ""), pathEdge("right-b-merge", "right-b", "merge", "sequence", ""),
			pathEdge("merge-end", "merge", "end", "sequence", ""),
		},
	}
	result, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, []model.ExecutionPathChoice{
		{RouteNodeID: "left-route", BranchID: "left-a-branch"},
		{RouteNodeID: "right-route", BranchID: "right-b-branch"},
	})
	if err != nil || !result.Complete {
		t.Fatalf("并行嵌套选择失败：result=%+v err=%v", result, err)
	}
	wants := map[string]bool{"parallel-left": true, "parallel-right": true, "left-a": true, "right-b": true}
	for _, id := range result.ReachableEdgeIDs {
		delete(wants, id)
	}
	if len(wants) != 0 {
		t.Fatalf("并行分支未全部纳入：%v", wants)
	}
}

// TestExecutionPathAnalyzerRejectsDuplicateMissingExtraAndCrossGraphChoice 验证不可信选择的拒绝边界。
func TestExecutionPathAnalyzerRejectsDuplicateMissingExtraAndCrossGraphChoice(t *testing.T) {
	graph := model.FlowGraph{
		EntryNodeIDs: []string{"route"},
		Nodes:        []model.FlowGraphNode{pathNode("route", "condition"), pathNode("a", "end"), pathNode("b", "end")},
		Edges: []model.FlowGraphEdge{
			pathEdge("route-a", "route", "a", "condition", "branch-a"),
			pathEdge("route-b", "route", "b", "condition", "branch-b"),
		},
	}
	pathAnalyzer := analyzer.NewExecutionPathAnalyzer()
	missing, err := pathAnalyzer.Analyze(graph, nil)
	if err != nil || missing.Complete || len(missing.MissingRouteNodeIDs) != 1 {
		t.Fatalf("缺失选择未正确报告：result=%+v err=%v", missing, err)
	}
	invalidChoices := [][]model.ExecutionPathChoice{
		{{RouteNodeID: "route", BranchID: "branch-a"}, {RouteNodeID: "route", BranchID: "branch-b"}},
		{{RouteNodeID: "route", BranchID: "other-branch"}},
		{{RouteNodeID: "other-route", BranchID: "branch-a"}},
	}
	for _, choices := range invalidChoices {
		if _, err := pathAnalyzer.Analyze(graph, choices); !errors.Is(err, analyzer.ErrExecutionPathInvalid) {
			t.Fatalf("非法选择未被拒绝：choices=%+v err=%v", choices, err)
		}
	}
}

// TestExecutionPathAnalyzerEnumeratesStableParallelCombinations 验证批量枚举包含并行支线中的完整笛卡尔组合并保持分支顺序。
func TestExecutionPathAnalyzerEnumeratesStableParallelCombinations(t *testing.T) {
	graph := model.FlowGraph{
		EntryNodeIDs: []string{"parallel"},
		Nodes: []model.FlowGraphNode{
			pathNode("parallel", "parallel"), pathNode("left", "condition"), pathNode("right", "manual"),
			pathNode("la", "end"), pathNode("lb", "end"), pathNode("ra", "end"), pathNode("rb", "end"),
		},
		Edges: []model.FlowGraphEdge{
			pathEdge("parallel-left", "parallel", "left", "parallel", "pl"),
			pathEdge("parallel-right", "parallel", "right", "parallel", "pr"),
			pathEdge("left-a", "left", "la", "condition", "la"), pathEdge("left-b", "left", "lb", "condition", "lb"),
			pathEdge("right-a", "right", "ra", "manual", "ra"), pathEdge("right-b", "right", "rb", "manual", "rb"),
		},
	}
	paths, err := analyzer.NewExecutionPathAnalyzer().EnumerateAll(graph, 128)
	if err != nil || len(paths) != 4 {
		t.Fatalf("并行组合枚举失败：count=%d err=%v", len(paths), err)
	}
	want := [][]model.ExecutionPathChoice{
		{{RouteNodeID: "left", BranchID: "la"}, {RouteNodeID: "right", BranchID: "ra"}},
		{{RouteNodeID: "left", BranchID: "la"}, {RouteNodeID: "right", BranchID: "rb"}},
		{{RouteNodeID: "left", BranchID: "lb"}, {RouteNodeID: "right", BranchID: "ra"}},
		{{RouteNodeID: "left", BranchID: "lb"}, {RouteNodeID: "right", BranchID: "rb"}},
	}
	for index := range want {
		if len(paths[index]) != len(want[index]) {
			t.Fatalf("第 %d 条组合长度不正确：%+v", index, paths[index])
		}
		for choiceIndex := range want[index] {
			if paths[index][choiceIndex] != want[index][choiceIndex] {
				t.Fatalf("组合顺序不稳定：got=%+v want=%+v", paths, want)
			}
		}
	}
}

// TestExecutionPathAnalyzerEnumerationHasNoBusinessCountLimit 验证大于旧上限的合法组合完整返回。
func TestExecutionPathAnalyzerEnumerationHasNoBusinessCountLimit(t *testing.T) {
	graph := binaryRouteChain(8)
	paths, err := analyzer.NewExecutionPathAnalyzer().EnumerateAll(graph, 0)
	if err != nil || len(paths) != 256 {
		t.Fatalf("大路径组合没有完整返回：count=%d err=%v", len(paths), err)
	}
	paths, err = analyzer.NewExecutionPathAnalyzer().EnumerateAll(binaryRouteChain(7), 0)
	if err != nil || len(paths) != 128 {
		t.Fatalf("128 条边界被错误拒绝：count=%d err=%v", len(paths), err)
	}
}

// binaryRouteChain 构造每层两个分支都汇入下一层的稳定组合图。
func binaryRouteChain(depth int) model.FlowGraph {
	graph := model.FlowGraph{EntryNodeIDs: []string{"route-0"}}
	for index := 0; index < depth; index++ {
		routeID := fmt.Sprintf("route-%d", index)
		nextID := fmt.Sprintf("route-%d", index+1)
		if index == depth-1 {
			nextID = "end"
		}
		graph.Nodes = append(graph.Nodes, pathNode(routeID, "condition"))
		graph.Edges = append(graph.Edges,
			pathEdge(routeID+"-a", routeID, nextID, "condition", "a"),
			pathEdge(routeID+"-b", routeID, nextID, "condition", "b"),
		)
	}
	graph.Nodes = append(graph.Nodes, pathNode("end", "end"))
	return graph
}
