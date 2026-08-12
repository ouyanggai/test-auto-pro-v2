package backend_test

import (
	"encoding/json"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
)

// TestPathConfigAnalyzerProjectsFieldsActionsAndValuesInPathOrder 验证节点配置只投影人员与动作，不携带表单字段或组件缺口。
func TestPathConfigAnalyzerProjectsFieldsActionsAndValuesInPathOrder(t *testing.T) {
	tree := pathConfigTree()
	fields := pathConfigFields()
	graph := requirementGraph(t, tree)
	path := model.ExecutionPath{SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}
	analysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, path.Choices)
	if err != nil {
		t.Fatalf("准备配置路径失败：%v", err)
	}
	instanceValues := map[string]any{"amount": 2500.5}
	configuration, validation, err := analyzer.NewPathConfigAnalyzer().Analyze(
		graph, tree, fields, path, analysis, instanceValues, nil, nil,
	)
	if err != nil {
		t.Fatalf("路径配置投影失败：%v", err)
	}
	if configuration.Revision != 0 || len(configuration.Groups) != 1 {
		t.Fatalf("配置分组不正确：%+v", configuration.Groups)
	}
	mainGroup := configuration.Groups[0]
	if mainGroup.Title != "主线" || len(mainGroup.Nodes) < 4 {
		t.Fatalf("主线节点顺序不正确：%s nodes=%d", mainGroup.Title, len(mainGroup.Nodes))
	}
	start := mainGroup.Nodes[0]
	if start.Kind != "start" || len(start.Actions) != 1 || start.Actions[0].Kind != "submit" || start.Actions[0].Current != "submit" {
		t.Fatalf("发起节点动作不正确：%+v", start)
	}
	approval := mainGroup.Nodes[2]
	if approval.Kind != "common" {
		t.Fatalf("审批节点顺序不正确：%+v", approval)
	}
	if len(approval.Actions) != 1 || approval.Actions[0].Kind != "agree_disagree" || approval.Actions[0].Default != "agree" || approval.Actions[0].Current != "agree" {
		t.Fatalf("审批动作默认推荐不正确：%+v", approval.Actions)
	}
	if len(approval.Fields) != 0 || len(approval.Gaps) != 0 || len(validation.FieldTokens) != 0 {
		t.Fatalf("表单字段或缺口不应进入节点配置：fields=%+v gaps=%+v tokens=%+v", approval.Fields, approval.Gaps, validation.FieldTokens)
	}
	for _, requirement := range approval.Requirements {
		if requirement.Title == "字段权限" {
			t.Fatalf("字段权限不应出现在节点模板要求中：%+v", approval.Requirements)
		}
	}
	assertPathConfigPublicSafety(t, configuration)
}

// TestPathConfigAnalyzerOverlaysStoredValuesAndMarksAffected 验证历史字段值不会重新进入节点配置或把节点标记失效。
func TestPathConfigAnalyzerOverlaysStoredValuesAndMarksAffected(t *testing.T) {
	tree := pathConfigTree()
	fields := pathConfigFields()
	graph := requirementGraph(t, tree)
	path := model.ExecutionPath{SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}
	analysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, path.Choices)
	if err != nil {
		t.Fatalf("准备配置路径失败：%v", err)
	}
	storedFields := map[string]map[string]string{"approve-a": {"amount": "888", "type": "\"removed\""}}
	configuration, _, err := analyzer.NewPathConfigAnalyzer().Analyze(
		graph, tree, fields, path, analysis, nil, storedFields, nil,
	)
	if err != nil {
		t.Fatalf("已保存值叠加失败：%v", err)
	}
	approval := findConfigNode(configuration.Groups, "财务审批")
	if len(approval.Fields) != 0 || len(approval.Gaps) != 0 || configuration.Status == "affected" {
		t.Fatalf("历史字段值错误污染节点状态：node=%+v status=%s", approval, configuration.Status)
	}
	assertPathConfigPublicSafety(t, configuration)
}

// TestPathConfigAnalyzerStoredValueWinsOverInstanceValue 验证实例值与已保存表单值都只属于表单工作区。
func TestPathConfigAnalyzerStoredValueWinsOverInstanceValue(t *testing.T) {
	tree := pathConfigTree()
	fields := pathConfigFields()
	graph := requirementGraph(t, tree)
	path := model.ExecutionPath{SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}
	analysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, path.Choices)
	if err != nil {
		t.Fatalf("准备配置路径失败：%v", err)
	}
	instanceValues := map[string]any{"amount": 2500.5, "type": "instance-value"}
	storedFields := map[string]map[string]string{"approve-a": {"amount": "888", "type": "\"a\""}}
	configuration, _, err := analyzer.NewPathConfigAnalyzer().Analyze(
		graph, tree, fields, path, analysis, instanceValues, storedFields, nil,
	)
	if err != nil {
		t.Fatalf("已保存值优先投影失败：%v", err)
	}
	approval := findConfigNode(configuration.Groups, "财务审批")
	if len(approval.Fields) != 0 || len(approval.Gaps) != 0 {
		t.Fatalf("实例值或已保存值错误进入节点配置：%+v", approval)
	}
}

// TestPathConfigAnalyzerMarksAffectedWhenStoredValueInvalidEvenWithInstanceValue 验证失效表单值不会影响节点配置状态。
func TestPathConfigAnalyzerMarksAffectedWhenStoredValueInvalidEvenWithInstanceValue(t *testing.T) {
	tree := pathConfigTree()
	fields := pathConfigFields()
	graph := requirementGraph(t, tree)
	path := model.ExecutionPath{SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}
	analysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, path.Choices)
	if err != nil {
		t.Fatalf("准备配置路径失败：%v", err)
	}
	instanceValues := map[string]any{"amount": 2500.5}
	storedFields := map[string]map[string]string{"approve-a": {"amount": "\"abc\""}}
	configuration, _, err := analyzer.NewPathConfigAnalyzer().Analyze(
		graph, tree, fields, path, analysis, instanceValues, storedFields, nil,
	)
	if err != nil {
		t.Fatalf("失效已保存值投影失败：%v", err)
	}
	approval := findConfigNode(configuration.Groups, "财务审批")
	if len(approval.Fields) != 0 || len(approval.Gaps) != 0 || configuration.Status == "affected" {
		t.Fatalf("失效表单值错误影响节点配置：node=%+v status=%s", approval, configuration.Status)
	}
}

