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
	"test-auto-pro-v2/internal/logging"
	"test-auto-pro-v2/internal/model"
)

// UnverifiedActionError 表示步骤动作不在当前原子动作目录内。
type UnverifiedActionError struct {
	Action model.ActionKey
}

// Error 返回中文说明：未知动作必须在写请求前停止，不能静默替换目标端点。
func (e *UnverifiedActionError) Error() string {
	return fmt.Sprintf("动作 %s 不在已支持的原子动作清单中", string(e.Action))
}

// Executor 执行一条路径运行上的一步七阶段。
// 边界：只经 RunStateControl 推进状态机、只经 TargetClient 发目标请求、
// 事实只经 RunFactsStore 落账；submit 阶段只允许连接尚未建立时的受控重试。
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
		policy: RetryPolicy{
			Attempts: runConfig.ReadOnlyRetryAttempts, BaseDelay: runConfig.ReadOnlyRetryBaseDelay, MaxDelay: runConfig.ReadOnlyRetryMaxDelay,
			WriteConnectAttempts: runConfig.WriteConnectRetryAttempts, WriteConnectBaseDelay: runConfig.WriteConnectRetryBaseDelay,
			WriteConnectMaxDelay: runConfig.WriteConnectRetryMaxDelay, Now: now,
		},
		now: now,
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

// withStepScope 把步骤号、尝试序号与执行阶段注入目标请求上下文的日志作用域，
// 传输层记录 network.log 时按 step_id/attempt/phase 稳定归属每条请求（F-030 评审 P1）。
// WithScope 是合并语义：只补充本步维度，不覆盖上游已注入的计划/路径/运行身份。
func withStepScope(ctx context.Context, stepNo, attemptNo int, phase string) context.Context {
	return logging.WithScope(ctx, logging.Scope{
		StepID:  formatUint(uint64(stepNo)),
		Attempt: formatUint(uint64(attemptNo)),
		Phase:   phase,
	})
}

// stepLogFor 返回该路径运行的 step.log 写入器；未注入工厂时返回空写入器（写入静默跳过）。
func (e *Executor) stepLogFor(runCtx RunContext) *StepLog {
	if e.logFactory == nil {
		return nil
	}
	return e.logFactory(runCtx)
}

// annotateInstance 把目标实例身份（ID 与名称）补进日志作用域与请求上下文：
// 之后的 step.log 阶段行、network.log/curl.log 与运行目录 meta.json 都带上同一实例。
// 日志目录键始终是页面的 runId/pathRunId——实例名称只作业务信息，
// 读取慢、读不到或后来改名都不会搬迁日志，也不会让运行等待目标名称。
func (e *Executor) annotateInstance(ctx context.Context, log *StepLog, instanceID, instanceName, note string) context.Context {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return ctx
	}
	log.SetInstance(instanceID, instanceName, note)
	return logging.WithScope(ctx, logging.Scope{
		InstanceID:            instanceID,
		InstanceName:          strings.TrimSpace(instanceName),
		InstanceNameAvailable: strings.TrimSpace(instanceName) != "",
		InstanceNameNote:      strings.TrimSpace(note),
	})
}

// annotateInstanceFromFacts 在实例事实可读时补上实例 ID 与名称；
// 事实不可读或没有实例时保持原样——没有事实就不写名称，绝不用计划名或路径名冒充实例名。
func (e *Executor) annotateInstanceFromFacts(ctx context.Context, runCtx RunContext, log *StepLog, facts InstanceFacts) context.Context {
	if !facts.Found {
		return ctx
	}
	note := ""
	if strings.TrimSpace(facts.InstanceName) == "" {
		note = "目标未返回实例名称"
	}
	return e.annotateInstance(ctx, log, runCtx.PathRun.MainInstanceRef, facts.InstanceName, note)
}

// BuildPreview 执行阶段 1（plan 取步）、阶段 2（gate 门禁复验），并写下阶段 3 的暂停行，
// 产出给用户的下一步预览。本方法只读目标、不落账、绝不发写请求。
// reportProgressSafe 安全调用预览进度上报器：未接收集方时是空操作，
// 保证旧的 BuildPreview 调用路径（无上报）行为完全不变。
func reportProgressSafe(reportProgress func(phase, note string), phase, note string) {
	if reportProgress != nil {
		reportProgress(phase, note)
	}
}

// BuildPreview 执行阶段 1（plan 取步）、阶段 2（gate 门禁复验），并写下阶段 3 的暂停行，
// 产出给用户的下一步预览。本方法只读目标、不落账、绝不发写请求。
func (e *Executor) BuildPreview(ctx context.Context, runCtx RunContext, nextIndex int) (*StepPreview, bool, error) {
	return e.BuildPreviewWithProgress(ctx, runCtx, nextIndex, nil)
}

