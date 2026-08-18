package service

import (
	"context"
	"errors"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// RequirementSnapshotReader 读取同一真实流程树、当前入口和表单字段元数据。
type RequirementSnapshotReader interface {
	FlowRequirementSnapshot(context.Context, string, string, string) (target.FlowRequirementSnapshot, error)
}

// RequirementAnalyzer 把完整路径投影为安全中文要求。
type RequirementAnalyzer interface {
	Analyze(model.FlowGraph, *target.FlowNodeTemplate, []target.FormFieldMetadata, model.ExecutionPath, model.ExecutionPathAnalysis) (model.PathRequirements, error)
}

// PathRequirementService 组织计划身份、路径归属、当前真实图和要求分析。
type PathRequirementService struct {
	plans          *PlanService
	target         RequirementSnapshotReader
	flowAnalyzer   FlowAnalyzer
	pathAnalyzer   ExecutionPathChoiceAnalyzer
	requirements   RequirementAnalyzer
	pathRepository repository.ExecutionPathRepository
}

// NewPathRequirementService 创建只读路径要求服务，不持久化任何要求快照。
func NewPathRequirementService(plans *PlanService, targetReader RequirementSnapshotReader, flowAnalyzer FlowAnalyzer, pathAnalyzer ExecutionPathChoiceAnalyzer, requirementAnalyzer RequirementAnalyzer, pathRepository repository.ExecutionPathRepository) *PathRequirementService {
	return &PathRequirementService{
		plans: plans, target: targetReader, flowAnalyzer: flowAnalyzer, pathAnalyzer: pathAnalyzer,
		requirements: requirementAnalyzer, pathRepository: pathRepository,
	}
}

// Get 校验路径属于计划后重读当前真实配置，并拒绝已失效或不完整的保存路径。
func (s *PathRequirementService) Get(ctx context.Context, planID, pathID uint64) (model.PathRequirements, error) {
	if planID == 0 || pathID == 0 {
		return model.PathRequirements{}, &ExecutionPathError{Kind: ExecutionPathErrorInvalidArgument, Message: "计划或路径 ID 不正确"}
	}
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return model.PathRequirements{}, err
	}
	path, err := s.pathRepository.Get(ctx, planID, pathID)
	if err != nil {
		return model.PathRequirements{}, mapExecutionPathRepositoryError(err)
	}

	snapshot, err := s.target.FlowRequirementSnapshot(ctx, plan.Account, plan.FlowSource, plan.TargetObjectID)
	if err != nil {
		return model.PathRequirements{}, err
	}
	nodes, edges, warnings, err := s.flowAnalyzer.Analyze(snapshot.Tree)
	if err != nil {
		if errors.Is(err, analyzer.ErrFlowStructureInvalid) {
			return model.PathRequirements{}, analyzer.ErrFlowStructureInvalid
		}
		return model.PathRequirements{}, err
	}
	entries, err := validateEntryNodeIDs(snapshot.EntryNodeIDs, nodes)
	if err != nil {
		return model.PathRequirements{}, err
	}
	graph := model.FlowGraph{
		PlanID: plan.ID, TargetName: plan.TargetObjectName, FlowSource: plan.FlowSource,
		EntryNodeIDs: entries, Nodes: nodes, Edges: edges, Warnings: warnings,
	}
	analysis, err := s.pathAnalyzer.Analyze(graph, path.Choices)
	if err != nil || !analysis.Complete {
		return model.PathRequirements{}, &ExecutionPathError{Kind: ExecutionPathErrorInvalid, Message: "当前已保存路径与真实流程不一致，请先编辑路径"}
	}
	result, err := s.requirements.Analyze(graph, snapshot.Tree, snapshot.FormFields, path, analysis)
	if err != nil {
		return model.PathRequirements{}, &ExecutionPathError{Kind: ExecutionPathErrorInvalid, Message: "当前已保存路径与真实流程不一致，请先编辑路径"}
	}
	return result, nil
}
