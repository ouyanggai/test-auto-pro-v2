package actioncatalog

import (
	"strings"

	"test-auto-pro-v2/internal/model"
)

// Service 是无状态的动作目录门禁服务；每次构建都必须使用最新目标上下文。
type Service struct{}

// Builder 是动作目录只读计算边界，便于路径配置服务注入而不携带目标写客户端。
type Builder interface {
	Build(model.ActionContext) []model.ActionCatalogItem
}

type actionDefinition struct {
	action             model.ActionKey
	category           model.ActionCategory
	scope              model.ActionScope
	label              string
	description        string
	targetOperation    string
	parameters         []model.ActionParameter
	expectedEffect     string
	reloadRequirements []string
	systemOnly         bool
}

type gateResult struct {
	enabled       bool
	reason        string
	preconditions []model.ActionPrecondition
}

// actionDefinitions 固定目录顺序与目标页面的业务分组顺序，避免浏览器按 map 顺序显示动作。
var actionDefinitions = []actionDefinition{
	{
		action: model.ActionSaveDraft, category: model.ActionCategoryLifecycle, scope: model.ActionScopeInitiator,
		label: "保存草稿", description: "发起端保存或保持草稿实例，不创建待办，也不推进流程节点。",
		targetOperation: "/web/flowInstanceApi/submit",
		parameters: []model.ActionParameter{
			{Name: "id", Required: false, Description: "已有草稿实例的目标实例键，由服务端运行时解析"},
			{Name: "formProxyId", Required: true, Description: "目标流程代理引用"},
			{Name: "status", Value: "draft", Required: true, Description: "固定保存为草稿状态"},
			{Name: "name", Required: true, Description: "实例标题"},
			{Name: "flowInstanceBizRelevanceList", Required: false, Description: "目标业务关联列表"},
			{Name: "formDataMongoVo.data", Required: true, Description: "form-runtime 捕获的目标原始表单数据"},
		},
		expectedEffect:     "目标实例保持 draft；不生成待办、不改变当前审批节点。",
		reloadRequirements: []string{"实例状态", "实例原始表单数据"},
	},
	{
		action: model.ActionSubmit, category: model.ActionCategoryLifecycle, scope: model.ActionScopeInitiator,
		label: "提交", description: "发起端提交新实例，按目标引擎重新解析路径并创建首个待办。已有草稿走重新提交。",
		targetOperation: "/web/flowInstanceApi/submit",
		parameters: []model.ActionParameter{
			{Name: "id", Required: false, Description: "草稿实例键，由服务端绑定"},
			{Name: "flowProxyId", Required: true, Description: "目标流程代理引用"},
			{Name: "formDataMongoVo.data", Required: true, Description: "form-runtime 捕获的目标原始表单数据"},
			{Name: "nextAuditorList", Required: false, Description: "目标引擎解析或用户明确选择的下一处理人"},
			{Name: "auditRecord", Required: false, Description: "目标提交审计上下文"},
		},
		expectedEffect:     "实例进入 run；目标引擎沿真实条件、并行和人员规则创建首个待办。",
		reloadRequirements: []string{"实例状态", "流程代理", "当前节点", "首个待办", "实际路径"},
	},
	{
		action: model.ActionResubmit, category: model.ActionCategoryLifecycle, scope: model.ActionScopeInitiator,
		label: "重新提交", description: "发起人从目标允许的草稿、驳回或撤回状态重新发起。",
		targetOperation: "/web/flowInstanceApi/reSubmit",
		parameters: []model.ActionParameter{
			{Name: "id", Required: true, Description: "目标已有实例键，由服务端绑定"},
			{Name: "formDataMongoVo.data", Required: true, Description: "form-runtime 捕获的目标原始表单数据"},
			{Name: "nextAuditorList", Required: false, Description: "目标引擎重新解析的下一处理人"},
			{Name: "auditRecord", Required: false, Description: "重新提交审计上下文"},
		},
		expectedEffect:     "实例离开 draft、rejected 或 withdraw，重新进入 run 并按当前数据解析路径。",
		reloadRequirements: []string{"实例状态", "流程代理", "当前节点", "待办", "实际路径"},
	},
	{
		action: model.ActionStorageFormData, category: model.ActionCategoryCurrentTodo, scope: model.ActionScopeTask,
		label: "暂存当前表单", description: "审批端保存当前用户、批次和节点的表单检查点；不推进任务或实例。",
		targetOperation: "/web/flowInstanceApi/storageFormData",
		parameters: []model.ActionParameter{
			{Name: "id", Required: true, Description: "目标流程实例键，由当前待办绑定"},
			{Name: "currentNodeProxyId", Required: true, Description: "当前节点代理键，由当前待办绑定"},
			{Name: "auditRecord.executeDesc", Required: false, Description: "审批暂存说明"},
			{Name: "formDataMongoVo.data", Required: true, Description: "form-runtime 捕获的目标原始表单数据"},
		},
		expectedEffect:     "仅更新当前用户/批次/节点检查点；实例状态、任务状态和当前节点保持不变。",
		reloadRequirements: []string{"当前待办", "当前节点检查点", "当前表单数据"},
	},
	{
		action: model.ActionAddSign, category: model.ActionCategoryCurrentTodo, scope: model.ActionScopeTask,
		label: "加签", description: "在当前活动人工待办追加受限处理人，必要时先分离实例私有代理。",
		targetOperation: "/web/flowInstanceApi/approverAppend",
		parameters: []model.ActionParameter{
			{Name: "id", Required: true, Description: "目标流程实例键"},
			{Name: "jobTaskId", Required: true, Description: "当前待办键"},
			{Name: "batchNo", Required: true, Description: "当前实例批次"},
			{Name: "flowNodeProxyId", Required: true, Description: "当前节点代理键"},
			{Name: "approverAppendVo.flowNodeProxyId", Required: true, Description: "追加人员所属节点代理键"},
			{Name: "approverAppendVo.userIds", Required: true, Description: "实时受限候选中的人员键集合"},
		},
		expectedEffect:     "通过 updateFlowProxy 必要时创建实例私有代理并追加审批人；后续任务按新代理继续。",
		reloadRequirements: []string{"实例私有流程代理", "当前节点", "当前待办与演员", "代理任务映射"},
	},
	{
		action: model.ActionTransfer, category: model.ActionCategoryCurrentTodo, scope: model.ActionScopeTask,
		label: "移交", description: "把当前活动待办移交给实时受限候选演员，不推进流程节点。",
		targetOperation: "/web/flowInstanceApi/approverAppend",
		parameters: []model.ActionParameter{
			{Name: "id", Required: true, Description: "目标流程实例键"},
			{Name: "jobTaskId", Required: true, Description: "当前待办键"},
			{Name: "batchNo", Required: true, Description: "当前实例批次"},
			{Name: "auditRecord.auditStatus", Value: "transfer", Required: true, Description: "固定移交结果"},
			{Name: "auditRecord.executeDesc", Required: false, Description: "移交说明"},
			{Name: "approverAppendVo.flowNodeProxyId", Required: true, Description: "当前节点代理键"},
			{Name: "approverAppendVo.userIds", Required: true, Description: "实时受限候选中的新演员键集合"},
		},
		expectedEffect:     "当前待办演员切换为目标候选；节点位置不变，后续步骤必须重读任务。",
		reloadRequirements: []string{"当前待办", "当前演员", "节点代理"},
	},
	{
		action: model.ActionApprove, category: model.ActionCategoryCurrentTodo, scope: model.ActionScopeTask,
		label: "同意", description: "处理当前活动人工待办并按目标引擎推进下一节点或结束实例。",
		targetOperation: "/flowInstanceApi/audit",
		parameters: []model.ActionParameter{
			{Name: "id", Required: true, Description: "目标流程实例键"},
			{Name: "jobTaskId", Required: true, Description: "当前待办键，由服务端绑定"},
			{Name: "auditRecord.auditStatus", Value: "pass", Required: true, Description: "固定同意结果"},
			{Name: "auditRecord.executeDesc", Required: false, Description: "审批意见"},
			{Name: "formDataMongoVo.data", Required: true, Description: "form-runtime 捕获的目标原始表单数据"},
			{Name: "nextAuditorList", Required: false, Description: "目标引擎解析的下一处理人"},
			{Name: "tracking", Required: false, Description: "当前用户关注状态"},
		},
		expectedEffect:     "当前待办记录为 pass；实例按真实路径推进，可能创建下一待办或进入 end。",
		reloadRequirements: []string{"实例状态", "流程代理", "当前节点", "待办列表", "实际路径"},
	},
	{
		action: model.ActionReject, category: model.ActionCategoryCurrentTodo, scope: model.ActionScopeTask,
		label: "不同意", description: "处理当前活动人工待办并按目标规则把实例退回发起人。",
		targetOperation: "/flowInstanceApi/audit",
		parameters: []model.ActionParameter{
			{Name: "id", Required: true, Description: "目标流程实例键"},
			{Name: "jobTaskId", Required: true, Description: "当前待办键，由服务端绑定"},
			{Name: "auditRecord.auditStatus", Value: "no_pass", Required: true, Description: "固定不同意结果"},
			{Name: "auditRecord.executeDesc", Required: false, Description: "审批意见"},
			{Name: "formDataMongoVo.data", Required: true, Description: "form-runtime 捕获的目标原始表单数据"},
			{Name: "tracking", Required: false, Description: "当前用户关注状态"},
		},
		expectedEffect:     "当前待办记录为 no_pass；实例进入 rejected 并等待发起人按目标规则重新提交。",
		reloadRequirements: []string{"实例状态", "当前节点", "待办列表", "发起人重提状态"},
	},
	{
		action: model.ActionRollback, category: model.ActionCategoryCurrentTodo, scope: model.ActionScopeTask,
		label: "回退上一节点", description: "只回退到目标引擎解析出的直接前一待办，不接受任意目标节点。",
		targetOperation: "/web/flowInstanceApi/rollBackThePreviousLevel",
		parameters: []model.ActionParameter{
			{Name: "id", Required: true, Description: "目标流程实例键"},
			{Name: "jobTaskId", Required: true, Description: "当前待办键，由服务端绑定"},
		},
		expectedEffect:     "实例回到真实直接前一待办并切换对应演员；前驱为发起节点时按目标规则阻止。",
		reloadRequirements: []string{"实例当前节点", "直接前一待办", "当前演员", "待办列表"},
	},
	{
		action: model.ActionRetrieve, category: model.ActionCategoryDoneRecovery, scope: model.ActionScopeCompletedTask,
		label: "取回", description: "由已办任务所属演员在后继尚未处理时恢复该审批步骤。",
		targetOperation: "/web/flowInstanceApi/retrieveProcess",
		parameters: []model.ActionParameter{
			{Name: "id", Required: true, Description: "目标流程实例键"},
			{Name: "jobTaskId", Required: true, Description: "操作者拥有的已完成任务键"},
		},
		expectedEffect:     "目标引擎建立新批次并恢复当前审批待办；后继已处理、发起节点或终态实例均不得取回。",
		reloadRequirements: []string{"实例批次", "恢复节点", "当前待办", "后继处理状态"},
	},
	{
		action: model.ActionWithdraw, category: model.ActionCategoryInstanceManagement, scope: model.ActionScopeInstance,
		label: "撤回", description: "仅运行中实例的创建人可撤回主实例。",
		targetOperation: "/web/flowInstanceApi/revocation",
		parameters: []model.ActionParameter{
			{Name: "id", Required: true, Description: "目标流程实例键"},
			{Name: "withdrawDesc", Required: false, Description: "撤回说明"},
		},
		expectedEffect:     "主实例由 run 进入 withdraw，并按目标实现处理当前待办；不会创建新的运行记录。",
		reloadRequirements: []string{"实例状态", "创建人归属", "当前待办"},
	},
	{
		action: model.ActionUrge, category: model.ActionCategoryInstanceManagement, scope: model.ActionScopeInstance,
		label: "催办", description: "向当前实例真实待办接收人发送催办，不改变主实例游标。",
		targetOperation: "/web/urgeHandleRecord/sendUrgeMessage",
		parameters: []model.ActionParameter{
			{Name: "flowInstanceId", Required: true, Description: "目标流程实例键"},
			{Name: "data", Required: true, Description: "目标催办协议数据"},
		},
		expectedEffect:     "向当前待办接收人发送一次催办记录；实例、任务和节点状态不变。",
		reloadRequirements: []string{"实例状态", "当前待办接收人", "催办状态"},
	},
	{
		action: model.ActionForward, category: model.ActionCategoryInstanceManagement, scope: model.ActionScopeInstance,
		label: "转发", description: "从待办、已发或已办上下文创建系统默认转发流程辅助实例。",
		targetOperation: "/web/flowInstanceApi/transpond",
		parameters: []model.ActionParameter{
			{Name: "receiverId", Required: true, Description: "转发接收人键，由实时候选选择"},
			{Name: "data.name", Required: true, Description: "辅助转发实例标题"},
			{Name: "data.flowInstanceBizRelevanceList", Required: false, Description: "目标业务关联列表"},
			{Name: "formDataMongoVo.data", Required: true, Description: "目标原始表单数据"},
		},
		expectedEffect:     "创建系统默认转发流程辅助实例；主实例继续保持原节点和状态。",
		reloadRequirements: []string{"主实例状态", "辅助实例状态", "转发接收人"},
	},
	{
		action: model.ActionFollow, category: model.ActionCategoryInstanceManagement, scope: model.ActionScopeInstance,
		label: "关注", description: "按当前用户保存主实例关注状态，不迁移任务。",
		targetOperation: "/web/flowInstanceApi/flowTracking",
		parameters: []model.ActionParameter{
			{Name: "data.id", Required: true, Description: "目标流程实例键"},
			{Name: "tracking", Value: "true", Required: true, Description: "固定开启当前用户关注"},
		},
		expectedEffect:     "当前用户关注主实例；流程状态、任务状态和节点位置不变。",
		reloadRequirements: []string{"当前用户关注状态"},
	},
	{
		action: model.ActionUnfollow, category: model.ActionCategoryInstanceManagement, scope: model.ActionScopeInstance,
		label: "取消关注", description: "按当前用户清除主实例关注状态，不迁移任务。",
		targetOperation: "/web/flowInstanceApi/flowTracking",
		parameters: []model.ActionParameter{
			{Name: "data.id", Required: true, Description: "目标流程实例键"},
			{Name: "tracking", Value: "false", Required: true, Description: "固定关闭当前用户关注"},
		},
		expectedEffect:     "当前用户取消关注主实例；流程状态、任务状态和节点位置不变。",
		reloadRequirements: []string{"当前用户关注状态"},
	},
}

