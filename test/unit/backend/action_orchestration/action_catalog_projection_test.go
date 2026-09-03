package action_orchestration_test

import (
	"context"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// catalogProjectionTree 创建发起、两个人工审批和结束节点的最小真实流程；第二个审批节点没有候选目录。
func catalogProjectionTree() *target.FlowNodeTemplate {
	return &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{
		ID: "review", Type: "common", Name: "一级审批",
		AuditConfig:       &target.FlowNodeAuditConfig{AuditType: "appoint", Mode: "all", Candidates: []target.FlowAuditCandidate{{ID: "user-a", Name: "用户 A"}}},
		AddSignCandidates: []target.FlowAuditCandidate{{ID: "user-a", Name: "用户 A"}},
		Child: &target.FlowNodeTemplate{ID: "second", Type: "common", Name: "二级审批",
			AuditConfig: &target.FlowNodeAuditConfig{AuditType: "appoint", Mode: "all"},
			Child:       &target.FlowNodeTemplate{ID: "end", Type: "end"},
		},
	}}
}

// newCatalogProjectionService 组装使用真实动作门禁的路径配置服务。
func newCatalogProjectionService(t *testing.T, planID, pathID uint64) (*service.PathConfigService, model.Plan, model.ExecutionPath) {
	t.Helper()
	plan := model.Plan{ID: planID, Account: "account-a", FlowSource: "new", TargetObjectID: "flow-a", Status: model.PlanStatusNotStarted}
	path := model.ExecutionPath{ID: pathID, PlanID: planID, SequenceNo: 1, Name: "审批路径"}
	store := &actionHistoryStore{}
	config := service.NewPathConfigService(service.NewPlanService(actionPlanRepository{plan: plan}),
		&actionTargetReader{snapshot: target.PathConfigurationSnapshot{Tree: catalogProjectionTree(), EntryNodeIDs: []string{"start"}, FlowCode: "flow-a", FlowName: "审批流程", RenderType: target.FormRenderTypeFormMaking}},
		analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathConfigAnalyzer(), actionPathRepository{path: path})
	config.SetHistoryWorkspaceStores(store, store)
	return config, plan, path
}

// catalogKinds 返回目录项动作键顺序。
func catalogKinds(items []model.PathConfigActionCatalogItem) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.Kind)
	}
	return result
}

// findCatalogItem 按动作键定位目录项。
func findCatalogItem(items []model.PathConfigActionCatalogItem, kind string) model.PathConfigActionCatalogItem {
	for _, item := range items {
		if item.Kind == kind {
			return item
		}
	}
	return model.PathConfigActionCatalogItem{}
}

// nodeByKey 在路径配置分组中定位语义节点。
func nodeByKey(configuration model.PathConfiguration, key string) *model.PathConfigNode {
	for groupIndex := range configuration.Groups {
		for nodeIndex := range configuration.Groups[groupIndex].Nodes {
			if configuration.Groups[groupIndex].Nodes[nodeIndex].Key == key {
				return &configuration.Groups[groupIndex].Nodes[nodeIndex]
			}
		}
	}
	return nil
}

// gateBlockedReason 断言保存被动作门禁阻断并返回首个阻断动作的定位信息。
func gateBlockedReason(t *testing.T, err error, expectedAction string) string {
	t.Helper()
	if err == nil {
		t.Fatalf("动作 %s 未被门禁阻断", expectedAction)
	}
	configError, ok := err.(*service.PathConfigError)
	if !ok {
		t.Fatalf("动作 %s 的阻断错误类型不正确：%T", expectedAction, err)
	}
	if configError.Kind != service.PathConfigErrorInvalid || configError.Message != "动作顺序无法恢复，请修正首个阻断动作" {
		t.Fatalf("动作 %s 的阻断错误不符合预期：%+v", expectedAction, configError)
	}
	if len(configError.Affected) != 1 || configError.Affected[0].Kind != "action" || configError.Affected[0].Name == "" {
		t.Fatalf("动作 %s 的阻断未定位到具体动作：%+v", expectedAction, configError.Affected)
	}
	return configError.Affected[0].Reason
}

