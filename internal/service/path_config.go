package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// PathConfigErrorKind 是路径配置业务错误的稳定分类。
type PathConfigErrorKind string

const (
	PathConfigErrorInvalidArgument  PathConfigErrorKind = "invalid_argument"
	PathConfigErrorNotFound         PathConfigErrorKind = "not_found"
	PathConfigErrorLocked           PathConfigErrorKind = "locked"
	PathConfigErrorRevisionConflict PathConfigErrorKind = "revision_conflict"
	PathConfigErrorInvalid          PathConfigErrorKind = "invalid"
	PathConfigErrorStorage          PathConfigErrorKind = "storage"
)

// PathConfigError 携带业务错误分类与可公开的受影响项目，供 API 定位保存失败位置。
type PathConfigError struct {
	Kind     PathConfigErrorKind
	Message  string
	Affected []model.PathConfigAffectedItem
}

// Error 返回可映射为稳定 API 错误的人类可读说明。
func (e *PathConfigError) Error() string { return e.Message }

// IsPathConfigErrorKind 判断错误是否属于指定的路径配置业务边界。
func IsPathConfigErrorKind(err error, kind PathConfigErrorKind) bool {
	var configErr *PathConfigError
	return errors.As(err, &configErr) && configErr.Kind == kind
}

// PathConfigReader 读取同一真实流程树、当前入口、字段详情和实例现值。
type PathConfigReader interface {
	PathConfigurationSnapshot(context.Context, string, string, string) (target.PathConfigurationSnapshot, error)
}

// PathConfigAnalyzer 把已验证路径投影为配置 DTO 与保存校验索引。
type PathConfigAnalyzer interface {
	Analyze(model.FlowGraph, *target.FlowNodeTemplate, []target.FormFieldDetail, model.ExecutionPath, model.ExecutionPathAnalysis, map[string]any, map[string]map[string]string, map[string]string) (model.PathConfiguration, analyzer.PathConfigValidation, error)
}

// PathConfigService 组织计划身份、路径归属、目标重验、配置投影与事务保存。
type PathConfigService struct {
	plans            *PlanService
	target           PathConfigReader
	flowAnalyzer     FlowAnalyzer
	pathAnalyzer     ExecutionPathChoiceAnalyzer
	configAnalyzer   PathConfigAnalyzer
	pathRepository   repository.ExecutionPathRepository
	configRepository repository.PathConfigurationRepository
	now              func() time.Time
}

// NewPathConfigService 组装路径配置服务依赖。
func NewPathConfigService(plans *PlanService, targetReader PathConfigReader, flowAnalyzer FlowAnalyzer, pathAnalyzer ExecutionPathChoiceAnalyzer, configAnalyzer PathConfigAnalyzer, pathRepository repository.ExecutionPathRepository, configRepository repository.PathConfigurationRepository) *PathConfigService {
	return &PathConfigService{
		plans: plans, target: targetReader, flowAnalyzer: flowAnalyzer, pathAnalyzer: pathAnalyzer,
		configAnalyzer: configAnalyzer, pathRepository: pathRepository, configRepository: configRepository,
		now: time.Now,
	}
}

// Get 校验计划与路径归属后重读当前真实配置，并叠加本路径已保存的值与动作。
func (s *PathConfigService) Get(ctx context.Context, planID, pathID uint64) (model.PathConfiguration, error) {
	if planID == 0 || pathID == 0 {
		return model.PathConfiguration{}, &PathConfigError{Kind: PathConfigErrorInvalidArgument, Message: "计划或路径 ID 不正确"}
	}
	path, err := s.ownedPath(ctx, planID, pathID)
	if err != nil {
		return model.PathConfiguration{}, err
	}
	if err := s.validateConfigMutablePlan(ctx, planID); err != nil {
		return model.PathConfiguration{}, err
	}
	stored, found, err := s.configRepository.FindByPath(ctx, pathID)
	if err != nil {
		return model.PathConfiguration{}, mapPathConfigRepositoryError(err)
	}
	snapshot, err := s.readVerifiedSnapshot(ctx, planID)
	if err != nil {
		return model.PathConfiguration{}, err
	}
	analysis, err := s.analyzeOwnedPath(ctx, planID, snapshot, path)
	if err != nil {
		return model.PathConfiguration{}, err
	}
	if !found {
		stored = model.StoredPathConfig{PathID: pathID, FieldValues: map[string]map[string]string{}, ActionValues: map[string]string{}}
	}
	configuration, _, err := s.configAnalyzer.Analyze(
		analysis.graph, snapshot.Tree, snapshot.FormFields, path, analysis.pathAnalysis,
		snapshot.InstanceValues, stored.FieldValues, stored.ActionValues,
	)
	if err != nil {
		return model.PathConfiguration{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "执行路径配置无法投影，请重新核对路径"}
	}
	configuration.Revision = stored.Revision
	if found {
		configuration.Status = stored.Status
	}
	return configuration, nil
}

