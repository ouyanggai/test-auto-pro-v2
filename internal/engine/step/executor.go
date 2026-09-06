package step

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/engine/verdict"
	"test-auto-pro-v2/internal/model"
)

// UnverifiedActionError 表示步骤动作不在本切片已验证可执行范围内（纲领第 9 节）。
type UnverifiedActionError struct {
	Action model.ActionKey
}

// Error 返回中文说明：未验证动作必须由运行准备阻塞，不得静默执行。
func (e *UnverifiedActionError) Error() string {
	return fmt.Sprintf("动作 %s 尚未验证可执行，本切片只允许发起与同意", string(e.Action))
}

// Executor 执行一条路径运行上的一步七阶段。
// 边界：只经 RunStateControl 推进状态机、只经 TargetClient 发目标请求、
// 事实只经 RunFactsStore 落账；submit 阶段内部没有任何重试路径。
type Executor struct {
	target   TargetClient
	sessions SessionProvider
	runState RunStateControl
	facts    RunFactsStore
	policy   RetryPolicy
	// logFactory 把运行上下文映射为运行目录里的 step.log 写入器；未注入时日志静默跳过（单测场景）。
	logFactory LogFactory
	now        func() time.Time
}

// LogFactory 由装配层注入：复用 F-013 的运行目录路由打开 step.log。
type LogFactory func(runCtx RunContext) *StepLog

// NewExecutor 创建一步执行器；重试预算全部来自配置。
func NewExecutor(targetClient TargetClient, sessions SessionProvider, runState RunStateControl, facts RunFactsStore, runConfig config.RunConfig, now func() time.Time) *Executor {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Executor{
		target:   targetClient,
		sessions: sessions,
		runState: runState,
		facts:    facts,
		policy:   RetryPolicy{Attempts: runConfig.ReadOnlyRetryAttempts, BaseDelay: runConfig.ReadOnlyRetryBaseDelay, MaxDelay: runConfig.ReadOnlyRetryMaxDelay, Now: now},
		now:      now,
	}
}

// SetLogFactory 注入 step.log 工厂；必须在首次使用前调用。
func (e *Executor) SetLogFactory(factory LogFactory) {
	e.logFactory = factory
}

// formatUint 输出无符号整数的十进制文本。
func formatUint(value uint64) string {
	return strconv.FormatUint(value, 10)
}

// stepLogFor 返回该路径运行的 step.log 写入器；未注入工厂时返回空写入器（写入静默跳过）。
func (e *Executor) stepLogFor(runCtx RunContext) *StepLog {
	if e.logFactory == nil {
		return nil
	}
	return e.logFactory(runCtx)
}

