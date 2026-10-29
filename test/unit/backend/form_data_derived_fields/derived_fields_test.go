package form_data_derived_fields_test

import (
	"encoding/json"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

func TestClassifyTargetDerivedFieldKinds(t *testing.T) {
	text := target.ClassifyTargetDerivedField("auto_audit_info_1")
	if text.Ownership != target.FieldOwnershipTargetDerived || text.DerivedKind != target.DerivedFieldKindText {
		t.Fatalf("文本字段分类错误：%+v", text)
	}
	object := target.ClassifyTargetDerivedField("auto_audit_info_obj_1")
	if object.DerivedKind != target.DerivedFieldKindObject || object.TextKey != "auto_audit_info_1" {
		t.Fatalf("对象字段必须先精确匹配 obj_：%+v", object)
	}
	list := target.ClassifyTargetDerivedField("auto_audit_info_obj_list_1")
	if list.DerivedKind != target.DerivedFieldKindList || len(list.CompanionKeys) != 3 {
		t.Fatalf("列表字段必须带三类伴生键：%+v", list)
	}
	if target.IsTargetDerivedField("auto_audit_info") || target.IsTargetDerivedField("reason") {
		t.Fatal("缺少序号或普通业务字段不得判为衍生字段")
	}
}

func TestDerivedFieldClearedByTool(t *testing.T) {
	if !target.DerivedFieldClearedByTool("刘志阳 同意 2026-09-14 15:19", "") {
		t.Fatal("目标非空意见被写空必须判定为工具破坏")
	}
	if target.DerivedFieldClearedByTool("", "刘志阳 同意 2026-09-14 15:19") {
		t.Fatal("空到目标生成意见是正常衍生，不得判破坏")
	}
	if target.IsEmptyDerivedValue(map[string]any{"auditName": "刘志阳"}) {
		t.Fatal("非空对象不是空衍生值")
	}
	if !target.IsEmptyDerivedValue([]any{}) {
		t.Fatal("空列表是空衍生值")
	}
}

// TestCrossNodeDerivedOpinionDoesNotBlock 验证普通审批保留目标当前生成的审核意见。
func TestCrossNodeDerivedOpinionDoesNotBlock(t *testing.T) {
	runCtx := step.RunContext{
		EffectiveFormData: []byte(`{"reason":"发起填写","auto_audit_info_1":""}`),
		Nodes: map[string]step.NodeInfo{
			"start": {EditableFields: []string{"reason"}},
			"audit": {EditableFields: []string{"managerMemo"}},
		},
		NodeEditableFields: map[string][]string{
			"start": {"reason"},
			"audit": {"managerMemo"},
		},
	}
	plan, err := step.BuildNodeFormData(runCtx, model.CompiledActionStep{Sequence: 2, Action: model.ActionApprove, NodeKey: "audit"}, map[string]any{
		"reason":                "发起填写",
		"auto_audit_info_1":     "刘志阳 同意 2026-09-14 15:19",
		"auto_audit_info_obj_1": map[string]any{"auditName": "刘志阳"},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(plan.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["auto_audit_info_1"] != "刘志阳 同意 2026-09-14 15:19" {
		t.Fatalf("不得用历史空值覆盖当前意见：%v", payload)
	}

}

// TestCurrentEmptyOpinionIsNotReplaced 验证目标当前空意见不会被历史意见复原，也不会按历史值阻塞。
func TestCurrentEmptyOpinionIsNotReplaced(t *testing.T) {
	runCtx := step.RunContext{EffectiveFormData: []byte(`{"auto_audit_info_1":"历史非空意见"}`), Nodes: map[string]step.NodeInfo{"audit": {}}}
	plan, err := step.BuildNodeFormData(runCtx, model.CompiledActionStep{Sequence: 3, Action: model.ActionApprove, NodeKey: "audit"}, map[string]any{"auto_audit_info_1": ""}, true)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(plan.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["auto_audit_info_1"] != "" {
		t.Fatalf("必须保留目标当前意见：%v", payload)
	}
}

// TestResubmitResetsApprovalOpinionFamilies 锁定重新发起的业务语义：审核意见及已返回的伴生结构必须显式清空，
// 且不能被上一节点“禁止工具清空衍生字段”的保护规则拦截。
func TestResubmitResetsApprovalOpinionFamilies(t *testing.T) {
	runCtx := step.RunContext{
		EffectiveFormData:  []byte(`{"reason":"保留业务值","auto_audit_info_1":"配置阶段旧意见"}`),
		Nodes:              map[string]step.NodeInfo{"start": {EditableFields: []string{"reason"}}},
		NodeEditableFields: map[string][]string{"start": {"reason"}},
	}
	plan, err := step.BuildNodeFormData(runCtx, model.CompiledActionStep{
		Sequence: 4, Action: model.ActionResubmit, NodeKey: "start",
	}, map[string]any{
		"reason":                     "保留业务值",
		"auto_audit_info_1":          "上一轮审批意见",
		"auto_audit_info_obj_1":      map[string]any{"auditName": "审批人"},
		"auto_audit_info_obj_list_1": []any{map[string]any{"opinion": "同意"}},
		"auto_audit_info_2":          "另一条意见",
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(plan.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["reason"] != "保留业务值" {
		t.Fatalf("普通业务字段必须保留：%v", payload["reason"])
	}
	for _, key := range []string{"auto_audit_info_1", "auto_audit_info_2"} {
		if value, ok := payload[key].(string); !ok || value != "" {
			t.Fatalf("审核意见文本字段必须清空：%s=%#v", key, payload[key])
		}
		if !containsField(plan.Overlaid, key) {
			t.Fatalf("重置字段必须进入覆盖清单：%s -> %v", key, plan.Overlaid)
		}
	}
	if object, ok := payload["auto_audit_info_obj_1"].(map[string]any); !ok || len(object) != 0 {
		t.Fatalf("审核意见对象字段必须清空：%#v", payload["auto_audit_info_obj_1"])
	}
	if list, ok := payload["auto_audit_info_obj_list_1"].([]any); !ok || len(list) != 0 {
		t.Fatalf("审核意见列表字段必须清空：%#v", payload["auto_audit_info_obj_list_1"])
	}
}

// containsField 判断字段清单是否包含指定键。
func containsField(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
