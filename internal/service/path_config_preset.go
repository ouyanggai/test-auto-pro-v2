package service

import (
	"context"
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
	item     model.PathConfigPresetNodeItem
	input    model.PathNodeSaveInput
	writable bool
}

// PreviewPreset 预览安全默认项在指定路径范围的逐节点结果，绝不写入工具侧或目标平台。
func (s *PathConfigService) PreviewPreset(ctx context.Context, planID, pathID uint64, scope string) (model.PathConfigPresetPreview, error) {
	candidates, err := s.pathConfigPresetCandidates(ctx, planID, pathID, scope)
	if err != nil {
		return model.PathConfigPresetPreview{}, err
	}
	preview := model.PathConfigPresetPreview{Scope: scope, Paths: make([]model.PathConfigPresetPath, 0, len(candidates))}
	for _, candidate := range candidates {
		items := make([]model.PathConfigPresetNodeItem, 0)
		for _, decision := range pathConfigPresetDecisions(candidate) {
			items = append(items, decision.item)
		}
		preview.Paths = append(preview.Paths, model.PathConfigPresetPath{Path: model.PathConfigPath{SequenceNo: candidate.path.SequenceNo, Name: candidate.path.Name}, Items: items})
	}
	return preview, nil
}

// ApplyPreset 写入预览中唯一安全的默认项；已保存动作、人员和循环都保持原样。
func (s *PathConfigService) ApplyPreset(ctx context.Context, planID, pathID uint64, scope string) (model.PathConfigPresetApplyResult, error) {
	candidates, err := s.pathConfigPresetCandidates(ctx, planID, pathID, scope)
	if err != nil {
		return model.PathConfigPresetApplyResult{}, err
	}
	result := model.PathConfigPresetApplyResult{Preview: model.PathConfigPresetPreview{Scope: scope, Paths: make([]model.PathConfigPresetPath, 0, len(candidates))}}
	for _, candidate := range candidates {
		decisions := pathConfigPresetDecisions(candidate)
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
		return nil, &PathConfigError{Kind: PathConfigErrorInvalidArgument, Message: "一键预设范围不正确"}
	}
	if err := s.validateConfigMutablePlan(ctx, planID); err != nil {
		return nil, err
	}
	current, err := s.ownedPath(ctx, planID, pathID)
	if err != nil {
		return nil, err
	}
	paths, err := s.pathRepository.List(ctx, planID)
	if err != nil {
		return nil, mapExecutionPathRepositoryError(err)
	}
	snapshot, err := s.readVerifiedSnapshot(ctx, planID)
	if err != nil {
		return nil, err
	}
	result := make([]pathConfigPresetCandidate, 0, len(paths))
	for _, path := range paths {
		if scope == pathConfigPresetCurrent && path.ID != current.ID {
			continue
		}
		if scope == pathConfigPresetCompatible && pathConfigStructuralSignature(path) != pathConfigStructuralSignature(current) {
			continue
		}
		stored, found, findErr := s.configRepository.FindByPath(ctx, path.ID)
		if findErr != nil {
			return nil, mapPathConfigRepositoryError(findErr)
		}
		if scope == pathConfigPresetSelected && stored.ActionValues["f008:test-included"] != "true" {
			continue
		}
		owned, analysisErr := s.analyzeOwnedPath(ctx, planID, snapshot, path)
		if analysisErr != nil {
			return nil, analysisErr
		}
		configuration, validation, projectionErr := s.configAnalyzer.Analyze(owned.graph, snapshot.Tree, snapshot.FormFields, path, owned.pathAnalysis, snapshot.InstanceValues, stored.FieldValues, stored.ActionValues, found)
		if projectionErr != nil {
			return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前路径无法生成一键预设"}
		}
		result = append(result, pathConfigPresetCandidate{path: path, stored: stored, found: found, configuration: configuration, validation: validation, snapshot: snapshot})
	}
	return result, nil
}

// pathConfigPresetDecisions 只为提交与同意生成默认项；人员没有目标默认时明确交给人工处理。
func pathConfigPresetDecisions(candidate pathConfigPresetCandidate) []pathConfigPresetDecision {
	result := make([]pathConfigPresetDecision, 0)
	for _, group := range candidate.configuration.Groups {
		for _, node := range group.Nodes {
			target, exists := candidate.validation.NodeTokens[node.Key]
			if !exists {
				continue
			}
			item := model.PathConfigPresetNodeItem{NodeKey: node.Key, NodeName: node.Name}
			if candidate.stored.ActionValues[analyzer.PathConfigActionConfigurationStorageKey(target.NodeID)] != "" || candidate.stored.ActionValues[analyzer.PathConfigPersonPlanStorageKey(target.NodeID)] != "" {
				item.Status, item.Detail = "keep", "已保留人工配置"
				result = append(result, pathConfigPresetDecision{item: item})
				continue
			}
			kind := ""
			switch node.Kind {
			case "start":
				kind = "submit"
			case "common", "synergy":
				kind = "approve_pass"
			}
			if kind == "" || !target.ActionKinds[kind] {
				item.Status, item.Detail = "skip", "当前节点没有安全默认动作"
				result = append(result, pathConfigPresetDecision{item: item})
				continue
			}
			item.Action = pathConfigPresetActionLabel(kind)
			input := model.PathNodeSaveInput{Actions: []model.PathConfigConfiguredActionInput{{Key: "preset", Kind: kind, Count: 1}}}
			if target.Person != nil {
				person, ok := pathConfigPresetDefaultPerson(node, target.Person.Key)
				if !ok {
					item.Status, item.Detail = "manual", "没有可证明的目标默认处理人"
					result = append(result, pathConfigPresetDecision{item: item})
					continue
				}
				input.Persons = []model.PathConfigPersonStrategyInput{person}
			}
			if len(target.Blockers) > 0 {
				item.Status, item.Detail = "manual", "当前目标规则需要人工处理"
				result = append(result, pathConfigPresetDecision{item: item})
				continue
			}
			item.Status, item.Detail = "write", "将写入安全默认动作"
			result = append(result, pathConfigPresetDecision{item: item, input: input, writable: true})
		}
	}
	return result
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
