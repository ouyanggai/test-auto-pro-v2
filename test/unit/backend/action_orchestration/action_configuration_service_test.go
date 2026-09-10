package action_orchestration_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	"test-auto-pro-v2/internal/service"
)

type actionPlanRepository struct {
	repository.PlanRepository
	plan model.Plan
}

// Get 返回动作配置测试使用的计划事实。
func (r actionPlanRepository) Get(_ context.Context, id uint64) (model.Plan, error) {
	if id != r.plan.ID {
		return model.Plan{}, repository.ErrPlanNotFound
	}
	return r.plan, nil
}

type actionPathRepository struct {
	repository.ExecutionPathRepository
	path model.ExecutionPath
}

// Get 返回动作配置测试使用的完整执行路径。
func (r actionPathRepository) Get(_ context.Context, planID, pathID uint64) (model.ExecutionPath, error) {
	if planID != r.path.PlanID || pathID != r.path.ID {
		return model.ExecutionPath{}, repository.ErrExecutionPathNotFound
	}
	return r.path, nil
}

type actionTargetReader struct {
	snapshot target.PathConfigurationSnapshot
	reads    int
}

// PathConfigurationSnapshot 返回动作编译所需的目标真实流程树，不执行目标写操作。
func (r *actionTargetReader) PathConfigurationSnapshot(context.Context, string, string, string) (target.PathConfigurationSnapshot, error) {
	r.reads++
	return r.snapshot, nil
}

type actionHistoryStore struct {
	repository.HistoryReplayStore
	record               repository.HistoryPathConfigRecord
	found                bool
	writes               int
	conflicts            int
	conflictActions      []byte
	lastExpectedRevision uint64
}

// GetPathConfig 返回当前路径的动作领域配置记录。
func (s *actionHistoryStore) GetPathConfig(_ context.Context, pathID uint64) (repository.HistoryPathConfigRecord, bool, error) {
	if !s.found || s.record.PathID != pathID {
		return repository.HistoryPathConfigRecord{}, false, nil
	}
	return s.record, true, nil
}

// SavePathConfig 保存动作场景并执行路径修订和幂等屏障。
func (s *actionHistoryStore) SavePathConfig(_ context.Context, record repository.HistoryPathConfigRecord, expectedRevision uint64, now time.Time) (repository.HistoryPathConfigRecord, error) {
	s.lastExpectedRevision = expectedRevision
	if s.conflicts > 0 {
		s.conflicts--
		s.record.Revision++
		if len(s.conflictActions) > 0 {
			s.record.UserActions = append([]byte(nil), s.conflictActions...)
		}
		return repository.HistoryPathConfigRecord{}, repository.ErrHistoryPathConfigConflict
	}
	if s.found && s.record.IdempotencyKey == record.IdempotencyKey {
		return s.record, nil
	}
	if (!s.found && expectedRevision != 0) || (s.found && expectedRevision != s.record.Revision) {
		return repository.HistoryPathConfigRecord{}, repository.ErrHistoryPathConfigConflict
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	record.Revision = s.record.Revision + 1
	if record.NodeRevision == 0 {
		record.NodeRevision = s.record.NodeRevision + 1
	}
	record.CreatedAt, record.UpdatedAt = s.record.CreatedAt, now.UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now.UTC()
	}
	s.record, s.found, s.writes = record, true, s.writes+1
	return record, nil
}

// TestSaveActionConfigurationDoesNotRequireClientRevision 验证第一版节点动作保存始终合并服务端最新配置，
// 表单写入导致整体修订领先于节点修订时也不能阻止用户新增动作。
func TestSaveActionConfigurationDoesNotRequireClientRevision(t *testing.T) {
	plan := model.Plan{ID: 806, Account: "account-a", FlowSource: "new", TargetObjectID: "flow-a", Status: model.PlanStatusNotStarted}
	path := model.ExecutionPath{ID: 816, PlanID: plan.ID, SequenceNo: 1, Name: "审批路径"}
	reader := &actionTargetReader{snapshot: target.PathConfigurationSnapshot{Tree: actionConfigurationTree(), EntryNodeIDs: []string{"start"}, FlowCode: "flow-a", FlowName: "审批流程", RenderType: target.FormRenderTypeFormMaking}}
	startKey := analyzer.PathConfigNodeToken("start")
	storedActions, err := json.Marshal([]model.ConfiguredAction{{
		Key: "draft-1", Action: model.ActionSaveDraft, Scope: model.ActionScopeInitiator, NodeKey: startKey, Order: 1, Revision: 1,
	}})
	if err != nil {
		t.Fatalf("构造已保存动作失败：%v", err)
	}
	store := &actionHistoryStore{found: true, record: repository.HistoryPathConfigRecord{
		PathID: path.ID, Revision: 2, NodeRevision: 1, DataRevision: 3, ActionRevision: 1,
		UserActions: storedActions, EffectiveFormData: []byte(`{"title":"已保存表单"}`),
	}}
	config := service.NewPathConfigService(service.NewPlanService(actionPlanRepository{plan: plan}), reader,
		analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathConfigAnalyzer(), actionPathRepository{path: path})
	config.SetHistoryWorkspaceStores(store, store)

	result, err := config.SaveActionConfiguration(context.Background(), plan.ID, path.ID, startKey, "123e4567-e89b-12d3-a456-426614174806", model.ActionConfigurationInput{
		Actions: []model.ConfiguredAction{
			{Key: "draft-1", Action: model.ActionSaveDraft, Scope: model.ActionScopeInitiator, NodeKey: startKey, Order: 1},
			{Key: "submit-1", Action: model.ActionSubmit, Scope: model.ActionScopeInitiator, NodeKey: startKey, Order: 2},
		},
	})
	if err != nil {
		t.Fatalf("整体修订领先时新增提交动作仍失败：%v", err)
	}
	if store.lastExpectedRevision != 2 {
		t.Fatalf("落库没有使用服务端最新整体修订：got=%d want=2", store.lastExpectedRevision)
	}
	if result.Revision != 3 || result.NodeRevision != 2 || len(result.Actions) != 2 {
		t.Fatalf("动作保存结果不完整：%+v", result)
	}
	if string(store.record.EffectiveFormData) != `{"title":"已保存表单"}` {
		t.Fatalf("节点保存覆盖了表单数据：%s", store.record.EffectiveFormData)
	}
}

