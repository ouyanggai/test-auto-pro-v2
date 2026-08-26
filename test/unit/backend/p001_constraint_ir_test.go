package backend_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/formdata"
	"test-auto-pro-v2/internal/model"
)

// TestP001ConstraintIRReportsEmptyNumericIntersection 验证冲突数值约束返回精确空区间问题。
func TestP001ConstraintIRReportsEmptyNumericIntersection(t *testing.T) {
	fields := []formdata.Field{{Path: "amount", Name: "申请金额", Type: "number", Required: true}}
	ir := formdata.CompileConstraintIR(fields, []formdata.Constraint{
		{Field: "amount", FieldType: "number", Op: "gt", Value: float64(10000)},
		{Field: "amount", FieldType: "number", Op: "lt", Value: float64(5000)},
	}, nil, nil)
	if ir.Status != formdata.ConstraintStatusBlocked || len(ir.Issues) != 1 {
		t.Fatalf("冲突区间没有被静态阻断：%+v", ir)
	}
	issue := ir.Issues[0]
	if issue.Code != "empty_numeric_interval" || issue.FieldPath != "amount" || issue.Operator != "interval" || issue.Source != "compile" {
		t.Fatalf("空区间问题缺少结构化上下文：%+v", issue)
	}
}

// TestP001ConstraintIRPropagatesConditionalRequired 验证静态条件成立后会从真实枚举候选补齐依赖必填字段。
func TestP001ConstraintIRPropagatesConditionalRequired(t *testing.T) {
	fields := []formdata.Field{
		{Path: "projectType", Name: "项目类型", Type: "select", Options: []any{"A", "B"}},
		{Path: "department", Name: "归属部门", Type: "select", Options: []any{"研发部", "财务部"}},
	}
	ir := formdata.CompileConstraintIR(fields, []formdata.Constraint{{Field: "department", Op: "required_if", ValueField: "projectType", Value: "A"}}, nil, nil)
	result := formdata.PropagateConstraintIR(ir, map[string]any{"projectType": "A"}, 7)
	if result.Status != formdata.ConstraintStatusReady || result.Values["department"] == nil {
		t.Fatalf("条件必填字段没有从模板候选传播：%+v", result)
	}
	if !reflect.DeepEqual(result.GeneratedFieldPaths, []string{"department"}) {
		t.Fatalf("传播所有权记录错误：%v", result.GeneratedFieldPaths)
	}
}

// TestP001ConstraintIRBlocksDependencyCycle 验证字段依赖环被阻断且列出相关字段。
func TestP001ConstraintIRBlocksDependencyCycle(t *testing.T) {
	fields := []formdata.Field{{Path: "a", Name: "字段 A", Type: "input"}, {Path: "b", Name: "字段 B", Type: "input"}}
	ir := formdata.CompileConstraintIR(fields, []formdata.Constraint{
		{Field: "a", Op: "required_if", ValueField: "b", Value: "yes"},
		{Field: "b", Op: "required_if", ValueField: "a", Value: "yes"},
	}, nil, nil)
	if ir.Status != formdata.ConstraintStatusBlocked || len(ir.Issues) != 1 || ir.Issues[0].Code != "dependency_cycle" {
		t.Fatalf("循环依赖没有被结构化阻断：%+v", ir)
	}
	if !reflect.DeepEqual(ir.Issues[0].RelatedFields, []string{"a", "b"}) {
		t.Fatalf("循环依赖字段不完整：%v", ir.Issues[0].RelatedFields)
	}
}

