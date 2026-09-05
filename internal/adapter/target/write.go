package target

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// 写端点白名单（F-016）：本切片允许调用的目标写端点只有以下两个，
// 其余写端点由运行前检查（F-015）直接阻塞，不得静默降级。
const (
	// WriteEndpointSubmit 发起主实例（动作：发起；无表单流程的保存草稿同端点但本切片不使用）。
	WriteEndpointSubmit = "/web/flowInstanceApi/submit"
	// WriteEndpointAudit 处理当前活动人工待办（动作：同意）。
	WriteEndpointAudit = "/flowInstanceApi/audit"
)

// WriteResponse 是写请求收到完整响应时的事实摘要，供判定包使用。
// isSuccess 显式存在与否语义不同：缺失说明成功判据不存在，只能落不可解释失败（F-014 第 1.6 节）。
type WriteResponse struct {
	StatusCode       int
	IsSuccess        bool
	IsSuccessPresent bool
	Code             string
	Message          string
}

// BusinessRejection 是目标业务包络的失败响应（isSuccess=false）。
// 必须原样携带 code 与 message，供判定包按「端点+精确文案」做清单匹配，禁止模糊化或改写。
type BusinessRejection struct {
	Code    string
	Message string
}

// Error 返回稳定的中文分类文案；真实文案只进判定输入，不进错误链展示。
func (e *BusinessRejection) Error() string {
	return "目标平台业务拒绝"
}

// NextAuditor 对应目标 FlowNodeAuditDetailConfigTemplateVo：分支选人与下一节点选人的最小结构。
// 只承载目标协议要求的字段；bizId 是真实处理人的目标用户 ID。
type NextAuditor struct {
	Name           string `json:"name,omitempty"`
	BizID          string `json:"bizId,omitempty"`
	AuditDetailTyp string `json:"auditDetailType,omitempty"`
	NodeProxyID    string `json:"nodeProxyId,omitempty"`
}

// BizRelevance 对应目标的 flowInstanceBizRelevanceList 元素：业务关联（公司、项目等）。
type BizRelevance struct {
	OtherBiz   string `json:"otherBiz,omitempty"`
	OtherBizID string `json:"otherBizId,omitempty"`
}

// SubmitFlowInstanceRequest 是发起主实例的语义意图。执行器给出意图，适配层负责协议。
type SubmitFlowInstanceRequest struct {
	// Name 是实例显示名；目标已发列表按名称展示。
	Name string
	// FlowProxyID 是发布流程代理 ID；FormProxyID 是表单代理 ID。
	// 参考实现要求二者至少有一个（有表单传 formProxyId，无表单传 flowProxyId）。
	FlowProxyID string
	FormProxyID string
	// CompanyID 是目标公司 ID（发起人所属公司上下文）。
	CompanyID string
	// FixedExecuteNodeID 是手动条件分支所选的目标节点 ID（FlowInstanceProtocol.fixedExecuteNodeId）。
	FixedExecuteNodeID string
	// FormData 是表单数据容器 formDataMongoVo.data 的原始 JSON。
	// 必须按原始文本透传，不得重新序列化（避免数字字面量被改写）。
	FormData json.RawMessage
	// BizRelevance 是业务关联列表，可空。
	BizRelevance []BizRelevance
	// NextAuditors 是分支选择/下一节点选人，可空。
	NextAuditors []NextAuditor
}

// SubmitFlowInstanceResult 是发起返回的目标事实摘要，供核验重读与运行记录使用。
type SubmitFlowInstanceResult struct {
	InstanceID          string
	Status              string
	CurrentNodeProxyID  string
	BatchNo             string
	CurrentNodeProxyIDs []string
}

