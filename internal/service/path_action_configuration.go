package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/engine/scenario"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// PathActionConfigurationService 提供 F-012 有序动作保存和只读场景预览。
type PathActionConfigurationService interface {
	GetActionConfiguration(context.Context, uint64, uint64) (model.ActionConfigurationResult, error)
	GetCompiledScenario(context.Context, uint64, uint64) (model.ActionConfigurationResult, error)
	SaveActionConfiguration(context.Context, uint64, uint64, string, string, model.ActionConfigurationInput) (model.ActionConfigurationResult, error)
}

// GetActionConfiguration 重读当前目标路径并返回已保存动作及其编译场景。
func (s *PathConfigService) GetActionConfiguration(ctx context.Context, planID, pathID uint64) (model.ActionConfigurationResult, error) {
	path, snapshot, analysis, stored, found, _, err := s.loadWorkspace(ctx, planID, pathID)
	if err != nil {
		return model.ActionConfigurationResult{}, err
	}
	actions := []model.ConfiguredAction{}
	if found {
		actions = decodeWorkspaceActions(stored.UserActions)
	}
	validation, err := s.pathActionGates(snapshot, path, analysis, found)
	if err != nil {
		return model.ActionConfigurationResult{}, err
	}
	compiled, compileErr := compilePathActions(actions, analysis.graph, analysis.pathAnalysis, actionCatalogGates(validation))
	result := actionConfigurationResult(path, stored, compiled)
	if compileErr != nil {
		result.Issues = compileIssues(compileErr)
	}
	return result, nil
}

// GetCompiledScenario 返回当前路径的有序用户动作和系统插入步骤只读预览。
func (s *PathConfigService) GetCompiledScenario(ctx context.Context, planID, pathID uint64) (model.ActionConfigurationResult, error) {
	return s.GetActionConfiguration(ctx, planID, pathID)
}

// SaveActionConfiguration 保存当前语义节点动作并在同一配置修订内编译完整主实例场景。
func (s *PathConfigService) SaveActionConfiguration(ctx context.Context, planID, pathID uint64, nodeKey, idempotencyKey string, input model.ActionConfigurationInput) (model.ActionConfigurationResult, error) {
	if planID == 0 || pathID == 0 || strings.TrimSpace(nodeKey) == "" || !validUUID(strings.TrimSpace(idempotencyKey)) {
		return model.ActionConfigurationResult{}, &PathConfigError{Kind: PathConfigErrorInvalidArgument, Message: "动作配置参数不正确"}
	}
	if s.historyConfigStore == nil {
		return model.ActionConfigurationResult{}, &PathConfigError{Kind: PathConfigErrorStorage, Message: "动作配置存储暂不可用"}
	}
	path, snapshot, analysis, current, found, _, err := s.loadWorkspace(ctx, planID, pathID)
	if err != nil {
		return model.ActionConfigurationResult{}, err
	}
	// 相同幂等键允许客户端带着原始修订重试；其他旧修订仍须在编译前直接冲突，避免返回误导性的动作问题。
	sameIdempotency := found && strings.TrimSpace(idempotencyKey) != "" && current.IdempotencyKey == strings.TrimSpace(idempotencyKey)
	if input.Revision != current.Revision && !sameIdempotency {
		return model.ActionConfigurationResult{}, &PathConfigError{Kind: PathConfigErrorRevisionConflict, Message: "动作配置已被其他操作更新，请刷新后重试"}
	}
	existing := []model.ConfiguredAction{}
	if found {
		existing = decodeWorkspaceActions(current.UserActions)
	}
	validation, err := s.pathActionGates(snapshot, path, analysis, found)
	if err != nil {
		return model.ActionConfigurationResult{}, err
	}
	if err := validateActionPersons(validation, nodeKey, input.Persons, input.Actions); err != nil {
		return model.ActionConfigurationResult{}, err
	}
	personStrategies := decodeHistoryPersonStrategies(current.PersonStrategies)
	for _, person := range input.Persons {
		person.Key = strings.TrimSpace(person.Key)
		person.Selected = append([]string(nil), person.Selected...)
		personStrategies[person.Key] = person
	}
	actions, mergeErr := mergeNodeActions(existing, input.Actions, nodeKey, analysis.graph, analysis.pathAnalysis)
	if mergeErr != nil {
		return model.ActionConfigurationResult{}, mergeErr
	}
	compiled, compileErr := compilePathActions(actions, analysis.graph, analysis.pathAnalysis, actionCatalogGates(validation))
	if compileErr != nil {
		result := actionConfigurationResult(path, current, compiled)
		result.Issues = compileIssues(compileErr)
		return result, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "动作顺序无法恢复，请修正首个阻断动作", Affected: actionConfigurationAffected(compileErr)}
	}
	if sameIdempotency {
		// 幂等重试只复用正文完全相同的已保存结果；同键提交不同动作不能返回旧结果或产生第二次修订。
		storedResult := scenario.Result{Actions: decodeWorkspaceActions(current.UserActions), Steps: decodeWorkspaceSteps(current.CompiledSteps), Issues: decodeActionConfigurationIssues(current.Issues)}
		if sameConfiguredActions(storedResult.Actions, compiled.Actions) && sameCompiledSteps(storedResult.Steps, compiled.Steps) && samePersonStrategies(current.PersonStrategies, personStrategies) {
			return actionConfigurationResult(path, current, storedResult), nil
		}
		return model.ActionConfigurationResult{}, &PathConfigError{Kind: PathConfigErrorRevisionConflict, Message: "相同幂等键不能复用不同动作配置"}
	}
	// 每次整体场景重编译都把动作记录绑定到同一个动作配置修订，避免单条记录落在旧修订上。
	nextActionRevision := current.ActionRevision + 1
	for index := range compiled.Actions {
		compiled.Actions[index].Revision = nextActionRevision
	}
	actionJSON, err := json.Marshal(compiled.Actions)
	if err != nil {
		return model.ActionConfigurationResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "动作配置无法编码"}
	}
	stepJSON, err := json.Marshal(compiled.Steps)
	if err != nil {
		return model.ActionConfigurationResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "动作场景无法编码"}
	}
	issueJSON, err := mergeActionConfigurationIssues(current.Issues, compiled.Issues)
	if err != nil {
		return model.ActionConfigurationResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "动作问题无法编码"}
	}
	latestJSON, err := json.Marshal(map[string]any{"idempotencyKey": strings.TrimSpace(idempotencyKey), "actionRevision": current.ActionRevision + 1})
	if err != nil {
		return model.ActionConfigurationResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "幂等结果无法编码"}
	}
	record := current
	record.PathID = pathID
	record.IdempotencyKey = strings.TrimSpace(idempotencyKey)
	record.UserActions = actionJSON
	record.CompiledSteps = stepJSON
	record.Issues = issueJSON
	record.LatestIdempotency = latestJSON
	record.ActionRevision = current.ActionRevision + 1
	record.NodeRevision = current.NodeRevision + 1
	record.Revision = current.Revision + 1
	record.ConfigStatus = actionConfigStatus(len(compiled.Actions))
	record.NodeStatus = record.ConfigStatus
	if strings.TrimSpace(record.SourceMode) == "" {
		record.SourceMode = model.HistorySourceModeNone
	}
	if strings.TrimSpace(record.RuntimeType) == "" {
		record.RuntimeType = string(target.FormRenderTypeUnknown)
	}
	if strings.TrimSpace(record.DataStatus) == "" {
		record.DataStatus = model.HistoryDataStatusEmpty
	}
	if len(record.PersonStrategies) == 0 {
		record.PersonStrategies, err = json.Marshal(personStrategies)
		if err != nil {
			return model.ActionConfigurationResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "人员策略无法编码"}
		}
	} else if len(input.Persons) > 0 {
		record.PersonStrategies, err = json.Marshal(personStrategies)
		if err != nil {
			return model.ActionConfigurationResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "人员策略无法编码"}
		}
	}
	record.ConfirmedNodeKeys, err = confirmedNodeKeysJSON(current.ConfirmedNodeKeys, compiled.Actions, nodeKey)
	if err != nil {
		return model.ActionConfigurationResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "已确认节点无法编码"}
	}
	if len(record.EffectiveFormData) == 0 {
		record.EffectiveFormData = []byte(`{}`)
	}
	if len(record.BranchPatches) == 0 {
		record.BranchPatches = []byte(`[]`)
	}
	if len(record.RuntimeValidation) == 0 {
		record.RuntimeValidation = []byte(`{}`)
	}
	saved, err := s.historyConfigStore.SavePathConfig(ctx, record, input.Revision, s.now().UTC())
	if err != nil {
		return model.ActionConfigurationResult{}, mapHistoryWorkspaceStoreError(err)
	}
	return actionConfigurationResult(path, saved, compiled), nil
}

