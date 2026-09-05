package service

import (
	"encoding/json"
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
	// Hidden 是该节点声明隐藏（fieldPower=hide）的字段；隐藏只影响显示，不影响取值。
	Hidden []string
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
			hidden := make([]string, 0)
			for _, power := range node.FieldPowers {
				field := fieldpower.NormalizeFieldPath(power.EnglishName)
				if field == "" {
					continue
				}
				switch strings.TrimSpace(power.Power) {
				case "edit":
					editable = append(editable, field)
				case "hide":
					hidden = append(hidden, field)
				}
			}
			sort.Strings(editable)
			sort.Strings(hidden)
			powers = append(powers, routeNodePower{
				NodeID: node.ID, Name: node.Name, Type: node.Type, Editable: editable, Hidden: hidden,
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
//
// 除此之外还要解决"看到的和会提交的不一致"：只有后续节点才能编辑的字段，在这个视图里
// 既不会被提交（写载荷按同一套权限过滤），也不该带着历史值显示出来，否则用户会以为它这一步就会生效。
// 这类字段按目标自己的 hide 语义隐藏——权限声明里 hide 只影响显示，不影响取值，
// 因此保存时它们的值仍然完整保留，切到真正拥有它的节点视图就能看到。
// values 只用来决定"哪些键需要隐藏"，不参与任何改写。
func nodeFormViews(tree *target.FlowNodeTemplate, reachable []string, values map[string]any) []model.PathFormNodeView {
	powers := routeNodePowers(tree, reachable)
	views := make([]model.PathFormNodeView, 0, len(powers))
	for index, power := range powers {
		permissions := make([]model.PathFormPermission, 0, len(power.Editable)+len(power.Hidden))
		for _, field := range power.Editable {
			permissions = append(permissions, model.PathFormPermission{Field: field, Power: "edit"})
		}
		// 只沿用目标自己声明的隐藏；"只有后续节点能填"的字段不隐藏组件，只是不回显样本值。
		for _, field := range power.Hidden {
			permissions = append(permissions, model.PathFormPermission{Field: field, Power: "hide"})
		}
		views = append(views, model.PathFormNodeView{
			NodeName:    power.Name,
			IsInitiator: power.Type == "start",
			Permissions: permissions,
			BlankFields: laterOnlyFields(powers, index, values),
		})
	}
	return views
}

// laterOnlyFields 返回在第 index 个节点视图里不回显样本值的表单数据键：
// 只有它之后的节点才有编辑权限的字段。到这一步为止（含本节点）有任何节点能编辑的字段都要显示，
// 因为运行到这一步时那些值确实已经在表单上了。
func laterOnlyFields(powers []routeNodePower, index int, values map[string]any) []string {
	if index < 0 || index >= len(powers) {
		return []string{}
	}
	upto := make([]string, 0)
	for position := 0; position <= index; position++ {
		upto = append(upto, powers[position].Editable...)
	}
	later := make([]string, 0)
	for position := index + 1; position < len(powers); position++ {
		later = append(later, powers[position].Editable...)
	}
	if len(later) == 0 || len(values) == 0 {
		return []string{}
	}
	hidden := make([]string, 0)
	for key := range values {
		if fieldpower.Covers(upto, key) {
			continue
		}
		if fieldpower.Covers(later, key) {
			hidden = append(hidden, key)
		}
	}
	sort.Strings(hidden)
	return hidden
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
func NodeFormViewsForTest(tree *target.FlowNodeTemplate, reachable []string, values map[string]any) []model.PathFormNodeView {
	return nodeFormViews(tree, reachable, values)
}

// viewEditableFields 返回指定节点视图（按中文节点名称匹配）声明可编辑的字段。
// 找不到视图时返回 false：调用方据此判断"这次保存没有按节点视图编辑"，不做任何权限收窄。
func viewEditableFields(tree *target.FlowNodeTemplate, reachable []string, viewNodeName string) ([]string, bool) {
	name := strings.TrimSpace(viewNodeName)
	if name == "" {
		return nil, false
	}
	for _, power := range routeNodePowers(tree, reachable) {
		if strings.TrimSpace(power.Name) == name {
			return power.Editable, true
		}
	}
	return nil, false
}

// restoreFieldsOutsideView 把不属于该视图编辑权限的键恢复为服务端基线值。
// 一个节点上真实用户只能改该节点声明可编辑的字段，其余字段的取值不该因为"在这个视图里没回显"
// 或浏览器侧任何漂移而改变；基线里没有的键保持提交值，避免把新增的伴生键误删。
func restoreFieldsOutsideView(submitted, baseline map[string]any, editable []string) []string {
	restored := make([]string, 0)
	for key, value := range baseline {
		if fieldpower.Covers(editable, key) {
			continue
		}
		if existing, ok := submitted[key]; ok && valuesLooselyEqual(existing, value) {
			continue
		}
		submitted[key] = value
		restored = append(restored, key)
	}
	sort.Strings(restored)
	return restored
}

// valuesLooselyEqual 只用于判断"要不要恢复"，按 JSON 文本比较即可，不参与任何写入决策。
func valuesLooselyEqual(left, right any) bool {
	leftText, leftErr := json.Marshal(left)
	rightText, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return string(leftText) == string(rightText)
}

// RestoreFieldsOutsideViewForTest 暴露按视图恢复无权限字段，供 test 目录下的定向用例锁定行为。
func RestoreFieldsOutsideViewForTest(submitted, baseline map[string]any, editable []string) []string {
	return restoreFieldsOutsideView(submitted, baseline, editable)
}
