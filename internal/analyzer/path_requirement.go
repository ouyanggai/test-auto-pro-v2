package analyzer

import (
	"fmt"
	"sort"
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/model"
)

// PathRequirementAnalyzer 把已验证的真实路径投影为不含内部标识的中文要求。
type PathRequirementAnalyzer struct{}

// NewPathRequirementAnalyzer 创建无状态的路径要求分析器。
func NewPathRequirementAnalyzer() *PathRequirementAnalyzer { return &PathRequirementAnalyzer{} }

type requirementProjection struct {
	graphNodes    map[string]model.FlowGraphNode
	targetNodes   map[string]*target.FlowNodeTemplate
	outgoing      map[string][]model.FlowGraphEdge
	reachableEdge map[string]bool
	choices       map[string]string
	fields        []target.FormFieldMetadata
	groups        []model.RequirementGroup
	groupByKey    map[string]int
	visited       map[string]bool
	parallelIndex int
}

// Analyze 只遍历执行路径分析器确认可达的节点，并按主线与并行分支生成要求。
func (a *PathRequirementAnalyzer) Analyze(graph model.FlowGraph, tree *target.FlowNodeTemplate, fields []target.FormFieldMetadata, path model.ExecutionPath, analysis model.ExecutionPathAnalysis) (model.PathRequirements, error) {
	if tree == nil || !analysis.Complete || len(graph.EntryNodeIDs) == 0 {
		return model.PathRequirements{}, ErrExecutionPathInvalid
	}
	projection := &requirementProjection{
		graphNodes:    make(map[string]model.FlowGraphNode, len(graph.Nodes)),
		targetNodes:   make(map[string]*target.FlowNodeTemplate, len(graph.Nodes)),
		outgoing:      make(map[string][]model.FlowGraphEdge, len(graph.Nodes)),
		reachableEdge: make(map[string]bool, len(analysis.ReachableEdgeIDs)),
		choices:       make(map[string]string, len(path.Choices)),
		fields:        fields, groupByKey: make(map[string]int), visited: make(map[string]bool),
	}
	for _, node := range graph.Nodes {
		projection.graphNodes[node.ID] = node
	}
	for _, edge := range graph.Edges {
		projection.outgoing[edge.Source] = append(projection.outgoing[edge.Source], edge)
	}
	for _, edgeID := range analysis.ReachableEdgeIDs {
		projection.reachableEdge[edgeID] = true
	}
	for _, choice := range path.Choices {
		projection.choices[choice.RouteNodeID] = choice.BranchID
	}
	if err := collectTargetNodes(tree, projection.targetNodes, make(map[string]bool)); err != nil {
		return model.PathRequirements{}, err
	}
	projection.ensureGroup("main", "主线", "main")
	for _, entryID := range graph.EntryNodeIDs {
		if err := projection.walk(entryID, "main", ""); err != nil {
			return model.PathRequirements{}, err
		}
	}
	result := model.PathRequirements{
		Path:   model.PathRequirementPath{SequenceNo: path.SequenceNo, Name: path.Name},
		Groups: projection.groups,
	}
	result.Summary = summarizeRequirements(result.Groups)
	return result, nil
}

// collectTargetNodes 建立同一真实树的节点索引，重复或循环仍按结构异常处理。
func collectTargetNodes(node *target.FlowNodeTemplate, result map[string]*target.FlowNodeTemplate, stack map[string]bool) error {
	if node == nil || strings.TrimSpace(node.ID) == "" || stack[node.ID] {
		return ErrExecutionPathInvalid
	}
	if _, exists := result[node.ID]; exists {
		return nil
	}
	stack[node.ID] = true
	defer delete(stack, node.ID)
	result[node.ID] = node
	for _, branch := range append(append([]target.FlowBranchTemplate(nil), node.ConditionNodes...), node.ParallelNodes...) {
		if branch.Child != nil {
			if err := collectTargetNodes(branch.Child, result, stack); err != nil {
				return err
			}
		}
	}
	if node.Child != nil {
		return collectTargetNodes(node.Child, result, stack)
	}
	return nil
}

