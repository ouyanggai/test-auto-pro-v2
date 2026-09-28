package step

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/engine/verdict"
	"test-auto-pro-v2/internal/logging"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/runtimeexec"
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
	// runtime 执行能力：nil 时保持既有 Go 写路径；配置后七个表单动作经独立执行服务，
	// 服务不可用时写前阻塞，绝不回退 Go 写造成双写。
	runtimeRunner RuntimeRunner
	// runtimeBaseURL 是页面装载载荷的目标网关地址；空串=页面用相对路径请求。
	runtimeBaseURL string
	// handlerWait/handlerPollInterval 是「目标节点已到达但处理人尚未生成」的等待预算与复查间隔，
	// 全部来自配置不写死；目标是人工流转系统，生成下一处理人可能有明显延迟，等够了再判。
	handlerWait         time.Duration
	handlerPollInterval time.Duration
}

// LogFactory 由装配层注入：复用 F-013 的运行目录路由打开 step.log。
type LogFactory func(runCtx RunContext) *StepLog

// NewExecutor 创建一步执行器；重试预算全部来自配置。
func NewExecutor(targetClient TargetClient, sessions SessionProvider, runState RunStateControl, facts RunFactsStore, runConfig config.RunConfig, now func() time.Time) *Executor {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Executor{
		runtimeRunner: nil,

		target:   targetClient,
		sessions: sessions,
		runState: runState,
		facts:    facts,
		policy: RetryPolicy{
			Attempts: runConfig.ReadOnlyRetryAttempts, BaseDelay: runConfig.ReadOnlyRetryBaseDelay, MaxDelay: runConfig.ReadOnlyRetryMaxDelay,
			WriteConnectAttempts: runConfig.WriteConnectRetryAttempts, WriteConnectBaseDelay: runConfig.WriteConnectRetryBaseDelay,
			WriteConnectMaxDelay: runConfig.WriteConnectRetryMaxDelay, Now: now,
		},
		now:                 now,
		handlerWait:         orDefaultDuration(runConfig.HandlerGenerationWait, config.DefaultHandlerGenerationWait),
		handlerPollInterval: orDefaultDuration(runConfig.HandlerGenerationPollInterval, config.DefaultHandlerGenerationPollInterval),
	}
}

// orDefaultDuration 在配置为零值或负值时回退默认值，避免等待预算退化成 0。
func orDefaultDuration(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

// SetLogFactory 注入 step.log 工厂；必须在首次使用前调用。
// EnableRuntimeExecution 配置可选的 runtime 执行能力：
// runner 为 nil 时全部动作保持既有 Go 写路径；配置后七个表单动作经 runtime 执行。
// 执行服务不可用时写前阻塞，绝不回退 Go 写路径。
func (e *Executor) EnableRuntimeExecution(runner RuntimeRunner, loadBaseURL string) {
	e.runtimeRunner = runner
	e.runtimeBaseURL = loadBaseURL
}

// shouldRunViaRuntime 判定本步是否走 runtime：原生页面支持的渲染类型统一承载七个表单动作。
// Go 直连动作（加签/移交/回退/取回/撤回/催办/关注/取消关注）与系统节点永不经 runtime。
func (e *Executor) shouldRunViaRuntime(runCtx RunContext, step model.CompiledActionStep) bool {
	if e.runtimeRunner == nil {
		return false
	}
	switch step.Action {
	case model.ActionSaveDraft, model.ActionSubmit, model.ActionStorageFormData,
		model.ActionApprove, model.ActionReject, model.ActionResubmit:
		return true
	default:
		return false
	}
}

// runViaRuntime 经 runtime 执行本步动作并回填 preview 写事实；返回 true 表示已接管。
// 结果映射（设计 T012）：success 交给既有事实核验裁决；uncertain/timeout → 结果待确认；
// target_rejected/business_failed → 确定失败（事实核验仍会复核）；failed/unsupported → 零写入失败。
func (e *Executor) runViaRuntime(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, session target.Session, preview *StepPreview, log *StepLog, approved ApprovedStep) bool {
	if preview.runtimeHandled {
		return true
	}
	reportPhase(approved, "submit", fmt.Sprintf("正在经无头表单运行时执行\"%s\"：真实页面组件将按目标顺序执行业务与流程请求，完成后仍会重读目标平台核验", firstNonEmpty(preview.ActionName, string(step.Action))))
	action := string(step.Action)
	if err := e.runtimeRunner.Health(ctx); err != nil {
		preview.writeSent = false
		preview.writeErr = fmt.Errorf("执行服务不可用，拒绝执行且不回退旧接口：%w", err)
		preview.writeErrClass = model.FailureClassGateBlocked
		preview.runtimeHandled = true
		log.Phase("submit", step.Sequence, approved.Attempt, preview.writeErr.Error())
		return true
	}
	// 转发接收人是目标转发弹窗的强制输入；缺失时在任何目标读取前阻塞。
	receiverID := ""
	receiverName := ""
	if step.Action == model.ActionForward {
		receiverID = firstActionPersonID(runCtx.ActionPersonIDs[actionPersonKey(step.NodeKey, step.Action)])
		if strings.TrimSpace(receiverID) == "" {
			preview.writeSent = false
			preview.writeErr = fmt.Errorf("转发动作未配置接收人，拒绝经 runtime 执行")
			preview.writeErrClass = model.FailureClassGateBlocked
			preview.runtimeHandled = true
			log.Phase("submit", step.Sequence, approved.Attempt, preview.writeErr.Error())
			return true
		}
		receiverName = personNameFor(runCtx, receiverID)
	}
	// 审批族动作的目标载荷硬依赖任务号（audit 载荷 data.jobTaskId）：

	// 与 Go 写路径同一纪律——发送前按当前处理人会话现场刷新任务快照，
	// 读取失败或目标已无待办一律阻塞，绝不带空任务号进入 runtime。
	jobTaskID := ""
	switch step.Action {
	case model.ActionApprove, model.ActionReject, model.ActionStorageFormData, model.ActionForward:
		// 实例私有代理在加签、恢复或条件重建后会换掉节点 ID；门禁已经用 currentAuditUserInfo
		// 找到了本次真实任务，因此提交前刷新必须优先沿用该任务节点，不能退回编译期旧 ID。
		nodeID := firstNonEmpty(
			strings.TrimSpace(preview.Facts.CurrentTaskNodeID),
			strings.TrimSpace(runCtx.Nodes[step.NodeKey].TargetNodeID),
		)
		if nodeID == "" {
			// 实例级动作（转发/催办等）不绑定配置节点，NodeKey 为空，按空节点查待办必然查空，
			// 于是每一步都报「目标上已无当前处理人在本节点的待办任务」而停在写前
			// （2026-09-21 运行 78 路径 1 第 43 步实测：前 40 步全部成功，只剩收尾的转发失败）。
			// 这类动作作用于**实例的当前活动待办**，因此节点标识取自实例事实里已核实的当前任务节点。
			// 只做标识回退，不放宽待办要求：仍然要求真实读到 jobTaskId，读不到一样在写前阻塞。
			nodeID = strings.TrimSpace(preview.Facts.CurrentTaskNodeID)
		}
		// 视角用户取自预览阶段已核实的当前处理人事实（门禁用同一个 ID 查到过该待办）；
		// 没有事实时为空串，由 refreshPendingTaskSnapshotForAssignee 回退本人视角并留下原因。
		assigneeUserID := strings.TrimSpace(preview.Facts.CurrentTaskAssigneeID)
		_, snapshot, snapErr := e.refreshPendingTaskSnapshotForAssignee(ctx, runCtx, step, nodeID, session, assigneeUserID)
		if snapErr != nil {
			preview.writeSent = false
			preview.writeErr = fmt.Errorf("runtime 执行前读取当前任务失败：%w", snapErr)
			preview.writeErrClass = model.FailureClassGateBlocked
			preview.runtimeHandled = true
			log.Phase("submit", step.Sequence, approved.Attempt, preview.writeErr.Error())
			return true
		}
		if strings.TrimSpace(snapshot.JobTaskID) == "" {
			preview.writeSent = false
			preview.writeErr = fmt.Errorf("目标上已无当前处理人在本节点的待办任务，无法经 runtime 执行%s", actionName(step.Action))
			preview.writeErrClass = model.FailureClassGateBlocked
			preview.runtimeHandled = true
			log.Phase("submit", step.Sequence, approved.Attempt, preview.writeErr.Error())
			return true
		}
		jobTaskID = strings.TrimSpace(snapshot.JobTaskID)
		// 说明：转发所需的"执行者=当前处理人"口径统一由 prepare 阶段解析并写入
		// preview.ActorAccount / preview.ActorName（实例级动作与任务级动作同一条路径），
		// 这里不再单独切换会话——两处各切一次会让身份来源分叉，难以核对。
	}
	// 发起人身份必须是计划账号的真实用户事实（工具发起的实例一律由计划账号发起）。
	// 主实例引用是实例 ID，把它填进发起人用户 ID 会让目标把实例当人；取不到用户事实就阻塞。
	initiator := runtimeexec.ActorIdentity{}
	if planSession, planErr := e.sessions.Current(ctx, runCtx.PlanAccount); planErr == nil {
		initiator = runtimeexec.ActorIdentity{UserID: planSession.UserID, Name: planSession.Summary.DisplayName}
	}
	if strings.TrimSpace(initiator.UserID) == "" {
		preview.writeSent = false
		preview.writeErr = fmt.Errorf("无法取得计划账号 %s 的用户事实，runtime 装载缺少原发起人身份", runCtx.PlanAccount)
		preview.writeErrClass = model.FailureClassGateBlocked
		preview.runtimeHandled = true
		log.Phase("submit", step.Sequence, approved.Attempt, preview.writeErr.Error())
		return true
	}
	// runtime 身份门禁（§3.3）：装载必须携带完整的实时会话身份。读取能力不存在、实时读取失败或
	// 必需字段缺失都在创建 runtime 会话与发出任何浏览器写请求前落 GateBlocked，绝不静默用会话缓存冒充。
	actorAccount := firstNonEmpty(strings.TrimSpace(preview.ActorAccount), strings.TrimSpace(session.Summary.Account), strings.TrimSpace(runCtx.PlanAccount))
	sessionFacts, activeSession, sessionErr := e.sessionFactsForRuntime(ctx, actorAccount, session)
	if sessionErr != nil {
		preview.writeSent = false
		preview.writeErr = sessionErr
		preview.writeErrClass = model.FailureClassGateBlocked
		preview.runtimeHandled = true
		log.Phase("submit", step.Sequence, approved.Attempt, "runtime 身份门禁阻塞："+sessionErr.Error())
		return true
	}
	session = activeSession
	// 目标代理与节点事实（评审 P1-3）：FlowDialog/审批组件的最终流程载荷直接消费这些字段。
	// 表单代理优先用实例当前事实（重提/加签后可能变化），回落实计划模板事实。
	formProxyID := firstNonEmpty(preview.Facts.FormProxyID, runCtx.FormProxyID)
	flowProxyID := firstNonEmpty(preview.Facts.CurrentTaskFlowProxy, preview.Facts.FlowProxyID, runCtx.FlowProxyID)
	runRequest := runtimeexec.RuntimeRunRequest{
		RunID:        fmt.Sprintf("%d", runCtx.Run.RunNo),
		PathRunID:    fmt.Sprintf("%d", runCtx.PathRun.ID),
		StepID:       fmt.Sprintf("%d", step.Sequence),
		Account:      preview.ActorAccount,
		Action:       action,
		ToolFlowName: instanceName(runCtx, step),
		Session:      sessionFacts,
		Initiator:    initiator,
		Values:       formDataFromPreviewRequest(preview),
		InstanceID:   runCtx.PathRun.MainInstanceRef,
		JobTaskID:    jobTaskID,
		NodeProxyID:  pendingTaskNodeIDOf(runCtx, step, preview.Facts),
		AuditDesc:    auditMessage(runCtx, step),
		// 权限=当前节点声明可编辑字段；变更字段=本步实际覆盖的字段（预览阶段已按权限构造）。
		Permissions:    runCtx.Nodes[step.NodeKey].EditableFields,
		ChangedFields:  preview.FormOverlaid,
		ReadonlyFields: runtimeFormPersonReadonlyFields(runCtx, step, approved.NextIndex),
		// 代理与节点事实：目标提交按 formId 决定发送 formProxyId/flowProxyId（评审 P1-3 根因）。
		FormProxyID: formProxyID,
		FlowProxyID: flowProxyID,
		// 发起装载专用的计划绑定模板 ID：新发起来源下计划目标对象 ID 就是 /web/flowTemplateApi/list
		// 返回的模板 ID（target_read.go 以它调 ReadTemplateConfiguration）；与表单/流程代理 ID 语义不同，
		// 绝不互用（P1-2）。用恒定的发布代理 ID：加签会把 FlowProxyID 改成实例私有代理，模板 ID 仍须是
		// 计划绑定对象，否则重新发起装载会带着私有代理去权限探测被拒。
		FlowTemplateID:       firstNonEmpty(runCtx.PublishedFlowProxyID, runCtx.FlowProxyID),
		SelectFlowType:       runCtx.FlowType,
		FlowNodeType:         runCtx.Nodes[step.NodeKey].AuditType,
		NextNodeProxyID:      nextNodeTargetIDOf(runCtx, approved.NextIndex),
		BizRelevance:         runtimeBizRelevance(preview.Facts.BizRelevance),
		ReceiverID:           receiverID,
		ReceiverName:         receiverName,
		BranchNodeAuditTypes: branchAuditTypes(runCtx),
		BaseURL:              e.runtimeBaseURL,
		// 平台码取本步已读的实例事实：实例重建类装载（重提/编辑/审批/转发）必须按实例真实平台
		// 发目标请求；发起类实例尚不存在，留空由运行时页面回落默认平台码。
		PlatformCode: strings.TrimSpace(preview.Facts.PlatformCode),
	}
	// 装载模式由持久化实例事实决定：首次草稿走发起，已有主实例的后续草稿走待发编辑。
	switch step.Action {
	case model.ActionSaveDraft:
		if strings.TrimSpace(runCtx.PathRun.MainInstanceRef) == "" {
			runRequest.LoadMode = runtimeexec.LoadModeInitiate
		} else {
			runRequest.LoadMode = runtimeexec.LoadModeEdit
		}
	case model.ActionSubmit:
		runRequest.LoadMode = runtimeexec.LoadModeInitiate
	case model.ActionResubmit:
		runRequest.LoadMode = runtimeexec.LoadModeEdit
		// 重新发起弹窗的权限探测（checkFlowPermission 只带 flowProxyId）要求该 ID 能在目标
		// 发起映射表（flow_form_scope_template/proxy）中查到；加签触发目标所有权复制后实例私有
		// 代理不在其中，探测被「流程模板已被更新，请重新刷新页面，加载最新流程模板数据」拒绝，
		// 页面随即中止、永远不发出 reSubmit（运行 8 实测）。新发起来源下计划绑定对象就是发布流程
		// 代理（映射表中存在），装载行事实改用它——即目标报错自身要求的「最新流程模板」；真正的
		// reSubmit 载荷由页面构造且不带 flowProxyId，后端按实例自身代理推进，不受此选择影响。
		// 待发/已发来源下计划绑定对象是实例 ID，只能保持实例当前事实代理：实例未加签时
		// 与发布代理一致可过探测。新发起来源下用恒定的发布流程代理 ID（不被加签重建覆盖），
		// 加签后的重新发起装载据此过权限探测，页面再发出真正的 reSubmit（载荷不带 flowProxyId）。
		if !strings.EqualFold(strings.TrimSpace(runCtx.Source), "pending") {
			runRequest.FlowProxyID = firstNonEmpty(runCtx.PublishedFlowProxyID, runCtx.FlowProxyID)
		}
	case model.ActionForward:
		runRequest.LoadMode = runtimeexec.LoadModeForward
		runRequest.ForwardCheckList = stringSliceParameter(step.Parameters, "forwardCheckList")
		// 转发必须装载「待办」(backlog) 列表页，不能按实例来源改选「待发」(dueout)。
		//
		// 为什么（2026-09-21 运行 80 路径 1 第 43 步实测）：原生列表页的「转发」入口只在
		// 非 dueout/timedout 的标签页上渲染，且 transpond() 直接委托给当前激活标签页组件
		// （this.$refs[this.activeName].transpond(...)）。dueout（待发）页既没有转发按钮，
		// 其组件也没有 transpond 入口，装载必然等不到可转发的行而超时：
		//
		//	runtime 装载失败：执行服务返回 HTTP 500：page.waitForFunction: Timeout 60000ms exceeded
		//
		// 转发的语义是"把本人当前待办里的这条任务转给别人"，可转发的任务只出现在待办列表，
		// 与实例最初是新发起还是待发来源无关。因此固定取 backlog，不再按来源分支。
		runRequest.ForwardTab = ForwardTabForAction()
	default:
		runRequest.LoadMode = runtimeexec.LoadModeApprove
	}
	// 选择闭环配置（T010B）：分支选择是「分支节点 ID → 所选分支入口节点 ID」，
	// 键值各归其位；只带入口会丢掉分支节点归属，目标无法匹配候选分支。
	// 评审 P1-6：入口必须经真实图边校验——它必须是该分支节点的直接后继，
	// 防止目标返回的临时节点集合诱导工具发出与已保存配置无关的分支。
	// 没有边表的旧数据保持既有行为（与 F-034 T04 同一口径）；有边表而校验不过即写前阻塞。
	for branchNodeID, entryID := range runCtx.BranchSelections {
		if len(runCtx.GraphEdges) > 0 {
			reachable := false
			for _, edge := range runCtx.GraphEdges[branchNodeID] {
				if edge.Target == entryID {
					reachable = true
					break
				}
			}
			if !reachable {
				preview.writeSent = false
				preview.writeErr = fmt.Errorf("分支选择与真实流程图不一致：入口 %s 不是分支节点 %s 的直接后继，拒绝经 runtime 执行", entryID, branchNodeID)
				preview.writeErrClass = model.FailureClassGateBlocked
				preview.runtimeHandled = true
				log.Phase("submit", step.Sequence, approved.Attempt, preview.writeErr.Error())
				return true
			}
		}
		runRequest.BranchSelections = append(runRequest.BranchSelections, runtimeexec.BranchSelection{NodeProxyID: branchNodeID, EntryNodeID: entryID})
	}
	for nodeKey, auditors := range runCtx.NextNodeAuditors {
		if info, ok := runCtx.Nodes[nodeKey]; ok {
			for _, auditor := range auditors {
				// 空人员条目绝不下发：runtime 会把它们原样装入目标的 nextAuditorList，
				// 「不能用空人员代替配置结果」（T010B）；残缺条目等于把未配置事实发给目标。
				if strings.TrimSpace(auditor.BizID) == "" || strings.TrimSpace(auditor.Name) == "" {
					continue
				}
				runRequest.NextNodeAuditors = append(runRequest.NextNodeAuditors, runtimeexec.NextAuditorEntry{
					NodeProxyID: info.TargetNodeID, BizID: auditor.BizID, Name: auditor.Name, AuditDetailType: "personnel",
				})
			}
		}
	}
	result, err := e.runtimeRunner.RunAction(ctx, runRequest)
	if err != nil {
		preview.runtimeHandled = true
		switch {
		case errors.Is(err, runtimeexec.ErrUncertain):
			// 连接断开且已有目标请求写出：写状态不明。必须按「已发出、结果未知」处理，
			// 走既有核验与结果待确认流程；判成零写入失败会诱发重复执行真实业务（评审 P1）。
			preview.writeSent = true
			preview.writeErr = &target.Error{Kind: target.ErrorUnavailable, Transport: target.TransportInterrupted, Cause: err}
			e.rememberRuntimeFacts(preview, result)
			log.Phase("submit", step.Sequence, approved.Attempt, "runtime 连接断开且已有目标请求写出："+err.Error())
		case errors.Is(err, runtimeexec.ErrFailedCleanly):
			// 连接断开且确认没有任何目标请求写出：零写入失败，归工具侧缺陷。
			preview.writeSent = false
			preview.writeErr = err
			preview.writeErrClass = model.FailureClassToolBug
			log.Phase("submit", step.Sequence, approved.Attempt, "runtime 连接断开且无目标请求写出："+err.Error())
		default:
			// 会话创建/装载/协议校验失败：业务命令尚未发出，写请求必然没有写出，零写入失败。
			preview.writeSent = false
			preview.writeErr = err
			preview.writeErrClass = model.FailureClassToolBug
			log.Phase("submit", step.Sequence, approved.Attempt, "runtime 执行失败："+err.Error())
		}
		return true
	}
	result = e.protectRuntimeFormBody(ctx, runCtx, step, session, result, log, approved.Attempt)
	preview.runtimeHandled = true
	e.rememberRuntimeFacts(preview, result)
	preview.writeResponse = target.WriteResponse{StatusCode: 200, IsSuccessPresent: true, IsSuccess: result.Status == "success"}
	switch result.Status {
	case "success":
		preview.writeSent = true
		preview.writeErr = nil
		preview.writeResponse.IsSuccess = true
		// 发起链：runtime 在浏览器里拿到写响应，新实例 ID 经业务摘要回传。
		// 填进 writeResult 后走与 Go 直连写相同的「主实例引用落库」路径，
		// 否则核验阶段拿不到实例 ID，真实成功的发起会被误判为结果待确认。
		if strings.TrimSpace(result.NewInstanceID) != "" {
			preview.writeResult = &target.SubmitFlowInstanceResult{InstanceID: result.NewInstanceID}
		}
		log.Phase("submit", step.Sequence, approved.Attempt, "runtime 完成：业务与流程请求已全部收敛，进入目标事实核验")
	case "uncertain", "timeout":
		// 写状态不明：结果待确认，绝不重发。
		preview.writeSent = true
		preview.writeErr = &target.Error{Kind: target.ErrorUnavailable, Transport: target.TransportResponded, Cause: errors.New(result.Message)}
		log.Phase("submit", step.Sequence, approved.Attempt, "runtime 判定结果待确认："+result.Message)
	case "target_rejected":
		preview.writeSent = true
		preview.writeErr = &target.BusinessRejection{Code: "RUNTIME_TARGET_REJECTED", Message: result.Message}
		log.Phase("submit", step.Sequence, approved.Attempt, "runtime 判定目标拒绝："+result.Message)
	case "business_failed":
		preview.writeSent = true
		preview.writeErr = &target.BusinessRejection{Code: "RUNTIME_BUSINESS_FAILED", Message: result.Message}
		log.Phase("submit", step.Sequence, approved.Attempt, "runtime 判定后置业务失败："+result.Message)
	default:
		// failed / unsupported：零写入失败。
		preview.writeSent = false
		preview.writeErr = errors.New(result.Message)
		preview.writeErrClass = model.FailureClassGateBlocked
		log.Phase("submit", step.Sequence, approved.Attempt, "runtime 判定失败："+result.Message)
	}
	return true
}

// sessionFactsForRuntime 从当前会话与目标实时人员目录读取完整身份事实，供 runtime 装载注入
// window.localStorage（含 companyName/departmentName/dutyId/dutyName 等）。
// 与 nodeFormData 同一阻塞口径（protocol_matrix.CurrentUserIdentity 合约）：读取能力不存在、实时读取失败、
// 或必需身份字段缺失都返回门禁错误，绝不静默回退会话缓存冒充实时事实——否则 runtime 会拿着不完整或
// 过期的公司/部门/岗位继续写目标。只允许继续使用已核实的当前会话固有字段（CustomerCode/Account）。
func (e *Executor) sessionFactsForRuntime(ctx context.Context, account string, session target.Session) (runtimeexec.SessionFacts, target.Session, error) {
	identityReader, canReadIdentity := e.target.(runtimeIdentityReader)
	if !canReadIdentity {
		return runtimeexec.SessionFacts{}, session, fmt.Errorf("目标客户端不支持实时身份读取，runtime 装载不能按当前会话覆盖身份")
	}
	identity, active, err := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, account, session,
		func(callContext context.Context, currentSession target.Session) (target.UserIdentity, error) {
			return identityReader.CurrentUserIdentity(callContext, currentSession)
		})
	if err != nil {
		return runtimeexec.SessionFacts{}, active, fmt.Errorf("读取当前会话实时身份失败：%s", target.UserFacingErrorMessage(target.WriteResponse{}, err))
	}
	if !identity.Complete() {
		return runtimeexec.SessionFacts{}, active, fmt.Errorf("当前会话在目标平台缺少身份字段 %s，runtime 写请求前已阻塞", strings.Join(identity.MissingFields(), "、"))
	}
	// 实时身份完整：runtime 收到的公司/部门/岗位/用户与当前读取结果一致。
	return runtimeexec.SessionFacts{
		UserID:         identity.UserID,
		CompanyID:      identity.CompanyID,
		CompanyName:    identity.CompanyName,
		DepartmentID:   identity.DepartmentID,
		DepartmentName: identity.DepartmentName,
		DutyID:         identity.DutyID,
		DutyName:       identity.DutyName,
		CustomerCode:   active.CustomerCode,
		Account:        active.Summary.Account,
		DisplayName:    identity.UserName,
	}, active, nil
}

