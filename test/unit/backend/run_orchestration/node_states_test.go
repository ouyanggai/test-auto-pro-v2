package run_orchestration_test

import (
	"testing"
	"time"

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

// TestF037FailedStepKeepsNodeFact 验证第 9 步确定失败时，失败节点不会因刷新详情重新变成未开始。
func TestF037FailedStepKeepsNodeFact(t *testing.T) {
	graph := model.FlowGraph{Nodes: []model.FlowGraphNode{{ID: "node-start"}, {ID: "node-submit"}}}
	token := analyzer.PathConfigNodeToken("node-submit")
	startToken := analyzer.PathConfigNodeToken("node-start")
	steps := []model.RunStep{
		{NodeKey: startToken, Status: model.RunStepSucceeded},
		{NodeKey: token, Status: model.RunStepFailed},
	}
	failure := model.FailureClassToolBug
	pathRun := model.PathRun{Status: model.PathRunStatusFailed, FailureClass: &failure}
	states := service.BuildNodeStatesForTest(graph, steps, pathRun, nil, []string{token})
	if got := states["node-submit"].StatusName; got != model.PathRunStatusName(model.PathRunStatusFailed) {
		t.Fatalf("失败步骤刷新后应保持失败，实际 %s", got)
	}
	if got := states["node-start"].StatusName; got != model.PathRunStatusName(model.PathRunStatusCompleted) {
		t.Fatalf("此前已落账节点不应被刷新抹成未开始，实际 %s", got)
	}
}

// TestF016RunDetailGraphReadBudget 验证运行详情不会用低于一秒的预算误杀正常的目标结构读取。
func TestF016RunDetailGraphReadBudget(t *testing.T) {
	if timeout := service.RunDetailGraphTimeoutForTest(); timeout < time.Second {
		t.Fatalf("运行详情结构读取预算过短：%s", timeout)
	}
}

// TestF039VerifyingKeepsCurrentNodeVisible 验证写后核验期间当前节点不被清空，
// 状态直接显示“确认结果中”，避免画布在当前节点与下一节点之间出现空白期。
func TestF039VerifyingKeepsCurrentNodeVisible(t *testing.T) {
	graph := model.FlowGraph{Nodes: []model.FlowGraphNode{{ID: "node-audit"}, {ID: "node-next"}}}
	preview := &service.RunPreviewDTO{NodeKey: analyzer.PathConfigNodeToken("node-audit")}
	states := service.BuildNodeStatesForTest(graph, nil, model.PathRun{Status: model.PathRunStatusVerifying}, preview, []string{preview.NodeKey})
	if got := states["node-audit"].StatusName; got != model.PathRunStatusName(model.PathRunStatusVerifying) {
		t.Fatalf("核验期间当前节点应保持确认中，实际 %s", got)
	}
	if got := states["node-next"].StatusName; got == model.PathRunStatusName(model.PathRunStatusRunning) || got == model.PathRunStatusName(model.PathRunStatusVerifying) {
		t.Fatalf("下一节点不应在当前节点核验完成前提前显示活动态，实际 %s", got)
	}
}

// TestParallelActiveNodesHighlightedTogether 验证三个真实并行待办一起高亮，汇合节点仍等待。
func TestParallelActiveNodesHighlightedTogether(t *testing.T) {
	graph := model.FlowGraph{Nodes: []model.FlowGraphNode{
		{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "after"},
	}}
	token := analyzer.PathConfigNodeToken("a")
	preview := &service.RunPreviewDTO{NodeKey: token, Facts: map[string]any{
		"currentNodes": []string{"target-a", "target-b", "target-c"},
		"dueNodes":     []string{"target-b"},
	}}
	tokenMap := map[string]string{token: "a", "token-b": "b", "token-c": "c"}
	targetMap := map[string]string{token: "target-a", "token-b": "target-b", "token-c": "target-c"}
	active := service.ActiveRunNodeIDsForTest(graph, preview, tokenMap, targetMap)
	if len(active) != 3 {
		t.Fatalf("三个并行节点都应呈现活动状态：%v", active)
	}
	states := service.BuildNodeStatesWithTokenMapForTest(graph, nil, model.PathRun{Status: model.PathRunStatusRunning},
		preview, []string{"a", "b", "c", "after"}, tokenMap, active...)
	for _, id := range []string{"a", "b", "c"} {
		if states[id].Status != string(model.PathRunStatusRunning) {
			t.Fatalf("活动分支 %s 未高亮：%+v", id, states)
		}
	}
	if states["after"].Status != string(model.PathRunStatusWaiting) {
		t.Fatalf("全部分支完成前汇合后继只能等待：%+v", states)
	}
}

// TestF039CompletedGraphHasStableVirtualEnd 验证完成态补出的虚拟结束节点和终端连线稳定且幂等。
func TestF039CompletedGraphHasStableVirtualEnd(t *testing.T) {
	graph := model.FlowGraph{
		EntryNodeIDs: []string{"start"},
		Nodes:        []model.FlowGraphNode{{ID: "start", Type: "start"}, {ID: "audit", Type: "common"}},
		Edges:        []model.FlowGraphEdge{{ID: "start|audit|sequence|", Source: "start", Target: "audit", Kind: "sequence"}},
	}
	first := service.EnsureVirtualEndNodeForTest(graph)
	second := service.EnsureVirtualEndNodeForTest(first)
	if len(first.Nodes) != 3 || len(first.Edges) != 2 {
		t.Fatalf("完成态应补 1 个结束节点和 1 条终端边，实际 nodes=%d edges=%d", len(first.Nodes), len(first.Edges))
	}
	if first.Nodes[2].ID != "__run_virtual_end__" || first.Nodes[2].Type != "end" || first.Nodes[2].Name != "结束" {
		t.Fatalf("虚拟结束节点形状错误：%+v", first.Nodes[2])
	}
	if first.Edges[1].Source != "audit" || first.Edges[1].Target != "__run_virtual_end__" {
		t.Fatalf("终端边错误：%+v", first.Edges[1])
	}
	if len(second.Nodes) != len(first.Nodes) || len(second.Edges) != len(first.Edges) {
		t.Fatalf("重复投影不应重复追加结束节点或连线：第一次=%d/%d 第二次=%d/%d", len(first.Nodes), len(first.Edges), len(second.Nodes), len(second.Edges))
	}
}

// TestF039VirtualEndStateFollowsPathCompletion 锁定结束节点的运行态只由「整条路径是否走到头」决定：
// 完成时点亮为成功（画布上绿色高亮），未完成保持未开始。结束节点不参与「等待运行」的路线语义，
// 否则它会因为不在已配置节点序列里而永远停在「未开始」，完成时也不会变绿。
func TestF039VirtualEndStateFollowsPathCompletion(t *testing.T) {
	graph := model.FlowGraph{
		EntryNodeIDs: []string{"start"},
		Nodes:        []model.FlowGraphNode{{ID: "start", Type: "start"}, {ID: "audit", Type: "common"}},
		Edges:        []model.FlowGraphEdge{{ID: "start|audit|sequence|", Source: "start", Target: "audit", Kind: "sequence"}},
	}
	endGraph := service.EnsureVirtualEndNodeForTest(graph)

	completed := service.BuildNodeStatesForTest(endGraph, nil, model.PathRun{Status: model.PathRunStatusCompleted}, nil, nil)
	if got := completed[service.VirtualRunEndNodeIDForTest()].StatusName; got != model.PathRunStatusName(model.PathRunStatusCompleted) {
		t.Fatalf("路径完成时结束节点应为已完成，实际 %s", got)
	}

	running := service.BuildNodeStatesForTest(endGraph, nil, model.PathRun{Status: model.PathRunStatusRunning}, nil, nil)
	if got := running[service.VirtualRunEndNodeIDForTest()].StatusName; got != model.PathRunStatusName(model.PathRunStatusNotStarted) {
		t.Fatalf("路径未完成时结束节点应保持未开始（不显示状态文案），实际 %s", got)
	}
	if got := running[service.VirtualRunEndNodeIDForTest()].StatusName; got == model.PathRunStatusName(model.PathRunStatusWaiting) {
		t.Fatalf("结束节点不应进入「等待运行」，它只表达整条路径是否走到头，实际 %s", got)
	}
}

// TestStructuralPathsMapTokensWhenControlSessionCleared 验证当控制层现场被释放时，
// 静态 Token 与动态图节点能通过结构路径成功对齐，避免完成态刷新后节点全部退化为未开始。
func TestStructuralPathsMapTokensWhenControlSessionCleared(t *testing.T) {
	// staticGraph 节点
	staticGraph := model.FlowGraph{Nodes: []model.FlowGraphNode{
		{ID: "plan-start", StructuralPath: "root"},
		{ID: "plan-audit", StructuralPath: "root/child"},
	}}
	_ = staticGraph
	// factGraph 为动态代理图
	factGraph := model.FlowGraph{Nodes: []model.FlowGraphNode{
		{ID: "dyn-start", StructuralPath: "root"},
		{ID: "dyn-audit", StructuralPath: "root/child"},
	}}
	staticStartToken := analyzer.PathConfigNodeToken("plan-start")
	staticAuditToken := analyzer.PathConfigNodeToken("plan-audit")
	steps := []model.RunStep{
		{NodeKey: staticStartToken, Status: model.RunStepSucceeded},
		{NodeKey: staticAuditToken, Status: model.RunStepSucceeded},
	}
	pathRun := model.PathRun{Status: model.PathRunStatusCompleted}

	// 模拟未开启 dynamicProjectionOK 且 control session 已清除时的结构路径映射
	tokenToGraphID := map[string]string{
		staticStartToken: "dyn-start",
		staticAuditToken: "dyn-audit",
	}

	states := service.BuildNodeStatesWithTokenMapForTest(factGraph, steps, pathRun, nil, nil, tokenToGraphID)

	if got := states["dyn-start"].StatusName; got != model.PathRunStatusName(model.PathRunStatusCompleted) {
		t.Fatalf("静态 Token 应通过结构路径映射到动态节点 dyn-start，实际 %s", got)
	}
	if got := states["dyn-audit"].StatusName; got != model.PathRunStatusName(model.PathRunStatusCompleted) {
		t.Fatalf("静态 Token 应通过结构路径映射到动态节点 dyn-audit，实际 %s", got)
	}
}

// TestComplexBranchWithVirtualEndPaths 验证包含复杂分支与虚拟结束节点的图能够成功计算结构路径。
func TestComplexBranchWithVirtualEndPaths(t *testing.T) {
	// 构造多条件分支汇合树并补齐虚拟结束节点
	tree := branchTree("test", true)
	analyzerImpl := analyzer.NewFlowGraphAnalyzer()
	nodes, edges, _, err := analyzerImpl.Analyze(tree)
	if err != nil {
		t.Fatalf("Analyze 失败：%v", err)
	}
	nodesWithoutEnd := make([]model.FlowGraphNode, 0, len(nodes))
	for _, n := range nodes {
		if n.Type != "end" {
			nodesWithoutEnd = append(nodesWithoutEnd, n)
		}
	}
	edgesWithoutEnd := make([]model.FlowGraphEdge, 0, len(edges))
	for _, e := range edges {
		if e.Target != "test-end" {
			edgesWithoutEnd = append(edgesWithoutEnd, e)
		}
	}
	graph := service.EnsureVirtualEndNodeForTest(model.FlowGraph{Nodes: nodesWithoutEnd, Edges: edgesWithoutEnd})
	paths, ok := service.FlowGraphStructuralPathsForTest(graph)
	if !ok || len(paths) == 0 {
		t.Fatalf("带有虚拟结束节点的复杂分支树应成功计算结构路径，实际 ok=%v, paths len=%d", ok, len(paths))
	}
	if paths[service.VirtualRunEndNodeIDForTest()] != "root/__end__" {
		t.Fatalf("虚拟结束节点路径应为 root/__end__，实际 %s", paths[service.VirtualRunEndNodeIDForTest()])
	}
}
