package step

import (
	"encoding/json"
	"fmt"
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/actioncatalog"
	"test-auto-pro-v2/internal/engine/verdict"
	"test-auto-pro-v2/internal/model"
)

// buildGateContext 把目标实时事实投影为动作目录可判定的上下文。
// 投影规则：
//   - 发起步骤：实例尚未持久化，状态留空即“新建”事实（actioncatalog 的 sourceIsNew 判据），
//     发起人就是计划账号；
//   - 审批步骤：实例事实来自发起人会话的重读，待办事实来自演员会话的重读，
//     缺一不可，不得凭配置推断。
func buildGateContext(runCtx RunContext, step model.CompiledActionStep, facts InstanceFacts, info NodeInfo) model.ActionContext {
	ctx := model.ActionContext{
		FlowSource:      runCtx.Source,
		CurrentNodeKey:  step.NodeKey,
		CurrentNodeType: info.Type,
	}
	if step.Action == model.ActionSubmit || step.Action == model.ActionSaveDraft || step.Action == model.ActionResubmit {
		// 发起节点动作都由计划账号以发起人身份执行：
		// 2026-09-07 实测修复——save_draft 此前落入审批分支，IsInitiator 恒为 false，
		// 发起人本人的草稿也被门禁「只有流程发起人可以保存草稿」误拒。
		ctx.FlowSource = runCtx.Source
		// 新发起没有实例创建人可读；已有实例必须使用目标返回的创建人事实。
		ctx.IsInitiator = !facts.CreatorRead || facts.IsInitiator
		if step.Action == model.ActionSubmit {
			// 新发起提交：实例还不存在，“新建且非草稿”由运行上下文保证。
			ctx.InstanceStatus = ""
			ctx.InstanceVisible = false
			return ctx
		}
		// 保存草稿作用于真实实例（待发草稿或新建），状态事实来自目标重读，
		// 必须带上，否则「新建或草稿」判据拿空状态恒失败（2026-09-07 实测第二层）。
		ctx.InstanceStatus = facts.Status
		ctx.InstanceVisible = facts.Found
		if facts.CurrentTaskRead {
			ctx.HasCurrentTask = facts.CurrentTaskFound
			ctx.CurrentTaskDone = facts.Found && !facts.CurrentTaskFound
		} else {
			ctx.HasCurrentTask = len(facts.DueNodes) > 0
		}
		return ctx
	}
	ctx.InstanceStatus = facts.Status
	ctx.InstanceVisible = facts.Found
	if facts.Found && strings.EqualFold(strings.TrimSpace(ctx.FlowSource), "new") {
		// 提交后的同一运行仍保留计划来源 new；实例已经由目标创建后，实例级动作应按已发实例门禁判断。
		ctx.FlowSource = "submitted"
	}
	if step.Action == model.ActionWithdraw {
		// 撤回只能由目标实例创建人执行；没有成功读取创建人事实时必须保持不可用。
		ctx.IsInitiator = facts.CreatorRead && facts.IsInitiator
	}
	if facts.CurrentTaskRead {
		ctx.HasCurrentTask = facts.CurrentTaskFound
		ctx.CurrentTaskDone = facts.Found && !facts.CurrentTaskFound
	} else {
		ctx.HasCurrentTask = len(facts.DueNodes) > 0
		ctx.CurrentTaskDone = facts.Found && len(facts.DueNodes) == 0
	}
	// 实例级动作也必须基于本次目标读取决定可用性：关注状态、待办接收人和已办归属
	// 都不能从用户之前保存的编排配置推断。
	ctx.HasPendingRecipient = len(facts.DueNodes) > 0 || (facts.PendingTaskRead && facts.PendingTaskFound)
	ctx.Followed = facts.TrackingRead && facts.Tracking
	ctx.HasEditableProxy = facts.EditableProxyRead
	ctx.CanSwitchActor = facts.ActorSwitchRead
	ctx.HasCompletedTask = facts.CompletedTaskRead && facts.CompletedTaskFound
	ctx.SuccessorStateKnown = facts.SuccessorStateKnown
	ctx.NextTaskProcessed = facts.NextTaskProcessed
	ctx.PreviousTaskExists = facts.PreviousTaskRead && facts.PreviousTaskExists
	ctx.PreviousNodeType = facts.PreviousNodeType
	ctx.PreviousNodeIsStart = facts.PreviousNodeIsStart
	ctx.RetrieveNodeIsStart = facts.RetrieveNodeIsStart
	ctx.RetrieveAlreadyUsed = facts.RetrieveAlreadyUsed
	ctx.CurrentTaskHandledByOther = facts.CurrentTaskHandledByOther
	ctx.CurrentTaskCountersign = facts.CurrentTaskCountersign
	ctx.CurrentTaskParallel = facts.CurrentTaskParallel
	return ctx
}