// BuildPreviewWithProgress 是带实时阶段上报的预览构建（F-030 评审 P1）：
// reportProgress 在长耗时读取（登录、目标事实、候选处理人扫描）前后回调，
// 页面等待预览时能看到正在检查什么；为 nil 时行为与旧 BuildPreview 完全一致。
// 同时把 step_id/attempt/phase 注入目标请求上下文，network.log 每条请求可稳定归属到本步。
func (e *Executor) BuildPreviewWithProgress(ctx context.Context, runCtx RunContext, nextIndex int, reportProgress func(phase, note string)) (*StepPreview, bool, error) {
	if nextIndex >= len(runCtx.Steps) {
		return nil, true, nil
	}
	var releaseUsage func()
	if locker, ok := e.sessions.(interface{ LockAccountUsage(string) func() }); ok {
		releaseUsage = locker.LockAccountUsage(runCtx.PlanAccount)
		defer releaseUsage()
	}
	// F-030/T02：任务列表 memo 覆盖整个预览调用树（gate 事实读取、候选人扫描、
	// 当前处理人切换都对同一账号×实例×状态复用同一次目标分页扫描）。
	// 作用域只到本次预览：放行后的核验（RunApprovedStep）不经过这里，天然全新读取。
	ctx = target.AttachTaskSnapshotScope(ctx)
	step := runCtx.Steps[nextIndex]
	// 预览请求统一归属到本步第 1 次尝试：预览只读不写，不存在别的尝试号。
	ctx = withStepScope(ctx, step.Sequence, 1, "plan")
	reportProgressSafe(reportProgress, "plan", fmt.Sprintf("正在确认第 %d 步要做什么：查看要操作的节点和动作，确认它属于当前流程", step.Sequence))
	log := e.stepLogFor(runCtx)
	log.Phase("plan", step.Sequence, 1, fmt.Sprintf("取第 %d 步：来源 %s，动作 %s，节点 %s", step.Sequence, step.Source, string(step.Action), step.NodeKey))

	// 导航步骤（system_navigation）：只读校验步骤，不发出写请求。
	// 真实执行语义 = 实例事实可读即视为通过（目标引擎自动推进实例经过系统节点）。
	if step.Source == model.ActionStepSourceNavigation {
		actorName := runCtx.PlanAccount
		session, sessionErr := e.sessionWithRetry(ctx, runCtx, log, step.Sequence, "gate")
		if sessionErr != nil {
			return e.blockedPreview(runCtx, step, actorName, "当前处理人登录失败："+userFacingError(sessionErr, target.WriteResponse{}), model.FailureClassActorUnresolved), false, nil
		}
		if session.Summary.DisplayName != "" {
			actorName = session.Summary.DisplayName
		}
		facts, session, readErr := e.readFactsWithRetry(ctx, runCtx, session, step)
		if readErr != nil {
			message := userFacingError(readErr, target.WriteResponse{})
			log.Phase("gate", step.Sequence, 1, "目标状态确认失败："+message)
			return e.blockedPreview(runCtx, step, actorName, "目标状态确认失败："+message, model.FailureClassGateBlocked), false, nil
		}
		// 实例事实可读就把实例 ID 与名称补进日志作用域（名称读不到只影响展示）。
		ctx = e.annotateInstanceFromFacts(ctx, runCtx, log, facts)
		preview := &StepPreview{
			PathRunID: runCtx.PathRun.ID, StepNo: step.Sequence, TotalSteps: len(runCtx.Steps),
			ReleaseGroup: step.ReleaseGroup, ReleaseRequired: step.ReleaseRequired,
			Action: step.Action, ActionName: "导航校验", NodeKey: step.NodeKey, TargetNodeID: runCtx.Nodes[step.NodeKey].TargetNodeID,
			NodeName: runCtx.Nodes[step.NodeKey].Name, ActorAccount: runCtx.PlanAccount, ActorName: actorName,
			GateAllowed: true, Facts: facts, Navigation: true,
			GateItems: []model.ActionPrecondition{},
		}
		log.Phase("gate", step.Sequence, 1, "导航步骤无需写请求，仅校验实例事实")
		log.Phase("control", step.Sequence, 1, "导航步骤就绪（只读）")
		return preview, false, nil
	}

	// 阶段 2：计划账号会话取得会话后重读目标实时事实，投影为门禁上下文重新计算门禁。
	// 配置时通过、此刻不通过就停止：门禁不通过绝不跳过。
	// F-030 评审 P1：阶段说明同时回答现在做什么、为什么、下一步是什么。
	actorName := runCtx.PlanAccount
	reportProgressSafe(reportProgress, "gate", fmt.Sprintf("正在检查第 %d 步现在能不能做：用计划账号登录目标平台，重新查看流程的当前状态（节点、待办）并复核放行条件", step.Sequence))
	session, sessionErr := e.sessionWithRetry(ctx, runCtx, log, step.Sequence, "gate")
	if sessionErr != nil {
		message := userFacingError(sessionErr, target.WriteResponse{})
		log.Phase("gate", step.Sequence, 1, "当前处理人登录失败："+message)
		return e.blockedPreview(runCtx, step, actorName,
			"当前处理人登录失败："+message, model.FailureClassActorUnresolved), false, nil
	}
	if session.Summary.DisplayName != "" {
		actorName = session.Summary.DisplayName
	}
	// 目标事实重读可能包含候选处理人扫描（逐个切换只读会话），是预览阶段最长的等待，
	// 必须把正在检查什么上报给页面，不能让用户对着无说明的等待猜进度。
	// 此处动作名尚未从目录解析，不提前使用内部动作键，避免页面出现内部枚举。
	reportProgressSafe(reportProgress, "gate", fmt.Sprintf("正在重新查看流程在目标平台的实时状态（当前节点、待办与实际处理人），确认第 %d 步现在能不能做；检查完成后会展示这一步的预览并等待确认", step.Sequence))
	// 进入 gate 阶段的目标读取：更新请求上下文的阶段标记，network.log 能区分预览取步与门禁事实读取。
	ctx = withStepScope(ctx, step.Sequence, 1, "gate")
	facts, session, readErr := e.readFactsWithRetry(ctx, runCtx, session, step)
	if readErr != nil {
		message := userFacingError(readErr, target.WriteResponse{})
		log.Phase("gate", step.Sequence, 1, "目标状态确认失败："+message)
		return e.blockedPreview(runCtx, step, actorName,
			"目标状态确认失败："+message, model.FailureClassGateBlocked), false, nil
	}
	ctx = e.annotateInstanceFromFacts(ctx, runCtx, log, facts)
	if name := strings.TrimSpace(facts.CurrentTaskAssigneeName); name == "" {
	} else {
		actorName = name
	}
	// F-035：目标节点已到达但未生成处理人/待办时，进入“正在生成处理人”有界轮询；
	// 总等待不超过 10 秒、最多 5 次，期间不发任何写请求。超时后按 assignment_missing 阻塞，
	// 绝不显示“当前待办已经处理”，也不用计划账号或候选人代替目标返回的真实处理人。
	if step.Scope == model.ActionScopeTask && facts.HandlerGenerationRead && facts.HandlerMissing && stepTargetNodeIDOf(runCtx, step) != "" {
		pollFacts, pollSession, pollErr := e.pollForHandlerGeneration(ctx, runCtx, session, step, log)
		if pollErr != nil {
			return e.blockedPreview(runCtx, step, actorName, pollErr.Error(), model.FailureClassGateBlocked), false, nil
		}
		if pollSession != nil {
			session = *pollSession
		}
		facts = pollFacts
	}
	info := runCtx.Nodes[step.NodeKey]
	// 目标自动跳过适配：模板约束「无处理人时跳过该节点」在人员规则（如扩展属性）解析为空时
	// 生效，实例待办直接落到路径上更靠后的节点，本步的同意永远等不到待办。只有「实例当前
	// 待办节点在已配置路径上严格位于本步之后」这一可证明事实才允许跳过；其余情形仍按既有
	// 门禁判定，绝不猜测。跳过不发出任何写请求，放行后按「已跳过」落账并推进。
	if step.Scope == model.ActionScopeTask || step.Scope == model.ActionScopeCompletedTask {
		action, reason, pendingName, diag := targetSkippedStepReason(runCtx, step, facts)
		// 判定依据必须落 step.log：否则界面上「当前待办已经处理」无法解释实例究竟停在哪。
		log.Phase("gate", step.Sequence, 1, "目标跳过判定："+diag)
		switch action {
		case "skip":
			preview := &StepPreview{
				PathRunID: runCtx.PathRun.ID, StepNo: step.Sequence, TotalSteps: len(runCtx.Steps),
				ReleaseGroup: step.ReleaseGroup, ReleaseRequired: step.ReleaseRequired,
				Action: step.Action, ActionName: "目标跳过确认", NodeKey: step.NodeKey,
				TargetNodeID: info.TargetNodeID, NodeName: info.Name,
				ActorAccount: runCtx.PlanAccount, ActorName: actorName,
				GateAllowed: true, Facts: facts, TargetSkipped: true, SkipReason: reason,
				GateItems: []model.ActionPrecondition{},
			}
			log.Phase("control", step.Sequence, 1, fmt.Sprintf("该节点已被目标自动跳过（实例待办已在「%s」）；放行将记录跳过并继续", pendingName))
			return preview, false, nil
		case "block":
			// F-034：模板不允许跳过或自选节点缺人，目标不可能自动越过本节点；
			// 必须在发送前阻塞，绝不发写请求，也不用空人员或随机人员凑数。
			log.Phase("control", step.Sequence, 1, "目标不可能自动越过本节点，已在发送前阻塞："+reason)
			return e.blockedPreview(runCtx, step, actorName, reason, model.FailureClassGateBlocked), false, nil
		}
	}
	catalogItem, allowed := evaluateGate(step, buildGateContext(runCtx, step, facts, info))
	// 固定人员发现的结论随门禁一并披露：拒绝原因要能说明「待办在谁的账号下、工具找过了谁」。
	if !allowed && strings.TrimSpace(facts.AssigneeDiag) != "" {
		log.Phase("gate", step.Sequence, 1, "固定人员发现："+facts.AssigneeDiag)
	}
	log.Phase("gate", step.Sequence, 1, gateSummary(catalogItem, allowed))

	preview := &StepPreview{
		PathRunID:       runCtx.PathRun.ID,
		StepNo:          step.Sequence,
		TotalSteps:      len(runCtx.Steps),
		ReleaseGroup:    step.ReleaseGroup,
		ReleaseRequired: step.ReleaseRequired,
		Action:          step.Action,
		ActionName:      catalogItem.Label,
		NodeKey:         step.NodeKey,
		TargetNodeID:    info.TargetNodeID,
		NodeName:        info.Name,
		ActorAccount:    runCtx.PlanAccount,
		ActorName:       actorName,
		ExpectedEffect:  catalogItem.ExpectedEffect,
		GateAllowed:     allowed,
		GateItems:       catalogItem.Preconditions,
		Facts:           facts,
	}
	if !allowed {
		reason := catalogItem.DisabledReason
		if reason == "" {
			reason = "放行条件不满足"
		}
		// 固定人员发现的结论拼进拒绝原因：只说「当前待办已经处理」解释不了实例停在哪、
		// 工具找过了谁；界面与 step.log 必须给出可操作的下一步。
		if strings.TrimSpace(facts.AssigneeDiag) != "" {
			reason = reason + "（" + facts.AssigneeDiag + "）"
		}
		preview.GateReason = reason
		preview.BlockReason = "放行条件不满足：" + reason
		preview.BlockFailureClass = model.FailureClassGateBlocked
		log.Phase("control", step.Sequence, 1, "单步暂停，等待放行；本步条件未满足："+reason)
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
	formPlan, session, formErr := e.nodeFormData(ctx, runCtx, step, session)
	if formErr != nil {
		message := userFacingError(formErr, target.WriteResponse{})
		log.Phase("gate", step.Sequence, 1, "读取当前表单失败："+message)
		return e.blockedPreview(runCtx, step, actorName,
			"读取当前表单失败："+message, model.FailureClassGateBlocked), false, nil
	}
	if len(formPlan.Withheld) > 0 || len(formPlan.Overlaid) > 0 {
		log.Phase("gate", step.Sequence, 1, fmt.Sprintf("表单数据按节点权限构造：基线=%s，覆盖 %d 个字段 %v，按权限未带 %d 个字段 %v",
			formBaseName(formPlan.BaseFromInstance), len(formPlan.Overlaid), formPlan.Overlaid, len(formPlan.Withheld), formPlan.Withheld))
	}
	// F-035/T05：每个节点的表单决策完整落 step.log（内网系统不脱敏），
	// 字段所有权、基线版本指纹与校验结论可逐节点追溯。
	if formPlan.Decision != nil {
		if encoded, marshalErr := json.Marshal(formPlan.Decision); marshalErr == nil {
			log.Phase("gate", step.Sequence, 1, "表单节点决策："+string(encoded))
		}
	}
	preview.FormOverlaid = formPlan.Overlaid
	preview.FormWithheld = formPlan.Withheld
	preview.FormBaseFromInstance = formPlan.BaseFromInstance
	preview.FormDataDecision = formPlan.Decision

	// 构造与实际发出的请求严格同源的类型化请求与载荷预览（不含 SID）。
	// 字段存在性由逐接口协议矩阵在适配层构造器内强制（F-035），不再做通用字段禁令校验。
	// 提交类和同意类都要按真正的后续业务节点构造 nextAuditorList；当前步骤自身不是下一节点。
	nextNodeKey := FollowingActionNodeKey(runCtx.Steps, nextIndex)
	request, endpoint, payload, requestErr := buildRequestWithFacts(runCtx, step, session, formPlan.Payload, nextNodeKey, facts)
	if requestErr != nil {
		message := userFacingError(requestErr, target.WriteResponse{})
		preview.BlockReason = "构造写请求失败：" + message
		preview.BlockFailureClass = model.FailureClassToolBug
		log.Phase("gate", step.Sequence, 1, "构造写请求失败："+message)
		return preview, false, nil
	}
	preview.Endpoint = endpoint
	preview.RequestPayload = payload
	preview.RequestPreview = previewJSON(payload)
	preview.request = request
	log.Phase("control", step.Sequence, 1, "单步暂停，等待放行")
	reportProgressSafe(reportProgress, "control", fmt.Sprintf("第 %d 步已就绪，等你确认执行：点击“执行这一步”后，会由“%s”在“%s”节点执行“%s”，只发送一次目标请求", step.Sequence, actorName, runCtx.Nodes[step.NodeKey].Name, catalogItem.Label))
	return preview, false, nil
}

// FollowingActionNodeKey 返回当前动作之后第一个非导航、且不同于当前节点的业务节点。
// 系统导航不产生待办，恢复步骤可能留在当前节点；把它们当作目标下一节点会让目标拒绝实际人员选择。
func FollowingActionNodeKey(steps []model.CompiledActionStep, currentIndex int) string {
	if currentIndex < 0 || currentIndex >= len(steps) {
		return ""
	}
	currentNodeKey := strings.TrimSpace(steps[currentIndex].NodeKey)
	for index := currentIndex + 1; index < len(steps); index++ {
		candidate := steps[index]
		if candidate.Source == model.ActionStepSourceNavigation {
			continue
		}
		candidateNodeKey := strings.TrimSpace(candidate.NodeKey)
		if candidateNodeKey == "" || candidateNodeKey == currentNodeKey {
			continue
		}
		return candidateNodeKey
	}
	return ""
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
// 读取期间如果会话失效，返回刷新后的会话，后续预览载荷和放行写请求必须继续使用它。
func (e *Executor) nodeFormData(ctx context.Context, runCtx RunContext, compiled model.CompiledActionStep, session target.Session) (FormDataPlan, target.Session, error) {
	if !ActionCarriesFormData(compiled.Action) {
		return FormDataPlan{}, session, nil
	}
	var current map[string]any
	hasInstance := false
	if instanceRef := strings.TrimSpace(runCtx.PathRun.MainInstanceRef); instanceRef != "" {
		hasInstance = true
		account := strings.TrimSpace(session.Summary.Account)
		if account == "" {
			account = runCtx.PlanAccount
		}
		read, active, err := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, account, session,
			func(callContext context.Context, active target.Session) (map[string]any, error) {
				return e.target.ReadInstanceCurrentData(callContext, active, instanceRef)
			})
		if err != nil {
			return FormDataPlan{}, active, err
		}
		session = active
		current = read
	}
	// 实例存在但数据为空时必须保持实例分支（空基线），不得退回发起分支提交整份历史配置。
	// F-035/T05：上一节点决策非空时执行跨节点核对，上一节点覆盖字段在目标实例上丢失立即阻塞。
	plan, err := BuildNodeFormData(runCtx, compiled, current, hasInstance, runCtx.LastFormDataDecision)
	if err == nil && plan.Decision != nil && len(plan.Decision.ValidationIssues) > 0 {
		return plan, session, fmt.Errorf("%s", strings.Join(plan.Decision.ValidationIssues, "；"))
	}
	return plan, session, err
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
		ReleaseGroup:      step.ReleaseGroup,
		ReleaseRequired:   step.ReleaseRequired,
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

// stepTargetNodeIDOf 返回编译步骤节点在目标平台的真实标识；缺失时返回空串由调用方兜底。
func stepTargetNodeIDOf(runCtx RunContext, step model.CompiledActionStep) string {
	return strings.TrimSpace(runCtx.Nodes[step.NodeKey].TargetNodeID)
}

// pollForHandlerGeneration 对“目标节点已到达但处理人/待办尚未生成”的场景执行有界轮询（F-035/T07）：
// 最多 5 次、总等待不超过 10 秒，每次只重读实例事实（只读），绝不重发写请求。
// 轮询期间任何一次读到处理人或待办即返回最新事实；全部超时仍无处理人时返回阻塞结论，
// 停止原因按 assignment_missing 分型（目标没有产生处理人），不伪装成“已处理”。
func (e *Executor) pollForHandlerGeneration(ctx context.Context, runCtx RunContext, session target.Session, step model.CompiledActionStep, log *StepLog) (InstanceFacts, *target.Session, error) {
	const maxPolls = 5
	const pollInterval = 2 * time.Second
	const totalBudget = 10 * time.Second
	deadline := e.now().Add(totalBudget)
	facts := InstanceFacts{}
	for attempt := 1; attempt <= maxPolls; attempt++ {
		select {
		case <-ctx.Done():
			return facts, nil, fmt.Errorf("等待目标生成处理人被取消：%w", ctx.Err())
		case <-time.After(pollInterval):
		}
		if e.now().After(deadline) {
			break
		}
		log.Phase("gate", step.Sequence, attempt, fmt.Sprintf("目标节点已到达但尚未生成处理人，正在第 %d/%d 次复查（不发送任何写请求）", attempt, maxPolls))
		pollFacts, pollSession, err := e.readFactsWithRetry(ctx, runCtx, session, step)
		if err != nil {
			return facts, nil, fmt.Errorf("等待目标生成处理人期间读取失败：%s", target.UserFacingErrorMessage(target.WriteResponse{}, err))
		}
		session = pollSession
		facts = pollFacts
		if !pollFacts.HandlerMissing {
			return pollFacts, &session, nil
		}
	}
	if facts.ReadError != "" {
		return facts, nil, fmt.Errorf("目标节点未生成处理人，且复查读取失败：%s", facts.ReadError)
	}
	return facts, nil, fmt.Errorf("目标节点已到达但 %d 秒内未生成处理人和待办（assignment_missing）；已阻塞本步，不发送写请求，请到目标平台确认该节点的处理人配置", int(totalBudget.Seconds()))
}

// targetSkippedStepReason 判断本步节点是否已被目标自动跳过（F-034：按目标 isSkip 与审批类型分型）。
// 返回值：
//   - action="skip"：模板声明允许跳过（isSkip=true）且实例待办可证明已越过本节点，无写请求记“目标已跳过”；
//   - action="block"：待办已越过本节点但模板未声明允许跳过（isSkip 未声明或为 false），
//     或本节点是 run_node_choose 自选审批——目标不可能自动越过，必须阻塞，不能用空人员凑数；
//   - action=""：无法证明越过，按既有门禁失败处理，绝不猜测。
func targetSkippedStepReason(runCtx RunContext, step model.CompiledActionStep, facts InstanceFacts) (action, reason, pendingName, diag string) {
	info := runCtx.Nodes[step.NodeKey]
	if !facts.Found {
		return "", "", "", "实例不可读（found=false）"
	}
	if strings.TrimSpace(info.TargetNodeID) == "" {
		return "", "", "", "本步节点缺少目标真实标识"
	}
	position := map[string]int{}
	for index, s := range runCtx.Steps {
		key := strings.TrimSpace(s.NodeKey)
		if key == "" {
			continue
		}
		if _, exists := position[key]; !exists {
			position[key] = index
		}
	}
	expectedIndex, ok := position[strings.TrimSpace(step.NodeKey)]
	if !ok {
		return "", "", "", "本步节点不在编译场景中"
	}
	pendingIndex := -1
	seen := map[string]bool{}
	for _, nodeID := range append(append([]string{}, facts.DueNodes...), facts.CurrentNodes...) {
		id := strings.TrimSpace(nodeID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		for key, nodeInfo := range runCtx.Nodes {
			if strings.TrimSpace(nodeInfo.TargetNodeID) != id {
				continue
			}
			index, exists := position[key]
			if !exists {
				continue
			}
			if pendingIndex < 0 || index < pendingIndex {
				pendingIndex = index
				pendingName = nodeInfo.Name
			}
			break
		}
	}
	if pendingIndex < 0 {
		return "", "", "", fmt.Sprintf("实例当前/待办节点 %v 都不在已配置路径上，无法判定跳过", append(append([]string{}, facts.DueNodes...), facts.CurrentNodes...))
	}
	if pendingIndex <= expectedIndex {
		return "", "", "", fmt.Sprintf("实例待办仍在「%s」（本步或更早），不构成跳过", pendingName)
	}
	// 待办确实越过本节点。分型依据目标后端规则：只有模板声明 isSkip=true 才递归跳过；
	// run_node_choose 自选节点无论声明如何都要求显式人员，缺失即抛“未设置审批人”。
	if strings.TrimSpace(info.AuditType) == "run_node_choose" {
		return "block", fmt.Sprintf("节点「%s」是运行时自选审批节点，待办已越过但没有可用的已选处理人，不能自动跳过", info.Name), pendingName,
			fmt.Sprintf("实例待办在「%s」，本节点为 run_node_choose，必须阻塞", pendingName)
	}
	if info.IsSkip == nil || !*info.IsSkip {
		return "block", fmt.Sprintf("目标未在节点「%s」声明“无处理人时跳过”，而实例待办已越过该节点，无法安全继续", info.Name), pendingName,
			fmt.Sprintf("实例待办在「%s」，模板未声明 isSkip，必须阻塞", pendingName)
	}
	return "skip", fmt.Sprintf("实例待办已在「%s」，本节点已被目标自动跳过（无处理人时跳过该节点），没有可执行的审批动作", pendingName), pendingName,
		fmt.Sprintf("实例待办在「%s」，模板声明允许跳过（isSkip=true）", pendingName)
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
	// Reverify 表示本次放行只重新核验、绝不重发写请求（2026-09-11 用户裁决：
	// 核验时工具自己会话失效读不到事实，就停在原地，恢复后重新核验，而不是判终局）。
	Reverify bool
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
// 一次尝试最多一次进入目标业务写入；仅连接尚未写出时允许有限重试，写出后响应丢失绝不重发。
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

	if approved.Reverify {
		// 重新核验（2026-09-11 用户裁决）：上次核验读取失败，这次只重读结果、绝不重发写请求。
		// 用写出时的当前处理人会话重读（其已办/待办才是本步写是否生效的直接事实）；
		// 会话已失效时 readFactsWithRetry 会按该账号自动重登。
		var releasePlanUsage func()
		var releaseActorUsage func()
		if locker, ok := e.sessions.(interface{ LockAccountUsage(string) func() }); ok {
			releasePlanUsage = locker.LockAccountUsage(runCtx.PlanAccount)
			defer releasePlanUsage()
			account := strings.TrimSpace(preview.ActorAccount)
			if account != "" && !strings.EqualFold(account, strings.TrimSpace(runCtx.PlanAccount)) {
				releaseActorUsage = locker.LockAccountUsage(account)
				defer releaseActorUsage()
			}
		}
		if err := e.runState.MarkVerifying(ctx, runCtx.PathRun.ID); err != nil {
			return outcome, 0, err
		}
		// 重新核验只读重读，同样注入步骤作用域，与首次执行的请求区分尝试号。
		ctx = withStepScope(ctx, step.Sequence, attemptNo, "verify")
		reportPhase(approved, "verify", "正在重新读取执行结果：只重新查看目标平台确认上次请求是否生效，不会重复发写请求")
		actorAccount := strings.TrimSpace(preview.ActorAccount)
		if actorAccount == "" {
			actorAccount = runCtx.PlanAccount
		}
		session, sessionErr := e.sessions.Current(ctx, actorAccount)
		if sessionErr != nil && sessionRefreshAllowed(sessionErr) {
			session, sessionErr = e.sessions.Refresh(ctx, actorAccount)
		}
		if sessionErr != nil {
			// 登录失败也停住不判终局：用户稍后重新放行再试。
			outcome.RereadFailed = true
			return outcome, 0, nil
		}
		after, _, readErr := e.readFactsWithRetry(ctx, runCtx, session, step)
		after.StepNodeKey = stepTargetNodeIDOf(runCtx, step)
		if readErr != nil || after.ReadError != "" {
			outcome.RereadFailed = true
			return outcome, 0, nil
		}
		before := approved.Preview.Facts
		before.StepNodeKey = after.StepNodeKey
		reread := ClassifyReread(string(step.Action), after.StepNodeKey, before, after)
		observation := buildObservation(approved.Preview.Endpoint, nil, approved.Preview.writeResponse, reread)
		observation.Action = string(step.Action)
		observation.ActionFactVerified = ActionFactVerified(step.Action, reread)
		verdictResult := verdict.Evaluate(observation)
		userMessage := userResultMessage(step.Action, verdictResult, approved.Preview.writeResponse, approved.Preview.writeErr, after)
		lineNo := log.Phase("settle", step.Sequence, attemptNo, "重新核验："+userMessage)
		record := model.RunStep{
			PathRunID: runCtx.PathRun.ID, StepNo: step.Sequence, Source: string(step.Source),
			Action: string(step.Action), NodeKey: step.NodeKey, ActorSummary: preview.ActorName,
			Status: statusOfVerdict(verdictResult.Outcome), StartedAt: startedAt, FinishedAt: e.now(),
		}
		attempt := model.RunStepAttempt{
			PathRunID: runCtx.PathRun.ID, AttemptNo: attemptNo, Verdict: string(verdictResult.Outcome),
			SideEffect: string(verdictResult.SideEffect), Initial: string(verdictResult.Initial),
			Reread: string(reread), Reason: userMessage, Basis: verdictResult.Basis,
			TraceID: approved.Preview.writeTraceID, CurlTraceID: approved.Preview.writeTraceID,
			DurationMs: e.now().Sub(startedAt).Milliseconds(),
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
		case verdict.OutcomeFailed:
			class := model.FailureClassTargetRejected
			if _, err := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusFailed, runResultOf(model.RunResultFailed), &class,
				"重新核验判定失败："+userMessage); err != nil {
				return outcome, lineNo, err
			}
		default:
			class := model.FailureClassWriteUncertain
			if _, err := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusAwaitingReconciliation, runResultOf(model.RunResultAwaitingReconcile), &class,
				"重新核验仍未确认执行结果："+userMessage); err != nil {
				return outcome, lineNo, err
			}
		}
		return outcome, lineNo, nil
	}

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
			SideEffect: string(verdict.SideEffectNone), Reason: "执行成功：已确认流程节点", Basis: "实例事实可读",
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
	if preview.TargetSkipped {
		// 目标自动跳过：本节点没有待办（模板「无处理人时跳过该节点」约束生效），没有可执行
		// 的写动作。只读核实后按「已跳过」落账并推进游标，绝不代替目标补发任何写请求；
		// 与导航步骤同一推进语义：落账、返回确定成功让控制层推进下一步。
		lineNo := log.Phase("settle", step.Sequence, 1, "落账："+preview.SkipReason)
		record := model.RunStep{
			PathRunID: runCtx.PathRun.ID, StepNo: step.Sequence, Source: string(step.Source),
			Action: string(step.Action), NodeKey: step.NodeKey, ActorSummary: preview.ActorName,
			Status: model.RunStepSkipped, StartedAt: startedAt, FinishedAt: e.now(),
			GateSnapshot: gateSnapshotJSON(preview, approved.RunCtx.SubmitBranchTargetNodeID),
		}
		attempt := model.RunStepAttempt{
			PathRunID: runCtx.PathRun.ID, AttemptNo: 1, Verdict: StepVerdictTargetSkipped,
			SideEffect: string(verdict.SideEffectNone), Reason: preview.SkipReason,
			Basis:   "实例待办已越过本节点，按目标事实记录跳过",
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
		log.Phase("settle", step.Sequence, 1, "跳过步骤完成")
		return outcome, lineNo, nil
	}
	if preview.BlockReason != "" {
		// 被阻塞的步骤不允许放行：路径运行在这里失败，而不是带病前进。
		class := preview.BlockFailureClass
		// 门禁阻塞也落一条尝试记录：失败原因（含目标平台原话）与日志行直接进界面，
		// 不让失败步骤只剩一句泛化提示。此时没有写请求，副作用如实记 none。
		lineNo := log.Phase("control", step.Sequence, attemptNo, "放行被拒绝（"+preview.BlockReason+"），路径运行置为失败")
		record := model.RunStep{
			PathRunID: runCtx.PathRun.ID, StepNo: step.Sequence, Source: string(step.Source),
			Action: string(step.Action), NodeKey: step.NodeKey, ActorSummary: preview.ActorName,
			Status: model.RunStepFailed, StartedAt: startedAt, FinishedAt: e.now(),
			GateSnapshot: gateSnapshotJSON(preview, approved.RunCtx.SubmitBranchTargetNodeID),
		}
		attempt := model.RunStepAttempt{
			PathRunID: runCtx.PathRun.ID, AttemptNo: attemptNo, Verdict: string(verdict.OutcomeFailed),
			SideEffect: string(verdict.SideEffectNone), Reason: preview.BlockReason, Basis: "放行条件未满足，没有发出写请求",
			FailureClass: &class, LogPath: log.RelativePath(), LogLine: lineNo,
		}
		if _, recordErr := e.facts.RecordStepAttempt(ctx, record, attempt, e.now()); recordErr != nil {
			return outcome, lineNo, recordErr
		}
		if _, err := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusFailed, runResultOf(model.RunResultFailed), &class,
			"路径在第 "+formatUint(uint64(step.Sequence))+" 步失败："+preview.BlockReason); err != nil {
			return outcome, lineNo, err
		}
		reportPhase(approved, "control", "放行被拒绝："+preview.BlockReason)
		return outcome, lineNo, nil
	}

	// 阶段 4：领取推进权并就绪当前处理人的会话。领取失败说明已有其他执行者，调用方必须放弃。
	fencingToken, err := e.runState.ClaimExecution(ctx, runCtx.PathRun.ID)
	if err != nil {
		return outcome, 0, err
	}
	// 从 prepare 起把步骤号、尝试号、阶段注入目标请求上下文：本阶段所有目标请求
	// （会话探活、任务新鲜读取）在 network.log 中按 step_id/attempt/phase 稳定归属。
	ctx = withStepScope(ctx, step.Sequence, attemptNo, "prepare")
	reportPhase(approved, "prepare", fmt.Sprintf("正在准备第 %d 步的实际操作人：为“%s”准备目标平台登录会话；这里只做账号准备，不会提交审批动作，准备好后才会发送一次“%s”请求", step.Sequence, preview.ActorName, firstNonEmpty(preview.ActionName, string(step.Action))))
	// 从取得执行会话开始独占计划账号，页面读取和其他执行不能并发刷新同一账号 SID。
	var releasePlanUsage func()
	var releaseActorUsage func()
	usageAccount := ""
	if locker, ok := e.sessions.(interface{ LockAccountUsage(string) func() }); ok {
		releasePlanUsage = locker.LockAccountUsage(runCtx.PlanAccount)
		usageAccount = strings.TrimSpace(runCtx.PlanAccount)
		defer func() {
			if releaseActorUsage != nil {
				releaseActorUsage()
			}
			if releasePlanUsage != nil {
				releasePlanUsage()
			}
		}()
	}
	// 写步骤的会话策略对齐 V1 长期验证的模式（2026-09-07 修正）：复用缓存会话，
	// 写请求被会话失效拒绝时由 resubmitOnSessionRejected 恢复（仅自动重登+重发一次）。
	// 每一步强制重登会让登录频率放大数倍，实测触发了目标平台对账号的会话限制
	// （连只读都秒级失效），反而摧毁运行现场；V1 的「缓存复用+失效恢复」多年无此问题。
	session, sessionErr := func() (target.Session, error) {
		if cached, err := e.sessions.Current(ctx, runCtx.PlanAccount); err == nil {
			return cached, nil
		} else if !sessionRefreshAllowed(err) {
			return target.Session{}, err
		}
		return RunWithConnectRetry(ctx, e.policy, func(callContext context.Context) (target.Session, error) {
			return e.sessions.Refresh(callContext, runCtx.PlanAccount)
		}, func(attempt int, nextDelay time.Duration) {
			note := fmt.Sprintf("会话刷新第 %d 次失败，%s 后重试", attempt, nextDelay)
			log.Phase("prepare", step.Sequence, attemptNo, note)
			reportPhase(approved, "prepare", note)
		})
	}()
	if sessionErr != nil {
		class := model.FailureClassActorUnresolved
		reason := "当前处理人登录失败：" + userFacingError(sessionErr, target.WriteResponse{})
		lineNo := log.Phase("prepare", step.Sequence, attemptNo, reason)
		// prepare 失败也必须落一条失败事实行：没有它界面只能把失败标记回退到上一个成功节点，
		// 把「本节点失败」误显示成上一个节点失败，用户找不到真正出问题的地方。
		if recordErr := e.recordPrepareFailure(ctx, runCtx, step, preview, approved.RunCtx.SubmitBranchTargetNodeID,
			attemptNo, approved.IsReplay, startedAt, class, reason, "当前处理人登录会话未就绪，没有发出写请求", log.RelativePath(), lineNo); recordErr != nil {
			return outcome, lineNo, recordErr
		}
		if _, finishErr := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusFailed, runResultOf(model.RunResultFailed), &class, reason); finishErr != nil {
			return outcome, lineNo, finishErr
		}
		return outcome, lineNo, nil
	}
	// 任务级动作以目标实时待办的真实处理人身份发出（工作包 D）：
	// 事实里的 currentPendingUserId 是目标裁决的实际处理人，必须解析其登录账号并切换到当前处理人的会话，
	// 绝不冒用计划账号审批他人任务。解析失败就如实置败并指出节点与人员，不能静默回退到计划账号。
	if step.Scope == model.ActionScopeTask || step.Scope == model.ActionScopeCompletedTask {
		if assigneeID := strings.TrimSpace(preview.Facts.CurrentTaskAssigneeID); assigneeID != "" {
			if account, name, resolveErr := e.assigneeAccount(ctx, session, assigneeID); resolveErr != nil {
				class := model.FailureClassActorUnresolved
				reason := "无法解析本节点实际处理人（" + nameOrFallback(preview.Facts.CurrentTaskAssigneeName, assigneeID) + "）的登录账号：" + userFacingError(resolveErr, target.WriteResponse{})
				lineNo := log.Phase("prepare", step.Sequence, attemptNo, reason)
				if recordErr := e.recordPrepareFailure(ctx, runCtx, step, preview, approved.RunCtx.SubmitBranchTargetNodeID,
					attemptNo, approved.IsReplay, startedAt, class, reason, "当前处理人登录身份未确认，没有发出写请求", log.RelativePath(), lineNo); recordErr != nil {
					return outcome, lineNo, recordErr
				}
				if _, finishErr := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusFailed, runResultOf(model.RunResultFailed), &class, reason); finishErr != nil {
					return outcome, lineNo, finishErr
				}
				return outcome, lineNo, nil
			} else if account != "" && !strings.EqualFold(account, runCtx.PlanAccount) {
				if locker, canLock := e.sessions.(interface{ LockAccountUsage(string) func() }); canLock && !strings.EqualFold(usageAccount, account) {
					releaseActorUsage = locker.LockAccountUsage(account)
					usageAccount = strings.TrimSpace(account)
				}
				actorSession, actorErr := e.sessions.Current(ctx, account)
				if actorErr != nil {
					class := model.FailureClassActorUnresolved
					reason := "无法登录本节点实际处理人 " + nameOrFallback(name, account) + "：" + userFacingError(actorErr, target.WriteResponse{})
					lineNo := log.Phase("prepare", step.Sequence, attemptNo, reason)
					if recordErr := e.recordPrepareFailure(ctx, runCtx, step, preview, approved.RunCtx.SubmitBranchTargetNodeID,
						attemptNo, approved.IsReplay, startedAt, class, reason, "当前处理人登录会话未就绪，没有发出写请求", log.RelativePath(), lineNo); recordErr != nil {
						return outcome, lineNo, recordErr
					}
					if _, finishErr := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusFailed, runResultOf(model.RunResultFailed), &class, reason); finishErr != nil {
						return outcome, lineNo, finishErr
					}
					return outcome, lineNo, nil
				}
				session = actorSession
				preview.ActorAccount = account
				if name != "" {
					preview.ActorName = name
				}
			}
		}
	}
	log.Phase("prepare", step.Sequence, attemptNo, fmt.Sprintf("当前处理人 %s（登录账号 %s）会话就绪，即将发出 %s", preview.ActorName, preview.ActorAccount, preview.Endpoint))
	// F-035/T09 协议摘要：把代理 ID、批次号、nextAuditorList 条目数、表单数据版本与处理人来源
	// 逐行落 step.log，页面请求与 curl.log 可逐字段对照；敏感值（SID、密码、表单正文）不进日志。
	log.Phase("prepare", step.Sequence, attemptNo, protocolSummary(runCtx, step, preview))
	reportPhase(approved, "prepare", fmt.Sprintf("当前处理人“%s”会话已就绪，即将在“%s”节点发送一次“%s”请求", preview.ActorName, runCtx.Nodes[step.NodeKey].Name, firstNonEmpty(preview.ActionName, string(step.Action))))
	// 当前处理人账号可能不同于计划账号：计划账号锁与处理人锁同时持有到核验结束，
	// 保证实例“已发”视角读取和真实待办写入不会互相覆盖 SID。

	// 阶段 5：发出唯一一次写请求。审批任务 ID 在发送前现场新鲜读取（当前处理人与待办的新鲜复验）。
	// 上报发生在发出之前：本次调用同步阻塞到目标响应返回，指示器在窗口内如实表达 submit 进行中。
	// submit 阶段的目标请求（含写前待办新鲜读取）同样注入步骤作用域，network.log 可按阶段归属。
	ctx = withStepScope(ctx, step.Sequence, attemptNo, "submit")
	reportPhase(approved, "submit", fmt.Sprintf("正在向目标平台提交第 %d 步：“%s”在“%s”节点发送一次“%s”请求，发送后不会重复提交；完成后会重新查看目标平台确认结果", step.Sequence, preview.ActorName, runCtx.Nodes[step.NodeKey].Name, firstNonEmpty(preview.ActionName, string(step.Action))))
	// 写请求前的租约续期：目标存在约 30 秒的慢请求，租约若在写请求期间过期，
	// 另一执行者可能在核验未完成时领取推进权（评审缺陷 12：RenewLease 此前无调用方）。
	// 续期失败说明推进权已易主，本步必须放弃且绝不发出写请求。
	if err := e.runState.RenewLease(ctx, runCtx.PathRun.ID, fencingToken); err != nil {
		return outcome, 0, err
	}
	// 写端点可能明确拒绝过期会话，执行器会在确认请求未进入业务后换新会话重发。
	// 后续事实重读必须沿用真正完成写入的会话，否则会拿旧 SID 做完整退避，
	// 把已经成功的目标写误判成“声明成功但事实不可读”。
	session = e.refreshAndSubmit(ctx, runCtx, step, session, preview, log, reportPhase, approved, attemptNo)
	if !preview.writeSent {
		// 零写入：写请求没有发出（发送前的待办新鲜复验失败或载荷缺失）。
		// 没有发出的请求不存在“结果不确定”——把零写入判成不确定会把无副作用的失败
		// 说成“可能已经写进目标”，还会把用户引向对账；这里按真实分类如实置败。
		outcome.Verdict = string(verdict.OutcomeFailed)
		class := preview.writeErrClass
		if class == "" {
			class = model.FailureClassToolBug
		}
		reason := "执行失败：" + firstResultMessage(preview.writeResponse, preview.writeErr, "写请求没有发出")
		lineNo := log.Phase("settle", step.Sequence, attemptNo, reason)
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
			Basis:      "写请求没有发出，不存在目标侧执行结果",
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
		log.Phase("settle", step.Sequence, attemptNo, "执行失败已记录，路径运行置为失败")
		return outcome, lineNo, nil
	}
	if step.Action == model.ActionAddSign && preview.writeErr == nil {
		proxyID, nodeID, parseErr := parseAddSignWriteData(preview.writeResponse.Data)
		if parseErr != nil {
			log.Phase("verify", step.Sequence, attemptNo, "加签响应未返回新的流程代理标识，后续任务将重新读取实时待办："+parseErr.Error())
		} else {
			outcome.FlowProxyID = proxyID
			outcome.CurrentNodeProxyID = nodeID
		}
	}
	// 写请求已发出：从这一行起 step.log 携带链路 ID，submit 之后的阶段行可与 network.log、curl.log 互查。
	if preview.writeTraceID != "" {
		log.SetTraceID(preview.writeTraceID)
	}
	log.Phase("submit", step.Sequence, attemptNo, submitSummary(preview.writeResponse, preview.writeErr, preview.writeTraceID, preview.writeDurationMs))

	// 发起成功后尽早落库主实例引用（独占不可改写）：即使核验前崩溃，
	// 恢复出的待对账路径运行仍有实例引用可供对账。
	if result, ok := preview.writeResult.(*target.SubmitFlowInstanceResult); ok && result != nil && result.InstanceID != "" {
		if err := e.runState.SetMainInstanceRef(ctx, runCtx.PathRun.ID, result.InstanceID); err != nil {
			return outcome, 0, err
		}
		runCtx.PathRun.MainInstanceRef = result.InstanceID
		outcome.MainInstanceRef = result.InstanceID
		// 实例刚创建：先把实例 ID 补进作用域，名称等紧随其后的核验重读读到再补；
		// 这一步不等待、不额外读目标，目录与日志行不会因为名称未到而延后。
		ctx = e.annotateInstance(ctx, log, result.InstanceID, "", "目标实例名称尚未读取")
	}

	// 阶段 6：事实重读。路径运行先进入核验中——从此刻起崩溃恢复会把该路径置为待对账。
	if err := e.runState.MarkVerifying(ctx, runCtx.PathRun.ID); err != nil {
		return outcome, 0, err
	}
	// 核验阶段的目标请求注入 verify 阶段作用域，与写请求、准备请求在日志中可区分。
	ctx = withStepScope(ctx, step.Sequence, attemptNo, "verify")
	reportPhase(approved, "verify", fmt.Sprintf("第 %d 步的请求已经发出，正在重新查看目标平台的流程节点和待办，确认“%s”是否已经生效；这里不会再次发送动作，确认后保存结果并继续下一步", step.Sequence, firstNonEmpty(preview.ActionName, string(step.Action))))
	// 重读对照一律用目标真实节点标识：目标返回的当前节点与待办都是真实标识，
	// 拿工具侧不透明键去比会永远"待办已消失"，把没生效的写误判成已前进。
	stepTargetNodeID := runCtx.Nodes[step.NodeKey].TargetNodeID
	before := preview.Facts
	before.StepNodeKey = stepTargetNodeID
	after, session, readErr := e.readFactsWithRetry(ctx, runCtx, session, step)
	after.StepNodeKey = stepTargetNodeID
	if readErr == nil && strings.TrimSpace(after.ReadError) == "" {
		// 核验重读同时把实例名称补进日志作用域与 meta.json；写后屏障之后的新事实优先。
		ctx = e.annotateInstanceFromFacts(ctx, runCtx, log, after)
	}
	if readErr != nil || strings.TrimSpace(after.ReadError) != "" {
		// 核验重读失败（工具侧会话失效或目标抖动）：写请求已发出且目标声明成功，但结果未确认。
		// 2026-09-11 用户裁决：工具自己的读取错误不得把运行判成终局——停在本步、保留现场，
		// 不落结论尝试行；用户重新放行只重读核验结果（Reverify），绝不重复发写请求。
		detail := firstNonEmpty(after.ReadError, userFacingError(readErr, target.WriteResponse{}))
		message := "核验读取失败：" + detail + "；已停在本步，重新放行只重新读取结果，不会重复发写请求"
		log.Phase("verify", step.Sequence, attemptNo, message)
		reportPhase(approved, "verify", message)
		outcome.RereadFailed = true
		if backErr := e.runState.BackToRunning(ctx, runCtx.PathRun.ID); backErr != nil {
			return outcome, 0, backErr
		}
		return outcome, 0, nil
	}
	// 核验事实摘要落 step.log：会签等多人场景下「节点待办未清空」是常态，
	// 不把判定依据（当前处理人待办/已办核对结果）写下来，事后无法解释结论怎么来的。
	log.Phase("verify", step.Sequence, attemptNo, fmt.Sprintf("核验事实：实例可读=%v 状态=%s 当前处理人待办=%v（发现能力=%v） 当前处理人已办=%v/%v 实例当前节点=%v",
		after.Found, after.Status, after.CurrentTaskFound, after.CurrentTaskRead,
		after.CompletedTaskRead, after.CompletedTaskFound, after.CurrentNodes))
	reread := ClassifyReread(string(step.Action), stepTargetNodeID, before, after)
	if reread == verdict.RereadUnreadable && preview.writeSent && preview.writeErr == nil {
		// 核验读取失败（工具会话失效/目标抖动导致事实不可读）：写请求已发出且目标声明成功，
		// 但工具没能确认结果。2026-09-11 用户裁决：工具自己的读取错误不得把运行判成终局——
		// 停在本步、保留现场、不落结论尝试行；重新放行只重读核验结果（Reverify），不重发写请求。
		message := "核验读取失败，已停在本步；重新放行只重新读取结果，不会重复发写请求"
		log.Phase("verify", step.Sequence, attemptNo, message)
		reportPhase(approved, "verify", message)
		outcome.RereadFailed = true
		if backErr := e.runState.BackToRunning(ctx, runCtx.PathRun.ID); backErr != nil {
			return outcome, 0, backErr
		}
		return outcome, 0, nil
	}
	if step.Action == model.ActionTransfer {
		reread = e.classifyTransferReread(ctx, runCtx, step, session, preview, after)
	} else if step.Action == model.ActionRetrieve {
		reread = e.classifyRetrieveReread(ctx, runCtx, step, session, preview, after)
	} else if step.Action == model.ActionRollback {
		reread = e.classifyRollbackReread(ctx, runCtx.PlanAccount, runCtx.PathRun.MainInstanceRef, session, preview.Facts, after)
	} else if step.Action == model.ActionAddSign {
		reread = e.classifyAddSignReread(ctx, runCtx, step, session, preview, after, outcome)
	} else if step.Action == model.ActionForward {
		reread, outcome.AuxiliaryInstanceRef = e.classifyForwardReread(ctx, runCtx.PlanAccount, session, preview)
	}
	observation := buildObservation(preview.Endpoint, preview.writeErr, preview.writeResponse, reread)
	observation.Action = string(step.Action)
	observation.ActionFactVerified = ActionFactVerified(step.Action, reread)
	if step.Action == model.ActionForward && reread == verdict.RereadAdvanced {
		observation.ActionFactVerified = true
	}
	verdictResult := verdict.Evaluate(observation)
	userMessage := userResultMessage(step.Action, verdictResult, preview.writeResponse, preview.writeErr, after)
	log.Phase("verify", step.Sequence, attemptNo, "执行结果："+userMessage)

	// 阶段 7：落账。事实表只 INSERT；随后按结论推进路径运行状态。
	lineNo := log.Phase("settle", step.Sequence, attemptNo, settleSummary(verdictResult, userMessage))
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
		Reason:      userMessage,
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
			nextStep := runCtx.Steps[approved.NextIndex+1]
			// 恢复链与系统导航本来就会跨越节点或回到前序节点，不能按“下一步必须仍在路径上”判定偏离；
			// 普通用户步骤和固定尾动作仍做真实节点对照，真实分支偏离继续立即停止。
			if nextStep.Source != model.ActionStepSourceRecovery && nextStep.Source != model.ActionStepSourceNavigation {
				expectedNextKey := nextStep.NodeKey
				expectedNextTarget := ""
				if nodeInfo, ok := runCtx.Nodes[expectedNextKey]; ok {
					expectedNextTarget = strings.TrimSpace(nodeInfo.TargetNodeID)
				}
				if after.Found && len(after.CurrentNodes) > 0 && expectedNextTarget != "" && !containsNode(after.CurrentNodes, expectedNextTarget) {
					outcome.DeviationDetected = true
					log.Phase("settle", step.Sequence, attemptNo, fmt.Sprintf("路径偏离：实际当前节点 %v，已配置路径的下一个预期节点是 %s", after.CurrentNodes, expectedNextTarget))
				}
			}
		}
		if !outcome.NoMoreSteps {
			if err := e.runState.BackToRunning(ctx, runCtx.PathRun.ID); err != nil {
				return outcome, lineNo, err
			}
		}
		// 落账后释放推进权是尽力而为：释放失败只影响下一次领取的即时性，不影响已落账事实。
		_ = e.runState.ReleaseExecution(ctx, runCtx.PathRun.ID, fencingToken)
		log.Phase("settle", step.Sequence, attemptNo, userMessage)
	case verdict.OutcomeFailed:
		outcome.Verdict = string(verdict.OutcomeFailed)
		class := model.FailureClassTargetRejected
		if _, err := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusFailed, runResultOf(model.RunResultFailed), &class,
			"第 "+formatUint(uint64(step.Sequence))+" 步"+userMessage); err != nil {
			return outcome, lineNo, err
		}
		log.Phase("settle", step.Sequence, attemptNo, userMessage+"，路径运行置为失败")
	default:
		// 无法确认执行结果：路径运行进入待确认并停止，避免重复执行真实业务操作。
		outcome.Verdict = string(verdict.OutcomeUncertain)
		class := model.FailureClassWriteUncertain
		if _, err := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusAwaitingReconciliation, runResultOf(model.RunResultAwaitingReconcile), &class,
			"第 "+formatUint(uint64(step.Sequence))+" 步"+userMessage); err != nil {
			return outcome, lineNo, err
		}
		log.Phase("settle", step.Sequence, attemptNo, userMessage+"，路径运行已停止")
	}
	return outcome, lineNo, nil
}

