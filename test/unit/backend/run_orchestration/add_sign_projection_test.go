package run_orchestration_test

import (
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/engine/control"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// linearTree 构造一条直链：发起人 -> 锚点审批 -> 后继审批 -> 结束。
// withInsert=true 时在锚点之后、后继之前插入一个新的 common 审批节点（加签的真实形状）。
func linearTree(prefix string, withInsert bool) *target.FlowNodeTemplate {
	successor := &target.FlowNodeTemplate{
		ID: prefix + "-next", Type: "common", Name: "后继审批",
		Child: &target.FlowNodeTemplate{ID: prefix + "-end", Type: "end", Name: "结束"},
	}
	afterAnchor := successor
	if withInsert {
		afterAnchor = &target.FlowNodeTemplate{
			ID: prefix + "-inserted", Type: "common", Name: "加签审批节点",
			Child: successor,
		}
	}
	return &target.FlowNodeTemplate{
		ID: prefix + "-root", Type: "start", Name: "发起人",
		Child: &target.FlowNodeTemplate{
			ID: prefix + "-anchor", Type: "common", Name: "锚点审批",
			Child: afterAnchor,
		},
	}
}

// TestAddSignInsertedNodeSurvivesProjection 复现用户报告：加签成功后运行画布看不到插入的节点。
// 静态计划图没有插入节点，动态代理树（加签后）多一个 common 节点并使后继整体位移一段。
// 断言：投影必须可绑定，且插入节点以独立 ID 出现在投影结果里（不能被后继吞掉或整图丢弃）。
func TestAddSignInsertedNodeSurvivesProjection(t *testing.T) {
	analyzerImpl := analyzer.NewFlowGraphAnalyzer()
	staticNodes, staticEdges, _, err := analyzerImpl.Analyze(linearTree("plan", false))
	if err != nil {
		t.Fatalf("分析计划树失败：%v", err)
	}
	dynamicNodes, dynamicEdges, _, err := analyzerImpl.Analyze(linearTree("proxy", true))
	if err != nil {
		t.Fatalf("分析加签后代理树失败：%v", err)
	}
	staticGraph := model.FlowGraph{Nodes: staticNodes, Edges: staticEdges}
	dynamicGraph := model.FlowGraph{Nodes: dynamicNodes, Edges: dynamicEdges}

	// proxy-inserted 是加签插入节点的权威真实 ID（真实运行里来自已落账注入步骤）。
	projected, bindable := service.ProjectRuntimeGraphForTest(staticGraph, dynamicGraph, "proxy-inserted")
	if !bindable {
		t.Fatalf("加签后代理树合法，投影必须可绑定，实际 bindable=false（前端会丢掉动态图导致看不到加签节点）")
	}
	// 计划 4 个节点 + 加签 1 个 = 投影后必须是 5 个节点。
	if len(projected.Nodes) != len(staticNodes)+1 {
		t.Fatalf("投影节点数 = %d，期望 %d（计划 %d + 加签 1）", len(projected.Nodes), len(staticNodes)+1, len(staticNodes))
	}
	// 插入节点必须以目标真实 ID（proxy-inserted）单独存在，名称为加签节点名。
	foundInserted := false
	for _, node := range projected.Nodes {
		if node.ID == "proxy-inserted" {
			foundInserted = true
			if node.Name != "加签审批节点" {
				t.Fatalf("加签节点名称丢失：%+v", node)
			}
		}
	}
	if !foundInserted {
		t.Fatalf("投影结果里没有加签插入节点（proxy-inserted），节点列表：%+v", projectedIDs(projected))
	}
	// 画布入口必须是结构根（发起人节点），不是目标运行态返回的「当前节点」：否则前端从中段节点
	// BFS 覆盖不到全图，会判定动态图不可用并回退静态计划图，加签节点随之消失（复现 run33 症状）。
	if len(projected.EntryNodeIDs) != 1 || projected.EntryNodeIDs[0] != "plan-root" {
		t.Fatalf("投影入口应为结构根 plan-root，实际 %+v", projected.EntryNodeIDs)
	}
	// 从入口出发必须能走到每一个投影节点（含加签插入节点），否则前端 BFS 覆盖不全。
	incoming := map[string]bool{}
	for _, e := range projected.Edges {
		incoming[e.Target] = true
	}
	reach := map[string]bool{}
	queue := append([]string{}, projected.EntryNodeIDs...)
	adj := map[string][]string{}
	for _, e := range projected.Edges {
		adj[e.Source] = append(adj[e.Source], e.Target)
	}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if reach[n] {
			continue
		}
		reach[n] = true
		queue = append(queue, adj[n]...)
	}
	for _, node := range projected.Nodes {
		if !reach[node.ID] {
			t.Fatalf("投影入口无法到达节点 %s（%s），前端画布会判定动态图不可用", node.ID, node.Name)
		}
	}
}

func projectedIDs(graph model.FlowGraph) []string {
	ids := make([]string, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		ids = append(ids, node.ID)
	}
	return ids
}

// TestAddSignInsertedNodeRunStateBindsToRealNode 复现用户报告的第二半：即便加签节点画出来了，
// 它也没有运行/选中状态。插入节点的步骤键是合成键「add-sign-inserted-<真实ID>」，必须翻译回真实
// 图节点 ID，否则「已完成/运行中」落到幽灵合成键上，真实节点永远停在「未开始」。
func TestAddSignInsertedNodeRunStateBindsToRealNode(t *testing.T) {
	// 投影后的图里，加签节点以真实 ID（proxy-inserted）单独存在。
	graph := model.FlowGraph{Nodes: []model.FlowGraphNode{
		{ID: "plan-root"}, {ID: "plan-anchor"}, {ID: "proxy-inserted", Name: "加签审批节点"}, {ID: "plan-next"},
	}}
	// 已落账步骤：锚点同意用配置令牌键，加签插入节点用合成键（真实运行里注入步骤就是这个键）。
	steps := []model.RunStep{
		{NodeKey: analyzer.PathConfigNodeToken("plan-anchor"), Status: model.RunStepSucceeded},
		{NodeKey: control.AddSignInsertedNodeKey("proxy-inserted"), Status: model.RunStepSucceeded},
	}
	pathRun := model.PathRun{Status: model.PathRunStatusRunning}

	states := service.BuildNodeStatesForTest(graph, steps, pathRun, nil, nil)

	if got := states["proxy-inserted"].StatusName; got != model.PathRunStatusName(model.PathRunStatusCompleted) {
		t.Fatalf("加签插入节点的已完成状态没有落到真实图节点上，实际 %s（合成键未翻译回真实 ID）", got)
	}
	if got := states["plan-anchor"].StatusName; got != model.PathRunStatusName(model.PathRunStatusCompleted) {
		t.Fatalf("锚点节点应已完成，实际 %s", got)
	}
	// 当前预览停在加签插入节点时，也必须按合成键翻译出「运行中」。
	preview := &service.RunPreviewDTO{NodeKey: control.AddSignInsertedNodeKey("proxy-inserted")}
	running := service.BuildNodeStatesForTest(graph, steps[:1], pathRun, preview, nil)
	if got := running["proxy-inserted"].StatusName; got != model.PathRunStatusName(model.PathRunStatusRunning) {
		t.Fatalf("当前停在加签节点时应标记运行中，实际 %s", got)
	}
}
