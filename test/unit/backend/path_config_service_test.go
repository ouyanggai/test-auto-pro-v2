package backend_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	"test-auto-pro-v2/internal/service"
)

type pathConfigReader struct {
	snapshot target.PathConfigurationSnapshot
	err      error
	calls    int
	account  string
	source   string
	targetID string
}

// PathConfigurationSnapshot 记录服务传入的计划身份并返回预设快照。
func (r *pathConfigReader) PathConfigurationSnapshot(_ context.Context, account, source, targetID string) (target.PathConfigurationSnapshot, error) {
	r.calls++
	r.account, r.source, r.targetID = account, source, targetID
	return r.snapshot, r.err
}

type memoryPathConfigRepository struct {
	records   map[uint64]model.StoredPathConfig
	saveCalls int
	saveErr   error
}

// FindByPath 返回内存中指定路径的当前配置。
func (r *memoryPathConfigRepository) FindByPath(_ context.Context, pathID uint64) (model.StoredPathConfig, bool, error) {
	record, found := r.records[pathID]
	return record, found, nil
}

// FindByPathAndKey 只在指定路径内按幂等键返回已保存配置。
func (r *memoryPathConfigRepository) FindByPathAndKey(_ context.Context, pathID uint64, idempotencyKey string) (model.StoredPathConfig, bool, error) {
	record, found := r.records[pathID]
	if !found || record.IdempotencyKey != idempotencyKey {
		return model.StoredPathConfig{}, false, nil
	}
	return record, true, nil
}

// Save 校验期望修订号后保存内存配置并推进修订号。
func (r *memoryPathConfigRepository) Save(_ context.Context, record model.StoredPathConfig, expectedRevision uint64, now time.Time) (model.StoredPathConfig, error) {
	r.saveCalls++
	if r.saveErr != nil {
		return model.StoredPathConfig{}, r.saveErr
	}
	if r.records == nil {
		r.records = make(map[uint64]model.StoredPathConfig)
	}
	if existing, found := r.records[record.PathID]; found && existing.Revision != expectedRevision {
		return model.StoredPathConfig{}, repository.ErrPathConfigConflict
	}
	if _, found := r.records[record.PathID]; !found && expectedRevision != 0 {
		return model.StoredPathConfig{}, repository.ErrPathConfigConflict
	}
	r.records[record.PathID] = record
	return record, nil
}

// newPathConfigService 组装使用真实分析器与内存仓储的配置服务。
func newPathConfigService(t *testing.T, plans *memoryPlanRepository, reader service.PathConfigReader, paths *memoryExecutionPathRepository, configs *memoryPathConfigRepository) *service.PathConfigService {
	t.Helper()
	return service.NewPathConfigService(
		service.NewPlanService(plans), reader, analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(),
		analyzer.NewPathConfigAnalyzer(), paths, configs,
	)
}

// TestPathConfigServiceUsesPersistedIdentityAndPlanScopedPath 验证读取只使用计划身份且不泄露其他计划路径。
func TestPathConfigServiceUsesPersistedIdentityAndPlanScopedPath(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration, Account: "saved-account", FlowSource: "new", TargetObjectID: "saved-template", TargetObjectName: "流程"}}
	reader := &pathConfigReader{snapshot: target.PathConfigurationSnapshot{Tree: pathConfigTree(), EntryNodeIDs: []string{"start"}, FormFields: pathConfigFields()}}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{
		{ID: 31, PlanID: 8, SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}},
		{ID: 32, PlanID: 7, SequenceNo: 2, Name: "本计划路径", Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}},
	}}
	serviceUnderTest := newPathConfigService(t, plans, reader, paths, &memoryPathConfigRepository{})
	configuration, err := serviceUnderTest.Get(context.Background(), 7, 32)
	if err != nil || configuration.Path.SequenceNo != 2 || configuration.Path.Name != "本计划路径" || len(configuration.Groups) == 0 {
		t.Fatalf("读取本计划配置失败：config=%+v err=%v", configuration, err)
	}
	if reader.account != "saved-account" || reader.source != "new" || reader.targetID != "saved-template" {
		t.Fatalf("配置读取没有只使用计划身份：%+v", reader)
	}
	if _, err := serviceUnderTest.Get(context.Background(), 7, 31); !service.IsPathConfigErrorKind(err, service.PathConfigErrorNotFound) {
		t.Fatalf("其他计划路径归属没有被隔离：%v", err)
	}
}

