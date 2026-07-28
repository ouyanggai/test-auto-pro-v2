package analyzer

import (
	"errors"
	"strings"

	"test-auto-pro-v2/internal/model"
)

var ErrExecutionPathInvalid = errors.New("执行路径选择无效")

type ExecutionPathAnalyzer struct{}

func NewExecutionPathAnalyzer() *ExecutionPathAnalyzer { return &ExecutionPathAnalyzer{} }

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
		return model.ExecutionPathAnalysis{}, ErrExecutionPathInvalid
	}
	return model.ExecutionPathAnalysis{
		Complete: len(missing) == 0, MissingRouteNodeIDs: missing,
		ReachableNodeIDs: reachableNodes, ReachableEdgeIDs: reachableEdges,
	}, nil
}