// stringSliceParameter 从动作参数读取字符串数组；非数组或空项直接忽略，绝不猜测配置值。
func stringSliceParameter(parameters map[string]any, key string) []string {
	value, ok := parameters[key]
	if !ok {
		return nil
	}
	result := []string{}
	switch items := value.(type) {
	case []string:
		for _, item := range items {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				result = append(result, trimmed)
			}
		}
	case []any:
		for _, item := range items {
			if trimmed := strings.TrimSpace(fmt.Sprint(item)); trimmed != "" {
				result = append(result, trimmed)
			}
		}
	}
	return result
}

// protectRuntimeFormBody 用目标写后回读补齐正文证据，并在字段数明显减少时改判业务失败。
// 页面快照只证明写前装载完整；写后事实必须以 Go 重新读取目标实例为准。
func (e *Executor) protectRuntimeFormBody(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, session target.Session, result runtimeexec.ActionRunResult, log *StepLog, attempt int) runtimeexec.ActionRunResult {
	if result.Status != "success" {
		return result
	}
	switch step.Action {
	case model.ActionStorageFormData, model.ActionApprove, model.ActionReject:
	default:
		return result
	}
	instanceID := strings.TrimSpace(runCtx.PathRun.MainInstanceRef)
	if instanceID == "" {
		return result
	}
	current, err := e.target.ReadInstanceCurrentData(ctx, session, instanceID)
	if err != nil {
		result.Status = "business_failed"
		result.Message = "runtime 表单正文写后回读失败，无法证明写后正文完整：" + err.Error()
		log.Phase("verify", step.Sequence, attempt, result.Message)
		return result
	}
	summary := map[string]any{}
	if len(result.BusinessSummary) > 0 {
		_ = json.Unmarshal(result.BusinessSummary, &summary)
	}
	beforeMap, _ := summary["formBefore"].(map[string]any)
	beforeClean := businessFormKeys(beforeMap)
	afterClean := businessFormKeys(current)

	before := len(beforeClean)
	if before == 0 {
		before = runtimeSummaryFieldCount(summary["fieldCountBefore"])
	}
	after := len(afterClean)
	summary["formAfter"] = current
	summary["fieldCountAfter"] = after
	if encoded, encodeErr := json.Marshal(summary); encodeErr == nil {
		result.BusinessSummary = encoded
	}
	// 表单正文保护：防止写动作意外清空整个表单正文（如整表被空对象覆盖）。
	// 为什么这样处理：
	// 浏览器页面中可能携带空字符串键（如未命名容器绑定的 "" 属性），而在服务端持久化时这些无效空键会被自动剔除；
	// 若盲目按未清洗的原始数量对比，会因空键过滤（例如写前 41 vs 写后 40）误报字段减少。
	// 这里通过清洗无效键并双重核对业务字段是否存在，只有在写前确实有字段、写后数量下跌且存在实质性业务字段缺失时才判定为失败。
	if before > 0 && after < before {
		missingKeys := missingBusinessFieldNames(beforeClean, afterClean)
		missing := len(missingKeys)
		if missing > 0 {
			result.Status = "business_failed"
			result.Message = fmt.Sprintf("写后表单正文字段明显减少：写前 %d 个，写后 %d 个，缺失 %d 个业务字段（%s）；已阻止继续执行", before, after, missing, strings.Join(missingKeys, ", "))
			log.Phase("verify", step.Sequence, attempt, result.Message)
		}
	}
	return result
}

// cleanFormKeys 过滤掉空字符串等无效字段键，保持有效业务字段的准确统计。
func cleanFormKeys(data map[string]any) map[string]any {
	cleaned := make(map[string]any, len(data))
	for k, v := range data {
		trimmed := strings.TrimSpace(k)
		if trimmed == "" {
			continue
		}
		cleaned[trimmed] = v
	}
	return cleaned
}

// businessFormKeys 在 cleanFormKeys 的空键清洗之上，剔除目标引擎自管的系统结构性字段和登录上下文字段。
// 为什么：runtime 写后回读用它做字段数统计与「缺失业务字段」比较；前者由目标按实例重写，后者由当前会话覆盖，
// 两者都不代表业务正文丢失，若计入会把正常目标行为误判成正文减少而改判 business_failed。
func businessFormKeys(data map[string]any) map[string]any {
	cleaned := cleanFormKeys(data)
	stripSystemStructuralFields(cleaned)
	for key := range cleaned {
		if target.IsTargetContextField(key) {
			delete(cleaned, key)
		}
	}
	return cleaned
}

// missingBusinessFieldNames 返回写前存在、写后缺失的业务字段名，并按稳定顺序排序。
// 具体字段名进入阻塞消息，便于直接定位目标接口或节点权限造成的正文丢失。
func missingBusinessFieldNames(before, after map[string]any) []string {
	missing := make([]string, 0)
	for key := range before {
		if _, ok := after[key]; ok {
			continue
		}
		// 人员选择组件的伴生字段由目标页面维护。目标在节点流转后可能只清洗
		// __formPersonId / __condition / __virtualName，但仍保留主人员字段；此时业务值没有丢失。
		// 只有主字段也缺失时才继续按正文丢失阻塞，避免吞掉真正的人员数据损坏。
		if primary, ok := personPickerPrimaryField(key); ok {
			if _, primaryExists := after[primary]; primaryExists {
				continue
			}
		}
		missing = append(missing, key)
	}
	sort.Strings(missing)
	return missing
}

// personPickerPrimaryField 返回人员选择伴生字段对应的主字段名。
// 非伴生字段或去掉后缀后主字段为空时返回 false，避免把普通业务字段误归类。
func personPickerPrimaryField(field string) (string, bool) {
	if !target.IsPersonPickerCompanionField(field) {
		return "", false
	}
	key := strings.TrimSpace(field)
	for _, suffix := range []string{"__condition", "__formPersonId", "__virtualName"} {
		if strings.HasSuffix(key, suffix) {
			primary := strings.TrimSpace(strings.TrimSuffix(key, suffix))
			return primary, primary != ""
		}
	}
	return "", false
}

// runtimeSummaryFieldCount 把 JSON 解码后的字段数收敛为非负整数。
func runtimeSummaryFieldCount(value any) int {
	switch typed := value.(type) {
	case float64:
		if typed > 0 {
			return int(typed)
		}
	case json.Number:
		parsed, _ := typed.Int64()
		if parsed > 0 {
			return int(parsed)
		}
	case int:
		if typed > 0 {
			return typed
		}
	}
	return 0
}

// ShouldRunViaRuntimeForTest 暴露路由判定，供契约测试锁定「哪些动作走 runtime」。
func (e *Executor) ShouldRunViaRuntimeForTest(runCtx RunContext, step model.CompiledActionStep) bool {
	return e.shouldRunViaRuntime(runCtx, step)
}

// rememberRuntimeFacts 把一次 runtime 动作的判定事实回填到预览：七值状态、业务说明与
// 逐请求轨迹，并在同一处构造三阶段投影（运行详情的生产数据源，评审 P1-7）。
// 成功与失败路径都要回填：连接断开不等于请求没发出，已收集的轨迹必须留档。
func (e *Executor) rememberRuntimeFacts(preview *StepPreview, result runtimeexec.ActionRunResult) {
	preview.runtimeStatus = result.Status
	if strings.TrimSpace(result.Message) != "" {
		preview.runtimeMessage = result.Message
	}
	if len(result.RequestTrace) > 0 {
		preview.runtimeRequestTrace = result.RequestTrace
	}
	if len(result.WriteTraceIDs) > 0 {
		preview.runtimeWriteTraceIDs = append(preview.runtimeWriteTraceIDs, result.WriteTraceIDs...)
		// 首个写请求 trace 直接作为本步尝试的 trace：尝试行、network.log 与能力登记证据同源。
		if preview.writeTraceID == "" {
			preview.writeTraceID = result.WriteTraceIDs[0]
		}
	}
	// 轨迹为空时投影仍会给出「流程动作未发出」的三阶段视图，如实呈现比留空更可解释。
	projection := runtimeexec.BuildDetailProjection(
		runtimeexec.ActionRunResult{Status: preview.runtimeStatus, Message: preview.runtimeMessage, BusinessSummary: result.BusinessSummary},
		preview.runtimeRequestTrace,
		runtimeexec.ProtocolVersion,
	)
	if encoded, err := json.Marshal(projection); err == nil {
		preview.runtimeDetailJSON = string(encoded)
	}
	// 执行器墙钟真实耗时：仅当本次结果带回时记为已知，连接断开等无结果场景保持未知（落库写 NULL）。
	if result.ExecutorMs != nil {
		preview.executorMs = *result.ExecutorMs
		preview.executorMsKnown = true
	}
}

// settleEventDetail 组装「本步落账」事件流补充明细：节点、动作、调用的目标写端点与目标返回结果原话。
// 目的：运行事件流按节点直接看到「这个节点调了什么接口、目标返回了什么」，目标原始错误原话随 userMessage 带出。
func settleEventDetail(preview *StepPreview, userMessage string) string {
	parts := make([]string, 0, 3)
	head := strings.TrimSpace(preview.NodeName)
	action := strings.TrimSpace(preview.ActionName)
	switch {
	case head != "" && action != "":
		parts = append(parts, "节点「"+head+"」·"+action)
	case action != "":
		parts = append(parts, action)
	case head != "":
		parts = append(parts, "节点「"+head+"」")
	}
	if ep := strings.TrimSpace(preview.Endpoint); ep != "" {
		parts = append(parts, "接口 "+ep)
	}
	if msg := strings.TrimSpace(userMessage); msg != "" {
		parts = append(parts, msg)
	}
	return strings.Join(parts, " · ")
}

// RunViaRuntimeForTest 暴露 runtime 执行分支，供契约测试锁定无双写与结果映射。
func (e *Executor) RunViaRuntimeForTest(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, session target.Session, preview *StepPreview, approved ApprovedStep) bool {
	return e.runViaRuntime(ctx, runCtx, step, session, preview, nil, approved)
}

