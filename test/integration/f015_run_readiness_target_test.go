package integration_test

import (
	"context"
	"testing"

	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/model"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
	"test-auto-pro-v2/internal/service"
)

// TestF015CandidatesComeFromRealTargetStructure 连真实目标做只读复验：
// 断言候选必须全部来自真实流程结构里类型为结束的可达节点，工具不得凭空造出候选。
// 同时回归 F-014 记录的无 Redis 流程图详情链路：读不到真实结构就直接失败，不静默降级。
func TestF015CandidatesComeFromRealTargetStructure(t *testing.T) {
	planDB := config.LoadPlanDBConfig()
	if missing := planDB.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-015 只读复验要求测试数据库配置，缺失：%v", missing)
	}
	targetConfig := config.LoadTargetConfig()
	if missing := targetConfig.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-015 只读复验要求真实目标配置，缺失：%v", missing)
	}
	graph, path := f015RealGraphAndPath(t)
	analysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, path.Choices)
	if err != nil {
		// 选择在当前真实结构里不成立本身就是运行准备要拦住的情况，这里如实失败而不是跳过。
		t.Fatalf("真实路径的分支选择在当前真实流程结构里不成立：%v", err)
	}
	candidates := service.SuccessAssertionCandidates(graph, analysis)
	reachable := map[string]bool{}
	for _, nodeID := range analysis.ReachableNodeIDs {
		reachable[nodeID] = true
	}
	reachableEdges := map[string]bool{}
	for _, edgeID := range analysis.ReachableEdgeIDs {
		reachableEdges[edgeID] = true
	}
	hasOutgoing := map[string]bool{}
	for _, edge := range graph.Edges {
		if reachableEdges[edge.ID] {
			hasOutgoing[edge.Source] = true
		}
	}
	nodeType := map[string]string{}
	for _, node := range graph.Nodes {
		nodeType[node.ID] = node.Type
	}
	pending := map[string]bool{}
	for _, nodeID := range analysis.MissingRouteNodeIDs {
		pending[nodeID] = true
	}
	if len(candidates) == 0 {
		t.Fatal("真实路径必须至少推导出一个结束节点候选，否则成功断言在真实数据上无法配置")
	}
	for _, candidate := range candidates {
		if !reachable[candidate.NodeKey] {
			t.Fatalf("候选 %s 不在这条路径的真实可达线路上", candidate.NodeKey)
		}
		// 结束节点的结构事实：走到就没有可达后继；目标显式给出 end 类型时也算。
		if hasOutgoing[candidate.NodeKey] && nodeType[candidate.NodeKey] != "end" {
			t.Fatalf("候选 %s 在本路径上仍有可达后继，不是结束节点", candidate.NodeKey)
		}
		if pending[candidate.NodeKey] {
			t.Fatalf("候选 %s 是尚未选择分支的路由节点，不得当成结束节点", candidate.NodeKey)
		}
		if candidate.ArrivalCount == 0 {
			t.Fatalf("候选 %s 的到达次数不能为 0", candidate.NodeKey)
		}
	}
	t.Logf("本路径候选 %d 个，路径选择完整=%v，真实结构里 end 类型节点 %d 个",
		len(candidates), analysis.Complete, countNodeType(graph, "end"))
	// 目标真实状态集合必须与判定共用同一份来源。
	if len(model.FlowInstanceStatusOptions()) != 8 {
		t.Fatal("目标真实实例状态取值应为八个")
	}
}

// f015RealGraphAndPath 读一条真实计划的真实流程结构与第一条执行路径。
// 用工具自己的服务组装，确保走的是与运行准备相同的读取链路。
func f015RealGraphAndPath(t *testing.T) (model.FlowGraph, model.ExecutionPath) {
	t.Helper()
	database, err := planmysql.OpenAndMigrate(context.Background(), config.LoadPlanDBConfig())
	if err != nil {
		t.Fatalf("打开计划数据库失败：%v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	plans := service.NewPlanService(planmysql.NewPlanRepository(database.DB))
	list, err := plans.List(context.Background(), "", "")
	if err != nil {
		t.Fatalf("读取计划列表失败：%v", err)
	}
	pathRepository := planmysql.NewExecutionPathRepository(database.DB)
	graphs := service.NewFlowGraphService(plans, service.NewTargetReadService(config.LoadTargetConfig()), analyzer.NewFlowGraphAnalyzer())
	for _, plan := range list {
		summaries, listErr := pathRepository.List(context.Background(), plan.ID)
		if listErr != nil || len(summaries) == 0 {
			continue
		}
		detail, getErr := pathRepository.Get(context.Background(), plan.ID, summaries[0].ID)
		if getErr != nil {
			continue
		}
		graph, graphErr := graphs.Get(context.Background(), plan.ID)
		if graphErr != nil {
			t.Fatalf("读取真实流程结构失败，无 Redis 详情链路可能已变：%v", graphErr)
		}
		return graph, detail
	}
	t.Fatal("测试数据库里没有任何带执行路径的计划，无法做真实结构复验")
	return model.FlowGraph{}, model.ExecutionPath{}
}

// countNodeType 统计真实结构里某个节点类型的数量，只用于日志说明。
func countNodeType(graph model.FlowGraph, nodeType string) int {
	count := 0
	for _, node := range graph.Nodes {
		if node.Type == nodeType {
			count++
		}
	}
	return count
}
