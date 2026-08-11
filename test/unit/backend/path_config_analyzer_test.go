package backend_test

import (
	"encoding/json"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
)

// TestPathConfigAnalyzerProjectsFieldsActionsAndValuesInPathOrder 验证字段、动作、默认值与实例现值按真实节点顺序投影。
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
	if len(approval.Fields) != 2 {
		t.Fatalf("可编辑字段数量不正确：%+v", approval.Fields)
	}
	amount := findConfigField(approval.Fields, "申请金额")
	if amount == nil || amount.Type != "number" || amount.Required != true || amount.Value != "2500.5" {
		t.Fatalf("实例现值没有覆盖数字字段：%+v", amount)
	}
	kind := findConfigField(approval.Fields, "类型")
	if kind == nil || kind.Type != "singleSelect" || len(kind.Options) != 2 || kind.Options[0].Label != "A" {
		t.Fatalf("单选字段选项不正确：%+v", kind)
	}
	if _, exists := validation.FieldTokens[amount.Key]; !exists || validation.FieldTokens[amount.Key].NodeID == "" || validation.FieldTokens[amount.Key].FieldKey != "amount" {
		t.Fatalf("字段回写索引不正确：%+v", validation.FieldTokens[amount.Key])
	}
	assertPathConfigPublicSafety(t, configuration)
}

// TestPathConfigAnalyzerOverlaysStoredValuesAndMarksAffected 验证已保存值覆盖默认值，选项变化时标出受影响项目。
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
	amount := findConfigField(approval.Fields, "申请金额")
	if amount == nil || amount.Value != "888" || amount.Affected {
		t.Fatalf("可对应已保存值没有保留：%+v", amount)
	}
	kind := findConfigField(approval.Fields, "类型")
	if kind == nil || !kind.Affected || !strings.Contains(kind.Note, "选项已变化") {
		t.Fatalf("失效选项没有标记受影响：%+v", kind)
	}
	if configuration.Status != "affected" {
		t.Fatalf("结构变化状态没有反映到配置状态：%s", configuration.Status)
	}
	assertPathConfigPublicSafety(t, configuration)
}

// TestPathConfigAnalyzerStoredValueWinsOverInstanceValue 验证同一字段同时存在实例现值与已保存值时已保存值优先。
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
	amount := findConfigField(approval.Fields, "申请金额")
	if amount == nil || amount.Value != "888" || amount.Affected {
		t.Fatalf("已保存数字值被实例现值覆盖：%+v", amount)
	}
	kind := findConfigField(approval.Fields, "类型")
	if kind == nil || kind.Value != "\"a\"" || kind.Affected {
		t.Fatalf("已保存单选值没有优先或错误标记受影响：%+v", kind)
	}
	if configuration.Status != "configured" {
		t.Fatalf("有效已保存值不应改变配置状态：%s", configuration.Status)
	}
}

// TestPathConfigAnalyzerMarksAffectedWhenStoredValueInvalidEvenWithInstanceValue 验证已保存值失效时保留原值并标记受影响。
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
	amount := findConfigField(approval.Fields, "申请金额")
	if amount == nil || amount.Value != "\"abc\"" || !amount.Affected || !strings.Contains(amount.Note, "已变化") {
		t.Fatalf("失效已保存值没有保留并标记受影响：%+v", amount)
	}
	if configuration.Status != "affected" {
		t.Fatalf("失效已保存值没有把配置状态置为 affected：%s", configuration.Status)
	}
}

// TestPathConfigAnalyzerUsesInstanceValueWithoutStoredValue 验证已发/待发未保存过配置时仍显示实例当前值。
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
	amount := findConfigField(approval.Fields, "申请金额")
	if amount == nil || amount.Value != "2500.5" || amount.Affected {
		t.Fatalf("未保存字段没有显示实例现值：%+v", amount)
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

// TestPathConfigAnalyzerProjectsStructuredPersonNames 验证人员、岗位、岗级、角色和组织名称结构化公开，无名称范围只给出运行时解析说明。
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
		{name: "组织", auditType: "department", details: []target.FlowAuditDetail{{Name: "财务部", Type: "department"}}, wantCategory: "组织", wantName: "财务部", wantItemCount: 1},
		{name: "无名称范围", auditType: "run_node_choose", scopes: []target.FlowAuditScope{{Type: "level"}, {Type: "level"}}, wantCategory: "岗级", wantName: "名称需运行时解析", wantItemCount: 2},
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
			if len(person.Items) != 1 || person.Items[0].Category != testCase.wantCategory || person.Items[0].Name != testCase.wantName || person.Items[0].Count != testCase.wantItemCount {
				t.Fatalf("结构化人员名称不正确：%+v", person.Items)
			}
			encoded, _ := json.Marshal(person.Items)
			if strings.Contains(string(encoded), "person-secret") || strings.Contains(string(encoded), "bizId") {
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

// TestPathConfigAnalyzerDistinguishesDateAndDateTimeControls 验证目标元数据的日期模式映射为严格控件类型。
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
	if findConfigField(approvalConfig.Fields, "日期").Type != analyzer.PathConfigTypeDate || findConfigField(approvalConfig.Fields, "日期时间").Type != analyzer.PathConfigTypeDateTime {
		t.Fatalf("日期与日期时间控件没有区分：%+v", approvalConfig.Fields)
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

// TestPathConfigAnalyzerMapsUnsupportedFieldsToGaps 验证未知控件、明细表和缺选项字段转为不可编辑缺口。
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
	configuration, _, err := analyzer.NewPathConfigAnalyzer().Analyze(graph, tree, fields, model.ExecutionPath{SequenceNo: 1}, analysis, nil, nil, nil)
	if err != nil {
		t.Fatalf("缺口投影失败：%v", err)
	}
	start := configuration.Groups[0].Nodes[0]
	if len(start.Fields) != 0 || len(start.Gaps) != 4 {
		t.Fatalf("缺口数量不正确：fields=%+v gaps=%+v", start.Fields, start.Gaps)
	}
	reasons := make([]string, 0, len(start.Gaps))
	for _, gap := range start.Gaps {
		reasons = append(reasons, gap.Reason)
	}
	joined := strings.Join(reasons, "|")
	for _, want := range []string{"明细表", "附件", "选项来源无法确认", "字段权限不明确"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("缺口缺少原因 %q：%s", want, joined)
		}
	}
	assertPathConfigPublicSafety(t, configuration)
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