// pathActionGates 重读当前真实路径的动作门禁与人员目录投影，保存和预览共用同一份事实。
func (s *PathConfigService) pathActionGates(snapshot target.PathConfigurationSnapshot, path model.ExecutionPath, analysis ownedPathAnalysis, found bool) (analyzer.PathConfigValidation, error) {
	if s.configAnalyzer == nil {
		return analyzer.PathConfigValidation{}, &PathConfigError{Kind: PathConfigErrorStorage, Message: "动作门禁校验服务暂不可用"}
	}
	_, validation, err := s.configAnalyzer.Analyze(analysis.graph, snapshot.Tree, snapshot.FormFields, path, analysis.pathAnalysis, snapshot.InstanceValues, map[string]map[string]string{}, map[string]string{}, found)
	if err != nil {
		return analyzer.PathConfigValidation{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前路径动作门禁无法核对"}
	}
	return validation, nil
}

// actionCatalogGates 汇总全部配置位置的动作门禁项，按稳定键排序供场景编译器逐条复验。
func actionCatalogGates(validation analyzer.PathConfigValidation) []model.ActionCatalogItem {
	keys := make([]string, 0, len(validation.NodeTokens))
	for key := range validation.NodeTokens {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	items := []model.ActionCatalogItem{}
	for _, key := range keys {
		items = append(items, validation.NodeTokens[key].Catalog...)
	}
	return items
}

// validateActionPersons 按当前真实节点的人员候选和人数边界复验动作保存附带的人员策略。
func validateActionPersons(validation analyzer.PathConfigValidation, nodeKey string, persons []model.PathConfigPersonStrategyInput, actions []model.ConfiguredAction) error {
	if len(persons) == 0 && !actionNeedsPerson(actions) {
		return nil
	}
	target, ok := validation.NodeTokens[strings.TrimSpace(nodeKey)]
	if !ok {
		return &PathConfigError{Kind: PathConfigErrorInvalidArgument, Message: "当前节点没有可保存的人员策略"}
	}
	for index := range persons {
		persons[index].Key = strings.TrimSpace(persons[index].Key)
		if persons[index].Key == "" {
			return &PathConfigError{Kind: PathConfigErrorInvalid, Message: "人员策略缺少稳定键", Affected: []model.PathConfigAffectedItem{{Kind: "person", Name: "人员策略", Reason: "候选键不能为空"}}}
		}
	}
	byKey := make(map[string]model.PathConfigPersonStrategyInput, len(persons))
	for _, person := range persons {
		if _, exists := byKey[person.Key]; exists {
			return &PathConfigError{Kind: PathConfigErrorInvalid, Message: "人员策略不能重复提交", Affected: []model.PathConfigAffectedItem{{Kind: "person", Name: person.Key, Reason: "稳定键重复"}}}
		}
		byKey[person.Key] = person
	}
	if target.Person != nil {
		person, ok := byKey[target.Person.Key]
		if !ok {
			return &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前节点人员策略不完整", Affected: []model.PathConfigAffectedItem{{Kind: "person", Name: target.Person.Name, Reason: "缺少节点处理人员策略"}}}
		}
		if _, reason := analyzer.EncodePathConfigPersonStrategy(*target.Person, person); reason != "" {
			return &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前节点人员策略不合法", Affected: []model.PathConfigAffectedItem{{Kind: "person", Name: target.Person.Name, Reason: reason}}}
		}
		delete(byKey, target.Person.Key)
	}
	for _, action := range actions {
		if action.Action != model.ActionAddSign {
			continue
		}
		personTarget := target.ActionPersons[string(model.ActionAddSign)]
		if personTarget == nil {
			return &PathConfigError{Kind: PathConfigErrorInvalid, Message: "加签动作缺少当前候选目录", Affected: []model.PathConfigAffectedItem{{Kind: "person", Name: "加签处理人", Reason: "目标未返回可用候选"}}}
		}
		person, ok := byKey[personTarget.Key]
		if !ok {
			return &PathConfigError{Kind: PathConfigErrorInvalid, Message: "加签动作人员策略不完整", Affected: []model.PathConfigAffectedItem{{Kind: "person", Name: personTarget.Name, Reason: "动作需要明确候选人员"}}}
		}
		if _, reason := analyzer.EncodePathConfigPersonStrategy(*personTarget, person); reason != "" {
			return &PathConfigError{Kind: PathConfigErrorInvalid, Message: "动作人员策略不合法", Affected: []model.PathConfigAffectedItem{{Kind: "person", Name: personTarget.Name, Reason: reason}}}
		}
	}
	for key, person := range byKey {
		var actionTarget *analyzer.PathConfigPersonTarget
		for _, candidate := range target.ActionPersons {
			if candidate != nil && candidate.Key == key {
				actionTarget = candidate
				break
			}
		}
		if actionTarget == nil {
			return &PathConfigError{Kind: PathConfigErrorInvalid, Message: "人员策略不属于当前节点", Affected: []model.PathConfigAffectedItem{{Kind: "person", Name: key, Reason: "当前节点没有对应的人员候选目录"}}}
		}
		if _, reason := analyzer.EncodePathConfigPersonStrategy(*actionTarget, person); reason != "" {
			return &PathConfigError{Kind: PathConfigErrorInvalid, Message: "动作人员策略不合法", Affected: []model.PathConfigAffectedItem{{Kind: "person", Name: actionTarget.Name, Reason: reason}}}
		}
	}
	return nil
}

// actionNeedsPerson 判断动作保存是否必须读取动作私有人员目录。
func actionNeedsPerson(actions []model.ConfiguredAction) bool {
	for _, action := range actions {
		if action.Action == model.ActionAddSign {
			return true
		}
	}
	return false
}

// decodeHistoryPersonStrategies 读取独立人员列的稳定键对象，损坏正文返回空集合并交由仓储校验阻断。
func decodeHistoryPersonStrategies(raw []byte) map[string]model.PathConfigPersonStrategyInput {
	result := make(map[string]model.PathConfigPersonStrategyInput)
	if len(raw) == 0 {
		return result
	}
	var object map[string]model.PathConfigPersonStrategyInput
	if json.Unmarshal(raw, &object) == nil && object != nil {
		for key, value := range object {
			if strings.TrimSpace(key) != "" {
				result[key] = value
			}
		}
		return result
	}
	return result
}

// applyHistoryActionProjection 把 F-012 独立修订投影回节点工作台，保证保存后刷新不会丢失人员和动作草稿。
func (s *PathConfigService) applyHistoryActionProjection(ctx context.Context, pathID uint64, configuration *model.PathConfiguration) error {
	if s.historyConfigStore == nil || configuration == nil {
		return nil
	}
	record, found, err := s.historyConfigStore.GetPathConfig(ctx, pathID)
	if err != nil {
		return mapHistoryWorkspaceStoreError(err)
	}
	if !found {
		return nil
	}
	configuration.Revision, configuration.NodeRevision = record.Revision, record.NodeRevision
	actions := decodeWorkspaceActions(record.UserActions)
	persons := decodeHistoryPersonStrategies(record.PersonStrategies)
	byNode := make(map[string]*model.PathConfigNode)
	for groupIndex := range configuration.Groups {
		for nodeIndex := range configuration.Groups[groupIndex].Nodes {
			node := &configuration.Groups[groupIndex].Nodes[nodeIndex]
			byNode[node.Key] = node
			for personIndex := range node.Persons {
				person := &node.Persons[personIndex]
				strategy, ok := persons[person.Key]
				if !ok {
					continue
				}
				person.Strategy, person.StrategySeed = strategy.Strategy, strategy.Seed
				person.Selected = projectedPersonSelection(*person, strategy)
			}
			node.ActionConfiguration.Actions = []model.PathConfigConfiguredAction{}
		}
	}
	configuration.InstanceActions.Actions = []model.PathConfigConfiguredAction{}
	for _, action := range actions {
		key := strings.TrimSpace(action.NodeKey)
		if key == "" {
			// 实例动作作用于同一主实例，投影回独立容器而不是伪造节点归属。
			configuration.InstanceActions.Actions = append(configuration.InstanceActions.Actions, model.PathConfigConfiguredAction{
				Key: action.Key, Kind: actionDisplayKind(action.Action), Label: actionDisplayLabel(action.Action),
				Parameters: cloneActionParameterMap(action.Parameters), ActorPolicy: action.ActorPolicy, Note: action.Note,
			})
			continue
		}
		node := byNode[key]
		if node == nil {
			// 节点已不在当前路径上时不伪造归属，由编译场景端点完整返回。
			continue
		}
		node.ActionConfiguration.Actions = append(node.ActionConfiguration.Actions, model.PathConfigConfiguredAction{
			Key: action.Key, Kind: actionDisplayKind(action.Action), Label: actionDisplayLabel(action.Action),
			Person: projectActionPerson(node, action, persons), Parameters: cloneActionParameterMap(action.Parameters), ActorPolicy: action.ActorPolicy, Note: action.Note,
		})
	}
	return nil
}

// projectActionPerson 从动作目录当前候选和独立人员策略中恢复动作私有人员，避免刷新后加签或移交配置退回默认值。
func projectActionPerson(node *model.PathConfigNode, action model.ConfiguredAction, persons map[string]model.PathConfigPersonStrategyInput) *model.PathConfigPersonStrategyInput {
	if node == nil {
		return nil
	}
	kind := actionDisplayKind(action.Action)
	var source *model.PathConfigPerson
	for index := range node.ActionConfiguration.Catalog {
		item := &node.ActionConfiguration.Catalog[index]
		if item.Kind == kind && item.Person != nil {
			source = item.Person
			break
		}
	}
	if source == nil {
		return nil
	}
	result := &model.PathConfigPersonStrategyInput{Key: source.Key, Strategy: source.Strategy, Seed: source.StrategySeed, Selected: append([]string{}, source.Selected...)}
	if saved, ok := persons[source.Key]; ok {
		result = &model.PathConfigPersonStrategyInput{Key: saved.Key, Strategy: saved.Strategy, Seed: saved.Seed, Selected: projectedPersonSelection(*source, saved)}
	}
	if strings.TrimSpace(action.ActorPolicy) != "" {
		result.Strategy = strings.TrimSpace(action.ActorPolicy)
	}
	if selected := actionPersonSelected(action.Parameters); selected != nil {
		result.Selected = selected
	}
	return result
}

// projectedPersonSelection 按当前候选和已保存策略恢复公开选中项，并保证空集合编码为 JSON 数组。
func projectedPersonSelection(person model.PathConfigPerson, strategy model.PathConfigPersonStrategyInput) []string {
	switch strings.TrimSpace(strategy.Strategy) {
	case "target_default":
		return append([]string{}, person.DefaultSelected...)
	case "all":
		selected := make([]string, 0, len(person.Options))
		for _, option := range person.Options {
			selected = append(selected, option.Value)
		}
		return selected
	case "random":
		if len(person.Options) == 0 {
			return []string{}
		}
		count := person.MinCount
		if count < 1 {
			count = 1
		}
		if person.MaxCount > 0 && count > person.MaxCount {
			count = person.MaxCount
		}
		if count > len(person.Options) {
			count = len(person.Options)
		}
		seed := strategy.Seed
		if seed < 1 {
			seed = 1
		}
		start := int(uint64(seed) % uint64(len(person.Options)))
		selected := make([]string, 0, count)
		for index := 0; index < count; index++ {
			selected = append(selected, person.Options[(start+index)%len(person.Options)].Value)
		}
		return selected
	default:
		return append([]string{}, strategy.Selected...)
	}
}

// actionPersonSelected 读取目标加签/移交接口的候选键集合，不把参数正文转换为新的数据模型。
func actionPersonSelected(parameters map[string]any) []string {
	if parameters == nil {
		return nil
	}
	raw, ok := parameters["approverAppendVo.userIds"]
	if !ok {
		return nil
	}
	values := []string{}
	switch list := raw.(type) {
	case []string:
		values = append(values, list...)
	case []any:
		for _, item := range list {
			if value, ok := item.(string); ok && strings.TrimSpace(value) != "" {
				values = append(values, value)
			}
		}
	}
	return values
}

// cloneActionParameterMap 复制动作参数顶层键，防止节点投影与已保存场景共享可变映射。
func cloneActionParameterMap(parameters map[string]any) map[string]any {
	if parameters == nil {
		return nil
	}
	result := make(map[string]any, len(parameters))
	for key, value := range parameters {
		result[key] = value
	}
	return result
}

// actionDisplayKind 将稳定动作键投影为当前节点工作台识别的动作语义，不改变持久化正文。
func actionDisplayKind(action model.ActionKey) string {
	switch action {
	case model.ActionStorageFormData:
		return "storage_form_data"
	default:
		return string(action)
	}
}

// actionDisplayLabel 返回动作在节点工作台中的中文标签；未知稳定键保留原值供定位。
func actionDisplayLabel(action model.ActionKey) string {
	switch action {
	case model.ActionStorageFormData:
		return "暂存当前表单"
	case model.ActionReject:
		return "不同意"
	case model.ActionRollback:
		return "回退上一节点"
	case model.ActionAddSign:
		return "加签"
	default:
		return string(action)
	}
}

// mergeNodeActions 将当前节点动作替换为本次独立记录并保留其他节点动作。
func mergeNodeActions(existing, submitted []model.ConfiguredAction, nodeKey string, graph model.FlowGraph, analysis model.ExecutionPathAnalysis) ([]model.ConfiguredAction, error) {
	nodeKey = strings.TrimSpace(nodeKey)
	// 实例动作容器使用独立不透明键保存，落盘时统一为空节点键，避免把实例动作绑定到语义节点。
	instanceContainer := nodeKey == analyzer.PathConfigInstanceActionKey()
	storedKey := nodeKey
	if instanceContainer {
		storedKey = ""
	}
	merged := make([]model.ConfiguredAction, 0, len(existing)+len(submitted))
	submittedOrders := make(map[int]string, len(submitted))
	for _, action := range existing {
		if strings.TrimSpace(action.NodeKey) == storedKey {
			continue
		}
		merged = append(merged, cloneConfiguredAction(action))
	}
	for _, action := range submitted {
		action = cloneConfiguredAction(action)
		if strings.TrimSpace(action.Key) == "" {
			return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "动作记录必须携带稳定键", Affected: []model.PathConfigAffectedItem{{Kind: "action", Name: "动作", Reason: "浏览器不能让服务端猜测或生成动作身份"}}}
		}
		if action.Order > 0 {
			if previousKey, exists := submittedOrders[action.Order]; exists {
				return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前节点动作顺序不能重复", Affected: []model.PathConfigAffectedItem{{Kind: "action", Name: action.Key, Reason: "顺序与动作 " + previousKey + " 重复"}}}
			}
			submittedOrders[action.Order] = action.Key
		}
		if action.Scope == model.ActionScopeInstance {
			if !instanceContainer {
				return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "实例动作只能在实例动作容器保存", Affected: []model.PathConfigAffectedItem{{Kind: "action", Name: action.Key, Reason: "实例动作不属于当前语义节点"}}}
			}
			if key := strings.TrimSpace(action.NodeKey); key != "" && key != nodeKey {
				return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "实例动作不能绑定节点", Affected: []model.PathConfigAffectedItem{{Kind: "action", Name: action.Key, Reason: "实例动作必须使用空节点键"}}}
			}
			action.NodeKey = ""
		} else {
			if instanceContainer {
				return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "实例动作容器只能保存实例作用域动作", Affected: []model.PathConfigAffectedItem{{Kind: "action", Name: action.Key, Reason: "节点动作必须在对应语义节点保存"}}}
			}
			if strings.TrimSpace(action.NodeKey) == "" {
				action.NodeKey = nodeKey
			}
			if action.NodeKey != nodeKey {
				return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "一次只能保存当前语义节点的动作", Affected: []model.PathConfigAffectedItem{{Kind: "action", Name: action.Key, Reason: "动作节点键与当前保存节点不一致"}}}
			}
		}
		merged = append(merged, action)
	}
	// 路径节点顺序是主实例恢复的唯一排序依据；同一节点内继续使用用户 order。
	positions := semanticNodePositions(graph, analysis)
	sort.SliceStable(merged, func(i, j int) bool {
		left, right := positions[strings.TrimSpace(merged[i].NodeKey)], positions[strings.TrimSpace(merged[j].NodeKey)]
		if left != right {
			return left < right
		}
		if merged[i].Order != merged[j].Order {
			return merged[i].Order < merged[j].Order
		}
		return merged[i].Key < merged[j].Key
	})
	for index := range merged {
		merged[index].Order = index + 1
	}
	return merged, nil
}

