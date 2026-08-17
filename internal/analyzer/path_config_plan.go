package analyzer

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/model"
)

const (
	pathConfigActionPlanVersion = 1
	maxPathConfigAddSignNodes   = 10
	maxPathConfigActionSteps    = 100
	maxPathConfigSafeSeed       = int64(9007199254740991)
)

type storedPathConfigPersonPlan struct {
	Strategy string   `json:"strategy"`
	Seed     int64    `json:"seed"`
	Selected []string `json:"selected"`
}

type storedPathConfigActionPlan struct {
	Version          int                                `json:"version"`
	Arrivals         []storedPathConfigArrival          `json:"arrivals"`
	Actions          []storedPathConfigConfiguredAction `json:"actions,omitempty"`
	CombinationCount int                                `json:"combinationCount,omitempty"`
}

type storedPathConfigConfiguredAction struct {
	Kind   string                      `json:"kind"`
	Count  int                         `json:"count"`
	Target string                      `json:"target,omitempty"`
	Person *storedPathConfigPersonPlan `json:"person,omitempty"`
}

type storedPathConfigArrival struct {
	Visit int                    `json:"visit"`
	Steps []storedPathConfigStep `json:"steps"`
}

type storedPathConfigStep struct {
	Kind    string                      `json:"kind"`
	Opinion string                      `json:"opinion,omitempty"`
	Target  string                      `json:"target,omitempty"`
	Person  *storedPathConfigPersonPlan `json:"person,omitempty"`
}

// projectPathConfigPersonStrategy 兼容旧人员数组与新版策略 JSON，并用当前候选重新确认旧配置。
func projectPathConfigPersonStrategy(nodeID string, stored map[string]string, target *PathConfigPersonTarget, rawToToken map[string]string) (string, int64, []string, bool, string) {
	strategy := "manual"
	seed := stablePathConfigSeed(nodeID)
	seedAffected := false
	selectedIDs := []string{}
	planRaw, hasPlan := stored[PathConfigPersonPlanStorageKey(nodeID)]
	if hasPlan {
		var plan storedPathConfigPersonPlan
		if json.Unmarshal([]byte(planRaw), &plan) != nil {
			return strategy, seed, []string{}, true, "旧人员策略无法解析，需要重新确认"
		}
		strategy, seed, selectedIDs = strings.TrimSpace(plan.Strategy), plan.Seed, append([]string(nil), plan.Selected...)
	} else if legacyRaw, exists := stored[PathConfigPersonStorageKey(nodeID)]; exists {
		if json.Unmarshal([]byte(legacyRaw), &selectedIDs) != nil {
			return strategy, seed, []string{}, true, "旧人员选择无法解析，需要重新确认"
		}
	} else {
		if target.AllowedStrategies["target_default"] {
			strategy = "target_default"
			selectedIDs = append([]string(nil), target.DefaultIDs...)
		}
		return strategy, seed, rawPersonIDsToTokens(selectedIDs, rawToToken), false, ""
	}
	normalizedSeed := normalizePathConfigSeed(seed)
	if normalizedSeed != seed {
		// 旧配置可能来自修复前的 63 位 seed；公开前必须收敛到 JavaScript 可精确表达的范围，避免页面名单与保存名单分叉。
		seedAffected = true
		seed = normalizedSeed
	}
	resolved, reason := resolveStoredPersonStrategy(target, strategy, seed, selectedIDs)
	public := rawPersonIDsToTokens(resolved, rawToToken)
	if reason != "" {
		// 已失效的真实 ID 不向浏览器公开；仍可对应的选择保留下来供用户最小修正。
		return strategy, seed, public, true, reason
	}
	if seedAffected {
		return strategy, seed, public, true, "旧人员策略随机种子超出安全范围，需要重新确认"
	}
	return strategy, seed, public, false, ""
}

// EncodePathConfigPersonStrategy 校验浏览器策略和不透明候选，并编码为仅供 V2 存储的内部 JSON。
func EncodePathConfigPersonStrategy(target PathConfigPersonTarget, input model.PathConfigPersonStrategyInput) (string, string) {
	strategy := strings.TrimSpace(input.Strategy)
	if !target.AllowedStrategies[strategy] {
		return "", "人员策略不属于当前模板允许范围"
	}
	// 浏览器 Number 与 Go int64 的精度边界不同；统一把非法值规范为 1，确保页面预览和服务端最终名单使用同一个 seed。
	seed := normalizePathConfigSeed(input.Seed)
	selectedIDs := make([]string, 0, len(input.Selected))
	seen := make(map[string]bool, len(input.Selected))
	for _, token := range input.Selected {
		token = strings.TrimSpace(token)
		id, exists := target.CandidateTokens[token]
		if token == "" || !exists || seen[token] {
			return "", "包含不属于当前模板的人员候选"
		}
		seen[token] = true
		selectedIDs = append(selectedIDs, id)
	}
	resolved, reason := resolveSubmittedPersonStrategy(&target, strategy, seed, selectedIDs)
	if reason != "" {
		return "", reason
	}
	encoded, err := json.Marshal(storedPathConfigPersonPlan{Strategy: strategy, Seed: seed, Selected: resolved})
	if err != nil {
		return "", "人员策略无法保存"
	}
	return string(encoded), ""
}

