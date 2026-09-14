package special_business_test

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

// F-035 特殊业务契约：前置保存顺序、失败不得继续主流程、已有关联跳过重复保存、
// 出版委托日期改写、合同盖章写前阻塞。fixture 形状对齐 FlowDialog.saveCostFundsBusiness
// 与 EnterpriseExamineOpinion.handleSubmitCheck，不用登记表自证。

// specialSession 构造契约测试会话：身份字段全部用占位符，不绑定真实账号。
func specialSession() target.Session {
	return target.Session{
		SID: "sid-special", CustomerCode: "cust-1", UserID: "user-pm", CompanyID: "company-1",
		Summary: target.AccountSummary{Account: "plan-account"},
	}
}

// TestCostFundsPreSaveThenMainSubmitOrder 锁定资金往来先保存请款业务再发 FormMaking submit。
func TestCostFundsPreSaveThenMainSubmitOrder(t *testing.T) {
	var calls []string
	var lastSave, lastSubmit map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		switch {
		case strings.Contains(r.URL.Path, "costFundsTransactions/save"):
			calls = append(calls, "save")
			lastSave = body
			_, _ = w.Write([]byte(`{"isSuccess":true,"data":{"id":"biz-funds-1"}}`))
		case strings.Contains(r.URL.Path, "flowInstanceApi/submit"):
			calls = append(calls, "submit")
			lastSubmit = body
			_, _ = w.Write([]byte(`{"isSuccess":true,"data":{"id":"instance-1"}}`))
		default:
			t.Fatalf("未预期端点：%s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := mustClient(t, server.URL)
	spec, ok := target.LookupSpecialBusiness("cost_funds_transactions")
	if !ok || spec.Status != target.SpecialBusinessImplemented {
		t.Fatal("资金往来必须登记为已实现")
	}
	form := json.RawMessage(`{
		"requestDepName":"{\"id\":\"dep-1\",\"name\":\"请款部门\"}",
		"requestUserName":"{\"id\":\"user-1\",\"name\":\"请款人\"}",
		"remark":"备注",
		"applicationFundsVo_payCompanyId":"pay-co",
		"applicationFundsVo_payCompanyName":"付款公司",
		"applicationFundsVo_payMethod":"转账",
		"applicationFundsVo_payMoney":"100.00",
		"applicationFundsVo_payName":"收款人",
		"applicationFundsVo_openingBank":"开户行",
		"applicationFundsVo_account":"账号",
		"applicationFundsVo_paymentAccount":"付款账号"
	}`)
	saved, err := client.SaveSpecialBusiness(context.Background(), specialSession(), spec, form, "submit")
	if err != nil {
		t.Fatal(err)
	}
	if saved.BusinessID != "biz-funds-1" {
		t.Fatalf("业务保存必须返回 id：%+v", saved)
	}
	submit := target.BuildSubmitBody(target.SubmitFlowInstanceRequest{
		Name: "计划-路径-时间", FormProxyID: "form-1", CompanyID: "company-1",
		FormData: json.RawMessage(`{"remark":"备注"}`), BatchCode: "batch-32-chars-xxxxxxxxxxxxxxxx",
		BizRelevance: target.AppendBusinessRelevance(nil, target.SpecialBusinessOtherBiz(spec.FlowType), saved.BusinessID),
	})
	if err := client.CallWriteForTest(context.Background(), target.WriteEndpointSubmit, "sid-special", submit); err != nil {
		t.Fatal(err)
	}
	if strings.Join(calls, ">") != "save>submit" {
		t.Fatalf("资金往来必须先保存业务再发主流程：%v", calls)
	}
	data := lastSave["data"].(map[string]any)
	if data["type"] != "1" || data["status"] != "1" || data["examineStatus"] != "0" {
		t.Fatalf("资金往来业务保存形状错误：%v", data)
	}
	if data["depId"] != "dep-1" || data["expenseUserId"] != "user-1" {
		t.Fatalf("请款部门/人员必须从选择器 JSON 取出 id：%v", data)
	}
	if _, exists := lastSave["batchCode"]; exists {
		t.Fatal("业务保存不得携带主流程 batchCode")
	}
	submitData := lastSubmit["data"].(map[string]any)
	if submitData["formProxyId"] != "form-1" {
		t.Fatal("FormMaking 主流程必须发 formProxyId")
	}
	if _, exists := submitData["flowProxyId"]; exists {
		t.Fatal("FormMaking 主流程禁止同时发 flowProxyId")
	}
	list, _ := submitData["flowInstanceBizRelevanceList"].([]any)
	if len(list) == 0 {
		t.Fatal("主流程必须带上业务 id 关联")
	}
}

// TestCostFundsInvestUsesType2 锁定投资款业务保存 type=2。
func TestCostFundsInvestUsesType2(t *testing.T) {
	var saved map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &saved)
		_, _ = w.Write([]byte(`{"isSuccess":true,"data":{"id":"biz-invest-1"}}`))
	}))
	defer server.Close()
	client := mustClient(t, server.URL)
	spec, _ := target.LookupSpecialBusiness("cost_funds_invest")
	_, err := client.SaveSpecialBusiness(context.Background(), specialSession(), spec, json.RawMessage(`{}`), "submit")
	if err != nil {
		t.Fatal(err)
	}
	data := saved["data"].(map[string]any)
	if data["type"] != "2" {
		t.Fatalf("投资款 type 必须为 2：%v", data)
	}
}

