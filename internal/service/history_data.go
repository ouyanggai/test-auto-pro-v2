package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/jsonvalues"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// HistoryDataErrorKind 是历史来源接口向 HTTP 层公开的稳定错误分类。
type HistoryDataErrorKind string

const (
	HistoryDataErrorInvalidArgument HistoryDataErrorKind = "invalid_argument"
	HistoryDataErrorNotFound        HistoryDataErrorKind = "not_found"
	HistoryDataErrorConflict        HistoryDataErrorKind = "revision_conflict"
	HistoryDataErrorStorage         HistoryDataErrorKind = "storage"
	HistoryDataErrorTarget          HistoryDataErrorKind = "target"
)

// HistoryDataError 把目标读取、计划归属和来源修订错误收敛为脱敏业务错误。
type HistoryDataError struct {
	Kind    HistoryDataErrorKind
	Message string
}

// Error 返回稳定历史数据错误文案。
func (e *HistoryDataError) Error() string {
	return e.Message
}

// IsHistoryDataErrorKind 判断错误是否属于指定历史数据错误类别。
func IsHistoryDataErrorKind(err error, kind HistoryDataErrorKind) bool {
	var historyErr *HistoryDataError
	return errors.As(err, &historyErr) && historyErr.Kind == kind
}

// HistoryDataManager 组织计划历史候选、不可变快照和默认/路径来源写入。
type HistoryDataManager struct {
	plans  *PlanService
	paths  repository.ExecutionPathRepository
	target HistoryTargetReader
	store  repository.HistoryReplayStore
	now    func() time.Time
}

// NewHistoryDataService 创建历史数据来源服务；当前服务只执行目标只读请求。
func NewHistoryDataService(plans *PlanService, paths repository.ExecutionPathRepository, targetReader HistoryTargetReader, store repository.HistoryReplayStore) *HistoryDataManager {
	return &HistoryDataManager{plans: plans, paths: paths, target: targetReader, store: store, now: time.Now}
}

// Candidates 读取当前计划对应流程的历史候选摘要，不返回目标完整表单数据。
func (s *HistoryDataManager) Candidates(ctx context.Context, planID, pathID uint64, query string, page, pageSize int) (model.HistoryCandidatePage, error) {
	query = strings.TrimSpace(query)
	plan, err := s.getPlan(ctx, planID)
	if err != nil {
		return model.HistoryCandidatePage{}, err
	}
	if s.target == nil {
		return model.HistoryCandidatePage{}, &HistoryDataError{Kind: HistoryDataErrorTarget, Message: "目标历史数据读取暂不可用"}
	}
	identity, err := s.target.HistoryIdentity(ctx, plan.Account, plan.FlowSource, plan.TargetObjectID)
	if err != nil {
		return model.HistoryCandidatePage{}, mapHistoryTargetError(err)
	}
	defaultSource, pathSource, err := s.selectionSources(ctx, planID, pathID, identity.TemplateSummary)
	if err != nil {
		return model.HistoryCandidatePage{}, err
	}
	result, err := s.target.HistoryCandidates(ctx, plan.Account, identity.FlowCode, identity.FormName, identity.FlowName, page, pageSize)
	if err != nil {
		return model.HistoryCandidatePage{}, mapHistoryTargetError(err)
	}
	items := make([]model.HistoryCandidate, 0, len(result.Items))
	for _, instance := range result.Items {
		if query != "" && !historyCandidateMatchesQuery(instance, query) {
			continue
		}
		items = append(items, projectHistoryCandidate(plan.Account, identity, instance))
	}
	total := result.Total
	if query != "" {
		// 搜索只在已经按目标原始字段筛选的有限页内执行，避免把名称猜测当作目标身份匹配。
		total = len(items)
	}
	return model.HistoryCandidatePage{
		Items: items, Page: result.Page, PageSize: result.PageSize, Total: total,
		HasMore: result.HasMore && query == "", DefaultSource: defaultSource, PathSource: pathSource,
	}, nil
}

