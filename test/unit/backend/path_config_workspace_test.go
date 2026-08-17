package backend_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

type workspacePathConfigReader struct {
	*pathConfigReader
	samples      []map[string]any
	session      target.FormRuntimeSession
	sampleCalls  int
	sessionCalls int
}

// RecentFormSamples 返回预设近期样本并记录调用次数。
func (r *workspacePathConfigReader) RecentFormSamples(_ context.Context, _ string, _ int) ([]map[string]any, error) {
	r.sampleCalls++
	return r.samples, nil
}

// FormRuntimeSession 返回预设短期 SID 会话并记录调用次数。
func (r *workspacePathConfigReader) FormRuntimeSession(_ context.Context, _ string) (target.FormRuntimeSession, error) {
	r.sessionCalls++
	return r.session, nil
}

// TestPathConfigWorkspaceGeneratesAndPersistsFormSeparately 验证表单生成、完整 values 保存与逐节点确认互不冒充整条路径完成。
func TestPathConfigWorkspaceGeneratesAndPersistsFormSeparately(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration, Account: "account-a", FlowSource: "new", TargetObjectID: "template-a"}}
	baseReader := &pathConfigReader{snapshot: pathConfigWorkspaceSnapshot()}
	reader := &workspacePathConfigReader{
		pathConfigReader: baseReader,
		samples:          []map[string]any{{"amount": float64(3600), "type": "b", "note": "近期样本"}},
		session:          target.FormRuntimeSession{SID: "runtime-sid", BaseURL: "http://target.invalid/api", AccountName: "测试发起人"},
	}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{
		ID: 32, PlanID: 7, SequenceNo: 1, Name: "路径一",
		Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}},
	}}}
	configs := &memoryPathConfigRepository{}
	serviceUnderTest := newPathConfigService(t, plans, reader, paths, configs)

	initial, err := serviceUnderTest.Get(context.Background(), 7, 32)
	if err != nil || initial.Form.Status != "empty" || initial.Status != "pending" {
		t.Fatalf("首次表单或路径状态错误：configuration=%+v err=%v", initial, err)
	}
	generated, err := serviceUnderTest.GenerateForm(context.Background(), 7, 32, 73, nil, nil, false)
	if err != nil || generated.Status != "draft" || generated.Values["amount"] != float64(3600) || generated.SampleSummary.Recent == 0 {
		t.Fatalf("智能生成没有使用近期样本：generated=%+v err=%v", generated, err)
	}
	savedForm, err := serviceUnderTest.SaveForm(context.Background(), 7, 32, "123e4567-e89b-12d3-a456-426614174801", model.PathFormSaveInput{
		Revision: 0, Values: generated.Values, Seed: generated.Seed,
		GeneratedFieldPaths: generated.GeneratedFieldPaths, ManualOverridePaths: generated.ManualOverridePaths,
		SampleSummary: generated.SampleSummary, Validated: true,
	})
	if err != nil || savedForm.FormRevision != 1 || savedForm.Status != "pending" {
		t.Fatalf("表单独立保存错误地完成整条路径：saved=%+v err=%v", savedForm, err)
	}
	stored := configs.records[32]
	if stored.FormStatus != "valid" || !stored.FormValidated || stored.SampleSummary.Recent == 0 || !stored.SampleSummary.Saved {
		t.Fatalf("完整表单元数据没有持久化：%+v", stored)
	}

	afterForm, err := serviceUnderTest.Get(context.Background(), 7, 32)
	if err != nil || afterForm.Form.Status != "valid" || afterForm.Progress.Pending == 0 || afterForm.Status != "pending" {
		t.Fatalf("表单保存后节点进度或权威状态错误：configuration=%+v err=%v", afterForm, err)
	}
	for _, group := range afterForm.Groups {
		for _, node := range group.Nodes {
			if !hasEnabledWorkspaceAction(node) && !hasEditableWorkspacePerson(node) {
				continue
			}
			result, saveErr := serviceUnderTest.SaveNode(context.Background(), 7, 32, node.Key, nextWorkspaceSaveKey(configs.saveCalls), model.PathNodeSaveInput{
				Revision: configs.records[32].NodeRevision, Persons: workspaceNodePersons(node), ActionPlan: workspaceNodeActionPlan(node),
			})
			if saveErr != nil {
				t.Fatalf("逐节点保存失败：node=%s result=%+v err=%v", node.Name, result, saveErr)
			}
		}
	}
	complete, err := serviceUnderTest.Get(context.Background(), 7, 32)
	if err != nil || complete.Status != "configured" || complete.Progress.Pending != 0 {
		t.Fatalf("表单与必需节点全部完成后没有得到 configured：configuration=%+v err=%v", complete, err)
	}
}