// ensureGroup 按投影内部键创建稳定顺序的主线或并行分组。
func (p *requirementProjection) ensureGroup(key, title, kind string) int {
	if index, exists := p.groupByKey[key]; exists {
		return index
	}
	index := len(p.groups)
	p.groupByKey[key] = index
	p.groups = append(p.groups, model.RequirementGroup{Title: title, Kind: kind, Nodes: []model.RequirementNode{}})
	return index
}

// walk 沿当前已选路径递归投影；并行分支在共享汇合点前停止，汇合后只回到父组一次。
func (p *requirementProjection) walk(nodeID, groupKey, stopID string) error {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" || nodeID == stopID {
		return nil
	}
	if p.visited[nodeID] {
		return nil
	}
	graphNode, exists := p.graphNodes[nodeID]
	if !exists {
		return ErrExecutionPathInvalid
	}
	targetNode := p.targetNodes[nodeID]
	if targetNode == nil {
		return ErrExecutionPathInvalid
	}
	p.visited[nodeID] = true
	items := p.nodeRequirements(graphNode, targetNode)
	groupIndex := p.groupByKey[groupKey]
	p.groups[groupIndex].Nodes = append(p.groups[groupIndex].Nodes, model.RequirementNode{
		Name: graphNode.Name, TypeName: graphNode.TypeName, Items: items,
	})

	edges := p.reachableOutgoing(nodeID)
	switch graphNode.Type {
	case "parallel":
		mergeID := strings.TrimSpace(graphNode.MergeTargetID)
		for _, edge := range edges {
			if edge.Kind != "parallel" {
				continue
			}
			p.parallelIndex++
			branchKey := fmt.Sprintf("parallel-%d", p.parallelIndex)
			branchTitle := "并行分支"
			if strings.TrimSpace(edge.Label) != "" {
				branchTitle += "：" + strings.TrimSpace(edge.Label)
			}
			p.ensureGroup(branchKey, branchTitle, "parallel")
			if err := p.walk(edge.Target, branchKey, mergeID); err != nil {
				return err
			}
		}
		if mergeID != "" {
			return p.walk(mergeID, groupKey, stopID)
		}
		return nil
	case "condition", "manual":
		if len(edges) != 1 {
			return ErrExecutionPathInvalid
		}
		return p.walk(edges[0].Target, groupKey, stopID)
	default:
		if len(edges) > 1 {
			return ErrExecutionPathInvalid
		}
		if len(edges) == 1 {
			return p.walk(edges[0].Target, groupKey, stopID)
		}
		return nil
	}
}

// reachableOutgoing 保持后端图中的稳定边顺序，并过滤当前路径未纳入的边。
func (p *requirementProjection) reachableOutgoing(nodeID string) []model.FlowGraphEdge {
	result := make([]model.FlowGraphEdge, 0)
	for _, edge := range p.outgoing[nodeID] {
		if p.reachableEdge[edge.ID] {
			result = append(result, edge)
		}
	}
	return result
}