// selectionSources 读取候选弹窗所需的默认和路径来源摘要，路径未保存来源时实时继承计划默认值。
func (s *HistoryDataManager) selectionSources(ctx context.Context, planID, pathID uint64, currentTemplateSummary map[string]any) (*model.HistoryDataSource, *model.HistoryDataSource, error) {
	if s.store == nil {
		return nil, nil, &HistoryDataError{Kind: HistoryDataErrorStorage, Message: "历史数据存储暂不可用"}
	}
	defaultRecord, defaultFound, err := s.store.GetDefault(ctx, planID)
	if err != nil {
		return nil, nil, mapHistoryStoreError(err)
	}
	var defaultSource *model.HistoryDataSource
	if defaultFound {
		value, sourceErr := s.sourceFromSnapshot(ctx, planID, model.HistorySourceModeDefault, defaultRecord.SnapshotID, defaultRecord.Revision, currentTemplateSummary)
		if sourceErr != nil {
			return nil, nil, sourceErr
		}
		defaultSource = &value
	}
	if pathID == 0 {
		return defaultSource, nil, nil
	}
	if s.paths == nil {
		return nil, nil, &HistoryDataError{Kind: HistoryDataErrorStorage, Message: "执行路径存储暂不可用"}
	}
	if _, err := s.paths.Get(ctx, planID, pathID); err != nil {
		if errors.Is(err, repository.ErrExecutionPathNotFound) {
			return nil, nil, &HistoryDataError{Kind: HistoryDataErrorNotFound, Message: "执行路径不存在"}
		}
		return nil, nil, &HistoryDataError{Kind: HistoryDataErrorStorage, Message: "执行路径存储暂不可用"}
	}
	pathRecord, pathFound, err := s.store.GetPathSource(ctx, pathID)
	if err != nil {
		return nil, nil, mapHistoryStoreError(err)
	}
	if pathFound {
		value, sourceErr := s.sourceFromPathRecord(ctx, planID, pathRecord, currentTemplateSummary)
		if sourceErr != nil {
			return nil, nil, sourceErr
		}
		return defaultSource, &value, nil
	}
	if defaultSource != nil {
		value := *defaultSource
		value.Revision = 0
		return defaultSource, &value, nil
	}
	value := model.HistoryDataSource{Mode: model.HistorySourceModeNone, DataStatus: model.HistoryDataStatusEmpty, Issues: []model.HistoryDataIssue{}, Revision: 0}
	return defaultSource, &value, nil
}

// SaveDefault 创建或替换计划默认历史来源，并在同一调用内保存目标原始数据快照。
func (s *HistoryDataManager) SaveDefault(ctx context.Context, planID uint64, input model.HistoryDefaultSaveInput, idempotencyKey string) (model.HistoryDataSource, error) {
	if err := validateHistoryWriteKey(idempotencyKey); err != nil {
		return model.HistoryDataSource{}, err
	}
	input.CandidateKey = strings.TrimSpace(input.CandidateKey)
	if err := validateHistoryCandidateKey(input.CandidateKey); err != nil {
		return model.HistoryDataSource{}, err
	}
	if s.store == nil {
		return model.HistoryDataSource{}, &HistoryDataError{Kind: HistoryDataErrorStorage, Message: "历史数据存储暂不可用"}
	}
	plan, err := s.getMutablePlan(ctx, planID)
	if err != nil {
		return model.HistoryDataSource{}, err
	}
	current, found, err := s.store.GetDefault(ctx, planID)
	if err != nil {
		return model.HistoryDataSource{}, mapHistoryStoreError(err)
	}
	if found && current.IdempotencyKey == idempotencyKey {
		result, sourceErr := s.sourceFromSnapshot(ctx, planID, model.HistorySourceModeDefault, current.SnapshotID, current.Revision, nil)
		if sourceErr != nil {
			return model.HistoryDataSource{}, sourceErr
		}
		if result.Summary == nil || result.Summary.CandidateKey != input.CandidateKey {
			return model.HistoryDataSource{}, &HistoryDataError{Kind: HistoryDataErrorConflict, Message: "相同请求标识不能用于不同历史候选"}
		}
		return result, nil
	}
	if s.target == nil {
		return model.HistoryDataSource{}, &HistoryDataError{Kind: HistoryDataErrorTarget, Message: "目标历史数据读取暂不可用"}
	}
	identity, err := s.target.HistoryIdentity(ctx, plan.Account, plan.FlowSource, plan.TargetObjectID)
	if err != nil {
		return model.HistoryDataSource{}, mapHistoryTargetError(err)
	}
	source, err := s.target.ReadHistorySnapshot(ctx, plan.Account, identity.FlowCode, identity.FormName, identity.FlowName, input.CandidateKey)
	if err != nil {
		return model.HistoryDataSource{}, mapHistoryTargetError(err)
	}
	if err := validateHistorySnapshotSource(plan.Account, input.CandidateKey, identity, &source); err != nil {
		return model.HistoryDataSource{}, err
	}
	snapshot, err := s.buildSnapshot(plan, identity, source)
	if err != nil {
		return model.HistoryDataSource{}, err
	}
	persisted, record, err := s.store.SaveDefaultWithSnapshot(ctx, snapshot, repository.HistoryDefaultRecord{
		PlanID: planID, IdempotencyKey: idempotencyKey,
	}, input.Revision, s.now().UTC())
	if err != nil {
		return model.HistoryDataSource{}, mapHistoryStoreError(err)
	}
	return projectHistorySource(model.HistorySourceModeDefault, persisted, record.Revision, identity.TemplateSummary), nil
}

