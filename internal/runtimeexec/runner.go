// T012 Runtime 执行器：七阶段执行器的可选 runtime 运行面。
// 一个动作经 runtime 执行的完整编排：创建会话 → 装载（发起/审批）→ WS 业务命令 →
// 选择闭环自动继续（最多一次，仅限三种明确选择错误）→ 消费完成检测结果。
package runtimeexec

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// NextAuditorEntry 是 runtime 协议的节点人员条目（与 TS 侧 NextAuditorEntry 对齐）。
type NextAuditorEntry struct {
	NodeProxyID     string `json:"nodeProxyId"`
	BizID           string `json:"bizId"`
	Name            string `json:"name"`
	AuditDetailType string `json:"auditDetailType"`
}

// BranchSelection 是一条手动分支选择（分支节点代理 ID → 选中分支入口节点代理 ID）。
type BranchSelection struct {
	NodeProxyID string
	EntryNodeID string
}

// RuntimeRunRequest 是一次 runtime 动作执行的全部输入；v2 不再携带 Go 渲染模板。
type RuntimeRunRequest struct {
	RunID     string
	PathRunID string
	StepID    string
	Account   string
	Action    string
	// ToolFlowName 是工具流程名称：最终流程请求中目标脚本不得改写。
	ToolFlowName         string
	Session              SessionFacts
	Initiator            ActorIdentity
	Values               json.RawMessage
	Permissions          []string
	ChangedFields        []string
	InstanceID           string
	JobTaskID            string
	NodeProxyID          string
	AuditDesc            string
	FormProxyID          string
	FlowProxyID          string
	// FlowTemplateID 是计划绑定的流程模板 ID：发起装载用它精确选中模板（P1-2 专字段）。
	FlowTemplateID       string
	SelectFlowType       string
	FlowNodeType         string
	NextNodeProxyID      string
	BizRelevance         []BizRelevance
	ReceiverID           string
	ReceiverName         string
	ForwardCheckList     []string
	BranchSelections     []BranchSelection
	NextNodeAuditors     []NextAuditorEntry
	BranchNodeAuditTypes map[string]string
	BaseURL              string
	LoadMode             string
}

// ActionRunResult 是 runtime 动作的最终判定事实（状态为协议七值之一）。
// BusinessSummary 携带 runtime 回传的业务摘要（分发事实与写端点判定输入），供证据落盘；
// NewInstanceID 是发起链写响应里的新实例 ID（json:"-"：由本包从摘要解析，外部不可注入）。
type ActionRunResult struct {
	Status          string
	Message         string
	RequestTrace    json.RawMessage
	BusinessSummary json.RawMessage
	NewInstanceID   string   `json:"-"`
	WriteTraceIDs   []string `json:"-"`
}

// RuntimeRunner 是执行器对 runtime 能力的接口面（*RuntimeExecutor 满足）。
type RuntimeRunner interface {
	Health(ctx context.Context) error
	RunAction(ctx context.Context, request RuntimeRunRequest) (ActionRunResult, error)
}

// RuntimeExecutor 基于 Client 实现完整动作编排。
type RuntimeExecutor struct {
	client *Client
}

// NewRuntimeExecutor 构造 runtime 执行器。
func NewRuntimeExecutor(client *Client) *RuntimeExecutor {
	return &RuntimeExecutor{client: client}
}

// Health 探测独立执行服务；失败时七个动作写前阻塞。
func (r *RuntimeExecutor) Health(ctx context.Context) error {
	if r == nil || r.client == nil {
		return fmt.Errorf("执行服务客户端未配置")
	}
	return r.client.Health(ctx)
}

// loadModeFor 按动作推导装载模式：发起链两个动作走发起装载，转发走转发装载，其余走审批装载。
func loadModeFor(action string) string {
	switch action {
	case "save_draft", "submit":
		return LoadModeInitiate
	case "forward":
		return LoadModeForward
	case "resubmit":
		return LoadModeEdit
	default:
		return LoadModeApprove
	}
}

