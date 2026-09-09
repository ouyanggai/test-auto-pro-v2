// F-019 全动作写方法：动作目录 15 类动作对应的 12 个写端点的适配层实现。
// 纪律：一次尝试最多一次写请求（CallWrite，无重试）；载荷与参考源码及前端实际发送逐字同源；
// 响应按判定包需要分层返回（传输失败经 error，完整响应经 WriteResponse，业务拒绝原样携带 code/message）。
package target

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// F-019 写端点白名单（动作目录声明的全部端点，除 Submit 和 Audit 在 write.go 已定义）。
const (
	WriteEndpointReSubmit        = "/web/flowInstanceApi/reSubmit"
	WriteEndpointStorageForm     = "/web/flowInstanceApi/storageFormData"
	WriteEndpointApproverAppend  = "/web/flowInstanceApi/approverAppend"
	WriteEndpointUpdateFlowProxy = "/web/flowInstanceApi/updateFlowProxy"
	WriteEndpointRollBack        = "/web/flowInstanceApi/rollBackThePreviousLevel"
	WriteEndpointRetrieve        = "/web/flowInstanceApi/retrieveProcess"
	WriteEndpointRevocation      = "/web/flowInstanceApi/revocation"
	WriteEndpointUrge            = "/web/urgeHandleRecord/sendUrgeMessage"
	WriteEndpointTranspond       = "/web/flowInstanceApi/transpond"
	WriteEndpointFlowTracking    = "/web/flowInstanceApi/flowTracking"
)

// ActionWriteRequest 是动作写请求的统一意图：执行器按动作填充，适配层负责协议。
type ActionWriteRequest struct {
	// Action 是动作键，决定端点与载荷形状（同一端点不同动作不混淆）。
	Action string
	// InstanceID 是主实例 ID（data.id）。
	InstanceID string
	// JobTaskID 是待办任务链接 ID（审批类动作硬性必填）。
	JobTaskID string
	// BatchNo 是任务快照返回的当前批次；移交必须原样带给目标。
	BatchNo string
	// FlowProxyID / FormProxyID 是重提等动作需要的代理标识。
	FlowProxyID string
	FormProxyID string
	// CompanyID 是重新提交时目标前端携带的当前公司标识。
	// 目标后端在缺失时会回填旧实例公司，但执行器有实时会话时应按前端协议直接发送。
	CompanyID string
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
	// UserIDs 是移交动作的实时受限目标人员 ID 集合；加签把人员写入 FlowProxyTree。
	UserIDs []string
	// FlowProxyTree 是加签时从目标读取并修改后的完整 FlowProxyVo 原始 JSON。
	// 目标 updateFlowProxy 会按整棵树重建实例私有代理，不能传简化节点模型。
	FlowProxyTree json.RawMessage
	// ReceiverID 是转发的被转发人 ID（顶层 receiverId）。
	ReceiverID string
	// Name 是转发辅助实例名。
	Name string
	// Tracking 是关注/取消关注布尔（协议顶层字段）。
	Tracking *bool
	// BizRelevance 是实例已有业务关联；重提、审批和转发需要按目标协议带回。
	BizRelevance []BizRelevance
}

// RequestValidationError 表示动作写请求在目标 HTTP 请求发出前被本地校验拒绝。
// 该错误携带原始中文原因，执行器据此确认没有副作用，不能进入结果待确认状态。
type RequestValidationError struct {
	Message string
}

// Error 返回本地校验的直接原因，供页面显示和执行器分类使用。
func (e *RequestValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "动作写请求校验失败"
	}
	return e.Message
}

