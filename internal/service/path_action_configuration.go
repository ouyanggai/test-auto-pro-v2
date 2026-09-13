package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
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

const actionConfigurationSaveAttempts = 3

// SaveActionConfiguration 保存当前语义节点动作；浏览器不参与版本协商，内部并发冲突由服务端重新合并。
func (s *PathConfigService) SaveActionConfiguration(ctx context.Context, planID, pathID uint64, nodeKey, idempotencyKey string, input model.ActionConfigurationInput) (model.ActionConfigurationResult, error) {
	for attempt := 1; attempt <= actionConfigurationSaveAttempts; attempt++ {
		result, err := s.saveActionConfigurationOnce(ctx, planID, pathID, nodeKey, idempotencyKey, input)
		if err == nil {
			return result, nil
		}
		if !errors.Is(err, repository.ErrHistoryPathConfigConflict) {
			return result, err
		}
	}
	return model.ActionConfigurationResult{}, &PathConfigError{Kind: PathConfigErrorStorage, Message: "节点配置保存繁忙，请重试"}
}

// saveActionConfigurationOnce 基于一次服务端最新读取合并并保存动作；仓储竞争由外层重新读取后重试。
func (s *PathConfigService) saveActionConfigurationOnce(ctx context.Context, planID, pathID uint64, nodeKey, idempotencyKey string, input model.ActionConfigurationInput) (model.ActionConfigurationResult, error) {
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
	// 第一版动作保存不把浏览器修订号作为门禁：服务端始终基于刚读取的最新配置合并当前节点，
	// 避免表单数据和节点动作各自推进计数后互相制造无意义的版本冲突。
	// 幂等键仍用于识别响应丢失后的同一次重试，不能用同一键提交不同正文。
	sameIdempotency := found && strings.TrimSpace(idempotencyKey) != "" && current.IdempotencyKey == strings.TrimSpace(idempotencyKey)
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
		return result, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "动作配置存在结构问题，请修正首个阻断项", Affected: actionConfigurationAffected(compileErr)}
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
	// 浏览器不参与版本协商；仓储只用本次服务端读取到的整体修订做事务内并发保护。
	saved, err := s.historyConfigStore.SavePathConfig(ctx, record, current.Revision, s.now().UTC())
	if err != nil {
		if !errors.Is(err, repository.ErrHistoryPathConfigConflict) {
			return model.ActionConfigurationResult{}, mapHistoryWorkspaceStoreError(err)
		}
		return model.ActionConfigurationResult{}, err
	}
	return actionConfigurationResult(path, saved, compiled), nil
}

