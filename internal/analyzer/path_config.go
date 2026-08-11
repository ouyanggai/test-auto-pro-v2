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
	NodeID          string
	StorageKey      string
	Kind            string
	Name            string
	CandidateTokens map[string]string
	Required        bool
	MinCount        int
	MaxCount        int
}

// PathConfigPersonTarget 保存节点人员策略校验所需的当前候选、默认与人数边界。
type PathConfigPersonTarget struct {
	Key               string
	Name              string
	CandidateTokens   map[string]string
	CandidateNames    map[string]string
	CandidateOrder    []string
	DefaultIDs        []string
	AllowedStrategies map[string]bool
	Required          bool
	MinCount          int
	MaxCount          int
}

// PathConfigNodeTarget 保存当前节点动作目录、回退目标和可复用人员范围，不进入公开响应。
type PathConfigNodeTarget struct {
	NodeID          string
	Name            string
	Person          *PathConfigPersonTarget
	ActionKinds     map[string]bool
	RollbackTargets map[string]string
	Blockers        []model.PathConfigAffectedItem
}

// PathConfigValidation 是不透明回写键到当前真实节点与字段的内部映射。
type PathConfigValidation struct {
	FieldTokens  map[string]PathConfigFieldTarget
	ActionTokens map[string]PathConfigActionTarget
	NodeTokens   map[string]PathConfigNodeTarget
	Blockers     []model.PathConfigAffectedItem
}

// PathConfigPersonSelectionIssue 按当前模板判断人员数量是否有效，空字符串表示满足约束。
func PathConfigPersonSelectionIssue(required bool, minCount, maxCount, selectedCount int) string {
	if required && selectedCount == 0 {
		return "选择人数不足"
	}
	// 可跳过只豁免完整的零选择；一旦已有选择，最低人数仍须整体满足。
	if selectedCount > 0 && selectedCount < minCount {
		return "选择人数不足"
	}
	if maxCount > 0 && selectedCount > maxCount {
		return "选择人数超过模板限制"
	}
	return ""
}

const pathConfigPersonStoragePrefix = "person:"

const pathConfigPersonPlanStoragePrefix = "person-plan:"

const pathConfigActionPlanStoragePrefix = "action-plan:"

// PathConfigNodeToken 生成配置节点与真实流程节点之间的稳定不透明映射键。
func PathConfigNodeToken(nodeID string) string {
	return pathConfigToken("node", nodeID, "configuration")
}

// PathConfigPersonToken 生成节点人员选择的不透明回写键。
func PathConfigPersonToken(nodeID string) string {
	return pathConfigToken("person", nodeID, "selection")
}

// PathConfigPersonOptionToken 生成单个合法人员候选的不透明回写键。
func PathConfigPersonOptionToken(nodeID, candidateID string) string {
	return pathConfigToken("person-option", nodeID, candidateID)
}

// PathConfigPersonStorageKey 生成配置表 action_values 内部的人员命名空间键，避免与节点动作键冲突。
func PathConfigPersonStorageKey(nodeID string) string {
	return pathConfigPersonStoragePrefix + strings.TrimSpace(nodeID)
}

// PathConfigPersonPlanStorageKey 生成版本化人员策略的内部 JSON 键。
func PathConfigPersonPlanStorageKey(nodeID string) string {
	return pathConfigPersonPlanStoragePrefix + strings.TrimSpace(nodeID)
}

