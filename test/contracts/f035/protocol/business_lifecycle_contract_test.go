package protocol_test

import (
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
)

func TestSpecialBusinessLifecycleRegistry(t *testing.T) {
	special := map[string]bool{
		"contract_compliance_review": true, "contract_seal_review": true,
		"cost_funds_transactions": true, "cost_funds_invest": true,
		"publication_commission": true, "profession_indirect_provide": true,
		"staff_annual_performance": true, "staff_annual_assessment": true,
		"expense_budget": true,
	}
	registered := target.SpecialBusinessFlowTypes()
	if len(registered) != len(special) {
		t.Fatalf("登记清单与评审依据不一致：%v", registered)
	}
	for _, flowType := range registered {
		if !special[flowType] {
			t.Fatalf("登记了评审依据之外的流程类型：%s", flowType)
		}
		if !target.HasSpecialBusinessLifecycle(flowType) {
			t.Fatalf("登记类型未命中判定：%s", flowType)
		}
	}
	for _, plain := range []string{"", "contract_review", "vacate", "monthly_perf_reserveTalent"} {
		if target.HasSpecialBusinessLifecycle(plain) {
			t.Fatalf("普通流程类型被误判为特殊业务：%q", plain)
		}
	}
	if reason := target.SpecialBusinessBlockReason("contract_seal_review", "submit"); reason == "" {
		t.Fatal("未实现的合同盖章必须写前阻塞")
	}
	if reason := target.SpecialBusinessBlockReason("publication_commission", "approve"); reason != "" {
		t.Fatalf("已实现的出版委托不得统一阻塞：%s", reason)
	}
	if !target.NeedsSpecialBusinessPreSave("cost_funds_transactions", "submit") {
		t.Fatal("资金往来发起必须先保存业务")
	}
	if target.NeedsSpecialBusinessPreSave("cost_funds_transactions", "approve") {
		t.Fatal("资金往来审批不得再发前置保存")
	}
	if target.NeedsSpecialBusinessPreSave("publication_commission", "submit") {
		t.Fatal("出版委托没有独立业务保存接口")
	}
	if !target.NeedsNoFormFlowPreSave("contract_review", "submit") {
		t.Fatal("合同评审无表单页必须先 mixin.saveData")
	}
	if target.NeedsNoFormFlowPreSave("NoFormFlow", "submit") {
		t.Fatal("未实现的通用无表单入口不得假装已有前置保存")
	}
	if got := target.SpecialBusinessOtherBiz("cost_funds_invest"); got != "cost_funds_invest" {
		t.Fatalf("投资款 otherBiz 必须与目标 selectFlowType 一致：%s", got)
	}
	if got := target.NoFormFlowOtherBiz("buy_plan"); got != "buy_plan" {
		t.Fatalf("采购计划 otherBiz 必须用 componentName：%s", got)
	}

}

func TestCollectFormPersonFieldsByAction(t *testing.T) {
	tree := &target.FlowNodeTemplate{
		ID: "start",
		Child: &target.FlowNodeTemplate{
			ID:          "node-1",
			AuditConfig: &target.FlowNodeAuditConfig{AuditType: "form_person", FormPersonField: "myUserName__formPersonId, myDepName__formPersonId"},
			Child: &target.FlowNodeTemplate{
				ID:          "node-2",
				AuditConfig: &target.FlowNodeAuditConfig{AuditType: "company"},
				ConditionNodes: []target.FlowBranchTemplate{{
					Child: &target.FlowNodeTemplate{
						ID:          "node-3",
						AuditConfig: &target.FlowNodeAuditConfig{AuditType: "form_person", FormPersonField: "nextApprover__formPersonId"},
					},
				}},
				ParallelNodes: []target.FlowBranchTemplate{{
					Child: &target.FlowNodeTemplate{
						ID:          "node-4",
						AuditConfig: &target.FlowNodeAuditConfig{AuditType: "form_person", FormPersonField: "parallelApprover__formPersonId"},
					},
				}},
			},
		},
	}
	all := target.CollectFormPersonFields(tree)
	if len(all) != 4 {
		t.Fatalf("全树应收集 4 个字段：%+v", all)
	}
	approval := target.CollectFormPersonFieldsForAction(tree, "approve", "vacate", "node-3")
	if len(approval) != 1 || approval[0].Field != "nextApprover__formPersonId" || approval[0].Scope != target.FormPersonScopeApprovalNextEntry {
		t.Fatalf("审批只应收下一入口：%+v", approval)
	}
	initiate := target.CollectFormPersonFieldsForAction(tree, "submit", "staff_annual_performance", "")
	if len(initiate) != 4 {
		t.Fatalf("年度绩效发起必须全树：%+v", initiate)
	}
}

func TestApplyFormPersonFieldRule(t *testing.T) {
	field := target.NodeFormPersonField{NodeID: "node-1", Field: "approverName__formPersonId", Scope: target.FormPersonScopeApprovalNextEntry}
	values := map[string]any{"approverName": `{"id":"user-9","name":"张三"}`}
	if !target.ApplyFormPersonFieldRule(values, field) || values["approverName__formPersonId"] != "user-9" {
		t.Fatalf("JSON 源应解析出真实用户 ID：%v", values)
	}
	values = map[string]any{"approverName": "张三"}
	if !target.ApplyFormPersonFieldRule(values, field) || values["approverName__formPersonId"] != "张三" {
		t.Fatalf("纯文本源应取原值：%v", values)
	}
	values = map[string]any{"approverName": "张三", "approverName__formPersonId": "already-set"}
	if target.ApplyFormPersonFieldRule(values, field) || values["approverName__formPersonId"] != "already-set" {
		t.Fatalf("已有值不得覆盖：%v", values)
	}
	values = map[string]any{"other": 1}
	if target.ApplyFormPersonFieldRule(values, field) {
		t.Fatalf("源字段缺失不得产出：%v", values)
	}
}

func TestUnregisteredEndpointRejectsWrite(t *testing.T) {
	err := target.ValidateBodyMatrix("/web/unknown", map[string]any{"sid": "s"}, "cust")
	if err == nil || !strings.Contains(err.Error(), "未登记协议矩阵") {
		t.Fatalf("未登记端点必须拒绝：%v", err)
	}
}
