package step

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
)

// ProxyRemap 是加签重建代理树后一次性替换运行上下文所需的事实集合。
// 映射只接受唯一结构路径，不按名称猜测；新树中未映射的节点保留为动态节点供详情展示。
type ProxyRemap struct {
	FlowTree                 *target.FlowNodeTemplate
	NodeIDs                  map[string]string
	GraphEdges               map[string][]GraphEdgeInfo
	GraphNodeTypes           map[string]string
	BranchSelections         map[string]string
	SubmitBranchTargetNodeID string
	NextNodeAuditors         map[string][]target.NextAuditor
}

// BuildProxyRemap 读取加签后的完整代理树，并按旧树与新树的唯一结构路径建立节点映射。
// 读取失败、路径缺失或结构不唯一时返回错误，调用方必须停止后续放行，不能继续使用旧节点 ID。
func (e *Executor) BuildProxyRemap(ctx context.Context, runCtx RunContext, session target.Session, flowProxyID string) (ProxyRemap, error) {
	flowProxyID = strings.TrimSpace(flowProxyID)
	if flowProxyID == "" {
		return ProxyRemap{}, fmt.Errorf("加签响应缺少新的流程代理标识")
	}
	if runCtx.FlowTree == nil {
		return ProxyRemap{}, fmt.Errorf("运行上下文缺少加签前流程树，无法证明节点对应关系")
	}
	reader, ok := e.target.(flowProxyTreeReader)
	if !ok {
		return ProxyRemap{}, fmt.Errorf("目标客户端不支持读取加签后的流程代理树")
	}
	newTree, err := reader.ReadProxyTree(ctx, session, flowProxyID)
	if err != nil {
		return ProxyRemap{}, fmt.Errorf("读取加签后的流程代理树失败：%w", err)
	}
	if newTree == nil {
		return ProxyRemap{}, fmt.Errorf("加签后的流程代理树为空")
	}
	oldPaths, oldTypes, err := proxyTreeFacts(runCtx.FlowTree)
	if err != nil {
		return ProxyRemap{}, fmt.Errorf("加签前流程树结构不可证明：%w", err)
	}
	newPaths, newTypes, err := proxyTreeFacts(newTree)
	if err != nil {
		return ProxyRemap{}, fmt.Errorf("加签后流程树结构不可证明：%w", err)
	}
	newIDByPath := make(map[string]string, len(newPaths))
	for id, path := range newPaths {
		if previous, exists := newIDByPath[path]; exists && previous != id {
			return ProxyRemap{}, fmt.Errorf("加签后结构路径 %q 对应多个节点", path)
		}
		newIDByPath[path] = id
	}

	nodeIDs := make(map[string]string, len(oldPaths))
	usedNewIDs := make(map[string]string, len(oldPaths))
	for oldID, path := range oldPaths {
		candidates := make([]string, 0, 1)
		for newPath, newID := range newIDByPath {
			if newTypes[newID] != oldTypes[oldID] || !compatibleProxyStructurePath(path, newPath) {
				continue
			}
			candidates = append(candidates, newID)
		}
		if len(candidates) != 1 {
			return ProxyRemap{}, fmt.Errorf("旧节点 %q 的结构路径 %q 无法唯一绑定到加签后流程树", oldID, path)
		}
		newID := candidates[0]
		if previous, exists := usedNewIDs[newID]; exists && previous != oldID {
			return ProxyRemap{}, fmt.Errorf("加签后节点 %q 被旧节点 %q 和 %q 重复绑定", newID, previous, oldID)
		}
		usedNewIDs[newID] = oldID
		nodeIDs[oldID] = newID
	}
	for nodeKey, info := range runCtx.Nodes {
		oldID := strings.TrimSpace(info.TargetNodeID)
		if oldID == "" {
			continue
		}
		if _, exists := nodeIDs[oldID]; !exists {
			return ProxyRemap{}, fmt.Errorf("配置节点 %q 无法在加签后流程树中唯一绑定", nodeKey)
		}
	}

	newGraphNodes, newGraphEdges, _, err := analyzer.NewFlowGraphAnalyzer().Analyze(newTree)
	if err != nil {
		return ProxyRemap{}, fmt.Errorf("加签后流程树无法分析：%w", err)
	}
	graphEdges := make(map[string][]GraphEdgeInfo, len(newGraphNodes))
	graphNodeTypes := make(map[string]string, len(newGraphNodes))
	for _, node := range newGraphNodes {
		graphNodeTypes[node.ID] = node.Type
	}
	for _, edge := range newGraphEdges {
		graphEdges[edge.Source] = append(graphEdges[edge.Source], GraphEdgeInfo{Target: edge.Target, BranchID: edge.BranchID})
	}
	branchSelections := make(map[string]string, len(runCtx.BranchSelections))
	for oldRoute, oldTarget := range runCtx.BranchSelections {
		newRoute, routeOK := nodeIDs[strings.TrimSpace(oldRoute)]
		newTarget, targetOK := nodeIDs[strings.TrimSpace(oldTarget)]
		if !routeOK || !targetOK {
			return ProxyRemap{}, fmt.Errorf("已保存分支选择无法绑定到加签后流程树")
		}
		branchSelections[newRoute] = newTarget
	}
	submitTarget := ""
	if oldTarget := strings.TrimSpace(runCtx.SubmitBranchTargetNodeID); oldTarget != "" {
		var exists bool
		submitTarget, exists = nodeIDs[oldTarget]
		if !exists {
			return ProxyRemap{}, fmt.Errorf("提交分支目标无法绑定到加签后流程树")
		}
	}
	nextAuditors := cloneNextNodeAuditors(runCtx.NextNodeAuditors)
	for nodeKey, auditors := range nextAuditors {
		for index := range auditors {
			oldID := strings.TrimSpace(auditors[index].NodeProxyID)
			if oldID == "" {
				continue
			}
			newID, exists := nodeIDs[oldID]
			if !exists {
				return ProxyRemap{}, fmt.Errorf("下一节点处理人参数 %q 无法绑定到加签后流程树", nodeKey)
			}
			auditors[index].NodeProxyID = newID
		}
		nextAuditors[nodeKey] = auditors
	}
	return ProxyRemap{
		FlowTree: newTree, NodeIDs: nodeIDs, GraphEdges: graphEdges, GraphNodeTypes: graphNodeTypes,
		BranchSelections: branchSelections, SubmitBranchTargetNodeID: submitTarget, NextNodeAuditors: nextAuditors,
	}, nil
}

