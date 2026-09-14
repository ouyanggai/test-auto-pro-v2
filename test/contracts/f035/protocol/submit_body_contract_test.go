package protocol_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
)

// TestSubmitBodyMatchesFormMakingPageContract 锁定 FormMaking submit/draft 的目标页面同源载荷形状（F-035/T01）。
// 证据：FlowDialog.enterpriseHandleSubmit（参考代码 FlowDialog.vue:757-905）与人工成功 curl。
func TestSubmitBodyMatchesFormMakingPageContract(t *testing.T) {
	body := target.BuildSubmitBody(target.SubmitFlowInstanceRequest{
		Name:        "计划-路径-时间",
		FormProxyID: "form-proxy-1",
		FlowProxyID: "flow-proxy-1", // FormMaking 时必须被忽略：目标页面 formId 非空只发 formProxyId
		CompanyID:   "company-1",
		FormData:    json.RawMessage(`{"amount":3}`),
		BatchCode:   "batch-32-chars",
	})
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	var flat map[string]any
	if err := json.Unmarshal(encoded, &flat); err != nil {
		t.Fatal(err)
	}
	data := flat["data"].(map[string]any)
	if data["formProxyId"] != "form-proxy-1" {
		t.Fatalf("FormMaking 必须发送 formProxyId：%v", data)
	}
	if _, exists := data["flowProxyId"]; exists {
		t.Fatal("FormMaking 禁止同时发送 flowProxyId（目标页面二选一）")
	}
	if _, exists := data["customerCode"]; exists {
		// customerCode 由统一出口注入（目标 axios 拦截器），构造器不负责；此处确认不重复注入。
		t.Fatal("customerCode 应由统一出口注入而不是构造器")
	}
	if list, ok := data["flowInstanceBizRelevanceList"].([]any); !ok {
		t.Fatalf("业务关联必须固定为数组：%v", data["flowInstanceBizRelevanceList"])
	} else if len(list) == 0 {
		// 空列表允许：调用方未给业务关联时保留空数组形状，绝不能省略字段。
		t.Log("空业务关联合法（保留数组形状）")
	}
	if auditors, ok := flat["nextAuditorList"].([]any); !ok || len(auditors) != 0 {
		t.Fatalf("无显式选人时 nextAuditorList 必须是空数组（人工 curl 证据）：%v", flat["nextAuditorList"])
	}
	if flat["batchCode"] != "batch-32-chars" {
		t.Fatalf("submit 顶层必须携带 batchCode：%v", flat["batchCode"])
	}
	if formData := flat["formDataMongoVo"].(map[string]any)["data"]; !reflect.DeepEqual(formData, map[string]any{"amount": float64(3)}) {
		t.Fatalf("表单容器形状错误：%v", formData)
	}
}

// TestSubmitBodyNoFormUsesFlowProxyOnly 锁定无表单流程只发送 flowProxyId。
func TestSubmitBodyNoFormUsesFlowProxyOnly(t *testing.T) {
	body := target.BuildSubmitBody(target.SubmitFlowInstanceRequest{
		FlowProxyID: "flow-proxy-1",
		CompanyID:   "company-1",
		FormData:    json.RawMessage(`{}`),
		BatchCode:   "batch-32-chars",
	})
	encoded, _ := json.Marshal(body)
	var flat map[string]any
	_ = json.Unmarshal(encoded, &flat)
	data := flat["data"].(map[string]any)
	if data["flowProxyId"] != "flow-proxy-1" {
		t.Fatalf("无表单必须发送 flowProxyId：%v", data)
	}
	if _, exists := data["formProxyId"]; exists {
		t.Fatal("无表单禁止发送 formProxyId")
	}
}

// TestAuditBodyAlwaysCarriesTrackingAndBizRelevance 锁定审批载荷的无条件字段（F-035 矩阵）：
// tracking 顶层布尔必发；业务关联固定数组；batchCode 禁止出现。
func TestAuditBodyAlwaysCarriesTrackingAndBizRelevance(t *testing.T) {
	body := target.BuildAuditBody(target.AuditCurrentTaskRequest{
		InstanceID: "instance-1", JobTaskID: "task-1", FlowProxyID: "flow-1",
		AuditStatus: "pass", ExecuteDesc: "同意",
	})
	encoded, _ := json.Marshal(body)
	var flat map[string]any
	_ = json.Unmarshal(encoded, &flat)
	if tracking, exists := flat["tracking"]; !exists {
		t.Fatal("tracking 必须无条件携带（页面 this.tracking 直发）")
	} else if tracking != false {
		t.Fatalf("未配置 tracking 时应发 false：%v", tracking)
	}
	if _, ok := flat["data"].(map[string]any)["flowInstanceBizRelevanceList"].([]any); !ok {
		t.Fatal("业务关联必须固定为数组")
	}
	if flat["nextAuditorList"] != nil {
		t.Fatal("非 pass 显式人员时 nextAuditorList 不发送（页面仅 pass 分支 map）")
	}
	if _, exists := flat["batchCode"]; exists {
		t.Fatal("审批禁止携带 batchCode（矩阵 forbidden）")
	}
}

