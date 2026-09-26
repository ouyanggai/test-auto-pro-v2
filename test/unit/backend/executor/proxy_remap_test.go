package executor_test

import (
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
)

// TestValidateCurrentNodesInRemapRequiresKnownCurrentNodes 验证取回写后必须拿到非空且属于新代理树的当前节点，
// 缺失或不属于新树时不能沿用旧节点继续放行。
func TestValidateCurrentNodesInRemapRequiresKnownCurrentNodes(t *testing.T) {
	remap := step.ProxyRemap{GraphNodeTypes: map[string]string{"new-current": "common"}}
	if err := step.ValidateCurrentNodesInRemap(nil, remap); err == nil {
		t.Fatal("写后缺少当前节点时必须阻断代理重绑定")
	}
	if err := step.ValidateCurrentNodesInRemap([]string{"old-current"}, remap); err == nil {
		t.Fatal("写后当前节点不在新代理树时必须阻断代理重绑定")
	}
	if err := step.ValidateCurrentNodesInRemap([]string{"new-current", "new-current"}, remap); err != nil {
		t.Fatalf("新代理树中的当前节点应通过校验：%v", err)
	}
}

func TestApplyProxyRemapRemapsWholeRunContext(t *testing.T) {
	runCtx := step.RunContext{
		FlowTree: &target.FlowNodeTemplate{ID: "old-root"},
		Nodes: map[string]step.NodeInfo{
			"first":  {TargetNodeID: "old-root"},
			"second": {TargetNodeID: "old-next"},
		},
		GraphEdges: map[string][]step.GraphEdgeInfo{
			"old-root": {{Target: "old-next", BranchID: "old-branch"}},
		},
		GraphNodeTypes:           map[string]string{"old-root": "start", "old-next": "common"},
		BranchSelections:         map[string]string{"old-root": "old-next"},
		SubmitBranchTargetNodeID: "old-next",
		NextNodeAuditors: map[string][]target.NextAuditor{
			"second": {{Name: "张三", BizID: "user-1", NodeProxyID: "old-next"}},
		},
	}
	remap := step.ProxyRemap{
		FlowTree: &target.FlowNodeTemplate{ID: "new-root"},
		NodeIDs:  map[string]string{"old-root": "new-root", "old-next": "new-next"},
		GraphEdges: map[string][]step.GraphEdgeInfo{
			"new-root": {{Target: "new-next", BranchID: "new-branch"}},
		},
		GraphNodeTypes:           map[string]string{"new-root": "start", "new-next": "common"},
		BranchSelections:         map[string]string{"new-root": "new-next"},
		SubmitBranchTargetNodeID: "new-next",
		NextNodeAuditors: map[string][]target.NextAuditor{
			"second": {{Name: "张三", BizID: "user-1", NodeProxyID: "new-next"}},
		},
	}

	if err := step.ApplyProxyRemap(&runCtx, remap); err != nil {
		t.Fatalf("ApplyProxyRemap() error = %v", err)
	}
	if got := runCtx.Nodes["first"].TargetNodeID; got != "new-root" {
		t.Fatalf("first node id = %q, want new-root", got)
	}
	if got := runCtx.Nodes["second"].TargetNodeID; got != "new-next" {
		t.Fatalf("second node id = %q, want new-next", got)
	}
	if got := runCtx.GraphEdges["new-root"][0].Target; got != "new-next" {
		t.Fatalf("edge target = %q, want new-next", got)
	}
	if got := runCtx.BranchSelections["new-root"]; got != "new-next" {
		t.Fatalf("branch target = %q, want new-next", got)
	}
	if got := runCtx.SubmitBranchTargetNodeID; got != "new-next" {
		t.Fatalf("submit branch target = %q, want new-next", got)
	}
	if got := runCtx.NextNodeAuditors["second"][0].NodeProxyID; got != "new-next" {
		t.Fatalf("auditor node id = %q, want new-next", got)
	}
	if got := runCtx.FlowTree.ID; got != "new-root" {
		t.Fatalf("flow tree id = %q, want new-root", got)
	}
}

// TestApplyProxyRemapRejectsUnmappedConfiguredNode 验证配置节点漏映射时不能留下半更新上下文。
func TestApplyProxyRemapRejectsUnmappedConfiguredNode(t *testing.T) {
	runCtx := step.RunContext{Nodes: map[string]step.NodeInfo{"configured": {TargetNodeID: "old-node"}}}
	remap := step.ProxyRemap{
		FlowTree: &target.FlowNodeTemplate{ID: "new-root"},
		NodeIDs:  map[string]string{"other-node": "new-other"},
	}
	if err := step.ApplyProxyRemap(&runCtx, remap); err == nil {
		t.Fatal("ApplyProxyRemap() error = nil, want unmapped node error")
	}
}
