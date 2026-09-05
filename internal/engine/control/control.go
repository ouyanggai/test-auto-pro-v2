// Package control 实现运行控制（纲领第 4.3 阶段 3、4.5、5.1-5.4 节）：
// 启动（模式三选一）、断点增删与命中、暂停、三条放行命令、停止。
// 边界：不实现对账与安全重试（F-018）、导航与恢复步骤（F-019）、多路径调度（F-020）；
// 不提供批量放行；不改写已发生事实。
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
	// ErrLoopRunning 表示连续执行循环存活，不接受新的放行命令（先暂停或等待停止条件）。
	ErrLoopRunning = errors.New("连续执行中；请先暂停（本步走完后生效）或等待停止条件")
	// ErrVersionConflict 表示控制版本不匹配：界面状态已过期，需以返回的当前状态为准。
	ErrVersionConflict = errors.New("控制状态已变化，请以当前运行状态为准后重试")
	// ErrCursorConflict 表示步游标不匹配：当前等待放行的步骤已不是命令所指的那一步。
	ErrCursorConflict = errors.New("当前等待放行的步骤已变化，请以最新预览为准")
	// ErrCommandNotAllowed 表示当前模式或状态下该命令不可用。
	ErrCommandNotAllowed = errors.New("当前状态下该命令不可用")
	// ErrNotRunnable 表示路径运行当前状态不接受该控制动作（如待对账、已结束）。
	ErrNotRunnable = errors.New("路径运行当前状态不接受该操作")
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

// activeStep 是一条路径运行当前停在阶段 3 的控制现场（单进程内存态）。
// 进程重启后恢复逻辑把运行中/核验中置为待对账、暂停保持暂停，绝不基于残缺内存态自动继续。
type activeStep struct {
	runCtx    step.RunContext
	preview   *step.StepPreview
	nextIndex int
	// mode 是本次运行的执行模式（启动时确定，运行中不可切换）。
	mode model.RunMode
	// breakpoints 是当前生效断点集合；由预置与控制事实回放初始化，运行中增删即时生效。
	breakpoints *BreakpointSet
	// pauseRequested/stopRequested 是已落事实、待阶段 3 生效的请求。
	pauseRequested bool
	stopRequested  bool
	// deviationStalled 表示路径偏离断点已强制停止：不产出放行类命令。
	deviationStalled bool
	// loopRunning 表示连续执行循环存活。
	loopRunning bool
	// stopReason 是「为什么停在这里」的中文主因。
	stopReason string
	// version 是控制命令条件写的版本（每次控制状态变化自增）。
	version int64
	// executedStepNos/executedNodeKeys 供断点挂载校验（只能挂未执行对象）。
	executedStepNos  map[int]bool
	executedNodeKeys map[string]bool
	// finished 表示路径运行已到终态，现场可回收。
	finished bool
	// reconcile 是最近一次只读对账的结论（待对账工作区数据源）。
	reconcile *ReconcileResultView
	// replaysUsed 是已执行的重放次数。
	replaysUsed int
	// recoveryLog 把对账过程写进运行目录的 recovery.log。
	recoveryLog *RecoveryLog
	// progress 是当前步的实时阶段进度（执行器上报，指示器轮询的数据源）。
	progress stepPhaseProgress
}

// stepPhaseProgress 是一次尝试内执行器上报的阶段进度快照。
type stepPhaseProgress struct {
	// phase 取七阶段名；note 是给用户看的中文补充（如重试退避说明）。
	phase string
	note  string
	// since 是进入该阶段的时刻，供界面计算阶段内已耗时。
	since time.Time
}

// pauseState 推导当前控制状态分类（供可用命令集合计算）。
func (sess *activeStep) pauseState() PauseState {
	if sess.deviationStalled {
		return PauseStateDeviation
	}
	if sess.finished {
		return PauseStateFinished
	}
	return PauseStateWaiting
}

// Service 是运行控制入口：启动（模式与断点）、放行命令、断点增删、暂停、停止。
type Service struct {
	runs  *run.Service
	steps *step.Executor
	store repository.RunStore
	now   func() time.Time
	// controlLog 把控制事实同步写进运行目录的 control.log；未注入时只落库。
	controlLog *ControlLog
	// recoveryLog 把对账过程写进运行目录的 recovery.log；未注入时只落库。
	recoveryLog *RecoveryLog

	mu     sync.Mutex
	active map[uint64]*activeStep
}