// ProtectRuntimeFormBodyForTest 暴露 runtime 写后正文保护，供回归用例锁定系统结构性字段不被计入业务字段丢失。
func (e *Executor) ProtectRuntimeFormBodyForTest(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, session target.Session, result runtimeexec.ActionRunResult, attempt int) runtimeexec.ActionRunResult {
	return e.protectRuntimeFormBody(ctx, runCtx, step, session, result, nil, attempt)
}

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
	if step.Source == model.ActionStepSourceRecovery && strings.TrimSpace(step.SourceActionKey) != "" &&
		recoveryTriggerWasSkipped(e.facts, runCtx, step) {
		info := runCtx.Nodes[step.NodeKey]
		reason := "触发恢复链的动作被目标跳过，恢复链未适用；不发送重新提交或恢复审批请求"
		return &StepPreview{
			PathRunID: runCtx.PathRun.ID, StepNo: step.Sequence, TotalSteps: len(runCtx.Steps),
			ReleaseGroup: step.ReleaseGroup, ReleaseRequired: step.ReleaseRequired,
			Action: step.Action, ActionName: "恢复链未适用", NodeKey: step.NodeKey,
			TargetNodeID: info.TargetNodeID, NodeName: info.Name, ActorAccount: runCtx.PlanAccount,
			GateAllowed: true, RecoveryNotApplicable: true, SkipReason: reason,
			GateItems: []model.ActionPrecondition{},
		}, false, nil
	}
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
	if name := strings.TrimSpace(facts.CurrentTaskAssigneeName); name != "" {
		actorName = name
	} else if assigneeID := strings.TrimSpace(facts.CurrentTaskAssigneeID); assigneeID != "" {
		// 目标偶尔只在待办中返回处理人 ID；优先从人员目录解析真实中文姓名，
		// 避免加签插入节点或普通审批在放行前预览把 32 位 UUID 裸露在界面上。
		if resolver, ok := e.target.(userAccountResolver); ok {
			if accounts, _, err := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, session.Summary.Account, session,
				func(callContext context.Context, active target.Session) (map[string]target.UserAccount, error) {
					return resolver.UserAccountsByID(callContext, active, []string{assigneeID})
				}); err == nil {
				if info, ok := accounts[assigneeID]; ok {
					if resolvedName := firstNonEmpty(strings.TrimSpace(info.Name), strings.TrimSpace(info.Account)); resolvedName != "" {
						actorName = resolvedName
						facts.CurrentTaskAssigneeName = resolvedName
					}
				}
			}
		}
		if actorName == runCtx.PlanAccount && assigneeID != "" {
			actorName = assigneeID
		}
	}
	if persisted := loadPersistedFormDataVerifyIssue(e.facts, runCtx.PathRun.ID, step.Sequence); persisted != "" {
		facts.FormDataVerifyIssue = persisted
	}
	if strings.TrimSpace(facts.FormDataVerifyIssue) != "" {
		reason := "上一步表单核对未通过：" + facts.FormDataVerifyIssue + "；已阻塞本步，请先到对应步骤和日志核对目标实例当前表单"
		log.Phase("gate", step.Sequence, 1, reason)
		return e.blockedPreview(runCtx, step, actorName, reason, model.FailureClassGateBlocked), false, nil
	}
	info := runCtx.Nodes[step.NodeKey]
	// 取回动作若紧跟一个已被目标自动跳过的准备同意，目标从未为该节点生成已办审核记录，
	// 继续把取回当成可执行写动作只会得到“没有已办任务”并错误停机。沿用已落账的目标跳过事实，
	// 把取回记为零写入的目标跳过，让路径继续到后续步骤。
	if step.Action == model.ActionRetrieve && retrieveFollowsTargetSkippedPreparation(e.facts, runCtx, nextIndex) {
		reason := "取回目标节点已被目标自动跳过，未产生可取回的已办任务；取回不适用，不发送请求"
		log.Phase("control", step.Sequence, 1, reason)
		return &StepPreview{
			PathRunID: runCtx.PathRun.ID, StepNo: step.Sequence, TotalSteps: len(runCtx.Steps),
			ReleaseGroup: step.ReleaseGroup, ReleaseRequired: step.ReleaseRequired,
			Action: step.Action, ActionName: "取回（目标节点已跳过，不适用）", NodeKey: step.NodeKey,
			TargetNodeID: info.TargetNodeID, NodeName: info.Name, ActorAccount: runCtx.PlanAccount, ActorName: actorName,
			GateAllowed: true, Facts: facts, TargetSkipped: true, SkipReason: reason,
			GateItems: []model.ActionPrecondition{},
		}, false, nil
	}
	// 目标已明确把待办推进到后续节点时，节点整体已经不存在可执行处理人；
	// 先按目标 isSkip 分型，避免无处理人轮询把"允许自动跳过"的节点错误阻塞。
	// 用户配置动作也属于该节点的一部分，节点跳过时动作随节点一起不执行，绝不凭空制造处理人或发送写请求。
	targetSkipPreview := func(currentFacts InstanceFacts) (*StepPreview, bool) {
		if step.Scope != model.ActionScopeTask && step.Scope != model.ActionScopeCompletedTask {
			return nil, false
		}
		// 恢复型动作以实例已越过本步为合法前提；但取回准备同意是推进型前置步骤，
		// 必须服从目标 isSkip 规则，否则目标已递归越过无人节点时工具会错误阻塞。
		if isRecoveryEffectStep(step) {
			return nil, false
		}
		action, reason, pendingName, diag := targetSkippedStepReason(runCtx, step, currentFacts)
		log.Phase("gate", step.Sequence, 1, "目标跳过判定："+diag)
		switch action {
		case "skip":
			preview := &StepPreview{
				PathRunID: runCtx.PathRun.ID, StepNo: step.Sequence, TotalSteps: len(runCtx.Steps),
				ReleaseGroup: step.ReleaseGroup, ReleaseRequired: step.ReleaseRequired,
				Action: step.Action, ActionName: "目标跳过确认", NodeKey: step.NodeKey,
				TargetNodeID: info.TargetNodeID, NodeName: info.Name,
				ActorAccount: runCtx.PlanAccount, ActorName: actorName,
				GateAllowed: true, Facts: currentFacts, TargetSkipped: true, SkipReason: reason,
				GateItems: []model.ActionPrecondition{},
			}
			log.Phase("control", step.Sequence, 1, fmt.Sprintf("该节点已被目标自动跳过（实例待办已在「%s」）；节点动作不执行，放行将记录跳过并继续", pendingName))
			return preview, true
		case "block":
			// 目标不允许跳过或本节点是运行时自选审批节点时，越过事实与动作配置冲突；
			// 必须在发送前阻塞并展示原因，不能用计划账号、候选人或空人员代替目标处理人。
			log.Phase("control", step.Sequence, 1, "目标不可能自动越过本节点，已在发送前阻塞："+reason)
			return e.blockedPreview(runCtx, step, actorName, reason, model.FailureClassGateBlocked), true
		default:
			return nil, false
		}
	}
	if preview, decided := targetSkipPreview(facts); decided {
		return preview, false, nil
	}
	// F-035：目标节点已到达但未生成处理人/待办时，进入"正在生成处理人"有界轮询；
	// 总等待不超过 10 秒、最多 5 次，期间不发任何写请求。超时后按 assignment_missing 阻塞，
	// 绝不显示"当前待办已经处理"，也不用计划账号或候选人代替目标返回的真实处理人。
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
	// 轮询期间目标可能已经把待办推进到后续节点；用最新事实再次判定，不能把已跳过节点误作无处理人阻塞。
	if preview, decided := targetSkipPreview(facts); decided {
		return preview, false, nil
	}
	// F-035 第四轮：已知特殊业务必须实现；只有未实现或未登记证明不足的分支才写前阻塞。
	if reason := target.SpecialBusinessBlockReason(runCtx.FlowType, string(step.Action)); reason != "" {
		log.Phase("gate", step.Sequence, 1, "特殊业务未实现阻塞："+reason)
		return e.blockedPreview(runCtx, step, actorName, reason, model.FailureClassGateBlocked), false, nil
	}
	// 取回准备同意的运行时门禁（T04 retrieve_prepare_approve）：编译期只是提示，运行时以目标
	// 已办事实为准。当前用户在本节点已有已办任务时，准备同意无必要，跳过本步直接取回，绝不多发
	// 一次同意写请求；无已办时落到下方普通同意门禁——没有可同意的活动待办即阻塞，绝不无依据准备同意。
	//
	// 为什么不能只看"已办读取命中"（2026-09-21 运行 100/101 路径 5 第 12 步实测）：done 读取按本步
	// 节点等值过滤，而目标在本节点办结后会把该任务关联行的 flowNodeProxyID 改写成"实例流转到的下一节点"，
	// 于是这里拿到的"命中"可能根本不是本节点的已办。路径 5 正是如此：省区经理从未被审批过，
	// 本步却按一行他处已办跳过，紧接着的取回步骤便指向一个没有已办任务的节点，
	// 实例停在原地，下一步报「当前待办已经处理」而停止整条路径。
	// 因此跳过必须要求审核记录正面证明本节点确实办结过；证明不足时不跳过，
	// 落到普通同意门禁按实时待办裁决（该门禁本来就会在无可同意待办时阻塞）。
	if step.RetrievePrepareConsent {
		nodeID := strings.TrimSpace(runCtx.Nodes[step.NodeKey].TargetNodeID)
		doneSession, doneSnapshot, doneErr := e.refreshTaskSnapshot(ctx, runCtx, step, nodeID, session, "done")
		if doneErr == nil {
			session = doneSession
			// 已办读取命中只是必要条件：还必须能证明"本步节点确实办结过"，否则不得跳过。
			if e.retrieveDoneRowProvesNodeHandled(ctx, session, strings.TrimSpace(runCtx.PathRun.MainInstanceRef), nodeID, doneSnapshot) {
				info := runCtx.Nodes[step.NodeKey]
				reason := "当前用户在本节点已有已办任务，无需准备同意，直接取回"
				log.Phase("control", step.Sequence, 1, "取回准备同意跳过："+reason)
				return &StepPreview{
					PathRunID: runCtx.PathRun.ID, StepNo: step.Sequence, TotalSteps: len(runCtx.Steps),
					ReleaseGroup: step.ReleaseGroup, ReleaseRequired: step.ReleaseRequired,
					Action: step.Action, ActionName: "取回准备同意（已有已办，跳过）", NodeKey: step.NodeKey,
					TargetNodeID: info.TargetNodeID, NodeName: info.Name,
					ActorAccount: runCtx.PlanAccount, ActorName: actorName,
					GateAllowed: true, Facts: facts, TargetSkipped: true, SkipReason: reason,
					GateItems: []model.ActionPrecondition{},
				}, false, nil
			}
		} else {
			log.Phase("gate", step.Sequence, 1, "取回准备同意的已办事实读取失败，按普通同意门禁继续判定："+userFacingError(doneErr, target.WriteResponse{}))
		}
	}
	catalogItem, allowed := evaluateGate(step, buildGateContext(runCtx, step, facts, info))
	// 固定人员发现的结论随门禁一并披露：拒绝原因要能说明「待办在谁的账号下、工具找过了谁」。
	if !allowed && strings.TrimSpace(facts.AssigneeDiag) != "" {
		log.Phase("gate", step.Sequence, 1, "固定人员发现："+facts.AssigneeDiag)
	}
	// 取回的身份与已办视角解析结论必须无条件披露：它决定取回以谁的身份发出、指向哪一条已办。
	// 只在拒绝时才打印会让"放行但身份解析为空"这类缺陷在日志里完全不可见
	// （2026-09-21 路径 5/6 实测：门禁放行、取回以空登录账号发出，现场日志只剩一个空账号，无从定位）。
	if step.Action == model.ActionRetrieve && strings.TrimSpace(facts.AssigneeDiag) != "" {
		log.Phase("gate", step.Sequence, 1, "取回身份与已办视角："+facts.AssigneeDiag)
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
		ActorAccount:    "",
		ActorName:       actorName,
		ExpectedEffect:  catalogItem.ExpectedEffect,
		GateAllowed:     allowed,
		GateItems:       catalogItem.Preconditions,
		Facts:           facts,
	}
	if step.Action == model.ActionForward {
		preview.ReceiverID = firstActionPersonID(runCtx.ActionPersonIDs[actionPersonKey(step.NodeKey, step.Action)])
		if preview.ReceiverID != "" {
			preview.ReceiverName = personNameFor(runCtx, preview.ReceiverID)
		}
	}
	if step.Scope != model.ActionScopeTask && step.Scope != model.ActionScopeCompletedTask {
		preview.ActorAccount = runCtx.PlanAccount
	}
	if !allowed {
		// 设为跟踪 / 取消跟踪的幂等跳过：当前已处于目标状态时，跳过重复写请求，不中断执行器。
		if step.Action == model.ActionFollow && facts.TrackingRead && facts.Tracking {
			reason := "当前实例已处于跟踪状态，无需重复设置"
			log.Phase("control", step.Sequence, 1, reason)
			preview.GateAllowed = true
			preview.TargetSkipped = true
			preview.SkipReason = reason
			return preview, false, nil
		}
		if step.Action == model.ActionUnfollow && facts.TrackingRead && !facts.Tracking {
			reason := "当前实例已处于未跟踪状态，无需重复设置"
			log.Phase("control", step.Sequence, 1, reason)
			preview.GateAllowed = true
			preview.TargetSkipped = true
			preview.SkipReason = reason
			return preview, false, nil
		}
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

	// 设为跟踪 / 取消跟踪的幂等跳过：当前已处于目标状态时，跳过重复写请求，不中断执行器。
	if step.Action == model.ActionFollow && facts.TrackingRead && facts.Tracking {
		preview.TargetSkipped = true
		preview.SkipReason = "当前实例已处于跟踪状态，无需重复设置"
		log.Phase("control", step.Sequence, 1, preview.SkipReason)
		return preview, false, nil
	}
	if step.Action == model.ActionUnfollow && facts.TrackingRead && !facts.Tracking {
		preview.TargetSkipped = true
		preview.SkipReason = "当前实例已处于未跟踪状态，无需重复设置"
		log.Phase("control", step.Sequence, 1, preview.SkipReason)
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
	reportProgressSafe(reportProgress, "control", fmt.Sprintf("第 %d 步已就绪，等你确认执行：点击\"执行这一步\"后，会由\"%s\"在\"%s\"节点执行\"%s\"，只发送一次目标请求", step.Sequence, actorName, runCtx.Nodes[step.NodeKey].Name, catalogItem.Label))
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
// 必须先切真实会话并读取当前身份，再构造表单：动态字段、选项和伴生键不能按旧账号生成。
func (e *Executor) nodeFormData(ctx context.Context, runCtx RunContext, compiled model.CompiledActionStep, session target.Session) (FormDataPlan, target.Session, error) {
	if !ActionCarriesFormData(compiled.Action) {
		return FormDataPlan{}, session, nil
	}
	createGlobal := compiled.Action == model.ActionSubmit || compiled.Action == model.ActionSaveDraft || compiled.Action == model.ActionResubmit
	switch compiled.Action {
	case model.ActionSubmit, model.ActionSaveDraft, model.ActionResubmit, model.ActionApprove:
		identityReader, canReadIdentity := e.target.(runtimeIdentityReader)
		if !canReadIdentity {
			return FormDataPlan{}, session, fmt.Errorf("目标客户端不支持实时身份读取，不能按当前会话覆盖表单身份")
		}
		account := strings.TrimSpace(session.Summary.Account)
		if account == "" {
			account = runCtx.PlanAccount
		}
		identity, active, identityErr := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, account, session,
			func(callContext context.Context, active target.Session) (target.UserIdentity, error) {
				return identityReader.CurrentUserIdentity(callContext, active)
			})
		session = active
		if identityErr != nil {
			return FormDataPlan{}, session, fmt.Errorf("读取当前会话实时身份失败：%s", target.UserFacingErrorMessage(target.WriteResponse{}, identityErr))
		}
		if !identity.Complete() {
			return FormDataPlan{}, session, fmt.Errorf("当前会话在目标平台缺少身份字段 %s，写请求前已阻塞", strings.Join(identity.MissingFields(), "、"))
		}
		configured, decodeErr := decodeConfiguredFormData(runCtx.EffectiveFormData)
		if decodeErr != nil {
			return FormDataPlan{}, session, decodeErr
		}
		target.ApplyUserIdentityForAction(configured, identity, createGlobal)
		encoded, encodeErr := json.Marshal(configured)
		if encodeErr != nil {
			return FormDataPlan{}, session, fmt.Errorf("实时身份覆盖结果编码失败：%w", encodeErr)
		}
		runCtx.EffectiveFormData = encoded
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
		if createGlobal {
			identityReader, canReadIdentity := e.target.(runtimeIdentityReader)
			if canReadIdentity {
				identity, _, identityErr := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, account, session,
					func(callContext context.Context, active target.Session) (target.UserIdentity, error) {
						return identityReader.CurrentUserIdentity(callContext, active)
					})
				if identityErr == nil && identity.Complete() {
					if current == nil {
						current = map[string]any{}
					}
					target.ApplyUserIdentityForAction(current, identity, true)
				}
			}
		}
	}
	plan, err := BuildNodeFormData(runCtx, compiled, current, hasInstance, runCtx.LastFormDataDecision)
	if err == nil && plan.Decision != nil && len(plan.Decision.ValidationIssues) > 0 {
		return plan, session, fmt.Errorf("%s", strings.Join(plan.Decision.ValidationIssues, "；"))
	}
	if err != nil {
		return plan, session, err
	}
	return plan, session, nil
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

// pendingTaskNodeIDOf 返回待办动作本次真正命中的目标节点 ID。
// 实例私有代理重建后必须优先沿用门禁读到的实时任务节点；没有待办事实的发起类动作才回退编译节点。
func pendingTaskNodeIDOf(runCtx RunContext, step model.CompiledActionStep, facts InstanceFacts) string {
	return firstNonEmpty(strings.TrimSpace(facts.CurrentTaskNodeID), stepTargetNodeIDOf(runCtx, step))
}

// nextNodeTargetIDOf 返回本次流转下一业务节点在目标平台的真实代理 ID；
// 审批组件把它作为 nextAuditorList 缺省归属兜底（评审 P1-3）。取不到返回空串。
func nextNodeTargetIDOf(runCtx RunContext, nextIndex int) string {
	nextNodeKey := FollowingActionNodeKey(runCtx.Steps, nextIndex)
	if nextNodeKey == "" {
		return ""
	}
	return strings.TrimSpace(runCtx.Nodes[nextNodeKey].TargetNodeID)
}

// runtimeFormPersonReadonlyFields 计算审批页装载时必须保留的 form_person 源字段。
// 这些键只用于恢复目标页面解析下一节点所需的真实用户值，不进入当前节点编辑权限或本步变更字段。
func runtimeFormPersonReadonlyFields(runCtx RunContext, step model.CompiledActionStep, nextIndex int) []string {
	nextNodeKey := FollowingActionNodeKey(runCtx.Steps, nextIndex)
	nextNodeID := ""
	if info, ok := runCtx.Nodes[nextNodeKey]; ok {
		nextNodeID = strings.TrimSpace(info.TargetNodeID)
	}
	fields := target.CollectFormPersonFieldsForAction(runCtx.FlowTree, string(step.Action), runCtx.FlowType, nextNodeID)
	if len(fields) == 0 {
		fields = filterFormPersonFields(runCtx.FormPersonFields, string(step.Action), runCtx.FlowType, nextNodeID)
	}
	if len(fields) == 0 {
		fields = runCtx.FormPersonFields
	}
	if len(fields) == 0 {
		return nil
	}
	result := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		key := strings.TrimSpace(field.Field)
		for _, suffix := range []string{"__formPersonId", "__condition", "__virtualName"} {
			key = strings.TrimSuffix(key, suffix)
		}
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result
}

// personNameFor 在启动时已解析的人员目录里反查目标用户 ID 对应的姓名；
// 找不到返回空串——宁可少一个展示名，也不用猜测值冒充目标事实。
func personNameFor(runCtx RunContext, userID string) string {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ""
	}
	for _, auditors := range runCtx.NextNodeAuditors {
		for _, auditor := range auditors {
			if strings.TrimSpace(auditor.BizID) == userID {
				return auditor.Name
			}
		}
	}
	return ""
}

// runtimeBizRelevance 把实例当前业务关联转成 runtime 装载载荷形状：
// 审批与转发沿用目标事实，不在页面里凭空重建（评审 P1-3）。
func runtimeBizRelevance(relevance []target.BizRelevance) []runtimeexec.BizRelevance {
	result := make([]runtimeexec.BizRelevance, 0, len(relevance))
	for _, item := range relevance {
		result = append(result, runtimeexec.BizRelevance{OtherBiz: item.OtherBiz, OtherBizID: item.OtherBizID})
	}
	return result
}

// branchAuditTypes 反查节点表生成「目标节点代理 ID → 审批方式」映射：
// runtime 选择闭环据此判定返回分支是否必须携带已配置人员（评审 P1-6）。
func branchAuditTypes(runCtx RunContext) map[string]string {
	result := map[string]string{}
	for _, info := range runCtx.Nodes {
		nodeID := strings.TrimSpace(info.TargetNodeID)
		auditType := strings.TrimSpace(info.AuditType)
		if nodeID != "" && auditType != "" {
			result[nodeID] = auditType
		}
	}
	return result
}

// runtimeIdentityReader 是读取当前会话实时身份的可选能力面（F-035 评审补充）：
// 真实目标客户端实现它；测试假件不必实现，缺省时发起/审批阻塞而不是用历史身份冒充。
type runtimeIdentityReader interface {
	CurrentUserIdentity(ctx context.Context, active target.Session) (target.UserIdentity, error)
}

// verifyFormDataAfterWrite 在写请求成功后重读目标实例当前表单，并按字段所有权核对。
func (e *Executor) verifyFormDataAfterWrite(ctx context.Context, runCtx RunContext, session target.Session, decision *NodeFormDataDecision, log *StepLog) string {
	instanceRef := strings.TrimSpace(runCtx.PathRun.MainInstanceRef)
	if instanceRef == "" {
		return ""
	}
	current, err := e.target.ReadInstanceCurrentData(ctx, session, instanceRef)
	if err != nil {
		return ""
	}
	issues := []string{}
	overlaidSet := map[string]bool{}
	for _, field := range decision.OverlaidFields {
		overlaidSet[field] = true
		// 系统结构性字段（如 flowInstanceId 实例自身 ID）由目标引擎管理，读回可能缺失或被规范化，
		// 不纳入业务写后核对，避免把「实例自身 ID 未原样回显」误判成写入丢失而阻塞整条路径。
		if target.IsSystemStructuralField(field) {
			continue
		}
		if target.IsTargetContextField(field) {
			// 目标会按当前登录会话重写身份上下文，节点间变化不属于业务正文丢失。
			continue
		}
		// 人员选择组件伴生字段（__condition、__formPersonId、__virtualName 后缀）在节点流转后
		// 不一定出现在下一节点的表单视图里，不能以"字段未出现"来判断保存失败。
		if target.IsPersonPickerCompanionField(field) {
			continue
		}
		written, hasWritten := decision.OverlaidValues[field]
		if !hasWritten {
			continue
		}
		ownership := target.ClassifyFormField(field, true, false)
		actual, exists := current[field]
		if ownership.Ownership == target.FieldOwnershipTargetDerived {
			// 目标可能在清空后省略空的伴生键；重发起只要回读不到非空意见即可视为已重置。
			if decision.Action == string(model.ActionResubmit) && exists && !target.IsEmptyDerivedValue(actual) {
				issues = append(issues, fmt.Sprintf("重新发起应清空审核意见字段 %s，但目标仍为 %v", field, actual))
			}
			continue
		}
		if !exists {
			issues = append(issues, fmt.Sprintf("字段 %s 发送后未出现在目标实例数据中", field))
			continue
		}
		if !deepEqualNormalized(written, actual) {
			issues = append(issues, fmt.Sprintf("字段 %s 发送的值未被目标保存（发送 %v，目标 %v）", field, written, actual))
		}
	}
	for field, baseValue := range decision.BaseValues {
		if overlaidSet[field] || containsString(decision.WithheldFields, field) {
			continue
		}
		if target.IsSystemStructuralField(field) {
			continue
		}
		if target.IsTargetContextField(field) {
			// 当前处理人、部门和公司字段由目标会话维护，不做上游业务值保留核对。
			continue
		}
		// 人员选择组件伴生字段在节点流转后不在下一节点的表单视图里，不做跨节点保留核对。
		if target.IsPersonPickerCompanionField(field) {
			continue
		}
		ownership := target.ClassifyFormField(field, false, false)
		actual, exists := current[field]
		if ownership.Ownership == target.FieldOwnershipTargetDerived {
			continue
		}
		if !exists {
			issues = append(issues, fmt.Sprintf("上游业务字段 %s 从目标实例中删除，不能继续", field))
			continue
		}
		if !deepEqualNormalized(baseValue, actual) {
			issues = append(issues, fmt.Sprintf("上游字段 %s 的值在本次写后被改写（写前 %v，目标 %v）", field, baseValue, actual))
		}
	}
	if len(issues) == 0 {
		log.Phase("verify", decision.StepNo, 1, fmt.Sprintf("表单数据写后核对通过：覆盖 %d 个字段均已保存，基线保留字段未被改写（指纹 %s）",
			len(decision.OverlaidFields), decision.PayloadFingerprint))
		return ""
	}
	message := "表单数据写后核对不一致：" + strings.Join(issues, "；")
	log.Phase("verify", decision.StepNo, 1, message)
	return message
}

