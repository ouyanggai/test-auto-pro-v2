// Package runtimeexec 是 Go 侧与独立执行服务（无头 Chromium）通信的唯一实现。
// F-036 起协议权威定义迁到 executor-service/src/protocol.ts；T004 将同步升级为 exec-engine/v2。
// 当前仍与历史夹具锁定字段、动作全集、结果七值。
// 纪律：每条消息必须携带 requestId、runId、pathRunId、stepId；除 create 外还必须携带 sessionId；
// 旧协议版本直接拒绝，不静默兼容；runtime 不返回 Vue 实例，只返回 JSON 数据。
package runtimeexec

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ProtocolVersion 是协议版本；字段形状或语义发生不兼容变化时必须递增，并与 TS 侧同步。
const ProtocolVersion = "exec-engine/v2"

// 消息类型与唯一合法方向（Go 发出 / runtime 发出）。
const (
	KindCreate         = "create"          // Go 创建执行会话
	KindLoad           = "load"            // Go 装载身份、流程和表单
	KindExecute        = "execute"         // Go 执行业务命令
	KindRequestForward = "request_forward" // runtime 转发拦截到的目标请求
	KindRequestResp    = "request_response"
	KindEvent          = "event"  // runtime 推送事件（选择要求、页面异常、业务完成等）
	KindResult         = "result" // runtime 返回动作终局
	KindDestroy        = "destroy"
)

// messageDirections 记录每个消息类型的唯一合法方向；校验只认类型存在，方向由收发双方各自保证。
var messageDirections = map[string]string{
	KindCreate:         "go_to_runtime",
	KindLoad:           "go_to_runtime",
	KindExecute:        "go_to_runtime",
	KindRequestForward: "runtime_to_go",
	KindRequestResp:    "go_to_runtime",
	KindEvent:          "runtime_to_go",
	KindResult:         "runtime_to_go",
	KindDestroy:        "go_to_runtime",
}

// runtimeActions 是 execute 命令允许的动作全集：七个 runtime 业务动作 + 页面只读命令 + 选择闭环继续命令。
// add_sign/transfer/rollback_previous/retrieve/withdraw/urge/follow/unfollow 由 Go 目标客户端直连，
// 绝不允许出现在本协议中（docs/s/03-审批页面全量动作对照.md 第 2 节）。
var runtimeActions = map[string]bool{
	"load_initiate":     true,
	"load_approve":      true,
	"capture":           true,
	"validate":          true,
	"save_draft":        true,
	"submit":            true,
	"storage_form_data": true,
	"approve":           true,
	"reject":            true,
	"resubmit":          true,
	"forward":           true,
	"choice_continue":   true,
}

// 结果状态固定七值（与 TS 侧 RESULT_STATUSES 一致）：
// success 成功；failed 未发出流程写请求即失败；target_rejected 目标明确拒绝；
// uncertain 响应丢失/崩溃无法判断；business_failed 流程已推进但后置业务失败；
// unsupported 原生缺少入口或数据接口；timeout 动作超时。

const (
	ResultSuccess        = "success"
	ResultFailed         = "failed"
	ResultTargetRejected = "target_rejected"
	ResultUncertain      = "uncertain"
	ResultBusinessFailed = "business_failed"
	ResultUnsupported    = "unsupported"
	ResultTimeout        = "timeout"
)

var resultStatuses = map[string]bool{
	ResultSuccess:        true,
	ResultFailed:         true,
	ResultTargetRejected: true,
	ResultUncertain:      true,
	ResultBusinessFailed: true,
	ResultUnsupported:    true,
	ResultTimeout:        true,
}

// 事件子类型固定五种：choice_required 目标第一次调用返回明确选择错误；
// request_traced 一笔目标请求完成转发的只读摘要；page_error 页面异常；
// business_done 业务完成事件；log 阶段日志。
const (
	EventChoiceRequired = "choice_required"
	EventRequestTraced  = "request_traced"
	EventPageError      = "page_error"
	EventBusinessDone   = "business_done"
	EventLog            = "log"
)

var eventTypes = map[string]bool{
	EventChoiceRequired: true,
	EventRequestTraced:  true,
	EventPageError:      true,
	EventBusinessDone:   true,
	EventLog:            true,
}

