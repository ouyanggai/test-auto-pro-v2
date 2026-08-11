package analyzer

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"

	"test-auto-pro-v2/internal/model"
)

const (
	pathConfigActionPlanVersion = 1
	maxPathConfigArrivals       = 10
	maxPathConfigActionSteps    = 100
)

type storedPathConfigPersonPlan struct {
	Strategy string   `json:"strategy"`
	Seed     int64    `json:"seed"`
	Selected []string `json:"selected"`
}

type storedPathConfigActionPlan struct {
	Version  int                       `json:"version"`
	Arrivals []storedPathConfigArrival `json:"arrivals"`
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
	if seed == 0 {
		seed = stablePathConfigSeed(nodeID)
	}
	resolved, reason := resolveStoredPersonStrategy(target, strategy, seed, selectedIDs)
	public := rawPersonIDsToTokens(resolved, rawToToken)
	if reason != "" {
		// 已失效的真实 ID 不向浏览器公开；仍可对应的选择保留下来供用户最小修正。
		return strategy, seed, public, true, reason
	}
	return strategy, seed, public, false, ""
}

// EncodePathConfigPersonStrategy 校验浏览器策略和不透明候选，并编码为仅供 V2 存储的内部 JSON。
func EncodePathConfigPersonStrategy(target PathConfigPersonTarget, input model.PathConfigPersonStrategyInput) (string, string) {
	strategy := strings.TrimSpace(input.Strategy)
	if !target.AllowedStrategies[strategy] {
		return "", "人员策略不属于当前模板允许范围"
	}
	seed := input.Seed
	if seed == 0 {
		seed = stablePathConfigSeed(target.Key)
	}
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
	start := int(uint64(seed) % uint64(len(candidateIDs)))
	result := make([]string, 0, count)
	for index := 0; index < count; index++ {
		result = append(result, candidateIDs[(start+index)%len(candidateIDs)])
	}
	return result
}

