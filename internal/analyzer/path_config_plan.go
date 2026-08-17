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
	pathConfigActionConfigurationVersion = 1
	maxPathConfigConfiguredActions       = 10
	maxPathConfigActionExecutions        = 100
	maxPathConfigSafeSeed                = int64(9007199254740991)
)

type storedPathConfigPersonPlan struct {
	Strategy string   `json:"strategy"`
	Seed     int64    `json:"seed"`
	Selected []string `json:"selected"`
}
type storedPathConfigActionConfiguration struct {
	Version int                                `json:"version"`
	Actions []storedPathConfigConfiguredAction `json:"actions"`
}
type storedPathConfigConfiguredAction struct {
	Kind   string                      `json:"kind"`
	Count  int                         `json:"count"`
	Person *storedPathConfigPersonPlan `json:"person,omitempty"`
}

// projectPathConfigPersonStrategy 只读取当前人员策略结构；开发阶段的旧数组不再转换或展示。
func projectPathConfigPersonStrategy(nodeID string, stored map[string]string, target *PathConfigPersonTarget, rawToToken map[string]string) (string, int64, []string, bool, string) {
	strategy, seed := "random", stablePathConfigSeed(nodeID)
	planRaw, hasPlan := stored[PathConfigPersonPlanStorageKey(nodeID)]
	if !hasPlan {
		resolved, reason := expectedPathConfigPersonIDs(target, strategy, seed, nil)
		if reason != "" {
			return strategy, seed, []string{}, true, reason
		}
		return strategy, seed, rawPersonIDsToTokens(resolved, rawToToken), false, ""
	}
	var plan storedPathConfigPersonPlan
	if json.Unmarshal([]byte(planRaw), &plan) != nil {
		return strategy, seed, []string{}, true, "已保存的人员配置无法解析，请重新配置"
	}
	strategy, seed = strings.TrimSpace(plan.Strategy), normalizePathConfigSeed(plan.Seed)
	resolved, reason := resolveStoredPersonStrategy(target, strategy, seed, plan.Selected)
	if reason != "" {
		return strategy, seed, rawPersonIDsToTokens(resolved, rawToToken), true, reason
	}
	return strategy, seed, rawPersonIDsToTokens(resolved, rawToToken), false, ""
}

// EncodePathConfigPersonStrategy 校验浏览器人员策略，并仅编码当前的工具侧 JSON。
func EncodePathConfigPersonStrategy(target PathConfigPersonTarget, input model.PathConfigPersonStrategyInput) (string, string) {
	strategy := strings.TrimSpace(input.Strategy)
	if !target.AllowedStrategies[strategy] {
		return "", "人员策略不属于当前模板允许范围"
	}
	seed, selectedIDs, seen := normalizePathConfigSeed(input.Seed), make([]string, 0, len(input.Selected)), map[string]bool{}
	for _, token := range input.Selected {
		token = strings.TrimSpace(token)
		id, exists := target.CandidateTokens[token]
		if token == "" || !exists || seen[token] {
			return "", "包含不属于当前模板的人员候选"
		}
		seen[token] = true
		selectedIDs = append(selectedIDs, id)
	}
	resolved, reason := expectedPathConfigPersonIDs(&target, strategy, seed, selectedIDs)
	if reason != "" {
		return "", reason
	}
	encoded, err := json.Marshal(storedPathConfigPersonPlan{Strategy: strategy, Seed: seed, Selected: resolved})
	if err != nil {
		return "", "人员配置暂时无法保存"
	}
	return string(encoded), ""
}

// resolveStoredPersonStrategy 重新核对已经保存的人员，候选变化时要求人工确认。
func resolveStoredPersonStrategy(target *PathConfigPersonTarget, strategy string, seed int64, selected []string) ([]string, string) {
	if !target.AllowedStrategies[strategy] {
		return validRawPersonIDs(target, selected), "已保存的人员策略不再适用，请重新配置"
	}
	expected, reason := expectedPathConfigPersonIDs(target, strategy, seed, selected)
	if reason != "" {
		return validRawPersonIDs(target, selected), reason + "，请重新配置"
	}
	if strategy != "manual" && !sameStrings(expected, selected) {
		return validRawPersonIDs(target, selected), "目标默认人员或候选已变化，请重新配置"
	}
	return expected, ""
}