// compilePathActions 将真实图节点转换为不透明语义键并调用纯场景编译器，同时带上当前动作门禁。
func compilePathActions(actions []model.ConfiguredAction, graph model.FlowGraph, analysis model.ExecutionPathAnalysis, catalog []model.ActionCatalogItem) (scenario.Result, error) {
	nodes, sequence := semanticScenarioNodes(graph, analysis)
	return scenario.Compile(scenario.Input{Actions: actions, Nodes: nodes, NodeSequence: sequence, FinalNodeKey: lastString(sequence), Catalog: catalog})
}

// semanticScenarioNodes 为场景编译器建立目标节点到工具语义键的内部索引。
func semanticScenarioNodes(graph model.FlowGraph, analysis model.ExecutionPathAnalysis) ([]model.FlowGraphNode, []string) {
	byID := make(map[string]model.FlowGraphNode, len(graph.Nodes))
	for _, node := range graph.Nodes {
		byID[node.ID] = node
	}
	ids := analysis.ReachableNodeIDs
	if len(ids) == 0 {
		for _, node := range graph.Nodes {
			ids = append(ids, node.ID)
		}
	}
	result := make([]model.FlowGraphNode, 0, len(ids))
	sequence := make([]string, 0, len(ids))
	for _, id := range ids {
		node, ok := byID[id]
		if !ok {
			continue
		}
		key := analyzer.PathConfigNodeToken(node.ID)
		node.ID = key
		result = append(result, node)
		sequence = append(sequence, key)
	}
	return result, sequence
}

