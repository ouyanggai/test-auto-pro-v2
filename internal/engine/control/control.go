// Package control 实现运行控制（纲领第 4.3 阶段 3、4.5、5.1-5.4 节）：
// 启动（模式三选一）、断点增删与命中、暂停、三条放行命令、停止。
// 写结果无法确认是终局（2026-09-06 产品裁决）：现场作废、运行聚合收尾，不提供对账与恢复动作。
// 边界：不实现导航与恢复步骤（F-019）、多路径调度（F-020）；
// 不提供批量放行；不改写已发生事实。
package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/run"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/engine/verdict"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// controlErrorMessage 把目标错误链中的原始 message/code传给运行状态，避免控制台只显示内部错误分类。
func controlErrorMessage(err error) string {
	if message := target.UserFacingErrorMessage(target.WriteResponse{}, err); message != "" {
		return message
	}
	if err != nil {
		return err.Error()
	}
	return "未知错误"
}

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
	// ErrStepInFlight 表示当前步骤已经在后台执行，重复放行不能再次发起真实写请求。
	ErrStepInFlight = errors.New("当前步骤正在执行，请等待结果")
	// ErrVersionConflict 表示控制版本不匹配：界面状态已过期，需以返回的当前状态为准。
	ErrVersionConflict = errors.New("控制状态已变化，请以当前运行状态为准后重试")
	// ErrCursorConflict 表示步游标不匹配：当前等待放行的步骤已不是命令所指的那一步。
	ErrCursorConflict = errors.New("当前等待放行的步骤已变化，请以最新预览为准")
	// ErrStepAlreadyExecuted 表示该步已经落账成功而下一步预览尚未构建好（常见于目标结构读取瞬断）。
	// 放行绝不能把同一步再执行一遍：写步骤会重复写，只读步骤也会重复落账虚增进度。
	ErrStepAlreadyExecuted = errors.New("该步骤已执行完成，正在等待目标状态恢复以准备下一步；请稍后刷新后重试")
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
	// pendingMode 是已落事实、待安全边界（当前步走完核验与落账）生效的模式切换请求。
	// nil 表示没有待生效的切换；切换只影响当前路径运行，绝不暗中改变其他路径。
	pendingMode *model.RunMode
	// loopRunning 表示连续执行循环存活。
	loopRunning bool
	// stepInFlight 表示一步正在执行（从取步租约到落账）：这期间停止必须延后到本步
	// 走完 verify 与 settle，绝不能立刻终结路径运行——否则写请求返回后事实无处落账（纲领第 4.5 节）。
	stepInFlight bool
	// stopReason 是「为什么停在这里」的中文主因。
	stopReason string
	// version 是控制命令条件写的版本（每次控制状态变化自增）。
	version int64
	// executedStepNos/executedNodeKeys 供断点挂载校验（只能挂未执行对象）。
	executedStepNos  map[int]bool
	executedNodeKeys map[string]bool
	// finished 表示路径运行已到终态，现场可回收。
	finished bool
	// recoveryLog 把写结果无法确认的内部判定写进运行目录的 recovery.log。
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
	if sess.finished {
		return PauseStateFinished
	}
	if sess.deviationStalled {
		return PauseStateDeviation
	}
	// 结果待确认的会话在不确定落账时立即作废，正常运行到不了 PauseStateUncertain；
	// 保留这个分支作为安全兜底：万一出现该状态，可用命令集合必须是空集，绝不给放行入口。
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

// SetRecoveryLog 注入 recovery.log 写入器；必须在首次执行前调用。
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
	return s.initSession(ctx, runCtx, mode, preset)
}