// nodeRequirements 根据业务节点类型组合条件、人员、动作与只读约束。
func (p *requirementProjection) nodeRequirements(graphNode model.FlowGraphNode, node *target.FlowNodeTemplate) []model.RequirementItem {
	items := make([]model.RequirementItem, 0)
	switch graphNode.Type {
	case "condition":
		items = append(items, p.conditionRequirement(graphNode, node))
	case "manual":
		items = append(items, p.manualRequirement(graphNode))
	case "parallel":
		items = append(items, model.RequirementItem{Category: "条件", Title: "并行规则", Detail: "全部分支并行必经", Status: model.RequirementAutomatic})
	case "start":
		items = append(items, model.RequirementItem{Category: "动作", Title: "发起动作", Detail: "提交", Status: model.RequirementPending})
	case "common", "synergy":
		items = append(items, p.personRequirement(node))
		items = append(items, model.RequirementItem{Category: "动作", Title: "处理结果", Detail: "需要准备同意或不同意的处理结果", Status: model.RequirementPending})
		items = append(items, model.RequirementItem{Category: "约束", Title: "运行态特殊操作", Detail: "回退上一级、添加审批人和转发是否可用需结合实际任务核对", Status: model.RequirementRuntime})
	case "unknown":
		items = append(items, model.RequirementItem{Category: "约束", Title: "节点类型", Detail: "当前节点类型尚未识别", Status: model.RequirementReview})
	}
	items = append(items, p.fieldPowerRequirements(node)...)
	items = append(items, nodeConstraintRequirements(node)...)
	return compactRequirementItems(items)
}

// conditionRequirement 翻译当前已选条件分支；稳定顺序中的最后一支始终按兜底语义表达。
func (p *requirementProjection) conditionRequirement(graphNode model.FlowGraphNode, node *target.FlowNodeTemplate) model.RequirementItem {
	selectedBranchID := p.choices[graphNode.ID]
	allEdges := p.outgoing[graphNode.ID]
	selectedIndex := -1
	for index, edge := range allEdges {
		if edge.Kind == "condition" && edge.BranchID == selectedBranchID {
			selectedIndex = index
			break
		}
	}
	if selectedIndex < 0 {
		return model.RequirementItem{Category: "条件", Title: "分支条件", Detail: "当前已选分支无法对应真实条件", Status: model.RequirementReview}
	}
	if selectedIndex == len(allEdges)-1 {
		return model.RequirementItem{Category: "条件", Title: selectedBranchName(allEdges[selectedIndex]), Detail: "其他条件均不满足时进入；后续数据需保证前序分支均不成立", Status: model.RequirementPending}
	}
	branch := findTargetBranch(node.ConditionNodes, selectedBranchID)
	if branch == nil || len(branch.Conditions) == 0 {
		return model.RequirementItem{Category: "条件", Title: selectedBranchName(allEdges[selectedIndex]), Detail: "当前分支缺少可识别条件", Status: model.RequirementReview}
	}
	expressions := make([]string, 0, len(branch.Conditions))
	status := model.RequirementPending
	for index, condition := range branch.Conditions {
		expression, recognized := p.translateCondition(condition, node.FieldPowers)
		if !recognized {
			status = model.RequirementReview
		}
		expressions = append(expressions, expression)
		if index < len(branch.Conditions)-1 {
			connector, ok := conditionConnector(condition.ConditionType)
			if !ok {
				status = model.RequirementReview
			}
			expressions = append(expressions, connector)
		}
	}
	return model.RequirementItem{Category: "条件", Title: selectedBranchName(allEdges[selectedIndex]), Detail: strings.Join(expressions, " "), Status: status}
}

// manualRequirement 将手动分支保持为运行时选择语义，不伪造成条件表达式。
func (p *requirementProjection) manualRequirement(graphNode model.FlowGraphNode) model.RequirementItem {
	branchID := p.choices[graphNode.ID]
	label := "所选分支"
	for _, edge := range p.outgoing[graphNode.ID] {
		if edge.BranchID == branchID && strings.TrimSpace(edge.Label) != "" {
			label = strings.TrimSpace(edge.Label)
			break
		}
	}
	return model.RequirementItem{Category: "条件", Title: "手动分支", Detail: "运行时选择该分支：" + label, Status: model.RequirementPending}
}

