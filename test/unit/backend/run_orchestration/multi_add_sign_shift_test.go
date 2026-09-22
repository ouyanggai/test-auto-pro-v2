package run_orchestration_test

import (
	"strings"
	"testing"

	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/engine/control"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// chainShiftGraph 构造一条纯链图：ids 为节点 ID 序列，types 为对应类型，
// 结构路径按「root + /child×深度」生成，与目标代理树分析器的树路径口径一致。
func chainShiftGraph(ids []string, types []string) model.FlowGraph {
	nodes := make([]model.FlowGraphNode, 0, len(ids))
	edges := make([]model.FlowGraphEdge, 0, len(ids)-1)
	for index, id := range ids {
		nodes = append(nodes, model.FlowGraphNode{
			ID: id, Type: types[index],
			StructuralPath: "root" + strings.Repeat("/child", index),
		})
		if index > 0 {
			edges = append(edges, model.FlowGraphEdge{
				ID: ids[index-1] + "|" + id + "|sequence|", Source: ids[index-1], Target: id, Kind: "sequence",
			})
		}
	}
	return model.FlowGraph{EntryNodeIDs: []string{ids[0]}, Nodes: nodes, Edges: edges}
}

// TestMultipleAddSignShiftsProjectBackToPlanIDs 锁定同一链路连续多次加签后的结构绑定：
// 每次加签让锚点之后子链的结构路径整体多出一跳，位移按锚点累加（1 段、2 段、3 段）。
// 旧实现只容错恰好一段位移，第二次加签之后的所有节点投影失败，
// 画布退化为目标动态 UUID、步骤 nodeId 全空、节点状态全部退回未开始（运行 124 的真实缺陷）。
func TestMultipleAddSignShiftsProjectBackToPlanIDs(t *testing.T) {
	staticIDs := []string{"n-start", "n-audit", "n-synergy", "n-branch", "n-prov", "n-leader", "n-sun",
		"n-route1", "n-leader2", "n-route2", "n-gm", "n-final"}
	staticTypes := []string{"start", "common", "synergy", "manual", "common", "common", "common",
		"condition", "common", "condition", "common", "synergy"}
	staticGraph := chainShiftGraph(staticIDs, staticTypes)

	// 动态树：三次加签全部发生在链路上，且前两次插入节点的真实 ID 已随后续代理树重建更换
	//（重建后整树换 ID），只有最后一次插入节点 ID 仍与已落账注入步骤一致。
	dynamicIDs := []string{"d-start", "d-audit", "dx-1", "d-synergy", "d-branch", "d-prov", "d-leader",
		"d-sun", "dx-2", "d-route1", "d-leader2", "dx-3", "d-route2", "d-gm", "d-final"}
	dynamicTypes := []string{"start", "common", "common", "synergy", "manual", "common", "common",
		"common", "common", "condition", "common", "common", "condition", "common", "synergy"}
	dynamicGraph := chainShiftGraph(dynamicIDs, dynamicTypes)

	// 锚点令牌来自已落账的三次加签步骤（锚点分别为 n-audit、n-sun、n-leader2），
	// 位移量据此推导：dx-1 之后 +1 段、dx-2 之后 +2 段、dx-3 之后 +3 段。
	anchorTokens := []string{
		analyzer.PathConfigNodeToken("n-audit"),
		analyzer.PathConfigNodeToken("n-sun"),
		analyzer.PathConfigNodeToken("n-leader2"),
	}
	projected, ok := service.ProjectRuntimeGraphForTest(staticGraph, dynamicGraph, []string{"dx-3"}, anchorTokens)
	if !ok {
		t.Fatalf("三次加签位移的动态图应能唯一投影回计划 ID 空间")
	}
	projectedIDs := make(map[string]string, len(projected.Nodes))
	for _, node := range projected.Nodes {
		projectedIDs[node.ID] = node.Type
	}
	for _, staticID := range staticIDs {
		if _, exists := projectedIDs[staticID]; !exists {
			t.Fatalf("静态节点 %s 投影后丢失，投影节点：%v", staticID, projectedIDs)
		}
	}
	// 前两次插入节点的真实 ID 已无法与任何计划节点对应，必须保留动态 ID 单独展示。
	for _, insertedID := range []string{"dx-1", "dx-2", "dx-3"} {
		if _, exists := projectedIDs[insertedID]; !exists {
			t.Fatalf("加签插入节点 %s 应保留真实动态 ID 展示", insertedID)
		}
	}
}

// TestMultipleAddSignShiftsMapStepTokensWhenProjectionDegrades 锁定投影失败时的兜底映射：
// 已落账步骤的静态 Token 必须全部映射到事实图（动态代理图）节点，
// 位移 2 段、3 段的节点与位移 1 段的节点同等对待，控制层重绑定事实缺失时结构匹配兜底。
func TestMultipleAddSignShiftsMapStepTokensWhenProjectionDegrades(t *testing.T) {
	staticIDs := []string{"n-start", "n-audit", "n-synergy", "n-branch", "n-prov", "n-leader", "n-sun",
		"n-route1", "n-leader2", "n-route2", "n-gm", "n-final"}
	staticTypes := []string{"start", "common", "synergy", "manual", "common", "common", "common",
		"condition", "common", "condition", "common", "synergy"}
	staticGraph := chainShiftGraph(staticIDs, staticTypes)

	dynamicIDs := []string{"d-start", "d-audit", "dx-1", "d-synergy", "d-branch", "d-prov", "d-leader",
		"d-sun", "dx-2", "d-route1", "d-leader2", "dx-3", "d-route2", "d-gm", "d-final"}
	dynamicTypes := []string{"start", "common", "common", "synergy", "manual", "common", "common",
		"common", "common", "condition", "common", "common", "condition", "common", "synergy"}
	factGraph := service.EnsureVirtualEndNodeForTest(chainShiftGraph(dynamicIDs, dynamicTypes))

	// 期望对应关系：n1→d1 精确；n2 起 +1 段；n7/n8 +2 段；n9 起 +3 段。
	expect := map[string]string{
		"n-start": "d-start", "n-audit": "d-audit", "n-synergy": "d-synergy", "n-branch": "d-branch",
		"n-prov": "d-prov", "n-leader": "d-leader", "n-sun": "d-sun",
		"n-route1": "d-route1", "n-leader2": "d-leader2",
		"n-route2": "d-route2", "n-gm": "d-gm", "n-final": "d-final",
	}
	anchorTokens := []string{
		analyzer.PathConfigNodeToken("n-audit"),
		analyzer.PathConfigNodeToken("n-sun"),
		analyzer.PathConfigNodeToken("n-leader2"),
	}
	mapping := service.BindStepTokensToFactGraphForTest(staticGraph, factGraph, nil, anchorTokens, []string{"dx-3"}, nil)
	for staticID, dynamicID := range expect {
		token := analyzer.PathConfigNodeToken(staticID)
		if got := mapping[token]; got != dynamicID {
			t.Fatalf("静态 Token %s 应映射到 %s，实际 %q", token, dynamicID, got)
		}
		if got := mapping[staticID]; got != dynamicID {
			t.Fatalf("静态节点 %s 应直连映射到 %s，实际 %q", staticID, dynamicID, got)
		}
	}
}

// TestMultipleAddSignShiftsLightCanvasStates 端到端锁定运行 124 缺陷的用户可见后果：
// 多次加签后重新打开运行详情，所有已落账步骤必须点亮对应动态节点为已完成，
// 不得遗留任何「按原始 Token 裸键」漂浮的孤立状态。
func TestMultipleAddSignShiftsLightCanvasStates(t *testing.T) {
	staticIDs := []string{"n-start", "n-audit", "n-synergy", "n-branch", "n-prov", "n-leader", "n-sun",
		"n-route1", "n-leader2", "n-route2", "n-gm", "n-final"}
	staticTypes := []string{"start", "common", "synergy", "manual", "common", "common", "common",
		"condition", "common", "condition", "common", "synergy"}
	staticGraph := chainShiftGraph(staticIDs, staticTypes)

	dynamicIDs := []string{"d-start", "d-audit", "dx-1", "d-synergy", "d-branch", "d-prov", "d-leader",
		"d-sun", "dx-2", "d-route1", "d-leader2", "dx-3", "d-route2", "d-gm", "d-final"}
	dynamicTypes := []string{"start", "common", "common", "synergy", "manual", "common", "common",
		"common", "common", "condition", "common", "common", "condition", "common", "synergy"}
	factGraph := service.EnsureVirtualEndNodeForTest(chainShiftGraph(dynamicIDs, dynamicTypes))

	// 步骤事实：12 个计划节点各一步 + 三次加签的插入审批各一步。
	// dx-3 的合成键记录的是当前真实 ID；dx-1/dx-2 的合成键记录的是已被后续代理树重建更换的旧 ID
	//（运行 124 的真实形状），必须靠锚点插入位置对齐，而不是靠合成键里的 ID。
	steps := make([]model.RunStep, 0, len(staticIDs)+3)
	for _, staticID := range staticIDs {
		steps = append(steps, model.RunStep{NodeKey: analyzer.PathConfigNodeToken(staticID), Status: model.RunStepSucceeded})
	}
	steps = append(steps,
		model.RunStep{Action: string(model.ActionAddSign), NodeKey: analyzer.PathConfigNodeToken("n-audit"), Status: model.RunStepSucceeded},
		model.RunStep{Action: "approve", NodeKey: control.AddSignInsertedNodeKey("lost-inserted-1"), Status: model.RunStepSucceeded},
		model.RunStep{Action: string(model.ActionAddSign), NodeKey: analyzer.PathConfigNodeToken("n-sun"), Status: model.RunStepSucceeded},
		model.RunStep{Action: "approve", NodeKey: control.AddSignInsertedNodeKey("lost-inserted-2"), Status: model.RunStepSucceeded},
		model.RunStep{Action: string(model.ActionAddSign), NodeKey: analyzer.PathConfigNodeToken("n-leader2"), Status: model.RunStepSucceeded},
		model.RunStep{Action: "approve", NodeKey: control.AddSignInsertedNodeKey("dx-3"), Status: model.RunStepSucceeded},
	)
	anchorTokens := []string{
		analyzer.PathConfigNodeToken("n-audit"),
		analyzer.PathConfigNodeToken("n-sun"),
		analyzer.PathConfigNodeToken("n-leader2"),
	}
	mapping := service.BindStepTokensToFactGraphForTest(staticGraph, factGraph, nil, anchorTokens, []string{"dx-3"}, steps)
	states := service.BuildNodeStatesWithTokenMapForTest(factGraph, steps, model.PathRun{Status: model.PathRunStatusCompleted}, nil, nil, mapping)
	for _, dynamicID := range dynamicIDs {
		if got := states[dynamicID].StatusName; got != model.PathRunStatusName(model.PathRunStatusCompleted) {
			t.Fatalf("动态节点 %s 应为已完成，实际 %s", dynamicID, got)
		}
	}
	if got := states[service.VirtualRunEndNodeIDForTest()].StatusName; got != model.PathRunStatusName(model.PathRunStatusCompleted) {
		t.Fatalf("完成时虚拟结束节点应为已完成，实际 %s", got)
	}
	if len(states) != len(dynamicIDs)+1 {
		t.Fatalf("节点状态不应再有裸 Token 漂浮键：期望 %d 项，实际 %d 项", len(dynamicIDs)+1, len(states))
	}
}