// NewService 创建无状态动作目录服务。
func NewService() *Service { return &Service{} }

// Build 根据一次目标只读上下文构建稳定顺序的动作目录；函数不会执行任何目标写操作。
func Build(ctx model.ActionContext) []model.ActionCatalogItem {
	return NewService().Build(ctx)
}

// BuildCatalog 是 Build 的语义别名，供调用方以目录命名接入。
func BuildCatalog(ctx model.ActionContext) []model.ActionCatalogItem { return Build(ctx) }

// Build 使用实时目标事实计算每个动作的启用状态、前置事实和中文禁用原因。
func (s *Service) Build(ctx model.ActionContext) []model.ActionCatalogItem {
	items := make([]model.ActionCatalogItem, 0, len(actionDefinitions)+1)
	for _, definition := range actionDefinitions {
		gate := evaluate(definition.action, ctx)
		items = append(items, model.ActionCatalogItem{
			Action:             definition.action,
			Category:           definition.category,
			Scope:              definition.scope,
			NodeKey:            strings.TrimSpace(ctx.CurrentNodeKey),
			Label:              definition.label,
			Description:        definition.description,
			TargetOperation:    definition.targetOperation,
			Enabled:            gate.enabled,
			DisabledReason:     gate.reason,
			Parameters:         parameterNames(definition.parameters),
			ParameterDetails:   cloneParameters(definition.parameters),
			Preconditions:      gate.preconditions,
			ExpectedEffect:     definition.expectedEffect,
			RequiresReload:     true,
			ReloadRequirements: cloneStrings(definition.reloadRequirements),
			SystemOnly:         definition.systemOnly,
		})
	}
	items = append(items, buildSystemItem(ctx))
	return items
}

