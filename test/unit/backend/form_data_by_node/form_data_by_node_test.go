package form_data_by_node_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// F-035/T05 分节点表单数据：发起节点只发发起节点可填字段，后续节点字段禁止提前写入；
// 有实例时以目标实例当前完整数据为基线，只覆盖当前节点可编辑字段并保留其他节点已填值。

// nodeFormRunContext 提供两节点权限与历史配置，供当前基线构造回归使用。
func nodeFormRunContext() step.RunContext {
	return step.RunContext{
		EffectiveFormData: []byte(`{"amount":100,"reason":"发起填写","managerMemo":"审批节点才能填"}`),
		Nodes: map[string]step.NodeInfo{
			"start": {EditableFields: []string{"amount", "reason"}},
			"audit": {EditableFields: []string{"managerMemo"}},
		},
		NodeEditableFields: map[string][]string{
			"start": {"amount", "reason"},
			"audit": {"managerMemo"},
		},
	}
}

// TestSubmitKeepsLaterNodeFieldsWithheld 锁定：发起载荷不含后续节点专属字段。
func TestSubmitKeepsLaterNodeFieldsWithheld(t *testing.T) {
	runCtx := nodeFormRunContext()
	steps := []model.CompiledActionStep{{Sequence: 1, Action: model.ActionSubmit, NodeKey: "start"}}
	plan, err := step.BuildNodeFormData(runCtx, steps[0], nil, false)
	_ = steps
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(plan.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if _, exists := payload["managerMemo"]; exists {
		t.Fatal("后续节点字段不得在发起请求中提前写入")
	}
	if payload["amount"] != float64(100) || payload["reason"] != "发起填写" {
		t.Fatalf("发起节点可填字段必须进入载荷：%v", payload)
	}
	if !contains(plan.Withheld, "managerMemo") {
		t.Fatalf("被扣留字段必须可追溯：%v", plan.Withheld)
	}
}

// TestAuditPreservesPreviousNodeValues 锁定：审批节点以实例当前数据为基线，
// 上一节点写入的值被保留，当前节点只覆盖自身可编辑字段。
func TestAuditPreservesPreviousNodeValues(t *testing.T) {
	runCtx := nodeFormRunContext()
	instanceCurrent := map[string]any{
		"amount": float64(100), "reason": "发起填写", "managerMemo": "实例旧值",
	}
	compiled := model.CompiledActionStep{Sequence: 2, Action: model.ActionApprove, NodeKey: "audit"}
	plan, err := step.BuildNodeFormData(runCtx, compiled, instanceCurrent, true)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(plan.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["amount"] != float64(100) || payload["reason"] != "发起填写" {
		t.Fatal("节点 1 写入的值必须在节点 2 请求中被保留")
	}
	// managerMemo 是当前节点声明可编辑且已配置的字段：按配置值覆盖实例旧值；
	// 非本节点字段（amount/reason）只能保留。
	if payload["managerMemo"] != "审批节点才能填" {
		t.Fatalf("当前节点可编辑且已配置的字段必须按配置覆盖：%v", payload["managerMemo"])
	}
	if !contains(plan.Overlaid, "managerMemo") {
		t.Fatalf("managerMemo 必须落在覆盖或扣留清单：%+v", plan)
	}
}

// TestInstanceBaselineNeverFallsBackToInitiation 锁定：实例存在但数据为空时保持空基线，
// 绝不退回发起态整份历史配置（历史快照覆盖上游已填内容）。
func TestInstanceBaselineNeverFallsBackToInitiation(t *testing.T) {
	runCtx := nodeFormRunContext()
	runCtx.EffectiveFormData = []byte(`{"amount":100,"reason":"r"}`)
	compiled := model.CompiledActionStep{Sequence: 2, Action: model.ActionApprove, NodeKey: "audit"}
	plan, err := step.BuildNodeFormData(runCtx, compiled, map[string]any{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.BaseFromInstance {
		t.Fatal("有实例时基线必须来自实例")
	}
	var payload map[string]any
	if len(plan.Payload) > 0 {
		if err := json.Unmarshal(plan.Payload, &payload); err != nil {
			t.Fatal(err)
		}
	}
	if reflect.DeepEqual(payload, map[string]any{"amount": float64(100), "reason": "r"}) {
		t.Fatal("空实例基线被发起态历史配置覆盖")
	}
}

// contains 检查字段决策清单。
func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

// TestNodeFormDataDecisionRecordsOwnership 锁定 F-035/T05 决策记录：
// 每个节点的基线来源、覆盖/保留/扣留字段与最终 JSON 指纹都可追溯，扣留字段绝不出现在载荷。
func TestNodeFormDataDecisionRecordsOwnership(t *testing.T) {
	runCtx := nodeFormRunContext()
	plan, err := step.BuildNodeFormData(runCtx, model.CompiledActionStep{Sequence: 1, Action: model.ActionSubmit, NodeKey: "start"}, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	d := plan.Decision
	if d == nil {
		t.Fatal("决策记录缺失")
	}
	if d.BaselineSource != "initiation" {
		t.Fatalf("发起节点基线应为 initiation：%s", d.BaselineSource)
	}
	if !contains(d.WithheldFields, "managerMemo") {
		t.Fatalf("后续节点专属字段必须列入 Withheld：%v", d.WithheldFields)
	}
	if d.PayloadFingerprint == "" || len(d.PayloadFingerprint) != 64 {
		t.Fatalf("最终载荷指纹必须是 SHA-256 hex：%q", d.PayloadFingerprint)
	}
}

// TestTargetCurrentDataIsAuthoritative 验证目标删除、规范化、联动计算及嵌套修改均以最新实例为准，不建立跨节点不变性门禁。
func TestTargetCurrentDataIsAuthoritative(t *testing.T) {
	for _, current := range []map[string]any{
		{"amount": float64(100)},
		{"amount": float64(0), "reason": "原生规范值", "currentDepName": "当前部门"},
		{"detail": []any{map[string]any{"money": float64(999)}}},
	} {
		runCtx := nodeFormRunContext()
		plan, err := step.BuildNodeFormData(runCtx, model.CompiledActionStep{Sequence: 2, Action: model.ActionApprove, NodeKey: "audit"}, current, true)
		if err != nil {
			t.Fatal(err)
		}
		var payload map[string]any
		if err := json.Unmarshal(plan.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		for key, value := range current {
			if !reflect.DeepEqual(payload[key], value) {
				t.Fatalf("目标当前值必须保留：%s=%v，实际 %v", key, value, payload[key])
			}
		}
		if _, exists := current["reason"]; !exists {
			if _, restored := payload["reason"]; restored {
				t.Fatal("不得恢复上游历史字段")
			}
		}
		if !plan.BaseFromInstance || payload["managerMemo"] != "审批节点才能填" {
			t.Fatalf("必须保留基线与当前授权补丁：%v", payload)
		}
	}
}

// TestCNYAmountReadbackOmitsUnusedField 验证本次线上人民币合同场景：目标省略未使用的人民币折算字段时可继续使用当前数据。
func TestCNYAmountReadbackOmitsUnusedField(t *testing.T) {
	runCtx := step.RunContext{EffectiveFormData: []byte(`{"currency":"CNY","contractSum":5000004,"contractAmountRmb":null}`),
		Nodes:              map[string]step.NodeInfo{"start": {EditableFields: []string{"currency", "contractSum", "contractAmountRmb"}}, "audit": {}},
		NodeEditableFields: map[string][]string{"start": {"currency", "contractSum", "contractAmountRmb"}}}
	plan, err := step.BuildNodeFormData(runCtx, model.CompiledActionStep{Sequence: 2, Action: model.ActionApprove, NodeKey: "audit"},
		map[string]any{"currency": "CNY", "contractSum": json.Number("5000004")}, true)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(plan.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if _, exists := payload["contractAmountRmb"]; exists {
		t.Fatal("不得按历史快照补回目标省略的字段")
	}
	if payload["currency"] != "CNY" || payload["contractSum"] != float64(5000004) {
		t.Fatalf("目标人民币合同值改变：%v", payload)
	}
}

// TestAlignSubformColumnControlFields 锁定通用规则：以 _col 结尾的子表单列控制占位字段若为空，
// 自动从子表单明细对应列获取非空值，避免因历史空值导致目标原生表单校验失败。
func TestAlignSubformColumnControlFields(t *testing.T) {
	runCtx := step.RunContext{
		EffectiveFormData: []byte(`{
			"businessType_col": "",
			"details": [
				{"businessType": "001", "money": 100}
			]
		}`),
		Nodes: map[string]step.NodeInfo{
			"start": {EditableFields: []string{"businessType_col", "details"}},
		},
		NodeEditableFields: map[string][]string{
			"start": {"businessType_col", "details"},
		},
	}
	steps := []model.CompiledActionStep{{Sequence: 1, Action: model.ActionSubmit, NodeKey: "start"}}
	plan, err := step.BuildNodeFormData(runCtx, steps[0], nil, false)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(plan.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["businessType_col"] != "001" {
		t.Fatalf("businessType_col 必须自动对齐子表单同名列值 '001'，实际为：%v", payload["businessType_col"])
	}
}
