package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/formdata/branchoverlay"
	"test-auto-pro-v2/internal/logging"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

const (
	historyReplayWorkerBatchSize = 8
	historyReplayWorkerTimeout   = 10 * time.Minute
	historyReplayWorkerStopWait  = 2 * time.Second
)

// HistoryReplayErrorKind 是历史回放接口向 HTTP 层公开的稳定错误分类。
type HistoryReplayErrorKind string

const (
	HistoryReplayErrorInvalidArgument HistoryReplayErrorKind = "invalid_argument"
	HistoryReplayErrorNotFound        HistoryReplayErrorKind = "not_found"
	HistoryReplayErrorConflict        HistoryReplayErrorKind = "conflict"
	HistoryReplayErrorState           HistoryReplayErrorKind = "state"
	HistoryReplayErrorStorage         HistoryReplayErrorKind = "storage"
	HistoryReplayErrorTarget          HistoryReplayErrorKind = "target"
)

// HistoryReplayError 把任务状态、路径归属和目标读取失败收敛为脱敏业务错误。
type HistoryReplayError struct {
	Kind    HistoryReplayErrorKind
	Message string
}

// Error 返回稳定历史回放错误文案。
func (e *HistoryReplayError) Error() string { return e.Message }

// IsHistoryReplayErrorKind 判断错误是否属于指定历史回放错误类别。
func IsHistoryReplayErrorKind(err error, kind HistoryReplayErrorKind) bool {
	var replayErr *HistoryReplayError
	return errors.As(err, &replayErr) && replayErr.Kind == kind
}

// HistoryReplayRuntimeValidator 是复制 form-runtime 的窄验证边界，只接受目标原始数据和运行时类型。
// 实现不得把数据包裹为工具侧 DTO，也不得调用目标平台写接口。
type HistoryReplayRuntimeValidator interface {
	Validate(context.Context, target.FormRenderType, map[string]any) (model.HistoryRuntimeValidation, error)
}

// HistoryReplayRuntimeValidatorFunc 将函数适配为历史回放运行时校验器，便于隔离测试和 iframe 桥接。
type HistoryReplayRuntimeValidatorFunc func(context.Context, target.FormRenderType, map[string]any) (model.HistoryRuntimeValidation, error)

// Validate 执行函数适配器并保持原始 map 作为唯一数据载体。
func (f HistoryReplayRuntimeValidatorFunc) Validate(ctx context.Context, renderType target.FormRenderType, values map[string]any) (model.HistoryRuntimeValidation, error) {
	if f == nil {
		return model.HistoryRuntimeValidation{}, errors.New("form-runtime 校验器未配置")
	}
	return f(ctx, renderType, values)
}

// HistoryReplayTargetReader 读取同计划当前真实流程树，不暴露目标写接口或旧页面映射。
type HistoryReplayTargetReader interface {
	PathConfigurationSnapshot(context.Context, string, string, string) (target.PathConfigurationSnapshot, error)
}

// historyReplayPathConfigReader 读取路径配置记录中的来源模式，保证批量任务与数据工作区使用同一来源事实。
type historyReplayPathConfigReader interface {
	GetPathConfig(context.Context, uint64) (repository.HistoryPathConfigRecord, bool, error)
}

// HistoryReplayService 创建和驱动可恢复的历史回放任务。
type HistoryReplayService struct {
	plans   *PlanService
	paths   repository.ExecutionPathRepository
	target  HistoryReplayTargetReader
	store   repository.HistoryReplayStore
	runtime HistoryReplayRuntimeValidator
	actions HistoryReplayActionConfigurator
	config  historyReplayPathConfigReader
	now     func() time.Time
	worker  string
	mu      sync.Mutex
	running map[string]struct{}
	cancel  map[string]context.CancelFunc
	done    map[string]chan struct{}
}

// HistoryReplayActionConfigurator 在一键配置时按真实门禁为路径补齐节点动作配置。
type HistoryReplayActionConfigurator interface {
	AutoConfigurePathActions(ctx context.Context, planID, pathID uint64) error
}

// NewHistoryReplayService 组装历史回放任务的计划、路径、目标只读和持久化边界。
func NewHistoryReplayService(plans *PlanService, paths repository.ExecutionPathRepository, targetReader HistoryReplayTargetReader, store repository.HistoryReplayStore) *HistoryReplayService {
	var config historyReplayPathConfigReader
	if reader, ok := store.(historyReplayPathConfigReader); ok {
		config = reader
	}
	return &HistoryReplayService{
		plans: plans, paths: paths, target: targetReader, store: store, config: config, now: time.Now,
		worker: "history-replay-worker", running: make(map[string]struct{}), cancel: make(map[string]context.CancelFunc), done: make(map[string]chan struct{}),
	}
}