// evaluate 分发每个稳定动作键的目标门禁；未知键一律拒绝，避免新增动作绕过门禁。
func evaluate(action model.ActionKey, ctx model.ActionContext) gateResult {
	switch action {
	case model.ActionSaveDraft:
		return evaluateSaveDraft(ctx)
	case model.ActionSubmit:
		return evaluateSubmit(ctx)
	case model.ActionResubmit:
		return evaluateResubmit(ctx)
	case model.ActionStorageFormData, model.ActionApprove, model.ActionReject:
		return evaluateCurrentTask(ctx)
	case model.ActionAddSign:
		return evaluateAddSign(ctx)
	case model.ActionTransfer:
		return evaluateTransfer(ctx)
	case model.ActionRollback:
		return evaluateRollback(ctx)
	case model.ActionRetrieve:
		return evaluateRetrieve(ctx)
	case model.ActionWithdraw:
		return evaluateWithdraw(ctx)
	case model.ActionUrge:
		return evaluateUrge(ctx)
	case model.ActionForward:
		return evaluateForward(ctx)
	case model.ActionFollow:
		return evaluateFollow(ctx, false)
	case model.ActionUnfollow:
		return evaluateFollow(ctx, true)
	default:
		return denied("动作不属于当前目标动作目录", nil)
	}
}

