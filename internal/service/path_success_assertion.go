package service

import (
	"sort"
	"strings"

	"test-auto-pro-v2/internal/model"
)

// successAssertionEndNodeType 是目标真实流程结构里结束节点的类型值，与分析器识别的取值保持一致。
const successAssertionEndNodeType = "end"

// SuccessAssertionCandidates 从该路径已保存的真实线路与当前真实流程结构推导可选结束节点。
// 只接受分析器判定为可达的节点，不接受自由输入的节点键，也不猜测未选择路由之后的下游。
//
// ArrivalCount 取"本路径上真实通向该结束节点的可达边数量"：
// 顺序线路只有一条边进入，计 1；并行分支汇入同一个结束节点时每条支线各算一次到达，
// 这与目标引擎把并行支线整体纳入同一路径的语义一致，因此这种结束节点必须指定第几次到达。
func SuccessAssertionCandidates(graph model.FlowGraph, analysis model.ExecutionPathAnalysis) []model.SuccessAssertionEndNodeCandidate {
	reachableNodes := make(map[string]bool, len(analysis.ReachableNodeIDs))
	for _, nodeID := range analysis.ReachableNodeIDs {
		reachableNodes[nodeID] = true
	}
	reachableEdges := make(map[string]bool, len(analysis.ReachableEdgeIDs))
	for _, edgeID := range analysis.ReachableEdgeIDs {
		reachableEdges[edgeID] = true
	}
	arrivals := make(map[string]uint, len(reachableNodes))
	for _, edge := range graph.Edges {
		if reachableEdges[edge.ID] && reachableNodes[edge.Target] {
			arrivals[edge.Target]++
		}
	}
	candidates := make([]model.SuccessAssertionEndNodeCandidate, 0, 4)
	for _, node := range graph.Nodes {
		if node.Type != successAssertionEndNodeType || !reachableNodes[node.ID] {
			continue
		}
		count := arrivals[node.ID]
		if count == 0 {
			// 入口本身就是结束节点这种退化结构：路径确实会到达它一次。
			count = 1
		}
		candidates = append(candidates, model.SuccessAssertionEndNodeCandidate{
			NodeKey: node.ID, Name: strings.TrimSpace(node.Name), ArrivalCount: count,
		})
	}
	sort.Slice(candidates, func(first, second int) bool { return candidates[first].NodeKey < candidates[second].NodeKey })
	return candidates
}

// SuccessAssertionInput 是保存成功断言时来自浏览器的原始输入，只含用户能决定的三项。
type SuccessAssertionInput struct {
	EndNodeKey     string `json:"endNodeKey"`
	ExpectedStatus string `json:"expectedStatus"`
	ArrivalOrdinal uint   `json:"arrivalOrdinal"`
	// Revision 是浏览器读到的断言修订，用于并发保存时的冲突检测；首次保存传 0。
	Revision uint64 `json:"revision"`
}

// ValidateSuccessAssertion 按真实候选与目标真实状态集合校验输入，返回可落库的断言。
// 校验失败一律给中文原因，不自动修正、不静默回落到某个默认值。
func ValidateSuccessAssertion(candidates []model.SuccessAssertionEndNodeCandidate, input SuccessAssertionInput) (model.PathSuccessAssertion, string) {
	if len(candidates) == 0 {
		return model.PathSuccessAssertion{}, "这条路径在当前真实流程结构里没有结束节点，无法配置成功断言"
	}
	nodeKey := strings.TrimSpace(input.EndNodeKey)
	if nodeKey == "" {
		return model.PathSuccessAssertion{}, "请选择这条路径的结束节点"
	}
	var chosen *model.SuccessAssertionEndNodeCandidate
	for index := range candidates {
		if candidates[index].NodeKey == nodeKey {
			chosen = &candidates[index]
			break
		}
	}
	if chosen == nil {
		return model.PathSuccessAssertion{}, "所选结束节点不在这条路径的真实线路上，请重新选择"
	}
	status := strings.TrimSpace(input.ExpectedStatus)
	if status == "" {
		return model.PathSuccessAssertion{}, "请选择期望的实例状态"
	}
	if !model.IsFlowInstanceStatus(status) {
		return model.PathSuccessAssertion{}, "期望的实例状态不在目标平台真实取值范围内"
	}
	ordinal, reason := validateArrivalOrdinal(*chosen, input.ArrivalOrdinal)
	if reason != "" {
		return model.PathSuccessAssertion{}, reason
	}
	return model.PathSuccessAssertion{
		EndNodeKey: chosen.NodeKey, EndNodeName: chosen.Name,
		ExpectedStatus: status, ExpectedStatusLabel: model.FlowInstanceStatusLabel(status),
		ArrivalOrdinal: ordinal,
	}, ""
}

// validateArrivalOrdinal 校验第几次到达：只到达一次时不要求填写也不允许填越界，多次到达时必填且必须在范围内。
func validateArrivalOrdinal(candidate model.SuccessAssertionEndNodeCandidate, ordinal uint) (uint, string) {
	if candidate.ArrivalCount <= 1 {
		if ordinal > 1 {
			return 0, "这个结束节点在本路径上只会到达一次，不需要指定第几次到达"
		}
		return 1, ""
	}
	if ordinal == 0 {
		return 0, "这个结束节点在本路径上会到达多次，请指定第几次到达算成功"
	}
	if ordinal > candidate.ArrivalCount {
		return 0, "第几次到达超出了这个结束节点在本路径上的到达次数"
	}
	return ordinal, ""
}

// RevalidateSuccessAssertion 对已保存的断言做只读复验，返回中文问题清单；没有问题时返回空切片。
// 复验只判断"还站不站得住"，绝不自动修正已保存的取值。
func RevalidateSuccessAssertion(candidates []model.SuccessAssertionEndNodeCandidate, assertion model.PathSuccessAssertion) []model.PathConfigAffectedItem {
	issues := make([]model.PathConfigAffectedItem, 0, 3)
	display := strings.TrimSpace(assertion.EndNodeName)
	if display == "" {
		display = assertion.EndNodeKey
	}
	var chosen *model.SuccessAssertionEndNodeCandidate
	for index := range candidates {
		if candidates[index].NodeKey == assertion.EndNodeKey {
			chosen = &candidates[index]
			break
		}
	}
	if chosen == nil {
		issues = append(issues, model.PathConfigAffectedItem{
			Kind: "success_assertion", Name: display, Reason: "成功断言引用的结束节点已不在这条路径的真实线路上，请重新配置",
		})
	} else if assertion.ArrivalOrdinal > chosen.ArrivalCount {
		issues = append(issues, model.PathConfigAffectedItem{
			Kind: "success_assertion", Name: display, Reason: "成功断言的第几次到达已超出这个结束节点当前的到达次数，请重新配置",
		})
	}
	if !model.IsFlowInstanceStatus(assertion.ExpectedStatus) {
		issues = append(issues, model.PathConfigAffectedItem{
			Kind: "success_assertion", Name: display, Reason: "成功断言的期望实例状态已不在目标平台真实取值范围内，请重新配置",
		})
	}
	return issues
}
