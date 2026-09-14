package target

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"test-auto-pro-v2/internal/jsonvalues"
)

// SpecialBusinessSaveResult 是特殊业务前置保存的目标事实：返回业务 ID 供主流程关联。
type SpecialBusinessSaveResult struct {
	BusinessID string
	Response   WriteResponse
	TraceID    string
}

// SaveSpecialBusiness 按目标页面顺序发出特殊业务前置写。
// 业务保存失败不得继续主流程；成功后把返回 id 交给调用方写入 flowInstanceBizRelevanceList。
// 无独立业务 URL 的分支（出版委托/专业提资/年度绩效意见改写）返回空 ID，由调用方只改表单。
func (c *Client) SaveSpecialBusiness(ctx context.Context, session Session, spec SpecialBusinessSpec, formData json.RawMessage, action string) (SpecialBusinessSaveResult, error) {
	if strings.TrimSpace(spec.BusinessEndpoint) == "" {
		return SpecialBusinessSaveResult{}, nil
	}
	if spec.Status != SpecialBusinessImplemented {
		return SpecialBusinessSaveResult{}, fmt.Errorf("%s", SpecialBusinessBlockReason(spec.FlowType, action))
	}
	body, err := buildSpecialBusinessSaveBody(spec, formData, action)
	if err != nil {
		return SpecialBusinessSaveResult{}, err
	}
	return c.callBusinessSave(ctx, session, spec.BusinessEndpoint, body)
}

// SaveNoFormFlow 按 mixin.saveData 发出无表单业务保存，再由调用方用返回 id 走 flowProxyId 主流程。
func (c *Client) SaveNoFormFlow(ctx context.Context, session Session, spec NoFormFlowSpec, formData json.RawMessage, action string) (SpecialBusinessSaveResult, error) {
	if strings.TrimSpace(spec.BusinessEndpoint) == "" {
		return SpecialBusinessSaveResult{}, fmt.Errorf("无表单页面「%s」缺少业务保存接口，不能发通用 submit 顶替", spec.PageKey)
	}
	if spec.Status != SpecialBusinessImplemented || spec.Chain == NoFormFlowChainBlocked {
		return SpecialBusinessSaveResult{}, fmt.Errorf("%s", NoFormFlowBlockReason(spec.PageKey, action))
	}
	body, err := buildNoFormFlowSaveBody(spec, formData, action)
	if err != nil {
		return SpecialBusinessSaveResult{}, err
	}
	return c.callBusinessSave(ctx, session, spec.BusinessEndpoint, body)
}

// callBusinessSave 走白名单写出口发出业务保存；未登记端点会被矩阵拒绝。
func (c *Client) callBusinessSave(ctx context.Context, session Session, endpoint string, body map[string]any) (SpecialBusinessSaveResult, error) {
	return c.CallBusinessSave(ctx, session, endpoint, body)
}

// buildSpecialBusinessSaveBody 按目标源码符号构造前置保存载荷，不猜测未出现的字段。
func buildSpecialBusinessSaveBody(spec SpecialBusinessSpec, formData json.RawMessage, action string) (map[string]any, error) {
	values, err := decodeFormObject(formData)
	if err != nil {
		return nil, err
	}
	switch spec.FlowType {
	case "cost_funds_transactions", "cost_funds_invest":
		return buildCostFundsSaveBody(spec.FlowType, values, action), nil
	case "staff_annual_assessment":
		return buildAnnualAssessmentSaveBody(values), nil
	case "expense_budget":
		return buildExpenseBudgetSaveBody(values), nil
	default:
		return map[string]any{"data": values}, nil
	}
}

// buildNoFormFlowSaveBody 对齐 mixin.submit：{data: 页面字段 + status + projectId}。
func buildNoFormFlowSaveBody(spec NoFormFlowSpec, formData json.RawMessage, action string) (map[string]any, error) {
	values, err := decodeFormObject(formData)
	if err != nil {
		return nil, err
	}
	switch spec.PageKey {
	case "expense_budget":
		return buildExpenseBudgetSaveBody(values), nil
	case "staff_annual_assessment":
		return buildAnnualAssessmentSaveBody(values), nil
	default:
		data := cloneStringAnyMap(values)
		if action == "save_draft" {
			data["status"] = 0
		} else if _, exists := data["status"]; !exists {
			data["status"] = 1
		}
		if _, exists := data["projectId"]; !exists {
			data["projectId"] = ""
		}
		return map[string]any{"data": data}, nil
	}
}

// buildCostFundsSaveBody 对齐 FlowDialog.saveCostFundsBusiness：先保存请款业务再发主流程。
func buildCostFundsSaveBody(flowType string, values map[string]any, action string) map[string]any {
	depID, depName := pickerJSONIDName(values["requestDepName"])
	userID, userName := pickerJSONIDName(values["requestUserName"])
	business := map[string]any{
		"depId":           depID,
		"depName":         depName,
		"expenseUserId":   userID,
		"expenseUserName": userName,
		"remarks":         formString(values, "remark"),
		"payCompanyId":    formString(values, "applicationFundsVo_payCompanyId"),
		"payCompanyName":  formString(values, "applicationFundsVo_payCompanyName"),
		"payMethod":       formString(values, "applicationFundsVo_payMethod"),
		"payMoney":        formString(values, "applicationFundsVo_payMoney"),
		"name":            formString(values, "applicationFundsVo_payName"),
		"openingBank":     formString(values, "applicationFundsVo_openingBank"),
		"account":         formString(values, "applicationFundsVo_account"),
		"paymentAccount":  formString(values, "applicationFundsVo_paymentAccount"),
	}
	if action == "save_draft" {
		business["status"] = "0"
	} else {
		business["status"] = "1"
		business["examineStatus"] = "0"
	}
	if flowType == "cost_funds_invest" {
		business["type"] = "2"
	} else {
		business["type"] = "1"
	}
	if id := formString(values, "id", "businessId"); id != "" {
		business["id"] = id
	}
	return map[string]any{"data": business}
}

