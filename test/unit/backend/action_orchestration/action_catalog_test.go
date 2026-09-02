package action_orchestration_test

import (
	"testing"

	"test-auto-pro-v2/internal/engine/actioncatalog"
	"test-auto-pro-v2/internal/model"
)

// TestActionCatalogExposesStableGroupsAndTargetParameters 验证目录分组、目标接口参数和重读要求完整。
func TestActionCatalogExposesStableGroupsAndTargetParameters(t *testing.T) {
	items := actioncatalog.Build(model.ActionContext{FlowSource: "new", IsInitiator: true})
	if len(items) != 16 {
		t.Fatalf("动作目录数量 = %d，期望 16 个（15 个用户动作和 1 个系统语义）", len(items))
	}
	wantOrder := []model.ActionKey{
		model.ActionSaveDraft, model.ActionSubmit, model.ActionResubmit,
		model.ActionStorageFormData, model.ActionAddSign, model.ActionTransfer,
		model.ActionApprove, model.ActionReject, model.ActionRollback,
		model.ActionRetrieve,
		model.ActionWithdraw, model.ActionUrge, model.ActionForward, model.ActionFollow, model.ActionUnfollow,
		model.ActionSystemAutomatic,
	}
	for index, want := range wantOrder {
		if items[index].Action != want {
			t.Fatalf("目录顺序[%d] = %q，期望 %q", index, items[index].Action, want)
		}
		if items[index].Category == "" || items[index].Scope == "" || items[index].Label == "" || items[index].Description == "" {
			t.Fatalf("目录项 %q 缺少分组或说明：%+v", want, items[index])
		}
		if len(items[index].Parameters) != len(items[index].ParameterDetails) {
			t.Fatalf("目录项 %q 的参数投影不一致：%+v", want, items[index])
		}
		if len(items[index].ReloadRequirements) == 0 || !items[index].RequiresReload {
			t.Fatalf("目录项 %q 缺少重读屏障：%+v", want, items[index])
		}
	}
	byAction := indexCatalog(items)
	for _, name := range []string{"formDataMongoVo.data", "auditRecord.executeDesc"} {
		if !contains(byAction[model.ActionStorageFormData].Parameters, name) {
			t.Fatalf("审批暂存缺少真实参数 %q：%+v", name, byAction[model.ActionStorageFormData])
		}
	}
	if contains(byAction[model.ActionStorageFormData].Parameters, "status") {
		t.Fatal("审批暂存混入了发起端保存草稿参数")
	}
	if !hasParameterValue(byAction[model.ActionSaveDraft], "status", "draft") {
		t.Fatal("保存草稿缺少固定 draft 参数")
	}
	if byAction[model.ActionSaveDraft].TargetOperation != "/web/flowInstanceApi/submit" || byAction[model.ActionStorageFormData].TargetOperation != "/web/flowInstanceApi/storageFormData" {
		t.Fatalf("草稿和审批暂存目标接口不正确：draft=%s storage=%s", byAction[model.ActionSaveDraft].TargetOperation, byAction[model.ActionStorageFormData].TargetOperation)
	}
}

// TestActionCatalogLifecycleGatesSeparatesDraftAndResubmit 验证发起生命周期状态集合互斥且原因可解释。
func TestActionCatalogLifecycleGatesSeparatesDraftAndResubmit(t *testing.T) {
	newItems := indexCatalog(actioncatalog.Build(model.ActionContext{FlowSource: "new", IsInitiator: true}))
	if !newItems[model.ActionSaveDraft].Enabled || !newItems[model.ActionSubmit].Enabled || newItems[model.ActionResubmit].Enabled {
		t.Fatalf("新建上下文生命周期门禁错误：draft=%+v submit=%+v resubmit=%+v", newItems[model.ActionSaveDraft], newItems[model.ActionSubmit], newItems[model.ActionResubmit])
	}
	rejectedItems := indexCatalog(actioncatalog.Build(model.ActionContext{FlowSource: "submitted", InstanceStatus: "rejected", IsInitiator: true}))
	if rejectedItems[model.ActionSaveDraft].Enabled || rejectedItems[model.ActionSubmit].Enabled || !rejectedItems[model.ActionResubmit].Enabled {
		t.Fatalf("驳回上下文生命周期门禁错误：draft=%+v submit=%+v resubmit=%+v", rejectedItems[model.ActionSaveDraft], rejectedItems[model.ActionSubmit], rejectedItems[model.ActionResubmit])
	}
	draftItems := indexCatalog(actioncatalog.Build(model.ActionContext{FlowSource: "submitted", InstanceStatus: "draft", IsInitiator: true}))
	if !draftItems[model.ActionSaveDraft].Enabled || draftItems[model.ActionSubmit].Enabled || !draftItems[model.ActionResubmit].Enabled {
		t.Fatalf("已有草稿生命周期门禁错误：draft=%+v submit=%+v resubmit=%+v", draftItems[model.ActionSaveDraft], draftItems[model.ActionSubmit], draftItems[model.ActionResubmit])
	}
	if rejectedItems[model.ActionSubmit].DisabledReason == "" || rejectedItems[model.ActionSaveDraft].DisabledReason == "" {
		t.Fatal("禁用生命周期动作未返回中文原因")
	}
	nonInitiator := indexCatalog(actioncatalog.Build(model.ActionContext{FlowSource: "new", IsInitiator: false}))
	if nonInitiator[model.ActionSaveDraft].Enabled || nonInitiator[model.ActionSaveDraft].DisabledReason != "只有流程发起人可以保存草稿" {
		t.Fatalf("非发起人保存草稿门禁错误：%+v", nonInitiator[model.ActionSaveDraft])
	}
}