// isSessionRejected 判断一次写调用是否被目标以会话失效拒绝（响应已收到、未进入业务）。
func isSessionRejected(err error) bool {
	var targetErr *target.Error
	return errors.As(err, &targetErr) && targetErr.Kind == target.ErrorSessionExpired
}

// isRequestValidationError 判断错误是否发生在动作写请求发出之前。
// 只有适配层明确标记的本地校验错误才允许把 writeSent 回退为 false。
func isRequestValidationError(err error) bool {
	var validationErr *target.RequestValidationError
	return errors.As(err, &validationErr)
}

// resubmitOnSessionRejected 在写请求被会话失效拒绝后恢复：仅允许一次「换新会话 + 重发」。
// 每次被拒都证明请求未进入业务、无副作用（RESP401/AUTH_401 发生在业务逻辑之前），
// 因此写请求进入业务的次数仍至多一次；再次被拒时立即停步并按原错误上报。
func resubmitOnSessionRejected[R any](
	ctx context.Context, refresh func(account string) (target.Session, error), account string,
	step model.CompiledActionStep, attemptNo int,
	send func(refreshed target.Session) (R, target.WriteResponse, string, error),
	log *StepLog, reportPhase func(ApprovedStep, string, string), approved ApprovedStep,
) (R, target.WriteResponse, string, target.Session, error) {
	var zero R
	log.Phase("submit", step.Sequence, attemptNo,
		"写请求被目标以会话失效拒绝（第_1_次，未进入业务、无副作用），重新取得会话后重发")
	reportPhase(approved, "submit", "目标会话失效已拒绝 1 次（无副作用），正在重新取得会话")
	refreshed, refreshErr := refresh(account)
	if refreshErr != nil {
		return zero, target.WriteResponse{}, "", target.Session{}, refreshErr
	}
	result, response, traceID, err := send(refreshed)
	return result, response, traceID, refreshed, err
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

// taskStatusPendingValue 是目标任务列表里「当前待办」的状态名，与目标协议一致。
// 只有这个状态才需要按当前处理人事实发现任务；已办（done）由取回动作按会话本人读取。
const taskStatusPendingValue = "pending"

// taskSnapshotReader 是目标任务身份的可选能力面；旧的测试假件仍可用 FindDueTaskID，
// 真实客户端必须实现完整快照以提供 jobTaskId、batchNo 和已办任务范围。
type taskSnapshotReader interface {
	FindTaskSnapshot(context.Context, target.Session, string, string, string) (target.TaskSnapshot, error)
}

// flowProxyDocumentReader 是加签更新实例私有流程代理所需的可选能力面；
// 仅真实目标客户端提供，旧测试假件缺少时必须在写请求前失败。
type flowProxyDocumentReader interface {
	ReadFlowProxyDocument(context.Context, target.Session, string) (json.RawMessage, error)
}

// readTaskSnapshot 读取指定实例、节点和状态下的唯一任务身份；代理重建后允许按整实例唯一任务回退。
// 该方法不刷新会话，便于写请求被会话拒绝后用新会话重新读取而不复用旧任务号。
// F-030/T02：同一门禁边界内对 (实例, 状态) 的列表扫描只发一次目标分页请求，
// 后续按节点过滤命中内存结果；写请求后必须调 invalidateTaskListCache 清除，
// 核验永远重新读目标（写后缓存失效屏障，绝不拿旧任务号判定写后状态）。
func (e *Executor) readTaskSnapshot(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, nodeID string, session target.Session, status string) (target.TaskSnapshot, error) {
	instanceID := strings.TrimSpace(runCtx.PathRun.MainInstanceRef)
	nodeID = strings.TrimSpace(nodeID)
	if reader, ok := e.target.(taskSnapshotReader); ok {
		snapshot, err := reader.FindTaskSnapshot(ctx, session, instanceID, nodeID, status)
		if err != nil || strings.TrimSpace(snapshot.JobTaskID) != "" || nodeID == "" || !runCtx.FlowProxyRemapped {
			return snapshot, err
		}
		// updateFlowProxy 会重建实例私有代理，旧节点 ID 可能失效；只有整实例恰好一条任务时才允许回退。
		return reader.FindTaskSnapshot(ctx, session, instanceID, "", status)
	}
	if status == "done" {
		return target.TaskSnapshot{}, fmt.Errorf("目标客户端不支持已办任务快照读取")
	}
	jobTaskID, err := e.target.FindDueTaskID(ctx, session, instanceID, nodeID)
	return target.TaskSnapshot{JobTaskID: jobTaskID, FlowNodeProxyID: nodeID}, err
}

// refreshTaskSnapshot 在任务读取被目标判定为会话失效时换取新会话并重新读取，返回实际使用的会话。
func (e *Executor) refreshTaskSnapshot(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, nodeID string, session target.Session, status string) (target.Session, target.TaskSnapshot, error) {
	// 会话失效后的重登必须跟随当前会话的账号：会签/指定人员链路里当前会话是节点处理人，
	// 静默换成发起人重登会让后续待办读取换一个视角，任务也对不上号（2026-09-11 用户指正）。
	account := strings.TrimSpace(session.Summary.Account)
	if account == "" {
		account = runCtx.PlanAccount
	}
	snapshot, active, err := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, account, session,
		func(callContext context.Context, active target.Session) (target.TaskSnapshot, error) {
			return e.readTaskSnapshot(callContext, runCtx, step, nodeID, active, status)
		})
	return active, snapshot, err
}

