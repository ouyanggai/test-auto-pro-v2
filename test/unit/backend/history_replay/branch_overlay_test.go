package history_replay_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/formdata/branchoverlay"
	"test-auto-pro-v2/internal/model"
)

// TestBranchOverlayTargetOperators 覆盖目标 Java 已核实的十种操作符和关键失败边界。
func TestBranchOverlayTargetOperators(t *testing.T) {
	tests := []struct {
		name      string
		condition target.FlowCondition
		values    map[string]any
		satisfied bool
		evaluable bool
		reason    string
	}{
		{name: "lt", condition: target.FlowCondition{FieldA: "amount", ValueB: "2.00", Judge: "lt"}, values: map[string]any{"amount": "1.10"}, satisfied: true, evaluable: true},
		{name: "gt", condition: target.FlowCondition{FieldA: "amount", ValueB: "2", Judge: "gt"}, values: map[string]any{"amount": 3}, satisfied: true, evaluable: true},
		{name: "lte", condition: target.FlowCondition{FieldA: "amount", ValueB: "2", Judge: "lte"}, values: map[string]any{"amount": "2.00"}, satisfied: true, evaluable: true},
		{name: "gte", condition: target.FlowCondition{FieldA: "amount", ValueB: "2", Judge: "gte"}, values: map[string]any{"amount": "2"}, satisfied: true, evaluable: true},
		{name: "eq-number", condition: target.FlowCondition{FieldA: "amount", ValueB: "2", Judge: "eq"}, values: map[string]any{"amount": 2}, satisfied: true, evaluable: true},
		{name: "eq-bigdecimal-scale", condition: target.FlowCondition{FieldA: "amount", ValueB: "2.00", Judge: "eq"}, values: map[string]any{"amount": 2}, satisfied: false, evaluable: true},
		{name: "eq-bigdecimal-negative-scale", condition: target.FlowCondition{FieldA: "amount", ValueB: "1000", Judge: "eq"}, values: map[string]any{"amount": json.Number("1E+3")}, satisfied: false, evaluable: true},
		{name: "neq", condition: target.FlowCondition{FieldA: "amount", ValueB: "2", Judge: "neq"}, values: map[string]any{"amount": "3"}, satisfied: true, evaluable: true},
		{name: "contains-b-includes-a", condition: target.FlowCondition{FieldA: "needle", ValueB: "abcdef", Judge: "contains"}, values: map[string]any{"needle": "bcd"}, satisfied: true, evaluable: true},
		{name: "is-update", condition: target.FlowCondition{FieldA: "current", ValueB: "old", Judge: "is_update"}, values: map[string]any{"current": "new"}, satisfied: true, evaluable: true},
		{name: "is-not-null", condition: target.FlowCondition{FieldA: "items", Judge: "is_not_null"}, values: map[string]any{"items": []any{"one"}}, satisfied: true, evaluable: true},
		{name: "boolean-contradiction", condition: target.FlowCondition{FieldA: "flag", Judge: "boolean_value"}, values: map[string]any{"flag": "Y"}, satisfied: false, evaluable: true, reason: "不可满足"},
		{name: "unknown-operator", condition: target.FlowCondition{FieldA: "amount", ValueB: "2", Judge: "regex"}, values: map[string]any{"amount": 2}, satisfied: false, evaluable: false},
		{name: "number-conversion", condition: target.FlowCondition{FieldA: "amount", ValueB: "two", Judge: "gt"}, values: map[string]any{"amount": 3}, satisfied: false, evaluable: false},
		{name: "contains-type", condition: target.FlowCondition{FieldA: "needle", ValueB: "abcdef", Judge: "contains"}, values: map[string]any{"needle": []any{"bcd"}}, satisfied: false, evaluable: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := branchoverlay.EvaluateCondition(test.condition, test.values)
			if got.Satisfied != test.satisfied || got.Evaluable != test.evaluable {
				t.Fatalf("求值结果 = %#v，期望 satisfied=%v evaluable=%v", got, test.satisfied, test.evaluable)
			}
			if test.reason != "" && !strings.Contains(got.Reason, test.reason) {
				t.Fatalf("求值原因 %q 不包含 %q", got.Reason, test.reason)
			}
		})
	}
}