// TestPathConfigAnalyzerUsesInstanceValueWithoutStoredValue 验证未保存实例值不会进入节点配置。
func TestPathConfigAnalyzerUsesInstanceValueWithoutStoredValue(t *testing.T) {
	tree := pathConfigTree()
	fields := pathConfigFields()
	graph := requirementGraph(t, tree)
	path := model.ExecutionPath{SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}
	analysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, path.Choices)
	if err != nil {
		t.Fatalf("准备配置路径失败：%v", err)
	}
	instanceValues := map[string]any{"amount": 2500.5}
	configuration, _, err := analyzer.NewPathConfigAnalyzer().Analyze(
		graph, tree, fields, path, analysis, instanceValues, nil, nil,
	)
	if err != nil {
		t.Fatalf("实例现值投影失败：%v", err)
	}
	approval := findConfigNode(configuration.Groups, "财务审批")
	if len(approval.Fields) != 0 || len(approval.Gaps) != 0 {
		t.Fatalf("实例当前值错误进入节点配置：%+v", approval)
	}
}

// TestPathConfigAnalyzerProjectsOpaqueNodeStatusAndTemplatePersonRules 验证节点映射键、状态和模板受限人员候选按真实节点投影。
func TestPathConfigAnalyzerProjectsOpaqueNodeStatusAndTemplatePersonRules(t *testing.T) {
	tree := pathConfigTree()
	approval := tree.Child.ConditionNodes[0].Child
	approval.AuditConfig = &target.FlowNodeAuditConfig{
		AuditType: "run_node_choose", Mode: "countersign", CountersignNum: intPointer(2),
		Candidates: []target.FlowAuditCandidate{{ID: "person-1", Name: "张三"}, {ID: "person-2", Name: "李四"}},
	}
	graph := requirementGraph(t, tree)
	path := model.ExecutionPath{SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}
	analysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, path.Choices)
	if err != nil {
		t.Fatalf("准备人员配置路径失败：%v", err)
	}
	configuration, validation, err := analyzer.NewPathConfigAnalyzer().Analyze(graph, tree, pathConfigFields(), path, analysis, nil, nil, nil)
	if err != nil {
		t.Fatalf("人员配置投影失败：%v", err)
	}
	approvalConfig := findConfigNode(configuration.Groups, "财务审批")
	if approvalConfig == nil || approvalConfig.Key == "" || approvalConfig.Key == "approve-a" || approvalConfig.Status != "pending" || approvalConfig.StatusName != "待配置" {
		t.Fatalf("节点不透明键或待配置状态不正确：%+v", approvalConfig)
	}
	if len(approvalConfig.Persons) != 1 {
		t.Fatalf("审批节点缺少人员规则：%+v", approvalConfig.Persons)
	}
	person := approvalConfig.Persons[0]
	if !person.Editable || person.Mode != "select" || !person.Multiple || person.MinCount != 2 || len(person.Options) != 2 || person.Options[0].Label != "张三" {
		t.Fatalf("模板受限人员候选不正确：%+v", person)
	}
	if person.Options[0].Value == "person-1" || validation.ActionTokens[person.Key].CandidateTokens[person.Options[0].Value] != "person-1" {
		t.Fatalf("人员候选未使用不透明回写键：person=%+v validation=%+v", person, validation.ActionTokens[person.Key])
	}
	if len(approvalConfig.Requirements) == 0 || configuration.NextNodeKey == "" || configuration.Progress.Pending == 0 {
		t.Fatalf("节点要求或配置进度没有生成：node=%+v progress=%+v next=%q", approvalConfig, configuration.Progress, configuration.NextNodeKey)
	}
	assertPathConfigPublicSafety(t, configuration)
}

// TestPathConfigAnalyzerProjectsStructuredPersonNames 验证人员、岗位、岗级、角色和组织名称结构化公开，无名称对象不进入公开列表。
func TestPathConfigAnalyzerProjectsStructuredPersonNames(t *testing.T) {
	tests := []struct {
		name          string
		auditType     string
		details       []target.FlowAuditDetail
		scopes        []target.FlowAuditScope
		wantCategory  string
		wantName      string
		wantItemCount int
	}{
		{name: "人员", auditType: "assign", details: []target.FlowAuditDetail{{Name: "张三", Type: "personnel"}}, wantCategory: "人员", wantName: "张三", wantItemCount: 1},
		{name: "岗位", auditType: "position", details: []target.FlowAuditDetail{{Name: "主任", Type: "position"}}, wantCategory: "岗位", wantName: "主任", wantItemCount: 1},
		{name: "岗级", auditType: "level", details: []target.FlowAuditDetail{{Name: "二级岗", Type: "level"}}, wantCategory: "岗级", wantName: "二级岗", wantItemCount: 1},
		{name: "角色", auditType: "role", details: []target.FlowAuditDetail{{Name: "财务审批角色"}}, wantCategory: "角色", wantName: "财务审批角色", wantItemCount: 1},
		{name: "部门", auditType: "department", details: []target.FlowAuditDetail{{Name: "财务部", Type: "department"}}, wantCategory: "部门", wantName: "财务部", wantItemCount: 1},
		{name: "无名称对象", auditType: "department", details: []target.FlowAuditDetail{{Type: "department"}}, wantItemCount: 0},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			tree := pathConfigTree()
			tree.Child.ConditionNodes[0].Child.AuditConfig = &target.FlowNodeAuditConfig{
				AuditType: testCase.auditType, Mode: "scramble", Details: testCase.details, Scopes: testCase.scopes,
			}
			graph := requirementGraph(t, tree)
			path := model.ExecutionPath{SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}
			analysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, path.Choices)
			if err != nil {
				t.Fatalf("准备人员名称路径失败：%v", err)
			}
			configuration, _, err := analyzer.NewPathConfigAnalyzer().Analyze(graph, tree, pathConfigFields(), path, analysis, nil, nil, nil)
			if err != nil {
				t.Fatalf("结构化人员名称投影失败：%v", err)
			}
			person := findConfigNode(configuration.Groups, "财务审批").Persons[0]
			for _, requirement := range findConfigNode(configuration.Groups, "财务审批").Requirements {
				if requirement.Category == "人员" {
					t.Fatalf("结构化人员区存在时不应重复输出长文本人员要求：%+v", requirement)
				}
			}
			if testCase.wantItemCount == 0 {
				if len(person.Items) != 0 {
					t.Fatalf("无真实名称的人员对象不应进入公开列表：%+v", person.Items)
				}
			} else if len(person.Items) != 1 || person.Items[0].Category != testCase.wantCategory || person.Items[0].Name != testCase.wantName || person.Items[0].Count != testCase.wantItemCount {
				t.Fatalf("结构化人员名称不正确：%+v", person.Items)
			}
			encoded, _ := json.Marshal(person.Items)
			if strings.Contains(string(encoded), "person-secret") || strings.Contains(string(encoded), "bizId") || strings.Contains(string(encoded), "名称需运行时解析") {
				t.Fatalf("结构化人员名称泄露内部标识：%s", encoded)
			}
			assertPathConfigPublicSafety(t, configuration)
		})
	}
}