// TestSaveActionConfigurationRetriesInternalConflict 验证服务端写入前发生并发变化时会重新读取并合并，
// 不把内部整体修订冲突作为 CONFIG_REVISION_CONFLICT 暴露给第一版页面。
func TestSaveActionConfigurationRetriesInternalConflict(t *testing.T) {
	plan := model.Plan{ID: 807, Account: "account-a", FlowSource: "new", TargetObjectID: "flow-a", Status: model.PlanStatusNotStarted}
	path := model.ExecutionPath{ID: 817, PlanID: plan.ID, SequenceNo: 1, Name: "审批路径"}
	reviewKey := analyzer.PathConfigNodeToken("review")
	concurrentActions, err := json.Marshal([]model.ConfiguredAction{{Key: "approve-1", Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: reviewKey, Order: 1, Revision: 1}})
	if err != nil {
		t.Fatalf("构造并发动作失败：%v", err)
	}
	store := &actionHistoryStore{found: true, conflicts: 1, conflictActions: concurrentActions, record: repository.HistoryPathConfigRecord{PathID: path.ID, Revision: 2, NodeRevision: 1}}
	config := service.NewPathConfigService(service.NewPlanService(actionPlanRepository{plan: plan}),
		&actionTargetReader{snapshot: target.PathConfigurationSnapshot{Tree: actionConfigurationTree(), EntryNodeIDs: []string{"start"}, FlowCode: "flow-a", FlowName: "审批流程", RenderType: target.FormRenderTypeFormMaking}},
		analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathConfigAnalyzer(), actionPathRepository{path: path})
	config.SetHistoryWorkspaceStores(store, store)

	result, err := config.SaveActionConfiguration(context.Background(), plan.ID, path.ID, analyzer.PathConfigNodeToken("start"), "123e4567-e89b-12d3-a456-426614174807", model.ActionConfigurationInput{
		Actions: []model.ConfiguredAction{{Key: "submit-1", Action: model.ActionSubmit, Scope: model.ActionScopeInitiator, Order: 1}},
	})
	if err != nil {
		t.Fatalf("内部并发冲突没有由服务端重新合并：%v", err)
	}
	if store.conflicts != 0 || store.writes != 1 || store.lastExpectedRevision != 3 || result.Revision != 4 || len(result.Actions) != 2 {
		t.Fatalf("内部冲突重试结果不正确：conflicts=%d writes=%d expected=%d result=%+v", store.conflicts, store.writes, store.lastExpectedRevision, result)
	}
	if result.Actions[0].Action != model.ActionSubmit || result.Actions[1].Action != model.ActionApprove {
		t.Fatalf("重新合并覆盖了并发保存的其他节点动作：%+v", result.Actions)
	}
}

// GetPathSource 表示动作配置测试没有路径级历史来源覆盖。
func (s *actionHistoryStore) GetPathSource(context.Context, uint64) (repository.HistoryPathSourceRecord, bool, error) {
	return repository.HistoryPathSourceRecord{}, false, nil
}

// GetDefault 表示动作配置测试没有计划默认历史来源，动作保存不依赖表单快照。
func (s *actionHistoryStore) GetDefault(context.Context, uint64) (repository.HistoryDefaultRecord, bool, error) {
	return repository.HistoryDefaultRecord{}, false, nil
}

// actionConfigurationTree 创建发起、人工审批和结束节点的最小真实流程。
func actionConfigurationTree() *target.FlowNodeTemplate {
	return &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{
		ID: "review", Type: "common", AuditConfig: &target.FlowNodeAuditConfig{AuditType: "appoint", Mode: "all"},
		AddSignCandidates: []target.FlowAuditCandidate{{ID: "user-a", Name: "用户 A"}},
		Child:             &target.FlowNodeTemplate{ID: "end", Type: "end"},
	}}
}