// semanticNodePositions 返回当前路径语义节点顺序，实例动作放在路径末端保持稳定。
func semanticNodePositions(graph model.FlowGraph, analysis model.ExecutionPathAnalysis) map[string]int {
	_, sequence := semanticScenarioNodes(graph, analysis)
	result := make(map[string]int, len(sequence)+1)
	for index, key := range sequence {
		result[key] = index
	}
	result[""] = len(sequence) + 1
	return result
}

// actionConfigurationResult 只投影动作领域修订和只读步骤，不返回目标内部数据。
func actionConfigurationResult(path model.ExecutionPath, stored repository.HistoryPathConfigRecord, compiled scenario.Result) model.ActionConfigurationResult {
	status := strings.TrimSpace(stored.ConfigStatus)
	if status == "" {
		status = actionConfigStatus(len(compiled.Actions))
	}
	return model.ActionConfigurationResult{
		Path: model.PathConfigPath{SequenceNo: path.SequenceNo, Name: path.Name}, Revision: stored.Revision,
		NodeRevision: stored.NodeRevision, ActionRevision: stored.ActionRevision, Status: status,
		Actions: append([]model.ConfiguredAction(nil), compiled.Actions...), CompiledScenario: append([]model.CompiledActionStep(nil), compiled.Steps...),
		Issues: append([]model.ActionConfigurationIssue(nil), compiled.Issues...),
	}
}