// TestPathConfigAnalyzerRevalidatesStoredPersonCounts 验证存量人员选择按当前模板人数重新投影并驱动节点状态。
func TestPathConfigAnalyzerRevalidatesStoredPersonCounts(t *testing.T) {
	tests := []struct {
		name         string
		isSkip       bool
		mode         string
		selectedIDs  []string
		wantAffected bool
	}{
		{name: "可跳过零人合法", isSkip: true, mode: "countersign", selectedIDs: []string{}},
		{name: "可跳过一人低于最低人数", isSkip: true, mode: "countersign", selectedIDs: []string{"person-1"}, wantAffected: true},
		{name: "可跳过两人满足最低人数", isSkip: true, mode: "countersign", selectedIDs: []string{"person-1", "person-2"}},
		{name: "必选零人仍失效", isSkip: false, mode: "countersign", selectedIDs: []string{}, wantAffected: true},
		{name: "单选存量两人超过上限", isSkip: true, mode: "scramble", selectedIDs: []string{"person-1", "person-2"}, wantAffected: true},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			tree := pathConfigTree()
			approval := tree.Child.ConditionNodes[0].Child
			approval.IsSkip = &testCase.isSkip
			approval.AuditConfig = &target.FlowNodeAuditConfig{
				AuditType: "run_node_choose", Mode: testCase.mode, CountersignNum: intPointer(2),
				Candidates: []target.FlowAuditCandidate{{ID: "person-1", Name: "张三"}, {ID: "person-2", Name: "李四"}, {ID: "person-3", Name: "王五"}},
			}
			graph := requirementGraph(t, tree)
			path := model.ExecutionPath{SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}
			analysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, path.Choices)
			if err != nil {
				t.Fatalf("准备存量人员配置路径失败：%v", err)
			}
			encoded, err := json.Marshal(testCase.selectedIDs)
			if err != nil {
				t.Fatalf("存量人员测试数据无法编码：%v", err)
			}
			storedActions := map[string]string{analyzer.PathConfigPersonStorageKey("approve-a"): string(encoded)}
			configuration, _, err := analyzer.NewPathConfigAnalyzer().Analyze(graph, tree, pathConfigFields(), path, analysis, nil, nil, storedActions, true)
			if err != nil {
				t.Fatalf("存量人员配置投影失败：%v", err)
			}
			approvalConfig := findConfigNode(configuration.Groups, "财务审批")
			if approvalConfig == nil || len(approvalConfig.Persons) != 1 {
				t.Fatalf("存量人员配置节点缺失：%+v", approvalConfig)
			}
			person := approvalConfig.Persons[0]
			if person.Affected != testCase.wantAffected {
				t.Fatalf("存量人员影响状态不正确：person=%+v", person)
			}
			if testCase.wantAffected {
				if approvalConfig.Status != "affected" || configuration.Status != "affected" || !strings.Contains(person.Note, "重新确认") {
					t.Fatalf("无效存量人员没有驱动配置失效：configuration=%+v node=%+v person=%+v", configuration, approvalConfig, person)
				}
			} else if approvalConfig.Status != "configured" || configuration.Status != "configured" {
				t.Fatalf("合法存量人员被错误标记：configuration=%+v node=%+v", configuration, approvalConfig)
			}
		})
	}
}

// TestPathConfigAnalyzerKeepsRuntimePersonRulesReadOnly 验证目标未返回合法候选时审批人自选保持运行时只读。
func TestPathConfigAnalyzerKeepsRuntimePersonRulesReadOnly(t *testing.T) {
	tree := pathConfigTree()
	tree.Child.ConditionNodes[0].Child.AuditConfig = &target.FlowNodeAuditConfig{AuditType: "run_node_choose", Mode: "scramble"}
	graph := requirementGraph(t, tree)
	path := model.ExecutionPath{SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}
	analysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, path.Choices)
	if err != nil {
		t.Fatalf("准备运行时人员路径失败：%v", err)
	}
	configuration, validation, err := analyzer.NewPathConfigAnalyzer().Analyze(graph, tree, pathConfigFields(), path, analysis, nil, nil, nil)
	if err != nil {
		t.Fatalf("运行时人员投影失败：%v", err)
	}
	person := findConfigNode(configuration.Groups, "财务审批").Persons[0]
	if person.Editable || person.Mode != "runtime" || len(person.Options) != 0 || !strings.Contains(person.Detail, "真实运行节点") {
		t.Fatalf("运行时人员被伪造成可选候选：%+v", person)
	}
	if len(validation.ActionTokens) != 3 {
		t.Fatalf("运行时人员不应增加人员回写键：%+v", validation.ActionTokens)
	}
}