// TestPathConfigurationProjectsRealActionCatalog 验证节点与实例配置位置投影真实门禁目录而不是固定可用子集。
func TestPathConfigurationProjectsRealActionCatalog(t *testing.T) {
	config, plan, path := newCatalogProjectionService(t, 821, 831)
	configuration, err := config.Get(context.Background(), plan.ID, path.ID)
	if err != nil {
		t.Fatalf("读取路径配置失败：%v", err)
	}
	start := nodeByKey(configuration, analyzer.PathConfigNodeToken("start"))
	review := nodeByKey(configuration, analyzer.PathConfigNodeToken("review"))
	second := nodeByKey(configuration, analyzer.PathConfigNodeToken("second"))
	end := nodeByKey(configuration, analyzer.PathConfigNodeToken("end"))
	if start == nil || review == nil || second == nil || end == nil {
		t.Fatalf("路径配置缺少语义节点：%+v", configuration.Groups)
	}
	if got := strings.Join(catalogKinds(start.ActionConfiguration.Catalog), ","); got != "save_draft,submit,resubmit" {
		t.Fatalf("发起节点目录 = %q，期望完整发起生命周期动作", got)
	}
	if got := strings.Join(catalogKinds(review.ActionConfiguration.Catalog), ","); got != "storage_form_data,add_sign,transfer,approve,reject,rollback_previous,retrieve" {
		t.Fatalf("审批节点目录 = %q，期望当前待办与已办恢复动作", got)
	}
	if item := findCatalogItem(start.ActionConfiguration.Catalog, "resubmit"); !item.Enabled || item.RuntimeNote == "" {
		t.Fatalf("重新提交缺少顺序前置说明：%+v", item)
	}
	addSign := findCatalogItem(review.ActionConfiguration.Catalog, "add_sign")
	if !addSign.Enabled || !addSign.RequiresPerson || addSign.Person == nil || len(addSign.Preconditions) == 0 || !addSign.RequiresReload {
		t.Fatalf("加签目录项缺少候选人员或重读事实：%+v", addSign)
	}
	rollback := findCatalogItem(review.ActionConfiguration.Catalog, "rollback_previous")
	if rollback.Enabled || !strings.Contains(rollback.DisabledReason, "发起节点") {
		t.Fatalf("首个业务节点回退未按目标规则禁用：%+v", rollback)
	}
	if item := findCatalogItem(second.ActionConfiguration.Catalog, "rollback_previous"); !item.Enabled {
		t.Fatalf("存在真实业务前驱时回退应可配置：%+v", item)
	}
	for _, kind := range []string{"add_sign", "transfer"} {
		item := findCatalogItem(second.ActionConfiguration.Catalog, kind)
		if item.Enabled || item.DisabledReason == "" {
			t.Fatalf("缺少候选目录的 %s 应给出中文禁用原因：%+v", kind, item)
		}
	}
	if len(end.ActionConfiguration.Catalog) != 1 || !end.ActionConfiguration.Catalog[0].SystemOnly || end.ActionConfiguration.Catalog[0].SystemNodeType == "" {
		t.Fatalf("结束节点应只显示系统自动语义：%+v", end.ActionConfiguration.Catalog)
	}
	if configuration.InstanceActionKey != analyzer.PathConfigInstanceActionKey() {
		t.Fatalf("实例动作容器缺少稳定键：%q", configuration.InstanceActionKey)
	}
	if got := strings.Join(catalogKinds(configuration.InstanceActions.Catalog), ","); got != "withdraw,urge,forward,follow,unfollow" {
		t.Fatalf("实例动作目录 = %q，期望完整实例管理动作", got)
	}
	if item := findCatalogItem(configuration.InstanceActions.Catalog, "unfollow"); !item.Enabled || item.RuntimeNote == "" {
		t.Fatalf("取消关注缺少顺序前置说明：%+v", item)
	}
}

