package analyzer

import (
	"errors"
	"strings"

	"test-auto-pro-v2/internal/model"
)

var (
	ErrExecutionPathInvalid          = errors.New("执行路径选择无效")
	ErrExecutionPathEnumerationLimit = errors.New("执行路径组合超过上限")
)

type ExecutionPathAnalyzer struct{}

// NewExecutionPathAnalyzer 创建无状态的执行路径分析器。
func NewExecutionPathAnalyzer() *ExecutionPathAnalyzer { return &ExecutionPathAnalyzer{} }

// Analyze 从后端核实的入口遍历当前真实图，只接受恰好覆盖可达条件与手动路由的选择。
func (a *ExecutionPathAnalyzer) Analyze(graph model.FlowGraph, choices []model.ExecutionPathChoice) (model.ExecutionPathAnalysis, error) {
	nodeByID := make(map[string]model.FlowGraphNode, len(graph.Nodes))
	outgoing := make(map[string][]model.FlowGraphEdge, len(graph.Nodes))
	for _, node := range graph.Nodes {
		if node.ID == "" {
			return model.ExecutionPathAnalysis{}, ErrExecutionPathInvalid
		}
		nodeByID[node.ID] = node
	}
	for _, edge := range graph.Edges {
		if _, ok := nodeByID[edge.Source]; !ok {
			return model.ExecutionPathAnalysis{}, ErrExecutionPathInvalid
		}
		if _, ok := nodeByID[edge.Target]; !ok {
			return model.ExecutionPathAnalysis{}, ErrExecutionPathInvalid
		}
		outgoing[edge.Source] = append(outgoing[edge.Source], edge)
	}
	if len(graph.EntryNodeIDs) == 0 {
		return model.ExecutionPathAnalysis{}, ErrExecutionPathInvalid
	}

	choiceByRoute := make(map[string]string, len(choices))
	// 浏览器选择不是可信图结构；先拒绝空值和同一路由重复选择，再进入可达性判断。
	for _, choice := range choices {
		routeID := strings.TrimSpace(choice.RouteNodeID)
		branchID := strings.TrimSpace(choice.BranchID)
		if routeID == "" || branchID == "" {
			return model.ExecutionPathAnalysis{}, ErrExecutionPathInvalid
		}
		if _, exists := choiceByRoute[routeID]; exists {
			return model.ExecutionPathAnalysis{}, ErrExecutionPathInvalid
		}
		choiceByRoute[routeID] = branchID
	}

	visited := make(map[string]bool, len(graph.Nodes))
	usedChoices := make(map[string]bool, len(choices))
	missing := make([]string, 0)
	reachableNodes := make([]string, 0, len(graph.Nodes))
	reachableEdges := make([]string, 0, len(graph.Edges))
	queue := append([]string(nil), graph.EntryNodeIDs...)
	// 多入口和并行支线可能在汇合点重叠，visited 保证共享后继只分析一次。
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]
		if visited[nodeID] {
			continue
		}
		node, exists := nodeByID[nodeID]
		if !exists {
			return model.ExecutionPathAnalysis{}, ErrExecutionPathInvalid
		}
		visited[nodeID] = true
		reachableNodes = append(reachableNodes, nodeID)
		edges := outgoing[nodeID]
		switch node.Type {
		case "condition", "manual":
			// 单选路由缺少选择时只记录待选点，不继续猜测任何下游分支。
			branchID, selected := choiceByRoute[nodeID]
			if !selected {
				missing = append(missing, nodeID)
				continue
			}
			var selectedEdge *model.FlowGraphEdge
			for index := range edges {
				if edges[index].Kind != node.Type || strings.TrimSpace(edges[index].BranchID) != branchID {
					continue
				}
				selectedEdge = &edges[index]
				break
			}
			if selectedEdge == nil {
				return model.ExecutionPathAnalysis{}, ErrExecutionPathInvalid
			}
			usedChoices[nodeID] = true
			reachableEdges = append(reachableEdges, selectedEdge.ID)
			queue = append(queue, selectedEdge.Target)
		case "parallel":
			// 并行分支属于同一执行路径，必须整体纳入，浏览器没有取消其中一支的权力。
			if len(edges) == 0 {
				return model.ExecutionPathAnalysis{}, ErrExecutionPathInvalid
			}
			for _, edge := range edges {
				if edge.Kind != "parallel" {
					return model.ExecutionPathAnalysis{}, ErrExecutionPathInvalid
				}
				reachableEdges = append(reachableEdges, edge.ID)
				queue = append(queue, edge.Target)
			}
		default:
			if len(edges) > 1 {
				return model.ExecutionPathAnalysis{}, ErrExecutionPathInvalid
			}
			if len(edges) == 1 {
				if edges[0].Kind != "sequence" {
					return model.ExecutionPathAnalysis{}, ErrExecutionPathInvalid
				}
				reachableEdges = append(reachableEdges, edges[0].ID)
				queue = append(queue, edges[0].Target)
			}
		}
	}
	if len(usedChoices) != len(choiceByRoute) {
		// 未被遍历使用的选择来自不可达路由或其他图，必须整条拒绝。
		return model.ExecutionPathAnalysis{}, ErrExecutionPathInvalid
	}
	return model.ExecutionPathAnalysis{
		Complete: len(missing) == 0, MissingRouteNodeIDs: missing,
		ReachableNodeIDs: reachableNodes, ReachableEdgeIDs: reachableEdges,
	}, nil
}

// EnumerateAll 按真实分支顺序枚举全部合法完整路径，超过上限时立即停止并返回专用错误。
func (a *ExecutionPathAnalyzer) EnumerateAll(graph model.FlowGraph, limit int) ([][]model.ExecutionPathChoice, error) {
	if limit < 1 {
		return nil, ErrExecutionPathInvalid
	}
	outgoing := make(map[string][]model.FlowGraphEdge, len(graph.Nodes))
	for _, edge := range graph.Edges {
		outgoing[edge.Source] = append(outgoing[edge.Source], edge)
	}
	results := make([][]model.ExecutionPathChoice, 0)
	var visit func([]model.ExecutionPathChoice) error
	visit = func(choices []model.ExecutionPathChoice) error {
		analysis, err := a.Analyze(graph, choices)
		if err != nil {
			return err
		}
		if analysis.Complete {
			if len(results) >= limit {
				return ErrExecutionPathEnumerationLimit
			}
			results = append(results, append([]model.ExecutionPathChoice(nil), choices...))
			return nil
		}
		if len(analysis.MissingRouteNodeIDs) == 0 {
			return ErrExecutionPathInvalid
		}
		routeID := analysis.MissingRouteNodeIDs[0]
		branches := outgoing[routeID]
		if len(branches) == 0 {
			return ErrExecutionPathInvalid
		}
		for _, edge := range branches {
			if edge.Kind != "condition" && edge.Kind != "manual" {
				return ErrExecutionPathInvalid
			}
			// 每次只扩展分析器确认的首个待选点，可自然覆盖并行支线中的笛卡尔组合，同时保持图中稳定顺序。
			next := append(append([]model.ExecutionPathChoice(nil), choices...), model.ExecutionPathChoice{
				RouteNodeID: routeID,
				BranchID:    edge.BranchID,
			})
			if err := visit(next); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(nil); err != nil {
		return nil, err
	}
	return results, nil
}