// TestPathConfigAnalyzerProjectsActionPeopleIndependently 验证固定主处理人不妨碍按当前模板候选投影加签与移交动作人员。
func TestPathConfigAnalyzerProjectsActionPeopleIndependently(t *testing.T) {
	tree := pathConfigTree()
	approval := tree.Child.ConditionNodes[0].Child
	approval.AuditConfig = &target.FlowNodeAuditConfig{
		AuditType: "assign", Mode: "scramble",
		Details:    []target.FlowAuditDetail{{ID: "leader", Name: "部门负责人", Type: "personnel"}},
		Candidates: []target.FlowAuditCandidate{{ID: "person-1", Name: "张三"}, {ID: "person-2", Name: "李四"}},
	}
	graph := requirementGraph(t, tree)
	path := model.ExecutionPath{Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}
	analysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, path.Choices)
	if err != nil {
		t.Fatalf("准备固定人员动作路径失败：%v", err)
	}
	configuration, validation, err := analyzer.NewPathConfigAnalyzer().Analyze(graph, tree, pathConfigFields(), path, analysis, nil, nil, nil)
	if err != nil {
		t.Fatalf("固定人员动作投影失败：%v", err)
	}
	node := findConfigNode(configuration.Groups, "财务审批")
	if node == nil || node.Persons[0].Editable {
		t.Fatalf("主处理人应保持固定只读：%+v", node)
	}
	for _, kind := range []string{"add_sign", "transfer_approver"} {
		item := findActionCatalogItem(t, node.ActionPlan.Catalog, kind)
		if !item.RequiresPerson || item.Person == nil || len(item.Person.Options) != 2 {
			t.Fatalf("%s 没有独立受限人员范围：%+v", kind, item)
		}
		if !item.Person.Multiple || item.Person.MinCount != 1 || item.Person.MaxCount != len(item.Person.Options) {
			t.Fatalf("%s 必须允许当前受限候选范围内的一人或多人选择：%+v", kind, item.Person)
		}
		if validation.NodeTokens[node.Key].ActionPersons[kind] == nil {
			t.Fatalf("%s 没有进入服务端动作人员校验映射", kind)
		}
	}
}

// TestPathConfigAnalyzerDistinguishesDateAndDateTimeControls 验证日期与日期时间组件不再投影为节点控件。
func TestPathConfigAnalyzerDistinguishesDateAndDateTimeControls(t *testing.T) {
	tree := pathConfigTree()
	approval := tree.Child.ConditionNodes[0].Child
	approval.FieldPowers = append(approval.FieldPowers,
		target.FlowNodeFieldPower{FormID: "form-a", FieldID: "field-date", EnglishName: "date", Power: "edit"},
		target.FlowNodeFieldPower{FormID: "form-a", FieldID: "field-datetime", EnglishName: "datetime", Power: "edit"},
	)
	fields := append(pathConfigFields(),
		target.FormFieldDetail{FormID: "form-a", FieldID: "field-date", Name: "日期", EnglishName: "date", FieldType: "dateType", ComponentType: "date", DateMode: "date"},
		target.FormFieldDetail{FormID: "form-a", FieldID: "field-datetime", Name: "日期时间", EnglishName: "datetime", FieldType: "dateType", ComponentType: "date", DateMode: "datetime"},
	)
	graph := requirementGraph(t, tree)
	analysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}})
	if err != nil {
		t.Fatalf("准备日期路径失败：%v", err)
	}
	configuration, _, err := analyzer.NewPathConfigAnalyzer().Analyze(graph, tree, fields, model.ExecutionPath{SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}, analysis, nil, nil, nil)
	if err != nil {
		t.Fatalf("日期控件投影失败：%v", err)
	}
	approvalConfig := findConfigNode(configuration.Groups, "财务审批")
	if len(approvalConfig.Fields) != 0 || len(approvalConfig.Gaps) != 0 {
		t.Fatalf("日期组件错误进入节点配置：%+v", approvalConfig)
	}
}

// TestPathConfigAnalyzerBlocksLineAfterDisagree 验证不同意动作之后的节点不再按原路径继续。
func TestPathConfigAnalyzerBlocksLineAfterDisagree(t *testing.T) {
	tree := pathConfigTree()
	graph := requirementGraph(t, tree)
	path := model.ExecutionPath{SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}
	analysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, path.Choices)
	if err != nil {
		t.Fatalf("准备配置路径失败：%v", err)
	}
	configuration, _, err := analyzer.NewPathConfigAnalyzer().Analyze(
		graph, tree, pathConfigFields(), path, analysis, nil, nil, map[string]string{"approve-a": "disagree"},
	)
	if err != nil {
		t.Fatalf("不同意动作投影失败：%v", err)
	}
	approval := findConfigNode(configuration.Groups, "财务审批")
	if approval == nil || approval.Actions[0].Current != "disagree" {
		t.Fatalf("不同意动作没有保存展示：%+v", approval)
	}
	next := findConfigNode(configuration.Groups, "部门审批")
	if next == nil || !next.LineBlocked {
		t.Fatalf("不同意动作之后的节点没有标记不再继续：%+v", next)
	}
}

