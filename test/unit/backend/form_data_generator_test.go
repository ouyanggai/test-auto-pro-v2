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

// TestFormDataGeneratorKeepsComplexComponentsUnsupported 验证复杂业务组件只报告缺口，不伪造成普通文本值。
func TestFormDataGeneratorKeepsComplexComponentsUnsupported(t *testing.T) {
	template := map[string]any{"list": []any{
		map[string]any{"type": "grid", "columns": []any{map[string]any{"list": []any{
			map[string]any{"type": "component", "model": "contract", "name": "合同组件", "options": map[string]any{}},
		}}}},
	}}
	result := formdata.Generate(formdata.GenerateInput{Template: template, Seed: 7})
	if len(result.Unsupported) != 1 || !strings.Contains(result.Unsupported[0], "合同组件") {
		t.Fatalf("复杂组件没有稳定标记 unsupported：%+v", result)
	}
	if _, exists := result.Values["contract"]; exists {
		t.Fatalf("复杂组件被错误生成普通值：%+v", result.Values)
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