// NewService 创建运行控制服务。
func NewService(runs *run.Service, steps *step.Executor, store repository.RunStore, now func() time.Time) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{runs: runs, steps: steps, store: store, now: now, active: map[uint64]*activeStep{}}
}

// SetControlLog 注入 control.log 写入器；必须在首次控制动作前调用。
func (s *Service) SetControlLog(log *ControlLog) {
	s.controlLog = log
}

// SetRecoveryLog 注入 recovery.log 写入器；必须在首次对账前调用。
func (s *Service) SetRecoveryLog(log *RecoveryLog) {
	s.recoveryLog = log
}

// logFact 把控制事实同步写一行 control.log（日志失败不影响事实落库）。
func (s *Service) logFact(pathRunID uint64, control model.RunControl, stepNo int) {
	s.controlLog.LogFact(pathRunID, control, stepNo)
}

// Start 按默认单步模式启动（F-016 兼容入口，行为与已验收的完全一致）。
func (s *Service) Start(ctx context.Context, runCtx step.RunContext) (*StartResult, error) {
	return s.StartWithMode(ctx, runCtx, model.RunModeSingleStep, nil)
}

// startSession 是模式无关的启动主体：创建运行、初始化断点（预置逐条落事实）、构建第一步预览并停在阶段 3。
func (s *Service) startSession(ctx context.Context, runCtx step.RunContext, mode model.RunMode, preset []Breakpoint) (*StartResult, *activeStep, error) {
	startedRun, startedPathRun, err := s.runs.StartRunWithMode(ctx, runCtx.Run.PlanID, runCtx.PathRun.ExecutionPathID, mode)
	if err != nil {
		return nil, nil, err
	}
	runCtx.Run = startedRun
	runCtx.PathRun = startedPathRun

	// 模式选定事实与中文事件（control.log 由 T06 写入器同步落盘）。
	modeFact := model.RunControl{
		RunID: startedRun.ID, PathRunID: startedPathRun.ID,
		Kind: model.ControlFactModeSelected, Action: model.RunControlAction(mode),
		Source: model.RunControlSourceUI, CreatedAt: s.now(),
	}
	if err := s.store.AppendRunControl(ctx, modeFact, s.now()); err != nil {
		return nil, nil, err
	}
	s.logFact(startedPathRun.ID, modeFact, 0)
	_ = s.store.AppendRunEvent(ctx, model.RunEvent{
		RunID: startedRun.ID, PathRunID: &startedPathRun.ID,
		Kind: "mode_selected", Label: fmt.Sprintf("运行模式选定：%s", model.RunModeName(mode)),
	}, s.now())

	session := &activeStep{
		runCtx: runCtx, mode: mode,
		breakpoints:     NewBreakpointSet(),
		version:         1,
		recoveryLog:     s.recoveryLog,
		executedStepNos: map[int]bool{}, executedNodeKeys: map[string]bool{},
	}
	for _, bp := range preset {
		if err := ValidateBreakpointTarget(bp, session.executedStepNos, session.executedNodeKeys); err != nil {
			return nil, nil, err
		}
		session.breakpoints.Add(bp)
		objectKind, objectKey := BreakpointToObject(bp)
		setFact := model.RunControl{
			RunID: startedRun.ID, PathRunID: startedPathRun.ID,
			Kind: model.ControlFactBreakpointSet, BreakpointType: bp.Type,
			ObjectKind: objectKind, ObjectKey: objectKey,
			Source: model.RunControlSourceUI, CreatedAt: s.now(),
		}
		if err := s.store.AppendRunControl(ctx, setFact, s.now()); err != nil {
			return nil, nil, err
		}
		s.logFact(startedPathRun.ID, setFact, 0)
		_ = s.store.AppendRunEvent(ctx, model.RunEvent{
			RunID: startedRun.ID, PathRunID: &startedPathRun.ID,
			Kind: "breakpoint_set", Label: fmt.Sprintf("断点已设置：%s", bp.Label()),
		}, s.now())
	}

	preview, finished, err := s.steps.BuildPreview(ctx, runCtx, 0)
	if err != nil {
		return nil, nil, err
	}
	result := &StartResult{Run: startedRun, PathRun: startedPathRun, Preview: preview}
	if finished {
		class := model.FailureClassToolBug
		if _, err := s.runs.Finish(ctx, startedPathRun.ID, model.PathRunStatusFailed, runResultOf(model.RunResultFailed), &class,
			"编译场景为空，无法执行"); err != nil {
			return nil, nil, err
		}
		result.PathFinished = true
		return result, session, nil
	}
	session.preview = preview
	session.nextIndex = 0
	return result, session, nil
}

