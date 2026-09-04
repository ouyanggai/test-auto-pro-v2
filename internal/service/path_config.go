package service

import (
	"context"
	"encoding/json"
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

// PathConfigAnalyzer 把已验证路径投影为配置 DTO 与保存校验索引。
type PathConfigAnalyzer interface {
	Analyze(model.FlowGraph, *target.FlowNodeTemplate, []target.FormFieldDetail, model.ExecutionPath, model.ExecutionPathAnalysis, map[string]any, map[string]map[string]string, map[string]string, ...bool) (model.PathConfiguration, analyzer.PathConfigValidation, error)
}

// PathConfigService 组织计划身份、路径归属、目标重读和 F012 配置投影。
type PathConfigService struct {
	plans              *PlanService
	target             PathConfigReader
	flowAnalyzer       FlowAnalyzer
	pathAnalyzer       ExecutionPathChoiceAnalyzer
	configAnalyzer     PathConfigAnalyzer
	pathRepository     repository.ExecutionPathRepository
	historyStore       repository.HistoryReplayStore
	historyConfigStore repository.HistoryPathConfigStore
	companyDirectory   PathDataCompanyDirectory
	now                func() time.Time
}

// NewPathConfigService 组装 F012 路径配置服务依赖；持久化统一由历史路径配置仓储负责。
func NewPathConfigService(plans *PlanService, targetReader PathConfigReader, flowAnalyzer FlowAnalyzer, pathAnalyzer ExecutionPathChoiceAnalyzer, configAnalyzer PathConfigAnalyzer, pathRepository repository.ExecutionPathRepository) *PathConfigService {
	return &PathConfigService{plans: plans, target: targetReader, flowAnalyzer: flowAnalyzer, pathAnalyzer: pathAnalyzer, configAnalyzer: configAnalyzer, pathRepository: pathRepository, now: time.Now}
}

// Get 校验计划和路径归属后重读真实流程，并叠加 F012 独立动作与人员列。
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
	snapshot, err := s.readVerifiedSnapshot(ctx, planID)
	if err != nil {
		return model.PathConfiguration{}, err
	}
	analysis, err := s.analyzeOwnedPath(ctx, planID, snapshot, path)
	if err != nil {
		return model.PathConfiguration{}, err
	}
	configuration, _, err := s.configAnalyzer.Analyze(
		analysis.graph, snapshot.Tree, snapshot.FormFields, path, analysis.pathAnalysis,
		snapshot.InstanceValues, map[string]map[string]string{}, map[string]string{}, false,
	)
	if err != nil {
		return model.PathConfiguration{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "执行路径配置无法投影，请重新核对路径"}
	}
	var stored repository.HistoryPathConfigRecord
	if s.historyConfigStore != nil {
		stored, _, err = s.historyConfigStore.GetPathConfig(ctx, pathID)
		if err != nil {
			return model.PathConfiguration{}, mapHistoryWorkspaceStoreError(err)
		}
	}
	configuration.Revision, configuration.NodeRevision = stored.Revision, stored.NodeRevision
	applyConfirmedNodeState(&configuration, decodeConfirmedNodeKeys(stored.ConfirmedNodeKeys))
	if err := s.applyHistoryActionProjection(ctx, pathID, &configuration); err != nil {
		return model.PathConfiguration{}, err
	}
	configuration.Status = derivePathConfigurationStatus(configuration)
	return configuration, nil
}

// RuntimeSession 校验计划与路径归属后返回当前账号的短期复制 runtime 会话。
func (s *PathConfigService) RuntimeSession(ctx context.Context, planID, pathID uint64) (model.PathFormRuntimeSession, error) {
	if _, err := s.ownedPath(ctx, planID, pathID); err != nil {
		return model.PathFormRuntimeSession{}, err
	}
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return model.PathFormRuntimeSession{}, err
	}
	reader, ok := s.target.(interface {
		FormRuntimeSession(context.Context, string) (target.FormRuntimeSession, error)
	})
	if !ok {
		return model.PathFormRuntimeSession{}, &PathConfigError{Kind: PathConfigErrorStorage, Message: "表单运行时会话暂不可用"}
	}
	active, err := reader.FormRuntimeSession(ctx, plan.Account)
	if err != nil {
		return model.PathFormRuntimeSession{}, err
	}
	return model.PathFormRuntimeSession{
		SID: active.SID, BaseURL: active.BaseURL, AccountName: active.AccountName,
		UserID: active.UserID, CompanyID: active.CompanyID, CustomerCode: active.CustomerCode, CompanyName: active.CompanyName,
		DepartmentID: active.DepartmentID, DepartmentName: active.DepartmentName,
	}, nil
}

