package backend_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// TestF009DefaultBranchSolvesEarlierConditionsAndDateRange 验证兜底路径避开全部前置分支并同步请假日期。
func TestF009DefaultBranchSolvesEarlierConditionsAndDateRange(t *testing.T) {
	tree := f009SortedRouteTree()
	result := f009Generate(t, tree, []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "fallback"}}, f009LeaveTemplate(), 1)
	if result.GenerationState != "complete" || !result.RouteVerification.Matched {
		t.Fatalf("兜底路径没有得到完整解：%+v", result)
	}
	days, ok := result.Values["vacateDayNum"].(float64)
	if !ok || days > 1 {
		t.Fatalf("兜底路径仍会命中前置审批分支：%#v", result.Values["vacateDayNum"])
	}
	rangeValue, ok := result.Values["vacateDate"].([]any)
	if !ok || len(rangeValue) != 2 || rangeValue[0] != rangeValue[1] {
		t.Fatalf("一天请假没有同步为同日起止日期：%#v", result.Values["vacateDate"])
	}
}

// TestF009SortedMiddleBranchSatisfiesSelectedAndRejectsEarlier 验证中间分支同时成立且排序更靠前分支不成立。
func TestF009SortedMiddleBranchSatisfiesSelectedAndRejectsEarlier(t *testing.T) {
	result := f009Generate(t, f009SortedRouteTree(), []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "short"}}, f009LeaveTemplate(), 7)
	days, ok := result.Values["vacateDayNum"].(float64)
	if !ok || days <= 1 || days > 3 || !result.RouteVerification.Matched {
		t.Fatalf("中间分支没有遵守完整排序语义：days=%#v result=%+v", result.Values["vacateDayNum"], result.RouteVerification)
	}
}

// TestF009SolverSupportsAndOrFieldComparisonContainsAndIn 验证已批准比较方式和字段对字段均可确定性求解。
func TestF009SolverSupportsAndOrFieldComparisonContainsAndIn(t *testing.T) {
	end := &target.FlowNodeTemplate{ID: "end", Name: "结束", Type: "end"}
	route := &target.FlowNodeTemplate{ID: "route", Name: "组合条件", Type: "condition", Child: end, ConditionNodes: []target.FlowBranchTemplate{
		{ID: "selected", Name: "当前路径", Sort: 1, Conditions: []target.FlowCondition{
			{FieldA: "amount", FieldB: "limit", Judge: "gte", ConditionType: "and"},
			{FieldA: "note", ValueB: "测试", Judge: "contains", ConditionType: "and"},
			{FieldA: "kind", ValueB: `["A","B"]`, Judge: "in"},
		}, Child: f009RouteLeaf("selected-node")},
		{ID: "fallback", Name: "其他", Sort: 2, Child: f009RouteLeaf("fallback-node")},
	}}
	start := &target.FlowNodeTemplate{ID: "start", Name: "发起", Type: "start", Child: route, FieldPowers: []target.FlowNodeFieldPower{
		{EnglishName: "amount", Power: "edit"}, {EnglishName: "limit", Power: "edit"}, {EnglishName: "note", Power: "edit"}, {EnglishName: "kind", Power: "edit"},
	}}
	template := `{"list":[{"type":"number","model":"amount","name":"金额","options":{"required":true}},{"type":"number","model":"limit","name":"额度","options":{"required":true}},{"type":"input","model":"note","name":"说明","options":{"required":true}},{"type":"select","model":"kind","name":"类型","options":{"required":true,"options":[{"label":"甲","value":"A"},{"label":"乙","value":"B"}]}}]}`
	result := f009Generate(t, start, []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "selected"}}, template, 11)
	if result.GenerationState != "complete" || !result.RouteVerification.Matched {
		t.Fatalf("AND、字段比较、contains 或 in 未求解：%+v values=%#v", result, result.Values)
	}
}