// TestPathConfigAnalyzerMapsUnsupportedFieldsToGaps 验证未知组件和字段权限不会成为节点缺口。
func TestPathConfigAnalyzerMapsUnsupportedFieldsToGaps(t *testing.T) {
	tree := &target.FlowNodeTemplate{
		ID: "start", Name: "发起", Type: "start",
		FieldPowers: []target.FlowNodeFieldPower{
			{FormID: "form-a", FieldID: "field-sub", EnglishName: "sub", Power: "edit"},
			{FormID: "form-a", FieldID: "field-file", EnglishName: "file", Power: "edit"},
			{FormID: "form-a", FieldID: "field-list", EnglishName: "list", Power: "edit"},
			{FormID: "form-a", FieldID: "field-hide", EnglishName: "hidden", Power: "hide"},
			{FormID: "form-a", FieldID: "field-blank", EnglishName: "blank", Power: ""},
		},
		Child: &target.FlowNodeTemplate{ID: "end", Name: "结束", Type: "end"},
	}
	fields := []target.FormFieldDetail{
		{FormID: "form-a", FieldID: "field-sub", Name: "明细", EnglishName: "sub", FieldType: "stringType", ComponentType: "subform"},
		{FormID: "form-a", FieldID: "field-file", Name: "附件", EnglishName: "file", FieldType: "stringType", ComponentType: "fileupload"},
		{FormID: "form-a", FieldID: "field-list", Name: "多选", EnglishName: "list", FieldType: "listType"},
	}
	graph := requirementGraph(t, tree)
	analysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, nil)
	if err != nil {
		t.Fatalf("准备无分支路径失败：%v", err)
	}
	configuration, validation, err := analyzer.NewPathConfigAnalyzer().Analyze(graph, tree, fields, model.ExecutionPath{SequenceNo: 1}, analysis, nil, nil, nil)
	if err != nil {
		t.Fatalf("缺口投影失败：%v", err)
	}
	start := configuration.Groups[0].Nodes[0]
	if len(start.Fields) != 0 || len(start.Gaps) != 0 || len(validation.FieldTokens) != 0 || len(validation.Blockers) != 0 {
		t.Fatalf("表单组件错误进入节点字段、缺口或保存 blocker：node=%+v validation=%+v", start, validation)
	}
	assertPathConfigPublicSafety(t, configuration)
}