// evaluateGate 用动作目录对当前事实重新计算门禁。
// 配置时通过、此刻不通过就停止：门禁不通过绝不跳过（纲领第 4.3 节）。
func evaluateGate(step model.CompiledActionStep, ctx model.ActionContext) (model.ActionCatalogItem, bool) {
	catalog := actioncatalog.BuildCatalog(ctx)
	for _, item := range catalog {
		if item.Action != step.Action {
			continue
		}
		if item.Scope != "" && item.Scope != step.Scope {
			continue
		}
		return item, item.Enabled
	}
	return model.ActionCatalogItem{}, false
}

// buildRequest 构造本步的类型化写请求与其协议载荷（载荷由适配层导出的构造器生成，
// 与实际发出的请求严格同源）。审批任务 ID 不在此处填写：它必须在发送前现场新鲜读取。
// 端点必须落在白名单内；未验证动作直接拒绝，绝不静默换端点。
//
// formData 是 BuildNodeFormData 已按节点权限算好的完整表单数据：目标保存是整份覆盖，
// 所以除了明确不带表单数据的动作，这里一律提交这一份，不再直接透传历史快照。
func buildRequest(runCtx RunContext, step model.CompiledActionStep, session target.Session, formData json.RawMessage, nextNodeKey string) (any, string, map[string]any, error) {
	return buildRequestWithFacts(runCtx, step, session, formData, nextNodeKey, InstanceFacts{})
}