// SetRuntimeValidator 注入复制 form-runtime 的校验桥接；未注入时任务必须明确落 needs_input。
// SetActionConfigurator 注入一键配置时的自动动作配置协作者；未注入时只做业务数据回放。
func (s *HistoryReplayService) SetActionConfigurator(configurator HistoryReplayActionConfigurator) {
	s.mu.Lock()
	s.actions = configurator
	s.mu.Unlock()
}

func (s *HistoryReplayService) SetRuntimeValidator(validator HistoryReplayRuntimeValidator) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runtime = validator
}

// Create 按明确路径列表冻结来源快照并创建单计划活动任务，重复幂等键只返回同一任务。
func (s *HistoryReplayService) Create(ctx context.Context, planID uint64, input model.HistoryReplayCreateInput, idempotencyKey string) (model.HistoryReplayJob, error) {
	if s == nil || s.store == nil || s.paths == nil || s.plans == nil {
		return model.HistoryReplayJob{}, historyReplayStorageError()
	}
	if planID == 0 {
		return model.HistoryReplayJob{}, historyReplayInvalid("计划 ID 不正确")
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !validUUID(idempotencyKey) {
		return model.HistoryReplayJob{}, historyReplayInvalid("批量准备请求标识不正确，请重试")
	}
	pathIDs, err := normalizeReplayPathIDs(input.PathIDs)
	if err != nil {
		return model.HistoryReplayJob{}, err
	}
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return model.HistoryReplayJob{}, mapHistoryReplayRepositoryError(err)
	}
	if plan.Status != model.PlanStatusNotStarted {
		return model.HistoryReplayJob{}, &HistoryReplayError{Kind: HistoryReplayErrorConflict, Message: "计划已经不能执行批量准备"}
	}
	paths, err := s.paths.GetMany(ctx, planID, pathIDs)
	if err != nil {
		return model.HistoryReplayJob{}, mapHistoryReplayRepositoryError(err)
	}
	if len(paths) != len(pathIDs) {
		return model.HistoryReplayJob{}, &HistoryReplayError{Kind: HistoryReplayErrorNotFound, Message: "部分执行路径不存在或已删除"}
	}
	pathByID := make(map[uint64]model.ExecutionPath, len(paths))
	for _, path := range paths {
		pathByID[path.ID] = path
	}
	items := make([]model.HistoryReplayItem, 0, len(pathIDs))
	for _, pathID := range pathIDs {
		path := pathByID[pathID]
		snapshotID, sourceIssue, sourceErr := s.resolveReplaySnapshot(ctx, planID, pathID)
		if sourceErr != nil {
			return model.HistoryReplayJob{}, sourceErr
		}
		issues := []model.HistoryDataIssue{}
		if sourceIssue != nil {
			issues = append(issues, *sourceIssue)
		}
		items = append(items, model.HistoryReplayItem{
			PathID: path.ID, PathRevision: path.ConfigurationRevision, SnapshotID: snapshotID,
			Status: model.HistoryReplayItemStatusPending, DataStatus: model.HistoryDataStatusEmpty,
			Issues: issues, BranchPatches: []model.HistoryBranchPatch{}, EffectiveFormData: map[string]any{},
		})
	}
	now := s.now().UTC()
	job := model.HistoryReplayJob{ID: idempotencyKey, IdempotencyKey: idempotencyKey, PlanID: planID, Status: model.HistoryReplayStatusQueued, CreatedAt: now, UpdatedAt: now}
	persisted, _, err := s.store.CreateReplay(ctx, job, items)
	if err != nil {
		return model.HistoryReplayJob{}, mapHistoryReplayRepositoryError(err)
	}
	s.startWorker(ctx, persisted)
	return publicHistoryReplayJob(persisted), nil
}