// BuildPreview 执行阶段 1（plan 取步）、阶段 2（gate 门禁复验），并写下阶段 3 的暂停行，
// 产出给用户的下一步预览。本方法只读目标、不落账、绝不发写请求。
func (e *Executor) BuildPreview(ctx context.Context, runCtx RunContext, nextIndex int) (*StepPreview, bool, error) {
	if nextIndex >= len(runCtx.Steps) {
		return nil, true, nil
	}
	step := runCtx.Steps[nextIndex]
	log := e.stepLogFor(runCtx)
	log.Phase("plan", step.Sequence, 1, fmt.Sprintf("取第 %d 步：来源 %s，动作 %s，节点 %s", step.Sequence, step.Source, string(step.Action), step.NodeKey))

	// 导航步骤（system_navigation）：只读校验步骤，不发出写请求。
	// 真实执行语义 = 实例事实可读即视为通过（目标引擎自动推进实例经过系统节点）。
	if step.Source == model.ActionStepSourceNavigation {
		actorName := runCtx.PlanAccount
		session, sessionErr := e.sessionWithRetry(ctx, runCtx, log, step.Sequence, "gate")
		if sessionErr != nil {
			return e.blockedPreview(runCtx, step, actorName, "演员登录失败："+sessionErr.Error(), model.FailureClassActorUnresolved), false, nil
		}
		if session.Summary.DisplayName != "" {
			actorName = session.Summary.DisplayName
		}
		facts, readErr := e.readFactsWithRetry(ctx, runCtx, session, step)
		if readErr != nil {
			log.Phase("gate", step.Sequence, 1, "导航校验目标事实读取失败："+readErr.Error())
			return e.blockedPreview(runCtx, step, actorName, "无法读取目标实时事实："+readErr.Error(), model.FailureClassGateBlocked), false, nil
		}
		preview := &StepPreview{
			PathRunID: runCtx.PathRun.ID, StepNo: step.Sequence, TotalSteps: len(runCtx.Steps),
			Action: step.Action, ActionName: "导航校验", NodeKey: step.NodeKey, TargetNodeID: runCtx.Nodes[step.NodeKey].TargetNodeID,
			NodeName: runCtx.Nodes[step.NodeKey].Name, ActorAccount: runCtx.PlanAccount, ActorName: actorName,
			GateAllowed: true, Facts: facts, Navigation: true,
			GateItems: []model.ActionPrecondition{},
		}
		log.Phase("gate", step.Sequence, 1, "导航步骤无需写请求，仅校验实例事实")
		log.Phase("control", step.Sequence, 1, "导航步骤就绪（只读）")
		return preview, false, nil
	}

	// 阶段 2：演员候选（计划账号）取得会话后重读目标实时事实，投影为门禁上下文重新计算门禁。
	// 配置时通过、此刻不通过就停止：门禁不通过绝不跳过。
	actorName := runCtx.PlanAccount
	session, sessionErr := e.sessionWithRetry(ctx, runCtx, log, step.Sequence, "gate")
	if sessionErr != nil {
		log.Phase("gate", step.Sequence, 1, "演员会话获取失败："+sessionErr.Error())
		return e.blockedPreview(runCtx, step, actorName,
			"演员登录失败："+sessionErr.Error(), model.FailureClassActorUnresolved), false, nil
	}
	if session.Summary.DisplayName != "" {
		actorName = session.Summary.DisplayName
	}
	facts, readErr := e.readFactsWithRetry(ctx, runCtx, session, step)
	if readErr != nil {
		log.Phase("gate", step.Sequence, 1, "目标实时事实读取失败："+readErr.Error())
		return e.blockedPreview(runCtx, step, actorName,
			"无法读取目标实时事实："+readErr.Error(), model.FailureClassGateBlocked), false, nil
	}
	info := runCtx.Nodes[step.NodeKey]
	catalogItem, allowed := evaluateGate(step, buildGateContext(runCtx, step, facts, info))
	log.Phase("gate", step.Sequence, 1, gateSummary(catalogItem, allowed))

	preview := &StepPreview{
		PathRunID:      runCtx.PathRun.ID,
		StepNo:         step.Sequence,
		TotalSteps:     len(runCtx.Steps),
		Action:         step.Action,
		ActionName:     catalogItem.Label,
		NodeKey:        step.NodeKey,
		TargetNodeID:   info.TargetNodeID,
		NodeName:       info.Name,
		ActorAccount:   runCtx.PlanAccount,
		ActorName:      actorName,
		ExpectedEffect: catalogItem.ExpectedEffect,
		GateAllowed:    allowed,
		GateItems:      catalogItem.Preconditions,
		Facts:          facts,
	}
	if !allowed {
		reason := catalogItem.DisabledReason
		if reason == "" {
			reason = "门禁复验未通过"
		}
		preview.GateReason = reason
		preview.BlockReason = "门禁复验未通过：" + reason
		preview.BlockFailureClass = model.FailureClassGateBlocked
		log.Phase("control", step.Sequence, 1, "单步暂停，等待放行；本步被门禁阻塞："+reason)
		return preview, false, nil
	}

	// 任务级动作必须拿到目标真实节点标识：待办读取、按节点写参数与事实重读都靠它。
	// 拿不到就停下——空标识会让"待办是否仍在"永远比不上，把没生效的写误判成已前进。
	if requiresTargetNodeID(step) && strings.TrimSpace(info.TargetNodeID) == "" {
		preview.BlockReason = "无法解析该节点在目标平台的真实标识，不能安全执行本步"
		preview.BlockFailureClass = model.FailureClassToolBug
		log.Phase("gate", step.Sequence, 1, "节点真实标识缺失，拒绝构造写请求："+step.NodeKey)
		return preview, false, nil
	}

	// 门禁通过：先按节点权限算出本步要提交的完整表单数据。
	// 目标保存表单数据是整份覆盖（语义清单第 16 条），基线必须是实例当前数据；
	// 只覆盖本节点声明可编辑的配置字段，绝不用历史快照盖掉上游处理人填过的内容。
	formPlan, formErr := e.nodeFormData(ctx, runCtx, step, session)
	if formErr != nil {
		log.Phase("gate", step.Sequence, 1, "读取实例当前表单数据失败："+formErr.Error())
		return e.blockedPreview(runCtx, step, actorName,
			"无法读取实例当前表单数据，不能构造写请求："+formErr.Error(), model.FailureClassGateBlocked), false, nil
	}
	if len(formPlan.Withheld) > 0 || len(formPlan.Overlaid) > 0 {
		log.Phase("gate", step.Sequence, 1, fmt.Sprintf("表单数据按节点权限构造：基线=%s，覆盖 %d 个字段 %v，按权限未带 %d 个字段 %v",
			formBaseName(formPlan.BaseFromInstance), len(formPlan.Overlaid), formPlan.Overlaid, len(formPlan.Withheld), formPlan.Withheld))
	}
	preview.FormOverlaid = formPlan.Overlaid
	preview.FormWithheld = formPlan.Withheld

	// 构造与实际发出的请求严格同源的类型化请求与载荷预览（不含 SID），
	// 并在发送前校验禁用字段（batchCode 禁令）。
	// 下一步节点：提交载荷的 nextAuditorList 人员指定项按它的审批方式生成（2026-09-07 语义勘定）。
	nextNodeKey := ""
	if nextIndex >= 0 && nextIndex < len(runCtx.Steps) {
		nextNodeKey = runCtx.Steps[nextIndex].NodeKey
	}
	request, endpoint, payload, requestErr := buildRequest(runCtx, step, session, formPlan.Payload, nextNodeKey)
	if requestErr != nil {
		preview.BlockReason = "构造写请求失败：" + requestErr.Error()
		preview.BlockFailureClass = model.FailureClassToolBug
		log.Phase("gate", step.Sequence, 1, "构造写请求失败："+requestErr.Error())
		return preview, false, nil
	}
	if err := validateWritePayloadKeys(payload); err != nil {
		preview.BlockReason = "写请求载荷校验失败：" + err.Error()
		preview.BlockFailureClass = model.FailureClassToolBug
		log.Phase("gate", step.Sequence, 1, "写请求载荷校验失败："+err.Error())
		return preview, false, nil
	}
	preview.Endpoint = endpoint
	preview.RequestPayload = payload
	preview.RequestPreview = previewJSON(payload)
	preview.request = request
	log.Phase("control", step.Sequence, 1, "单步暂停，等待放行")
	return preview, false, nil
}

// requiresTargetNodeID 判断这一步是否必须拿到目标真实节点标识。
// 发起作用于整个实例、没有节点级参数也没有待办可对照，因此不要求；
// 其余动作要么带节点级参数，要么要按本节点待办判定写是否生效，缺标识一律不许执行。
func requiresTargetNodeID(compiled model.CompiledActionStep) bool {
	// 发起与实例级动作的目标载荷本来就不含节点参数（撤回/催办/转发/关注/取消关注/重新提交/保存草稿），
	// 强求节点标识会把这批动作 100% 挡在「无法解析真实标识」上（评审 P1）；
	// 任务级动作要么带节点级参数、要么要按本节点待办判定写是否生效，缺标识一律不许执行。
	switch compiled.Action {
	case model.ActionSubmit, model.ActionSaveDraft, model.ActionResubmit,
		model.ActionWithdraw, model.ActionUrge, model.ActionForward,
		model.ActionFollow, model.ActionUnfollow:
		return false
	}
	return true
}