// TestSaveActionConfigurationPersistsCompiledScenario 验证动作保存写入独立领域列并可重读完整预览。
func TestSaveActionConfigurationPersistsCompiledScenario(t *testing.T) {
	plan := model.Plan{ID: 801, Account: "account-a", FlowSource: "new", TargetObjectID: "flow-a", Status: model.PlanStatusNotStarted}
	path := model.ExecutionPath{ID: 811, PlanID: plan.ID, SequenceNo: 1, Name: "审批路径"}
	reader := &actionTargetReader{snapshot: target.PathConfigurationSnapshot{Tree: actionConfigurationTree(), EntryNodeIDs: []string{"start"}, FlowCode: "flow-a", FlowName: "审批流程", RenderType: target.FormRenderTypeFormMaking}}
	store := &actionHistoryStore{record: repository.HistoryPathConfigRecord{PathID: path.ID, Issues: []byte(`[{"code":"runtime_validation","message":"表单需要复核","blocking":true}]`)}}
	store.found = true
	config := service.NewPathConfigService(service.NewPlanService(actionPlanRepository{plan: plan}), reader,
		analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathConfigAnalyzer(), actionPathRepository{path: path})
	config.SetHistoryWorkspaceStores(store, store)
	reviewKey := analyzer.PathConfigNodeToken("review")
	result, err := config.SaveActionConfiguration(context.Background(), plan.ID, path.ID, reviewKey, "123e4567-e89b-12d3-a456-426614174801", model.ActionConfigurationInput{
		Actions: []model.ConfiguredAction{{Key: "approve-1", Action: model.ActionApprove, Scope: model.ActionScopeTask, Order: 1}},
	})
	if err != nil {
		t.Fatalf("保存动作场景失败：%v", err)
	}
	if store.writes != 1 || result.ActionRevision != 1 || len(result.Actions) != 1 || len(result.CompiledScenario) < 2 {
		t.Fatalf("动作场景保存结果不完整：writes=%d result=%+v", store.writes, result)
	}
	if result.Actions[0].NodeKey == reviewKey {
	} else {
		t.Fatalf("动作没有绑定语义节点：%+v", result.Actions[0])
	}
	if len(result.CompiledScenario) >= 2 && result.CompiledScenario[0].Source == model.ActionStepSourceSystemDefault && result.CompiledScenario[0].Action == model.ActionSubmit && result.CompiledScenario[1].Source == model.ActionStepSourceSystemDefault && result.CompiledScenario[1].Action == model.ActionApprove {
	} else {
		t.Fatalf("显式同意未归一化为固定尾动作：%+v", result.CompiledScenario)
	}
	if result.Actions[0].Revision != result.ActionRevision {
		t.Fatalf("动作没有绑定当前动作配置修订：action=%d revision=%d", result.Actions[0].Revision, result.ActionRevision)
	}
	if !strings.Contains(string(store.record.Issues), `"code":"runtime_validation"`) || !strings.Contains(string(store.record.Issues), `"message":"表单需要复核"`) {
		t.Fatalf("保存动作不应丢失既有表单问题：%s", store.record.Issues)
	}
	last := result.CompiledScenario[len(result.CompiledScenario)-1]
	if last.Source == model.ActionStepSourceNavigation && last.Action == model.ActionSystemAutomatic {
	} else {
		t.Fatalf("缺少末端系统导航步骤：%+v", last)
	}
	preview, err := config.GetCompiledScenario(context.Background(), plan.ID, path.ID)
	if err != nil || len(preview.CompiledScenario) != len(result.CompiledScenario) || preview.Actions[0].Key != "approve-1" {
		t.Fatalf("动作场景重读不一致：err=%v preview=%+v", err, preview)
	}
	if reader.reads < 2 {
		t.Fatal("动作保存和预览没有重读目标流程事实")
	}
	retry, err := config.SaveActionConfiguration(context.Background(), plan.ID, path.ID, reviewKey, "123e4567-e89b-12d3-a456-426614174801", model.ActionConfigurationInput{
		Actions: []model.ConfiguredAction{{Key: "approve-1", Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: reviewKey, Order: 1}},
	})
	if err != nil || retry.Revision != result.Revision || store.writes != 1 {
		t.Fatalf("相同幂等键重试未复用原结果：err=%v retry=%+v writes=%d", err, retry, store.writes)
	}
	_, err = config.SaveActionConfiguration(context.Background(), plan.ID, path.ID, reviewKey, "123e4567-e89b-12d3-a456-426614174801", model.ActionConfigurationInput{
		Actions: []model.ConfiguredAction{{Key: "approve-1", Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: reviewKey, Order: 1, Note: "changed"}},
	})
	if err == nil {
		t.Fatal("相同幂等键提交不同动作正文却未阻断")
	}
}