// evaluateSaveDraft 只允许发起人在新建或已有草稿上下文保存草稿。
func evaluateSaveDraft(ctx model.ActionContext) gateResult {
	g := newGate()
	status := normalizedStatus(ctx.InstanceStatus)
	newOrDraft := sourceIsNew(ctx) || status == "draft"
	add(&g, "initiator", "当前用户是流程发起人", true, ctx.IsInitiator)
	add(&g, "new_or_draft", "实例处于新建或草稿状态", true, newOrDraft)
	add(&g, "not_ended", "实例尚未进入不可恢复终态", true, !instanceEnded(ctx))
	if !ctx.IsInitiator {
		return denyWith(g, "只有流程发起人可以保存草稿")
	}
	if instanceEnded(ctx) {
		return denyWith(g, "实例已经结束，不能保存草稿")
	}
	if !newOrDraft {
		return denyWith(g, "保存草稿只适用于新建或已有草稿实例")
	}
	return g
}

// evaluateSubmit 只允许发起人提交新建实例；已有草稿、驳回或撤回实例必须走重新提交。
func evaluateSubmit(ctx model.ActionContext) gateResult {
	g := newGate()
	status := normalizedStatus(ctx.InstanceStatus)
	newInstance := sourceIsNew(ctx) && status != "draft"
	add(&g, "initiator", "当前用户是流程发起人", true, ctx.IsInitiator)
	add(&g, "new_instance", "实例处于尚未持久化的新建状态", true, newInstance)
	add(&g, "not_ended", "实例尚未进入不可恢复终态", true, !instanceEnded(ctx))
	if !ctx.IsInitiator {
		return denyWith(g, "只有流程发起人可以提交实例")
	}
	if instanceEnded(ctx) {
		return denyWith(g, "实例已经结束，不能提交")
	}
	if !newInstance {
		return denyWith(g, "提交只适用于新建实例，草稿、驳回或撤回请使用重新提交")
	}
	return g
}