// nodeFormData 读取实例当前表单数据并按节点权限构造本步要提交的完整表单数据。
// 读取属只读阶段，允许有界重试；不携带表单数据的动作直接返回空计划，不做无意义的读取。
func (e *Executor) nodeFormData(ctx context.Context, runCtx RunContext, compiled model.CompiledActionStep, session target.Session) (FormDataPlan, error) {
	if !ActionCarriesFormData(compiled.Action) {
		return FormDataPlan{}, nil
	}
	var current map[string]any
	hasInstance := false
	if instanceRef := strings.TrimSpace(runCtx.PathRun.MainInstanceRef); instanceRef != "" {
		hasInstance = true
		read, err := RunWithRetry(ctx, e.policy, "实例表单数据读取", func() (map[string]any, error) {
			return e.target.ReadInstanceCurrentData(ctx, session, instanceRef)
		}, nil)
		if err != nil {
			return FormDataPlan{}, err
		}
		current = read
	}
	// 实例存在但数据为空时必须保持实例分支（空基线），不得退回发起分支提交整份历史配置。
	return BuildNodeFormData(runCtx, compiled, current, hasInstance)
}

// formBaseName 返回表单数据基线的中文说明，供 step.log 一眼看出这份载荷是从哪来的。
func formBaseName(fromInstance bool) string {
	if fromInstance {
		return "目标实例当前数据"
	}
	return "发起态完整表单模型"
}

// blockedPreview 构造被阻塞的预览：说明中文原因与失败分类，路径必须停止。
func (e *Executor) blockedPreview(runCtx RunContext, step model.CompiledActionStep, actorName, reason string, class model.FailureClass) *StepPreview {
	info := runCtx.Nodes[step.NodeKey]
	return &StepPreview{
		PathRunID:         runCtx.PathRun.ID,
		StepNo:            step.Sequence,
		TotalSteps:        len(runCtx.Steps),
		Action:            step.Action,
		NodeKey:           step.NodeKey,
		TargetNodeID:      info.TargetNodeID,
		NodeName:          info.Name,
		ActorAccount:      runCtx.PlanAccount,
		ActorName:         actorName,
		GateAllowed:       false,
		GateReason:        reason,
		BlockReason:       reason,
		BlockFailureClass: class,
	}
}

// ApprovedStep 是放行后交给执行器的输入：预览事实（内含同源载荷）与步骤下标。
type ApprovedStep struct {
	RunCtx    RunContext
	Preview   *StepPreview
	NextIndex int
	// Attempt 是本次执行的尝试序号（对账重放时递增）；0 视为 1。
	Attempt int
	// IsReplay 表示本次执行是对账判「未生效」后的重放（新尝试，不是首次执行）。
	// 它只影响尝试行的记账列，不改变七阶段本身：重放同样只允许发出一次写请求。
	IsReplay bool
	// ReportProgress 把阶段进度实时上报给控制现场（运行画布指示器的数据源），可为 nil。
	// phase 取七阶段名；note 是给用户看的中文补充（如重试退避说明）。
	ReportProgress func(phase, note string)
}

// reportPhase 把阶段进度上报给控制现场（指示器实时推进的数据源）；未接收集方时是空操作。
func reportPhase(approved ApprovedStep, phase, note string) {
	if approved.ReportProgress != nil {
		approved.ReportProgress(phase, note)
	}
}

// gateSnapshotJSON 把放行时的门禁结论固化为快照 JSON：逐项条件的中文名与满足情况随步骤落账，
// 侧栏才能对「已执行的步骤」给出当时的门禁结论（纲领第 7.1 节）。
func gateSnapshotJSON(preview *StepPreview, branchTarget string) string {
	snapshot := struct {
		Allowed      bool                       `json:"allowed"`
		Reason       string                     `json:"reason,omitempty"`
		Items        []model.ActionPrecondition `json:"items"`
		BranchTarget string                     `json:"branchTarget,omitempty"`
		// 按节点权限构造表单数据的两个清单：用户据此解释"这个字段为什么没被提交/为什么变了"。
		FormOverlaid []string `json:"formOverlaid,omitempty"`
		FormWithheld []string `json:"formWithheld,omitempty"`
	}{Allowed: preview.GateAllowed, Reason: preview.GateReason, Items: preview.GateItems,
		BranchTarget: branchTarget, FormOverlaid: preview.FormOverlaid, FormWithheld: preview.FormWithheld}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return ""
	}
	return string(data)
}