// TestActionCatalogCurrentTodoGatesCoverPrivateProxyActorAndRollback 验证当前待办、私有代理、移交演员和直接前驱门禁。
func TestActionCatalogCurrentTodoGatesCoverPrivateProxyActorAndRollback(t *testing.T) {
	base := model.ActionContext{FlowSource: "pending", InstanceStatus: "run", CurrentNodeType: "common", HasCurrentTask: true, PreviousTaskExists: true, PreviousNodeType: "common"}
	items := indexCatalog(actioncatalog.Build(base))
	for _, key := range []model.ActionKey{model.ActionStorageFormData, model.ActionApprove, model.ActionReject, model.ActionRollback} {
		if !items[key].Enabled {
			t.Fatalf("合法当前待办动作 %q 未启用：%+v", key, items[key])
		}
	}
	if items[model.ActionAddSign].Enabled || items[model.ActionAddSign].DisabledReason == "" {
		t.Fatal("无可编辑私有代理时加签未被阻止")
	}
	withProxy := base
	withProxy.HasEditableProxy, withProxy.CanSwitchActor = true, true
	items = indexCatalog(actioncatalog.Build(withProxy))
	if !items[model.ActionAddSign].Enabled || !items[model.ActionTransfer].Enabled {
		t.Fatalf("私有代理或移交演员事实未生效：add=%+v transfer=%+v", items[model.ActionAddSign], items[model.ActionTransfer])
	}
	priorStart := withProxy
	priorStart.PreviousNodeType, priorStart.PreviousNodeIsStart = "start", true
	items = indexCatalog(actioncatalog.Build(priorStart))
	if items[model.ActionRollback].Enabled || items[model.ActionRollback].DisabledReason != "直接前一节点是发起节点，请按目标规则使用不同意" {
		t.Fatalf("发起节点前驱回退门禁错误：%+v", items[model.ActionRollback])
	}
	unknownPrior := withProxy
	unknownPrior.PreviousNodeType = ""
	items = indexCatalog(actioncatalog.Build(unknownPrior))
	if items[model.ActionRollback].Enabled || items[model.ActionRollback].DisabledReason == "" {
		t.Fatal("未重读直接前驱类型却允许回退")
	}
	forwarded := withProxy
	forwarded.ForwardedContext = true
	items = indexCatalog(actioncatalog.Build(forwarded))
	if items[model.ActionApprove].Enabled || items[model.ActionApprove].DisabledReason != "转发辅助流程不提供主实例审批动作" {
		t.Fatalf("转发上下文审批门禁错误：%+v", items[model.ActionApprove])
	}
}