// TestSaveActionConfigurationDeleteRemovesStoredActions 复现交付缺口：删除动作→保存→重读，被删动作必须消失。
func TestSaveActionConfigurationDeleteRemovesStoredActions(t *testing.T) {
	plan := model.Plan{ID: 805, Account: "account-a", FlowSource: "new", TargetObjectID: "flow-a", Status: model.PlanStatusNotStarted}
	path := model.ExecutionPath{ID: 815, PlanID: plan.ID, SequenceNo: 1, Name: "审批路径"}
	reader := &actionTargetReader{snapshot: target.PathConfigurationSnapshot{Tree: actionConfigurationTree(), EntryNodeIDs: []string{"start"}, FlowCode: "flow-a", FlowName: "审批流程", RenderType: target.FormRenderTypeFormMaking}}
	store := &actionHistoryStore{}
	config := service.NewPathConfigService(service.NewPlanService(actionPlanRepository{plan: plan}), reader,
		analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathConfigAnalyzer(), actionPathRepository{path: path})
	config.SetHistoryWorkspaceStores(store, store)
	reviewKey := analyzer.PathConfigNodeToken("review")
	personKey := analyzer.PathConfigPersonToken("review:add_sign")
	personToken := analyzer.PathConfigPersonOptionToken("review:add_sign", "user-a")
	persons := []model.PathConfigPersonStrategyInput{{Key: personKey, Strategy: "manual", Seed: 1, Selected: []string{personToken}}}
	first, err := config.SaveActionConfiguration(context.Background(), plan.ID, path.ID, reviewKey, "123e4567-e89b-12d3-a456-426614174805", model.ActionConfigurationInput{
		Persons: persons,
		Actions: []model.ConfiguredAction{
			{Key: "approve-1", Action: model.ActionApprove, Scope: model.ActionScopeTask, Order: 1},
			{Key: "sign-1", Action: model.ActionAddSign, Scope: model.ActionScopeTask, NodeKey: reviewKey, Order: 2, ActorPolicy: "manual"},
		},
	})
	if err != nil {
		t.Fatalf("首次保存两条动作失败：%v", err)
	}
	// 删除 approve-1：模拟用户在编辑器里删掉第一条后整节点保存，只剩 sign-1。
	second, err := config.SaveActionConfiguration(context.Background(), plan.ID, path.ID, reviewKey, "123e4567-e89b-12d3-a456-426614174806", model.ActionConfigurationInput{
		Persons: persons,
		Actions: []model.ConfiguredAction{{Key: "sign-1", Action: model.ActionAddSign, Scope: model.ActionScopeTask, NodeKey: reviewKey, Order: 1, ActorPolicy: "manual"}},
	})
	if err != nil {
		t.Fatalf("删除动作后的保存失败：%v", err)
	}
	reloaded, err := config.GetCompiledScenario(context.Background(), plan.ID, path.ID)
	if err != nil {
		t.Fatalf("保存后重读失败：%v", err)
	}
	if len(reloaded.Actions) != 1 || reloaded.Actions[0].Key != "sign-1" {
		t.Fatalf("被删除的动作在重读后仍然存在：%+v", reloaded.Actions)
	}
	if second.ActionRevision <= first.ActionRevision {
		t.Fatalf("删除保存没有推进动作修订：%d -> %d", first.ActionRevision, second.ActionRevision)
	}
	// 全部删空：节点动作清空必须同样可保存、可重读。
	_, err = config.SaveActionConfiguration(context.Background(), plan.ID, path.ID, reviewKey, "123e4567-e89b-12d3-a456-426614174807", model.ActionConfigurationInput{
		Actions: []model.ConfiguredAction{},
	})
	if err != nil {
		t.Fatalf("清空节点动作保存失败：%v", err)
	}
	emptied, err := config.GetCompiledScenario(context.Background(), plan.ID, path.ID)
	if err != nil {
		t.Fatalf("清空后重读失败：%v", err)
	}
	if len(emptied.Actions) != 0 {
		t.Fatalf("节点动作清空保存后仍返回动作：%+v", emptied.Actions)
	}
}