// evaluateResubmit 复刻目标重提服务只接受 draft、rejected 和 withdraw 的状态集合。
func evaluateResubmit(ctx model.ActionContext) gateResult {
	g := newGate()
	status := normalizedStatus(ctx.InstanceStatus)
	allowed := status == "draft" || status == "rejected" || status == "withdraw"
	add(&g, "initiator", "当前用户是流程发起人", true, ctx.IsInitiator)
	add(&g, "resubmittable_status", "实例状态属于 draft、rejected 或 withdraw", true, allowed)
	add(&g, "not_ended", "实例尚未进入不可恢复终态", true, !instanceEnded(ctx))
	if !ctx.IsInitiator {
		return denyWith(g, "只有流程发起人可以重新提交")
	}
	if instanceEnded(ctx) {
		return denyWith(g, "实例已经进入不可恢复终态，不能重新提交")
	}
	if !allowed {
		return denyWith(g, "当前实例状态不允许重新提交")
	}
	return g
}

// evaluateCurrentTask 统一校验审批暂存、同意和不同意的活动人工待办门禁。
func evaluateCurrentTask(ctx model.ActionContext) gateResult {
	g := newGate()
	active := ctx.HasCurrentTask && !ctx.CurrentTaskDone
	running := instanceRunning(ctx)
	human := isHumanNode(ctx.CurrentNodeType)
	forwarded := !ctx.ForwardedContext
	add(&g, "active_task", "当前用户拥有未处理的活动待办", true, active)
	add(&g, "running_instance", "实例处于 run 状态", true, running)
	add(&g, "human_node", "当前节点是人工审批或协同节点", true, human)
	add(&g, "not_forwarded", "当前上下文不是系统转发辅助流程", true, forwarded)
	if system := systemNodeType(ctx.CurrentNodeType); system != "" {
		return denyWith(g, "当前节点由系统自动处理，不提供用户动作配置")
	}
	if ctx.ForwardedContext {
		return denyWith(g, "转发辅助流程不提供主实例审批动作")
	}
	if !active {
		if ctx.CurrentTaskDone {
			return denyWith(g, "当前待办已经处理，不能重复执行")
		}
		return denyWith(g, "当前没有属于当前用户的活动待办")
	}
	if !running {
		return denyWith(g, "实例当前不在运行中")
	}
	if !human {
		return denyWith(g, "当前节点不是可处理的人工审批或协同节点")
	}
	return g
}

// evaluateAddSign 在活动人工待办基础上要求当前代理可编辑且已完成候选核对。
func evaluateAddSign(ctx model.ActionContext) gateResult {
	g := evaluateCurrentTask(ctx)
	add(&g, "editable_proxy", "当前实例流程代理及加签候选结构可编辑", true, ctx.HasEditableProxy)
	if !g.enabled {
		return g
	}
	if !ctx.HasEditableProxy {
		return denyWith(g, "当前流程代理不可编辑或加签候选尚未完整核对")
	}
	return g
}

// evaluateTransfer 要求目标后端允许切换当前待办演员，并保持当前任务不推进。
func evaluateTransfer(ctx model.ActionContext) gateResult {
	g := evaluateCurrentTask(ctx)
	add(&g, "actor_switch_permission", "当前待办允许从实时受限候选切换演员", true, ctx.CanSwitchActor)
	if !g.enabled {
		return g
	}
	if !ctx.CanSwitchActor {
		return denyWith(g, "当前待办没有可用的移交演员候选或权限")
	}
	return g
}

// evaluateRollback 只允许回退到已重读并确认的直接前一业务待办。
func evaluateRollback(ctx model.ActionContext) gateResult {
	g := evaluateCurrentTask(ctx)
	previousType := normalizedNodeType(ctx.PreviousNodeType)
	knownPrevious := ctx.PreviousTaskExists && previousType != ""
	notStart := knownPrevious && !ctx.PreviousNodeIsStart && previousType != "start"
	add(&g, "previous_task", "存在目标引擎解析出的直接前一待办", true, ctx.PreviousTaskExists)
	add(&g, "previous_node_known", "直接前一节点类型已经重读", true, knownPrevious)
	add(&g, "previous_not_start", "直接前一节点不是发起节点", true, notStart)
	if !g.enabled {
		return g
	}
	if !ctx.PreviousTaskExists {
		return denyWith(g, "当前待办没有可回退的直接前一节点")
	}
	if !knownPrevious {
		return denyWith(g, "直接前一节点类型尚未重读，无法安全回退")
	}
	if !notStart {
		return denyWith(g, "直接前一节点是发起节点，请按目标规则使用不同意")
	}
	return g
}