// translateCondition 把字段键、比较枚举和右值转换为安全中文表达。
func (p *requirementProjection) translateCondition(condition target.FlowCondition, powers []target.FlowNodeFieldPower) (string, bool) {
	left, leftOK := p.resolveField(condition.FieldA, powers)
	judge, judgeOK := conditionJudge(condition.Judge)
	right := strings.TrimSpace(condition.ValueB)
	rightOK := true
	if strings.TrimSpace(condition.FieldB) != "" {
		right, rightOK = p.resolveField(condition.FieldB, powers)
	} else if strings.TrimSpace(condition.ValueType) == "person" {
		// 人员条件值常为目标业务 ID，公开层只能说明其业务语义，不能把原值透出。
		right = "目标配置的人员值"
	} else if right == "" && judge != "已填写" && judge != "已修改" {
		right = "未识别的条件值"
		rightOK = false
	}
	if left == "" {
		left = "未识别的表单字段"
	}
	if right == "" {
		return strings.TrimSpace(left + " " + judge), leftOK && judgeOK && rightOK
	}
	return strings.TrimSpace(left + " " + judge + " " + right), leftOK && judgeOK && rightOK
}

// resolveField 使用节点表单提示缩小同名键范围，无法唯一匹配时明确降级而不猜字段。
func (p *requirementProjection) resolveField(key string, powers []target.FlowNodeFieldPower) (string, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "未识别的表单字段", false
	}
	preferredForms := make(map[string]bool)
	for _, power := range powers {
		if strings.TrimSpace(power.EnglishName) == key || strings.TrimSpace(power.FieldID) == key {
			if formID := strings.TrimSpace(power.FormID); formID != "" {
				preferredForms[formID] = true
			}
		}
	}
	matches := make([]target.FormFieldMetadata, 0)
	for _, field := range p.fields {
		if strings.TrimSpace(field.EnglishName) != key && strings.TrimSpace(field.FieldID) != key {
			continue
		}
		if len(preferredForms) > 0 && !preferredForms[strings.TrimSpace(field.FormID)] {
			continue
		}
		matches = append(matches, field)
	}
	if len(matches) != 1 || strings.TrimSpace(matches[0].Name) == "" {
		return "未识别的表单字段", false
	}
	return strings.TrimSpace(matches[0].Name), true
}

// personRequirement 将审批类型映射为后续准备责任，并只展示名称和数量摘要。
func (p *requirementProjection) personRequirement(node *target.FlowNodeTemplate) model.RequirementItem {
	config := node.AuditConfig
	if config == nil || strings.TrimSpace(config.AuditType) == "" {
		return model.RequirementItem{Category: "人员", Title: "处理人规则", Detail: "当前节点缺少处理人配置", Status: model.RequirementReview}
	}
	title, status, known := auditTypePresentation(config.AuditType)
	detailParts := []string{auditModeText(config.Mode, config.CountersignNum)}
	if names := auditDetailNames(config.Details); len(names) > 0 {
		detailParts = append(detailParts, "范围："+strings.Join(names, "、"))
	} else if len(config.Scopes) > 0 || len(config.Details) > 0 {
		detailParts = append(detailParts, fmt.Sprintf("已配置 %d 项范围", len(config.Scopes)+len(config.Details)))
	}
	if strings.TrimSpace(config.AuditType) == "form_person" {
		fieldName, ok := p.resolveField(config.FormPersonField, node.FieldPowers)
		if ok {
			detailParts = append(detailParts, "人员字段："+fieldName)
		} else {
			detailParts = append(detailParts, "人员字段：未识别的表单字段")
			status = model.RequirementReview
		}
	}
	if !known || !auditModeValid(config.Mode, config.CountersignNum) {
		status = model.RequirementReview
	}
	return model.RequirementItem{Category: "人员", Title: title, Detail: strings.Join(nonEmptyStrings(detailParts), "；"), Status: status}
}