// ownedPath 先确认计划存在，再读取完整路径选择，防止跨计划访问配置。
func (s *PathConfigService) ownedPath(ctx context.Context, planID, pathID uint64) (model.ExecutionPath, error) {
	if s.plans == nil || s.pathRepository == nil {
		return model.ExecutionPath{}, &PathConfigError{Kind: PathConfigErrorStorage, Message: "执行路径存储暂不可用"}
	}
	if _, err := s.plans.Get(ctx, planID); err != nil {
		return model.ExecutionPath{}, mapExecutionPathRepositoryError(err)
	}
	path, err := s.pathRepository.Get(ctx, planID, pathID)
	if err != nil {
		return model.ExecutionPath{}, mapExecutionPathRepositoryError(err)
	}
	return path, nil
}

// validateConfigMutablePlan 只允许尚未运行的计划继续修改配置。
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

// readVerifiedSnapshot 按计划持久化的目标身份重读流程，不接受浏览器覆盖来源或目标对象。
func (s *PathConfigService) readVerifiedSnapshot(ctx context.Context, planID uint64) (target.PathConfigurationSnapshot, error) {
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return target.PathConfigurationSnapshot{}, err
	}
	if s.target == nil {
		return target.PathConfigurationSnapshot{}, &PathConfigError{Kind: PathConfigErrorStorage, Message: "目标配置读取服务暂不可用"}
	}
	snapshot, err := s.target.PathConfigurationSnapshot(ctx, plan.Account, plan.FlowSource, plan.TargetObjectID)
	if err != nil {
		return target.PathConfigurationSnapshot{}, err
	}
	return snapshot, nil
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
	if s.flowAnalyzer == nil || s.pathAnalyzer == nil || s.configAnalyzer == nil {
		return ownedPathAnalysis{}, &PathConfigError{Kind: PathConfigErrorStorage, Message: "路径配置分析服务暂不可用"}
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

// decodeConfirmedNodeKeys 解码 F012 节点确认列，损坏内容按未确认处理。
func decodeConfirmedNodeKeys(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return []string{}
	}
	return values
}

// derivePathConfigurationStatus 只按节点人员与动作确认状态派生路径配置状态。
func derivePathConfigurationStatus(configuration model.PathConfiguration) string {
	for _, group := range configuration.Groups {
		for _, node := range group.Nodes {
			if node.Status == "affected" {
				return "affected"
			}
		}
	}
	if configuration.Progress.Pending == 0 {
		return "configured"
	}
	if configuration.Progress.Completed > 0 {
		return "partial"
	}
	return "pending"
}

// applyConfirmedNodeState 用 F012 独立确认事实覆盖节点配置状态，避免“有记录即完成”。
func applyConfirmedNodeState(configuration *model.PathConfiguration, confirmedKeys []string) {
	confirmed := make(map[string]bool, len(confirmedKeys))
	for _, key := range confirmedKeys {
		confirmed[strings.TrimSpace(key)] = true
	}
	for groupIndex := range configuration.Groups {
		for nodeIndex := range configuration.Groups[groupIndex].Nodes {
			node := &configuration.Groups[groupIndex].Nodes[nodeIndex]
			if node.Status == "affected" || node.Status == "partial" || node.LineBlocked || node.Status == "not_required" || node.Status == "runtime" {
				continue
			}
			requiresSave := len(node.Persons) > 0 || len(node.ActionConfiguration.Catalog) > 0
			if !requiresSave {
				continue
			}
			if confirmed[node.Key] {
				node.Status, node.StatusName = "configured", "已完成"
			} else {
				node.Status, node.StatusName = "pending", "待配置"
			}
		}
	}
	configuration.Progress, configuration.NextNodeKey = summarizeConfigurationProgress(configuration.Groups)
}

// summarizeConfigurationProgress 重新统计逐节点确认后的权威进度。
func summarizeConfigurationProgress(groups []model.PathConfigGroup) (model.PathConfigProgress, string) {
	progress := model.PathConfigProgress{}
	next := ""
	for _, group := range groups {
		for _, node := range group.Nodes {
			if node.Status == "not_required" || node.Status == "runtime" {
				continue
			}
			progress.Total++
			if node.Status == "configured" {
				progress.Completed++
			} else {
				progress.Pending++
				if next == "" {
					next = node.Key
				}
			}
		}
	}
	return progress, next
}
