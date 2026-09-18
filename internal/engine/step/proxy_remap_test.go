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

// TestBuildProxyRemapMapsThroughUniqueInsertedChild 验证插入异类型 child 后旧节点仍能唯一重绑定。
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

	remap, err := executor.BuildProxyRemap(context.Background(), runCtx, target.Session{}, "proxy-new")
	if err != nil {
		t.Fatalf("BuildProxyRemap() error = %v", err)
	}
	if got := remap.NodeIDs["old-approve"]; got != "new-approve" {
		t.Fatalf("old approval mapped to %q, want new-approve", got)
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

	_, err := executor.BuildProxyRemap(context.Background(), runCtx, target.Session{}, "proxy-new")
	if err == nil || !strings.Contains(err.Error(), "结构不可证明") {
		t.Fatalf("BuildProxyRemap() error = %v, want unprovable-structure failure", err)
	}
}