// TestBranchOverlayFieldRightAndOrderedLogic 验证字段右值和 Java 顺序连接语义不被重排。
func TestBranchOverlayFieldRightAndOrderedLogic(t *testing.T) {
	fieldRight := branchoverlay.EvaluateCondition(target.FlowCondition{FieldA: "start", FieldB: "end", Judge: "lt"}, map[string]any{"start": 1, "end": "2"})
	if !fieldRight.Evaluable || !fieldRight.Satisfied {
		t.Fatalf("字段右值未按目标 BigDecimal 语义命中：%#v", fieldRight)
	}
	ordered := []target.FlowCondition{
		{FieldA: "a", ValueB: "1", Judge: "eq", ConditionType: "and"},
		{FieldA: "b", ValueB: "2", Judge: "eq", ConditionType: "or"},
		{FieldA: "c", ValueB: "3", Judge: "eq"},
	}
	got := branchoverlay.EvaluateConditions(ordered, map[string]any{"a": 1, "b": 0, "c": 3})
	if !got.Evaluable || !got.Satisfied {
		t.Fatalf("目标顺序 and/or 聚合未命中：%#v", got)
	}
	first := branchoverlay.EvaluateConditions([]target.FlowCondition{
		{FieldA: "first", ValueB: "yes", Judge: "eq"},
		{FieldA: "later", ValueB: "yes", Judge: "eq"},
	}, map[string]any{"first": "no", "later": "yes"})
	if !first.Evaluable || first.Satisfied {
		t.Fatalf("目标首个无连接条件没有立即结束：%#v", first)
	}
}

// TestBranchOverlayMinimalPatchPreservesRawData 验证嵌套路径只改条件字段，其他原始正文保持深复制。
func TestBranchOverlayMinimalPatchPreservesRawData(t *testing.T) {
	tree := conditionTree([]target.FlowBranchTemplate{
		{ID: "branch-yes", Sort: 1, Conditions: []target.FlowCondition{{FieldA: "invoice.amount", ValueB: "10", Judge: "gt"}}},
		{ID: "branch-default", Sort: 2},
	})
	values := map[string]any{
		"invoice": map[string]any{"amount": 5.0, "memo": "keep"},
		"rows":    []any{map[string]any{"code": "A", "values": []any{1.0, 2.0, 3.0}}},
	}
	result := branchoverlay.Apply(branchoverlay.Input{
		Tree: tree, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-yes"}}, Values: values,
	})
	if result.Status != branchoverlay.StatusReady {
		t.Fatalf("最小补丁未就绪：%#v", result)
	}
	amount, ok := result.Values["invoice"].(map[string]any)["amount"]
	if !ok || amount != 11.0 {
		t.Fatalf("目标边界候选未命中，得到 %#v", amount)
	}
	if len(result.Patches) != 1 || result.Patches[0].Path != "invoice.amount" || result.Patches[0].BranchKey != "branch-yes" {
		t.Fatalf("补丁明细错误：%#v", result.Patches)
	}
	if got := result.Values["invoice"].(map[string]any)["memo"]; got != "keep" {
		t.Fatalf("普通字段被改写：%#v", got)
	}
	if !reflect.DeepEqual(result.Values["rows"], values["rows"]) {
		t.Fatalf("嵌套数组正文未保留：%#v", result.Values["rows"])
	}
	result.Values["rows"].([]any)[0].(map[string]any)["code"] = "changed"
	if values["rows"].([]any)[0].(map[string]any)["code"] != "A" {
		t.Fatalf("补丁结果未深复制原始正文")
	}
}

// TestBranchOverlayNestedArrayPath 验证条件声明的显式数组首行路径可读写且不扩展数组结构。
func TestBranchOverlayNestedArrayPath(t *testing.T) {
	tree := conditionTree([]target.FlowBranchTemplate{
		{ID: "first-row", Sort: 1, Conditions: []target.FlowCondition{{FieldA: "rows[].amount", ValueB: "10", Judge: "gt"}}},
		{ID: "fallback", Sort: 2},
	})
	result := branchoverlay.Apply(branchoverlay.Input{
		Tree: tree, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "first-row"}},
		Values: map[string]any{"rows": []any{map[string]any{"amount": 5.0}, map[string]any{"amount": 99.0}}},
	})
	if result.Status != branchoverlay.StatusReady {
		t.Fatalf("数组首行路径未能补丁：%#v", result)
	}
	rows := result.Values["rows"].([]any)
	if rows[0].(map[string]any)["amount"] != 11.0 || rows[1].(map[string]any)["amount"] != 99.0 {
		t.Fatalf("数组补丁越界或未命中：%#v", rows)
	}
}

// TestBranchOverlayJSONPointerPath 验证目标字段若以 JSON Pointer 声明仍只按精确下标读写。
func TestBranchOverlayJSONPointerPath(t *testing.T) {
	tree := conditionTree([]target.FlowBranchTemplate{
		{ID: "first-row", Sort: 1, Conditions: []target.FlowCondition{{FieldA: "/rows/0/amount", ValueB: "10", Judge: "gt"}}},
		{ID: "fallback", Sort: 2},
	})
	result := branchoverlay.Apply(branchoverlay.Input{
		Tree: tree, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "first-row"}},
		Values: map[string]any{"rows": []any{map[string]any{"amount": 5.0}}},
	})
	if result.Status != branchoverlay.StatusReady || result.Values["rows"].([]any)[0].(map[string]any)["amount"] != 11.0 {
		t.Fatalf("JSON Pointer 字段路径未按目标边界补丁：%#v", result)
	}
}

