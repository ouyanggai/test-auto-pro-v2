package step

import (
	"context"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
)

// remapTreeTarget 仅提供代理树读取能力，其余目标能力由嵌入接口占位。
// BuildProxyRemap 的映射阶段不应触碰写能力，因此未实现的方法不会被调用。
type remapTreeTarget struct {
	TargetClient
	tree *target.FlowNodeTemplate
}

// ReadProxyTree 返回测试用的加签后代理树。
func (t remapTreeTarget) ReadProxyTree(context.Context, target.Session, string) (*target.FlowNodeTemplate, error) {
	return t.tree, nil
}

// TestBuildProxyRemapMapsThroughUniqueInsertedChild 验证给出插入锚点后，锚点子链上的旧节点
// 按插入后路径确定性重绑定，不会被新插入节点抢占。
func TestBuildProxyRemapMapsThroughUniqueInsertedChild(t *testing.T) {
	oldTree := &target.FlowNodeTemplate{
		ID: "old-root", Type: "start",
		Child: &target.FlowNodeTemplate{ID: "old-approve", Type: "approve"},
	}
	newTree := &target.FlowNodeTemplate{
		ID: "new-root", Type: "start",
		Child: &target.FlowNodeTemplate{
			ID: "new-sign", Type: "add_sign",
			Child: &target.FlowNodeTemplate{ID: "new-approve", Type: "approve"},
		},
	}
	executor := &Executor{target: remapTreeTarget{tree: newTree}}
	runCtx := RunContext{
		FlowTree: oldTree,
		Nodes:    map[string]NodeInfo{"approval": {TargetNodeID: "old-approve"}},
	}

	remap, err := executor.BuildProxyRemap(context.Background(), runCtx, target.Session{}, "proxy-new", "old-root")
	if err != nil {
		t.Fatalf("BuildProxyRemap() error = %v", err)
	}
	if got := remap.NodeIDs["old-approve"]; got != "new-approve" {
		t.Fatalf("old approval mapped to %q, want new-approve", got)
	}
}

// TestBuildProxyRemapSameTypeInsertionBindsByAnchor 验证同类型节点插入时必须依赖锚点：
// 给出锚点后旧后继按插入后路径精确绑定，绝不会被新插入节点抢占（无锚点的精确匹配
// 无法区分两者，因此控制层对加签重建强制要求锚点，见 control.go 的守卫）。
func TestBuildProxyRemapSameTypeInsertionBindsByAnchor(t *testing.T) {
	oldTree := &target.FlowNodeTemplate{
		ID: "old-root", Type: "start",
		Child: &target.FlowNodeTemplate{ID: "old-approve", Type: "common"},
	}
	newTree := &target.FlowNodeTemplate{
		ID: "new-root", Type: "start",
		Child: &target.FlowNodeTemplate{
			ID: "new-sign", Type: "common",
			Child: &target.FlowNodeTemplate{ID: "new-approve", Type: "common"},
		},
	}
	executor := &Executor{target: remapTreeTarget{tree: newTree}}
	runCtx := RunContext{
		FlowTree: oldTree,
		Nodes:    map[string]NodeInfo{"approval": {TargetNodeID: "old-approve"}},
	}

	remap, err := executor.BuildProxyRemap(context.Background(), runCtx, target.Session{}, "proxy-new", "old-root")
	if err != nil {
		t.Fatalf("BuildProxyRemap() error = %v", err)
	}
	if got := remap.NodeIDs["old-approve"]; got != "new-approve" {
		t.Fatalf("old approval mapped to %q, want new-approve", got)
	}
	// 锚点自身不平移：仍按原路径绑定到新树同位置节点。
	if got := remap.NodeIDs["old-root"]; got != "new-root" {
		t.Fatalf("old root mapped to %q, want new-root", got)
	}
	// 锚点不在旧树中时无法定位插入位置，必须拒绝。
	if _, err := executor.BuildProxyRemap(context.Background(), runCtx, target.Session{}, "proxy-new", "missing-anchor"); err == nil {
		t.Fatal("BuildProxyRemap() with unknown anchor error = nil, want failure")
	}
}

// TestBuildProxyRemapRejectsDuplicateNodeIdentity 验证目标树出现重复节点标识时继续阻止放行。
func TestBuildProxyRemapRejectsDuplicateNodeIdentity(t *testing.T) {
	oldTree := &target.FlowNodeTemplate{
		ID: "old-root", Type: "start",
		Child: &target.FlowNodeTemplate{
			ID: "old-middle", Type: "empty",
			Child: &target.FlowNodeTemplate{ID: "old-approve", Type: "approve"},
		},
	}
	newTree := &target.FlowNodeTemplate{
		ID: "new-root", Type: "start",
		Child: &target.FlowNodeTemplate{
			ID: "new-middle", Type: "empty",
			Child: &target.FlowNodeTemplate{
				ID: "new-approve", Type: "approve",
				Child: &target.FlowNodeTemplate{ID: "new-approve", Type: "approve"},
			},
		},
	}
	executor := &Executor{target: remapTreeTarget{tree: newTree}}
	runCtx := RunContext{
		FlowTree: oldTree,
		Nodes:    map[string]NodeInfo{"approval": {TargetNodeID: "old-approve"}},
	}

	_, err := executor.BuildProxyRemap(context.Background(), runCtx, target.Session{}, "proxy-new", "")
	if err == nil || !strings.Contains(err.Error(), "结构不可证明") {
		t.Fatalf("BuildProxyRemap() error = %v, want unprovable-structure failure", err)
	}
}
// TestValidateAddSignCurrentNode 验证常规线性加签精确匹配、并行网关分流祖先放行以及不匹配阻断。
func TestValidateAddSignCurrentNode(t *testing.T) {
	remap := ProxyRemap{
		GraphNodeTypes: map[string]string{
			"p-gateway": "parallel",
			"task-a":    "common",
			"task-b":    "common",
			"other":     "common",
		},
		GraphEdges: map[string][]GraphEdgeInfo{
			"p-gateway": {{Target: "task-a"}, {Target: "task-b"}},
		},
	}

	// 1. 常规精确匹配通过
	if err := ValidateAddSignCurrentNode("task-a", "task-a", remap); err != nil {
		t.Fatalf("精确匹配不应报错：%v", err)
	}

	// 2. 并行网关场景：响应当前节点为并行网关，映射锚点为分支子节点，应放行
	if err := ValidateAddSignCurrentNode("p-gateway", "task-a", remap); err != nil {
		t.Fatalf("并行祖先网关不应报错：%v", err)
	}

	// 3. 不匹配且不是并行祖先：应报错阻断
	if err := ValidateAddSignCurrentNode("other", "task-a", remap); err == nil {
		t.Fatalf("无关节点不匹配必须报错阻断")
	}
}
