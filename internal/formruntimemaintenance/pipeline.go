package formruntimemaintenance

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"
)

// WorkerOptions 控制 Worker 身份和租约边界。
type WorkerOptions struct {
	WorkerID        string
	LeaseDuration   time.Duration
	RenewalInterval time.Duration
}

// Pipeline 实现旧 V2 已验证的七阶段状态机，并把部署对象抽象为 pnpm 版本目录。
type Pipeline struct {
	store         Store
	inspector     SourceInspector
	operator      RuntimeOperator
	logs          LogStore
	workerID      string
	leaseDuration time.Duration
	renewInterval time.Duration
}

// NewPipeline 创建可恢复维护流水线。
func NewPipeline(store Store, inspector SourceInspector, operator RuntimeOperator, logs LogStore, options WorkerOptions) *Pipeline {
	workerID := strings.TrimSpace(options.WorkerID)
	if workerID == "" {
		workerID = "form-runtime-maintenance"
	}
	if options.LeaseDuration <= 0 {
		options.LeaseDuration = 2 * time.Minute
	}
	if options.RenewalInterval <= 0 {
		options.RenewalInterval = options.LeaseDuration / 3
	}
	return &Pipeline{store: store, inspector: inspector, operator: operator, logs: logs, workerID: workerID,
		leaseDuration: options.LeaseDuration, renewInterval: options.RenewalInterval}
}

// ProcessNext 领取一个任务，并从持久阶段执行或恢复。
func (p *Pipeline) ProcessNext(ctx context.Context) (bool, error) {
	job, err := p.store.ClaimNext(ctx, Claim{WorkerID: p.workerID, LeaseDuration: p.leaseDuration})
	if errors.Is(err, ErrJobNotReady) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	output, err := p.logs.Open(ctx, job.ID)
	if err != nil {
		return true, p.fail(ctx, job, job.Stage, fmt.Errorf("打开同步日志: %w", err), nil)
	}
	defer output.Close()
	// RESTART/VERIFY 前已持久化候选与 previous，Worker 接管时不能重新同步或重建另一个候选。
	if job.Stage == StageRestart || job.Stage == StageVerify {
		return p.resumeDeployment(ctx, job, output)
	}
	writeStage(output, StageInspect, "复核固定来源快照")
	var current SourceState
	if err := p.withLease(ctx, job, func(work context.Context) error {
		var inspectErr error
		current, inspectErr = p.inspector.Inspect(work)
		return inspectErr
	}); err != nil {
		return true, p.fail(ctx, job, StageInspect, err, output)
	}
	if current.Dirty || !sameSource(job.Source, current) {
		return true, p.fail(ctx, job, StageInspect, errors.New("同步来源在任务创建后发生变化，请重新创建任务"), output)
	}
	if err := p.runStage(ctx, job, StageSync, output, p.operator.Sync); err != nil {
		return true, err
	}
	if err := p.runStage(ctx, job, StageCheck, output, p.operator.SyncCheck); err != nil {
		return true, err
	}
	if err := p.progress(ctx, job, StageBuild, "", ""); err != nil {
		return true, p.fail(ctx, job, StageBuild, err, output)
	}
	writeStage(output, StageBuild, "使用 pnpm 构建隔离候选")
	var candidate string
	if err := p.withLease(ctx, job, func(work context.Context) error {
		var buildErr error
		candidate, buildErr = p.operator.BuildCandidate(work, job.ID, output)
		return buildErr
	}); err != nil {
		return true, p.fail(ctx, job, StageBuild, err, output)
	}
	previous := job.Previous
	if previous == "" {
		if err := p.withLease(ctx, job, func(work context.Context) error {
			var currentErr error
			previous, currentErr = p.operator.CurrentVersion(work)
			return currentErr
		}); err != nil {
			return true, p.fail(ctx, job, StageBuild, err, output)
		}
	}
	if err := p.progress(ctx, job, StageRestart, candidate, previous); err != nil {
		return true, p.fail(ctx, job, StageRestart, err, output)
	}
	writeStage(output, StageRestart, "原子切换候选版本")
	if err := p.withLease(ctx, job, func(work context.Context) error {
		return p.operator.Restart(work, candidate, previous, output)
	}); err != nil {
		return true, p.fail(ctx, job, StageRestart, err, output)
	}
	if err := p.progress(ctx, job, StageVerify, candidate, previous); err != nil {
		return true, p.fail(ctx, job, StageVerify, err, output)
	}
	return p.verifyAndComplete(ctx, job, candidate, previous, output)
}

// resumeDeployment 从 RESTART/VERIFY 阶段恢复，避免重复同步和构建覆盖已运行候选。
func (p *Pipeline) resumeDeployment(ctx context.Context, job Job, output io.Writer) (bool, error) {
	if job.Candidate == "" || job.Previous == "" {
		return true, p.fail(ctx, job, job.Stage, errors.New("恢复任务缺少候选或 previous 版本"), output)
	}
	if job.Stage == StageRestart {
		var current string
		err := p.withLease(ctx, job, func(work context.Context) error {
			var currentErr error
			current, currentErr = p.operator.CurrentVersion(work)
			return currentErr
		})
		if err != nil || current == job.Previous {
			writeStage(output, StageRestart, "恢复候选版本切换")
			if restartErr := p.withLease(ctx, job, func(work context.Context) error {
				return p.operator.Restart(work, job.Candidate, job.Previous, output)
			}); restartErr != nil {
				return true, p.fail(ctx, job, StageRestart, restartErr, output)
			}
		} else if current != job.Candidate {
			return true, p.fail(ctx, job, StageRestart, fmt.Errorf("当前版本 %s 不属于恢复任务", current), output)
		}
		if progressErr := p.progress(ctx, job, StageVerify, job.Candidate, job.Previous); progressErr != nil {
			return true, p.fail(ctx, job, StageVerify, progressErr, output)
		}
	}
	return p.verifyAndComplete(ctx, job, job.Candidate, job.Previous, output)
}

