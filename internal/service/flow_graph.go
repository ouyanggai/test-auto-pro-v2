package service

import (
	"context"
	"errors"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
)

type FlowTreeReader interface {
	FlowTree(context.Context, string, string, string) (*target.FlowNodeTemplate, error)
}

type FlowAnalyzer interface {
	Analyze(*target.FlowNodeTemplate) ([]model.FlowGraphNode, []model.FlowGraphEdge, []string, error)
}

type FlowGraphService struct {
	plans    *PlanService
	target   FlowTreeReader
	analyzer FlowAnalyzer
}

func NewFlowGraphService(plans *PlanService, targetReader FlowTreeReader, flowAnalyzer FlowAnalyzer) *FlowGraphService {
	return &FlowGraphService{plans: plans, target: targetReader, analyzer: flowAnalyzer}
}

func (s *FlowGraphService) Get(ctx context.Context, planID uint64) (model.FlowGraph, error) {
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return model.FlowGraph{}, err
	}
	tree, err := s.target.FlowTree(ctx, plan.Account, plan.FlowSource, plan.TargetObjectID)
	if err != nil {
		return model.FlowGraph{}, err
	}
	nodes, edges, warnings, err := s.analyzer.Analyze(tree)
	if err != nil {
		if errors.Is(err, analyzer.ErrFlowStructureInvalid) {
			return model.FlowGraph{}, analyzer.ErrFlowStructureInvalid
		}
		return model.FlowGraph{}, err
	}
	return model.FlowGraph{
		PlanID: plan.ID, TargetName: plan.TargetObjectName, FlowSource: plan.FlowSource,
		Nodes: nodes, Edges: edges, Warnings: warnings,
	}, nil
}
