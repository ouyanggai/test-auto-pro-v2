package analyzer

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/model"
)

var ErrFlowStructureInvalid = errors.New("目标流程结构异常")

const (
	maxFlowNodes = 500
	maxFlowDepth = 200
)

type FlowGraphAnalyzer struct{}

func NewFlowGraphAnalyzer() *FlowGraphAnalyzer { return &FlowGraphAnalyzer{} }

type graphBuilder struct {
	nodes     map[string]model.FlowGraphNode
	nodeOrder []string
	edges     map[string]model.FlowGraphEdge
	edgeOrder []string
	warnings  []string
	stack     map[string]bool
	seen      map[string]bool
}

func (a *FlowGraphAnalyzer) Analyze(root *target.FlowNodeTemplate) ([]model.FlowGraphNode, []model.FlowGraphEdge, []string, error) {
	if root == nil {
		return nil, nil, nil, ErrFlowStructureInvalid
	}
	builder := &graphBuilder{
		nodes: make(map[string]model.FlowGraphNode), edges: make(map[string]model.FlowGraphEdge),
		stack: make(map[string]bool), seen: make(map[string]bool),
	}
	if _, err := builder.parse(root, 1); err != nil {
		return nil, nil, nil, err
	}
	nodes := make([]model.FlowGraphNode, 0, len(builder.nodeOrder))
	for _, id := range builder.nodeOrder {
		nodes = append(nodes, builder.nodes[id])
	}
	edges := make([]model.FlowGraphEdge, 0, len(builder.edgeOrder))
	for _, id := range builder.edgeOrder {
		edges = append(edges, builder.edges[id])
	}
	return nodes, edges, builder.warnings, nil
}

func (b *graphBuilder) parse(node *target.FlowNodeTemplate, depth int) ([]string, error) {
	if node == nil || depth > maxFlowDepth {
		return nil, ErrFlowStructureInvalid
	}
	id := strings.TrimSpace(node.ID)
	if id == "" || b.stack[id] || b.seen[id] {
		return nil, ErrFlowStructureInvalid
	}
	b.seen[id] = true
	b.stack[id] = true
	defer delete(b.stack, id)

	typeName, known := flowNodeType(node.Type, node.BranchExecuteType)
	if !known && (len(node.ConditionNodes) > 0 || len(node.ParallelNodes) > 0) {
		return nil, ErrFlowStructureInvalid
	}
	if _, exists := b.nodes[id]; !exists {
		if len(b.nodes) >= maxFlowNodes {
			return nil, ErrFlowStructureInvalid
		}
		name := strings.TrimSpace(node.Name)
		if name == "" {
			name = typeName
		}
		b.nodes[id] = model.FlowGraphNode{ID: id, Name: name, Type: normalizedNodeType(node.Type, node.BranchExecuteType), TypeName: typeName}
		b.nodeOrder = append(b.nodeOrder, id)
		if !known {
			b.warnings = append(b.warnings, fmt.Sprintf("节点“%s”使用未知类型", name))
		}
	}

	branches, kind := branchesFor(node)
	if kind != "" {
		if len(branches) == 0 {
			return nil, ErrFlowStructureInvalid
		}
		sort.SliceStable(branches, func(i, j int) bool {
			if branches[i].Sort == branches[j].Sort {
				return false
			}
			return branches[i].Sort < branches[j].Sort
		})
		branchExits := make([]string, 0)
		for index, branch := range branches {
			branchID := strings.TrimSpace(branch.ID)
			if branchID == "" || branch.Child == nil || strings.TrimSpace(branch.Child.ID) == "" {
				return nil, ErrFlowStructureInvalid
			}
			label := strings.TrimSpace(branch.Name)
			if label == "" && kind == "parallel" {
				label = fmt.Sprintf("分支 %d", index+1)
			}
			if err := b.addEdge(id, branch.Child.ID, kind, label, branchID); err != nil {
				return nil, err
			}
			exits, err := b.parse(branch.Child, depth+1)
			if err != nil {
				return nil, err
			}
			branchExits = appendUnique(branchExits, exits...)
		}
		if node.Child == nil {
			return branchExits, nil
		}
		nextID := strings.TrimSpace(node.Child.ID)
		if nextID == "" {
			return nil, ErrFlowStructureInvalid
		}
		graphNode := b.nodes[id]
		graphNode.MergeTargetID = nextID
		b.nodes[id] = graphNode
		for _, exit := range branchExits {
			if err := b.addEdge(exit, nextID, "sequence", "", ""); err != nil {
				return nil, err
			}
		}
		exits, err := b.parse(node.Child, depth+1)
		if err != nil {
			return nil, err
		}
		return exits, nil
	}

	if node.Child == nil {
		return []string{id}, nil
	}
	nextID := strings.TrimSpace(node.Child.ID)
	if nextID == "" {
		return nil, ErrFlowStructureInvalid
	}
	if err := b.addEdge(id, nextID, "sequence", "", ""); err != nil {
		return nil, err
	}
	exits, err := b.parse(node.Child, depth+1)
	if err != nil {
		return nil, err
	}
	return exits, nil
}

func (b *graphBuilder) addEdge(source, destination, kind, label, branchID string) error {
	source = strings.TrimSpace(source)
	destination = strings.TrimSpace(destination)
	if source == "" || destination == "" || source == destination {
		return ErrFlowStructureInvalid
	}
	id := strings.Join([]string{source, destination, kind, branchID}, "|")
	if _, exists := b.edges[id]; exists {
		return nil
	}
	b.edges[id] = model.FlowGraphEdge{ID: id, Source: source, Target: destination, Kind: kind, Label: label, BranchID: branchID}
	b.edgeOrder = append(b.edgeOrder, id)
	return nil
}

func flowNodeType(value, branchExecuteType string) (string, bool) {
	switch strings.TrimSpace(value) {
	case "start":
		return "发起", true
	case "empty":
		return "空节点", true
	case "parallel":
		return "并行", true
	case "synergy":
		return "协同", true
	case "common":
		return "审批", true
	case "condition":
		if strings.TrimSpace(branchExecuteType) == "custom_choose" {
			return "手动分支", true
		}
		return "条件", true
	case "end":
		return "结束", true
	default:
		return "未知类型", false
	}
}

func normalizedNodeType(value, branchExecuteType string) string {
	value = strings.TrimSpace(value)
	if value == "condition" && strings.TrimSpace(branchExecuteType) == "custom_choose" {
		return "manual"
	}
	if _, known := flowNodeType(value, branchExecuteType); !known {
		return "unknown"
	}
	return value
}

func branchesFor(node *target.FlowNodeTemplate) ([]target.FlowBranchTemplate, string) {
	if node.Type == "parallel" {
		return append([]target.FlowBranchTemplate(nil), node.ParallelNodes...), "parallel"
	}
	if node.Type == "condition" {
		kind := "condition"
		if node.BranchExecuteType == "custom_choose" {
			kind = "manual"
		}
		return append([]target.FlowBranchTemplate(nil), node.ConditionNodes...), kind
	}
	return nil, ""
}

func appendUnique(values []string, additions ...string) []string {
	seen := make(map[string]bool, len(values)+len(additions))
	for _, value := range values {
		seen[value] = true
	}
	for _, value := range additions {
		if !seen[value] {
			seen[value] = true
			values = append(values, value)
		}
	}
	return values
}
