package backend_test

import (
	"context"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// TestPathConfigWorkspaceUsesTargetOrderedBranchRuntime 验证生成、提示和保存共用目标平台的有序分支语义与日期区间绑定。
func TestPathConfigWorkspaceUsesTargetOrderedBranchRuntime(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration, Account: "account-a", FlowSource: "new", TargetObjectID: "template-a"}}
	snapshot := pathConfigWorkspaceSnapshot()
	longApproval := snapshot.Tree.Child.ConditionNodes[0].Child
	fallbackApproval := snapshot.Tree.Child.ConditionNodes[1].Child
	snapshot.Tree.Child.ConditionNodes = []target.FlowBranchTemplate{
		{ID: "two-days", Name: "两天", Sort: 10, Conditions: []target.FlowCondition{{FieldA: "durationValue", Judge: "lte", ValueB: "2"}}, Child: &target.FlowNodeTemplate{ID: "approve-two", Name: "两天审批", Type: "common"}},
		{ID: "four-days", Name: "四天", Sort: 20, Conditions: []target.FlowCondition{{FieldA: "durationValue", Judge: "gt", ValueB: "2", ConditionType: "and"}, {FieldA: "durationValue", Judge: "lte", ValueB: "4"}}, Child: &target.FlowNodeTemplate{ID: "approve-four", Name: "四天审批", Type: "common"}},
		{ID: "fifteen-days", Name: "十五天", Sort: 30, Conditions: []target.FlowCondition{{FieldA: "durationValue", Judge: "gte", ValueB: "15"}}, Child: longApproval},
		{ID: "fallback", Name: "其他天数", Sort: 40, Child: fallbackApproval},
	}
	snapshot.Tree.FieldPowers = []target.FlowNodeFieldPower{{EnglishName: "durationValue", Power: "edit"}, {EnglishName: "periodValue", Power: "edit"}, {EnglishName: "reason", Power: "edit"}}
	snapshot.Forms[0].TemplateData = `{"list":[{"type":"number","model":"durationValue","name":"数值字段","options":{"required":true}},{"type":"date","model":"periodValue","name":"日期字段","options":{"required":true,"type":"daterange"}},{"type":"textarea","model":"reason","name":"原因","options":{"required":true}}],"config":{}}`
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "fifteen-days"}}}}}
	configs := &memoryPathConfigRepository{}
	serviceUnderTest := newPathConfigService(t, plans, &pathConfigReader{snapshot: snapshot}, paths, configs)

	generated, err := serviceUnderTest.GenerateForm(context.Background(), 7, 32, 9, nil, nil, false)
	if err != nil {
		t.Fatalf("有序条件分支生成失败：%v", err)
	}
	duration, ok := workspaceNumber(generated.Values["durationValue"])
	if !ok || duration < 15 || duration != float64(int(duration)) {
		t.Fatalf("十五天分支没有生成有效条件天数：%+v", generated.Values)
	}
	dateRange, ok := generated.Values["periodValue"].([]any)
	if !ok || len(dateRange) != 2 {
		t.Fatalf("日期区间没有使用 FormMaking 数组格式：%+v", generated.Values["periodValue"])
	}
	start, startErr := time.Parse("2006-01-02", dateRange[0].(string))
	end, endErr := time.Parse("2006-01-02", dateRange[1].(string))
	if startErr != nil || endErr != nil || end.Sub(start).Hours()/24 != duration-1 {
		t.Fatalf("日期区间没有按条件天数的自然日含首尾生成：duration=%v range=%+v", duration, dateRange)
	}
	if len(generated.ConditionHints) != 1 || !generated.ConditionHints[0].Protected || !generated.ConditionHints[0].Active || generated.ConditionHints[0].BranchName != "十五天" {
		t.Fatalf("提示没有只展示并高亮当前路径的十五天分支：%+v", generated.ConditionHints)
	}
	if len(generated.FieldRules) != 1 || generated.FieldRules[0].Field != "durationValue" || !generated.FieldRules[0].Disabled {
		t.Fatalf("实际命中条件字段没有在 iframe 渲染前禁用：%+v", generated.FieldRules)
	}

	invalid := cloneWorkspaceValues(generated.Values)
	invalid["periodValue"] = []any{dateRange[0], dateRange[0]}
	_, err = serviceUnderTest.SaveForm(context.Background(), 7, 32, "123e4567-e89b-12d3-a456-426614174891", model.PathFormSaveInput{Revision: 0, Values: invalid, Seed: generated.Seed, Validated: true})
	if !service.IsPathConfigErrorKind(err, service.PathConfigErrorInvalid) {
		t.Fatalf("不匹配的日期区间没有被保存校验拒绝：%v", err)
	}
	if _, err = serviceUnderTest.SaveForm(context.Background(), 7, 32, "123e4567-e89b-12d3-a456-426614174892", model.PathFormSaveInput{Revision: 0, Values: generated.Values, Seed: generated.Seed, Validated: true}); err != nil {
		t.Fatalf("正确命中的日期区间无法保存：%v", err)
	}
}

// TestPathConfigWorkspaceRejectsEarlierOrderedBranch 验证生成器不会把满足较早策略的数据伪装成后续分支数据。
func TestPathConfigWorkspaceRejectsEarlierOrderedBranch(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration, Account: "account-a", FlowSource: "new", TargetObjectID: "template-a"}}
	snapshot := pathConfigWorkspaceSnapshot()
	laterApproval := snapshot.Tree.Child.ConditionNodes[0].Child
	fallbackApproval := snapshot.Tree.Child.ConditionNodes[1].Child
	snapshot.Tree.Child.ConditionNodes = []target.FlowBranchTemplate{
		{ID: "first", Name: "两天以上", Sort: 1, Conditions: []target.FlowCondition{{FieldA: "amount", Judge: "gte", ValueB: "2"}}, Child: &target.FlowNodeTemplate{ID: "approve-first", Name: "优先审批", Type: "common"}},
		{ID: "later", Name: "十五天以上", Sort: 2, Conditions: []target.FlowCondition{{FieldA: "amount", Judge: "gte", ValueB: "15"}}, Child: laterApproval},
		{ID: "fallback", Name: "其他", Sort: 3, Child: fallbackApproval},
	}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "later"}}}}}
	_, err := newPathConfigService(t, plans, &pathConfigReader{snapshot: snapshot}, paths, &memoryPathConfigRepository{}).GenerateForm(context.Background(), 7, 32, 5, nil, nil, false)
	if !service.IsPathConfigErrorKind(err, service.PathConfigErrorInvalid) {
		t.Fatalf("满足排序更靠前策略时错误生成后续分支：%v", err)
	}
}

// cloneWorkspaceValues 深复制测试表单值，避免故意构造的非法值污染成功保存断言。
func cloneWorkspaceValues(values map[string]any) map[string]any {
	result := make(map[string]any, len(values))
	for key, value := range values {
		if rangeValues, ok := value.([]any); ok {
			result[key] = append([]any(nil), rangeValues...)
			continue
		}
		result[key] = value
	}
	return result
}

// workspaceNumber 统一测试中生成器可能返回的整数和 JSON 浮点数。
func workspaceNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case float64:
		return typed, true
	default:
		return 0, false
	}
}
