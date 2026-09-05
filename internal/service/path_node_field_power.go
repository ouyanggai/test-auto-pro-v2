package service

import (
	"sort"
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/formdata/fieldpower"
	"test-auto-pro-v2/internal/model"
)

// routeNodePower 是路线上一个节点的字段权限投影：名称、类型与该节点声明可编辑的字段。
// 顺序即路线顺序（发起节点在最前），因此"第一个能填某字段的节点"就是它在这条路线上的填写时机。
type routeNodePower struct {
	NodeID   string
	Name     string
	Type     string
	Editable []string
}

// routeNodePowers 沿目标流程树按路线顺序投影可达节点的字段权限。
// 只取路线内节点：路线外节点这次运行不会执行，它的声明与本次填写时机无关。
// 人工节点才有"人来填"的语义，条件/并行等结构节点没有表单也没有待办，不参与填写时机判断。
func routeNodePowers(tree *target.FlowNodeTemplate, reachable []string) []routeNodePower {
	reachableSet := make(map[string]bool, len(reachable))
	for _, id := range reachable {
		reachableSet[id] = true
	}
	powers := make([]routeNodePower, 0, len(reachable))
	visited := make(map[string]bool, len(reachable))
	var visit func(*target.FlowNodeTemplate)
	visit = func(node *target.FlowNodeTemplate) {
		if node == nil || visited[node.ID] {
			return
		}
		visited[node.ID] = true
		if reachableSet[node.ID] && nodeAcceptsHumanInput(node.Type) {
			editable := make([]string, 0, len(node.FieldPowers))
			for _, power := range node.FieldPowers {
				if strings.TrimSpace(power.Power) != "edit" {
					continue
				}
				if field := fieldpower.NormalizeFieldPath(power.EnglishName); field != "" {
					editable = append(editable, field)
				}
			}
			sort.Strings(editable)
			powers = append(powers, routeNodePower{
				NodeID: node.ID, Name: node.Name, Type: node.Type, Editable: editable,
			})
		}
		for _, branch := range node.ConditionNodes {
			visit(branch.Child)
		}
		for _, branch := range node.ParallelNodes {
			visit(branch.Child)
		}
		visit(node.Child)
	}
	visit(tree)
	return powers
}

// nodeAcceptsHumanInput 判断这个目标节点类型是否由真实处理人填写表单。
// 条件、并行等结构节点没有表单也没有待办，工具在那里发不出任何写请求，
// 因此"到分支节点再填"这句话在实现上只能落到分支之前最近的人工节点。
func nodeAcceptsHumanInput(nodeType string) bool {
	switch strings.TrimSpace(nodeType) {
	case "start", "common", "synergy":
		return true
	default:
		return false
	}
}

// fieldFillNode 返回路线上第一个能编辑该字段的节点；一个都没有时返回 false。
func fieldFillNode(powers []routeNodePower, field string) (routeNodePower, bool) {
	for _, power := range powers {
		if fieldpower.Covers(power.Editable, field) {
			return power, true
		}
	}
	return routeNodePower{}, false
}

// keyFieldFillHints 为每个条件字段算出填写时机：谁能填、是不是发起人。
// 目标条件求值只认本次写请求带上来的表单数据（语义清单第 17 条），
// 所以发起人无权编辑的条件字段可以由后续有权节点在自己的写请求里带上，分支按那次请求重新计算。
func keyFieldFillHints(fields []model.HistoryKeyField, powers []routeNodePower) []model.HistoryKeyField {
	if len(powers) == 0 {
		return fields
	}
	initiator := powers[0]
	result := make([]model.HistoryKeyField, 0, len(fields))
	for _, field := range fields {
		node, found := fieldFillNode(powers, field.Path)
		switch {
		case !found:
			field.FillNodeName = ""
			field.FillableAtStart = false
		case node.NodeID == initiator.NodeID && initiator.Type == "start":
			field.FillNodeName = node.Name
			field.FillableAtStart = true
		default:
			field.FillNodeName = node.Name
			field.FillableAtStart = false
		}
		result = append(result, field)
	}
	return result
}

// unfillableKeyFieldIssues 对没有任何节点能填的决定性条件字段产生阻断问题。
// 这类字段我们在这条路线上填不出来，分支走向完全由目标现有数据决定：
// 必须如实告诉用户而不是让运行到那一步再莫名走错分支。
func unfillableKeyFieldIssues(fields []model.HistoryKeyField, powers []routeNodePower) []model.HistoryDataIssue {
	if len(powers) == 0 {
		return nil
	}
	issues := make([]model.HistoryDataIssue, 0)
	for _, field := range fields {
		if !field.Decisive || field.FillNodeName != "" {
			continue
		}
		label := field.Label
		if strings.TrimSpace(label) == "" {
			label = field.Path
		}
		issues = append(issues, model.HistoryDataIssue{
			Code:    "CONDITION_FIELD_NOT_FILLABLE",
			Path:    field.Path,
			Fields:  []string{field.Path},
			Message: "条件字段「" + label + "」在这条路线上没有任何节点有编辑权限，工具填不出这个值，分支走向只能由目标现有数据决定，请人工确认",
			// 阻断：这条路线的分支命中无法由工具保证，继续跑等于把结果交给运气。
			Blocking: true,
		})
	}
	if len(issues) == 0 {
		return nil
	}
	return issues
}

// nodeFormViews 投影按节点切换的表单权限视图：发起人视图在最前，其余按路线顺序。
// 数据仍是同一份表单数据，视图只切换"这个节点能改哪些字段"，与目标审批页的渲染口径一致：
// 整张表单先禁用，再只放开该节点声明可编辑的字段。
func nodeFormViews(tree *target.FlowNodeTemplate, reachable []string) []model.PathFormNodeView {
	powers := routeNodePowers(tree, reachable)
	views := make([]model.PathFormNodeView, 0, len(powers))
	for _, power := range powers {
		permissions := make([]model.PathFormPermission, 0, len(power.Editable))
		for _, field := range power.Editable {
			permissions = append(permissions, model.PathFormPermission{Field: field, Power: "edit"})
		}
		views = append(views, model.PathFormNodeView{
			NodeName:    power.Name,
			IsInitiator: power.Type == "start",
			Permissions: permissions,
		})
	}
	return views
}

// KeyFieldFillHintsForTest 暴露条件字段填写时机投影，供 test 目录下的定向用例锁定行为。
func KeyFieldFillHintsForTest(tree *target.FlowNodeTemplate, reachable []string, fields []model.HistoryKeyField) []model.HistoryKeyField {
	return keyFieldFillHints(fields, routeNodePowers(tree, reachable))
}

// UnfillableKeyFieldIssuesForTest 暴露不可填条件字段的阻断问题，供 test 目录下的定向用例锁定行为。
func UnfillableKeyFieldIssuesForTest(tree *target.FlowNodeTemplate, reachable []string, fields []model.HistoryKeyField) []model.HistoryDataIssue {
	powers := routeNodePowers(tree, reachable)
	return unfillableKeyFieldIssues(keyFieldFillHints(fields, powers), powers)
}

// NodeFormViewsForTest 暴露按节点表单权限视图，供 test 目录下的定向用例锁定行为。
func NodeFormViewsForTest(tree *target.FlowNodeTemplate, reachable []string) []model.PathFormNodeView {
	return nodeFormViews(tree, reachable)
}