// TestBranchOverlayRouteSemantics 覆盖首命中、末分支兜底、嵌套、手动和并行路由。
func TestBranchOverlayRouteSemantics(t *testing.T) {
	firstTree := conditionTree([]target.FlowBranchTemplate{
		{ID: "first", Sort: 1, Conditions: []target.FlowCondition{{FieldA: "amount", ValueB: "10", Judge: "gt"}}},
		{ID: "second", Sort: 2, Conditions: []target.FlowCondition{{FieldA: "amount", ValueB: "5", Judge: "gt"}}},
		{ID: "fallback", Sort: 3},
	})
	first := branchoverlay.Apply(branchoverlay.Input{Tree: firstTree, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "second"}}, Values: map[string]any{"amount": 7}})
	if first.Status != branchoverlay.StatusReady || len(first.Patches) != 0 {
		t.Fatalf("首个不满足时第二分支应直接命中：%#v", first)
	}
	fallback := branchoverlay.Apply(branchoverlay.Input{Tree: firstTree, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "fallback"}}, Values: map[string]any{"amount": 11}})
	if fallback.Status != branchoverlay.StatusReady || len(fallback.Patches) == 0 {
		t.Fatalf("首个分支命中时不能直接选择末分支：%#v", fallback)
	}

	manualTree := conditionTreeWithType("condition", "custom_choose", []target.FlowBranchTemplate{
		{ID: "manual-a", Sort: 1, Conditions: []target.FlowCondition{{FieldA: "flag", ValueB: "Y", Judge: "boolean_value"}}},
		{ID: "manual-b", Sort: 2},
	})
	manual := branchoverlay.Apply(branchoverlay.Input{Tree: manualTree, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "manual-a"}}, Values: map[string]any{"flag": "N", "untouched": map[string]any{"x": true}}})
	if manual.Status != branchoverlay.StatusReady || len(manual.Patches) != 0 || manual.Values["flag"] != "N" {
		t.Fatalf("手动分支不应由表单值补丁：%#v", manual)
	}

	parallel := &target.FlowNodeTemplate{ID: "parallel", Type: "parallel", ParallelNodes: []target.FlowBranchTemplate{
		{ID: "p-a", Sort: 1, Child: conditionTreeID("nested-route", []target.FlowBranchTemplate{{ID: "nested", Sort: 1, Conditions: []target.FlowCondition{{FieldA: "nested.value", ValueB: "1", Judge: "eq"}}}, {ID: "nested-default", Sort: 2}})},
		{ID: "p-b", Sort: 2},
	}}
	parallelResult := branchoverlay.Apply(branchoverlay.Input{Tree: parallel, Choices: []model.ExecutionPathChoice{{RouteNodeID: "nested-route", BranchID: "nested"}}, Values: map[string]any{"nested": map[string]any{"value": 1}}})
	if parallelResult.Status != branchoverlay.StatusReady {
		t.Fatalf("并行全部分支遍历失败：%#v", parallelResult)
	}
}

