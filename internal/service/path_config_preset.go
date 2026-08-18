package service

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
)

const (
	pathConfigPresetCurrent    = "current"
	pathConfigPresetSelected   = "selected"
	pathConfigPresetCompatible = "compatible"
)

type pathConfigPresetCandidate struct {
	path          model.ExecutionPath
	stored        model.StoredPathConfig
	found         bool
	configuration model.PathConfiguration
	validation    analyzer.PathConfigValidation
	snapshot      target.PathConfigurationSnapshot
}

type pathConfigPresetDecision struct {
	item  model.PathConfigPresetNodeItem
	input model.PathNodeSaveInput
}

type pathConfigPresetActionSlot struct {
	candidateIndex int
	decisionIndex  int
	node           model.PathConfigNode
	seed           string
}

// PreviewPreset 预览指定范围内每个节点的随机动作预设结果，绝不写入工具侧或目标平台。
func (s *PathConfigService) PreviewPreset(ctx context.Context, planID, pathID uint64, scope string) (model.PathConfigPresetPreview, error) {
	candidates, err := s.pathConfigPresetCandidates(ctx, planID, pathID, scope)
	if err != nil {
		return model.PathConfigPresetPreview{}, err
	}
	preview := model.PathConfigPresetPreview{Scope: scope, Paths: make([]model.PathConfigPresetPath, 0, len(candidates))}
	decisionsByCandidate := pathConfigPresetDecisions(candidates)
	for candidateIndex, candidate := range candidates {
		items := make([]model.PathConfigPresetNodeItem, 0)
		for _, decision := range decisionsByCandidate[candidateIndex] {
			items = append(items, decision.item)
		}
		preview.Paths = append(preview.Paths, model.PathConfigPresetPath{Path: model.PathConfigPath{SequenceNo: candidate.path.SequenceNo, Name: candidate.path.Name}, Items: items})
	}
	return preview, nil
}

// ApplyPreset 写入预览中的随机动作；已保存动作、人员和循环都保持原样。
func (s *PathConfigService) ApplyPreset(ctx context.Context, planID, pathID uint64, scope string) (model.PathConfigPresetApplyResult, error) {
	candidates, err := s.pathConfigPresetCandidates(ctx, planID, pathID, scope)
	if err != nil {
		return model.PathConfigPresetApplyResult{}, err
	}
	result := model.PathConfigPresetApplyResult{Preview: model.PathConfigPresetPreview{Scope: scope, Paths: make([]model.PathConfigPresetPath, 0, len(candidates))}}
	decisionsByCandidate := pathConfigPresetDecisions(candidates)
	for candidateIndex, candidate := range candidates {
		decisions := decisionsByCandidate[candidateIndex]
		items := make([]model.PathConfigPresetNodeItem, 0, len(decisions))
		values := copyPathConfigActionValues(candidate.stored.ActionValues)
		confirmed := append([]string(nil), candidate.stored.ConfirmedNodeKeys...)
		changed := false
		for _, decision := range decisions {
			items = append(items, decision.item)
			switch decision.item.Status {
			case "write":
				encoded, validateErr := validatePathConfigNodeSubmission(candidate.validation.NodeTokens[decision.item.NodeKey], decision.input)
				if validateErr != nil {
					items[len(items)-1].Status, items[len(items)-1].Detail = "manual", "当前目标规则需要人工处理"
					result.Manual++
					continue
				}
				for key, value := range encoded {
					values[key] = value
				}
				confirmed = appendUnique(confirmed, decision.item.NodeKey)
				changed, result.Written = true, result.Written+1
			case "keep":
				result.Kept++
			case "skip":
				result.Skipped++
			case "manual":
				result.Manual++
			}
		}
		result.Preview.Paths = append(result.Preview.Paths, model.PathConfigPresetPath{Path: model.PathConfigPath{SequenceNo: candidate.path.SequenceNo, Name: candidate.path.Name}, Items: items})
		if !changed {
			continue
		}
		// 开发数据不保留旧动作计划；本次安全写入后只留下 F-008 命名空间、循环与测试选择。
		for key := range values {
			if strings.HasPrefix(key, "action-plan:") || strings.HasPrefix(key, "person:") {
				delete(values, key)
			}
		}
		stored := candidate.stored
		stored.PathID, stored.ActionValues, stored.ConfirmedNodeKeys = candidate.path.ID, values, confirmed
		stored.Revision, stored.NodeRevision, stored.ConfigVersion = stored.Revision+1, stored.NodeRevision+1, currentPathConfigVersion
		stored.IdempotencyKey = ""
		if !candidate.found {
			stored.FormStatus = initialStoredFormStatus(candidate.snapshot)
		}
		stored.Status = s.deriveStoredStatus(ctx, planID, candidate.path, candidate.snapshot, stored)
		if _, saveErr := s.configRepository.Save(ctx, stored, stored.Revision-1, s.now().UTC()); saveErr != nil {
			return model.PathConfigPresetApplyResult{}, mapPathConfigRepositoryError(saveErr)
		}
	}
	return result, nil
}