// TestP001ConstraintIRSynchronizesDateBinding 验证日期绑定在数值传播后按自然日同步。
func TestP001ConstraintIRSynchronizesDateBinding(t *testing.T) {
	fields := []formdata.Field{
		{Path: "days", Name: "天数", Type: "number"},
		{Path: "period", Name: "日期区间", Type: "date", Mode: "daterange"},
	}
	ir := formdata.CompileConstraintIR(fields, nil, []formdata.DateRangeBinding{{DurationField: "days", RangeField: "period"}}, nil)
	result := formdata.PropagateConstraintIR(ir, map[string]any{"days": float64(3), "period": []any{"2026-08-01", "2026-08-01"}}, 1)
	if result.Status != formdata.ConstraintStatusReady || !reflect.DeepEqual(result.Values["period"], []any{"2026-08-01", "2026-08-03"}) {
		t.Fatalf("日期绑定没有同步或复验失败：%+v", result)
	}
}

// TestP001ConstraintIRIsByteStableAndOnlyChangesOwnedField 验证相同 seed 字节级稳定，换 seed 不改写无关人工字段。
func TestP001ConstraintIRIsByteStableAndOnlyChangesOwnedField(t *testing.T) {
	fields := []formdata.Field{
		{Path: "amount", Name: "金额", Type: "number"},
		{Path: "title", Name: "标题", Type: "input"},
	}
	constraints := []formdata.Constraint{
		{Field: "amount", Op: "gte", Value: float64(1)},
		{Field: "amount", Op: "lte", Value: float64(5)},
	}
	ir := formdata.CompileConstraintIR(fields, constraints, nil, nil)
	base := map[string]any{"amount": float64(2), "title": "人工标题"}
	first := formdata.PropagateConstraintIR(ir, base, 1)
	repeated := formdata.PropagateConstraintIR(ir, base, 1)
	next := formdata.PropagateConstraintIR(ir, base, 2)
	firstJSON, _ := json.Marshal(first)
	repeatedJSON, _ := json.Marshal(repeated)
	if string(firstJSON) != string(repeatedJSON) {
		t.Fatalf("相同 seed 的传播 JSON 不稳定：%s != %s", firstJSON, repeatedJSON)
	}
	if first.Values["title"] != "人工标题" || next.Values["title"] != "人工标题" || first.Values["amount"] == next.Values["amount"] {
		t.Fatalf("换 seed 未限制在约束字段所有权内：first=%#v next=%#v", first.Values, next.Values)
	}
}

// TestP001PathSolverMarksSuccessfulFallbackSource 验证传播命中靠前分支时由现有有界搜索纠正并公开 fallback 来源。
func TestP001PathSolverMarksSuccessfulFallbackSource(t *testing.T) {
	end := &target.FlowNodeTemplate{ID: "end", Name: "结束", Type: "end"}
	tree := &target.FlowNodeTemplate{ID: "start", Name: "发起", Type: "start", FieldPowers: []target.FlowNodeFieldPower{{EnglishName: "kind", Power: "edit"}}, Child: &target.FlowNodeTemplate{
		ID: "route", Name: "类型条件", Type: "condition", Child: end, ConditionNodes: []target.FlowBranchTemplate{
			{ID: "earlier", Sort: 1, Conditions: []target.FlowCondition{{FieldA: "kind", ValueB: "B", Judge: "eq"}}, Child: f009RouteLeaf("earlier-node")},
			{ID: "selected", Sort: 2, Conditions: []target.FlowCondition{{FieldA: "kind", ValueB: "A", Judge: "neq"}}, Child: f009RouteLeaf("selected-node")},
			{ID: "fallback", Sort: 3, Child: f009RouteLeaf("fallback-node")},
		},
	}}
	template := `{"list":[{"type":"select","model":"kind","name":"类型","options":{"required":true,"options":[{"label":"甲","value":"A"},{"label":"乙","value":"B"},{"label":"丙","value":"C"}]}}]}`
	result := f009Generate(t, tree, []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "selected"}}, template, 2)
	if !result.RouteVerification.Matched || result.RouteVerification.Source != "fallback" || result.Values["kind"] != "C" {
		t.Fatalf("有界回退未纠正靠前分支或缺少来源：%+v values=%#v", result.RouteVerification, result.Values)
	}
}
