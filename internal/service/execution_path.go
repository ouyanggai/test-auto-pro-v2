package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

type ExecutionPathErrorKind string

const (
	ExecutionPathErrorInvalidArgument  ExecutionPathErrorKind = "invalid_argument"
	ExecutionPathErrorNotFound         ExecutionPathErrorKind = "not_found"
	ExecutionPathErrorInvalid          ExecutionPathErrorKind = "invalid"
	ExecutionPathErrorLimit            ExecutionPathErrorKind = "limit"
	ExecutionPathErrorEnumerationLimit ExecutionPathErrorKind = "enumeration_limit"
	ExecutionPathErrorLocked           ExecutionPathErrorKind = "locked"
	ExecutionPathErrorStorage          ExecutionPathErrorKind = "storage"
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
	EnumerateAll(model.FlowGraph, int) ([][]model.ExecutionPathChoice, error)
}

type ExecutionPathService struct {
	plans        *PlanService
	graphs       CurrentFlowGraphReader
	analyzer     ExecutionPathChoiceAnalyzer
	repository   repository.ExecutionPathRepository
	now          func() time.Time
	generationMu sync.RWMutex
	generations  map[string]*PathGenerationJob
}

// PathGenerationJob 是全路径后台任务的可查询进度快照，路径明细仍通过列表按需读取。
type PathGenerationJob struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Total     int       `json:"total"`
	Completed int       `json:"completed"`
	Created   int       `json:"created"`
	Error     string    `json:"error,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
	planID    uint64
	cancel    context.CancelFunc
}

// NewExecutionPathService 组装计划、真实图、路径分析与事务仓储边界。
func NewExecutionPathService(plans *PlanService, graphs CurrentFlowGraphReader, pathAnalyzer ExecutionPathChoiceAnalyzer, pathRepository repository.ExecutionPathRepository) *ExecutionPathService {
	return &ExecutionPathService{plans: plans, graphs: graphs, analyzer: pathAnalyzer, repository: pathRepository, now: time.Now, generations: make(map[string]*PathGenerationJob)}
}

// StartGeneration 创建或恢复指定幂等键的后台全路径解析任务。
func (s *ExecutionPathService) StartGeneration(_ context.Context, planID uint64, createKey string) (PathGenerationJob, error) {
	createKey = strings.TrimSpace(createKey)
	if !validUUID(createKey) {
		return PathGenerationJob{}, &ExecutionPathError{Kind: ExecutionPathErrorInvalidArgument, Message: "后台解析请求标识不正确，请重试"}
	}
	s.generationMu.Lock()
	if existing, found := s.generations[createKey]; found {
		copy := *existing
		s.generationMu.Unlock()
		return copy, nil
	}
	jobCtx, cancel := context.WithCancel(context.Background())
	job := &PathGenerationJob{ID: createKey, Status: "queued", UpdatedAt: s.now().UTC(), planID: planID, cancel: cancel}
	s.generations[createKey] = job
	s.generationMu.Unlock()
	go s.runGeneration(jobCtx, createKey)
	return *job, nil
}

// runGeneration 在后台执行完整路径解析，完成后只刷新任务快照。
func (s *ExecutionPathService) runGeneration(ctx context.Context, jobID string) {
	s.updateGeneration(jobID, func(job *PathGenerationJob) { job.Status = "running" })
	s.generationMu.RLock()
	planID := s.generations[jobID].planID
	s.generationMu.RUnlock()
	result, _, err := s.generatePathsBatch(ctx, planID, jobID)
	s.updateGeneration(jobID, func(job *PathGenerationJob) {
		if job.Status == "cancelled" {
			return
		}
		if err != nil {
			job.Status, job.Error = "failed", err.Error()
			return
		}
		job.Status, job.Total, job.Completed, job.Created = "completed", result.TotalCount, result.TotalCount, result.CreatedCount
	})
}

// GetGeneration 返回后台解析任务的最新进度。
func (s *ExecutionPathService) GetGeneration(_ context.Context, _ uint64, jobID string) (PathGenerationJob, error) {
	s.generationMu.RLock()
	job, found := s.generations[jobID]
	if found {
		copy := *job
		s.generationMu.RUnlock()
		return copy, nil
	}
	s.generationMu.RUnlock()
	return PathGenerationJob{}, &ExecutionPathError{Kind: ExecutionPathErrorNotFound, Message: "后台解析任务不存在"}
}

// CancelGeneration 取消尚未完成的后台解析任务，不回滚已提交路径。
func (s *ExecutionPathService) CancelGeneration(_ context.Context, _ uint64, jobID string) error {
	return s.updateGeneration(jobID, func(job *PathGenerationJob) {
		if job.Status == "queued" || job.Status == "running" {
			if job.cancel != nil {
				job.cancel()
			}
			job.Status = "cancelled"
		}
	})
}

// ResumeGeneration 恢复已取消或失败的任务，仍使用原幂等键避免重复路径。
func (s *ExecutionPathService) ResumeGeneration(_ context.Context, _ uint64, jobID string) (PathGenerationJob, error) {
	s.generationMu.Lock()
	job, found := s.generations[jobID]
	if !found {
		s.generationMu.Unlock()
		return PathGenerationJob{}, &ExecutionPathError{Kind: ExecutionPathErrorNotFound, Message: "后台解析任务不存在"}
	}
	if job.Status != "cancelled" && job.Status != "failed" {
		copy := *job
		s.generationMu.Unlock()
		return copy, nil
	}
	jobCtx, cancel := context.WithCancel(context.Background())
	job.Status, job.Error, job.UpdatedAt, job.cancel = "queued", "", s.now().UTC(), cancel
	copy := *job
	s.generationMu.Unlock()
	go s.runGeneration(jobCtx, jobID)
	return copy, nil
}

// updateGeneration 在锁内更新任务快照并刷新时间。
func (s *ExecutionPathService) updateGeneration(jobID string, update func(*PathGenerationJob)) error {
	s.generationMu.Lock()
	defer s.generationMu.Unlock()
	job, found := s.generations[jobID]
	if !found {
		return fmt.Errorf("后台解析任务不存在")
	}
	update(job)
	job.UpdatedAt = s.now().UTC()
	return nil
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

// Get 按计划归属读取单条路径 choices，列表摘要不会携带这些线路数据。
func (s *ExecutionPathService) Get(ctx context.Context, planID, pathID uint64) (model.ExecutionPath, error) {
	if planID == 0 || pathID == 0 {
		return model.ExecutionPath{}, &ExecutionPathError{Kind: ExecutionPathErrorInvalidArgument, Message: "计划或路径 ID 不正确"}
	}
	path, err := s.repository.Get(ctx, planID, pathID)
	if err != nil {
		return model.ExecutionPath{}, mapExecutionPathRepositoryError(err)
	}
	return path, nil
}

// Create 优先返回已成功的幂等记录；新请求才重读真实图并事务创建路径。
func (s *ExecutionPathService) Create(ctx context.Context, planID uint64, createKey, name string, choices []model.ExecutionPathChoice) (model.ExecutionPath, bool, error) {
	createKey = strings.TrimSpace(createKey)
	if planID == 0 || !validUUID(createKey) {
		return model.ExecutionPath{}, false, &ExecutionPathError{Kind: ExecutionPathErrorInvalidArgument, Message: "创建请求标识不正确，请重试"}
	}
	// 首次响应可能在到达浏览器前丢失；幂等命中必须先于目标图读取，避免目标随后不可用或变化时把已成功操作误报为失败。
	if existing, found, err := s.repository.FindByCreateKey(ctx, planID, createKey); err != nil {
		return model.ExecutionPath{}, false, mapExecutionPathRepositoryError(err)
	} else if found {
		return existing, false, nil
	}
	normalized, err := normalizeExecutionPathChoices(choices)
	if err != nil {
		return model.ExecutionPath{}, false, err
	}
	name, err = normalizeExecutionPathName(name)
	if err != nil {
		return model.ExecutionPath{}, false, err
	}
	if err := s.validateMutablePlan(ctx, planID); err != nil {
		return model.ExecutionPath{}, false, err
	}
	if err := s.validateCurrentChoices(ctx, planID, normalized); err != nil {
		return model.ExecutionPath{}, false, err
	}
	path, created, err := s.repository.Create(ctx, planID, createKey, name, normalized, s.now().UTC())
	if err != nil {
		return model.ExecutionPath{}, false, mapExecutionPathRepositoryError(err)
	}
	return path, created, nil
}

// Update 重新核实真实图后原位替换指定路径的全部分支选择。
func (s *ExecutionPathService) Update(ctx context.Context, planID, pathID uint64, name string, choices []model.ExecutionPathChoice) (model.ExecutionPath, error) {
	if planID == 0 || pathID == 0 {
		return model.ExecutionPath{}, &ExecutionPathError{Kind: ExecutionPathErrorInvalidArgument, Message: "计划或路径 ID 不正确"}
	}
	normalized, err := normalizeExecutionPathChoices(choices)
	if err != nil {
		return model.ExecutionPath{}, err
	}
	name, err = normalizeExecutionPathName(name)
	if err != nil {
		return model.ExecutionPath{}, err
	}
	if err := s.validateMutablePlan(ctx, planID); err != nil {
		return model.ExecutionPath{}, err
	}
	if err := s.validateCurrentChoices(ctx, planID, normalized); err != nil {
		return model.ExecutionPath{}, err
	}
	path, err := s.repository.Update(ctx, planID, pathID, name, normalized, s.now().UTC())
	if err != nil {
		return model.ExecutionPath{}, mapExecutionPathRepositoryError(err)
	}
	return path, nil
}

// generatePathsBatch 在后台任务中读取一次真实图并持久化全部合法完整线路。
func (s *ExecutionPathService) generatePathsBatch(ctx context.Context, planID uint64, createKey string) (model.ExecutionPathBatchResult, bool, error) {
	createKey = strings.TrimSpace(createKey)
	if planID == 0 || !validUUID(createKey) {
		return model.ExecutionPathBatchResult{}, false, &ExecutionPathError{Kind: ExecutionPathErrorInvalidArgument, Message: "批量请求标识不正确，请重试"}
	}
	// 已提交批次必须先于计划和目标读取返回，保证目标稍后变化或不可用时重试仍得到同一事实。
	if existing, found, err := s.repository.FindBatchByCreateKey(ctx, planID, createKey); err != nil {
		return model.ExecutionPathBatchResult{}, false, mapExecutionPathRepositoryError(err)
	} else if found {
		return existing, false, nil
	}
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return model.ExecutionPathBatchResult{}, false, err
	}
	if plan.Status != model.PlanStatusPendingConfiguration {
		return model.ExecutionPathBatchResult{}, false, &ExecutionPathError{Kind: ExecutionPathErrorLocked, Message: "计划已经不能修改执行路径"}
	}
	if plan.FlowSource != "new" {
		return model.ExecutionPathBatchResult{}, false, &ExecutionPathError{Kind: ExecutionPathErrorInvalid, Message: "只有新发起计划可以自动解析全部路径"}
	}
	graph, err := s.graphs.Get(ctx, planID)
	if err != nil {
		return model.ExecutionPathBatchResult{}, false, err
	}
	candidates, err := s.analyzer.EnumerateAll(graph, 0)
	if errors.Is(err, analyzer.ErrExecutionPathEnumerationLimit) {
		return model.ExecutionPathBatchResult{}, false, &ExecutionPathError{Kind: ExecutionPathErrorEnumerationLimit, Message: "当前流程路径解析超过资源保护阈值，请恢复任务"}
	}
	if err != nil {
		return model.ExecutionPathBatchResult{}, false, &ExecutionPathError{Kind: ExecutionPathErrorInvalid, Message: "当前流程结构无法生成完整路径"}
	}
	result, created, err := s.repository.GeneratePathsBatch(ctx, planID, createKey, candidates, s.now().UTC())
	if err != nil {
		return model.ExecutionPathBatchResult{}, false, mapExecutionPathRepositoryError(err)
	}
	return result, created, nil
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

// normalizeExecutionPathName 去除首尾空格并限制自定义名称，空值由事务按实际序号恢复默认名。
func normalizeExecutionPathName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > 50 {
		return "", &ExecutionPathError{Kind: ExecutionPathErrorInvalidArgument, Message: "路径名称最多 50 个字符"}
	}
	return name, nil
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
	case errors.Is(err, repository.ErrExecutionPathSource):
		return &ExecutionPathError{Kind: ExecutionPathErrorInvalid, Message: "只有新发起计划可以自动解析全部路径"}
	case errors.Is(err, repository.ErrExecutionPathPlanLocked):
		return &ExecutionPathError{Kind: ExecutionPathErrorLocked, Message: "计划已经不能修改执行路径"}
	case errors.Is(err, repository.ErrExecutionPathDataInvalid):
		return &ExecutionPathError{Kind: ExecutionPathErrorInvalidArgument, Message: "执行路径数据不正确"}
	default:
		return &ExecutionPathError{Kind: ExecutionPathErrorStorage, Message: "路径存储暂不可用"}
	}
}

var _ ExecutionPathChoiceAnalyzer = (*analyzer.ExecutionPathAnalyzer)(nil)