// pathConfigPresetCandidates 选择当前、已选或结构完全一致路径，并以同一目标快照投影每条路径。
func (s *PathConfigService) pathConfigPresetCandidates(ctx context.Context, planID, pathID uint64, scope string) ([]pathConfigPresetCandidate, error) {
	scope = strings.TrimSpace(scope)
	if scope != pathConfigPresetCurrent && scope != pathConfigPresetSelected && scope != pathConfigPresetCompatible {
		return nil, &PathConfigError{Kind: PathConfigErrorInvalidArgument, Message: "一键配置范围不正确"}
	}
	if err := s.validateConfigMutablePlan(ctx, planID); err != nil {
		return nil, err
	}
	current, err := s.ownedPath(ctx, planID, pathID)
	if err != nil {
		return nil, err
	}
	pathSummaries, err := s.pathRepository.List(ctx, planID)
	if err != nil {
		return nil, mapExecutionPathRepositoryError(err)
	}
	snapshot, err := s.readVerifiedSnapshot(ctx, planID)
	if err != nil {
		return nil, err
	}
	result := make([]pathConfigPresetCandidate, 0, len(pathSummaries))
	for _, summary := range pathSummaries {
		if scope == pathConfigPresetCurrent && summary.ID != current.ID {
			continue
		}
		stored, found, findErr := s.configRepository.FindByPath(ctx, summary.ID)
		if findErr != nil {
			return nil, mapPathConfigRepositoryError(findErr)
		}
		if scope == pathConfigPresetSelected && stored.ActionValues["f008:test-included"] != "true" {
			continue
		}
		path, pathErr := s.ownedPath(ctx, planID, summary.ID)
		if pathErr != nil {
			return nil, pathErr
		}
		if scope == pathConfigPresetCompatible && pathConfigStructuralSignature(path) != pathConfigStructuralSignature(current) {
			continue
		}
		owned, analysisErr := s.analyzeOwnedPath(ctx, planID, snapshot, path)
		if analysisErr != nil {
			return nil, analysisErr
		}
		configuration, validation, projectionErr := s.configAnalyzer.Analyze(owned.graph, snapshot.Tree, snapshot.FormFields, path, owned.pathAnalysis, snapshot.InstanceValues, stored.FieldValues, stored.ActionValues, found)
		if projectionErr != nil {
			return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前路径无法生成一键配置"}
		}
		result = append(result, pathConfigPresetCandidate{path: path, stored: stored, found: found, configuration: configuration, validation: validation, snapshot: snapshot})
	}
	return result, nil
}

// pathConfigPresetDecisions 为所有目标节点选择一个可用额外动作，并按范围尽量覆盖动作目录。
func pathConfigPresetDecisions(candidates []pathConfigPresetCandidate) [][]pathConfigPresetDecision {
	decisionsByCandidate := make([][]pathConfigPresetDecision, len(candidates))
	coverage := map[string]int{}
	slots := make([]pathConfigPresetActionSlot, 0)
	for candidateIndex, candidate := range candidates {
		result := make([]pathConfigPresetDecision, 0)
		for _, group := range candidate.configuration.Groups {
			for _, node := range group.Nodes {
				target, exists := candidate.validation.NodeTokens[node.Key]
				if !exists {
					continue
				}
				item := model.PathConfigPresetNodeItem{NodeKey: node.Key, NodeName: node.Name}
				if candidate.stored.ActionValues[analyzer.PathConfigActionConfigurationStorageKey(target.NodeID)] != "" {
					item.Status, item.Detail = "keep", "已保留人工配置"
					result = append(result, pathConfigPresetDecision{item: item})
					continue
				}
				if len(pathConfigPresetActionCatalog(node.ActionConfiguration.Catalog)) == 0 {
					item.Status, item.Detail = "skip", "当前节点没有可自动配置的动作"
					result = append(result, pathConfigPresetDecision{item: item})
					continue
				}
				decisionIndex := len(result)
				result = append(result, pathConfigPresetDecision{item: item})
				slots = append(slots, pathConfigPresetActionSlot{candidateIndex: candidateIndex, decisionIndex: decisionIndex, node: node, seed: target.NodeID})
			}
		}
		decisionsByCandidate[candidateIndex] = result
	}
	for _, candidate := range candidates {
		for _, group := range candidate.configuration.Groups {
			for _, node := range group.Nodes {
				for _, action := range node.ActionConfiguration.Actions {
					coverage[action.Kind]++
				}
			}
		}
	}
	for _, slot := range slots {
		decision := &decisionsByCandidate[slot.candidateIndex][slot.decisionIndex]
		action, ok := choosePathConfigPresetAction(pathConfigPresetActionCatalog(slot.node.ActionConfiguration.Catalog), coverage, slot.seed)
		if !ok {
			decision.item.Status, decision.item.Detail = "manual", "当前动作需要人工补充人员"
			continue
		}
		input, valid := pathConfigPresetActionInput(slot.node, action)
		if !valid {
			decision.item.Status, decision.item.Detail = "manual", "当前动作需要人工补充人员"
			continue
		}
		decision.item.Action, decision.item.Status, decision.item.Detail = action.Label, "write", "将为该节点添加一个随机动作"
		decision.input = input
		coverage[action.Kind]++
	}
	return decisionsByCandidate
}