// actionConfigStatus 从动作编译成功与否派生路径节点配置状态。
func actionConfigStatus(count int) string {
	if count == 0 {
		return "pending"
	}
	return "configured"
}

// compileIssues 从纯编译错误中复制结构化首个阻断列表。
func compileIssues(err error) []model.ActionConfigurationIssue {
	if compileErr, ok := err.(*scenario.CompileError); ok {
		return append([]model.ActionConfigurationIssue(nil), compileErr.Issues...)
	}
	return []model.ActionConfigurationIssue{{Code: "ACTION_COMPILE_FAILED", Message: err.Error(), Blocking: true}}
}

// actionConfigurationAffected 把编译阻断映射为路径配置保存错误的安全定位项。
func actionConfigurationAffected(err error) []model.PathConfigAffectedItem {
	issues := compileIssues(err)
	if len(issues) == 0 {
		return nil
	}
	first := issues[0]
	name := first.ActionID
	if name == "" {
		name = string(first.ActionKey)
	}
	return []model.PathConfigAffectedItem{{Kind: "action", Name: name, Reason: first.Message}}
}

// cloneConfiguredAction 深复制动作记录和参数，保存失败时不污染调用方草稿。
func cloneConfiguredAction(action model.ConfiguredAction) model.ConfiguredAction {
	copy := action
	if action.Parameters != nil {
		copy.Parameters = make(map[string]any, len(action.Parameters))
		for key, value := range action.Parameters {
			copy.Parameters[key] = value
		}
	}
	return copy
}