// SavePathSource 设置路径动态继承计划默认来源或覆盖为独立历史快照。
func (s *HistoryDataManager) SavePathSource(ctx context.Context, planID, pathID uint64, input model.HistoryPathSourceInput, idempotencyKey string) (model.HistoryDataSource, error) {
	if err := validateHistoryWriteKey(idempotencyKey); err != nil {
		return model.HistoryDataSource{}, err
	}
	input.Mode = strings.TrimSpace(input.Mode)
	input.CandidateKey = strings.TrimSpace(input.CandidateKey)
	switch input.Mode {
	case model.HistorySourceModeDefault:
		if input.CandidateKey != "" {
			return model.HistoryDataSource{}, &HistoryDataError{Kind: HistoryDataErrorInvalidArgument, Message: "继承计划默认来源时不能指定历史候选"}
		}
	case model.HistorySourceModeOverride:
		if err := validateHistoryCandidateKey(input.CandidateKey); err != nil {
			return model.HistoryDataSource{}, err
		}
	default:
		return model.HistoryDataSource{}, &HistoryDataError{Kind: HistoryDataErrorInvalidArgument, Message: "历史来源模式只允许继承默认或独立覆盖"}
	}
	if s.store == nil {
		return model.HistoryDataSource{}, &HistoryDataError{Kind: HistoryDataErrorStorage, Message: "历史数据存储暂不可用"}
	}
	plan, err := s.getMutablePlan(ctx, planID)
	if err != nil {
		return model.HistoryDataSource{}, err
	}
	if s.paths == nil {
		return model.HistoryDataSource{}, &HistoryDataError{Kind: HistoryDataErrorStorage, Message: "执行路径存储暂不可用"}
	}
	if _, err := s.paths.Get(ctx, planID, pathID); err != nil {
		if errors.Is(err, repository.ErrExecutionPathNotFound) {
			return model.HistoryDataSource{}, &HistoryDataError{Kind: HistoryDataErrorNotFound, Message: "执行路径不存在"}
		}
		return model.HistoryDataSource{}, &HistoryDataError{Kind: HistoryDataErrorStorage, Message: "执行路径存储暂不可用"}
	}
	current, found, err := s.store.GetPathSource(ctx, pathID)
	if err != nil {
		return model.HistoryDataSource{}, mapHistoryStoreError(err)
	}
	if found && current.IdempotencyKey == idempotencyKey {
		if current.Mode != input.Mode {
			return model.HistoryDataSource{}, &HistoryDataError{Kind: HistoryDataErrorConflict, Message: "相同请求标识不能用于不同历史来源"}
		}
		result, sourceErr := s.sourceFromPathRecord(ctx, planID, current, nil)
		if sourceErr != nil {
			return model.HistoryDataSource{}, sourceErr
		}
		if input.Mode == model.HistorySourceModeOverride && (result.Summary == nil || result.Summary.CandidateKey != input.CandidateKey) {
			return model.HistoryDataSource{}, &HistoryDataError{Kind: HistoryDataErrorConflict, Message: "相同请求标识不能用于不同历史候选"}
		}
		return result, nil
	}
	var snapshot model.HistorySnapshot
	var currentTemplateSummary map[string]any
	switch input.Mode {
	case model.HistorySourceModeDefault:
		defaultRecord, defaultFound, defaultErr := s.store.GetDefault(ctx, planID)
		if defaultErr != nil {
			return model.HistoryDataSource{}, mapHistoryStoreError(defaultErr)
		}
		if !defaultFound {
			return model.HistoryDataSource{}, &HistoryDataError{Kind: HistoryDataErrorNotFound, Message: "请先设置计划默认历史来源"}
		}
		snapshot, err = s.store.GetSnapshot(ctx, planID, defaultRecord.SnapshotID)
		if err != nil {
			return model.HistoryDataSource{}, mapHistoryStoreError(err)
		}
	case model.HistorySourceModeOverride:
		if s.target == nil {
			return model.HistoryDataSource{}, &HistoryDataError{Kind: HistoryDataErrorTarget, Message: "目标历史数据读取暂不可用"}
		}
		identity, identityErr := s.target.HistoryIdentity(ctx, plan.Account, plan.FlowSource, plan.TargetObjectID)
		if identityErr != nil {
			return model.HistoryDataSource{}, mapHistoryTargetError(identityErr)
		}
		source, readErr := s.target.ReadHistorySnapshot(ctx, plan.Account, identity.FlowCode, identity.FormName, identity.FlowName, input.CandidateKey)
		if readErr != nil {
			return model.HistoryDataSource{}, mapHistoryTargetError(readErr)
		}
		if err := validateHistorySnapshotSource(plan.Account, input.CandidateKey, identity, &source); err != nil {
			return model.HistoryDataSource{}, err
		}
		snapshot, err = s.buildSnapshot(plan, identity, source)
		if err != nil {
			return model.HistoryDataSource{}, err
		}
		currentTemplateSummary = identity.TemplateSummary
	}
	var record repository.HistoryPathSourceRecord
	if input.Mode == model.HistorySourceModeOverride {
		var persisted model.HistorySnapshot
		persisted, record, err = s.store.SavePathSourceWithSnapshot(ctx, planID, snapshot, repository.HistoryPathSourceRecord{
			PathID: pathID, Mode: input.Mode, IdempotencyKey: idempotencyKey,
		}, input.Revision, s.now().UTC())
		snapshot = persisted
	} else {
		record, err = s.store.SavePathSource(ctx, planID, repository.HistoryPathSourceRecord{
			PathID: pathID, Mode: input.Mode, IdempotencyKey: idempotencyKey,
		}, input.Revision, s.now().UTC())
	}
	if err != nil {
		return model.HistoryDataSource{}, mapHistoryStoreError(err)
	}
	return projectHistorySource(input.Mode, snapshot, record.Revision, currentTemplateSummary), nil
}