// Get 读取指定计划任务的真实聚合计数。
func (s *HistoryReplayService) Get(ctx context.Context, planID uint64, jobID string) (model.HistoryReplayJob, error) {
	if s == nil || s.store == nil {
		return model.HistoryReplayJob{}, historyReplayStorageError()
	}
	if planID == 0 || strings.TrimSpace(jobID) == "" {
		return model.HistoryReplayJob{}, historyReplayInvalid("计划或任务 ID 不正确")
	}
	job, err := s.store.GetReplay(ctx, planID, strings.TrimSpace(jobID))
	if err != nil {
		return model.HistoryReplayJob{}, mapHistoryReplayRepositoryError(err)
	}
	return publicHistoryReplayJob(job), nil
}

// Active 返回当前计划排队或运行中的唯一任务。
func (s *HistoryReplayService) Active(ctx context.Context, planID uint64) (model.HistoryReplayJob, bool, error) {
	if s == nil || s.store == nil {
		return model.HistoryReplayJob{}, false, historyReplayStorageError()
	}
	if planID == 0 {
		return model.HistoryReplayJob{}, false, historyReplayInvalid("计划 ID 不正确")
	}
	job, found, err := s.store.FindActiveReplay(ctx, planID)
	if err != nil {
		return model.HistoryReplayJob{}, false, mapHistoryReplayRepositoryError(err)
	}
	if !found {
		return model.HistoryReplayJob{}, false, nil
	}
	s.startWorker(ctx, job)
	return publicHistoryReplayJob(job), true, nil
}

// Cancel 取消排队或运行任务并保留已完成明细，当前租约中的明细会退回待处理。
func (s *HistoryReplayService) Cancel(ctx context.Context, planID uint64, jobID string) (model.HistoryReplayJob, error) {
	if s == nil || s.store == nil {
		return model.HistoryReplayJob{}, historyReplayStorageError()
	}
	if planID == 0 || strings.TrimSpace(jobID) == "" {
		return model.HistoryReplayJob{}, historyReplayInvalid("计划或任务 ID 不正确")
	}
	jobID = strings.TrimSpace(jobID)
	job, err := s.store.UpdateReplayStatus(ctx, planID, jobID, model.HistoryReplayStatusCancelled, s.now().UTC())
	if err != nil {
		return model.HistoryReplayJob{}, mapHistoryReplayRepositoryError(err)
	}
	// 先由数据库状态锁定取消事实，再停止进程内 worker；这样并发完成只能胜出或被拒绝，不会产生半取消状态。
	s.stopWorker(jobID)
	return publicHistoryReplayJob(job), nil
}

// Resume 从取消或失败任务的未完成检查点重新排队，已完成明细不会重复处理。
func (s *HistoryReplayService) Resume(ctx context.Context, planID uint64, jobID string) (model.HistoryReplayJob, error) {
	if s == nil || s.store == nil {
		return model.HistoryReplayJob{}, historyReplayStorageError()
	}
	if planID == 0 || strings.TrimSpace(jobID) == "" {
		return model.HistoryReplayJob{}, historyReplayInvalid("计划或任务 ID 不正确")
	}
	job, err := s.store.UpdateReplayStatus(ctx, planID, strings.TrimSpace(jobID), model.HistoryReplayStatusQueued, s.now().UTC())
	if err != nil {
		return model.HistoryReplayJob{}, mapHistoryReplayRepositoryError(err)
	}
	s.startWorker(ctx, job)
	return publicHistoryReplayJob(job), nil
}

// ListItems 返回按自增明细 ID 的有界游标分页结果。
func (s *HistoryReplayService) ListItems(ctx context.Context, planID uint64, jobID string, cursor uint64, limit int) (model.HistoryReplayItemPage, error) {
	if s == nil || s.store == nil {
		return model.HistoryReplayItemPage{}, historyReplayStorageError()
	}
	if planID == 0 || strings.TrimSpace(jobID) == "" || limit < 1 || limit > 100 {
		return model.HistoryReplayItemPage{}, historyReplayInvalid("任务明细分页参数不正确")
	}
	page, err := s.store.ListReplayItems(ctx, planID, strings.TrimSpace(jobID), cursor, limit)
	if err != nil {
		return model.HistoryReplayItemPage{}, mapHistoryReplayRepositoryError(err)
	}
	return page, nil
}

// Recover 在服务启动时重新领取排队或租约已过期的任务，保证进程重启不会遗失持久化检查点。
func (s *HistoryReplayService) Recover(ctx context.Context) error {
	if s == nil || s.store == nil {
		return historyReplayStorageError()
	}
	jobs, err := s.store.ListRecoverableReplays(ctx)
	if err != nil {
		return mapHistoryReplayRepositoryError(err)
	}
	for _, job := range jobs {
		s.startWorker(ctx, job)
	}
	return nil
}