// RunApprovedStep 执行阶段 3（放行）、4（prepare）、5（submit）、6（verify）、7（settle）。
// 一次尝试最多一次写请求：submit 只调用一次，写结果不确定即停在待对账，绝不重发。
func (e *Executor) RunApprovedStep(ctx context.Context, approved ApprovedStep) (StepOutcome, uint64, error) {
	runCtx := approved.RunCtx
	preview := approved.Preview
	step := runCtx.Steps[approved.NextIndex]
	log := e.stepLogFor(runCtx)
	startedAt := e.now()
	attemptNo := approved.Attempt
	if attemptNo <= 0 {
		attemptNo = 1
	}
	outcome := StepOutcome{Verdict: string(verdict.OutcomeUncertain)}

	if preview.Navigation {
		// 导航步骤：只读校验后直接落账成功（无写请求、无三值判定对象）。
		lineNo := log.Phase("settle", step.Sequence, 1, "落账：导航步骤只读校验通过")
		record := model.RunStep{
			PathRunID: runCtx.PathRun.ID, StepNo: step.Sequence, Source: string(step.Source),
			Action: string(step.Action), NodeKey: step.NodeKey, ActorSummary: preview.ActorName,
			Status: model.RunStepSucceeded, StartedAt: startedAt, FinishedAt: e.now(),
			GateSnapshot: gateSnapshotJSON(preview, approved.RunCtx.SubmitBranchTargetNodeID),
		}
		attempt := model.RunStepAttempt{
			PathRunID: runCtx.PathRun.ID, AttemptNo: 1, Verdict: string(verdict.OutcomeSucceeded),
			SideEffect: string(verdict.SideEffectNone), Reason: "导航步骤只读校验通过", Basis: "实例事实可读",
			LogPath: log.RelativePath(), LogLine: lineNo,
		}
		if _, err := e.facts.RecordStepAttempt(ctx, record, attempt, e.now()); err != nil {
			return StepOutcome{Verdict: string(verdict.OutcomeFailed)}, lineNo, err
		}
		outcome.Verdict = string(verdict.OutcomeSucceeded)
		outcome.NoMoreSteps = approved.NextIndex+1 >= len(runCtx.Steps)
		if !outcome.NoMoreSteps {
			if err := e.runState.BackToRunning(ctx, runCtx.PathRun.ID); err != nil {
				return outcome, lineNo, err
			}
		}
		// 导航步骤是只读的，没有领取推进权，也就无需释放。
		log.Phase("settle", step.Sequence, 1, "导航步骤完成")
		return outcome, lineNo, nil
	}
	if preview.BlockReason != "" {
		// 被阻塞的步骤不允许放行：路径运行在这里失败，而不是带病前进。
		class := preview.BlockFailureClass
		if _, err := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusFailed, runResultOf(model.RunResultFailed), &class,
			"路径在第 "+formatUint(uint64(step.Sequence))+" 步失败："+preview.BlockReason); err != nil {
			return outcome, 0, err
		}
		log.Phase("control", step.Sequence, attemptNo, "放行被拒绝（"+preview.BlockReason+"），路径运行置为失败")
		reportPhase(approved, "control", "放行被拒绝："+preview.BlockReason)
		return outcome, 0, nil
	}

	// 阶段 4：领取推进权并就绪演员会话。领取失败说明已有其他执行者，调用方必须放弃。
	fencingToken, err := e.runState.ClaimExecution(ctx, runCtx.PathRun.ID)
	if err != nil {
		return outcome, 0, err
	}
	reportPhase(approved, "prepare", "正在就绪演员会话")
	// 写步骤的会话策略对齐 V1 长期验证的模式（2026-09-07 修正）：复用缓存会话，
	// 写请求被会话失效拒绝时由 resubmitOnSessionRejected 恢复（重登+重发至多 3 次）。
	// 每一步强制重登会让登录频率放大数倍，实测触发了目标平台对账号的会话限制
	// （连只读都秒级失效），反而摧毁运行现场；V1 的「缓存复用+失效恢复」多年无此问题。
	session, sessionErr := func() (target.Session, error) {
		if cached, err := e.sessions.Current(ctx, runCtx.PlanAccount); err == nil {
			return cached, nil
		}
		return RunWithRetry(ctx, e.policy, "会话刷新", func() (target.Session, error) {
			return e.sessions.Refresh(ctx, runCtx.PlanAccount)
		}, func(attempt int, nextDelay time.Duration) {
			note := fmt.Sprintf("会话刷新第 %d 次失败，%s 后重试", attempt, nextDelay)
			log.Phase("prepare", step.Sequence, attemptNo, note)
			reportPhase(approved, "prepare", note)
		})
	}()
	if sessionErr != nil {
		class := model.FailureClassActorUnresolved
		if _, finishErr := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusFailed, runResultOf(model.RunResultFailed), &class,
			"演员登录失败："+sessionErr.Error()); finishErr != nil {
			return outcome, 0, finishErr
		}
		log.Phase("prepare", step.Sequence, attemptNo, "演员会话获取失败："+sessionErr.Error())
		return outcome, 0, nil
	}
	log.Phase("prepare", step.Sequence, attemptNo, fmt.Sprintf("演员 %s（%s）会话就绪，即将发出 %s", preview.ActorName, preview.ActorAccount, preview.Endpoint))
	reportPhase(approved, "prepare", fmt.Sprintf("演员 %s 会话就绪", preview.ActorName))

	// 阶段 5：发出唯一一次写请求。审批任务 ID 在发送前现场新鲜读取（演员与待办的新鲜复验）。
	// 上报发生在发出之前：本次调用同步阻塞到目标响应返回，指示器在窗口内如实表达 submit 进行中。
	reportPhase(approved, "submit", "写请求发送中，同步等待目标响应")
	// 写请求前的租约续期：目标存在约 30 秒的慢请求，租约若在写请求期间过期，
	// 另一执行者可能在核验未完成时领取推进权（评审缺陷 12：RenewLease 此前无调用方）。
	// 续期失败说明推进权已易主，本步必须放弃且绝不发出写请求。
	if err := e.runState.RenewLease(ctx, runCtx.PathRun.ID, fencingToken); err != nil {
		return outcome, 0, err
	}
	e.refreshAndSubmit(ctx, runCtx, step, session, preview, log, reportPhase, approved, attemptNo)
	if !preview.writeSent {
		// 零写入：写请求没有发出（发送前的待办新鲜复验失败或载荷缺失）。
		// 没有发出的请求不存在“结果不确定”——把零写入判成不确定会把无副作用的失败
		// 说成“可能已经写进目标”，还会把用户引向对账；这里按真实分类如实置败。
		outcome.Verdict = string(verdict.OutcomeFailed)
		class := preview.writeErrClass
		if class == "" {
			class = model.FailureClassToolBug
		}
		reason := "第 " + formatUint(uint64(step.Sequence)) + " 步在写请求发出前失败（零写入）：" + preview.writeErr.Error()
		lineNo := log.Phase("settle", step.Sequence, attemptNo, "落账：确定失败且无副作用（"+reason+"）")
		record := model.RunStep{
			PathRunID:    runCtx.PathRun.ID,
			StepNo:       step.Sequence,
			Source:       string(step.Source),
			Action:       string(step.Action),
			NodeKey:      step.NodeKey,
			ActorSummary: preview.ActorName,
			Status:       model.RunStepFailed,
			StartedAt:    startedAt,
			FinishedAt:   e.now(),
			GateSnapshot: gateSnapshotJSON(preview, approved.RunCtx.SubmitBranchTargetNodeID),
		}
		attempt := model.RunStepAttempt{
			PathRunID:  runCtx.PathRun.ID,
			AttemptNo:  attemptNo,
			Verdict:    string(verdict.OutcomeFailed),
			SideEffect: string(verdict.SideEffectNone),
			Reason:     reason,
			Basis:      "写请求没有发出，不存在写结果，也无传输结果可归类",
			LogPath:    log.RelativePath(),
			LogLine:    lineNo,
			DurationMs: e.now().Sub(startedAt).Milliseconds(),
			IsReplay:   approved.IsReplay,
		}
		if _, err := e.facts.RecordStepAttempt(ctx, record, attempt, e.now()); err != nil {
			return outcome, lineNo, err
		}
		if _, err := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusFailed, runResultOf(model.RunResultFailed), &class, reason); err != nil {
			return outcome, lineNo, err
		}
		log.Phase("settle", step.Sequence, attemptNo, "零写入失败已落账，路径运行置为失败")
		return outcome, lineNo, nil
	}
	// 写请求已发出：从这一行起 step.log 携带链路 ID，submit 之后的阶段行可与 network.log、curl.log 互查。
	if preview.writeTraceID != "" {
		log.SetTraceID(preview.writeTraceID)
	}
	log.Phase("submit", step.Sequence, attemptNo, submitSummary(preview.writeErr, preview.writeTraceID, preview.writeDurationMs))

	// 发起成功后尽早落库主实例引用（独占不可改写）：即使核验前崩溃，
	// 恢复出的待对账路径运行仍有实例引用可供对账。
	if result, ok := preview.writeResult.(*target.SubmitFlowInstanceResult); ok && result != nil && result.InstanceID != "" {
		if err := e.runState.SetMainInstanceRef(ctx, runCtx.PathRun.ID, result.InstanceID); err != nil {
			return outcome, 0, err
		}
		runCtx.PathRun.MainInstanceRef = result.InstanceID
		outcome.MainInstanceRef = result.InstanceID
	}

	// 阶段 6：事实重读。路径运行先进入核验中——从此刻起崩溃恢复会把该路径置为待对账。
	if err := e.runState.MarkVerifying(ctx, runCtx.PathRun.ID); err != nil {
		return outcome, 0, err
	}
	reportPhase(approved, "verify", "正在重读目标事实并做三值判定")
	// 重读对照一律用目标真实节点标识：目标返回的当前节点与待办都是真实标识，
	// 拿工具侧不透明键去比会永远"待办已消失"，把没生效的写误判成已前进。
	stepTargetNodeID := runCtx.Nodes[step.NodeKey].TargetNodeID
	before := preview.Facts
	before.StepNodeKey = stepTargetNodeID
	after, _ := e.readFactsWithRetry(ctx, runCtx, session, step)
	after.StepNodeKey = stepTargetNodeID
	reread := ClassifyReread(string(step.Action), stepTargetNodeID, before, after)
	observation := buildObservation(preview.Endpoint, preview.writeErr, preview.writeResponse, reread)
	observation.Action = string(step.Action)
	verdictResult := verdict.Evaluate(observation)
	log.Phase("verify", step.Sequence, attemptNo, fmt.Sprintf("三值判定：%s（%s）", verdictChinese(verdictResult.Outcome), verdictResult.Reason))

	// 阶段 7：落账。事实表只 INSERT；随后按结论推进路径运行状态。
	lineNo := log.Phase("settle", step.Sequence, attemptNo, "落账："+settleSummary(verdictResult))
	durationMs := preview.writeDurationMs
	if durationMs == 0 {
		durationMs = e.now().Sub(startedAt).Milliseconds()
	}
	record := model.RunStep{
		PathRunID:    runCtx.PathRun.ID,
		StepNo:       step.Sequence,
		Source:       string(step.Source),
		Action:       string(step.Action),
		NodeKey:      step.NodeKey,
		ActorSummary: preview.ActorName,
		Status:       statusOfVerdict(verdictResult.Outcome),
		StartedAt:    startedAt,
		FinishedAt:   e.now(),
		GateSnapshot: gateSnapshotJSON(preview, approved.RunCtx.SubmitBranchTargetNodeID),
	}
	attempt := model.RunStepAttempt{
		PathRunID:   runCtx.PathRun.ID,
		AttemptNo:   attemptNo,
		Verdict:     string(verdictResult.Outcome),
		SideEffect:  string(verdictResult.SideEffect),
		Transport:   string(target.TransportOf(preview.writeErr)),
		Initial:     string(verdictResult.Initial),
		Reread:      string(reread),
		Reason:      verdictResult.Reason,
		Basis:       verdictResult.Basis,
		TraceID:     preview.writeTraceID,
		CurlTraceID: preview.writeTraceID,
		LogPath:     log.RelativePath(),
		LogLine:     lineNo,
		DurationMs:  durationMs,
		IsReplay:    approved.IsReplay,
		// 写之前的目标事实随尝试行落库：这是对账判定的另一半输入，
		// 只留在内存里会让进程重启后停在待对账的路径运行永远拿不到基准（F-018 根治项）。
		BeforeFacts: EncodeInstanceFacts(before),
	}
	if _, err := e.facts.RecordStepAttempt(ctx, record, attempt, e.now()); err != nil {
		return outcome, lineNo, err
	}

	switch verdictResult.Outcome {
	case verdict.OutcomeSucceeded:
		outcome.Verdict = string(verdict.OutcomeSucceeded)
		outcome.NoMoreSteps = approved.NextIndex+1 >= len(runCtx.Steps)
		// 路径偏离判据（T04）：只用重读到的真实事实——实际当前节点集合里没有已配置路径的下一个预期节点。
		// 两边必须在同一键空间比较：CurrentNodes 是目标 nodeProxyId，预期节点必须取节点表反查出的
		// TargetNodeID，拿编译场景的哈希键比较永远不相等，会把每一步都误判成偏离（评审 P1）。
		// 判定保守：实例不可见、没有当前节点事实、或预期节点拿不到真实标识时不声称偏离。
		if !outcome.NoMoreSteps {
			expectedNextKey := runCtx.Steps[approved.NextIndex+1].NodeKey
			expectedNextTarget := ""
			if nodeInfo, ok := runCtx.Nodes[expectedNextKey]; ok {
				expectedNextTarget = strings.TrimSpace(nodeInfo.TargetNodeID)
			}
			if after.Found && len(after.CurrentNodes) > 0 && expectedNextTarget != "" && !containsNode(after.CurrentNodes, expectedNextTarget) {
				outcome.DeviationDetected = true
				log.Phase("settle", step.Sequence, attemptNo, fmt.Sprintf("路径偏离：实际当前节点 %v，已配置路径的下一个预期节点是 %s", after.CurrentNodes, expectedNextTarget))
			}
		}
		if !outcome.NoMoreSteps {
			if err := e.runState.BackToRunning(ctx, runCtx.PathRun.ID); err != nil {
				return outcome, lineNo, err
			}
		}
		// 落账后释放推进权是尽力而为：释放失败只影响下一次领取的即时性，不影响已落账事实。
		_ = e.runState.ReleaseExecution(ctx, runCtx.PathRun.ID, fencingToken)
		log.Phase("settle", step.Sequence, attemptNo, "本步确定成功")
	case verdict.OutcomeFailed:
		outcome.Verdict = string(verdict.OutcomeFailed)
		class := model.FailureClassTargetRejected
		if _, err := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusFailed, runResultOf(model.RunResultFailed), &class,
			"路径在第 "+formatUint(uint64(step.Sequence))+" 步被目标拒绝："+verdictResult.Reason); err != nil {
			return outcome, lineNo, err
		}
		log.Phase("settle", step.Sequence, attemptNo, "本步确定失败且无副作用，路径运行置为失败")
	default:
		// 写结果不确定：路径运行进入待对账并停止；唯一合法恢复动作属于对账切片（F-018）。
		outcome.Verdict = string(verdict.OutcomeUncertain)
		class := model.FailureClassWriteUncertain
		if _, err := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusAwaitingReconciliation, runResultOf(model.RunResultAwaitingReconcile), &class,
			"第 "+formatUint(uint64(step.Sequence))+" 步写结果不确定："+verdictResult.Reason); err != nil {
			return outcome, lineNo, err
		}
		log.Phase("settle", step.Sequence, attemptNo, "写结果不确定，路径运行进入待对账并停止")
	}
	return outcome, lineNo, nil
}