// Message 是两侧通信的公共信封；Payload 按 Kind 决定形状，Go 侧保持 raw 以免二次序列化改写字段。
type Message struct {
	Version   string          `json:"version"`
	Kind      string          `json:"kind"`
	RequestID string          `json:"requestId"`
	SessionID string          `json:"sessionId"`
	RunID     string          `json:"runId"`
	PathRunID string          `json:"pathRunId"`
	StepID    string          `json:"stepId"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// ProtocolError 是协议校验失败的稳定错误；Code 与 TS 侧 ProtocolError.code 一致，供契约测试逐项断言。
type ProtocolError struct {
	Code    string
	Message string
}

// Error 返回中文校验原因。
func (e *ProtocolError) Error() string {
	if e == nil {
		return "协议校验失败"
	}
	return e.Message
}

// newProtocolError 构造带稳定错误码的协议错误。
func newProtocolError(code, format string, args ...any) *ProtocolError {
	return &ProtocolError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// ValidateMessage 校验信封与 kind 相关规则，规则与 TS 侧 validateMessage 完全一致：
// 先版本（旧版本绝不静默兼容），再 kind，再公共编号，最后 kind 专属载荷。
func ValidateMessage(raw []byte) (*Message, error) {
	var message Message
	if err := json.Unmarshal(raw, &message); err != nil {
		return nil, newProtocolError("message_not_object", "协议消息必须是 JSON 对象：%v", err)
	}
	if message.Version != ProtocolVersion {
		return nil, newProtocolError("version_rejected", "协议版本必须是 %s，收到 %s", ProtocolVersion, message.Version)
	}
	if messageDirections[message.Kind] == "" {
		return nil, newProtocolError("kind_unknown", "未知消息类型：%s", message.Kind)
	}
	if nonEmpty(message.RequestID) == "" {
		return nil, newProtocolError("id_missing", "消息缺少 requestId")
	}
	if nonEmpty(message.RunID) == "" {
		return nil, newProtocolError("id_missing", "消息缺少 runId")
	}
	if nonEmpty(message.PathRunID) == "" {
		return nil, newProtocolError("id_missing", "消息缺少 pathRunId")
	}
	if nonEmpty(message.StepID) == "" {
		return nil, newProtocolError("id_missing", "消息缺少 stepId")
	}
	if message.Kind != KindCreate && nonEmpty(message.SessionID) == "" {
		return nil, newProtocolError("session_missing", "%s 消息缺少 sessionId", message.Kind)
	}
	switch message.Kind {
	case KindLoad:
		var payload map[string]json.RawMessage
		if len(message.Payload) == 0 {
			return nil, newProtocolError("load_payload_invalid", "load 载荷必须是 JSON 对象")
		}
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			return nil, newProtocolError("load_payload_invalid", "load 载荷必须是 JSON 对象：%v", err)
		}
		if _, exists := payload["template"]; exists {
			return nil, newProtocolError("template_forbidden", "exec-engine/v2 装载不得携带 Go 渲染模板")
		}
		var mode string
		if raw, ok := payload["mode"]; ok {
			_ = json.Unmarshal(raw, &mode)
		}
		switch strings.TrimSpace(mode) {
		case "initiate", "audit", "edit", "view", "approve", "forward":
		default:
			return nil, newProtocolError("load_mode_invalid", "装载模式不支持：%s", mode)
		}
	case KindExecute:
		var payload struct {
			Action string `json:"action"`
		}
		if len(message.Payload) > 0 {
			if err := json.Unmarshal(message.Payload, &payload); err != nil {
				return nil, newProtocolError("action_unknown", "execute 载荷必须是 JSON 对象：%v", err)
			}
		}
		if !runtimeActions[payload.Action] {
			return nil, newProtocolError("action_unknown", "未知 runtime 动作：%s", payload.Action)
		}

	case KindResult:
		var payload struct {
			Status string `json:"status"`
		}
		if len(message.Payload) > 0 {
			if err := json.Unmarshal(message.Payload, &payload); err != nil {
				return nil, newProtocolError("status_invalid", "result 载荷必须是 JSON 对象：%v", err)
			}
		}
		if !resultStatuses[payload.Status] {
			return nil, newProtocolError("status_invalid", "结果状态必须是固定七值之一，收到 %s", payload.Status)
		}
	case KindEvent:
		var payload struct {
			Type string `json:"type"`
		}
		if len(message.Payload) > 0 {
			if err := json.Unmarshal(message.Payload, &payload); err != nil {
				return nil, newProtocolError("event_type_invalid", "event 载荷必须是 JSON 对象：%v", err)
			}
		}
		if !eventTypes[payload.Type] {
			return nil, newProtocolError("event_type_invalid", "事件子类型必须是 choice_required/request_traced/page_error/business_done/log")
		}
	}
	return &message, nil
}

// EncodeMessage 构造待发送消息的原始 JSON；调用方保证字段完整，发送前仍会走 ValidateMessage 复核。
func EncodeMessage(message Message) ([]byte, error) {
	message.Version = ProtocolVersion
	raw, err := json.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("协议消息编码失败：%w", err)
	}
	if _, err := ValidateMessage(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// nonEmpty 返回去除空白后的字符串；空白编号视为缺失。
func nonEmpty(value string) string {
	return strings.TrimSpace(value)
}

// IsProtocolError 判定错误是否为协议校验失败，供客户端把协议错误与传输错误分开上报。
func IsProtocolError(err error) bool {
	var protocolErr *ProtocolError
	return errors.As(err, &protocolErr)
}
