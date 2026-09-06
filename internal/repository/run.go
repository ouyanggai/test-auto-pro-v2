package repository

import (
	"context"
	"errors"
	"time"

	"test-auto-pro-v2/internal/model"
)

// 运行记录仓储的稳定错误：调用方按错误种类给中文结论，不把 SQL 细节透出。
var (
	// ErrRunNotFound 表示运行或路径运行不存在。
	ErrRunNotFound = errors.New("运行记录不存在")
	// ErrRunInvalidInput 表示启动输入不合法（如未勾选任何路径）。
	ErrRunInvalidInput = errors.New("运行启动输入不合法")
	// ErrRunStatusConflict 表示状态迁移被拒绝：状态只前进不回退，终态不可离开。
	ErrRunStatusConflict = errors.New("运行状态不允许该变更")
	// ErrLeaseHeld 表示同一路径运行已被其他 Worker 持有有效租约。
	ErrLeaseHeld = errors.New("路径运行已被其他执行者占用")
	// ErrStaleLease 表示租约校验未命中：持有者或 fencing token 已过期，不得再推进该路径运行。
	ErrStaleLease = errors.New("路径运行租约已失效")
)

// RunStore 是运行记录（runs、path_runs、run_events）的持久化边界。
// 纪律：事实表只 INSERT；聚合表状态列单向前进，且每次更新与事件行在同一事务内提交。
// CreateRunInput 是一次启动的持久化输入：勾选路径集合在创建时固化（F-020 调度约束）。
type CreateRunInput struct {
	PlanID           uint64
	ExecutionPathIDs []uint64
	Mode             model.RunMode
	Trigger          model.RunTriggerKind
	MaxConcurrency   *int
	// IdempotencyKey 非空时按（计划, 键）唯一约束幂等：同键重试返回已有运行，绝不创建第二份。
	IdempotencyKey string
	// PresetBreakpoints 是预置断点集合的原始 JSON：每条路径运行开始时重放同一预置。
	PresetBreakpoints string
}

