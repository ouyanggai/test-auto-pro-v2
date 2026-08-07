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

// TestPathRequirementAnalyzerMapsAllApprovedAuditTypes 验证功能文档列出的人员规则全部落入证据化中文状态。
func TestPathRequirementAnalyzerMapsAllApprovedAuditTypes(t *testing.T) {
	tests := []struct {
		auditType   string
		title       string
		status      model.RequirementStatus
		rangeNeeded bool
	}{
		{auditType: "assign", title: "指定人员", status: model.RequirementAutomatic, rangeNeeded: true},
		{auditType: "company", title: "项目指定人员", status: model.RequirementAutomatic, rangeNeeded: true},
		{auditType: "company_id", title: "指定公司", status: model.RequirementAutomatic, rangeNeeded: true},
		{auditType: "department", title: "指定部门", status: model.RequirementAutomatic, rangeNeeded: true},
		{auditType: "position", title: "指定岗位", status: model.RequirementAutomatic, rangeNeeded: true},
		{auditType: "role", title: "选择角色", status: model.RequirementAutomatic, rangeNeeded: true},
		{auditType: "initiator", title: "发起人自己", status: model.RequirementRuntime},
		{auditType: "department_supervisor", title: "发起人部门主管", status: model.RequirementRuntime},
		{auditType: "branched_passage_manager", title: "发起人分管副总", status: model.RequirementRuntime},
		{auditType: "level", title: "指定岗级", status: model.RequirementRuntime},
		{auditType: "extendedAttribute", title: "扩展属性", status: model.RequirementRuntime},
		{auditType: "run_node_choose", title: "审批人自选", status: model.RequirementPending},
		{auditType: "form_person", title: "指定表单人员", status: model.RequirementPending},
	}
	for _, test := range tests {
		t.Run(test.auditType, func(t *testing.T) {
			config := &target.FlowNodeAuditConfig{AuditType: test.auditType, Mode: "scramble"}
			if test.rangeNeeded {
				config.Details = []target.FlowAuditDetail{{Name: "已配置范围"}}
			}
			node := &target.FlowNodeTemplate{ID: "approval", Name: "审批", Type: "common", AuditConfig: config}
			fields := []target.FormFieldMetadata(nil)
			if test.auditType == "form_person" {
				config.FormPersonField = "owner"
				node.FieldPowers = []target.FlowNodeFieldPower{{FormID: "form", EnglishName: "owner"}}
				fields = []target.FormFieldMetadata{{FormID: "form", Name: "经办人", EnglishName: "owner"}}
			}
			tree := &target.FlowNodeTemplate{ID: "start", Name: "发起", Type: "start", Child: node}
			graph := requirementGraph(t, tree)
			path := model.ExecutionPath{SequenceNo: 1}
			analysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, nil)
			if err != nil {
				t.Fatalf("准备人员规则路径失败：%v", err)
			}
			result, err := analyzer.NewPathRequirementAnalyzer().Analyze(graph, tree, fields, path, analysis)
			if err != nil {
				t.Fatalf("人员规则分析失败：%v", err)
			}
			if !hasRequirementItem(result, test.title, test.status) {
				t.Fatalf("人员规则映射不正确：type=%s result=%s", test.auditType, requirementText(result))
			}
		})
	}
}

// TestPathRequirementAnalyzerPreservesAndOrFieldComparisonAndManualBranch 验证字段对字段、多条件连接和手动分支语义。
func TestPathRequirementAnalyzerPreservesAndOrFieldComparisonAndManualBranch(t *testing.T) {
	end := &target.FlowNodeTemplate{ID: "end", Name: "结束", Type: "end"}
	manual := &target.FlowNodeTemplate{
		ID: "manual", Name: "人工选择", Type: "condition", BranchExecuteType: "custom_choose",
		ConditionNodes: []target.FlowBranchTemplate{
			{ID: "manual-a", Name: "加签", Sort: 1, Child: end},
			{ID: "manual-b", Name: "直接通过", Sort: 2, Child: &target.FlowNodeTemplate{ID: "end-b", Name: "另一结束", Type: "end"}},
		},
	}
	route := &target.FlowNodeTemplate{
		ID: "route", Name: "组合条件", Type: "condition", Child: manual,
		ConditionNodes: []target.FlowBranchTemplate{
			{ID: "branch-a", Name: "预算内", Sort: 1, Conditions: []target.FlowCondition{
				{FieldA: "amount", FieldB: "budget", Judge: "lte", ConditionType: "and"},
				{FieldA: "status", ValueB: "已确认", Judge: "eq"},
			}, Child: &target.FlowNodeTemplate{ID: "empty", Name: "空节点", Type: "empty"}},
			{ID: "branch-last", Name: "其他", Sort: 2, Child: &target.FlowNodeTemplate{ID: "fallback", Name: "兜底", Type: "empty"}},
		},
	}
	tree := &target.FlowNodeTemplate{ID: "start", Name: "发起", Type: "start", Child: route}
	fields := []target.FormFieldMetadata{
		{Name: "申请金额", EnglishName: "amount"}, {Name: "可用预算", EnglishName: "budget"}, {Name: "确认状态", EnglishName: "status"},
	}
	graph := requirementGraph(t, tree)
	path := model.ExecutionPath{SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}, {RouteNodeID: "manual", BranchID: "manual-a"}}}
	analysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, path.Choices)
	if err != nil {
		t.Fatalf("准备组合条件路径失败：%v", err)
	}
	result, err := analyzer.NewPathRequirementAnalyzer().Analyze(graph, tree, fields, path, analysis)
	if err != nil {
		t.Fatalf("组合条件与手动分支分析失败：%v", err)
	}
	body := requirementText(result)
	for _, want := range []string{"申请金额 小于等于 可用预算 并且 确认状态 等于 已确认", "运行时选择该分支：加签"} {
		if !strings.Contains(body, want) {
			t.Fatalf("组合条件或手动分支语义缺少 %q：%s", want, body)
		}
	}
	for _, group := range result.Groups {
		for _, node := range group.Nodes {
			if (node.Name == "空节点" || node.Name == "结束") && len(node.Items) != 0 {
				t.Fatalf("空节点或结束节点错误产生动作要求：%+v", node)
			}
		}
	}
}