// hasEnabledWorkspaceAction 判断节点是否存在需要用户保存的可用动作，禁用目录只用于解释规则。
func hasEnabledWorkspaceAction(node model.PathConfigNode) bool {
	for _, action := range node.ActionPlan.Catalog {
		if action.Enabled {
			return true
		}
	}
	return false
}

// TestPathConfigWorkspaceFormIdempotencyReconcilesLostResponse 验证表单保存响应丢失后同键重试不再读取目标且返回同一事实。
func TestPathConfigWorkspaceFormIdempotencyReconcilesLostResponse(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration, Account: "account-a", FlowSource: "new", TargetObjectID: "template-a"}}
	reader := &pathConfigReader{snapshot: pathConfigWorkspaceSnapshot()}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}}}
	configs := &memoryPathConfigRepository{}
	serviceUnderTest := newPathConfigService(t, plans, reader, paths, configs)
	key := "123e4567-e89b-12d3-a456-426614174811"
	input := model.PathFormSaveInput{
		Values: map[string]any{"amount": float64(2800), "type": "a", "note": "已保存"},
		Seed:   9, GeneratedFieldPaths: []string{"amount", "type", "note"}, Validated: true,
	}
	first, err := serviceUnderTest.SaveForm(context.Background(), 7, 32, key, input)
	if err != nil || first.FormRevision != 1 || reader.calls != 1 || configs.saveCalls != 1 {
		t.Fatalf("首次表单保存失败：result=%+v calls=%d saves=%d err=%v", first, reader.calls, configs.saveCalls, err)
	}
	reader.err = errors.New("目标随后不可用")
	input.Unsupported = []string{"迟到的运行时状态不应覆盖已成功事实"}
	retried, err := serviceUnderTest.SaveForm(context.Background(), 7, 32, key, input)
	if err != nil || retried.FormRevision != 1 || reader.calls != 1 || configs.saveCalls != 1 {
		t.Fatalf("同键对账没有直接返回原表单事实：result=%+v calls=%d saves=%d err=%v", retried, reader.calls, configs.saveCalls, err)
	}
}

// TestPathConfigWorkspaceReadOnlyUsesInstanceValues 验证已发或待发只投影实例当前值且不提供表单写入。
func TestPathConfigWorkspaceReadOnlyUsesInstanceValues(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration, Account: "account-a", FlowSource: "started", TargetObjectID: "instance-a"}}
	snapshot := pathConfigWorkspaceSnapshot()
	snapshot.InstanceValues = map[string]any{"amount": float64(9100), "type": "b", "note": "实例当前值"}
	reader := &pathConfigReader{snapshot: snapshot}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}}}
	serviceUnderTest := newPathConfigService(t, plans, reader, paths, &memoryPathConfigRepository{})
	configuration, err := serviceUnderTest.Get(context.Background(), 7, 32)
	if err != nil || !configuration.Form.ReadOnly || configuration.Form.Status != "valid" || configuration.Form.Values["note"] != "实例当前值" {
		t.Fatalf("已发实例当前值没有只读投影：configuration=%+v err=%v", configuration, err)
	}
	_, err = serviceUnderTest.SaveForm(context.Background(), 7, 32, "123e4567-e89b-12d3-a456-426614174821", model.PathFormSaveInput{Validated: true})
	if !service.IsPathConfigErrorKind(err, service.PathConfigErrorLocked) {
		t.Fatalf("已发表单写入没有被拒绝：%v", err)
	}
}

