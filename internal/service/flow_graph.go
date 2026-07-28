package service

import (
	"context"
	"errors"
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
)

type FlowTreeReader interface {
	FlowTreeSnapshot(context.Context, string, string, string) (target.FlowTreeSnapshot, error)
}

type FlowAnalyzer interface {
	Analyze(*target.FlowNodeTemplate) ([]model.FlowGraphNode, []model.FlowGraphEdge, []string, error)
}

type FlowGraphService struct {
	plans    *PlanService
	target   FlowTreeReader
	analyzer FlowAnalyzer
}

// NewFlowGraphService 组装持久化计划身份、目标树读取和安全图分析。
func NewFlowGraphService(plans *PlanService, targetReader FlowTreeReader, flowAnalyzer FlowAnalyzer) *FlowGraphService {
	return &FlowGraphService{plans: plans, target: targetReader, analyzer: flowAnalyzer}
}

// Get 按计划保存的身份重新读取真实图，并附加本次运行态入口集合。
func (s *FlowGraphService) Get(ctx context.Context, planID uint64) (model.FlowGraph, error) {
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return model.FlowGraph{}, err
	}
	snapshot, err := s.target.FlowTreeSnapshot(ctx, plan.Account, plan.FlowSource, plan.TargetObjectID)
	if err != nil {
		return model.FlowGraph{}, err
	}
	nodes, edges, warnings, err := s.analyzer.Analyze(snapshot.Tree)
	if err != nil {
		if errors.Is(err, analyzer.ErrFlowStructureInvalid) {
			return model.FlowGraph{}, analyzer.ErrFlowStructureInvalid
		}
		return model.FlowGraph{}, err
	}
	entries, err := validateEntryNodeIDs(snapshot.EntryNodeIDs, nodes)
	if err != nil {
		return model.FlowGraph{}, err
	}
	return model.FlowGraph{
		PlanID: plan.ID, TargetName: plan.TargetObjectName, FlowSource: plan.FlowSource,
		EntryNodeIDs: entries, Nodes: nodes, Edges: edges, Warnings: warnings,
	}, nil
}

// validateEntryNodeIDs 去重入口并确认每个入口都属于本次分析出的真实代理树。
func validateEntryNodeIDs(entryNodeIDs []string, nodes []model.FlowGraphNode) ([]string, error) {
	known := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		known[node.ID] = struct{}{}
	}
	entries := make([]string, 0, len(entryNodeIDs))
	seen := make(map[string]struct{}, len(entryNodeIDs))
	for _, rawID := range entryNodeIDs {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if _, exists := known[id]; !exists {
			return nil, ErrTargetFlowNotConfigurable
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		entries = append(entries, id)
	}
	if len(entries) == 0 {
		return nil, ErrTargetFlowNotConfigurable
	}
	return entries, nil
}
