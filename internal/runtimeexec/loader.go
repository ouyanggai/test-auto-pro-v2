// T008 发起表单装载：构造 load 命令载荷的 Go 侧契约。
// 载荷字段与页面侧 App.vue 的 load 命令消费严格对齐（renderType/template/values/permissions/identity），
// 身份字段只来自 Go 当前会话，浏览器不能提交或覆盖（docs/s/01-执行引擎设计.md 5.2）。
// SID 由 Go 在 Load 调用前注入载荷，页面从载荷读取后写入 localStorage 供 axios 使用。
package runtimeexec

import (
	"encoding/json"
	"fmt"
	"strings"
)

// 装载模式：发起装载创建发起表单组件；审批装载（T009）复用同一形状并注入两套身份；
// 转发装载（P1-4 修复）复用审批宿主并注入转发弹窗等价的行事实与接收人；
// edit=重提/待发编辑（isReInitiate），view=只读核对（operaType=check）。
const (
	LoadModeInitiate = "initiate"
	LoadModeApprove  = "approve"
	LoadModeForward  = "forward"
	LoadModeEdit     = "edit"
	LoadModeView     = "view"
)

// SessionFacts 是装载需要的会话事实子集，由调用方从 target.Session 投影；SID 不在其中。
type SessionFacts struct {
	UserID       string
	CompanyID    string
	DepartmentID string
	CustomerCode string
	Account      string
	DisplayName  string
}

// LoadPayload 是 load 命令的载荷形状。
type LoadPayload struct {
	// Mode 取 initiate 或 approve。
	Mode string `json:"mode"`
	// Values 是当前节点允许覆盖的配置补丁，不是完整渲染模板。
	Values json.RawMessage `json:"values,omitempty"`
	// Permissions 是当前节点可编辑字段键列表；空数组表示由原生页面权限决定。
	Permissions []string `json:"permissions"`
	// ChangedFields 是补丁触及的字段：等待选项及联动完成后再回读。
	ChangedFields []string `json:"changedFields"`
	// ToolFlowName 是工具流程名称：最终流程请求中目标脚本不得改写。
	ToolFlowName string `json:"toolFlowName"`
	// InstanceID 是主实例 ID；新建草稿/新发起时为空。
	InstanceID string `json:"instanceId,omitempty"`
	// JobTaskID 是当前待办任务 ID、NodeProxyID 是当前节点代理 ID：审批装载的硬事实（T009）。
	JobTaskID   string `json:"jobTaskId,omitempty"`
	NodeProxyID string `json:"nodeProxyId,omitempty"`
	// Initiator / CurrentOperator 分开保存发起人与当前操作人，禁止当前人覆盖发起人字段。
	Initiator       ActorIdentity `json:"initiator"`
	CurrentOperator ActorIdentity `json:"currentOperator"`
	CompanyID       string        `json:"companyId,omitempty"`
	CompanyName     string        `json:"companyName,omitempty"`
	DepartmentID    string        `json:"departmentId,omitempty"`
	DepartmentName  string        `json:"departmentName,omitempty"`
	CustomerCode    string        `json:"customerCode,omitempty"`
	UserID          string        `json:"userId,omitempty"`
	AccountName     string        `json:"accountName,omitempty"`
	SID             string        `json:"sid,omitempty"`
	PlatformCode    string        `json:"platformCode,omitempty"`
	BaseURL         string        `json:"baseURL,omitempty"`
	// FormProxyID 是 FormMaking 表单代理 ID：FlowDialog 据 formId 决定发送 formProxyId；
	// 缺失时目标提交会退回发送 flowProxyId: ""（评审 P1-3 的空代理 ID 根因）。
	FormProxyID string `json:"formProxyId,omitempty"`
	// FlowTemplateID 是流程模板 ID（/web/flowTemplateApi/list 返回的 id）：发起装载专字段。
	// 语义独立的第三种 ID——不得用表单代理 ID 或发布流程代理 ID 顶替（P1-2 根因）。
	FlowTemplateID string `json:"flowTemplateId,omitempty"`
	// FlowProxyID 是发布流程代理 ID：审批组件与转发链的流程归属事实。
	FlowProxyID string `json:"flowProxyId,omitempty"`
	// SelectFlowType 是流程类型（auditWay）：目标组件按它组装业务关联与表单业务分支。
	SelectFlowType string `json:"selectFlowType,omitempty"`
	// FlowNodeType 是当前节点审批方式：审批组件据它判定选人入口。
	FlowNodeType string `json:"flowNodeType,omitempty"`
	// NextNodeProxyID 是本次流转下一节点的真实代理 ID：审批 nextAuditorList 缺省归属兜底。
	NextNodeProxyID string `json:"nextNodeProxyId,omitempty"`
	// FlowInstanceBizRelevanceList 是实例当前业务关联：审批与转发沿用目标事实，不凭空重建。
	FlowInstanceBizRelevanceList []BizRelevance `json:"flowInstanceBizRelevanceList,omitempty"`
	// —— 转发事实（P1-4）：等价于用户在转发弹窗中的输入 ——
	// ReceiverID/ReceiverName 是转发接收人：缺失必须在装载前被拒绝（空接收人绝不下发）。
	ReceiverID   string `json:"receiverId,omitempty"`
	ReceiverName string `json:"receiverName,omitempty"`
	// ForwardCheckList 是转发勾选项（fuyan=附言 / yijian=流程日志）。
	ForwardCheckList []string `json:"forwardCheckList,omitempty"`
	// ForwardTab 是原生列表页标签：待办=backlog，待发=dueout。
	ForwardTab string `json:"forwardTab,omitempty"`
}

