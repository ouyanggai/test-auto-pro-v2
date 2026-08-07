package analyzer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/model"
)

// 基础字段控件类型，前端按此类型渲染受约束控件。
const (
	PathConfigTypeText         = "text"
	PathConfigTypeNumber       = "number"
	PathConfigTypeDate         = "date"
	PathConfigTypeDateTime     = "dateTime"
	PathConfigTypeSingleSelect = "singleSelect"
	PathConfigTypeMultiSelect  = "multiSelect"
	PathConfigTypeSwitch       = "switch"
)

// PathConfigAnalyzer 把已验证路径投影为可操作的字段与动作配置，并产出保存校验所需的内部索引。
type PathConfigAnalyzer struct{}

// NewPathConfigAnalyzer 创建无状态的路径配置分析器。
func NewPathConfigAnalyzer() *PathConfigAnalyzer { return &PathConfigAnalyzer{} }

// PathConfigFieldTarget 保存校验所需的最小内部字段定位，不进入公开响应。
type PathConfigFieldTarget struct {
	NodeID   string
	FieldKey string
	Name     string
	Type     string
	Required bool
	Options  []model.PathConfigOption
}

// PathConfigActionTarget 保存校验所需的最小内部动作定位，不进入公开响应。
type PathConfigActionTarget struct {
	NodeID string
	Kind   string
	Name   string
}

// PathConfigValidation 是不透明回写键到当前真实节点与字段的内部映射。
type PathConfigValidation struct {
	FieldTokens  map[string]PathConfigFieldTarget
	ActionTokens map[string]PathConfigActionTarget
}

// PathConfigFieldToken 生成字段的不透明回写键；同一节点同一字段在每次响应中保持稳定。
func PathConfigFieldToken(nodeID, fieldKey string) string {
	return pathConfigToken("field", nodeID, fieldKey)
}

// PathConfigActionToken 生成动作的不透明回写键；同一节点同一动作在每次响应中保持稳定。
func PathConfigActionToken(nodeID, actionKind string) string {
	return pathConfigToken("action", nodeID, actionKind)
}

// pathConfigToken 用节点与字段或动作键的哈希派生不透明键，浏览器无法反推出真实标识。
func pathConfigToken(kind, first, second string) string {
	sum := sha256.Sum256([]byte(kind + ":" + strings.TrimSpace(first) + ":" + strings.TrimSpace(second)))
	return hex.EncodeToString(sum[:16])
}

// Analyze 沿当前已验证路径按真实节点顺序生成字段分组与动作，并优先保留已保存值。
func (a *PathConfigAnalyzer) Analyze(
	graph model.FlowGraph,
	tree *target.FlowNodeTemplate,
	fields []target.FormFieldDetail,
	path model.ExecutionPath,
	analysis model.ExecutionPathAnalysis,
	instanceValues map[string]any,
	storedFields map[string]map[string]string,
	storedActions map[string]string,
) (model.PathConfiguration, PathConfigValidation, error) {
	if tree == nil || !analysis.Complete || len(graph.EntryNodeIDs) == 0 {
		return model.PathConfiguration{}, PathConfigValidation{}, ErrExecutionPathInvalid
	}
	projection := &pathConfigProjection{
		graphNodes:    make(map[string]model.FlowGraphNode, len(graph.Nodes)),
		targetNodes:   make(map[string]*target.FlowNodeTemplate, len(graph.Nodes)),
		outgoing:      make(map[string][]model.FlowGraphEdge, len(graph.Nodes)),
		reachableEdge: make(map[string]bool, len(analysis.ReachableEdgeIDs)),
		choices:       make(map[string]string, len(path.Choices)),
		fields:        fields, instanceValues: instanceValues,
		storedFields: storedFields, storedActions: storedActions,
		groupByKey: make(map[string]int), visited: make(map[string]bool),
		validation: PathConfigValidation{
			FieldTokens:  make(map[string]PathConfigFieldTarget),
			ActionTokens: make(map[string]PathConfigActionTarget),
		},
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
		return model.PathConfiguration{}, PathConfigValidation{}, err
	}
	if err := projection.projectEntries(graph.EntryNodeIDs); err != nil {
		return model.PathConfiguration{}, PathConfigValidation{}, err
	}
	result := model.PathConfiguration{
		Path:     model.PathConfigPath{SequenceNo: path.SequenceNo, Name: path.Name},
		Status:   "configured",
		Groups:   projection.groups,
		Warnings: nonNilStrings(projection.warnings),
	}
	if projection.affected {
		result.Status = "affected"
		result.Warnings = append(result.Warnings, "目标结构已变化，部分字段需要重新核对")
	}
	return result, projection.validation, nil
}

