package step

import (
	"fmt"
	"strings"

	"test-auto-pro-v2/internal/model"
)

// GraphEdgeInfo 是运行上下文里的一条真实图边（目标节点 ID 空间）。
type GraphEdgeInfo struct {
	Target   string
	BranchID string
}

// resolveBranchEntriesForTransition 沿真实图边从当前节点走到下一业务节点，
// 返回本次实际跨越的手动分支入口（目标节点 ID 列表，按穿越顺序）。
// 第二个返回值非空表示本次流转无法确定分支路径，必须在发送前阻塞（F-034 T04）。
//
// 规则（语义清单第 4 条与目标 FlowOperateServiceImpl 校验一致）：
//  1. 手动分支路由节点必须有已保存分支选择：没有选择的入口就无法生成 nodeProxyId，
//     直接阻塞，绝不用路线第一条分支或名称猜测兜底；
//  2. 条件分支的选择只是预测，目标自动求值；有选择沿选择走，没有选择沿任意出边尝试，
//     条件分支入口不计入手动分支条目；
//  3. 必须精确走到下一业务节点才算解析成功：找不到目标说明当前 transition 与路径选择不符。
func resolveBranchEntriesForTransition(currentTargetID, nextBusinessTargetID string, runCtx RunContext) ([]string, string) {
	currentTargetID = strings.TrimSpace(currentTargetID)
	nextBusinessTargetID = strings.TrimSpace(nextBusinessTargetID)
	if currentTargetID == "" || nextBusinessTargetID == "" || nextBusinessTargetID == currentTargetID {
		return nil, ""
	}
	visited := map[string]bool{}
	found, entries, blockReason := walkBranchPath(currentTargetID, nextBusinessTargetID, runCtx, visited, nil)
	if !found {
		if blockReason != "" {
			return nil, blockReason
		}
		// 图边可用但走不到目标：说明本次流转与路径选择不一致，必须阻塞，不能猜路径。
		if len(runCtx.GraphEdges) > 0 {
			return nil, fmt.Sprintf("当前路径没有找到从“分支”到下一节点的入口，请重新选择路径（%s → %s）", currentTargetID, nextBusinessTargetID)
		}
		// 图结构完全不完整（运行上下文没有边表，旧数据）时保持空结果由既有门禁裁决，不虚构阻塞。
		return nil, ""
	}
	return entries, ""
}

// walkBranchPath 深度优先沿已选分支走图：找到目标返回穿越的手动分支入口；结果按路径稳定。
func walkBranchPath(nodeID, target string, runCtx RunContext, visited map[string]bool, entries []string) (bool, []string, string) {
	if nodeID == target {
		return true, entries, ""
	}
	if visited[nodeID] {
		return false, nil, ""
	}
	visited[nodeID] = true
	edges, exists := runCtx.GraphEdges[nodeID]
	if !exists || len(edges) == 0 {
		return false, nil, ""
	}
	// 分支路由节点：有选择沿选择走。F-034 评审 #1：手动分支有已保存选择时只允许沿该选择继续，
	// 选中路径无法到达下一业务节点时直接阻塞，绝不再遍历其他出边——否则会把请求发到与路径选择
	// 不一致的另一条分支，重新引入“选择与实际发送路径不一致”的缺陷。
	// 只有条件分支等非手动节点（目标运行时自动求值）才允许在选择走不通时遍历全部出边。
	isManual := strings.EqualFold(strings.TrimSpace(runCtx.GraphNodeTypes[nodeID]), "manual")
	if selection, ok := runCtx.BranchSelections[nodeID]; ok {
		nextEntries := entries
		if isManual {
			nextEntries = append(append([]string(nil), entries...), selection)
		}
		found, result, nestedBlock := walkBranchPath(selection, target, runCtx, visited, nextEntries)
		if found || nestedBlock != "" {
			return found, result, nestedBlock
		}
		if isManual {
			return false, nil, "当前路径没有找到从“分支”到下一节点的入口，请重新选择路径"
		}
		return walkAllEdges(nodeID, target, runCtx, visited, entries, edges)
	}
	// 手动分支节点没有选择：目标无法确定实际走向，必须阻塞。
	if isManual {
		return false, nil, "当前路径没有找到从“分支”选择的入口，请重新选择路径"
	}
	// 非手动路由节点（条件等）没有选择时目标自动求值，允许沿任意出边尝试到达目标。
	return walkAllEdges(nodeID, target, runCtx, visited, entries, edges)
}

// walkAllEdges 依次尝试节点全部出边，返回第一条能到达目标的路径结果。
func walkAllEdges(nodeID, target string, runCtx RunContext, visited map[string]bool, entries []string, edges []GraphEdgeInfo) (bool, []string, string) {
	for _, edge := range edges {
		found, result, nestedBlock := walkBranchPath(edge.Target, target, runCtx, visited, entries)
		if found || nestedBlock != "" {
			return found, result, nestedBlock
		}
	}
	return false, nil, ""
}

// branchEntriesForStep 返回本步流转需要携带的手动分支入口；阻塞时返回原因供写前门禁拦截。
// submit、resubmit、approve 共用同一解析器，预览阶段与写前待办刷新使用同一结果（F-034 T04）。
func branchEntriesForStep(runCtx RunContext, step model.CompiledActionStep, nextNodeKey string) ([]string, string) {
	currentTargetID := strings.TrimSpace(runCtx.Nodes[strings.TrimSpace(step.NodeKey)].TargetNodeID)
	nextTargetID := strings.TrimSpace(runCtx.Nodes[strings.TrimSpace(nextNodeKey)].TargetNodeID)
	if currentTargetID == "" || nextTargetID == "" {
		// 任一端缺目标标识时无法做跨节点解析：保持空结果，由既有节点标识门禁处理。
		return nil, ""
	}
	return resolveBranchEntriesForTransition(currentTargetID, nextTargetID, runCtx)
}

// nonEmptyList 把单个可选入口包装成入口列表；空串返回 nil，兼容旧回退路径。
func nonEmptyList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return []string{value}
}

// BranchEntriesForTransitionForTest 暴露跨节点手动分支入口解析，供 test 目录锁定 F-034 行为：
// 入口是空节点时同样精确携带；无选择时发送前阻塞。
func BranchEntriesForTransitionForTest(currentNodeKey, nextNodeKey string, runCtx RunContext) ([]string, string) {
	return branchEntriesForStep(runCtx, model.CompiledActionStep{NodeKey: currentNodeKey}, nextNodeKey)
}
