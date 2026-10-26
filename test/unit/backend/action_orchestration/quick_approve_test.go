package action_orchestration_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	"test-auto-pro-v2/internal/service"
)

// quickApproveTree 使用当前目标支持的指定人员与竞签规则，避免用未知规则掩盖真实阻塞。
func quickApproveTree() *target.FlowNodeTemplate {
	tree := actionConfigurationTree()
	tree.Child.AuditConfig = &target.FlowNodeAuditConfig{AuditType: "assign", Mode: "scramble", Candidates: []target.FlowAuditCandidate{{ID: "user-a", Name: "用户甲"}}}
	return tree
}

// quickApproveFixture 组装只读目标树和真实配置服务，不发送目标提交或审批请求。
func quickApproveFixture(tree *target.FlowNodeTemplate, record repository.HistoryPathConfigRecord) (*service.PathConfigService, *actionHistoryStore, *actionTargetReader, model.Plan, model.ExecutionPath) {
	plan := model.Plan{ID: 1050, Account: "account-a", FlowSource: "new", TargetObjectID: "flow-a", Status: model.PlanStatusNotStarted}
	path := model.ExecutionPath{ID: 1051, PlanID: plan.ID, SequenceNo: 1, Name: "快速同意路径"}
	record.PathID = path.ID
	store := &actionHistoryStore{found: record.Revision > 0, record: record}
	reader := &actionTargetReader{snapshot: target.PathConfigurationSnapshot{Tree: tree, EntryNodeIDs: []string{"start"}, FlowCode: "flow-a", FlowName: "快速同意流程", RenderType: target.FormRenderTypeFormMaking}}
	config := service.NewPathConfigService(service.NewPlanService(actionPlanRepository{plan: plan}), reader,
		analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathConfigAnalyzer(), actionPathRepository{path: path})
	config.SetHistoryWorkspaceStores(store, store)
	return config, store, reader, plan, path
}

// TestQuickApprovePersistsDefaultStepsWithoutExtraActions 锁定纯默认同意没有额外动作也能完整准备，且重复准备不重复写。
func TestQuickApprovePersistsDefaultStepsWithoutExtraActions(t *testing.T) {
	for _, selectable := range []bool{false, true} {
		t.Run(map[bool]string{false: "目标固定人员", true: "工具自选人员"}[selectable], func(t *testing.T) {
			tree := quickApproveTree()
			if selectable {
				tree.Child.AuditConfig = &target.FlowNodeAuditConfig{AuditType: "run_node_choose", Mode: "scramble", Candidates: []target.FlowAuditCandidate{{ID: "user-a", Name: "用户甲"}}}
			}
			config, store, reader, plan, path := quickApproveFixture(tree, repository.HistoryPathConfigRecord{
				Revision: 1, DataRevision: 3, DataStatus: model.HistoryDataStatusBaseReady, EffectiveFormData: []byte(`{"title":"保留业务数据"}`),
			})
			if err := config.AutoConfigurePathActions(context.Background(), plan.ID, path.ID, model.HistoryReplayActionModeApproveOnly); err != nil {
				t.Fatal(err)
			}
			if reader.reads != 1 || store.writes != 1 {
				t.Fatalf("每次准备只读一次目标并写一次配置：reads=%d writes=%d", reader.reads, store.writes)
			}
			var actions []model.ConfiguredAction
			var steps []model.CompiledActionStep
			if err := json.Unmarshal(store.record.UserActions, &actions); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(store.record.CompiledSteps, &steps); err != nil {
				t.Fatal(err)
			}
			if len(actions) != 0 || len(steps) != 3 || steps[0].Action != model.ActionSubmit || steps[1].Action != model.ActionApprove || steps[2].Source != model.ActionStepSourceNavigation {
				t.Fatalf("纯同意场景只应包含提交和同意：actions=%+v steps=%+v", actions, steps)
			}
			if store.record.ConfigStatus != "configured" || store.record.NodeStatus != "configured" {
				t.Fatalf("不能因零额外动作把默认同意判为待配置：%+v", store.record)
			}
			configuration, err := config.Get(context.Background(), plan.ID, path.ID)
			if err != nil || configuration.Status != "configured" || configuration.Progress.Pending != 0 {
				t.Fatalf("重读配置页仍应完成：status=%s progress=%+v err=%v", configuration.Status, configuration.Progress, err)
			}
			path.ConfigurationStatus, path.DataStatus = store.record.NodeStatus, store.record.DataStatus
			readiness := service.EvaluatePathReadiness(service.PathReadinessInput{Path: path, ConfigFound: true, CompiledStepCount: len(steps)})
			if !readiness.Runnable {
				t.Fatalf("已准备的纯同意路径不应被运行前检查拦住：%+v", readiness.Blocks)
			}
			persons := string(store.record.PersonStrategies)
			if err := config.AutoConfigurePathActions(context.Background(), plan.ID, path.ID, model.HistoryReplayActionModeApproveOnly); err != nil {
				t.Fatal(err)
			}
			if store.writes != 1 || string(store.record.PersonStrategies) != persons || string(store.record.EffectiveFormData) != `{"title":"保留业务数据"}` || store.record.DataRevision != 3 {
				t.Fatalf("重复准备必须幂等并保留人员和数据：writes=%d record=%+v", store.writes, store.record)
			}
		})
	}
}