// TestResubmitBodyContract 锁定重提载荷：代理互斥、空 nextAuditorList 数组、无 batchCode。
func TestResubmitBodyContract(t *testing.T) {
	body, endpoint, err := target.BuildActionBody(target.ActionWriteRequest{
		Action: "resubmit", InstanceID: "instance-1", FormProxyID: "form-live",
		FlowProxyID: "flow-live", CompanyID: "company-1", FormData: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if endpoint != target.WriteEndpointReSubmit {
		t.Fatalf("重提端点错误：%s", endpoint)
	}
	encoded, _ := json.Marshal(body)
	var flat map[string]any
	_ = json.Unmarshal(encoded, &flat)
	data := flat["data"].(map[string]any)
	if data["formProxyId"] != "form-live" {
		t.Fatal("FormMaking 重提必须发实时代理 formProxyId")
	}
	if _, exists := data["flowProxyId"]; exists {
		t.Fatal("重提禁止同时发送 flowProxyId")
	}
	if auditors, ok := flat["nextAuditorList"].([]any); !ok || len(auditors) != 0 {
		t.Fatal("重提页面无条件 map 出 nextAuditorList，空时必须是 []")
	}
	if _, exists := flat["batchCode"]; exists {
		t.Fatal("重提不携带 batchCode（矩阵 forbidden）")
	}
}

// TestEnvelopeInjectsProjectIDAndCustomerCode 锁定统一出口的信封注入（目标 axios 拦截器同语义）：
// 顶层 sid/projectId 与 data.customerCode 在带会话的请求上注入，projectId 空字符串必须保留。
func TestEnvelopeInjectsProjectIDAndCustomerCode(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &received)
		_, _ = w.Write([]byte(`{"isSuccess":true,"data":{}}`))
	}))
	defer server.Close()
	client, err := target.NewClient(target.ClientConfig{
		BaseURL: server.URL, Timeout: time.Second, CustomerCode: "cust-1", PlatformCode: "200001",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CallWriteForTest(context.Background(), "/web/flowInstanceApi/submit", "sid-1", map[string]any{
		"data": map[string]any{"name": "n"},
	}); err != nil {
		t.Fatalf("写出口请求失败：%v", err)
	}
	if received["sid"] != "sid-1" {
		t.Fatalf("顶层 sid 注入缺失：%v", received)
	}
	if id, ok := received["projectId"]; !ok || id != "" {
		t.Fatalf("顶层 projectId 必须注入且空字符串保留：%v", received["projectId"])
	}
	data := received["data"].(map[string]any)
	if data["customerCode"] != "cust-1" {
		t.Fatalf("data.customerCode 必须由出口注入：%v", data)
	}
}

// TestProtocolMatrixRegistry 锁定矩阵登记表覆盖三个核心写端点且取值在四态集合内。
func TestProtocolMatrixRegistry(t *testing.T) {
	matrix := target.ProtocolMatrix()
	valid := map[target.FieldPresence]bool{
		target.PresenceRequired: true, target.PresenceOptional: true,
		target.PresenceEmpty: true, target.PresenceForbidden: true,
	}
	for _, endpoint := range []string{target.WriteEndpointSubmit, target.WriteEndpointReSubmit, target.WriteEndpointAudit} {
		fields, ok := matrix[endpoint]
		if !ok {
			t.Fatalf("端点 %s 未登记协议矩阵", endpoint)
		}
		if len(fields) == 0 {
			t.Fatalf("端点 %s 矩阵为空", endpoint)
		}
		for field, presence := range fields {
			if !valid[presence] {
				t.Fatalf("端点 %s 字段 %s 存在性取值非法：%s", endpoint, field, presence)
			}
		}
	}
	if matrix[target.WriteEndpointSubmit]["batchCode"] != target.PresenceRequired {
		t.Fatal("submit 的 batchCode 必须登记为 required")
	}
	if matrix[target.WriteEndpointAudit]["batchCode"] != target.PresenceForbidden {
		t.Fatal("audit 的 batchCode 必须登记为 forbidden")
	}
}

// TestNewBatchCodeShape 锁定批次号形状：32 位十六进制，与目标 generateRandomId(32) 同形状。
func TestNewBatchCodeShape(t *testing.T) {
	code := target.NewBatchCode()
	if len(code) != 32 {
		t.Fatalf("批次号长度应为 32，实际 %d", len(code))
	}
	for _, r := range code {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			t.Fatalf("批次号必须是十六进制：%s", code)
		}
	}
	if code == target.NewBatchCode() {
		t.Fatal("两次生成的批次号不应相同")
	}
}