// resolveStoredPersonStrategy 重新核对存量策略的最终人员；候选或默认变化必须标记 affected，不能静默改人。
func resolveStoredPersonStrategy(target *PathConfigPersonTarget, strategy string, seed int64, selected []string) ([]string, string) {
	if !target.AllowedStrategies[strategy] {
		return validRawPersonIDs(target, selected), "旧人员策略已不被当前模板允许，需要重新确认"
	}
	expected, reason := expectedPathConfigPersonIDs(target, strategy, seed, selected)
	if reason != "" {
		return validRawPersonIDs(target, selected), reason + "，需要重新确认"
	}
	if strategy != "manual" && !sameStrings(expected, selected) {
		return validRawPersonIDs(target, selected), "目标候选或默认人员已变化，需要重新确认"
	}
	return expected, ""
}

// resolveSubmittedPersonStrategy 根据提交策略生成最终内部人员列表；随机和全选不信任浏览器伪造的结果。
func resolveSubmittedPersonStrategy(target *PathConfigPersonTarget, strategy string, seed int64, selected []string) ([]string, string) {
	return expectedPathConfigPersonIDs(target, strategy, seed, selected)
}

// expectedPathConfigPersonIDs 统一目标默认、手动、确定性随机和全选的人数与范围规则。
func expectedPathConfigPersonIDs(target *PathConfigPersonTarget, strategy string, seed int64, selected []string) ([]string, string) {
	var result []string
	switch strategy {
	case "manual":
		result = validRawPersonIDs(target, selected)
		if len(result) != len(selected) {
			return result, "包含已不属于当前模板的人员候选"
		}
	case "target_default":
		if len(target.DefaultIDs) == 0 {
			return nil, "当前模板没有可用的目标默认人员"
		}
		result = validRawPersonIDs(target, target.DefaultIDs)
	case "all":
		result = validRawPersonIDs(target, target.CandidateOrder)
	case "random":
		count := target.MinCount
		if count < 1 {
			count = 1
		}
		if target.MaxCount > 0 && count > target.MaxCount {
			count = target.MaxCount
		}
		result = deterministicPathConfigPeople(target.CandidateOrder, seed, count)
	default:
		return nil, "人员策略不属于当前模板允许范围"
	}
	if issue := PathConfigPersonSelectionIssue(target.Required, target.MinCount, target.MaxCount, len(result)); issue != "" {
		return result, issue
	}
	return result, ""
}

// deterministicPathConfigPeople 以稳定 seed 轮转目标候选顺序，前后端可得到同一可复现结果且不会扩大范围。
func deterministicPathConfigPeople(candidateIDs []string, seed int64, count int) []string {
	if len(candidateIDs) == 0 || count <= 0 {
		return []string{}
	}
	if count > len(candidateIDs) {
		count = len(candidateIDs)
	}
	start := int(uint64(normalizePathConfigSeed(seed)) % uint64(len(candidateIDs)))
	result := make([]string, 0, count)
	for index := 0; index < count; index++ {
		result = append(result, candidateIDs[(start+index)%len(candidateIDs)])
	}
	return result
}

// stablePathConfigSeed 为未设置 seed 的存量或首次策略生成 JavaScript 可精确表达的稳定正整数。
func stablePathConfigSeed(value string) int64 {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	seed := int64(binary.BigEndian.Uint64(sum[:8]) & uint64(maxPathConfigSafeSeed))
	if seed == 0 {
		return 1
	}
	return seed
}

// normalizePathConfigSeed 把外部或存量 seed 收敛到 Go 与 JavaScript 都能无损表达的统一范围。
func normalizePathConfigSeed(seed int64) int64 {
	if seed < 1 || seed > maxPathConfigSafeSeed {
		return 1
	}
	return seed
}