// PathConfigActionPlanStorageKey 生成版本化到达动作计划的内部 JSON 键。
func PathConfigActionPlanStorageKey(nodeID string) string {
	return pathConfigActionPlanStoragePrefix + strings.TrimSpace(nodeID)
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
	storedPresent ...bool,
) (model.PathConfiguration, PathConfigValidation, error) {
	if tree == nil || !analysis.Complete || len(graph.EntryNodeIDs) == 0 {
		return model.PathConfiguration{}, PathConfigValidation{}, ErrExecutionPathInvalid
	}
	hasStored := len(storedPresent) > 0 && storedPresent[0]
	if !hasStored {
		hasStored = len(storedFields) > 0 || len(storedActions) > 0
	}
	projection := &pathConfigProjection{
		graphNodes:    make(map[string]model.FlowGraphNode, len(graph.Nodes)),
		targetNodes:   make(map[string]*target.FlowNodeTemplate, len(graph.Nodes)),
		outgoing:      make(map[string][]model.FlowGraphEdge, len(graph.Nodes)),
		reachableEdge: make(map[string]bool, len(analysis.ReachableEdgeIDs)),
		choices:       make(map[string]string, len(path.Choices)),
		fields:        fields, instanceValues: instanceValues,
		storedFields: storedFields, storedActions: storedActions, storedPresent: hasStored,
		groupByKey: make(map[string]int), visited: make(map[string]bool),
		personTargets: make(map[string]*PathConfigPersonTarget),
		validation: PathConfigValidation{
			FieldTokens:  make(map[string]PathConfigFieldTarget),
			ActionTokens: make(map[string]PathConfigActionTarget),
			NodeTokens:   make(map[string]PathConfigNodeTarget),
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
	projection.requirements = newConfigRequirementProjection(projection)
	if err := projection.projectEntries(graph.EntryNodeIDs); err != nil {
		return model.PathConfiguration{}, PathConfigValidation{}, err
	}
	result := model.PathConfiguration{
		Path:     model.PathConfigPath{SequenceNo: path.SequenceNo, Name: path.Name},
		Status:   "configured",
		Groups:   projection.groups,
		Warnings: nonNilStrings(projection.warnings),
	}
	result.Progress, result.NextNodeKey = summarizePathConfigProgress(result.Groups)
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
	storedPresent  bool
	requirements   *requirementProjection
	groups         []model.PathConfigGroup
	groupByKey     map[string]int
	visited        map[string]bool
	parallelIndex  int
	validation     PathConfigValidation
	personTargets  map[string]*PathConfigPersonTarget
	businessOrder  []string
	warnings       []string
	affected       bool
}

// newConfigRequirementProjection 复用 F-006 的同一条件、人员和约束翻译规则，避免配置页另建一套业务推导。
func newConfigRequirementProjection(config *pathConfigProjection) *requirementProjection {
	fields := make([]target.FormFieldMetadata, 0, len(config.fields))
	for _, field := range config.fields {
		fields = append(fields, target.FormFieldMetadata{
			FormID: field.FormID, FormName: field.FormName, FieldID: field.FieldID,
			Name: field.Name, EnglishName: field.EnglishName,
		})
	}
	return &requirementProjection{
		graphNodes: config.graphNodes, targetNodes: config.targetNodes, outgoing: config.outgoing,
		reachableEdge: config.reachableEdge, choices: config.choices, fields: fields,
	}
}

// summarizePathConfigProgress 只统计真正需要工具侧处理的节点，并返回第一个待处理节点的不透明键。
func summarizePathConfigProgress(groups []model.PathConfigGroup) (model.PathConfigProgress, string) {
	progress := model.PathConfigProgress{}
	nextKey := ""
	for _, group := range groups {
		for _, node := range group.Nodes {
			switch node.Status {
			case "not_required", "runtime":
				continue
			case "configured":
				progress.Total++
				progress.Completed++
			default:
				progress.Total++
				progress.Pending++
				if nextKey == "" {
					nextKey = node.Key
				}
			}
		}
	}
	return progress, nextKey
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

	nextBlocked := blocked || pathConfigActionPlanBlocksLine(node.ActionPlan)
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
		Key: PathConfigNodeToken(graphNode.ID), Name: graphNode.Name, TypeName: graphNode.TypeName, Kind: graphNode.Type,
		LineBlocked: blocked, Actions: []model.PathConfigAction{}, Persons: []model.PathConfigPerson{}, Requirements: []model.RequirementItem{},
	}
	result.Fields, result.Gaps = p.fieldConfig(node)
	if p.requirements != nil {
		result.Requirements = p.requirements.nodeRequirements(graphNode, node)
	}
	switch graphNode.Type {
	case "start":
		result.Actions = append(result.Actions, p.submitAction(graphNode.ID))
	case "common", "synergy":
		result.Persons = append(result.Persons, p.personConfig(graphNode.ID, node))
		result.Actions = append(result.Actions, p.approvalAction(graphNode.ID, graphNode.Name))
	}
	if graphNode.Type == "start" || graphNode.Type == "common" || graphNode.Type == "synergy" {
		p.businessOrder = append(p.businessOrder, graphNode.ID)
	}
	result.ActionPlan = p.actionPlan(graphNode.ID, graphNode.Name, graphNode.Type, result.Persons, result.Gaps)
	if len(result.Persons) > 0 {
		// 节点侧栏已经用结构化人员项呈现真实名称，移除旧要求中的长文本名单，避免同一信息重复挤占空间。
		result.Requirements = pathConfigNonPersonRequirements(result.Requirements)
	}
	if !blocked {
		for _, gap := range result.Gaps {
			p.validation.Blockers = append(p.validation.Blockers, model.PathConfigAffectedItem{Kind: "field", Name: gap.Name, Reason: gap.Reason})
		}
		for _, person := range result.Persons {
			if person.Mode == "review" {
				p.validation.Blockers = append(p.validation.Blockers, model.PathConfigAffectedItem{Kind: "person", Name: person.Title, Reason: person.Detail})
			}
		}
	}
	result.Status, result.StatusName = pathConfigNodeStatus(result, p.storedPresent)
	return result
}

// pathConfigNonPersonRequirements 仅在节点配置 DTO 中移除已由 Persons 承载的人员要求，不改变 F-006 独立分析结果。
func pathConfigNonPersonRequirements(requirements []model.RequirementItem) []model.RequirementItem {
	result := make([]model.RequirementItem, 0, len(requirements))
	for _, requirement := range requirements {
		if strings.TrimSpace(requirement.Category) == "人员" {
			continue
		}
		result = append(result, requirement)
	}
	return result
}

// pathConfigNodeStatus 按节点可保存项、运行时规则和失效事实派生画布状态，不把结构节点伪装成待配置。
func pathConfigNodeStatus(node model.PathConfigNode, storedPresent bool) (string, string) {
	if node.LineBlocked {
		return "not_required", "无需配置"
	}
	for _, field := range node.Fields {
		if field.Affected {
			return "affected", "配置失效"
		}
	}
	for _, person := range node.Persons {
		if person.Affected {
			return "affected", "配置失效"
		}
	}
	if node.ActionPlan.Affected {
		return "affected", "配置失效"
	}
	if len(node.Gaps) > 0 {
		return "partial", "部分完成"
	}
	hasRuntime := false
	hasEditablePerson := false
	for _, person := range node.Persons {
		if person.Mode == "review" {
			return "partial", "部分完成"
		}
		if person.Mode == "runtime" {
			hasRuntime = true
		}
		if person.Editable {
			hasEditablePerson = true
		}
	}
	hasConfigItem := len(node.Fields) > 0 || len(node.ActionPlan.Catalog) > 0 || hasEditablePerson
	if !hasConfigItem {
		if hasRuntime {
			return "runtime", "运行时确定"
		}
		return "not_required", "无需配置"
	}
	if !storedPresent {
		return "pending", "待配置"
	}
	return "configured", "已完成"
}

// submitAction 发起节点固定提交，不提供其他候选。
func (p *pathConfigProjection) submitAction(nodeID string) model.PathConfigAction {
	key := PathConfigActionToken(nodeID, "submit")
	p.validation.ActionTokens[key] = PathConfigActionTarget{NodeID: nodeID, StorageKey: nodeID, Kind: "submit", Name: "发起动作"}
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
	p.validation.ActionTokens[key] = PathConfigActionTarget{NodeID: nodeID, StorageKey: nodeID, Kind: "agree_disagree", Name: nodeName}
	return model.PathConfigAction{
		Key: key, Kind: "agree_disagree", Label: "处理结果", Current: current, Default: "agree",
		Options: []model.PathConfigActionOption{
			{Value: "agree", Label: "同意"},
			{Value: "disagree", Label: "不同意"},
		},
		DisagreeWarning: "选择不同意会提前结束或改变后续线路，保存后不再按原路径继续",
	}
}

// personConfig 按目标审批类型生成只读规则或受限策略；目录失败不能降级成模糊运行时说明。
func (p *pathConfigProjection) personConfig(nodeID string, node *target.FlowNodeTemplate) model.PathConfigPerson {
	config := node.AuditConfig
	if config == nil || strings.TrimSpace(config.AuditType) == "" {
		return model.PathConfigPerson{Title: "处理人规则", Mode: "review", Detail: "当前节点缺少处理人配置", Items: []model.PathConfigPersonDisplayItem{}, Selected: []string{}, Options: []model.PathConfigPersonOption{}, Strategies: []model.PathConfigPersonStrategyOption{}}
	}
	title, requirementStatus, known := auditTypePresentation(config.AuditType)
	detail := auditModeText(config.Mode, config.CountersignNum)
	items := pathConfigPersonDisplayItems(config)
	if len(config.ResolutionIssues) > 0 {
		reasons := make([]string, 0, len(config.ResolutionIssues))
		for _, issue := range config.ResolutionIssues {
			reasons = append(reasons, issue.Category+"："+issue.Reason)
		}
		return model.PathConfigPerson{Title: title, Mode: "review", Detail: strings.Join(reasons, "；"), Items: items, Selected: []string{}, Options: []model.PathConfigPersonOption{}, Strategies: []model.PathConfigPersonStrategyOption{}}
	}
	if !known || !auditModeValid(config.Mode, config.CountersignNum) {
		return model.PathConfigPerson{Title: title, Mode: "review", Detail: detail + "；处理规则需要人工核对", Items: items, Selected: []string{}, Options: []model.PathConfigPersonOption{}, Strategies: []model.PathConfigPersonStrategyOption{}}
	}
	if strings.TrimSpace(config.AuditType) != "run_node_choose" {
		mode := "fixed"
		if requirementStatus == model.RequirementRuntime || requirementStatus == model.RequirementPending {
			mode = "runtime"
		}
		return model.PathConfigPerson{Title: title, Mode: mode, Detail: detail, Items: items, Selected: []string{}, Options: []model.PathConfigPersonOption{}, Strategies: []model.PathConfigPersonStrategyOption{}}
	}
	if len(config.Candidates) == 0 {
		if len(config.Scopes) > 0 {
			return model.PathConfigPerson{Title: title, Mode: "review", Detail: detail + "；当前合法范围内没有可选人员", Items: items, Selected: []string{}, Options: []model.PathConfigPersonOption{}, Strategies: []model.PathConfigPersonStrategyOption{}}
		}
		// 没有静态范围时只允许明确说明真实依赖，不能伪造全公司候选。
		return model.PathConfigPerson{Title: title, Mode: "runtime", Detail: detail + "；依赖真实任务上下文，只能在真实运行节点到达时加载候选", Items: items, Selected: []string{}, Options: []model.PathConfigPersonOption{}, Strategies: []model.PathConfigPersonStrategyOption{}}
	}
	key := PathConfigPersonToken(nodeID)
	options := make([]model.PathConfigPersonOption, 0, len(config.Candidates))
	candidateTokens := make(map[string]string, len(config.Candidates))
	candidateNames := make(map[string]string, len(config.Candidates))
	rawToToken := make(map[string]string, len(config.Candidates))
	for _, candidate := range config.Candidates {
		token := PathConfigPersonOptionToken(nodeID, candidate.ID)
		candidateTokens[token] = candidate.ID
		candidateNames[candidate.ID] = candidate.Name
		rawToToken[candidate.ID] = token
		options = append(options, model.PathConfigPersonOption{Label: candidate.Name, Value: token})
	}
	multiple := strings.TrimSpace(config.Mode) == "countersign"
	minCount := 1
	maxCount := 1
	if multiple {
		maxCount = len(options)
		if config.CountersignNum != nil && *config.CountersignNum == -1 {
			minCount = len(options)
		} else if config.CountersignNum != nil && *config.CountersignNum > 0 {
			minCount = *config.CountersignNum
		}
	}
	required := node.IsSkip == nil || !*node.IsSkip
	defaultIDs := make([]string, 0, len(config.DefaultCandidates))
	defaultSelected := make([]string, 0, len(config.DefaultCandidates))
	for _, candidate := range config.DefaultCandidates {
		if token, exists := rawToToken[candidate.ID]; exists {
			defaultIDs = append(defaultIDs, candidate.ID)
			defaultSelected = append(defaultSelected, token)
		}
	}
	strategies := []model.PathConfigPersonStrategyOption{{Value: "manual", Label: "手动选择"}, {Value: "random", Label: "确定性随机"}}
	allowedStrategies := map[string]bool{"manual": true, "random": true}
	if len(defaultIDs) > 0 && PathConfigPersonSelectionIssue(required, minCount, maxCount, len(defaultIDs)) == "" {
		strategies = append([]model.PathConfigPersonStrategyOption{{Value: "target_default", Label: "目标默认"}}, strategies...)
		allowedStrategies["target_default"] = true
	}
	if multiple && config.CountersignNum != nil && *config.CountersignNum == -1 {
		strategies = append(strategies, model.PathConfigPersonStrategyOption{Value: "all", Label: "全部候选"})
		allowedStrategies["all"] = true
	}
	target := &PathConfigPersonTarget{
		Key: key, Name: title, CandidateTokens: candidateTokens, CandidateNames: candidateNames,
		CandidateOrder: candidateOrder(config.Candidates), DefaultIDs: defaultIDs, AllowedStrategies: allowedStrategies,
		Required: required, MinCount: minCount, MaxCount: maxCount,
	}
	p.personTargets[nodeID] = target
	strategy, seed, selected, affected, note := projectPathConfigPersonStrategy(nodeID, p.storedActions, target, rawToToken)
	_, hasStoredPlan := p.storedActions[PathConfigPersonPlanStorageKey(nodeID)]
	_, hasLegacySelection := p.storedActions[PathConfigPersonStorageKey(nodeID)]
	if (hasStoredPlan || hasLegacySelection) && !affected {
		if issue := PathConfigPersonSelectionIssue(required, minCount, maxCount, len(selected)); issue != "" {
			affected = true
			note = issue + "，需要重新确认"
		}
	}
	if p.storedPresent && !hasStoredPlan && !hasLegacySelection && required {
		affected = true
		note = "旧配置缺少人员策略，需要重新确认"
	}
	if affected {
		p.affected = true
	}
	// 旧整份保存接口继续接受原人员键；逐节点新接口只使用 NodeTokens 中的策略目标。
	p.validation.ActionTokens[key] = PathConfigActionTarget{
		NodeID: nodeID, StorageKey: PathConfigPersonStorageKey(nodeID), Kind: "person_select", Name: title,
		CandidateTokens: candidateTokens, Required: required, MinCount: minCount, MaxCount: maxCount,
	}
	return model.PathConfigPerson{
		Key: key, Title: title, Mode: "select", Detail: detail, Items: items, Editable: true, Multiple: multiple,
		Required: required, MinCount: minCount, Selected: selected, DefaultSelected: defaultSelected, Options: options,
		MaxCount: maxCount, Strategy: strategy, StrategySeed: seed, Strategies: strategies, Affected: affected, Note: note,
	}
}

// candidateOrder 保持目标候选响应顺序，确定性随机不能依赖 Go map 的非稳定遍历。
func candidateOrder(candidates []target.FlowAuditCandidate) []string {
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if id := strings.TrimSpace(candidate.ID); id != "" {
			result = append(result, id)
		}
	}
	return result
}