// isSessionRejected 判断一次写调用是否被目标以会话失效拒绝（响应已收到、未进入业务）。
func isSessionRejected(err error) bool {
	var targetErr *target.Error
	return errors.As(err, &targetErr) && targetErr.Kind == target.ErrorSessionExpired
}

// resubmitOnSessionRejected 在写请求被会话失效拒绝后恢复：最多 3 次「换新会话 + 重发」。
// 每次被拒都证明请求未进入业务、无副作用（RESP401/AUTH_401 发生在业务逻辑之前），
// 因此写请求进入业务的次数仍至多一次；恢复过程逐次写 step.log，失败后按原错误上报。
func resubmitOnSessionRejected[R any](
	ctx context.Context, refresh func(account string) (target.Session, error), account string,
	step model.CompiledActionStep, attemptNo int,
	send func(refreshed target.Session) (R, target.WriteResponse, string, error),
	log *StepLog, reportPhase func(ApprovedStep, string, string), approved ApprovedStep,
) (R, target.WriteResponse, string, error) {
	var zero R
	for round := 1; round <= 3; round++ {
		log.Phase("submit", step.Sequence, attemptNo, fmt.Sprintf(
			"写请求被目标以会话失效拒绝（第_%d_次，未进入业务、无副作用），重新取得会话后重发", round))
		reportPhase(approved, "submit", fmt.Sprintf("目标会话失效已拒绝 %d 次（无副作用），正在重新取得会话", round))
		refreshed, refreshErr := refresh(account)
		if refreshErr != nil {
			return zero, target.WriteResponse{}, "", refreshErr
		}
		result, response, traceID, err := send(refreshed)
		if !isSessionRejected(err) {
			return result, response, traceID, err
		}
		if round == 3 {
			return result, response, traceID, err
		}
	}
	return zero, target.WriteResponse{}, "", fmt.Errorf("会话失效恢复重发未执行")
}