// BizRelevance 是一条实例业务关联（与目标载荷 {otherBiz, otherBizId} 同形）。
type BizRelevance struct {
	OtherBiz   string `json:"otherBiz"`
	OtherBizID string `json:"otherBizId"`
}

// ActorIdentity 是一个业务身份的最小事实。
type ActorIdentity struct {
	UserID string `json:"userId,omitempty"`
	Name   string `json:"name,omitempty"`
}

// LoadInput 汇总装载所需的调用方事实。
type LoadInput struct {
	Mode          string
	Session       SessionFacts
	ToolFlowName  string
	InstanceID    string
	JobTaskID     string
	NodeProxyID   string
	Permissions   []string
	ChangedFields []string
	BaseURL       string
	Initiator     ActorIdentity
	// —— 目标代理与节点事实（评审 P1-3）——
	FormProxyID    string
	FlowProxyID    string
	SelectFlowType string
	// FlowTemplateID 是计划绑定的流程模板 ID（新发起来源=计划目标对象 ID）：发起装载必填。
	FlowTemplateID               string
	FlowNodeType                 string
	NextNodeProxyID              string
	FlowInstanceBizRelevanceList []BizRelevance
	// —— 转发事实（评审 P1-4）——
	ReceiverID       string
	ReceiverName     string
	ForwardCheckList []string
	ForwardTab       string
}