// TestSaveActionConfigurationRetainsActionPersonStrategy 验证动作私有人员策略与动作场景在同一次保存中持久化。
func TestSaveActionConfigurationRetainsActionPersonStrategy(t *testing.T) {
	plan := model.Plan{ID: 802, Account: "account-a", FlowSource: "new", TargetObjectID: "flow-a", Status: model.PlanStatusNotStarted}
	path := model.ExecutionPath{ID: 812, PlanID: plan.ID, SequenceNo: 1, Name: "审批路径"}
	reader := &actionTargetReader{snapshot: target.PathConfigurationSnapshot{Tree: actionConfigurationTree(), EntryNodeIDs: []string{"start"}, FlowCode: "flow-a", FlowName: "审批流程", RenderType: target.FormRenderTypeFormMaking}}
	store := &actionHistoryStore{}
	config := service.NewPathConfigService(service.NewPlanService(actionPlanRepository{plan: plan}), reader,
		analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathConfigAnalyzer(), actionPathRepository{path: path})
	config.SetHistoryWorkspaceStores(store, store)
	reviewKey := analyzer.PathConfigNodeToken("review")
	personKey := analyzer.PathConfigPersonToken("review:add_sign")
	personToken := analyzer.PathConfigPersonOptionToken("review:add_sign", "user-a")
	_, err := config.SaveActionConfiguration(context.Background(), plan.ID, path.ID, reviewKey, "123e4567-e89b-12d3-a456-426614174802", model.ActionConfigurationInput{
		Persons: []model.PathConfigPersonStrategyInput{{Key: personKey, Strategy: "manual", Seed: 1, Selected: []string{personToken}}},
		Actions: []model.ConfiguredAction{{Key: "sign-1", Action: model.ActionAddSign, Scope: model.ActionScopeTask, NodeKey: reviewKey, Order: 1, ActorPolicy: "manual"}},
	})
	if err != nil {
		t.Fatalf("保存动作私有人员失败：%v", err)
	}
	if !strings.Contains(string(store.record.PersonStrategies), personKey) || !strings.Contains(string(store.record.PersonStrategies), personToken) {
		t.Fatalf("动作私有人员策略未持久化：%s", store.record.PersonStrategies)
	}
}

// TestGetPathConfigurationProjectsActionPersonStrategy 验证刷新路径配置会从 F-012 独立列恢复动作人员和参数。
func TestGetPathConfigurationProjectsActionPersonStrategy(t *testing.T) {
	plan := model.Plan{ID: 803, Account: "account-a", FlowSource: "new", TargetObjectID: "flow-a", Status: model.PlanStatusNotStarted}
	path := model.ExecutionPath{ID: 813, PlanID: plan.ID, SequenceNo: 1, Name: "审批路径"}
	reviewKey := analyzer.PathConfigNodeToken("review")
	personKey := analyzer.PathConfigPersonToken("review:add_sign")
	personToken := analyzer.PathConfigPersonOptionToken("review:add_sign", "user-a")
	actions := `[{
"key":"sign-1","action":"add_sign","scope":"task","nodeKey":"` + reviewKey + `","order":1,"actorPolicy":"manual","parameters":{"remark":"保留"}
}]`
	persons := `{"` + personKey + `":{"key":"` + personKey + `","strategy":"manual","seed":1,"selected":["` + personToken + `"]}}`
	store := &actionHistoryStore{found: true, record: repository.HistoryPathConfigRecord{PathID: path.ID, Revision: 2, NodeRevision: 2, ActionRevision: 1, UserActions: []byte(actions), PersonStrategies: []byte(persons)}}
	config := service.NewPathConfigService(service.NewPlanService(actionPlanRepository{plan: plan}),
		&actionTargetReader{snapshot: target.PathConfigurationSnapshot{Tree: actionConfigurationTree(), EntryNodeIDs: []string{"start"}, FlowCode: "flow-a", FlowName: "审批流程", RenderType: target.FormRenderTypeFormMaking}},
		analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathConfigAnalyzer(), actionPathRepository{path: path})
	config.SetHistoryWorkspaceStores(store, store)
	configuration, err := config.Get(context.Background(), plan.ID, path.ID)
	if err != nil {
		t.Fatalf("读取动作配置失败：%v", err)
	}
	var review *model.PathConfigNode
	for groupIndex := range configuration.Groups {
		for nodeIndex := range configuration.Groups[groupIndex].Nodes {
			if configuration.Groups[groupIndex].Nodes[nodeIndex].Key == reviewKey {
				review = &configuration.Groups[groupIndex].Nodes[nodeIndex]
			}
		}
	}
	if review == nil || len(review.ActionConfiguration.Actions) != 1 {
		t.Fatalf("刷新后动作没有投影到语义节点：%+v", configuration.Groups)
	}
	action := review.ActionConfiguration.Actions[0]
	if action.Person == nil || action.Person.Key != personKey || len(action.Person.Selected) != 1 || action.Person.Selected[0] != personToken || action.Parameters["remark"] != "保留" {
		t.Fatalf("刷新后动作人员或参数丢失：%+v", action)
	}
}

// selectableApproverTree 创建含静态自选审批人的最小流程，用于验证人员策略回显。
func selectableApproverTree() *target.FlowNodeTemplate {
	return &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{
		ID: "review", Type: "common", AuditConfig: &target.FlowNodeAuditConfig{
			AuditType: "run_node_choose", Mode: "scramble",
			Candidates: []target.FlowAuditCandidate{{ID: "user-a", Name: "用户 A"}, {ID: "user-b", Name: "用户 B"}},
		},
		Child: &target.FlowNodeTemplate{ID: "end", Type: "end"},
	}}
}