// stablePathConfigSeed 为未设置 seed 的存量或首次策略生成稳定非零值。
func stablePathConfigSeed(value string) int64 {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	seed := int64(binary.BigEndian.Uint64(sum[:8]) & uint64(^uint64(0)>>1))
	if seed == 0 {
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

// actionPlan 生成当前节点动作目录、回退目标和版本化到达计划，并登记保存校验映射。
func (p *pathConfigProjection) actionPlan(nodeID, nodeName, nodeKind string, persons []model.PathConfigPerson, gaps []model.PathConfigGap) model.PathConfigActionPlan {
	result := model.PathConfigActionPlan{Catalog: []model.PathConfigActionCatalogItem{}, RollbackTargets: []model.PathConfigActionOption{}, Arrivals: []model.PathConfigArrivalPlan{}, MaxArrivals: maxPathConfigArrivals, MaxPathSteps: maxPathConfigActionSteps}
	target := PathConfigNodeTarget{NodeID: nodeID, Name: nodeName, Person: p.personTargets[nodeID], ActionKinds: make(map[string]bool), RollbackTargets: make(map[string]string)}
	for _, person := range persons {
		if person.Mode == "review" || person.Affected {
			target.Blockers = append(target.Blockers, model.PathConfigAffectedItem{Kind: "person", Name: person.Title, Reason: firstNonEmptyPathConfig(person.Note, person.Detail)})
		}
	}
	for _, gap := range gaps {
		target.Blockers = append(target.Blockers, model.PathConfigAffectedItem{Kind: "field", Name: gap.Name, Reason: gap.Reason})
	}
	appendAction := func(kind, label, description string, opinion, targetRequired, personRequired bool) {
		result.Catalog = append(result.Catalog, model.PathConfigActionCatalogItem{Kind: kind, Label: label, Description: description, AllowsOpinion: opinion, RequiresTarget: targetRequired, RequiresPerson: personRequired})
		target.ActionKinds[kind] = true
	}
	switch nodeKind {
	case "start":
		appendAction("submit", "提交", "提交当前发起节点并进入后续流程", false, false, false)
	case "common", "synergy":
		appendAction("approve_pass", "同意", "审批或协同通过并继续当前路径", true, false, false)
		appendAction("reject_no_pass", "不同意", "审批不通过，可能改变或结束后续线路", true, false, false)
		appendAction("draft_save", "暂存", "保存当前处理内容但不推进流程", false, false, false)
		for _, earlierID := range p.businessOrder[:maxInt(0, len(p.businessOrder)-1)] {
			earlierNode := p.graphNodes[earlierID]
			if earlierNode.Type != "start" && earlierNode.Type != "common" && earlierNode.Type != "synergy" {
				continue
			}
			token := pathConfigToken("rollback-target", nodeID, earlierID)
			target.RollbackTargets[token] = earlierID
			result.RollbackTargets = append(result.RollbackTargets, model.PathConfigActionOption{Value: token, Label: earlierNode.Name})
		}
		if len(result.RollbackTargets) > 0 {
			appendAction("rollback_previous", "回退上一级", "回退到当前路径上更早且可达的业务节点", true, true, false)
		}
		if target.Person != nil {
			appendAction("add_sign", "加签", "在当前节点增加受模板候选范围约束的处理人", false, false, true)
			appendAction("transfer_approver", "移交", "把当前处理任务移交给受模板候选范围约束的处理人", false, false, true)
		}
	}
	if len(result.Catalog) == 0 {
		return result
	}
	plan, affected, note := p.projectStoredActionPlan(nodeID, nodeKind, target, result.Catalog)
	result.Arrivals, result.Affected, result.Note = plan, affected, note
	if affected {
		p.affected = true
	}
	p.validation.NodeTokens[PathConfigNodeToken(nodeID)] = target
	return result
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

// projectStoredActionPlan 兼容旧 agree/disagree，并按当前动作目录重新核验已保存到达计划。
func (p *pathConfigProjection) projectStoredActionPlan(nodeID, nodeKind string, target PathConfigNodeTarget, catalog []model.PathConfigActionCatalogItem) ([]model.PathConfigArrivalPlan, bool, string) {
	raw, exists := p.storedActions[PathConfigActionPlanStorageKey(nodeID)]
	if !exists {
		kind := "approve_pass"
		if nodeKind == "start" {
			kind = "submit"
		} else if legacy, hasLegacy := p.storedActions[nodeID]; hasLegacy {
			switch strings.TrimSpace(legacy) {
			case "agree":
				kind = "approve_pass"
			case "disagree":
				kind = "reject_no_pass"
			default:
				return []model.PathConfigArrivalPlan{{Visit: 1, Steps: []model.PathConfigActionStep{{Kind: kind, Label: pathConfigActionLabel(kind)}}}}, true, "旧节点动作不能准确映射，需要重新确认"
			}
		}
		return []model.PathConfigArrivalPlan{{Visit: 1, Steps: []model.PathConfigActionStep{{Kind: kind, Label: pathConfigActionLabel(kind)}}}}, false, ""
	}
	var stored storedPathConfigActionPlan
	if json.Unmarshal([]byte(raw), &stored) != nil || stored.Version != pathConfigActionPlanVersion {
		return []model.PathConfigArrivalPlan{}, true, "旧动作计划无法解析，需要重新确认"
	}
	projected, reason := projectStoredArrivals(target, stored.Arrivals)
	return projected, reason != "", reason
}

// projectStoredArrivals 把内部动作目标和人员转换成不透明公开值，并同时执行连续到达与动作上限校验。
func projectStoredArrivals(target PathConfigNodeTarget, arrivals []storedPathConfigArrival) ([]model.PathConfigArrivalPlan, string) {
	if len(arrivals) == 0 || len(arrivals) > maxPathConfigArrivals {
		return nil, "到达次数不符合 1 至 10 次限制"
	}
	result := make([]model.PathConfigArrivalPlan, 0, len(arrivals))
	total := 0
	for index, arrival := range arrivals {
		if arrival.Visit != index+1 || len(arrival.Steps) == 0 {
			return result, "到达序号必须连续且每次至少包含一个动作"
		}
		public := model.PathConfigArrivalPlan{Visit: arrival.Visit, Steps: []model.PathConfigActionStep{}}
		for stepIndex, step := range arrival.Steps {
			total++
			if total > maxPathConfigActionSteps || !target.ActionKinds[step.Kind] {
				return result, "动作计划超过上限或包含当前节点不允许的动作"
			}
			if pathConfigTerminalAction(step.Kind) && stepIndex != len(arrival.Steps)-1 {
				return result, "推进、回退或暂存动作必须位于本次到达的最后一步"
			}
			projected := model.PathConfigActionStep{Kind: step.Kind, Label: pathConfigActionLabel(step.Kind), Opinion: step.Opinion}
			if step.Kind == "rollback_previous" {
				for token, id := range target.RollbackTargets {
					if id == step.Target {
						projected.Target = token
					}
				}
				if projected.Target == "" {
					return result, "回退目标已不属于当前路径的更早业务节点"
				}
			}
			if step.Kind == "add_sign" || step.Kind == "transfer_approver" {
				if target.Person == nil || step.Person == nil {
					return result, "加签或移交缺少合法人员策略"
				}
				resolved, reason := resolveStoredPersonStrategy(target.Person, step.Person.Strategy, step.Person.Seed, step.Person.Selected)
				if reason != "" {
					return result, "加签或移交人员已失效：" + reason
				}
				projected.Person = &model.PathConfigPersonStrategyInput{Key: target.Person.Key, Strategy: step.Person.Strategy, Seed: step.Person.Seed, Selected: rawPersonIDsToTokens(resolved, invertTokenMap(target.Person.CandidateTokens))}
			}
			public.Steps = append(public.Steps, projected)
		}
		result = append(result, public)
	}
	return result, ""
}

// invertTokenMap 构造内部 ID 到公开 token 的反向映射，仅供投影已保存人员策略。
func invertTokenMap(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for token, id := range values {
		result[id] = token
	}
	return result
}

// EncodePathConfigActionPlan 校验浏览器有序动作、回退目标和嵌套人员策略并编码内部 JSON。
func EncodePathConfigActionPlan(target PathConfigNodeTarget, arrivals []model.PathConfigArrivalInput) (string, int, string) {
	if len(arrivals) == 0 || len(arrivals) > maxPathConfigArrivals {
		return "", 0, "到达次数不符合 1 至 10 次限制"
	}
	stored := storedPathConfigActionPlan{Version: pathConfigActionPlanVersion, Arrivals: make([]storedPathConfigArrival, 0, len(arrivals))}
	total := 0
	for index, arrival := range arrivals {
		if arrival.Visit != index+1 || len(arrival.Steps) == 0 {
			return "", total, "到达序号必须连续且每次至少包含一个动作"
		}
		storedArrival := storedPathConfigArrival{Visit: arrival.Visit, Steps: make([]storedPathConfigStep, 0, len(arrival.Steps))}
		for stepIndex, step := range arrival.Steps {
			total++
			kind := strings.TrimSpace(step.Kind)
			if total > maxPathConfigActionSteps || !target.ActionKinds[kind] {
				return "", total, "动作计划超过上限或包含当前节点不允许的动作"
			}
			if len([]rune(step.Opinion)) > 1000 {
				return "", total, "处理意见不能超过 1000 个字符"
			}
			if pathConfigTerminalAction(kind) && stepIndex != len(arrival.Steps)-1 {
				return "", total, "推进、回退或暂存动作必须位于本次到达的最后一步"
			}
			storedStep := storedPathConfigStep{Kind: kind, Opinion: strings.TrimSpace(step.Opinion)}
			if kind == "rollback_previous" {
				id, exists := target.RollbackTargets[strings.TrimSpace(step.Target)]
				if !exists {
					return "", total, "回退目标不属于当前路径的更早业务节点"
				}
				storedStep.Target = id
			}
			if kind == "add_sign" || kind == "transfer_approver" {
				if target.Person == nil || step.Person == nil {
					return "", total, "加签或移交缺少合法人员策略"
				}
				personRaw, reason := EncodePathConfigPersonStrategy(*target.Person, *step.Person)
				if reason != "" {
					return "", total, "加签或移交人员不合法：" + reason
				}
				var person storedPathConfigPersonPlan
				_ = json.Unmarshal([]byte(personRaw), &person)
				storedStep.Person = &person
			}
			storedArrival.Steps = append(storedArrival.Steps, storedStep)
		}
		if !pathConfigTerminalAction(storedArrival.Steps[len(storedArrival.Steps)-1].Kind) {
			return "", total, "每次到达必须以提交、处理结果、回退或暂存结束"
		}
		stored.Arrivals = append(stored.Arrivals, storedArrival)
	}
	encoded, err := json.Marshal(stored)
	if err != nil {
		return "", total, "动作计划无法保存"
	}
	return string(encoded), total, ""
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

// pathConfigActionPlanBlocksLine 仅把最终不同意视为当前路径后续不再需要配置；回退由显式后续到达表达。
func pathConfigActionPlanBlocksLine(plan model.PathConfigActionPlan) bool {
	if len(plan.Arrivals) == 0 {
		return false
	}
	lastArrival := plan.Arrivals[len(plan.Arrivals)-1]
	if len(lastArrival.Steps) == 0 {
		return false
	}
	return lastArrival.Steps[len(lastArrival.Steps)-1].Kind == "reject_no_pass"
}

// pathConfigTerminalAction 标识结束单次到达的动作，后续步骤不得排在其后。
func pathConfigTerminalAction(kind string) bool {
	switch kind {
	case "submit", "approve_pass", "reject_no_pass", "draft_save", "rollback_previous":
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
		return "加签"
	case "transfer_approver":
		return "移交"
	default:
		return fmt.Sprintf("未知动作（%s）", kind)
	}
}

// maxInt 返回两个整数中的较大值，用于安全截取已访问业务节点。
func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
