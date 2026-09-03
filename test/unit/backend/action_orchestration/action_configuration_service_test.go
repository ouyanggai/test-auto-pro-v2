package action_orchestration_test

import (
	"context"
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
	record repository.HistoryPathConfigRecord
	found  bool
	writes int
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
	if result.Actions[0].NodeKey != reviewKey || result.CompiledScenario[0].Source != model.ActionStepSourceUser {
		t.Fatalf("动作没有绑定语义节点或用户步骤来源错误：%+v", result)
	}
	if result.Actions[0].Revision != result.ActionRevision {
		t.Fatalf("动作没有绑定当前动作配置修订：action=%d revision=%d", result.Actions[0].Revision, result.ActionRevision)
	}
	if !strings.Contains(string(store.record.Issues), `"code":"runtime_validation"`) || !strings.Contains(string(store.record.Issues), `"message":"表单需要复核"`) {
		t.Fatalf("保存动作不应丢失既有表单问题：%s", store.record.Issues)
	}
	last := result.CompiledScenario[len(result.CompiledScenario)-1]
	if last.Source != model.ActionStepSourceNavigation || last.Action != model.ActionApprove {
		t.Fatalf("缺少最终系统导航步骤：%+v", last)
	}
	preview, err := config.GetCompiledScenario(context.Background(), plan.ID, path.ID)
	if err != nil || len(preview.CompiledScenario) != len(result.CompiledScenario) || preview.Actions[0].Key != "approve-1" {
		t.Fatalf("动作场景重读不一致：err=%v preview=%+v", err, preview)
	}
	if reader.reads < 2 {
		t.Fatal("动作保存和预览没有重读目标流程事实")
	}
	retry, err := config.SaveActionConfiguration(context.Background(), plan.ID, path.ID, reviewKey, "123e4567-e89b-12d3-a456-426614174801", model.ActionConfigurationInput{
		Revision: 0,
		Actions:  []model.ConfiguredAction{{Key: "approve-1", Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: reviewKey, Order: 1}},
	})
	if err != nil || retry.Revision != result.Revision || store.writes != 1 {
		t.Fatalf("相同幂等键重试未复用原结果：err=%v retry=%+v writes=%d", err, retry, store.writes)
	}
	_, err = config.SaveActionConfiguration(context.Background(), plan.ID, path.ID, reviewKey, "123e4567-e89b-12d3-a456-426614174801", model.ActionConfigurationInput{
		Revision: 0,
		Actions:  []model.ConfiguredAction{{Key: "approve-1", Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: reviewKey, Order: 1, Note: "changed"}},
	})
	if err == nil {
		t.Fatal("相同幂等键提交不同动作正文却未阻断")
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