// TestGetPathConfigurationRestoresRandomPersonSelection 验证自动配置保存的空选择不会投影为 null 或导致页面初始化失败。
func TestGetPathConfigurationRestoresRandomPersonSelection(t *testing.T) {
	plan := model.Plan{ID: 804, Account: "account-a", FlowSource: "new", TargetObjectID: "flow-a", Status: model.PlanStatusNotStarted}
	path := model.ExecutionPath{ID: 814, PlanID: plan.ID, SequenceNo: 1, Name: "审批路径"}
	personKey := analyzer.PathConfigPersonToken("review")
	personJSON, err := json.Marshal(map[string]model.PathConfigPersonStrategyInput{
		personKey: {Key: personKey, Strategy: "random", Seed: 2, Selected: nil},
	})
	if err != nil {
		t.Fatalf("构造人员策略失败：%v", err)
	}
	store := &actionHistoryStore{found: true, record: repository.HistoryPathConfigRecord{
		PathID: path.ID, Revision: 1, NodeRevision: 1, PersonStrategies: personJSON,
	}}
	config := service.NewPathConfigService(service.NewPlanService(actionPlanRepository{plan: plan}),
		&actionTargetReader{snapshot: target.PathConfigurationSnapshot{Tree: selectableApproverTree(), EntryNodeIDs: []string{"start"}, FlowCode: "flow-a", FlowName: "审批流程", RenderType: target.FormRenderTypeFormMaking}},
		analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathConfigAnalyzer(), actionPathRepository{path: path})
	config.SetHistoryWorkspaceStores(store, store)
	configuration, err := config.Get(context.Background(), plan.ID, path.ID)
	if err != nil {
		t.Fatalf("读取随机人员配置失败：%v", err)
	}
	var selected []string
	for _, group := range configuration.Groups {
		for _, node := range group.Nodes {
			for _, person := range node.Persons {
				if person.Key == personKey {
					selected = person.Selected
				}
			}
		}
	}
	if selected == nil || len(selected) != 1 {
		t.Fatalf("随机人员选择未恢复为非空数组：%v", selected)
	}
	encoded, err := json.Marshal(configuration)
	if err != nil {
		t.Fatalf("编码路径配置失败：%v", err)
	}
	if strings.Contains(string(encoded), `"selected":null`) {
		t.Fatalf("路径配置仍返回 null 人员集合：%s", encoded)
	}
}

// autoConfigureNode 构造一个待配置节点：两个可用动作、一个可编辑人员和一个必填参数动作。
func autoConfigureNode(key string, kinds []string) model.PathConfigNode {
	catalog := make([]model.PathConfigActionCatalogItem, 0, len(kinds)+2)
	for _, kind := range kinds {
		catalog = append(catalog, model.PathConfigActionCatalogItem{Kind: kind, Scope: "task", Label: kind, Enabled: true})
	}
	catalog = append(catalog,
		model.PathConfigActionCatalogItem{Kind: "resubmit", Scope: "initiator", Label: "重新提交", Enabled: true, SystemInserted: true},
		// 需要显式选人的动作不由自动配置替用户挑处理人。
		model.PathConfigActionCatalogItem{Kind: "transfer", Scope: "task", Label: "移交", Enabled: true, RequiresPerson: true},
	)
	return model.PathConfigNode{
		Key: key, Name: key, Kind: "common", Status: "pending",
		ActionConfiguration: model.PathConfigActionConfiguration{Catalog: catalog},
	}
}

// autoInitiatorNode 构造一个发起端待配置节点：全部目录项都是发起端动作，无编译器插入项和选人项。
func autoInitiatorNode(key string, kinds []string) model.PathConfigNode {
	catalog := make([]model.PathConfigActionCatalogItem, 0, len(kinds))
	for _, kind := range kinds {
		catalog = append(catalog, model.PathConfigActionCatalogItem{Kind: kind, Scope: "initiator", Label: kind, Enabled: true})
	}
	return model.PathConfigNode{
		Key: key, Name: key, Kind: "start", Status: "pending",
		ActionConfiguration: model.PathConfigActionConfiguration{Catalog: catalog},
	}
}

// TestAutoNodeActionPrefersUncoveredEnabledActions 验证一键配置的自动动作选择：
// 只取已启用且不需要人员和必填参数的动作，跨节点优先覆盖尚未用过的可流转动作，且结果可复现。
// 覆盖轮转只发生在可流转动作之间；拒绝/暂存等让路径停住的动作不参与自动编排。
func TestAutoNodeActionPrefersUncoveredEnabledActions(t *testing.T) {
	used := map[string]bool{}
	first, ok := service.AutoNodeActionForTest(41, 51, autoInitiatorNode("node-a", []string{"save_draft", "resubmit"}), used)
	if !ok || first.NodeKey != "node-a" {
		t.Fatalf("第一个节点没有自动选出动作：%+v", first)
	}
	used[string(first.Action)] = true
	second, ok := service.AutoNodeActionForTest(41, 51, autoInitiatorNode("node-b", []string{"save_draft", "resubmit"}), used)
	if !ok || second.Action == first.Action {
		t.Fatalf("第二个节点没有优先覆盖尚未用过的动作：first=%s second=%s", first.Action, second.Action)
	}
	for _, action := range []model.ConfiguredAction{first, second} {
		if action.Action == "transfer" {
			t.Fatalf("需要显式选人的动作不应被自动编排：%+v", action)
		}
	}
	repeat, _ := service.AutoNodeActionForTest(41, 51, autoInitiatorNode("node-a", []string{"save_draft", "resubmit"}), map[string]bool{})
	if repeat.Action != first.Action || repeat.Key != first.Key {
		t.Fatalf("同一节点重复自动配置结果不稳定：%+v / %+v", first, repeat)
	}
}