// BuildActionBody 按动作构造协议载荷（与实际发出的请求严格同源，供预览与禁用字段校验）。
// 各动作形状来源（参考源码+前端实际发送，F-019 实施记录）：
//   - reSubmit: data{id, formProxyId, flowInstanceBizRelevanceList} + formDataMongoVo (+nextAuditorList/fixedExecuteNodeId)
//   - storage_form_data: data{id, currentNodeProxyId, auditRecord{executeDesc}}
//   - updateFlowProxy(加签): data{id} + flowProxyProtocol{data:<完整 FlowProxyVo>}
//   - approverAppend(移交): data{id, jobTaskId, batchNo, auditRecord{auditStatus,executeDesc}} + approverAppendVo{flowNodeProxyId, userIds}
//   - roll_back: data{id, jobTaskId, withdrawDesc}
//   - retrieve: data{jobTaskId, id}
//   - revocation: data{id, withdrawDesc}
//   - urge: 顶层 flowInstanceId + data{}（与其他动作不同，不走 data 容器）
//   - audit(不同意): data{id, jobTaskId, flowProxyId, auditRecord{auditStatus=no_pass, executeDesc}} + formDataMongoVo
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
		if companyID := strings.TrimSpace(request.CompanyID); companyID != "" {
			data["companyId"] = companyID
		}
		if len(request.BizRelevance) > 0 {
			data["flowInstanceBizRelevanceList"] = request.BizRelevance
		}
		body := map[string]any{"data": data}
		formData := request.FormData
		if len(formData) == 0 {
			formData = json.RawMessage(`{}`)
		}
		body["formDataMongoVo"] = map[string]any{"data": formData}
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
		formData := request.FormData
		if len(formData) == 0 {
			formData = json.RawMessage(`{}`)
		}
		body["formDataMongoVo"] = map[string]any{"data": formData}
		return body, WriteEndpointStorageForm, nil
	case "approve":
		// 同意与不同意共用审批端点，差别在 auditRecord.auditStatus=pass
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
		auditRecord := map[string]any{"auditStatus": strings.TrimSpace(request.AuditStatus)}
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
		if request.Tracking != nil {
			body["tracking"] = *request.Tracking
		}
		return body, WriteEndpointAudit, nil
	case "reject":
		// 不同意与同意共用审批端点，差别只在 auditRecord.auditStatus=no_pass
		// （FlowAuditServiceImpl 按 ExecuteResultEnum 分派）。表单数据同样整份提交：
		// 审批必调 saveFormData 且是整份覆盖（语义清单第 16 条），少带字段等于把其余字段清空。
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
		auditRecord := map[string]any{"auditStatus": strings.TrimSpace(request.AuditStatus)}
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
		return body, WriteEndpointAudit, nil
	case "add_sign":
		tree := bytes.TrimSpace(request.FlowProxyTree)
		// 预览阶段尚未读取目标代理树，先保留 endpoint 与协议形状；真正发送由
		// ExecuteActionWrite 再次硬性校验，避免把 null 树发到目标。
		return map[string]any{
			"data":              map[string]any{"id": strings.TrimSpace(request.InstanceID)},
			"flowProxyProtocol": map[string]any{"data": json.RawMessage(tree)},
		}, WriteEndpointUpdateFlowProxy, nil
	case "transfer":
		data := map[string]any{
			"id":        request.InstanceID,
			"jobTaskId": request.JobTaskID,
			"auditRecord": map[string]any{
				"auditStatus": request.AuditStatus,
				"executeDesc": request.ExecuteDesc,
			},
		}
		if batchNo := strings.TrimSpace(request.BatchNo); batchNo != "" {
			data["batchNo"] = batchNo
		}
		userIDs := make([]string, 0, len(request.UserIDs))
		seen := make(map[string]bool, len(request.UserIDs))
		for _, userID := range request.UserIDs {
			userID = strings.TrimSpace(userID)
			if userID != "" && !seen[userID] {
				seen[userID] = true
				userIDs = append(userIDs, userID)
			}
		}
		body := map[string]any{
			"data":             data,
			"approverAppendVo": map[string]any{"flowNodeProxyId": request.NodeProxyID, "userIds": userIDs},
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
		formData := request.FormData
		if len(formData) == 0 {
			// 目标转发服务会直接读取 formDataMongoVo；空表单也必须发送空对象，
			// 否则目标会在业务处理前因空容器失败。
			formData = json.RawMessage(`{}`)
		}
		body := map[string]any{
			"data":            data,
			"receiverId":      request.ReceiverID,
			"formDataMongoVo": map[string]any{"data": formData},
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
	if err := validateActionWriteRequest(request); err != nil {
		return WriteResponse{}, "", &RequestValidationError{Message: err.Error()}
	}
	body, endpoint, err := BuildActionBody(request)
	if err != nil {
		return WriteResponse{}, "", &RequestValidationError{Message: err.Error()}
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
	response.Data = append(json.RawMessage(nil), envelope.Data...)
	if !responseSucceeded(envelope) {
		return response, traceID, &BusinessRejection{Code: envelope.Code, Message: envelope.Message}
	}
	return response, traceID, nil
}

// validateActionWriteRequest 校验各原子动作在目标写端点上的必填事实，避免把空标识或错误枚举发给目标。
// 预览阶段允许构造不完整载荷；真正发送前必须通过这里的动作级校验。
func validateActionWriteRequest(request ActionWriteRequest) error {
	require := func(value, name string) error {
		if strings.TrimSpace(value) == "" {
			return errors.New(name + "不能为空，拒绝发送")
		}
		return nil
	}
	switch strings.TrimSpace(request.Action) {
	case "resubmit":
		if err := require(request.InstanceID, "流程实例 id"); err != nil {
			return err
		}
		if strings.TrimSpace(request.FlowProxyID) == "" && strings.TrimSpace(request.FormProxyID) == "" {
			return errors.New("重新提交缺少 flowProxyId/formProxyId，拒绝发送")
		}
	case "storage_form_data":
		if err := require(request.InstanceID, "流程实例 id"); err != nil {
			return err
		}
		return require(request.NodeProxyID, "当前节点代理 id")
	case "approve":
		// 同意与不同意同用审批端点；同意必须显式携带 pass，防止把空 auditStatus 发成目标默认行为。
		if err := require(request.InstanceID, "流程实例 id"); err != nil {
			return err
		}
		if err := require(request.JobTaskID, "待办任务 id"); err != nil {
			return err
		}
		if strings.TrimSpace(request.AuditStatus) != "pass" {
			return errors.New("同意动作必须使用 auditStatus=pass，拒绝发送")
		}
	case "reject":
		if err := require(request.InstanceID, "流程实例 id"); err != nil {
			return err
		}
		if err := require(request.JobTaskID, "待办任务 id"); err != nil {
			return err
		}
		if strings.TrimSpace(request.AuditStatus) != "no_pass" {
			return errors.New("不同意动作必须使用 auditStatus=no_pass，拒绝发送")
		}
	case "add_sign":
		if err := require(request.InstanceID, "流程实例 id"); err != nil {
			return err
		}
		if err := require(request.NodeProxyID, "当前节点代理 id"); err != nil {
			return err
		}
		if len(nonEmptyIDs(request.UserIDs)) == 0 {
			return errors.New("加签未解析到目标人员，拒绝发送")
		}
		tree := bytes.TrimSpace(request.FlowProxyTree)
		if len(tree) == 0 || bytes.Equal(tree, []byte("null")) {
			return errors.New("加签流程代理树为空，拒绝发送")
		}
	case "transfer":
		if err := require(request.InstanceID, "流程实例 id"); err != nil {
			return err
		}
		if err := require(request.JobTaskID, "待办任务 id"); err != nil {
			return err
		}
		if err := require(request.BatchNo, "待办 batchNo"); err != nil {
			return err
		}
		if err := require(request.NodeProxyID, "当前节点代理 id"); err != nil {
			return err
		}
		if strings.TrimSpace(request.AuditStatus) != "transfer" {
			return errors.New("移交动作必须使用 auditStatus=transfer，拒绝发送")
		}
		if len(nonEmptyIDs(request.UserIDs)) == 0 {
			return errors.New("移交未解析到目标人员，拒绝发送")
		}
	case "rollback_previous":
		if err := require(request.InstanceID, "流程实例 id"); err != nil {
			return err
		}
		return require(request.JobTaskID, "待办任务 id")
	case "retrieve":
		if err := require(request.InstanceID, "流程实例 id"); err != nil {
			return err
		}
		return require(request.JobTaskID, "已办任务 id")
	case "withdraw", "urge":
		return require(request.InstanceID, "流程实例 id")
	case "forward":
		if err := require(request.ReceiverID, "转发接收人 id"); err != nil {
			return err
		}
		if err := require(request.Name, "转发实例名称"); err != nil {
			return err
		}
	case "follow", "unfollow":
		if err := require(request.InstanceID, "流程实例 id"); err != nil {
			return err
		}
		if request.Tracking == nil {
			return errors.New("关注状态不能为空，拒绝发送")
		}
	default:
		return errors.New("动作不在原子写白名单内，拒绝发送：" + strings.TrimSpace(request.Action))
	}
	return nil
}

// nonEmptyIDs 去除动作人员列表中的空值，供发送前判断和协议构造共用同一语义。
func nonEmptyIDs(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// 保留常用引用。
var _ = strings.TrimSpace