// TestPathRequirementAnalyzerDisambiguatesDuplicateFieldKeysByNodeForm 验证节点字段权限能唯一定位表单时公开“表单 / 字段”。
func TestPathRequirementAnalyzerDisambiguatesDuplicateFieldKeysByNodeForm(t *testing.T) {
	tree := requirementConditionTree()
	tree.Child.FieldPowers = []target.FlowNodeFieldPower{{FormID: "form-budget", EnglishName: "amount", Power: "only_read"}}
	fields := []target.FormFieldMetadata{
		{FormID: "form-request", FormName: "申请表", FieldID: "field-request", Name: "金额", EnglishName: "amount"},
		{FormID: "form-budget", FormName: "预算表", FieldID: "field-budget", Name: "金额", EnglishName: "amount"},
	}
	graph := requirementGraph(t, tree)
	path := model.ExecutionPath{SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}
	analysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, path.Choices)
	if err != nil {
		t.Fatalf("准备跨表单同键路径失败：%v", err)
	}
	result, err := analyzer.NewPathRequirementAnalyzer().Analyze(graph, tree, fields, path, analysis)
	if err != nil {
		t.Fatalf("跨表单同键消歧失败：%v", err)
	}
	body := requirementText(result)
	if !strings.Contains(body, "预算表 / 金额 大于等于 10000") {
		t.Fatalf("节点表单提示没有输出表单名与字段名：%s", body)
	}
	for _, forbidden := range []string{"amount", "form-budget", "field-budget"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("跨表单消歧泄露内部关联键 %q：%s", forbidden, body)
		}
	}
}

// TestPathRequirementAnalyzerReviewsAmbiguousDuplicateFieldKeys 验证跨表单同键无法唯一定位时保持人工核对。
func TestPathRequirementAnalyzerReviewsAmbiguousDuplicateFieldKeys(t *testing.T) {
	tree := requirementConditionTree()
	fields := []target.FormFieldMetadata{
		{FormID: "form-request", FormName: "申请表", Name: "金额", EnglishName: "amount"},
		{FormID: "form-budget", FormName: "预算表", Name: "金额", EnglishName: "amount"},
	}
	graph := requirementGraph(t, tree)
	path := model.ExecutionPath{SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}
	analysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, path.Choices)
	if err != nil {
		t.Fatalf("准备歧义字段路径失败：%v", err)
	}
	result, err := analyzer.NewPathRequirementAnalyzer().Analyze(graph, tree, fields, path, analysis)
	if err != nil {
		t.Fatalf("歧义字段不应让整页失败：%v", err)
	}
	item, found := findRequirementItem(result, "大额")
	if !found || item.Status != model.RequirementReview || !strings.Contains(item.Detail, "未识别的表单字段") {
		t.Fatalf("歧义字段没有稳定降级人工核对：item=%+v result=%s", item, requirementText(result))
	}
	for _, forbidden := range []string{"amount", "form-request", "form-budget"} {
		if strings.Contains(item.Detail, forbidden) {
			t.Fatalf("歧义字段降级泄露内部关联键 %q：%s", forbidden, item.Detail)
		}
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

// hasRequirementItem 判断结果中是否存在指定标题和状态的要求项。
func hasRequirementItem(result model.PathRequirements, title string, status model.RequirementStatus) bool {
	for _, group := range result.Groups {
		for _, node := range group.Nodes {
			for _, item := range node.Items {
				if item.Title == title && item.Status == status {
					return true
				}
			}
		}
	}
	return false
}

// findRequirementItem 按公开标题查找单条要求，供精确断言状态与文案。
func findRequirementItem(result model.PathRequirements, title string) (model.RequirementItem, bool) {
	for _, group := range result.Groups {
		for _, node := range group.Nodes {
			for _, item := range node.Items {
				if item.Title == title {
					return item, true
				}
			}
		}
	}
	return model.RequirementItem{}, false
}