// pollForHandlerGeneration 对"目标节点已到达但处理人/待办尚未生成"的场景执行有界轮询（F-035/T07）：
// 最多 5 次、总等待不超过 10 秒，每次只重读实例事实（只读），绝不重发写请求。
// 轮询期间任何一次读到处理人或待办即返回最新事实；全部超时仍无处理人时返回阻塞结论，
// 停止原因按 assignment_missing 分型（目标没有产生处理人），不伪装成"已处理"。
func (e *Executor) pollForHandlerGeneration(ctx context.Context, runCtx RunContext, session target.Session, step model.CompiledActionStep, log *StepLog) (InstanceFacts, *target.Session, error) {
	// 预算来自配置（TEST_AUTO_PRO_HANDLER_GENERATION_WAIT / _POLL_INTERVAL），不再写死 10 秒：
	// 用户裁决——目标暂时未就绪要多等一会、重试后再判，而不是立刻硬阻断整条运行。
	totalBudget := orDefaultDuration(e.handlerWait, config.DefaultHandlerGenerationWait)
	pollInterval := orDefaultDuration(e.handlerPollInterval, config.DefaultHandlerGenerationPollInterval)
	maxPolls := int(totalBudget / pollInterval)
	if maxPolls < 1 {
		maxPolls = 1
	}
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
	return facts, nil, fmt.Errorf("目标节点已到达，但等了 %s 仍未生成处理人和待办（assignment_missing）；本步没有发送任何写请求，请到目标平台确认该节点的处理人配置", humanWaitDuration(totalBudget))
}

// humanWaitDuration 把等待预算写成人能读的时长：满一秒按秒，不足一秒按毫秒（单测用毫秒级预算时不显示「0 秒」）。
func humanWaitDuration(d time.Duration) string {
	if d >= time.Second {
		return fmt.Sprintf("%d 秒", int(d.Seconds()))
	}
	return fmt.Sprintf("%d 毫秒", d.Milliseconds())
}

// isRecoveryEffectStep 判断本步是否是"把实例拉回已经走过的节点"的恢复型动作。
// 取回准备同意属于推进型前置步骤，必须遵循目标 isSkip 递归跳过规则；真正的取回、回退和撤回
// 才以实例已经越过目标节点作为合法前提，并由动作专属事实继续裁决。
// 输入边界：只读 step，不改状态；调用方仍须保证动作专属门禁不被跳过。
func isRecoveryEffectStep(step model.CompiledActionStep) bool {
	switch step.Action {
	case model.ActionRetrieve, model.ActionRollback, model.ActionWithdraw:
		return true
	default:
		return false
	}
}

