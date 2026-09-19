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
