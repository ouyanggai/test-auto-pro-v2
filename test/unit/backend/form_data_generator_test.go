package backend_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
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

// TestFormDataGeneratorKeepsRuntimeComponentValue 验证非必填运行时组件保留已有值且不制造人工待办。
func TestFormDataGeneratorKeepsRuntimeComponentValue(t *testing.T) {
	template := map[string]any{"list": []any{
		map[string]any{"type": "grid", "columns": []any{map[string]any{"list": []any{
			map[string]any{"type": "component", "model": "contract", "name": "合同组件", "options": map[string]any{}},
		}}}},
	}}
	result := formdata.Generate(formdata.GenerateInput{Template: template, Base: map[string]any{"contract": `{"selected":"历史值"}`}, Seed: 7})
	if len(result.Unsupported) != 0 || result.Pending != 0 {
		t.Fatalf("非必填运行时组件不应制造人工待填或未知能力：%+v", result)
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

// TestFormDataGeneratorUsesOptionValueForExplicitHiddenLabel 验证模板显式关闭 showLabel 时虚拟条件字段使用真实选项值。
func TestFormDataGeneratorUsesOptionValueForExplicitHiddenLabel(t *testing.T) {
	template := map[string]any{"list": []any{map[string]any{
		"type": "select", "model": "leaveType", "name": "请假类别",
		"options": map[string]any{"showLabel": false, "options": []any{
			map[string]any{"label": "采购供应链管理", "value": "陪产假"},
		}},
	}}}
	result := formdata.Generate(formdata.GenerateInput{Template: template, Base: map[string]any{"leaveType": "陪产假"}, Seed: 1})
	if result.Values["leaveType__virtualName"] != "陪产假" {
		t.Fatalf("关闭 showLabel 后虚拟条件字段仍错误使用展示标签：%#v", result.Values)
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

// TestFormDataGeneratorFillsLtdOrDepSelectWithArrayShape 验证公司部门多选组件仍使用宿主要求的 JSON 数组，而信息单选组件使用对象。
func TestFormDataGeneratorFillsLtdOrDepSelectWithArrayShape(t *testing.T) {
	template := map[string]any{"list": []any{map[string]any{
		"type": "custom", "el": "ltd-or-dep-select", "model": "handlingDepartment", "name": "办理部门", "options": map[string]any{"required": true},
	}}}
	identity := formdata.IdentityContext{Department: formdata.IdentityNode{ID: "d1", Name: "测试部", Type: "2", CompanyID: "c1"}}
	result := formdata.Generate(formdata.GenerateInput{Template: template, Seed: 1, Identity: identity})
	var values []map[string]any
	if result.Pending != 0 || json.Unmarshal([]byte(result.Values["handlingDepartment"].(string)), &values) != nil || len(values) != 1 || values[0]["id"] != "d1" {
		t.Fatalf("公司部门组件没有按数组协议生成：%+v", result)
	}
	if reasons := formdata.Validate(template, result.Values, nil); len(reasons) != 0 {
		t.Fatalf("公司部门组件数组值未通过保存复验：%v", reasons)
	}
}

// TestFormDataGeneratorUsesRegisteredCustomIdentityShape 验证已注册人员组件按宿主源码要求生成 flowList JSON，而非要求用户手填。
func TestFormDataGeneratorUsesRegisteredCustomIdentityShape(t *testing.T) {
	template := map[string]any{"list": []any{map[string]any{
		"type": "custom", "el": "person-mulSelect", "model": "reviewers", "options": map[string]any{"required": true},
	}}}
	identity := formdata.IdentityContext{Company: formdata.IdentityNode{ID: "c1", Name: "测试公司"}, Department: formdata.IdentityNode{ID: "d1", Name: "测试部"}, User: formdata.IdentityNode{ID: "u1", Name: "测试人", Type: "5"}}
	result := formdata.Generate(formdata.GenerateInput{Template: template, Seed: 1, Identity: identity})
	if result.Pending != 0 || result.Identity != 1 || len(result.Unsupported) != 0 {
		t.Fatalf("已注册人员组件不应成为人工阻断：%+v", result)
	}
	value := map[string][]map[string]any{}
	if err := json.Unmarshal([]byte(result.Values["reviewers"].(string)), &value); err != nil || len(value["flowList"]) != 1 || value["flowList"][0]["id"] != "u1" {
		t.Fatalf("人员组件值形态不符合宿主约定：%+v err=%v", result.Values["reviewers"], err)
	}
}

// TestFormDataGeneratorKeepsExternalCustomPartial 验证外部对象组件没有真实候选时只保留待处理项，不误报为未知组件或伪造引用。
func TestFormDataGeneratorKeepsExternalCustomPartial(t *testing.T) {
	template := map[string]any{"list": []any{map[string]any{
		"type": "custom", "el": "custome-select-project", "model": "project", "options": map[string]any{"required": true},
	}}}
	result := formdata.Generate(formdata.GenerateInput{Template: template, Seed: 1})
	if result.Pending != 1 || len(result.Unsupported) != 0 {
		t.Fatalf("外部对象无候选时应是部分待处理：%+v", result)
	}
	if _, exists := result.Values["project"]; exists {
		t.Fatalf("外部对象无真实候选时不能伪造引用：%+v", result.Values)
	}
}

// TestFormDataGeneratorUsesInitiatorCustomCandidates 验证外部组件只消费调用方传入的当前发起人候选，并生成可通过同一规则复验的形状。
func TestFormDataGeneratorUsesInitiatorCustomCandidates(t *testing.T) {
	template := map[string]any{"list": []any{map[string]any{
		"type": "custom", "el": "custome-select-project", "model": "project", "options": map[string]any{"required": true},
	}}}
	result := formdata.Generate(formdata.GenerateInput{
		Template: template, Seed: 1,
		ComponentCandidates: map[string][]any{"project": {map[string]any{"id": "p-current", "name": "当前账号项目"}}},
	})
	if result.Pending != 0 || result.Recent != 1 || len(result.GeneratedFieldPaths) != 1 {
		t.Fatalf("当前发起人候选没有用于自定义组件生成：%+v", result)
	}
	value, ok := result.Values["project"].(string)
	if !ok || !strings.Contains(value, "p-current") || len(formdata.Validate(template, result.Values, nil)) != 0 {
		t.Fatalf("自定义组件候选值没有按宿主 JSON 形状通过复验：%+v", result.Values)
	}
	encoded, encodeErr := json.Marshal(result.Values)
	decoded := map[string]any{}
	decodeErr := json.Unmarshal(encoded, &decoded)
	if encodeErr != nil || decodeErr != nil || len(formdata.Validate(template, decoded, nil)) != 0 {
		t.Fatalf("自定义组件生成值没有通过保存 JSON 往返复验：encoded=%s encodeErr=%v decodeErr=%v", encoded, encodeErr, decodeErr)
	}
}

// TestFormDataGeneratorBoxesMaterialCandidate 验证材料端点返回的单条对象按组件协议装箱为 JSON 数组，生成后不会被服务端形状校验拒绝。
func TestFormDataGeneratorBoxesMaterialCandidate(t *testing.T) {
	template := map[string]any{"list": []any{map[string]any{
		"type": "custom", "el": "out-bound-material-select", "model": "materials", "options": map[string]any{"required": true},
	}}}
	result := formdata.Generate(formdata.GenerateInput{
		Template: template, Seed: 2,
		ComponentCandidates: map[string][]any{"materials": {map[string]any{"id": "m-current", "name": "当前账号材料"}}},
	})
	value, ok := result.Values["materials"].(string)
	var decoded []map[string]any
	decodeErr := json.Unmarshal([]byte(value), &decoded)
	if !ok || decodeErr != nil || len(decoded) != 1 || decoded[0]["id"] != "m-current" || result.Pending != 0 || len(formdata.Validate(template, result.Values, nil)) != 0 {
		t.Fatalf("材料候选没有按数组协议生成并复验：result=%+v value=%q err=%v", result, value, decodeErr)
	}
}

// TestFormDataGeneratorDoesNotReuseExternalSample 验证外部对象不会从近期样本复用其他账号可能无权访问的对象标识。
func TestFormDataGeneratorDoesNotReuseExternalSample(t *testing.T) {
	template := map[string]any{"list": []any{map[string]any{
		"type": "custom", "el": "custome-select-project", "model": "project", "options": map[string]any{"required": true},
	}}}
	result := formdata.Generate(formdata.GenerateInput{
		Template: template, Seed: 1, Samples: []map[string]any{{"project": `{"id":"historical-project"}`}},
	})
	if result.Pending != 1 {
		t.Fatalf("外部对象不应从近期样本继承：%+v", result)
	}
	if _, found := result.Values["project"]; found {
		t.Fatalf("外部对象样本 ID 泄露到当前账号草稿：%+v", result.Values)
	}
}

// TestFormDataGeneratorFallsBackStaticCustomOptions 验证明确声明静态默认能力的组件可在无候选时使用模板选项。
func TestFormDataGeneratorFallsBackStaticCustomOptions(t *testing.T) {
	template := map[string]any{"list": []any{map[string]any{
		"type": "custom", "el": "custom-weather", "model": "weather", "options": map[string]any{
			"required": true, "options": []any{map[string]any{"label": "晴", "value": "sunny"}},
		},
	}}}
	result := formdata.Generate(formdata.GenerateInput{Template: template, Seed: 1})
	if result.Pending != 0 || result.Values["weather"] != "sunny" || len(formdata.Validate(template, result.Values, nil)) != 0 {
		t.Fatalf("静态自定义组件没有使用模板选项生成可保存值：%+v", result)
	}
}

// TestFormDataCustomCapabilityRegistryCoversRuntimeRegistry 验证实际运行时注册的组件都具有值、候选、序列化、校验和证据能力，而非粗粒度标签。
func TestFormDataCustomCapabilityRegistryCoversRuntimeRegistry(t *testing.T) {
	capabilities := formdata.CustomComponentCapabilities()
	mainSource, err := os.ReadFile(filepath.Join(f010ProjectRoot(t), "form-runtime", "runtime-source", "src", "main.js"))
	if err != nil {
		t.Fatalf("读取宿主实际组件注册表失败：%v", err)
	}
	registrationPattern := regexp.MustCompile(`\{\s*name:\s*'([^']+)'\s*,\s*component:`)
	registrations := registrationPattern.FindAllStringSubmatch(string(mainSource), -1)
	if len(capabilities) != len(registrations) || len(registrations) != 20 {
		t.Fatalf("组件能力注册数量与宿主注册表不一致：capabilities=%d runtime=%d", len(capabilities), len(registrations))
	}
	for _, registration := range registrations {
		if _, exists := capabilities[registration[1]]; !exists {
			t.Fatalf("宿主实际组件 %s 缺少能力证据", registration[1])
		}
	}
	for name, capability := range capabilities {
		for _, key := range []string{
			"valueType", "candidateKind", "serialization", "candidateSource", "defaultAllowed", "conditionValue",
			"validation", "requiredValidation", "businessValidation", "changeGroup", "formMakingPlayback",
			"vuePlayback", "saveRoundTrip", "permissionBoundary", "evidence",
		} {
			if capability[key] == "" {
				t.Fatalf("组件 %s 缺少能力 %s：%+v", name, key, capability)
			}
		}
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

// TestFormDataGeneratorSupportsCascaderAndCollectionShape 验证级联使用完整候选路径，子表单字段保持数组行结构。
func TestFormDataGeneratorSupportsCascaderAndCollectionShape(t *testing.T) {
	template := map[string]any{"list": []any{
		map[string]any{"type": "cascader", "model": "category", "name": "分类", "options": map[string]any{"required": true, "options": []any{
			map[string]any{"value": "a", "label": "甲", "children": []any{map[string]any{"value": "a1", "label": "甲一"}}},
		}}},
		map[string]any{"type": "subform", "model": "items", "list": []any{map[string]any{"type": "input", "model": "name", "name": "明细名称", "options": map[string]any{"required": true}}}},
	}}
	result := formdata.Generate(formdata.GenerateInput{Template: template, Base: map[string]any{"items": []any{map[string]any{"name": "已有明细"}}}, ManualOverridePaths: []string{"items[].name"}, Seed: 1})
	category, ok := result.Values["category"].([]any)
	if !ok || len(category) != 2 || category[0] != "a" || category[1] != "a1" {
		t.Fatalf("级联没有生成完整候选路径：%#v", result.Values["category"])
	}
	items, ok := result.Values["items"].([]any)
	if !ok || len(items) != 1 || items[0].(map[string]any)["name"] != "已有明细" {
		t.Fatalf("子表单行结构或人工值被破坏：%#v", result.Values["items"])
	}
	if reasons := formdata.Validate(template, result.Values, nil); len(reasons) != 0 {
		t.Fatalf("级联和子表单生成结果未通过校验：%v", reasons)
	}
}

// TestFormDataGeneratorKeepsFileUploadManualOnly 验证附件字段不伪造外部引用，只返回人工待补。
func TestFormDataGeneratorKeepsFileUploadManualOnly(t *testing.T) {
	template := map[string]any{"list": []any{map[string]any{"type": "fileupload", "model": "attachment", "name": "附件", "options": map[string]any{"required": true}}}}
	result := formdata.Generate(formdata.GenerateInput{Template: template, Seed: 2})
	fields, _ := formdata.ParseTemplate(template)
	if result.Pending != 1 || len(result.Unsupported) != 0 || len(fields) != 1 || !fields[0].ManualOnly {
		t.Fatalf("附件应进入人工待补而不是伪造或误报未知组件：%+v", result)
	}
}

// TestFormDataGeneratorInventoriesUnknownTemplateCapabilities 验证规则盘点对未知组件和动态脚本明确阻断。
func TestFormDataGeneratorInventoriesUnknownTemplateCapabilities(t *testing.T) {
	inventory := formdata.InventoryTemplateRules(map[string]any{"list": []any{map[string]any{
		"type": "custom", "el": "unknown-widget", "model": "customValue", "options": map[string]any{"requestURL": "/api/options"},
		"eventScript": "doSomethingDangerous()",
	}}})
	if inventory.ComponentTypes["custom"] != 1 || len(inventory.DataSources) != 1 {
		t.Fatalf("模板规则盘点遗漏组件或数据源：%+v", inventory)
	}
	if len(inventory.NeedsAttention) == 0 {
		t.Fatal("未知组件或动态脚本没有进入 needs_attention")
	}
}

// TestF009TemplateCoverageReportKeepsAllRealTemplatesAndUnknownCapabilities 检查覆盖报告不会漏计空模板或吞掉未分类能力。
func TestF009TemplateCoverageReportKeepsAllRealTemplatesAndUnknownCapabilities(t *testing.T) {
	templates := make([]map[string]any, 196)
	for index := range templates {
		typeName := "input"
		if index == 195 {
			typeName = "vendor-widget"
		}
		templates[index] = map[string]any{"list": []any{map[string]any{"type": typeName, "model": fmt.Sprintf("field-%d", index)}}}
	}
	report := formdata.BuildTemplateCoverageReport(templates)
	if report.TemplateCount != 196 || report.ComponentTypes["input"] != 195 || report.ComponentTypes["vendor-widget"] != 1 {
		t.Fatalf("196 个模板的组件覆盖统计不准确：%+v", report)
	}
	if report.NeedsAttentionTemplates != 1 || len(report.NeedsAttention) == 0 {
		t.Fatalf("未知能力没有进入覆盖阻断：%+v", report)
	}
}

// TestF009GeneratorRecognizesRuntimeStandardAndRegisteredComponents 验证目标运行时标准扩展和已注册组件不会被误报为未知能力。
func TestF009GeneratorRecognizesRuntimeStandardAndRegisteredComponents(t *testing.T) {
	template := map[string]any{"list": []any{
		map[string]any{"type": "col", "list": []any{map[string]any{"type": "td", "list": []any{
			map[string]any{"type": "datetime", "model": "createdAt", "options": map[string]any{"required": true}},
		}}}},
		map[string]any{"type": "datetimerange", "model": "period", "options": map[string]any{"required": true}},
		map[string]any{"type": "imgupload", "model": "photos", "options": map[string]any{"required": true}},
		map[string]any{"type": "custom", "el": "custome-select-project", "model": "project", "options": map[string]any{"required": true}},
		map[string]any{"type": "button", "model": "submitButton"},
		map[string]any{"type": "rate", "model": "score", "options": map[string]any{"required": true}},
		map[string]any{"type": "slider", "model": "range", "options": map[string]any{"required": true, "range": true}},
		map[string]any{"type": "color", "model": "color", "options": map[string]any{"required": true}},
		map[string]any{"type": "transfer", "model": "members", "options": map[string]any{"required": true, "data": []any{
			map[string]any{"key": "member-a", "label": "成员甲"},
		}}},
		map[string]any{"type": "editor", "model": "content", "options": map[string]any{"required": true}},
		map[string]any{"type": "alert", "options": map[string]any{"title": "提示"}},
		map[string]any{"type": "upload", "model": "legacyAttachment", "options": map[string]any{"required": true}},
	}}
	fields, unsupported := formdata.ParseTemplate(template)
	if len(unsupported) != 0 {
		t.Fatalf("标准扩展或已注册组件被错误列为不支持：%v", unsupported)
	}
	var dateModes []string
	for _, field := range fields {
		if field.Type == "date" {
			dateModes = append(dateModes, field.Mode)
		}
	}
	if len(dateModes) != 2 || dateModes[0] != "datetime" || dateModes[1] != "datetimerange" {
		t.Fatalf("日期扩展没有保持真实值模式：%v", dateModes)
	}
	inventory := formdata.InventoryTemplateRules(template)
	if len(inventory.NeedsAttention) != 0 {
		t.Fatalf("已注册组件和标准布局不应进入未知能力阻断：%v", inventory.NeedsAttention)
	}
	result := formdata.Generate(formdata.GenerateInput{Template: template, Seed: 9})
	if result.Pending != 3 {
		t.Fatalf("两类附件和外部项目应保留三个必填人工项：%+v", result)
	}
	generatedPaths := make(map[string]bool, len(result.GeneratedFieldPaths))
	for _, fieldPath := range result.GeneratedFieldPaths {
		generatedPaths[fieldPath] = true
	}
	if reasons := formdata.ValidateEditable(template, result.Values, nil, generatedPaths); len(reasons) != 0 {
		t.Fatalf("运行时标准组件生成值没有保持真实形态：%v values=%+v", reasons, result.Values)
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
