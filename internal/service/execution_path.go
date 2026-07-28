package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

type ExecutionPathErrorKind string

const (
	ExecutionPathErrorInvalidArgument ExecutionPathErrorKind = "invalid_argument"
	ExecutionPathErrorNotFound        ExecutionPathErrorKind = "not_found"
	ExecutionPathErrorInvalid         ExecutionPathErrorKind = "invalid"
	ExecutionPathErrorLimit           ExecutionPathErrorKind = "limit"
	ExecutionPathErrorLocked          ExecutionPathErrorKind = "locked"
	ExecutionPathErrorStorage         ExecutionPathErrorKind = "storage"
)

type ExecutionPathError struct {
	Kind    ExecutionPathErrorKind
	Message string
}

// Error 返回可映射为稳定 API 错误的人类可读说明。
func (e *ExecutionPathError) Error() string { return e.Message }

// IsExecutionPathErrorKind 判断错误是否属于指定的执行路径业务边界。
func IsExecutionPathErrorKind(err error, kind ExecutionPathErrorKind) bool {
	var pathErr *ExecutionPathError
	return errors.As(err, &pathErr) && pathErr.Kind == kind
}

type CurrentFlowGraphReader interface {
	Get(context.Context, uint64) (model.FlowGraph, error)
}

type ExecutionPathChoiceAnalyzer interface {
	Analyze(model.FlowGraph, []model.ExecutionPathChoice) (model.ExecutionPathAnalysis, error)
}

type ExecutionPathService struct {
	plans      *PlanService
	graphs     CurrentFlowGraphReader
	analyzer   ExecutionPathChoiceAnalyzer
	repository repository.ExecutionPathRepository
	now        func() time.Time
}

// NewExecutionPathService 组装计划、真实图、路径分析与事务仓储边界。
func NewExecutionPathService(plans *PlanService, graphs CurrentFlowGraphReader, pathAnalyzer ExecutionPathChoiceAnalyzer, pathRepository repository.ExecutionPathRepository) *ExecutionPathService {
	return &ExecutionPathService{plans: plans, graphs: graphs, analyzer: pathAnalyzer, repository: pathRepository, now: time.Now}
}

// List 读取属于指定计划的持久化路径，不伪造当前图有效性。
func (s *ExecutionPathService) List(ctx context.Context, planID uint64) ([]model.ExecutionPath, error) {
	if planID == 0 {
		return nil, &ExecutionPathError{Kind: ExecutionPathErrorInvalidArgument, Message: "计划 ID 不正确"}
	}
	if _, err := s.plans.Get(ctx, planID); err != nil {
		return nil, err
	}
	paths, err := s.repository.List(ctx, planID)
	if err != nil {
		return nil, mapExecutionPathRepositoryError(err)
	}
	return paths, nil
}

// Create 重新读取当前真实图验证完整选择，再以幂等键事务创建路径。
func (s *ExecutionPathService) Create(ctx context.Context, planID uint64, createKey string, choices []model.ExecutionPathChoice) (model.ExecutionPath, bool, error) {
	if planID == 0 || !validUUID(strings.TrimSpace(createKey)) {
		return model.ExecutionPath{}, false, &ExecutionPathError{Kind: ExecutionPathErrorInvalidArgument, Message: "创建请求标识不正确，请重试"}
	}
	normalized, err := normalizeExecutionPathChoices(choices)
	if err != nil {
		return model.ExecutionPath{}, false, err
	}
	if err := s.validateMutablePlan(ctx, planID); err != nil {
		return model.ExecutionPath{}, false, err
	}
	if err := s.validateCurrentChoices(ctx, planID, normalized); err != nil {
		return model.ExecutionPath{}, false, err
	}
	path, created, err := s.repository.Create(ctx, planID, strings.TrimSpace(createKey), normalized, s.now().UTC())
	if err != nil {
		return model.ExecutionPath{}, false, mapExecutionPathRepositoryError(err)
	}
	return path, created, nil
}