// getPlan 统一校验计划主键并隐藏计划仓储错误细节。
func (s *HistoryDataManager) getPlan(ctx context.Context, planID uint64) (model.Plan, error) {
	if s.plans == nil {
		return model.Plan{}, &HistoryDataError{Kind: HistoryDataErrorStorage, Message: "计划存储暂不可用"}
	}
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		if IsPlanErrorKind(err, PlanErrorNotFound) {
			return model.Plan{}, &HistoryDataError{Kind: HistoryDataErrorNotFound, Message: "计划不存在"}
		}
		return model.Plan{}, &HistoryDataError{Kind: HistoryDataErrorStorage, Message: "计划存储暂不可用"}
	}
	return plan, nil
}

// getMutablePlan 只允许未进入运行事实的计划修改历史来源。
func (s *HistoryDataManager) getMutablePlan(ctx context.Context, planID uint64) (model.Plan, error) {
	plan, err := s.getPlan(ctx, planID)
	if err != nil {
		return model.Plan{}, err
	}
	if plan.Status != model.PlanStatusNotStarted {
		return model.Plan{}, &HistoryDataError{Kind: HistoryDataErrorConflict, Message: "计划已经不能修改历史来源"}
	}
	return plan, nil
}

// buildSnapshot 深复制目标原始数据并构造待事务保存的不可变快照，不在内存中改写业务正文。
func (s *HistoryDataManager) buildSnapshot(plan model.Plan, identity target.HistoryIdentity, source target.HistorySnapshotSource) (model.HistorySnapshot, error) {
	raw, err := cloneHistoryMap(source.RawFormData)
	if err != nil {
		return model.HistorySnapshot{}, &HistoryDataError{Kind: HistoryDataErrorTarget, Message: "目标历史表单数据无法完整复制"}
	}
	template, err := cloneHistoryMap(source.TemplateSummary)
	if err != nil {
		return model.HistorySnapshot{}, &HistoryDataError{Kind: HistoryDataErrorTarget, Message: "目标历史模板摘要无法完整复制"}
	}
	if len(template) == 0 {
		source.Issues = append(source.Issues, "目标历史模板或页面版本无法确认")
	}
	if len(source.Issues) > 0 {
		template["issues"] = append([]string(nil), source.Issues...)
	}
	instanceSummary := historyInstanceSummary(source.Instance)
	digest := historySourceDigest(raw, identity.FlowCode, source.Instance.ID)
	snapshot := model.HistorySnapshot{
		PlanID: plan.ID, SourceAccount: plan.Account, CandidateKey: HistoryCandidateKey(plan.Account, source.Instance),
		FlowCode:    strings.TrimSpace(source.Instance.FlowCode),
		FormName:    strings.TrimSpace(source.Instance.FormName),
		FlowName:    strings.TrimSpace(source.Instance.FlowName),
		RuntimeType: string(source.RenderType), InstanceStatus: strings.TrimSpace(source.Instance.Status),
		InstanceSummary: instanceSummary, TemplateSummary: template, RawFormData: raw,
		SourceDigest: digest, CreatedAt: s.now().UTC(),
	}
	if snapshot.CandidateKey == strings.Repeat("0", 64) {
		return model.HistorySnapshot{}, &HistoryDataError{Kind: HistoryDataErrorInvalidArgument, Message: "历史候选标识不正确"}
	}
	return snapshot, nil
}

