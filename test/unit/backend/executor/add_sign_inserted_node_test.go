package executor_test

import (
	"context"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// TestBuildProxyRemapSurfacesInsertedHumanNode 验证加签后代理树重绑定会确定性地暴露新插入的独立
// 审批节点（按结构路径，不按名称猜测）：旧节点整体平移映射，新树里未绑定到任何旧节点的人工节点
// 即为加签插入节点，供运行时作为独立步骤处理，而不是被原节点固定尾同意吞并。
func TestBuildProxyRemapSurfacesInsertedHumanNode(t *testing.T) {
	oldTree := &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{
		ID: "node-a", Type: "common", Child: &target.FlowNodeTemplate{ID: "node-next", Type: "common"},
	}}
	// 加签在锚点 node-a 之后插入一个独立审批节点 node-inserted。
	newTree := &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{
		ID: "node-a", Type: "common", Child: &target.FlowNodeTemplate{
			ID: "node-inserted", Type: "common", Child: &target.FlowNodeTemplate{ID: "node-next", Type: "common"},
		},
	}}
	targetFake := &taskActionFactsTarget{
		fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run"}},
		tree:       newTree,
	}
	runCtx := newRunContext([]model.CompiledActionStep{})
	runCtx.FlowTree = oldTree
	runCtx.Nodes = map[string]step.NodeInfo{
		"a":    {Name: "A", Type: "common", TargetNodeID: "node-a"},
		"next": {Name: "Next", Type: "common", TargetNodeID: "node-next"},
	}
	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	remap, err := executor.BuildProxyRemap(context.Background(), runCtx, target.Session{}, "proxy-new", "node-a")
	if err != nil {
		t.Fatalf("加签重绑定应成功：%v", err)
	}
	if len(remap.InsertedHumanNodeIDs) != 1 || remap.InsertedHumanNodeIDs[0] != "node-inserted" {
		t.Fatalf("应确定性暴露插入的加签审批节点，实际：%v", remap.InsertedHumanNodeIDs)
	}
	if remap.NodeIDs["node-a"] != "node-a" || remap.NodeIDs["node-next"] != "node-next" {
		t.Fatalf("旧节点应整体平移唯一映射，实际：%v", remap.NodeIDs)
	}
}