// TestPathConfigWorkspaceNewFormUsesEntryNodePermissions 验证新发起只开放真实入口节点 edit 字段，不合并下游审批权限。
func TestPathConfigWorkspaceNewFormUsesEntryNodePermissions(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration, Account: "account-a", FlowSource: "new", TargetObjectID: "template-a"}}
	snapshot := pathConfigWorkspaceSnapshot()
	snapshot.Tree.FieldPowers = []target.FlowNodeFieldPower{{EnglishName: "amount", Power: "edit"}}
	reader := &pathConfigReader{snapshot: snapshot}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}}}
	serviceUnderTest := newPathConfigService(t, plans, reader, paths, &memoryPathConfigRepository{})
	configuration, err := serviceUnderTest.Get(context.Background(), 7, 32)
	if err != nil {
		t.Fatalf("读取新发起表单权限失败：%v", err)
	}
	powers := make(map[string]string, len(configuration.Form.Permissions))
	for _, permission := range configuration.Form.Permissions {
		powers[permission.Field] = permission.Power
	}
	if powers["amount"] != "edit" || powers["type"] == "edit" || powers["note"] == "edit" {
		t.Fatalf("新发起错误合并下游节点权限：%+v", configuration.Form.Permissions)
	}
}

// TestPathConfigWorkspaceProjectsConditionFieldRules 验证仅实际命中分支高亮和禁用，其他条件保留普通说明。
func TestPathConfigWorkspaceProjectsConditionFieldRules(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration, Account: "account-a", FlowSource: "new", TargetObjectID: "template-a"}}
	snapshot := pathConfigWorkspaceSnapshot()
	snapshot.Tree.Child.ConditionNodes[0].Conditions = []target.FlowCondition{{FieldA: "amount", Judge: "gte", ValueB: "3000"}}
	snapshot.Tree.Child.ConditionNodes[1].Conditions = []target.FlowCondition{{FieldA: "unknown_$$_field", Judge: "eq", ValueB: "x"}}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}}}
	serviceUnderTest := newPathConfigService(t, plans, &pathConfigReader{snapshot: snapshot}, paths, &memoryPathConfigRepository{})
	configuration, err := serviceUnderTest.Get(context.Background(), 7, 32)
	if err != nil {
		t.Fatalf("读取条件字段规则失败：%v", err)
	}
	if len(configuration.Form.ConditionHints) != 2 || len(configuration.Form.FieldRules) != 0 {
		t.Fatalf("没有实际表单值时不应伪造命中或禁用字段：%+v", configuration.Form)
	}
	generated, err := serviceUnderTest.GenerateForm(context.Background(), 7, 32, 3, nil, nil, false)
	if err != nil || len(generated.ConditionHints) != 2 || !generated.ConditionHints[0].Mapped || !generated.ConditionHints[0].Protected || generated.ConditionHints[1].Mapped || generated.ConditionHints[1].Protected {
		t.Fatalf("只有实际命中的精确字段应高亮保护：generated=%+v err=%v", generated, err)
	}
	if len(generated.FieldRules) != 1 || generated.FieldRules[0].Field != "amount" || !generated.FieldRules[0].Disabled || len(generated.FieldRules[0].ConditionHints) != 1 {
		t.Fatalf("实际命中字段没有生成真实组件禁用规则：%+v", generated.FieldRules)
	}

	snapshot.Tree.Child.ConditionNodes[0].Conditions = []target.FlowCondition{{FieldA: "unknown_$$_field", Judge: "eq", ValueB: "x"}}
	configuration, err = newPathConfigService(t, plans, &pathConfigReader{snapshot: snapshot}, paths, &memoryPathConfigRepository{}).Get(context.Background(), 7, 32)
	if err != nil || len(configuration.Form.FieldRules) != 0 || len(configuration.Form.ConditionHints) != 2 || configuration.Form.ConditionHints[0].Mapped || configuration.Form.ConditionHints[0].Protected {
		t.Fatalf("无法精确映射字段被错误禁用或未提示：form=%+v err=%v", configuration.Form, err)
	}
}

