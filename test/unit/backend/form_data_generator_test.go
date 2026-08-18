package backend_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

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

// TestFormDataGeneratorProtectsPathConditions 验证分支条件字段不接受人工覆盖，普通字段仍可保留人工值。
func TestFormDataGeneratorProtectsPathConditions(t *testing.T) {
	template := generatorTemplate()
	result := formdata.Generate(formdata.GenerateInput{
		Template: template, Seed: 11,
		Base:                map[string]any{"kind": "a", "title": "人工标题"},
		Constraints:         []formdata.Constraint{{Field: "kind", Op: "eq", Value: "b"}},
		ManualOverridePaths: []string{"kind", "title"},
		ProtectedPaths:      map[string]bool{"kind": true},
	})
	if result.Values["kind"] != "b" || result.Values["title"] != "人工标题" {
		t.Fatalf("条件字段没有强制保持当前路径、普通人工字段被错误覆盖：%+v", result.Values)
	}
	found := false
	for _, path := range result.GeneratedFieldPaths {
		if path == "kind" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("受保护条件字段没有记录为路径生成值：%v", result.GeneratedFieldPaths)
	}
}

// TestFormDataGeneratorSupportsFieldToFieldConstraint 验证字段对字段条件由同一生成与保存校验语义处理。
func TestFormDataGeneratorSupportsFieldToFieldConstraint(t *testing.T) {
	template := map[string]any{"list": []any{
		map[string]any{"type": "number", "model": "amount", "name": "申请金额", "options": map[string]any{"required": true}},
		map[string]any{"type": "number", "model": "limit", "name": "对比金额", "options": map[string]any{"required": true}},
	}}
	constraints := []formdata.Constraint{{Field: "amount", Op: "gte", ValueField: "limit"}}
	result := formdata.Generate(formdata.GenerateInput{
		Template: template, Seed: 12, Base: map[string]any{"amount": float64(1), "limit": float64(9)},
		Constraints: constraints, ProtectedPaths: map[string]bool{"amount": true, "limit": true},
	})
	if result.Values["amount"] != float64(9) {
		t.Fatalf("字段对字段条件没有以右侧值生成：%+v", result.Values)
	}
	if reasons := formdata.ValidateEditable(template, result.Values, constraints, nil); len(reasons) != 0 {
		t.Fatalf("字段对字段生成结果未通过保存复验：%v", reasons)
	}
}

