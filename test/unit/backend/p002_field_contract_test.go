package backend_test

import (
	"reflect"
	"testing"

	"test-auto-pro-v2/internal/formdata"
	"test-auto-pro-v2/internal/service"
)

// TestP002VueFieldsExposeCompleteRoundTripContract 验证真实 Vue 页面字段都公开值形态、序列化、候选、校验和证据。
func TestP002VueFieldsExposeCompleteRoundTripContract(t *testing.T) {
	total := 0
	for _, pageKey := range []string{"company_annual_budget", "request_funds", "loan", "travel_expense", "contract_review"} {
		page := service.AnalyzeVueCustomPageRule(f010ProjectRoot(t), pageKey, "FLOW-"+pageKey, "noForm")
		for _, field := range page.Fields {
			total++
			if field.ValueShape == "" || field.Serialization == "" || field.CandidateSource == "" || field.Evidence == "" || len(field.ValidationCapability) == 0 {
				t.Fatalf("Vue 字段契约不完整：page=%s field=%+v", pageKey, field)
			}
		}
	}
	if total < 15 {
		t.Fatalf("真实 Vue 页面字段数量不足以验证往返契约：%d", total)
	}
}

// TestP002CustomComponentsExposeVirtualAndRoundTripCapabilities 验证身份、项目和流程列表组件具备完整候选与保存往返能力。
func TestP002CustomComponentsExposeVirtualAndRoundTripCapabilities(t *testing.T) {
	capabilities := formdata.CustomComponentCapabilities()
	for _, component := range []string{"custome-info-select", "custome-select-project", "person-mulSelect"} {
		capability := capabilities[component]
		for _, key := range []string{"valueType", "serialization", "candidateSource", "validation", "formMakingPlayback", "vuePlayback", "saveRoundTrip", "virtualFields", "evidence"} {
			if capability[key] == "" {
				t.Fatalf("组件 %s 缺少 %s 能力：%+v", component, key, capability)
			}
		}
	}
	if capabilities["custome-info-select"]["virtualFields"] != "__condition" || capabilities["person-mulSelect"]["virtualFields"] != "__formPersonId" {
		t.Fatalf("身份或流程列表虚拟字段登记错误：info=%+v person=%+v", capabilities["custome-info-select"], capabilities["person-mulSelect"])
	}
}

// TestP002FormMakingKeepsCascaderRowsAndExternalBoundary 验证级联、子表单行和附件边界保持真实值形态。
func TestP002FormMakingKeepsCascaderRowsAndExternalBoundary(t *testing.T) {
	template := map[string]any{"list": []any{
		map[string]any{"type": "cascader", "model": "category", "name": "分类", "options": map[string]any{"options": []any{
			map[string]any{"label": "根", "value": "root", "children": []any{map[string]any{"label": "叶", "value": "leaf"}}},
		}}},
		map[string]any{"type": "subform", "model": "lines", "list": []any{
			map[string]any{"type": "input", "model": "name", "name": "明细名称", "options": map[string]any{}},
		}},
		map[string]any{"type": "fileupload", "model": "attachment", "name": "附件", "options": map[string]any{"required": true}},
	}}
	fields, unsupported := formdata.ParseTemplate(template)
	if len(unsupported) != 0 || len(fields) != 3 {
		t.Fatalf("复杂字段解析失败：fields=%+v unsupported=%v", fields, unsupported)
	}
	if !reflect.DeepEqual(fields[0].OptionPaths, [][]any{{"root", "leaf"}}) || fields[1].Path != "lines[].name" || fields[1].CollectionRoot != "lines" || !fields[2].ManualOnly {
		t.Fatalf("复杂字段值形态或人工边界错误：%+v", fields)
	}
	result := formdata.Generate(formdata.GenerateInput{Template: template, Seed: 1, Base: map[string]any{"lines": []any{map[string]any{"name": "已有行"}}}})
	if !reflect.DeepEqual(result.Values["category"], []any{"root", "leaf"}) || result.Pending != 1 {
		t.Fatalf("级联完整路径或附件人工待办错误：%+v", result)
	}
}
