// T008 发起表单装载：构造 load 命令载荷的 Go 侧契约。
// 载荷字段与页面侧 App.vue 的 load 命令消费严格对齐（renderType/template/values/permissions/identity），
// 身份字段只来自 Go 当前会话，浏览器不能提交或覆盖（docs/s/01-执行引擎设计.md 5.2）。
// SID 绝不进入载荷：它由执行服务的会话现场持有，页面不发目标请求。
package runtimeexec

import (
	"encoding/json"
	"fmt"
	"strings"
)

// 装载模式：发起装载创建发起表单组件；审批装载（T009）复用同一形状并注入两套身份。
const (
	LoadModeInitiate = "initiate"
	LoadModeApprove  = "approve"
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
	// RenderType 固定 formmaking；vue_custom 无表单流程不走 runtime 装载。
	RenderType string `json:"renderType"`
	// Template 是目标表单代理的原始 FormMaking JSON（原样透传，不重排序）。
	Template json.RawMessage `json:"template"`
	// Values 是历史表单原始数据（深复制后的原文）。
	Values json.RawMessage `json:"values"`
	// Permissions 是当前节点可编辑字段键列表；空数组表示全只读口径由页面按模板决定。
	Permissions []string `json:"permissions"`
	// ChangedFields 是补丁触及的字段：触发选项协调与字段联动重放。
	ChangedFields []string `json:"changedFields"`
	// ToolFlowName 是工具流程名称：最终流程请求中目标脚本不得改写。
	ToolFlowName string `json:"toolFlowName"`
	// InstanceID 是主实例 ID；新建草稿/新发起时为空。
	InstanceID string `json:"instanceId,omitempty"`
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
	// BaseURL 是目标网关地址：页面据此构造目标请求（由 Go 拦截转发，绝不直连）。
	BaseURL string `json:"baseURL,omitempty"`
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
	Permissions   []string
	ChangedFields []string
	BaseURL       string
	Initiator     ActorIdentity
}

// BuildLoadPayload 构造并校验装载载荷；模板与表单数据必须是合法 JSON，工具流程名称必填。
func BuildLoadPayload(input LoadInput, template, values json.RawMessage) (*LoadPayload, error) {
	if input.Mode != LoadModeInitiate && input.Mode != LoadModeApprove {
		return nil, fmt.Errorf("装载模式只支持 initiate/approve：%s", input.Mode)
	}
	if strings.TrimSpace(input.ToolFlowName) == "" {
		return nil, fmt.Errorf("装载缺少工具流程名称")
	}
	if len(template) == 0 {
		return nil, fmt.Errorf("装载缺少目标表单模板")
	}
	if !json.Valid(template) {
		return nil, fmt.Errorf("目标表单模板不是合法 JSON")
	}
	if len(values) == 0 {
		values = json.RawMessage(`{}`)
	}
	if !json.Valid(values) {
		return nil, fmt.Errorf("历史表单数据不是合法 JSON")
	}
	current := ActorIdentity{
		UserID: strings.TrimSpace(input.Session.UserID),
		Name:   strings.TrimSpace(input.Session.DisplayName),
	}
	if input.Mode == LoadModeInitiate && current.UserID == "" {
		// 发起人就是当前操作人：缺失用户事实时必须阻止，否则身份字段替换会把空值写进表单。
		return nil, fmt.Errorf("装载缺少当前账号用户事实")
	}
	permissions := input.Permissions
	if permissions == nil {
		permissions = []string{}
	}
	changed := input.ChangedFields
	if changed == nil {
		changed = []string{}
	}
	return &LoadPayload{
		Mode:            input.Mode,
		RenderType:      "formmaking",
		Template:        template,
		Values:          values,
		Permissions:     permissions,
		ChangedFields:   changed,
		ToolFlowName:    strings.TrimSpace(input.ToolFlowName),
		InstanceID:      strings.TrimSpace(input.InstanceID),
		Initiator:       input.Initiator,
		CurrentOperator: current,
		CompanyID:       strings.TrimSpace(input.Session.CompanyID),
		DepartmentID:    strings.TrimSpace(input.Session.DepartmentID),
		CustomerCode:    strings.TrimSpace(input.Session.CustomerCode),
		UserID:          current.UserID,
		AccountName:     strings.TrimSpace(input.Session.Account),
		BaseURL:         strings.TrimRight(input.BaseURL, "/"),
	}, nil
}

// EncodeLoadMessage 把装载载荷包成 load 协议信封。
func EncodeLoadMessage(sessionID, requestID, runID, pathRunID, stepID string, payload *LoadPayload) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("装载载荷编码失败：%w", err)
	}
	return EncodeMessage(Message{
		Kind:      KindLoad,
		RequestID: requestID,
		SessionID: sessionID,
		RunID:     runID,
		PathRunID: pathRunID,
		StepID:    stepID,
		Payload:   raw,
	})
}