// TestPathConfigServiceGetWithoutStoredConfigReportsPending 验证首次读取无配置记录时公开待保存状态。
func TestPathConfigServiceGetWithoutStoredConfigReportsPending(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration}}
	reader := &pathConfigReader{snapshot: target.PathConfigurationSnapshot{Tree: pathConfigTree(), EntryNodeIDs: []string{"start"}, FormFields: pathConfigFields()}}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}}}
	configuration, err := newPathConfigService(t, plans, reader, paths, &memoryPathConfigRepository{}).Get(context.Background(), 7, 32)
	if err != nil || configuration.Status != "pending" {
		t.Fatalf("无配置记录没有显示待保存状态：configuration=%+v err=%v", configuration, err)
	}
}

// TestPathConfigServiceIdempotentRetrySkipsTargetRead 验证已成功保存后同键重试直接返回原结果且不再读取目标图。
func TestPathConfigServiceIdempotentRetrySkipsTargetRead(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration}}
	reader := &pathConfigReader{snapshot: target.PathConfigurationSnapshot{Tree: pathConfigTree(), EntryNodeIDs: []string{"start"}, FormFields: pathConfigFields()}}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}}}
	configs := &memoryPathConfigRepository{}
	serviceUnderTest := newPathConfigService(t, plans, reader, paths, configs)
	var fields []model.PathConfigFieldValue
	first, err := serviceUnderTest.Save(context.Background(), 7, 32, "123e4567-e89b-12d3-a456-426614174501", 0, fields, nil)
	if err != nil || first.Revision != 1 || reader.calls != 1 {
		t.Fatalf("首次保存失败：result=%+v calls=%d err=%v", first, reader.calls, err)
	}
	reader.err = errors.New("目标随后不可用")
	retried, err := serviceUnderTest.Save(context.Background(), 7, 32, "123e4567-e89b-12d3-a456-426614174501", 0, nil, nil)
	if err != nil || retried.Revision != 1 || reader.calls != 1 {
		t.Fatalf("同键重试没有直接返回原结果：result=%+v calls=%d err=%v", retried, reader.calls, err)
	}
	if configs.saveCalls != 1 {
		t.Fatalf("同键重试产生了重复写入：%d", configs.saveCalls)
	}
}

// TestPathConfigServiceRejectsRevisionConflict 验证修订号不一致时拒绝保存并保留原配置。
func TestPathConfigServiceRejectsRevisionConflict(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration}}
	reader := &pathConfigReader{snapshot: target.PathConfigurationSnapshot{Tree: pathConfigTree(), EntryNodeIDs: []string{"start"}, FormFields: pathConfigFields()}}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}}}
	configs := &memoryPathConfigRepository{}
	serviceUnderTest := newPathConfigService(t, plans, reader, paths, configs)
	var fields []model.PathConfigFieldValue
	if _, err := serviceUnderTest.Save(context.Background(), 7, 32, "123e4567-e89b-12d3-a456-426614174502", 0, fields, nil); err != nil {
		t.Fatalf("首次保存失败：%v", err)
	}
	if _, err := serviceUnderTest.Save(context.Background(), 7, 32, "123e4567-e89b-12d3-a456-426614174503", 0, fields, nil); !service.IsPathConfigErrorKind(err, service.PathConfigErrorRevisionConflict) {
		t.Fatalf("过期修订号没有被拒绝：%v", err)
	}
	if _, err := serviceUnderTest.Save(context.Background(), 7, 32, "123e4567-e89b-12d3-a456-426614174503", 1, fields, nil); err != nil {
		t.Fatalf("正确修订号保存失败：%v", err)
	}
}