// sourceFromSnapshot 读取已有默认来源并只返回摘要和状态。
func (s *HistoryDataManager) sourceFromSnapshot(ctx context.Context, planID uint64, mode string, snapshotID, revision uint64, currentTemplateSummary map[string]any) (model.HistoryDataSource, error) {
	snapshot, err := s.store.GetSnapshot(ctx, planID, snapshotID)
	if err != nil {
		return model.HistoryDataSource{}, mapHistoryStoreError(err)
	}
	return projectHistorySource(mode, snapshot, revision, currentTemplateSummary), nil
}

// sourceFromPathRecord 将路径来源持久化记录投影为浏览器摘要。
func (s *HistoryDataManager) sourceFromPathRecord(ctx context.Context, planID uint64, record repository.HistoryPathSourceRecord, currentTemplateSummary map[string]any) (model.HistoryDataSource, error) {
	if record.Mode == model.HistorySourceModeNone {
		return model.HistoryDataSource{Mode: record.Mode, DataStatus: model.HistoryDataStatusEmpty, Issues: []model.HistoryDataIssue{}, Revision: record.Revision}, nil
	}
	if record.Mode == model.HistorySourceModeDefault {
		defaultRecord, found, err := s.store.GetDefault(ctx, planID)
		if err != nil {
			return model.HistoryDataSource{}, mapHistoryStoreError(err)
		}
		if !found {
			return model.HistoryDataSource{Mode: record.Mode, DataStatus: model.HistoryDataStatusEmpty, Issues: []model.HistoryDataIssue{{Code: "HISTORY_DEFAULT_MISSING", Message: "计划默认历史来源尚未设置", Blocking: true}}, Revision: record.Revision}, nil
		}
		return s.sourceFromSnapshot(ctx, planID, record.Mode, defaultRecord.SnapshotID, record.Revision, currentTemplateSummary)
	}
	if record.SnapshotID == 0 {
		return model.HistoryDataSource{}, &HistoryDataError{Kind: HistoryDataErrorStorage, Message: "路径历史来源数据异常"}
	}
	snapshot, err := s.store.GetSnapshot(ctx, planID, record.SnapshotID)
	if err != nil {
		return model.HistoryDataSource{}, mapHistoryStoreError(err)
	}
	return projectHistorySource(record.Mode, snapshot, record.Revision, currentTemplateSummary), nil
}