// expectedPathConfigPersonIDs 在服务端重算策略结果，不能信任浏览器传入的随机或全选结果。
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

// deterministicPathConfigPeople 用稳定 seed 选择目标候选，保证前后端预览一致。
func deterministicPathConfigPeople(candidateIDs []string, seed int64, count int) []string {
	if len(candidateIDs) == 0 || count <= 0 {
		return []string{}
	}
	if count > len(candidateIDs) {
		count = len(candidateIDs)
	}
	start, result := int(uint64(normalizePathConfigSeed(seed))%uint64(len(candidateIDs))), make([]string, 0, count)
	for index := 0; index < count; index++ {
		result = append(result, candidateIDs[(start+index)%len(candidateIDs)])
	}
	return result
}

// stablePathConfigSeed 为首次人员策略生成 JavaScript 可精确表示的稳定正整数。
func stablePathConfigSeed(value string) int64 {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	seed := int64(binary.BigEndian.Uint64(sum[:8]) & uint64(maxPathConfigSafeSeed))
	if seed == 0 {
		return 1
	}
	return seed
}

// normalizePathConfigSeed 收敛外部 seed 到 Go 与 JavaScript 的共同精确范围。
func normalizePathConfigSeed(seed int64) int64 {
	if seed < 1 || seed > maxPathConfigSafeSeed {
		return 1
	}
	return seed
}