// evaluateRetrieve 复刻目标取回的已办归属、运行状态、后继未处理及会签边界。
func evaluateRetrieve(ctx model.ActionContext) gateResult {
	g := newGate()
	running := instanceRunning(ctx)
	notEnded := !instanceEnded(ctx)
	notStart := !ctx.RetrieveNodeIsStart && normalizedNodeType(ctx.CurrentNodeType) != "start"
	notUsed := !ctx.RetrieveAlreadyUsed
	notHandledByOther := !ctx.CurrentTaskHandledByOther
	add(&g, "owned_completed_task", "当前用户拥有目标已完成任务", true, ctx.HasCompletedTask)
	add(&g, "running_instance", "实例处于 run 状态", true, running)
	add(&g, "not_ended", "实例尚未完结", true, notEnded)
	add(&g, "successor_unprocessed", "取回任务的后继尚未处理", true, !ctx.NextTaskProcessed)
	add(&g, "retrieve_node_not_start", "已办任务不在发起节点", true, notStart)
	add(&g, "not_already_retrieved", "该已办任务尚未被取回", true, notUsed)
	add(&g, "parallel_or_countersign_clear", "会签或并行后继没有其他演员已处理", true, notHandledByOther)
	if !ctx.HasCompletedTask {
		return denyWith(g, "当前用户没有可取回的已完成任务")
	}
	if !running {
		return denyWith(g, "实例当前不在运行中，不能取回")
	}
	if !notEnded {
		return denyWith(g, "流程已完结，不支持取回")
	}
	if !notStart {
		return denyWith(g, "发起节点不支持取回")
	}
	if !notUsed {
		return denyWith(g, "当前已办任务已经取回，不能重复取回")
	}
	if ctx.NextTaskProcessed {
		return denyWith(g, "后继任务已经处理，不支持取回")
	}
	if !notHandledByOther {
		return denyWith(g, "会签或并行节点已有其他演员处理，不支持取回")
	}
	return g
}

// evaluateWithdraw 只允许运行中实例的创建人撤回，不把终止性管理能力混入目录。
func evaluateWithdraw(ctx model.ActionContext) gateResult {
	g := newGate()
	running := instanceRunning(ctx)
	notEnded := !instanceEnded(ctx)
	add(&g, "initiator", "当前用户是流程创建人", true, ctx.IsInitiator)
	add(&g, "running_instance", "实例处于 run 状态", true, running)
	add(&g, "not_ended", "实例尚未进入不可恢复终态", true, notEnded)
	if !ctx.IsInitiator {
		return denyWith(g, "只有流程创建人可以撤回实例")
	}
	if !running {
		return denyWith(g, "当前实例不在运行中，不能撤回")
	}
	if !notEnded {
		return denyWith(g, "实例已经结束，不能撤回")
	}
	return g
}

// evaluateUrge 允许有真实当前待办接收人的运行中实例催办；已发视图由 HasPendingRecipient 提供事实。
func evaluateUrge(ctx model.ActionContext) gateResult {
	g := newGate()
	running := instanceRunning(ctx)
	pendingRecipient := ctx.HasPendingRecipient || (ctx.HasCurrentTask && !ctx.CurrentTaskDone)
	add(&g, "running_instance", "实例处于 run 状态", true, running)
	add(&g, "pending_recipient", "实例存在当前待办接收人", true, pendingRecipient)
	if !running {
		return denyWith(g, "当前实例不在运行中，不能催办")
	}
	if !pendingRecipient {
		return denyWith(g, "当前实例没有可催办的待办接收人")
	}
	return g
}

// evaluateForward 只接受目标页面开放的待办、已发和已办实例上下文，并明确转发是旁支流程。
func evaluateForward(ctx model.ActionContext) gateResult {
	g := newGate()
	visible := instanceVisible(ctx)
	source := normalizedSource(ctx.FlowSource)
	allowedSource := source == "pending" || source == "submitted" || source == "done"
	notDueOut := source != "dueout" && source != "timedout"
	add(&g, "visible_instance", "当前账号已重读到可转发实例", true, visible)
	add(&g, "forward_context", "当前上下文属于待办、已发或已办实例", true, allowedSource)
	add(&g, "not_dueout", "当前上下文不是待发/超时列表", true, notDueOut)
	if !visible {
		return denyWith(g, "当前账号看不到可转发的目标实例")
	}
	if !notDueOut {
		return denyWith(g, "待发或超时上下文不提供转发入口")
	}
	if !allowedSource {
		return denyWith(g, "当前上下文不是目标页面开放的待办、已发或已办实例")
	}
	return g
}