// TestF009SolverSupportsEqNeqLtLteAndOr 验证剩余比较方式和目标顺序 OR 聚合都可求解。
func TestF009SolverSupportsEqNeqLtLteAndOr(t *testing.T) {
	end := &target.FlowNodeTemplate{ID: "end", Name: "结束", Type: "end"}
	orRoute := &target.FlowNodeTemplate{ID: "or-route", Name: "补充条件", Type: "condition", Child: end, ConditionNodes: []target.FlowBranchTemplate{
		{ID: "or-selected", Name: "当前路径", Sort: 1, Conditions: []target.FlowCondition{
			{FieldA: "note", ValueB: "命中", Judge: "contains", ConditionType: "or"},
			{FieldA: "kind", ValueB: "B", Judge: "eq"},
		}, Child: f009RouteLeaf("or-selected-node")},
		{ID: "or-fallback", Name: "其他", Sort: 2, Child: f009RouteLeaf("or-fallback-node")},
	}}
	numericRoute := &target.FlowNodeTemplate{ID: "numeric-route", Name: "基础条件", Type: "condition", Child: orRoute, ConditionNodes: []target.FlowBranchTemplate{
		{ID: "numeric-selected", Name: "当前路径", Sort: 1, Conditions: []target.FlowCondition{
			{FieldA: "amount", ValueB: "5", Judge: "lt", ConditionType: "and"},
			{FieldA: "amount", ValueB: "4", Judge: "lte", ConditionType: "and"},
			{FieldA: "kind", ValueB: "C", Judge: "neq", ConditionType: "and"},
			{FieldA: "kind", ValueB: "A", Judge: "eq"},
		}, Child: f009RouteLeaf("numeric-selected-node")},
		{ID: "numeric-fallback", Name: "其他", Sort: 2, Child: f009RouteLeaf("numeric-fallback-node")},
	}}
	start := &target.FlowNodeTemplate{ID: "start", Name: "发起", Type: "start", Child: numericRoute, FieldPowers: []target.FlowNodeFieldPower{
		{EnglishName: "amount", Power: "edit"}, {EnglishName: "kind", Power: "edit"}, {EnglishName: "note", Power: "edit"},
	}}
	template := `{"list":[{"type":"number","model":"amount","name":"金额","options":{"required":true}},{"type":"select","model":"kind","name":"类型","options":{"required":true,"options":[{"label":"甲","value":"A"},{"label":"乙","value":"B"},{"label":"丙","value":"C"}]}},{"type":"input","model":"note","name":"说明","options":{"required":true}}]}`
	result := f009Generate(t, start, []model.ExecutionPathChoice{
		{RouteNodeID: "numeric-route", BranchID: "numeric-selected"},
		{RouteNodeID: "or-route", BranchID: "or-selected"},
	}, template, 29)
	if result.GenerationState != "complete" || !result.RouteVerification.Matched {
		t.Fatalf("eq、neq、lt、lte 或 OR 未按目标顺序求解：%+v values=%#v", result, result.Values)
	}
}

// TestF009SolverIsStableBySeedAndVariesOnlyWithinRoute 验证同 seed 结果稳定，不同 seed 的候选仍命中同一路径。
func TestF009SolverIsStableBySeedAndVariesOnlyWithinRoute(t *testing.T) {
	end := &target.FlowNodeTemplate{ID: "end", Name: "结束", Type: "end"}
	tree := &target.FlowNodeTemplate{ID: "start", Name: "发起", Type: "start", FieldPowers: []target.FlowNodeFieldPower{{EnglishName: "amount", Power: "edit"}}, Child: &target.FlowNodeTemplate{
		ID: "route", Name: "金额条件", Type: "condition", Child: end, ConditionNodes: []target.FlowBranchTemplate{
			{ID: "selected", Sort: 1, Conditions: []target.FlowCondition{{FieldA: "amount", ValueB: "1", Judge: "gt"}}, Child: f009RouteLeaf("selected-node")},
			{ID: "fallback", Sort: 2, Child: f009RouteLeaf("fallback-node")},
		},
	}}
	choices := []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "selected"}}
	template := `{"list":[{"type":"number","model":"amount","name":"金额","options":{"required":true}}]}`
	first := f009Generate(t, tree, choices, template, 1)
	repeated := f009Generate(t, tree, choices, template, 1)
	if !reflect.DeepEqual(first.Values, repeated.Values) {
		t.Fatalf("相同模板、路径和 seed 生成结果不稳定：first=%#v repeated=%#v", first.Values, repeated.Values)
	}
	foundDifferent := false
	for seed := int64(2); seed <= 32; seed++ {
		next := f009Generate(t, tree, choices, template, seed)
		if !next.RouteVerification.Matched {
			t.Fatalf("换一组生成了不命中当前路径的候选：seed=%d values=%#v", seed, next.Values)
		}
		if !reflect.DeepEqual(first.Values, next.Values) {
			foundDifferent = true
			break
		}
	}
	if !first.RouteVerification.Matched || !foundDifferent {
		t.Fatalf("有多个有效候选时不同 seed 没有产生下一组：first=%#v", first.Values)
	}
}