type pathConfigProjection struct {
	graphNodes     map[string]model.FlowGraphNode
	targetNodes    map[string]*target.FlowNodeTemplate
	outgoing       map[string][]model.FlowGraphEdge
	reachableEdge  map[string]bool
	choices        map[string]string
	fields         []target.FormFieldDetail
	instanceValues map[string]any
	storedFields   map[string]map[string]string
	storedActions  map[string]string
	groups         []model.PathConfigGroup
	groupByKey     map[string]int
	visited        map[string]bool
	parallelIndex  int
	validation     PathConfigValidation
	warnings       []string
	affected       bool
}

// projectEntries 与路径要求分析一致：单入口走主线，多入口各自形成并行分组并按共同汇合回主线。
func (p *pathConfigProjection) projectEntries(entryIDs []string) error {
	if len(entryIDs) == 1 {
		p.ensureGroup("main", "主线", "main")
		return p.walk(entryIDs[0], "main", "", false)
	}
	mergeID := p.commonReachableMerge(entryIDs)
	if mergeID != "" {
		p.ensureGroup("main", "主线", "main")
	}
	for index, entryID := range entryIDs {
		p.parallelIndex++
		groupKey := fmt.Sprintf("active-parallel-%d", p.parallelIndex)
		p.ensureGroup(groupKey, fmt.Sprintf("并行活动分支 %d", index+1), "parallel")
		if err := p.walk(entryID, groupKey, mergeID, false); err != nil {
			return err
		}
	}
	if mergeID != "" {
		return p.walk(mergeID, "main", "", false)
	}
	return nil
}

// commonReachableMerge 复用路径要求的共同汇合判定，找不到可证明汇合时保持分组。
func (p *pathConfigProjection) commonReachableMerge(entryIDs []string) string {
	distances := make([]map[string]int, 0, len(entryIDs))
	for _, entryID := range entryIDs {
		distances = append(distances, p.reachableDistances(entryID))
	}
	incomingCount := make(map[string]int)
	for sourceID := range p.outgoing {
		for _, edge := range p.reachableOutgoing(sourceID) {
			incomingCount[edge.Target]++
		}
	}
	bestID := ""
	bestMaxDistance := int(^uint(0) >> 1)
	bestTotalDistance := bestMaxDistance
	ambiguous := false
	for candidateID, firstDistance := range distances[0] {
		if incomingCount[candidateID] < 2 {
			continue
		}
		maxDistance := firstDistance
		totalDistance := firstDistance
		common := true
		for _, entryDistances := range distances[1:] {
			distance, exists := entryDistances[candidateID]
			if !exists {
				common = false
				break
			}
			if distance > maxDistance {
				maxDistance = distance
			}
			totalDistance += distance
		}
		if !common {
			continue
		}
		if maxDistance < bestMaxDistance || (maxDistance == bestMaxDistance && totalDistance < bestTotalDistance) {
			bestID, bestMaxDistance, bestTotalDistance, ambiguous = candidateID, maxDistance, totalDistance, false
			continue
		}
		if maxDistance == bestMaxDistance && totalDistance == bestTotalDistance {
			ambiguous = true
		}
	}
	if ambiguous {
		return ""
	}
	return bestID
}

// reachableDistances 以当前已选路径的边计算入口到各节点最短距离。
func (p *pathConfigProjection) reachableDistances(entryID string) map[string]int {
	entryID = strings.TrimSpace(entryID)
	result := map[string]int{entryID: 0}
	queue := []string{entryID}
	for len(queue) > 0 {
		sourceID := queue[0]
		queue = queue[1:]
		for _, edge := range p.reachableOutgoing(sourceID) {
			if _, exists := result[edge.Target]; exists {
				continue
			}
			result[edge.Target] = result[sourceID] + 1
			queue = append(queue, edge.Target)
		}
	}
	return result
}

// ensureGroup 按内部键创建稳定顺序的主线或并行分组。
func (p *pathConfigProjection) ensureGroup(key, title, kind string) int {
	if index, exists := p.groupByKey[key]; exists {
		return index
	}
	index := len(p.groups)
	p.groupByKey[key] = index
	p.groups = append(p.groups, model.PathConfigGroup{Title: title, Kind: kind, Nodes: []model.PathConfigNode{}})
	return index
}