// Save 幂等键命中时直接返回已保存结果；新请求重读真实图并校验字段键、类型、选项、必填和动作后整份保存。
func (s *PathConfigService) Save(ctx context.Context, planID, pathID uint64, idempotencyKey string, revision uint64, fields []model.PathConfigFieldValue, actions []model.PathConfigActionValue) (model.PathConfigSaveResult, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if planID == 0 || pathID == 0 || !validUUID(idempotencyKey) {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalidArgument, Message: "计划、路径或保存标识不正确，请重试"}
	}
	path, err := s.ownedPath(ctx, planID, pathID)
	if err != nil {
		return model.PathConfigSaveResult{}, err
	}
	if err := s.validateConfigMutablePlan(ctx, planID); err != nil {
		return model.PathConfigSaveResult{}, err
	}
	// 幂等命中必须先于计划和目标读取返回，避免目标随后不可用或变化时把已成功保存误报为失败。
	if existing, found, err := s.configRepository.FindByPathAndKey(ctx, pathID, idempotencyKey); err != nil {
		return model.PathConfigSaveResult{}, mapPathConfigRepositoryError(err)
	} else if found {
		return model.PathConfigSaveResult{Path: model.PathConfigPath{SequenceNo: path.SequenceNo, Name: path.Name}, Revision: existing.Revision, Status: existing.Status}, nil
	}
	stored, found, err := s.configRepository.FindByPath(ctx, pathID)
	if err != nil {
		return model.PathConfigSaveResult{}, mapPathConfigRepositoryError(err)
	}
	expectedRevision := uint64(0)
	if found {
		expectedRevision = stored.Revision
	}
	if revision != expectedRevision {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorRevisionConflict, Message: "配置已被其他操作更新，请刷新后重试"}
	}
	snapshot, err := s.readVerifiedSnapshot(ctx, planID)
	if err != nil {
		return model.PathConfigSaveResult{}, err
	}
	analysis, err := s.analyzeOwnedPath(ctx, planID, snapshot, path)
	if err != nil {
		return model.PathConfigSaveResult{}, err
	}
	_, validation, err := s.configAnalyzer.Analyze(
		analysis.graph, snapshot.Tree, snapshot.FormFields, path, analysis.pathAnalysis,
		snapshot.InstanceValues, map[string]map[string]string{}, map[string]string{},
	)
	if err != nil {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "执行路径配置无法投影，请重新核对路径"}
	}
	fieldValues, actionValues, err := validatePathConfigSubmission(validation, fields, actions)
	if err != nil {
		return model.PathConfigSaveResult{}, err
	}
	nextRevision := expectedRevision + 1
	saved, err := s.configRepository.Save(ctx, model.StoredPathConfig{
		PathID: pathID, Revision: nextRevision, IdempotencyKey: idempotencyKey, Status: "configured",
		FieldValues: fieldValues, ActionValues: actionValues,
	}, expectedRevision, s.now().UTC())
	if err != nil {
		return model.PathConfigSaveResult{}, mapPathConfigRepositoryError(err)
	}
	return model.PathConfigSaveResult{
		Path:     model.PathConfigPath{SequenceNo: path.SequenceNo, Name: path.Name},
		Revision: saved.Revision, Status: saved.Status,
	}, nil
}

// ownedPath 只从计划内列表查找路径，避免通过路径 ID 探测其他计划的数据。
func (s *PathConfigService) ownedPath(ctx context.Context, planID, pathID uint64) (model.ExecutionPath, error) {
	if _, err := s.plans.Get(ctx, planID); err != nil {
		return model.ExecutionPath{}, err
	}
	paths, err := s.pathRepository.List(ctx, planID)
	if err != nil {
		return model.ExecutionPath{}, mapExecutionPathRepositoryError(err)
	}
	for _, candidate := range paths {
		if candidate.ID == pathID && candidate.PlanID == planID {
			return candidate, nil
		}
	}
	return model.ExecutionPath{}, &PathConfigError{Kind: PathConfigErrorNotFound, Message: "执行路径不存在"}
}

// validateConfigMutablePlan 只允许仍处于待配置状态的计划继续保存配置。
func (s *PathConfigService) validateConfigMutablePlan(ctx context.Context, planID uint64) error {
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return err
	}
	if plan.Status != model.PlanStatusPendingConfiguration {
		return &PathConfigError{Kind: PathConfigErrorLocked, Message: "计划已经不能修改路径配置"}
	}
	return nil
}

