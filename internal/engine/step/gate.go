package step

import (
	"encoding/json"
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
	if step.Action == model.ActionSubmit {
		// 新发起：实例还不存在，“新建且非草稿”与“当前用户是发起人”都由运行上下文保证。
		ctx.FlowSource = runCtx.Source
		ctx.InstanceStatus = ""
		ctx.IsInitiator = true
		ctx.InstanceVisible = false
		return ctx
	}
	ctx.InstanceStatus = facts.Status
	ctx.InstanceVisible = facts.Found
	ctx.HasCurrentTask = len(facts.DueNodes) > 0
	ctx.CurrentTaskDone = facts.Found && len(facts.DueNodes) == 0
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
func buildRequest(runCtx RunContext, step model.CompiledActionStep, session target.Session) (any, string, map[string]any, error) {
	nextAuditors := nextAuditorsOf(step)
	switch step.Action {
	case model.ActionSubmit:
		// 手动条件分支（custom_choose）的选择必须随提交以 nextAuditorList[].nodeProxyId 传递
		// （FlowOperateServiceImpl.validateHandBranchAndReturnExecuteNode 按 nodeProxyId 匹配候选分支节点），
		// 缺失时目标以「手动条件分支,请选择」拒绝。fixedExecuteNodeId 是并行条件分支的另一机制，此处不用。
		auditors := nextAuditors
		if branchTarget := runCtx.SubmitBranchTargetNodeID; branchTarget != "" {
			auditors = append([]target.NextAuditor{{NodeProxyID: branchTarget}}, auditors...)
		}
		request := target.SubmitFlowInstanceRequest{
			Name:         instanceName(runCtx, step),
			FlowProxyID:  runCtx.FlowProxyID,
			CompanyID:    session.CompanyID,
			FormData:     json.RawMessage(runCtx.EffectiveFormData),
			NextAuditors: auditors,
		}
		return &request, target.WriteEndpointSubmit, target.BuildSubmitBody(request), nil
	case model.ActionApprove:
		request := target.AuditCurrentTaskRequest{
			InstanceID:   runCtx.PathRun.MainInstanceRef,
			FlowProxyID:  runCtx.FlowProxyID,
			AuditStatus:  "pass",
			ExecuteDesc:  auditMessage(runCtx, step),
			FormData:     branchFormData(step),
			NextAuditors: nextAuditors,
		}
		return &request, target.WriteEndpointAudit, target.BuildAuditBody(request), nil
	default:
		return nil, "", nil, &UnverifiedActionError{Action: step.Action}
	}
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

// branchFormData 提取分支判断字段：条件字段在放行前可能被人工调整，参数里的 branchFormData
// 是控制层确认过的最终值，按原始 JSON 透传。
func branchFormData(step model.CompiledActionStep) json.RawMessage {
	raw, err := json.Marshal(step.Parameters["branchFormData"])
	if err != nil || len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return raw
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
