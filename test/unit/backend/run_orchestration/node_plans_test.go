package run_orchestration_test

import (
	"testing"

	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// TestNodePlansGroupedByGraphNodeID 锁定运行详情「节点配置」的两条硬口径：
// 一是编译场景的令牌键必须翻译成图节点 ID（画布点节点后才取得到计划），
// 二是动作、来源与作用范围一律输出中文，界面不得拿到内部枚举值（纲领第 12.1 节）。
func TestNodePlansGroupedByGraphNodeID(t *testing.T) {
	startToken := analyzer.PathConfigNodeToken("node-start")
	auditToken := analyzer.PathConfigNodeToken("node-audit")
	compiled := []model.CompiledActionStep{
		{
			Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionSubmit,
			Scope: model.ActionScopeInitiator, NodeKey: startToken,
			Precondition: "保存时已核对门禁", ExpectedEffect: "实例进入运行态",
			Parameters: map[string]any{"opinion": "同意"}, ReloadRequired: true,
		},
		{
			Sequence: 2, Source: model.ActionStepSourceNavigation, Action: model.ActionSystemAutomatic,
			Scope: model.ActionScopeTask, NodeKey: auditToken,
		},
	}
	tokenToGraphID := map[string]string{startToken: "node-start", auditToken: "node-audit"}

	plans := service.BuildNodePlansForTest(compiled, tokenToGraphID)

	startPlan, ok := plans["node-start"]
	if !ok || len(startPlan) != 1 {
		t.Fatalf("发起节点应有 1 条计划动作，实际 %#v", plans)
	}
	if startPlan[0].ActionName != "提交" {
		t.Fatalf("动作名应为中文「提交」，实际 %s", startPlan[0].ActionName)
	}
	if startPlan[0].SourceName != "用户配置" || startPlan[0].ScopeName != "发起实例" {
		t.Fatalf("来源与作用范围应输出中文，实际 %s / %s", startPlan[0].SourceName, startPlan[0].ScopeName)
	}
	if startPlan[0].ParameterCount != 1 {
		t.Fatalf("动作参数只给项数，实际 %d", startPlan[0].ParameterCount)
	}
	auditPlan := plans["node-audit"]
	if len(auditPlan) != 1 || auditPlan[0].SourceName != "系统导航" || auditPlan[0].ScopeName != "当前待办" {
		t.Fatalf("导航步骤应归到审批节点并输出中文来源，实际 %#v", auditPlan)
	}
}

// TestNodePlansKeepUnmappedTokenKey 锁定降级行为：真实结构变化导致令牌键翻译不出图节点 ID 时，
// 计划必须按原键保留，绝不静默丢掉这个节点的配置（画布读不到就退化为不显示，而不是假装没有配置）。
func TestNodePlansKeepUnmappedTokenKey(t *testing.T) {
	compiled := []model.CompiledActionStep{{Sequence: 1, Action: model.ActionApprove, NodeKey: "token-unknown"}}

	plans := service.BuildNodePlansForTest(compiled, map[string]string{})

	if len(plans["token-unknown"]) != 1 {
		t.Fatalf("翻译不出图节点 ID 时应按原键保留计划，实际 %#v", plans)
	}
	if plans["token-unknown"][0].ActionName != "同意" {
		t.Fatalf("动作名应为中文「同意」，实际 %s", plans["token-unknown"][0].ActionName)
	}
}
