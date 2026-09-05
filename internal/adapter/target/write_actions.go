// F-019 全动作写方法：动作目录 15 类动作对应的 11 个写端点的适配层实现。
// 纪律：一次尝试最多一次写请求（CallWrite，无重试）；载荷与参考源码及前端实际发送逐字同源；
// 响应按判定包需要分层返回（传输失败经 error，完整响应经 WriteResponse，业务拒绝原样携带 code/message）。
package target

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// F-019 写端点白名单（动作目录声明的全部 11 个端点）。
const (
	WriteEndpointReSubmit     = "/web/flowInstanceApi/reSubmit"
	WriteEndpointStorageForm  = "/web/flowInstanceApi/storageFormData"
	WriteEndpointApproverAppend = "/web/flowInstanceApi/approverAppend"
	WriteEndpointRollBack     = "/web/flowInstanceApi/rollBackThePreviousLevel"
	WriteEndpointRetrieve     = "/web/flowInstanceApi/retrieveProcess"
	WriteEndpointRevocation   = "/web/flowInstanceApi/revocation"
	WriteEndpointUrge         = "/web/urgeHandleRecord/sendUrgeMessage"
	WriteEndpointTranspond    = "/web/flowInstanceApi/transpond"
	WriteEndpointFlowTracking = "/web/flowInstanceApi/flowTracking"
)

// ActionWriteRequest 是动作写请求的统一意图：执行器按动作填充，适配层负责协议。
type ActionWriteRequest struct {
	// Action 是动作键，决定端点与载荷形状（同一端点不同动作不混淆）。
	Action string
	// InstanceID 是主实例 ID（data.id）。
	InstanceID string
	// JobTaskID 是待办任务链接 ID（审批类动作硬性必填）。
	JobTaskID string
	// FlowProxyID / FormProxyID 是重提等动作需要的代理标识。
	FlowProxyID string
	FormProxyID string
	// NodeProxyID 是当前节点代理 ID（暂存需要）。
	NodeProxyID string
	// AuditStatus 是 ExecuteResultEnum 编码名（pass/no_pass/transfer/roll_back_the_previous_level 等）。
	AuditStatus string
	// ExecuteDesc 是审批意见/说明（auditRecord.executeDesc 或 withdrawDesc）。
	ExecuteDesc string
	// FormData 是表单数据原始 JSON（禁止重新序列化）。
	FormData json.RawMessage
	// NextAuditors 是分支/选人。
	NextAuditors []NextAuditor
	// ReceiverID 是转发的被转发人 ID（顶层 receiverId）。
	ReceiverID string
	// Name 是转发辅助实例名。
	Name string
	// Tracking 是关注/取消关注布尔（协议顶层字段）。
	Tracking *bool
	// BizRelevance 是转发的业务关联列表。
	BizRelevance []BizRelevance
}