// readVerifiedSnapshot 按计划持久化身份重读当前真实配置，浏览器不能覆盖来源或目标。
func (s *PathConfigService) readVerifiedSnapshot(ctx context.Context, planID uint64) (target.PathConfigurationSnapshot, error) {
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return target.PathConfigurationSnapshot{}, err
	}
	return s.target.PathConfigurationSnapshot(ctx, plan.Account, plan.FlowSource, plan.TargetObjectID)
}

// ownedPathAnalysis 是当前真实图与路径分析的组合结果。
type ownedPathAnalysis struct {
	graph        model.FlowGraph
	pathAnalysis model.ExecutionPathAnalysis
}

// analyzeOwnedPath 生成当前真实图并校验已保存选择仍然完整可投影。
func (s *PathConfigService) analyzeOwnedPath(ctx context.Context, planID uint64, snapshot target.PathConfigurationSnapshot, path model.ExecutionPath) (ownedPathAnalysis, error) {
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return ownedPathAnalysis{}, err
	}
	nodes, edges, warnings, err := s.flowAnalyzer.Analyze(snapshot.Tree)
	if err != nil {
		if errors.Is(err, analyzer.ErrFlowStructureInvalid) {
			return ownedPathAnalysis{}, err
		}
		return ownedPathAnalysis{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前流程结构无法生成配置"}
	}
	entries, err := validateEntryNodeIDs(snapshot.EntryNodeIDs, nodes)
	if err != nil {
		return ownedPathAnalysis{}, err
	}
	graph := model.FlowGraph{
		PlanID: plan.ID, TargetName: plan.TargetObjectName, FlowSource: plan.FlowSource,
		EntryNodeIDs: entries, Nodes: nodes, Edges: edges, Warnings: warnings,
	}
	pathAnalysis, err := s.pathAnalyzer.Analyze(graph, path.Choices)
	if err != nil || !pathAnalysis.Complete {
		return ownedPathAnalysis{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "执行路径选择不完整或已失效"}
	}
	return ownedPathAnalysis{graph: graph, pathAnalysis: pathAnalysis}, nil
}

// validatePathConfigSubmission 把浏览器回写的字段与动作收敛为按真实节点键控的存储映射。
func validatePathConfigSubmission(validation analyzer.PathConfigValidation, fields []model.PathConfigFieldValue, actions []model.PathConfigActionValue) (map[string]map[string]string, map[string]string, error) {
	if len(fields) > 500 || len(actions) > 500 {
		return nil, nil, &PathConfigError{Kind: PathConfigErrorInvalidArgument, Message: "路径配置项目数量过多"}
	}
	seenFields := make(map[string]bool, len(fields))
	fieldValues := make(map[string]map[string]string)
	affected := make([]model.PathConfigAffectedItem, 0, len(fields))
	for _, field := range fields {
		key := strings.TrimSpace(field.Key)
		if key == "" || seenFields[key] {
			affected = append(affected, model.PathConfigAffectedItem{Kind: "field", Name: "字段", Reason: "字段回写键重复或为空"})
			continue
		}
		seenFields[key] = true
		target, exists := validation.FieldTokens[key]
		if !exists {
			// 不透明键无法对应当前真实节点时整次拒绝，避免浏览器用过期结构写库。
			affected = append(affected, model.PathConfigAffectedItem{Kind: "field", Name: "字段", Reason: "字段已不属于当前路径或目标结构已变化"})
			continue
		}
		if reason := validateConfigFieldValue(target, field.Value); reason != "" {
			affected = append(affected, model.PathConfigAffectedItem{Kind: "field", Name: target.Name, Reason: reason})
			continue
		}
		if fieldValues[target.NodeID] == nil {
			fieldValues[target.NodeID] = make(map[string]string)
		}
		fieldValues[target.NodeID][target.FieldKey] = field.Value
	}
	for key, target := range validation.FieldTokens {
		if !seenFields[key] {
			affected = append(affected, model.PathConfigAffectedItem{Kind: "field", Name: target.Name, Reason: "缺少该字段的保存值，需要重新选择"})
		}
	}
	if len(affected) > 0 {
		return nil, nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "路径配置不完整或已失效，请重新选择", Affected: affected}
	}

	seenActions := make(map[string]bool, len(actions))
	actionValues := make(map[string]string)
	for _, action := range actions {
		key := strings.TrimSpace(action.Key)
		actionValue := strings.TrimSpace(action.Action)
		if key == "" || seenActions[key] {
			return nil, nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "路径动作配置不正确，请重新选择", Affected: []model.PathConfigAffectedItem{{Kind: "action", Name: "动作", Reason: "动作回写键重复或为空"}}}
		}
		seenActions[key] = true
		target, exists := validation.ActionTokens[key]
		if !exists {
			return nil, nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "路径动作配置已失效，请重新选择", Affected: []model.PathConfigAffectedItem{{Kind: "action", Name: "动作", Reason: "动作已不属于当前路径或目标结构已变化"}}}
		}
		if !validPathConfigAction(target.Kind, actionValue) {
			return nil, nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "路径动作配置不正确，请重新选择", Affected: []model.PathConfigAffectedItem{{Kind: "action", Name: target.Name, Reason: "动作候选不合法"}}}
		}
		if target.Kind != "submit" {
			actionValues[target.NodeID] = actionValue
		}
	}
	// 未显式提交的审批/协同动作按默认推荐同意保存，发起节点固定提交无需落库。
	for key, target := range validation.ActionTokens {
		if !seenActions[key] && target.Kind == "agree_disagree" {
			actionValues[target.NodeID] = "agree"
		}
	}
	return fieldValues, actionValues, nil
}