// StartWithMode 按模式启动：单步/人工控制停在第一步之前；自动运行立即进入连续执行
//（首个写步骤被默认开启的首次写断点拦下——这是安全阀，不是可选项）。
func (s *Service) StartWithMode(ctx context.Context, runCtx step.RunContext, mode model.RunMode, preset []Breakpoint) (*StartResult, error) {
	result, session, err := s.startSession(ctx, runCtx, mode, preset)
	if err != nil {
		return nil, err
	}
	if result.PathFinished {
		return result, nil
	}
	s.mu.Lock()
	s.active[result.PathRun.ID] = session
	s.mu.Unlock()
	if mode == model.RunModeAuto {
		s.startLoop(ctx, result.PathRun.ID, session, model.CommandContinue)
	}
	return result, nil
}

// CurrentPreview 返回当前等待放行的步骤预览；没有现场时返回 nil。
func (s *Service) CurrentPreview(pathRunID uint64) *step.StepPreview {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := s.active[pathRunID]
	if session == nil {
		return nil
	}
	return session.preview
}

// SessionView 是详情页需要的控制现场摘要。
type SessionView struct {
	Mode           model.RunMode
	Breakpoints    []Breakpoint
	StopReason     string
	Commands       []model.ControlCommand
	PauseState     PauseState
	Version        int64
	LoopRunning    bool
	StopRequested  bool
	PauseRequested bool
	// Reconcile 是最近一次只读对账的结论；待对账工作区的数据源。
	Reconcile *ReconcileResultView
	// CurrentPhase/CurrentPhaseNote 是当前步的实时阶段与中文补充；CurrentPhaseSince 是进入时刻。
	CurrentPhase      string
	CurrentPhaseNote  string
	CurrentPhaseSince time.Time
}

// View 返回当前控制现场摘要；无现场时返回 nil。
func (s *Service) View(pathRunID uint64) *SessionView {
	s.mu.Lock()
	session := s.active[pathRunID]
	if session == nil {
		s.mu.Unlock()
		return nil
	}
	view := &SessionView{
		Mode: session.mode, Breakpoints: session.breakpoints.List(),
		StopReason: session.stopReason, Version: session.version,
		LoopRunning: session.loopRunning, StopRequested: session.stopRequested,
		PauseRequested: session.pauseRequested,
	}
	view.PauseState = session.pauseState()
	view.Commands = AvailableCommands(session.mode, view.PauseState)
	view.Reconcile = session.reconcile
	view.CurrentPhase = session.progress.phase
	view.CurrentPhaseNote = session.progress.note
	view.CurrentPhaseSince = session.progress.since
	s.mu.Unlock()
	return view
}

// SetBreakpoint 运行中增加断点：校验挂载对象（只能挂未执行对象）、落事实、即时生效并写事件。
func (s *Service) SetBreakpoint(ctx context.Context, pathRunID uint64, bp Breakpoint) ([]Breakpoint, error) {
	s.mu.Lock()
	session := s.active[pathRunID]
	s.mu.Unlock()
	if session == nil {
		return nil, ErrNoActiveStep
	}
	if err := ValidateBreakpointTarget(bp, session.executedStepNos, session.executedNodeKeys); err != nil {
		return nil, err
	}
	session.breakpoints.Add(bp)
	pathRun, err := s.runs.GetPathRun(ctx, pathRunID)
	if err != nil {
		return nil, err
	}
	objectKind, objectKey := BreakpointToObject(bp)
	setFact := model.RunControl{
		RunID: pathRun.RunID, PathRunID: pathRunID,
		Kind: model.ControlFactBreakpointSet, BreakpointType: bp.Type,
		ObjectKind: objectKind, ObjectKey: objectKey,
		Source: model.RunControlSourceUI, CreatedAt: s.now(),
	}
	if err := s.store.AppendRunControl(ctx, setFact, s.now()); err != nil {
		return nil, err
	}
	s.logFact(pathRunID, setFact, previewStepNo(session))
	_ = s.store.AppendRunEvent(ctx, model.RunEvent{
		RunID: pathRun.RunID, PathRunID: &pathRunID,
		Kind: "breakpoint_set", Label: fmt.Sprintf("断点已设置：%s", bp.Label()),
	}, s.now())
	return session.breakpoints.List(), nil
}