// TestPathConfigAnalyzerProjectsPersonStrategiesAndDeterministicRandom 验证目标默认、手动、确定性随机和全选均受当前候选与人数边界约束。
func TestPathConfigAnalyzerProjectsPersonStrategiesAndDeterministicRandom(t *testing.T) {
	tree := pathConfigTree()
	approval := tree.Child.ConditionNodes[0].Child
	approval.AuditConfig = &target.FlowNodeAuditConfig{
		AuditType: "run_node_choose", Mode: "countersign", CountersignNum: intPointer(-1),
		Candidates:        []target.FlowAuditCandidate{{ID: "person-1", Name: "张三"}, {ID: "person-2", Name: "李四"}, {ID: "person-3", Name: "王五"}},
		DefaultCandidates: []target.FlowAuditCandidate{{ID: "person-1", Name: "张三"}, {ID: "person-2", Name: "李四"}, {ID: "person-3", Name: "王五"}},
	}
	graph := requirementGraph(t, tree)
	path := model.ExecutionPath{Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}
	pathAnalysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, path.Choices)
	if err != nil {
		t.Fatalf("准备人员策略路径失败：%v", err)
	}
	configuration, validation, err := analyzer.NewPathConfigAnalyzer().Analyze(graph, tree, pathConfigFields(), path, pathAnalysis, nil, nil, nil)
	if err != nil {
		t.Fatalf("人员策略投影失败：%v", err)
	}
	person := findConfigNode(configuration.Groups, "财务审批").Persons[0]
	if person.Strategy != "target_default" || len(person.Selected) != 3 || len(person.Strategies) != 4 {
		t.Fatalf("目标默认或策略目录不正确：%+v", person)
	}
	if person.StrategySeed < 1 || person.StrategySeed > 9007199254740991 {
		t.Fatalf("默认稳定 seed 超出 JavaScript 安全整数范围：%d", person.StrategySeed)
	}
	legacySeedPlan := `{"strategy":"random","seed":9007199254740992,"selected":["person-2","person-3","person-1"]}`
	legacyConfiguration, _, err := analyzer.NewPathConfigAnalyzer().Analyze(
		graph, tree, pathConfigFields(), path, pathAnalysis, nil, nil,
		map[string]string{analyzer.PathConfigPersonPlanStorageKey("approve-a"): legacySeedPlan}, true,
	)
	if err != nil {
		t.Fatalf("不安全存量 seed 投影失败：%v", err)
	}
	legacyPerson := findConfigNode(legacyConfiguration.Groups, "财务审批").Persons[0]
	if legacyPerson.StrategySeed != 1 || !legacyPerson.Affected || !strings.Contains(legacyPerson.Note, "随机种子") {
		t.Fatalf("不安全存量 seed 未收敛并标记重确认：%+v", legacyPerson)
	}
	targetPlan := validation.NodeTokens[findConfigNode(configuration.Groups, "财务审批").Key].Person
	if targetPlan == nil {
		t.Fatal("人员策略没有进入节点校验索引")
	}
	manual := model.PathConfigPersonStrategyInput{Key: person.Key, Strategy: "manual", Seed: 7, Selected: []string{person.Options[0].Value, person.Options[1].Value, person.Options[2].Value}}
	if _, reason := analyzer.EncodePathConfigPersonStrategy(*targetPlan, manual); reason != "" {
		t.Fatalf("合法手动策略被拒绝：%s", reason)
	}
	random := model.PathConfigPersonStrategyInput{Key: person.Key, Strategy: "random", Seed: 9, Selected: []string{person.Options[0].Value}}
	first, reason := analyzer.EncodePathConfigPersonStrategy(*targetPlan, random)
	if reason != "" {
		t.Fatalf("合法随机策略被拒绝：%s", reason)
	}
	second, reason := analyzer.EncodePathConfigPersonStrategy(*targetPlan, random)
	if reason != "" || first != second {
		t.Fatalf("相同 seed 的随机策略不可复现：first=%s second=%s reason=%s", first, second, reason)
	}
	all := model.PathConfigPersonStrategyInput{Key: person.Key, Strategy: "all", Seed: 1, Selected: []string{person.Options[0].Value}}
	if encoded, reason := analyzer.EncodePathConfigPersonStrategy(*targetPlan, all); reason != "" || !strings.Contains(encoded, "person-3") {
		t.Fatalf("全选策略没有覆盖当前受限候选：encoded=%s reason=%s", encoded, reason)
	}
	manual.Selected = append(manual.Selected, "forged-token")
	if _, reason := analyzer.EncodePathConfigPersonStrategy(*targetPlan, manual); !strings.Contains(reason, "不属于") {
		t.Fatalf("越界人员候选没有被拒绝：%s", reason)
	}
	nodeTarget := validation.NodeTokens[findConfigNode(configuration.Groups, "财务审批").Key]
	actionPerson := findActionCatalogItem(t, findConfigNode(configuration.Groups, "财务审批").ActionPlan.Catalog, "add_sign").Person
	if actionPerson == nil || len(actionPerson.Options) != 3 {
		t.Fatalf("动作人员范围没有独立投影当前目标候选：%+v", actionPerson)
	}
	transferPerson := findActionCatalogItem(t, findConfigNode(configuration.Groups, "财务审批").ActionPlan.Catalog, "transfer_approver").Person
	if transferPerson == nil || len(transferPerson.Options) != 3 || !transferPerson.Multiple || transferPerson.MinCount != 1 || transferPerson.MaxCount != 3 {
		t.Fatalf("移交人员范围没有独立投影多人候选：%+v", transferPerson)
	}
	if !containsPersonStrategy(transferPerson.Strategies, "all") {
		t.Fatalf("移交多人候选缺少全选策略：%+v", transferPerson.Strategies)
	}
	actionManual := model.PathConfigPersonStrategyInput{Key: actionPerson.Key, Strategy: "manual", Seed: 7, Selected: []string{actionPerson.Options[0].Value, actionPerson.Options[1].Value, actionPerson.Options[2].Value}}
	transferManual := model.PathConfigPersonStrategyInput{Key: transferPerson.Key, Strategy: "manual", Seed: 9, Selected: []string{transferPerson.Options[0].Value, transferPerson.Options[1].Value}}
	actions := []model.PathConfigArrivalInput{{Visit: 1, Steps: []model.PathConfigActionStepInput{
		{Kind: "add_sign", Person: &actionManual},
		{Kind: "transfer_approver", Person: &transferManual},
	}}}
	if _, count, reason := analyzer.EncodePathConfigActionPlan(nodeTarget, actions); reason != "" || count != 2 {
		t.Fatalf("加签或移交没有复用当前合法人员策略：count=%d reason=%s", count, reason)
	}
	transferAll := model.PathConfigPersonStrategyInput{Key: transferPerson.Key, Strategy: "all", Seed: 9, Selected: []string{transferPerson.Options[0].Value}}
	if _, _, reason := analyzer.EncodePathConfigActionPlan(nodeTarget, []model.PathConfigArrivalInput{{Visit: 1, Steps: []model.PathConfigActionStepInput{{Kind: "transfer_approver", Person: &transferAll}}}}); reason != "" {
		t.Fatalf("移交全选策略被错误拒绝：%s", reason)
	}
	transferEmpty := model.PathConfigPersonStrategyInput{Key: transferPerson.Key, Strategy: "manual", Seed: 9, Selected: []string{}}
	if _, _, reason := analyzer.EncodePathConfigActionPlan(nodeTarget, []model.PathConfigArrivalInput{{Visit: 1, Steps: []model.PathConfigActionStepInput{{Kind: "transfer_approver", Person: &transferEmpty}}}}); !strings.Contains(reason, "人数不足") {
		t.Fatalf("移交空选没有被拒绝：%s", reason)
	}
	transferForged := model.PathConfigPersonStrategyInput{Key: transferPerson.Key, Strategy: "manual", Seed: 9, Selected: []string{transferPerson.Options[0].Value, "forged-token"}}
	if _, _, reason := analyzer.EncodePathConfigActionPlan(nodeTarget, []model.PathConfigArrivalInput{{Visit: 1, Steps: []model.PathConfigActionStepInput{{Kind: "transfer_approver", Person: &transferForged}}}}); !strings.Contains(reason, "不属于") {
		t.Fatalf("移交伪造候选没有被拒绝：%s", reason)
	}
	transferThenApprove := []model.PathConfigArrivalInput{{Visit: 1, Steps: []model.PathConfigActionStepInput{
		{Kind: "transfer_approver", Person: &transferManual},
		{Kind: "approve_pass", Opinion: "同意"},
	}}}
	if _, _, reason := analyzer.EncodePathConfigActionPlan(nodeTarget, transferThenApprove); !strings.Contains(reason, "最后一步") {
		t.Fatalf("移交后的同次到达动作没有被拒绝：%s", reason)
	}
}

// containsPersonStrategy 判断公开动作人员目录是否包含预期策略，避免测试依赖目录显示顺序。
func containsPersonStrategy(items []model.PathConfigPersonStrategyOption, expected string) bool {
	for _, item := range items {
		if item.Value == expected {
			return true
		}
	}
	return false
}

// findActionCatalogItem 按稳定动作枚举查找测试目录项，缺失时立即失败。
func findActionCatalogItem(t *testing.T, items []model.PathConfigActionCatalogItem, kind string) model.PathConfigActionCatalogItem {
	t.Helper()
	for _, item := range items {
		if item.Kind == kind {
			return item
		}
	}
	t.Fatalf("缺少动作目录：%s", kind)
	return model.PathConfigActionCatalogItem{}
}