// TestF009SolverTraversesNestedParallelConditions 验证并行分支中的嵌套条件都参与完整路径复验。
func TestF009SolverTraversesNestedParallelConditions(t *testing.T) {
	endA := &target.FlowNodeTemplate{ID: "end-a", Name: "结束甲", Type: "end"}
	endB := &target.FlowNodeTemplate{ID: "end-b", Name: "结束乙", Type: "end"}
	routeA := &target.FlowNodeTemplate{ID: "route-a", Name: "甲条件", Type: "condition", Child: endA, ConditionNodes: []target.FlowBranchTemplate{
		{ID: "a-high", Name: "高", Sort: 1, Conditions: []target.FlowCondition{{FieldA: "amount", ValueB: "5", Judge: "gt"}}, Child: f009RouteLeaf("a-high-node")},
		{ID: "a-low", Name: "低", Sort: 2, Child: f009RouteLeaf("a-low-node")},
	}}
	routeB := &target.FlowNodeTemplate{ID: "route-b", Name: "乙条件", Type: "condition", Child: endB, ConditionNodes: []target.FlowBranchTemplate{
		{ID: "b-a", Name: "甲类", Sort: 1, Conditions: []target.FlowCondition{{FieldA: "kind", ValueB: `["A"]`, Judge: "in"}}, Child: f009RouteLeaf("b-a-node")},
		{ID: "b-other", Name: "其他", Sort: 2, Child: f009RouteLeaf("b-other-node")},
	}}
	parallelEnd := &target.FlowNodeTemplate{ID: "parallel-end", Name: "并行结束", Type: "end"}
	parallel := &target.FlowNodeTemplate{ID: "parallel", Name: "并行", Type: "parallel", Child: parallelEnd, ParallelNodes: []target.FlowBranchTemplate{{ID: "pa", Child: routeA}, {ID: "pb", Child: routeB}}}
	start := &target.FlowNodeTemplate{ID: "start", Name: "发起", Type: "start", Child: parallel, FieldPowers: []target.FlowNodeFieldPower{{EnglishName: "amount", Power: "edit"}, {EnglishName: "kind", Power: "edit"}}}
	template := `{"list":[{"type":"number","model":"amount","name":"金额","options":{"required":true}},{"type":"select","model":"kind","name":"类型","options":{"required":true,"options":[{"label":"甲","value":"A"},{"label":"乙","value":"B"}]}}]}`
	result := f009Generate(t, start, []model.ExecutionPathChoice{{RouteNodeID: "route-a", BranchID: "a-low"}, {RouteNodeID: "route-b", BranchID: "b-a"}}, template, 19)
	if result.GenerationState != "complete" || !result.RouteVerification.Matched {
		t.Fatalf("并行嵌套条件未完整求解：%+v values=%#v", result, result.Values)
	}
}

// TestF009SolverTraversesManualBranchCommonContinuation 验证手动分支选中后仍会求解公共续接上的全部条件节点。
func TestF009SolverTraversesManualBranchCommonContinuation(t *testing.T) {
	end := &target.FlowNodeTemplate{ID: "end", Name: "结束", Type: "end"}
	routeB := &target.FlowNodeTemplate{ID: "route-b", Name: "类型条件", Type: "condition", Child: end, ConditionNodes: []target.FlowBranchTemplate{
		{ID: "type-a", Name: "甲类", Sort: 1, Conditions: []target.FlowCondition{{FieldA: "kind", ValueB: `["A"]`, Judge: "in"}}, Child: f009RouteLeaf("type-a-node")},
		{ID: "type-other", Name: "其他", Sort: 2, Child: f009RouteLeaf("type-other-node")},
	}}
	routeA := &target.FlowNodeTemplate{ID: "route-a", Name: "金额条件", Type: "condition", Child: routeB, ConditionNodes: []target.FlowBranchTemplate{
		{ID: "high", Name: "高金额", Sort: 1, Conditions: []target.FlowCondition{{FieldA: "amount", ValueB: "5", Judge: "gt"}}, Child: f009RouteLeaf("high-node")},
		{ID: "low", Name: "低金额", Sort: 2, Child: f009RouteLeaf("low-node")},
	}}
	manual := &target.FlowNodeTemplate{ID: "manual", Name: "人工选择", Type: "condition", BranchExecuteType: "custom_choose", Child: routeA, ConditionNodes: []target.FlowBranchTemplate{
		{ID: "manual-a", Name: "线路甲", Child: f009RouteLeaf("manual-a-node")},
		{ID: "manual-b", Name: "线路乙", Child: f009RouteLeaf("manual-b-node")},
	}}
	start := &target.FlowNodeTemplate{ID: "start", Name: "发起", Type: "start", Child: manual, FieldPowers: []target.FlowNodeFieldPower{{EnglishName: "amount", Power: "edit"}, {EnglishName: "kind", Power: "edit"}}}
	template := `{"list":[{"type":"number","model":"amount","name":"金额","options":{"required":true}},{"type":"select","model":"kind","name":"类型","options":{"required":true,"options":[{"label":"甲","value":"A"},{"label":"乙","value":"B"}]}}]}`
	result := f009Generate(t, start, []model.ExecutionPathChoice{
		{RouteNodeID: "manual", BranchID: "manual-a"},
		{RouteNodeID: "route-a", BranchID: "low"},
		{RouteNodeID: "route-b", BranchID: "type-a"},
	}, template, 23)
	amount, amountOK := result.Values["amount"].(float64)
	if result.GenerationState != "complete" || !result.RouteVerification.Matched || !amountOK || amount > 5 || result.Values["kind"] != "A" {
		t.Fatalf("手动分支后的公共条件没有完整求解：result=%+v values=%#v", result, result.Values)
	}
}

