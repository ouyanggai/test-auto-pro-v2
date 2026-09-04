package run

import (
	"context"
	"errors"
	"fmt"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// Service 承载运行与路径运行的状态机推进、租约与启动恢复（纲领第 4.1、4.2 节）。
// 边界：本包不直接发目标请求（目标写只由 internal/adapter/target 发出），
// 也不做写结果判定（判定只由 internal/engine/verdict 完成）；
// 这里只负责“状态怎么变、由谁推进、每次变更留下什么痕迹”。
type Service struct {
	store         repository.RunStore
	now           func() time.Time
	workerID      string
	leaseDuration time.Duration
}

// NewService 创建运行状态机服务。workerID 是本进程的执行者标识，
// leaseDuration 是推进租约时长，二者共同保证同一路径运行同时只有一个 Worker 推进。
func NewService(store repository.RunStore, workerID string, leaseDuration time.Duration, now func() time.Time) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{store: store, now: now, workerID: workerID, leaseDuration: leaseDuration}
}

// StartRun 创建一次单步运行与它唯一的路径运行，并把两者推进到运行中。
// 本切片只交付单步与手动启动：模式与触发方式是固定值，不是可选项（避免出现“看起来能选自动”的假象）；
// 一次运行只跑一条路径，最大并发固定为 1。
func (s *Service) StartRun(ctx context.Context, planID uint64, executionPathID uint64) (model.Run, model.PathRun, error) {
	singleConcurrency := 1
	createdRun, createdPathRun, err := s.store.CreateRun(ctx, planID, executionPathID,
		model.RunModeSingleStep, model.RunTriggerManual, &singleConcurrency, s.now())
	if err != nil {
		return model.Run{}, model.PathRun{}, err
	}
	startedRun, err := s.store.AdvanceRunStatus(ctx, createdRun.ID,
		model.RunStatusPending, model.RunStatusRunning, model.RunEvent{
			Kind:  "run_started",
			Label: fmt.Sprintf("运行开始（%s，手动启动）", model.RunModeName(model.RunModeSingleStep)),
		}, s.now())
	if err != nil {
		return startedRun, createdPathRun, err
	}
	startedPathRun, err := s.store.AdvancePathRunStatus(ctx, createdPathRun.ID,
		model.PathRunStatusWaiting, model.PathRunStatusRunning, model.RunEvent{
			Kind:  "path_run_started",
			Label: "路径运行开始，准备第一步预览",
		}, s.now())
	if err != nil {
		return startedRun, startedPathRun, err
	}
	return startedRun, startedPathRun, nil
}

// ClaimExecution 在一步真正开始执行（prepare 起）前领取推进权，返回 fencing token。
// 领取失败说明已有其他执行者持有有效租约或路径运行已到终态，调用方必须放弃推进。
func (s *Service) ClaimExecution(ctx context.Context, pathRunID uint64) (uint64, error) {
	token, err := s.store.ClaimPathRunLease(ctx, pathRunID, s.workerID, s.leaseDuration, s.now())
	if err != nil {
		return 0, err
	}
	return token, nil
}

// RenewLease 在一步执行期间续租，防止长慢请求（目标存在稳定约 30 秒的慢请求）期间租约过期被接管。
func (s *Service) RenewLease(ctx context.Context, pathRunID uint64, fencingToken uint64) error {
	return s.store.RenewPathRunLease(ctx, pathRunID, s.workerID, fencingToken, s.leaseDuration, s.now())
}

// ReleaseExecution 在一步走完落账后释放推进权；未命中说明租约已被接管，调用方不得再推进该路径运行。
func (s *Service) ReleaseExecution(ctx context.Context, pathRunID uint64, fencingToken uint64) error {
	return s.store.ReleasePathRunLease(ctx, pathRunID, s.workerID, fencingToken, s.now())
}

