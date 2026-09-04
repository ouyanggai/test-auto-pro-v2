package step

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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

	// 门禁通过：构造与实际发出的请求严格同源的类型化请求与载荷预览（不含 SID），
	// 并在发送前校验禁用字段（batchCode 禁令）。
	request, endpoint, payload, requestErr := buildRequest(runCtx, step, session)
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

// blockedPreview 构造被阻塞的预览：说明中文原因与失败分类，路径必须停止。
func (e *Executor) blockedPreview(runCtx RunContext, step model.CompiledActionStep, actorName, reason string, class model.FailureClass) *StepPreview {
	info := runCtx.Nodes[step.NodeKey]
	return &StepPreview{
		PathRunID:         runCtx.PathRun.ID,
		StepNo:            step.Sequence,
		TotalSteps:        len(runCtx.Steps),
		Action:            step.Action,
		NodeKey:           step.NodeKey,
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
}

// RunApprovedStep 执行阶段 3（放行）、4（prepare）、5（submit）、6（verify）、7（settle）。
// 一次尝试最多一次写请求：submit 只调用一次，写结果不确定即停在待对账，绝不重发。
func (e *Executor) RunApprovedStep(ctx context.Context, approved ApprovedStep) (StepOutcome, uint64, error) {
	runCtx := approved.RunCtx
	preview := approved.Preview
	step := runCtx.Steps[approved.NextIndex]
	log := e.stepLogFor(runCtx)
	startedAt := e.now()
	outcome := StepOutcome{Verdict: string(verdict.OutcomeUncertain)}

	if preview.BlockReason != "" {
		// 被阻塞的步骤不允许放行：路径运行在这里失败，而不是带病前进。
		class := preview.BlockFailureClass
		if _, err := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusFailed, runResultOf(model.RunResultFailed), &class,
			"路径在第 "+formatUint(uint64(step.Sequence))+" 步失败："+preview.BlockReason); err != nil {
			return outcome, 0, err
		}
		log.Phase("control", step.Sequence, 1, "放行被拒绝（"+preview.BlockReason+"），路径运行置为失败")
		return outcome, 0, nil
	}

	// 阶段 4：领取推进权并就绪演员会话。领取失败说明已有其他执行者，调用方必须放弃。
	fencingToken, err := e.runState.ClaimExecution(ctx, runCtx.PathRun.ID)
	if err != nil {
		return outcome, 0, err
	}
	// 写步骤的会话必须现场刷新并探活：submit 发出后不允许任何重登，
	// 过期会话只会白白浪费唯一一次写机会。实测目标存在“首次登录的 sid 立即失效”的现象，
	// 因此刷新后立刻做一次只读探活，失效则在只读重试预算内重新登录。
	session, sessionErr := RunWithRetry(ctx, e.policy, "会话刷新", func() (target.Session, error) {
		fresh, err := e.sessions.Refresh(ctx, runCtx.PlanAccount)
		if err != nil {
			return fresh, err
		}
		if pinger, ok := e.target.(interface{ Ping(context.Context, target.Session) error }); ok {
			if err := pinger.Ping(ctx, fresh); err != nil {
				return fresh, err
			}
		}
		return fresh, nil
	}, func(attempt int, nextDelay time.Duration) {
		log.Phase("prepare", step.Sequence, 1, fmt.Sprintf("会话刷新第 %d 次失败，%s 后重试", attempt, nextDelay))
	})
	if sessionErr != nil {
		class := model.FailureClassActorUnresolved
		if _, finishErr := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusFailed, runResultOf(model.RunResultFailed), &class,
			"演员登录失败："+sessionErr.Error()); finishErr != nil {
			return outcome, 0, finishErr
		}
		log.Phase("prepare", step.Sequence, 1, "演员会话获取失败："+sessionErr.Error())
		return outcome, 0, nil
	}
	log.Phase("prepare", step.Sequence, 1, fmt.Sprintf("演员 %s（%s）会话就绪，即将发出 %s", preview.ActorName, preview.ActorAccount, preview.Endpoint))

	// 阶段 5：发出唯一一次写请求。审批任务 ID 在发送前现场新鲜读取（演员与待办的新鲜复验）。
	e.refreshAndSubmit(ctx, runCtx, step, session, preview)
	log.Phase("submit", step.Sequence, 1, submitSummary(preview.writeErr, preview.writeTraceID, preview.writeDurationMs))

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
	before := preview.Facts
	before.StepNodeKey = step.NodeKey
	after, _ := e.readFactsWithRetry(ctx, runCtx, session, step)
	after.StepNodeKey = step.NodeKey
	reread := ClassifyReread(string(step.Action), step.NodeKey, before, after)
	observation := buildObservation(preview.Endpoint, preview.writeErr, preview.writeResponse, reread)
	observation.Action = string(step.Action)
	verdictResult := verdict.Evaluate(observation)
	log.Phase("verify", step.Sequence, 1, fmt.Sprintf("三值判定：%s（%s）", verdictChinese(verdictResult.Outcome), verdictResult.Reason))

	// 阶段 7：落账。事实表只 INSERT；随后按结论推进路径运行状态。
	lineNo := log.Phase("settle", step.Sequence, 1, "落账："+settleSummary(verdictResult))
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
	}
	attempt := model.RunStepAttempt{
		PathRunID:   runCtx.PathRun.ID,
		AttemptNo:   1,
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
	}
	if _, err := e.facts.RecordStepAttempt(ctx, record, attempt, e.now()); err != nil {
		return outcome, lineNo, err
	}

	switch verdictResult.Outcome {
	case verdict.OutcomeSucceeded:
		outcome.Verdict = string(verdict.OutcomeSucceeded)
		outcome.NoMoreSteps = approved.NextIndex+1 >= len(runCtx.Steps)
		if !outcome.NoMoreSteps {
			if err := e.runState.BackToRunning(ctx, runCtx.PathRun.ID); err != nil {
				return outcome, lineNo, err
			}
		}
		// 落账后释放推进权是尽力而为：释放失败只影响下一次领取的即时性，不影响已落账事实。
		_ = e.runState.ReleaseExecution(ctx, runCtx.PathRun.ID, fencingToken)
		log.Phase("settle", step.Sequence, 1, "本步确定成功")
	case verdict.OutcomeFailed:
		outcome.Verdict = string(verdict.OutcomeFailed)
		class := model.FailureClassTargetRejected
		if _, err := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusFailed, runResultOf(model.RunResultFailed), &class,
			"路径在第 "+formatUint(uint64(step.Sequence))+" 步被目标拒绝："+verdictResult.Reason); err != nil {
			return outcome, lineNo, err
		}
		log.Phase("settle", step.Sequence, 1, "本步确定失败且无副作用，路径运行置为失败")
	default:
		// 写结果不确定：路径运行进入待对账并停止；唯一合法恢复动作属于对账切片（F-018）。
		outcome.Verdict = string(verdict.OutcomeUncertain)
		class := model.FailureClassWriteUncertain
		if _, err := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusAwaitingReconciliation, runResultOf(model.RunResultAwaitingReconcile), &class,
			"第 "+formatUint(uint64(step.Sequence))+" 步写结果不确定："+verdictResult.Reason); err != nil {
			return outcome, lineNo, err
		}
		log.Phase("settle", step.Sequence, 1, "写结果不确定，路径运行进入待对账并停止")
	}
	return outcome, lineNo, nil
}