// resolveReplaySnapshot 在创建事务前解析路径覆盖或计划默认来源，明细保存的是明确快照 ID。
func (s *HistoryReplayService) resolveReplaySnapshot(ctx context.Context, planID, pathID uint64) (*uint64, *model.HistoryDataIssue, error) {
	mode := model.HistorySourceModeDefault
	var pathSource repository.HistoryPathSourceRecord
	pathSourceFound := false
	var configSource repository.HistoryPathConfigRecord
	configFound := false
	if s.config != nil {
		var err error
		configSource, configFound, err = s.config.GetPathConfig(ctx, pathID)
		if err != nil {
			return nil, nil, mapHistoryReplayRepositoryError(err)
		}
	}
	// 路径配置记录是工作台当前来源的最高优先级；只有没有明确来源时才读取路径来源覆盖记录。
	if configFound && strings.TrimSpace(configSource.SourceMode) != "" {
		mode = configSource.SourceMode
	} else {
		var err error
		pathSource, pathSourceFound, err = s.store.GetPathSource(ctx, pathID)
		if err != nil {
			return nil, nil, mapHistoryReplayRepositoryError(err)
		}
		if pathSourceFound {
			mode = pathSource.Mode
		}
	}
	// 新路径配置行的初始 none 不是用户的路径覆盖选择；没有独立来源记录时与工作台一致地继承计划默认。
	if mode == model.HistorySourceModeNone && !pathSourceFound {
		mode = model.HistorySourceModeDefault
	}
	switch mode {
	case model.HistorySourceModeNone:
		return nil, historyReplayIssue("HISTORY_SOURCE_MISSING", "路径尚未选择基础表单数据", true), nil
	case model.HistorySourceModeOverride:
		snapshotID := pathSource.SnapshotID
		if configFound && configSource.SnapshotID != nil {
			snapshotID = *configSource.SnapshotID
		}
		if snapshotID == 0 {
			return nil, historyReplayIssue("HISTORY_SNAPSHOT_MISSING", "路径独立基础表单数据不存在", true), nil
		}
		if _, err := s.store.GetSnapshot(ctx, planID, snapshotID); err != nil {
			if errors.Is(err, repository.ErrHistorySnapshotNotFound) {
				return nil, historyReplayIssue("HISTORY_SNAPSHOT_MISSING", "路径基础表单数据不存在", true), nil
			}
			return nil, nil, mapHistoryReplayRepositoryError(err)
		}
		value := snapshotID
		return &value, nil, nil
	case model.HistorySourceModeDefault:
		defaultRecord, defaultFound, defaultErr := s.store.GetDefault(ctx, planID)
		if defaultErr != nil {
			return nil, nil, mapHistoryReplayRepositoryError(defaultErr)
		}
		if !defaultFound || defaultRecord.SnapshotID == 0 {
			return nil, historyReplayIssue("HISTORY_DEFAULT_MISSING", "计划默认基础表单数据尚未设置", true), nil
		}
		if _, err := s.store.GetSnapshot(ctx, planID, defaultRecord.SnapshotID); err != nil {
			if errors.Is(err, repository.ErrHistorySnapshotNotFound) {
				return nil, historyReplayIssue("HISTORY_SNAPSHOT_MISSING", "计划默认基础表单数据不存在", true), nil
			}
			return nil, nil, mapHistoryReplayRepositoryError(err)
		}
		value := defaultRecord.SnapshotID
		return &value, nil, nil
	default:
		return nil, historyReplayIssue("HISTORY_SOURCE_INVALID", "路径基础表单数据来源模式不正确", true), nil
	}
}

// startWorker 为每个任务只启动一个进程内 worker，持久化租约仍是跨进程的最终约束。
// requestContext 只用来取日志作用域和补计划名，worker 的生命周期仍与请求解绑，
// 否则请求一结束后台任务就会被取消。
func (s *HistoryReplayService) startWorker(requestContext context.Context, job model.HistoryReplayJob) {
	if s == nil || strings.TrimSpace(job.ID) == "" {
		return
	}
	// 作用域解析在计划名缺失时会查库，必须放在取锁之前：s.mu 还护着取消、恢复与 worker 状态，
	// 数据库慢的时候不能把这些一起堵住。任务是否已经在跑仍由锁内的判断最终裁决。
	scope := backgroundLogScope(requestContext, s.plans, job.PlanID)
	s.mu.Lock()
	if _, exists := s.running[job.ID]; exists {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithTimeout(logging.WithScope(context.Background(), scope), historyReplayWorkerTimeout)
	done := make(chan struct{})
	s.running[job.ID] = struct{}{}
	s.cancel[job.ID] = cancel
	s.done[job.ID] = done
	s.mu.Unlock()
	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.running, job.ID)
			delete(s.cancel, job.ID)
			close(done)
			delete(s.done, job.ID)
			s.mu.Unlock()
		}()
		defer cancel()
		s.runWorker(ctx, job.ID, job.PlanID)
	}()
}

