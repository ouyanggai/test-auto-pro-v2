package backend_test

import (
	"reflect"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/formdata"
)

// TestFormDataGeneratorParsesNestedTargetTemplate 验证真实 FormMaking 栅格、报表行列和列表容器都会递归解析。
func TestFormDataGeneratorParsesNestedTargetTemplate(t *testing.T) {
	template := map[string]any{"list": []any{
		map[string]any{"type": "grid", "columns": []any{map[string]any{"list": []any{
			map[string]any{"type": "input", "model": "title", "name": "标题", "options": map[string]any{"required": true}},
		}}}},
		map[string]any{"type": "report", "rows": []any{map[string]any{"columns": []any{map[string]any{"list": []any{
			map[string]any{"type": "number", "model": "amount", "name": "金额", "options": map[string]any{}},
		}}}}}},
		map[string]any{"type": "card", "list": []any{
			map[string]any{"type": "select", "model": "kind", "name": "类型", "options": map[string]any{"options": []any{
				map[string]any{"label": "甲", "value": "a"}, map[string]any{"label": "乙", "value": "b"},
			}}},
		}},
	}}
	fields, unsupported := formdata.ParseTemplate(template)
	if len(unsupported) != 0 || len(fields) != 3 {
		t.Fatalf("嵌套基础字段没有完整解析：fields=%+v unsupported=%v", fields, unsupported)
	}
	paths := []string{fields[0].Path, fields[1].Path, fields[2].Path}
	if !reflect.DeepEqual(paths, []string{"title", "amount", "kind"}) {
		t.Fatalf("嵌套字段顺序或路径错误：%v", paths)
	}
}

// TestFormDataGeneratorSkipsComplexComponentsWithoutBlockingForm 验证复杂组件留给真实表单人工填写，不伪造值或阻断整张表单。
func TestFormDataGeneratorKeepsComplexComponentsUnsupported(t *testing.T) {
	template := map[string]any{"list": []any{
		map[string]any{"type": "grid", "columns": []any{map[string]any{"list": []any{
			map[string]any{"type": "component", "model": "contract", "name": "合同组件", "options": map[string]any{}},
		}}}},
	}}
	result := formdata.Generate(formdata.GenerateInput{Template: template, Base: map[string]any{"contract": `{"selected":"历史值"}`}, Seed: 7})
	if len(result.Unsupported) != 0 || result.Pending != 1 {
		t.Fatalf("复杂组件应只计入人工待填而非阻断表单：%+v", result)
	}
	if result.Values["contract"] != `{"selected":"历史值"}` {
		t.Fatalf("复杂组件已有值被生成器覆盖或删除：%+v", result.Values)
	}
}

// TestFormDataGeneratorIsDeterministicAndHonorsPathConstraints 验证相同种子可复现且当前路径条件覆盖生成器拥有字段。
func TestFormDataGeneratorIsDeterministicAndHonorsPathConstraints(t *testing.T) {
	template := generatorTemplate()
	input := formdata.GenerateInput{
		Template: template, Seed: 19, Initiator: "测试发起人",
		Samples:     []map[string]any{{"title": "历史标题", "kind": "a"}},
		Constraints: []formdata.Constraint{{Field: "kind", Op: "eq", Value: "b"}},
	}
	first := formdata.Generate(input)
	second := formdata.Generate(input)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("相同种子生成结果不稳定：first=%+v second=%+v", first, second)
	}
	if first.Values["title"] != "历史标题" || first.Values["kind"] != "b" || first.Values["kind__virtualName"] != "乙" {
		t.Fatalf("样本、条件或虚拟字段没有正确合并：%+v", first.Values)
	}
	if reasons := formdata.Validate(template, first.Values, input.Constraints); len(reasons) != 0 {
		t.Fatalf("生成结果没有通过相同模板和路径条件校验：%v", reasons)
	}
}

// TestFormDataGeneratorNextGroupPreservesManualOverrides 验证换一组只替换生成器拥有字段，人工修改保持不变。
func TestFormDataGeneratorNextGroupPreservesManualOverrides(t *testing.T) {
	current := map[string]any{"title": "人工标题", "amount": float64(12)}
	generated := map[string]any{"title": "新标题", "amount": float64(88)}
	merged := formdata.MergeGenerated(current, generated, []string{"title", "amount"}, []string{"title"})
	if merged["title"] != "人工标题" || merged["amount"] != float64(88) {
		t.Fatalf("换组覆盖了人工字段或没有更新生成字段：%+v", merged)
	}
}

