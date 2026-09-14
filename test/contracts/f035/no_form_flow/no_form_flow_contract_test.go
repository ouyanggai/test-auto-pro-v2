package no_form_flow_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// F-035 无表单契约：mixin.saveData 前置保存、flowProxyId 主流程、initiatorRange、
// 未登记入口拒绝、已实现页不再配置阻断。证据：NoFormFLow/Flow.vue submitFinal。

// noFormSession 构造无表单契约测试会话，身份全部用占位符。
func noFormSession() target.Session {
	return target.Session{
		SID: "sid-noform", CustomerCode: "cust-1", UserID: "user-init", CompanyID: "company-1",
		Summary: target.AccountSummary{Account: "plan-account"},
	}
}

// TestContractReviewNoFormSavesThenSubmitFlowProxy 锁定合同评审先 mixin.saveData 再发 flowProxyId 主流程。
func TestContractReviewNoFormSavesThenSubmitFlowProxy(t *testing.T) {
	var calls []string
	var lastSave, lastSubmit map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		switch {
		case strings.Contains(r.URL.Path, "contractReview/save"):
			calls = append(calls, "save")
			lastSave = body
			_, _ = w.Write([]byte(`{"isSuccess":true,"data":{"id":"biz-review-1"}}`))
		case strings.Contains(r.URL.Path, "flowInstanceApi/submit"):
			calls = append(calls, "submit")
			lastSubmit = body
			_, _ = w.Write([]byte(`{"isSuccess":true,"data":{"id":"instance-1"}}`))
		default:
			t.Fatalf("未预期端点：%s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := mustNoFormClient(t, server.URL)
	spec, ok := target.LookupNoFormFlow("contract_review")
	if !ok || spec.Status != target.SpecialBusinessImplemented {
		t.Fatal("合同评审无表单页必须登记为已实现")
	}
	saved, err := client.SaveNoFormFlow(context.Background(), noFormSession(), spec, json.RawMessage(`{"contractName":"合同A"}`), "submit")
	if err != nil {
		t.Fatal(err)
	}
	if saved.BusinessID != "biz-review-1" {
		t.Fatalf("mixin.saveData 必须返回业务 id：%+v", saved)
	}
	submit := target.BuildSubmitBody(target.SubmitFlowInstanceRequest{
		Name: "计划-路径-时间", FlowProxyID: "flow-proxy-1", CompanyID: "company-1",
		FormData:     json.RawMessage(`{"initiatorRange":"user-init","contractName":"合同A"}`),
		BatchCode:    "batch-32-chars-xxxxxxxxxxxxxxxx",
		BizRelevance: target.AppendBusinessRelevance(nil, target.NoFormFlowOtherBiz(spec.PageKey), saved.BusinessID),
	})
	if err := client.CallWriteForTest(context.Background(), target.WriteEndpointSubmit, "sid-noform", submit); err != nil {
		t.Fatal(err)
	}
	if strings.Join(calls, ">") != "save>submit" {
		t.Fatalf("无表单必须先 mixin.saveData 再发主流程：%v", calls)
	}
	saveData := lastSave["data"].(map[string]any)
	if saveData["status"] != float64(1) || saveData["contractName"] != "合同A" {
		t.Fatalf("mixin.saveData 必须带 status=1 与页面字段：%v", saveData)
	}
	submitData := lastSubmit["data"].(map[string]any)
	if submitData["flowProxyId"] != "flow-proxy-1" {
		t.Fatal("无表单主流程必须发 flowProxyId")
	}
	if _, exists := submitData["formProxyId"]; exists {
		t.Fatal("无表单主流程禁止发 formProxyId")
	}
}

// TestNoFormSubmitMongoAlwaysCarriesInitiatorRange 锁定已有表单数据仍必须写入当前会话 initiatorRange。
func TestNoFormSubmitMongoAlwaysCarriesInitiatorRange(t *testing.T) {
	runCtx := step.RunContext{RenderType: string(target.FormRenderTypeVueCustom), FlowType: "buy_plan", FlowProxyID: "flow-proxy-1"}
	compiled := model.CompiledActionStep{Action: model.ActionSubmit, NodeKey: "node-start"}
	session := noFormSession()
	_, _, body, err := step.BuildRequestForTest(runCtx, compiled, session, json.RawMessage(`{"contractName":"合同A","initiatorRange":"old-user"}`), "")
	if err != nil {
		t.Fatal(err)
	}
	container := body["formDataMongoVo"].(map[string]any)
	var form map[string]any
	switch typed := container["data"].(type) {
	case json.RawMessage:
		_ = json.Unmarshal(typed, &form)
	case []byte:
		_ = json.Unmarshal(typed, &form)
	case map[string]any:
		form = typed
	default:
		encoded, _ := json.Marshal(typed)
		_ = json.Unmarshal(encoded, &form)
	}
	if form["initiatorRange"] != "user-init" {
		t.Fatalf("已有表单数据时仍必须覆盖为当前会话 initiatorRange：%v", form)
	}
	if form["contractName"] != "合同A" {
		t.Fatalf("页面字段不得被 initiatorRange 覆盖丢掉：%v", form)
	}
	data := body["data"].(map[string]any)
	if data["flowProxyId"] != "flow-proxy-1" {
		t.Fatal("vue_custom 发起必须只发 flowProxyId")
	}
	if _, exists := data["formProxyId"]; exists {
		t.Fatal("vue_custom 发起禁止发 formProxyId")
	}
}

// TestUnregisteredNoFormPageRejected 锁定未登记无表单页写前拒绝。
func TestUnregisteredNoFormPageRejected(t *testing.T) {
	reason := target.NoFormFlowBlockReason("unknown_custom_page", "submit")
	if reason == "" || !strings.Contains(reason, "未登记") {
		t.Fatalf("未登记无表单页必须拒绝：%s", reason)
	}
	if target.NeedsNoFormFlowPreSave("unknown_custom_page", "submit") {
		t.Fatal("未登记页不得假装已有前置保存")
	}
}

// TestImplementedNoFormPageNotConfigBlocked 锁定已实现页不再配置阻断，通用入口仍阻断。
func TestImplementedNoFormPageNotConfigBlocked(t *testing.T) {
	page := target.ResolveVueCustomPage(target.FormRenderTypeVueCustom, "buy_plan", "采购计划")
	if page.Status != "complete" || len(page.Issues) != 0 {
		t.Fatalf("已实现采购计划不得再配置阻断：%+v", page)
	}
	generic := target.ResolveVueCustomPage(target.FormRenderTypeVueCustom, "NoFormFlow", "通用")
	if generic.Status != "partial" || len(generic.Issues) == 0 {
		t.Fatalf("通用无表单入口必须继续阻断：%+v", generic)
	}
}

// TestAnnualAssessmentNoFormSaveBody 锁定年度考核无表单业务保存字段。
func TestAnnualAssessmentNoFormSaveBody(t *testing.T) {
	var saved map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &saved)
		_, _ = w.Write([]byte(`{"isSuccess":true,"data":{"id":"biz-assess-1"}}`))
	}))
	defer server.Close()
	client := mustNoFormClient(t, server.URL)
	spec, _ := target.LookupNoFormFlow("staff_annual_assessment")
	_, err := client.SaveNoFormFlow(context.Background(), noFormSession(), spec, json.RawMessage(`{
		"userId":"user-1","year":"2026","userName":"考核人","companyName":"公司","deptName":"部门"
	}`), "submit")
	if err != nil {
		t.Fatal(err)
	}
	data := saved["data"].(map[string]any)
	if data["userId"] != "user-1" || data["assessmentScore"] != "0" || data["finalScore"] != "0" {
		t.Fatalf("年度考核保存必须带页面字段与默认分数：%v", data)
	}
}

// mustNoFormClient 创建指向假目标的写客户端。
func mustNoFormClient(t *testing.T, baseURL string) *target.Client {
	t.Helper()
	client, err := target.NewClient(target.ClientConfig{
		BaseURL: baseURL, Timeout: time.Second, CustomerCode: "cust-1", PlatformCode: "200001",
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}