// TestActionCatalogRecoveryAndInstanceGates 验证取回、撤回、催办、转发和关注状态门禁。
func TestActionCatalogRecoveryAndInstanceGates(t *testing.T) {
	retrieve := model.ActionContext{FlowSource: "done", InstanceStatus: "run", CurrentNodeType: "common", HasCompletedTask: true}
	items := indexCatalog(actioncatalog.Build(retrieve))
	if !items[model.ActionRetrieve].Enabled {
		t.Fatalf("合法已办取回未启用：%+v", items[model.ActionRetrieve])
	}
	blocked := retrieve
	blocked.NextTaskProcessed = true
	items = indexCatalog(actioncatalog.Build(blocked))
	if items[model.ActionRetrieve].Enabled || items[model.ActionRetrieve].DisabledReason != "后继任务已经处理，不支持取回" {
		t.Fatalf("后继已处理却允许取回：%+v", items[model.ActionRetrieve])
	}
	for _, context := range []struct {
		name string
		set  func(*model.ActionContext)
	}{
		{name: "会签", set: func(value *model.ActionContext) {
			value.CurrentTaskCountersign, value.CurrentTaskHandledByOther = true, true
		}},
		{name: "并行", set: func(value *model.ActionContext) {
			value.CurrentTaskParallel, value.CurrentTaskHandledByOther = true, true
		}},
	} {
		gated := retrieve
		context.set(&gated)
		items = indexCatalog(actioncatalog.Build(gated))
		if items[model.ActionRetrieve].Enabled || items[model.ActionRetrieve].DisabledReason != "会签或并行节点已有其他演员处理，不支持取回" {
			t.Fatalf("%s其他演员已处理却允许取回：%+v", context.name, items[model.ActionRetrieve])
		}
	}
	initiator := model.ActionContext{FlowSource: "submitted", InstanceStatus: "run", IsInitiator: true, HasPendingRecipient: true}
	items = indexCatalog(actioncatalog.Build(initiator))
	for _, key := range []model.ActionKey{model.ActionWithdraw, model.ActionUrge, model.ActionForward, model.ActionFollow} {
		if !items[key].Enabled {
			t.Fatalf("合法实例管理动作 %q 未启用：%+v", key, items[key])
		}
	}
	if items[model.ActionUnfollow].Enabled {
		t.Fatal("未关注实例却允许取消关注")
	}
	followed := initiator
	followed.Followed = true
	items = indexCatalog(actioncatalog.Build(followed))
	if items[model.ActionFollow].Enabled || !items[model.ActionUnfollow].Enabled {
		t.Fatalf("关注状态门禁错误：follow=%+v unfollow=%+v", items[model.ActionFollow], items[model.ActionUnfollow])
	}
	done := initiator
	done.InstanceStatus, done.InstanceEnded = "end", true
	items = indexCatalog(actioncatalog.Build(done))
	if items[model.ActionWithdraw].Enabled || items[model.ActionUrge].Enabled || items[model.ActionFollow].Enabled {
		t.Fatalf("终态实例仍允许撤回、催办或关注：withdraw=%+v urge=%+v follow=%+v", items[model.ActionWithdraw], items[model.ActionUrge], items[model.ActionFollow])
	}
}

// TestActionCatalogSystemNodesAreReadOnly 验证系统节点仅投影自动语义且不出现不可恢复终止动作。
func TestActionCatalogSystemNodesAreReadOnly(t *testing.T) {
	allItems := actioncatalog.Build(model.ActionContext{FlowSource: "pending", InstanceStatus: "run", CurrentNodeType: "condition", HasCurrentTask: true})
	for _, nodeType := range []string{"condition", "manual", "parallel", "merge", "empty", "end", "timer", "subprocess", "callback"} {
		items := indexCatalog(actioncatalog.Build(model.ActionContext{FlowSource: "pending", InstanceStatus: "run", CurrentNodeType: nodeType, HasCurrentTask: true}))
		system := items[model.ActionSystemAutomatic]
		if !system.Enabled || !system.SystemOnly || system.SystemNodeType != nodeType || system.Category != model.ActionCategorySystemAutomatic {
			t.Fatalf("%s 节点系统语义投影错误：%+v", nodeType, system)
		}
		for _, key := range []model.ActionKey{model.ActionStorageFormData, model.ActionApprove, model.ActionReject, model.ActionAddSign, model.ActionTransfer} {
			if items[key].Enabled || items[key].DisabledReason == "" {
				t.Fatalf("%s 节点动作 %q 未被明确禁用：%+v", nodeType, key, items[key])
			}
		}
	}
	for _, item := range allItems {
		forbidden := item.Action == model.ActionKey("delete") || item.Action == model.ActionKey("terminate") || item.Action == model.ActionKey("abandon")
		if forbidden {
			t.Fatalf("不可恢复终止动作进入目录：%q", item.Action)
		}
	}
}

// indexCatalog 将目录切片按稳定动作键索引，测试不依赖实现内部 map。
func indexCatalog(items []model.ActionCatalogItem) map[model.ActionKey]model.ActionCatalogItem {
	result := make(map[model.ActionKey]model.ActionCatalogItem, len(items))
	for _, item := range items {
		result[item.Action] = item
	}
	return result
}

// contains 判断字符串参数投影是否包含目标参数名。
func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// hasParameterValue 判断目录参数是否以目标字段和值投影固定协议约束。
func hasParameterValue(item model.ActionCatalogItem, name, value string) bool {
	for _, parameter := range item.ParameterDetails {
		if parameter.Name == name && parameter.Value == value {
			return true
		}
	}
	return false
}