// pathConfigPersonDisplayItems 只把目标已证明的具体中文名称投影为展示项；动态依赖或目录失败统一由明确状态说明承载。
func pathConfigPersonDisplayItems(config *target.FlowNodeAuditConfig) []model.PathConfigPersonDisplayItem {
	if config == nil {
		return []model.PathConfigPersonDisplayItem{}
	}
	items := make([]model.PathConfigPersonDisplayItem, 0, len(config.Details)+len(config.Scopes))
	positions := make(map[string]int, len(config.Details)+len(config.Scopes))
	appendItem := func(category, name string) {
		category = strings.TrimSpace(category)
		name = strings.TrimSpace(name)
		if category == "" {
			category = "处理对象"
		}
		if name == "" {
			// 无名称对象不能以模糊占位伪装为已解析；person.detail 或 ResolutionIssues 会给出具体动态依赖或读取失败原因。
			return
		}
		key := category + "\x00" + name
		if index, exists := positions[key]; exists {
			items[index].Count++
			return
		}
		positions[key] = len(items)
		items = append(items, model.PathConfigPersonDisplayItem{Category: category, Name: name, Count: 1})
	}
	for _, detail := range config.Details {
		appendItem(pathConfigPersonCategory(detail.Type, config.AuditType), detail.Name)
	}
	for _, scope := range config.Scopes {
		appendItem(pathConfigPersonCategory(scope.Type, config.AuditType), scope.Name)
	}
	return items
}