// RemoveBreakpoint 运行中删除断点；路径偏离断点拒绝删除并给中文原因。
func (s *Service) RemoveBreakpoint(ctx context.Context, pathRunID uint64, bp Breakpoint) ([]Breakpoint, error) {
	s.mu.Lock()
	session := s.active[pathRunID]
	s.mu.Unlock()
	if session == nil {
		return nil, ErrNoActiveStep
	}
	if bp.Type == model.BreakpointPathDeviation {
		return nil, fmt.Errorf("路径偏离断点强制开启，不能关闭")
	}
	if !session.breakpoints.Remove(bp) {
		return session.breakpoints.List(), nil
	}
	pathRun, err := s.runs.GetPathRun(ctx, pathRunID)
	if err != nil {
		return nil, err
	}
	objectKind, objectKey := BreakpointToObject(bp)
	removeFact := model.RunControl{
		RunID: pathRun.RunID, PathRunID: pathRunID,
		Kind: model.ControlFactBreakpointRemove, BreakpointType: bp.Type,
		ObjectKind: objectKind, ObjectKey: objectKey,
		Source: model.RunControlSourceUI, CreatedAt: s.now(),
	}
	if err := s.store.AppendRunControl(ctx, removeFact, s.now()); err != nil {
		return nil, err
	}
	s.logFact(pathRunID, removeFact, previewStepNo(session))
	_ = s.store.AppendRunEvent(ctx, model.RunEvent{
		RunID: pathRun.RunID, PathRunID: &pathRunID,
		Kind: "breakpoint_removed", Label: fmt.Sprintf("断点已删除：%s", bp.Label()),
	}, s.now())
	return session.breakpoints.List(), nil
}

// ListBreakpoints 返回当前生效断点（由控制事实回放得出，与内存集合同源核对）。
func (s *Service) ListBreakpoints(ctx context.Context, pathRunID uint64) ([]Breakpoint, error) {
	controls, err := s.store.ListRunControls(ctx, pathRunID)
	if err != nil {
		return nil, err
	}
	return ReplayBreakpoints(controls).List(), nil
}

// RequestPause 提交暂停请求：随时可提交、只在阶段 3 生效。
// 连续执行循环存活时，本步走完 verify 与 settle 后停下并落「暂停生效」事实。
func (s *Service) RequestPause(ctx context.Context, pathRunID uint64) error {
	pathRun, err := s.runs.GetPathRun(ctx, pathRunID)
	if err != nil {
		return err
	}
	if err := s.store.AppendRunControl(ctx, model.RunControl{
		RunID: pathRun.RunID, PathRunID: pathRunID,
		Kind: model.ControlFactPauseRequested, Source: model.RunControlSourceUI, CreatedAt: s.now(),
	}, s.now()); err != nil {
		return err
	}
	s.mu.Lock()
	session := s.active[pathRunID]
	if session != nil {
		session.pauseRequested = true
	}
	loopRunning := session != nil && session.loopRunning
	s.mu.Unlock()
	if !loopRunning {
		// 未在连续执行中：当前本来就停在阶段 3，请求如实落档即可。
		return nil
	}
	return s.store.AppendRunControl(ctx, model.RunControl{
		RunID: pathRun.RunID, PathRunID: pathRunID,
		Kind: model.ControlFactPaused, Source: model.RunControlSourceUI, CreatedAt: s.now(),
	}, s.now())
}