// validateConfigFieldValue 按字段类型校验必填、类型、选项与值边界，返回可公开的中文原因。
func validateConfigFieldValue(target analyzer.PathConfigFieldTarget, raw string) string {
	if utf8.RuneCountInString(raw) > 20000 {
		return "字段值过长"
	}
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return "字段值格式不正确"
	}
	switch target.Type {
	case analyzer.PathConfigTypeText, analyzer.PathConfigTypeDateTime:
		text, ok := parsed.(string)
		if !ok {
			return "字段值必须是文本"
		}
		if target.Required && strings.TrimSpace(text) == "" {
			return "必填字段不能为空"
		}
	case analyzer.PathConfigTypeNumber:
		_, empty, ok := parseConfigNumber(parsed)
		if !ok {
			return "字段值必须是数字"
		}
		if target.Required && empty {
			return "必填字段不能为空"
		}
	case analyzer.PathConfigTypeSingleSelect:
		text, ok := parsed.(string)
		if !ok {
			return "单选字段值不正确"
		}
		if target.Required && strings.TrimSpace(text) == "" {
			return "必填字段不能为空"
		}
		if text != "" && !configOptionExists(target.Options, text) {
			return "选项已不在当前目标候选中"
		}
	case analyzer.PathConfigTypeMultiSelect:
		items, ok := parsed.([]any)
		if !ok {
			return "多选字段值不正确"
		}
		if target.Required && len(items) == 0 {
			return "必填字段不能为空"
		}
		for _, item := range items {
			text, ok := item.(string)
			if !ok || !configOptionExists(target.Options, text) {
				return "多选值包含已不在当前目标候选中的选项"
			}
		}
	case analyzer.PathConfigTypeSwitch:
		if _, ok := parsed.(bool); !ok {
			return "开关字段值必须是布尔值"
		}
	default:
		return "字段类型暂不支持保存"
	}
	return ""
}

// parseConfigNumber 解析数字字段值；空字符串允许但标记为空。
func parseConfigNumber(parsed any) (json.Number, bool, bool) {
	switch typed := parsed.(type) {
	case float64:
		return json.Number(strconv.FormatFloat(typed, 'f', -1, 64)), false, true
	case string:
		value := strings.TrimSpace(typed)
		if value == "" {
			return json.Number(""), true, true
		}
		number := json.Number(value)
		if _, err := number.Float64(); err != nil {
			return json.Number(""), false, false
		}
		return number, false, true
	default:
		return json.Number(""), false, false
	}
}

// configOptionExists 判断单选/多选值是否仍属于当前真实选项。
func configOptionExists(options []model.PathConfigOption, value string) bool {
	for _, option := range options {
		if option.Value == value {
			return true
		}
	}
	return false
}

// validPathConfigAction 校验动作值属于该动作类型的固定候选。
func validPathConfigAction(kind, value string) bool {
	switch kind {
	case "submit":
		return value == "submit"
	case "agree_disagree":
		return value == "agree" || value == "disagree"
	default:
		return false
	}
}

// mapPathConfigRepositoryError 把配置仓储错误收敛为稳定业务错误。
func mapPathConfigRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrPathConfigConflict):
		return &PathConfigError{Kind: PathConfigErrorRevisionConflict, Message: "配置已被其他操作更新，请刷新后重试"}
	case errors.Is(err, repository.ErrPathConfigDataInvalid):
		return &PathConfigError{Kind: PathConfigErrorStorage, Message: "路径配置数据异常，请重试"}
	case errors.Is(err, repository.ErrPlanNotFound):
		return &PathConfigError{Kind: PathConfigErrorNotFound, Message: "计划不存在"}
	default:
		return &PathConfigError{Kind: PathConfigErrorStorage, Message: "路径配置存储暂不可用，请重试"}
	}
}