// buildRequestWithFacts 在构造动作载荷时接收放行前刚读取的目标事实；实例代理和业务关联必须使用实时值，
// 避免重提、审批、不同意或转发覆盖后丢失目标已有上下文。事实为空时保留测试和纯载荷构造调用的既有行为。
func buildRequestWithFacts(runCtx RunContext, step model.CompiledActionStep, session target.Session, formData json.RawMessage, nextNodeKey string, facts InstanceFacts) (any, string, map[string]any, error) {
	targetNodeID := runCtx.Nodes[step.NodeKey].TargetNodeID
	switch step.Action {
	case model.ActionSubmit, model.ActionSaveDraft:
		nextAuditors := nextAuditorsOf(step)
		if step.Action == model.ActionSubmit {
			var err error
			nextAuditors, err = nextAuditorsForTransition(runCtx, step, nextNodeKey, true)
			if err != nil {
				return nil, "", nil, err
			}
		}
		request := target.SubmitFlowInstanceRequest{
			InstanceID:   runCtx.PathRun.MainInstanceRef,
			Name:         instanceName(runCtx, step),
			FlowProxyID:  runCtx.FlowProxyID,
			CompanyID:    session.CompanyID,
			FormData:     formData,
			BizRelevance: cloneBizRelevance(facts.BizRelevance),
			NextAuditors: nextAuditors,
		}
		if step.Action == model.ActionSaveDraft {
			request.Status = "draft"
		}
		return &request, target.WriteEndpointSubmit, target.BuildSubmitBody(request), nil
	case model.ActionApprove:
		nextAuditors, err := nextAuditorsForTransition(runCtx, step, nextNodeKey, false)
		if err != nil {
			return nil, "", nil, err
		}
		request := target.AuditCurrentTaskRequest{
			InstanceID:   runCtx.PathRun.MainInstanceRef,
			FlowProxyID:  firstNonEmpty(facts.CurrentTaskFlowProxy, facts.FlowProxyID, runCtx.FlowProxyID),
			AuditStatus:  "pass",
			ExecuteDesc:  auditMessage(runCtx, step),
			FormData:     formData,
			BizRelevance: cloneBizRelevance(facts.BizRelevance),
			NextAuditors: nextAuditors,
		}
		return &request, target.WriteEndpointAudit, target.BuildAuditBody(request), nil
	case model.ActionResubmit:
		nextAuditors, err := nextAuditorsForTransition(runCtx, step, nextNodeKey, true)
		if err != nil {
			return nil, "", nil, err
		}
		request := target.ActionWriteRequest{
			Action:       string(step.Action),
			InstanceID:   runCtx.PathRun.MainInstanceRef,
			FlowProxyID:  firstNonEmpty(facts.FlowProxyID, runCtx.FlowProxyID),
			FormProxyID:  facts.FormProxyID,
			CompanyID:    session.CompanyID,
			FormData:     formData,
			BizRelevance: cloneBizRelevance(facts.BizRelevance),
			NextAuditors: nextAuditors,
		}
		body, endpoint, err := target.BuildActionBody(request)
		if err != nil {
			return nil, "", nil, err
		}
		return &request, endpoint, body, nil
	default:
		// F-019 全动作分派：不同意(no_pass)、暂存、重新提交、回退、取回、撤回、催办、转发、
		// 加签/移交、关注/取消关注全部经动作目录语义走统一载荷构造器（端点在白名单内）。
		request := target.ActionWriteRequest{
			Action:       string(step.Action),
			InstanceID:   runCtx.PathRun.MainInstanceRef,
			FlowProxyID:  firstNonEmpty(facts.CurrentTaskFlowProxy, facts.FlowProxyID, runCtx.FlowProxyID),
			FormData:     formData,
			BizRelevance: cloneBizRelevance(facts.BizRelevance),
			NextAuditors: nextAuditorsOf(step),
		}
		switch step.Action {
		case model.ActionReject, model.ActionTransfer:
			request.AuditStatus = auditStatusOf(step.Action)
			request.ExecuteDesc = auditMessage(runCtx, step)
			request.NodeProxyID = targetNodeID
			request.UserIDs = append([]string(nil), runCtx.ActionPersonIDs[actionPersonKey(step.NodeKey, step.Action)]...)
		case model.ActionAddSign:
			request.ExecuteDesc = auditMessage(runCtx, step)
			request.NodeProxyID = targetNodeID
			request.UserIDs = append([]string(nil), runCtx.ActionPersonIDs[actionPersonKey(step.NodeKey, step.Action)]...)
		case model.ActionStorageFormData:
			request.ExecuteDesc = auditMessage(runCtx, step)
			request.NodeProxyID = targetNodeID
		case model.ActionRollback:
			request.JobTaskID = "" // 发送前按待办现场新鲜读取
			request.ExecuteDesc = auditMessage(runCtx, step)
		case model.ActionRetrieve, model.ActionUrge:
			request.JobTaskID = ""
		case model.ActionWithdraw:
			request.ExecuteDesc = auditMessage(runCtx, step)
		case model.ActionForward:
			request.ReceiverID = firstActionPersonID(runCtx.ActionPersonIDs[actionPersonKey(step.NodeKey, step.Action)])
			request.Name = instanceName(runCtx, step) + "（转发）"
		case model.ActionFollow:
			request.Tracking = boolPtr(true)
		case model.ActionUnfollow:
			request.Tracking = boolPtr(false)
		default:
			return nil, "", nil, &UnverifiedActionError{Action: step.Action}
		}
		body, endpoint, err := target.BuildActionBody(request)
		if err != nil {
			return nil, "", nil, err
		}
		return &request, endpoint, body, nil
	}
}

// nextAuditorsForTransition 按目标提交、重新提交和同意共用的协议构造下一节点选人数据。
// 仅 run_node_choose 需要本平台指定真实用户；其他动态审批方式由目标按当前表单和组织上下文解析，
// 不能伪造空 bizId 的节点占位项，否则目标会把它当成无效人员配置。
func nextAuditorsForTransition(runCtx RunContext, step model.CompiledActionStep, nextNodeKey string, includeSubmitBranch bool) ([]target.NextAuditor, error) {
	auditors := append([]target.NextAuditor(nil), nextAuditorsOf(step)...)
	if includeSubmitBranch {
		// 手动条件分支（custom_choose）的选择必须以 nextAuditorList[].nodeProxyId 传递。
		if branchTarget := strings.TrimSpace(runCtx.SubmitBranchTargetNodeID); branchTarget != "" {
			auditors = append([]target.NextAuditor{{NodeProxyID: branchTarget}}, auditors...)
		}
	}
	info, exists := runCtx.Nodes[strings.TrimSpace(nextNodeKey)]
	if !exists || strings.TrimSpace(info.AuditType) != "run_node_choose" {
		return auditors, nil
	}
	if strings.TrimSpace(info.TargetNodeID) == "" {
		return nil, fmt.Errorf("下一节点「%s」缺少目标节点标识，无法选择处理人", info.Name)
	}
	selected := runCtx.NextNodeAuditors[strings.TrimSpace(nextNodeKey)]
	if len(selected) == 0 {
		return nil, fmt.Errorf("下一节点「%s」需要选择处理人，但未解析到当前有效人员", info.Name)
	}
	for _, candidate := range selected {
		candidate.BizID = strings.TrimSpace(candidate.BizID)
		candidate.Name = strings.TrimSpace(candidate.Name)
		if candidate.BizID == "" || candidate.Name == "" {
			return nil, fmt.Errorf("下一节点「%s」存在无效处理人，无法发送目标请求", info.Name)
		}
		candidate.AuditDetailTyp = "personnel"
		candidate.NodeProxyID = info.TargetNodeID
		auditors = append(auditors, candidate)
	}
	return auditors, nil
}