// walk 沿当前已选路径递归投影节点；blocked 表示此前已出现不同意动作，后续节点不再按原路径继续。
func (p *pathConfigProjection) walk(nodeID, groupKey, stopID string, blocked bool) error {
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
	node := p.nodeConfig(graphNode, targetNode, blocked)
	groupIndex := p.groupByKey[groupKey]
	p.groups[groupIndex].Nodes = append(p.groups[groupIndex].Nodes, node)

	nextBlocked := blocked
	for _, action := range node.Actions {
		if action.Kind == "agree_disagree" && action.Current == "disagree" {
			nextBlocked = true
		}
	}
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
			if err := p.walk(edge.Target, branchKey, mergeID, nextBlocked); err != nil {
				return err
			}
		}
		if mergeID != "" {
			return p.walk(mergeID, groupKey, stopID, nextBlocked)
		}
		return nil
	case "condition", "manual":
		if len(edges) != 1 {
			return ErrExecutionPathInvalid
		}
		return p.walk(edges[0].Target, groupKey, stopID, nextBlocked)
	default:
		if len(edges) > 1 {
			return ErrExecutionPathInvalid
		}
		if len(edges) == 1 {
			return p.walk(edges[0].Target, groupKey, stopID, nextBlocked)
		}
		return nil
	}
}

// reachableOutgoing 保持图中稳定边顺序并过滤当前路径未纳入的边。
func (p *pathConfigProjection) reachableOutgoing(nodeID string) []model.FlowGraphEdge {
	result := make([]model.FlowGraphEdge, 0)
	for _, edge := range p.outgoing[nodeID] {
		if p.reachableEdge[edge.ID] {
			result = append(result, edge)
		}
	}
	return result
}

// nodeConfig 按节点类型组合字段、缺口与标准动作。
func (p *pathConfigProjection) nodeConfig(graphNode model.FlowGraphNode, node *target.FlowNodeTemplate, blocked bool) model.PathConfigNode {
	result := model.PathConfigNode{
		Name: graphNode.Name, TypeName: graphNode.TypeName, Kind: graphNode.Type,
		LineBlocked: blocked, Actions: []model.PathConfigAction{},
	}
	result.Fields, result.Gaps = p.fieldConfig(node)
	switch graphNode.Type {
	case "start":
		result.Actions = append(result.Actions, p.submitAction(graphNode.ID))
	case "common", "synergy":
		result.Actions = append(result.Actions, p.approvalAction(graphNode.ID, graphNode.Name))
	}
	return result
}

// submitAction 发起节点固定提交，不提供其他候选。
func (p *pathConfigProjection) submitAction(nodeID string) model.PathConfigAction {
	key := PathConfigActionToken(nodeID, "submit")
	p.validation.ActionTokens[key] = PathConfigActionTarget{NodeID: nodeID, Kind: "submit", Name: "发起动作"}
	return model.PathConfigAction{
		Key: key, Kind: "submit", Label: "发起动作", Current: "submit", Default: "submit",
		Options: []model.PathConfigActionOption{{Value: "submit", Label: "提交"}},
	}
}

// approvalAction 审批或协同节点提供同意与不同意，默认推荐同意，不同意明确影响后续线路。
func (p *pathConfigProjection) approvalAction(nodeID, nodeName string) model.PathConfigAction {
	key := PathConfigActionToken(nodeID, "agree_disagree")
	current := strings.TrimSpace(p.storedActions[nodeID])
	if current == "" {
		current = "agree"
	}
	p.validation.ActionTokens[key] = PathConfigActionTarget{NodeID: nodeID, Kind: "agree_disagree", Name: nodeName}
	return model.PathConfigAction{
		Key: key, Kind: "agree_disagree", Label: "处理结果", Current: current, Default: "agree",
		Options: []model.PathConfigActionOption{
			{Value: "agree", Label: "同意"},
			{Value: "disagree", Label: "不同意"},
		},
		DisagreeWarning: "选择不同意会提前结束或改变后续线路，保存后不再按原路径继续",
	}
}