// stopWorker 取消进程内 worker 上下文，数据库状态仍由仓储事务负责最终裁决。
func (s *HistoryReplayService) stopWorker(jobID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	cancel := s.cancel[jobID]
	done := s.done[jobID]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(historyReplayWorkerStopWait):
		}
	}
}

// runWorker 逐批领取明细；取消只阻止后续领取，已完成检查点不回滚。
func (s *HistoryReplayService) runWorker(ctx context.Context, jobID string, planID uint64) {
	for {
		if ctx.Err() != nil {
			return
		}
		items, err := s.store.ClaimReplayItems(ctx, jobID, historyReplayWorkerBatchSize, s.worker, s.now().UTC())
		if err != nil {
			if !errors.Is(err, repository.ErrHistoryReplayState) {
				_, _ = s.store.UpdateReplayStatus(context.Background(), planID, jobID, model.HistoryReplayStatusFailed, s.now().UTC())
			}
			return
		}
		if len(items) == 0 {
			_, _ = s.store.RecountReplay(context.Background(), jobID, s.now().UTC())
			return
		}
		for _, item := range items {
			if ctx.Err() != nil {
				return
			}
			result := s.replayItem(ctx, planID, item)
			result.LeaseOwner, result.FencingToken = s.worker, item.FencingToken
			if err := s.store.CompleteReplayItem(context.Background(), jobID, item.ID, result, s.now().UTC()); err != nil {
				if errors.Is(err, repository.ErrHistoryReplayState) {
					return
				}
				_, _ = s.store.UpdateReplayStatus(context.Background(), planID, jobID, model.HistoryReplayStatusFailed, s.now().UTC())
				return
			}
		}
	}
}