// refreshAndSubmit 在发送前完成待办任务 ID 的新鲜读取，然后发出唯一一次写请求。
// 请求本体与预览同源（preview.request）；本方法及其调用路径不存在任何重试。
func (e *Executor) refreshAndSubmit(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, session target.Session, preview *StepPreview) {
	switch request := preview.request.(type) {
	case *target.SubmitFlowInstanceRequest:
		started := e.now()
		result, response, traceID, err := e.target.SubmitFlowInstance(ctx, session, *request)
		preview.writeResult, preview.writeResponse, preview.writeTraceID, preview.writeErr = result, response, traceID, err
		preview.writeDurationMs = e.now().Sub(started).Milliseconds()
	case *target.AuditCurrentTaskRequest:
		started := e.now()
		jobTaskID, err := e.target.FindDueTaskID(ctx, session, runCtx.PathRun.MainInstanceRef, step.NodeKey)
		if err != nil {
			preview.writeErr = err
			return
		}
		if jobTaskID == "" {
			// 目标上已无本演员在本节点的活动待办：演员或待办已变化，绝不冒名发送。
			preview.writeErr = &UnverifiedActionError{Action: model.ActionApprove}
			return
		}
		request.JobTaskID = jobTaskID
		result, response, traceID, err := e.target.AuditCurrentTask(ctx, session, *request)
		preview.writeResult, preview.writeResponse, preview.writeTraceID, preview.writeErr = result, response, traceID, err
		preview.writeDurationMs = e.now().Sub(started).Milliseconds()
	default:
		preview.writeErr = errors.New("写请求载荷缺失，拒绝发送")
	}
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
	return RunWithRetry(ctx, e.policy, "事实重读", func() (InstanceFacts, error) {
		return e.readInstanceFacts(ctx, session, runCtx.PathRun.MainInstanceRef, step.NodeKey)
	}, nil)
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