// verifyAndComplete 完成最终健康核验，并只在核验成功后标记 COMPLETED。
func (p *Pipeline) verifyAndComplete(ctx context.Context, job Job, candidate, previous string, output io.Writer) (bool, error) {
	writeStage(output, StageVerify, "核验切换后的运行时")
	if err := p.withLease(ctx, job, func(work context.Context) error {
		return p.operator.Verify(work, candidate, previous, output)
	}); err != nil {
		return true, p.fail(ctx, job, StageVerify, err, output)
	}
	writeStage(output, StageCompleted, "同步、切换和健康核验完成")
	if err := p.store.Complete(ctx, Completion{ID: job.ID, WorkerID: p.workerID, FencingToken: job.FencingToken}); err != nil {
		return true, err
	}
	return true, nil
}

// runStage 先持久化阶段再执行，保证进程退出时不会伪称上一阶段仍未开始。
func (p *Pipeline) runStage(ctx context.Context, job Job, stage Stage, output io.Writer, run func(context.Context, io.Writer) error) error {
	if err := p.progress(ctx, job, stage, "", ""); err != nil {
		return p.fail(ctx, job, stage, err, output)
	}
	writeStage(output, stage, "开始")
	if err := p.withLease(ctx, job, func(work context.Context) error { return run(work, output) }); err != nil {
		return p.fail(ctx, job, stage, err, output)
	}
	return nil
}

// withLease 在长操作中续租；任何 fencing/续租失败都会取消当前命令。
func (p *Pipeline) withLease(ctx context.Context, job Job, run func(context.Context) error) error {
	work, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	renewalError := make(chan error, 1)
	go func() {
		defer close(done)
		ticker := time.NewTicker(p.renewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-work.Done():
				return
			case <-ticker.C:
				if err := p.store.RenewLease(work, LeaseRenewal{ID: job.ID, WorkerID: p.workerID, FencingToken: job.FencingToken, LeaseDuration: p.leaseDuration}); err != nil {
					renewalError <- err
					cancel()
					return
				}
			}
		}
	}()
	runErr := run(work)
	cancel()
	<-done
	select {
	case err := <-renewalError:
		return fmt.Errorf("续租表单运行时维护任务: %w", err)
	default:
		return runErr
	}
}

// progress 持久化阶段和部署版本，并顺带延长当前租约。
func (p *Pipeline) progress(ctx context.Context, job Job, stage Stage, candidate, previous string) error {
	return p.store.UpdateProgress(ctx, Progress{ID: job.ID, WorkerID: p.workerID, FencingToken: job.FencingToken,
		LeaseDuration: p.leaseDuration, Stage: stage, Candidate: candidate, Previous: previous})
}

// fail 持久化失败及回退结果，发生在切换前时明确当前可用版本未受影响。
func (p *Pipeline) fail(ctx context.Context, job Job, stage Stage, cause error, output io.Writer) error {
	if output != nil {
		_, _ = fmt.Fprintf(output, "[FAILED] %s\n", cause)
	}
	recovery, message := recoveryOutcome(stage, cause)
	if err := p.store.Complete(ctx, Completion{ID: job.ID, WorkerID: p.workerID, FencingToken: job.FencingToken,
		FailureReason: cause.Error(), RecoveryStatus: recovery, RecoveryMessage: message}); err != nil {
		return fmt.Errorf("%v；持久化维护失败结果: %w", cause, err)
	}
	return cause
}

// recoveryOutcome 统一解释候选切换边界，避免 UI 从错误文字猜测是否已回退。
func recoveryOutcome(stage Stage, cause error) (RecoveryStatus, string) {
	var recovery *RecoveryError
	if errors.As(cause, &recovery) {
		return recovery.Status, recovery.Message
	}
	if stage == StageRestart || stage == StageVerify {
		return RecoveryUnknown, "自动恢复结果未能确认，请保留现场并查看日志。"
	}
	return RecoveryNotRequired, "失败发生在切换之前，当前可用表单运行时未受影响。"
}

// writeStage 以稳定阶段前缀写入在线日志。
func writeStage(output io.Writer, stage Stage, message string) {
	_, _ = fmt.Fprintf(output, "[%s] %s\n", stage, message)
}

// sameSource 忽略检查时间，只比较任务创建后不能变化的来源事实。
func sameSource(expected, actual SourceState) bool {
	return expected.Repository == actual.Repository && expected.Branch == actual.Branch && expected.Head == actual.Head &&
		expected.Dirty == actual.Dirty && reflect.DeepEqual(expected.ChangedFiles, actual.ChangedFiles)
}