// sameConfiguredActions 比较动作语义正文而忽略服务端为每次保存分配的动作修订号。
func sameConfiguredActions(left, right []model.ConfiguredAction) bool {
	left = cloneConfiguredActions(left)
	right = cloneConfiguredActions(right)
	for index := range left {
		left[index].Revision = 0
	}
	for index := range right {
		right[index].Revision = 0
	}
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
}

// sameCompiledSteps 比较确定性编译步骤，避免同一幂等键复用不同的恢复或导航场景。
func sameCompiledSteps(left, right []model.CompiledActionStep) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
}

// samePersonStrategies 比较幂等重试的人员策略正文，避免相同键覆盖已保存的人员事实。
func samePersonStrategies(raw []byte, strategies map[string]model.PathConfigPersonStrategyInput) bool {
	encoded, err := json.Marshal(strategies)
	if err != nil {
		return false
	}
	current := decodeHistoryPersonStrategies(raw)
	currentEncoded, err := json.Marshal(current)
	return err == nil && bytes.Equal(currentEncoded, encoded)
}

// cloneConfiguredActions 复制动作记录切片，供幂等比较时清除服务端修订字段。
func cloneConfiguredActions(actions []model.ConfiguredAction) []model.ConfiguredAction {
	result := make([]model.ConfiguredAction, len(actions))
	for index, action := range actions {
		result[index] = cloneConfiguredAction(action)
	}
	return result
}

// decodeActionConfigurationIssues 解码动作场景保存的结构化阻断列表，不把表单问题误当作动作问题。
func decodeActionConfigurationIssues(raw []byte) []model.ActionConfigurationIssue {
	var objects []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &objects); err != nil || objects == nil {
		return []model.ActionConfigurationIssue{}
	}
	result := make([]model.ActionConfigurationIssue, 0, len(objects))
	for _, object := range objects {
		if _, actionIndex := object["index"]; !actionIndex {
			continue
		}
		encoded, err := json.Marshal(object)
		if err != nil {
			continue
		}
		var issue model.ActionConfigurationIssue
		if json.Unmarshal(encoded, &issue) == nil {
			result = append(result, issue)
		}
	}
	return result
}

// mergeActionConfigurationIssues 在共享 issues 列中替换旧动作问题，同时保留表单和来源问题。
func mergeActionConfigurationIssues(raw []byte, actionIssues []model.ActionConfigurationIssue) ([]byte, error) {
	objects := []map[string]json.RawMessage{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &objects); err != nil {
			return nil, err
		}
	}
	kept := make([]map[string]json.RawMessage, 0, len(objects)+len(actionIssues))
	for _, object := range objects {
		if _, actionIndex := object["index"]; actionIndex {
			continue
		}
		if _, actionID := object["actionId"]; actionID {
			continue
		}
		kept = append(kept, object)
	}
	for _, issue := range actionIssues {
		encoded, err := json.Marshal(issue)
		if err != nil {
			return nil, err
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &object); err != nil {
			return nil, err
		}
		kept = append(kept, object)
	}
	return json.Marshal(kept)
}

// lastString 返回路径节点序列最后一项。
func lastString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[len(values)-1]
}

// confirmedNodeKeysJSON 把已确认节点列写成 confirmed_node_keys：
// 已保存动作绑定的语义节点即视为已确认，另外并入显式传入的节点键（人员已保存但没有动作的节点）。
// 这一列原来从来没有人写入，导致节点状态永远停在待配置。
func confirmedNodeKeysJSON(current []byte, actions []model.ConfiguredAction, extra ...string) ([]byte, error) {
	confirmed := make(map[string]bool)
	for _, key := range decodeConfirmedNodeKeys(current) {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			confirmed[trimmed] = true
		}
	}
	for _, action := range actions {
		if key := strings.TrimSpace(action.NodeKey); key != "" {
			confirmed[key] = true
		}
	}
	for _, key := range extra {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			confirmed[trimmed] = true
		}
	}
	keys := make([]string, 0, len(confirmed))
	for key := range confirmed {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return json.Marshal(keys)
}

