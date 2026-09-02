package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// PathConfigErrorKind 是路径配置业务错误的稳定分类。
type PathConfigErrorKind string

const (
	PathConfigErrorInvalidArgument   PathConfigErrorKind = "invalid_argument"
	PathConfigErrorNotFound          PathConfigErrorKind = "not_found"
	PathConfigErrorLocked            PathConfigErrorKind = "locked"
	PathConfigErrorRevisionConflict  PathConfigErrorKind = "revision_conflict"
	PathConfigErrorInvalid           PathConfigErrorKind = "invalid"
	PathConfigErrorRouteConfirmation PathConfigErrorKind = "route_confirmation"
	PathConfigErrorStorage           PathConfigErrorKind = "storage"
)

// PathConfigError 携带可公开的业务错误与受影响项。
type PathConfigError struct {
	Kind              PathConfigErrorKind
	Message           string
	Affected          []model.PathConfigAffectedItem
	RouteChange       *model.PathConfigurationRouteChange
	ConfirmationToken string
}

// Error 返回可映射为稳定 API 错误的人类可读说明。
func (e *PathConfigError) Error() string { return e.Message }

// IsPathConfigErrorKind 判断错误是否属于指定路径配置边界。
func IsPathConfigErrorKind(err error, kind PathConfigErrorKind) bool {
	var configErr *PathConfigError
	return errors.As(err, &configErr) && configErr.Kind == kind
}

// PathConfigReader 读取同一真实流程树、当前入口、字段详情和实例现值。
type PathConfigReader interface {
	PathConfigurationSnapshot(context.Context, string, string, string) (target.PathConfigurationSnapshot, error)
}

// PathConfigTemplateRuleReader 只读取本地已分析的模板规则，路径配置不得在用户操作中重新扫描宿主源码。
type PathConfigTemplateRuleReader interface {
	GetByFlowCode(context.Context, string) (model.TemplateRuleCatalogItem, bool, error)
}

// PathConfigAnalyzer 把已验证路径投影为配置 DTO 与保存校验索引。
type PathConfigAnalyzer interface {
	Analyze(model.FlowGraph, *target.FlowNodeTemplate, []target.FormFieldDetail, model.ExecutionPath, model.ExecutionPathAnalysis, map[string]any, map[string]map[string]string, map[string]string, ...bool) (model.PathConfiguration, analyzer.PathConfigValidation, error)
}

// PathConfigService 组织计划身份、路径归属、目标重验、配置投影与事务保存。
type PathConfigService struct {
	plans              *PlanService
	target             PathConfigReader
	flowAnalyzer       FlowAnalyzer
	pathAnalyzer       ExecutionPathChoiceAnalyzer
	configAnalyzer     PathConfigAnalyzer
	pathRepository     repository.ExecutionPathRepository
	configRepository   repository.PathConfigurationRepository
	historyStore       repository.HistoryReplayStore
	historyConfigStore repository.HistoryPathConfigStore
	templateRules      PathConfigTemplateRuleReader
	candidateCache     *ComponentCandidateCache
	now                func() time.Time
}

// NewPathConfigService 组装路径配置服务依赖。
func NewPathConfigService(plans *PlanService, targetReader PathConfigReader, flowAnalyzer FlowAnalyzer, pathAnalyzer ExecutionPathChoiceAnalyzer, configAnalyzer PathConfigAnalyzer, pathRepository repository.ExecutionPathRepository, configRepository repository.PathConfigurationRepository) *PathConfigService {
	return &PathConfigService{plans: plans, target: targetReader, flowAnalyzer: flowAnalyzer, pathAnalyzer: pathAnalyzer, configAnalyzer: configAnalyzer, pathRepository: pathRepository, configRepository: configRepository, now: time.Now}
}

// SetTemplateRuleCatalog 注入已持久化的规则目录；未同步的 Vue 页面必须保留为需处理而不是临时猜测。
func (s *PathConfigService) SetTemplateRuleCatalog(catalog PathConfigTemplateRuleReader) {
	s.templateRules = catalog
}

// SetComponentCandidateCache 注入组件候选缓存服务。
func (s *PathConfigService) SetComponentCandidateCache(cache *ComponentCandidateCache) {
	s.candidateCache = cache
}