// RunAction 执行一次完整 runtime 动作并返回完成检测判定。
// 会话生命周期归本方法所有：无论成败，返回前销毁会话并释放 Context。
func (r *RuntimeExecutor) RunAction(ctx context.Context, request RuntimeRunRequest) (ActionRunResult, error) {
	handle, err := r.client.CreateSession(ctx, SessionSpec{
		AccountKey: request.Account,
		RunID:      request.RunID,
		PathRunID:  request.PathRunID,
		StepID:     request.StepID,
	})
	if err != nil {
		return ActionRunResult{}, fmt.Errorf("创建 runtime 会话失败：%w", err)
	}
	defer func() {
		destroyCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = r.client.Destroy(destroyCtx, handle.SessionID)
	}()

	// ⓪ 预热命令通道：装载期页面的目标读取（模板、字段权限、远程选项）要经 WS 转发 Go，
	// 必须先于装载连接；若等 Execute 才连通道，装载期读取全部被本地中止，表单装载不完
	// （2026-09-16 真实验收实测）。预热失败不阻断（Execute 仍会重连）。
	if closeWS, prewarmErr := r.client.Prewarm(ctx, handle.SessionID, request.Account); prewarmErr == nil {
		defer closeWS()
	}

	// ① 装载：发起模式（submit/save_draft）由目标自装载模板（载荷带 flowId 由执行器填写）；
	// 审批模式注入模板/值/权限。
	loadMode := request.LoadMode
	if loadMode == "" {
		loadMode = loadModeFor(request.Action)
	}
	permissions := request.Permissions
	if permissions == nil {
		permissions = []string{}
	}
	loadPayload, err := BuildLoadPayload(LoadInput{
		Mode:          loadMode,
		Session:       request.Session,
		ToolFlowName:  request.ToolFlowName,
		InstanceID:    request.InstanceID,
		JobTaskID:     request.JobTaskID,
		NodeProxyID:   request.NodeProxyID,
		Permissions:   permissions,
		ChangedFields: request.ChangedFields,
		BaseURL:       request.BaseURL,
		Initiator:     request.Initiator,
		// 目标代理与节点事实（评审 P1-3）：目标组件按这些字段组装最终流程载荷。
		FormProxyID:                  request.FormProxyID,
		FlowTemplateID:               request.FlowTemplateID,
		FlowProxyID:                  request.FlowProxyID,
		SelectFlowType:               request.SelectFlowType,
		FlowNodeType:                 request.FlowNodeType,
		NextNodeProxyID:              request.NextNodeProxyID,
		FlowInstanceBizRelevanceList: request.BizRelevance,
		// 转发事实（评审 P1-4）：装载阶段缺接收人直接失败，不再等到页面动作抛错。
		ReceiverID:       request.ReceiverID,
		ReceiverName:     request.ReceiverName,
		ForwardCheckList: request.ForwardCheckList,
	}, request.Values)

	if err != nil {
		return ActionRunResult{}, fmt.Errorf("装载载荷构造失败：%w", err)
	}
	// 装载信封必须携带全部协议编号：EncodeLoadMessage 产出完整信封后，
	// 这里按信封原文解码回 Message 再交给 Load 统一复核——绝不能把信封塞进
	// 外层 Payload 二次包装，否则外层缺编号会被协议校验直接拒绝。
	loadEnvelope, err := EncodeLoadMessage(handle.SessionID, fmt.Sprintf("load-%d", time.Now().UnixNano()), request.RunID, request.PathRunID, request.StepID, loadPayload)
	if err != nil {
		return ActionRunResult{}, err
	}
	var loadMessage Message
	if err := json.Unmarshal(loadEnvelope, &loadMessage); err != nil {
		return ActionRunResult{}, fmt.Errorf("装载信封解析失败：%w", err)
	}
	if _, err := r.client.Load(ctx, handle.SessionID, loadMessage); err != nil {
		return ActionRunResult{}, fmt.Errorf("runtime 装载失败：%w", err)
	}

	// ② 业务命令 + 选择闭环：仅三种明确选择错误允许继续一次。
	choiceSent := false
	// writeTraces 收集页面写请求经 Go 转发的传输层 trace：证据与尝试行 trace 的唯一来源。
	writeTraces := []string{}
	callbacks := ExecuteCallbacks{
		OnWriteTrace: func(traceID string) {
			if strings.TrimSpace(traceID) != "" {
				writeTraces = append(writeTraces, strings.TrimSpace(traceID))
			}
		},
		OnChoiceRequired: func(eventData json.RawMessage) *Message {
			if choiceSent {
				return nil
			}
			choice := r.buildChoice(request, eventData)
			if choice == nil {
				return nil
			}
			choiceSent = true
			choicePayload, _ := json.Marshal(map[string]any{
				"action": "choice_continue",
				"choice": choice,
			})
			return &Message{
				Kind:      KindExecute,
				RequestID: fmt.Sprintf("choice-%d", time.Now().UnixNano()),
				SessionID: handle.SessionID,
				RunID:     request.RunID,
				PathRunID: request.PathRunID,
				StepID:    request.StepID,
				Payload:   choicePayload,
			}
		},
	}
	actionPayload, err := json.Marshal(map[string]any{
		"action":           normalizeAction(request.Action),
		"instanceId":       request.InstanceID,
		"jobTaskId":        request.JobTaskID,
		"nodeProxyId":      request.NodeProxyID,
		"auditDesc":        request.AuditDesc,
		"branchSelections": branchSelectionMap(request.BranchSelections),
		"nextNodeAuditors": request.NextNodeAuditors,
	})
	if err != nil {
		return ActionRunResult{}, err
	}
	result, err := r.client.Execute(ctx, handle.SessionID, request.Account, Message{
		Kind:      KindExecute,
		RequestID: fmt.Sprintf("exec-%d", time.Now().UnixNano()),
		SessionID: handle.SessionID,
		RunID:     request.RunID,
		PathRunID: request.PathRunID,
		StepID:    request.StepID,
		Payload:   actionPayload,
	}, callbacks)
	if err != nil {
		// 连接断开等错误同样带回已收集的写请求 trace：不确定判定的对账与登记证据不能丢。
		return ActionRunResult{WriteTraceIDs: writeTraces}, err
	}
	var payload ActionRunResult
	if err := json.Unmarshal(result.Payload, &payload); err != nil {
		return ActionRunResult{}, fmt.Errorf("结果载荷解析失败：%w", err)
	}
	payload.WriteTraceIDs = writeTraces
	payload.NewInstanceID = extractNewInstanceID(payload.BusinessSummary)
	return payload, nil
}