// BuildSubmitBody 构造发起请求的协议载荷（不含 SID 等会话敏感信息，会话由唯一出口注入）。
// 导出是为了让执行器的「即将发出的请求」预览与实际发出的载荷严格同源，不允许两套拼装逻辑。
func BuildSubmitBody(request SubmitFlowInstanceRequest) map[string]any {
	data := map[string]any{}
	if name := strings.TrimSpace(request.Name); name != "" {
		data["name"] = name
	}
	if id := strings.TrimSpace(request.FormProxyID); id != "" {
		data["formProxyId"] = id
	}
	if id := strings.TrimSpace(request.FlowProxyID); id != "" {
		data["flowProxyId"] = id
	}
	if id := strings.TrimSpace(request.CompanyID); id != "" {
		data["companyId"] = id
	}
	if len(request.BizRelevance) > 0 {
		data["flowInstanceBizRelevanceList"] = request.BizRelevance
	}
	body := map[string]any{"data": data}
	if id := strings.TrimSpace(request.FixedExecuteNodeID); id != "" {
		// 条件分支手动指定节点是协议的顶层字段，不属于 data 容器（FlowInstanceProtocol:47）。
		body["fixedExecuteNodeId"] = id
	}
	if len(request.FormData) > 0 {
		body["formDataMongoVo"] = map[string]any{"data": request.FormData}
	}
	if len(request.NextAuditors) > 0 {
		body["nextAuditorList"] = request.NextAuditors
	}
	return body
}

// SubmitFlowInstance 发起主实例。会话属于发起人；本方法内部不做任何重试。
// 返回值按判定包需要分层：传输/解析失败经 error，完整响应事实经 WriteResponse，
// 业务失败以 BusinessRejection 原样携带 code 与 message。
func (c *Client) SubmitFlowInstance(ctx context.Context, session Session, request SubmitFlowInstanceRequest) (*SubmitFlowInstanceResult, WriteResponse, string, error) {
	body := BuildSubmitBody(request)
	envelope, traceID, err := c.CallWrite(ctx, WriteEndpointSubmit, session.SID, body)
	response := WriteResponse{}
	if err != nil {
		// 传输层失败没有可信状态码；只有“完整响应被拒收”的少数错误才带 HTTP 状态。
		if targetErr := asError(err); targetErr != nil {
			response.StatusCode = targetErr.HTTPStatus
		}
		return nil, response, traceID, err
	}
	response.StatusCode = http.StatusOK
	response.IsSuccessPresent = true
	response.IsSuccess = responseSucceeded(envelope)
	response.Code = envelope.Code
	response.Message = envelope.Message
	if !responseSucceeded(envelope) {
		return nil, response, traceID, &BusinessRejection{Code: envelope.Code, Message: envelope.Message}
	}
	result := &SubmitFlowInstanceResult{}
	var raw struct {
		ID                   string          `json:"id"`
		Status               string          `json:"status"`
		CurrentNodeProxyID   string          `json:"currentNodeProxyId"`
		BatchNo              string          `json:"batchNo"`
		CurrentAuditUserInfo json.RawMessage `json:"currentAuditUserInfo"`
	}
	if err := json.Unmarshal(envelope.Data, &raw); err != nil {
		// 发起已受理但响应结构不符合预期：如实作为响应异常上抛，由判定包按不可解释处理。
		return nil, response, traceID, invalidResponse("submit response is not a flow instance")
	}
	result.InstanceID = strings.TrimSpace(raw.ID)
	result.Status = strings.TrimSpace(raw.Status)
	result.CurrentNodeProxyID = strings.TrimSpace(raw.CurrentNodeProxyID)
	result.BatchNo = strings.TrimSpace(raw.BatchNo)
	// 活动节点集合优先于单一 currentNodeProxyId，避免并行入口被压缩成一个节点。
	result.CurrentNodeProxyIDs = auditNodeIDs(raw.CurrentAuditUserInfo)
	if len(result.CurrentNodeProxyIDs) == 0 && result.CurrentNodeProxyID != "" {
		result.CurrentNodeProxyIDs = []string{result.CurrentNodeProxyID}
	}
	return result, response, traceID, nil
}