// targetSkippedStepReason 逐一核对本步与全部活动待办，再判断本步是否已被目标跳过。
// 返回值：
//   - action="skip"：模板声明允许跳过（isSkip=true）且实例待办可证明已越过本节点，无写请求记"目标已跳过"；
//   - action="block"：并行上游尚有待办，或待办已越过本节点但模板未声明允许跳过，
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
	pendingNodeID := ""
	expectedNodeID := strings.TrimSpace(info.TargetNodeID)
	seen := map[string]bool{}
	for _, nodeID := range append(append([]string{}, facts.DueNodes...), facts.CurrentNodes...) {
		id := strings.TrimSpace(nodeID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		if id == expectedNodeID {
			return "", "", "", fmt.Sprintf("本步节点「%s」仍有活动待办，不构成跳过", info.Name)
		}
		// 本步下游的共同后继必须等每一条并行支线完成；任何尚在上游的待办
		// 都不能被较早的线性步骤序号掩盖，更不能套用 isSkip 越过汇合点。
		upstream := map[string]bool{}
		forwardReachable(id, expectedNodeID, runCtx, upstream)
		if upstream[expectedNodeID] {
			name := id
			for _, node := range runCtx.Nodes {
				if node.TargetNodeID == id {
					name = node.Name
					break
				}
			}
			return "block", fmt.Sprintf("并行分支「%s」仍有待办，节点「%s」尚未到达；必须完成所有分支后才能继续主流程", name, info.Name), name,
				fmt.Sprintf("活动待办 %s 是本步节点 %s 的真实上游，汇合前不得推进", id, expectedNodeID)
		}
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
				pendingNodeID = id
			}
			break
		}
	}
	if pendingIndex < 0 {
		return "", "", "", fmt.Sprintf("实例当前/待办节点 %v 都不在已配置路径上，无法判定跳过", append(append([]string{}, facts.DueNodes...), facts.CurrentNodes...))
	}
	if pendingIndex <= expectedIndex && !isParallelBranch(expectedNodeID, pendingNodeID, runCtx) {
		return "", "", "", fmt.Sprintf("实例待办仍在「%s」（本步或更早），不构成跳过", pendingName)
	}
	// 兄弟分支正在处理并不能证明本支已经完成：目标可能尚在生成待办，
	// 或刚重新发起全部分支。不能因为线性步骤序号就把本支零写入略过。
	if isParallelBranch(strings.TrimSpace(info.TargetNodeID), pendingNodeID, runCtx) {
		return "block", fmt.Sprintf("并行分支「%s」仍有待办，但无法证明节点「%s」已完成或被目标跳过；请核对全部分支待办后继续", pendingName, info.Name), pendingName,
			fmt.Sprintf("并行兄弟分支「%s」活动不能证明本步节点已完成，禁止按线性序号跳过", pendingName)
	}
	// 待办确实越过本节点。分型依据目标后端规则：只有模板声明 isSkip=true 才递归跳过；
	// run_node_choose 自选节点无论声明如何都要求显式人员，缺失即抛"未设置审批人"。
	if strings.TrimSpace(info.AuditType) == "run_node_choose" {
		return "block", fmt.Sprintf("节点「%s」是运行时自选审批节点，待办已越过但没有可用的已选处理人，不能自动跳过", info.Name), pendingName,
			fmt.Sprintf("实例待办在「%s」，本节点为 run_node_choose，必须阻塞", pendingName)
	}
	if info.IsSkip == nil || !*info.IsSkip {
		return "block", fmt.Sprintf("本节点没有解析到可用处理人；目标未在节点「%s」声明\"无处理人时跳过\"，而实例待办已越过该节点，无法安全继续", info.Name), pendingName,
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
		ReceiverID   string   `json:"receiverId,omitempty"`
		ReceiverName string   `json:"receiverName,omitempty"`
	}{Allowed: preview.GateAllowed, Reason: preview.GateReason, Items: preview.GateItems,
		BranchTarget: branchTarget, FormOverlaid: preview.FormOverlaid, FormWithheld: preview.FormWithheld,
		ReceiverID: preview.ReceiverID, ReceiverName: preview.ReceiverName}
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
		after.StepNodeKey = firstNonEmpty(
			strings.TrimSpace(preview.Facts.CurrentTaskNodeID),
			stepTargetNodeIDOf(runCtx, step),
		)
		if readErr != nil || after.ReadError != "" {
			outcome.RereadFailed = true
			return outcome, 0, nil
		}
		before := approved.Preview.Facts
		before.StepNodeKey = after.StepNodeKey
		reread := ClassifyReread(string(step.Action), after.StepNodeKey,
			forwardPathTargetNodeIDs(approved.RunCtx, approved.NextIndex), before, after)
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
			// 重新核验沿用首次执行的 runtime 投影：写请求没有重发，投影仍是这次尝试的事实。
			RuntimeDetail: preview.runtimeDetailJSON,
			ExecutorMs:    preview.executorMs, ExecutorMsKnown: preview.executorMsKnown,
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
			outcome.SettleDetail = settleEventDetail(preview, userMessage)
			if !outcome.NoMoreSteps {
				if err := e.runState.BackToRunning(ctx, runCtx.PathRun.ID, settleEventDetail(preview, userMessage)); err != nil {
					return outcome, lineNo, err
				}
			}
		case verdict.OutcomeFailed:
			class := model.FailureClassTargetRejected
			if _, err := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusFailed, runResultOf(model.RunResultFailed), &class,
				"重新核验判定失败："+userMessage, settleEventDetail(preview, userMessage)); err != nil {
				return outcome, lineNo, err
			}
		default:
			class := model.FailureClassWriteUncertain
			if _, err := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusAwaitingReconciliation, runResultOf(model.RunResultAwaitingReconcile), &class,
				"重新核验仍未确认执行结果："+userMessage, settleEventDetail(preview, userMessage)); err != nil {
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
		outcome.SettleDetail = settleEventDetail(preview, "导航步骤只读校验通过")
		if !outcome.NoMoreSteps {
			if err := e.runState.BackToRunning(ctx, runCtx.PathRun.ID, settleEventDetail(preview, "导航步骤只读校验通过")); err != nil {
				return outcome, lineNo, err
			}
		}
		// 导航步骤是只读的，没有领取推进权，也就无需释放。
		log.Phase("settle", step.Sequence, 1, "导航步骤完成")
		return outcome, lineNo, nil
	}
	if preview.RecoveryNotApplicable {
		lineNo := log.Phase("settle", step.Sequence, 1, "恢复链未适用："+preview.SkipReason)
		record := model.RunStep{
			PathRunID: runCtx.PathRun.ID, StepNo: step.Sequence, Source: string(step.Source),
			Action: string(step.Action), NodeKey: step.NodeKey, ActorSummary: preview.ActorName,
			Status: model.RunStepSkipped, StartedAt: startedAt, FinishedAt: e.now(),
			GateSnapshot: gateSnapshotJSON(preview, approved.RunCtx.SubmitBranchTargetNodeID),
		}
		attempt := model.RunStepAttempt{
			PathRunID: runCtx.PathRun.ID, AttemptNo: attemptNo, Verdict: "recovery_not_applicable",
			SideEffect: string(verdict.SideEffectNone), Reason: preview.SkipReason,
			Basis:   "触发动作已记录为目标跳过，恢复链无适用前提",
			LogPath: log.RelativePath(), LogLine: lineNo,
		}
		if _, err := e.facts.RecordStepAttempt(ctx, record, attempt, e.now()); err != nil {
			return StepOutcome{Verdict: string(verdict.OutcomeFailed)}, lineNo, err
		}
		outcome.Verdict = string(verdict.OutcomeSucceeded)
		outcome.NoMoreSteps = approved.NextIndex+1 >= len(runCtx.Steps)
		outcome.SettleDetail = settleEventDetail(preview, preview.SkipReason)
		if !outcome.NoMoreSteps {
			if err := e.runState.BackToRunning(ctx, runCtx.PathRun.ID, outcome.SettleDetail); err != nil {
				return outcome, lineNo, err
			}
		}
		log.Phase("settle", step.Sequence, 1, preview.SkipReason)
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
		outcome.SettleDetail = settleEventDetail(preview, preview.SkipReason)
		if !outcome.NoMoreSteps {
			if err := e.runState.BackToRunning(ctx, runCtx.PathRun.ID, settleEventDetail(preview, preview.SkipReason)); err != nil {
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
			"路径在第 "+formatUint(uint64(step.Sequence))+" 步失败："+preview.BlockReason, ""); err != nil {
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
	reportPhase(approved, "prepare", fmt.Sprintf("正在准备第 %d 步的实际操作人：为\"%s\"准备目标平台登录会话；这里只做账号准备，不会提交审批动作，准备好后才会发送一次\"%s\"请求", step.Sequence, preview.ActorName, firstNonEmpty(preview.ActionName, string(step.Action))))
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
		if _, finishErr := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusFailed, runResultOf(model.RunResultFailed), &class, reason, ""); finishErr != nil {
			return outcome, lineNo, finishErr
		}
		return outcome, lineNo, nil
	}
	// 任务级动作以目标实时待办的真实处理人身份发出（工作包 D）：
	// 事实里的 currentPendingUserId 是目标裁决的实际处理人，必须解析其登录账号并切换到当前处理人的会话，
	// 绝不冒用计划账号审批他人任务。解析失败就如实置败并指出节点与人员，不能静默回退到计划账号。
	//
	// 实例级转发同样必须切换：它作用于"本人当前待办里的这条任务"，而原生待办列表按登录人过滤——
	// 用计划账号装载会一直等不到属于处理人的那一行而超时
	// （2026-09-21 运行 85 路径 1 第 43 步实测：节点、标签页、实例与任务号全部正确，
	// 网关侧会话也切到了处理人，但 preview.ActorAccount 仍是计划账号，
	// 于是执行服务拿到的身份依旧是计划账号，backlog 列表渲染了 10 行却无一行属于本实例）。
	// 判据集中在 ScopeActsOnCurrentTodo，今后新增同类动作自动获得同一口径。
	if ScopeActsOnCurrentTodo(step.Scope) {
		if assigneeID := strings.TrimSpace(preview.Facts.CurrentTaskAssigneeID); assigneeID != "" {
			if account, name, resolveErr := e.assigneeAccount(ctx, session, assigneeID); resolveErr != nil {
				class := model.FailureClassActorUnresolved
				reason := "无法解析本节点实际处理人（" + nameOrFallback(preview.Facts.CurrentTaskAssigneeName, assigneeID) + "）的登录账号：" + userFacingError(resolveErr, target.WriteResponse{})
				lineNo := log.Phase("prepare", step.Sequence, attemptNo, reason)
				if recordErr := e.recordPrepareFailure(ctx, runCtx, step, preview, approved.RunCtx.SubmitBranchTargetNodeID,
					attemptNo, approved.IsReplay, startedAt, class, reason, "当前处理人登录身份未确认，没有发出写请求", log.RelativePath(), lineNo); recordErr != nil {
					return outcome, lineNo, recordErr
				}
				if _, finishErr := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusFailed, runResultOf(model.RunResultFailed), &class, reason, ""); finishErr != nil {
					return outcome, lineNo, finishErr
				}
				return outcome, lineNo, nil
			} else if account != "" {
				if !strings.EqualFold(account, runCtx.PlanAccount) {
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
						if _, finishErr := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusFailed, runResultOf(model.RunResultFailed), &class, reason, ""); finishErr != nil {
							return outcome, lineNo, finishErr
						}
						return outcome, lineNo, nil
					}
					session = actorSession
				}
				// 处理人就是计划账号本人时也必须回填执行者账号：runtime 会话创建按 accountKey
				// 强校验，留空会被执行服务以 400 阻塞（2026-09-18 单路径真实运行实测，
				// 协同发起人节点的同意因此失败）。此时会话本就是计划账号会话，无需切换。
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
	reportPhase(approved, "prepare", fmt.Sprintf("当前处理人\"%s\"会话已就绪，即将在\"%s\"节点发送一次\"%s\"请求", preview.ActorName, runCtx.Nodes[step.NodeKey].Name, firstNonEmpty(preview.ActionName, string(step.Action))))
	// 当前处理人账号可能不同于计划账号：计划账号锁与处理人锁同时持有到核验结束，
	// 保证实例"已发"视角读取和真实待办写入不会互相覆盖 SID。

	// 阶段 5：发出唯一一次写请求。审批任务 ID 在发送前现场新鲜读取（当前处理人与待办的新鲜复验）。
	// 上报发生在发出之前：本次调用同步阻塞到目标响应返回，指示器在窗口内如实表达 submit 进行中。
	// submit 阶段的目标请求（含写前待办新鲜读取）同样注入步骤作用域，network.log 可按阶段归属。
	ctx = withStepScope(ctx, step.Sequence, attemptNo, "submit")
	reportPhase(approved, "submit", fmt.Sprintf("正在向目标平台提交第 %d 步：\"%s\"在\"%s\"节点发送一次\"%s\"请求，发送后不会重复提交；完成后会重新查看目标平台确认结果", step.Sequence, preview.ActorName, runCtx.Nodes[step.NodeKey].Name, firstNonEmpty(preview.ActionName, string(step.Action))))
	// 写请求前的租约续期：目标存在约 30 秒的慢请求，租约若在写请求期间过期，
	// 另一执行者可能在核验未完成时领取推进权（评审缺陷 12：RenewLease 此前无调用方）。
	// 续期失败说明推进权已易主，本步必须放弃且绝不发出写请求。
	if err := e.runState.RenewLease(ctx, runCtx.PathRun.ID, fencingToken); err != nil {
		return outcome, 0, err
	}
	// 写端点可能明确拒绝过期会话，执行器会在确认请求未进入业务后换新会话重发。
	// 后续事实重读必须沿用真正完成写入的会话，否则会拿旧 SID 做完整退避，
	// 把已经成功的目标写误判成"声明成功但事实不可读"。
	if e.shouldRunViaRuntime(runCtx, step) {
		// runtime 已接管写路径：回填 preview 写事实后走既有核验/落账，绝不调用 TargetClient 写出口。
		e.runViaRuntime(ctx, runCtx, step, session, preview, log, approved)
	} else {
		session = e.refreshAndSubmit(ctx, runCtx, step, session, preview, log, reportPhase, approved, attemptNo)
	}
	if !preview.writeSent {
		// 零写入：写请求没有发出（发送前的待办新鲜复验失败或载荷缺失）。
		// 没有发出的请求不存在"结果不确定"——把零写入判成不确定会把无副作用的失败
		// 说成"可能已经写进目标"，还会把用户引向对账；这里按真实分类如实置败。
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
			// runtime 零写入失败同样保留三阶段投影：请求轨迹能证明「流程动作未发出」。
			RuntimeDetail: preview.runtimeDetailJSON,
			ExecutorMs:    preview.executorMs, ExecutorMsKnown: preview.executorMsKnown,
		}
		if _, err := e.facts.RecordStepAttempt(ctx, record, attempt, e.now()); err != nil {
			return outcome, lineNo, err
		}
		if _, err := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusFailed, runResultOf(model.RunResultFailed), &class, reason, ""); err != nil {
			return outcome, lineNo, err
		}
		log.Phase("settle", step.Sequence, attemptNo, "执行失败已记录，路径运行置为失败")
		return outcome, lineNo, nil
	}
	if step.Action == model.ActionAddSign && preview.writeErr == nil {
		// 记录加签写请求实际使用的锚点（写前真实当前待办节点）：控制层重绑定必须用它作插入锚点，
		// 而不是离线编译配置的节点键——真实流转后两者可能不一致，用错锚点会导致整棵子链平移错位。
		if request, ok := preview.request.(*target.ActionWriteRequest); ok {
			outcome.AddSignAnchorNodeID = strings.TrimSpace(request.NodeProxyID)
		}
		proxyID, nodeID, parseErr := parseAddSignWriteData(preview.writeResponse.Data)
		if parseErr != nil {
			log.Phase("verify", step.Sequence, attemptNo, "加签响应未返回新的流程代理标识，后续任务将重新读取实时待办："+parseErr.Error())
		} else {
			outcome.FlowProxyID = proxyID
			outcome.AddSignResponseNodeID = nodeID
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
	reportPhase(approved, "verify", fmt.Sprintf("第 %d 步的请求已经发出，正在重新查看目标平台的流程节点和待办，确认\"%s\"是否已经生效；这里不会再次发送动作，确认后保存结果并继续下一步", step.Sequence, firstNonEmpty(preview.ActionName, string(step.Action))))
	// 重读对照一律用目标真实节点标识：目标返回的当前节点与待办都是真实标识，
	// 拿工具侧不透明键去比会永远"待办已消失"，把没生效的写误判成已前进。
	// 动态代理重建后，编译节点 ID 可能已经失效；写前门禁命中的真实任务节点才是本次写的核验锚点。
	stepTargetNodeID := firstNonEmpty(
		strings.TrimSpace(preview.Facts.CurrentTaskNodeID),
		strings.TrimSpace(runCtx.Nodes[step.NodeKey].TargetNodeID),
	)
	before := preview.Facts
	before.StepNodeKey = stepTargetNodeID
	after, session, readErr := e.readFactsWithRetry(ctx, runCtx, session, step)
	after.StepNodeKey = stepTargetNodeID
	// F-035 评审补充：写后表单数据逐字段核对——本次发送的覆盖字段必须已保存成目标值，
	// 基线里保留的上游字段必须保持原值。这不是目标表单校验（用户已豁免），
	// 而是工具自己的写副作用事实核对；核对结论随核验事实落账，供下一步门禁阻塞使用。
	if readErr == nil && strings.TrimSpace(after.ReadError) == "" && preview.FormDataDecision != nil && ActionCarriesFormData(step.Action) {
		if issue := e.verifyFormDataAfterWrite(ctx, runCtx, session, preview.FormDataDecision, log); issue != "" {
			after.FormDataVerifyIssue = issue
			after.FormDataVerifyResults = []string{issue}
		}
		after.FormDataVerifyAt = e.now().UTC().Format(time.RFC3339)
		after.FormDataVerifyFingerprint = preview.FormDataDecision.PayloadFingerprint
	}
	if readErr == nil && strings.TrimSpace(after.ReadError) == "" {
		// 核验重读同时把实例名称补进日志作用域与 meta.json；写后屏障之后的新事实优先。
		ctx = e.annotateInstanceFromFacts(ctx, runCtx, log, after)
		if step.Action == model.ActionRetrieve {
			// 取回写后目标会返回新批次的当前节点集合；控制层必须以这份事实重绑定代理树，
			// 否则下一步仍会把编译期旧节点当作当前待办，无法发现目标新建的 pending。
			outcome.FlowProxyID = strings.TrimSpace(after.FlowProxyID)
			outcome.CurrentNodeProxyIDs = append([]string(nil), after.CurrentNodes...)
			if len(outcome.CurrentNodeProxyIDs) > 0 {
				outcome.CurrentNodeProxyID = strings.TrimSpace(outcome.CurrentNodeProxyIDs[0])
			}
		}
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
		if backErr := e.runState.BackToRunning(ctx, runCtx.PathRun.ID, ""); backErr != nil {
			return outcome, 0, backErr
		}
		return outcome, 0, nil
	}
	// 核验事实摘要落 step.log：会签等多人场景下「节点待办未清空」是常态，
	// 不把判定依据（当前处理人待办/已办核对结果）写下来，事后无法解释结论怎么来的。
	// 「当前处理人已办」必须打印语义分型而非布尔对：布尔只能表达读没读/读没读到，
	// 无法区分「确实核对过并命中」「核对过但没有」「根本没做这项核对」（第三轮复核 P0-4）。
	log.Phase("verify", step.Sequence, attemptNo, fmt.Sprintf("核验事实：实例可读=%v 状态=%s 当前处理人待办=%v（发现能力=%v） 当前处理人已办=%s 实例当前节点=%v",
		after.Found, after.Status, after.CurrentTaskFound, after.CurrentTaskRead,
		completedTaskBasisName(after.CompletedTaskBasis), after.CurrentNodes))
	reread := ClassifyReread(string(step.Action), stepTargetNodeID,
		forwardPathTargetNodeIDs(runCtx, approved.NextIndex), before, after)
	if reread == verdict.RereadUnreadable && preview.writeSent && preview.writeErr == nil {
		// 核验读取失败（工具会话失效/目标抖动导致事实不可读）：写请求已发出且目标声明成功，
		// 但工具没能确认结果。2026-09-11 用户裁决：工具自己的读取错误不得把运行判成终局——
		// 停在本步、保留现场、不落结论尝试行；重新放行只重读核验结果（Reverify），不重发写请求。
		message := "核验读取失败，已停在本步；重新放行只重新读取结果，不会重复发写请求"
		log.Phase("verify", step.Sequence, attemptNo, message)
		reportPhase(approved, "verify", message)
		outcome.RereadFailed = true
		if backErr := e.runState.BackToRunning(ctx, runCtx.PathRun.ID, ""); backErr != nil {
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
		// 加签响应的 data 在部分目标版本只返回当前节点，完整的新代理标识只出现在写后事实。
		// 先把事实中的 flowProxyId 回填到结果，控制层才能触发整棵代理树重绑定。
		if strings.TrimSpace(outcome.FlowProxyID) == "" {
			outcome.FlowProxyID = strings.TrimSpace(after.FlowProxyID)
		}
		// 加签响应里的 currentNodeProxyId 只作诊断；并行/条件流转时它可能是接口
		// 处理游标，而写后实例 CurrentNodes 才是后续代理重绑定的权威活动节点集合。
		outcome.CurrentNodeProxyIDs = append([]string(nil), after.CurrentNodes...)
		if len(outcome.CurrentNodeProxyIDs) > 0 {
			outcome.CurrentNodeProxyID = strings.TrimSpace(outcome.CurrentNodeProxyIDs[0])
		}
		if responseNodeID := strings.TrimSpace(outcome.AddSignResponseNodeID); responseNodeID != "" {
			log.Phase("verify", step.Sequence, attemptNo, fmt.Sprintf(
				"加签响应节点仅作接口游标诊断：响应=%s，写后实例活动节点=%v",
				responseNodeID, outcome.CurrentNodeProxyIDs,
			))
		}
		reread = e.classifyAddSignReread(ctx, runCtx, step, session, preview, after, outcome)
	} else if step.Action == model.ActionForward {
		reread, outcome.AuxiliaryInstanceRef = e.classifyForwardReread(ctx, runCtx.PlanAccount, session, preview)
		// 转发的重读依据必须落进 step.log：转发不推进节点，"结果无法确认"这条结论
		// 若不带读到了什么、缺了什么，事后无法复核，也无法与目标事实对照。
		if diag := strings.TrimSpace(preview.Facts.AssigneeDiag); diag != "" {
			log.Phase("verify", step.Sequence, attemptNo, "转发结果核对依据："+diag)
		}
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
	// 静态可预期的编排冲突（用户配置的可选动作 + 确定失败 + 无副作用 + 非恢复链触发动作）不阻断整条运行：
	// 按「警告并跳过该动作」处理，运行继续往下走。恢复链触发动作（不同意/撤回/回退）失败不在此列——
	// 它们后面挂着依赖其生效的恢复步骤，跳过会留下悬空恢复链，仍按失败停止。
	skippableWarning := verdictResult.Outcome == verdict.OutcomeFailed &&
		verdictResult.SideEffect == verdict.SideEffectNone &&
		step.Source == model.ActionStepSourceUser &&
		!isForwardRecoveryAction(step.Action)
	settleStatus := statusOfVerdict(verdictResult.Outcome)
	settleVerdictTag := string(verdictResult.Outcome)
	settleReason := userMessage
	if skippableWarning {
		settleStatus = model.RunStepSkipped
		settleVerdictTag = StepVerdictSkippedWarning
		// 跳过的步骤不该显示成「执行失败」：去掉失败前缀，改成明确的「已跳过并继续」并保留目标原话。
		settleReason = "已跳过并继续（该动作在当前流程状态下无法执行）：" + strings.TrimPrefix(userMessage, "执行失败：")
	}
	record := model.RunStep{
		PathRunID:    runCtx.PathRun.ID,
		StepNo:       step.Sequence,
		Source:       string(step.Source),
		Action:       string(step.Action),
		NodeKey:      step.NodeKey,
		ActorSummary: preview.ActorName,
		Status:       settleStatus,
		StartedAt:    startedAt,
		FinishedAt:   e.now(),
		GateSnapshot: gateSnapshotJSON(preview, approved.RunCtx.SubmitBranchTargetNodeID),
	}
	attempt := model.RunStepAttempt{
		PathRunID:   runCtx.PathRun.ID,
		AttemptNo:   attemptNo,
		Verdict:     settleVerdictTag,
		SideEffect:  string(verdictResult.SideEffect),
		Transport:   string(target.TransportOf(preview.writeErr)),
		Initial:     string(verdictResult.Initial),
		Reread:      string(reread),
		Reason:      settleReason,
		Basis:       verdictResult.Basis,
		TraceID:     preview.writeTraceID,
		CurlTraceID: preview.writeTraceID,
		LogPath:     log.RelativePath(),
		LogLine:     lineNo,
		DurationMs:  durationMs,
		IsReplay:    approved.IsReplay,
		// BeforeFacts 复用现有 JSON 列同时携带写后核对结论：不新增表。
		// 下一步按 pathRunId 读取最近尝试，重启后仍能恢复阻塞结论。
		BeforeFacts: EncodeInstanceFacts(mergeVerifyIssue(before, after)),
		// runtime 三阶段投影与请求轨迹随尝试行落库（评审 P1-7）：详情接口据此展示
		// runtime 三阶段、请求轨迹与业务失败状态；非 runtime 步骤为空。
		RuntimeDetail: preview.runtimeDetailJSON,
		ExecutorMs:    preview.executorMs, ExecutorMsKnown: preview.executorMsKnown,
	}
	if _, err := e.facts.RecordStepAttempt(ctx, record, attempt, e.now()); err != nil {
		return outcome, lineNo, err
	}

	if skippableWarning {
		// 预期内失败：不结束路径运行，标记跳过并推进游标继续下一步（与目标自动跳过同一推进语义）。
		outcome.Verdict = string(verdict.OutcomeSucceeded)
		outcome.NoMoreSteps = approved.NextIndex+1 >= len(runCtx.Steps)
		outcome.SettleDetail = settleEventDetail(preview, settleReason)
		if !outcome.NoMoreSteps {
			if err := e.runState.BackToRunning(ctx, runCtx.PathRun.ID, settleEventDetail(preview, settleReason)); err != nil {
				return outcome, lineNo, err
			}
		}
		// 必须释放本步的推进权（fencing token）：否则租约仍被本步持有，下一步 ClaimExecution
		// 会拿到「路径运行已被其他执行者占用」而卡住整条运行（与确定成功分支同一释放纪律）。
		_ = e.runState.ReleaseExecution(ctx, runCtx.PathRun.ID, fencingToken)
		log.Phase("settle", step.Sequence, attemptNo, settleReason)
		return outcome, lineNo, nil
	}

	switch verdictResult.Outcome {
	case verdict.OutcomeSucceeded:
		outcome.Verdict = string(verdict.OutcomeSucceeded)
		outcome.NoMoreSteps = approved.NextIndex+1 >= len(runCtx.Steps)
		outcome.SettleDetail = settleEventDetail(preview, userMessage)
		// 路径偏离判据（T04）：只用重读到的真实事实——实际当前节点集合里没有已配置路径的下一个预期节点。
		// 两边必须在同一键空间比较：CurrentNodes 是目标 nodeProxyId，预期节点必须取节点表反查出的
		// TargetNodeID，拿编译场景的哈希键比较永远不相等，会把每一步都误判成偏离（评审 P1）。
		// 判定保守：实例不可见、没有当前节点事实、或预期节点拿不到真实标识时不声称偏离。
		// 加签会在控制层随后读取最新待办和代理树并整体重绑定节点；此处仍处于
		// 重绑定前，旧配置节点 ID 与目标写后新节点 ID 不在同一事实空间，不能据此
		// 判定路径偏离。若重绑定无法唯一证明，控制层会清空结构并强制停止放行。
		if step.Action != model.ActionAddSign && !outcome.NoMoreSteps {
			nextStep := runCtx.Steps[approved.NextIndex+1]
			// 恢复链与系统导航本来就会跨越节点或回到前序节点，不能按"下一步必须仍在路径上"判定偏离；
			// 普通用户步骤和固定尾动作仍做真实节点对照，真实分支偏离继续立即停止。
			if nextStep.Source != model.ActionStepSourceRecovery && nextStep.Source != model.ActionStepSourceNavigation {
				currentStepTarget := strings.TrimSpace(preview.TargetNodeID)
				stillAtCurrentNode := currentStepTarget != "" && containsNode(after.CurrentNodes, currentStepTarget)

				// 收集当前动作之后路径上所有合法的后续目标节点（含紧随的下一个节点、跳过节点后的下一业务节点）：
				// 流程向前推进时，目标可能穿透空节点或跳过无处理人节点前进到后续节点。
				// 只有当前节点既不是当前步节点、也不是路径上任何合法的后续预期节点时，才判定为真正的路径偏离。
				hasValidNextNode := false
				for futureIdx := approved.NextIndex + 1; futureIdx < len(runCtx.Steps); futureIdx++ {
					futureKey := strings.TrimSpace(runCtx.Steps[futureIdx].NodeKey)
					if futureKey == "" {
						continue
					}
					if nodeInfo, ok := runCtx.Nodes[futureKey]; ok {
						futureTarget := strings.TrimSpace(nodeInfo.TargetNodeID)
						if futureTarget != "" && containsNode(after.CurrentNodes, futureTarget) {
							hasValidNextNode = true
							break
						}
					}
				}

				if after.Found && len(after.CurrentNodes) > 0 && !stillAtCurrentNode && !hasValidNextNode {
					outcome.DeviationDetected = true
					log.Phase("settle", step.Sequence, attemptNo, fmt.Sprintf("路径偏离：实际当前节点 %v 不在当前节点或后续预期路径节点中", after.CurrentNodes))
				}
			}
		}
		if !outcome.NoMoreSteps {
			if err := e.runState.BackToRunning(ctx, runCtx.PathRun.ID, settleEventDetail(preview, userMessage)); err != nil {
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
			"第 "+formatUint(uint64(step.Sequence))+" 步"+userMessage, settleEventDetail(preview, userMessage)); err != nil {
			return outcome, lineNo, err
		}
		log.Phase("settle", step.Sequence, attemptNo, userMessage+"，路径运行置为失败")
	default:
		// 无法确认执行结果：路径运行进入待确认并停止，避免重复执行真实业务操作。
		outcome.Verdict = string(verdict.OutcomeUncertain)
		class := model.FailureClassWriteUncertain
		if _, err := e.runState.Finish(ctx, runCtx.PathRun.ID, model.PathRunStatusAwaitingReconciliation, runResultOf(model.RunResultAwaitingReconcile), &class,
			"第 "+formatUint(uint64(step.Sequence))+" 步"+userMessage, settleEventDetail(preview, userMessage)); err != nil {
			return outcome, lineNo, err
		}
		log.Phase("settle", step.Sequence, attemptNo, userMessage+"，路径运行已停止")
	}
	return outcome, lineNo, nil
}

// isForwardRecoveryAction 判断动作是否属于恢复链关键动作（不同意/撤回/回退/取回）。
// 这类动作失败不能「跳过继续」：取回是用户配置的关键流转动作，其后有依赖其生效的恢复链；
// 目标明确拒绝时必须如实判定为失败并保留目标原话，绝不能伪装跳过。
func isForwardRecoveryAction(action model.ActionKey) bool {
	switch action {
	case model.ActionReject, model.ActionWithdraw, model.ActionRollback, model.ActionRetrieve:
		return true
	default:
		return false
	}
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

// taskStatusDoneValue 是目标任务列表里「已办」的状态名，与目标协议一致。
// 取回动作只能绑定这个状态下、且属于本步节点的任务：已办与节点强绑定，不允许撇开节点回退匹配。
const taskStatusDoneValue = "done"

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
		//
		// done（已办）绝不允许这份"撇开节点"的回退（2026-09-21 夜间路径 9 实测）：
		// 取回要求的是"本人在本节点留下的那条已办"，一旦去掉节点过滤，回退会把实例上任意一条
		// 已办任务（实测命中加签节点 2843705d… 的 done 任务）绑到取回请求的 jobTaskId 上，
		// 目标对该节点返回「空指针异常」；更危险的是它可能成功取回一个并非本步节点的已办，
		// 让工具在错误节点上产生真实副作用。节点 ID 因代理重建失效时，正确做法是让本步
		// 如实读空并按"没有已办"阻塞，绝不用其他节点的已办顶替。
		if strings.TrimSpace(status) == taskStatusDoneValue {
			return snapshot, err
		}
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

// refreshPendingTaskSnapshotForAssignee 是提交阶段的待办刷新，必须带「视角用户」。
//
// 为什么必须带视角（第三轮复核 P0-3）：目标的 `/web/flowJobTaskLink/list` 只在
// 请求顶层 queryUserId 为空时才按 SID 解析当前用户。引擎此前在 submit 阶段一律发空视角查询，
// 等于「只看执行者自己名下的待办」：处理人恰好是执行者本人时侥幸命中（run 2 正是这种巧合），
// 一旦待办挂在别的账号名下就必然查空，再被上层解释成「目标节点未生成处理人」——
// 这正是历史上该误判的直接成因。
//
// 视角选择纪律（不猜测、不越权）：门禁阶段已解析出处理人时，提交阶段直接使用同一处理人
// 视角读取任务；不能先用执行者视角碰运气，否则会把"执行者本人恰好能看到"误当成通用保证。
// 只有门禁没有解析出处理人时，才允许沿用当前会话视角，且该路径会在无任务时阻塞。
//
// 视角补查用的是门禁阶段已经核实过的同一个人，不是候选名单，也不是用计划账号冒充处理人；
// 补查未命中时如实返回空快照，由调用方按既有门禁阻塞，绝不猜测任务号。
//
// 返回值 session 是真正完成读取的会话，供后续写请求与核验复用。
func (e *Executor) refreshPendingTaskSnapshotForAssignee(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, nodeID string, session target.Session, assigneeUserID string) (target.Session, target.TaskSnapshot, error) {
	instanceID := strings.TrimSpace(runCtx.PathRun.MainInstanceRef)
	viewReader, hasView := e.target.(taskSnapshotViewReader)
	assigneeUserID = strings.TrimSpace(assigneeUserID)
	if hasView && instanceID != "" && assigneeUserID != "" {
		account := strings.TrimSpace(session.Summary.Account)
		if account == "" {
			account = runCtx.PlanAccount
		}
		snapshots, active, viewErr := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, account, session,
			func(callContext context.Context, active target.Session) ([]target.TaskSnapshot, error) {
				return viewReader.ListTaskSnapshotsForUser(callContext, active, instanceID, "pending", assigneeUserID)
			})
		if viewErr != nil {
			return active, target.TaskSnapshot{}, viewErr
		}
		wantNode := strings.TrimSpace(nodeID)
		for _, candidate := range snapshots {
			if wantNode != "" && strings.TrimSpace(candidate.FlowNodeProxyID) != wantNode {
				continue
			}
			candidate.PendingUserID = assigneeUserID
			// 命中待办后把会话切到**处理人本人**：待办列表按登录人过滤，后续装载/提交必须由
			// 待办归属人发起，否则目标侧看不到这一行（转发装载等待行超时的根因）。
			// 处理人账号解析不到时保持原会话并如实回落，不猜测、不冒用计划账号。
			if handlerSession, ok := e.handlerSessionFor(ctx, runCtx, assigneeUserID); ok {
				return handlerSession, candidate, nil
			}
			return active, candidate, nil
		}
		return active, target.TaskSnapshot{}, nil
	}
	if assigneeUserID != "" {
		return session, target.TaskSnapshot{}, fmt.Errorf("目标客户端不支持按已核实处理人视角读取待办")
	}
	// 只有门禁没有解析出当前处理人时才允许沿用会话自身视角；此时日志事实会说明"执行者即视角来源"。
	return e.refreshTaskSnapshot(ctx, runCtx, step, nodeID, session, "pending")
}

// handlerSessionFor 按处理人用户 ID 解析出其登录账号并取得会话，供需要以处理人身份发起的动作使用。
//
// 为什么需要它：原生待办列表按登录人过滤，转发这类"作用于本人待办"的动作必须由待办归属人发起；
// 用计划账号装载会一直等不到匹配行（2026-09-21 运行 82 路径 1 第 43 步实测：
// 装载模式=forward、标签页=backlog、实例与任务号均正确，但会话仍是计划账号，而待办属于他人）。
//
// 解析口径：走人员目录按用户 ID 反查账号，命中且账号非空才登录；
// 解析不到、账号为空、登录失败都返回 false，由调用方保持原会话并如实回落——
// 绝不猜测人员，也绝不用计划账号冒充处理人。
// 输入边界：assigneeUserID 为空直接返回 false，不发任何目标请求。
func (e *Executor) handlerSessionFor(ctx context.Context, runCtx RunContext, assigneeUserID string) (target.Session, bool) {
	assigneeUserID = strings.TrimSpace(assigneeUserID)
	if assigneeUserID == "" {
		return target.Session{}, false
	}
	resolver, ok := e.target.(userAccountResolver)
	if !ok {
		return target.Session{}, false
	}
	planSession, planErr := e.sessions.Current(ctx, runCtx.PlanAccount)
	if planErr != nil {
		return target.Session{}, false
	}
	accounts, _, resolveErr := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, planSession.Summary.Account, planSession,
		func(callContext context.Context, active target.Session) (map[string]target.UserAccount, error) {
			return resolver.UserAccountsByID(callContext, active, []string{assigneeUserID})
		})
	if resolveErr != nil {
		return target.Session{}, false
	}
	info, found := accounts[assigneeUserID]
	if !found {
		return target.Session{}, false
	}
	account := strings.TrimSpace(info.Account)
	if account == "" {
		return target.Session{}, false
	}
	handlerSession, sessionErr := e.sessions.Current(ctx, account)
	if sessionErr != nil {
		return target.Session{}, false
	}
	return handlerSession, true
}

// refreshPendingTaskSnapshotWithFallback 保留任务级动作的会话优先顺序；自身视角读空时才补查已核实处理人。
func (e *Executor) refreshPendingTaskSnapshotWithFallback(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, nodeID string, session target.Session, assigneeUserID string) (target.Session, target.TaskSnapshot, error) {
	active, snapshot, err := e.refreshTaskSnapshot(ctx, runCtx, step, nodeID, session, "pending")
	if err != nil || strings.TrimSpace(snapshot.JobTaskID) != "" {
		return active, snapshot, err
	}
	instanceID := strings.TrimSpace(runCtx.PathRun.MainInstanceRef)
	viewReader, hasView := e.target.(taskSnapshotViewReader)
	assigneeUserID = strings.TrimSpace(assigneeUserID)
	if !hasView || instanceID == "" || assigneeUserID == "" {
		return active, snapshot, nil
	}
	account := strings.TrimSpace(active.Summary.Account)
	if account == "" {
		account = runCtx.PlanAccount
	}
	snapshots, active, viewErr := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, account, active,
		func(callContext context.Context, current target.Session) ([]target.TaskSnapshot, error) {
			return viewReader.ListTaskSnapshotsForUser(callContext, current, instanceID, "pending", assigneeUserID)
		})
	if viewErr != nil {
		return active, snapshot, nil
	}
	for _, candidate := range snapshots {
		if nodeID != "" && strings.TrimSpace(candidate.FlowNodeProxyID) != strings.TrimSpace(nodeID) {
			continue
		}
		candidate.PendingUserID = assigneeUserID
		return active, candidate, nil
	}
	return active, snapshot, nil
}

// prepareAuditTask 在同意动作写入前读取实时任务号与流程代理，避免会话刷新后沿用预览阶段的旧身份。
func (e *Executor) prepareAuditTask(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, request *target.AuditCurrentTaskRequest, session target.Session, allowRefresh bool, facts InstanceFacts) (target.Session, error) {
	nodeID := pendingTaskNodeIDOf(runCtx, step, facts)
	var snapshot target.TaskSnapshot
	var err error
	if allowRefresh {
		// 待办刷新必须带门禁阶段已核实的处理人视角：空视角查询等于只看执行者自己名下的待办，
		// 处理人不是执行者时必然查空（第三轮复核 P0-3）。
		session, snapshot, err = e.refreshPendingTaskSnapshotForAssignee(ctx, runCtx, step, nodeID, session, strings.TrimSpace(facts.CurrentTaskAssigneeID))
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

// retrieveDoneSnapshotByAuditLink 用审核记录锁定的关联行 ID 找回被目标改写节点的已办行。
//
// 职责：读本实例审核记录，取本步骤节点最近一次「通过」记录的 FlowJobTaskID（任务关联行 ID），
// 再到已办列表里取出同 LinkID 的那一行，作为取回写请求的任务身份。
//
// 为什么必须存在（2026-09-21 运行 94 路径 3 第 6 步实测）：目标在本节点办结后，会把该任务关联行的
// flowNodeProxyID 改写成"实例流转到的下一节点"（实测同一条 jobTaskId 858ded51… 从发起人
// a408500c… 变成省区经理 b9fc434e…）。因此"按本步节点等值过滤已办行"必然读空，
// 写前报「目标上已无当前处理人在本节点的已办任务，无法执行取回」，把可取回的实例拦在写前。
// 审核记录按办结时点落库、节点字段不被改写，是找回同一行的同源依据。
//
// 输入边界：
//   - 只读，不发写请求；
//   - facts 为 nil、节点为空或目标不支持所需读取时返回 ok=false，由调用方按既有纪律阻塞。
//
// 关键约束：
//   - 只接受 audit 里出现的关联行 ID 与已办行 LinkID 严格相等的一行；命中多行说明事实自相矛盾，
//     返回 ok=false 让本步阻塞，绝不任选其一；
//   - 找不到任何一行时返回 ok=false，绝不用其他节点的已办顶替，也绝不猜测任务号；
//   - 补救过程与失败原因写入 facts.AssigneeDiag，随既有诊断日志输出，不新增日志依赖。
func (e *Executor) retrieveDoneSnapshotByAuditLink(ctx context.Context, runCtx RunContext, session target.Session, step model.CompiledActionStep, nodeID string, facts *InstanceFacts) (target.TaskSnapshot, bool) {
	nodeID = strings.TrimSpace(nodeID)
	instanceID := strings.TrimSpace(runCtx.PathRun.MainInstanceRef)
	if facts == nil || nodeID == "" || instanceID == "" {
		return target.TaskSnapshot{}, false
	}
	auditReader, hasAuditReader := e.target.(auditRecordsReader)
	listReader, hasListReader := e.target.(taskSnapshotListReader)
	if !hasAuditReader || !hasListReader {
		return target.TaskSnapshot{}, false
	}
	// 审核记录里本节点最近一次通过记录的关联行 ID：后写覆盖即取最近一次。
	linkID := ""
	records, auditErr := auditReader.ListAuditRecords(ctx, session, instanceID)
	if auditErr != nil {
		facts.AssigneeDiag = "取回已办行补救：审核记录读取失败，按既有纪律阻塞：" + auditErr.Error()
		return target.TaskSnapshot{}, false
	}
	nodeName := ""
	if info, ok := runCtx.Nodes[step.NodeKey]; ok {
		nodeName = strings.TrimSpace(info.Name)
	}
	for _, record := range records {
		match := strings.TrimSpace(record.FlowNodeProxyID) == nodeID
		if !match && nodeName != "" && strings.TrimSpace(record.AuditNodeName) != "" && strings.TrimSpace(record.AuditNodeName) == nodeName {
			match = true
		}
		if !match {
			continue
		}
		if !strings.EqualFold(record.AuditStatus, "pass") {
			continue
		}
		if candidate := strings.TrimSpace(record.FlowJobTaskID); candidate != "" {
			linkID = candidate
		}
	}
	if linkID == "" {
		facts.AssigneeDiag = "取回已办行补救：审核记录里没有本节点的通过记录，按既有纪律阻塞"
		return target.TaskSnapshot{}, false
	}
	done, listErr := listReader.ListTaskSnapshots(ctx, session, instanceID, "done")
	if listErr != nil {
		facts.AssigneeDiag = "取回已办行补救：已办列表读取失败，按既有纪律阻塞：" + listErr.Error()
		return target.TaskSnapshot{}, false
	}
	matched := make([]target.TaskSnapshot, 0, 1)
	for _, snapshot := range done {
		if strings.TrimSpace(snapshot.LinkID) == linkID {
			matched = append(matched, snapshot)
		}
	}
	if len(matched) != 1 {
		// 0 条=确实找不回；>1 条=实例事实自相矛盾。两种都不能任选，保持阻塞。
		facts.AssigneeDiag = fmt.Sprintf("取回已办行补救：按关联行 ID 定位到 %d 条已办行，无法唯一确定取回对象，按既有纪律阻塞", len(matched))
		return target.TaskSnapshot{}, false
	}
	facts.AssigneeDiag = "取回已办行按审核记录关联行 ID 找回：本步节点=" + nodeID + "，该行节点已被目标改写为流转后的节点，改用关联行 ID 定位"
	return matched[0], true
}

// RetrieveDoneSnapshotByAuditLinkForTest 暴露取回写前的已办行补救入口，供 test 目录锁定：
// 已办行按节点过滤读空时，用审核记录里本节点最近一次通过记录的关联行 ID 找回同一行；
// 拿不到唯一一行时返回 ok=false（绝不任选）。
func (e *Executor) RetrieveDoneSnapshotByAuditLinkForTest(ctx context.Context, runCtx RunContext, session target.Session, step model.CompiledActionStep, nodeID string) (target.TaskSnapshot, bool) {
	facts := InstanceFacts{}
	return e.retrieveDoneSnapshotByAuditLink(ctx, runCtx, session, step, nodeID, &facts)
}

// retrieveNodeHasAuditPass 判断本步节点是否确实被办结过，作为取回准备同意能否跳过的前提。
//
// 职责：读本实例审核记录，确认存在节点等于本步节点、状态为「通过」的记录。
//
// 为什么必须存在（2026-09-21 运行 100/101 路径 5 第 12 步实测）：done 读取按本步节点等值过滤，
// 而目标在本节点办结后会把该任务关联行的 flowNodeProxyID 改写成"实例流转到的下一节点"，
// 所以"读到了已办"并不等于"这个节点办结过"。路径 5 的省区经理从未被审批，本步却按一行他处已办
// 跳过准备同意，紧接着的取回便指向没有已办任务的节点，实例停在原地，
// 下一步报「当前待办已经处理」而停止整条路径。
//
// 输入边界：
//   - 只读，不发写请求；instanceID 或 nodeID 为空时返回 false，由调用方落到普通同意门禁；
//   - 目标不支持审核记录读取时返回 false（宁可多走一次普通同意门禁，也不无依据跳过）。
//
// 关键约束：
//   - 只认节点严格相等且状态为「通过」的记录；
//   - 读取失败返回 false，绝不吞错后当作"已办结"放行跳过。
func (e *Executor) retrieveNodeHasAuditPass(ctx context.Context, session target.Session, instanceID, nodeID string) bool {
	return e.retrieveNodeHasAuditPassWithName(ctx, session, instanceID, nodeID, "")
}

func (e *Executor) retrieveNodeHasAuditPassWithName(ctx context.Context, session target.Session, instanceID, nodeID, nodeName string) bool {
	if strings.TrimSpace(instanceID) == "" || (strings.TrimSpace(nodeID) == "" && strings.TrimSpace(nodeName) == "") {
		return false
	}
	auditReader, hasAuditReader := e.target.(auditRecordsReader)
	if !hasAuditReader {
		return false
	}
	records, auditErr := auditReader.ListAuditRecords(ctx, session, instanceID)
	if auditErr != nil {
		return false
	}
	for _, record := range records {
		match := nodeID != "" && strings.TrimSpace(record.FlowNodeProxyID) == strings.TrimSpace(nodeID)
		if !match && nodeName != "" && strings.TrimSpace(record.AuditNodeName) != "" && strings.TrimSpace(record.AuditNodeName) == strings.TrimSpace(nodeName) {
			match = true
		}
		if !match {
			continue
		}
		if strings.EqualFold(record.AuditStatus, "pass") {
			return true
		}
	}
	return false
}

// retrieveDoneRowProvesNodeHandled 判断已办读取回来的这一行能否证明"本步节点确实已办结"。
//
// 职责：把取回准备同意的跳过判据收敛成一个明确的证明关系：
// 已办行必须读到了任务号，且必须经审核记录正面证明本节点存在「通过」记录；
// 避免因目标后端将前驱节点的已办任务节点 ID 改写为流转后的当前节点，导致本节点准备同意被误跳过。
//
// 输入边界：
//   - 快照任务号为空时直接返回 false（没有已办可依据）；
//   - instanceID 或 nodeID 为空时返回 false。
//
// 关键约束：
//   - 必须有审核记录正面证明本节点通过；没有审核记录正面证明时绝不跳过准备同意，
//     落到普通同意门禁按实时待办裁决。
func (e *Executor) retrieveDoneRowProvesNodeHandled(ctx context.Context, session target.Session, instanceID, nodeID string, doneSnapshot target.TaskSnapshot) bool {
	if strings.TrimSpace(doneSnapshot.JobTaskID) == "" {
		return false
	}
	// 无论已办行上的节点字段如何，目标系统都可能将前驱已办行节点改写为流转后的当前节点。
	// 因此跳过准备同意必须以审核记录正面证明本节点已通过为必要条件，绝不凭单行已办轻率跳过。
	return e.retrieveNodeHasAuditPass(ctx, session, instanceID, nodeID)
}

// prepareActionWrite 按动作类型刷新任务身份，并在加签前读取完整代理文档；allowRefresh=false 用于会话恢复重发。
func (e *Executor) prepareActionWrite(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, request *target.ActionWriteRequest, session target.Session, allowRefresh bool, facts InstanceFacts) (target.Session, error) {
	status := "pending"
	if step.Action == model.ActionRetrieve {
		status = "done"
	}
	switch step.Action {
	case model.ActionStorageFormData, model.ActionReject, model.ActionTransfer, model.ActionAddSign, model.ActionRollback, model.ActionRetrieve:
	default:
		return session, nil
	}
	// 待办动作沿用门禁命中的实时任务节点；取回操作的是已办行，不能把当前 pending 节点套给它，
	// 仍以本步编译节点配合审核记录关联行补救来定位真实已办。
	nodeID := stepTargetNodeIDOf(runCtx, step)
	if status == "pending" {
		nodeID = pendingTaskNodeIDOf(runCtx, step, facts)
	}
	var snapshot target.TaskSnapshot
	var err error
	// facts 需要按地址传给补救逻辑以登记诊断原因；用可变副本，避免隐式改动调用方的值拷贝。
	localFacts := facts
	if allowRefresh {
		// 取回按已办（done）核对，视角由 done 的 executorId 语义承载，保持原读取；
		// 其余动作是待办刷新，必须带门禁阶段已核实的处理人视角（第三轮复核 P0-3）。
		if status == "pending" {
			session, snapshot, err = e.refreshPendingTaskSnapshotWithFallback(ctx, runCtx, step, nodeID, session, strings.TrimSpace(facts.CurrentTaskAssigneeID))
		} else {
			session, snapshot, err = e.refreshTaskSnapshot(ctx, runCtx, step, nodeID, session, status)
		}
	} else {
		snapshot, err = e.readTaskSnapshot(ctx, runCtx, step, nodeID, session, status)
	}
	if err != nil {
		return session, err
	}
	// 取回的已办行刷新同样受"目标改写节点"影响，必须与门禁同源补救。
	//
	// 为什么不能只按节点重读（2026-09-21 运行 94 路径 3 第 6 步实测）：门禁已按审核记录确认本节点
	// 确已办结并放行，但写前这一步又用本步节点等值过滤已办行；目标在本节点办结后会把该任务关联行的
	// flowNodeProxyID 改写成"实例流转到的下一节点"，于是这里必然读空，
	// 报「目标上已无当前处理人在本节点的已办任务，无法执行取回」——门禁放行、写前又拦，
	// 路径停在取回写前且重跑结果相同。
	//
	// 补救口径与门禁严格一致：只认审核记录里本节点最近一次「通过」的关联行 ID，
	// 用它去已办列表里取回同一行（同源可比，不按节点猜）。
	// 边界：审核记录里没有本节点通过记录、或按关联行 ID 找不到唯一一行时，保持读空并按既有纪律阻塞，
	// 绝不用其他节点的已办顶替，也绝不猜测任务号。
	if status == "done" && strings.TrimSpace(snapshot.JobTaskID) == "" {
		recovered, ok := e.retrieveDoneSnapshotByAuditLink(ctx, runCtx, session, step, nodeID, &localFacts)
		if ok {
			snapshot = recovered
		}
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
	// 加签语义对齐目标平台手动加签：在当前节点之后插入加签审批节点（整棵树随 updateFlowProxy
	// 发出），而不是把人员并入既有节点——后者不会产生新节点，目标「查看流程」看不到加签节点。
	// 姓名必须随写请求一起落进目标人员明细，否则目标只保存 ID：节点名称退化成通用「审核人」，
	// 运行画布上没有审批人可显示。姓名来自启动时用目标公司人员目录核实过的 ActionPersonNames。
	request.FlowProxyTree, treeErr = target.InsertAddSignNode(read.tree, request.NodeProxyID, request.UserIDs, runCtx.ActionPersonNames)
	if treeErr != nil {
		return session, treeErr
	}
	return session, nil
}

// refreshActionTask 在任务级动作发出前重读当前任务身份，并在会话明确失效时只重取一次。
// 任务号、批次和已办/待办状态都不能从编排配置或上一次写请求沿用。
func (e *Executor) refreshActionTask(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, request *target.ActionWriteRequest, session target.Session, facts InstanceFacts) (target.Session, error) {
	return e.prepareActionWrite(ctx, runCtx, step, request, session, true, facts)
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
	preSaveSent := preview.writeSent
	session = e.ensureBusinessPreSave(ctx, runCtx, step, session, preview, log, reportPhase, approved, attemptNo)
	if preview.writeErr != nil {
		return session
	}
	preSaveSent = preSaveSent || preview.writeSent
	switch request := preview.request.(type) {
	case *target.SubmitFlowInstanceRequest:
		started := e.now()
		result, response, traceID, err, attempted := e.runWriteWithNetworkRetry(ctx, step, attemptNo, log, reportPhase, approved,
			func(callContext context.Context, active target.Session) (*target.SubmitFlowInstanceResult, target.WriteResponse, string, error) {
				return e.target.SubmitFlowInstance(callContext, active, *request)
			}, session)
		preview.writeSent = preSaveSent || attempted
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
		session, err = e.prepareAuditTask(ctx, runCtx, step, request, session, true, preview.Facts)
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
		preview.writeSent = preSaveSent || attempted
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
					retrySession, prepareErr = e.prepareAuditTask(ctx, runCtx, step, request, refreshed, false, preview.Facts)
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
			session, taskErr = e.refreshActionTask(ctx, runCtx, step, actionRequest, session, preview.Facts)
			if taskErr != nil {
				preview.writeErr = taskErr
				preview.writeErrClass = model.FailureClassActorUnresolved
				return session
			}
			response, traceID, err, attempted := e.runActionWriteWithNetworkRetry(ctx, step, attemptNo, log, reportPhase, approved, session, *actionRequest)
			preview.writeSent = preSaveSent || attempted
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
						retrySession, prepareErr = e.prepareActionWrite(ctx, runCtx, step, actionRequest, refreshed, false, preview.Facts)
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
			if isRequestValidationError(err) && !preSaveSent {
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

// ensureBusinessPreSave 在主流程写请求前按目标页面顺序发出特殊业务/无表单前置保存。
// 业务保存失败不得继续主流程；成功后把返回 id 写入主流程关联。
// 已有同类型关联或本步已落账的 businessSaveId 时跳过，避免重启后重复保存。
func (e *Executor) ensureBusinessPreSave(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, session target.Session, preview *StepPreview, log *StepLog, reportPhase func(ApprovedStep, string, string), approved ApprovedStep, attemptNo int) target.Session {
	action := string(step.Action)
	noFormPreSave := target.NeedsNoFormFlowPreSave(runCtx.FlowType, action)
	if !target.NeedsSpecialBusinessPreSave(runCtx.FlowType, action) && !noFormPreSave {
		return session
	}
	otherBiz, restoredID := restoreBusinessSave(preview, runCtx, e.facts, step.Sequence)
	if restoredID != "" {
		attachBusinessRelevance(preview, otherBiz, restoredID)
		log.Phase("submit", step.Sequence, attemptNo, "复用已保存业务 id="+restoredID+"，不再重复前置保存")
		return session
	}
	writer, ok := e.target.(specialBusinessWriter)
	if !ok {
		preview.writeErr = fmt.Errorf("流程类型「%s」已登记专用业务保存，但当前目标客户端未实现前置写出口，不能发通用请求顶替", runCtx.FlowType)
		preview.writeErrClass = model.FailureClassToolBug
		return session
	}
	formData := formDataFromPreviewRequest(preview)
	reportPhase(approved, "submit", "正在按目标页面顺序保存业务数据，成功后再发主流程")
	var result target.SpecialBusinessSaveResult
	var err error
	var attempted bool
	if noFormPreSave {
		spec, found := target.LookupNoFormFlow(runCtx.FlowType)
		if !found {
			preview.writeErr = fmt.Errorf("无表单页面「%s」未登记专用业务链路，不能发通用 submit 顶替", runCtx.FlowType)
			preview.writeErrClass = model.FailureClassToolBug
			return session
		}
		otherBiz = target.NoFormFlowOtherBiz(spec.PageKey)
		result, err, attempted = e.runBusinessSaveWithNetworkRetry(ctx, step, attemptNo, log, reportPhase, approved,
			func(callContext context.Context, active target.Session) (target.SpecialBusinessSaveResult, error) {
				return writer.SaveNoFormFlow(callContext, active, spec, formData, action)
			}, session)
	} else {
		spec, found := target.LookupSpecialBusiness(runCtx.FlowType)
		if !found {
			return session
		}
		otherBiz = target.SpecialBusinessOtherBiz(spec.FlowType)
		result, err, attempted = e.runBusinessSaveWithNetworkRetry(ctx, step, attemptNo, log, reportPhase, approved,
			func(callContext context.Context, active target.Session) (target.SpecialBusinessSaveResult, error) {
				return writer.SaveSpecialBusiness(callContext, active, spec, formData, action)
			}, session)
	}
	if attempted {
		preview.writeSent = true
		preview.writeResponse = result.Response
		preview.writeTraceID = result.TraceID
	}
	if err != nil {
		preview.writeErr = err
		if isRequestValidationError(err) {
			preview.writeSent = false
			preview.writeErrClass = model.FailureClassToolBug
		}
		log.Phase("submit", step.Sequence, attemptNo, "业务前置保存失败，不得继续主流程："+err.Error())
		return session
	}
	if strings.TrimSpace(result.BusinessID) == "" {
		preview.writeErr = errors.New("业务保存成功但响应未返回业务 id，不能继续主流程")
		preview.writeErrClass = model.FailureClassToolBug
		preview.writeSent = attempted
		return session
	}
	attachBusinessRelevance(preview, otherBiz, result.BusinessID)
	preview.Facts.BusinessSaveID = result.BusinessID
	preview.Facts.BusinessSaveOtherBiz = otherBiz
	log.Phase("submit", step.Sequence, attemptNo, "业务前置保存成功，id="+result.BusinessID+"，即将发送主流程")
	return session
}

// runBusinessSaveWithNetworkRetry 为前置业务保存复用未写出连接重试边界。
// 请求一旦写出，响应丢失不得重发；主流程尚未发出。
func (e *Executor) runBusinessSaveWithNetworkRetry(ctx context.Context, step model.CompiledActionStep, attemptNo int, log *StepLog, reportPhase func(ApprovedStep, string, string), approved ApprovedStep, call func(context.Context, target.Session) (target.SpecialBusinessSaveResult, error), session target.Session) (target.SpecialBusinessSaveResult, error, bool) {
	result, response, traceID, err, attempted := RunWithWriteConnectRetry(ctx, e.policy, func(callContext context.Context) (target.SpecialBusinessSaveResult, target.WriteResponse, string, error) {
		saved, saveErr := call(callContext, session)
		return saved, saved.Response, saved.TraceID, saveErr
	}, func(attempt int, nextDelay time.Duration) {
		note := fmt.Sprintf("业务保存连接未建立，第 %d 次后将在 %s 重试", attempt, nextDelay)
		log.Phase("submit", step.Sequence, attemptNo, note)
		reportPhase(approved, "submit", note)
	})
	if result.TraceID == "" {
		result.TraceID = traceID
	}
	if result.Response.StatusCode == 0 {
		result.Response = response
	}
	return result, err, attempted
}

// restoreBusinessSave 从本步预览事实、运行现场或同一步已落账 JSON 恢复已成功的业务保存 id。
func restoreBusinessSave(preview *StepPreview, runCtx RunContext, store RunFactsStore, stepNo int) (otherBiz, businessID string) {
	if preview != nil {
		if id := strings.TrimSpace(preview.Facts.BusinessSaveID); id != "" {
			return firstNonEmpty(preview.Facts.BusinessSaveOtherBiz, target.SpecialBusinessOtherBiz(runCtx.FlowType)), id
		}
		if otherBiz, id := relevanceBusinessID(preview.Facts.BizRelevance, runCtx); id != "" {
			return otherBiz, id
		}
	}
	if id := strings.TrimSpace(runCtx.LastBeforeFacts.BusinessSaveID); id != "" {
		return firstNonEmpty(runCtx.LastBeforeFacts.BusinessSaveOtherBiz, target.SpecialBusinessOtherBiz(runCtx.FlowType)), id
	}
	if otherBiz, id := relevanceBusinessID(runCtx.LastBeforeFacts.BizRelevance, runCtx); id != "" {
		return otherBiz, id
	}
	return loadPersistedBusinessSave(store, runCtx, stepNo)
}

// relevanceBusinessID 从关联列表取出本流程类型对应的非空业务 id。
func relevanceBusinessID(values []target.BizRelevance, runCtx RunContext) (string, string) {
	keys := []string{
		target.SpecialBusinessOtherBiz(runCtx.FlowType),
		target.NoFormFlowOtherBiz(runCtx.FlowType),
		strings.TrimSpace(runCtx.FlowType),
	}
	for _, item := range values {
		otherBiz := strings.TrimSpace(item.OtherBiz)
		id := strings.TrimSpace(item.OtherBizID)
		if otherBiz == "" || id == "" {
			continue
		}
		for _, key := range keys {
			if key != "" && strings.EqualFold(otherBiz, key) {
				return otherBiz, id
			}
		}
	}
	return "", ""
}

// attachBusinessRelevance 把业务保存返回的 id 写入即将发出的主流程请求关联列表。
func attachBusinessRelevance(preview *StepPreview, otherBiz, businessID string) {
	if preview == nil {
		return
	}
	preview.Facts.BizRelevance = target.AppendBusinessRelevance(preview.Facts.BizRelevance, otherBiz, businessID)
	preview.Facts.BusinessSaveID = businessID
	preview.Facts.BusinessSaveOtherBiz = otherBiz
	switch request := preview.request.(type) {
	case *target.SubmitFlowInstanceRequest:
		request.BizRelevance = target.AppendBusinessRelevance(request.BizRelevance, otherBiz, businessID)
		preview.request = request
		preview.RequestPayload = target.BuildSubmitBody(*request)
	case *target.ActionWriteRequest:
		request.BizRelevance = target.AppendBusinessRelevance(request.BizRelevance, otherBiz, businessID)
		preview.request = request
		if body, _, err := target.BuildActionBody(*request); err == nil {
			preview.RequestPayload = body
		}
	}
}

// formDataFromPreviewRequest 取出本步即将发送的表单原文，供前置业务保存按目标页面构造载荷。
func formDataFromPreviewRequest(preview *StepPreview) json.RawMessage {
	if preview == nil {
		return nil
	}
	switch request := preview.request.(type) {
	case *target.SubmitFlowInstanceRequest:
		return request.FormData
	case *target.AuditCurrentTaskRequest:
		return request.FormData
	case *target.ActionWriteRequest:
		return request.FormData
	default:
		return nil
	}
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

// classifyAddSignReread 核对加签后的完整实例代理树是否真的插入了本次加签审批节点。
// updateFlowProxy 触发目标所有权复制后所有节点都会换新 ID，不能按写前节点 ID 回查；
// 必须读回目标树，确认恰好一个节点以 personnel 明细包含每个本次请求的人员，
// 零个说明未持久化，多个说明无法唯一证明，均按未变化/不可读处理。
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
	if proxyID == "" {
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
	updated, err := target.HasAddSignNode(document, request.UserIDs)
	if err != nil {
		return verdict.RereadUnreadable
	}
	if updated {
		return verdict.RereadAdvanced
	}
	return verdict.RereadUnchanged
}

// hasAuditRecordForTaskLink 判断实例审核记录里是否存在「指定任务关联行 + 指定 auditStatus」的记录。
// 审核记录与写动作在目标同一事务落库、节点字段不被改写，是抗「待办快照滞后」的原子正面证据
// （2026-09-24 多动作核验审计:移交/取回等只凭待办快照会在读写时序窗口内把成功误判为暂时无法确认）。
// linkID 或 instanceID 为空、目标不支持审核记录读取、或读取失败时一律返回 false——只作正面补强，
// 绝不因读不到就把未生效判成已生效。
func (e *Executor) hasAuditRecordForTaskLink(ctx context.Context, account string, session target.Session, instanceID, linkID, auditStatus string) bool {
	linkID = strings.TrimSpace(linkID)
	instanceID = strings.TrimSpace(instanceID)
	if linkID == "" || instanceID == "" {
		return false
	}
	reader, ok := e.target.(auditRecordsReader)
	if !ok {
		return false
	}
	records, _, err := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, account, session,
		func(callContext context.Context, active target.Session) ([]target.AuditRecordSnapshot, error) {
			return reader.ListAuditRecords(callContext, active, instanceID)
		})
	if err != nil {
		return false
	}
	for _, record := range records {
		if strings.TrimSpace(record.FlowJobTaskID) == linkID && strings.EqualFold(strings.TrimSpace(record.AuditStatus), auditStatus) {
			return true
		}
	}
	return false
}

// classifyTransferReread 核对移交后当前处理人的任务是否已经换成新任务。
// 移交不推进节点，单看实例节点会把成功误判为"未变化"；任务 ID 变化或当前处理人不再有待办才算前进。
func (e *Executor) classifyTransferReread(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, session target.Session, preview *StepPreview, after InstanceFacts) verdict.Reread {
	if after.ReadError != "" {
		return verdict.RereadUnreadable
	}
	request, ok := preview.request.(*target.ActionWriteRequest)
	if !ok || strings.TrimSpace(request.JobTaskID) == "" {
		return verdict.RereadUnreadable
	}
	// 移交不推进节点，写后必须重读写请求实际绑定的任务节点；动态代理重建后编译节点已经失效。
	nodeID := firstNonEmpty(strings.TrimSpace(request.NodeProxyID), pendingTaskNodeIDOf(runCtx, step, preview.Facts))
	_, snapshot, err := e.refreshTaskSnapshot(ctx, runCtx, step, nodeID, session, "pending")
	if err != nil {
		return verdict.RereadUnreadable
	}
	if snapshot.JobTaskID == "" || strings.TrimSpace(snapshot.JobTaskID) != strings.TrimSpace(request.JobTaskID) {
		return verdict.RereadAdvanced
	}
	// 兜底正面证据：移交在目标同事务写入一条 transfer 审核记录(原任务关联行)，节点不推进、原任务办结。
	// 原处理人 pending 快照晚一拍仍返回本人待办时，用它确认移交已生效，避免与「会签末签」同型的误判。
	account := strings.TrimSpace(session.Summary.Account)
	if account == "" {
		account = runCtx.PlanAccount
	}
	if e.hasAuditRecordForTaskLink(ctx, account, session, strings.TrimSpace(runCtx.PathRun.MainInstanceRef), preview.Facts.CurrentTaskLinkID, "transfer") {
		return verdict.RereadAdvanced
	}
	return verdict.RereadUnchanged
}

// classifyRetrieveReread 核对取回后目标是否在原审批节点创建了新的待办。
// 取回会保留原节点位置，因此实例节点和待办节点都可能不变；目标以新的 jobTaskId 表示新批次，
// 必须对照写入前的已办任务号，不能把同节点的新待办误判为"没有变化"。
func (e *Executor) classifyRetrieveReread(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, session target.Session, preview *StepPreview, after InstanceFacts) verdict.Reread {
	if after.ReadError != "" {
		return verdict.RereadUnreadable
	}
	request, ok := preview.request.(*target.ActionWriteRequest)
	if !ok || strings.TrimSpace(request.JobTaskID) == "" {
		return verdict.RereadUnreadable
	}
	// 取回后应在原已办节点生成新待办；这里使用写前已办事实，不能改用实例此刻的其他 pending 节点。
	nodeID := firstNonEmpty(strings.TrimSpace(preview.Facts.CompletedTaskNodeID), stepTargetNodeIDOf(runCtx, step))
	_, snapshot, err := e.refreshTaskSnapshot(ctx, runCtx, step, nodeID, session, "pending")
	if err != nil {
		return verdict.RereadUnreadable
	}
	if snapshot.JobTaskID != "" && strings.TrimSpace(snapshot.JobTaskID) != strings.TrimSpace(request.JobTaskID) {
		return verdict.RereadAdvanced
	}
	// 兜底正面证据：取回在目标同事务写入一条 retrieve 审核记录(取回的已办关联行)。取回门禁已确保
	// 本已办行此前不存在 retrieve 记录(RetrieveAlreadyUsed 命中即阻塞)，故写后出现即本次取回所致；
	// 新 pending 快照晚一拍不可见时用它确认取回已生效。
	account := strings.TrimSpace(session.Summary.Account)
	if account == "" {
		account = runCtx.PlanAccount
	}
	if e.hasAuditRecordForTaskLink(ctx, account, session, strings.TrimSpace(runCtx.PathRun.MainInstanceRef), preview.Facts.CompletedTaskLinkID, "retrieve") {
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

// classifyForwardReread 判定转发写请求是否已在目标生效。
//
// 转发的语义是"把本人当前待办里的这条任务转给他人"，它**不推进流程节点**：
// 写成功后节点位置与"本人待办"都可能保持原样，因此不能用"节点是否前进"或
// "待办是否消失"作判据——那会把一次成功的转发判成"明确未变"，进而升级为
// "执行结果暂时无法确认"并让整条运行终局停止（2026-09-21 运行 86/87 路径 1 第 43 步实测）。
//
// 判据来源必须选 runtime 真正回传的事实：写发生在执行服务的浏览器里，
// 引擎侧 preview.writeResponse 只承载状态码与成败，**没有响应体**（见 submit 阶段
// 构造 WriteResponse 的调用点），因此按"响应 data.id"判定必然恒为空、恒判无法确认。
// 这里改用两条真实可得的事实：
//  1. runtime 回传的辅助实例标识（转发在目标侧派生关联实例时非空）：
//     以"已发"列表可见性核对，可见即视为已生效；
//  2. 转发写请求本身的成败：执行服务已把浏览器里的写响应收敛为 success，
//     且写请求确实发出（writeSent），即"目标已接受本次转发"。
//
// 两者都拿不到时返回 RereadUnreadable，绝不猜测成功——写结果不确定的保守纪律不变。
//
// 第二个返回值是辅助实例引用：转发可能在目标侧派生一条新的关联实例，
// 控制层据此重绑定代理树；读不到时为空串。
func (e *Executor) classifyForwardReread(ctx context.Context, account string, session target.Session, preview *StepPreview) (verdict.Reread, string) {
	// 写没发出或明确失败：如实按未生效处理，不做任何"可能成功了"的推断。
	if !preview.writeSent {
		e.recordForwardRereadReason(preview, "转发写请求未发出，无从核对结果")
		return verdict.RereadUnchanged, ""
	}
	if preview.writeErr != nil {
		e.recordForwardRereadReason(preview, "转发写请求未成功："+preview.writeErr.Error())
		return verdict.RereadUnchanged, ""
	}
	// 辅助实例：只有响应体里带的关联实例才需要再做可见性核对；
	// 取不到时不影响主判定——转发不产生新主实例是常态。
	auxiliary := forwardAuxiliaryInstanceID(preview.writeResponse.Data)
	if auxiliary == "" {
		e.recordForwardRereadReason(preview, "转发写请求已被目标接受（响应 isSuccess=true），未派生关联实例")
		return verdict.RereadAdvanced, ""
	}
	reader, ok := e.target.(interface {
		FindSubmittedFlow(context.Context, target.Session, string) (string, []string, string, []string, bool, error)
	})
	if !ok {
		e.recordForwardRereadReason(preview, "转发已成功但目标客户端不支持已发列表读取，无法核对关联实例 "+auxiliary)
		return verdict.RereadAdvanced, auxiliary
	}
	found, _, err := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, account, session,
		func(callContext context.Context, active target.Session) (bool, error) {
			_, _, _, _, hit, readErr := reader.FindSubmittedFlow(callContext, active, auxiliary)
			return hit, readErr
		})
	if err != nil {
		// 读不到只影响"关联实例可见性"这条附加证据，不影响已经成立的写成功事实。
		e.recordForwardRereadReason(preview, "转发写请求已被目标接受；关联实例 "+auxiliary+" 可见性核对读取失败："+err.Error())
		return verdict.RereadAdvanced, auxiliary
	}
	if !found {
		e.recordForwardRereadReason(preview, "转发写请求已被目标接受；关联实例 "+auxiliary+" 暂未出现在已发列表")
		return verdict.RereadAdvanced, auxiliary
	}
	e.recordForwardRereadReason(preview, "转发写请求已被目标接受，关联实例 "+auxiliary+" 已在已发列表可见")
	return verdict.RereadAdvanced, auxiliary
}

// forwardAuxiliaryInstanceID 从转发响应体中取关联实例标识；没有响应体或没有该字段时返回空串。
//
// 为什么单独成函数：响应体是否携带关联实例取决于目标版本，取值必须与"判定成功"解耦——
// 取不到关联实例只意味着不需要做附加核对，不意味着转发没生效。
// 输入边界：raw 为空、为 null、非法 JSON 或缺 id 字段一律返回空串，绝不猜测。
func forwardAuxiliaryInstanceID(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var parsed struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.ID)
}

// recordForwardRereadReason 把转发结果核对的具体依据写入预览事实诊断。
//
// 为什么需要：转发不推进节点，"结果无法确认"这条结论必须带上它读到了什么、
// 缺了什么，否则界面与日志都只剩一句无法复核的结论。只写非敏感事实，不含 SID 与表单正文。
func (e *Executor) recordForwardRereadReason(preview *StepPreview, reason string) {
	if preview == nil {
		return
	}
	preview.Facts.AssigneeDiag = firstNonEmpty(reason, preview.Facts.AssigneeDiag)
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
// 每次重试都如实写进 step.log，不允许出现"看起来只调了一次"的日志。
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

// forwardPathTargetNodeIDs 汇总本步之后路径上所有合法后续业务节点的目标标识，供写后核验判断实例是否已经推进。
// 取值口径与门禁放行的分支持久化一致，并覆盖会签末签等「跨节点前进」形态：
//   - 运行期分支选择优先纳入：跨手动分支路由的同意，其下一跳写在 runCtx.SubmitBranchTargetNodeID；
//     它也是编译末步唯一可用的锚点（路径声明步数可能大于实际编译步数，末步没有"下一步"可取）。
//   - 再纳入编译步骤表里本步之后每一个用户步骤与固定尾动作的节点：目标可能穿透空节点或无处理人
//     节点直接前进到更靠后的已配置节点，因此"已推进"不能只认紧邻下一跳，必须覆盖全部后续合法节点
//     （2026-09-24 会签末签实测：末签同意后实例直接推进到后续节点，只认紧邻下一跳会漏判成"暂时无法确认"）。
//   - 普通步骤不能把后面的恢复链当作正向锚点；但当前步骤本身是恢复链同意时，同一触发动作下的
//     后续恢复同意就是目标应到达的合法节点，必须纳入。运行 8 路径 9 正是因一律排除恢复步骤，
//     已推进到下一恢复节点仍被旧待办快照误判为“暂时无法确认”。系统导航仍一律排除。
//
// 取不到任何锚点时返回空切片，调用方据此放弃这条判据，绝不猜测下一跳。
func forwardPathTargetNodeIDs(runCtx RunContext, nextIndex int) []string {
	ids := make([]string, 0, 4)
	seen := map[string]bool{}
	// add 去重登记一个非空目标节点标识。
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		ids = append(ids, id)
	}
	add(runCtx.SubmitBranchTargetNodeID)
	var current model.CompiledActionStep
	if nextIndex >= 0 && nextIndex < len(runCtx.Steps) {
		current = runCtx.Steps[nextIndex]
	}
	for idx := nextIndex + 1; idx < len(runCtx.Steps); idx++ {
		st := runCtx.Steps[idx]
		if st.Source == model.ActionStepSourceNavigation {
			continue
		}
		if st.Source == model.ActionStepSourceRecovery {
			// 只有同一恢复触发动作下的“恢复同意 -> 后续恢复同意”可作为推进锚点；
			// 普通步骤、不同恢复组或重新提交步骤仍不能借恢复链证明自己成功。
			if current.Source != model.ActionStepSourceRecovery ||
				current.Action != model.ActionApprove || st.Action != model.ActionApprove ||
				strings.TrimSpace(current.SourceActionKey) == "" ||
				strings.TrimSpace(st.SourceActionKey) != strings.TrimSpace(current.SourceActionKey) {
				continue
			}
		}
		if info, ok := runCtx.Nodes[strings.TrimSpace(st.NodeKey)]; ok {
			add(info.TargetNodeID)
		}
	}
	return ids
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
// 解析出的姓名优先使用人员目录真实中文姓名，姓名为空时使用登录账号兜底，绝不把 32 位用户 ID 充当姓名展示。
// 目录读取沿用读路径会话纪律（readOnlyWithSessionRetry）：目标会话可能在门禁读取与
// 账号解析之间被作废（实测返回 RESP401「SID已失效!」），失效只允许重登并重放同一只读请求
// （有界、重登间退避防触发目标会话限制），不能把会话失效直接放大成处理人解析失败。
func (e *Executor) assigneeAccount(ctx context.Context, session target.Session, assigneeUserID string) (string, string, error) {
	resolver, ok := e.target.(userAccountResolver)
	if !ok {
		return "", "", errors.New("目标客户端不支持人员目录账号解析")
	}
	accounts, _, err := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, session.Summary.Account, session,
		func(callContext context.Context, active target.Session) (map[string]target.UserAccount, error) {
			return resolver.UserAccountsByID(callContext, active, []string{assigneeUserID})
		})
	if err != nil {
		return "", "", err
	}
	info, ok := accounts[assigneeUserID]
	account := strings.TrimSpace(info.Account)
	if !ok || account == "" {
		return "", "", errors.New("人员目录中没有该用户的登录账号")
	}
	name := strings.TrimSpace(info.Name)
	if name == "" {
		name = account
	}
	return account, name, nil
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
	nodeName := strings.TrimSpace(runCtx.Nodes[step.NodeKey].Name)
	handler, ok := currentHandlerForNode(facts, nodeID, nodeName)
	if !ok {
		instanceID := strings.TrimSpace(runCtx.PathRun.MainInstanceRef)
		if len(facts.CurrentHandlers) == 0 {
			// 目标响应整段缺少 currentAuditUserInfo：把缺失字段与实例 ID 一起说清楚，
			// 由门禁停在本步（F-031/T04），绝不用配置候选人顶替当前处理人。
			return target.TaskSnapshot{}, "", "", sess, false, fmt.Sprintf("目标响应缺少 currentAuditUserInfo（实例 %s），无法确定本节点当前处理人", instanceID)
		}
		return target.TaskSnapshot{}, "", "", sess, false, fmt.Sprintf("实例 %s 的 currentAuditUserInfo 里没有本节点的处理人信息", instanceID)
	}
	// 处理人事实命中后，后续待办查询必须使用目标本次返回的真实节点 ID。
	// 名称兜底只负责把旧编译节点映射到唯一同名动态节点，不允许继续拿旧 ID 过滤任务。
	taskNodeID := firstNonEmpty(strings.TrimSpace(handler.NodeID), strings.TrimSpace(nodeID))
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
	// 竞签/多处理人节点：优先以「可用处理人」的身份审批，跳过不可用的人（停用/无平台权限/被排除——
	// 这些人登录必然失败）。任一可用处理人推进即可，绝不因为先命中一个不可用的人就整条路径停机。
	// 全部不可用时保留原顺序继续尝试并在末尾报告：让「谁都无法登录」的真实原因如实暴露。
	matches, excludedActorNames := partitionUsableHandlers(matches, runCtx.ExcludedActorIDs)
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
			wantNode := taskNodeID
			for _, snapshot := range snapshots {
				if wantNode != "" && strings.TrimSpace(snapshot.FlowNodeProxyID) != wantNode {
					continue
				}
				// 命中：尝试登录该处理人会话；若当前处理人无法登录（例如停用、无权限等），
				// 绝不能直接认作有效处理人放行——抢签/多处理人节点必须继续尝试下一个候选人，
				// 避免单人凭据不可用阻断整个节点推进（2026-09-24 路径 1 审核人 5 抢签节点赵飞无权限实测）。
				account := strings.TrimSpace(match.Account)
				if account != "" {
					var release func()
					if locker, ok := e.sessions.(interface{ LockAccountUsage(string) func() }); ok &&
						!strings.EqualFold(account, strings.TrimSpace(runCtx.PlanAccount)) {
						release = locker.LockAccountUsage(account)
					}
					actorSession, sessionErr := e.sessions.Current(ctx, account)
					if sessionErr != nil && sessionRefreshAllowed(sessionErr) {
						actorSession, sessionErr = e.sessions.Refresh(ctx, account)
					}
					if release != nil {
						release()
					}
					if sessionErr == nil {
						snapshot.PendingUserID = match.UserID
						name := firstNonEmpty(match.Name, actorSession.Summary.DisplayName, session.Summary.DisplayName)
						snapshot.PendingUserName = name
						return snapshot, match.UserID, name, actorSession, true, ""
					}
					log := e.stepLogFor(runCtx)
					log.Phase("gate", step.Sequence, 1, fmt.Sprintf("候选处理人 %s（账号 %s）会话登录失败（%s），尝试下一位候选处理人", firstNonEmpty(match.Name, account), account, userFacingError(sessionErr, target.WriteResponse{})))
					// 账号明确登录失败（无权限、密码错误等），放弃当前候选人的当前快照，继续扫描下一个快照或下一位候选人
					break
				}
				// 账号为空表示沿用当前会话（不额外登录，如单人节点或计划账号本人），直接返回当前会话
				snapshot.PendingUserID = match.UserID
				name := firstNonEmpty(match.Name, session.Summary.DisplayName)
				snapshot.PendingUserName = name
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
			// 这里按"计划 -> 候选"顺序获取，避免多账号执行时锁顺序反转。
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
			snapshot, err = e.readTaskSnapshot(ctx, runCtx, step, taskNodeID, actorSession, status)
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
		if len(excludedActorNames) > 0 {
			// 本节点的处理人全部在排除/不可用名单里（停用、无当前平台权限或被手动排除），
			// 登录必然失败：如实说清是谁，指引去目标平台调整该节点处理人或授权，不冒用他人身份。
			return target.TaskSnapshot{}, "", "", sess, false,
				"本节点处理人均无法使用（" + strings.Join(excludedActorNames, "、") + "）：账号已停用、无当前平台权限或已被排除，无法登录；请在目标平台调整该节点处理人或为账号授权"
		}
		return target.TaskSnapshot{}, "", "", sess, false, "当前处理人未能解析出登录账号"
	}
	return target.TaskSnapshot{}, "", "", sess, false, "已依次核对当前处理人（" + strings.Join(tried, "、") + "）的待办，均未发现本节点待办"
}

// partitionUsableHandlers 把处理人候选分成「可用」和「不可用（在排除/停用/无权限集合里）」两部分，
// 返回时可用的排在前面，让竞签/多处理人节点优先以可用处理人身份审批。
// 若全部可用或 excluded 为空，原样返回、排除名单为空。返回的第二个值是被跳到后面的不可用处理人姓名，
// 供「全部不可用」时的诊断说明。
func partitionUsableHandlers(matches []target.HandlerAccount, excluded map[string]bool) ([]target.HandlerAccount, []string) {
	if len(excluded) == 0 || len(matches) == 0 {
		return matches, nil
	}
	usable := make([]target.HandlerAccount, 0, len(matches))
	blocked := make([]target.HandlerAccount, 0, len(matches))
	blockedNames := make([]string, 0, len(matches))
	for _, match := range matches {
		if excluded[strings.TrimSpace(match.UserID)] {
			blocked = append(blocked, match)
			blockedNames = append(blockedNames, firstNonEmpty(match.Name, match.Account, match.UserID))
			continue
		}
		usable = append(usable, match)
	}
	if len(usable) == 0 {
		// 全部不可用：保持原候选顺序继续尝试（沿用既有登录失败路径），并带回姓名供诊断。
		return matches, blockedNames
	}
	// 有可用处理人：只用可用的，被排除的完全不参与（竞签任一可用人推进即可）。
	_ = blocked
	return usable, nil
}

// currentHandlerForNode 从实例事实里取本节点的待办处理人信息。
// 先按目标节点 ID 精确匹配；实例私有代理重建导致旧 ID 失效时，只允许唯一同名节点兜底。
// 同名命中不唯一时返回失败，绝不猜测使用哪一个处理人。
func currentHandlerForNode(facts InstanceFacts, nodeID, nodeName string) (target.NodeCurrentHandler, bool) {
	nodeID = strings.TrimSpace(nodeID)
	for _, handler := range facts.CurrentHandlers {
		if strings.TrimSpace(handler.NodeID) == nodeID {
			return handler, true
		}
	}
	nodeName = strings.TrimSpace(nodeName)
	if nodeName == "" {
		return target.NodeCurrentHandler{}, false
	}
	var matched target.NodeCurrentHandler
	count := 0
	for _, handler := range facts.CurrentHandlers {
		if strings.TrimSpace(handler.NodeName) != nodeName {
			continue
		}
		matched = handler
		count++
	}
	if count == 1 {
		return matched, true
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
	UserAccountsByID(ctx context.Context, active target.Session, ids []string) (map[string]target.UserAccount, error)
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

// recoveryTriggerWasSkipped 判断恢复链触发动作是否已经被目标跳过。
// 先读当前运行内存事实，再读最近一次已落账尝试，确保进程重启后不会重新发送无前置副作用的重提或恢复审批。
func recoveryTriggerWasSkipped(store RunFactsStore, runCtx RunContext, step model.CompiledActionStep) bool {
	triggerKey := strings.TrimSpace(step.SourceActionKey)
	if triggerKey == "" || step.Source != model.ActionStepSourceRecovery {
		return false
	}
	if runCtx.RecoveryTriggerOutcomes[triggerKey] == "target_skipped" {
		return true
	}
	reader, ok := store.(persistedAttemptReader)
	if !ok || runCtx.PathRun.ID == 0 {
		return false
	}
	previous, attempt, err := reader.LatestStepAttempt(context.Background(), runCtx.PathRun.ID)
	if err != nil || previous.StepNo >= step.Sequence {
		return false
	}
	if attempt.Verdict == "recovery_not_applicable" && previous.Source == string(model.ActionStepSourceRecovery) {
		return true
	}
	if attempt.Verdict != StepVerdictTargetSkipped {
		return false
	}
	switch model.ActionKey(previous.Action) {
	case model.ActionReject, model.ActionWithdraw, model.ActionRollback:
		return true
	default:
		return false
	}
}

// retrieveFollowsTargetSkippedPreparation 判断取回是否紧跟一个已被目标自动跳过的准备同意。
// 只有直接前置步骤、同一节点且已落账原因为“目标自动跳过”时才成立；已有真实已办而跳过准备同意
// 的情况必须继续走正常取回门禁，不能被误判成目标没有生成已办任务。
func retrieveFollowsTargetSkippedPreparation(store RunFactsStore, runCtx RunContext, nextIndex int) bool {
	if nextIndex <= 0 || nextIndex >= len(runCtx.Steps) || runCtx.PathRun.ID == 0 {
		return false
	}
	current := runCtx.Steps[nextIndex]
	previous := runCtx.Steps[nextIndex-1]
	if current.Action != model.ActionRetrieve || !previous.RetrievePrepareConsent ||
		previous.Action != model.ActionApprove || previous.NodeKey != current.NodeKey ||
		previous.Source != model.ActionStepSourceRecovery {
		return false
	}
	reader, ok := store.(persistedAttemptReader)
	if !ok {
		return false
	}
	stepRow, attempt, err := reader.LatestStepAttempt(context.Background(), runCtx.PathRun.ID)
	if err != nil || stepRow.StepNo != previous.Sequence || stepRow.StepNo >= current.Sequence {
		return false
	}
	if attempt.Verdict != StepVerdictTargetSkipped {
		return false
	}
	return strings.Contains(strings.TrimSpace(attempt.Reason), "目标自动跳过")
}

// isSkip=true 且越过本节点 → 跳过；isSkip 未声明/false 且越过 → 阻塞；run_node_choose 越过 → 阻塞。
func TargetSkippedClassificationForTest(runCtx RunContext, step model.CompiledActionStep, facts InstanceFacts) (string, string) {
	action, reason, _, _ := targetSkippedStepReason(runCtx, step, facts)
	return action, reason
}

// IsRecoveryEffectStepForTest 暴露恢复型动作判定，供 test 目录锁定 2026-09-21 取回阻塞缺陷：
// 取回/回退/撤回的成立前提就是实例待办已越过目标节点，绝不能套用推进型的目标跳过判定。
func IsRecoveryEffectStepForTest(step model.CompiledActionStep) bool {
	return isRecoveryEffectStep(step)
}

// ScopeActsOnCurrentTodo 判断该动作作用域是否"作用于本人当前的待办任务"。
//
// 为什么单独成函数：这类动作在 prepare 阶段必须把执行者身份切换到目标裁决的当前处理人，
// 因为原生待办列表按登录人过滤——用计划账号发起会看不到属于他人的待办行。
// 覆盖三种作用域：
//   - task / completed_task：本节点本人待办（审批、暂存、驳回、取回等）；
//   - instance：实例的当前活动待办（转发等），配置里没有节点键，但同样是"本人的待办"。
//
// 为什么不用动作名白名单：白名单每加一个同类动作就要同步改一处，漏改就退化成
// "用计划账号冒充处理人"；按作用域判定让新增动作自动继承正确口径。
//
// 输入边界：initiator 作用域（发起人身份动作）不属于本人待办，返回 false。
func ScopeActsOnCurrentTodo(scope model.ActionScope) bool {
	switch scope {
	case model.ActionScopeTask, model.ActionScopeCompletedTask, model.ActionScopeInstance:
		return true
	default:
		return false
	}
}

// ForwardTabForAction 返回转发动作必须装载的原生列表页标签。
//
// 恒为 backlog（待办）：原生列表页的「转发」入口只在非 dueout/timedout 的标签页渲染，
// 且页面的 transpond() 委托给当前激活标签页组件，dueout 页没有该入口；
// 而"可转发的任务"本身只出现在待办列表里。按实例来源改选标签页会让装载等不到可转发行
// （2026-09-21 运行 80 路径 1 第 43 步实测装载超时）。因此不提供按来源分支的口径。
//
// 输入边界：无入参；返回值为执行服务协议直接消费的标签键，不得改名。
func ForwardTabForAction() string {
	return "backlog"
}

// ForwardAuxiliaryInstanceIDForTest 暴露转发响应体的关联实例取值，供 test 目录锁定：
// 取不到关联实例时返回空串（不猜测），且非法 JSON 不得导致误判。
func ForwardAuxiliaryInstanceIDForTest(raw json.RawMessage) string {
	return forwardAuxiliaryInstanceID(raw)
}

// RefreshPendingTaskForAssigneeForTest 暴露提交阶段的待办刷新入口，供 test 目录锁定 T2：
// 会话本人视角优先、读空时按已核实的处理人视角补查，且补查不改变既有任务身份。
func (e *Executor) RefreshPendingTaskForAssigneeForTest(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, nodeID string, session target.Session, assigneeUserID string) (target.Session, target.TaskSnapshot, error) {
	return e.refreshPendingTaskSnapshotForAssignee(ctx, runCtx, step, nodeID, session, assigneeUserID)
}

// ForwardPathTargetNodeIDsForTest 暴露「本步之后合法路径节点集合」取值口径，供 test 目录锁定：
// 运行期分支选择与编译步骤表里全部后续用户步骤都纳入，恢复链与系统导航排除，末步靠分支选择仍能取到锚点。
func ForwardPathTargetNodeIDsForTest(runCtx RunContext, nextIndex int) []string {
	return forwardPathTargetNodeIDs(runCtx, nextIndex)
}