// TestCostFundsSaveFailureBlocksMainSubmit 锁定业务保存失败不得继续主流程。
func TestCostFundsSaveFailureBlocksMainSubmit(t *testing.T) {
	var submitCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "costFundsTransactions/save") {
			_, _ = w.Write([]byte(`{"isSuccess":false,"message":"业务保存被目标拒绝"}`))
			return
		}
		submitCalls++
		_, _ = w.Write([]byte(`{"isSuccess":true,"data":{"id":"instance-1"}}`))
	}))
	defer server.Close()
	client := mustClient(t, server.URL)
	spec, _ := target.LookupSpecialBusiness("cost_funds_transactions")
	_, err := client.SaveSpecialBusiness(context.Background(), specialSession(), spec, json.RawMessage(`{}`), "submit")
	if err == nil {
		t.Fatal("业务保存失败必须返回错误")
	}
	if submitCalls != 0 {
		t.Fatal("业务保存失败后不得继续主流程")
	}
}

// TestExistingBusinessRelevanceSkipsDuplicateSave 锁定已有同类型业务 id 不得重复追加。
func TestExistingBusinessRelevanceSkipsDuplicateSave(t *testing.T) {
	values := []target.BizRelevance{{OtherBiz: "cost_funds_transactions", OtherBizID: "biz-funds-1"}}
	if !target.HasBusinessRelevance(values, "cost_funds_transactions") {
		t.Fatal("已有同类型非空 id 必须判定已关联")
	}
	appended := target.AppendBusinessRelevance(values, "cost_funds_transactions", "biz-funds-1")
	if len(appended) != 1 {
		t.Fatalf("已有同类型 id 不得重复追加：%v", appended)
	}
}

// TestPublicationCommissionApproveRewritesManageUserDate 锁定出版委托同意改写项目经理日期。
func TestPublicationCommissionApproveRewritesManageUserDate(t *testing.T) {
	runCtx := step.RunContext{FlowType: "publication_commission"}
	compiled := model.CompiledActionStep{Action: model.ActionApprove}
	session := target.Session{UserID: "user-pm"}
	form := json.RawMessage(`{"manageUserName":"{\"id\":\"user-pm\",\"name\":\"项目经理\"}","manageUserDate":""}`)
	rewritten := step.ApplySpecialBusinessFormDataForTest(runCtx, compiled, session, form)
	values, err := decodeObject(rewritten)
	if err != nil {
		t.Fatal(err)
	}
	date, _ := values["manageUserDate"].(string)
	if date == "" || len(date) != 10 {
		t.Fatalf("出版委托同意且当前用户是项目经理时必须写入 YYYY-MM-DD：%v", values)
	}
}

// TestContractSealReviewRemainsBlocked 锁定未实现合同盖章写前阻塞。
func TestContractSealReviewRemainsBlocked(t *testing.T) {
	reason := target.SpecialBusinessBlockReason("contract_seal_review", "submit")
	if reason == "" {
		t.Fatal("未实现的合同盖章必须写前阻塞")
	}
	if !strings.Contains(reason, "不能发通用请求顶替") {
		t.Fatalf("阻塞文案必须禁止通用 submit 顶替：%s", reason)
	}
}

// TestUnimplementedSpecialBusinessSaveRejected 锁定未实现分支不能发业务保存。
func TestUnimplementedSpecialBusinessSaveRejected(t *testing.T) {
	client := mustClient(t, "http://127.0.0.1:9")
	spec, _ := target.LookupSpecialBusiness("contract_seal_review")
	_, err := client.SaveSpecialBusiness(context.Background(), specialSession(), spec, json.RawMessage(`{}`), "submit")
	if err == nil || !strings.Contains(err.Error(), "尚未实现") {
		t.Fatalf("未实现分支保存必须拒绝：%v", err)
	}
}

// mustClient 创建指向假目标的写客户端。
func mustClient(t *testing.T, baseURL string) *target.Client {
	t.Helper()
	client, err := target.NewClient(target.ClientConfig{
		BaseURL: baseURL, Timeout: time.Second, CustomerCode: "cust-1", PlatformCode: "200001",
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

// decodeObject 解码表单 JSON 对象。
func decodeObject(raw json.RawMessage) (map[string]any, error) {
	var values map[string]any
	err := json.Unmarshal(raw, &values)
	return values, err
}