// fieldConfig 投影节点字段权限允许编辑且可可靠映射的基础字段，其余项目转为明确缺口。
func (p *pathConfigProjection) fieldConfig(node *target.FlowNodeTemplate) ([]model.PathConfigField, []model.PathConfigGap) {
	fields := make([]model.PathConfigField, 0)
	gaps := make([]model.PathConfigGap, 0)
	for _, power := range node.FieldPowers {
		powerValue := strings.TrimSpace(power.Power)
		if powerValue != "edit" {
			// 明确只读或隐藏的字段不属于当前节点允许修改，不占缺口；权限不明才提示人工核对。
			if powerValue == "" {
				gaps = append(gaps, model.PathConfigGap{Name: "字段", Reason: "字段权限不明确，需要人工核对"})
			}
			continue
		}
		detail, detailOK := p.resolveFieldDetail(power)
		if !detailOK {
			gaps = append(gaps, model.PathConfigGap{Name: "字段", Reason: "未识别的表单字段，需要人工核对"})
			continue
		}
		field, gap := p.buildField(node.ID, detail)
		if gap != nil {
			gaps = append(gaps, *gap)
			continue
		}
		fields = append(fields, field)
	}
	return fields, gaps
}

// resolveFieldDetail 按节点表单范围优先定位字段详情，无法安全对应时返回人工核对。
func (p *pathConfigProjection) resolveFieldDetail(power target.FlowNodeFieldPower) (target.FormFieldDetail, bool) {
	formID := strings.TrimSpace(power.FormID)
	fieldID := strings.TrimSpace(power.FieldID)
	englishName := strings.TrimSpace(power.EnglishName)
	var fallback *target.FormFieldDetail
	for index := range p.fields {
		candidate := &p.fields[index]
		if formID != "" && strings.TrimSpace(candidate.FormID) != formID {
			continue
		}
		if fieldID != "" && strings.TrimSpace(candidate.FieldID) == fieldID {
			return *candidate, true
		}
		if englishName != "" && strings.TrimSpace(candidate.EnglishName) == englishName {
			if fallback == nil {
				candidateCopy := *candidate
				fallback = &candidateCopy
			} else {
				// 同一表单内同键命中多个字段无法安全对应，保持人工核对。
				return target.FormFieldDetail{}, false
			}
		}
	}
	if fallback != nil {
		return *fallback, true
	}
	return target.FormFieldDetail{}, false
}

// buildField 把字段详情映射为基础控件并叠加当前值；无法映射时返回具体缺口原因。
func (p *pathConfigProjection) buildField(nodeID string, detail target.FormFieldDetail) (model.PathConfigField, *model.PathConfigGap) {
	controlType, ok := pathConfigControlType(detail)
	if !ok {
		return model.PathConfigField{}, &model.PathConfigGap{Name: detail.Name, Reason: pathConfigUnsupportedReason(detail)}
	}
	if (controlType == PathConfigTypeSingleSelect || controlType == PathConfigTypeMultiSelect) && len(detail.Options) == 0 {
		return model.PathConfigField{}, &model.PathConfigGap{Name: detail.Name, Reason: "缺少真实选项，需要人工核对"}
	}
	fieldKey := strings.TrimSpace(detail.EnglishName)
	key := PathConfigFieldToken(nodeID, fieldKey)
	options := pathConfigOptions(detail.Options)
	value, note := p.fieldValue(nodeID, fieldKey, controlType, detail.Required, detail.DefaultValue, options)
	affected := note != "" && strings.Contains(note, "已变化")
	field := model.PathConfigField{
		Key: key, Name: detail.Name, Type: controlType, Required: detail.Required,
		Value: value, Editable: true, Affected: affected, Note: note,
		Options: options,
	}
	if affected {
		p.affected = true
	}
	p.validation.FieldTokens[key] = PathConfigFieldTarget{
		NodeID: nodeID, FieldKey: fieldKey, Name: detail.Name, Type: controlType,
		Required: detail.Required, Options: field.Options,
	}
	return field, nil
}