// TestBuildProxyRemapAnchorMustBeRealCurrentNode 复现真实加签失败：当前节点经协同/条件流转后已前移，
// 加签写请求用真实当前节点（协同节点）作锚点插入，其后是带分支的条件结构。用真实锚点重绑定能穿过
// 分支唯一绑定并暴露插入节点；用离线编译配置的靠前节点当锚点则整棵子链平移错位、绑定失败。
func TestBuildProxyRemapAnchorMustBeRealCurrentNode(t *testing.T) {
	// 旧树：start → common(A) → synergy(锚点) → condition(分支) → synergy(尾)；条件两分支各挂一个审批节点。
	oldTree := &target.FlowNodeTemplate{ID: "s", Type: "start", Child: &target.FlowNodeTemplate{
		ID: "a", Type: "common", Child: &target.FlowNodeTemplate{
			ID: "anchor", Type: "synergy", Child: &target.FlowNodeTemplate{
				ID: "cond", Type: "condition",
				ConditionNodes: []target.FlowBranchTemplate{
					{ID: "br0", Sort: 0, Child: &target.FlowNodeTemplate{ID: "b1", Type: "common"}},
					{ID: "br1", Sort: 1, Child: &target.FlowNodeTemplate{ID: "b2", Type: "common"}},
				},
				Child: &target.FlowNodeTemplate{ID: "tail", Type: "synergy"},
			},
		},
	}}
	// 新树：目标在 synergy 锚点之后插入独立审批节点 ins，其后的条件子链整体下移一跳；所有 ID 重新生成。
	newTree := &target.FlowNodeTemplate{ID: "s2", Type: "start", Child: &target.FlowNodeTemplate{
		ID: "a2", Type: "common", Child: &target.FlowNodeTemplate{
			ID: "anchor2", Type: "synergy", Child: &target.FlowNodeTemplate{
				ID: "ins", Type: "common", Child: &target.FlowNodeTemplate{
					ID: "cond2", Type: "condition",
					ConditionNodes: []target.FlowBranchTemplate{
						{ID: "br0b", Sort: 0, Child: &target.FlowNodeTemplate{ID: "b1b", Type: "common"}},
						{ID: "br1b", Sort: 1, Child: &target.FlowNodeTemplate{ID: "b2b", Type: "common"}},
					},
					Child: &target.FlowNodeTemplate{ID: "tail2", Type: "synergy"},
				},
			},
		},
	}}
	newExecutor := func() *step.Executor {
		return step.NewExecutor(&taskActionFactsTarget{
			fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run"}},
			tree:       newTree,
		}, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)
	}
	baseCtx := func() step.RunContext {
		runCtx := newRunContext([]model.CompiledActionStep{})
		runCtx.FlowTree = oldTree
		runCtx.Nodes = map[string]step.NodeInfo{
			"a": {Name: "A", Type: "common", TargetNodeID: "a"},
			"anchor": {Name: "锚点", Type: "synergy", TargetNodeID: "anchor"},
			"tail": {Name: "尾", Type: "synergy", TargetNodeID: "tail"},
		}
		return runCtx
	}

	// 真实锚点（协同节点）→ 穿过分支唯一绑定并暴露插入节点。
	remap, err := newExecutor().BuildProxyRemap(context.Background(), baseCtx(), target.Session{}, "proxy-new", "anchor")
	if err != nil {
		t.Fatalf("用真实当前节点作锚点应能唯一重绑定：%v", err)
	}
	if remap.NodeIDs["anchor"] != "anchor2" || remap.NodeIDs["cond"] != "cond2" || remap.NodeIDs["b1"] != "b1b" || remap.NodeIDs["tail"] != "tail2" {
		t.Fatalf("真实锚点重绑定映射不正确：%v", remap.NodeIDs)
	}
	if len(remap.InsertedHumanNodeIDs) != 1 || remap.InsertedHumanNodeIDs[0] != "ins" {
		t.Fatalf("应暴露插入的加签审批节点 ins，实际：%v", remap.InsertedHumanNodeIDs)
	}

	// 错误锚点（离线配置的靠前 common 节点）→ 子链平移错位，绑定失败。
	if _, err := newExecutor().BuildProxyRemap(context.Background(), baseCtx(), target.Session{}, "proxy-new", "a"); err == nil {
		t.Fatal("用错误的靠前节点当锚点应绑定失败（证明锚点必须是真实当前节点）")
	}
}

// TestBuildProxyRemapNoInsertedNodeOnIsomorphicRebuild 验证同构重建（仅人员明细变化、无新增人工节点）
// 不会误报插入节点。
func TestBuildProxyRemapNoInsertedNodeOnIsomorphicRebuild(t *testing.T) {
	tree := &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{
		ID: "node-a", Type: "common", Child: &target.FlowNodeTemplate{ID: "node-next", Type: "common"},
	}}
	targetFake := &taskActionFactsTarget{
		fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run"}},
		tree:       tree,
	}
	runCtx := newRunContext([]model.CompiledActionStep{})
	runCtx.FlowTree = tree
	runCtx.Nodes = map[string]step.NodeInfo{
		"a":    {Name: "A", Type: "common", TargetNodeID: "node-a"},
		"next": {Name: "Next", Type: "common", TargetNodeID: "node-next"},
	}
	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	remap, err := executor.BuildProxyRemap(context.Background(), runCtx, target.Session{}, "proxy-new", "")
	if err != nil {
		t.Fatalf("同构重建应成功：%v", err)
	}
	if len(remap.InsertedHumanNodeIDs) != 0 {
		t.Fatalf("同构重建不应报告插入节点，实际：%v", remap.InsertedHumanNodeIDs)
	}
}