// pathConfigPresetActionCatalog 只保留可自动配置的额外动作，系统同意/提交不进入预设。
func pathConfigPresetActionCatalog(catalog []model.PathConfigActionCatalogItem) []model.PathConfigActionCatalogItem {
	result := make([]model.PathConfigActionCatalogItem, 0, len(catalog))
	for _, item := range catalog {
		if item.Enabled && (!item.RequiresPerson || item.Person != nil) {
			result = append(result, item)
		}
	}
	return result
}

// choosePathConfigPresetAction 在覆盖次数最低的动作中稳定抽取一个，保证预览和应用结果一致。
func choosePathConfigPresetAction(catalog []model.PathConfigActionCatalogItem, coverage map[string]int, seed string) (model.PathConfigActionCatalogItem, bool) {
	if len(catalog) == 0 {
		return model.PathConfigActionCatalogItem{}, false
	}
	minimum := -1
	for _, item := range catalog {
		count := coverage[item.Kind]
		if minimum < 0 || count < minimum {
			minimum = count
		}
	}
	candidates := make([]model.PathConfigActionCatalogItem, 0, len(catalog))
	for _, item := range catalog {
		if coverage[item.Kind] == minimum {
			candidates = append(candidates, item)
		}
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(seed)))
	index := binary.BigEndian.Uint64(sum[:8]) % uint64(len(candidates))
	return candidates[index], true
}

// pathConfigPresetActionInput 生成一个动作次数为 1 的自动预设节点载荷。
func pathConfigPresetActionInput(node model.PathConfigNode, action model.PathConfigActionCatalogItem) (model.PathNodeSaveInput, bool) {
	input := model.PathNodeSaveInput{Persons: make([]model.PathConfigPersonStrategyInput, 0), Actions: []model.PathConfigConfiguredActionInput{{Key: "preset-" + node.Key, Kind: action.Kind, Count: 1}}}
	for _, person := range node.Persons {
		if !person.Editable {
			continue
		}
		strategy := strings.TrimSpace(person.Strategy)
		if strategy == "" {
			strategy = "random"
		}
		input.Persons = append(input.Persons, model.PathConfigPersonStrategyInput{Key: person.Key, Strategy: strategy, Seed: person.StrategySeed, Selected: append([]string(nil), person.Selected...)})
	}
	if !action.RequiresPerson {
		return input, true
	}
	if action.Person == nil || !action.Person.Editable {
		return model.PathNodeSaveInput{}, false
	}
	for _, option := range action.Person.Strategies {
		if option.Value == "random" {
			input.Actions[0].Person = &model.PathConfigPersonStrategyInput{Key: action.Person.Key, Strategy: "random", Seed: action.Person.StrategySeed}
			return input, true
		}
	}
	return model.PathNodeSaveInput{}, false
}

// pathConfigPresetDefaultPerson 仅接受公开模型中目标默认且当前合法的人员策略。
func pathConfigPresetDefaultPerson(node model.PathConfigNode, key string) (model.PathConfigPersonStrategyInput, bool) {
	for _, person := range node.Persons {
		if person.Key == key && person.Editable && len(person.DefaultSelected) > 0 {
			for _, strategy := range person.Strategies {
				if strategy.Value == "target_default" {
					return model.PathConfigPersonStrategyInput{Key: key, Strategy: "target_default", Seed: person.StrategySeed, Selected: append([]string(nil), person.DefaultSelected...)}, true
				}
			}
		}
	}
	return model.PathConfigPersonStrategyInput{}, false
}

// pathConfigStructuralSignature 使用完整分支选择序列限定可批量处理和未来循环复制的路径结构。
func pathConfigStructuralSignature(path model.ExecutionPath) string {
	values := make([]string, 0, len(path.Choices))
	for _, choice := range path.Choices {
		values = append(values, strings.TrimSpace(choice.RouteNodeID)+":"+strings.TrimSpace(choice.BranchID))
	}
	return strings.Join(values, "|")
}

// pathConfigPresetActionLabel 返回预览中可读的默认动作名称。
func pathConfigPresetActionLabel(kind string) string {
	if kind == "submit" {
		return "发起提交"
	}
	return "同意"
}