// RemapProxyTreeForRun 使用运行计划账号的当前目标会话读取新代理树并构造整体映射。
// 代理重建发生在写后核验阶段，控制层不能复用已结束的动作会话，也不能在读取失败时继续使用旧树。
func (e *Executor) RemapProxyTreeForRun(ctx context.Context, runCtx RunContext, flowProxyID string) (ProxyRemap, error) {
	account := strings.TrimSpace(runCtx.PlanAccount)
	if account == "" {
		return ProxyRemap{}, fmt.Errorf("运行上下文缺少计划账号，无法读取加签后的流程代理树")
	}
	session, err := e.sessions.Current(ctx, account)
	if err != nil && sessionRefreshAllowed(err) {
		session, err = e.sessions.Refresh(ctx, account)
	}
	if err != nil {
		return ProxyRemap{}, fmt.Errorf("取得读取加签代理树的目标会话失败：%w", err)
	}
	return e.BuildProxyRemap(ctx, runCtx, session, flowProxyID)
}

// ApplyProxyRemap 将加签后的代理结构整体应用到运行上下文，避免只更新当前动作节点留下旧 ID。
// 该函数在控制层应用前再次核对所有已配置节点，任何遗漏都必须阻止后续执行。
func ApplyProxyRemap(runCtx *RunContext, remap ProxyRemap) error {
	if runCtx == nil || remap.FlowTree == nil || len(remap.NodeIDs) == 0 {
		return fmt.Errorf("加签后的代理映射为空")
	}
	for key, info := range runCtx.Nodes {
		oldID := strings.TrimSpace(info.TargetNodeID)
		if oldID == "" {
			continue
		}
		newID, exists := remap.NodeIDs[oldID]
		if !exists {
			return fmt.Errorf("配置节点 %q 未完成代理节点重绑定", key)
		}
		info.TargetNodeID = newID
		runCtx.Nodes[key] = info
	}
	runCtx.FlowTree = remap.FlowTree
	runCtx.GraphEdges = cloneGraphEdges(remap.GraphEdges)
	runCtx.GraphNodeTypes = cloneGraphNodeTypes(remap.GraphNodeTypes)
	runCtx.BranchSelections = cloneStringMap(remap.BranchSelections)
	runCtx.SubmitBranchTargetNodeID = remap.SubmitBranchTargetNodeID
	runCtx.NextNodeAuditors = cloneNextNodeAuditors(remap.NextNodeAuditors)
	return nil
}