// BuildLoadPayload 构造并校验装载载荷。v2 不再接受 Go 渲染模板；
// 发起必须带 auditWay 与计划绑定 flowTemplateId，实例必须带 taskContext；
// edit（重提/待发编辑）必须带实例 ID（P1-3：重提走 /flow/detail 的 edit 模式）。
func BuildLoadPayload(input LoadInput, values json.RawMessage) (*LoadPayload, error) {
	if input.Mode != LoadModeInitiate && input.Mode != LoadModeApprove && input.Mode != LoadModeForward && input.Mode != LoadModeEdit && input.Mode != LoadModeView {
		return nil, fmt.Errorf("装载模式只支持 initiate/approve/forward/edit/view：%s", input.Mode)
	}
	if strings.TrimSpace(input.ToolFlowName) == "" {
		return nil, fmt.Errorf("装载缺少工具流程名称")
	}
	if input.Mode == LoadModeInitiate && strings.TrimSpace(input.SelectFlowType) == "" {
		return nil, fmt.Errorf("发起装载缺少 auditWay，不能只传 flowTemplateId")
	}
	if input.Mode == LoadModeInitiate && strings.TrimSpace(input.FlowTemplateID) == "" {
		return nil, fmt.Errorf("发起装载缺少计划绑定模板 ID（flowTemplateId）")
	}
	if input.Mode != LoadModeInitiate && strings.TrimSpace(input.InstanceID) == "" {
		return nil, fmt.Errorf("实例装载模式 %s 缺少实例 ID", input.Mode)
	}
	if (input.Mode == LoadModeApprove || input.Mode == LoadModeForward) && strings.TrimSpace(input.JobTaskID) == "" {
		return nil, fmt.Errorf("审批/转发装载缺少当前任务 ID")
	}
	if len(values) == 0 {
		values = json.RawMessage(`{}`)
	}
	if !json.Valid(values) {
		return nil, fmt.Errorf("当前节点数据补丁不是合法 JSON")
	}
	current := ActorIdentity{
		UserID: strings.TrimSpace(input.Session.UserID),
		Name:   strings.TrimSpace(input.Session.DisplayName),
	}
	if input.Mode == LoadModeInitiate && current.UserID == "" {
		return nil, fmt.Errorf("装载缺少当前账号用户事实")
	}
	if input.Mode == LoadModeForward {
		if strings.TrimSpace(input.ReceiverID) == "" {
			return nil, fmt.Errorf("转发装载缺少接收人，拒绝进入目标页面")
		}
	}
	permissions := input.Permissions
	if permissions == nil {
		permissions = []string{}
	}
	changed := input.ChangedFields
	if changed == nil {
		changed = []string{}
	}
	relevance := input.FlowInstanceBizRelevanceList
	if relevance == nil {
		relevance = []BizRelevance{}
	}
	checkList := input.ForwardCheckList
	if checkList == nil {
		checkList = []string{}
	}
	return &LoadPayload{
		Mode:                         input.Mode,
		Values:                       values,
		Permissions:                  permissions,
		ChangedFields:                changed,
		ToolFlowName:                 strings.TrimSpace(input.ToolFlowName),
		InstanceID:                   strings.TrimSpace(input.InstanceID),
		JobTaskID:                    strings.TrimSpace(input.JobTaskID),
		NodeProxyID:                  strings.TrimSpace(input.NodeProxyID),
		Initiator:                    input.Initiator,
		CurrentOperator:              current,
		CompanyID:                    strings.TrimSpace(input.Session.CompanyID),
		DepartmentID:                 strings.TrimSpace(input.Session.DepartmentID),
		CustomerCode:                 strings.TrimSpace(input.Session.CustomerCode),
		UserID:                       current.UserID,
		AccountName:                  strings.TrimSpace(input.Session.Account),
		BaseURL:                      strings.TrimRight(input.BaseURL, "/"),
		FormProxyID:                  strings.TrimSpace(input.FormProxyID),
		FlowTemplateID:               strings.TrimSpace(input.FlowTemplateID),
		FlowProxyID:                  strings.TrimSpace(input.FlowProxyID),
		SelectFlowType:               strings.TrimSpace(input.SelectFlowType),
		FlowNodeType:                 strings.TrimSpace(input.FlowNodeType),
		NextNodeProxyID:              strings.TrimSpace(input.NextNodeProxyID),
		FlowInstanceBizRelevanceList: relevance,
		ReceiverID:                   strings.TrimSpace(input.ReceiverID),
		ReceiverName:                 strings.TrimSpace(input.ReceiverName),
		ForwardCheckList:             checkList,
		ForwardTab:                   strings.TrimSpace(input.ForwardTab),
	}, nil
}

// EncodeLoadMessage 把装载载荷包成 load 协议信封。
func EncodeLoadMessage(sessionID, requestID, runID, pathRunID, stepID string, payload *LoadPayload) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("装载载荷编码失败：%w", err)
	}
	return EncodeMessage(Message{
		Version:   ProtocolVersion,
		Kind:      KindLoad,
		RequestID: requestID,
		SessionID: sessionID,
		RunID:     runID,
		PathRunID: pathRunID,
		StepID:    stepID,
		Payload:   raw,
	})

}
