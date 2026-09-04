package run_readiness_test

import (
	"testing"

	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// branchGraph 是一条带单选路由的真实结构：入口 -> 路由 -> 两条分支各自到达自己的结束节点。
func branchGraph() model.FlowGraph {
	return model.FlowGraph{
		EntryNodeIDs: []string{"start"},
		Nodes: []model.FlowGraphNode{
			{ID: "start", Name: "发起", Type: "start"},
			{ID: "route", Name: "条件路由", Type: "condition"},
			{ID: "end-a", Name: "同意结束", Type: "end"},
			{ID: "end-b", Name: "驳回结束", Type: "end"},
		},
		Edges: []model.FlowGraphEdge{
			{ID: "start-route", Source: "start", Target: "route", Kind: "sequence"},
			{ID: "edge-a", Source: "route", Target: "end-a", Kind: "condition", BranchID: "branch-a"},
			{ID: "edge-b", Source: "route", Target: "end-b", Kind: "condition", BranchID: "branch-b"},
		},
	}
}

// parallelGraph 是并行支线汇入同一个结束节点的真实结构：该结束节点会被到达两次。
func parallelGraph() model.FlowGraph {
	return model.FlowGraph{
		EntryNodeIDs: []string{"start"},
		Nodes: []model.FlowGraphNode{
			{ID: "start", Name: "发起", Type: "parallel"},
			{ID: "left", Name: "左支审批", Type: "audit"},
			{ID: "right", Name: "右支审批", Type: "audit"},
			{ID: "end-all", Name: "汇合结束", Type: "end"},
		},
		Edges: []model.FlowGraphEdge{
			{ID: "start-left", Source: "start", Target: "left", Kind: "parallel"},
			{ID: "start-right", Source: "start", Target: "right", Kind: "parallel"},
			{ID: "left-end", Source: "left", Target: "end-all", Kind: "sequence"},
			{ID: "right-end", Source: "right", Target: "end-all", Kind: "sequence"},
		},
	}
}

// candidatesFor 用真实分析器算出该选择集合的可达线路，再推导结束节点候选。
func candidatesFor(t *testing.T, graph model.FlowGraph, choices []model.ExecutionPathChoice) []model.SuccessAssertionEndNodeCandidate {
	t.Helper()
	analysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, choices)
	if err != nil {
		t.Fatalf("分析真实线路失败：%v", err)
	}
	return service.SuccessAssertionCandidates(graph, analysis)
}

// TestCandidatesComeFromRealLineOnly 验证候选只来自该路径真实走到的结束节点，
// 没被选中的分支上的结束节点不得出现在候选里。
func TestCandidatesComeFromRealLineOnly(t *testing.T) {
	candidates := candidatesFor(t, branchGraph(), []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}})
	if len(candidates) != 1 {
		t.Fatalf("候选应只有本路径真实到达的一个结束节点：%+v", candidates)
	}
	if candidates[0].NodeKey != "end-a" || candidates[0].Name != "同意结束" {
		t.Fatalf("候选节点不正确：%+v", candidates[0])
	}
	if candidates[0].ArrivalCount != 1 {
		t.Fatalf("顺序线路的结束节点只应到达一次：%d", candidates[0].ArrivalCount)
	}
}

// TestParallelConvergenceCountsEachArrival 验证并行支线汇入同一结束节点时按支线数量计到达次数。
func TestParallelConvergenceCountsEachArrival(t *testing.T) {
	candidates := candidatesFor(t, parallelGraph(), nil)
	if len(candidates) != 1 || candidates[0].NodeKey != "end-all" {
		t.Fatalf("并行结构的候选不正确：%+v", candidates)
	}
	if candidates[0].ArrivalCount != 2 {
		t.Fatalf("两条并行支线汇入同一结束节点应计两次到达：%d", candidates[0].ArrivalCount)
	}
}

// TestStatusOptionsComeFromTargetEnum 验证期望状态只用目标平台真实取值与目标自己的中文标签。
func TestStatusOptionsComeFromTargetEnum(t *testing.T) {
	options := model.FlowInstanceStatusOptions()
	if len(options) != 8 {
		t.Fatalf("目标真实实例状态应为八个取值：%d", len(options))
	}
	for _, option := range options {
		if option.Label == "" {
			t.Fatalf("状态 %s 缺少目标中文标签", option.Value)
		}
		if !model.IsFlowInstanceStatus(option.Value) {
			t.Fatalf("状态 %s 不在校验集合内", option.Value)
		}
	}
	if model.IsFlowInstanceStatus("finished") {
		t.Fatal("集合外的自造状态必须被拒绝")
	}
}

// realShapeGraph 复刻真实目标模板的形态：没有任何 end 类型节点，流程直接终止在审批节点上，
// 并且带一个尚未选择分支的路由。实测本机目标环境的节点类型只有
// start、manual、condition、common、empty、synergy。
func realShapeGraph() model.FlowGraph {
	return model.FlowGraph{
		EntryNodeIDs: []string{"start"},
		Nodes: []model.FlowGraphNode{
			{ID: "start", Name: "发起人", Type: "start"},
			{ID: "audit", Name: "发起人部门领导", Type: "common"},
			{ID: "route", Name: "路由", Type: "condition"},
			{ID: "tail", Name: "财资部-会计", Type: "common"},
			{ID: "unreached", Name: "未走到的审批", Type: "common"},
		},
		Edges: []model.FlowGraphEdge{
			{ID: "start-audit", Source: "start", Target: "audit", Kind: "sequence"},
			{ID: "audit-route", Source: "audit", Target: "route", Kind: "sequence"},
			{ID: "route-tail", Source: "route", Target: "tail", Kind: "condition", BranchID: "pass"},
			{ID: "route-other", Source: "route", Target: "unreached", Kind: "condition", BranchID: "other"},
		},
	}
}

// TestCandidatesFallBackToTerminalNodesWhenTargetHasNoEndType 锁定实测发现的现实：
// 真实模板可能一个 end 类型节点都没有，只认类型会让候选永远为空、断言在真实数据上没法配。
// 因此结束节点按结构事实判断——走到就没有可达后继的节点。
func TestCandidatesFallBackToTerminalNodesWhenTargetHasNoEndType(t *testing.T) {
	graph := realShapeGraph()
	for _, node := range graph.Nodes {
		if node.Type == "end" {
			t.Fatal("这个夹具刻意不含 end 类型节点")
		}
	}
	candidates := candidatesFor(t, graph, []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "pass"}})
	if len(candidates) != 1 || candidates[0].NodeKey != "tail" {
		t.Fatalf("应把没有可达后继的审批节点当成结束节点：%+v", candidates)
	}
	if candidates[0].Name != "财资部-会计" {
		t.Fatalf("候选应带上真实中文节点名：%+v", candidates[0])
	}
}

// TestPendingRouteIsNeverOfferedAsEndNode 验证尚未选择分支的路由节点不会被当成结束节点。
// 遍历在待选路由处停住，它同样没有可达后继，必须排除。
func TestPendingRouteIsNeverOfferedAsEndNode(t *testing.T) {
	candidates := candidatesFor(t, realShapeGraph(), nil)
	for _, candidate := range candidates {
		if candidate.NodeKey == "route" {
			t.Fatalf("待选路由被当成了结束节点：%+v", candidates)
		}
	}
}