// projectHistoryCandidate 投影目标实例摘要并仅生成不透明候选键。
func projectHistoryCandidate(account string, identity target.HistoryIdentity, instance target.HistoryInstance) model.HistoryCandidate {
	completeness := "partial"
	notice := "该实例未处于已完成状态，数据可能不完整"
	if historyStatusRank(instance.Status) == 0 && historyCandidateSummaryComplete(identity, instance) {
		completeness = "complete"
		notice = ""
	} else if !historyCandidateSummaryComplete(identity, instance) {
		notice = "目标候选摘要字段不完整，选择后需要人工核对"
	}
	if identity.FormName == "" && (strings.TrimSpace(identity.FlowName) == "" || strings.TrimSpace(instance.FlowName) == "") {
		completeness = "partial"
		notice = "目标缺少稳定流程或页面名称，选择后需要人工核对"
	}
	return model.HistoryCandidate{
		CandidateKey:    HistoryCandidateKey(account, instance),
		FlowCode:        strings.TrimSpace(instance.FlowCode),
		FormName:        strings.TrimSpace(instance.FormName),
		FlowName:        strings.TrimSpace(instance.FlowName),
		RuntimeType:     string(identity.RenderType),
		InstanceTitle:   strings.TrimSpace(instance.Title),
		BusinessSummary: strings.TrimSpace(instance.BusinessSummary),
		Initiator:       strings.TrimSpace(instance.Initiator),
		CompanyName:     strings.TrimSpace(instance.CompanyName),
		CreatedAt:       strings.TrimSpace(instance.CreatedAt),
		Status:          strings.TrimSpace(instance.Status),
		StatusName:      strings.TrimSpace(instance.StatusName),
		Completeness:    completeness, IntegrityNotice: notice, SnapshotAvailable: strings.TrimSpace(instance.ID) != "",
	}
}

// historyCandidateSummaryComplete 核对选择弹窗要求的目标原始摘要字段，不用名称拼装缺失身份。
func historyCandidateSummaryComplete(identity target.HistoryIdentity, instance target.HistoryInstance) bool {
	stableIdentity := strings.TrimSpace(instance.FormName) != ""
	if strings.TrimSpace(identity.FormName) == "" {
		stableIdentity = strings.TrimSpace(instance.FlowName) != ""
	}
	return strings.TrimSpace(instance.ID) != "" && strings.TrimSpace(instance.FlowCode) != "" && stableIdentity &&
		(strings.TrimSpace(instance.Title) != "" || strings.TrimSpace(instance.BusinessSummary) != "") &&
		strings.TrimSpace(instance.Initiator) != "" && strings.TrimSpace(instance.CompanyName) != "" &&
		strings.TrimSpace(instance.CreatedAt) != "" && strings.TrimSpace(instance.Status) != "" &&
		(identity.RenderType == target.FormRenderTypeFormMaking || identity.RenderType == target.FormRenderTypeVueCustom)
}

// projectHistorySource 投影历史来源摘要，永不包含完整表单正文。
func projectHistorySource(mode string, snapshot model.HistorySnapshot, revision uint64, currentTemplateSummary map[string]any) model.HistoryDataSource {
	issues := []model.HistoryDataIssue{{Code: "HISTORY_REPLAY_REQUIRED", Message: "需要完成当前路径回放和 form-runtime 校验", Blocking: true}}
	// 来源选择本身不能宣称 ready；只有后续路径回放和复制 runtime 校验都通过才能转为 ready。
	status := model.HistoryDataStatusNeedsInput
	if len(snapshot.RawFormData) == 0 {
		status = model.HistoryDataStatusNeedsInput
		issues = append(issues, model.HistoryDataIssue{Code: "HISTORY_DATA_EMPTY", Message: "目标历史表单数据为空，需要人工补充", Blocking: true})
	}
	if snapshot.RuntimeType != string(target.FormRenderTypeFormMaking) && snapshot.RuntimeType != string(target.FormRenderTypeVueCustom) {
		status = model.HistoryDataStatusNeedsInput
		issues = append(issues, model.HistoryDataIssue{Code: "HISTORY_RUNTIME_UNKNOWN", Message: "目标表单运行时类型无法确认，需要人工核对", Blocking: true})
	}
	for _, message := range historySnapshotIssues(snapshot.TemplateSummary["issues"]) {
		status = model.HistoryDataStatusNeedsInput
		issues = append(issues, model.HistoryDataIssue{Code: "HISTORY_SOURCE_INCOMPLETE", Message: message, Blocking: true})
	}
	sourceVersion := stringMapValue(snapshot.TemplateSummary, "runtimeVersionDigest")
	if sourceVersion == "" {
		issues = append(issues, model.HistoryDataIssue{Code: "HISTORY_SOURCE_VERSION_UNVERIFIED", Message: "目标历史模板或页面版本无法完整确认", Blocking: true})
	} else if currentTemplateSummary != nil {
		currentVersion := stringMapValue(currentTemplateSummary, "runtimeVersionDigest")
		if currentVersion == "" {
			issues = append(issues, model.HistoryDataIssue{Code: "HISTORY_CURRENT_VERSION_UNVERIFIED", Message: "当前目标模板或页面版本无法完整确认", Blocking: true})
		} else if currentVersion != sourceVersion {
			status = model.HistoryDataStatusAffected
			issues = append(issues, model.HistoryDataIssue{Code: "HISTORY_RUNTIME_VERSION_CHANGED", Message: "历史来源与当前目标模板或页面版本不同，需要重新核对", Blocking: true})
		}
	}
	return model.HistoryDataSource{
		Mode: mode, SnapshotID: snapshot.ID, Summary: snapshotSummary(snapshot),
		DataStatus: status, Issues: issues, Revision: revision,
	}
}