// extractNewInstanceID 从 runtime 回传的业务摘要中取新发起实例 ID：
// 写发生在浏览器里，发起链的实例 ID 只能来自写响应体；引擎核验与后续步骤都依赖它。
func extractNewInstanceID(summary json.RawMessage) string {
	if len(summary) == 0 {
		return ""
	}
	var parsed struct {
		NewInstanceID string `json:"newInstanceId"`
	}
	if err := json.Unmarshal(summary, &parsed); err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.NewInstanceID)
}

// BuildChoiceForTest 暴露选择闭环的继续载荷构造，供单元测试锁定严格校验口径（评审 P1-6）。
func (r *RuntimeExecutor) BuildChoiceForTest(request RuntimeRunRequest, eventData json.RawMessage) map[string]any {
	return r.buildChoice(request, eventData)
}

// buildChoice 按目标返回的选择要求匹配已配置数据，构造继续命令载荷。
// 匹配不到已配置分支/人员时返回 nil：执行器据此阻塞（绝不猜测分支或用空人员）。
func (r *RuntimeExecutor) buildChoice(request RuntimeRunRequest, eventData json.RawMessage) map[string]any {
	var data struct {
		ErrorType string `json:"errorType"`
		Data      struct {
			ErrorType   string `json:"errorType"`
			BranchNodes []struct {
				ID       string `json:"id"`
				NodeName string `json:"nodeName"`
			} `json:"branchNodes"`
			Node struct {
				ID string `json:"id"`
			} `json:"node"`
		} `json:"data"`
	}
	if err := json.Unmarshal(eventData, &data); err != nil {
		return nil
	}
	errorType := data.Data.ErrorType
	if errorType == "" {
		errorType = data.ErrorType
	}
	switch errorType {
	case "custom_choose":
		// 分支：必须找到与已保存分支入口完全相同的一项（入口归属已在发送前按真实图边校验）。
		// 评审 P1-6：自选分支没有已配置人员时绝不继续——目标返回的临时候选不能替代配置。
		for _, node := range data.Data.BranchNodes {
			for _, selection := range request.BranchSelections {
				if selection.EntryNodeID != "" && selection.EntryNodeID == node.ID {
					if !branchPersonnelSatisfied(request, node.ID) {
						return nil
					}
					return map[string]any{
						"kind":                    "branch",
						"branchEntryNodeProxyIds": []string{node.ID},
						"nextAuditors":            auditorsForNode(request, node.ID),
					}
				}
			}
		}
		return nil
	case "parallel_choose":
		// 并行：逐个分支携带各自节点人员。评审 P1-6：任一自选分支缺已配置人员时整体放弃继续
		//（由执行器阻塞），绝不发送不完整的 nextAuditors，也绝不用目标临时节点集合覆盖已保存配置。
		entries := []string{}
		auditors := []NextAuditorEntry{}
		for _, node := range data.Data.BranchNodes {
			if !branchPersonnelSatisfied(request, node.ID) {
				return nil
			}
			entries = append(entries, node.ID)
			auditors = append(auditors, auditorsForNode(request, node.ID)...)
		}
		if len(entries) == 0 {
			return nil
		}
		return map[string]any{
			"kind":                    "parallel",
			"branchEntryNodeProxyIds": entries,
			"nextAuditors":            auditors,
		}
	case "run_node_choose":
		// 自选人员：按返回节点 ID 匹配已配置人员。
		nodeID := data.Data.Node.ID
		if nodeID == "" {
			return nil
		}
		auditors := auditorsForNode(request, nodeID)
		if len(auditors) == 0 {
			return nil
		}
		return map[string]any{
			"kind":         "auditor",
			"nextAuditors": auditors,
		}
	default:
		return nil
	}
}