// TestPathConfigWorkspaceChangesOnlyWhenNextCandidateExists 验证换一组轮转同一近期样本来源且无新候选时明确拒绝。
func TestPathConfigWorkspaceChangesOnlyWhenNextCandidateExists(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration, Account: "account-a", FlowSource: "new", TargetObjectID: "template-a"}}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}}}
	reader := &workspacePathConfigReader{pathConfigReader: &pathConfigReader{snapshot: pathConfigWorkspaceSnapshot()}, samples: []map[string]any{
		{"amount": float64(1800), "type": "a", "note": "样本甲"},
		{"amount": float64(2800), "type": "b", "note": "样本乙"},
	}}
	serviceUnderTest := newPathConfigService(t, plans, reader, paths, &memoryPathConfigRepository{})
	first, err := serviceUnderTest.GenerateForm(context.Background(), 7, 32, 1, nil, nil, false)
	if err != nil {
		t.Fatalf("生成首组候选失败：%v", err)
	}
	next, err := serviceUnderTest.GenerateForm(context.Background(), 7, 32, 2, first.Values, nil, true)
	if err != nil || next.Values["amount"] == first.Values["amount"] || next.Values["note"] == first.Values["note"] {
		t.Fatalf("换一组没有切换到同源下一有效候选：first=%+v next=%+v err=%v", first.Values, next.Values, err)
	}

	reader.samples = []map[string]any{{"amount": next.Values["amount"], "type": next.Values["type"], "note": next.Values["note"]}}
	_, err = serviceUnderTest.GenerateForm(context.Background(), 7, 32, 3, next.Values, nil, true)
	if !service.IsPathConfigErrorKind(err, service.PathConfigErrorInvalidArgument) {
		t.Fatalf("没有下一候选时错误报告切换成功：%v", err)
	}
}

// TestPathConfigWorkspaceSupportsRegisteredCustomValuesSeparately 验证已注册自定义组件不阻断节点保存，完整值和虚拟字段可独立往返。
func TestPathConfigWorkspaceSupportsRegisteredCustomValuesSeparately(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration, Account: "account-a", FlowSource: "new", TargetObjectID: "template-a"}}
	snapshot := pathConfigWorkspaceSnapshot()
	snapshot.Tree.FieldPowers = append(snapshot.Tree.FieldPowers, target.FlowNodeFieldPower{FormID: "form-a", FieldID: "field-general", EnglishName: "generalInfo", Power: "edit"})
	snapshot.Forms[0].TemplateData = `{"list":[{"type":"number","model":"amount","name":"申请金额","options":{"required":true}},{"type":"select","model":"type","name":"类型","options":{"required":true,"options":[{"label":"A","value":"a"},{"label":"B","value":"b"}]}},{"type":"input","model":"note","name":"备注","options":{}},{"type":"custom","el":"custome-info-select","model":"generalInfo","name":"通用信息选择","options":{}}],"config":{}}`
	reader := &pathConfigReader{snapshot: snapshot}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}}}
	configs := &memoryPathConfigRepository{}
	serviceUnderTest := newPathConfigService(t, plans, reader, paths, configs)

	configuration, err := serviceUnderTest.Get(context.Background(), 7, 32)
	if err != nil || configuration.Form.Status == "unsupported" {
		t.Fatalf("已注册自定义组件被服务端误判为不支持：configuration=%+v err=%v", configuration, err)
	}
	for _, group := range configuration.Groups {
		for _, node := range group.Nodes {
			if len(node.Fields) != 0 || len(node.Gaps) != 0 {
				t.Fatalf("表单组件错误进入节点配置：%+v", node)
			}
		}
	}
	start := findConfigNode(configuration.Groups, "发起")
	if start == nil {
		t.Fatal("缺少发起节点")
	}
	if _, err := serviceUnderTest.SaveNode(context.Background(), 7, 32, start.Key, "123e4567-e89b-12d3-a456-426614174851", model.PathNodeSaveInput{
		Revision: configuration.NodeRevision, ActionPlan: workspaceNodeActionPlan(*start),
	}); err != nil {
		t.Fatalf("自定义组件错误阻断节点独立保存：%v", err)
	}

	values := map[string]any{
		"amount": float64(2800), "type": "a", "note": "人工填写",
		"generalInfo":               `{"id":"project-42","name":"示例项目"}`,
		"generalInfo__condition":    "示例项目",
		"generalInfo__formPersonId": "project-42",
	}
	generated, err := serviceUnderTest.GenerateForm(context.Background(), 7, 32, 17, values, []string{"generalInfo"}, false)
	if err != nil || len(generated.Unsupported) != 0 || generated.Values["generalInfo"] != values["generalInfo"] {
		t.Fatalf("生成器错误阻断或覆盖自定义组件值：generated=%+v err=%v", generated, err)
	}
	result, err := serviceUnderTest.SaveForm(context.Background(), 7, 32, "123e4567-e89b-12d3-a456-426614174852", model.PathFormSaveInput{
		Revision: generated.Revision, Values: generated.Values, Seed: generated.Seed,
		GeneratedFieldPaths: generated.GeneratedFieldPaths, ManualOverridePaths: generated.ManualOverridePaths,
		SampleSummary: generated.SampleSummary, Validated: true, Unsupported: []string{},
	})
	if err != nil || result.FormRevision != 1 {
		t.Fatalf("已注册自定义组件完整值保存失败：result=%+v err=%v", result, err)
	}
	reloaded, err := serviceUnderTest.Get(context.Background(), 7, 32)
	if err != nil ||
		reloaded.Form.Values["generalInfo"] != values["generalInfo"] ||
		reloaded.Form.Values["generalInfo__condition"] != values["generalInfo__condition"] ||
		reloaded.Form.Values["generalInfo__formPersonId"] != values["generalInfo__formPersonId"] {
		t.Fatalf("自定义组件或虚拟字段没有完整往返：form=%+v err=%v", reloaded.Form, err)
	}
}