// historySnapshotIssues 兼容 JSON 解码后的数组和值刚创建时的字符串数组，两者只用于同一快照摘要。
func historySnapshotIssues(value any) []string {
	result := make([]string, 0)
	switch typed := value.(type) {
	case []string:
		for _, item := range typed {
			if item = strings.TrimSpace(item); item != "" {
				result = append(result, item)
			}
		}
	case []any:
		for _, item := range typed {
			if text, ok := item.(string); ok {
				if text = strings.TrimSpace(text); text != "" {
					result = append(result, text)
				}
			}
		}
	}
	return result
}

// snapshotSummary 把不可变快照投影为不含原始数据的来源摘要。
func snapshotSummary(snapshot model.HistorySnapshot) *model.HistorySnapshotSummary {
	return &model.HistorySnapshotSummary{
		CandidateKey: snapshot.CandidateKey, FlowCode: snapshot.FlowCode, FormName: snapshot.FormName,
		FlowName: snapshot.FlowName, InstanceTitle: stringMapValue(snapshot.InstanceSummary, "instanceTitle"),
		BusinessSummary: stringMapValue(snapshot.InstanceSummary, "businessSummary"),
		Initiator:       stringMapValue(snapshot.InstanceSummary, "initiator"),
		CompanyName:     stringMapValue(snapshot.InstanceSummary, "companyName"),
		CreatedAt:       stringMapValue(snapshot.InstanceSummary, "createdAt"),
		Status:          snapshot.InstanceStatus, StatusName: stringMapValue(snapshot.InstanceSummary, "statusName"),
		RuntimeType: snapshot.RuntimeType,
	}
}

// historyInstanceSummary 只保存目标实例摘要字段，不把目标 ID 写入工具数据库。
func historyInstanceSummary(instance target.HistoryInstance) map[string]any {
	return map[string]any{
		"instanceTitle": strings.TrimSpace(instance.Title), "businessSummary": strings.TrimSpace(instance.BusinessSummary),
		"initiator": strings.TrimSpace(instance.Initiator), "companyName": strings.TrimSpace(instance.CompanyName),
		"createdAt": strings.TrimSpace(instance.CreatedAt), "status": strings.TrimSpace(instance.Status),
		"statusName": strings.TrimSpace(instance.StatusName), "currentNodeName": strings.TrimSpace(instance.CurrentNodeName),
	}
}