// prepareAuditTask 在同意动作写入前读取实时任务号与流程代理，避免会话刷新后沿用预览阶段的旧身份。
func (e *Executor) prepareAuditTask(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, request *target.AuditCurrentTaskRequest, session target.Session, allowRefresh bool) (target.Session, error) {
	nodeID := strings.TrimSpace(runCtx.Nodes[step.NodeKey].TargetNodeID)
	var snapshot target.TaskSnapshot
	var err error
	if allowRefresh {
		session, snapshot, err = e.refreshTaskSnapshot(ctx, runCtx, step, nodeID, session, "pending")
	} else {
		snapshot, err = e.readTaskSnapshot(ctx, runCtx, step, nodeID, session, "pending")
	}
	if err != nil {
		return session, err
	}
	if strings.TrimSpace(snapshot.JobTaskID) == "" {
		return session, fmt.Errorf("目标上已无当前处理人在本节点的待办任务，无法执行%s", actionName(step.Action))
	}
	request.JobTaskID = strings.TrimSpace(snapshot.JobTaskID)
	if flowProxyID := strings.TrimSpace(snapshot.FlowProxyID); flowProxyID != "" {
		request.FlowProxyID = flowProxyID
	}
	return session, nil
}

// prepareActionWrite 按动作类型刷新任务身份，并在加签前读取完整代理文档；allowRefresh=false 用于会话恢复重发。
func (e *Executor) prepareActionWrite(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, request *target.ActionWriteRequest, session target.Session, allowRefresh bool) (target.Session, error) {
	status := "pending"
	if step.Action == model.ActionRetrieve {
		status = "done"
	}
	switch step.Action {
	case model.ActionStorageFormData, model.ActionReject, model.ActionTransfer, model.ActionAddSign, model.ActionRollback, model.ActionRetrieve:
	default:
		return session, nil
	}
	nodeID := strings.TrimSpace(runCtx.Nodes[step.NodeKey].TargetNodeID)
	var snapshot target.TaskSnapshot
	var err error
	if allowRefresh {
		session, snapshot, err = e.refreshTaskSnapshot(ctx, runCtx, step, nodeID, session, status)
	} else {
		snapshot, err = e.readTaskSnapshot(ctx, runCtx, step, nodeID, session, status)
	}
	if err != nil {
		return session, err
	}
	if strings.TrimSpace(snapshot.JobTaskID) == "" {
		return session, fmt.Errorf("目标上已无当前处理人在本节点的%s任务，无法执行%s", taskStatusName(status), actionName(step.Action))
	}
	request.JobTaskID = strings.TrimSpace(snapshot.JobTaskID)
	if flowProxyID := strings.TrimSpace(snapshot.FlowProxyID); flowProxyID != "" {
		request.FlowProxyID = flowProxyID
	}
	if nodeProxyID := strings.TrimSpace(snapshot.FlowNodeProxyID); nodeProxyID != "" {
		request.NodeProxyID = nodeProxyID
	}
	if step.Action == model.ActionTransfer {
		request.BatchNo = strings.TrimSpace(snapshot.BatchNo)
		if request.BatchNo == "" {
			return session, fmt.Errorf("目标任务快照缺少 batchNo，无法执行%s", actionName(step.Action))
		}
		if len(request.UserIDs) == 0 {
			return session, fmt.Errorf("%s未解析到可用目标人员，拒绝发送空 userIds", actionName(step.Action))
		}
	}
	if step.Action != model.ActionAddSign {
		return session, nil
	}
	if len(request.UserIDs) == 0 {
		return session, fmt.Errorf("%s未解析到可用目标人员，拒绝发送空人员明细", actionName(step.Action))
	}
	reader, ok := e.target.(flowProxyDocumentReader)
	if !ok {
		return session, errors.New("目标客户端不支持读取完整流程代理树，无法执行加签")
	}
	type addSignRead struct {
		snapshot target.TaskSnapshot
		proxyID  string
		tree     json.RawMessage
	}
	account := strings.TrimSpace(session.Summary.Account)
	if account == "" {
		account = runCtx.PlanAccount
	}
	read, session, treeErr := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, account, session,
		func(callContext context.Context, active target.Session) (addSignRead, error) {
			// 会话刷新后任务代理也可能随目标上下文变化，必须和树一起重新读取，不能复用旧快照。
			freshSnapshot, snapshotErr := e.readTaskSnapshot(callContext, runCtx, step, nodeID, active, status)
			if snapshotErr != nil {
				return addSignRead{}, snapshotErr
			}
			if strings.TrimSpace(freshSnapshot.JobTaskID) == "" {
				return addSignRead{}, fmt.Errorf("目标上已无当前处理人在本节点的%s任务，无法执行%s", taskStatusName(status), actionName(step.Action))
			}
			proxyID := firstNonEmpty(freshSnapshot.FlowProxyID, request.FlowProxyID)
			if proxyID == "" {
				return addSignRead{}, errors.New("目标任务快照缺少 flowProxyId，无法执行加签")
			}
			tree, readErr := reader.ReadFlowProxyDocument(callContext, active, proxyID)
			if readErr != nil {
				return addSignRead{}, readErr
			}
			return addSignRead{snapshot: freshSnapshot, proxyID: proxyID, tree: tree}, nil
		})
	if treeErr != nil {
		return session, treeErr
	}
	snapshot = read.snapshot
	request.JobTaskID = strings.TrimSpace(snapshot.JobTaskID)
	request.FlowProxyID = read.proxyID
	if nodeProxyID := strings.TrimSpace(snapshot.FlowNodeProxyID); nodeProxyID != "" {
		request.NodeProxyID = nodeProxyID
	}
	request.NodeProxyID = strings.TrimSpace(request.NodeProxyID)
	request.FlowProxyTree, treeErr = target.AppendAddSignUsers(read.tree, request.NodeProxyID, request.UserIDs)
	if treeErr != nil {
		return session, treeErr
	}
	return session, nil
}