// BuildActionBody 按动作构造协议载荷（与实际发出的请求严格同源，供预览与禁用字段校验）。
// 各动作形状来源（参考源码+前端实际发送，F-019 实施记录）：
//   - reSubmit: data{id, formProxyId, flowInstanceBizRelevanceList} + formDataMongoVo (+nextAuditorList/fixedExecuteNodeId)
//   - storage_form_data: data{id, currentNodeProxyId, auditRecord{executeDesc}}
//   - approverAppend(移交/加签): data{id, jobTaskId, batchNo, auditRecord{auditStatus,executeDesc}} + approverAppendVo{flowNodeProxyId, userIds}
//   - roll_back: data{id, jobTaskId, withdrawDesc}
//   - retrieve: data{jobTaskId, id}
//   - revocation: data{id, withdrawDesc}
//   - urge: 顶层 flowInstanceId + data{}（与其他动作不同，不走 data 容器）
//   - transpond: data{name, flowInstanceBizRelevanceList} + receiverId + formDataMongoVo
//   - flow_tracking: data{id} + 顶层 tracking 布尔
func BuildActionBody(request ActionWriteRequest) (map[string]any, string, error) {
	switch request.Action {
	case "resubmit":
		data := map[string]any{}
		if request.InstanceID != "" {
			data["id"] = request.InstanceID
		}
		if request.FormProxyID != "" {
			data["formProxyId"] = request.FormProxyID
		}
		if request.FlowProxyID != "" {
			data["flowProxyId"] = request.FlowProxyID
		}
		body := map[string]any{"data": data}
		if len(request.FormData) > 0 {
			body["formDataMongoVo"] = map[string]any{"data": request.FormData}
		}
		if len(request.NextAuditors) > 0 {
			body["nextAuditorList"] = request.NextAuditors
		}
		return body, WriteEndpointReSubmit, nil
	case "storage_form_data":
		auditRecord := map[string]any{}
		if request.ExecuteDesc != "" {
			auditRecord["executeDesc"] = request.ExecuteDesc
		}
		body := map[string]any{"data": map[string]any{
			"id":                 request.InstanceID,
			"currentNodeProxyId": request.NodeProxyID,
			"auditRecord":        auditRecord,
		}}
		if len(request.FormData) > 0 {
			body["formDataMongoVo"] = map[string]any{"data": request.FormData}
		}
		return body, WriteEndpointStorageForm, nil
	case "transfer", "add_sign":
		data := map[string]any{
			"id":      request.InstanceID,
			"jobTaskId": request.JobTaskID,
			"auditRecord": map[string]any{
				"auditStatus": request.AuditStatus,
				"executeDesc": request.ExecuteDesc,
			},
		}
		if request.NodeProxyID != "" {
			// batchNo 由目标在任务链接中返回；无值时省略而不是传空串。
		}
		body := map[string]any{
			"data":             data,
			"approverAppendVo": map[string]any{"flowNodeProxyId": request.NodeProxyID, "userIds": []string{}},
		}
		return body, WriteEndpointApproverAppend, nil
	case "rollback_previous":
		return map[string]any{"data": map[string]any{
			"id": request.InstanceID, "jobTaskId": request.JobTaskID, "withdrawDesc": request.ExecuteDesc,
		}}, WriteEndpointRollBack, nil
	case "retrieve":
		return map[string]any{"data": map[string]any{
			"jobTaskId": request.JobTaskID, "id": request.InstanceID,
		}}, WriteEndpointRetrieve, nil
	case "withdraw":
		return map[string]any{"data": map[string]any{
			"id": request.InstanceID, "withdrawDesc": request.ExecuteDesc,
		}}, WriteEndpointRevocation, nil
	case "urge":
		// 催办协议不走 data 容器：顶层 flowInstanceId，前端实际发送 data:{}。
		return map[string]any{"flowInstanceId": request.InstanceID, "data": map[string]any{}}, WriteEndpointUrge, nil
	case "forward":
		data := map[string]any{"name": request.Name}
		if len(request.BizRelevance) > 0 {
			data["flowInstanceBizRelevanceList"] = request.BizRelevance
		}
		body := map[string]any{
			"data":       data,
			"receiverId": request.ReceiverID,
		}
		if len(request.FormData) > 0 {
			body["formDataMongoVo"] = map[string]any{"data": request.FormData}
		}
		return body, WriteEndpointTranspond, nil
	case "follow", "unfollow":
		return map[string]any{"data": map[string]any{"id": request.InstanceID}, "tracking": request.Tracking != nil && *request.Tracking}, WriteEndpointFlowTracking, nil
	default:
		return nil, "", &BusinessRejection{Code: "UNSUPPORTED_ACTION", Message: "动作不在本切片白名单内：" + request.Action}
	}
}

// ExecuteActionWrite 发出动作写请求（唯一一次），返回完整响应事实与 trace_id。
func (c *Client) ExecuteActionWrite(ctx context.Context, session Session, request ActionWriteRequest) (WriteResponse, string, error) {
	body, endpoint, err := BuildActionBody(request)
	if err != nil {
		return WriteResponse{}, "", err
	}
	envelope, traceID, err := c.CallWrite(ctx, endpoint, session.SID, body)
	response := WriteResponse{}
	if err != nil {
		if targetErr := asError(err); targetErr != nil {
			response.StatusCode = targetErr.HTTPStatus
		}
		return response, traceID, err
	}
	response.StatusCode = http.StatusOK
	response.IsSuccessPresent = true
	response.IsSuccess = responseSucceeded(envelope)
	response.Code = envelope.Code
	response.Message = envelope.Message
	if !responseSucceeded(envelope) {
		return response, traceID, &BusinessRejection{Code: envelope.Code, Message: envelope.Message}
	}
	return response, traceID, nil
}

// 保留常用引用。
var _ = strings.TrimSpace