// historySourceDigest 对目标原始数据和实例身份做摘要，用于不可变快照冲突检测。
func historySourceDigest(raw map[string]any, flowCode, instanceID string) string {
	payload, _ := json.Marshal(struct {
		FlowCode string         `json:"flowCode"`
		Instance string         `json:"instance"`
		Raw      map[string]any `json:"raw"`
	}{FlowCode: flowCode, Instance: instanceID, Raw: raw})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// cloneHistoryMap 深复制目标原始 JSON，任何复制失败都向上返回，禁止降级为空对象。
func cloneHistoryMap(value map[string]any) (map[string]any, error) {
	return jsonvalues.DeepCopyObject(value)
}

// historyCandidateMatchesQuery 仅在目标原始摘要字段上做可见搜索，不改变流程身份匹配。
func historyCandidateMatchesQuery(instance target.HistoryInstance, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	for _, value := range []string{instance.Title, instance.BusinessSummary, instance.Initiator, instance.CompanyName, instance.StatusName} {
		if strings.Contains(strings.ToLower(strings.TrimSpace(value)), query) {
			return true
		}
	}
	return false
}

// validateHistoryWriteKey 要求所有历史来源写入带标准幂等键。
func validateHistoryWriteKey(value string) error {
	if !validUUID(strings.TrimSpace(value)) {
		return &HistoryDataError{Kind: HistoryDataErrorInvalidArgument, Message: "历史数据写入请求标识不正确，请重试"}
	}
	return nil
}

// validateHistoryCandidateKey 校验浏览器只能回传后端生成的固定长度不透明摘要键。
func validateHistoryCandidateKey(value string) error {
	value = strings.TrimSpace(value)
	if len(value) != 64 {
		return &HistoryDataError{Kind: HistoryDataErrorInvalidArgument, Message: "历史候选标识不正确"}
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size {
		return &HistoryDataError{Kind: HistoryDataErrorInvalidArgument, Message: "历史候选标识不正确"}
	}
	return nil
}

// validateHistorySnapshotSource 复核目标读取结果仍属于计划身份和用户选择，防止候选变化后错绑快照。
func validateHistorySnapshotSource(account, candidateKey string, identity target.HistoryIdentity, source *target.HistorySnapshotSource) error {
	if source == nil || strings.TrimSpace(source.Instance.ID) == "" || HistoryCandidateKey(account, source.Instance) != candidateKey {
		return &HistoryDataError{Kind: HistoryDataErrorNotFound, Message: "历史候选不存在或已发生变化"}
	}
	if strings.TrimSpace(source.Instance.FlowCode) != strings.TrimSpace(identity.FlowCode) {
		return &HistoryDataError{Kind: HistoryDataErrorTarget, Message: "目标历史候选流程身份不一致"}
	}
	if expectedForm := strings.TrimSpace(identity.FormName); expectedForm != "" {
		if strings.TrimSpace(source.Instance.FormName) != expectedForm {
			return &HistoryDataError{Kind: HistoryDataErrorTarget, Message: "目标历史候选表单身份不一致"}
		}
		return nil
	}
	expectedFlowName := strings.TrimSpace(identity.FlowName)
	actualFlowName := strings.TrimSpace(source.Instance.FlowName)
	if expectedFlowName != "" && actualFlowName != "" && actualFlowName != expectedFlowName {
		return &HistoryDataError{Kind: HistoryDataErrorTarget, Message: "目标历史候选页面身份不一致"}
	}
	if expectedFlowName == "" || actualFlowName == "" {
		source.Issues = append(source.Issues, "目标无表单流程缺少稳定流程或页面名称")
	}
	return nil
}

// mapHistoryTargetError 隐藏目标响应原文并保持会话/超时语义。
func mapHistoryTargetError(err error) error {
	if errors.Is(err, ErrTargetFlowNotFound) {
		return &HistoryDataError{Kind: HistoryDataErrorNotFound, Message: "历史候选不存在或已不可见"}
	}
	return &HistoryDataError{Kind: HistoryDataErrorTarget, Message: "暂时无法读取目标历史数据，请重试"}
}

// mapHistoryStoreError 将仓储错误收敛为历史来源稳定错误。
func mapHistoryStoreError(err error) error {
	switch {
	case errors.Is(err, repository.ErrPlanNotFound):
		return &HistoryDataError{Kind: HistoryDataErrorNotFound, Message: "计划不存在"}
	case errors.Is(err, repository.ErrExecutionPathNotFound):
		return &HistoryDataError{Kind: HistoryDataErrorNotFound, Message: "执行路径不存在"}
	case errors.Is(err, repository.ErrHistorySnapshotNotFound):
		return &HistoryDataError{Kind: HistoryDataErrorNotFound, Message: "历史数据快照不存在"}
	case errors.Is(err, repository.ErrHistoryRevisionConflict):
		return &HistoryDataError{Kind: HistoryDataErrorConflict, Message: "历史数据来源已被其他请求更新，请刷新后重试"}
	default:
		return &HistoryDataError{Kind: HistoryDataErrorStorage, Message: "历史数据存储暂不可用"}
	}
}

// stringMapValue 读取摘要 JSON 字段并避免向浏览器暴露未知类型。
func stringMapValue(values map[string]any, key string) string {
	value, ok := values[key]
	if !ok {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}