// validRawPersonIDs 仅保留当前目录仍存在的内部人员，并保持原始顺序。
func validRawPersonIDs(target *PathConfigPersonTarget, values []string) []string {
	valid, seen, result := map[string]bool{}, map[string]bool{}, make([]string, 0, len(values))
	for _, id := range target.CandidateOrder {
		valid[id] = true
	}
	for _, id := range values {
		if valid[id] && !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

// rawPersonIDsToTokens 将服务端内部人员 ID 转回安全的浏览器候选键。
func rawPersonIDsToTokens(values []string, rawToToken map[string]string) []string {
	result := make([]string, 0, len(values))
	for _, id := range values {
		if token := rawToToken[id]; token != "" {
			result = append(result, token)
		}
	}
	return result
}

// sameStrings 判断人员选择是否连顺序也保持一致。
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

// actionConfiguration 投影 F-008 独立动作目录；每一项仅代表一次真实到达后的一个动作。
func (p *pathConfigProjection) actionConfiguration(nodeID, nodeName, nodeKind string, node *target.FlowNodeTemplate, persons []model.PathConfigPerson) model.PathConfigActionConfiguration {
	result := model.PathConfigActionConfiguration{Catalog: []model.PathConfigActionCatalogItem{}, Actions: []model.PathConfigConfiguredAction{}}
	validationTarget := PathConfigNodeTarget{NodeID: nodeID, Name: nodeName, Person: p.personTargets[nodeID], ActionPersons: map[string]*PathConfigPersonTarget{}, ActionKinds: map[string]bool{}}
	for _, person := range persons {
		if person.Mode == "review" || person.Affected {
			validationTarget.Blockers = append(validationTarget.Blockers, model.PathConfigAffectedItem{Kind: "person", Name: person.Title, Reason: firstNonEmptyPathConfig(person.Note, person.Detail)})
		}
	}
	appendAction := func(kind, label, description string, person *model.PathConfigPerson, personTarget *PathConfigPersonTarget) {
		result.Catalog = append(result.Catalog, model.PathConfigActionCatalogItem{Kind: kind, Label: label, Description: description, Enabled: true, RequiresPerson: person != nil, Person: person})
		validationTarget.ActionKinds[kind] = true
		if personTarget != nil {
			validationTarget.ActionPersons[kind] = personTarget
		}
	}
	switch nodeKind {
	case "start":
		result.Base = &model.PathConfigActionBase{Kind: "submit", Label: "提交", Count: 1, Detail: "发起节点固定提交 1 次"}
	case "common", "synergy":
		result.Base = &model.PathConfigActionBase{Kind: "approve_pass", Label: "同意", Count: 1, Detail: "系统默认同意 1 次"}
		appendAction("reject_no_pass", "不同意", "不同意后回到发起人；重新提交会重新解析条件、并行和人员。", nil, nil)
		appendAction("draft_save", "暂存", "暂存不推进待办，不能加入静态循环。", nil, nil)
		if previousID, _ := p.uniquePreviousBusinessNode(nodeID); previousID != "" {
			appendAction("rollback_previous", "回退上一步", "引擎只会回到真实上一待办，不能指定其他目标。", nil, nil)
		}
		if person, personTarget := addSignNodePersonConfig(nodeID, node); person != nil && personTarget != nil {
			appendAction("add_sign", "加签", "新增审批节点；不能加入静态循环。", person, personTarget)
		}
	}
	if len(validationTarget.ActionKinds) == 0 {
		return result
	}
	actions, affected, note := p.projectStoredActionConfiguration(nodeID, validationTarget)
	result.Actions, result.Affected, result.Note = actions, affected, note
	if affected {
		p.affected = true
	}
	p.validation.NodeTokens[PathConfigNodeToken(nodeID)] = validationTarget
	return result
}

// projectStoredActionConfiguration 只读取 F-008 actions 命名空间，旧动作计划不会进入新 DTO。
func (p *pathConfigProjection) projectStoredActionConfiguration(nodeID string, target PathConfigNodeTarget) ([]model.PathConfigConfiguredAction, bool, string) {
	raw, exists := p.storedActions[PathConfigActionConfigurationStorageKey(nodeID)]
	if !exists || strings.TrimSpace(raw) == "" {
		return []model.PathConfigConfiguredAction{}, false, ""
	}
	var stored storedPathConfigActionConfiguration
	if json.Unmarshal([]byte(raw), &stored) != nil || stored.Version != pathConfigActionConfigurationVersion {
		return []model.PathConfigConfiguredAction{}, true, "已保存的动作配置无法解析，请重新配置"
	}
	if len(stored.Actions) == 0 || len(stored.Actions) > maxPathConfigConfiguredActions {
		return []model.PathConfigConfiguredAction{}, true, "已保存的动作配置数量不合法，请重新配置"
	}
	result := make([]model.PathConfigConfiguredAction, 0, len(stored.Actions))
	for index, action := range stored.Actions {
		if !target.ActionKinds[action.Kind] || action.Count < 1 || action.Count > maxPathConfigConfiguredActions {
			return []model.PathConfigConfiguredAction{}, true, "已保存的动作不再适用于当前节点，请重新配置"
		}
		item := model.PathConfigConfiguredAction{Key: fmt.Sprintf("action-%d", index+1), Kind: action.Kind, Label: pathConfigActionLabel(action.Kind), Count: action.Count}
		if personTarget := target.ActionPersons[action.Kind]; personTarget != nil {
			person, reason := projectStoredActionPerson(personTarget, action.Person)
			if reason != "" {
				return []model.PathConfigConfiguredAction{}, true, reason
			}
			item.Person = person
		} else if action.Person != nil {
			return []model.PathConfigConfiguredAction{}, true, "已保存的动作人员参数不再适用，请重新配置"
		}
		result = append(result, item)
	}
	return result, false, ""
}

// projectStoredActionPerson 将内部动作人员策略按当前候选重新投影。
func projectStoredActionPerson(target *PathConfigPersonTarget, stored *storedPathConfigPersonPlan) (*model.PathConfigPersonStrategyInput, string) {
	if target == nil || stored == nil {
		return nil, "动作缺少必要人员配置，请重新配置"
	}
	resolved, reason := resolveStoredPersonStrategy(target, strings.TrimSpace(stored.Strategy), normalizePathConfigSeed(stored.Seed), stored.Selected)
	if reason != "" {
		return nil, reason
	}
	return &model.PathConfigPersonStrategyInput{Key: target.Key, Strategy: stored.Strategy, Seed: normalizePathConfigSeed(stored.Seed), Selected: rawPersonIDsToTokens(resolved, invertTokenMap(target.CandidateTokens))}, ""
}

// invertTokenMap 把公开候选 token 索引反转为内部 ID 到 token 的只读映射。
func invertTokenMap(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for token, value := range values {
		result[value] = token
	}
	return result
}

// EncodePathConfigActions 校验一次到达只能安排一个动作的 F-008 列表，并编码为版本化存储。
func EncodePathConfigActions(target PathConfigNodeTarget, input []model.PathConfigConfiguredActionInput) (string, int, string) {
	if len(input) == 0 {
		return "", 0, "请至少配置一个节点动作"
	}
	if len(input) > maxPathConfigConfiguredActions {
		return "", 0, "单个节点最多配置 10 个动作"
	}
	stored, total := storedPathConfigActionConfiguration{Version: pathConfigActionConfigurationVersion, Actions: make([]storedPathConfigConfiguredAction, 0, len(input))}, 0
	for _, action := range input {
		kind := strings.TrimSpace(action.Kind)
		if !target.ActionKinds[kind] {
			return "", 0, "动作不属于当前节点允许范围"
		}
		if action.Count < 1 || action.Count > maxPathConfigConfiguredActions {
			return "", 0, "动作次数必须在 1 到 10 之间"
		}
		item := storedPathConfigConfiguredAction{Kind: kind, Count: action.Count}
		if personTarget := target.ActionPersons[kind]; personTarget != nil {
			if action.Person == nil || strings.TrimSpace(action.Person.Key) != personTarget.Key {
				return "", 0, "动作缺少必要人员配置"
			}
			encoded, reason := EncodePathConfigPersonStrategy(*personTarget, *action.Person)
			if reason != "" {
				return "", 0, "动作人员：" + reason
			}
			var person storedPathConfigPersonPlan
			if json.Unmarshal([]byte(encoded), &person) != nil {
				return "", 0, "动作人员配置暂时无法保存"
			}
			item.Person = &person
		} else if action.Person != nil {
			return "", 0, "当前动作不需要人员参数"
		}
		total += action.Count
		if total > maxPathConfigActionExecutions {
			return "", 0, "整条路径的动作总数不能超过 100 个"
		}
		stored.Actions = append(stored.Actions, item)
	}
	encoded, err := json.Marshal(stored)
	if err != nil {
		return "", 0, "动作配置暂时无法保存"
	}
	return string(encoded), total, ""
}

// CountStoredPathConfigActionExecutions 统计新动作配置的总真实到达次数，并拒绝损坏数据。
func CountStoredPathConfigActionExecutions(values map[string]string) (int, bool) {
	total := 0
	for key, raw := range values {
		if !strings.HasPrefix(key, pathConfigActionConfigurationStoragePrefix) {
			continue
		}
		var stored storedPathConfigActionConfiguration
		if json.Unmarshal([]byte(raw), &stored) != nil || stored.Version != pathConfigActionConfigurationVersion || len(stored.Actions) == 0 || len(stored.Actions) > maxPathConfigConfiguredActions {
			return total, false
		}
		for _, action := range stored.Actions {
			if strings.TrimSpace(action.Kind) == "" || action.Count < 1 || action.Count > maxPathConfigConfiguredActions {
				return total, false
			}
			total += action.Count
			if total > maxPathConfigActionExecutions {
				return total, false
			}
		}
	}
	return total, true
}

// addSignNodePersonConfig 使用目标新审批节点人员目录生成加签人员策略。
func addSignNodePersonConfig(nodeID string, node *target.FlowNodeTemplate) (*model.PathConfigPerson, *PathConfigPersonTarget) {
	if node == nil || len(node.AddSignIssues) > 0 || len(node.AddSignCandidates) == 0 {
		return nil, nil
	}
	return actionCandidatePersonConfig(nodeID, "add_sign", "加签处理人", "候选来自当前账号可配置的新审批节点人员目录", node.AddSignCandidates, nil)
}

// actionCandidatePersonConfig 将目标人员候选转换为动作私有的不透明策略模型。
func actionCandidatePersonConfig(nodeID, actionKind, title, detail string, candidates, defaults []target.FlowAuditCandidate) (*model.PathConfigPerson, *PathConfigPersonTarget) {
	key, options, candidateTokens, rawToToken := PathConfigPersonToken(nodeID+":"+actionKind), make([]model.PathConfigPersonOption, 0, len(candidates)), map[string]string{}, map[string]string{}
	for _, candidate := range candidates {
		token := PathConfigPersonOptionToken(nodeID+":"+actionKind, candidate.ID)
		candidateTokens[token], rawToToken[candidate.ID] = candidate.ID, token
		options = append(options, model.PathConfigPersonOption{Label: candidate.Name, Value: token})
	}
	if len(options) == 0 {
		return nil, nil
	}
	defaultIDs, defaultSelected := []string{}, []string{}
	for _, candidate := range defaults {
		if token := rawToToken[candidate.ID]; token != "" {
			defaultIDs, defaultSelected = append(defaultIDs, candidate.ID), append(defaultSelected, token)
		}
	}
	strategies, allowed := []model.PathConfigPersonStrategyOption{{Value: "manual", Label: "手动选择"}, {Value: "random", Label: "确定性随机"}}, map[string]bool{"manual": true, "random": true}
	if len(defaultIDs) > 0 {
		strategies, allowed["target_default"] = append([]model.PathConfigPersonStrategyOption{{Value: "target_default", Label: "目标默认"}}, strategies...), true
	}
	if len(options) > 1 {
		strategies, allowed["all"] = append(strategies, model.PathConfigPersonStrategyOption{Value: "all", Label: "全部候选"}), true
	}
	personTarget := &PathConfigPersonTarget{Key: key, Name: title, CandidateTokens: candidateTokens, CandidateOrder: candidateOrder(candidates), DefaultIDs: defaultIDs, AllowedStrategies: allowed, Required: true, MinCount: 1, MaxCount: len(options)}
	seed := stablePathConfigSeed(nodeID + ":action")
	randomIDs := deterministicPathConfigPeople(candidateOrder(candidates), seed, 1)
	randomSelected := make([]string, 0, len(randomIDs))
	for _, id := range randomIDs {
		if token := rawToToken[id]; token != "" {
			randomSelected = append(randomSelected, token)
		}
	}
	person := &model.PathConfigPerson{Key: key, Title: title, Mode: "select", Detail: detail, Editable: true, Multiple: true, Required: true, MinCount: 1, MaxCount: len(options), Selected: randomSelected, DefaultSelected: defaultSelected, Options: options, Strategy: "random", StrategySeed: seed, Strategies: strategies, Items: []model.PathConfigPersonDisplayItem{}}
	return person, personTarget
}

// uniquePreviousBusinessNode 只接受当前已选路径上的唯一真实业务前驱，发起节点永不作为回退目标。
func (p *pathConfigProjection) uniquePreviousBusinessNode(nodeID string) (string, string) {
	queue, seen, candidates := []string{nodeID}, map[string]bool{nodeID: true}, map[string]bool{}
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
		return "", "无法从当前路径确定唯一真实上一待办"
	}
	for candidateID := range candidates {
		if p.graphNodes[candidateID].Type == "start" {
			return "", "上一步为发起人，请使用不同意"
		}
		return candidateID, ""
	}
	return "", "无法从当前路径确定唯一真实上一待办"
}

// firstNonEmptyPathConfig 返回用于公开反馈的第一个有效原因。
func firstNonEmptyPathConfig(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "当前配置需要重新核对"
}

// pathConfigActionLabel 将动作键固定映射为用户可见中文名称。
func pathConfigActionLabel(kind string) string {
	switch strings.TrimSpace(kind) {
	case "submit":
		return "提交"
	case "approve_pass":
		return "同意"
	case "reject_no_pass":
		return "不同意"
	case "draft_save":
		return "暂存"
	case "rollback_previous":
		return "回退上一步"
	case "add_sign":
		return "加签"
	default:
		return "节点动作"
	}
}