// refreshActionTask 在任务级动作发出前重读当前任务身份，并在会话明确失效时只重取一次。
// 任务号、批次和已办/待办状态都不能从编排配置或上一次写请求沿用。
func (e *Executor) refreshActionTask(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, request *target.ActionWriteRequest, session target.Session) (target.Session, error) {
	return e.prepareActionWrite(ctx, runCtx, step, request, session, true)
}

// taskStatusName 返回任务快照状态的中文名称，供零写入错误定位使用。
func taskStatusName(status string) string {
	if status == "done" {
		return "已办"
	}
	return "待办"
}

// actionName 返回动作稳定键的中文名称，避免执行阶段错误只显示内部枚举。
func actionName(action model.ActionKey) string {
	labels := map[model.ActionKey]string{
		model.ActionReject: "不同意", model.ActionTransfer: "移交", model.ActionAddSign: "加签",
		model.ActionRollback: "回退", model.ActionRetrieve: "取回",
	}
	if label := labels[action]; label != "" {
		return label
	}
	return string(action)
}

// protocolSummary 生成一次写请求的协议摘要（F-035/T09）：
// 接口、动作、代理 ID 来源、是否携带批次号、nextAuditorList 条目数、表单基线与处理人来源。
// 本项目为内网系统，日志按原样记录完整请求/响应（含 SID 与表单正文），摘要只做逐行索引，不做脱敏。
func protocolSummary(runCtx RunContext, step model.CompiledActionStep, preview *StepPreview) string {
	parts := []string{"协议摘要", "接口 " + preview.Endpoint, "动作 " + string(step.Action)}
	switch request := preview.request.(type) {
	case *target.SubmitFlowInstanceRequest:
		if request.FormProxyID != "" {
			parts = append(parts, "代理=formProxyId（运行上下文表单代理）")
		} else if request.FlowProxyID != "" {
			parts = append(parts, "代理=flowProxyId（计划流程代理）")
		}
		parts = append(parts, "batchCode="+boolText(request.BatchCode != ""))
		parts = append(parts, fmt.Sprintf("nextAuditorList 条目=%d", len(request.NextAuditors)))
	case *target.AuditCurrentTaskRequest:
		parts = append(parts, "代理=flowProxyId（目标任务快照）")
		parts = append(parts, "batchCode=否")
		parts = append(parts, fmt.Sprintf("nextAuditorList 条目=%d", len(request.NextAuditors)))
	case *target.ActionWriteRequest:
		parts = append(parts, "batchCode=否")
		parts = append(parts, fmt.Sprintf("nextAuditorList 条目=%d", len(request.NextAuditors)))
	}
	source := "发起态表单模型"
	if preview.FormBaseFromInstance {
		source = "目标实例当前数据"
	}
	parts = append(parts, "表单基线="+source)
	parts = append(parts, fmt.Sprintf("处理人=%s（账号 %s）", nameOrFallback(preview.ActorName, "未知"), preview.ActorAccount))
	return strings.Join(parts, "，")
}