// AuditCurrentTaskRequest 是处理当前活动人工待办（同意）的语义意图。
type AuditCurrentTaskRequest struct {
	// InstanceID 是主实例 ID（data.id）。
	InstanceID string
	// JobTaskID 是待办任务链接 ID，目标的硬性必填项。
	JobTaskID string
	// FlowProxyID 是发布流程代理 ID，目标用于取代理上下文。
	FlowProxyID string
	// AuditStatus 是目标 ExecuteResultEnum 的编码名；同意为 pass。
	AuditStatus string
	// ExecuteDesc 是审批意见（目标字段“处理备注”）。
	ExecuteDesc string
	// FormData 是分支判断字段/表单数据的原始 JSON，可空。
	FormData json.RawMessage
	// NextAuditors 是分支选择/下一节点选人，可空。
	NextAuditors []NextAuditor
}

// AuditCurrentTaskResult 是审批返回的目标事实摘要。
type AuditCurrentTaskResult struct {
	InstanceID string
	Status     string
	BatchNo    string
}

// BuildAuditBody 构造审批请求的协议载荷（不含会话敏感信息）。与预览严格同源，规则同 BuildSubmitBody。
func BuildAuditBody(request AuditCurrentTaskRequest) map[string]any {
	data := map[string]any{
		"id":        strings.TrimSpace(request.InstanceID),
		"jobTaskId": strings.TrimSpace(request.JobTaskID),
	}
	if id := strings.TrimSpace(request.FlowProxyID); id != "" {
		data["flowProxyId"] = id
	}
	auditRecord := map[string]any{
		"auditStatus": strings.TrimSpace(request.AuditStatus),
	}
	if desc := strings.TrimSpace(request.ExecuteDesc); desc != "" {
		auditRecord["executeDesc"] = desc
	}
	data["auditRecord"] = auditRecord
	body := map[string]any{"data": data}
	if len(request.FormData) > 0 {
		body["formDataMongoVo"] = map[string]any{"data": request.FormData}
	}
	if len(request.NextAuditors) > 0 {
		body["nextAuditorList"] = request.NextAuditors
	}
	return body
}

// AuditCurrentTask 处理当前活动人工待办。会话属于持待办的真实处理人；本方法内部不做任何重试。
func (c *Client) AuditCurrentTask(ctx context.Context, session Session, request AuditCurrentTaskRequest) (*AuditCurrentTaskResult, WriteResponse, string, error) {
	body := BuildAuditBody(request)
	envelope, traceID, err := c.CallWrite(ctx, WriteEndpointAudit, session.SID, body)
	// 传输失败时响应事实必须保持零值：连接被拒时伪造 200 会让判定包看到
	// 「声明没有收到响应却带回状态码」的矛盾，把可判确定失败的抖动升级成待对账。
	response := WriteResponse{}
	if err != nil {
		// 传输层失败没有可信状态码；只有“完整响应被拒收”的少数错误才带 HTTP 状态。
		if targetErr := asError(err); targetErr != nil && targetErr.Transport == TransportResponded {
			response.StatusCode = targetErr.HTTPStatus
		}
		return nil, response, traceID, err
	}
	response.StatusCode = http.StatusOK
	response.IsSuccessPresent = true
	response.IsSuccess = responseSucceeded(envelope)
	response.Code = envelope.Code
	response.Message = envelope.Message
	if !responseSucceeded(envelope) {
		return nil, response, traceID, &BusinessRejection{Code: envelope.Code, Message: envelope.Message}
	}
	result := &AuditCurrentTaskResult{}
	var raw struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		BatchNo string `json:"batchNo"`
	}
	if err := json.Unmarshal(envelope.Data, &raw); err != nil {
		// 审批已受理但响应结构不符合预期：如实作为响应异常上抛，由判定包按不可解释处理。
		return nil, response, traceID, invalidResponse("audit response is not a flow instance")
	}
	result.InstanceID, result.Status, result.BatchNo = strings.TrimSpace(raw.ID), strings.TrimSpace(raw.Status), strings.TrimSpace(raw.BatchNo)
	return result, response, traceID, nil
}