// TestF009UnknownConditionReturnsPartialResult 验证未知比较方式返回 2xx 可表达结果而不是业务冲突错误。
func TestF009UnknownConditionReturnsPartialResult(t *testing.T) {
	end := &target.FlowNodeTemplate{ID: "end", Name: "结束", Type: "end"}
	tree := &target.FlowNodeTemplate{ID: "start", Name: "发起", Type: "start", FieldPowers: []target.FlowNodeFieldPower{{EnglishName: "amount", Power: "edit"}}, Child: &target.FlowNodeTemplate{
		ID: "route", Name: "脚本条件", Type: "condition", Child: end, ConditionNodes: []target.FlowBranchTemplate{
			{ID: "script", Sort: 1, Conditions: []target.FlowCondition{{FieldA: "amount", ValueB: "1", Judge: "dynamic_script"}}, Child: f009RouteLeaf("script-node")},
			{ID: "fallback", Sort: 2, Child: f009RouteLeaf("script-fallback-node")},
		},
	}}
	result := f009Generate(t, tree, []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "script"}}, `{"list":[{"type":"number","model":"amount","name":"金额","options":{"required":true}}]}`, 3)
	if result.GenerationState == "complete" || len(result.Issues) == 0 || result.RouteVerification.Matched {
		t.Fatalf("未知比较方式被错误猜测或没有返回问题：%+v", result)
	}
}

// f009SortedRouteTree 构造三段有序请假天数路由，最后一项为真实兜底。
func f009SortedRouteTree() *target.FlowNodeTemplate {
	end := &target.FlowNodeTemplate{ID: "end", Name: "结束", Type: "end"}
	route := &target.FlowNodeTemplate{ID: "route", Name: "请假天数", Type: "condition", Child: end, ConditionNodes: []target.FlowBranchTemplate{
		{ID: "long", Name: "长期", Sort: 10, Conditions: []target.FlowCondition{{FieldA: "vacateDayNum", ValueB: "3", Judge: "gt"}}, Child: f009RouteLeaf("long-node")},
		{ID: "short", Name: "短期", Sort: 20, Conditions: []target.FlowCondition{{FieldA: "vacateDayNum", ValueB: "1", Judge: "gt"}}, Child: f009RouteLeaf("short-node")},
		{ID: "fallback", Name: "其他", Sort: 30, Child: f009RouteLeaf("fallback-node")},
	}}
	return &target.FlowNodeTemplate{ID: "start", Name: "发起", Type: "start", Child: route, FieldPowers: []target.FlowNodeFieldPower{{EnglishName: "vacateDayNum", Power: "edit"}, {EnglishName: "vacateDate", Power: "edit"}}}
}

// f009RouteLeaf 构造条件分支业务叶节点，由路由公共 child 负责汇合。
func f009RouteLeaf(id string) *target.FlowNodeTemplate {
	return &target.FlowNodeTemplate{ID: id, Name: id, Type: "empty"}
}

// f009LeaveTemplate 返回请假天数和日期区间的一对一真实模板结构。
func f009LeaveTemplate() string {
	return `{"list":[{"type":"number","model":"vacateDayNum","name":"请假天数","options":{"required":true}},{"type":"date","model":"vacateDate","name":"请假日期","options":{"required":true,"type":"daterange"}}]}`
}

// f009Generate 用真实配置服务边界执行单条智能生成。
func f009Generate(t *testing.T, tree *target.FlowNodeTemplate, choices []model.ExecutionPathChoice, template string, seed int64) model.PathFormGenerateResult {
	t.Helper()
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 91, Account: "account", FlowSource: "new", TargetObjectID: "template", TargetObjectName: "测试流程", Status: model.PlanStatusPendingConfiguration}}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 92, PlanID: 91, SequenceNo: 1, Name: "路径 1", Choices: choices}}}
	serviceUnderTest := service.NewPathConfigService(
		service.NewPlanService(plans),
		pathConfigSnapshotReader{snapshot: target.PathConfigurationSnapshot{Tree: tree, EntryNodeIDs: []string{"start"}, Forms: []target.FormRuntimeTemplate{{Name: "申请表", TemplateData: template}}}},
		analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathConfigAnalyzer(), paths, emptyPathConfigRepository{},
	)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := serviceUnderTest.GenerateForm(ctx, 91, 92, seed, map[string]any{}, []string{}, false)
	if err != nil {
		t.Fatalf("智能生成不应返回冲突错误：%v", err)
	}
	return result
}