// BeginPathRun 在调度器已创建的等待路径运行上开始执行会话（F-020）：
// 推进等待→运行中，其余与 startSession 完全同一条链路——模式与断点事实、第一步预览、自动模式进循环。
// runCtx.Run / runCtx.PathRun 必须已由编排层填充且路径运行处于等待运行。
func (s *Service) BeginPathRun(ctx context.Context, runCtx step.RunContext, mode model.RunMode, preset []Breakpoint) (*StartResult, error) {
	startedPathRun, err := s.runs.AdvancePathRun(ctx, runCtx.PathRun.ID,
		model.PathRunStatusWaiting, model.PathRunStatusRunning, model.RunEvent{
			Kind:  "path_run_started",
			Label: "调度器启动路径运行，准备第一步预览",
		})
	if err != nil {
		return nil, err
	}
	// 待发/已发来源的实例引用由编排层在启动前预填（草稿实例的门禁与事实重读依赖它）；
	// 状态推进从库回读的行没有这个内存值，覆盖前必须保留，否则门禁又读不到实例状态。
	if startedPathRun.MainInstanceRef == "" && runCtx.PathRun.MainInstanceRef != "" {
		startedPathRun.MainInstanceRef = runCtx.PathRun.MainInstanceRef
	}
	runCtx.PathRun = startedPathRun
	result, session, err := s.initSession(ctx, runCtx, mode, preset)
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

// initSession 是启动的公共主体：冻结总步骤数、模式选定事实、预置断点逐条落事实、构建第一步预览并停在阶段 3。
// 调用方负责创建/推进运行与路径运行并把真实身份填进 runCtx。
func (s *Service) initSession(ctx context.Context, runCtx step.RunContext, mode model.RunMode, preset []Breakpoint) (*StartResult, *activeStep, error) {
	startedRun := runCtx.Run
	startedPathRun := runCtx.PathRun

	// 冻结本次运行的总步骤数（进度分母）：取启动时已保存编译场景的长度，
	// 配置后续变化不得改变已运行进度的分母（2026-09-06 运行记录层级要求）。
	// 冻结失败直接拒绝启动：没有分母的进度不真实，宁可不开跑。
	if err := s.store.SetPathRunTotalSteps(ctx, startedPathRun.ID, len(runCtx.Steps), s.now()); err != nil {
		return nil, nil, err
	}

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
// （首个写步骤被默认开启的首次写断点拦下——这是安全阀，不是可选项）。
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

// ArmRetrySession 把重试后的控制现场装填回内存（F-028 失败动作重试）。
// 前置条件由编排层保证：路径运行已重开为运行中，runCtx.Run/PathRun 已填入重开后的真实身份，
// cursorIndex 是失败步骤在重编译场景中的下标，executed 集合只含已成功步骤。
// 与启动的差异：不创建运行、不重落模式选定事实、不改冻结的总步骤数；
// 断点按控制事实回放——本运行已有成功步骤时移除首次写断点，避免自动重试被安全阀二次拦停
// （首个写请求早已被放行过，安全阀只对「从失败的第一步重新开始」有意义）。
// 重试控制事实先落库再装填现场；装填后自动模式随即进入连续执行循环，单步/人工控制等待放行。
func (s *Service) ArmRetrySession(ctx context.Context, runCtx step.RunContext, cursorIndex int, executedStepNos map[int]bool, executedNodeKeys map[string]bool) (*StartResult, error) {
	controls, err := s.store.ListRunControls(ctx, runCtx.PathRun.ID)
	if err != nil {
		return nil, err
	}
	breakpoints := RetryBreakpointSet(controls, cursorIndex)
	// 预览构建依赖目标实时读取；失败步骤已不在场景中说明事实与场景错位，拒绝而不是静默跳过。
	preview, finished, err := s.steps.BuildPreview(ctx, runCtx, cursorIndex)
	if err != nil {
		return nil, err
	}
	if finished {
		return nil, fmt.Errorf("失败步骤已不在当前场景中，无法重试；请从计划重新发起运行")
	}
	session := &activeStep{
		runCtx: runCtx, mode: runCtx.Run.Mode,
		breakpoints:     breakpoints,
		version:         1,
		recoveryLog:     s.recoveryLog,
		executedStepNos: executedStepNos, executedNodeKeys: executedNodeKeys,
		preview:   preview,
		nextIndex: cursorIndex,
	}
	retryFact := model.RunControl{
		RunID: runCtx.Run.ID, PathRunID: runCtx.PathRun.ID,
		Kind: model.ControlFactRetryRequested, Action: model.RunControlRetry,
		Source: model.RunControlSourceUI, CreatedAt: s.now(),
	}
	if err := s.store.AppendRunControl(ctx, retryFact, s.now()); err != nil {
		return nil, err
	}
	s.logFact(runCtx.PathRun.ID, retryFact, preview.StepNo)
	s.mu.Lock()
	s.active[runCtx.PathRun.ID] = session
	s.mu.Unlock()
	if session.mode == model.RunModeAuto {
		s.startLoop(ctx, runCtx.PathRun.ID, session, model.CommandContinue)
	}
	return &StartResult{Run: runCtx.Run, PathRun: runCtx.PathRun, Preview: preview}, nil
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
	StepInFlight   bool
	StopRequested  bool
	PauseRequested bool
	// PendingMode 是待安全边界生效的模式切换请求；非 nil 时界面显示「将在本步完成后切换」。
	PendingMode *model.RunMode
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
		StepInFlight:   session.stepInFlight,
		PauseRequested: session.pauseRequested,
	}
	view.PauseState = session.pauseState()
	if !session.stepInFlight {
		view.Commands = AvailableCommands(session.mode, view.PauseState)
	}
	view.PendingMode = session.pendingMode
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
	s.mu.Lock()
	// 断点集合会被轮询（View）与执行循环并发读取：增删与校验必须整体持锁，
	// 否则 map 并发读写会直接带走整个进程（评审 P1）。
	if err := ValidateBreakpointTarget(bp, session.executedStepNos, session.executedNodeKeys); err != nil {
		s.mu.Unlock()
		return nil, err
	}
	session.breakpoints.Add(bp)
	updated := session.breakpoints.List()
	stepNo := previewStepNo(session)
	s.mu.Unlock()
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
	s.logFact(pathRunID, setFact, stepNo)
	_ = s.store.AppendRunEvent(ctx, model.RunEvent{
		RunID: pathRun.RunID, PathRunID: &pathRunID,
		Kind: "breakpoint_set", Label: fmt.Sprintf("断点已设置：%s", bp.Label()),
	}, s.now())
	return updated, nil
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
	s.mu.Lock()
	removed := session.breakpoints.Remove(bp)
	updated := session.breakpoints.List()
	stepNo := previewStepNo(session)
	s.mu.Unlock()
	if !removed {
		return updated, nil
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
	s.logFact(pathRunID, removeFact, stepNo)
	_ = s.store.AppendRunEvent(ctx, model.RunEvent{
		RunID: pathRun.RunID, PathRunID: &pathRunID,
		Kind: "breakpoint_removed", Label: fmt.Sprintf("断点已删除：%s", bp.Label()),
	}, s.now())
	return updated, nil
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
	stepInFlight := session != nil && session.stepInFlight
	s.mu.Unlock()
	if !loopRunning && !stepInFlight {
		// 未在连续执行中：当前本来就停在阶段 3，请求如实落档即可。
		return nil
	}
	// 连续执行或单步后台执行在本步结束后才落暂停生效事实，避免把正在写入的步骤
	// 误标成已经停下；执行器完成核验与落账后由对应后台收尾统一处理。
	return nil
}

// SwitchMode 运行中切换自动/单步模式（2026-09-06 交付验收）：
// 条件写带控制版本（重复点击只产生一次控制事实），只影响当前路径运行；
// 切换在安全步骤边界生效——当前停在阶段 3 时立即生效，本步正在执行或连续执行中时
// 在本步走完核验与落账后生效，绝不中途打断已发出的写请求。
func (s *Service) SwitchMode(ctx context.Context, pathRunID uint64, mode model.RunMode, version int64) (*SessionView, error) {
	if mode != model.RunModeAuto && mode != model.RunModeSingleStep {
		return nil, fmt.Errorf("运行中只能在自动与单步之间切换，不能切换为%s", model.RunModeName(mode))
	}
	s.mu.Lock()
	session := s.active[pathRunID]
	if session == nil {
		s.mu.Unlock()
		return nil, ErrNoActiveStep
	}
	if session.finished {
		s.mu.Unlock()
		return nil, ErrRunAlreadyFinished
	}
	if session.deviationStalled {
		s.mu.Unlock()
		return nil, fmt.Errorf("%w：路径偏离后不提供模式切换", ErrNotRunnable)
	}
	// 幂等：请求的模式已经（或将要在本步后）生效时，不产生新的控制事实。
	if session.mode == mode || (session.pendingMode != nil && *session.pendingMode == mode) {
		commands := []model.ControlCommand{}
		if !session.stepInFlight {
			commands = AvailableCommands(session.mode, session.pauseState())
		}
		view := &SessionView{
			Mode: session.mode, Breakpoints: session.breakpoints.List(),
			StopReason: session.stopReason, Version: session.version,
			LoopRunning: session.loopRunning, StepInFlight: session.stepInFlight, StopRequested: session.stopRequested,
			PauseRequested: session.pauseRequested, PendingMode: session.pendingMode,
			PauseState: session.pauseState(), Commands: commands,
			CurrentPhase: session.progress.phase, CurrentPhaseNote: session.progress.note,
			CurrentPhaseSince: session.progress.since,
		}
		s.mu.Unlock()
		return view, nil
	}
	if version != session.version {
		s.mu.Unlock()
		return nil, ErrVersionConflict
	}
	// 版本自增在锁内完成：并发重复请求到这里必然版本冲突，绝不产生第二条切换事实。
	session.pendingMode = &mode
	session.version++
	applyNow := !session.loopRunning && !session.stepInFlight
	applied := mode
	if applyNow {
		session.mode = mode
		session.pendingMode = nil
	} else {
		applied = model.RunMode("")
	}
	s.mu.Unlock()

	pathRun, err := s.runs.GetPathRun(ctx, pathRunID)
	if err != nil {
		return nil, err
	}
	requestedFact := model.RunControl{
		RunID: pathRun.RunID, PathRunID: pathRunID,
		Kind: model.ControlFactModeSwitchReq, Command: model.ControlCommand(mode),
		Source: model.RunControlSourceUI, CreatedAt: s.now(),
	}
	if err := s.store.AppendRunControl(ctx, requestedFact, s.now()); err != nil {
		return nil, err
	}
	s.logFact(pathRunID, requestedFact, 0)
	if applyNow {
		s.appendModeSwitchedFact(ctx, pathRunID, pathRun.RunID, applied, "本步尚未开始，立即生效")
	} else {
		_ = s.store.AppendRunEvent(ctx, model.RunEvent{
			RunID: pathRun.RunID, PathRunID: &pathRunID,
			Kind:  "mode_switch_requested",
			Label: fmt.Sprintf("请求切换为%s模式：将在本步完成后生效，已发出的写请求不受影响", model.RunModeName(mode)),
		}, s.now())
	}
	return s.View(pathRunID), nil
}

// appendModeSwitchedFact 落「模式切换生效」事实、control.log 与运行事件。
func (s *Service) appendModeSwitchedFact(ctx context.Context, pathRunID, runID uint64, mode model.RunMode, when string) {
	switchedFact := model.RunControl{
		RunID: runID, PathRunID: pathRunID,
		Kind: model.ControlFactModeSwitched, Command: model.ControlCommand(mode),
		Source: model.RunControlSourceUI, CreatedAt: s.now(),
	}
	if err := s.store.AppendRunControl(ctx, switchedFact, s.now()); err != nil {
		s.warnFactFailure(pathRunID, "模式切换生效事实", err)
	} else {
		s.logFact(pathRunID, switchedFact, 0)
	}
	_ = s.store.AppendRunEvent(ctx, model.RunEvent{
		RunID: runID, PathRunID: &pathRunID,
		Kind:  "mode_switched",
		Label: fmt.Sprintf("运行模式已切换为%s（%s）", model.RunModeName(mode), when),
	}, s.now())
}

// applyPendingMode 在安全边界（本步走完核验与落账）应用待生效的模式切换；返回是否切换为单步。
// 单步意味着「每步必停」，连续执行循环必须在下一个写请求之前退出。
func (s *Service) applyPendingMode(ctx context.Context, pathRunID uint64, session *activeStep, stepNo int) bool {
	s.mu.Lock()
	if session.pendingMode == nil {
		s.mu.Unlock()
		return false
	}
	mode := *session.pendingMode
	session.pendingMode = nil
	session.mode = mode
	runID := session.runCtx.Run.ID
	s.mu.Unlock()
	s.appendModeSwitchedFact(ctx, pathRunID, runID, mode, "本步已走完核验与落账")
	if mode == model.RunModeSingleStep {
		s.mu.Lock()
		session.stopReason = fmt.Sprintf("已切换为单步模式（第 %d 步走完后生效）：下一步执行前等待放行", stepNo)
		s.mu.Unlock()
		return true
	}
	return false
}

// ApproveWithCommand 按命令放行（条件写 + 幂等）：
// 命令携带当前步游标与控制版本；版本或游标不匹配返回中文冲突说明，重复提交只产生一次效果。
// step 命令同步执行一步；next_node/continue 启动连续执行循环后立即返回（前端轮询状态）。
func (s *Service) ApproveWithCommand(ctx context.Context, pathRunID uint64, command model.ControlCommand, cursor int, version int64) (*ApproveResult, error) {
	session, err := s.reserveApproval(pathRunID, command, cursor, version)
	if err != nil {
		return nil, err
	}
	pathRun, err := s.runs.GetPathRun(ctx, pathRunID)
	if err != nil {
		s.releaseApproval(pathRunID, session, command)
		return nil, err
	}
	if pathRun.Status != model.PathRunStatusRunning {
		s.releaseApproval(pathRunID, session, command)
		return nil, fmt.Errorf("路径运行当前为 %s，不能放行", model.PathRunStatusName(pathRun.Status))
	}
	approveFact := model.RunControl{
		RunID: pathRun.RunID, PathRunID: pathRunID,
		Kind: model.ControlFactApproved, Action: model.RunControlApprove,
		Command: command, Source: model.RunControlSourceUI, CreatedAt: s.now(),
	}
	if err := s.store.AppendRunControl(ctx, approveFact, s.now()); err != nil {
		s.releaseApproval(pathRunID, session, command)
		return nil, err
	}
	s.logFact(pathRunID, approveFact, previewStepNo(session))

	if command == model.CommandStep {
		result, err := s.approveOneStep(ctx, pathRunID, session, 1, false)
		if err == nil && result != nil && result.Outcome.Verdict == string(verdict.OutcomeSucceeded) && result.PathFinished == false && session.mode == model.RunModeManual {
			result, err = s.continueManualGroup(ctx, pathRunID, session, result)
		}
		if err != nil {
			s.sealPostWriteFailure(ctx, pathRunID, session, err)
			return result, err
		}
		// 本步走完 verify 与 settle 后，放行期间请求的模式切换在这里生效（2026-09-06）。
		stepNo := 0
		if session.preview != nil {
			stepNo = session.preview.StepNo
		}
		s.applyPendingMode(ctx, pathRunID, session, stepNo)
		// 本步走完 verify 与 settle 后，放行期间收到的停止请求在这里生效（纲领第 4.5 节）。
		s.mu.Lock()
		deferredStop := session.stopRequested && !session.finished &&
			result.Outcome.Verdict == string(verdict.OutcomeSucceeded) && !result.PathFinished
		s.mu.Unlock()
		if deferredStop {
			s.applyStop(ctx, pathRunID)
		}
		return result, err
	}
	// next_node / continue：启动连续执行循环并立即返回当前状态（前端按配置轮询）。
	s.startLoopReserved(ctx, pathRunID, session, command)
	return &ApproveResult{}, nil
}

// ApproveWithCommandAsync 按命令记录放行事实并立即返回；单步动作在后台完成七阶段执行，
// 调用方通过 View 读取步骤执行中、核验中和落账后的状态。HTTP 请求不能绑定目标慢读重试，
// 否则一次事实重读失败会把放行接口占住数分钟，前端看起来像“没有反应”。
func (s *Service) ApproveWithCommandAsync(ctx context.Context, pathRunID uint64, command model.ControlCommand, cursor int, version int64) error {
	session, err := s.reserveApproval(pathRunID, command, cursor, version)
	if err != nil {
		return err
	}
	pathRun, err := s.runs.GetPathRun(ctx, pathRunID)
	if err != nil {
		s.releaseApproval(pathRunID, session, command)
		return err
	}
	if pathRun.Status != model.PathRunStatusRunning {
		s.releaseApproval(pathRunID, session, command)
		return fmt.Errorf("路径运行当前为 %s，不能放行", model.PathRunStatusName(pathRun.Status))
	}
	approveFact := model.RunControl{
		RunID: pathRun.RunID, PathRunID: pathRunID,
		Kind: model.ControlFactApproved, Action: model.RunControlApprove,
		Command: command, Source: model.RunControlSourceUI, CreatedAt: s.now(),
	}
	if err := s.store.AppendRunControl(ctx, approveFact, s.now()); err != nil {
		s.releaseApproval(pathRunID, session, command)
		return err
	}
	s.logFact(pathRunID, approveFact, previewStepNo(session))
	if command == model.CommandStep {
		s.startSingleStepReserved(ctx, pathRunID, session)
		return nil
	}
	s.startLoopReserved(ctx, pathRunID, session, command)
	return nil
}

// reserveApproval 在任何外部读写前占用本次放行的执行槽，保证并发请求最多产生一条放行事实和一次真实动作。
// 单步占用 stepInFlight，连续命令占用 loopRunning；后续失败由 releaseApproval 释放，不能留下假忙状态。
// StepReReleaseGuard 判断单步放行是否撞上"同一步已落账"的防重复执行护栏。
// 游标所指步骤已经执行成功而下一步预览尚未建出来时（常见于目标结构读取瞬断），
// 再次放行会把同一步再跑一遍：写步骤重复写、只读步骤重复落账虚增进度，都必须拒绝。
func StepReReleaseGuard(command model.ControlCommand, executedStepNos map[int]bool, cursor int) error {
	if command == model.CommandStep && executedStepNos[cursor] {
		return ErrStepAlreadyExecuted
	}
	return nil
}

func (s *Service) reserveApproval(pathRunID uint64, command model.ControlCommand, cursor int, version int64) (*activeStep, error) {
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
	if session.stepInFlight {
		s.mu.Unlock()
		return nil, ErrStepInFlight
	}
	if session.finished {
		s.mu.Unlock()
		s.clear(pathRunID)
		return nil, ErrRunAlreadyFinished
	}
	if version != session.version {
		s.mu.Unlock()
		return nil, ErrVersionConflict
	}
	if session.preview == nil || cursor != session.preview.StepNo {
		s.mu.Unlock()
		return nil, ErrCursorConflict
	}
	// 防重复执行护栏：游标所指的步骤已经落账成功（下一步预览因瞬时故障没建出来）时，
	// 再次放行绝不能把同一步再跑一遍，必须先等下一步预览恢复。
	if err := StepReReleaseGuard(command, session.executedStepNos, cursor); err != nil {
		s.mu.Unlock()
		return nil, err
	}
	commands := AvailableCommands(session.mode, session.pauseState())
	allowed := false
	for _, available := range commands {
		if available == command {
			allowed = true
			break
		}
	}
	if !allowed {
		s.mu.Unlock()
		return nil, fmt.Errorf("%w：当前状态下可用命令为 %v", ErrCommandNotAllowed, commands)
	}
	if command == model.CommandStep {
		session.stepInFlight = true
	} else {
		session.loopRunning = true
	}
	s.mu.Unlock()
	return session, nil
}

// releaseApproval 释放尚未启动后台执行的放行占用，避免数据库或状态校验失败后详情页永久显示忙碌。
func (s *Service) releaseApproval(pathRunID uint64, session *activeStep, command model.ControlCommand) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if current := s.active[pathRunID]; current != session {
		return
	}
	if command == model.CommandStep {
		session.stepInFlight = false
		return
	}
	session.loopRunning = false
}

// startSingleStep 把单步执行放入后台，并在安全边界处理暂停、模式切换和延迟停止。
// 步骤执行中不设置连续循环标记，避免单步被误显示成可继续的自动循环。
func (s *Service) startSingleStep(ctx context.Context, pathRunID uint64, session *activeStep) {
	s.mu.Lock()
	if session == nil || session.stepInFlight {
		s.mu.Unlock()
		return
	}
	session.stepInFlight = true
	s.mu.Unlock()
	s.startSingleStepReserved(ctx, pathRunID, session)
}

// startSingleStepReserved 启动已由放行校验占用的单步；调用方必须先设置 stepInFlight。
func (s *Service) startSingleStepReserved(ctx context.Context, pathRunID uint64, session *activeStep) {
	go func() {
		detached := context.WithoutCancel(ctx)
		result, err := s.approveOneStep(detached, pathRunID, session, 1, false)
		if err == nil && result != nil && result.Outcome.Verdict == string(verdict.OutcomeSucceeded) && result.PathFinished == false && session.mode == model.RunModeManual {
			result, err = s.continueManualGroup(detached, pathRunID, session, result)
		}
		if err != nil {
			s.sealPostWriteFailure(detached, pathRunID, session, err)
			s.mu.Lock()
			if current := s.active[pathRunID]; current == session && !session.finished {
				session.stopReason = "执行失败：" + controlErrorMessage(err)
			}
			s.mu.Unlock()
			return
		}
		if result == nil || result.Outcome.Verdict != string(verdict.OutcomeSucceeded) || result.PathFinished {
			return
		}
		stepNo := 0
		s.mu.Lock()
		if current := s.active[pathRunID]; current == session && session.preview != nil {
			stepNo = session.preview.StepNo
		}
		s.mu.Unlock()
		s.applyPendingMode(detached, pathRunID, session, stepNo)
		s.mu.Lock()
		deferredStop := session.stopRequested && !session.finished && !result.PathFinished
		pauseRequested := session.pauseRequested && !session.finished
		s.mu.Unlock()
		if pauseRequested {
			s.appendPausedFact(detached, pathRunID, session)
		}
		if deferredStop {
			s.applyStop(detached, pathRunID)
		}
	}()
}

// continueManualGroup 在人工控制模式下连续执行当前动作组内部步骤，遇到下一组首个步骤或安全边界时停下。
// 动作组内部步骤仍各自走七阶段、独立重读和落账；普通断点边界由外层控制循环处理。
func (s *Service) continueManualGroup(ctx context.Context, pathRunID uint64, session *activeStep, result *ApproveResult) (*ApproveResult, error) {
	s.mu.Lock()
	group := ""
	if session.preview != nil {
		group = session.preview.ReleaseGroup
	}
	s.mu.Unlock()
	if group == "" {
		return result, nil
	}
	for {
		s.mu.Lock()
		next := session.preview
		if next == nil || next.ReleaseGroup != group || next.ReleaseRequired || session.stopRequested || session.pauseRequested || session.pendingMode != nil || session.deviationStalled || session.finished {
			s.mu.Unlock()
			return result, nil
		}
		session.loopRunning = true
		s.mu.Unlock()
		nextResult, err := s.approveOneStep(ctx, pathRunID, session, 1, false)
		s.mu.Lock()
		session.loopRunning = false
		s.mu.Unlock()
		if err != nil {
			return nextResult, err
		}
		result = nextResult
		if result == nil || result.Outcome.Verdict != string(verdict.OutcomeSucceeded) || result.PathFinished {
			return result, nil
		}
	}
}

// appendPausedFact 在一步安全落账后记录暂停生效事实；失败只保留现场并写明原因。
func (s *Service) appendPausedFact(ctx context.Context, pathRunID uint64, session *activeStep) {
	s.mu.Lock()
	if session == nil || session.finished {
		s.mu.Unlock()
		return
	}
	runID := session.runCtx.Run.ID
	stepNo := previewStepNo(session)
	s.mu.Unlock()
	pausedFact := model.RunControl{RunID: runID, PathRunID: pathRunID, Kind: model.ControlFactPaused, Source: model.RunControlSourceUI, CreatedAt: s.now()}
	if err := s.store.AppendRunControl(ctx, pausedFact, s.now()); err != nil {
		s.mu.Lock()
		if current := s.active[pathRunID]; current == session {
			session.stopReason = loopFailureReason("暂停事实", err)
		}
		s.mu.Unlock()
		return
	}
	s.logFact(pathRunID, pausedFact, stepNo)
	s.mu.Lock()
	if current := s.active[pathRunID]; current == session {
		session.stopReason = "暂停请求已生效（本步已走完核验与落账）"
		session.pauseRequested = false
	}
	s.mu.Unlock()
}

// approveOneStep 执行一步（单步模式、step 命令与对账重放共用），随后停在下一步之前或收尾。
// attemptNo 是本次尝试序号（对账重放时递增，作为新的一次尝试记录）。
func (s *Service) approveOneStep(ctx context.Context, pathRunID uint64, session *activeStep, attemptNo int, isReplay bool) (*ApproveResult, error) {
	s.mu.Lock()
	session.stepInFlight = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		session.stepInFlight = false
		s.mu.Unlock()
	}()
	// 保存写之前的目标事实基准，供对账收集器对照；同时标记基准确实取到了。
	// 这份基准随后会随尝试行落库（before_facts），进程重启后按它还原。
	session.runCtx.LastBeforeFacts = session.preview.Facts
	session.runCtx.LastBeforeFactsKnown = true
	// 阶段进度上报：执行器在各阶段边界回调，写进会话现场供详情轮询读取。
	reporter := func(phase, note string) {
		s.mu.Lock()
		session.progress = stepPhaseProgress{phase: phase, note: note, since: s.now()}
		s.mu.Unlock()
	}
	outcome, _, err := s.steps.RunApprovedStep(ctx, step.ApprovedStep{
		RunCtx: session.runCtx, Preview: session.preview, NextIndex: session.nextIndex,
		Attempt: attemptNo, IsReplay: isReplay, ReportProgress: reporter,
	})
	if err != nil {
		return nil, err
	}
	result := &ApproveResult{Outcome: outcome}
	if outcome.Verdict == string(verdict.OutcomeUncertain) {
		// 写结果无法确认：2026-09-06 产品裁决移除用户侧对账后这就是终局。
		// 现场立即作废，绝不自动继续、重放或重复发送真实写请求；
		// 运行聚合随之收尾（结果待确认计入已停止），用户只能从计划重新发起一次运行。
		// recovery.log 与运行事件都留下这条内部判定，日志与记录双向可查。
		s.mu.Lock()
		runID := session.runCtx.Run.ID
		stepNo := 0
		if session.preview != nil {
			stepNo = session.preview.StepNo
		}
		s.mu.Unlock()
		s.recoveryLog.LogFact(pathRunID, fmt.Sprintf(
			"write_uncertain=1 decision=stop run_id=%d step_no=%d reason=目标状态未确认，已停止推进；继续执行请从计划重新运行", runID, stepNo))
		_ = s.store.AppendRunEvent(ctx, model.RunEvent{
			RunID: runID, PathRunID: &pathRunID,
			Kind:  "run_uncertain_closed",
			Label: "目标状态未确认，运行已停止；请查看该步骤的错误信息后从计划重新发起运行",
		}, s.now())
		s.clear(pathRunID)
		if _, err := s.store.FinishRunIfAllPathsClosed(ctx, runID, s.now()); err != nil {
			// 聚合收尾失败不改变「已停止推进」的终局：路径运行已闭合，收尾本身幂等，可再次触发。
			s.warnFactFailure(pathRunID, "运行聚合收尾", err)
		}
		return result, nil
	}
	if outcome.Verdict != "confirmed_success" {
		// 确定失败：路径运行已进终态且没有任何合法后续动作，现场作废。
		s.clear(pathRunID)
		return result, nil
	}
	if outcome.MainInstanceRef != "" {
		session.runCtx.PathRun.MainInstanceRef = outcome.MainInstanceRef
	}
	if outcome.FlowProxyID != "" {
		if strings.TrimSpace(outcome.FlowProxyID) != strings.TrimSpace(session.runCtx.FlowProxyID) {
			session.runCtx.FlowProxyRemapped = true
		}
		session.runCtx.FlowProxyID = outcome.FlowProxyID
	}
	if outcome.CurrentNodeProxyID != "" {
		if nodeKey := session.preview.NodeKey; nodeKey != "" {
			info := session.runCtx.Nodes[nodeKey]
			if strings.TrimSpace(info.TargetNodeID) != strings.TrimSpace(outcome.CurrentNodeProxyID) {
				session.runCtx.FlowProxyRemapped = true
			}
			info.TargetNodeID = outcome.CurrentNodeProxyID
			session.runCtx.Nodes[nodeKey] = info
		}
	}
	if auxiliaryRef := strings.TrimSpace(outcome.AuxiliaryInstanceRef); auxiliaryRef != "" {
		// 转发不改变主实例，辅助实例引用必须另落一条追加事件；Detail 仅供记录与审计使用，
		// 事件流对页面只公开中文结果，避免把目标内部标识混入用户界面。
		s.appendEventOrWarn(ctx, pathRunID, model.RunEvent{
			RunID: session.runCtx.Run.ID, PathRunID: &pathRunID,
			Kind: "forward_auxiliary_created", Label: "转发已创建辅助流程", Detail: auxiliaryRef,
		}, session.preview.StepNo)
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

	// 会签适配（2026-09-11 用户裁决：会签每个处理人都要审批，竞签其中一人处理即可）：
	// 同意成功后实例仍停在本节点，说明还有其他处理人未审批。重建本步预览——事实重读会从
	// 「已发」currentAuditUserInfo 发现新的当前处理人并切换当前处理人——原步重跑。每次重跑都是
	// 独立七阶段、独立落账的一次尝试（一次尝试仍最多一次写请求）。竞签节点一人审批后
	// 实例前进，循环自然退出。上限取场景步数，防御数据异常导致的死循环。
	if session.preview.Action == model.ActionApprove && !outcome.NoMoreSteps && !outcome.DeviationDetected {
		for extra := 0; extra < len(session.runCtx.Steps); extra++ {
			still, probeErr := s.steps.InstanceStillAtStepNode(ctx, session.runCtx, session.runCtx.Steps[session.nextIndex])
			if probeErr != nil || !still {
				break
			}
			extraPreview, extraFinished, buildErr := s.steps.BuildPreview(ctx, session.runCtx, session.nextIndex)
			if buildErr != nil || extraFinished || extraPreview.BlockReason != "" {
				// 探测与预览之间存在竞态（实例恰好前进/被跳过）：跳出循环，交给下方
				// 下一步预览构建按目标真实事实裁决，绝不带着过期预览继续审批。
				break
			}
			extraApproved := step.ApprovedStep{
				RunCtx: session.runCtx, Preview: extraPreview, NextIndex: session.nextIndex,
				Attempt: extra + 2, IsReplay: false, ReportProgress: reporter,
			}
			reporter("submit", "会签：实例仍停在本节点，用新发现的当前处理人继续审批")
			extraOutcome, _, err := s.steps.RunApprovedStep(ctx, extraApproved)
			if err != nil {
				s.sealPostWriteFailure(ctx, pathRunID, session, err)
				result.Outcome = extraOutcome
				return result, err
			}
			if extraOutcome.Verdict != string(verdict.OutcomeSucceeded) {
				// 确定失败或待确认已由执行器落账并收尾：如实返回最后一次结论。
				s.clear(pathRunID)
				result.Outcome = extraOutcome
				return result, nil
			}
			s.mu.Lock()
			session.executedStepNos[extraPreview.StepNo] = true
			s.mu.Unlock()
			outcome = extraOutcome
		}
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
	// 下一步预览的构建依赖目标实时读取（结构、待办、实例事实），共享内网目标存在瞬断；
	// 构建失败若直接放弃会让游标停在本步、后续放行触发重复执行，因此做有界退避重试。
	var preview *step.StepPreview
	var finished bool
	for retry := 0; ; retry++ {
		preview, finished, err = s.steps.BuildPreview(ctx, session.runCtx, nextIndex)
		if err == nil || retry >= 2 {
			break
		}
		time.Sleep(time.Duration(retry+1) * time.Second)
	}
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
	stepInFlight := session != nil && session.stepInFlight
	if session != nil {
		session.stopRequested = true
	}
	s.mu.Unlock()
	if loopRunning || stepInFlight {
		// 循环存活或一步正在执行：落请求事实，由它在本步走完 verify 与 settle 后执行停止。
		// 单步放行与写请求在途的窗口里状态仍是运行中，绝不在这里直接 FinishPathRun——
		// 那会让已发出的写请求返回后无处落账（评审 P1）。
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
	// 多路径串行排队中的等待路径没有在执行任何步骤：停止语义对它是「取消排队」，
	// 直接进入已取消终态并释放队列位（F-020 失败隔离的对称操作）。
	if pathRun.Status == model.PathRunStatusWaiting {
		if _, err := s.runs.AdvancePathRun(ctx, pathRunID,
			model.PathRunStatusWaiting, model.PathRunStatusCancelled, model.RunEvent{
				Kind:  "path_run_cancelled",
				Label: "用户取消排队中的路径运行",
			}); err != nil {
			return model.PathRun{}, err
		}
		if err := s.store.AppendRunControl(ctx, model.RunControl{
			PathRunID: pathRunID, Kind: model.ControlFactStopped,
			Source: model.RunControlSourceUI, CreatedAt: s.now(),
		}, s.now()); err != nil {
			return model.PathRun{}, err
		}
		s.clear(pathRunID)
		pathRun.Status = model.PathRunStatusCancelled
		return pathRun, nil
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
	// 停止生效写 control.log：暂停/停止的完整流水是本切片的完成标准（评审 P1）。
	s.logFact(pathRunID, model.RunControl{
		RunID: pathRun.RunID, PathRunID: pathRunID,
		Kind: model.ControlFactStopped, Source: model.RunControlSourceUI, CreatedAt: s.now(),
	}, 0)
	s.clear(pathRunID)
	return stopped, nil
}

// clear 作废路径运行的内存现场。
func (s *Service) clear(pathRunID uint64) {
	s.mu.Lock()
	delete(s.active, pathRunID)
	s.mu.Unlock()
}

// sealPostWriteFailure 封存写请求已送达但本地后续处理失败的现场。
// 目标副作用已经存在时不清理内存会话、不提供再次放行；数据库收尾失败也只追加提示，优先保证进程内不会重复写。
func (s *Service) sealPostWriteFailure(ctx context.Context, pathRunID uint64, session *activeStep, cause error) {
	s.mu.Lock()
	if session == nil || s.active[pathRunID] != session || session.preview == nil || !session.preview.WriteSent() || session.finished {
		s.mu.Unlock()
		return
	}
	stepNo := session.preview.StepNo
	reason := fmt.Sprintf("执行失败：目标操作已经发出，但执行结果记录失败（第 %d 步）：%s；为避免重复操作，当前运行已停止", stepNo, controlErrorMessage(cause))
	session.finished = true
	session.stopReason = reason
	session.version++
	s.mu.Unlock()

	class := model.FailureClassWriteUncertain
	if _, finishErr := s.runs.Finish(ctx, pathRunID, model.PathRunStatusAwaitingReconciliation, runResultOf(model.RunResultAwaitingReconcile), &class, reason); finishErr != nil {
		s.mu.Lock()
		if s.active[pathRunID] == session {
			session.stopReason += "；运行状态保存失败，已禁止再次放行"
		}
		s.mu.Unlock()
	}
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