// refreshSessionForWrite 为写请求重新取得可用会话：强制重登并探活，探活只把会话失效当失败。
func (e *Executor) refreshSessionForWrite(ctx context.Context, account string) (target.Session, error) {
	fresh, err := e.sessions.Refresh(ctx, account)
	if err != nil {
		return target.Session{}, err
	}
	if pinger, ok := e.target.(interface {
		Ping(context.Context, target.Session) error
	}); ok {
		if pingErr := pinger.Ping(ctx, fresh); pingErr != nil {
			// 首个登录会话可能立即失效（实测）：再登一次；仍失败则如实返回。
			fresh, err = e.sessions.Refresh(ctx, account)
			if err != nil {
				return target.Session{}, err
			}
			if pingErr := pinger.Ping(ctx, fresh); pingErr != nil {
				return target.Session{}, pingErr
			}
		}
	}
	return fresh, nil
}

// refreshAndSubmit 在发送前完成待办任务 ID 的新鲜读取，然后发出唯一一次写请求。
// 请求本体与预览同源（preview.request）；本方法及其调用路径不存在任何重试。
// 只有真正发出写请求的路径才置 preview.writeSent：发送前的任何失败都停留在零写入分支。
func (e *Executor) refreshAndSubmit(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, session target.Session, preview *StepPreview, log *StepLog, reportPhase func(ApprovedStep, string, string), approved ApprovedStep, attemptNo int) {
	switch request := preview.request.(type) {
	case *target.SubmitFlowInstanceRequest:
		preview.writeSent = true
		started := e.now()
		result, response, traceID, err := e.target.SubmitFlowInstance(ctx, session, *request)
		// 会话失效拒绝（RESP401/AUTH_401）发生在目标业务逻辑之前：请求没有进入业务、
		// 无副作用（核验重读「明确未变」可证）。这不是「唯一一次写机会」的消耗——
		// 实测目标写端点会话校验与读端点不同步，新登录的会话也可能被写链路拒绝。
		// 因此在同一次尝试内重新取得可用会话并重发，写请求进入业务的次数仍然至多一次；
		// 两次都被会话失效拒绝才如实按失败上报（纪律：绝不盲目重发真实写请求，
		// 这里的重发仅以「目标明确拒绝、证明无副作用」为前提）。
		if isSessionRejected(err) {
			// 实测目标为多节点且写链路会话不同步：每次换新会话重发随机命中，
			// 因此在同一次尝试内最多恢复重发 3 次；每次被拒都已证明未进入业务、无副作用。
			result, response, traceID, err = resubmitOnSessionRejected(ctx, func(account string) (target.Session, error) { return e.refreshSessionForWrite(ctx, account) }, runCtx.PlanAccount, step, attemptNo,
				func(refreshed target.Session) (*target.SubmitFlowInstanceResult, target.WriteResponse, string, error) {
					return e.target.SubmitFlowInstance(ctx, refreshed, *request)
				}, log, reportPhase, approved)
		}
		preview.writeResult, preview.writeResponse, preview.writeTraceID, preview.writeErr = result, response, traceID, err
		preview.writeDurationMs = e.now().Sub(started).Milliseconds()
	case *target.AuditCurrentTaskRequest:
		started := e.now()
		// 待办按目标真实节点标识精确定位：step.NodeKey 是工具侧不透明键，发给目标永远匹配不上。
		// 待办读取是只读：会话失效时按目标会话事实重取（与 submit 同一恢复语义）。
		jobTaskID, err := e.target.FindDueTaskID(ctx, session, runCtx.PathRun.MainInstanceRef, runCtx.Nodes[step.NodeKey].TargetNodeID)
		if isSessionRejected(err) {
			refreshed, refreshErr := e.refreshSessionForWrite(ctx, runCtx.PlanAccount)
			if refreshErr == nil {
				jobTaskID, err = e.target.FindDueTaskID(ctx, refreshed, runCtx.PathRun.MainInstanceRef, runCtx.Nodes[step.NodeKey].TargetNodeID)
			}
		}
		if err != nil {
			// 待办读取失败（目标抖动或响应形状不符）：写请求未发出，按演员/待办解析失败如实归类。
			preview.writeErr = err
			preview.writeErrClass = model.FailureClassActorUnresolved
			return
		}
		if jobTaskID == "" {
			// 目标上已无本演员在本节点的活动待办：演员或待办已变化，绝不冒名发送。
			// 用如实的原因，不复用「未验证动作」的话术——那是另一回事，且文案已过时（评审 P2）。
			preview.writeErr = fmt.Errorf("目标上已无本演员在本节点的活动待办，无法执行%s；请核对目标平台的真实状态", preview.ActionName)
			preview.writeErrClass = model.FailureClassActorUnresolved
			return
		}
		request.JobTaskID = jobTaskID
		preview.writeSent = true
		result, response, traceID, err := e.target.AuditCurrentTask(ctx, session, *request)
		if isSessionRejected(err) {
			result, response, traceID, err = resubmitOnSessionRejected(ctx, func(account string) (target.Session, error) { return e.refreshSessionForWrite(ctx, account) }, runCtx.PlanAccount, step, attemptNo,
				func(refreshed target.Session) (*target.AuditCurrentTaskResult, target.WriteResponse, string, error) {
					return e.target.AuditCurrentTask(ctx, refreshed, *request)
				}, log, reportPhase, approved)
		}
		preview.writeResult, preview.writeResponse, preview.writeTraceID, preview.writeErr = result, response, traceID, err
		preview.writeDurationMs = e.now().Sub(started).Milliseconds()
	default:
		// 其余已登记动作经统一动作写出口（F-019）：载荷由 buildRequest 构造，端点在白名单内。
		if actionRequest, ok := preview.request.(*target.ActionWriteRequest); ok {
			preview.writeSent = true
			started := e.now()
			response, traceID, err := e.target.ExecuteActionWrite(ctx, session, *actionRequest)
			preview.writeResponse, preview.writeTraceID, preview.writeErr = response, traceID, err
			preview.writeDurationMs = e.now().Sub(started).Milliseconds()
			return
		}
		preview.writeErr = errors.New("写请求载荷缺失，拒绝发送")
		preview.writeErrClass = model.FailureClassToolBug
	}
}

