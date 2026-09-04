// Package control 实现本切片的单步运行控制（纲领第 4.3、4.5 节）：
// 启动停在第一步之前、每步放行、停止是终态。
// 边界：不实现断点与自动/人工模式（F-017），不提供批量放行（放行只作用于当前打开的这条路径运行），
// 不改写任何已发生事实。
package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"test-auto-pro-v2/internal/engine/run"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// 控制层的稳定错误：调用方映射为中文响应，不透出内部细节。
var (
	// ErrNoActiveStep 表示当前没有等待放行的步骤（未启动、已走完或已停止）。
	ErrNoActiveStep = errors.New("当前没有等待放行的步骤")
	// ErrStopDeferred 表示本步已进入提交阶段，停止将在本步结束后生效。
	ErrStopDeferred = errors.New("写请求执行中，停止将在本步结束后生效；请在本步结束并重新停下后再停止")
	// ErrRunAlreadyFinished 表示路径运行已进入终态，不能再放行或停止。
	ErrRunAlreadyFinished = errors.New("路径运行已结束")
)

// StartResult 是启动后的初始状态：运行、路径运行与第一步预览。
type StartResult struct {
	Run          model.Run
	PathRun      model.PathRun
	Preview      *step.StepPreview
	FinalFacts   *step.FinalTargetFacts
	PathFinished bool
}

// ApproveResult 是一次放行后的结果：要么停在下一步之前，要么场景走完并给出收尾重读。
type ApproveResult struct {
	Outcome      step.StepOutcome
	NextPreview  *step.StepPreview
	PathFinished bool
	FinalFacts   *step.FinalTargetFacts
}

// activeStep 是一条路径运行当前停在阶段 3 的步骤现场（单进程内存态）。
// 进程重启后恢复逻辑会把运行中/核验中的路径运行置为待对账，绝不基于残缺内存态自动继续，
// 因此内存现场丢失与崩溃恢复语义一致。
type activeStep struct {
	runCtx    step.RunContext
	preview   *step.StepPreview
	nextIndex int
}

// Service 是单步控制入口：启动、放行、停止。
type Service struct {
	runs  *run.Service
	steps *step.Executor
	store repository.RunStore
	now   func() time.Time

	mu     sync.Mutex
	active map[uint64]*activeStep
}

// NewService 创建单步控制服务。
func NewService(runs *run.Service, steps *step.Executor, store repository.RunStore, now func() time.Time) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{runs: runs, steps: steps, store: store, now: now, active: map[uint64]*activeStep{}}
}

// Start 启动一次单步运行：创建运行与路径运行，产出第一步预览并停在放行之前。
// 场景为空时按运行前检查的结论本不应到达这里，仍如实置为失败而不是静默成功。
func (s *Service) Start(ctx context.Context, runCtx step.RunContext) (*StartResult, error) {
	startedRun, startedPathRun, err := s.runs.StartRun(ctx, runCtx.Run.PlanID, runCtx.PathRun.ExecutionPathID)
	if err != nil {
		return nil, err
	}
	runCtx.Run = startedRun
	runCtx.PathRun = startedPathRun

	preview, finished, err := s.steps.BuildPreview(ctx, runCtx, 0)
	if err != nil {
		return nil, err
	}
	result := &StartResult{Run: startedRun, PathRun: startedPathRun, Preview: preview}
	if finished {
		// 编译场景为空：直接收尾为失败，留事件与摘要，不做无事实的成功宣称。
		class := model.FailureClassToolBug
		if _, err := s.runs.Finish(ctx, startedPathRun.ID, model.PathRunStatusFailed, runResultOf(model.RunResultFailed), &class,
			"编译场景为空，无法执行"); err != nil {
			return nil, err
		}
		result.PathFinished = true
		return result, nil
	}
	s.mu.Lock()
	s.active[startedPathRun.ID] = &activeStep{runCtx: runCtx, preview: preview, nextIndex: 0}
	s.mu.Unlock()
	return result, nil
}

// CurrentPreview 返回当前等待放行的步骤预览；没有现场时返回 ErrNoActiveStep。
func (s *Service) CurrentPreview(pathRunID uint64) *step.StepPreview {
	s.mu.Lock()
	defer s.mu.Unlock()
	active := s.active[pathRunID]
	if active == nil {
		return nil
	}
	return active.preview
}