// TestPathConfigAnalyzerNormalizesJavaScriptSafeSeeds 验证边界 seed 与非法 seed 在服务端统一编码，避免浏览器预览和保存结果分叉。
func TestPathConfigAnalyzerNormalizesJavaScriptSafeSeeds(t *testing.T) {
	targetPlan := analyzer.PathConfigPersonTarget{
		Key:               "person-key",
		Required:          true,
		MinCount:          1,
		MaxCount:          1,
		CandidateOrder:    []string{"person-1", "person-2"},
		CandidateTokens:   map[string]string{"token-1": "person-1", "token-2": "person-2"},
		AllowedStrategies: map[string]bool{"random": true},
	}
	tests := []struct {
		name string
		seed int64
		want int64
	}{
		{name: "最小值", seed: 1, want: 1},
		{name: "最大安全值", seed: 9007199254740991, want: 9007199254740991},
		{name: "零值", seed: 0, want: 1},
		{name: "负数", seed: -7, want: 1},
		{name: "超过安全值", seed: 9007199254740992, want: 1},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			raw, reason := analyzer.EncodePathConfigPersonStrategy(targetPlan, model.PathConfigPersonStrategyInput{Key: targetPlan.Key, Strategy: "random", Seed: testCase.seed})
			if reason != "" {
				t.Fatalf("seed 规范化失败：%s", reason)
			}
			var stored struct {
				Seed int64 `json:"seed"`
			}
			if err := json.Unmarshal([]byte(raw), &stored); err != nil || stored.Seed != testCase.want {
				t.Fatalf("seed 编码不一致：raw=%s err=%v want=%d", raw, err, testCase.want)
			}
		})
	}
}

// TestPathConfigAnalyzerProjectsOrderedActionsAndLegacyMigration 验证动作目录、回退目标、有序步骤与旧 agree/disagree 首访迁移。
func TestPathConfigAnalyzerProjectsOrderedActionsAndLegacyMigration(t *testing.T) {
	tree := pathConfigTree()
	graph := requirementGraph(t, tree)
	path := model.ExecutionPath{Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}
	pathAnalysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, path.Choices)
	if err != nil {
		t.Fatalf("准备动作路径失败：%v", err)
	}
	configuration, validation, err := analyzer.NewPathConfigAnalyzer().Analyze(graph, tree, pathConfigFields(), path, pathAnalysis, nil, nil, map[string]string{"approve-a": "disagree"}, true)
	if err != nil {
		t.Fatalf("动作计划投影失败：%v", err)
	}
	approval := findConfigNode(configuration.Groups, "财务审批")
	if approval == nil || len(approval.ActionPlan.Arrivals) != 1 || approval.ActionPlan.Arrivals[0].Steps[0].Kind != "reject_no_pass" {
		t.Fatalf("旧 disagree 没有准确迁移为首访不同意：%+v", approval)
	}
	wantKinds := map[string]bool{"approve_pass": true, "reject_no_pass": true, "draft_save": true, "rollback_previous": true}
	for _, item := range approval.ActionPlan.Catalog {
		delete(wantKinds, item.Kind)
	}
	if len(wantKinds) != 0 || len(approval.ActionPlan.RollbackTargets) == 0 {
		t.Fatalf("审批动作目录或回退目标不完整：missing=%v plan=%+v", wantKinds, approval.ActionPlan)
	}
	nodeTarget := validation.NodeTokens[approval.Key]
	input := []model.PathConfigArrivalInput{{Visit: 1, Steps: []model.PathConfigActionStepInput{
		{Kind: "draft_save"},
	}}}
	if _, _, reason := analyzer.EncodePathConfigActionPlan(nodeTarget, input); reason != "" {
		t.Fatalf("合法暂存动作被拒绝：%s", reason)
	}
	input = []model.PathConfigArrivalInput{{Visit: 1, Steps: []model.PathConfigActionStepInput{{Kind: "approve_pass"}, {Kind: "draft_save"}}}}
	if _, _, reason := analyzer.EncodePathConfigActionPlan(nodeTarget, input); !strings.Contains(reason, "最后一步") {
		t.Fatalf("终止动作后的额外步骤没有被拒绝：%s", reason)
	}
	tooMany := make([]model.PathConfigArrivalInput, 11)
	for index := range tooMany {
		tooMany[index] = model.PathConfigArrivalInput{Visit: index + 1, Steps: []model.PathConfigActionStepInput{{Kind: "approve_pass"}}}
	}
	if _, _, reason := analyzer.EncodePathConfigActionPlan(nodeTarget, tooMany); !strings.Contains(reason, "10") {
		t.Fatalf("超过十次到达没有被拒绝：%s", reason)
	}
}

// TestPathConfigAnalyzerRollbackTargetsExcludeParallelSiblings 验证并行遍历中更早出现的兄弟支线不能被伪造成回退前驱。
func TestPathConfigAnalyzerRollbackTargetsExcludeParallelSiblings(t *testing.T) {
	tree := requirementParallelTree()
	graph := requirementGraph(t, tree)
	pathAnalysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, nil)
	if err != nil {
		t.Fatalf("准备并行路径失败：%v", err)
	}
	configuration, _, err := analyzer.NewPathConfigAnalyzer().Analyze(graph, tree, nil, model.ExecutionPath{}, pathAnalysis, nil, nil, nil)
	if err != nil {
		t.Fatalf("并行动作目录投影失败：%v", err)
	}
	finance := findConfigNode(configuration.Groups, "财务协同")
	business := findConfigNode(configuration.Groups, "业务审批")
	if finance == nil || business == nil {
		t.Fatalf("并行业务节点缺失：finance=%+v business=%+v", finance, business)
	}
	for _, option := range business.ActionPlan.RollbackTargets {
		if option.Label == finance.Name {
			t.Fatalf("并行兄弟节点被错误加入回退目录：%+v", business.ActionPlan.RollbackTargets)
		}
	}
	if len(business.ActionPlan.RollbackTargets) != 1 || business.ActionPlan.RollbackTargets[0].Label != "发起" {
		t.Fatalf("业务支线应仅能回退到共同前驱：%+v", business.ActionPlan.RollbackTargets)
	}
}

