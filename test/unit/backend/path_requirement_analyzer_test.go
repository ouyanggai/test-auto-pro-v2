package backend_test

import (
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
)

// TestPathRequirementAnalyzerTranslatesConditionsAndFallback 验证条件中文翻译、字段关联和最后分支兜底语义。
func TestPathRequirementAnalyzerTranslatesConditionsAndFallback(t *testing.T) {
	tree := requirementConditionTree()
	graph := requirementGraph(t, tree)
	fields := []target.FormFieldMetadata{{FormID: "form-a", FormName: "采购申请", FieldID: "field-amount", Name: "申请金额", EnglishName: "amount"}}
	analyzerUnderTest := analyzer.NewPathRequirementAnalyzer()
	pathAnalyzer := analyzer.NewExecutionPathAnalyzer()

	path := model.ExecutionPath{SequenceNo: 1, Name: "大额审批", Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}
	analysis, err := pathAnalyzer.Analyze(graph, path.Choices)
	if err != nil {
		t.Fatalf("准备条件路径失败：%v", err)
	}
	result, err := analyzerUnderTest.Analyze(graph, tree, fields, path, analysis)
	if err != nil {
		t.Fatalf("条件要求分析失败：%v", err)
	}
	body := requirementText(result)
	for _, want := range []string{"申请金额 大于等于 10000", "指定人员", "会签，满足 2 人", "无处理人时跳过该节点", "提交", "同意或不同意"} {
		if !strings.Contains(body, want) {
			t.Fatalf("条件或审批要求缺少 %q：%s", want, body)
		}
	}
	for _, forbidden := range []string{"amount", "gte", "assign", "only_read", "field-amount"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("要求结果泄露内部字段或枚举 %q：%s", forbidden, body)
		}
	}

	fallback := model.ExecutionPath{SequenceNo: 2, Name: "兜底路径", Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-last"}}}
	fallbackAnalysis, err := pathAnalyzer.Analyze(graph, fallback.Choices)
	if err != nil {
		t.Fatalf("准备兜底路径失败：%v", err)
	}
	fallbackResult, err := analyzerUnderTest.Analyze(graph, tree, fields, fallback, fallbackAnalysis)
	if err != nil || !strings.Contains(requirementText(fallbackResult), "其他条件均不满足时进入") {
		t.Fatalf("最后分支没有稳定按兜底表达：result=%+v err=%v", fallbackResult, err)
	}
}

// TestPathRequirementAnalyzerProjectsParallelGroupsAndStatuses 验证并行分组、共享后继去重和四类状态汇总。
func TestPathRequirementAnalyzerProjectsParallelGroupsAndStatuses(t *testing.T) {
	tree := requirementParallelTree()
	graph := requirementGraph(t, tree)
	path := model.ExecutionPath{SequenceNo: 3, Name: "并行核对"}
	analysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, nil)
	if err != nil || !analysis.Complete {
		t.Fatalf("准备并行路径失败：analysis=%+v err=%v", analysis, err)
	}
	result, err := analyzer.NewPathRequirementAnalyzer().Analyze(graph, tree, nil, path, analysis)
	if err != nil {
		t.Fatalf("并行要求分析失败：%v", err)
	}
	if len(result.Groups) != 3 || result.Groups[0].Title != "主线" || !strings.HasPrefix(result.Groups[1].Title, "并行分支：") {
		t.Fatalf("主线与并行分组不正确：%+v", result.Groups)
	}
	endCount := 0
	for _, group := range result.Groups {
		for _, node := range group.Nodes {
			if node.Name == "结束" {
				endCount++
			}
		}
	}
	if endCount != 1 {
		t.Fatalf("并行汇合后的公共节点重复显示：%d", endCount)
	}
	if len(result.Summary) != 4 {
		t.Fatalf("四类状态汇总不稳定：%+v", result.Summary)
	}
	for _, status := range []model.RequirementStatus{model.RequirementPending, model.RequirementAutomatic, model.RequirementRuntime, model.RequirementReview} {
		if !hasRequirementStatus(result.Summary, status) {
			t.Fatalf("状态汇总缺少 %s：%+v", status, result.Summary)
		}
	}
}

// TestPathRequirementAnalyzerDegradesUnknownMetadata 验证未知人员、字段和比较规则只降级单项而不泄露原代码。
func TestPathRequirementAnalyzerDegradesUnknownMetadata(t *testing.T) {
	tree := requirementConditionTree()
	tree.Child.ConditionNodes[0].Conditions[0].Judge = "secret_judge"
	tree.Child.ConditionNodes[0].Child.AuditConfig.AuditType = "secret_auditor"
	graph := requirementGraph(t, tree)
	path := model.ExecutionPath{SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}
	analysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, path.Choices)
	if err != nil {
		t.Fatalf("准备未知元数据路径失败：%v", err)
	}
	result, err := analyzer.NewPathRequirementAnalyzer().Analyze(graph, tree, nil, path, analysis)
	if err != nil {
		t.Fatalf("未知元数据不应让整页失败：%v", err)
	}
	body := requirementText(result)
	if !strings.Contains(body, "需要人工核对") || strings.Contains(body, "secret_judge") || strings.Contains(body, "secret_auditor") {
		t.Fatalf("未知元数据降级或安全边界不正确：%s", body)
	}
}