type RunStore interface {
	// CreateRun 创建一次运行并在计划内单调递增分配运行号；本切片同时创建唯一的路径运行（等待运行）。
	CreateRun(ctx context.Context, planID uint64, executionPathID uint64, mode model.RunMode, trigger model.RunTriggerKind, maxConcurrency *int, now time.Time) (model.Run, model.PathRun, error)
	// GetRun 读取运行聚合。
	GetRun(ctx context.Context, runID uint64) (model.Run, error)
	// GetPathRun 读取路径运行聚合。
	GetPathRun(ctx context.Context, pathRunID uint64) (model.PathRun, error)
	// GetPathRunByRun 读取一次运行下的路径运行（单路径运行；多路径下作为兼容回退取第一条）。
	GetPathRunByRun(ctx context.Context, runID uint64) (model.PathRun, error)
	// CreateRunWithPaths 创建一次运行并按勾选路径集合批量创建路径运行（F-020）。
	// 幂等键非空且命中已有运行时原样返回那次运行，绝不创建第二份运行或路径实例。
	CreateRunWithPaths(ctx context.Context, input CreateRunInput) (model.Run, []model.PathRun, error)
	// ListPathRunsByRun 按运行列出全部路径运行（按创建顺序）。
	ListPathRunsByRun(ctx context.Context, runID uint64) ([]model.PathRun, error)
	// ListRunIDsNeedingScheduling 列出仍在运行中且带等待路径的运行 ID（调度器输入）。
	ListRunIDsNeedingScheduling(ctx context.Context) ([]uint64, error)
	// FinishRunIfAllPathsClosed 在全部路径闭合时收尾运行聚合（同事务）：写结果无法确认的
	// 终局与启动恢复都会调用它；还有未闭合路径时什么都不做并返回 false。
	FinishRunIfAllPathsClosed(ctx context.Context, runID uint64, now time.Time) (bool, error)
	// ListRunEvents 读取一次运行的事件流（afterID 游标增量，pathRunID 非零时按路径过滤，F-021 只读）。
	ListRunEvents(ctx context.Context, runID uint64, afterID uint64, limit int, pathRunID uint64) ([]model.RunEvent, error)
	// ListRunsFiltered 按状态与游标筛选计划下的运行（F-021 列表）。
	ListRunsFiltered(ctx context.Context, planID uint64, status string, beforeID uint64, limit int) ([]model.Run, error)
	// ListDueScheduledPlans 列出到点尚未消费的计划（数据库时间为准，F-020 定时触发）。
	ListDueScheduledPlans(ctx context.Context, now time.Time) ([]model.Plan, error)
	// ClaimScheduledPlan 原子领取到点计划的一次性消费标记；返回是否领取成功。
	ClaimScheduledPlan(ctx context.Context, planID uint64, now time.Time) (bool, error)
	// ListRunsByPlan 按计划列出运行（运行号倒序），供运行列表使用。
	ListRunsByPlan(ctx context.Context, planID uint64, limit int) ([]model.Run, error)
	// ListRunSteps 按路径运行列出已落账步骤（按步骤序号升序）。
	ListRunSteps(ctx context.Context, pathRunID uint64) ([]model.RunStep, error)
	// ListRunAttempts 按路径运行列出已落账尝试（按步骤序号与尝试序号升序）。
	ListRunAttempts(ctx context.Context, pathRunID uint64) ([]model.RunStepAttempt, error)
	// AdvanceRunStatus 在同一事务内校验并推进运行聚合状态、追加事件行；
	// 状态迁移非法或终态不可离开时返回 ErrRunStatusConflict，绝不落库。
	AdvanceRunStatus(ctx context.Context, runID uint64, from, to model.RunStatus, event model.RunEvent, now time.Time) (model.Run, error)
	// AdvancePathRunStatus 在同一事务内校验并推进路径运行状态、追加事件行；
	// 状态迁移非法或终态不可离开时返回 ErrRunStatusConflict，绝不落库。
	AdvancePathRunStatus(ctx context.Context, pathRunID uint64, from, to model.PathRunStatus, event model.RunEvent, now time.Time) (model.PathRun, error)
	// FinishPathRun 把路径运行置为终态并镜像收尾运行聚合（同事务：路径运行状态+事件+运行状态与结果）。
	FinishPathRun(ctx context.Context, pathRunID uint64, to model.PathRunStatus, result *model.RunResult, failureClass *model.FailureClass, event model.RunEvent, now time.Time) (model.PathRun, error)
	// ClaimPathRunLease 以租约与 fencing token 领取路径运行的推进权：
	// 仅当处于运行中/核验中且无有效租约时成功，fencing token 递增并返回。
	ClaimPathRunLease(ctx context.Context, pathRunID uint64, workerID string, leaseDuration time.Duration, now time.Time) (uint64, error)
	// RenewPathRunLease 校验 owner 与 fencing token 后续租；未命中返回 ErrStaleLease。
	RenewPathRunLease(ctx context.Context, pathRunID uint64, workerID string, fencingToken uint64, leaseDuration time.Duration, now time.Time) error
	// ReleasePathRunLease 在一步走完落账后释放租约；未命中返回 ErrStaleLease。
	ReleasePathRunLease(ctx context.Context, pathRunID uint64, workerID string, fencingToken uint64, now time.Time) error
	// AppendRunControl 追加一行人工控制事实（只 INSERT，可审计）。
	AppendRunControl(ctx context.Context, control model.RunControl, now time.Time) error
	// ListRunControls 按路径运行按时间正序列出控制事实，供断点集合回放与审计。
	ListRunControls(ctx context.Context, pathRunID uint64) ([]model.RunControl, error)
	// AppendRunEvent 追加一行运行事件（如路径偏离、断点命中），与聚合状态变更解耦的独立事实。
	AppendRunEvent(ctx context.Context, event model.RunEvent, now time.Time) error
	// LatestStepAttempt 返回路径运行最近一次落账的步骤与尝试事实。
	LatestStepAttempt(ctx context.Context, pathRunID uint64) (model.RunStep, model.RunStepAttempt, error)
	// SetFinalTargetSummary 落库最终目标事实摘要（收尾重读产物），与路径结果是两个独立字段。
	SetFinalTargetSummary(ctx context.Context, pathRunID uint64, summary string, now time.Time) error
	// SetMainInstanceRef 首次落库路径运行独占的主实例引用。
	// 一条路径运行独占一个真实主实例：引用已存在时拒绝改写，避免把后续写引到别的实例上。
	SetMainInstanceRef(ctx context.Context, pathRunID uint64, instanceRef string, now time.Time) error
	// RecordStepAttempt 把步骤事实与尝试事实在同一事务内 INSERT（事实表只 INSERT，永不改写）。
	// 返回新建步骤行的 ID；两行要么同时存在，要么都不存在，不存在“先占位再补写”的中间态。
	RecordStepAttempt(ctx context.Context, step model.RunStep, attempt model.RunStepAttempt, now time.Time) (uint64, error)
	// RecoverInterruptedPathRuns 把处于运行中/核验中的路径运行一律置为待对账并写事件行。
	// 运行聚合保持原状留给对账切片（F-018）裁决；进程启动时调用，绝不自动继续执行。
	RecoverInterruptedPathRuns(ctx context.Context, now time.Time) ([]uint64, error)
}