// evaluateFollow 根据当前用户已有关注状态分别计算关注和取消关注门禁。
func evaluateFollow(ctx model.ActionContext, unfollow bool) gateResult {
	g := newGate()
	visible := instanceVisible(ctx)
	running := instanceRunning(ctx)
	state := ctx.Followed
	add(&g, "visible_instance", "当前账号已重读到目标实例", true, visible)
	add(&g, "running_instance", "目标页面允许对运行中实例修改关注状态", true, running)
	add(&g, "tracking_state", "当前用户关注状态已重读", true, true)
	if !visible {
		return denyWith(g, "当前账号看不到目标实例，无法修改关注状态")
	}
	if !running {
		return denyWith(g, "当前实例不在运行中，不能修改关注状态")
	}
	if unfollow && !state {
		return denyWith(g, "当前用户尚未关注该实例")
	}
	if !unfollow && state {
		return denyWith(g, "当前用户已经关注该实例")
	}
	return g
}

// buildSystemItem 把系统节点投影为只读自动语义，不向用户暴露可编排写动作。
func buildSystemItem(ctx model.ActionContext) model.ActionCatalogItem {
	nodeType := systemNodeType(ctx.CurrentNodeType)
	item := model.ActionCatalogItem{
		Action:             model.ActionSystemAutomatic,
		Category:           model.ActionCategorySystemAutomatic,
		Scope:              model.ActionScopeTask,
		NodeKey:            strings.TrimSpace(ctx.CurrentNodeKey),
		Label:              "系统自动语义",
		Description:        "条件、手动、并行、汇聚、空、结束、定时、子流程和回调只读展示，不提供用户动作配置。",
		TargetOperation:    "",
		Enabled:            nodeType != "",
		Parameters:         []string{},
		ParameterDetails:   []model.ActionParameter{},
		Preconditions:      []model.ActionPrecondition{{Key: "system_node", Label: "当前节点属于目标系统自动节点", Required: true, Present: nodeType != ""}},
		ExpectedEffect:     systemEffect(nodeType),
		RequiresReload:     true,
		ReloadRequirements: []string{"系统节点实际状态", "自动路由或等待结果"},
		SystemOnly:         true,
		SystemNodeType:     nodeType,
	}
	if nodeType == "" {
		if strings.TrimSpace(ctx.CurrentNodeType) == "" {
			item.DisabledReason = "当前节点类型尚未重读，不能投影系统自动语义"
		} else {
			item.DisabledReason = "当前节点不是目标系统自动节点"
		}
	}
	return item
}

// systemEffect 返回系统节点的只读预期语义，不把自动路由伪装成用户动作。
func systemEffect(nodeType string) string {
	switch nodeType {
	case "condition":
		return "目标引擎按条件顺序取首个命中分支，末分支作为兜底；用户不能配置该自动步骤。"
	case "manual":
		return "目标引擎等待或读取人工分支选择；用户动作目录不替代该选择。"
	case "parallel":
		return "目标引擎保留全部并行分支并在汇聚处继续；用户不能删减分支。"
	case "merge":
		return "目标引擎等待已纳入分支汇聚后继续主线。"
	case "empty":
		return "目标引擎自动穿过空节点，不产生用户待办。"
	case "end":
		return "目标引擎结束当前流程路径，不提供终止动作配置。"
	case "timer":
		return "目标引擎按真实定时规则等待或触发，工具侧只读显示。"
	case "subprocess":
		return "目标引擎按子流程关系等待或推进，工具侧不创建运行记录。"
	case "callback":
		return "目标引擎按回调协议通知业务方，工具侧只读显示回调语义。"
	default:
		return "当前节点的系统自动语义仅供只读核对。"
	}
}

// newGate 建立启用默认值；后续门禁只会把明确不满足的事实降为禁用。
func newGate() gateResult {
	return gateResult{enabled: true, preconditions: []model.ActionPrecondition{}}
}

// add 以固定顺序追加一个前置事实，保证页面能解释每个动作为何可用或不可用。
func add(g *gateResult, key, label string, required, present bool) {
	g.preconditions = append(g.preconditions, model.ActionPrecondition{Key: key, Label: label, Required: required, Present: present})
}

// denyWith 使用首个业务门禁原因关闭动作；已满足的前置事实仍完整返回给页面。
func denyWith(g gateResult, reason string) gateResult {
	g.enabled = false
	g.reason = strings.TrimSpace(reason)
	if g.reason == "" {
		g.reason = "当前目标上下文未满足动作门禁"
	}
	return g
}

// denied 构造未知动作的安全拒绝结果。
func denied(reason string, preconditions []model.ActionPrecondition) gateResult {
	return gateResult{enabled: false, reason: reason, preconditions: preconditions}
}

// parameterNames 保持兼容的字符串参数投影，同时由 ParameterDetails 提供是否必填和目标语义。
func parameterNames(parameters []model.ActionParameter) []string {
	result := make([]string, 0, len(parameters))
	for _, parameter := range parameters {
		result = append(result, parameter.Name)
	}
	return result
}

// cloneParameters 复制目录定义，避免调用方修改全局动作元数据。
func cloneParameters(parameters []model.ActionParameter) []model.ActionParameter {
	result := make([]model.ActionParameter, len(parameters))
	copy(result, parameters)
	return result
}