// TestPathRequirementAnalyzerReviewsMissingAutomaticRange 验证指定类人员规则缺少范围时不会误报自动确定。
func TestPathRequirementAnalyzerReviewsMissingAutomaticRange(t *testing.T) {
	tree := requirementConditionTree()
	tree.Child.ConditionNodes[0].Child.AuditConfig.Details = nil
	graph := requirementGraph(t, tree)
	path := model.ExecutionPath{SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}
	analysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, path.Choices)
	if err != nil {
		t.Fatalf("准备缺失范围路径失败：%v", err)
	}
	result, err := analyzer.NewPathRequirementAnalyzer().Analyze(graph, tree, nil, path, analysis)
	if err != nil {
		t.Fatalf("缺失人员范围不应让整页失败：%v", err)
	}
	if body := requirementText(result); !strings.Contains(body, "处理范围缺失") || !strings.Contains(body, "需要人工核对") {
		t.Fatalf("缺失范围没有降级人工核对：%s", body)
	}
}

// requirementGraph 使用现有唯一流程图分析器生成要求测试图。
func requirementGraph(t *testing.T, tree *target.FlowNodeTemplate) model.FlowGraph {
	t.Helper()
	nodes, edges, warnings, err := analyzer.NewFlowGraphAnalyzer().Analyze(tree)
	if err != nil {
		t.Fatalf("生成要求测试图失败：%v", err)
	}
	return model.FlowGraph{EntryNodeIDs: []string{tree.ID}, Nodes: nodes, Edges: edges, Warnings: warnings}
}

// requirementConditionTree 构造含条件、审批配置、字段权限和共享后继的真实树形样本。
func requirementConditionTree() *target.FlowNodeTemplate {
	end := &target.FlowNodeTemplate{ID: "end", Name: "结束", Type: "end"}
	approval := func(id, name string) *target.FlowNodeTemplate {
		skip := true
		count := 2
		return &target.FlowNodeTemplate{
			ID: id, Name: name, Type: "common", IsSkip: &skip,
			AuditConfig: &target.FlowNodeAuditConfig{
				AuditType: "assign", Mode: "countersign", CountersignNum: &count,
				Details: []target.FlowAuditDetail{{Name: "审批人甲", Type: "personnel"}},
			},
			FieldPowers: []target.FlowNodeFieldPower{{FormID: "form-a", FieldID: "field-amount", EnglishName: "amount", Power: "only_read"}},
		}
	}
	route := &target.FlowNodeTemplate{
		ID: "route", Name: "金额条件", Type: "condition", Child: end,
		ConditionNodes: []target.FlowBranchTemplate{
			{ID: "branch-a", Name: "大额", Sort: 1, Conditions: []target.FlowCondition{{FieldA: "amount", ValueB: "10000", Judge: "gte"}}, Child: approval("approve-a", "财务审批")},
			{ID: "branch-b", Name: "普通", Sort: 2, Conditions: []target.FlowCondition{{FieldA: "amount", ValueB: "10000", Judge: "lt"}}, Child: approval("approve-b", "部门审批")},
			{ID: "branch-last", Name: "其他", Sort: 3, Child: approval("approve-last", "兜底审批")},
		},
	}
	return &target.FlowNodeTemplate{ID: "start", Name: "发起", Type: "start", Child: route}
}

// requirementParallelTree 构造两支并行且共享结束节点的样本。
func requirementParallelTree() *target.FlowNodeTemplate {
	end := &target.FlowNodeTemplate{ID: "end", Name: "结束", Type: "end"}
	parallel := &target.FlowNodeTemplate{
		ID: "parallel", Name: "并行处理", Type: "parallel", Child: end,
		ParallelNodes: []target.FlowBranchTemplate{
			{ID: "parallel-a", Name: "财务线", Sort: 1, Child: &target.FlowNodeTemplate{ID: "finance", Name: "财务协同", Type: "synergy", AuditConfig: &target.FlowNodeAuditConfig{AuditType: "run_node_choose", Mode: "scramble"}}},
			{ID: "parallel-b", Name: "业务线", Sort: 2, Child: &target.FlowNodeTemplate{ID: "business", Name: "业务审批", Type: "common", AuditConfig: &target.FlowNodeAuditConfig{AuditType: "initiator", Mode: "scramble"}}},
		},
	}
	return &target.FlowNodeTemplate{ID: "start", Name: "发起", Type: "start", Child: parallel}
}

// requirementText 把公开要求扁平为测试文本。
func requirementText(result model.PathRequirements) string {
	var parts []string
	for _, count := range result.Summary {
		parts = append(parts, string(count.Status))
	}
	for _, group := range result.Groups {
		parts = append(parts, group.Title)
		for _, node := range group.Nodes {
			parts = append(parts, node.Name, node.TypeName)
			for _, item := range node.Items {
				parts = append(parts, item.Category, item.Title, item.Detail, string(item.Status))
			}
		}
	}
	return strings.Join(parts, "|")
}

// hasRequirementStatus 判断汇总是否包含固定中文状态。
func hasRequirementStatus(counts []model.RequirementCount, status model.RequirementStatus) bool {
	for _, count := range counts {
		if count.Status == status {
			return true
		}
	}
	return false
}