// TestSaveActionConfigurationEnforcesCatalogGate 验证保存时按实时门禁阻断被禁用动作和系统节点动作。
func TestSaveActionConfigurationEnforcesCatalogGate(t *testing.T) {
	config, plan, path := newCatalogProjectionService(t, 822, 832)
	reviewKey := analyzer.PathConfigNodeToken("review")
	_, err := config.SaveActionConfiguration(context.Background(), plan.ID, path.ID, reviewKey, "123e4567-e89b-12d3-a456-426614174821", model.ActionConfigurationInput{
		Actions: []model.ConfiguredAction{{Key: "rollback-1", Action: model.ActionRollback, Scope: model.ActionScopeTask, Order: 1}},
	})
	if reason := gateBlockedReason(t, err, "rollback_previous"); !strings.Contains(reason, "发起节点") {
		t.Fatalf("回退阻断原因未复用目标门禁说明：%q", reason)
	}
	secondKey := analyzer.PathConfigNodeToken("second")
	_, err = config.SaveActionConfiguration(context.Background(), plan.ID, path.ID, secondKey, "123e4567-e89b-12d3-a456-426614174822", model.ActionConfigurationInput{
		Actions: []model.ConfiguredAction{{Key: "transfer-1", Action: model.ActionTransfer, Scope: model.ActionScopeTask, Order: 1, ActorPolicy: "manual"}},
	})
	if reason := gateBlockedReason(t, err, "transfer"); !strings.Contains(reason, "移交") {
		t.Fatalf("移交阻断原因未复用目标门禁说明：%q", reason)
	}
	_, err = config.SaveActionConfiguration(context.Background(), plan.ID, path.ID, analyzer.PathConfigNodeToken("end"), "123e4567-e89b-12d3-a456-426614174823", model.ActionConfigurationInput{
		Actions: []model.ConfiguredAction{{Key: "approve-1", Action: model.ActionApprove, Scope: model.ActionScopeTask, Order: 1}},
	})
	if reason := gateBlockedReason(t, err, "approve@end"); !strings.Contains(reason, "人工审批或协同节点") {
		t.Fatalf("系统节点保存未按节点类型阻断：%q", reason)
	}
	if _, err := config.SaveActionConfiguration(context.Background(), plan.ID, path.ID, reviewKey, "123e4567-e89b-12d3-a456-426614174824", model.ActionConfigurationInput{
		Actions: []model.ConfiguredAction{{Key: "approve-1", Action: model.ActionApprove, Scope: model.ActionScopeTask, Order: 1}},
	}); err != nil {
		t.Fatalf("门禁通过的同意动作被误阻断：%v", err)
	}
}

// TestSaveInstanceActionsUsesDedicatedContainer 验证实例动作使用独立容器保存、按顺序复验并可重读。
func TestSaveInstanceActionsUsesDedicatedContainer(t *testing.T) {
	config, plan, path := newCatalogProjectionService(t, 823, 833)
	instanceKey := analyzer.PathConfigInstanceActionKey()
	_, err := config.SaveActionConfiguration(context.Background(), plan.ID, path.ID, instanceKey, "123e4567-e89b-12d3-a456-426614174831", model.ActionConfigurationInput{
		Actions: []model.ConfiguredAction{{Key: "unfollow-1", Action: model.ActionUnfollow, Scope: model.ActionScopeInstance, Order: 1}},
	})
	if reason := gateBlockedReason(t, err, "unfollow"); !strings.Contains(reason, "关注") {
		t.Fatalf("取消关注顺序阻断原因不明确：%q", reason)
	}
	_, err = config.SaveActionConfiguration(context.Background(), plan.ID, path.ID, instanceKey, "123e4567-e89b-12d3-a456-426614174832", model.ActionConfigurationInput{
		Actions: []model.ConfiguredAction{{Key: "approve-1", Action: model.ActionApprove, Scope: model.ActionScopeTask, Order: 1}},
	})
	if err == nil || !strings.Contains(err.Error(), "实例动作容器只能保存实例作用域动作") {
		t.Fatalf("实例容器接受了节点动作：%v", err)
	}
	saved, err := config.SaveActionConfiguration(context.Background(), plan.ID, path.ID, instanceKey, "123e4567-e89b-12d3-a456-426614174833", model.ActionConfigurationInput{
		Actions: []model.ConfiguredAction{
			{Key: "follow-1", Action: model.ActionFollow, Scope: model.ActionScopeInstance, Order: 1},
			{Key: "unfollow-1", Action: model.ActionUnfollow, Scope: model.ActionScopeInstance, Order: 2},
		},
	})
	if err != nil {
		t.Fatalf("保存实例动作失败：%v", err)
	}
	if len(saved.Actions) != 2 || saved.Actions[0].NodeKey != "" || saved.Actions[1].NodeKey != "" {
		t.Fatalf("实例动作不能绑定语义节点：%+v", saved.Actions)
	}
	configuration, err := config.Get(context.Background(), plan.ID, path.ID)
	if err != nil {
		t.Fatalf("重读路径配置失败：%v", err)
	}
	if len(configuration.InstanceActions.Actions) != 2 || configuration.InstanceActions.Actions[0].Kind != "follow" {
		t.Fatalf("实例动作未投影回独立容器：%+v", configuration.InstanceActions)
	}
	for _, group := range configuration.Groups {
		for _, node := range group.Nodes {
			if len(node.ActionConfiguration.Actions) != 0 {
				t.Fatalf("实例动作被错误投影到语义节点 %s：%+v", node.Name, node.ActionConfiguration.Actions)
			}
		}
	}
}