// TestPathConfigServiceValidatesFieldBoundaries 验证未知键、缺少必填、选项外值、重复键都被稳定拒绝。
func TestPathConfigServiceValidatesFieldBoundaries(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration}}
	reader := &pathConfigReader{snapshot: target.PathConfigurationSnapshot{Tree: pathConfigTree(), EntryNodeIDs: []string{"start"}, FormFields: pathConfigFields()}}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}}}
	serviceUnderTest := newPathConfigService(t, plans, reader, paths, &memoryPathConfigRepository{})
	valid := validPathConfigSubmission()
	tests := []struct {
		name   string
		mutate func([]model.PathConfigFieldValue) []model.PathConfigFieldValue
	}{
		{name: "未知字段键", mutate: func(fields []model.PathConfigFieldValue) []model.PathConfigFieldValue {
			fields[0].Key = "00000000000000000000000000000000"
			return fields
		}},
		{name: "缺少必填字段", mutate: func(fields []model.PathConfigFieldValue) []model.PathConfigFieldValue { return fields[:1] }},
		{name: "选项外值", mutate: func(fields []model.PathConfigFieldValue) []model.PathConfigFieldValue {
			fields[1].Value = "\"removed\""
			return fields
		}},
		{name: "重复字段键", mutate: func(fields []model.PathConfigFieldValue) []model.PathConfigFieldValue {
			return append(fields, fields[0])
		}},
		{name: "必填为空", mutate: func(fields []model.PathConfigFieldValue) []model.PathConfigFieldValue {
			fields[0].Value = "\"\""
			return fields
		}},
	}
	for _, test := range tests {
		fields := append([]model.PathConfigFieldValue(nil), valid...)
		fields = test.mutate(fields)
		_, err := serviceUnderTest.Save(context.Background(), 7, 32, "123e4567-e89b-12d3-a456-426614174504", 0, fields, nil)
		if !service.IsPathConfigErrorKind(err, service.PathConfigErrorInvalid) {
			t.Fatalf("%s 没有被拒绝：%v", test.name, err)
		}
	}
}

// TestPathConfigServiceStoresDisagreeAndBlocksSubsequent 验证不同意动作可保存且值按节点隔离。
func TestPathConfigServiceStoresDisagreeAndBlocksSubsequent(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration}}
	reader := &pathConfigReader{snapshot: target.PathConfigurationSnapshot{Tree: pathConfigTree(), EntryNodeIDs: []string{"start"}, FormFields: pathConfigFields()}}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}}}
	serviceUnderTest := newPathConfigService(t, plans, reader, paths, &memoryPathConfigRepository{})
	disagreeKey := analyzer.PathConfigActionToken("approve-a", "agree_disagree")
	actions := []model.PathConfigActionValue{{Key: disagreeKey, Action: "disagree"}}
	result, err := serviceUnderTest.Save(context.Background(), 7, 32, "123e4567-e89b-12d3-a456-426614174505", 0, nil, actions)
	if err != nil || result.Revision != 1 {
		t.Fatalf("不同意动作保存失败：result=%+v err=%v", result, err)
	}
	configuration, err := serviceUnderTest.Get(context.Background(), 7, 32)
	if err != nil {
		t.Fatalf("重新读取配置失败：%v", err)
	}
	approval := findConfigNode(configuration.Groups, "财务审批")
	if approval == nil || approval.Actions[0].Current != "disagree" {
		t.Fatalf("不同意动作没有恢复：%+v", approval)
	}
	next := findConfigNode(configuration.Groups, "部门审批")
	if next == nil || !next.LineBlocked {
		t.Fatalf("不同意之后的节点没有阻断：%+v", next)
	}
}