// TestFormDataGeneratorKeepsInfoSelectConditionVirtualValue 验证信息选择组件的条件虚拟值由路径规则优先保持。
func TestFormDataGeneratorKeepsInfoSelectConditionVirtualValue(t *testing.T) {
	template := map[string]any{"list": []any{
		map[string]any{"type": "custom", "el": "custome-info-select", "model": "myUserName", "name": "人员", "options": map[string]any{}},
	}}
	result := formdata.Generate(formdata.GenerateInput{
		Template: template, Seed: 13,
		Base:           map[string]any{"myUserName": `{"id":"u1","name":"历史人员"}`},
		Constraints:    []formdata.Constraint{{Field: "myUserName__condition", Op: "eq", Value: "当前路径人员"}},
		ProtectedPaths: map[string]bool{"myUserName": true, "myUserName__condition": true},
	})
	if result.Values["myUserName__condition"] != "当前路径人员" {
		t.Fatalf("信息选择条件虚拟值被基础组件回填覆盖：%+v", result.Values)
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

// TestFormDataGeneratorFillsInfoSelectFromIdentity 验证选公司/部门/人员组件按账号身份目录节点生成组件约定 JSON 值。
func TestFormDataGeneratorFillsInfoSelectFromIdentity(t *testing.T) {
	template := map[string]any{"list": []any{
		map[string]any{"type": "text", "name": "公司", "model": ""},
		map[string]any{"type": "custom", "el": "custome-info-select", "model": "myCompanyName", "name": "通用信息选择", "options": map[string]any{"required": true}},
		map[string]any{"type": "text", "name": "部门", "model": ""},
		map[string]any{"type": "custom", "el": "custome-info-select", "model": "myDepName", "name": "通用信息选择", "options": map[string]any{"required": true}},
		map[string]any{"type": "text", "name": "姓名", "model": ""},
		map[string]any{"type": "custom", "el": "custome-info-select", "model": "myUserName", "name": "通用信息选择", "options": map[string]any{"required": true}},
	}}
	identity := formdata.IdentityContext{
		Company:    formdata.IdentityNode{ID: "c1", Name: "测试公司", Type: "1", ParentID: "g1"},
		Department: formdata.IdentityNode{ID: "d1", Name: "测试部", Type: "2", ParentID: "c1", CompanyID: "c1"},
		User:       formdata.IdentityNode{ID: "u1", Name: "测试人", Type: "5", ParentID: "d1"},
	}
	result := formdata.Generate(formdata.GenerateInput{Template: template, Seed: 1, Identity: identity})
	if result.Identity != 3 || result.Pending != 0 {
		t.Fatalf("信息选择组件没有按身份填充：%+v", result)
	}
	company := map[string]any{}
	if err := json.Unmarshal([]byte(result.Values["myCompanyName"].(string)), &company); err != nil || company["id"] != "c1" || company["name"] != "测试公司" {
		t.Fatalf("公司组件值不符合组件约定：%+v err=%v", result.Values["myCompanyName"], err)
	}
	department := map[string]any{}
	if err := json.Unmarshal([]byte(result.Values["myDepName"].(string)), &department); err != nil || department["id"] != "d1" || department["companyId"] != "c1" {
		t.Fatalf("部门组件值不符合组件约定：%+v err=%v", result.Values["myDepName"], err)
	}
	user := map[string]any{}
	if err := json.Unmarshal([]byte(result.Values["myUserName"].(string)), &user); err != nil || user["id"] != "u1" || user["parentId"] != "d1" {
		t.Fatalf("人员组件值不符合组件约定：%+v err=%v", result.Values["myUserName"], err)
	}
}

// TestFormDataGeneratorUsesLabelsAndSmartText 验证前置 text 标签成为字段名称并生成可读文本值。
func TestFormDataGeneratorUsesLabelsAndSmartText(t *testing.T) {
	template := map[string]any{"list": []any{
		map[string]any{"type": "text", "name": "请假原因", "model": ""},
		map[string]any{"type": "textarea", "model": "vacateReason", "name": "多行文本", "options": map[string]any{"required": true, "defaultValue": ""}},
		map[string]any{"type": "text", "name": "发起人", "model": ""},
		map[string]any{"type": "input", "model": "initiatorName", "name": "单行文本", "options": map[string]any{"required": true}},
	}}
	fields, unsupported := formdata.ParseTemplate(template)
	if len(unsupported) != 0 || len(fields) != 2 {
		t.Fatalf("标签解析结果不正确：fields=%+v unsupported=%v", fields, unsupported)
	}
	if fields[0].Name != "请假原因" || fields[1].Name != "发起人" {
		t.Fatalf("前置 text 标签没有成为字段名称：%+v", fields)
	}
	result := formdata.Generate(formdata.GenerateInput{Template: template, Seed: 5, Initiator: "骆蒙恩"})
	if result.Values["vacateReason"] != "个人事务需要处理" {
		t.Fatalf("请假原因没有生成可读文本：%+v", result.Values)
	}
	if result.Values["initiatorName"] != "骆蒙恩" {
		t.Fatalf("发起人字段没有使用账号姓名：%+v", result.Values)
	}
}

// TestFormDataGeneratorFillsDateRange 验证日期范围组件生成 [开始, 结束] 数组并通过同模板校验。
func TestFormDataGeneratorFillsDateRange(t *testing.T) {
	template := map[string]any{"list": []any{
		map[string]any{"type": "text", "name": "起讫时间", "model": ""},
		map[string]any{"type": "date", "model": "range", "name": "日期选择器", "options": map[string]any{"required": true, "type": "daterange"}},
	}}
	result := formdata.Generate(formdata.GenerateInput{Template: template, Seed: 3})
	list, ok := result.Values["range"].([]any)
	if !ok || len(list) != 2 {
		t.Fatalf("日期范围没有生成 [开始, 结束] 数组：%+v", result.Values["range"])
	}
	for _, item := range list {
		if _, err := time.Parse("2006-01-02", item.(string)); err != nil {
			t.Fatalf("日期范围元素格式错误：%+v", list)
		}
	}
	if reasons := formdata.Validate(template, result.Values, nil); len(reasons) != 0 {
		t.Fatalf("日期范围生成结果没有通过同模板校验：%v", reasons)
	}
}

// TestFormDataGeneratorBindsDateRangeToDuration 验证唯一结构绑定按自然日含首尾同步日期，并拒绝手工改成不匹配区间。
func TestFormDataGeneratorBindsDateRangeToDuration(t *testing.T) {
	template := map[string]any{"list": []any{
		map[string]any{"type": "number", "model": "durationValue", "name": "数值字段", "options": map[string]any{"required": true}},
		map[string]any{"type": "date", "model": "periodValue", "name": "日期字段", "options": map[string]any{"required": true, "type": "daterange"}},
	}}
	result := formdata.Generate(formdata.GenerateInput{
		Template: template, Samples: []map[string]any{{"durationValue": float64(15), "periodValue": []any{"2026-01-31", "2026-02-02"}}}, Seed: 4,
		Constraints:       []formdata.Constraint{{Field: "durationValue", Op: "gte", Value: 15}},
		DateRangeBindings: []formdata.DateRangeBinding{{DurationField: "durationValue", RangeField: "periodValue"}},
	})
	rangeValue := result.Values["periodValue"].([]any)
	if rangeValue[0] != "2026-01-31" || rangeValue[1] != "2026-02-14" {
		t.Fatalf("日期区间没有按十五天跨月计算：%+v", rangeValue)
	}
	if reasons := formdata.ValidateDateRangeBindings(result.Values, []formdata.DateRangeBinding{{DurationField: "durationValue", RangeField: "periodValue"}}); len(reasons) != 0 {
		t.Fatalf("同步后的日期区间被错误拒绝：%v", reasons)
	}
	result.Values["periodValue"] = []any{"2026-01-31", "2026-02-13"}
	if reasons := formdata.ValidateDateRangeBindings(result.Values, []formdata.DateRangeBinding{{DurationField: "durationValue", RangeField: "periodValue"}}); len(reasons) == 0 {
		t.Fatal("手工缩短日期区间后仍被当作有效")
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