// replayItem 在单路径边界内读取快照、复验条件和运行时，任何证据缺口都保留原始数据并转 needs_input。
func (s *HistoryReplayService) replayItem(ctx context.Context, planID uint64, item model.HistoryReplayItem) model.HistoryReplayItem {
	result := model.HistoryReplayItem{
		ID: item.ID, JobID: item.JobID, PathID: item.PathID, PathRevision: item.PathRevision, SnapshotID: item.SnapshotID,
		Status: model.HistoryReplayItemStatusNeedsInput, DataStatus: model.HistoryDataStatusNeedsInput,
		Issues: append([]model.HistoryDataIssue(nil), item.Issues...), BranchPatches: []model.HistoryBranchPatch{}, EffectiveFormData: map[string]any{},
		RuntimeType: item.RuntimeType,
	}
	path, err := s.paths.Get(ctx, planID, item.PathID)
	if err == nil {
		// 这条明细之后的目标读取与动作配置都属于这条执行路径，补上路径归属，
		// 让本项目产生的日志落进该执行路径自己的目录，而不是停在计划级目录。
		ctx = logging.WithScope(ctx, logging.Scope{
			ExecutionPathID: strconv.FormatUint(item.PathID, 10), ExecutionPathName: path.Name,
		})
	}
	if err != nil {
		if errors.Is(err, repository.ErrExecutionPathNotFound) {
			result.Status, result.DataStatus = model.HistoryReplayItemStatusAffected, model.HistoryDataStatusAffected
			result.Issues = append(result.Issues, model.HistoryDataIssue{Code: "HISTORY_PATH_DELETED", Message: "执行路径已删除，当前回放检查点失效", Blocking: true})
			return result
		}
		result.Status = model.HistoryReplayItemStatusFailed
		result.Issues = append(result.Issues, model.HistoryDataIssue{Code: "HISTORY_PATH_READ_FAILED", Message: "执行路径暂时无法读取", Blocking: true})
		return result
	}
	// 节点动作与业务数据相互独立：只要路径还在，就先按真实门禁补齐节点动作，
	// 不因为业务数据没准备好而让节点停在待配置。
	s.mu.Lock()
	configurator := s.actions
	s.mu.Unlock()
	if configurator != nil {
		if err := configurator.AutoConfigurePathActions(ctx, planID, item.PathID); err != nil {
			result.Issues = append(result.Issues, model.HistoryDataIssue{Code: "AUTO_ACTION_CONFIGURE_FAILED", Message: "节点动作未能自动配置完成，请打开路径手工确认", Blocking: false})
		}
	}
	if path.ConfigurationRevision != item.PathRevision {
		result.Status, result.DataStatus = model.HistoryReplayItemStatusAffected, model.HistoryDataStatusAffected
		result.Issues = append(result.Issues, model.HistoryDataIssue{Code: "HISTORY_PATH_REVISION_CHANGED", Message: "执行路径修订已变化，需要重新回放", Blocking: true})
		return result
	}
	if item.SnapshotID == nil || *item.SnapshotID == 0 {
		result.Issues = append(result.Issues, model.HistoryDataIssue{Code: "HISTORY_SOURCE_MISSING", Message: "基础表单数据不存在，不能退回空数据", Blocking: true})
		return result
	}
	snapshot, err := s.store.GetSnapshot(ctx, planID, *item.SnapshotID)
	if err != nil {
		result.Issues = append(result.Issues, model.HistoryDataIssue{Code: "HISTORY_SNAPSHOT_READ_FAILED", Message: "基础表单数据暂时无法读取", Blocking: true})
		return result
	}
	if s.target == nil {
		result.EffectiveFormData = clearAuditInfoValues(cloneWorkspaceMap(snapshot.RawFormData))
		result.Issues = append(result.Issues, model.HistoryDataIssue{Code: "HISTORY_TARGET_UNAVAILABLE", Message: "目标流程结构暂时无法读取，不能复验当前路径", Blocking: true})
		return result
	}
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		result.Issues = append(result.Issues, model.HistoryDataIssue{Code: "HISTORY_PLAN_READ_FAILED", Message: "计划身份暂时无法读取", Blocking: true})
		return result
	}
	current, err := s.target.PathConfigurationSnapshot(ctx, plan.Account, plan.FlowSource, plan.TargetObjectID)
	if err != nil {
		result.EffectiveFormData = clearAuditInfoValues(cloneWorkspaceMap(snapshot.RawFormData))
		result.Issues = append(result.Issues, model.HistoryDataIssue{Code: "HISTORY_TARGET_READ_FAILED", Message: "目标流程结构暂时无法读取", Blocking: true})
		return result
	}
	if snapshot.RuntimeType != string(current.RenderType) {
		result.EffectiveFormData = clearAuditInfoValues(cloneWorkspaceMap(snapshot.RawFormData))
		result.Status, result.DataStatus = model.HistoryReplayItemStatusAffected, model.HistoryDataStatusAffected
		result.Issues = append(result.Issues, model.HistoryDataIssue{Code: "HISTORY_RUNTIME_CHANGED", Message: "目标表单运行时类型已变化，需要重新核对", Blocking: true})
		return result
	}
	result.RuntimeType = string(current.RenderType)
	// 配置阶段是发起态：历史实例上的审批意见不带入本次业务数据。
	overlay := branchoverlay.Apply(branchoverlay.Input{Tree: current.Tree, Choices: path.Choices, Values: clearAuditInfoValues(cloneWorkspaceMap(snapshot.RawFormData))})
	result.EffectiveFormData = overlay.Values
	result.BranchPatches = overlay.Patches
	result.Issues = append(result.Issues, historyReplayOverlayIssues(overlay.Issues)...)
	if overlay.Status != branchoverlay.StatusReady {
		return result
	}
	s.mu.Lock()
	runtime := s.runtime
	s.mu.Unlock()
	if runtime == nil {
		// 批量任务里没有浏览器，复制的 form-runtime 无法在后台执行校验。
		// 目标条件复验已经通过，因此数据按已准备落盘，真正的运行时校验在用户打开表单数据页时完成。
		result.Status, result.DataStatus = model.HistoryReplayItemStatusReady, model.HistoryDataStatusReady
		result.Issues = append(result.Issues, model.HistoryDataIssue{Code: "HISTORY_RUNTIME_VALIDATION_DEFERRED", Message: "表单校验会在打开表单数据页时完成", Blocking: false})
		return result
	}
	validation, validateErr := runtime.Validate(ctx, current.RenderType, overlay.Values)
	if validateErr != nil {
		result.RuntimeValidation = model.HistoryRuntimeValidation{Accepted: false}
		result.Issues = append(result.Issues, model.HistoryDataIssue{Code: "HISTORY_RUNTIME_VALIDATION_FAILED", Message: "复制的 form-runtime 校验失败", Blocking: true})
		return result
	}
	result.RuntimeValidation = validation
	result.Issues = append(result.Issues, validation.Issues...)
	if !validation.Accepted {
		return result
	}
	result.Status, result.DataStatus = model.HistoryReplayItemStatusReady, model.HistoryDataStatusReady
	return result
}