// TestPathConfigServiceGetReportsAffectedAfterTargetOptionChange 验证表单选项变化不会污染节点配置状态。
func TestPathConfigServiceGetReportsAffectedAfterTargetOptionChange(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration}}
	reader := &pathConfigReader{snapshot: target.PathConfigurationSnapshot{Tree: pathConfigTree(), EntryNodeIDs: []string{"start"}, FormFields: pathConfigFields()}}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}}}
	configs := &memoryPathConfigRepository{}
	serviceUnderTest := newPathConfigService(t, plans, reader, paths, configs)
	if _, err := serviceUnderTest.Save(context.Background(), 7, 32, "123e4567-e89b-12d3-a456-426614174507", 0, nil, nil); err != nil {
		t.Fatalf("首次保存失败：%v", err)
	}
	// 目标平台把“类型”字段选项从 a/b 收缩为只剩 b，已保存的 a 变成失效值。
	changedFields := pathConfigFields()
	for index := range changedFields {
		if changedFields[index].EnglishName == "type" {
			changedFields[index].Options = []target.FormFieldOption{{Label: "B", Value: "b"}}
		}
	}
	reader.snapshot.FormFields = changedFields
	configuration, err := serviceUnderTest.Get(context.Background(), 7, 32)
	if err != nil {
		t.Fatalf("目标变化后重新读取配置失败：%v", err)
	}
	if configuration.Status == "affected" {
		t.Fatalf("表单选项变化错误污染节点配置状态：%s", configuration.Status)
	}
	approval := findConfigNode(configuration.Groups, "财务审批")
	if len(approval.Fields) != 0 || len(approval.Gaps) != 0 {
		t.Fatalf("表单字段变化错误进入节点配置：%+v", approval)
	}
}

// TestPathConfigServiceRejectsLooseDateText 验证日期与日期时间由独立表单保存接口严格校验。
func TestPathConfigServiceRejectsLooseDateText(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration, Account: "account-a", FlowSource: "new", TargetObjectID: "template-a"}}
	snapshot := pathConfigWorkspaceSnapshot()
	snapshot.Tree.FieldPowers = append(snapshot.Tree.FieldPowers,
		target.FlowNodeFieldPower{FormID: "form-a", FieldID: "field-date", EnglishName: "date", Power: "edit"},
		target.FlowNodeFieldPower{FormID: "form-a", FieldID: "field-datetime", EnglishName: "datetime", Power: "edit"},
	)
	snapshot.Forms[0].TemplateData = `{"list":[{"type":"number","model":"amount","name":"申请金额","options":{"required":true}},{"type":"select","model":"type","name":"类型","options":{"required":true,"options":[{"label":"A","value":"a"}] }},{"type":"input","model":"note","name":"备注","options":{}},{"type":"date","model":"date","name":"日期","options":{"required":true,"type":"date"}},{"type":"date","model":"datetime","name":"日期时间","options":{"required":true,"type":"datetime"}}],"config":{}}`
	reader := &pathConfigReader{snapshot: snapshot}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}}}
	configs := &memoryPathConfigRepository{}
	serviceUnderTest := newPathConfigService(t, plans, reader, paths, configs)
	valid := model.PathFormSaveInput{Validated: true, Values: map[string]any{
		"amount": float64(2500), "type": "a", "note": "备注内容", "date": "2026-08-07", "datetime": "2026-08-07 10:20:30",
	}}
	if _, err := serviceUnderTest.SaveForm(context.Background(), 7, 32, "123e4567-e89b-12d3-a456-426614174509", valid); err != nil {
		t.Fatalf("严格格式合法日期保存失败：%v", err)
	}
	invalid := valid
	invalid.Revision = 1
	invalid.Values = make(map[string]any, len(valid.Values))
	for key, value := range valid.Values {
		invalid.Values[key] = value
	}
	invalid.Values["datetime"] = "2026/08/07 10:20"
	if _, err := serviceUnderTest.SaveForm(context.Background(), 7, 32, "123e4567-e89b-12d3-a456-426614174510", invalid); !service.IsPathConfigErrorKind(err, service.PathConfigErrorInvalid) {
		t.Fatalf("任意日期时间文本没有被拒绝：%v", err)
	}
}

