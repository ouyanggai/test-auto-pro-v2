package action_orchestration_test

import (
	"testing"

	"test-auto-pro-v2/internal/engine/actioncatalog"
	"test-auto-pro-v2/internal/model"
)

// TestActionCatalogExposesStructureEffect 验证每个用户动作都登记稳定的写后结构效果，
// 供写后重读、代理树重绑定和运行画布判断旧节点标识是否失效。
func TestActionCatalogExposesStructureEffect(t *testing.T) {
	items := indexCatalog(actioncatalog.Build(model.ActionContext{
		FlowSource: "pending", InstanceStatus: "run", CurrentNodeType: "common",
		HasCurrentTask: true, HasEditableProxy: true, HasCompletedTask: true,
	}))
	want := map[model.ActionKey]model.ActionStructureEffect{
		model.ActionSaveDraft:       model.StructureEffectNone,
		model.ActionSubmit:          model.StructureEffectCursorMove,
		model.ActionResubmit:        model.StructureEffectCursorMove,
		model.ActionStorageFormData: model.StructureEffectNone,
		model.ActionAddSign:         model.StructureEffectInsertNode,
		model.ActionTransfer:        model.StructureEffectNone,
		model.ActionApprove:         model.StructureEffectCursorMove,
		model.ActionReject:          model.StructureEffectRecoveryChain,
		model.ActionRollback:        model.StructureEffectRecoveryChain,
		model.ActionRetrieve:        model.StructureEffectRecoveryChain,
		model.ActionWithdraw:        model.StructureEffectRecoveryChain,
		model.ActionUrge:            model.StructureEffectNone,
		model.ActionForward:         model.StructureEffectAuxiliaryInstance,
		model.ActionFollow:          model.StructureEffectNone,
		model.ActionUnfollow:        model.StructureEffectNone,
	}
	for action, effect := range want {
		if items[action].StructureEffect != effect {
			t.Fatalf("动作 %q 的写后结构效果 = %q，期望 %q", action, items[action].StructureEffect, effect)
		}
	}
	// 系统只读语义不是用户可编排候选，不应设置结构效果或候选状态。
	if items[model.ActionSystemAutomatic].CandidateStatus != "" {
		t.Fatalf("系统自动语义不应带候选状态：%+v", items[model.ActionSystemAutomatic])
	}
}

// TestActionCatalogAutoConfigurableWhenFactsComplete 验证目标事实完整时加签/取回/同意进入自动候选。
func TestActionCatalogAutoConfigurableWhenFactsComplete(t *testing.T) {
	items := indexCatalog(actioncatalog.Build(model.ActionContext{
		FlowSource: "pending", InstanceStatus: "run", CurrentNodeType: "common",
		HasCurrentTask: true, HasEditableProxy: true, HasCompletedTask: true,
	}))
	for _, action := range []model.ActionKey{model.ActionApprove, model.ActionAddSign, model.ActionRetrieve} {
		if !items[action].Enabled {
			t.Fatalf("目标事实完整时 %q 应可用：%+v", action, items[action])
		}
		if items[action].CandidateStatus != model.CandidateAutoConfigurable {
			t.Fatalf("目标事实完整时 %q 候选状态 = %q，期望 auto_configurable", action, items[action].CandidateStatus)
		}
		if items[action].CandidateReason != "" {
			t.Fatalf("可自动配置候选不应携带阻塞原因：%+v", items[action])
		}
	}
}

// TestActionCatalogFactUnavailableWhenHandlerMissing 验证目标节点未生成处理人时相关动作判为事实不可用，
// 并保留节点级原因（assignment_missing），不当作动作不适用而静默删除。
func TestActionCatalogFactUnavailableWhenHandlerMissing(t *testing.T) {
	items := indexCatalog(actioncatalog.Build(model.ActionContext{
		FlowSource: "pending", InstanceStatus: "run", CurrentNodeType: "common",
		HasCurrentTask: false, HandlerMissing: true,
	}))
	for _, action := range []model.ActionKey{model.ActionApprove, model.ActionAddSign} {
		if items[action].Enabled {
			t.Fatalf("处理人事实缺失时 %q 不应可用：%+v", action, items[action])
		}
		if items[action].CandidateStatus != model.CandidateFactUnavailable {
			t.Fatalf("处理人事实缺失时 %q 候选状态 = %q，期望 fact_unavailable", action, items[action].CandidateStatus)
		}
		if items[action].CandidateReason == "" {
			t.Fatalf("事实不可用候选必须携带节点级原因：%+v", items[action])
		}
	}
}

// TestActionCatalogManualOnlyWhenFactKnownButInapplicable 验证事实已知但当前不适用（缺已办、代理不可编辑）
// 时动作保留为仅人工配置候选并带节点级原因，目录项不被删除。
func TestActionCatalogManualOnlyWhenFactKnownButInapplicable(t *testing.T) {
	// 无已办任务：取回缺已办事实但事实本身已知（读到当前用户没有已办）。
	noDone := indexCatalog(actioncatalog.Build(model.ActionContext{
		FlowSource: "pending", InstanceStatus: "run", CurrentNodeType: "common",
		HasCurrentTask: true, HasCompletedTask: false,
	}))
	if noDone[model.ActionRetrieve].Enabled {
		t.Fatalf("缺已办时取回不应可用：%+v", noDone[model.ActionRetrieve])
	}
	if noDone[model.ActionRetrieve].CandidateStatus != model.CandidateManualOnly {
		t.Fatalf("缺已办时取回候选状态 = %q，期望 manual_only", noDone[model.ActionRetrieve].CandidateStatus)
	}
	if noDone[model.ActionRetrieve].CandidateReason == "" {
		t.Fatal("仅人工候选必须携带节点级原因")
	}

	// 代理不可编辑：加签结构无法证明可编辑时保留为仅人工候选。
	noProxy := indexCatalog(actioncatalog.Build(model.ActionContext{
		FlowSource: "pending", InstanceStatus: "run", CurrentNodeType: "common",
		HasCurrentTask: true, HasEditableProxy: false,
	}))
	if noProxy[model.ActionAddSign].Enabled {
		t.Fatalf("代理不可编辑时加签不应可用：%+v", noProxy[model.ActionAddSign])
	}
	if noProxy[model.ActionAddSign].CandidateStatus != model.CandidateManualOnly {
		t.Fatalf("代理不可编辑时加签候选状态 = %q，期望 manual_only", noProxy[model.ActionAddSign].CandidateStatus)
	}
}