// TestPathConfigWorkspaceRejectsRuntimeUnsupportedComponents 验证真实运行时报告的未知组件不能绕过正常表单保存流程。
func TestPathConfigWorkspaceRejectsRuntimeUnsupportedComponents(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration, Account: "account-a", FlowSource: "new", TargetObjectID: "template-a"}}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}}}
	configs := &memoryPathConfigRepository{}
	serviceUnderTest := newPathConfigService(t, plans, &pathConfigReader{snapshot: pathConfigWorkspaceSnapshot()}, paths, configs)
	configuration, err := serviceUnderTest.Get(context.Background(), 7, 32)
	if err != nil {
		t.Fatalf("读取未知组件场景的节点配置失败：%v", err)
	}
	start := findConfigNode(configuration.Groups, "发起")
	if start == nil {
		t.Fatal("未知组件场景缺少发起节点")
	}
	if _, err := serviceUnderTest.SaveNode(context.Background(), 7, 32, start.Key, "123e4567-e89b-12d3-a456-426614174853", model.PathNodeSaveInput{
		Revision: configuration.NodeRevision, ActionPlan: workspaceNodeActionPlan(*start),
	}); err != nil {
		t.Fatalf("未知组件错误阻断节点人员动作独立保存：%v", err)
	}
	_, err = serviceUnderTest.SaveForm(context.Background(), 7, 32, "123e4567-e89b-12d3-a456-426614174854", model.PathFormSaveInput{
		Revision:  configuration.Form.Revision,
		Validated: true, Unsupported: []string{"未知宿主组件：依赖 rsh-flow-components 宿主业务适配"},
	})
	if !service.IsPathConfigErrorKind(err, service.PathConfigErrorInvalid) || configs.saveCalls != 1 {
		t.Fatalf("运行时未知组件没有阻止保存：err=%v saves=%d", err, configs.saveCalls)
	}
}