// TestAutoPersonStrategyPrefersRangeRandom 验证一键配置不再把目标默认人员当成工具默认策略。
func TestAutoPersonStrategyPrefersRangeRandom(t *testing.T) {
	person := model.PathConfigPerson{
		Key:             "node-person",
		Required:        true,
		MinCount:        1,
		MaxCount:        1,
		DefaultSelected: []string{"candidate-default"},
		Options: []model.PathConfigPersonOption{
			{Label: "候选甲", Value: "candidate-a"},
			{Label: "候选乙", Value: "candidate-b"},
			{Label: "候选丙", Value: "candidate-c"},
		},
		Strategies: []model.PathConfigPersonStrategyOption{
			{Value: "target_default", Label: "目标默认"},
			{Value: "random", Label: "范围随机"},
			{Value: "manual", Label: "手动选择"},
		},
	}

	first := service.AutoPersonStrategyForTest(person, 2)
	second := service.AutoPersonStrategyForTest(person, 2)
	if first.Strategy != "random" || first.Seed != second.Seed || len(first.Selected) != 1 || first.Selected[0] != second.Selected[0] {
		t.Fatalf("一键配置应稳定随机选择一人，实际 first=%+v second=%+v", first, second)
	}
	if first.Selected[0] == person.DefaultSelected[0] {
		t.Fatalf("一键配置不应沿用目标默认名单：%+v", first)
	}
}

// TestAutoPersonStrategyHonorsCountersignMinimum 验证会签最少人数由节点投影传入，随机策略不会退化为一人。
func TestAutoPersonStrategyHonorsCountersignMinimum(t *testing.T) {
	person := model.PathConfigPerson{
		Key:      "countersign-person",
		Required: true,
		MinCount: 2,
		MaxCount: 3,
		Options: []model.PathConfigPersonOption{
			{Label: "候选甲", Value: "candidate-a"},
			{Label: "候选乙", Value: "candidate-b"},
			{Label: "候选丙", Value: "candidate-c"},
		},
		Strategies: []model.PathConfigPersonStrategyOption{{Value: "random", Label: "范围随机"}},
	}
	result := service.AutoPersonStrategyForTest(person, 7)
	if result.Strategy != "random" || len(result.Selected) != 2 {
		t.Fatalf("会签应按最少人数随机选择：%+v", result)
	}
}

// TestPersonSelectionIssueExplainsExactCount 验证人员数量错误包含当前数量与目标人数，不能退化为模糊提示。
func TestPersonSelectionIssueExplainsExactCount(t *testing.T) {
	if got := analyzer.PathConfigPersonSelectionIssue(true, 2, 3, 0); got != "至少需要选择 2 名处理人员，当前未选择" {
		t.Fatalf("空选人数提示不明确：%q", got)
	}
	if got := analyzer.PathConfigPersonSelectionIssue(true, 2, 3, 1); got != "至少需要选择 2 名处理人员，当前已选择 1 名" {
		t.Fatalf("人数不足提示不明确：%q", got)
	}
	if got := analyzer.PathConfigPersonSelectionIssue(true, 1, 2, 3); got != "最多只能选择 2 名处理人员，当前已选择 3 名" {
		t.Fatalf("人数超限提示不明确：%q", got)
	}
}

// TestAutoConfigurationStillFillsMissingPersonWithExistingAction 验证已有节点动作不会阻止一键补齐该节点遗漏的人员策略。
func TestAutoConfigurationStillFillsMissingPersonWithExistingAction(t *testing.T) {
	person := model.PathConfigPerson{
		Key: "node-person", Editable: true, Required: true, MinCount: 1, MaxCount: 1,
		Options:    []model.PathConfigPersonOption{{Label: "候选甲", Value: "candidate-a"}},
		Strategies: []model.PathConfigPersonStrategyOption{{Value: "random", Label: "范围随机"}},
	}
	result := service.AutoPersonStrategyForTest(person, 11)
	if result.Strategy != "random" || len(result.Selected) != 1 {
		t.Fatalf("已有动作节点补齐人员时仍应使用随机策略：%+v", result)
	}
}

// TestConfirmedNodeKeysCoverSavedActionNodes 锁定节点确认列真的会被写入：
// 这一列原来从来没有人写，节点状态永远停在待配置，一键配置和手工保存都看不到已配置。
func TestConfirmedNodeKeysCoverSavedActionNodes(t *testing.T) {
	raw, err := service.ConfirmedNodeKeysJSONForTest([]byte(`["node-old"]`),
		[]model.ConfiguredAction{{NodeKey: "node-a"}, {NodeKey: "node-a"}, {NodeKey: ""}}, "node-person-only", " ")
	if err != nil {
		t.Fatalf("编码已确认节点失败：%v", err)
	}
	var keys []string
	if err := json.Unmarshal(raw, &keys); err != nil {
		t.Fatalf("已确认节点不是合法 JSON 数组：%v", err)
	}
	if len(keys) != 3 {
		t.Fatalf("已确认节点没有去重或漏掉节点：%v", keys)
	}
	found := map[string]bool{}
	for _, key := range keys {
		found[key] = true
	}
	for _, expected := range []string{"node-old", "node-a", "node-person-only"} {
		if !found[expected] {
			t.Fatalf("已确认节点缺少 %s：%v", expected, keys)
		}
	}
}