// proxyTreeFacts 为目标树的每个节点生成只依赖拓扑位置的结构路径和类型；分支按目标顺序编号，不读取名称作为匹配依据。
func proxyTreeFacts(root *target.FlowNodeTemplate) (map[string]string, map[string]string, error) {
	paths := make(map[string]string)
	types := make(map[string]string)
	visiting := make(map[string]bool)
	var visit func(*target.FlowNodeTemplate, string) error
	visit = func(node *target.FlowNodeTemplate, path string) error {
		if node == nil {
			return fmt.Errorf("结构路径 %q 指向空节点", path)
		}
		id := strings.TrimSpace(node.ID)
		if id == "" {
			return fmt.Errorf("结构路径 %q 缺少节点标识", path)
		}
		if visiting[id] {
			return fmt.Errorf("节点 %q 存在循环引用", id)
		}
		if previous, exists := paths[id]; exists && previous != path {
			return fmt.Errorf("节点 %q 同时位于 %q 和 %q", id, previous, path)
		}
		if _, exists := paths[id]; exists {
			return nil
		}
		paths[id] = path
		types[id] = strings.TrimSpace(node.Type)
		visiting[id] = true
		defer delete(visiting, id)
		branches, kind := proxyTreeBranches(node)
		if kind != "" {
			sort.SliceStable(branches, func(i, j int) bool { return branches[i].Sort < branches[j].Sort })
			for index, branch := range branches {
				if branch.Child == nil {
					return fmt.Errorf("结构路径 %q 的分支 %d 缺少子节点", path, index+1)
				}
				if err := visit(branch.Child, fmt.Sprintf("%s/%s/%d", path, kind, index)); err != nil {
					return err
				}
			}
		}
		if node.Child != nil {
			if err := visit(node.Child, path+"/child"); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(root, "root"); err != nil {
		return nil, nil, err
	}
	return paths, types, nil
}

// compatibleProxyStructurePath 判断旧结构路径是否能按顺序落在新结构路径上。
// 新树允许在 child 链中插入节点；分支层级和分支序号必须完全一致，避免把不同分支误配到一起。
func compatibleProxyStructurePath(oldPath, newPath string) bool {
	oldParts := strings.Split(oldPath, "/")
	newParts := strings.Split(newPath, "/")
	oldChildren, newChildren := 0, 0
	oldBranches, newBranches := make([]string, 0), make([]string, 0)
	for _, part := range oldParts {
		if part == "child" {
			oldChildren++
		} else if part != "root" {
			oldBranches = append(oldBranches, part)
		}
	}
	for _, part := range newParts {
		if part == "child" {
			newChildren++
		} else if part != "root" {
			newBranches = append(newBranches, part)
		}
	}
	if oldChildren > newChildren || len(oldBranches) != len(newBranches) {
		return false
	}
	for index := range oldBranches {
		if oldBranches[index] != newBranches[index] {
			return false
		}
	}
	return true
}

// proxyTreeBranches 返回目标树的分支副本，避免排序改变目标读取事实的原始顺序。
func proxyTreeBranches(node *target.FlowNodeTemplate) ([]target.FlowBranchTemplate, string) {
	if node == nil {
		return nil, ""
	}
	if node.Type == "parallel" {
		return append([]target.FlowBranchTemplate(nil), node.ParallelNodes...), "parallel"
	}
	if node.Type == "condition" {
		kind := "condition"
		if node.BranchExecuteType == "custom_choose" {
			kind = "manual"
		}
		return append([]target.FlowBranchTemplate(nil), node.ConditionNodes...), kind
	}
	return nil, ""
}

// cloneGraphEdges 复制图边表，确保映射后的运行上下文不会与临时结果共享可变切片。
func cloneGraphEdges(input map[string][]GraphEdgeInfo) map[string][]GraphEdgeInfo {
	result := make(map[string][]GraphEdgeInfo, len(input))
	for source, edges := range input {
		result[source] = append([]GraphEdgeInfo(nil), edges...)
	}
	return result
}

// cloneGraphNodeTypes 复制节点类型表，保持映射结果的独立所有权。
func cloneGraphNodeTypes(input map[string]string) map[string]string {
	result := make(map[string]string, len(input))
	for id, nodeType := range input {
		result[id] = nodeType
	}
	return result
}

// cloneStringMap 复制字符串映射，避免分支选择被后续控制现场意外修改。
func cloneStringMap(input map[string]string) map[string]string {
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

// cloneNextNodeAuditors 复制下一节点处理人参数，并保留加签映射后的节点标识。
func cloneNextNodeAuditors(input map[string][]target.NextAuditor) map[string][]target.NextAuditor {
	result := make(map[string][]target.NextAuditor, len(input))
	for key, auditors := range input {
		result[key] = append([]target.NextAuditor(nil), auditors...)
	}
	return result
}