// TestPathConfigAnalyzerCountsWholePathActionLimit 验证整条路径超过一百个动作步骤时统一拒绝。
func TestPathConfigAnalyzerCountsWholePathActionLimit(t *testing.T) {
	targetPlan := analyzer.PathConfigNodeTarget{ActionKinds: map[string]bool{"draft_save": true}, RollbackTargets: map[string]string{}}
	arrivals := make([]model.PathConfigArrivalInput, 10)
	for index := range arrivals {
		arrivals[index] = model.PathConfigArrivalInput{Visit: index + 1, Steps: []model.PathConfigActionStepInput{{Kind: "draft_save"}}}
	}
	encoded, _, reason := analyzer.EncodePathConfigActionPlan(targetPlan, arrivals)
	if reason != "" {
		t.Fatalf("准备动作上限样本失败：%s", reason)
	}
	values := make(map[string]string)
	for index := 0; index < 10; index++ {
		values[analyzer.PathConfigActionPlanStorageKey(string(rune('a'+index)))] = encoded
	}
	if count, valid := analyzer.CountStoredPathConfigActionSteps(values); !valid || count != 100 {
		t.Fatalf("一百步边界计算错误：count=%d valid=%v", count, valid)
	}
	values[analyzer.PathConfigActionPlanStorageKey("overflow")] = encoded
	if count, valid := analyzer.CountStoredPathConfigActionSteps(values); valid || count != 110 {
		t.Fatalf("超过一百步没有被拒绝：count=%d valid=%v", count, valid)
	}
}

// pathConfigTree 构造含条件路由、两个审批节点和字段权限的真实树形样本。
func pathConfigTree() *target.FlowNodeTemplate {
	end := &target.FlowNodeTemplate{ID: "end", Name: "结束", Type: "end"}
	approvalB := &target.FlowNodeTemplate{
		ID: "approve-b", Name: "部门审批", Type: "common",
		AuditConfig: &target.FlowNodeAuditConfig{AuditType: "assign", Mode: "scramble", Details: []target.FlowAuditDetail{{Name: "部门负责人", Type: "personnel"}}},
		FieldPowers: []target.FlowNodeFieldPower{{FormID: "form-a", FieldID: "field-note", EnglishName: "note", Power: "edit"}},
	}
	approvalA := &target.FlowNodeTemplate{
		ID: "approve-a", Name: "财务审批", Type: "common",
		AuditConfig: &target.FlowNodeAuditConfig{AuditType: "assign", Mode: "scramble", Details: []target.FlowAuditDetail{{Name: "财务负责人", Type: "personnel"}}},
		FieldPowers: []target.FlowNodeFieldPower{
			{FormID: "form-a", FieldID: "field-amount", EnglishName: "amount", Power: "edit"},
			{FormID: "form-a", FieldID: "field-type", EnglishName: "type", Power: "edit"},
		},
	}
	approvalA.Child = approvalB
	route := &target.FlowNodeTemplate{
		ID: "route", Name: "金额条件", Type: "condition", Child: end,
		ConditionNodes: []target.FlowBranchTemplate{
			{ID: "branch-a", Name: "大额", Sort: 1, Child: approvalA},
			{ID: "branch-b", Name: "普通", Sort: 2, Child: &target.FlowNodeTemplate{ID: "approve-c", Name: "普通审批", Type: "common", AuditConfig: &target.FlowNodeAuditConfig{AuditType: "assign", Mode: "scramble", Details: []target.FlowAuditDetail{{Name: "审批负责人", Type: "personnel"}}}}},
		},
	}
	return &target.FlowNodeTemplate{ID: "start", Name: "发起", Type: "start", Child: route}
}

// pathConfigFields 构造与 pathConfigTree 对应的字段详情和 FormMaking 组件元数据。
func pathConfigFields() []target.FormFieldDetail {
	return []target.FormFieldDetail{
		{FormID: "form-a", FormName: "申请表", FieldID: "field-amount", Name: "申请金额", EnglishName: "amount", FieldType: "doubleType", DefaultValue: "1000", Required: true, ComponentType: "number"},
		{FormID: "form-a", FormName: "申请表", FieldID: "field-type", Name: "类型", EnglishName: "type", FieldType: "stringType", ComponentType: "select", Options: []target.FormFieldOption{{Label: "A", Value: "a"}, {Label: "B", Value: "b"}}},
		{FormID: "form-a", FormName: "申请表", FieldID: "field-note", Name: "备注", EnglishName: "note", FieldType: "stringType", ComponentType: "input"},
	}
}

// intPointer 为人员会签测试构造明确人数指针。
func intPointer(value int) *int { return &value }

// findConfigNode 按节点中文名查找配置节点。
func findConfigNode(groups []model.PathConfigGroup, name string) *model.PathConfigNode {
	for groupIndex := range groups {
		for nodeIndex := range groups[groupIndex].Nodes {
			node := &groups[groupIndex].Nodes[nodeIndex]
			if node.Name == name {
				return node
			}
		}
	}
	return nil
}

// findConfigField 按中文字段名查找配置字段。
func findConfigField(fields []model.PathConfigField, name string) *model.PathConfigField {
	for index := range fields {
		if fields[index].Name == name {
			return &fields[index]
		}
	}
	return nil
}

// assertPathConfigPublicSafety 断言公开配置不包含内部节点、表单或字段标识。
func assertPathConfigPublicSafety(t *testing.T, configuration model.PathConfiguration) {
	t.Helper()
	encoded, err := json.Marshal(configuration)
	if err != nil {
		t.Fatalf("配置 DTO 无法序列化：%v", err)
	}
	text := string(encoded)
	for _, forbidden := range []string{"approve-a", "approve-b", "approve-c", "route", "branch-a", "branch-b", "person-1", "person-2", "form-a", "field-amount", "field-type", "field-note", "englishName", "nodeId"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("配置公开 DTO 泄露内部标识 %q：%s", forbidden, text)
		}
	}
}