// fieldValue 按“已保存值优先于实例现值”的规则取值：用户保存过的字段不能被实例现值覆盖。
func (p *pathConfigProjection) fieldValue(nodeID, fieldKey, controlType string, required bool, defaultValue string, options []model.PathConfigOption) (string, string) {
	if stored, exists := p.storedFields[nodeID][fieldKey]; exists {
		valid, reason := validateStoredValue(stored, controlType, required, options)
		if valid {
			return stored, ""
		}
		// 已保存值在当前结构下失效时保留原值并标记受影响，让用户重新选择而不是静默回退到实例现值。
		p.affected = true
		return stored, reason
	}
	// 未保存过配置的已发/待发路径继续展示实例当前值，未编辑字段不会被空值覆盖。
	if p.instanceValues != nil {
		if raw, exists := p.instanceValues[fieldKey]; exists {
			encoded, ok := encodeConfigValue(raw, controlType)
			if ok {
				return encoded, ""
			}
			return "", "当前值来自业务对象或复杂结构，暂不支持配置"
		}
	}
	return encodeDefaultValue(defaultValue, controlType), ""
}

// encodeConfigValue 把实例原始值按字段类型编码为 JSON 文本；复杂对象与类型不匹配返回不可用。
func encodeConfigValue(raw any, controlType string) (string, bool) {
	if raw == nil {
		return encodeDefaultValue("", controlType), true
	}
	switch typed := raw.(type) {
	case map[string]any:
		return "", false
	case []any:
		if controlType != PathConfigTypeMultiSelect {
			return "", false
		}
		for _, item := range typed {
			if _, isScalar := scalarJSONValue(item); !isScalar {
				return "", false
			}
		}
	case string, float64, bool:
		if controlType == PathConfigTypeMultiSelect {
			return "", false
		}
	default:
		return "", false
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return "", false
	}
	return string(data), true
}

// scalarJSONValue 判断 JSON 值是否为字符串、数字或布尔标量。
func scalarJSONValue(value any) (any, bool) {
	switch value.(type) {
	case string, float64, bool:
		return value, true
	default:
		return nil, false
	}
}

// encodeDefaultValue 按控件类型编码空默认值，多选用空数组避免被误判为未初始化。
func encodeDefaultValue(value string, controlType string) string {
	if controlType == PathConfigTypeMultiSelect {
		return "[]"
	}
	if controlType == PathConfigTypeSwitch {
		if value == "true" {
			return "true"
		}
		if value == "false" {
			return "false"
		}
		return "false"
	}
	if controlType == PathConfigTypeNumber {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			if number, err := json.Number(trimmed).Float64(); err == nil {
				data, err := json.Marshal(number)
				if err == nil {
					return string(data)
				}
			}
		}
	}
	if value == "" {
		return "\"\""
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "\"\""
	}
	return string(data)
}

// validateStoredValue 校验已保存值在当前结构下仍然有效；选项或类型变化时返回影响说明。
func validateStoredValue(stored, controlType string, required bool, options []model.PathConfigOption) (bool, string) {
	var parsed any
	if err := json.Unmarshal([]byte(stored), &parsed); err != nil {
		return false, "目标结构已变化，值需要重新核对"
	}
	switch controlType {
	case PathConfigTypeText:
		text, ok := parsed.(string)
		return ok && (!required || strings.TrimSpace(text) != ""), "目标结构已变化，值需要重新核对"
	case PathConfigTypeDate:
		text, ok := parsed.(string)
		return ok && validPathConfigDate(text) && (!required || strings.TrimSpace(text) != ""), "目标结构已变化，值需要重新核对"
	case PathConfigTypeDateTime:
		text, ok := parsed.(string)
		return ok && validPathConfigDateTime(text) && (!required || strings.TrimSpace(text) != ""), "目标结构已变化，值需要重新核对"
	case PathConfigTypeNumber:
		switch typed := parsed.(type) {
		case float64:
			return true, ""
		case string:
			value := strings.TrimSpace(typed)
			if value == "" {
				return !required, "目标结构已变化，值需要重新核对"
			}
			if _, err := json.Number(value).Float64(); err != nil {
				return false, "目标结构已变化，值需要重新核对"
			}
			return true, ""
		default:
			return false, "目标结构已变化，值需要重新核对"
		}
	case PathConfigTypeSingleSelect:
		text, ok := parsed.(string)
		if !ok {
			return false, "目标结构已变化，值需要重新核对"
		}
		if text == "" {
			return !required, "目标结构已变化，值需要重新核对"
		}
		if !pathConfigOptionExists(options, text) {
			return false, "目标选项已变化，需要重新选择"
		}
		return true, ""
	case PathConfigTypeMultiSelect:
		items, ok := parsed.([]any)
		if !ok {
			return false, "目标结构已变化，值需要重新核对"
		}
		if required && len(items) == 0 {
			return false, "目标结构已变化，值需要重新核对"
		}
		for _, item := range items {
			text, ok := item.(string)
			if !ok || !pathConfigOptionExists(options, text) {
				return false, "目标选项已变化，需要重新选择"
			}
		}
		return true, ""
	case PathConfigTypeSwitch:
		_, ok := parsed.(bool)
		return ok, "目标结构已变化，值需要重新核对"
	default:
		return false, "目标结构已变化，值需要重新核对"
	}
}