// TestPathConfigServiceValidatesAndStoresTemplatePersonSelection 验证人员候选必须属于当前模板并复用既有动作 JSON 按节点隔离保存。
func TestPathConfigServiceValidatesAndStoresTemplatePersonSelection(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration}}
	tree := pathConfigTree()
	tree.Child.ConditionNodes[0].Child.AuditConfig = &target.FlowNodeAuditConfig{
		AuditType: "run_node_choose", Mode: "scramble",
		Candidates: []target.FlowAuditCandidate{{ID: "person-1", Name: "张三"}, {ID: "person-2", Name: "李四"}},
	}
	reader := &pathConfigReader{snapshot: target.PathConfigurationSnapshot{Tree: tree, EntryNodeIDs: []string{"start"}, FormFields: pathConfigFields()}}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}}}
	configs := &memoryPathConfigRepository{}
	serviceUnderTest := newPathConfigService(t, plans, reader, paths, configs)
	personKey := analyzer.PathConfigPersonToken("approve-a")
	invalid := []model.PathConfigActionValue{{Key: personKey, Action: `["forged-person"]`}}
	if _, err := serviceUnderTest.Save(context.Background(), 7, 32, "123e4567-e89b-12d3-a456-426614174511", 0, nil, invalid); !service.IsPathConfigErrorKind(err, service.PathConfigErrorInvalid) {
		t.Fatalf("模板外人员候选没有被拒绝：%v", err)
	}
	selectedToken := analyzer.PathConfigPersonOptionToken("approve-a", "person-1")
	valid := []model.PathConfigActionValue{{Key: personKey, Action: `["` + selectedToken + `"]`}}
	if _, err := serviceUnderTest.Save(context.Background(), 7, 32, "123e4567-e89b-12d3-a456-426614174512", 0, nil, valid); err != nil {
		t.Fatalf("合法人员候选保存失败：%v", err)
	}
	stored := configs.records[32].ActionValues[analyzer.PathConfigPersonStorageKey("approve-a")]
	if stored != `["person-1"]` || strings.Contains(stored, selectedToken) {
		t.Fatalf("人员选择没有转换为路径隔离的内部值：%s", stored)
	}
}

// TestPathConfigServiceValidatesOptionalPersonMinimum 验证可跳过节点仅允许零选择或一次满足模板最低人数。
func TestPathConfigServiceValidatesOptionalPersonMinimum(t *testing.T) {
	tests := []struct {
		name        string
		isSkip      bool
		selectedIDs []string
		wantInvalid bool
	}{
		{name: "可跳过零选择", isSkip: true, selectedIDs: []string{}},
		{name: "可跳过但一人不足", isSkip: true, selectedIDs: []string{"person-1"}, wantInvalid: true},
		{name: "可跳过且两人满足", isSkip: true, selectedIDs: []string{"person-1", "person-2"}},
		{name: "必选零选择仍拒绝", isSkip: false, selectedIDs: []string{}, wantInvalid: true},
	}
	for index, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			plans := newMemoryPlanRepository()
			plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration}}
			tree := pathConfigTree()
			tree.Child.ConditionNodes[0].Child.IsSkip = &testCase.isSkip
			tree.Child.ConditionNodes[0].Child.AuditConfig = &target.FlowNodeAuditConfig{
				AuditType: "run_node_choose", Mode: "countersign", CountersignNum: intPointer(2),
				Candidates: []target.FlowAuditCandidate{{ID: "person-1", Name: "张三"}, {ID: "person-2", Name: "李四"}, {ID: "person-3", Name: "王五"}},
			}
			reader := &pathConfigReader{snapshot: target.PathConfigurationSnapshot{Tree: tree, EntryNodeIDs: []string{"start"}, FormFields: pathConfigFields()}}
			paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}}}
			serviceUnderTest := newPathConfigService(t, plans, reader, paths, &memoryPathConfigRepository{})
			selectedTokens := make([]string, 0, len(testCase.selectedIDs))
			for _, personID := range testCase.selectedIDs {
				selectedTokens = append(selectedTokens, analyzer.PathConfigPersonOptionToken("approve-a", personID))
			}
			encoded, err := json.Marshal(selectedTokens)
			if err != nil {
				t.Fatalf("人员候选测试数据无法编码：%v", err)
			}
			actions := []model.PathConfigActionValue{{Key: analyzer.PathConfigPersonToken("approve-a"), Action: string(encoded)}}
			_, saveErr := serviceUnderTest.Save(context.Background(), 7, 32, fmt.Sprintf("123e4567-e89b-12d3-a456-4266141746%02d", index), 0, nil, actions)
			if testCase.wantInvalid && !service.IsPathConfigErrorKind(saveErr, service.PathConfigErrorInvalid) {
				t.Fatalf("不满足最低人数的选择没有被拒绝：%v", saveErr)
			}
			if !testCase.wantInvalid && saveErr != nil {
				t.Fatalf("合法人员选择保存失败：%v", saveErr)
			}
		})
	}
}