// fieldPowerRequirements 把字段权限转换为中文字段名；无法解析的关联单项降级人工核对。
func (p *requirementProjection) fieldPowerRequirements(node *target.FlowNodeTemplate) []model.RequirementItem {
	result := make([]model.RequirementItem, 0, len(node.FieldPowers))
	for _, power := range node.FieldPowers {
		fieldKey := firstNonEmptyString(power.EnglishName, power.FieldID)
		fieldName, fieldOK := p.resolveField(fieldKey, []target.FlowNodeFieldPower{power})
		powerName, powerOK := fieldPowerName(power.Power)
		status := model.RequirementAutomatic
		if !fieldOK || !powerOK {
			status = model.RequirementReview
		}
		result = append(result, model.RequirementItem{Category: "约束", Title: "字段权限", Detail: fieldName + "：" + powerName, Status: status})
	}
	return result
}

// nodeConstraintRequirements 只展示目标明确配置的跳过与时限约束，不补默认值。
func nodeConstraintRequirements(node *target.FlowNodeTemplate) []model.RequirementItem {
	result := make([]model.RequirementItem, 0, 2)
	if node.IsSkip != nil {
		detail := "无处理人时不跳过"
		if *node.IsSkip {
			detail = "无处理人时跳过该节点"
		}
		result = append(result, model.RequirementItem{Category: "约束", Title: "无处理人规则", Detail: detail, Status: model.RequirementAutomatic})
	}
	if node.Delay != nil && *node.Delay > 0 {
		unit, ok := delayUnitName(node.Unit)
		deadline, deadlineOK := deadlineTypeName(node.DeadlineType)
		status := model.RequirementAutomatic
		if !ok || !deadlineOK {
			status = model.RequirementReview
		}
		detail := fmt.Sprintf("延时 %d %s", *node.Delay, unit)
		if deadline != "" {
			detail += "，按" + deadline + "计算"
		}
		result = append(result, model.RequirementItem{Category: "约束", Title: "节点时限", Detail: detail, Status: status})
	}
	return result
}

// summarizeRequirements 按固定四类中文状态输出完整计数，即使某类为零也保持契约稳定。
func summarizeRequirements(groups []model.RequirementGroup) []model.RequirementCount {
	statuses := []model.RequirementStatus{model.RequirementPending, model.RequirementAutomatic, model.RequirementRuntime, model.RequirementReview}
	counts := make(map[model.RequirementStatus]int, len(statuses))
	for _, group := range groups {
		for _, node := range group.Nodes {
			for _, item := range node.Items {
				counts[item.Status]++
			}
		}
	}
	result := make([]model.RequirementCount, 0, len(statuses))
	for _, status := range statuses {
		result = append(result, model.RequirementCount{Status: status, Count: counts[status]})
	}
	return result
}

// findTargetBranch 按真实分支标识查找内部条件配置。
func findTargetBranch(branches []target.FlowBranchTemplate, branchID string) *target.FlowBranchTemplate {
	for index := range branches {
		if strings.TrimSpace(branches[index].ID) == strings.TrimSpace(branchID) {
			return &branches[index]
		}
	}
	return nil
}

// selectedBranchName 为缺名分支提供克制中文标题。
func selectedBranchName(edge model.FlowGraphEdge) string {
	if value := strings.TrimSpace(edge.Label); value != "" {
		return value
	}
	return "已选条件分支"
}

// conditionJudge 映射参考代码已支持的比较方式，未知代码不直接展示。
func conditionJudge(value string) (string, bool) {
	switch strings.TrimSpace(value) {
	case "gt":
		return "大于", true
	case "gte":
		return "大于等于", true
	case "lt":
		return "小于", true
	case "lte":
		return "小于等于", true
	case "eq":
		return "等于", true
	case "neq":
		return "不等于", true
	case "contains":
		return "包含", true
	case "boolean_value":
		return "为", true
	case "is_update":
		return "已修改", true
	case "is_not_null":
		return "已填写", true
	default:
		return "使用未识别的比较方式", false
	}
}

// conditionConnector 映射多条件连接语义，未知值降级为人工核对文案。
func conditionConnector(value string) (string, bool) {
	switch strings.TrimSpace(value) {
	case "and":
		return "并且", true
	case "or":
		return "或者", true
	default:
		return "连接关系待核对", false
	}
}