// TestQuickApproveReplacesExtraActionsOnlyAfterExplicitPreparation 锁定快速方案替换额外动作，保留节点主人员与表单。
func TestQuickApproveReplacesExtraActionsOnlyAfterExplicitPreparation(t *testing.T) {
	tree := quickApproveTree()
	tree.Child.AuditConfig = &target.FlowNodeAuditConfig{AuditType: "run_node_choose", Mode: "scramble", Candidates: []target.FlowAuditCandidate{{ID: "user-a", Name: "用户甲"}}}
	person := model.PathConfigPersonStrategyInput{Key: analyzer.PathConfigPersonToken("review"), Strategy: "random", Seed: 77, Selected: []string{analyzer.PathConfigPersonOptionToken("review", "user-a")}}
	persons, _ := json.Marshal(map[string]model.PathConfigPersonStrategyInput{person.Key: person, "removed-action-person": {Key: "removed-action-person", Strategy: "manual"}})
	actions, _ := json.Marshal([]model.ConfiguredAction{{Key: "saved-extra", NodeKey: analyzer.PathConfigNodeToken("review"), Action: model.ActionStorageFormData, Scope: model.ActionScopeTask, Order: 1}})
	config, store, _, plan, path := quickApproveFixture(tree, repository.HistoryPathConfigRecord{Revision: 1, UserActions: actions, PersonStrategies: persons, EffectiveFormData: []byte(`{"note":"原表单"}`)})
	if err := config.AutoConfigurePathActions(context.Background(), plan.ID, path.ID, model.HistoryReplayActionModeApproveOnly); err != nil {
		t.Fatal(err)
	}
	var saved []model.ConfiguredAction
	if err := json.Unmarshal(store.record.UserActions, &saved); err != nil || len(saved) != 0 {
		t.Fatalf("快速同意不能残留暂存或其他额外动作：%s err=%v", store.record.UserActions, err)
	}
	var remaining map[string]model.PathConfigPersonStrategyInput
	if err := json.Unmarshal(store.record.PersonStrategies, &remaining); err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 || remaining[person.Key].Seed != 77 || string(store.record.EffectiveFormData) != `{"note":"原表单"}` {
		t.Fatalf("仅清除动作专属人员，保留节点人员与表单：persons=%+v data=%s", remaining, store.record.EffectiveFormData)
	}
}