// auditStatusOf 返回需要 auditRecord 的动作对应的目标 ExecuteResultEnum 编码名。
// 加签通过 updateFlowProxy 写入完整代理树，不携带 auditStatus。
func auditStatusOf(action model.ActionKey) string {
	switch action {
	case model.ActionReject:
		return "no_pass"
	case model.ActionTransfer:
		return "transfer"
	default:
		return string(action)
	}
}

// firstActionPersonID 取动作人员策略解析出的首个真实用户 ID；转发目标必须来自服务端目录策略，不能信任浏览器参数。
func firstActionPersonID(values []string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// boolPtr 返回布尔指针。
func boolPtr(value bool) *bool {
	return &value
}

// actionPersonKey 生成运行上下文内人员解析结果的稳定索引，不把目标人员 ID 写入场景配置。
func actionPersonKey(nodeKey string, action model.ActionKey) string {
	return ActionPersonIndex(nodeKey, action)
}

// nextAuditorsOf 提取分支选择参数 fixedExecuteNodeId：条件分支的手动指定节点，
// 以目标“人员型选人”结构传递（与目标前端分支选择的传递方式一致）。
func nextAuditorsOf(step model.CompiledActionStep) []target.NextAuditor {
	nodeID, ok := step.Parameters["fixedExecuteNodeId"].(string)
	if !ok || nodeID == "" {
		return nil
	}
	return []target.NextAuditor{{NodeProxyID: nodeID, AuditDetailTyp: "personnel"}}
}

// instanceName 生成发起实例的显示名：优先取动作参数里的 instanceName，否则用路径名加运行号。
func instanceName(runCtx RunContext, step model.CompiledActionStep) string {
	if value, ok := step.Parameters["instanceName"].(string); ok {
		if name := strings.TrimSpace(value); name != "" {
			return name
		}
	}
	return strings.TrimSpace(runCtx.PathName) + "-运行" + formatUint(runCtx.Run.RunNo)
}

// auditMessage 生成审批意见：优先取动作参数里的 approveMessage。
func auditMessage(runCtx RunContext, step model.CompiledActionStep) string {
	if value, ok := step.Parameters["approveMessage"].(string); ok {
		if message := strings.TrimSpace(value); message != "" {
			return message
		}
	}
	return "流程自动化测试平台代为同意（运行" + formatUint(runCtx.Run.RunNo) + "）"
}

// validateWritePayloadKeys 递归收集载荷里的全部键名并交给判定包校验，
// 确保写请求绝不携带 batchCode（F-014 第 2.2 节禁令），发送前强制执行。
func validateWritePayloadKeys(payload map[string]any) error {
	return verdict.ValidateWritePayload(collectKeys(payload))
}

// collectKeys 深度优先收集载荷对象树的全部键名，含数组内嵌套对象。
func collectKeys(value any) []string {
	keys := make([]string, 0, 8)
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			for key, child := range typed {
				keys = append(keys, key)
				walk(child)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case json.RawMessage:
			var decoded any
			if err := json.Unmarshal(typed, &decoded); err == nil {
				walk(decoded)
			}
		}
	}
	walk(value)
	return keys
}

// BuildRequestForTest 暴露提交载荷构造，供 test 目录下的定向用例锁定 nextAuditorList 语义。
func BuildRequestForTest(runCtx RunContext, step model.CompiledActionStep, session target.Session, formData json.RawMessage, nextNodeKey string) (any, string, map[string]any, error) {
	return buildRequest(runCtx, step, session, formData, nextNodeKey)
}

// BuildRequestWithFactsForTest 暴露带实时实例事实的请求构造，供 test 目录锁定业务关联不会在执行器层丢失。
func BuildRequestWithFactsForTest(runCtx RunContext, step model.CompiledActionStep, session target.Session, formData json.RawMessage, nextNodeKey string, facts InstanceFacts) (any, string, map[string]any, error) {
	return buildRequestWithFacts(runCtx, step, session, formData, nextNodeKey, facts)
}