// buildAnnualAssessmentSaveBody 对齐 AnnualAssessmentNoForm.saveBusinessData。
func buildAnnualAssessmentSaveBody(values map[string]any) map[string]any {
	data := map[string]any{
		"userId":          formString(values, "userId"),
		"year":            values["year"],
		"firstQuarter":    values["firstQuarter"],
		"secondQuarter":   values["secondQuarter"],
		"thirdQuarter":    values["thirdQuarter"],
		"fourQuarter":     values["fourQuarter"],
		"customerCode":    formString(values, "customerCode"),
		"companyName":     formString(values, "companyName"),
		"deptName":        firstNonEmpty(formString(values, "deptName"), formString(values, "departmentName")),
		"userName":        formString(values, "userName"),
		"assessmentScore": firstNonEmpty(formString(values, "assessmentScore", "reportScore"), "0"),
		"finalScore":      firstNonEmpty(formString(values, "finalScore"), "0"),
		"opinion":         formString(values, "opinion"),
	}
	if id := formString(values, "id", "bizId"); id != "" {
		data["id"] = id
	}
	return map[string]any{"data": data}
}

// buildExpenseBudgetSaveBody 对齐 ExpensesClaimForm.getParam/postData：data 容器加三类明细列表。
func buildExpenseBudgetSaveBody(values map[string]any) map[string]any {
	if _, hasLists := values["expenseBudgetList"]; hasLists {
		body := cloneStringAnyMap(values)
		if _, exists := body["data"]; !exists {
			body["data"] = map[string]any{}
		}
		return body
	}
	return map[string]any{"data": cloneStringAnyMap(values)}
}

// decodeFormObject 以数字保真方式解码表单对象；空正文按空对象处理。
func decodeFormObject(formData json.RawMessage) (map[string]any, error) {
	if len(bytesTrimSpace(formData)) == 0 {
		return map[string]any{}, nil
	}
	values, err := jsonvalues.DecodeObject(formData)
	if err != nil {
		return nil, fmt.Errorf("业务保存表单数据解码失败：%w", err)
	}
	return values, nil
}

// parseBusinessSaveID 读取目标业务保存响应的 id：对象取 data.id，标量字符串原样使用。
func parseBusinessSaveID(data json.RawMessage) string {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	if strings.HasPrefix(trimmed, "\"") {
		var text string
		if err := json.Unmarshal(data, &text); err == nil {
			return strings.TrimSpace(text)
		}
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err == nil {
		if id := formString(object, "id"); id != "" {
			return id
		}
	}
	return strings.Trim(trimmed, `"`)
}

// pickerJSONIDName 从人员/部门选择器 JSON 文本取出 id 与 name。
func pickerJSONIDName(raw any) (id, name string) {
	text, ok := raw.(string)
	if !ok {
		return "", ""
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", ""
	}
	var decoded struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return "", ""
	}
	return strings.TrimSpace(decoded.ID), strings.TrimSpace(decoded.Name)
}

// formString 按候选键读取表单字符串；空值跳过，不伪造。
func formString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		raw, exists := values[key]
		if !exists || raw == nil {
			continue
		}
		switch typed := raw.(type) {
		case string:
			if text := strings.TrimSpace(typed); text != "" {
				return text
			}
		case json.Number:
			return typed.String()
		default:
			text := strings.TrimSpace(fmt.Sprint(typed))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

// cloneStringAnyMap 浅拷贝表单对象，避免改写调用方的 map。
func cloneStringAnyMap(values map[string]any) map[string]any {
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

// bytesTrimSpace 去掉 JSON 原文两端空白，供空载荷判断。
func bytesTrimSpace(raw json.RawMessage) []byte {
	return []byte(strings.TrimSpace(string(raw)))
}

// AppendBusinessRelevance 把业务保存返回的 id 追加到关联列表；已有同类型项不重复写。
func AppendBusinessRelevance(values []BizRelevance, otherBiz, businessID string) []BizRelevance {
	otherBiz = strings.TrimSpace(otherBiz)
	businessID = strings.TrimSpace(businessID)
	if otherBiz == "" || businessID == "" {
		return values
	}
	result := append([]BizRelevance(nil), values...)
	for _, item := range result {
		if strings.EqualFold(strings.TrimSpace(item.OtherBiz), otherBiz) && strings.TrimSpace(item.OtherBizID) == businessID {
			return result
		}
	}
	return append(result, BizRelevance{OtherBiz: otherBiz, OtherBizID: businessID})
}

// HasBusinessRelevance 判断关联列表是否已有该业务类型的非空 id，避免重复保存。
func HasBusinessRelevance(values []BizRelevance, otherBiz string) bool {
	otherBiz = strings.TrimSpace(otherBiz)
	if otherBiz == "" {
		return false
	}
	for _, item := range values {
		if strings.EqualFold(strings.TrimSpace(item.OtherBiz), otherBiz) && strings.TrimSpace(item.OtherBizID) != "" {
			return true
		}
	}
	return false
}