// pathConfigPersonCategory 将目标审批详情和范围枚举映射为用户可读类别，未知值不原样公开目标代码。
func pathConfigPersonCategory(value, auditType string) string {
	switch strings.TrimSpace(value) {
	case "personnel":
		return "人员"
	case "position":
		return "岗位"
	case "level", "grade":
		return "岗级"
	case "role", "c":
		return "角色"
	case "department":
		return "部门"
	case "company":
		return "公司"
	case "extendedAttribute":
		return "扩展属性"
	}
	switch strings.TrimSpace(auditType) {
	case "assign", "company", "initiator", "form_person":
		return "人员"
	case "position":
		return "岗位"
	case "level":
		return "岗级"
	case "role":
		return "角色"
	case "department", "department_supervisor", "branched_passage_manager":
		return "部门"
	case "company_id":
		return "公司"
	case "extendedAttribute":
		return "扩展属性"
	case "run_node_choose":
		return "候选范围"
	default:
		return "处理对象"
	}
}

// pathConfigStoredPersonSelection 把内部候选 ID 恢复为当前响应的不透明键，失效候选只标记影响而不猜测替代项。
func pathConfigStoredPersonSelection(raw string, rawToToken map[string]string) ([]string, bool, string) {
	if strings.TrimSpace(raw) == "" {
		return []string{}, false, ""
	}
	var stored []string
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return []string{}, true, "已保存人员数据无法识别，需要重新确认"
	}
	selected := make([]string, 0, len(stored))
	for _, candidateID := range stored {
		token, exists := rawToToken[candidateID]
		if !exists {
			return selected, true, "目标人员候选已变化，需要重新选择"
		}
		selected = append(selected, token)
	}
	return selected, false, ""
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
