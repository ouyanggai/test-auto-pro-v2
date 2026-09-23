package target

import "strings"

// AlignProxyTreeBranches 将目标树（如加签后重建的动态代理树）的网关分支顺序与基准树（如计划静态模板树）对齐。
//
// 职责与关键约束：
// 目标平台在加签（updateFlowProxy）保存后返回的新流程代理树中，parallelNodes / conditionNodes 的切片顺序
// 可能会发生变动（例如目标后端按更新时间倒序或主键顺序返回，将刚刚发生加签修改的分支排到了最前）。
// 若直接按目标平台返回的切片顺序生成结构路径（branch/0、branch/1……）或绘制画布，会导致：
// 1. 画布上分支左右位置发生颠倒跳变（如张政原本在右边，加签后跳到左边）；
// 2. 代理树重映射（ProxyRemap）依据切片下标对齐节点时，将不同分支上的同类型节点错位对调，
//    造成节点 TargetNodeID 张冠李戴，后续待办查询与审批时误判为“待办已处理”或“处理人缺失”而异常停止。
//
// 为什么这样处理：
// 加签仅在分支内部指定节点之后插入新节点，分支本身的业务语义（名称、首节点类型与名称）在加签前后是唯一恒定的。
// 本函数递归比对基准树与目标树对应网关上的各分支，按“分支名称优先、首节点名称与类型兜底”的原则，
// 将目标树的分支切片调整为与基准树严格一致的顺序，消除非确定性数组返回引起的拓扑错位。
// 输入边界：任一入参为空时安全静默返回；分支数量不一致时保持目标树自身顺序。
func AlignProxyTreeBranches(referenceTree, targetTree *FlowNodeTemplate) {
	if referenceTree == nil || targetTree == nil {
		return
	}
	if len(referenceTree.ParallelNodes) > 0 && len(targetTree.ParallelNodes) > 0 {
		targetTree.ParallelNodes = reorderBranches(referenceTree.ParallelNodes, targetTree.ParallelNodes)
	}
	if len(referenceTree.ConditionNodes) > 0 && len(targetTree.ConditionNodes) > 0 {
		targetTree.ConditionNodes = reorderBranches(referenceTree.ConditionNodes, targetTree.ConditionNodes)
	}

	for i := range targetTree.ParallelNodes {
		if i < len(referenceTree.ParallelNodes) {
			AlignProxyTreeBranches(referenceTree.ParallelNodes[i].Child, targetTree.ParallelNodes[i].Child)
		}
	}
	for i := range targetTree.ConditionNodes {
		if i < len(referenceTree.ConditionNodes) {
			AlignProxyTreeBranches(referenceTree.ConditionNodes[i].Child, targetTree.ConditionNodes[i].Child)
		}
	}

	if referenceTree.Child != nil && targetTree.Child != nil {
		AlignProxyTreeBranches(referenceTree.Child, targetTree.Child)
	}
}

// reorderBranches 按照基准分支切片的语义特征重新排列目标分支切片。
func reorderBranches(referenceBranches, targetBranches []FlowBranchTemplate) []FlowBranchTemplate {
	if len(referenceBranches) != len(targetBranches) || len(targetBranches) <= 1 {
		return targetBranches
	}
	result := make([]FlowBranchTemplate, len(referenceBranches))
	used := make([]bool, len(targetBranches))

	for i, ref := range referenceBranches {
		matchedIdx := -1
		// 1. 优先按分支名（如“节点1”、“节点2”或条件名）精确匹配
		refName := strings.TrimSpace(ref.Name)
		if refName != "" {
			for j, tgt := range targetBranches {
				if !used[j] && strings.TrimSpace(tgt.Name) == refName {
					matchedIdx = j
					break
				}
			}
		}
		// 2. 其次按分支首个子节点的名称和类型匹配（加签插入在节点之后，首节点名称与类型始终保持不变）
		if matchedIdx < 0 && ref.Child != nil {
			refChildName := strings.TrimSpace(ref.Child.Name)
			refChildType := strings.TrimSpace(ref.Child.Type)
			for j, tgt := range targetBranches {
				if !used[j] && tgt.Child != nil &&
					strings.TrimSpace(tgt.Child.Name) == refChildName &&
					strings.TrimSpace(tgt.Child.Type) == refChildType {
					matchedIdx = j
					break
				}
			}
		}
		if matchedIdx >= 0 {
			result[i] = targetBranches[matchedIdx]
			used[matchedIdx] = true
		} else {
			// 未能语义匹配时，按原序填充第一个尚未被使用的分支
			for j, tgt := range targetBranches {
				if !used[j] {
					result[i] = tgt
					used[j] = true
					break
				}
			}
		}
	}
	return result
}
