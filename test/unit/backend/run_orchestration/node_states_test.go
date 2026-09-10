package run_orchestration_test

import (
	"testing"

	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// TestF016WaitingNodesDerivedFromConfiguredRoute 复核评审缺陷 9 的修复回归：
// 「等待运行」必须依据已配置路线的节点序列推导——已配置、未落账、无失败标记的节点等待运行；
// 路线外节点保持未开始。旧实现遍历已落账步骤检查自身，条件恒不成立，该状态从未出现过。
func TestF016WaitingNodesDerivedFromConfiguredRoute(t *testing.T) {
	graph := model.FlowGraph{Nodes: []model.FlowGraphNode{
		{ID: "node-start"}, {ID: "node-audit"}, {ID: "node-next"}, {ID: "node-outside"},
	}}
	// 步骤与配置序列携带的是配置令牌键（与图节点 ID 不同键空间）：
	// 状态必须被翻译回图节点 ID 输出，否则画布永远读不到（评审 P1 的回归锁定）。
	token := analyzer.PathConfigNodeToken("node-start")
	steps := []model.RunStep{{NodeKey: token, Status: model.RunStepSucceeded}}
	pathRun := model.PathRun{Status: model.PathRunStatusRunning}
	configured := []string{token, analyzer.PathConfigNodeToken("node-audit"), analyzer.PathConfigNodeToken("node-next")}

	states := service.BuildNodeStatesForTest(graph, steps, pathRun, nil, configured)

	if states["node-start"].StatusName != model.PathRunStatusName(model.PathRunStatusCompleted) {
		t.Fatalf("已落账节点应已完成，实际 %s", states["node-start"].StatusName)
	}
	for _, nodeID := range []string{"node-audit", "node-next"} {
		if states[nodeID].StatusName != model.PathRunStatusName(model.PathRunStatusWaiting) {
			t.Fatalf("已配置未到达节点 %s 应为等待运行，实际 %s", nodeID, states[nodeID].StatusName)
		}
	}
	if states["node-outside"].StatusName != model.PathRunStatusName(model.PathRunStatusNotStarted) {
		t.Fatalf("路线外节点应保持未开始，实际 %s", states["node-outside"].StatusName)
	}
}

// TestF016StructureWarningStartsAfterExecution 验证尚未放行第一步时不把画布降级误报成运行状态异常。
func TestF016StructureWarningStartsAfterExecution(t *testing.T) {
	if note := service.StructureNoteForTest(true, false); note != "" {
		t.Fatalf("尚无执行事实时不应显示结构降级警告：%q", note)
	}
	if note := service.StructureNoteForTest(true, true); note == "" {
		t.Fatal("执行开始后结构读取失败应保留明确警告")
	}
	if note := service.StructureNoteForTest(false, true); note != "" {
		t.Fatalf("结构读取正常时不应显示警告：%q", note)
	}
}