// pathActionGates 重读当前真实路径的动作门禁与人员目录投影，保存和预览共用同一份事实。
func (s *PathConfigService) pathActionGates(snapshot target.PathConfigurationSnapshot, path model.ExecutionPath, analysis ownedPathAnalysis, found bool) (analyzer.PathConfigValidation, error) {
	if s.configAnalyzer == nil {
		return analyzer.PathConfigValidation{}, &PathConfigError{Kind: PathConfigErrorStorage, Message: "动作条件校验服务暂不可用"}
	}
	_, validation, err := s.configAnalyzer.Analyze(analysis.graph, snapshot.Tree, snapshot.FormFields, path, analysis.pathAnalysis, snapshot.InstanceValues, map[string]map[string]string{}, map[string]string{}, found)
	if err != nil {
		return analyzer.PathConfigValidation{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前路径动作条件无法核对"}
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
		if action.Action != model.ActionAddSign && action.Action != model.ActionTransfer && action.Action != model.ActionForward {
			continue
		}
		personTarget := target.ActionPersons[string(model.ActionAddSign)]
		if action.Action == model.ActionTransfer {
			personTarget = target.ActionPersons[string(model.ActionTransfer)]
		} else if action.Action == model.ActionForward {
			personTarget = target.ActionPersons[string(model.ActionForward)]
		}
		if personTarget == nil {
			if action.Action == model.ActionTransfer {
				// 当前目标没有完整处理人候选时，保留动作交给实时目录门禁返回统一的“移交不可用”原因；
				// 这里不能提前返回人员策略错误，否则保存接口会丢失动作级阻断定位。
				continue
			}
			label := "加签处理人"
			if action.Action == model.ActionForward {
				label = "转发接收人"
			}
			return &PathConfigError{Kind: PathConfigErrorInvalid, Message: label + "缺少当前候选目录", Affected: []model.PathConfigAffectedItem{{Kind: "person", Name: label, Reason: "目标未返回可用候选"}}}
		}
		person, ok := byKey[personTarget.Key]
		if !ok {
			return &PathConfigError{Kind: PathConfigErrorInvalid, Message: personTarget.Name + "策略不完整", Affected: []model.PathConfigAffectedItem{{Kind: "person", Name: personTarget.Name, Reason: "动作需要明确候选人员"}}}
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
		if action.Action == model.ActionAddSign || action.Action == model.ActionTransfer || action.Action == model.ActionForward {
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

// ResolveActionPersonIDs 按运行启动时的真实目标目录把动作人员策略解析为内部用户 ID。
// 浏览器只保存不透明候选键；目标结构或候选变化时必须重新校验并阻止运行，不能猜测替代。
func (s *PathConfigService) ResolveActionPersonIDs(ctx context.Context, planID, pathID uint64, nodeKey string, action model.ActionKey) ([]string, error) {
	if planID == 0 || pathID == 0 || strings.TrimSpace(nodeKey) == "" {
		return nil, &PathConfigError{Kind: PathConfigErrorInvalidArgument, Message: "人员策略解析参数不正确"}
	}
	path, snapshot, analysis, current, found, _, err := s.loadWorkspace(ctx, planID, pathID)
	if err != nil {
		return nil, err
	}
	validation, err := s.pathActionGates(snapshot, path, analysis, found)
	if err != nil {
		return nil, err
	}
	node, ok := validation.NodeTokens[strings.TrimSpace(nodeKey)]
	if !ok {
		return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "动作节点不属于当前真实路径"}
	}
	personTarget := node.ActionPersons[string(action)]
	if personTarget == nil {
		return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前动作没有可用的目标人员候选"}
	}
	strategies := decodeHistoryPersonStrategies(current.PersonStrategies)
	strategy, ok := strategies[personTarget.Key]
	if !ok {
		return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前动作缺少已保存的人员策略"}
	}
	encoded, reason := analyzer.EncodePathConfigPersonStrategy(*personTarget, strategy)
	if reason != "" {
		return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前动作人员策略已失效：" + reason}
	}
	var planData struct {
		Selected []string `json:"selected"`
	}
	if err := json.Unmarshal([]byte(encoded), &planData); err != nil || len(planData.Selected) == 0 {
		return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前动作没有可执行的目标人员"}
	}
	return append([]string(nil), planData.Selected...), nil
}

// ResolveNodeAuditors 按当前目标目录解析 run_node_choose 节点的真实处理人。
// 浏览器只保存不透明候选键；启动运行时必须重新映射为目标 bizId 和名称，候选变动则拒绝发起。
func (s *PathConfigService) ResolveNodeAuditors(ctx context.Context, planID, pathID uint64, nodeKey string) ([]target.NextAuditor, error) {
	if planID == 0 || pathID == 0 || strings.TrimSpace(nodeKey) == "" {
		return nil, &PathConfigError{Kind: PathConfigErrorInvalidArgument, Message: "下一节点人员策略解析参数不正确"}
	}
	path, snapshot, analysis, current, found, _, err := s.loadWorkspace(ctx, planID, pathID)
	if err != nil {
		return nil, err
	}
	validation, err := s.pathActionGates(snapshot, path, analysis, found)
	if err != nil {
		return nil, err
	}
	node, ok := validation.NodeTokens[strings.TrimSpace(nodeKey)]
	if !ok || node.Person == nil {
		return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "下一节点没有可用的处理人目录"}
	}
	strategies := decodeHistoryPersonStrategies(current.PersonStrategies)
	strategy, ok := strategies[node.Person.Key]
	if !ok {
		return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "下一节点缺少已保存的处理人策略"}
	}
	encoded, reason := analyzer.EncodePathConfigPersonStrategy(*node.Person, strategy)
	if reason != "" {
		return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "下一节点处理人策略已失效：" + reason}
	}
	var planData struct {
		Selected []string `json:"selected"`
	}
	if err := json.Unmarshal([]byte(encoded), &planData); err != nil || len(planData.Selected) == 0 {
		return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "下一节点没有可执行的处理人"}
	}
	result := make([]target.NextAuditor, 0, len(planData.Selected))
	for _, id := range planData.Selected {
		id = strings.TrimSpace(id)
		name := strings.TrimSpace(node.Person.CandidateNames[id])
		if id == "" || name == "" {
			return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "下一节点处理人目录已失效，请重新配置"}
		}
		result = append(result, target.NextAuditor{BizID: id, Name: name, AuditDetailTyp: "personnel"})
	}
	return result, nil
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
				Person:     projectActionPersonFromCatalog(configuration.InstanceActions.Catalog, action, persons),
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
	return projectActionPersonFromCatalog(node.ActionConfiguration.Catalog, action, persons)
}