// AutoConfigurePathActions 一键配置时按真实门禁为路径上待配置节点补齐人员与动作。
// 只读取一次目标事实、只写一次配置行：逐节点调用保存接口会对同一账号发起十几次
// 目标流程读取，把目标只读网关打满，页面同时请求就会超时。
// 只从目录里已启用、非系统语义、非编译器插入、且不需要显式选人的动作中选择，不发明动作；
// 选择用计划、路径和节点键派生的确定性种子，并跨节点优先挑尚未用过的动作。
func (s *PathConfigService) AutoConfigurePathActions(ctx context.Context, planID, pathID uint64) error {
	if s.historyConfigStore == nil {
		return &PathConfigError{Kind: PathConfigErrorStorage, Message: "动作配置存储暂不可用"}
	}
	path, snapshot, analysis, current, found, _, err := s.loadWorkspace(ctx, planID, pathID)
	if err != nil {
		return err
	}
	// 节点、人员和动作目录直接从这次已经读到的目标事实投影，不再额外读一遍目标流程。
	configuration, _, err := s.configAnalyzer.Analyze(
		analysis.graph, snapshot.Tree, snapshot.FormFields, path, analysis.pathAnalysis,
		snapshot.InstanceValues, map[string]map[string]string{}, map[string]string{}, false,
	)
	if err != nil {
		return &PathConfigError{Kind: PathConfigErrorInvalid, Message: "执行路径配置无法投影，请重新核对路径"}
	}
	validation, err := s.pathActionGates(snapshot, path, analysis, found)
	if err != nil {
		return err
	}
	existing := []model.ConfiguredAction{}
	if found {
		existing = decodeWorkspaceActions(current.UserActions)
	}
	configuredNodes := make(map[string]bool)
	for _, action := range existing {
		configuredNodes[strings.TrimSpace(action.NodeKey)] = true
	}
	personStrategies := decodeHistoryPersonStrategies(current.PersonStrategies)
	actions := append([]model.ConfiguredAction(nil), existing...)
	used := make(map[string]bool)
	for _, action := range existing {
		used[string(action.Action)] = true
	}
	changed := false
	confirmedNodes := make([]string, 0, 8)
	for _, group := range configuration.Groups {
		for _, node := range group.Nodes {
			if node.LineBlocked || configuredNodes[node.Key] {
				continue
			}
			for _, person := range node.Persons {
				if !person.Editable {
					continue
				}
				personStrategies[person.Key] = autoPersonStrategy(person, autoConfigureSeed(planID, pathID, node.Key+":"+person.Key))
				changed = true
			}
			candidates := autoNodeActionCandidates(node, autoConfigureSeed(planID, pathID, node.Key), used)
			if len(candidates) == 0 {
				// 没有可编排动作的节点，只要人员已按目标默认或随机策略填好就算配置完成。
				if changed {
					confirmedNodes = append(confirmedNodes, node.Key)
				}
				continue
			}
			// 逐个候选试编译：动作顺序必须能被场景编译器恢复，无法恢复的候选直接换下一个。
			for _, action := range candidates {
				merged, mergeErr := mergeNodeActions(actions, []model.ConfiguredAction{action}, node.Key, analysis.graph, analysis.pathAnalysis)
				if mergeErr != nil {
					continue
				}
				if _, compileErr := compilePathActions(merged, analysis.graph, analysis.pathAnalysis, actionCatalogGates(validation)); compileErr != nil {
					continue
				}
				actions = merged
				used[string(action.Action)] = true
				confirmedNodes = append(confirmedNodes, node.Key)
				changed = true
				break
			}
		}
	}
	if !changed {
		return nil
	}
	compiled, compileErr := compilePathActions(actions, analysis.graph, analysis.pathAnalysis, actionCatalogGates(validation))
	if compileErr != nil {
		return &PathConfigError{Kind: PathConfigErrorInvalid, Message: "自动动作配置无法编译为可恢复场景"}
	}
	return s.persistAutoConfiguredActions(ctx, pathID, current, compiled, personStrategies, confirmedNodes)
}

// persistAutoConfiguredActions 用与手工保存相同的落盘规则一次性写入自动配置结果。
func (s *PathConfigService) persistAutoConfiguredActions(ctx context.Context, pathID uint64, current repository.HistoryPathConfigRecord, compiled scenario.Result, personStrategies map[string]model.PathConfigPersonStrategyInput, confirmedNodes []string) error {
	nextActionRevision := current.ActionRevision + 1
	for index := range compiled.Actions {
		compiled.Actions[index].Revision = nextActionRevision
	}
	actionJSON, err := json.Marshal(compiled.Actions)
	if err != nil {
		return &PathConfigError{Kind: PathConfigErrorInvalid, Message: "动作配置无法编码"}
	}
	stepJSON, err := json.Marshal(compiled.Steps)
	if err != nil {
		return &PathConfigError{Kind: PathConfigErrorInvalid, Message: "动作场景无法编码"}
	}
	issueJSON, err := mergeActionConfigurationIssues(current.Issues, compiled.Issues)
	if err != nil {
		return &PathConfigError{Kind: PathConfigErrorInvalid, Message: "动作问题无法编码"}
	}
	personJSON, err := json.Marshal(personStrategies)
	if err != nil {
		return &PathConfigError{Kind: PathConfigErrorInvalid, Message: "人员策略无法编码"}
	}
	latestJSON, err := json.Marshal(map[string]any{"idempotencyKey": "", "actionRevision": nextActionRevision})
	if err != nil {
		return &PathConfigError{Kind: PathConfigErrorInvalid, Message: "幂等结果无法编码"}
	}
	record := current
	record.PathID = pathID
	record.IdempotencyKey = ""
	record.UserActions = actionJSON
	record.CompiledSteps = stepJSON
	record.Issues = issueJSON
	record.PersonStrategies = personJSON
	record.LatestIdempotency = latestJSON
	record.ActionRevision = nextActionRevision
	record.NodeRevision = current.NodeRevision + 1
	record.Revision = current.Revision + 1
	record.ConfigStatus = actionConfigStatus(len(compiled.Actions))
	record.NodeStatus = record.ConfigStatus
	if strings.TrimSpace(record.SourceMode) == "" {
		record.SourceMode = model.HistorySourceModeNone
	}
	if strings.TrimSpace(record.RuntimeType) == "" {
		record.RuntimeType = string(target.FormRenderTypeUnknown)
	}
	if strings.TrimSpace(record.DataStatus) == "" {
		record.DataStatus = model.HistoryDataStatusEmpty
	}
	record.ConfirmedNodeKeys, err = confirmedNodeKeysJSON(current.ConfirmedNodeKeys, compiled.Actions, confirmedNodes...)
	if err != nil {
		return &PathConfigError{Kind: PathConfigErrorInvalid, Message: "已确认节点无法编码"}
	}
	if len(record.BranchPatches) == 0 {
		record.BranchPatches = []byte(`[]`)
	}
	for _, empty := range []*[]byte{&record.EffectiveFormData, &record.RuntimeValidation} {
		if len(*empty) == 0 {
			*empty = []byte(`{}`)
		}
	}
	if _, err := s.historyConfigStore.SavePathConfig(ctx, record, current.Revision, s.now().UTC()); err != nil {
		return mapHistoryWorkspaceStoreError(err)
	}
	return nil
}