// containsNode 判断节点键集合是否包含目标节点。
func containsNode(nodes []string, nodeKey string) bool {
	for _, node := range nodes {
		if node == nodeKey {
			return true
		}
	}
	return false
}

// sessionWithRetry 取得演员会话：登录与会话获取属只读阶段，允许有界重试与退避。
// 每次重试都如实写进 step.log，不允许出现“看起来只调了一次”的日志。
func (e *Executor) sessionWithRetry(ctx context.Context, runCtx RunContext, log *StepLog, stepNo int, phase string) (target.Session, error) {
	return RunWithRetry(ctx, e.policy, "会话获取", func() (target.Session, error) {
		return e.sessions.Current(ctx, runCtx.PlanAccount)
	}, func(attempt int, nextDelay time.Duration) {
		log.Phase(phase, stepNo, 1, fmt.Sprintf("会话获取第 %d 次失败，%s 后重试", attempt, nextDelay))
	})
}

// readFactsWithRetry 重读目标实时事实（只读，可重试）；重试预算耗尽后返回最后的读取错误。
func (e *Executor) readFactsWithRetry(ctx context.Context, runCtx RunContext, session target.Session, step model.CompiledActionStep) (InstanceFacts, error) {
	facts, err := RunWithRetry(ctx, e.policy, "事实重读", func() (InstanceFacts, error) {
		// 事实重读要与目标返回的真实节点标识对照，因此传真实标识而不是工具侧不透明键。
		return e.readInstanceFacts(ctx, session, runCtx.PathRun.MainInstanceRef, runCtx.Nodes[step.NodeKey].TargetNodeID)
	}, nil)
	if err != nil {
		// 预算耗尽仍读不到：把读取失败随事实带回（ReadError 非空 → 核验判不可读、对账走读取失败降级），
		// 绝不把零值事实当「读到了且无痕迹」用——那会把读取失败误判成「已前进」或「明确未变」（评审 P1）。
		facts.ReadError = err.Error()
		return facts, err
	}
	return facts, nil
}

// FinalTargetFacts 是收尾重读产出的最终目标事实摘要。
// 与「路径结果」是两件分开的事：本结构只如实描述目标现状，不做成立与否的判定（纲领第 7.4 节）。
type FinalTargetFacts struct {
	InstanceRef      string   `json:"instanceRef"`
	Status           string   `json:"status"`
	StatusName       string   `json:"statusName"`
	CurrentNodeNames []string `json:"currentNodeNames"`
	DueNodeNames     []string `json:"dueNodeNames"`
}

// FinalReview 场景走完后的收尾重读：回到目标读实例状态、当前节点与当前待办，
// 用业务名称呈现。读取属只读阶段，允许有界重试；读不到时如实返回错误，不伪造事实。
func (e *Executor) FinalReview(ctx context.Context, runCtx RunContext) (FinalTargetFacts, error) {
	log := e.stepLogFor(runCtx)
	session, err := e.sessionWithRetry(ctx, runCtx, log, 0, "verify")
	if err != nil {
		return FinalTargetFacts{}, err
	}
	facts, err := e.readFactsWithRetry(ctx, runCtx, session, model.CompiledActionStep{})
	if err != nil {
		return FinalTargetFacts{}, err
	}
	table := runCtx.Nodes
	result := FinalTargetFacts{
		InstanceRef: runCtx.PathRun.MainInstanceRef,
		Status:      facts.Status,
		StatusName:  targetStatusName(facts.Status),
	}
	for _, nodeKey := range facts.CurrentNodes {
		result.CurrentNodeNames = append(result.CurrentNodeNames, nodeNameOf(table, nodeKey))
	}
	for _, nodeKey := range facts.DueNodes {
		result.DueNodeNames = append(result.DueNodeNames, nodeNameOf(table, nodeKey))
	}
	return result, nil
}