// normalizeReplayPathIDs 校验明确勾选路径集合，拒绝重复和无界请求。
func normalizeReplayPathIDs(pathIDs []uint64) ([]uint64, error) {
	if len(pathIDs) == 0 || len(pathIDs) > 500 {
		return nil, historyReplayInvalid("请选择需要回放的执行路径")
	}
	seen := make(map[uint64]struct{}, len(pathIDs))
	result := make([]uint64, 0, len(pathIDs))
	for _, pathID := range pathIDs {
		if pathID == 0 {
			return nil, historyReplayInvalid("执行路径 ID 不正确")
		}
		if _, exists := seen[pathID]; exists {
			return nil, historyReplayInvalid("执行路径不能重复选择")
		}
		seen[pathID] = struct{}{}
		result = append(result, pathID)
	}
	return result, nil
}

// historyReplayOverlayIssues 把分支模块问题投影为稳定中文领域问题，不透传目标响应原文。
func historyReplayOverlayIssues(issues []branchoverlay.Issue) []model.HistoryDataIssue {
	result := make([]model.HistoryDataIssue, 0, len(issues))
	for _, issue := range issues {
		result = append(result, model.HistoryDataIssue{Code: "HISTORY_BRANCH_" + strings.ToUpper(strings.TrimSpace(issue.Code)), Path: issue.Path, Fields: append([]string(nil), issue.Fields...), Message: issue.Message, Blocking: true})
	}
	return result
}

// publicHistoryReplayJob 清理租约、幂等和 fencing 字段，防止公开响应泄露后台协调信息。
func publicHistoryReplayJob(job model.HistoryReplayJob) model.HistoryReplayJob {
	job.IdempotencyKey, job.LeaseOwner, job.LeaseExpiresAt, job.FencingToken = "", "", nil, 0
	return job
}

// historyReplayIssue 构造单条结构化来源问题。
func historyReplayIssue(code, message string, blocking bool) *model.HistoryDataIssue {
	return &model.HistoryDataIssue{Code: code, Message: message, Blocking: blocking}
}

// historyReplayInvalid 构造参数错误。
func historyReplayInvalid(message string) error {
	return &HistoryReplayError{Kind: HistoryReplayErrorInvalidArgument, Message: message}
}

// historyReplayStorageError 构造存储不可用错误。
func historyReplayStorageError() error {
	return &HistoryReplayError{Kind: HistoryReplayErrorStorage, Message: "批量准备存储暂不可用"}
}

// mapHistoryReplayRepositoryError 映射仓储错误并隐藏 SQL 和目标内部细节。
func mapHistoryReplayRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrHistoryReplayActive), errors.Is(err, repository.ErrHistoryReplayIdempotency):
		return &HistoryReplayError{Kind: HistoryReplayErrorConflict, Message: "当前计划已有批量准备任务或幂等键已被复用"}
	case errors.Is(err, repository.ErrHistoryReplayNotFound):
		return &HistoryReplayError{Kind: HistoryReplayErrorNotFound, Message: "批量准备任务不存在"}
	case errors.Is(err, repository.ErrHistoryReplayState):
		return &HistoryReplayError{Kind: HistoryReplayErrorState, Message: "批量准备任务状态不允许当前操作"}
	case errors.Is(err, repository.ErrHistorySnapshotNotFound), errors.Is(err, repository.ErrExecutionPathNotFound), errors.Is(err, repository.ErrPlanNotFound):
		return &HistoryReplayError{Kind: HistoryReplayErrorNotFound, Message: "批量准备来源或执行路径不存在"}
	default:
		return &HistoryReplayError{Kind: HistoryReplayErrorStorage, Message: "批量准备存储暂不可用"}
	}
}
