package backend_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/formdata"
	"test-auto-pro-v2/internal/model"
)

// TestF011GoldenFieldContractsRoundTrip 验证两类表单字段契约和值形态均能按固定样例回放。
func TestF011GoldenFieldContractsRoundTrip(t *testing.T) {
	fixture := f011ContractFixture(t)
	formMaking := fixture["formMaking"].(map[string]any)
	template := formMaking["template"].(map[string]any)
	fields, unsupported := formdata.ParseTemplate(template)
	if len(unsupported) != 0 {
		t.Fatalf("黄金模板包含未识别能力：%v", unsupported)
	}
	fieldByPath := make(map[string]formdata.Field, len(fields))
	for _, field := range fields {
		fieldByPath[field.Path] = field
	}
	if len(fields) != 7 || len(fieldByPath["region"].OptionPaths) != 1 || fieldByPath["items[].name"].CollectionRoot != "items" {
		t.Fatalf("FormMaking 字段、级联或集合契约发生漂移：%+v", fields)
	}
	if fieldByPath["project"].Capability != "custome-select-project" || fieldByPath["project"].Type != "custom" {
		t.Fatalf("JSON 字符串项目组件契约发生漂移：%+v", fieldByPath["project"])
	}

	identityRaw := formMaking["identity"].(map[string]any)
	result := formdata.Generate(formdata.GenerateInput{
		Template:            template,
		Base:                formMaking["base"].(map[string]any),
		Seed:                17,
		Initiator:           "测试用户",
		ManualOverridePaths: []string{"items[].name"},
		ComponentCandidates: f011CandidateMap(formMaking["componentCandidates"].(map[string]any)),
		Identity: formdata.IdentityContext{
			Company:    f011IdentityNode(identityRaw["company"].(map[string]any)),
			Department: f011IdentityNode(identityRaw["department"].(map[string]any)),
			User:       f011IdentityNode(identityRaw["user"].(map[string]any)),
		},
	})
	if result.Pending != 0 || len(result.Unsupported) != 0 {
		t.Fatalf("黄金 FormMaking 样例没有完整生成：%+v", result)
	}
	region, regionOK := result.Values["region"].([]any)
	items, itemsOK := result.Values["items"].([]any)
	project, projectOK := result.Values["project"].(string)
	applicant, applicantOK := result.Values["myUserName"].(string)
	if !regionOK || len(region) != 2 || region[1] != "shanghai" || !itemsOK || len(items) != 1 {
		t.Fatalf("级联完整路径或子表单行结构发生漂移：%#v", result.Values)
	}
	if !projectOK || !applicantOK || !json.Valid([]byte(project)) || !json.Valid([]byte(applicant)) {
		t.Fatalf("自定义组件不再保持 JSON 字符串序列化：project=%#v applicant=%#v", result.Values["project"], result.Values["myUserName"])
	}
	if reasons := formdata.Validate(template, result.Values, nil); len(reasons) != 0 {
		t.Fatalf("黄金 FormMaking 值没有通过同规则复验：%v", reasons)
	}

	encodedVue, err := json.Marshal(fixture["vueCustom"])
	if err != nil {
		t.Fatalf("Vue 样例编码失败：%v", err)
	}
	var vueRule target.VueCustomPageRule
	if err := json.Unmarshal(encodedVue, &vueRule); err != nil {
		t.Fatalf("Vue 样例解码失败：%v", err)
	}
	if vueRule.Status != "complete" || len(vueRule.Fields) != 2 || vueRule.Fields[1].Path != "form.lines" || !vueRule.Fields[1].Collection {
		t.Fatalf("vue_custom 字段路径和值形态契约发生漂移：%+v", vueRule)
	}
	if len(vueRule.ReadRequests) != 0 || len(vueRule.Issues) != 1 {
		t.Fatalf("空只读清单必须显式保留影子观测问题：%+v", vueRule)
	}
}