// validRawPersonIDs 只保留当前候选目录仍可证明存在的内部 ID，并去重。
func validRawPersonIDs(target *PathConfigPersonTarget, values []string) []string {
	valid := make(map[string]bool, len(target.CandidateOrder))
	for _, id := range target.CandidateOrder {
		valid[id] = true
	}
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, id := range values {
		if valid[id] && !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

// rawPersonIDsToTokens 把当前仍合法的内部人员 ID 转成公开不透明 token。
func rawPersonIDsToTokens(values []string, rawToToken map[string]string) []string {
	result := make([]string, 0, len(values))
	for _, id := range values {
		if token := rawToToken[id]; token != "" {
			result = append(result, token)
		}
	}
	return result
}

// sameStrings 判断策略生成的有序结果是否与存量一致，顺序变化也需要重新确认。
func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// actionPlan 生成当前节点动作目录、回退目标和版本化动作计划，并登记动作专用人员范围与保存校验映射。
func (p *pathConfigProjection) actionPlan(nodeID, nodeName, nodeKind string, node *target.FlowNodeTemplate, persons []model.PathConfigPerson) model.PathConfigActionPlan {
	result := model.PathConfigActionPlan{Catalog: []model.PathConfigActionCatalogItem{}, RollbackTargets: []model.PathConfigActionOption{}, Actions: []model.PathConfigConfiguredAction{}, CombinationCount: 1, AddSignNodes: []model.PathConfigAddSignNode{}}
	validationTarget := PathConfigNodeTarget{NodeID: nodeID, Name: nodeName, Person: p.personTargets[nodeID], ActionPersons: make(map[string]*PathConfigPersonTarget), ActionKinds: make(map[string]bool), RollbackTargets: make(map[string]string)}
	for _, person := range persons {
		if person.Mode == "review" || person.Affected {
			validationTarget.Blockers = append(validationTarget.Blockers, model.PathConfigAffectedItem{Kind: "person", Name: person.Title, Reason: firstNonEmptyPathConfig(person.Note, person.Detail)})
		}
	}
	appendAction := func(kind, label, description string, enabled bool, disabledReason string, targetRequired bool, person *model.PathConfigPerson, personTarget *PathConfigPersonTarget) {
		item := model.PathConfigActionCatalogItem{Kind: kind, Label: label, Description: description, Enabled: enabled, DisabledReason: disabledReason, RequiresTarget: targetRequired, RequiresPerson: person != nil, Person: person}
		result.Catalog = append(result.Catalog, item)
		if enabled {
			validationTarget.ActionKinds[kind] = true
		}
		if enabled && personTarget != nil {
			validationTarget.ActionPersons[kind] = personTarget
		}
	}
	switch nodeKind {
	case "start":
		appendAction("submit", "提交", "提交当前发起节点并进入后续流程；执行时仍会核对模板、表单和账号", true, "", false, nil, nil)
		appendDisabledApprovalActions(appendAction, "仅审批或协同节点可用")
	case "common", "synergy":
		appendAction("submit", "提交", "提交只适用于发起节点", false, "仅发起节点可用", false, nil, nil)
		appendAction("approve_pass", "同意", "审批或协同通过并继续当前路径；执行时仍会核对待办、权限、会签和并行状态", true, "", false, nil, nil)
		appendAction("reject_no_pass", "不同意", "审批不通过并回到发起侧；执行时仍会核对待办、权限和并行状态", true, "", false, nil, nil)
		appendAction("draft_save", "暂存", "保存当前处理内容但不推进流程；执行时仍会核对真实任务状态", true, "", false, nil, nil)
		previousID, rollbackReason := p.uniquePreviousBusinessNode(nodeID)
		if previousID != "" {
			previousNode := p.graphNodes[previousID]
			token := pathConfigToken("rollback-target", nodeID, previousID)
			validationTarget.RollbackTargets[token] = previousID
			result.RollbackTargets = append(result.RollbackTargets, model.PathConfigActionOption{Value: token, Label: previousNode.Name})
			appendAction("rollback_previous", "回退上一级", "回退到当前任务链唯一直接上一审批节点；执行时仍会核对实例、待办、pid 和权限", true, "", true, nil, nil)
		} else {
			appendAction("rollback_previous", "回退上一级", "回退目标必须由当前任务链的直接上一待办确定", false, rollbackReason, false, nil, nil)
		}
		if person, personTarget := addSignNodePersonConfig(nodeID, node); person != nil && personTarget != nil {
			// 目标 AddCounterSign 保存的是新审批节点结构；加签节点处理人必须独立于当前节点主处理人范围。
			appendAction("add_sign", "加签节点", "在当前流程中新增审批节点并配置其处理人；执行时仍会核对真实任务权限和目标模板", true, "", false, person, personTarget)
		} else {
			appendAction("add_sign", "加签节点", "在当前流程中新增审批节点", false, addSignDisabledReason(node), false, nil, nil)
		}
		if person, personTarget := transferPersonConfig(nodeID, node); person != nil && personTarget != nil {
			appendAction("transfer_approver", "移交", "把当前处理任务移交给受目标范围约束的一名或多名处理人并结束当前任务", true, "", false, person, personTarget)
			if transpondPerson, transpondTarget := transpondPersonConfig(nodeID, node); transpondPerson != nil && transpondTarget != nil {
				appendAction("transpond", "转发", "将当前处理任务转发给当前节点候选范围内的一名接收人；仅保存配置，不执行目标操作", true, "", false, transpondPerson, transpondTarget)
			}
		} else {
			appendAction("transfer_approver", "移交", "把当前处理任务移交给其他处理人", false, "当前模板未解析到可证明合法的移交人员范围", false, nil, nil)
			appendAction("transpond", "转发", "将当前处理任务转发给接收人", false, "当前模板未解析到可证明合法的转发人员范围", false, nil, nil)
		}
	default:
		appendAction("submit", "提交", "提交只适用于发起节点", false, "当前节点没有业务处理动作", false, nil, nil)
		appendDisabledApprovalActions(appendAction, "当前节点没有业务处理动作")
	}
	if len(validationTarget.ActionKinds) == 0 {
		return result
	}
	addSignNodes, actionResult, affected, note := p.projectStoredActionPlan(nodeID, nodeKind, validationTarget)
	result.AddSignNodes, result.Result, result.Affected, result.Note = addSignNodes, actionResult, affected, note
	result.Actions, result.CombinationCount = p.projectStoredConfiguredActions(nodeID, validationTarget, addSignNodes, actionResult)
	if affected {
		p.affected = true
	}
	p.validation.NodeTokens[PathConfigNodeToken(nodeID)] = validationTarget
	return result
}

// projectStoredConfiguredActions 读取仅供配置页使用的动作组合；旧 JSON 未携带该段时从兼容动作计划稳定派生。
func (p *pathConfigProjection) projectStoredConfiguredActions(nodeID string, target PathConfigNodeTarget, addSignNodes []model.PathConfigAddSignNode, result model.PathConfigActionStep) ([]model.PathConfigConfiguredAction, int) {
	raw := p.storedActions[PathConfigActionPlanStorageKey(nodeID)]
	var stored storedPathConfigActionPlan
	if raw != "" && json.Unmarshal([]byte(raw), &stored) == nil && stored.Version == pathConfigActionPlanVersion && len(stored.Actions) > 0 {
		actions := make([]model.PathConfigConfiguredAction, 0, len(stored.Actions))
		for index, action := range stored.Actions {
			if !target.ActionKinds[action.Kind] || action.Count < 1 || action.Count > maxPathConfigAddSignNodes {
				return derivedConfiguredActions(addSignNodes, result), 1
			}
			item := model.PathConfigConfiguredAction{Key: fmt.Sprintf("action-%d", index+1), Kind: action.Kind, Label: pathConfigActionLabel(action.Kind), Count: action.Count}
			if action.Kind == "rollback_previous" {
				for token, id := range target.RollbackTargets {
					if id == action.Target {
						item.Target = token
						break
					}
				}
				if item.Target == "" {
					return derivedConfiguredActions(addSignNodes, result), 1
				}
			}
			if action.Person != nil {
				person, reason := projectStoredActionPerson(target, storedPathConfigStep{Kind: action.Kind, Person: action.Person})
				if reason != "" {
					return derivedConfiguredActions(addSignNodes, result), 1
				}
				item.Person = person
			}
			actions = append(actions, item)
		}
		count := stored.CombinationCount
		if count < 1 || count > maxPathConfigAddSignNodes {
			count = 1
		}
		return actions, count
	}
	return derivedConfiguredActions(addSignNodes, result), 1
}

// derivedConfiguredActions 将旧加签节点和唯一结果转换为配置页动作列表，保障旧记录刷新后仍可编辑。
func derivedConfiguredActions(addSignNodes []model.PathConfigAddSignNode, result model.PathConfigActionStep) []model.PathConfigConfiguredAction {
	actions := make([]model.PathConfigConfiguredAction, 0, len(addSignNodes)+1)
	for index, addSign := range addSignNodes {
		person := addSign.Person
		actions = append(actions, model.PathConfigConfiguredAction{Key: fmt.Sprintf("action-%d", index+1), Kind: "add_sign", Label: pathConfigActionLabel("add_sign"), Count: addSign.Count, Person: &person})
	}
	resultKind := strings.TrimSpace(result.Kind)
	if resultKind == "" && strings.TrimSpace(result.Label) != "" {
		resultKind = "approve_pass"
	}
	if resultKind != "" {
		actions = append(actions, model.PathConfigConfiguredAction{Key: fmt.Sprintf("action-%d", len(actions)+1), Kind: resultKind, Label: firstNonEmptyPathConfig(result.Label, pathConfigActionLabel(resultKind)), Count: 1, Target: result.Target, Person: result.Person})
	}
	return actions
}

// appendDisabledApprovalActions 为非审批节点补齐稳定动作目录，界面可解释静态不可用原因而不是静默隐藏。
func appendDisabledApprovalActions(appendAction func(string, string, string, bool, string, bool, *model.PathConfigPerson, *PathConfigPersonTarget), reason string) {
	appendAction("approve_pass", "同意", "审批或协同通过", false, reason, false, nil, nil)
	appendAction("reject_no_pass", "不同意", "审批不通过", false, reason, false, nil, nil)
	appendAction("draft_save", "暂存", "暂存当前处理", false, reason, false, nil, nil)
	appendAction("rollback_previous", "回退上一级", "回退到直接上一审批节点", false, reason, false, nil, nil)
	appendAction("add_sign", "加签节点", "新增审批节点", false, reason, false, nil, nil)
	appendAction("transfer_approver", "移交", "移交当前任务", false, reason, false, nil, nil)
	appendAction("transpond", "转发", "转发当前任务", false, reason, false, nil, nil)
}

// transferPersonConfig 从当前节点已解析的合法候选生成移交人员规则，候选不得扩大到节点范围之外。
func transferPersonConfig(nodeID string, node *target.FlowNodeTemplate) (*model.PathConfigPerson, *PathConfigPersonTarget) {
	if node == nil || node.AuditConfig == nil || len(node.AuditConfig.ResolutionIssues) > 0 || len(node.AuditConfig.Candidates) == 0 {
		return nil, nil
	}
	config := node.AuditConfig
	return actionCandidatePersonConfig(nodeID, "transfer_approver", "移交人员", "候选来自当前节点目标模板的合法人员范围", config.Candidates, config.DefaultCandidates)
}

// transpondPersonConfig 复用当前节点合法候选，但按 V1 转发语义严格限制为单个接收人。
func transpondPersonConfig(nodeID string, node *target.FlowNodeTemplate) (*model.PathConfigPerson, *PathConfigPersonTarget) {
	person, personTarget := transferPersonConfig(nodeID, node)
	if person == nil || personTarget == nil {
		return nil, nil
	}
	person.Key = PathConfigPersonToken(nodeID + ":transpond")
	person.Title = "转发接收人"
	person.Multiple = false
	person.MaxCount = 1
	person.Strategies = removePathConfigPersonStrategy(person.Strategies, "all")
	personTarget.Key = person.Key
	personTarget.Name = person.Title
	personTarget.MaxCount = 1
	delete(personTarget.AllowedStrategies, "all")
	return person, personTarget
}

// removePathConfigPersonStrategy 移除与单选边界冲突的全选策略，保留默认、手动和随机策略。
func removePathConfigPersonStrategy(items []model.PathConfigPersonStrategyOption, value string) []model.PathConfigPersonStrategyOption {
	result := make([]model.PathConfigPersonStrategyOption, 0, len(items))
	for _, item := range items {
		if item.Value != value {
			result = append(result, item)
		}
	}
	return result
}

// addSignNodePersonConfig 从目标公司人员目录生成新增审批节点的独立处理人策略，不受当前节点人员配置是否可编辑影响。
func addSignNodePersonConfig(nodeID string, node *target.FlowNodeTemplate) (*model.PathConfigPerson, *PathConfigPersonTarget) {
	if node == nil || len(node.AddSignIssues) > 0 || len(node.AddSignCandidates) == 0 {
		return nil, nil
	}
	return actionCandidatePersonConfig(nodeID, "add_sign", "加签节点处理人", "候选来自当前账号可配置的新审批节点人员目录", node.AddSignCandidates, nil)
}

// addSignDisabledReason 返回加签节点不可配置的明确目录原因，避免错误归因于当前节点人员范围。
func addSignDisabledReason(node *target.FlowNodeTemplate) string {
	if node != nil && len(node.AddSignIssues) > 0 && strings.TrimSpace(node.AddSignIssues[0].Reason) != "" {
		return strings.TrimSpace(node.AddSignIssues[0].Reason)
	}
	return "当前账号没有解析到可配置的加签节点处理人"
}

// actionCandidatePersonConfig 把动作自己的真实人员候选转换为不透明策略模型，并统一人数与随机边界。
func actionCandidatePersonConfig(nodeID, actionKind, title, detail string, candidates, defaults []target.FlowAuditCandidate) (*model.PathConfigPerson, *PathConfigPersonTarget) {
	key := PathConfigPersonToken(nodeID + ":" + actionKind)
	options := make([]model.PathConfigPersonOption, 0, len(candidates))
	candidateTokens := make(map[string]string, len(candidates))
	candidateNames := make(map[string]string, len(candidates))
	rawToToken := make(map[string]string, len(candidates))
	for _, candidate := range candidates {
		token := PathConfigPersonOptionToken(nodeID+":"+actionKind, candidate.ID)
		candidateTokens[token] = candidate.ID
		candidateNames[candidate.ID] = candidate.Name
		rawToToken[candidate.ID] = token
		options = append(options, model.PathConfigPersonOption{Label: candidate.Name, Value: token})
	}
	if len(options) == 0 {
		return nil, nil
	}
	defaultIDs := make([]string, 0, len(defaults))
	defaultSelected := make([]string, 0, len(defaults))
	for _, candidate := range defaults {
		if token, exists := rawToToken[candidate.ID]; exists {
			defaultIDs = append(defaultIDs, candidate.ID)
			defaultSelected = append(defaultSelected, token)
		}
	}
	strategies := []model.PathConfigPersonStrategyOption{{Value: "manual", Label: "手动选择"}, {Value: "random", Label: "确定性随机"}}
	allowed := map[string]bool{"manual": true, "random": true}
	if len(defaultIDs) > 0 {
		strategies = append([]model.PathConfigPersonStrategyOption{{Value: "target_default", Label: "目标默认"}}, strategies...)
		allowed["target_default"] = true
	}
	if len(options) > 1 {
		strategies = append(strategies, model.PathConfigPersonStrategyOption{Value: "all", Label: "全部候选"})
		allowed["all"] = true
	}
	// 目标移交和新增审批节点都允许提交完整人员集合，不能把真实多人能力缩成单选。
	maxCount := len(options)
	multiple := true
	personTarget := &PathConfigPersonTarget{
		Key: key, Name: title, CandidateTokens: candidateTokens, CandidateNames: candidateNames,
		CandidateOrder: candidateOrder(candidates), DefaultIDs: defaultIDs, AllowedStrategies: allowed,
		Required: true, MinCount: 1, MaxCount: maxCount,
	}
	person := &model.PathConfigPerson{
		Key: key, Title: title, Mode: "select", Detail: detail, Editable: true,
		Multiple: multiple, Required: true, MinCount: 1, MaxCount: maxCount, Selected: []string{}, DefaultSelected: defaultSelected,
		Options: options, Strategy: "manual", StrategySeed: stablePathConfigSeed(nodeID + ":action"), Strategies: strategies,
		Items: []model.PathConfigPersonDisplayItem{},
	}
	return person, personTarget
}

// uniquePreviousBusinessNode 反向穿过条件、手动和并行路由，只接受唯一直接上一业务节点。
func (p *pathConfigProjection) uniquePreviousBusinessNode(nodeID string) (string, string) {
	queue := []string{nodeID}
	seen := map[string]bool{nodeID: true}
	candidates := make(map[string]bool)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range p.incoming[current] {
			if !p.reachableEdge[edge.ID] || seen[edge.Source] {
				continue
			}
			seen[edge.Source] = true
			previous := p.graphNodes[edge.Source]
			switch previous.Type {
			case "start", "common", "synergy":
				candidates[previous.ID] = true
			default:
				queue = append(queue, previous.ID)
			}
		}
	}
	if len(candidates) != 1 {
		return "", "无法从当前路径静态确定唯一直接上一审批节点，执行时将按真实任务链复验"
	}
	for candidateID := range candidates {
		if p.graphNodes[candidateID].Type == "start" {
			return "", "上一级为发起人，请使用不同意驳回"
		}
		return candidateID, ""
	}
	return "", "无法从当前路径静态确定唯一直接上一审批节点，执行时将按真实任务链复验"
}

// firstNonEmptyPathConfig 返回首个非空公开说明，避免阻塞项出现空原因。
func firstNonEmptyPathConfig(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "当前配置需要重新核对"
}

// projectStoredActionPlan 兼容旧 agree/disagree，并按当前动作目录重新核验已保存动作计划。
func (p *pathConfigProjection) projectStoredActionPlan(nodeID, nodeKind string, target PathConfigNodeTarget) ([]model.PathConfigAddSignNode, model.PathConfigActionStep, bool, string) {
	raw, exists := p.storedActions[PathConfigActionPlanStorageKey(nodeID)]
	defaultResult := model.PathConfigActionStep{Kind: "approve_pass", Label: pathConfigActionLabel("approve_pass")}
	if nodeKind == "start" {
		defaultResult = model.PathConfigActionStep{Kind: "submit", Label: pathConfigActionLabel("submit")}
	}
	if !exists {
		if nodeKind != "start" {
			if legacy, hasLegacy := p.storedActions[nodeID]; hasLegacy {
				switch strings.TrimSpace(legacy) {
				case "agree":
					defaultResult = model.PathConfigActionStep{Kind: "approve_pass", Label: pathConfigActionLabel("approve_pass")}
				case "disagree":
					defaultResult = model.PathConfigActionStep{Kind: "reject_no_pass", Label: pathConfigActionLabel("reject_no_pass")}
				default:
					return nil, defaultResult, true, "旧节点动作不能准确映射，需要重新确认"
				}
			}
		}
		return nil, defaultResult, false, ""
	}
	var stored storedPathConfigActionPlan
	if json.Unmarshal([]byte(raw), &stored) != nil || stored.Version != pathConfigActionPlanVersion {
		return nil, defaultResult, true, "旧动作计划无法解析，需要重新确认"
	}
	addSignNodes, result, reason := projectStoredActionPlan(target, stored.Arrivals)
	if reason != "" {
		return addSignNodes, defaultResult, true, reason
	}
	return addSignNodes, result, false, ""
}

// projectStoredActionPlan 把内部兼容 JSON 无损收敛为加签节点列表（相邻同人员折叠成次数）和唯一处理结果。
// 同意是普通节点的默认动作，投影时 Kind 留空，不进入界面配置。
func projectStoredActionPlan(target PathConfigNodeTarget, arrivals []storedPathConfigArrival) ([]model.PathConfigAddSignNode, model.PathConfigActionStep, string) {
	if len(arrivals) != 1 || arrivals[0].Visit != 1 || len(arrivals[0].Steps) == 0 {
		return nil, model.PathConfigActionStep{}, "旧动作计划包含重复处理或无法还原的状态推进，需要重新确认"
	}
	steps := arrivals[0].Steps
	if len(steps) > maxPathConfigActionSteps || len(steps)-1 > maxPathConfigAddSignNodes {
		return nil, model.PathConfigActionStep{}, "旧动作计划超过当前可配置范围，需要重新确认"
	}
	addSignNodes := make([]model.PathConfigAddSignNode, 0, len(steps)-1)
	for index, step := range steps {
		if !target.ActionKinds[step.Kind] {
			return addSignNodes, model.PathConfigActionStep{}, "旧动作计划包含当前节点不允许的动作，需要重新确认"
		}
		if index < len(steps)-1 {
			if step.Kind != "add_sign" {
				return addSignNodes, model.PathConfigActionStep{}, "处理结果之前只能配置独立加签节点，需要重新确认"
			}
			person, reason := projectStoredActionPerson(target, step)
			if reason != "" {
				return addSignNodes, model.PathConfigActionStep{}, reason
			}
			// 相邻且人员策略一致的加签节点折叠为同一行并累计次数，界面按次数呈现。
			if len(addSignNodes) > 0 && samePathConfigPersonPlan(addSignNodes[len(addSignNodes)-1].Person, *person) {
				addSignNodes[len(addSignNodes)-1].Count++
				continue
			}
			addSignNodes = append(addSignNodes, model.PathConfigAddSignNode{Person: *person, Count: 1})
			continue
		}
		if !pathConfigTerminalAction(step.Kind) || step.Kind == "submit" && len(steps) != 1 {
			return addSignNodes, model.PathConfigActionStep{}, "动作计划必须以唯一处理结果结束，需要重新确认"
		}
		// 同意是默认动作：投影成空 Kind，界面只显示“默认同意”，不提供同意的配置控件。
		if step.Kind == "approve_pass" {
			return addSignNodes, model.PathConfigActionStep{Label: pathConfigActionLabel("approve_pass")}, ""
		}
		result := model.PathConfigActionStep{Kind: step.Kind, Label: pathConfigActionLabel(step.Kind)}
		if step.Kind == "rollback_previous" {
			for token, id := range target.RollbackTargets {
				if id == step.Target {
					result.Target = token
				}
			}
			if result.Target == "" {
				return addSignNodes, model.PathConfigActionStep{}, "回退目标已不属于当前路径的直接上一审批节点"
			}
		}
		if step.Kind == "transfer_approver" || step.Kind == "transpond" {
			person, reason := projectStoredActionPerson(target, step)
			if reason != "" {
				return addSignNodes, model.PathConfigActionStep{}, reason
			}
			result.Person = person
		}
		return addSignNodes, result, ""
	}
	return addSignNodes, model.PathConfigActionStep{}, "动作计划缺少处理结果，需要重新确认"
}

// samePathConfigPersonPlan 判断两个人员策略是否完全一致，用于折叠相邻同人员加签节点。
func samePathConfigPersonPlan(left, right model.PathConfigPersonStrategyInput) bool {
	if left.Key != right.Key || left.Strategy != right.Strategy || left.Seed != right.Seed || len(left.Selected) != len(right.Selected) {
		return false
	}
	for index := range left.Selected {
		if left.Selected[index] != right.Selected[index] {
			return false
		}
	}
	return true
}

// projectStoredActionPerson 按当前候选把内部人员策略转换为公开不透明值。
func projectStoredActionPerson(target PathConfigNodeTarget, step storedPathConfigStep) (*model.PathConfigPersonStrategyInput, string) {
	personTarget := target.ActionPersons[step.Kind]
	if personTarget == nil || step.Person == nil {
		return nil, "加签节点或移交缺少合法人员策略"
	}
	seed := normalizePathConfigSeed(step.Person.Seed)
	if seed != step.Person.Seed {
		return nil, "加签节点或移交人员随机种子超出安全范围，需要重新确认"
	}
	resolved, reason := resolveStoredPersonStrategy(personTarget, step.Person.Strategy, seed, step.Person.Selected)
	if reason != "" {
		return nil, "加签节点或移交人员已失效：" + reason
	}
	return &model.PathConfigPersonStrategyInput{Key: personTarget.Key, Strategy: step.Person.Strategy, Seed: seed, Selected: rawPersonIDsToTokens(resolved, invertTokenMap(personTarget.CandidateTokens))}, ""
}

// invertTokenMap 构造内部 ID 到公开 token 的反向映射，仅供投影已保存人员策略。
func invertTokenMap(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for token, id := range values {
		result[id] = token
	}
	return result
}

// EncodePathConfigActionPlan 校验语义化加签节点列表和唯一处理结果，再编码为内部兼容 JSON。
// 处理结果 Kind 为空表示默认同意，编码时还原为 approve_pass（发起节点为 submit）；加签次数展开为等量独立步骤。
func EncodePathConfigActionPlan(target PathConfigNodeTarget, input model.PathConfigActionPlanInput) (string, int, string) {
	combinationCount := input.CombinationCount
	if combinationCount == 0 {
		combinationCount = 1
	}
	if combinationCount < 1 || combinationCount > maxPathConfigAddSignNodes {
		return "", 0, "动作组合循环次数必须在当前配置上限内"
	}
	totalAddSigns := 0
	for _, addSign := range input.AddSignNodes {
		if addSign.Count < 1 || addSign.Count > maxPathConfigAddSignNodes {
			return "", 0, "加签次数必须在当前配置上限内"
		}
		totalAddSigns += addSign.Count
	}
	if totalAddSigns > maxPathConfigAddSignNodes {
		return "", 0, "加签节点数量超过当前配置上限"
	}
	resultKind := strings.TrimSpace(input.Result.Kind)
	if resultKind == "" {
		// 空处理结果 = 默认同意；发起节点固定为提交。
		if target.ActionKinds["submit"] {
			resultKind = "submit"
		} else {
			resultKind = "approve_pass"
		}
	}
	if !target.ActionKinds[resultKind] || !pathConfigTerminalAction(resultKind) {
		return "", 0, "处理结果不属于当前节点允许范围"
	}
	if resultKind == "submit" && totalAddSigns > 0 {
		return "", 0, "发起节点只能提交，不能配置加签节点"
	}
	steps := make([]storedPathConfigStep, 0, totalAddSigns+1)
	for _, addSign := range input.AddSignNodes {
		count := addSign.Count
		if count < 1 {
			count = 1
		}
		for range count {
			step, reason := encodePathConfigActionStep(target, model.PathConfigActionStepInput{Kind: "add_sign", Person: &addSign.Person})
			if reason != "" {
				return "", len(steps), reason
			}
			steps = append(steps, step)
		}
	}
	resultStep, reason := encodePathConfigActionStep(target, model.PathConfigActionStepInput{Kind: resultKind, Target: input.Result.Target, Person: input.Result.Person})
	if reason != "" {
		return "", len(steps), reason
	}
	steps = append(steps, resultStep)
	configuredActions, reason := encodeConfiguredActions(target, input.Actions)
	if reason != "" {
		return "", len(steps), reason
	}
	stored := storedPathConfigActionPlan{Version: pathConfigActionPlanVersion, Arrivals: []storedPathConfigArrival{{Visit: 1, Steps: steps}}, Actions: configuredActions, CombinationCount: combinationCount}
	encoded, err := json.Marshal(stored)
	if err != nil {
		return "", len(steps), "动作计划无法保存"
	}
	return string(encoded), len(steps), ""
}

// encodeConfiguredActions 校验并保存配置期动作列表；它不改变既有兼容步骤，也不承担运行时执行。
func encodeConfiguredActions(target PathConfigNodeTarget, actions []model.PathConfigConfiguredActionInput) ([]storedPathConfigConfiguredAction, string) {
	if len(actions) == 0 {
		return []storedPathConfigConfiguredAction{}, ""
	}
	if len(actions) > maxPathConfigAddSignNodes {
		return nil, "动作数量超过当前配置上限"
	}
	result := make([]storedPathConfigConfiguredAction, 0, len(actions))
	for _, action := range actions {
		count := action.Count
		if count < 1 || count > maxPathConfigAddSignNodes {
			return nil, "动作次数必须在当前配置上限内"
		}
		step, reason := encodePathConfigActionStep(target, model.PathConfigActionStepInput{Kind: action.Kind, Target: action.Target, Person: action.Person})
		if reason != "" {
			return nil, reason
		}
		result = append(result, storedPathConfigConfiguredAction{Kind: step.Kind, Count: count, Target: step.Target, Person: step.Person})
	}
	return result, ""
}

// encodePathConfigActionStep 编码处理结果或一个独立加签节点，并重新核验必要参数。
func encodePathConfigActionStep(target PathConfigNodeTarget, input model.PathConfigActionStepInput) (storedPathConfigStep, string) {
	kind := strings.TrimSpace(input.Kind)
	if !target.ActionKinds[kind] {
		return storedPathConfigStep{}, "动作不属于当前节点允许范围"
	}
	stored := storedPathConfigStep{Kind: kind}
	if kind == "rollback_previous" {
		id, exists := target.RollbackTargets[strings.TrimSpace(input.Target)]
		if !exists {
			return storedPathConfigStep{}, "回退目标不属于当前路径的直接上一审批节点"
		}
		stored.Target = id
	}
	if kind == "add_sign" || kind == "transfer_approver" || kind == "transpond" {
		personTarget := target.ActionPersons[kind]
		if personTarget == nil || input.Person == nil {
			return storedPathConfigStep{}, "加签节点或移交缺少合法人员策略"
		}
		personRaw, reason := EncodePathConfigPersonStrategy(*personTarget, *input.Person)
		if reason != "" {
			return storedPathConfigStep{}, "加签节点或移交人员不合法：" + reason
		}
		var person storedPathConfigPersonPlan
		_ = json.Unmarshal([]byte(personRaw), &person)
		stored.Person = &person
	}
	return stored, ""
}

// CountStoredPathConfigActionSteps 统计整条路径版本化动作计划步数；损坏 JSON 返回 false。
func CountStoredPathConfigActionSteps(values map[string]string) (int, bool) {
	total := 0
	for key, raw := range values {
		if !strings.HasPrefix(key, pathConfigActionPlanStoragePrefix) {
			continue
		}
		var plan storedPathConfigActionPlan
		if json.Unmarshal([]byte(raw), &plan) != nil || plan.Version != pathConfigActionPlanVersion {
			return 0, false
		}
		for _, arrival := range plan.Arrivals {
			total += len(arrival.Steps)
		}
	}
	return total, total <= maxPathConfigActionSteps
}

// pathConfigActionPlanBlocksLine 仅把最终不同意视为当前路径后续不再需要配置；回退后的动作由后续动作列表表达。
func pathConfigActionPlanBlocksLine(plan model.PathConfigActionPlan) bool {
	return plan.Result.Kind == "reject_no_pass"
}

// pathConfigTerminalAction 标识结束当前任务处理的动作，其他动作不得排在其后。
func pathConfigTerminalAction(kind string) bool {
	switch kind {
	case "submit", "approve_pass", "reject_no_pass", "draft_save", "rollback_previous":
		return true
	case "transpond":
		return true
	case "transfer_approver":
		// 目标 handOver 成功后立即刷新待办，当前处理任务已经交给新处理人，不能再继续执行同意等动作。
		return true
	default:
		return false
	}
}

// pathConfigActionLabel 返回稳定中文动作名，目标内部枚举不进入公开页面。
func pathConfigActionLabel(kind string) string {
	switch kind {
	case "submit":
		return "提交"
	case "approve_pass":
		return "同意"
	case "reject_no_pass":
		return "不同意"
	case "draft_save":
		return "暂存"
	case "rollback_previous":
		return "回退上一级"
	case "add_sign":
		return "加签节点"
	case "transfer_approver":
		return "移交"
	case "transpond":
		return "转发"
	default:
		return fmt.Sprintf("未知动作（%s）", kind)
	}
}
