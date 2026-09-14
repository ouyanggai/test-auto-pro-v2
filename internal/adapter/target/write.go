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
	// WriteEndpointSubmit 发起主实例；保存草稿也使用该目标端点，但通过 data.status 区分语义。
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
	// Data 是目标成功响应的原始 data，供 updateFlowProxy 读取新代理和当前节点标识。
	Data json.RawMessage
}

// BusinessRejection 是目标业务包络的失败响应（isSuccess=false）。
// 必须原样携带 code 与 message，供页面直接展示和判定包按「端点+精确文案」做清单匹配，禁止模糊化或改写。
type BusinessRejection struct {
	Code    string
	Message string
}

// Error 返回稳定的中文分类文案；真实文案只进判定输入，不进错误链展示。
func (e *BusinessRejection) Error() string {
	if e == nil {
		return "目标平台业务拒绝"
	}
	if message := strings.TrimSpace(e.Message); message != "" {
		return message
	}
	if code := strings.TrimSpace(e.Code); code != "" {
		return code
	}
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
	OtherBiz   string `json:"otherBiz"`
	OtherBizID string `json:"otherBizId"`
}

// SubmitFlowInstanceRequest 是发起主实例的语义意图。执行器给出意图，适配层负责协议。
type SubmitFlowInstanceRequest struct {
	// InstanceID 是已有草稿实例的目标实例 ID；新建提交时为空。
	InstanceID string
	// Name 是实例显示名；目标已发列表按名称展示。
	Name string
	// Status 是提交状态；保存草稿固定为 draft，普通提交为空。
	Status string
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
	// NextAuditors 是分支选择/下一节点选人，可空；submit 按目标页面固定发送数组（空时为 []）。
	NextAuditors []NextAuditor
	// BatchCode 是随 submit/draft 顶层发送的批次号（F-035）：目标 FlowDialog 打开时生成一次，
	// 随提交原样携带；不是工具幂等键，重试/对账不得以它为依据。
	BatchCode string
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
// 字段存在性按 F-035 协议矩阵（protocol_matrix.go）执行：
//   - formProxyId 与 flowProxyId 互斥：FormMaking 只发 formProxyId，无表单只发 flowProxyId
//     （FlowDialog 按 formId 是否为空二选一，禁止同时发送两个代理字段）；
//   - nextAuditorList 固定发送数组，无显式选人时是 []（目标页面无条件 map 出数组）；
//   - batchCode 仅 submit/draft 顶层携带；sid/projectId 由统一出口注入。
func BuildSubmitBody(request SubmitFlowInstanceRequest) map[string]any {
	data := map[string]any{}
	if id := strings.TrimSpace(request.InstanceID); id != "" {
		data["id"] = id
	}
	if name := strings.TrimSpace(request.Name); name != "" {
		data["name"] = name
	}
	if status := strings.TrimSpace(request.Status); status != "" {
		data["status"] = status
	}
	// 代理 ID 互斥：目标页面按「有表单用 formProxyId、无表单用 flowProxyId」构造，
	// 把发布流程代理 ID 当表单代理 ID 或两者同发都是工具缺陷（F-035 根因 C）。
	if id := strings.TrimSpace(request.FormProxyID); id != "" {
		data["formProxyId"] = id
	} else if id := strings.TrimSpace(request.FlowProxyID); id != "" {
		data["flowProxyId"] = id
	}
	if id := strings.TrimSpace(request.CompanyID); id != "" {
		data["companyId"] = id
	}
	// 业务关联固定发送数组：目标页面无条件构造列表（至少公司关联），空时也要保留数组形状。
	if len(request.BizRelevance) > 0 {
		data["flowInstanceBizRelevanceList"] = request.BizRelevance
	} else {
		data["flowInstanceBizRelevanceList"] = []BizRelevance{}
	}
	body := map[string]any{"data": data}
	if id := strings.TrimSpace(request.FixedExecuteNodeID); id != "" {
		// 条件分支手动指定节点是协议的顶层字段，不属于 data 容器（FlowInstanceProtocol:47）。
		body["fixedExecuteNodeId"] = id
	}
	// 目标服务在 submit/reSubmit/audit 内部都会读取 formDataMongoVo；即使表单为空也必须发送空对象，
	// 否则目标把缺少容器当成请求结构错误，而不是合法的空表单。
	formData := request.FormData
	if len(formData) == 0 {
		formData = json.RawMessage(`{}`)
	}
	body["formDataMongoVo"] = map[string]any{"data": formData}
	// nextAuditorList 固定发送数组：目标页面 2025.9.9 改为无条件 map（空列表也发送），
	// 人工成功 curl 已确认空数组形状（F-035 根因 A）。空值不能被解释为已解析出人员。
	auditors := request.NextAuditors
	if auditors == nil {
		auditors = []NextAuditor{}
	}
	body["nextAuditorList"] = auditors
	// batchCode 仅 submit/draft 顶层携带；空值不发送（调用方未生成说明走的不是页面同源路径）。
	if code := strings.TrimSpace(request.BatchCode); code != "" {
		body["batchCode"] = code
	}
	return body
}

// SubmitFlowInstance 发起主实例。会话属于发起人；本方法内部不做任何重试。
// 返回值按判定包需要分层：传输/解析失败经 error，完整响应事实经 WriteResponse，
// 业务失败以 BusinessRejection 原样携带 code 与 message。
func (c *Client) SubmitFlowInstance(ctx context.Context, session Session, request SubmitFlowInstanceRequest) (*SubmitFlowInstanceResult, WriteResponse, string, error) {
	body := BuildSubmitBody(request)
	finalBody, err := ValidateFinalWriteBody(WriteEndpointSubmit, session.SID, session.CustomerCode, body)
	if err != nil {
		return nil, WriteResponse{}, "", &RequestValidationError{Message: err.Error()}
	}
	body = finalBody
	envelope, traceID, err := c.CallWrite(ctx, WriteEndpointSubmit, session.SID, session.CustomerCode, body)
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
	// BizRelevance 是实例已有业务关联；审批保存时必须原样带回，避免目标覆盖后丢失业务归属。
	BizRelevance []BizRelevance
	// NextAuditors 是分支选择/下一节点选人，可空。
	NextAuditors []NextAuditor
	// Tracking 是审批完成后是否关注实例；目标前端会把它作为 audit 顶层字段发送。
	Tracking *bool
}

// AuditCurrentTaskResult 是审批返回的目标事实摘要。
type AuditCurrentTaskResult struct {
	InstanceID string
	Status     string
	BatchNo    string
}

// BuildAuditBody 构造审批请求的协议载荷（不含会话敏感信息）。与预览严格同源。
// F-035 矩阵（评审补充）：id/jobTaskId/auditRecord/formDataMongoVo/tracking 固定发送；
// flowProxyId 与 flowInstanceBizRelevanceList 按页面条件发送（无表单页直传关联；
// FormMaking 审批页只在公共流程/案件场景设置，初值不携带——空数组不是页面行为）。
// nextAuditorList 仅 pass 分支发送；batchCode 审批不发送。
func BuildAuditBody(request AuditCurrentTaskRequest) map[string]any {
	data := map[string]any{
		"id":        strings.TrimSpace(request.InstanceID),
		"jobTaskId": strings.TrimSpace(request.JobTaskID),
	}
	if id := strings.TrimSpace(request.FlowProxyID); id != "" {
		data["flowProxyId"] = id
	}
	if len(request.BizRelevance) > 0 {
		data["flowInstanceBizRelevanceList"] = request.BizRelevance
	}
	auditRecord := map[string]any{
		"auditStatus": strings.TrimSpace(request.AuditStatus),
	}
	if desc := strings.TrimSpace(request.ExecuteDesc); desc != "" {
		auditRecord["executeDesc"] = desc
	}
	data["auditRecord"] = auditRecord
	body := map[string]any{"data": data}
	formData := request.FormData
	if len(formData) == 0 {
		formData = json.RawMessage(`{}`)
	}
	body["formDataMongoVo"] = map[string]any{"data": formData}
	if len(request.NextAuditors) > 0 {
		body["nextAuditorList"] = request.NextAuditors
	}
	// tracking 是页面无条件携带的顶层布尔；nil 视为 false，不能因“没配置”省略字段。
	tracking := false
	if request.Tracking != nil {
		tracking = *request.Tracking
	}
	body["tracking"] = tracking
	return body
}

// AuditCurrentTask 处理当前活动人工待办。会话属于持待办的真实处理人；本方法内部不做任何重试。
func (c *Client) AuditCurrentTask(ctx context.Context, session Session, request AuditCurrentTaskRequest) (*AuditCurrentTaskResult, WriteResponse, string, error) {
	body := BuildAuditBody(request)
	finalBody, err := ValidateFinalWriteBody(WriteEndpointAudit, session.SID, session.CustomerCode, body)
	if err != nil {
		return nil, WriteResponse{}, "", &RequestValidationError{Message: err.Error()}
	}
	body = finalBody
	envelope, traceID, err := c.CallWrite(ctx, WriteEndpointAudit, session.SID, session.CustomerCode, body)
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