// projectActionPersonFromCatalog 从动作目录候选与独立人员策略恢复动作人员，实例动作和节点动作共用同一规则。
func projectActionPersonFromCatalog(catalog []model.PathConfigActionCatalogItem, action model.ConfiguredAction, persons map[string]model.PathConfigPersonStrategyInput) *model.PathConfigPersonStrategyInput {
	kind := actionDisplayKind(action.Action)
	var source *model.PathConfigPerson
	for index := range catalog {
		item := &catalog[index]
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

// actionDisplayLabel 返回动作在节点工作台与运行准备面板中的中文标签。
// 覆盖动作目录当前全部 15 条动作与系统自动语义：界面必须用业务语言，不允许把稳定键当文案显示。
// 未知稳定键保留原值只作为定位兜底，出现即说明目录新增了动作而这里没跟上。
func actionDisplayLabel(action model.ActionKey) string {
	labels := map[model.ActionKey]string{
		model.ActionSaveDraft:       "保存草稿",
		model.ActionSubmit:          "提交",
		model.ActionResubmit:        "重新提交",
		model.ActionStorageFormData: "暂存当前表单",
		model.ActionAddSign:         "加签",
		model.ActionTransfer:        "移交",
		model.ActionApprove:         "同意",
		model.ActionReject:          "不同意",
		model.ActionRollback:        "回退上一节点",
		model.ActionRetrieve:        "取回",
		model.ActionWithdraw:        "撤回",
		model.ActionUrge:            "催办",
		model.ActionForward:         "转发",
		model.ActionFollow:          "关注",
		model.ActionUnfollow:        "取消关注",
		model.ActionSystemAutomatic: "系统自动动作",
	}
	if label, ok := labels[action]; ok {
		return label
	}
	return string(action)
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

// fixedTailKinds 是编译器固定尾动作集合：发起节点“提交”、审批/协同节点“同意”、
// 以及草稿/驳回/撤回后由编译器运行时改用的“重新提交”。
// 这三个键永远不是一键配置的额外用户动作；把它们写进 user_actions 会把“已配置”伪装出来（F-034 T02）。
func fixedTailKinds() map[model.ActionKey]bool {
	return map[model.ActionKey]bool{
		model.ActionSubmit: true, model.ActionApprove: true, model.ActionResubmit: true,
	}
}

// isAutoGeneratedFixedTailRecord 判断一条已保存动作是不是旧版一键配置写入的固定尾动作占位记录。
// 识别依据是动作键而非动作名：只有键等于 autoConfigureActionKey(nodeKey, action) 的确定性键、
// 且动作属于固定尾动作集合时才算。用户手工保存的同语义动作有浏览器生成的随机键，不会被误删。
func isAutoGeneratedFixedTailRecord(action model.ConfiguredAction) bool {
	if !fixedTailKinds()[action.Action] {
		return false
	}
	return action.Key == autoConfigureActionKey(strings.TrimSpace(action.NodeKey), string(action.Action))
}

// hasExtraUserAction 判断节点是否已经存在“固定尾动作之外”的额外用户动作。
// 一键配置以它而不是“是否有任意动作”判断是否需要补配：已有人工动作的节点不再追加，
// 只有固定尾动作占位记录的节点仍视为待补配。
func hasExtraUserAction(actions []model.ConfiguredAction, nodeKey string) bool {
	nodeKey = strings.TrimSpace(nodeKey)
	for _, action := range actions {
		if strings.TrimSpace(action.NodeKey) != nodeKey {
			continue
		}
		if fixedTailKinds()[action.Action] {
			continue
		}
		return true
	}
	return false
}

// autoExtraCandidate 是一键配置为单个节点评估出的一条额外动作候选。
type autoExtraCandidate struct {
	action model.ConfiguredAction
	// person 是动作私有人员策略（加签/移交）；只有候选自带完整目录时才生成，绝不复用节点主处理人键。
	person *model.PathConfigPersonStrategyInput
	// rank 是安全等级：相同等级内才按“未使用优先、稳定种子轮转”追求动作多样性。
	rank int
}

// autoExtraActionRanks 是审批/协同节点额外动作的安全等级：暂存表单检查点最安全，
// 其次是有完整候选的加签/移交，再次是可恢复的取回/不同意，回退需要真实前驱最后。
// 发起节点只有保存草稿一个合法额外动作，不进入这张表。
var autoExtraActionRanks = map[model.ActionKey]int{
	model.ActionStorageFormData: 0,
	model.ActionAddSign:         1, model.ActionTransfer: 1,
	model.ActionRetrieve: 2, model.ActionReject: 2,
	model.ActionRollback: 3,
}

// autoExtraActionCandidates 按目标前后端静态动作矩阵给出该节点可尝试的额外动作候选与淘汰原因（F-034 T02）。
// 矩阵约束：发起节点只允许“保存草稿”；审批/协同节点只允许暂存表单、加签、移交、不同意、回退、取回；
// 系统/空/条件等节点不补任何动作；需要人员的动作必须自带完整候选目录，找不到就给出精确原因。
// 候选只经过静态过滤，完整场景编译仍由调用方逐个验证；不覆盖人工事实、固定尾动作不计入额外动作。
func autoExtraActionCandidates(node model.PathConfigNode, seed uint64, used map[string]bool) ([]autoExtraCandidate, []string) {
	rejections := []string{}
	// 目标模板声明“无处理人时跳过”等运行时语义与动作选择无关，静态阶段只保证动作本身可保存。
	switch strings.TrimSpace(node.Kind) {
	case "start":
		// 发起节点固定尾动作负责“提交”；唯一的额外动作是保存草稿，重新提交是恢复链动作不许随机分配。
		for _, item := range node.ActionConfiguration.Catalog {
			if item.Kind != string(model.ActionSaveDraft) {
				continue
			}
			if !item.Enabled || item.SystemOnly || item.SystemInserted {
				rejections = append(rejections, "保存草稿："+autoCandidateDisabledReason(item))
				continue
			}
			return []autoExtraCandidate{{
				rank: 0,
				action: model.ConfiguredAction{Key: autoConfigureActionKey(node.Key, item.Kind), Action: model.ActionSaveDraft,
					Scope: model.ActionScopeInitiator, NodeKey: node.Key, Order: 1},
			}}, rejections
		}
		rejections = append(rejections, "保存草稿：当前目标目录未返回发起端动作，不能保存")
		return nil, rejections
	case "common", "synergy":
		available := make([]model.PathConfigActionCatalogItem, 0, len(node.ActionConfiguration.Catalog))
		for _, item := range node.ActionConfiguration.Catalog {
			kind := model.ActionKey(item.Kind)
			if _, allowed := autoExtraActionRanks[kind]; !allowed {
				continue
			}
			if !item.Enabled || item.SystemOnly || item.SystemInserted {
				rejections = append(rejections, item.Label+"："+autoCandidateDisabledReason(item))
				continue
			}
			available = append(available, item)
		}
		if len(available) == 0 {
			if len(rejections) == 0 {
				rejections = append(rejections, "当前目标目录没有返回可安全编排的额外动作")
			}
			return nil, rejections
		}
		// 排序（F-034 评审修正）：本路径尚未使用的动作优先（跨安全等级），
		// 同为未使用/已使用时再按安全等级与稳定键排，最后按种子在同优先级内轮转；
		// 不用全局随机数，重试可复现。
		sort.SliceStable(available, func(left, right int) bool {
			leftUsed, rightUsed := used[available[left].Kind], used[available[right].Kind]
			if leftUsed != rightUsed {
				return !leftUsed
			}
			leftRank, rightRank := autoExtraActionRanks[model.ActionKey(available[left].Kind)], autoExtraActionRanks[model.ActionKey(available[right].Kind)]
			if leftRank != rightRank {
				return leftRank < rightRank
			}
			return available[left].Kind < available[right].Kind
		})
		// 种子偏移只在同一优先级（未使用+等级相同的连续段）内部进行，不破坏前面的全局次序。
		rotated := make([]model.PathConfigActionCatalogItem, 0, len(available))
		for start := 0; start < len(available); {
			end := start + 1
			usedKey, rank := used[available[start].Kind], autoExtraActionRanks[model.ActionKey(available[start].Kind)]
			for end < len(available) && used[available[end].Kind] == usedKey && autoExtraActionRanks[model.ActionKey(available[end].Kind)] == rank {
				end++
			}
			group := available[start:end]
			offset := int(seed % uint64(len(group)))
			for index := range group {
				rotated = append(rotated, group[(offset+index)%len(group)])
			}
			start = end
		}
		candidates := make([]autoExtraCandidate, 0, len(rotated))
		for _, item := range rotated {
			kind := model.ActionKey(item.Kind)
			candidate := autoExtraCandidate{
				rank: autoExtraActionRanks[kind],
				action: model.ConfiguredAction{Key: autoConfigureActionKey(node.Key, item.Kind), Action: kind,
					Scope: model.ActionScope(item.Scope), NodeKey: node.Key, Order: 1},
			}
			if item.RequiresPerson {
				if item.Person == nil || len(item.Person.Options) == 0 {
					rejections = append(rejections, item.Label+"：目标没有返回动作专属处理人候选，不能自动配置")
					continue
				}
				strategy := autoPersonStrategy(*item.Person, autoConfigureSeed(0, 0, node.Key+":action:"+item.Kind))
				if len(strategy.Selected) == 0 {
					rejections = append(rejections, item.Label+"：动作专属候选为空，不能用空人员凑数")
					continue
				}
				// 完整策略/人数校验在接受循环进行（需要验证层的人员目标目录，见 validateAutoCandidatePerson）。
				person := strategy
				candidate.person = &person
			}
			candidates = append(candidates, candidate)
		}
		return candidates, rejections
	default:
		// 条件、手动、并行、汇聚、空、结束、定时、子流程、回调等系统节点不补用户动作，
		// 只保留导航/只读语义；这不是配置失败，不产生淘汰原因。
		return nil, rejections
	}
}

// autoCandidateDisabledReason 返回候选被目标目录禁用时的具体原因，供一键配置结果报告定位。
func autoCandidateDisabledReason(item model.PathConfigActionCatalogItem) string {
	if reason := strings.TrimSpace(item.DisabledReason); reason != "" {
		return reason
	}
	return "当前动作执行条件不满足"
}

// AutoConfigurePathActions 一键配置时按真实门禁为路径上待配置节点补齐人员与“固定尾动作之外”的一条额外动作（F-034 T02）。
// 边界：
//  1. 只读取一次目标事实、只写一次配置行；逐节点调用保存接口会对同一账号发起十几次目标读取，把只读网关打满。
//  2. 已有人工动作、参数和人员策略原样保留；只有没有额外用户动作的节点才补一条。
//  3. 固定尾动作（提交/同意/重新提交）由编译器生成，不计入额外动作；旧版一键配置写入的固定尾占位记录
//     按稳定键准确识别并清除，不靠动作名称猜测。
//  4. 候选先套目标静态动作矩阵（发起节点仅保存草稿；审批节点暂存/加签/移交/不同意/回退/取回；
//     系统节点不补动作），再逐候选完整编译；无安全候选时报告节点名与原因，不以“已配置”凑数。
//  5. 实例动作容器不参与本操作的自动补配：实例动作影响主实例生命周期，留给人工决策。
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
	// F-034 T02：旧版一键配置把固定尾动作占位记录写进了 user_actions，让节点看似“已配置”。
	// 重新生成时按动作来源稳定键准确识别并清除这些记录；人工保存的动作（随机键）不受影响。
	personStrategies := decodeHistoryPersonStrategies(current.PersonStrategies)
	actions := make([]model.ConfiguredAction, 0, len(existing))
	clearedFixedTail := false
	for _, action := range existing {
		if isAutoGeneratedFixedTailRecord(action) {
			clearedFixedTail = true
			continue
		}
		actions = append(actions, action)
	}
	if clearedFixedTail {
		personStrategies = normalizeAutoPersonStrategies(personStrategies, actions)
	}
	changed := clearedFixedTail
	confirmedNodes := make([]string, 0, 8)
	autoConfiguredNodes := make([]string, 0, 8)
	// F-034 评审修正：候选级失败只是诊断信息；只有业务节点没有任何可用动作才构成阻塞失败，
	// 避免“数据已保存但接口返回失败”的状态不一致。
	diagnostics := make([]string, 0, 8)
	blockingRejections := make([]string, 0, 8)
	for _, group := range configuration.Groups {
		for gi := range group.Nodes {
			node := &group.Nodes[gi]
			if node.LineBlocked {
				continue
			}
			personChanged := false
			for _, person := range node.Persons {
				if !person.Editable {
					continue
				}
				if _, alreadyConfigured := personStrategies[person.Key]; alreadyConfigured {
					// 一键配置只能补齐空白人员项，人工保存的策略属于用户事实，不能被随机覆盖。
					continue
				}
				if person.Affected {
					// 候选数量或目录状态已不满足模板约束时保留阻塞事实，不写入一份必然失效的随机策略。
					continue
				}
				personStrategies[person.Key] = autoPersonStrategy(person, autoConfigureSeed(planID, pathID, node.Key+":"+person.Key))
				personChanged = true
				changed = true
			}
			// F-034：判断维度从“节点是否有任意动作”改为“节点是否已有额外用户动作”；
			// 固定尾动作由编译器生成，有它的节点仍然需要补配。
			if hasExtraUserAction(actions, node.Key) {
				if personChanged {
					confirmedNodes = append(confirmedNodes, node.Key)
				}
				continue
			}
			candidates, rejections := autoExtraActionCandidates(*node, autoConfigureSeed(planID, pathID, node.Key), usedInPath(actions))
			// F-034 评审修正：候选级失败先记入诊断，只有业务节点最终没有任何可用动作才升级为阻塞失败。
			diagnostics = append(diagnostics, rejections...)
			if len(candidates) == 0 {
				// 系统节点没有候选是正常情况；业务节点没有候选时不假装已配置，原因升级为阻塞。
				if isConfigurableBusinessNode(node.Kind) {
					blockingRejections = append(blockingRejections, "节点 "+node.Name+" 未找到可安全配置的额外动作："+strings.Join(dedupStrings(rejections), "；"))
				}
				if personChanged {
					confirmedNodes = append(confirmedNodes, node.Key)
				}
				continue
			}
			// 逐个候选试编译：额外动作必须能通过与手工保存完全相同的结构、人员与场景编译验证；
			// 编译不通过（恢复链不可生成、参数无法安全构造、目录禁用等）就换下一个并记录原因。
			accepted := false
			for _, candidate := range candidates {
				// F-034 评审修正：候选接受前必须通过与手工保存相同的策略合法性、
				// 最小/最大人数与人员令牌校验；人数不足或非法策略直接淘汰，不写入空/短人员集合。
				if reason := validateAutoCandidatePerson(validation, node.Key, candidate); reason != "" {
					diagnostics = append(diagnostics, node.Name+"·"+actionDisplayLabel(candidate.action.Action)+"：人员策略不合法（"+reason+"）")
					continue
				}
				merged, mergeErr := mergeNodeActions(actions, []model.ConfiguredAction{candidate.action}, node.Key, analysis.graph, analysis.pathAnalysis)
				if mergeErr != nil {
					diagnostics = append(diagnostics, node.Name+"·"+actionDisplayLabel(candidate.action.Action)+"：场景合并被拒绝")
					continue
				}
				if _, compileErr := compilePathActions(merged, analysis.graph, analysis.pathAnalysis, actionCatalogGates(validation)); compileErr != nil {
					diagnostics = append(diagnostics, node.Name+"·"+actionDisplayLabel(candidate.action.Action)+"："+firstCompileIssueMessage(compileErr))
					continue
				}
				actions = merged
				if candidate.person != nil {
					personStrategies[candidate.person.Key] = *candidate.person
				}
				confirmedNodes = append(confirmedNodes, node.Key)
				autoConfiguredNodes = append(autoConfiguredNodes, node.Name+"（"+actionDisplayLabel(candidate.action.Action)+"）")
				changed = true
				accepted = true
				break
			}
			if !accepted {
				// 该业务节点最终没有任何额外动作：属于必须让用户看到的失败，候选级失败原因一并附上。
				blockingRejections = append(blockingRejections, "节点 "+node.Name+" 未找到可安全配置的额外动作："+strings.Join(dedupStrings(append([]string{diagnosticSummaryOfNode(node.Name, diagnostics)}, rejections...)), "；"))
			}
		}
	}
	// 只有业务节点没有任何可用动作才返回失败；候选级失败只是诊断，不改变整体成功语义。
	var rejectionError error
	if len(blockingRejections) > 0 {
		rejectionError = &PathConfigError{Kind: PathConfigErrorInvalid,
			Message: strings.Join(dedupStrings(blockingRejections), "；")}
	}
	if !changed {
		// 部分节点无安全候选也要报告；写入没有变化时同样返回原因，不静默吞掉。
		return rejectionError
	}
	compiled, compileErr := compilePathActions(actions, analysis.graph, analysis.pathAnalysis, actionCatalogGates(validation))
	if compileErr != nil {
		return &PathConfigError{Kind: PathConfigErrorInvalid, Message: "自动动作配置无法编译为可恢复场景"}
	}
	if err := s.persistAutoConfiguredActions(ctx, pathID, current, compiled, personStrategies, confirmedNodes); err != nil {
		return err
	}
	return rejectionError
}

// validateAutoCandidatePerson 按当前验证层的人员目标目录复验动作私有人员策略（F-034 评审修正）：
// 策略合法性、最小/最大人数与人员令牌必须与手工保存同一套规则（analyzer.EncodePathConfigPersonStrategy）；
// 目录没有该动作的人员目标时返回淘汰原因，绝不复用节点主处理人键或写入空/短人员集合。
func validateAutoCandidatePerson(validation analyzer.PathConfigValidation, nodeKey string, candidate autoExtraCandidate) string {
	if candidate.person == nil {
		return ""
	}
	nodeTarget, ok := validation.NodeTokens[strings.TrimSpace(nodeKey)]
	if !ok {
		return "当前节点没有人员候选目录"
	}
	personTarget := nodeTarget.ActionPersons[string(candidate.action.Action)]
	if personTarget == nil {
		return "当前动作没有动作专属人员候选目录"
	}
	_, reason := analyzer.EncodePathConfigPersonStrategy(*personTarget, *candidate.person)
	return reason
}

// diagnosticSummaryOfNode 取该节点最新一条诊断文案；没有时返回空串，由 rejections 兜底。
func diagnosticSummaryOfNode(nodeName string, diagnostics []string) string {
	for index := len(diagnostics) - 1; index >= 0; index-- {
		if strings.HasPrefix(diagnostics[index], nodeName+"·") {
			return diagnostics[index]
		}
	}
	return ""
}

// usedInPath 统计当前路径已保存动作键，供跨节点“未使用动作优先”排序。
func usedInPath(actions []model.ConfiguredAction) map[string]bool {
	used := make(map[string]bool, len(actions))
	for _, action := range actions {
		used[string(action.Action)] = true
	}
	return used
}

// isConfigurableBusinessNode 判断节点是否属于可配置业务节点：发起与审批/协同是，系统/空节点不是。
func isConfigurableBusinessNode(kind string) bool {
	switch strings.TrimSpace(kind) {
	case "start", "common", "synergy":
		return true
	default:
		return false
	}
}

// firstCompileIssueMessage 提取编译错误的首个阻断文案，供一键配置报告具体淘汰原因。
func firstCompileIssueMessage(err error) string {
	if compileErr, ok := err.(*scenario.CompileError); ok && len(compileErr.Issues) > 0 {
		return compileErr.Issues[0].Message
	}
	return err.Error()
}

// normalizeAutoPersonStrategies 清除旧固定尾占位记录后保持策略集合稳定，
// 由下次读取时的目录校验自然淘汰失效项；这里只保证不因清除而 panic 或丢节点策略。
func normalizeAutoPersonStrategies(strategies map[string]model.PathConfigPersonStrategyInput, actions []model.ConfiguredAction) map[string]model.PathConfigPersonStrategyInput {
	_ = actions
	return strategies
}

// dedupStrings 去重并保持顺序，避免同一节点在多个候选上重复出现在一键配置报告中。
func dedupStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
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

// autoPersonStrategy 一键配置始终优先按当前合法范围稳定随机选人。
// 目标默认名单只描述目标模板事实，不能覆盖用户要求的默认随机策略；会签人数由人员项的最小人数约束决定。
func autoPersonStrategy(person model.PathConfigPerson, seed uint64) model.PathConfigPersonStrategyInput {
	strategies := make(map[string]bool, len(person.Strategies))
	for _, item := range person.Strategies {
		strategies[item.Value] = true
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

// AutoPersonStrategyForTest 暴露一键人员策略，供 test 目录锁定目标默认名单不得覆盖范围随机的约束。
func AutoPersonStrategyForTest(person model.PathConfigPerson, seed uint64) model.PathConfigPersonStrategyInput {
	return autoPersonStrategy(person, seed)
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

// autoExtraCandidatesForTest 暴露新候选矩阵的首选项与淘汰原因，供 test 目录锁定 F-034 行为：
// 发起节点只允许保存草稿；审批节点按安全等级与稳定种子排序；系统节点无候选。
func AutoExtraCandidatesForTest(planID, pathID uint64, node model.PathConfigNode, used map[string]bool) (model.ConfiguredAction, *model.PathConfigPersonStrategyInput, bool, []string) {
	candidates, rejections := autoExtraActionCandidates(node, autoConfigureSeed(planID, pathID, node.Key), used)
	if len(candidates) == 0 {
		return model.ConfiguredAction{}, nil, false, rejections
	}
	return candidates[0].action, candidates[0].person, true, rejections
}

// AutoPersonStrategyForTest 暴露一键人员策略，供 test 目录锁定目标默认名单不得覆盖范围随机的约束。

// ConfirmedNodeKeysJSONForTest 暴露已确认节点编码，供 test 目录下的定向用例锁定行为。
func ConfirmedNodeKeysJSONForTest(current []byte, actions []model.ConfiguredAction, extra ...string) ([]byte, error) {
	return confirmedNodeKeysJSON(current, actions, extra...)
}

// ValidateAutoCandidatePersonForTest 暴露动作人员策略完整校验，供 test 目录锁定 F-034 评审 #4 行为：
// 策略合法性、最小/最大人数与人员令牌必须通过 analyzer.EncodePathConfigPersonStrategy 校验。
func ValidateAutoCandidatePersonForTest(validation analyzer.PathConfigValidation, nodeKey string, action model.ActionKey, person *model.PathConfigPersonStrategyInput) string {
	return validateAutoCandidatePerson(validation, nodeKey, autoExtraCandidate{action: model.ConfiguredAction{Action: action}, person: person})
}
