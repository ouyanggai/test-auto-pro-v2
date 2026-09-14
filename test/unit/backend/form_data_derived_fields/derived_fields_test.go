package form_data_derived_fields_test

import (
	"encoding/json"
	"strings"
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
	previous := &step.NodeFormDataDecision{
		OverlaidFields: []string{"reason", "auto_audit_info_1"},
		OverlaidValues: map[string]any{"reason": "发起填写", "auto_audit_info_1": ""},
	}
	plan, err := step.BuildNodeFormData(runCtx, model.CompiledActionStep{Sequence: 2, Action: model.ActionApprove, NodeKey: "audit"}, map[string]any{
		"reason":            "发起填写",
		"auto_audit_info_1": "刘志阳 同意 2026-09-14 15:19",
		"auto_audit_info_obj_1": map[string]any{"auditName": "刘志阳"},
	}, true, previous)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Decision.ValidationIssues) != 0 {
		t.Fatalf("目标生成审批意见不得阻塞：%v", plan.Decision.ValidationIssues)
	}
}

func TestToolClearingDerivedOpinionBlocks(t *testing.T) {
	previous := &step.NodeFormDataDecision{
		OverlaidFields: []string{"auto_audit_info_1"},
		OverlaidValues: map[string]any{"auto_audit_info_1": "刘志阳 同意 2026-09-14 15:19"},
	}
	runCtx := step.RunContext{
		EffectiveFormData: []byte(`{"auto_audit_info_1":""}`),
		Nodes:             map[string]step.NodeInfo{"audit": {}},
	}
	plan, err := step.BuildNodeFormData(runCtx, model.CompiledActionStep{Sequence: 3, Action: model.ActionApprove, NodeKey: "audit"}, map[string]any{
		"auto_audit_info_1": "",
	}, true, previous)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(plan.Decision.ValidationIssues, "；")
	if !strings.Contains(joined, "被工具写空") {
		t.Fatalf("工具写空目标非空意见必须阻塞：%s", joined)
	}
	_ = json.RawMessage(nil)
}