// TestFormDataGeneratorValidatesDatesAndORGroups 验证日期时间严格格式与 OR 条件组不会被当作全部 AND。
func TestFormDataGeneratorValidatesDatesAndORGroups(t *testing.T) {
	template := map[string]any{"list": []any{
		map[string]any{"type": "date", "model": "happenedAt", "name": "发生时间", "options": map[string]any{"required": true, "type": "datetime"}},
		map[string]any{"type": "select", "model": "kind", "name": "类型", "options": map[string]any{"options": []any{map[string]any{"label": "甲", "value": "a"}, map[string]any{"label": "乙", "value": "b"}}}},
		map[string]any{"type": "number", "model": "amount", "name": "金额", "options": map[string]any{}},
	}}
	constraints := []formdata.Constraint{
		{Field: "kind", Op: "eq", Value: "a", Group: 1},
		{Field: "amount", Op: "gte", Value: float64(100), Group: 1},
	}
	valid := map[string]any{"happenedAt": "2026-08-10 09:30:00", "kind": "b", "amount": float64(120)}
	if reasons := formdata.Validate(template, valid, constraints); len(reasons) != 0 {
		t.Fatalf("满足 OR 组任一条件却被拒绝：%v", reasons)
	}
	invalidDate := map[string]any{"happenedAt": "2026/08/10 09:30", "kind": "a", "amount": float64(10)}
	if reasons := formdata.Validate(template, invalidDate, constraints); len(reasons) == 0 || !strings.Contains(strings.Join(reasons, ";"), "发生时间") {
		t.Fatalf("任意日期时间文本没有被严格拒绝：%v", reasons)
	}
}

// TestFormDataGeneratorFillsRequiredEmptyDefaults 验证空字符串默认值不会被当成有效值，必填基础字段仍回退生成。
func TestFormDataGeneratorFillsRequiredEmptyDefaults(t *testing.T) {
	template := map[string]any{"list": []any{
		map[string]any{"type": "textarea", "model": "reason", "name": "原因", "options": map[string]any{"required": true, "defaultValue": ""}},
	}}
	result := formdata.Generate(formdata.GenerateInput{Template: template, Seed: 3})
	if reason, _ := result.Values["reason"].(string); strings.TrimSpace(reason) == "" {
		t.Fatalf("必填字段空默认值没有被回退生成：%+v", result.Values)
	}
	if result.Pending != 0 {
		t.Fatalf("必填基础字段不应计入人工待填：%+v", result)
	}
}

// TestFormDataGeneratorCountsEachCustomFieldPending 验证多个同名自定义组件不会被去重，各自计入人工待填。
func TestFormDataGeneratorCountsEachCustomFieldPending(t *testing.T) {
	template := map[string]any{"list": []any{
		map[string]any{"type": "custom", "model": "myCompanyName", "name": "通用信息选择", "options": map[string]any{}},
		map[string]any{"type": "custom", "model": "myDepName", "name": "通用信息选择", "options": map[string]any{}},
		map[string]any{"type": "custom", "model": "myUserName", "name": "通用信息选择", "options": map[string]any{}},
	}}
	result := formdata.Generate(formdata.GenerateInput{Template: template, Seed: 1})
	if result.Pending != 3 {
		t.Fatalf("同名自定义组件应各自计入人工待填：%+v", result)
	}
}

// generatorTemplate 返回生成器测试共用的基础 FormMaking 模板。
func generatorTemplate() map[string]any {
	return map[string]any{"list": []any{
		map[string]any{"type": "input", "model": "title", "name": "标题", "options": map[string]any{"required": true}},
		map[string]any{"type": "number", "model": "amount", "name": "金额", "options": map[string]any{}},
		map[string]any{"type": "select", "model": "kind", "name": "类型", "options": map[string]any{"required": true, "options": []any{
			map[string]any{"label": "甲", "value": "a"}, map[string]any{"label": "乙", "value": "b"},
		}}},
	}}
}
