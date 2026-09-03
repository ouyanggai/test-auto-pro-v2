package analyzer

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/actioncatalog"
	"test-auto-pro-v2/internal/model"
)

const (
	maxPathConfigSafeSeed = int64(9007199254740991)
)

type storedPathConfigPersonPlan struct {
	Strategy string   `json:"strategy"`
	Seed     int64    `json:"seed"`
	Selected []string `json:"selected"`
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

// actionConfiguration 投影当前目标节点的可配置动作目录；每一项仅代表一次真实到达后的一个动作。
func (p *pathConfigProjection) actionConfiguration(nodeID, nodeName, nodeKind string, node *target.FlowNodeTemplate, persons []model.PathConfigPerson) model.PathConfigActionConfiguration {
	result := model.PathConfigActionConfiguration{Catalog: []model.PathConfigActionCatalogItem{}, Actions: []model.PathConfigConfiguredAction{}}
	scope := nodeActionScope(nodeKind)
	validationTarget := PathConfigNodeTarget{
		NodeID: nodeID, Name: nodeName, Scope: scope, Person: p.personTargets[nodeID],
		ActionPersons: map[string]*PathConfigPersonTarget{}, ActionKinds: map[string]bool{},
		Catalog: []model.ActionCatalogItem{},
	}
	for _, person := range persons {
		if person.Mode == "review" || person.Affected {
			validationTarget.Blockers = append(validationTarget.Blockers, model.PathConfigAffectedItem{Kind: "person", Name: person.Title, Reason: firstNonEmptyPathConfig(person.Note, person.Detail)})
		}
	}
	switch nodeKind {
	case "start":
		result.Base = &model.PathConfigActionBase{Kind: "submit", Label: "提交", Detail: "发起节点固定提交"}
	case "common", "synergy":
		result.Base = &model.PathConfigActionBase{Kind: "approve", Label: "同意", Detail: "系统默认同意"}
	}
	actionPersons := map[model.ActionKey]*model.PathConfigPerson{}
	if person, personTarget := addSignNodePersonConfig(nodeID, node); person != nil && personTarget != nil {
		actionPersons[model.ActionAddSign] = person
		validationTarget.ActionPersons[string(model.ActionAddSign)] = personTarget
	}
	ctx, previousReason := p.nodeActionContext(nodeID, nodeKind, node)
	result.Catalog = projectActionCatalog(ctx, scope, actionPersons, previousReason, &validationTarget)
	if len(validationTarget.ActionKinds) == 0 {
		return result
	}
	p.validation.NodeTokens[PathConfigNodeToken(nodeID)] = validationTarget
	return result
}

// instanceActionConfiguration 投影实例级动作容器；这些动作不绑定语义节点，由场景编译器排在节点动作之后。
func (p *pathConfigProjection) instanceActionConfiguration() model.PathConfigActionConfiguration {
	result := model.PathConfigActionConfiguration{Catalog: []model.PathConfigActionCatalogItem{}, Actions: []model.PathConfigConfiguredAction{}}
	validationTarget := PathConfigNodeTarget{
		Name: "实例动作", Scope: model.ActionScopeInstance,
		ActionPersons: map[string]*PathConfigPersonTarget{}, ActionKinds: map[string]bool{},
		Catalog: []model.ActionCatalogItem{},
	}
	result.Catalog = projectActionCatalog(instanceActionContext(), model.ActionScopeInstance, nil, "", &validationTarget)
	if len(validationTarget.ActionKinds) == 0 {
		return result
	}
	result.Note = "实例动作作用于同一主流程实例，不绑定语义节点。"
	p.validation.NodeTokens[PathConfigInstanceActionKey()] = validationTarget
	return result
}

// projectActionCatalog 使用动作目录门禁服务投影当前上下文可编排的动作，并登记保存时复用的门禁事实。
func projectActionCatalog(
	ctx model.ActionContext,
	scope model.ActionScope,
	persons map[model.ActionKey]*model.PathConfigPerson,
	previousReason string,
	validationTarget *PathConfigNodeTarget,
) []model.PathConfigActionCatalogItem {
	items := actioncatalog.Build(ctx)
	result := make([]model.PathConfigActionCatalogItem, 0, len(items))
	for _, item := range items {
		if item.SystemOnly {
			// 系统自动语义只在真实系统节点上只读展示，不进入可保存目录。
			if item.Enabled {
				result = append(result, projectedCatalogItem(item, nil, systemActionRuntimeNote()))
			}
			continue
		}
		if !catalogScopeAllowed(scope, item.Scope) {
			continue
		}
		item = resolveOrderDependentGate(item, ctx)
		person := persons[item.Action]
		result = append(result, projectedCatalogItem(item, person, actionRuntimeNote(item.Action, previousReason)))
		validationTarget.Catalog = append(validationTarget.Catalog, item)
		if item.Enabled {
			validationTarget.ActionKinds[string(item.Action)] = true
		}
	}
	return result
}

// nodeActionScope 按真实节点类型确定该配置位置可编排的动作作用域。
func nodeActionScope(nodeKind string) model.ActionScope {
	switch strings.TrimSpace(nodeKind) {
	case "start":
		return model.ActionScopeInitiator
	case "common", "synergy":
		return model.ActionScopeTask
	default:
		return ""
	}
}

// catalogScopeAllowed 判断动作作用域是否属于当前配置位置；已办恢复与当前待办共用同一审批节点。
func catalogScopeAllowed(context, item model.ActionScope) bool {
	switch context {
	case model.ActionScopeInitiator:
		return item == model.ActionScopeInitiator
	case model.ActionScopeTask:
		return item == model.ActionScopeTask || item == model.ActionScopeCompletedTask
	case model.ActionScopeInstance:
		return item == model.ActionScopeInstance
	default:
		return false
	}
}

// nodeActionContext 投影“按当前路径真实到达该节点时”的目标上下文；未证明的事实一律保持 false。
func (p *pathConfigProjection) nodeActionContext(nodeID, nodeKind string, node *target.FlowNodeTemplate) (model.ActionContext, string) {
	nodeKey, kind := PathConfigNodeToken(nodeID), strings.TrimSpace(nodeKind)
	switch kind {
	case "start":
		return model.ActionContext{FlowSource: "new", CurrentNodeKey: nodeKey, CurrentNodeType: kind, IsInitiator: true}, ""
	case "common", "synergy":
		ctx := model.ActionContext{
			FlowSource: "pending", InstanceStatus: "run", CurrentNodeKey: nodeKey, CurrentNodeType: kind,
			HasCurrentTask: true, InstanceVisible: true, HasCompletedTask: true,
			HasEditableProxy: node != nil && len(node.AddSignIssues) == 0 && len(node.AddSignCandidates) > 0,
			CanSwitchActor:   nodeActorSwitchable(node),
		}
		previousID, previousReason := p.singlePreviousBusinessNode(nodeID)
		if previousID != "" {
			previousType := strings.TrimSpace(p.graphNodes[previousID].Type)
			ctx.PreviousTaskExists, ctx.PreviousNodeType, ctx.PreviousNodeIsStart = true, previousType, previousType == "start"
		}
		return ctx, previousReason
	default:
		return model.ActionContext{CurrentNodeKey: nodeKey, CurrentNodeType: kind}, ""
	}
}

// instanceActionContext 投影实例级动作容器上下文：主实例已发起、运行中且由当前账号可见。
func instanceActionContext() model.ActionContext {
	return model.ActionContext{
		FlowSource: "pending", InstanceStatus: "run", IsInitiator: true,
		InstanceVisible: true, HasPendingRecipient: true,
	}
}

// nodeActorSwitchable 只在目标节点人员目录已完整解析且存在候选时认为可以切换演员。
func nodeActorSwitchable(node *target.FlowNodeTemplate) bool {
	if node == nil || node.AuditConfig == nil {
		return false
	}
	return len(node.AuditConfig.Candidates) > 0 && len(node.AuditConfig.ResolutionIssues) == 0
}

// resolveOrderDependentGate 用“前置动作已执行”的等价上下文重算顺序依赖动作，真实顺序由场景编译器强制。
func resolveOrderDependentGate(item model.ActionCatalogItem, ctx model.ActionContext) model.ActionCatalogItem {
	if item.Enabled {
		return item
	}
	variant := ctx
	switch item.Action {
	case model.ActionResubmit:
		variant.InstanceStatus = "draft"
	case model.ActionUnfollow:
		variant.Followed = true
	default:
		return item
	}
	for _, candidate := range actioncatalog.Build(variant) {
		if candidate.Action == item.Action {
			return candidate
		}
	}
	return item
}

// actionRuntimeNote 说明动作的顺序前置条件与运行时重读要求，页面不能只显示可用性。
func actionRuntimeNote(action model.ActionKey, previousReason string) string {
	switch action {
	case model.ActionResubmit:
		return "重新提交必须排在保存草稿、不同意或撤回之后，否则保存时会被阻止。"
	case model.ActionUnfollow:
		return "取消关注必须排在关注之后，否则保存时会被阻止。"
	case model.ActionRetrieve:
		return "取回需要当前节点已有已办任务；缺少已办事实时编译器会插入一次准备同意。"
	case model.ActionTransfer:
		return "移交演员只能来自目标运行时实时受限候选，保存时只记录演员策略。"
	case model.ActionAddSign:
		return "加签必要时会创建实例私有代理并重映射任务，后续步骤必须重读代理与待办。"
	case model.ActionRollback:
		if strings.TrimSpace(previousReason) != "" {
			return strings.TrimSpace(previousReason)
		}
		return "回退只会回到目标引擎的真实直接前一待办，不能指定其他节点。"
	case model.ActionForward:
		return "转发创建独立辅助流程，主实例游标不移动。"
	default:
		return ""
	}
}

// systemActionRuntimeNote 说明系统自动语义只用于只读核对。
func systemActionRuntimeNote() string {
	return "系统自动节点由目标引擎执行，工具侧只读核对，不能编排。"
}

// projectedCatalogItem 把动作门禁结果投影为配置项，不携带目标实例、任务或代理标识。
func projectedCatalogItem(item model.ActionCatalogItem, person *model.PathConfigPerson, note string) model.PathConfigActionCatalogItem {
	return model.PathConfigActionCatalogItem{
		Kind: string(item.Action), Category: string(item.Category), Scope: string(item.Scope),
		Label: item.Label, Description: item.Description, Enabled: item.Enabled, DisabledReason: item.DisabledReason,
		RequiresPerson: person != nil, Person: person, TargetOperation: item.TargetOperation,
		Parameters: item.Parameters, ParameterDetails: item.ParameterDetails, Preconditions: item.Preconditions,
		ExpectedEffect: item.ExpectedEffect, RequiresReload: item.RequiresReload, ReloadRequirements: item.ReloadRequirements,
		SystemOnly: item.SystemOnly, SystemNodeType: item.SystemNodeType, RuntimeNote: note,
	}
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
	strategies, allowed := []model.PathConfigPersonStrategyOption{{Value: "manual", Label: "手动选择"}, {Value: "random", Label: "范围随机"}}, map[string]bool{"manual": true, "random": true}
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

// singlePreviousBusinessNode 只接受当前已选路径上的唯一真实业务前驱；发起节点也返回，由动作门禁决定能否回退。
func (p *pathConfigProjection) singlePreviousBusinessNode(nodeID string) (string, string) {
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
