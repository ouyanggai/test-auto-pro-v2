package form_data_by_node_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// F-035/T05 分节点表单数据：发起节点只发发起节点可填字段，后续节点字段禁止提前写入；
// 有实例时以目标实例当前完整数据为基线，只覆盖当前节点可编辑字段并保留其他节点已填值。

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
	plan, err := step.BuildNodeFormData(runCtx, steps[0], nil, false, nil)
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
	plan, err := step.BuildNodeFormData(runCtx, compiled, instanceCurrent, true, nil)
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
	plan, err := step.BuildNodeFormData(runCtx, compiled, map[string]any{}, true, nil)
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
	plan, err := step.BuildNodeFormData(runCtx, model.CompiledActionStep{Sequence: 1, Action: model.ActionSubmit, NodeKey: "start"}, nil, false, nil)
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
	if len(d.ValidationIssues) != 0 {
		t.Fatalf("合法构造不应有校验问题：%v", d.ValidationIssues)
	}
}

// TestCrossNodeLossBlocks 锁定跨节点核对：上一节点覆盖字段在目标实例上丢失时，
// 当前节点构造必须带校验问题并阻塞，绝不能带着丢值继续写。
func TestCrossNodeLossBlocks(t *testing.T) {
	runCtx := nodeFormRunContext()
	previous := &step.NodeFormDataDecision{
		StepNo: 1, NodeKey: "start", Action: "submit", BaselineSource: "initiation",
		OverlaidFields: []string{"reason"}, WithheldFields: []string{"managerMemo"},
	}
	// 实例当前数据缺少 reason（上一节点写入的值被目标/他人覆盖或丢失）。
	instanceCurrent := map[string]any{"amount": float64(100), "managerMemo": "x"}
	plan, err := step.BuildNodeFormData(runCtx,
		model.CompiledActionStep{Sequence: 2, Action: model.ActionApprove, NodeKey: "audit"},
		instanceCurrent, true, previous)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Decision.ValidationIssues) == 0 {
		t.Fatal("上一节点覆盖字段丢失必须产生校验问题并阻塞")
	}
	if !strings.Contains(plan.Decision.ValidationIssues[0], "reason") {
		t.Fatalf("校验问题必须指明丢失字段：%v", plan.Decision.ValidationIssues)
	}
}

// TestCrossNodePreservedPasses 锁定跨节点核对通过路径：上一节点值仍在时无校验问题。
func TestCrossNodePreservedPasses(t *testing.T) {
	runCtx := nodeFormRunContext()
	previous := &step.NodeFormDataDecision{
		StepNo: 1, NodeKey: "start", Action: "submit", BaselineSource: "initiation",
		OverlaidFields: []string{"reason", "amount"}, WithheldFields: []string{"managerMemo"},
	}
	instanceCurrent := map[string]any{"amount": float64(100), "reason": "发起填写"}
	plan, err := step.BuildNodeFormData(runCtx,
		model.CompiledActionStep{Sequence: 2, Action: model.ActionApprove, NodeKey: "audit"},
		instanceCurrent, true, previous)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Decision.ValidationIssues) != 0 {
		t.Fatalf("上一节点值仍在时不应阻塞：%v", plan.Decision.ValidationIssues)
	}
	if !plan.BaseFromInstance {
		t.Fatal("有实例时基线必须来自实例")
	}
}