// TestPathConfigWorkspaceSavesPersonStrategyAndActionPlan 验证逐节点保存只合并当前人员策略与语义化动作计划，并在候选变化后权威标记失效。
func TestPathConfigWorkspaceSavesPersonStrategyAndActionPlan(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration, Account: "account-a", FlowSource: "new", TargetObjectID: "template-a"}}
	snapshot := pathConfigWorkspaceSnapshot()
	approvalTemplate := snapshot.Tree.Child.ConditionNodes[0].Child
	approvalTemplate.AuditConfig = &target.FlowNodeAuditConfig{
		AuditType: "run_node_choose", Mode: "countersign", CountersignNum: intPointer(2),
		Candidates: []target.FlowAuditCandidate{{ID: "person-1", Name: "张三"}, {ID: "person-2", Name: "李四"}, {ID: "person-3", Name: "王五"}},
	}
	reader := &pathConfigReader{snapshot: snapshot}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}}}
	configs := &memoryPathConfigRepository{}
	serviceUnderTest := newPathConfigService(t, plans, reader, paths, configs)
	configuration, err := serviceUnderTest.Get(context.Background(), 7, 32)
	if err != nil {
		t.Fatalf("读取人员动作配置失败：%v", err)
	}
	approval := findConfigNode(configuration.Groups, "财务审批")
	if approval == nil || len(approval.Persons) != 1 || len(approval.ActionPlan.Catalog) < 5 {
		t.Fatalf("人员策略或动作目录没有投影：%+v", approval)
	}
	person := approval.Persons[0]
	input := model.PathNodeSaveInput{
		Revision:   configuration.NodeRevision,
		Persons:    []model.PathConfigPersonStrategyInput{{Key: person.Key, Strategy: "manual", Seed: 11, Selected: []string{person.Options[0].Value, person.Options[1].Value}}},
		ActionPlan: model.PathConfigActionPlanInput{Result: model.PathConfigActionStepInput{Kind: "approve_pass", Opinion: "同意办理"}},
	}
	result, err := serviceUnderTest.SaveNode(context.Background(), 7, 32, approval.Key, "123e4567-e89b-12d3-a456-426614174880", input)
	if err != nil || result.NodeRevision != 1 {
		t.Fatalf("新版逐节点保存失败：result=%+v err=%v", result, err)
	}
	stored := configs.records[32]
	if stored.ActionValues["person-plan:approve-a"] == "" || stored.ActionValues["action-plan:approve-a"] == "" || stored.ActionValues["approve-a"] != "" {
		t.Fatalf("新版人员和动作没有进入独立命名空间：%+v", stored.ActionValues)
	}
	refreshed, err := serviceUnderTest.Get(context.Background(), 7, 32)
	if err != nil || findConfigNode(refreshed.Groups, "财务审批").Status != "configured" {
		t.Fatalf("刷新没有以已保存节点事实投影完成：configuration=%+v err=%v", refreshed, err)
	}
	approvalTemplate.AuditConfig.Candidates = approvalTemplate.AuditConfig.Candidates[:1]
	affected, err := serviceUnderTest.Get(context.Background(), 7, 32)
	if err != nil || findConfigNode(affected.Groups, "财务审批").Status != "affected" || affected.Status != "affected" {
		t.Fatalf("候选变化没有使存量策略失效：configuration=%+v err=%v", affected, err)
	}
}

// TestPathConfigWorkspaceRejectsDirectoryResolutionFailure 验证目标目录失败不能绕过页面直接保存为完成节点。
func TestPathConfigWorkspaceRejectsDirectoryResolutionFailure(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration, Account: "account-a", FlowSource: "new", TargetObjectID: "template-a"}}
	snapshot := pathConfigWorkspaceSnapshot()
	approvalTemplate := snapshot.Tree.Child.ConditionNodes[0].Child
	approvalTemplate.AuditConfig = &target.FlowNodeAuditConfig{
		AuditType: "run_node_choose", Mode: "scramble",
		ResolutionIssues: []target.FlowAuditResolutionIssue{{Category: "角色", Reason: "角色范围读取失败"}},
	}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}}}
	serviceUnderTest := newPathConfigService(t, plans, &pathConfigReader{snapshot: snapshot}, paths, &memoryPathConfigRepository{})
	configuration, err := serviceUnderTest.Get(context.Background(), 7, 32)
	if err != nil {
		t.Fatalf("读取目录失败节点配置失败：%v", err)
	}
	approval := findConfigNode(configuration.Groups, "财务审批")
	_, err = serviceUnderTest.SaveNode(context.Background(), 7, 32, approval.Key, "123e4567-e89b-12d3-a456-426614174881", model.PathNodeSaveInput{
		Revision:   configuration.NodeRevision,
		ActionPlan: model.PathConfigActionPlanInput{Result: model.PathConfigActionStepInput{Kind: "approve_pass"}},
	})
	if !service.IsPathConfigErrorKind(err, service.PathConfigErrorInvalid) {
		t.Fatalf("目录读取失败节点被错误保存：%v", err)
	}
}