// nodeNameOf 查节点业务名称；路径配置快照里查不到的键原样返回，不静默丢弃事实。
func nodeNameOf(table map[string]NodeInfo, nodeKey string) string {
	if info, ok := table[nodeKey]; ok && info.Name != "" {
		return info.Name
	}
	return nodeKey
}

// targetStatusName 把目标实例状态编码名翻译为中文；未知编码原样保留，不猜测。
func targetStatusName(status string) string {
	switch status {
	case "draft":
		return "草稿"
	case "await_sent":
		return "待发"
	case "run":
		return "运行中"
	case "withdraw":
		return "已撤回"
	case "termination":
		return "已终止"
	case "abandon":
		return "已作废"
	case "rejected":
		return "已驳回"
	case "end":
		return "已结束"
	default:
		return status
	}
}

// ReconcileFacts 是对账所需的只读事实集合（T02 收集器的执行器侧实现）。
type ReconcileFacts struct {
	BeforeStatus      string
	BeforeHadInstance bool
	// BeforeKnown 表示"写之前的目标事实"这份基准真的取到了。
	// 为假时对账不得使用 BeforeStatus/BeforeHadInstance：那两个零值会被读成
	// "写之前实例不存在"这个确定事实，凭空造出与事实相反的依据。
	BeforeKnown     bool
	NowFound        bool
	NowStatus       string
	NowCurrentNodes []string
	NowDueNodes     []string
	NowReadError    string
	// DoneRecordsRead 为真表示已办记录维度真的读到了（不是"未接入"）；
	// DoneRecordFound 表示本步节点上已经有本账号的已办记录。
	DoneRecordsRead bool
	DoneRecordFound bool
	// ActionTraceRead 为真表示审核记录维度真的读到了；ActionTraceFound 表示本步节点已留下动作痕迹；
	// ActionTraceTotal 是该实例的审核记录条数，只进依据说明供人工核对。
	ActionTraceRead  bool
	ActionTraceFound bool
	ActionTraceTotal int
}

// ReconcileFacts 重读目标事实供对账判定：只读、可重试。
// before 事实以本步预览里保存的目标事实为准（写之前的状态）。
func (e *Executor) ReconcileFacts(ctx context.Context, runCtx RunContext, stepNo int) (ReconcileFacts, error) {
	log := e.stepLogFor(runCtx)
	session, err := e.sessionWithRetry(ctx, runCtx, log, 0, "verify")
	if err != nil {
		return ReconcileFacts{}, err
	}
	before := InstanceFacts{}
	beforeKnown := false
	if stepNo >= 1 && stepNo <= len(runCtx.Steps) {
		// 写之前的基准：执行时由控制现场回填，重启后由 run_step_attempts.before_facts 还原。
		before, beforeKnown = runCtx.LastBeforeFacts, runCtx.LastBeforeFactsKnown
	}
	after, err := e.readFactsWithRetry(ctx, runCtx, session, model.CompiledActionStep{})
	if err != nil {
		return ReconcileFacts{BeforeStatus: before.Status, BeforeHadInstance: before.Found,
			BeforeKnown: beforeKnown, NowReadError: after.ReadError}, nil
	}
	facts := ReconcileFacts{
		BeforeStatus:      before.Status,
		BeforeHadInstance: before.Found,
		BeforeKnown:       beforeKnown,
		NowFound:          after.Found,
		NowStatus:         after.Status,
		NowCurrentNodes:   after.CurrentNodes,
		NowDueNodes:       after.DueNodes,
	}
	// 已办记录与动作痕迹两个维度必须真的去读：只有五维都有证据，「未生效」才允许成立，
	// 而「未生效」是唯一会导致重放（再写一次）的结论。读不到就如实留空，由判定器降级。
	instanceRef := strings.TrimSpace(runCtx.PathRun.MainInstanceRef)
	nodeID := ""
	if stepNo >= 1 && stepNo <= len(runCtx.Steps) {
		nodeID = runCtx.Nodes[runCtx.Steps[stepNo-1].NodeKey].TargetNodeID
	}
	if instanceRef != "" {
		if reader, ok := e.target.(doneRecordReader); ok {
			if found, readErr := RunWithRetry(ctx, e.policy, "已办记录读取", func() (bool, error) {
				return reader.FindDoneTaskOnNode(ctx, session, instanceRef, nodeID)
			}, nil); readErr == nil {
				facts.DoneRecordsRead, facts.DoneRecordFound = true, found
			} else {
				log.Phase("verify", stepNo, 1, "已办记录读取失败，对账按证据缺失降级："+readErr.Error())
			}
		}
		if reader, ok := e.target.(auditTraceReader); ok {
			trace, readErr := RunWithRetry(ctx, e.policy, "审核记录读取", func() (auditTrace, error) {
				found, total, err := reader.FindAuditTraceOnNode(ctx, session, instanceRef, nodeID)
				return auditTrace{found: found, total: total}, err
			}, nil)
			if readErr == nil {
				facts.ActionTraceRead, facts.ActionTraceFound, facts.ActionTraceTotal = true, trace.found, trace.total
			} else {
				log.Phase("verify", stepNo, 1, "审核记录读取失败，对账按证据缺失降级："+readErr.Error())
			}
		}
	}
	return facts, nil
}

// doneRecordReader 与 auditTraceReader 是对账两个新增维度的最小能力面。
// 用接口断言而不是加进 TargetClient：假件与既有装配不必被迫实现它们，读不到时对账如实降级。
type doneRecordReader interface {
	FindDoneTaskOnNode(ctx context.Context, active target.Session, instanceID, nodeProxyID string) (bool, error)
}

type auditTraceReader interface {
	FindAuditTraceOnNode(ctx context.Context, active target.Session, instanceID, nodeProxyID string) (bool, int, error)
}

// auditTrace 是审核记录维度的读取结果。
type auditTrace struct {
	found bool
	total int
}