// TestPathConfigServiceMarksLegacyConfigWithoutRequiredPersonAffected 验证旧配置缺少新证实的人员选择时要求重新确认。
func TestPathConfigServiceMarksLegacyConfigWithoutRequiredPersonAffected(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration}}
	tree := pathConfigTree()
	tree.Child.ConditionNodes[0].Child.AuditConfig = &target.FlowNodeAuditConfig{
		AuditType: "run_node_choose", Mode: "scramble",
		Candidates: []target.FlowAuditCandidate{{ID: "person-1", Name: "张三"}},
	}
	reader := &pathConfigReader{snapshot: target.PathConfigurationSnapshot{Tree: tree, EntryNodeIDs: []string{"start"}, FormFields: pathConfigFields()}}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}}}
	configs := &memoryPathConfigRepository{records: map[uint64]model.StoredPathConfig{32: {
		PathID: 32, Revision: 1, Status: "configured",
		FieldValues: map[string]map[string]string{
			"approve-a": {"amount": "2500", "type": `"a"`},
			"approve-b": {"note": `"备注内容"`},
		},
		ActionValues: map[string]string{"approve-a": "agree", "approve-b": "agree"},
	}}}
	configuration, err := newPathConfigService(t, plans, reader, paths, configs).Get(context.Background(), 7, 32)
	if err != nil {
		t.Fatalf("读取旧配置失败：%v", err)
	}
	approval := findConfigNode(configuration.Groups, "财务审批")
	if configuration.Status != "affected" || approval == nil || approval.Status != "affected" || !approval.Persons[0].Affected || !strings.Contains(approval.Persons[0].Note, "旧配置") {
		t.Fatalf("旧配置缺少人员选择没有标记重新确认：configuration=%+v node=%+v", configuration, approval)
	}
}

// TestPathConfigServiceRejectsLockedPlan 验证计划进入运行态后禁止继续保存配置。
func TestPathConfigServiceRejectsLockedPlan(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusRunning}}
	reader := &pathConfigReader{snapshot: target.PathConfigurationSnapshot{Tree: pathConfigTree(), EntryNodeIDs: []string{"start"}, FormFields: pathConfigFields()}}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, SequenceNo: 1, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}}}
	serviceUnderTest := newPathConfigService(t, plans, reader, paths, &memoryPathConfigRepository{})
	_, err := serviceUnderTest.Save(context.Background(), 7, 32, "123e4567-e89b-12d3-a456-426614174506", 0, validPathConfigSubmission(), nil)
	if !service.IsPathConfigErrorKind(err, service.PathConfigErrorLocked) {
		t.Fatalf("运行态计划没有被锁定：%v", err)
	}
}

// validPathConfigSubmission 返回当前测试树所有可编辑字段的合法提交值。
func validPathConfigSubmission() []model.PathConfigFieldValue {
	return []model.PathConfigFieldValue{
		{Key: analyzer.PathConfigFieldToken("approve-a", "amount"), Value: "2500"},
		{Key: analyzer.PathConfigFieldToken("approve-a", "type"), Value: "\"a\""},
		{Key: analyzer.PathConfigFieldToken("approve-b", "note"), Value: "\"备注内容\""},
	}
}