// MarkVerifying 在发出写请求、进入事实重读前把路径运行置为核验中。
// 这一步先行落库：进程若在此后崩溃，启动恢复会把该路径运行置为待对账，
// 因为写请求可能已在目标生效（纲领第 4.2 节）。
func (s *Service) MarkVerifying(ctx context.Context, pathRunID uint64) error {
	_, err := s.store.AdvancePathRunStatus(ctx, pathRunID,
		model.PathRunStatusRunning, model.PathRunStatusVerifying, model.RunEvent{
			Kind:  "path_run_verifying",
			Label: "写请求已发出，正在回目标重读事实",
		}, s.now())
	return err
}

// BackToRunning 在一步核验落账完毕、进入下一步时把路径运行从核验中带回运行中。
// 这是步骤循环的前进而非状态回退（迁移表允许该唯一一条“向后”通路）。
func (s *Service) BackToRunning(ctx context.Context, pathRunID uint64) error {
	_, err := s.store.AdvancePathRunStatus(ctx, pathRunID,
		model.PathRunStatusVerifying, model.PathRunStatusRunning, model.RunEvent{
			Kind:  "path_run_step_settled",
			Label: "本步落账完毕，进入下一步",
		}, s.now())
	return err
}

// Finish 把路径运行推进到终态并镜像收尾运行聚合。
// 待对账不镜像运行状态：运行保持运行中，唯一合法恢复动作属于对账切片（F-018）。
func (s *Service) Finish(ctx context.Context, pathRunID uint64, to model.PathRunStatus, result *model.RunResult, failureClass *model.FailureClass, label string) (model.PathRun, error) {
	return s.store.FinishPathRun(ctx, pathRunID, to, result, failureClass, model.RunEvent{
		Kind:  "path_run_finished",
		Label: label,
	}, s.now())
}

// Stop 处理用户停止：运行中或暂停的路径运行置为已停止；已发生的事实全部保留。
// 路径结果留空——停止时场景没有走完，成功/失败/待对账三种结论都不成立，状态本身就是结论。
func (s *Service) Stop(ctx context.Context, pathRunID uint64) (model.PathRun, error) {
	return s.Finish(ctx, pathRunID, model.PathRunStatusStopped, nil, nil, "用户停止运行，已发生的事实全部保留")
}

// Recover 是进程启动时的恢复入口：把处于运行中/核验中的路径运行一律置为待对账并写事件行。
// 这是不可破坏约束——崩溃前可能已经发出过一次写请求，重启后绝不自动继续。
func (s *Service) Recover(ctx context.Context) ([]uint64, error) {
	return s.store.RecoverInterruptedPathRuns(ctx, s.now())
}

// GetRun 读取运行聚合。
func (s *Service) GetRun(ctx context.Context, runID uint64) (model.Run, error) {
	return s.store.GetRun(ctx, runID)
}

// GetPathRun 读取路径运行聚合。
func (s *Service) GetPathRun(ctx context.Context, pathRunID uint64) (model.PathRun, error) {
	return s.store.GetPathRun(ctx, pathRunID)
}

// GetPathRunByRun 读取一次运行下的唯一路径运行。
func (s *Service) GetPathRunByRun(ctx context.Context, runID uint64) (model.PathRun, error) {
	return s.store.GetPathRunByRun(ctx, runID)
}

// ListRunsByPlan 按计划列出运行（最新在前）。
func (s *Service) ListRunsByPlan(ctx context.Context, planID uint64, limit int) ([]model.Run, error) {
	return s.store.ListRunsByPlan(ctx, planID, limit)
}

// StatusConflictErr 提供状态冲突哨兵，供 API 层映射为中文 409 响应。
func StatusConflictErr(err error) bool {
	return errors.Is(err, repository.ErrRunStatusConflict)
}

// LeaseErr 提供租约类哨兵判断，供执行器决定是否放弃推进。
func LeaseErr(err error) bool {
	return errors.Is(err, repository.ErrStaleLease) || errors.Is(err, repository.ErrLeaseHeld)
}