// TestBranchOverlayNeedsInputCases 覆盖布尔不可满足、字段循环、无解和同一路径等偏移的歧义。
func TestBranchOverlayNeedsInputCases(t *testing.T) {
	missing := branchoverlay.Apply(branchoverlay.Input{Tree: conditionTree(nil), Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "default"}}})
	if missing.Status != branchoverlay.StatusNeedsInput || !hasIssue(missing.Issues, "raw_data_missing") {
		t.Fatalf("缺少原始表单数据却退回空对象：%#v", missing)
	}
	uncopyable := branchoverlay.Apply(branchoverlay.Input{Tree: conditionTree(nil), Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "default"}}, Values: map[string]any{"unsupported": func() {}}})
	if uncopyable.Status != branchoverlay.StatusNeedsInput || !hasIssue(uncopyable.Issues, "raw_data_uncloneable") {
		t.Fatalf("不可复制的原始数据未阻止准备：%#v", uncopyable)
	}

	booleanTree := conditionTree([]target.FlowBranchTemplate{
		{ID: "bool", Sort: 1, Conditions: []target.FlowCondition{{FieldA: "flag", Judge: "boolean_value"}}},
		{ID: "default", Sort: 2},
	})
	boolean := branchoverlay.Apply(branchoverlay.Input{Tree: booleanTree, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "bool"}}, Values: map[string]any{"flag": "Y"}})
	if boolean.Status != branchoverlay.StatusNeedsInput || !hasIssue(boolean.Issues, "unsatisfiable_condition") {
		t.Fatalf("boolean_value 矛盾未阻止准备：%#v", boolean)
	}

	cycleTree := conditionTree([]target.FlowBranchTemplate{
		{ID: "cycle", Sort: 1, Conditions: []target.FlowCondition{{FieldA: "a", FieldB: "b", Judge: "eq"}, {FieldA: "b", FieldB: "a", Judge: "eq"}}},
		{ID: "default", Sort: 2},
	})
	cycle := branchoverlay.Apply(branchoverlay.Input{Tree: cycleTree, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "cycle"}}, Values: map[string]any{"a": 1, "b": 1}})
	if cycle.Status != branchoverlay.StatusNeedsInput || !hasIssue(cycle.Issues, "field_cycle") {
		t.Fatalf("字段循环未阻止准备：%#v", cycle)
	}

	ambiguousTree := conditionTree([]target.FlowBranchTemplate{
		{ID: "different", Sort: 1, Conditions: []target.FlowCondition{{FieldA: "amount", FieldB: "baseline", Judge: "neq"}}},
		{ID: "default", Sort: 2},
	})
	ambiguous := branchoverlay.Apply(branchoverlay.Input{Tree: ambiguousTree, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "different"}}, Values: map[string]any{"amount": 10, "baseline": 10}, Candidates: map[string][]any{"amount": []any{9, 11}}})
	if ambiguous.Status != branchoverlay.StatusNeedsInput || !hasIssue(ambiguous.Issues, "ambiguous_solution") {
		t.Fatalf("同字段同等偏移的多解未进入 needs_input：%#v", ambiguous)
	}

	invalid := branchoverlay.Apply(branchoverlay.Input{Tree: ambiguousTree, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "missing"}}, Values: map[string]any{"amount": 1, "baseline": 1}})
	if invalid.Status != branchoverlay.StatusNeedsInput || !hasIssue(invalid.Issues, "choice_invalid") {
		t.Fatalf("错误分支选择未拒绝：%#v", invalid)
	}
}

// conditionTree 创建固定目标条件路由节点。
func conditionTree(branches []target.FlowBranchTemplate) *target.FlowNodeTemplate {
	return conditionTreeID("route", branches)
}

// conditionTreeWithType 创建指定目标节点类型的测试流程树。
func conditionTreeWithType(nodeType, branchExecuteType string, branches []target.FlowBranchTemplate) *target.FlowNodeTemplate {
	return &target.FlowNodeTemplate{ID: "route", Type: nodeType, BranchExecuteType: branchExecuteType, ConditionNodes: branches}
}

// conditionTreeID 创建指定目标节点标识的条件路由节点。
func conditionTreeID(nodeID string, branches []target.FlowBranchTemplate) *target.FlowNodeTemplate {
	return &target.FlowNodeTemplate{ID: nodeID, Type: "condition", ConditionNodes: branches}
}

// hasIssue 判断结果是否包含指定结构化问题代码。
func hasIssue(issues []branchoverlay.Issue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

// TestKeyFieldsProjectDecisiveConditionFields 验证决定路径的关键字段投影：
// 只来自目标条件声明的真实字段路径，带现值、目标真实候选值和它影响的分支。
func TestKeyFieldsProjectDecisiveConditionFields(t *testing.T) {
	tree := &target.FlowNodeTemplate{
		ID: "start", Type: "start",
		Child: &target.FlowNodeTemplate{
			ID: "route", Type: "condition",
			ConditionNodes: []target.FlowBranchTemplate{
				{
					ID: "branch-long", Child: &target.FlowNodeTemplate{ID: "manager", Type: "common"},
					Conditions: []target.FlowCondition{{FieldA: "days", Judge: "gt", ValueB: "3"}},
				},
				{
					ID: "branch-short", Child: &target.FlowNodeTemplate{ID: "leader", Type: "common"},
					Conditions: []target.FlowCondition{{FieldA: "days", Judge: "lte", ValueB: "3"}},
				},
			},
		},
	}
	fields := branchoverlay.KeyFields(branchoverlay.Input{
		Tree:    tree,
		Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-long"}},
		Values:  map[string]any{"days": json.Number("2"), "remark": "无关字段"},
	})
	if len(fields) != 1 || fields[0].Path != "days" {
		t.Fatalf("关键字段只应包含条件声明的真实字段路径：%+v", fields)
	}
	field := fields[0]
	if !field.HasCurrent || fmt.Sprint(field.Current) != "2" {
		t.Fatalf("关键字段没有带上现值：%+v", field)
	}
	if !field.Decisive {
		t.Fatalf("已选分支涉及的字段必须标记为决定性字段：%+v", field)
	}
	if len(field.Candidates) == 0 {
		t.Fatalf("关键字段没有给出目标条件真实候选值：%+v", field)
	}
	if len(field.Operators) == 0 || len(field.Branches) == 0 {
		t.Fatalf("关键字段没有说明操作符和影响分支：%+v", field)
	}
}