// TestF011EightOperatorsKeepGoldenSemantics 验证现有八个路径操作符的接受与拒绝语义均有固定回放。
func TestF011EightOperatorsKeepGoldenSemantics(t *testing.T) {
	tests := []struct {
		name       string
		constraint formdata.Constraint
		accepted   any
		rejected   any
	}{
		{name: "eq", constraint: formdata.Constraint{Field: "value", Op: "eq", Value: "A"}, accepted: "A", rejected: "B"},
		{name: "neq", constraint: formdata.Constraint{Field: "value", Op: "neq", Value: "A"}, accepted: "B", rejected: "A"},
		{name: "gt", constraint: formdata.Constraint{Field: "value", Op: "gt", Value: float64(10)}, accepted: float64(11), rejected: float64(10)},
		{name: "gte", constraint: formdata.Constraint{Field: "value", Op: "gte", Value: float64(10)}, accepted: float64(10), rejected: float64(9)},
		{name: "lt", constraint: formdata.Constraint{Field: "value", Op: "lt", Value: float64(10)}, accepted: float64(9), rejected: float64(10)},
		{name: "lte", constraint: formdata.Constraint{Field: "value", Op: "lte", Value: float64(10)}, accepted: float64(10), rejected: float64(11)},
		{name: "contains", constraint: formdata.Constraint{Field: "value", Op: "contains", Value: "测试"}, accepted: "测试申请", rejected: "普通申请"},
		{name: "in", constraint: formdata.Constraint{Field: "value", Op: "in", Value: []any{"A", "B"}}, accepted: "B", rejected: "C"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			fieldType := "input"
			if testCase.name == "gt" || testCase.name == "gte" || testCase.name == "lt" || testCase.name == "lte" {
				fieldType = "number"
			}
			caseTemplate := map[string]any{"list": []any{map[string]any{"type": fieldType, "model": "value", "name": "契约字段", "options": map[string]any{}}}}
			if reasons := formdata.Validate(caseTemplate, map[string]any{"value": testCase.accepted}, []formdata.Constraint{testCase.constraint}); len(reasons) != 0 {
				t.Fatalf("操作符 %s 拒绝了黄金有效值：%v", testCase.name, reasons)
			}
			if reasons := formdata.Validate(caseTemplate, map[string]any{"value": testCase.rejected}, []formdata.Constraint{testCase.constraint}); len(reasons) == 0 {
				t.Fatalf("操作符 %s 接受了黄金无效值", testCase.name)
			}
		})
	}
}

// FormRuntimeSession 为路径冲突回放提供不含凭证的当前会话事实。
func (r pathConfigSnapshotReader) FormRuntimeSession(context.Context, string) (target.FormRuntimeSession, error) {
	return target.FormRuntimeSession{SID: "test-session"}, nil
}

// TestF011PathConflictKeepsStructuredBlockingIssue 验证无解路径明确返回字段、约束原因和阻断等级。
func TestF011PathConflictKeepsStructuredBlockingIssue(t *testing.T) {
	end := &target.FlowNodeTemplate{ID: "end", Name: "结束", Type: "end"}
	tree := &target.FlowNodeTemplate{
		ID: "start", Name: "发起", Type: "start",
		FieldPowers: []target.FlowNodeFieldPower{{EnglishName: "amount", Power: "edit"}},
		Child: &target.FlowNodeTemplate{
			ID: "route", Name: "金额条件", Type: "condition", Child: end,
			ConditionNodes: []target.FlowBranchTemplate{
				{ID: "conflict", Name: "冲突分支", Sort: 1, Conditions: []target.FlowCondition{
					{FieldA: "amount", ValueB: "10000", Judge: "gt", ConditionType: "and"},
					{FieldA: "amount", ValueB: "5000", Judge: "lt", ConditionType: "and"},
				}, Child: f009RouteLeaf("conflict-node")},
				{ID: "fallback", Name: "其他", Sort: 2, Child: f009RouteLeaf("fallback-node")},
			},
		},
	}
	result := f009Generate(t, tree, []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "conflict"}}, `{"list":[{"type":"number","model":"amount","name":"申请金额","options":{"required":true,"min":0,"max":20000}}]}`, 23)
	if result.GenerationState == "complete" || result.RouteVerification.Matched || len(result.Issues) == 0 {
		t.Fatalf("冲突路径被错误标记为完整：%+v", result)
	}
	issue := result.Issues[0]
	if strings.TrimSpace(issue.Field) == "" || strings.TrimSpace(issue.Reason) == "" || !issue.Blocking {
		t.Fatalf("冲突问题缺少字段、约束原因或阻断等级：%+v", issue)
	}
}

// f011ContractFixture 读取根目录 test/fixtures 中的无敏感信息黄金样例。
func f011ContractFixture(t *testing.T) map[string]any {
	t.Helper()
	path := filepath.Join("..", "..", "fixtures", "f011_smart_form_contract_baseline.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取智能表单契约样例失败：%v", err)
	}
	var fixture map[string]any
	if err := json.Unmarshal(content, &fixture); err != nil {
		t.Fatalf("解析智能表单契约样例失败：%v", err)
	}
	return fixture
}

// f011CandidateMap 把 JSON 样例中的字段候选转换为生成器输入。
func f011CandidateMap(raw map[string]any) map[string][]any {
	result := make(map[string][]any, len(raw))
	for field, candidates := range raw {
		result[field], _ = candidates.([]any)
	}
	return result
}

// f011IdentityNode 把 JSON 样例中的身份节点转换为生成器输入。
func f011IdentityNode(raw map[string]any) formdata.IdentityNode {
	return formdata.IdentityNode{
		ID:        strings.TrimSpace(raw["id"].(string)),
		Name:      strings.TrimSpace(raw["name"].(string)),
		Type:      strings.TrimSpace(raw["type"].(string)),
		ParentID:  strings.TrimSpace(f011OptionalText(raw["parentId"])),
		CompanyID: strings.TrimSpace(f011OptionalText(raw["companyId"])),
	}
}

// f011OptionalText 安全读取黄金样例中的可选字符串字段。
func f011OptionalText(value any) string {
	text, _ := value.(string)
	return text
}