// Update 重新核实真实图后原位替换指定路径的全部分支选择。
func (s *ExecutionPathService) Update(ctx context.Context, planID, pathID uint64, choices []model.ExecutionPathChoice) (model.ExecutionPath, error) {
	if planID == 0 || pathID == 0 {
		return model.ExecutionPath{}, &ExecutionPathError{Kind: ExecutionPathErrorInvalidArgument, Message: "计划或路径 ID 不正确"}
	}
	normalized, err := normalizeExecutionPathChoices(choices)
	if err != nil {
		return model.ExecutionPath{}, err
	}
	if err := s.validateMutablePlan(ctx, planID); err != nil {
		return model.ExecutionPath{}, err
	}
	if err := s.validateCurrentChoices(ctx, planID, normalized); err != nil {
		return model.ExecutionPath{}, err
	}
	path, err := s.repository.Update(ctx, planID, pathID, normalized, s.now().UTC())
	if err != nil {
		return model.ExecutionPath{}, mapExecutionPathRepositoryError(err)
	}
	return path, nil
}

// Delete 只删除本工具中属于待配置计划的路径，不访问目标系统。
func (s *ExecutionPathService) Delete(ctx context.Context, planID, pathID uint64) error {
	if planID == 0 || pathID == 0 {
		return &ExecutionPathError{Kind: ExecutionPathErrorInvalidArgument, Message: "计划或路径 ID 不正确"}
	}
	if err := s.repository.Delete(ctx, planID, pathID, s.now().UTC()); err != nil {
		return mapExecutionPathRepositoryError(err)
	}
	return nil
}

// validateMutablePlan 在访问目标图前阻止已经产生后续事实的计划继续修改。
func (s *ExecutionPathService) validateMutablePlan(ctx context.Context, planID uint64) error {
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return err
	}
	if plan.Status != model.PlanStatusPendingConfiguration {
		return &ExecutionPathError{Kind: ExecutionPathErrorLocked, Message: "计划已经不能修改执行路径"}
	}
	return nil
}

// validateCurrentChoices 用计划持久化身份重读真实图，禁止浏览器用过期或跨图选择写库。
func (s *ExecutionPathService) validateCurrentChoices(ctx context.Context, planID uint64, choices []model.ExecutionPathChoice) error {
	graph, err := s.graphs.Get(ctx, planID)
	if err != nil {
		return err
	}
	analysis, err := s.analyzer.Analyze(graph, choices)
	if err != nil || !analysis.Complete {
		return &ExecutionPathError{Kind: ExecutionPathErrorInvalid, Message: "执行路径选择不完整或已失效"}
	}
	return nil
}

// normalizeExecutionPathChoices 收敛请求标识并限制数据库字段与图规模边界。
func normalizeExecutionPathChoices(choices []model.ExecutionPathChoice) ([]model.ExecutionPathChoice, error) {
	if len(choices) > 500 {
		return nil, &ExecutionPathError{Kind: ExecutionPathErrorInvalidArgument, Message: "路径选择数量过多"}
	}
	normalized := make([]model.ExecutionPathChoice, 0, len(choices))
	for _, choice := range choices {
		choice.RouteNodeID = strings.TrimSpace(choice.RouteNodeID)
		choice.BranchID = strings.TrimSpace(choice.BranchID)
		if choice.RouteNodeID == "" || choice.BranchID == "" || utf8.RuneCountInString(choice.RouteNodeID) > 100 || utf8.RuneCountInString(choice.BranchID) > 100 {
			return nil, &ExecutionPathError{Kind: ExecutionPathErrorInvalidArgument, Message: "路径分支标识不正确"}
		}
		normalized = append(normalized, choice)
	}
	return normalized, nil
}

// mapExecutionPathRepositoryError 将事务仓储错误收敛为稳定的业务错误种类。
func mapExecutionPathRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrPlanNotFound):
		return &PlanError{Kind: PlanErrorNotFound, Message: "计划不存在"}
	case errors.Is(err, repository.ErrExecutionPathNotFound):
		return &ExecutionPathError{Kind: ExecutionPathErrorNotFound, Message: "执行路径不存在"}
	case errors.Is(err, repository.ErrExecutionPathLimit):
		return &ExecutionPathError{Kind: ExecutionPathErrorLimit, Message: "当前计划最多只能保存一条执行路径"}
	case errors.Is(err, repository.ErrExecutionPathPlanLocked):
		return &ExecutionPathError{Kind: ExecutionPathErrorLocked, Message: "计划已经不能修改执行路径"}
	case errors.Is(err, repository.ErrExecutionPathDataInvalid):
		return &ExecutionPathError{Kind: ExecutionPathErrorInvalidArgument, Message: "执行路径数据不正确"}
	default:
		return &ExecutionPathError{Kind: ExecutionPathErrorStorage, Message: "路径存储暂不可用"}
	}
}

var _ ExecutionPathChoiceAnalyzer = (*analyzer.ExecutionPathAnalyzer)(nil)