// pathConfigOptionExists 判断值是否仍属于当前真实选项。
func pathConfigOptionExists(options []model.PathConfigOption, value string) bool {
	for _, option := range options {
		if option.Value == value {
			return true
		}
	}
	return false
}

// pathConfigControlType 把目标字段类型与 FormMaking 组件类型映射为首轮支持的基础控件。
func pathConfigControlType(detail target.FormFieldDetail) (string, bool) {
	switch strings.TrimSpace(detail.ComponentType) {
	case "input", "textarea":
		return PathConfigTypeText, true
	case "number":
		return PathConfigTypeNumber, true
	case "date":
		if strings.EqualFold(strings.TrimSpace(detail.DateMode), "datetime") {
			return PathConfigTypeDateTime, true
		}
		return PathConfigTypeDate, true
	case "select":
		if detail.Multiple {
			return PathConfigTypeMultiSelect, true
		}
		return PathConfigTypeSingleSelect, true
	case "radio":
		return PathConfigTypeSingleSelect, true
	case "checkbox":
		return PathConfigTypeMultiSelect, true
	case "switch":
		return PathConfigTypeSwitch, true
	default:
		if strings.TrimSpace(detail.ComponentType) != "" {
			// 组件类型已知但不属于首轮基础控件时按缺口处理，不能降级成普通文本输入。
			return "", false
		}
		return pathConfigFallbackControlType(detail)
	}
}

// pathConfigFallbackControlType 组件缺失时按字段类型保守映射；listType 因缺少真实选项保持人工核对。
func pathConfigFallbackControlType(detail target.FormFieldDetail) (string, bool) {
	switch strings.TrimSpace(detail.FieldType) {
	case "stringType":
		return PathConfigTypeText, true
	case "intType", "doubleType":
		return PathConfigTypeNumber, true
	case "dateType":
		return PathConfigTypeDate, true
	default:
		return "", false
	}
}

// validPathConfigDate 校验日期字段只接受目标配置工作台使用的 ISO 日期格式。
func validPathConfigDate(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.UTC)
	return err == nil && parsed.Format("2006-01-02") == value
}

// validPathConfigDateTime 校验日期时间字段只接受秒级本地时间格式，拒绝任意文本伪装。
func validPathConfigDateTime(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.UTC)
	return err == nil && parsed.Format("2006-01-02 15:04:05") == value
}

// pathConfigUnsupportedReason 返回未知控件或复杂结构的稳定中文缺口原因。
func pathConfigUnsupportedReason(detail target.FormFieldDetail) string {
	componentType := strings.TrimSpace(detail.ComponentType)
	switch componentType {
	case "subform":
		return "明细表暂不支持配置"
	case "fileupload", "imgupload":
		return "附件暂不支持配置"
	case "editor", "html":
		return "富文本控件暂不支持配置"
	case "cascader", "transfer", "table", "group":
		return "业务对象或复杂控件暂不支持配置"
	case "component", "custom":
		return "自定义组件暂不支持配置"
	default:
		if componentType == "" && strings.TrimSpace(detail.FieldType) == "listType" {
			return "选项来源无法确认，需要人工核对"
		}
		return "控件类型暂不支持配置，需要人工核对"
	}
}

// pathConfigOptions 把目标选项映射为公开安全选项并保持稳定顺序。
func pathConfigOptions(raw []target.FormFieldOption) []model.PathConfigOption {
	result := make([]model.PathConfigOption, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, option := range raw {
		value := strings.TrimSpace(option.Value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		label := strings.TrimSpace(option.Label)
		if label == "" {
			label = value
		}
		result = append(result, model.PathConfigOption{Label: label, Value: value})
	}
	sort.SliceStable(result, func(i, j int) bool {
		left, right := result[i].Value, result[j].Value
		if left == right {
			return false
		}
		return left < right
	})
	return result
}

// nonNilStrings 保证公开警告列表在无警告时仍输出空数组。
func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