// Approve 放行当前步：落放行事实、执行阶段 4 到 7、推进游标；
// 场景走完时执行收尾重读并收尾路径运行。放行只作用于这一条路径运行，没有批量入口。
func (s *Service) Approve(ctx context.Context, pathRunID uint64) (*ApproveResult, error) {
	s.mu.Lock()
	active := s.active[pathRunID]
	s.mu.Unlock()
	if active == nil {
		return nil, ErrNoActiveStep
	}
	pathRun, err := s.runs.GetPathRun(ctx, pathRunID)
	if err != nil {
		return nil, err
	}
	if model.IsTerminalPathRunStatus(pathRun.Status) {
		s.clear(pathRunID)
		return nil, ErrRunAlreadyFinished
	}
	if pathRun.Status != model.PathRunStatusRunning {
		return nil, fmt.Errorf("路径运行当前为 %s，不能放行", model.PathRunStatusName(pathRun.Status))
	}
	if err := s.store.AppendRunControl(ctx, model.RunControl{
		RunID: pathRun.RunID, PathRunID: pathRunID,
		Action: model.RunControlApprove, Source: model.RunControlSourceUI, CreatedAt: s.now(),
	}, s.now()); err != nil {
		return nil, err
	}

	outcome, _, err := s.steps.RunApprovedStep(ctx, step.ApprovedStep{
		RunCtx: active.runCtx, Preview: active.preview, NextIndex: active.nextIndex,
	})
	if err != nil {
		return nil, err
	}
	result := &ApproveResult{Outcome: outcome}
	if outcome.Verdict != "confirmed_success" {
		// 确定失败或不确定：路径运行已进入终态，现场作废。
		s.clear(pathRunID)
		return result, nil
	}
	if outcome.MainInstanceRef != "" {
		active.runCtx.PathRun.MainInstanceRef = outcome.MainInstanceRef
	}
	nextIndex := active.nextIndex + 1
	if !outcome.NoMoreSteps {
		preview, finished, err := s.steps.BuildPreview(ctx, active.runCtx, nextIndex)
		if err != nil {
			return result, err
		}
		if !finished {
			active.preview = preview
			active.nextIndex = nextIndex
			result.NextPreview = preview
			return result, nil
		}
	}
	// 场景走完：收尾重读 + 最终目标事实摘要落库 + 路径运行收尾为已完成。
	facts, reviewErr := s.steps.FinalReview(ctx, active.runCtx)
	if reviewErr != nil {
		return result, reviewErr
	}
	summary, err := json.Marshal(facts)
	if err != nil {
		return result, err
	}
	if err := s.store.SetFinalTargetSummary(ctx, pathRunID, string(summary), s.now()); err != nil {
		return result, err
	}
	if _, err := s.runs.Finish(ctx, pathRunID, model.PathRunStatusCompleted, runResultOf(model.RunResultSucceeded), nil,
		"编译场景全部步骤确定成功，路径运行完成"); err != nil {
		return result, err
	}
	s.clear(pathRunID)
	result.PathFinished = true
	result.FinalFacts = &facts
	return result, nil
}

// Stop 处理用户停止：落停止事实并把路径运行置为已停止，已发生的事实全部保留。
// 本步已进入提交/核验阶段时停止延迟生效（返回 ErrStopDeferred），绝不打断已发出的写请求。
func (s *Service) Stop(ctx context.Context, pathRunID uint64) (model.PathRun, error) {
	pathRun, err := s.runs.GetPathRun(ctx, pathRunID)
	if err != nil {
		return model.PathRun{}, err
	}
	if model.IsTerminalPathRunStatus(pathRun.Status) {
		s.clear(pathRunID)
		return pathRun, ErrRunAlreadyFinished
	}
	if pathRun.Status == model.PathRunStatusVerifying {
		return pathRun, ErrStopDeferred
	}
	if pathRun.Status != model.PathRunStatusRunning {
		return pathRun, fmt.Errorf("路径运行当前为 %s，不能停止", model.PathRunStatusName(pathRun.Status))
	}
	if err := s.store.AppendRunControl(ctx, model.RunControl{
		RunID: pathRun.RunID, PathRunID: pathRunID,
		Action: model.RunControlStop, Source: model.RunControlSourceUI, CreatedAt: s.now(),
	}, s.now()); err != nil {
		return model.PathRun{}, err
	}
	stopped, err := s.runs.Stop(ctx, pathRunID)
	if err != nil {
		return model.PathRun{}, err
	}
	s.clear(pathRunID)
	return stopped, nil
}

// clear 作废路径运行的内存现场。
func (s *Service) clear(pathRunID uint64) {
	s.mu.Lock()
	delete(s.active, pathRunID)
	s.mu.Unlock()
}

// runResultOf 返回路径结果的指针形态。
func runResultOf(result model.RunResult) *model.RunResult {
	return &result
}