// autoNodeActionCandidates 按覆盖优先、其次确定性种子给出该节点可尝试的动作顺序。
// 只取已启用、非系统语义、非编译器插入、且不需要显式选人的动作；参数由执行器运行时填充，不影响可选性。
func autoNodeActionCandidates(node model.PathConfigNode, seed uint64, used map[string]bool) []model.ConfiguredAction {
	available := make([]model.PathConfigActionCatalogItem, 0, len(node.ActionConfiguration.Catalog))
	for _, item := range node.ActionConfiguration.Catalog {
		if !item.Enabled || item.SystemOnly || item.SystemInserted || item.RequiresPerson {
			continue
		}
		available = append(available, item)
	}
	if len(available) == 0 {
		return nil
	}
	sort.Slice(available, func(left, right int) bool { return available[left].Kind < available[right].Kind })
	offset := int(seed % uint64(len(available)))
	ordered := make([]model.PathConfigActionCatalogItem, 0, len(available))
	for index := range available {
		ordered = append(ordered, available[(offset+index)%len(available)])
	}
	// 尚未被本路径用过的动作优先，让一次一键配置尽量覆盖更多目标能力。
	sort.SliceStable(ordered, func(left, right int) bool { return !used[ordered[left].Kind] && used[ordered[right].Kind] })
	candidates := make([]model.ConfiguredAction, 0, len(ordered))
	for _, item := range ordered {
		candidates = append(candidates, model.ConfiguredAction{
			Key: autoConfigureActionKey(node.Key, item.Kind), Action: model.ActionKey(item.Kind),
			Scope: model.ActionScope(item.Scope), NodeKey: node.Key, Order: 1,
		})
	}
	return candidates
}

// autoPersonStrategy 优先沿用目标默认名单，没有默认值时按确定性种子随机取够最少人数。
func autoPersonStrategy(person model.PathConfigPerson, seed uint64) model.PathConfigPersonStrategyInput {
	strategies := make(map[string]bool, len(person.Strategies))
	for _, item := range person.Strategies {
		strategies[item.Value] = true
	}
	if len(person.DefaultSelected) > 0 && strategies["target_default"] {
		return model.PathConfigPersonStrategyInput{Key: person.Key, Strategy: "target_default", Seed: 1, Selected: append([]string(nil), person.DefaultSelected...)}
	}
	if strategies["random"] && len(person.Options) > 0 {
		strategy := model.PathConfigPersonStrategyInput{Key: person.Key, Strategy: "random", Seed: int64(seed%1_000_000) + 1}
		strategy.Selected = projectedPersonSelection(person, strategy)
		return strategy
	}
	selected := make([]string, 0, len(person.Options))
	count := person.MinCount
	if count < 1 {
		count = 1
	}
	for index := 0; index < len(person.Options) && len(selected) < count; index++ {
		selected = append(selected, person.Options[(int(seed)+index)%len(person.Options)].Value)
	}
	return model.PathConfigPersonStrategyInput{Key: person.Key, Strategy: "manual", Seed: 1, Selected: selected}
}

// autoConfigureSeed 由计划、路径和节点键派生确定性种子，保证同一计划重复一键配置结果一致。
func autoConfigureSeed(planID, pathID uint64, token string) uint64 {
	digest := sha256.Sum256([]byte(fmt.Sprintf("auto:%d:%d:%s", planID, pathID, token)))
	return binary.BigEndian.Uint64(digest[:8])
}

// autoConfigureActionKey 生成稳定动作记录键，重复一键配置不会产生新记录键。
func autoConfigureActionKey(nodeKey, kind string) string {
	digest := sha256.Sum256([]byte("auto-action:" + nodeKey + ":" + kind))
	return hex.EncodeToString(digest[:16])
}

// AutoNodeActionForTest 暴露自动动作选择的首选项，供 test 目录下的定向用例锁定覆盖与可复现行为。
func AutoNodeActionForTest(planID, pathID uint64, node model.PathConfigNode, used map[string]bool) (model.ConfiguredAction, bool) {
	candidates := autoNodeActionCandidates(node, autoConfigureSeed(planID, pathID, node.Key), used)
	if len(candidates) == 0 {
		return model.ConfiguredAction{}, false
	}
	return candidates[0], true
}

// ConfirmedNodeKeysJSONForTest 暴露已确认节点编码，供 test 目录下的定向用例锁定行为。
func ConfirmedNodeKeysJSONForTest(current []byte, actions []model.ConfiguredAction, extra ...string) ([]byte, error) {
	return confirmedNodeKeysJSON(current, actions, extra...)
}
