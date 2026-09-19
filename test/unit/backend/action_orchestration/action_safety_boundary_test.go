package action_orchestration_test

import (
	"testing"

	"test-auto-pro-v2/internal/engine/actioncatalog"
	"test-auto-pro-v2/internal/engine/scenario"
	"test-auto-pro-v2/internal/model"
)

// compileTailPathWithInstanceAction 在 review 节点排一个暂存动作后追加一个实例级动作，
// 用于断言实例级动作不会多生成固定尾同意。
func compileTailPathWithInstanceAction(action model.ActionKey, scope model.ActionScope) (scenario.Result, error) {
	return scenario.Compile(tailPath([]model.ConfiguredAction{
		{Key: "storage-1", Action: model.ActionStorageFormData, Scope: model.ActionScopeTask, NodeKey: "review", Order: 1},
		{Key: "instance-1", Action: action, Scope: scope, Order: 2},
	}))
}

// TestAllUserActionsShareUnifiedSafetyBoundary 验证每个用户动作都遵守统一安全边界所需的规格：
// 唯一目标端点、明确作用域、写后结构效果分类和至少一项写后重读要求。缺任一项都会让写后
// 无法核验或旧节点标识不失效，属安全边界漏洞。
func TestAllUserActionsShareUnifiedSafetyBoundary(t *testing.T) {
	items := indexCatalog(actioncatalog.Build(model.ActionContext{
		FlowSource: "pending", InstanceStatus: "run", CurrentNodeType: "common",
		HasCurrentTask: true, HasEditableProxy: true, HasCompletedTask: true,
	}))
	userActions := []model.ActionKey{
		model.ActionSaveDraft, model.ActionSubmit, model.ActionResubmit, model.ActionStorageFormData,
		model.ActionAddSign, model.ActionTransfer, model.ActionApprove, model.ActionReject,
		model.ActionRollback, model.ActionRetrieve, model.ActionWithdraw, model.ActionUrge,
		model.ActionForward, model.ActionFollow, model.ActionUnfollow,
	}
	for _, action := range userActions {
		item := items[action]
		if item.TargetOperation == "" {
			t.Fatalf("动作 %q 缺少唯一目标端点", action)
		}
		if item.Scope == "" {
			t.Fatalf("动作 %q 缺少作用域", action)
		}
		if item.StructureEffect == "" {
			t.Fatalf("动作 %q 缺少写后结构效果分类", action)
		}
		if len(item.ReloadRequirements) == 0 || !item.RequiresReload {
			t.Fatalf("动作 %q 缺少写后重读要求", action)
		}
	}
}

// TestActionStructureEffectMatchesSemanticGroup 验证写后结构效果与动作语义分组一致：
// 恢复链、插入节点、辅助实例和游标移动不能相互混淆，否则写后重读和旧节点处理会走错分支。
func TestActionStructureEffectMatchesSemanticGroup(t *testing.T) {
	items := indexCatalog(actioncatalog.Build(model.ActionContext{
		FlowSource: "pending", InstanceStatus: "run", CurrentNodeType: "common",
		HasCurrentTask: true, HasEditableProxy: true, HasCompletedTask: true,
	}))
	groups := map[model.ActionStructureEffect][]model.ActionKey{
		model.StructureEffectCursorMove:        {model.ActionSubmit, model.ActionResubmit, model.ActionApprove},
		model.StructureEffectRecoveryChain:     {model.ActionReject, model.ActionRollback, model.ActionRetrieve, model.ActionWithdraw},
		model.StructureEffectInsertNode:        {model.ActionAddSign},
		model.StructureEffectAuxiliaryInstance: {model.ActionForward},
		model.StructureEffectNone:              {model.ActionSaveDraft, model.ActionStorageFormData, model.ActionTransfer, model.ActionUrge, model.ActionFollow, model.ActionUnfollow},
	}
	for effect, actions := range groups {
		for _, action := range actions {
			if items[action].StructureEffect != effect {
				t.Fatalf("动作 %q 的结构效果 = %q，期望 %q", action, items[action].StructureEffect, effect)
			}
		}
	}
}

// TestInstanceScopeActionsNeverEmitFixedTailConsent 验证催办、转发、取消关注等实例级动作排在节点
// 动作之后时都不会多生成固定尾同意——实例级动作只改变实例级事实，绝不承载节点同意。
func TestInstanceScopeActionsNeverEmitFixedTailConsent(t *testing.T) {
	cases := []struct {
		name   string
		action model.ActionKey
		scope  model.ActionScope
	}{
		{"催办", model.ActionUrge, model.ActionScopeInstance},
		{"转发", model.ActionForward, model.ActionScopeInstance},
		{"关注", model.ActionFollow, model.ActionScopeInstance},
	}
	for _, item := range cases {
		item := item
		t.Run(item.name, func(t *testing.T) {
			result, err := compileTailPathWithInstanceAction(item.action, item.scope)
			if err != nil {
				t.Fatalf("%s 编译失败：%v", item.name, err)
			}
			if got := systemDefaultApproveCount(result.Steps); got != 1 {
				t.Fatalf("%s 后固定尾同意数量 = %d，期望恰好 1（仅 review 节点）：%+v", item.name, got, result.Steps)
			}
		})
	}
}