// cloneStrings 复制重读要求，保证一次目录响应不会共享可变切片。
func cloneStrings(values []string) []string {
	result := make([]string, len(values))
	copy(result, values)
	return result
}

// sourceIsNew 判断是否仍处于发起新实例或尚未持久化实例的上下文。
func sourceIsNew(ctx model.ActionContext) bool {
	source := normalizedSource(ctx.FlowSource)
	status := normalizedStatus(ctx.InstanceStatus)
	return (source == "" || source == "new" || source == "start") && (status == "" || status == "draft")
}

// instanceEnded 合并目标明确终态和显式结束标记；rejected/withdraw 仍允许目标重提，因此不视为终态。
func instanceEnded(ctx model.ActionContext) bool {
	if ctx.InstanceEnded {
		return true
	}
	switch normalizedStatus(ctx.InstanceStatus) {
	case "end", "termination", "abandon", "cancelled", "completed":
		return true
	default:
		return false
	}
}

// instanceRunning 只把目标明确返回的 run 状态视为运行中，缺失或未知状态保持禁用。
func instanceRunning(ctx model.ActionContext) bool {
	if instanceEnded(ctx) {
		return false
	}
	switch normalizedStatus(ctx.InstanceStatus) {
	case "run":
		return true
	default:
		return false
	}
}

// instanceVisible 判断当前账号是否有可重读的持久化实例上下文；新建表单不算实例管理对象。
func instanceVisible(ctx model.ActionContext) bool {
	if ctx.InstanceVisible {
		return true
	}
	if ctx.HasCurrentTask || ctx.HasCompletedTask || ctx.HasPendingRecipient {
		return true
	}
	status := normalizedStatus(ctx.InstanceStatus)
	source := normalizedSource(ctx.FlowSource)
	if source == "new" || (source == "" && status == "") {
		return false
	}
	return status != ""
}

// normalizedStatus 统一目标实例状态别名，避免页面中文或大小写差异改变门禁。
func normalizedStatus(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.ReplaceAll(value, "-", "_")
	switch value {
	case "running", "inprogress", "in_progress", "processing", "进行中", "运行中":
		return "run"
	case "finished", "done", "completed", "complete", "已完成", "已结束", "结束":
		return "end"
	case "撤回", "revoked":
		return "withdraw"
	case "驳回", "rejected":
		return "rejected"
	case "草稿", "draft":
		return "draft"
	case "termination", "terminated", "终止":
		return "termination"
	case "abandon", "abandoned", "废弃":
		return "abandon"
	case "cancelled", "canceled", "取消":
		return "cancelled"
	default:
		return value
	}
}

// normalizedSource 统一 GroupApproveManage 的列表上下文名称。
func normalizedSource(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "todo", "pending", "backlog", "待办":
		return "pending"
	case "sent", "submitted", "submit", "已发":
		return "submitted"
	case "done", "finished", "completed", "已办":
		return "done"
	case "due_out", "dueout", "待发":
		return "dueout"
	case "timed_out", "timedout", "超时":
		return "timedout"
	case "new", "start", "initiator", "发起":
		return "new"
	default:
		return value
	}
}

// normalizedNodeType 统一目标代理节点类型和 GroupApproveManage 的业务别名。
func normalizedNodeType(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.ReplaceAll(value, "-", "_")
	switch value {
	case "common", "normal", "approval", "approve", "audit", "审批", "普通", "人工审批":
		return "common"
	case "synergy", "cooperate", "collaboration", "协同", "协作":
		return "synergy"
	case "start", "begin", "发起", "开始":
		return "start"
	case "condition", "exclusive", "exclusive_gateway", "条件", "条件分支":
		return "condition"
	case "manual", "custom_choose", "manual_choose", "手动", "手动分支":
		return "manual"
	case "parallel", "parallel_gateway", "并行":
		return "parallel"
	case "merge", "converge", "汇聚", "汇合":
		return "merge"
	case "empty", "空":
		return "empty"
	case "end", "terminal", "结束":
		return "end"
	case "timer", "定时", "定时器":
		return "timer"
	case "subprocess", "sub_process", "子流程":
		return "subprocess"
	case "callback", "call_back", "回调":
		return "callback"
	default:
		return value
	}
}

// isHumanNode 判断当前节点是否是目标允许用户处理的人工审批或协同节点。
func isHumanNode(raw string) bool {
	typeName := normalizedNodeType(raw)
	return typeName == "common" || typeName == "synergy"
}

// systemNodeType 返回目标系统自动节点的规范名称；未知类型不被猜测为系统节点。
func systemNodeType(raw string) string {
	typeName := normalizedNodeType(raw)
	switch typeName {
	case "condition", "manual", "parallel", "merge", "empty", "end", "timer", "subprocess", "callback":
		return typeName
	default:
		return ""
	}
}