// TestQuickApproveRejectsInvalidPeople 验证人数不足与已保存失效候选仍明确阻塞，不能靠默认同意绕过。
func TestQuickApproveRejectsInvalidPeople(t *testing.T) {
	for _, invalidStored := range []bool{false, true} {
		t.Run(map[bool]string{false: "会签人数不足", true: "已保存人员失效"}[invalidStored], func(t *testing.T) {
			tree := actionConfigurationTree()
			count := 2
			tree.Child.AuditConfig = &target.FlowNodeAuditConfig{AuditType: "run_node_choose", Mode: "countersign", CountersignNum: &count, Candidates: []target.FlowAuditCandidate{{ID: "user-a", Name: "用户甲"}}}
			record := repository.HistoryPathConfigRecord{Revision: 1}
			if invalidStored {
				tree.Child.AuditConfig.Mode = "scramble"
				key := analyzer.PathConfigPersonToken("review")
				record.PersonStrategies, _ = json.Marshal(map[string]model.PathConfigPersonStrategyInput{key: {Key: key, Strategy: "manual", Selected: []string{"not-in-current-candidates"}}})
			}
			config, store, _, plan, path := quickApproveFixture(tree, record)
			err := config.AutoConfigurePathActions(context.Background(), plan.ID, path.ID, model.HistoryReplayActionModeApproveOnly)
			if err == nil || !strings.Contains(err.Error(), "审批") || store.writes != 0 {
				t.Fatalf("错误应包含节点且不得保存假成功：writes=%d err=%v", store.writes, err)
			}
		})
	}
}

// TestSavingOneDefaultNodeDoesNotCompleteWholePath 锁定手工保存与快速准备共用完整路径进度，零额外动作也能逐节点确认。
func TestSavingOneDefaultNodeDoesNotCompleteWholePath(t *testing.T) {
	config, store, _, plan, path := quickApproveFixture(quickApproveTree(), repository.HistoryPathConfigRecord{})
	for index, node := range []string{"start", "review"} {
		_, err := config.SaveActionConfiguration(context.Background(), plan.ID, path.ID, analyzer.PathConfigNodeToken(node), []string{"123e4567-e89b-12d3-a456-426614175001", "123e4567-e89b-12d3-a456-426614175002"}[index], model.ActionConfigurationInput{})
		if err != nil {
			t.Fatal(err)
		}
		if (store.record.NodeStatus == "configured") != (index == 1) {
			t.Fatalf("路径完成状态应与全部节点确认一致：index=%d status=%s", index, store.record.NodeStatus)
		}
	}
}

// TestQuickApproveCanPrepareAffectedConfiguration 验证同一节点已确认时仍可重新准备失效场景，不会卡在受影响状态。
func TestQuickApproveCanPrepareAffectedConfiguration(t *testing.T) {
	config, store, _, plan, path := quickApproveFixture(quickApproveTree(), repository.HistoryPathConfigRecord{})
	if err := config.AutoConfigurePathActions(context.Background(), plan.ID, path.ID, model.HistoryReplayActionModeApproveOnly); err != nil {
		t.Fatal(err)
	}
	store.record.ConfigStatus, store.record.NodeStatus = "affected", "affected"
	store.record.CompiledSteps = []byte(`[]`)
	if err := config.AutoConfigurePathActions(context.Background(), plan.ID, path.ID, model.HistoryReplayActionModeApproveOnly); err != nil {
		t.Fatal(err)
	}
	if store.writes != 2 || store.record.NodeStatus != "configured" || string(store.record.CompiledSteps) == `[]` {
		t.Fatalf("明确重新准备应修复状态并重新保存场景：writes=%d record=%+v", store.writes, store.record)
	}
}

// TestDefaultActionConfirmationStillRequiresNodePeople 锁定手工确认默认动作时不会绕过必须选择的节点人员。
func TestDefaultActionConfirmationStillRequiresNodePeople(t *testing.T) {
	tree := quickApproveTree()
	tree.Child.AuditConfig = &target.FlowNodeAuditConfig{AuditType: "run_node_choose", Mode: "scramble", Candidates: []target.FlowAuditCandidate{{ID: "user-a", Name: "用户甲"}}}
	config, store, _, plan, path := quickApproveFixture(tree, repository.HistoryPathConfigRecord{})
	for index, node := range []string{"start", "review"} {
		_, err := config.SaveActionConfiguration(context.Background(), plan.ID, path.ID, analyzer.PathConfigNodeToken(node), []string{"123e4567-e89b-12d3-a456-426614175003", "123e4567-e89b-12d3-a456-426614175004"}[index], model.ActionConfigurationInput{})
		if err != nil {
			t.Fatal(err)
		}
	}
	if store.record.NodeStatus == "configured" {
		t.Fatal("缺少节点人员不能因默认动作已确认而变成可运行")
	}
}