// Get 校验计划与路径归属后重读当前真实配置，并叠加 F-008 工具侧配置。
func (s *PathConfigService) Get(ctx context.Context, planID, pathID uint64) (model.PathConfiguration, error) {
	if planID == 0 || pathID == 0 {
		return model.PathConfiguration{}, &PathConfigError{Kind: PathConfigErrorInvalidArgument, Message: "计划或路径 ID 不正确"}
	}
	path, err := s.ownedPath(ctx, planID, pathID)
	if err != nil {
		return model.PathConfiguration{}, err
	}
	if err = s.validateConfigMutablePlan(ctx, planID); err != nil {
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
	configuration, _, err := s.configAnalyzer.Analyze(analysis.graph, snapshot.Tree, snapshot.FormFields, path, analysis.pathAnalysis, snapshot.InstanceValues, stored.FieldValues, stored.ActionValues, found)
	if err != nil {
		return model.PathConfiguration{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "执行路径配置无法投影，请重新核对路径"}
	}
	configuration.Revision, configuration.NodeRevision = stored.Revision, stored.NodeRevision
	applyConfirmedNodeState(&configuration, stored.ConfirmedNodeKeys)
	configuration.ActionCycles = projectPathConfigActionCycles(stored.ActionValues, configuration)
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return model.PathConfiguration{}, err
	}
	configuration.Form = projectPathForm(plan.FlowSource, snapshot, analysis.pathAnalysis, path.Choices, stored, found)
	configuration.Status = derivePathConfigurationStatus(configuration)
	configuration.Preparation = model.PathConfigPreparation{PreparedNodes: configuration.Progress.Completed, PendingItems: configuration.Progress.Pending, Included: stored.ActionValues["f008:test-included"] == "true"}
	return configuration, nil
}

// ownedPath 先确认计划存在，再按路径 ID 读取完整 choices，列表摘要不能参与线路完整性校验。
func (s *PathConfigService) ownedPath(ctx context.Context, planID, pathID uint64) (model.ExecutionPath, error) {
	if _, err := s.plans.Get(ctx, planID); err != nil {
		return model.ExecutionPath{}, err
	}
	path, err := s.pathRepository.Get(ctx, planID, pathID)
	if err != nil {
		return model.ExecutionPath{}, mapExecutionPathRepositoryError(err)
	}
	return path, nil
}

// validateConfigMutablePlan 只允许仍处于未运行状态的计划继续保存配置。
func (s *PathConfigService) validateConfigMutablePlan(ctx context.Context, planID uint64) error {
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return err
	}
	if plan.Status != model.PlanStatusNotStarted {
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
	snapshot, err := s.target.PathConfigurationSnapshot(ctx, plan.Account, plan.FlowSource, plan.TargetObjectID)
	if err != nil {
		return target.PathConfigurationSnapshot{}, err
	}
	return s.applyStoredTemplateRules(ctx, plan.Account, plan.FlowSource, plan.TargetObjectID, snapshot)
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
	graph := model.FlowGraph{PlanID: plan.ID, TargetName: plan.TargetObjectName, FlowSource: plan.FlowSource, EntryNodeIDs: entries, Nodes: nodes, Edges: edges, Warnings: warnings}
	pathAnalysis, err := s.pathAnalyzer.Analyze(graph, path.Choices)
	if err != nil || !pathAnalysis.Complete {
		return ownedPathAnalysis{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前已保存路径与真实流程不一致，请先编辑路径"}
	}
	return ownedPathAnalysis{graph: graph, pathAnalysis: pathAnalysis}, nil
}

// validatePathConfigNodeSubmission 只校验并编码一个节点的人员策略与 F-008 动作列表，不接受跨节点覆盖。
func validatePathConfigNodeSubmission(target analyzer.PathConfigNodeTarget, input model.PathNodeSaveInput) (map[string]string, error) {
	if len(target.Blockers) > 0 {
		return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前节点仍有无法安全确认的配置项", Affected: target.Blockers}
	}
	values := map[string]string{}
	if target.Person != nil {
		if len(input.Persons) != 1 || strings.TrimSpace(input.Persons[0].Key) != target.Person.Key {
			return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前节点人员策略不完整", Affected: []model.PathConfigAffectedItem{{Kind: "person", Name: target.Person.Name, Reason: "缺少或重复提交人员策略"}}}
		}
		encoded, reason := analyzer.EncodePathConfigPersonStrategy(*target.Person, input.Persons[0])
		if reason != "" {
			return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前节点人员策略不合法", Affected: []model.PathConfigAffectedItem{{Kind: "person", Name: target.Person.Name, Reason: reason}}}
		}
		values[analyzer.PathConfigPersonPlanStorageKey(target.NodeID)] = encoded
	} else if len(input.Persons) > 0 {
		return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前节点不允许配置处理人员"}
	}
	encoded, _, reason := analyzer.EncodePathConfigActions(target, input.Actions)
	if reason != "" {
		return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前节点动作配置不合法", Affected: []model.PathConfigAffectedItem{{Kind: "action", Name: target.Name, Reason: reason}}}
	}
	values[analyzer.PathConfigActionConfigurationStorageKey(target.NodeID)] = encoded
	return values, nil
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