// boolText 把布尔值转成中文日志友好的文本。
func boolText(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

// refreshAndSubmit 在发送前完成待办任务 ID 的新鲜读取，然后发出业务写请求。
// 网络连接尚未写出时允许有限重试；请求一旦写出，响应丢失不得重发。
// 只有实际尝试进入目标写请求的路径才置 preview.writeSent。
func (e *Executor) refreshAndSubmit(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, session target.Session, preview *StepPreview, log *StepLog, reportPhase func(ApprovedStep, string, string), approved ApprovedStep, attemptNo int) target.Session {
	switch request := preview.request.(type) {
	case *target.SubmitFlowInstanceRequest:
		started := e.now()
		result, response, traceID, err, attempted := e.runWriteWithNetworkRetry(ctx, step, attemptNo, log, reportPhase, approved,
			func(callContext context.Context, active target.Session) (*target.SubmitFlowInstanceResult, target.WriteResponse, string, error) {
				return e.target.SubmitFlowInstance(callContext, active, *request)
			}, session)
		preview.writeSent = attempted
		// 会话失效拒绝（RESP401/AUTH_401）发生在目标业务逻辑之前：请求没有进入业务、
		// 无副作用（核验重读「明确未变」可证）。这不是「唯一一次写机会」的消耗——
		// 实测目标写端点会话校验与读端点不同步，新登录的会话也可能被写链路拒绝。
		// 因此在同一次尝试内重新取得可用会话并重发，写请求进入业务的次数仍然至多一次；
		// 两次都被会话失效拒绝才如实按失败上报（纪律：绝不盲目重发真实写请求，
		// 这里的重发仅以「目标明确拒绝、证明无副作用」为前提）。
		if isSessionRejected(err) {
			// 只允许一次自动恢复重发；第二次会话失效必须停步，提示可能存在外部会话竞争。
			refreshAccount := strings.TrimSpace(session.Summary.Account)
			if refreshAccount == "" {
				refreshAccount = runCtx.PlanAccount
			}
			result, response, traceID, session, err = resubmitOnSessionRejected(ctx, func(account string) (target.Session, error) { return e.refreshSessionForWrite(ctx, account) }, refreshAccount, step, attemptNo,
				func(refreshed target.Session) (*target.SubmitFlowInstanceResult, target.WriteResponse, string, error) {
					result, response, traceID, err, _ = e.runWriteWithNetworkRetry(ctx, step, attemptNo, log, reportPhase, approved,
						func(callContext context.Context, active target.Session) (*target.SubmitFlowInstanceResult, target.WriteResponse, string, error) {
							return e.target.SubmitFlowInstance(callContext, active, *request)
						}, refreshed)
					return result, response, traceID, err
				}, log, reportPhase, approved)
		}
		preview.writeResult, preview.writeResponse, preview.writeTraceID, preview.writeErr = result, response, traceID, err
		preview.writeDurationMs = e.now().Sub(started).Milliseconds()
	case *target.AuditCurrentTaskRequest:
		started := e.now()
		// 待办按目标真实节点标识精确定位：step.NodeKey 是工具侧不透明键，发给目标永远匹配不上。
		// 写入前和会话恢复重发都重新读取任务，不能复用预览阶段或旧 SID 下的 jobTaskId。
		var err error
		session, err = e.prepareAuditTask(ctx, runCtx, step, request, session, true)
		if err != nil {
			// 待办读取失败（目标抖动或响应形状不符）：写请求未发出，按处理人/待办解析失败如实归类。
			preview.writeErr = err
			preview.writeErrClass = model.FailureClassActorUnresolved
			return session
		}
		result, response, traceID, err, attempted := e.runAuditWriteWithNetworkRetry(ctx, step, attemptNo, log, reportPhase, approved,
			func(callContext context.Context, active target.Session) (*target.AuditCurrentTaskResult, target.WriteResponse, string, error) {
				return e.target.AuditCurrentTask(callContext, active, *request)
			}, session)
		preview.writeSent = attempted
		if isSessionRejected(err) {
			var retrySession target.Session
			var prepareErr error
			refreshAccount := strings.TrimSpace(session.Summary.Account)
			if refreshAccount == "" {
				refreshAccount = runCtx.PlanAccount
			}
			result, response, traceID, session, err = resubmitOnSessionRejected(ctx, func(account string) (target.Session, error) { return e.refreshSessionForWrite(ctx, account) }, refreshAccount, step, attemptNo,
				func(refreshed target.Session) (*target.AuditCurrentTaskResult, target.WriteResponse, string, error) {
					retrySession = refreshed
					retrySession, prepareErr = e.prepareAuditTask(ctx, runCtx, step, request, refreshed, false)
					if prepareErr != nil {
						return nil, target.WriteResponse{}, "", prepareErr
					}
					result, response, traceID, err, _ = e.runAuditWriteWithNetworkRetry(ctx, step, attemptNo, log, reportPhase, approved,
						func(callContext context.Context, active target.Session) (*target.AuditCurrentTaskResult, target.WriteResponse, string, error) {
							return e.target.AuditCurrentTask(callContext, active, *request)
						}, retrySession)
					return result, response, traceID, err
				}, log, reportPhase, approved)
			if retrySession.SID != "" {
				session = retrySession
			}
		}
		preview.writeResult, preview.writeResponse, preview.writeTraceID, preview.writeErr = result, response, traceID, err
		preview.writeDurationMs = e.now().Sub(started).Milliseconds()
	default:
		// 其余已登记动作经统一动作写出口（F-019）：载荷由 buildRequest 构造，端点在白名单内。
		if actionRequest, ok := preview.request.(*target.ActionWriteRequest); ok {
			started := e.now()
			var taskErr error
			session, taskErr = e.refreshActionTask(ctx, runCtx, step, actionRequest, session)
			if taskErr != nil {
				preview.writeErr = taskErr
				preview.writeErrClass = model.FailureClassActorUnresolved
				return session
			}
			response, traceID, err, attempted := e.runActionWriteWithNetworkRetry(ctx, step, attemptNo, log, reportPhase, approved, session, *actionRequest)
			preview.writeSent = attempted
			if isSessionRejected(err) {
				var retrySession target.Session
				var prepareErr error
				refreshAccount := strings.TrimSpace(session.Summary.Account)
				if refreshAccount == "" {
					refreshAccount = runCtx.PlanAccount
				}
				_, response, traceID, session, err = resubmitOnSessionRejected(ctx, func(account string) (target.Session, error) { return e.refreshSessionForWrite(ctx, account) }, refreshAccount, step, attemptNo,
					func(refreshed target.Session) (struct{}, target.WriteResponse, string, error) {
						retrySession = refreshed
						retrySession, prepareErr = e.prepareActionWrite(ctx, runCtx, step, actionRequest, refreshed, false)
						if prepareErr != nil {
							return struct{}{}, target.WriteResponse{}, "", prepareErr
						}
						writeResponse, writeTraceID, writeErr, _ := e.runActionWriteWithNetworkRetry(ctx, step, attemptNo, log, reportPhase, approved, retrySession, *actionRequest)
						return struct{}{}, writeResponse, writeTraceID, writeErr
					}, log, reportPhase, approved)
				if retrySession.SID != "" {
					session = retrySession
				}
			}
			if isRequestValidationError(err) {
				// 适配层在 CallWrite 之前拒绝本地载荷：没有网络请求和目标副作用，
				// 必须走零写入确定失败分支，不能按写结果不确定停在待对账。
				preview.writeSent = false
				preview.writeErrClass = model.FailureClassToolBug
			}
			preview.writeResponse, preview.writeTraceID, preview.writeErr = response, traceID, err
			preview.writeDurationMs = e.now().Sub(started).Milliseconds()
			return session
		}
		preview.writeErr = errors.New("写请求载荷缺失，拒绝发送")
		preview.writeErrClass = model.FailureClassToolBug
	}
	return session
}

// runWriteWithNetworkRetry 只为连接阶段未写出的发起/审批请求提供有限重试。
// 传输阶段一旦不是 connect_refused，结果就交给执行器原有的不确定判定，不得再次发送业务写。
func (e *Executor) runWriteWithNetworkRetry(ctx context.Context, step model.CompiledActionStep, attemptNo int, log *StepLog, reportPhase func(ApprovedStep, string, string), approved ApprovedStep, call func(context.Context, target.Session) (*target.SubmitFlowInstanceResult, target.WriteResponse, string, error), session target.Session) (*target.SubmitFlowInstanceResult, target.WriteResponse, string, error, bool) {
	return RunWithWriteConnectRetry(ctx, e.policy, func(callContext context.Context) (*target.SubmitFlowInstanceResult, target.WriteResponse, string, error) {
		return call(callContext, session)
	}, func(attempt int, nextDelay time.Duration) {
		note := fmt.Sprintf("写请求连接未建立，第 %d 次后将在 %s 重试", attempt, nextDelay)
		log.Phase("submit", step.Sequence, attemptNo, note)
		reportPhase(approved, "submit", note)
	})
}

// runAuditWriteWithNetworkRetry 为审批写请求复用未写出连接重试边界。
func (e *Executor) runAuditWriteWithNetworkRetry(ctx context.Context, step model.CompiledActionStep, attemptNo int, log *StepLog, reportPhase func(ApprovedStep, string, string), approved ApprovedStep, call func(context.Context, target.Session) (*target.AuditCurrentTaskResult, target.WriteResponse, string, error), session target.Session) (*target.AuditCurrentTaskResult, target.WriteResponse, string, error, bool) {
	return RunWithWriteConnectRetry(ctx, e.policy, func(callContext context.Context) (*target.AuditCurrentTaskResult, target.WriteResponse, string, error) {
		return call(callContext, session)
	}, func(attempt int, nextDelay time.Duration) {
		note := fmt.Sprintf("写请求连接未建立，第 %d 次后将在 %s 重试", attempt, nextDelay)
		log.Phase("submit", step.Sequence, attemptNo, note)
		reportPhase(approved, "submit", note)
	})
}

// runActionWriteWithNetworkRetry 为统一动作写出口复用未写出连接重试边界。
func (e *Executor) runActionWriteWithNetworkRetry(ctx context.Context, step model.CompiledActionStep, attemptNo int, log *StepLog, reportPhase func(ApprovedStep, string, string), approved ApprovedStep, session target.Session, request target.ActionWriteRequest) (target.WriteResponse, string, error, bool) {
	_, response, traceID, err, attempted := RunWithWriteConnectRetry(ctx, e.policy, func(callContext context.Context) (struct{}, target.WriteResponse, string, error) {
		response, traceID, err := e.target.ExecuteActionWrite(callContext, session, request)
		return struct{}{}, response, traceID, err
	}, func(attempt int, nextDelay time.Duration) {
		note := fmt.Sprintf("写请求连接未建立，第 %d 次后将在 %s 重试", attempt, nextDelay)
		log.Phase("submit", step.Sequence, attemptNo, note)
		reportPhase(approved, "submit", note)
	})
	return response, traceID, err, attempted
}

// parseAddSignWriteData 提取 updateFlowProxy 成功响应中的新代理与当前节点标识。
// 字段缺失只影响后续上下文刷新，不能把已发出的写请求重新发送或改判为失败。
func parseAddSignWriteData(raw json.RawMessage) (string, string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "", "", errors.New("响应 data 为空")
	}
	var data struct {
		FlowProxyID        string `json:"flowProxyId"`
		CurrentNodeProxyID string `json:"currentNodeProxyId"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", "", fmt.Errorf("响应 data 不是有效对象：%w", err)
	}
	proxyID := strings.TrimSpace(data.FlowProxyID)
	nodeID := strings.TrimSpace(data.CurrentNodeProxyID)
	if proxyID == "" && nodeID == "" {
		return "", "", errors.New("响应 data 缺少 flowProxyId/currentNodeProxyId")
	}
	return proxyID, nodeID, nil
}

// classifyAddSignReread 核对加签后的完整实例代理树是否真的持久化了本次人员明细。
// updateFlowProxy 可能保持流程节点不变，不能只因响应带有代理标识就判成功；必须读回目标树，
// 确认当前节点包含每个本次请求的人员，节点或人员缺失均按未变化处理。
func (e *Executor) classifyAddSignReread(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, session target.Session, preview *StepPreview, after InstanceFacts, outcome StepOutcome) verdict.Reread {
	if after.ReadError != "" {
		return verdict.RereadUnreadable
	}
	request, ok := preview.request.(*target.ActionWriteRequest)
	if !ok {
		return verdict.RereadUnreadable
	}
	reader, ok := e.target.(flowProxyDocumentReader)
	if !ok {
		return verdict.RereadUnreadable
	}
	proxyID := firstNonEmpty(outcome.FlowProxyID, after.FlowProxyID, request.FlowProxyID, runCtx.FlowProxyID)
	nodeID := firstNonEmpty(outcome.CurrentNodeProxyID, request.NodeProxyID)
	if proxyID == "" || nodeID == "" {
		return verdict.RereadUnreadable
	}
	account := strings.TrimSpace(session.Summary.Account)
	if account == "" {
		account = runCtx.PlanAccount
	}
	document, _, err := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, account, session,
		func(callContext context.Context, active target.Session) (json.RawMessage, error) {
			return reader.ReadFlowProxyDocument(callContext, active, proxyID)
		})
	if err != nil {
		return verdict.RereadUnreadable
	}
	updated, err := target.HasAddSignUsers(document, nodeID, request.UserIDs)
	if err != nil {
		return verdict.RereadUnreadable
	}
	if updated {
		return verdict.RereadAdvanced
	}
	return verdict.RereadUnchanged
}

// classifyTransferReread 核对移交后当前处理人的任务是否已经换成新任务。
// 移交不推进节点，单看实例节点会把成功误判为“未变化”；任务 ID 变化或当前处理人不再有待办才算前进。
func (e *Executor) classifyTransferReread(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, session target.Session, preview *StepPreview, after InstanceFacts) verdict.Reread {
	if after.ReadError != "" {
		return verdict.RereadUnreadable
	}
	request, ok := preview.request.(*target.ActionWriteRequest)
	if !ok || strings.TrimSpace(request.JobTaskID) == "" {
		return verdict.RereadUnreadable
	}
	nodeID := strings.TrimSpace(runCtx.Nodes[step.NodeKey].TargetNodeID)
	_, snapshot, err := e.refreshTaskSnapshot(ctx, runCtx, step, nodeID, session, "pending")
	if err != nil {
		return verdict.RereadUnreadable
	}
	if snapshot.JobTaskID == "" || strings.TrimSpace(snapshot.JobTaskID) != strings.TrimSpace(request.JobTaskID) {
		return verdict.RereadAdvanced
	}
	return verdict.RereadUnchanged
}

// classifyRetrieveReread 核对取回后目标是否在原审批节点创建了新的待办。
// 取回会保留原节点位置，因此实例节点和待办节点都可能不变；目标以新的 jobTaskId 表示新批次，
// 必须对照写入前的已办任务号，不能把同节点的新待办误判为“没有变化”。
func (e *Executor) classifyRetrieveReread(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, session target.Session, preview *StepPreview, after InstanceFacts) verdict.Reread {
	if after.ReadError != "" {
		return verdict.RereadUnreadable
	}
	request, ok := preview.request.(*target.ActionWriteRequest)
	if !ok || strings.TrimSpace(request.JobTaskID) == "" {
		return verdict.RereadUnreadable
	}
	nodeID := strings.TrimSpace(runCtx.Nodes[step.NodeKey].TargetNodeID)
	_, snapshot, err := e.refreshTaskSnapshot(ctx, runCtx, step, nodeID, session, "pending")
	if err != nil {
		return verdict.RereadUnreadable
	}
	if snapshot.JobTaskID != "" && strings.TrimSpace(snapshot.JobTaskID) != strings.TrimSpace(request.JobTaskID) {
		return verdict.RereadAdvanced
	}
	return verdict.RereadUnchanged
}

// classifyRollbackReread 确认目标已为本次原任务写入回退审核记录且实例仍在运行。
// 并行分支回退后当前节点可能是策略节点而非直接前一审批节点，因此不能用节点相等判断成功；
// 审核记录的任务关联和 auditStatus 在同一目标事务内写入，能精确证明本次原子动作已完成。
func (e *Executor) classifyRollbackReread(ctx context.Context, account, instanceID string, session target.Session, before, after InstanceFacts) verdict.Reread {
	if after.ReadError != "" || !after.Found {
		return verdict.RereadUnreadable
	}
	if !strings.EqualFold(strings.TrimSpace(after.Status), "run") {
		return verdict.RereadContradictory
	}
	linkID := strings.TrimSpace(before.CurrentTaskLinkID)
	if linkID == "" || strings.TrimSpace(instanceID) == "" {
		return verdict.RereadUnreadable
	}
	reader, ok := e.target.(auditRecordsReader)
	if !ok {
		return verdict.RereadUnreadable
	}
	records, _, err := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, account, session,
		func(callContext context.Context, active target.Session) ([]target.AuditRecordSnapshot, error) {
			return reader.ListAuditRecords(callContext, active, instanceID)
		})
	if err != nil {
		return verdict.RereadUnreadable
	}
	for _, record := range records {
		if strings.TrimSpace(record.FlowJobTaskID) == linkID && strings.EqualFold(strings.TrimSpace(record.AuditStatus), "roll_back_the_previous_level") {
			return verdict.RereadAdvanced
		}
	}
	return verdict.RereadUnchanged
}

// classifyForwardReread 从转发响应中提取辅助实例并按目标实例接口确认其已创建。
// 转发不改变主实例，不能用主实例节点是否变化判断成功。
func (e *Executor) classifyForwardReread(ctx context.Context, account string, session target.Session, preview *StepPreview) (verdict.Reread, string) {
	response := preview.writeResponse.Data
	if len(response) == 0 || string(response) == "null" {
		return verdict.RereadUnreadable, ""
	}
	var raw struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(response, &raw); err != nil || strings.TrimSpace(raw.ID) == "" {
		return verdict.RereadUnreadable, ""
	}
	instanceID := strings.TrimSpace(raw.ID)
	reader, ok := e.target.(interface {
		FindSubmittedFlow(context.Context, target.Session, string) (string, []string, string, []string, bool, error)
	})
	if !ok {
		return verdict.RereadUnreadable, instanceID
	}
	result, _, err := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, account, session,
		func(callContext context.Context, active target.Session) (bool, error) {
			_, _, _, _, found, readErr := reader.FindSubmittedFlow(callContext, active, instanceID)
			return found, readErr
		})
	if err != nil {
		return verdict.RereadUnreadable, instanceID
	}
	if !result {
		return verdict.RereadUnchanged, instanceID
	}
	return verdict.RereadAdvanced, instanceID
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

// sessionWithRetry 取得当前处理人会话：登录与会话获取属只读阶段，允许有界重试与退避。
// 每次重试都如实写进 step.log，不允许出现“看起来只调了一次”的日志。
func (e *Executor) sessionWithRetry(ctx context.Context, runCtx RunContext, log *StepLog, stepNo int, phase string) (target.Session, error) {
	return RunWithConnectRetry(ctx, e.policy, func(callContext context.Context) (target.Session, error) {
		return e.sessions.Current(callContext, runCtx.PlanAccount)
	}, func(attempt int, nextDelay time.Duration) {
		log.Phase(phase, stepNo, 1, fmt.Sprintf("会话获取第 %d 次失败，%s 后重试", attempt, nextDelay))
	})
}

// sessionRefreshAllowed 判断 Current 失败后是否允许再刷新一次会话；登录、权限和业务拒绝是确定失败，
// 不能通过重复登录掩盖目标原始结果，网络瞬断和会话失效才交给 Refresh 处理。
func sessionRefreshAllowed(err error) bool {
	if err == nil || target.IsKind(err, target.ErrorLoginRejected) || target.IsKind(err, target.ErrorPermissionDenied) {
		return false
	}
	var rejection *target.BusinessRejection
	return !errors.As(err, &rejection)
}

// InstanceStillAtStepNode 只读探测：实例当前节点是否仍停在编译步骤的节点上。
// 会签节点需要全部处理人各自审批才前进：同意成功后实例仍停在本节点，就说明还有
// 其他处理人未审批，控制层据此用新发现的当前处理人原步重跑。探测用计划账号会话——
// 实例「已发」事实是发起人视角的数据（2026-09-11 用户强调的硬约束）。
func (e *Executor) InstanceStillAtStepNode(ctx context.Context, runCtx RunContext, step model.CompiledActionStep) (bool, error) {
	info, ok := runCtx.Nodes[step.NodeKey]
	if !ok || strings.TrimSpace(info.TargetNodeID) == "" {
		return false, nil
	}
	if locker, ok := e.sessions.(interface{ LockAccountUsage(string) func() }); ok {
		release := locker.LockAccountUsage(runCtx.PlanAccount)
		defer release()
	}
	session, err := e.sessions.Current(ctx, runCtx.PlanAccount)
	if err != nil {
		return false, err
	}
	facts, _, err := e.readFactsWithRetry(ctx, runCtx, session, step)
	if err != nil {
		return false, err
	}
	return facts.Found && containsNode(facts.CurrentNodes, info.TargetNodeID), nil
}

// assigneeAccount 把目标实时待办的实际处理人（用户 ID）解析为登录账号与姓名。
// 人员目录按公司全量分页读取并就地缓存一次调用；查不到说明目录里没有该用户，
// 调用方必须阻断本步，绝不能回退成计划账号冒充审批。
// 目录读取沿用读路径会话纪律（readOnlyWithSessionRetry）：目标会话可能在门禁读取与
// 账号解析之间被作废（实测返回 RESP401「SID已失效!」），失效只允许重登并重放同一只读请求
// （有界、重登间退避防触发目标会话限制），不能把会话失效直接放大成处理人解析失败。
func (e *Executor) assigneeAccount(ctx context.Context, session target.Session, assigneeUserID string) (string, string, error) {
	resolver, ok := e.target.(userAccountResolver)
	if !ok {
		return "", "", errors.New("目标客户端不支持人员目录账号解析")
	}
	accounts, _, err := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, session.Summary.Account, session,
		func(callContext context.Context, active target.Session) (map[string]string, error) {
			return resolver.UserAccountsByID(callContext, active, []string{assigneeUserID})
		})
	if err != nil {
		return "", "", err
	}
	account, ok := accounts[assigneeUserID]
	if !ok || strings.TrimSpace(account) == "" {
		return "", "", errors.New("人员目录中没有该用户的登录账号")
	}
	return strings.TrimSpace(account), "", nil
}

// recordPrepareFailure 把 prepare 阶段（写请求发出前）的步骤失败落一条失败事实行。
// 与门禁阻塞同一纪律：失败原因原文进事实与界面，副作用如实记 none；
// 没有这条事实行，详情只能把失败标记回退到上一个成功节点，用户找不到真正失败的节点，
// 重试装填也会把游标错放回已成功的步骤。
func (e *Executor) recordPrepareFailure(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, preview *StepPreview, submitBranchTargetNodeID string, attemptNo int, isReplay bool, startedAt time.Time, class model.FailureClass, reason, basis, logPath string, logLine uint64) error {
	record := model.RunStep{
		PathRunID: runCtx.PathRun.ID, StepNo: step.Sequence, Source: string(step.Source),
		Action: string(step.Action), NodeKey: step.NodeKey, ActorSummary: preview.ActorName,
		Status: model.RunStepFailed, StartedAt: startedAt, FinishedAt: e.now(),
		GateSnapshot: gateSnapshotJSON(preview, submitBranchTargetNodeID),
	}
	attempt := model.RunStepAttempt{
		PathRunID: runCtx.PathRun.ID, AttemptNo: attemptNo, Verdict: string(verdict.OutcomeFailed),
		SideEffect: string(verdict.SideEffectNone), Reason: reason, Basis: basis,
		FailureClass: &class, LogPath: logPath, LogLine: logLine,
		DurationMs: e.now().Sub(startedAt).Milliseconds(), IsReplay: isReplay,
	}
	_, err := e.facts.RecordStepAttempt(ctx, record, attempt, e.now())
	return err
}

// resolveTaskSnapshotForStep 发现并读取本步的精确任务快照，返回真正命中任务的处理人会话。
// 查询顺序固定（F-031/T03）：当前处理人事实（「已发」currentAuditUserInfo）→ 真实用户 ID/会话
// → 精确实例任务 → 精确节点上的唯一 jobTaskId。
// 配置里的 NextNodeAuditors 只表示下一次提交/审批要发送的选人参数，不是当前节点已经产生的待办处理人，
// 因此绝不参与当前任务发现：不再为配置候选人逐一登录、逐一查任务。返回的 session 是真正命中任务
// 的处理人会话，供后续代理树/审核记录读取继续使用。
func (e *Executor) resolveTaskSnapshotForStep(ctx context.Context, runCtx RunContext, session target.Session, step model.CompiledActionStep, nodeID, status string, facts *InstanceFacts, allowDiscovery bool) (target.TaskSnapshot, string, string, target.Session, error) {
	// 写前准备的待办动作只认当前处理人事实（F-031/T03，审查 P1）：事实里没有本节点处理人、
	// 或处理人账号/待办核对不命中时，一律停在当前步骤返回空快照，由门禁按「没有当前待办」阻断。
	// 绝不回退到计划账号或会话本人的待办读取：那会让计划账号冒充当前处理人，
	// 放行一次本不属于它的审批（计划账号读到自己名下的历史待办时尤其危险）。
	if allowDiscovery && strings.TrimSpace(status) == taskStatusPendingValue {
		if facts == nil {
			return target.TaskSnapshot{}, "", "", session, nil
		}
		snapshot, userID, userName, actorSession, found, diag := e.switchToCurrentHandler(ctx, runCtx, session, step, nodeID, status, *facts)
		if diag != "" {
			facts.AssigneeDiag = diag
		}
		if found {
			return snapshot, userID, userName, actorSession, nil
		}
		return target.TaskSnapshot{}, "", "", session, nil
	}
	// 当前会话本人的精确实例任务：写后核验（allowDiscovery=false）与已办读取（取回）都靠它。
	// 写后核验用的是真正执行过写请求的当前处理人会话，只核对本人任务，不存在冒充问题。
	snapshot, err := e.readTaskSnapshot(ctx, runCtx, step, nodeID, session, status)
	if err != nil {
		return target.TaskSnapshot{}, "", "", session, err
	}
	return snapshot, "", "", session, nil
}

// switchToCurrentHandler 从实例事实的 CurrentHandlers（「已发」列表 currentAuditUserInfo）
// 提取本节点当前待处理人员，逐个切换会话重读待办，命中任务即返回该处理人的会话与身份。
// 人员匹配优先用户 ID（bizIds），目标不再返回 ID 的「指定人员」节点按手机号或姓名匹配目录；
// 未命中时返回 found=false 与中文原因（写进事实诊断），由调用方按既有门禁失败处理，
// 绝不猜测人员或冒用计划账号审批他人任务。
func (e *Executor) switchToCurrentHandler(ctx context.Context, runCtx RunContext, session target.Session, step model.CompiledActionStep, nodeID, status string, facts InstanceFacts) (snap target.TaskSnapshot, userID, userName string, sess target.Session, found bool, diag string) {
	sess = session
	resolver, hasResolver := e.target.(handlerAccountMatcher)
	if !hasResolver {
		return target.TaskSnapshot{}, "", "", sess, false, "目标客户端不支持人员目录解析，无法发现当前处理人"
	}
	handler, ok := currentHandlerForNode(facts, nodeID)
	if !ok {
		instanceID := strings.TrimSpace(runCtx.PathRun.MainInstanceRef)
		if len(facts.CurrentHandlers) == 0 {
			// 目标响应整段缺少 currentAuditUserInfo：把缺失字段与实例 ID 一起说清楚，
			// 由门禁停在本步（F-031/T04），绝不用配置候选人顶替当前处理人。
			return target.TaskSnapshot{}, "", "", sess, false, fmt.Sprintf("目标响应缺少 currentAuditUserInfo（实例 %s），无法确定本节点当前处理人", instanceID)
		}
		return target.TaskSnapshot{}, "", "", sess, false, fmt.Sprintf("实例 %s 的 currentAuditUserInfo 里没有本节点的处理人信息", instanceID)
	}
	matches, _, err := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, session.Summary.Account, session,
		func(callContext context.Context, active target.Session) ([]target.HandlerAccount, error) {
			return resolver.MatchHandlerAccounts(callContext, active, handler)
		})
	if err != nil {
		return target.TaskSnapshot{}, "", "", sess, false, "当前处理人账号解析失败：" + err.Error()
	}
	// 解析结果按目标真实用户 ID 去重（F-031/T03）：同一个处理人可能同时命中 bizId、
	// 手机号和姓名三条匹配键，不去重就会对同一个人重复发同一视角的任务查询，
	// 既放大目标请求也把「一次事实核对」伪装成多次。
	matches = dedupeHandlerAccounts(matches)
	tried := []string{}
	// F-030 评审 P2：优先用计划会话按处理人用户 ID 视角查询（不新增端点），
	// 命中者才登录其会话；客户端不支持视角查询时回退旧的逐人登录重读。
	instanceID := strings.TrimSpace(runCtx.PathRun.MainInstanceRef)
	viewReader, hasView := e.target.(taskSnapshotViewReader)
	if hasView && instanceID != "" {
		for _, match := range matches {
			if strings.TrimSpace(match.UserID) == "" {
				continue
			}
			snapshots, _, viewErr := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, session.Summary.Account, session,
				func(callContext context.Context, active target.Session) ([]target.TaskSnapshot, error) {
					return viewReader.ListTaskSnapshotsForUser(callContext, active, instanceID, status, match.UserID)
				})
			if viewErr != nil {
				continue
			}
			wantNode := strings.TrimSpace(nodeID)
			for _, snapshot := range snapshots {
				if wantNode != "" && strings.TrimSpace(snapshot.FlowNodeProxyID) != wantNode {
					continue
				}
				// 命中：登录该处理人会话供后续读取；登录失败退回计划会话（写路径会重新解析）。
				snapshot.PendingUserID = match.UserID
				name := firstNonEmpty(match.Name, session.Summary.DisplayName)
				snapshot.PendingUserName = name
				if account := strings.TrimSpace(match.Account); account != "" {
					if actorSession, sessionErr := e.sessions.Current(ctx, account); sessionErr == nil {
						return snapshot, match.UserID, name, actorSession, true, ""
					}
				}
				return snapshot, match.UserID, name, session, true, ""
			}
		}
	}
	for _, match := range matches {
		if strings.TrimSpace(match.Account) == "" {
			continue
		}
		tried = append(tried, match.Account)
		var actorSession target.Session
		var snapshot target.TaskSnapshot
		var err error
		func() {
			// 候选账号的登录、待办读取也必须串行；计划账号锁由调用方持有，
			// 这里按“计划 -> 候选”顺序获取，避免多账号执行时锁顺序反转。
			var release func()
			if locker, ok := e.sessions.(interface{ LockAccountUsage(string) func() }); ok &&
				!strings.EqualFold(strings.TrimSpace(match.Account), strings.TrimSpace(runCtx.PlanAccount)) {
				release = locker.LockAccountUsage(match.Account)
				defer release()
			}
			actorSession, err = e.sessions.Current(ctx, match.Account)
			if err != nil && sessionRefreshAllowed(err) {
				actorSession, err = e.sessions.Refresh(ctx, match.Account)
			}
			if err != nil {
				return
			}
			snapshot, err = e.readTaskSnapshot(ctx, runCtx, step, nodeID, actorSession, status)
		}()
		if err != nil || strings.TrimSpace(snapshot.JobTaskID) == "" {
			continue
		}
		// PendingUserID 必须回填目标目录里的真实用户 ID：写请求与后续目录查询都按它解析，
		// 绝不能用匹配键（姓名/手机号）冒充用户 ID（实测会退化成「人员目录中没有该用户的登录账号」）。
		snapshot.PendingUserID = match.UserID
		name := firstNonEmpty(match.Name, actorSession.Summary.DisplayName)
		snapshot.PendingUserName = name
		return snapshot, match.UserID, name, actorSession, true, ""
	}
	if len(tried) == 0 {
		return target.TaskSnapshot{}, "", "", sess, false, "当前处理人未能解析出登录账号"
	}
	return target.TaskSnapshot{}, "", "", sess, false, "已依次核对当前处理人（" + strings.Join(tried, "、") + "）的待办，均未发现本节点待办"
}

// currentHandlerForNode 从实例事实里取本节点的待办处理人信息。
func currentHandlerForNode(facts InstanceFacts, nodeID string) (target.NodeCurrentHandler, bool) {
	nodeID = strings.TrimSpace(nodeID)
	for _, handler := range facts.CurrentHandlers {
		if strings.TrimSpace(handler.NodeID) == nodeID {
			return handler, true
		}
	}
	return target.NodeCurrentHandler{}, false
}

// taskSnapshotViewReader 是按指定用户视角读任务列表的可选能力面（真实客户端提供）。
type taskSnapshotViewReader interface {
	ListTaskSnapshotsForUser(ctx context.Context, active target.Session, instanceID, taskStatus, queryUserID string) ([]target.TaskSnapshot, error)
}

// dedupeHandlerAccounts 按目标真实用户 ID 去重待处理人员。
// 同一处理人在目录里可能同时命中 bizId、手机号与姓名三条匹配键，未去重会对同一个人
// 重复发同一视角的任务查询；用户 ID 为空（目录没有返回 ID）时保留原样，由调用方按匹配键继续核对。
func dedupeHandlerAccounts(matches []target.HandlerAccount) []target.HandlerAccount {
	result := make([]target.HandlerAccount, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		id := strings.TrimSpace(match.UserID)
		if id != "" {
			if seen[id] {
				continue
			}
			seen[id] = true
		}
		result = append(result, match)
	}
	return result
}

// nameOrFallback 有名字用名字，否则用 ID 兜底，供阻断文案指向具体人员。
func nameOrFallback(name, fallback string) string {
	if strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return strings.TrimSpace(fallback)
}

// handlerAccountMatcher 是当前处理人账号匹配能力的最小接口：按 bizId/手机号/姓名解析登录账号。
type handlerAccountMatcher interface {
	MatchHandlerAccounts(ctx context.Context, active target.Session, handler target.NodeCurrentHandler) ([]target.HandlerAccount, error)
}

// userAccountResolver 是人员目录账号解析能力的最小接口，由目标适配层实现。
type userAccountResolver interface {
	UserAccountsByID(ctx context.Context, active target.Session, ids []string) (map[string]string, error)
}

// readOnlyWithSessionRetry 执行目标只读操作；会话失效时先刷新会话再立即重读，其他临时错误按配置退避。
// 该函数只接受读取回调，明确隔离写请求，避免把已送达的业务动作放进重试循环。
func readOnlyWithSessionRetry[T any](ctx context.Context, policy RetryPolicy, sessions SessionProvider, account string, session target.Session, call func(context.Context, target.Session) (T, error)) (T, target.Session, error) {
	var zero T
	attempts := policy.Attempts
	if attempts < 1 {
		attempts = 1
	}
	active := session
	sessionRefreshed := false
	for attempt := 1; attempt <= attempts; attempt++ {
		callContext := target.WithRetryAttempt(ctx, attempt > 1, attempt)
		value, err := call(callContext, active)
		if err == nil {
			return value, active, nil
		}
		if target.IsKind(err, target.ErrorSessionExpired) {
			if sessions == nil {
				return zero, active, err
			}
			if sessionRefreshed {
				return zero, active, target.NewError(target.ErrorSessionExpired,
					errors.New("新会话仍然失效，可能存在浏览器或其他进程的外部会话竞争"))
			}
			sessionRefreshed = true
			// 连续刷新之间强制退避：实测高频重登会触发目标平台对账号的会话限制
			// （新发的 SID 连只读都秒级失效），把瞬断放大成账号级不可用；退避给目标恢复窗口。
			delay := time.Duration(attempt) * time.Second
			if policy.Sleep != nil {
				policy.Sleep(delay)
			} else {
				select {
				case <-ctx.Done():
					return zero, active, ctx.Err()
				case <-time.After(delay):
				}
			}
			refreshed, refreshErr := sessions.Refresh(ctx, account)
			if refreshErr != nil {
				return zero, active, refreshErr
			}
			if strings.TrimSpace(refreshed.SID) == "" {
				return zero, active, errors.New("目标会话刷新后没有返回有效 SID")
			}
			active = refreshed
			// 会话恢复是独立于网络重试预算的安全重放：即使调用方把只读网络预算设为 1，
			// 也必须完成一次换新会话后的重读，不能把会话失效误报成不可恢复失败。
			if attempt >= attempts {
				attempts = attempt + 1
			}
			continue
		}
		if attempt >= attempts || !retryableTargetError(err) {
			return zero, active, err
		}
		delay := policy.backoff(attempt)
		if policy.Sleep != nil {
			policy.Sleep(delay)
			continue
		}
		select {
		case <-ctx.Done():
			return zero, active, ctx.Err()
		case <-time.After(delay):
		}
	}
	return zero, active, errors.New("目标只读请求重试未执行")
}

// readFactsWithRetry 读取主实例事实，并按动作读取目标专用结果接口。
func (e *Executor) readFactsWithRetry(ctx context.Context, runCtx RunContext, session target.Session, step model.CompiledActionStep) (InstanceFacts, target.Session, error) {
	// 会话失效后的重登必须跟随当前会话的账号：核验阶段传的是当前处理人会话，若静默换成
	// 计划账号重登，会签场景下其他处理人未完成的待办会被当成「本步写未生效」
	// （2026-09-11 实测：审核人3会签第一人审批成功却被判结果待确认）。
	// 实例「已发」列表的发起人视角读取由 readInstanceFacts 内部单独切回计划账号。
	account := strings.TrimSpace(session.Summary.Account)
	if account == "" {
		account = runCtx.PlanAccount
	}
	facts, active, err := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, account, session,
		func(callContext context.Context, active target.Session) (InstanceFacts, error) {
			// 事实重读要与目标返回的真实节点标识对照，因此传真实标识而不是工具侧不透明键。
			return e.readInstanceFacts(callContext, runCtx, active, step)
		})
	if err != nil {
		// 预算耗尽仍读不到：把读取失败随事实带回（ReadError 非空 → 核验判不可读、对账走读取失败降级），
		// 绝不把零值事实当「读到了且无痕迹」用——那会把读取失败误判成「已前进」或「明确未变」（评审 P1）。
		facts.ReadError = target.UserFacingErrorMessage(target.WriteResponse{}, err)
		return facts, active, err
	}
	return facts, active, nil
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
	facts, _, err := e.readFactsWithRetry(ctx, runCtx, session, model.CompiledActionStep{})
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
	if locker, ok := e.sessions.(interface{ LockAccountUsage(string) func() }); ok {
		release := locker.LockAccountUsage(runCtx.PlanAccount)
		defer release()
	}
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
	after, session, err := e.readFactsWithRetry(ctx, runCtx, session, model.CompiledActionStep{})
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
			// 已办记录必须按「当前真实操作账号」核对，不能默认用计划账号（发起人）视角：
			// 会签/指定人员链路里本步写是以当前处理人身份发出的，用发起人视角查已办
			// 会把已经生效的写读成「没有已办痕迹」，而对账「未生效」是唯一会触发重放的结论。
			// 视角为空（会话没有用户 ID）时如实记录，不冒充任何人。
			operatorUserID := strings.TrimSpace(session.UserID)
			operatorAccount := strings.TrimSpace(session.Summary.Account)
			if operatorAccount == "" {
				operatorAccount = runCtx.PlanAccount
			}
			if operatorUserID == "" {
				log.Phase("verify", stepNo, 1, "已办记录核对没有可用的真实操作账号，按当前会话视角读取并如实记录空视角")
			}
			if found, _, readErr := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, operatorAccount, session,
				func(callContext context.Context, active target.Session) (bool, error) {
					return reader.FindDoneTaskOnNode(callContext, active, instanceRef, nodeID, operatorUserID)
				}); readErr == nil {
				facts.DoneRecordsRead, facts.DoneRecordFound = true, found
			} else {
				log.Phase("verify", stepNo, 1, "已办记录读取失败，对账按证据缺失降级："+userFacingError(readErr, target.WriteResponse{}))
			}
		}
		if reader, ok := e.target.(auditTraceReader); ok {
			trace, _, readErr := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, runCtx.PlanAccount, session,
				func(callContext context.Context, active target.Session) (auditTrace, error) {
					found, total, err := reader.FindAuditTraceOnNode(callContext, active, instanceRef, nodeID)
					return auditTrace{found: found, total: total}, err
				})
			if readErr == nil {
				facts.ActionTraceRead, facts.ActionTraceFound, facts.ActionTraceTotal = true, trace.found, trace.total
			} else {
				log.Phase("verify", stepNo, 1, "审核记录读取失败，对账按证据缺失降级："+userFacingError(readErr, target.WriteResponse{}))
			}
		}
	}
	return facts, nil
}

// doneRecordReader 与 auditTraceReader 是对账两个新增维度的最小能力面。
// 用接口断言而不是加进 TargetClient：假件与既有装配不必被迫实现它们，读不到时对账如实降级。
type doneRecordReader interface {
	FindDoneTaskOnNode(ctx context.Context, active target.Session, instanceID, nodeProxyID, executorUserID string) (bool, error)
}

type auditTraceReader interface {
	FindAuditTraceOnNode(ctx context.Context, active target.Session, instanceID, nodeProxyID string) (bool, int, error)
}

// auditTrace 是审核记录维度的读取结果。
type auditTrace struct {
	found bool
	total int
}

// TargetSkippedClassificationForTest 暴露跳过/阻塞分型判定，供 test 目录锁定 F-034 评审 #2 行为：
// isSkip=true 且越过本节点 → 跳过；isSkip 未声明/false 且越过 → 阻塞；run_node_choose 越过 → 阻塞。
func TargetSkippedClassificationForTest(runCtx RunContext, step model.CompiledActionStep, facts InstanceFacts) (string, string) {
	action, reason, _, _ := targetSkippedStepReason(runCtx, step, facts)
	return action, reason
}