// ApproveWithCommand 按命令放行（条件写 + 幂等）：
// 命令携带当前步游标与控制版本；版本或游标不匹配返回中文冲突说明，重复提交只产生一次效果。
// step 命令同步执行一步；next_node/continue 启动连续执行循环后立即返回（前端轮询状态）。
func (s *Service) ApproveWithCommand(ctx context.Context, pathRunID uint64, command model.ControlCommand, cursor int, version int64) (*ApproveResult, error) {
	s.mu.Lock()
	session := s.active[pathRunID]
	if session == nil {
		s.mu.Unlock()
		return nil, ErrNoActiveStep
	}
	if session.loopRunning {
		s.mu.Unlock()
		return nil, ErrLoopRunning
	}
	if version != session.version {
		s.mu.Unlock()
		return nil, ErrVersionConflict
	}
	if session.preview == nil || cursor != session.preview.StepNo {
		s.mu.Unlock()
		return nil, ErrCursorConflict
	}
	s.mu.Unlock()

	if session.finished {
		s.clear(pathRunID)
		return nil, ErrRunAlreadyFinished
	}
	// 可用命令集合由服务端按模式与状态给出：偏离停止无放行类命令。
	commands := AvailableCommands(session.mode, session.pauseState())
	allowed := false
	for _, available := range commands {
		if available == command {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("%w：当前状态下可用命令为 %v", ErrCommandNotAllowed, commands)
	}
	pathRun, err := s.runs.GetPathRun(ctx, pathRunID)
	if err != nil {
		return nil, err
	}
	if pathRun.Status != model.PathRunStatusRunning {
		return nil, fmt.Errorf("路径运行当前为 %s，不能放行", model.PathRunStatusName(pathRun.Status))
	}
	approveFact := model.RunControl{
		RunID: pathRun.RunID, PathRunID: pathRunID,
		Kind: model.ControlFactApproved, Action: model.RunControlApprove,
		Command: command, Source: model.RunControlSourceUI, CreatedAt: s.now(),
	}
	if err := s.store.AppendRunControl(ctx, approveFact, s.now()); err != nil {
		return nil, err
	}
	s.logFact(pathRunID, approveFact, previewStepNo(session))

	if command == model.CommandStep {
		return s.approveOneStep(ctx, pathRunID, session, 1)
	}
	// next_node / continue：启动连续执行循环并立即返回当前状态（前端按配置轮询）。
	s.startLoop(ctx, pathRunID, session, command)
	return &ApproveResult{}, nil
}

// approveOneStep 执行一步（单步模式、step 命令与对账重放共用），随后停在下一步之前或收尾。
// attemptNo 是本次尝试序号（对账重放时递增，作为新的一次尝试记录）。
func (s *Service) approveOneStep(ctx context.Context, pathRunID uint64, session *activeStep, attemptNo int) (*ApproveResult, error) {
	// 保存写之前的目标事实基准，供对账收集器对照。
	session.runCtx.LastBeforeFacts = session.preview.Facts
	// 阶段进度上报：执行器在各阶段边界回调，写进会话现场供详情轮询读取。
	reporter := func(phase, note string) {
		s.mu.Lock()
		session.progress = stepPhaseProgress{phase: phase, note: note, since: s.now()}
		s.mu.Unlock()
	}
	outcome, _, err := s.steps.RunApprovedStep(ctx, step.ApprovedStep{
		RunCtx: session.runCtx, Preview: session.preview, NextIndex: session.nextIndex,
		Attempt: attemptNo, ReportProgress: reporter,
	})
	if err != nil {
		return nil, err
	}
	result := &ApproveResult{Outcome: outcome}
	if outcome.Verdict != "confirmed_success" {
		// 确定失败或不确定：路径运行已进终态，现场作废。
		s.clear(pathRunID)
		return result, nil
	}
	if outcome.MainInstanceRef != "" {
		session.runCtx.PathRun.MainInstanceRef = outcome.MainInstanceRef
	}
	s.mu.Lock()
	session.executedStepNos[session.preview.StepNo] = true
	session.executedNodeKeys[session.preview.NodeKey] = true
	s.mu.Unlock()
	if outcome.DeviationDetected {
		s.mu.Lock()
		session.deviationStalled = true
		s.mu.Unlock()
		_ = s.store.AppendRunEvent(ctx, model.RunEvent{
			RunID: session.runCtx.Run.ID, PathRunID: &pathRunID,
			Kind: "path_deviation", Label: "核验发现实际分支与已配置路径不一致，下一步将强制停止且不提供放行",
		}, s.now())
	}

	nextIndex := session.nextIndex + 1
	if outcome.NoMoreSteps {
		facts, reviewErr := s.steps.FinalReview(ctx, session.runCtx)
		if reviewErr != nil {
			return result, reviewErr
		}
		if err := s.finishCompleted(ctx, pathRunID, session.runCtx, facts); err != nil {
			return result, err
		}
		s.clear(pathRunID)
		result.PathFinished = true
		result.FinalFacts = &facts
		return result, nil
	}
	preview, finished, err := s.steps.BuildPreview(ctx, session.runCtx, nextIndex)
	if err != nil {
		return result, err
	}
	if finished {
		facts, reviewErr := s.steps.FinalReview(ctx, session.runCtx)
		if reviewErr != nil {
			return result, reviewErr
		}
		if err := s.finishCompleted(ctx, pathRunID, session.runCtx, facts); err != nil {
			return result, err
		}
		s.clear(pathRunID)
		result.PathFinished = true
		result.FinalFacts = &facts
		return result, nil
	}
	s.mu.Lock()
	session.preview = preview
	session.nextIndex = nextIndex
	session.version++
	s.mu.Unlock()
	if outcome.DeviationDetected {
		s.mu.Lock()
		session.stopReason = "路径偏离：实际命中分支与已配置路径不一致，后续步骤不再提供放行，只能停止或查看"
		s.mu.Unlock()
	}
	result.NextPreview = preview
	return result, nil
}

// finishCompleted 场景走完的统一收尾：最终目标事实摘要落库 + 路径运行与运行聚合收尾为已完成。
func (s *Service) finishCompleted(ctx context.Context, pathRunID uint64, runCtx step.RunContext, facts step.FinalTargetFacts) error {
	summary, err := json.Marshal(facts)
	if err != nil {
		return err
	}
	if err := s.store.SetFinalTargetSummary(ctx, pathRunID, string(summary), s.now()); err != nil {
		return err
	}
	_, err = s.runs.Finish(ctx, pathRunID, model.PathRunStatusCompleted, runResultOf(model.RunResultSucceeded), nil,
		"编译场景全部步骤确定成功，路径运行完成")
	return err
}

// Stop 处理用户停止：停止是终态；连续执行循环存活时先落「请求停止」事实，由循环在本步走完后生效。
func (s *Service) Stop(ctx context.Context, pathRunID uint64) (model.PathRun, error) {
	pathRun, err := s.runs.GetPathRun(ctx, pathRunID)
	if err != nil {
		return model.PathRun{}, err
	}
	if model.IsTerminalPathRunStatus(pathRun.Status) {
		s.clear(pathRunID)
		return pathRun, ErrRunAlreadyFinished
	}
	s.mu.Lock()
	session := s.active[pathRunID]
	loopRunning := session != nil && session.loopRunning
	if session != nil {
		session.stopRequested = true
	}
	s.mu.Unlock()
	if loopRunning {
		// 循环存活：落请求事实，由循环在本步走完 verify 与 settle 后执行停止。
		if err := s.store.AppendRunControl(ctx, model.RunControl{
			RunID: pathRun.RunID, PathRunID: pathRunID,
			Kind: model.ControlFactStopRequested, Source: model.RunControlSourceUI, CreatedAt: s.now(),
		}, s.now()); err != nil {
			return model.PathRun{}, err
		}
		return pathRun, ErrStopDeferred
	}
	if pathRun.Status == model.PathRunStatusVerifying {
		return pathRun, ErrStopDeferred
	}
	if pathRun.Status != model.PathRunStatusRunning {
		return pathRun, fmt.Errorf("%w：路径运行当前为 %s，不能停止", ErrNotRunnable, model.PathRunStatusName(pathRun.Status))
	}
	if err := s.store.AppendRunControl(ctx, model.RunControl{
		RunID: pathRun.RunID, PathRunID: pathRunID,
		Kind: model.ControlFactStopRequested, Source: model.RunControlSourceUI, CreatedAt: s.now(),
	}, s.now()); err != nil {
		return model.PathRun{}, err
	}
	stopped, err := s.runs.Stop(ctx, pathRunID)
	if err != nil {
		return model.PathRun{}, err
	}
	if err := s.store.AppendRunControl(ctx, model.RunControl{
		RunID: pathRun.RunID, PathRunID: pathRunID,
		Kind: model.ControlFactStopped, Source: model.RunControlSourceUI, CreatedAt: s.now(),
	}, s.now()); err != nil {
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

// previewStepNo 返回会话当前预览的步骤序号（control.log 关联键）；无现场时为 0。
func previewStepNo(session *activeStep) int {
	if session == nil || session.preview == nil {
		return 0
	}
	return session.preview.StepNo
}