// TestAutoNodeActionCandidatesOrderCoversThenSeeds 验证自动动作候选顺序：
// 同一节点重复计算顺序稳定，不可流转的动作（暂存/不同意）不参与候选，
// 需要显式选人和编译器插入的动作不参与。
func TestAutoNodeActionCandidatesOrderCoversThenSeeds(t *testing.T) {
	node := autoConfigureNode("node-a", []string{"approve", "reject", "storage_form_data"})
	first, ok := service.AutoNodeActionForTest(41, 51, node, map[string]bool{})
	if !ok {
		t.Fatal("节点应至少给出一个候选动作")
	}
	if first.Action != "approve" {
		t.Fatalf("审批节点唯一可流转动作是同意，实际选出：%s", first.Action)
	}
	skipped, ok := service.AutoNodeActionForTest(41, 51, node, map[string]bool{string(first.Action): true})
	if !ok || skipped.Action != "approve" {
		t.Fatalf("已覆盖动作没有让位给剩余可流转动作：first=%s next=%s", first.Action, skipped.Action)
	}
	repeat, _ := service.AutoNodeActionForTest(41, 51, node, map[string]bool{})
	if repeat.Action != first.Action {
		t.Fatalf("同一节点候选顺序不稳定：%s / %s", first.Action, repeat.Action)
	}
}

// TestAutoNodeActionSkipsNonAdvancingKinds 验证一键配置只为节点选择能让主实例离开的动作：
// 暂存/取回/回退/不同意/移交/催办/关注即使"尚未覆盖"也不参与自动选择——
// 不同意会把实例退回发起端重走全链而上游动作已消费，其余动作单独配置必然停在当前待办；
// 覆盖动作种类不能替代路径可流转。
func TestAutoNodeActionSkipsNonAdvancingKinds(t *testing.T) {
	node := autoConfigureNode("node-a", []string{"approve", "reject", "storage_form_data", "rollback_previous"})
	nonAdvancing := map[string]bool{
		"reject": true, "storage_form_data": true, "rollback_previous": true, "retrieve": true,
		"transfer": true, "add_sign": true, "urge": true, "forward": true,
		"follow": true, "unfollow": true,
	}
	used := map[string]bool{}
	for {
		first, ok := service.AutoNodeActionForTest(41, 51, node, used)
		if !ok {
			t.Fatal("存在可流转动作时节点应给出候选")
		}
		if nonAdvancing[string(first.Action)] {
			t.Fatalf("一键配置选中了不可流转动作：%s", first.Action)
		}
		if used[string(first.Action)] {
			break
		}
		used[string(first.Action)] = true
		if len(used) >= 8 {
			t.Fatalf("候选轮转发散：%+v", used)
		}
	}
}

// TestAutoNodeActionInitiatorOnlyAdvancing 验证发起节点的自动选择：
// 保存草稿由编译器补提交恢复属于可流转动作；催办/关注不改变主实例状态，不得被选中。
func TestAutoNodeActionInitiatorOnlyAdvancing(t *testing.T) {
	node := model.PathConfigNode{
		Key: "node-init", Name: "发起人", Kind: "start", Status: "pending",
		ActionConfiguration: model.PathConfigActionConfiguration{Catalog: []model.PathConfigActionCatalogItem{
			{Kind: "save_draft", Scope: "initiator", Label: "保存草稿", Enabled: true},
			{Kind: "urge", Scope: "instance", Label: "催办", Enabled: true},
			{Kind: "follow", Scope: "instance", Label: "关注", Enabled: true},
		}},
	}
	first, ok := service.AutoNodeActionForTest(41, 51, node, map[string]bool{})
	if !ok || first.Action != "save_draft" {
		t.Fatalf("发起节点应只选可流转动作：%+v", first)
	}
}

// TestAutoNodeActionUnconfigurableWhenNoAdvancingKind 验证目录里没有可流转动作时不再假装可配置：
// 一键配置应放弃该节点（保持待配置、阻塞运行前检查），而不是选一个必然停住的暂存动作凑数。
func TestAutoNodeActionUnconfigurableWhenNoAdvancingKind(t *testing.T) {
	node := autoConfigureNode("node-only-storage", []string{"storage_form_data", "retrieve"})
	if first, ok := service.AutoNodeActionForTest(41, 51, node, map[string]bool{}); ok {
		t.Fatalf("没有可流转动作时应放弃自动选择，实际选出：%+v", first)
	}
}