// pathConfigWorkspaceSnapshot 构造带完整目标 FormMaking 模板的当前路径快照。
func pathConfigWorkspaceSnapshot() target.PathConfigurationSnapshot {
	tree := pathConfigTree()
	// 基础夹具将第一分支声明为可证明满足的条件，避免测试把无条件分支误当作生成目标。
	tree.Child.ConditionNodes[0].Conditions = []target.FlowCondition{{FieldA: "amount", Judge: "gte", ValueB: "0"}}
	tree.FieldPowers = []target.FlowNodeFieldPower{
		{FormID: "form-a", FieldID: "field-amount", EnglishName: "amount", Power: "edit"},
		{FormID: "form-a", FieldID: "field-type", EnglishName: "type", Power: "edit"},
		{FormID: "form-a", FieldID: "field-note", EnglishName: "note", Power: "edit"},
	}
	return target.PathConfigurationSnapshot{
		Tree: tree, EntryNodeIDs: []string{"start"}, FormFields: pathConfigFields(),
		Forms: []target.FormRuntimeTemplate{{Name: "申请表", TemplateData: `{"list":[{"type":"number","model":"amount","name":"申请金额","options":{"required":true}},{"type":"select","model":"type","name":"类型","options":{"required":true,"options":[{"label":"A","value":"a"},{"label":"B","value":"b"}]}},{"type":"input","model":"note","name":"备注","options":{}}],"config":{}}`}},
	}
}

// hasEditableWorkspacePerson 判断节点是否包含需要本轮保存的可编辑人员。
func hasEditableWorkspacePerson(node model.PathConfigNode) bool {
	for _, person := range node.Persons {
		if person.Editable {
			return true
		}
	}
	return false
}

// workspaceNodePersons 返回节点当前人员策略，用于验证新版逐节点保存边界。
func workspaceNodePersons(node model.PathConfigNode) []model.PathConfigPersonStrategyInput {
	result := make([]model.PathConfigPersonStrategyInput, 0, len(node.Persons))
	for _, person := range node.Persons {
		if person.Editable {
			result = append(result, model.PathConfigPersonStrategyInput{Key: person.Key, Strategy: person.Strategy, Seed: person.StrategySeed, Selected: append([]string(nil), person.Selected...)})
		}
	}
	return result
}

// workspaceNodeActionPlan 把公开加签节点和唯一处理结果转换成逐节点保存输入。
func workspaceNodeActionPlan(node model.PathConfigNode) model.PathConfigActionPlanInput {
	result := model.PathConfigActionPlanInput{
		AddSignNodes: make([]model.PathConfigAddSignNodeInput, 0, len(node.ActionPlan.AddSignNodes)),
		Result:       model.PathConfigActionStepInput{Kind: node.ActionPlan.Result.Kind, Target: node.ActionPlan.Result.Target, Person: node.ActionPlan.Result.Person},
	}
	for _, addSign := range node.ActionPlan.AddSignNodes {
		result.AddSignNodes = append(result.AddSignNodes, model.PathConfigAddSignNodeInput{Person: addSign.Person})
	}
	return result
}

// nextWorkspaceSaveKey 按保存次数生成不同且合法的测试幂等键。
func nextWorkspaceSaveKey(index int) string {
	return fmt.Sprintf("123e4567-e89b-12d3-a456-%012d", 426614174830+index)
}