// auditTypeRunNodeChoose 是目标「审批人自选」审批方式：这类分支必须由工具显式携带人员。
const auditTypeRunNodeChoose = "run_node_choose"

// branchPersonnelSatisfied 校验分支入口的人员配置：审批方式为自选（run_node_choose）的
// 分支必须携带至少一条已配置人员；其余审批方式由目标按节点配置自行解析，不要求页面选人。
func branchPersonnelSatisfied(request RuntimeRunRequest, entryNodeID string) bool {
	if request.BranchNodeAuditTypes[entryNodeID] != auditTypeRunNodeChoose {
		return true
	}
	return len(auditorsForNode(request, entryNodeID)) > 0
}

// auditorsForNode 取某节点已配置的人员；空人员绝不放行。
func auditorsForNode(request RuntimeRunRequest, nodeProxyID string) []NextAuditorEntry {
	result := []NextAuditorEntry{}
	for _, auditor := range request.NextNodeAuditors {
		if auditor.NodeProxyID == nodeProxyID && auditor.BizID != "" && auditor.Name != "" {
			result = append(result, auditor)
		}
	}
	return result
}

// branchSelectionMap 把分支选择转成协议键值形状。
func branchSelectionMap(selections []BranchSelection) map[string]string {
	result := map[string]string{}
	for _, selection := range selections {
		if selection.NodeProxyID != "" && selection.EntryNodeID != "" {
			result[selection.NodeProxyID] = selection.EntryNodeID
		}
	}
	return result
}

// normalizeAction 返回去除空白的动作键。
func normalizeAction(action string) string {
	return strings.TrimSpace(action)
}
