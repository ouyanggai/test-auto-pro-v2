package action_orchestration_test

import (
	"context"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// TestForwardNodeConfigurationUsesCurrentInstanceCandidates 验证编辑节点上已有转发时沿用真实实例候选，完整保留附言和历史勾选参数。
func TestForwardNodeConfigurationUsesCurrentInstanceCandidates(t *testing.T) {
	plan := model.Plan{ID: 846, Account: "account-a", FlowSource: "new", TargetObjectID: "flow-a"}
	path := model.ExecutionPath{ID: 946, PlanID: plan.ID, Name: "转发核对"}
	tree := catalogProjectionTree()
	tree.ForwardCandidates = []target.FlowAuditCandidate{{ID: "receiver-1", Name: "接收人"}}
	store := &actionHistoryStore{}
	config := service.NewPathConfigService(service.NewPlanService(actionPlanRepository{plan: plan}), &actionTargetReader{snapshot: target.PathConfigurationSnapshot{Tree: tree, EntryNodeIDs: []string{"start"}, FlowCode: "flow-a", RenderType: target.FormRenderTypeFormMaking}}, analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathConfigAnalyzer(), actionPathRepository{path: path})
	config.SetHistoryWorkspaceStores(store, store)
	view, err := config.Get(context.Background(), plan.ID, path.ID)
	if err != nil {
		t.Fatal(err)
	}
	person := findCatalogItem(view.InstanceActions.Catalog, "forward").Person
	if person == nil || len(person.Options) != 1 {
		t.Fatal("目标真实转发候选没有投影")
	}
	input := model.ActionConfigurationInput{
		Persons: []model.PathConfigPersonStrategyInput{{Key: person.Key, Strategy: "manual", Seed: 1, Selected: []string{person.Options[0].Value}}},
		Actions: []model.ConfiguredAction{{Key: "forward-1", Action: model.ActionForward, Scope: model.ActionScopeInstance, NodeKey: analyzer.PathConfigNodeToken("review"), Order: 1, Parameters: map[string]any{"approveMessage": "当前附言", "forwardCheckList": []string{"fuyan"}}}},
	}
	saved, err := config.SaveActionConfiguration(context.Background(), plan.ID, path.ID, analyzer.PathConfigNodeToken("review"), "123e4567-e89b-12d3-a456-426614174846", input)
	if err != nil || len(saved.Actions) != 1 || saved.Actions[0].Parameters["approveMessage"] != "当前附言" {
		t.Fatalf("编辑转发必须保留完整参数：%+v，错误=%v", saved, err)
	}
	input.Persons[0].Selected = []string{"不在当前目录中的人"}
	_, err = config.SaveActionConfiguration(context.Background(), plan.ID, path.ID, analyzer.PathConfigNodeToken("review"), "123e4567-e89b-12d3-a456-426614174847", input)
	if err == nil || store.writes != 1 {
		t.Fatal("复用实例目录不能放行不属于候选的人员")
	}
}