// auditTypePresentation 映射参考页面确认的人员规则与准备责任。
func auditTypePresentation(value string) (string, model.RequirementStatus, bool) {
	switch strings.TrimSpace(value) {
	case "assign":
		return "指定人员", model.RequirementAutomatic, true
	case "company":
		return "项目指定人员", model.RequirementAutomatic, true
	case "company_id":
		return "指定公司", model.RequirementAutomatic, true
	case "department":
		return "指定部门", model.RequirementAutomatic, true
	case "position":
		return "指定岗位", model.RequirementAutomatic, true
	case "role":
		return "选择角色", model.RequirementAutomatic, true
	case "initiator":
		return "发起人自己", model.RequirementRuntime, true
	case "department_supervisor":
		return "发起人部门主管", model.RequirementRuntime, true
	case "branched_passage_manager":
		return "发起人分管副总", model.RequirementRuntime, true
	case "level":
		return "指定岗级", model.RequirementRuntime, true
	case "extendedAttribute":
		return "扩展属性", model.RequirementRuntime, true
	case "run_node_choose":
		return "审批人自选", model.RequirementPending, true
	case "form_person":
		return "指定表单人员", model.RequirementPending, true
	default:
		return "未识别的处理人规则", model.RequirementReview, false
	}
}

// auditModeText 返回会签或竞签中文摘要，异常值仅返回待核对文案。
func auditModeText(mode string, count *int) string {
	switch strings.TrimSpace(mode) {
	case "countersign":
		if count == nil {
			return "会签，人数待核对"
		}
		if *count == -1 {
			return "会签，所有人"
		}
		if *count > 0 {
			return fmt.Sprintf("会签，满足 %d 人", *count)
		}
		return "会签，人数待核对"
	case "", "scramble":
		return "竞签"
	default:
		return "处理方式待核对"
	}
}

// auditModeValid 判断会签人数和处理方式是否自洽。
func auditModeValid(mode string, count *int) bool {
	if strings.TrimSpace(mode) == "countersign" {
		return count != nil && (*count == -1 || *count > 0)
	}
	return strings.TrimSpace(mode) == "" || strings.TrimSpace(mode) == "scramble"
}

// auditDetailNames 返回去重排序后的可展示范围名称。
func auditDetailNames(details []target.FlowAuditDetail) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(details))
	for _, detail := range details {
		name := strings.TrimSpace(detail.Name)
		if name != "" && !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result
}

// fieldPowerName 映射节点字段权限，未知代码不直接公开。
func fieldPowerName(value string) (string, bool) {
	switch strings.TrimSpace(value) {
	case "only_read":
		return "只读", true
	case "edit":
		return "可编辑", true
	case "hide":
		return "隐藏", true
	default:
		return "权限待核对", false
	}
}

// delayUnitName 映射常用节点延时单位。
func delayUnitName(value string) (string, bool) {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "second", "seconds", "s":
		return "秒", true
	case "minute", "minutes", "m":
		return "分钟", true
	case "hour", "hours", "h":
		return "小时", true
	case "day", "days", "d":
		return "天", true
	default:
		return "时间单位待核对", false
	}
}

// deadlineTypeName 映射自然日与工作日；空值表示目标未配置。
func deadlineTypeName(value string) (string, bool) {
	switch strings.TrimSpace(value) {
	case "":
		return "", true
	case "natural_day":
		return "自然日", true
	case "work_day":
		return "工作日", true
	default:
		return "时限口径待核对", false
	}
}

// compactRequirementItems 移除不应产生的空要求项。
func compactRequirementItems(items []model.RequirementItem) []model.RequirementItem {
	result := make([]model.RequirementItem, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Title) != "" && item.Status != "" {
			result = append(result, item)
		}
	}
	return result
}

// nonEmptyStrings 移除摘要中的空片段。
func nonEmptyStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// firstNonEmptyString 返回第一个非空字符串。
func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
