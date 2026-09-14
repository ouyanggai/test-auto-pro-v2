package protocol_test

import (
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
)

// F-035 评审补充：目标页面按流程类型分派特殊业务钩子（FlowDialog.formMakingFormBusiness、
// EnterpriseExamineOpinion.handleSubmitCheck）。工具无法执行自定义组件与业务数据变更，
// 命中登记清单的类型必须阻塞；未登记的类型走通用 submit 页面路径。

// TestSpecialBusinessLifecycleRegistry 锁定登记内容与源码分支一一对应：
// 合同合规/合同盖章（自定义组件）、资金往来/投资款（业务数据保存）、
// 出版委托/专业提资（审批改写日期）、年度绩效/考核（审批写意见）、费用报销（审批金额计算）。
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
	// 普通流程类型（走通用 enterpriseHandleSubmit 提交路径）不得被误登记。
	for _, plain := range []string{"", "contract_review", "vacate", "monthly_perf_reserveTalent"} {
		if target.HasSpecialBusinessLifecycle(plain) {
			t.Fatalf("普通流程类型被误判为特殊业务：%q", plain)
		}
	}
}

// TestCollectFormPersonFields 锁定目标 traverseFlowNode 递归收集规则：
// auditType=form_person 节点按 formPersonFields 声明人员选择器字段（逗号分隔），
// 条件分支与并行分支子树同样参与递归，字段按节点去重合并。
func TestCollectFormPersonFields(t *testing.T) {
	tree := &target.FlowNodeTemplate{
		ID: "node-1", AuditConfig: &target.FlowNodeAuditConfig{AuditType: "form_person", FormPersonField: " myUserName__formPersonId , myDepName__formPersonId "},
		Child: &target.FlowNodeTemplate{
			ID: "node-2", AuditConfig: &target.FlowNodeAuditConfig{AuditType: "company"},
			ConditionNodes: []target.FlowBranchTemplate{
				{Child: &target.FlowNodeTemplate{ID: "node-3", AuditConfig: &target.FlowNodeAuditConfig{AuditType: "form_person", FormPersonField: "approverName__formPersonId"}}},
			},
			ParallelNodes: []target.FlowBranchTemplate{
				{Child: &target.FlowNodeTemplate{ID: "node-4", AuditConfig: &target.FlowNodeAuditConfig{AuditType: "form_person", FormPersonField: "parallelUser__formPersonId"}}},
			},
		},
	}
	fields := target.CollectFormPersonFields(tree)
	if len(fields) != 4 {
		t.Fatalf("应收集到 4 个节点声明的人员字段：%+v", fields)
	}
	byNode := map[string][]string{}
	for _, field := range fields {
		byNode[field.NodeID] = append(byNode[field.NodeID], field.Field)
	}
	if len(byNode["node-1"]) != 2 || byNode["node-1"][0] != "myUserName__formPersonId" || byNode["node-1"][1] != "myDepName__formPersonId" {
		t.Fatalf("多字段/空白清理不正确：%+v", byNode["node-1"])
	}
	if _, ok := byNode["node-2"]; ok {
		t.Fatal("非 form_person 节点不得收集")
	}
	if len(byNode["node-3"]) != 1 || len(byNode["node-4"]) != 1 {
		t.Fatalf("条件/并行分支子树必须参与递归：%+v", byNode)
	}
	if got := target.CollectFormPersonFields(nil); len(got) != 0 {
		t.Fatalf("空树应返回空：%+v", got)
	}
}

// TestApplyFormPersonFieldRule 锁定目标字段落值规则（FlowDialog.traverseFlowNode / 表单人员字段处理）：
// JSON 文本取 id；纯文本取原值；声明字段已有值或源字段缺失时不产出、不伪造。
func TestApplyFormPersonFieldRule(t *testing.T) {
	field := target.NodeFormPersonField{NodeID: "node-1", Field: "approverName__formPersonId"}
	// JSON 文本取 id。
	values := map[string]any{"approverName": `{"id":"user-9","name":"张三"}`}
	if !target.ApplyFormPersonFieldRule(values, field) || values["approverName__formPersonId"] != "user-9" {
		t.Fatalf("JSON 源应解析出真实用户 ID：%v", values)
	}
	// 纯文本取原值。
	values = map[string]any{"approverName": "张三"}
	if !target.ApplyFormPersonFieldRule(values, field) || values["approverName__formPersonId"] != "张三" {
		t.Fatalf("纯文本源应取原值：%v", values)
	}
	// 已有值不覆盖。
	values = map[string]any{"approverName": "张三", "approverName__formPersonId": "already-set"}
	if target.ApplyFormPersonFieldRule(values, field) || values["approverName__formPersonId"] != "already-set" {
		t.Fatalf("已有值不得覆盖：%v", values)
	}
	// 源字段缺失不产出、不伪造。
	values = map[string]any{"other": 1}
	if target.ApplyFormPersonFieldRule(values, field) {
		t.Fatalf("源字段缺失不得产出：%v", values)
	}
	if _, exists := values["approverName__formPersonId"]; exists {
		t.Fatalf("不得伪造人员字段：%v", values)
	}
}
