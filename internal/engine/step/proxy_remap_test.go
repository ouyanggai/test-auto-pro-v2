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
	// 3. 加签后目标可能立即把当前节点返回为新插入的审批节点；只要它位于锚点之后，
	// 结构上就是合法的加签结果，不能再按旧锚点严格相等误报路径不一致。
	remap.InsertedHumanNodeIDs = []string{"new-sign"}
	remap.GraphNodeTypes["new-sign"] = "common"
	remap.GraphEdges["task-a"] = []GraphEdgeInfo{{Target: "new-sign"}}
	if err := ValidateAddSignCurrentNode("new-sign", "task-a", remap); err != nil {
		t.Fatalf("新插入节点位于锚点之后应放行：%v", err)
	}

	// 4. 不匹配且不是并行祖先或新插入节点：应报错阻断
	if err := ValidateAddSignCurrentNode("other", "task-a", remap); err == nil {
		t.Fatalf("无关节点不匹配必须报错阻断")
	}
}

// TestValidateAddSignUsesWrittenInstanceFactsForParallelCurrentNode 验证并行加签时接口响应游标
// 与写后实例活动节点不同，不应单独阻断代理重绑定；控制层会校验完整活动节点集合。
func TestValidateAddSignUsesWrittenInstanceFactsForParallelCurrentNode(t *testing.T) {
	remap := ProxyRemap{
		GraphNodeTypes: map[string]string{
			"anchor-new": "common", "current-from-instance": "common",
		},
		GraphEdges: map[string][]GraphEdgeInfo{
			"anchor-new": {{Target: "current-from-instance"}},
		},
	}
	if err := ValidateCurrentNodesInRemap([]string{"current-from-instance"}, remap); err != nil {
		t.Fatalf("写后实例当前节点属于新代理树时应通过：%v", err)
	}
}

// TestValidateCurrentNodesInRemapRejectsUnprovableFacts 验证加签后缺少或失真的活动节点事实时必须停步。
func TestValidateCurrentNodesInRemapRejectsUnprovableFacts(t *testing.T) {
	remap := ProxyRemap{
		GraphNodeTypes: map[string]string{"known-node": "common"},
	}
	tests := []struct {
		name string
		ids  []string
	}{
		{name: "活动节点为空", ids: nil},
		{name: "活动节点标识为空", ids: []string{""}},
		{name: "活动节点不在新树", ids: []string{"unknown-node"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateCurrentNodesInRemap(tt.ids, remap); err == nil {
				t.Fatalf("ValidateCurrentNodesInRemap(%v) 应阻断无法证明的写后事实", tt.ids)
			}
		})
	}
}

// TestBuildProxyRemapReorderedParallelBranches 验证加签导致新树并行分支颠倒时，重映射依然准确对齐，绝不张冠李戴。
func TestBuildProxyRemapReorderedParallelBranches(t *testing.T) {
	// 旧树：分支0是资金副主任，分支1是张政
	oldTree := &target.FlowNodeTemplate{
		ID: "old-root", Type: "start",
		Child: &target.FlowNodeTemplate{
			ID: "old-p", Type: "parallel",
			ParallelNodes: []target.FlowBranchTemplate{
				{
					ID:   "old-b2",
					Name: "节点2",
					Sort: 1,
					Child: &target.FlowNodeTemplate{
						ID:   "old-zj",
						Name: "财资部-资金副主任",
						Type: "common",
					},
				},
				{
					ID:   "old-b1",
					Name: "节点1",
					Sort: 1,
					Child: &target.FlowNodeTemplate{
						ID:   "old-zz",
						Name: "张政",
						Type: "common",
					},
				},
			},
		},
	}

	// 新树：目标平台加签后把张政排到了第0位，资金副主任排到了第1位，且张政后插入了宋恺
	newTree := &target.FlowNodeTemplate{
		ID: "new-root", Type: "start",
		Child: &target.FlowNodeTemplate{
			ID: "new-p", Type: "parallel",
			ParallelNodes: []target.FlowBranchTemplate{
				{
					ID:   "new-b1",
					Name: "节点1",
					Sort: 1,
					Child: &target.FlowNodeTemplate{
						ID:   "new-zz",
						Name: "张政",
						Type: "common",
						Child: &target.FlowNodeTemplate{
							ID:   "new-sign",
							Name: "宋恺",
							Type: "common",
						},
					},
				},
				{
					ID:   "new-b2",
					Name: "节点2",
					Sort: 1,
					Child: &target.FlowNodeTemplate{
						ID:   "new-zj",
						Name: "财资部-资金副主任",
						Type: "common",
					},
				},
			},
		},
	}

	executor := &Executor{target: remapTreeTarget{tree: newTree}}
	runCtx := RunContext{
		FlowTree: oldTree,
		Nodes: map[string]NodeInfo{
			"k_zj": {TargetNodeID: "old-zj", Name: "财资部-资金副主任"},
			"k_zz": {TargetNodeID: "old-zz", Name: "张政"},
		},
	}

	remap, err := executor.BuildProxyRemap(context.Background(), runCtx, target.Session{}, "proxy-new", "old-zz")
	if err != nil {
		t.Fatalf("BuildProxyRemap() error = %v", err)
	}

	// 核心断言：必须准确对齐，绝对不能颠倒
	if got := remap.NodeIDs["old-zj"]; got != "new-zj" {
		t.Errorf("资金副主任应映射为 new-zj，实际为 %q", got)
	}
	if got := remap.NodeIDs["old-zz"]; got != "new-zz" {
		t.Errorf("张政应映射为 new-zz，实际为 %q", got)
	}
	if len(remap.InsertedHumanNodeIDs) != 1 || remap.InsertedHumanNodeIDs[0] != "new-sign" {
		t.Errorf("新插入加签节点应为 new-sign，实际为 %v", remap.InsertedHumanNodeIDs)
	}
}
