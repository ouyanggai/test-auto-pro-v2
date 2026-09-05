package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/formdata/branchoverlay"
	"test-auto-pro-v2/internal/jsonvalues"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// PathConfigurationDataService 提供 F-012 原始表单数据工作区的读取与保存能力。
type PathConfigurationDataService interface {
	GetData(context.Context, uint64, uint64) (model.PathConfigurationF012, error)
	SaveData(context.Context, uint64, uint64, string, model.PathConfigurationDataInput) (model.PathConfigurationDataResult, error)
}

// SetHistoryWorkspaceStores 注入历史来源和迁移 022 路径配置仓储。
// 数据工作区不从旧路径配置仓储读取或写入生成字段，两个存储边界必须显式提供。
func (s *PathConfigService) SetHistoryWorkspaceStores(history repository.HistoryReplayStore, configs repository.HistoryPathConfigStore) {
	s.historyStore = history
	s.historyConfigStore = configs
}

// GetData 读取当前路径的数据工作区，并把目标原始表单数据直接投影给复制的 form-runtime。
func (s *PathConfigService) GetData(ctx context.Context, planID, pathID uint64) (model.PathConfigurationF012, error) {
	path, snapshot, analysis, stored, found, source, err := s.loadWorkspace(ctx, planID, pathID)
	if err != nil {
		return model.PathConfigurationF012{}, err
	}
	// 历史来源绑定时必须按"实例绑定的流程代理版本"渲染 FormMaking 模板：历史数据键与实例当时的
	// 表单模板一致，用当前已发布模板回显会在模板改版后整表对不上（宿主平台渲染旧实例也正是用
	// 实例绑定版本）。读取失败回落当前模板并如实记录问题，不让用户面对静默错版。
	runtimeSnapshot, resolved, runtimeErr := s.resolveInstanceBoundSnapshot(ctx, planID, source)
	var (
		template    map[string]any
		unsupported []string
	)
	switch {
	case runtimeErr != nil:
		template, unsupported = workspaceRuntimeTemplate(snapshot)
	case resolved:
		template, unsupported = workspaceRuntimeTemplate(runtimeSnapshot)
	default:
		template, unsupported = workspaceRuntimeTemplate(snapshot)
	}
	issues := append([]model.HistoryDataIssue(nil), source.dataSource.Issues...)
	values := map[string]any{}
	patches := []model.HistoryBranchPatch{}
	runtimeValidation := model.HistoryRuntimeValidation{}
	if found {
		// 已保存的旧工作区数据也必须在读取边界清理，否则修复上线前落盘的历史审批意见仍会回显。
		values = clearAuditInfoValues(decodeWorkspaceMap(stored.EffectiveFormData))
		patches = decodeWorkspacePatches(stored.BranchPatches)
		runtimeValidation = decodeWorkspaceValidation(stored.RuntimeValidation)
		storedIssues := decodeWorkspaceIssues(stored.Issues)
		// 保存成功后，来源投影仍会带有“尚未按当前路径核对”的初始占位问题。
		// 已存在的配置行才是本路径最后一次服务端复验事实；即使问题数组为空，也必须覆盖该占位问题。
		if strings.TrimSpace(stored.DataStatus) != "" {
			issues = storedIssues
		}
	}
	if runtimeErr != nil {
		issues = appendHistoryIssues(issues, []model.HistoryDataIssue{{Code: "HISTORY_TEMPLATE_INSTANCE_FALLBACK",
			Message: "实例绑定表单版本读取失败，已按当前模板回显，字段可能对不上，请重试或重新核对字段", Blocking: false}})
	}
	if len(unsupported) > 0 {
		issues = appendHistoryIssues(issues, historyIssuesFromStrings("HISTORY_RUNTIME_TEMPLATE_INVALID", unsupported))
	}
	// 来源配置行可能只保存了来源模式而没有有效数据；只有 empty 或未创建配置时才初始化快照，避免把用户明确保存的空对象改回历史正文。
	if source.snapshot != nil && (!found || strings.TrimSpace(stored.DataStatus) == "" || stored.DataStatus == model.HistoryDataStatusEmpty) {
		values = clearAuditInfoValues(cloneWorkspaceMap(source.snapshot.RawFormData))
		overlay := branchoverlay.Apply(branchoverlay.Input{Tree: snapshot.Tree, Choices: path.Choices, Values: values})
		if overlay.Status == branchoverlay.StatusReady {
			values = overlay.Values
			patches = overlay.Patches
		} else {
			issues = appendHistoryIssues(issues, historyIssuesFromOverlay(overlay.Issues))
		}
	}
	dataStatus := source.dataStatus
	if found && strings.TrimSpace(stored.DataStatus) != "" {
		dataStatus = stored.DataStatus
		// 旧版本在默认来源重复确认时会把数据标为 affected，但同时保留完整正文和已通过的 runtime 证据。
		// 读取时恢复这类可证明的 ready 状态，避免历史脏状态继续把已保存数据显示为待处理。
		if stored.DataStatus == model.HistoryDataStatusAffected && stored.SourceMode == model.HistorySourceModeDefault && stored.SnapshotID == nil && len(values) > 0 && runtimeValidation.Accepted {
			dataStatus = model.HistoryDataStatusReady
		}
	}
	if len(values) == 0 && dataStatus == "" {
		dataStatus = model.HistoryDataStatusEmpty
	}
	if !found && len(values) > 0 && len(issues) == 0 {
		dataStatus = model.HistoryDataStatusNeedsInput
	}
	if len(issues) == 0 && dataStatus == model.HistoryDataStatusEmpty {
		issues = []model.HistoryDataIssue{}
	}
	// 读取边界同步公司下拉真实 ID：已保存的数据或旧补丁可能只改了名称字段，必须在这里补齐 ID 才能保证回显一致。
	// 同步放在状态推导之后，新增的非阻断问题不改变数据状态语义。
	if syncIssues := s.syncLinkedCompanySelects(ctx, template, values); len(syncIssues) > 0 {
		issues = appendHistoryIssues(issues, syncIssues)
	}
	// 登录人上下文字段跟随当前计划账号：目标提交时总会用登录态覆盖该字段，历史发起人身份不能带进回放。
	if identity, identityErr := s.currentUserIdentity(ctx, planID); identityErr == nil {
		replaceUserIdentityValues(values, identity)
	}
	// 条件字段的填写时机按节点权限算出；一个节点都填不了的决定性条件字段必须阻断，
	// 否则运行到那一步只能听天由命地走目标现有数据决定的分支。
	keyFields := workspaceKeyFields(snapshot, path.Choices, values, analysis.pathAnalysis.ReachableNodeIDs)
	if unfillable := unfillableKeyFieldIssues(keyFields, routeNodePowers(snapshot.Tree, analysis.pathAnalysis.ReachableNodeIDs)); len(unfillable) > 0 {
		issues = appendHistoryIssues(issues, unfillable)
	}
	return model.PathConfigurationF012{
		Path: pathConfigPath(path), Revision: stored.Revision, NodeRevision: stored.NodeRevision,
		DataRevision: stored.DataRevision, ActionRevision: stored.ActionRevision,
		NodeStatus: defaultWorkspaceStatus(stored.NodeStatus, "pending"), DataStatus: dataStatus,
		HistorySource: source.dataSource, RuntimeType: string(snapshot.RenderType),
		RuntimeTemplate: template, RuntimePage: projectVueCustomPage(snapshot.VuePage),
		RuntimePermissions: workspacePermissions(snapshot, analysis), RuntimeReadRequests: workspaceReadRequests(snapshot, template), EffectiveFormData: values,
		BranchPatches: patches, RuntimeValidation: runtimeValidation, Issues: issues,
		KeyFields: keyFields, NodeViews: nodeFormViews(snapshot.Tree, analysis.pathAnalysis.ReachableNodeIDs),
		Actions: decodeWorkspaceActions(stored.UserActions), CompiledScenario: decodeWorkspaceSteps(stored.CompiledSteps),
	}, nil
}

// targetAuditInfoModelPrefix 是目标表单里自动回填审批意见的模型前缀（auto_audit_info_1、auto_audit_info_2 …）。
// 配置阶段的表单永远处于发起态，历史实例上的审批意见不属于这一次要提交的业务数据。
const targetAuditInfoModelPrefix = "auto_audit_info_"

// clearAuditInfoValues 清空历史业务数据里的审批意见字段，保留键与结构，不生成任何内容。
func clearAuditInfoValues(values map[string]any) map[string]any {
	for key, value := range values {
		if !strings.HasPrefix(strings.TrimSpace(key), targetAuditInfoModelPrefix) {
			continue
		}
		if _, isString := value.(string); isString || value == nil {
			values[key] = ""
			continue
		}
		delete(values, key)
	}
	return values
}

// keyFieldLabels 从目标字段详情和 FormMaking 可见标签建立"字段路径 → 中文名称"映射。
// 报表布局把标签与输入组件拆在相邻单元格中，因此需要用模板原始布局覆盖组件的通用名称。
func keyFieldLabels(snapshot target.PathConfigurationSnapshot) map[string]string {
	labels := make(map[string]string, len(snapshot.FormFields))
	for _, field := range snapshot.FormFields {
		path := strings.TrimSpace(field.EnglishName)
		name := strings.TrimSpace(field.Name)
		if path == "" || name == "" {
			continue
		}
		labels[path] = name
	}
	if snapshot.VuePage != nil {
		for _, field := range snapshot.VuePage.Fields {
			path := strings.TrimSpace(field.Path)
			name := strings.TrimSpace(field.Name)
			if path == "" || name == "" {
				continue
			}
			labels[path] = name
		}
	}
	for _, form := range snapshot.Forms {
		var template any
		if json.Unmarshal([]byte(form.TemplateData), &template) != nil {
			continue
		}
		collectTemplateFieldLabels(template, labels)
	}
	for path, label := range labels {
		if strings.HasSuffix(path, "__virtualName") {
			continue
		}
		virtualPath := path + "__virtualName"
		if _, exists := labels[virtualPath]; !exists {
			labels[virtualPath] = label
		}
	}
	return labels
}

// collectTemplateFieldLabels 递归读取模板中组件自身可见标签，以及报表行里紧邻输入组件之前的文本标签。
func collectTemplateFieldLabels(node any, labels map[string]string) {
	object, ok := node.(map[string]any)
	if !ok {
		if items, isList := node.([]any); isList {
			for _, item := range items {
				collectTemplateFieldLabels(item, labels)
			}
		}
		return
	}
	modelName := strings.TrimSpace(templateString(object["model"]))
	componentName := strings.TrimSpace(templateString(object["name"]))
	if modelName != "" && componentName != "" && !templateComponentHidesLabel(object) {
		labels[modelName] = componentName
	}
	if rows, ok := object["rows"].([]any); ok {
		for _, rowValue := range rows {
			row, isRow := rowValue.(map[string]any)
			if !isRow {
				continue
			}
			columns, isColumns := row["columns"].([]any)
			if !isColumns {
				continue
			}
			pendingLabel := ""
			for _, column := range columns {
				models := templateModels(column)
				if len(models) > 0 {
					if pendingLabel != "" {
						for _, model := range models {
							labels[model] = pendingLabel
						}
					}
					pendingLabel = ""
					continue
				}
				if label := templateTextLabel(column); label != "" {
					pendingLabel = label
				}
			}
		}
	}
	for _, child := range object {
		collectTemplateFieldLabels(child, labels)
	}
}

// templateModels 返回一个模板片段内声明的全部字段模型，保持首次出现顺序并去重。
func templateModels(node any) []string {
	models := []string{}
	seen := map[string]struct{}{}
	var visit func(any)
	visit = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			modelName := strings.TrimSpace(templateString(typed["model"]))
			if modelName != "" {
				if _, exists := seen[modelName]; !exists {
					seen[modelName] = struct{}{}
					models = append(models, modelName)
				}
			}
			for _, child := range typed {
				visit(child)
			}
		case []any:
			for _, child := range typed {
				visit(child)
			}
		}
	}
	visit(node)
	return models
}

// templateTextLabel 提取没有绑定字段模型的可见文本组件名称，作为相邻输入单元格的标签。
func templateTextLabel(node any) string {
	switch typed := node.(type) {
	case map[string]any:
		if strings.TrimSpace(templateString(typed["type"])) == "text" && strings.TrimSpace(templateString(typed["model"])) == "" {
			if name := strings.TrimSpace(templateString(typed["name"])); name != "" {
				return name
			}
		}
		for _, child := range typed {
			if label := templateTextLabel(child); label != "" {
				return label
			}
		}
	case []any:
		for _, child := range typed {
			if label := templateTextLabel(child); label != "" {
				return label
			}
		}
	}
	return ""
}

// templateComponentHidesLabel 判断组件是否明确隐藏自身标签。
func templateComponentHidesLabel(component map[string]any) bool {
	options, ok := component["options"].(map[string]any)
	if !ok {
		return false
	}
	hideLabel, _ := options["hideLabel"].(bool)
	return hideLabel
}

// templateString 安全读取模板字符串字段，拒绝把其他 JSON 类型格式化为标签。
func templateString(value any) string {
	text, _ := value.(string)
	return text
}

// workspaceKeyFields 投影决定当前路径的条件字段，让界面直接告诉用户先核对哪些字段，
// 并按节点字段权限标出每个字段的填写时机：发起人能填，还是要等到某个节点执行时自动带上。
func workspaceKeyFields(snapshot target.PathConfigurationSnapshot, choices []model.ExecutionPathChoice, values map[string]any, reachable []string) []model.HistoryKeyField {
	fields := branchoverlay.KeyFields(branchoverlay.Input{Tree: snapshot.Tree, Choices: choices, Values: values})
	labels := keyFieldLabels(snapshot)
	result := make([]model.HistoryKeyField, 0, len(fields))
	for _, field := range fields {
		result = append(result, model.HistoryKeyField{
			Path: field.Path, Label: labels[field.Path], HasCurrent: field.HasCurrent, Current: field.Current,
			Candidates: field.Candidates, Operators: field.Operators, Branches: field.Branches, Decisive: field.Decisive,
		})
	}
	return keyFieldFillHints(result, routeNodePowers(snapshot.Tree, reachable))
}

// SaveData 保存复制 form-runtime 捕获的原始表单数据，并在实际路径变化时要求一次性确认。
func (s *PathConfigService) SaveData(ctx context.Context, planID, pathID uint64, idempotencyKey string, input model.PathConfigurationDataInput) (model.PathConfigurationDataResult, error) {
	if err := validateHistoryWriteKey(idempotencyKey); err != nil {
		return model.PathConfigurationDataResult{}, err
	}
	if input.Values == nil {
		return model.PathConfigurationDataResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "目标原始表单数据不能为空", Affected: []model.PathConfigAffectedItem{{Kind: "form", Name: "表单数据", Reason: "runtime 未返回原始 values"}}}
	}
	path, snapshot, analysis, current, found, source, err := s.loadWorkspace(ctx, planID, pathID)
	if err != nil {
		return model.PathConfigurationDataResult{}, err
	}
	if source.snapshot == nil {
		return model.PathConfigurationDataResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "请先选择基础表单数据", Affected: []model.PathConfigAffectedItem{{Kind: "form", Name: "基础表单数据", Reason: "当前路径尚未绑定可用基础表单数据"}}}
	}
	// 浏览器提交的修订号是保存并发屏障的一部分；先在换路计算前核对，避免旧正文取得新的确认令牌。
	if input.Revision != current.Revision {
		return model.PathConfigurationDataResult{}, &PathConfigError{Kind: PathConfigErrorRevisionConflict, Message: "路径表单数据已被其他操作更新，请刷新后重试"}
	}
	// 保存入口再次清理浏览器回传值，避免旧页面或运行时脚本把历史审批意见重新写回工作区。
	values := clearAuditInfoValues(cloneWorkspaceMap(input.Values))
	actual := branchoverlay.ResolveActualPath(branchoverlay.Input{Tree: snapshot.Tree, Choices: path.Choices, Values: values})
	if actual.Status != branchoverlay.StatusReady {
		return model.PathConfigurationDataResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "表单数据无法确定目标实际路径", Affected: affectedFromOverlayIssues(actual.Issues)}
	}
	targetPath := path
	routeChanged := !sameWorkspaceChoices(path.Choices, actual.ActualChoices)
	var targetStored repository.HistoryPathConfigRecord
	var targetFound bool
	if routeChanged {
		matched, matchedOK, matchErr := s.findWorkspacePath(ctx, planID, actual.ActualChoices)
		if matchErr != nil {
			return model.PathConfigurationDataResult{}, matchErr
		}
		if !matchedOK {
			return model.PathConfigurationDataResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前实际路径尚未建立执行路径，不能自动创建新路径", Affected: []model.PathConfigAffectedItem{{Kind: "path", Name: "执行路径", Reason: "请先保存对应的真实分支选择"}}}
		}
		targetPath = matched
		if s.historyConfigStore == nil {
			return model.PathConfigurationDataResult{}, &PathConfigError{Kind: PathConfigErrorStorage, Message: "路径数据存储暂不可用"}
		}
		targetStored, targetFound, err = s.historyConfigStore.GetPathConfig(ctx, targetPath.ID)
		if err != nil {
			return model.PathConfigurationDataResult{}, mapHistoryWorkspaceStoreError(err)
		}
		routeChange := workspaceRouteChange(path, targetPath, targetFound, targetStored)
		token := workspaceConfirmationToken(planID, path.ID, targetPath.ID, values, current.Revision, targetStored.Revision)
		if strings.TrimSpace(input.ConfirmationToken) != token {
			return model.PathConfigurationDataResult{}, &PathConfigError{Kind: PathConfigErrorRouteConfirmation, Message: "表单数据实际命中其他执行路径，请确认换路和覆盖影响", RouteChange: &routeChange, ConfirmationToken: token}
		}
		if targetFound {
			current = targetStored
		}
		analysis, err = s.analyzeOwnedPath(ctx, planID, snapshot, targetPath)
		if err != nil {
			return model.PathConfigurationDataResult{}, err
		}
	}
	if !routeChanged {
		targetFound = found
		targetStored = current
	}
	// 再按最终路径复验一次，保存值只允许来自 runtime 捕获和目标条件窄范围结果。
	overlay := branchoverlay.Apply(branchoverlay.Input{Tree: snapshot.Tree, Choices: targetPath.Choices, Values: actual.Values})
	if overlay.Status != branchoverlay.StatusReady {
		return model.PathConfigurationDataResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "表单数据未通过目标路径复验", Affected: affectedFromOverlayIssues(overlay.Issues)}
	}
	issues := append([]model.HistoryDataIssue{}, historyIssuesFromOverlay(overlay.Issues)...)
	issues = appendHistoryIssues(issues, input.RuntimeValidation.Issues)
	// 保存边界同样同步公司下拉真实 ID：浏览器捕获值和直接接口保存都必须落成名称与 ID 一致的数据。
	// 同步只改 ID 与虚拟显示值，不改分支条件读取的名称字段，因此不影响已经完成的路径复验结论。
	if template, _ := workspaceRuntimeTemplate(snapshot); len(template) > 0 {
		issues = appendHistoryIssues(issues, s.syncLinkedCompanySelects(ctx, template, overlay.Values))
	}
	// 落盘前把登录人上下文字段替换为当前计划账号身份，历史发起人身份不允许进入工作区正文。
	if identity, identityErr := s.currentUserIdentity(ctx, planID); identityErr == nil {
		replaceUserIdentityValues(overlay.Values, identity)
	}
	dataStatus := workspaceDataStatus(input.RuntimeValidation, issues)
	record, err := workspaceRecord(snapshot, source, targetPath, targetStored, idempotencyKey, input, overlay, dataStatus, routeChanged)
	if err != nil {
		return model.PathConfigurationDataResult{}, err
	}
	var saved repository.HistoryPathConfigRecord
	if routeChanged {
		store, ok := s.historyConfigStore.(repository.HistoryPathDataStore)
		if !ok {
			return model.PathConfigurationDataResult{}, &PathConfigError{Kind: PathConfigErrorStorage, Message: "路径换路事务存储暂不可用"}
		}
		saved, err = store.SavePathData(ctx, planID, path.ID, targetPath.ID, record, targetStored.Revision, s.now().UTC())
	} else {
		if s.historyConfigStore == nil {
			return model.PathConfigurationDataResult{}, &PathConfigError{Kind: PathConfigErrorStorage, Message: "路径数据存储暂不可用"}
		}
		saved, err = s.historyConfigStore.SavePathConfig(ctx, record, input.Revision, s.now().UTC())
	}
	if err != nil {
		return model.PathConfigurationDataResult{}, mapHistoryWorkspaceStoreError(err)
	}
	return workspaceDataResult(snapshot, analysis, targetPath, saved, routeChanged, input.RuntimeValidation, issues, overlay), nil
}

type historyWorkspaceSource struct {
	dataSource model.HistoryDataSource
	snapshot   *model.HistorySnapshot
	dataStatus string
}

// loadWorkspace 统一读取计划、真实路径、目标原始快照和新路径配置行。
func (s *PathConfigService) loadWorkspace(ctx context.Context, planID, pathID uint64) (model.ExecutionPath, target.PathConfigurationSnapshot, ownedPathAnalysis, repository.HistoryPathConfigRecord, bool, historyWorkspaceSource, error) {
	if planID == 0 || pathID == 0 {
		return model.ExecutionPath{}, target.PathConfigurationSnapshot{}, ownedPathAnalysis{}, repository.HistoryPathConfigRecord{}, false, historyWorkspaceSource{}, &PathConfigError{Kind: PathConfigErrorInvalidArgument, Message: "计划或路径 ID 不正确"}
	}
	path, err := s.ownedPath(ctx, planID, pathID)
	if err != nil {
		return model.ExecutionPath{}, target.PathConfigurationSnapshot{}, ownedPathAnalysis{}, repository.HistoryPathConfigRecord{}, false, historyWorkspaceSource{}, err
	}
	if err := s.validateConfigMutablePlan(ctx, planID); err != nil {
		return model.ExecutionPath{}, target.PathConfigurationSnapshot{}, ownedPathAnalysis{}, repository.HistoryPathConfigRecord{}, false, historyWorkspaceSource{}, err
	}
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return model.ExecutionPath{}, target.PathConfigurationSnapshot{}, ownedPathAnalysis{}, repository.HistoryPathConfigRecord{}, false, historyWorkspaceSource{}, err
	}
	if s.target == nil {
		return model.ExecutionPath{}, target.PathConfigurationSnapshot{}, ownedPathAnalysis{}, repository.HistoryPathConfigRecord{}, false, historyWorkspaceSource{}, &PathConfigError{Kind: PathConfigErrorStorage, Message: "目标配置读取服务暂不可用"}
	}
	// T05 数据工作区直接读取目标原始配置；这里刻意不调用工具侧页面规则或映射。
	// 目标平台存在瞬断窗口（服务重启、会话互踢），读取失败先做有界重试再暴露错误。
	var snapshot target.PathConfigurationSnapshot
	if err := retryTransientTargetRead(ctx, 3, func(ctx context.Context) error {
		read, readErr := s.target.PathConfigurationSnapshot(ctx, plan.Account, plan.FlowSource, plan.TargetObjectID)
		if readErr != nil {
			return readErr
		}
		snapshot = read
		return nil
	}); err != nil {
		return model.ExecutionPath{}, target.PathConfigurationSnapshot{}, ownedPathAnalysis{}, repository.HistoryPathConfigRecord{}, false, historyWorkspaceSource{}, err
	}
	analysis, err := s.analyzeOwnedPath(ctx, planID, snapshot, path)
	if err != nil {
		return model.ExecutionPath{}, target.PathConfigurationSnapshot{}, ownedPathAnalysis{}, repository.HistoryPathConfigRecord{}, false, historyWorkspaceSource{}, err
	}
	var stored repository.HistoryPathConfigRecord
	found := false
	if s.historyConfigStore != nil {
		stored, found, err = s.historyConfigStore.GetPathConfig(ctx, pathID)
		if err != nil {
			return model.ExecutionPath{}, target.PathConfigurationSnapshot{}, ownedPathAnalysis{}, repository.HistoryPathConfigRecord{}, false, historyWorkspaceSource{}, mapHistoryWorkspaceStoreError(err)
		}
	}
	source, err := s.workspaceSource(ctx, planID, pathID, stored, found, snapshot)
	if err != nil {
		return model.ExecutionPath{}, target.PathConfigurationSnapshot{}, ownedPathAnalysis{}, repository.HistoryPathConfigRecord{}, false, historyWorkspaceSource{}, err
	}
	return path, snapshot, analysis, stored, found, source, nil
}

// workspaceSource 解析路径覆盖、计划默认和未选择三种来源，不冻结动态默认快照。
func (s *PathConfigService) workspaceSource(ctx context.Context, planID, pathID uint64, stored repository.HistoryPathConfigRecord, found bool, snapshot target.PathConfigurationSnapshot) (historyWorkspaceSource, error) {
	if s.historyStore == nil {
		return historyWorkspaceSource{dataSource: model.HistoryDataSource{Mode: model.HistorySourceModeNone, DataStatus: model.HistoryDataStatusEmpty, Issues: []model.HistoryDataIssue{}}}, nil
	}
	mode := strings.TrimSpace(stored.SourceMode)
	var pathSource repository.HistoryPathSourceRecord
	pathSourceFound := false
	if mode == "" {
		var err error
		pathSource, pathSourceFound, err = s.historyStore.GetPathSource(ctx, pathID)
		if err != nil {
			return historyWorkspaceSource{}, mapHistoryWorkspaceStoreError(err)
		}
		if pathSourceFound {
			mode = pathSource.Mode
		}
	}
	if mode == "" {
		mode = model.HistorySourceModeNone
	}
	currentSummary := historyTemplateSummary(snapshot)
	if mode == model.HistorySourceModeNone && !pathSourceFound {
		// 未保存路径覆盖时沿用计划默认来源；只有明确保存 none 才表示路径不使用历史数据。
		if defaultRecord, defaultFound, defaultErr := s.historyStore.GetDefault(ctx, planID); defaultErr != nil {
			return historyWorkspaceSource{}, mapHistoryWorkspaceStoreError(defaultErr)
		} else if defaultFound {
			snapshotValue, snapshotErr := s.historyStore.GetSnapshot(ctx, planID, defaultRecord.SnapshotID)
			if snapshotErr != nil {
				return historyWorkspaceSource{}, mapHistoryWorkspaceStoreError(snapshotErr)
			}
			source := projectHistorySource(model.HistorySourceModeDefault, snapshotValue, defaultRecord.Revision, currentSummary)
			return historyWorkspaceSource{dataSource: source, snapshot: &snapshotValue, dataStatus: source.DataStatus}, nil
		}
	}
	if mode == model.HistorySourceModeDefault {
		defaultRecord, defaultFound, err := s.historyStore.GetDefault(ctx, planID)
		if err != nil {
			return historyWorkspaceSource{}, mapHistoryWorkspaceStoreError(err)
		}
		if !defaultFound {
			return historyWorkspaceSource{dataSource: model.HistoryDataSource{Mode: mode, DataStatus: model.HistoryDataStatusEmpty, Issues: []model.HistoryDataIssue{{Code: "HISTORY_DEFAULT_MISSING", Message: "计划默认基础表单数据尚未设置", Blocking: true}}}}, nil
		}
		snapshotValue, err := s.historyStore.GetSnapshot(ctx, planID, defaultRecord.SnapshotID)
		if err != nil {
			return historyWorkspaceSource{}, mapHistoryWorkspaceStoreError(err)
		}
		source := projectHistorySource(mode, snapshotValue, defaultRecord.Revision, currentSummary)
		return historyWorkspaceSource{dataSource: source, snapshot: &snapshotValue, dataStatus: source.DataStatus}, nil
	}
	if mode != model.HistorySourceModeOverride {
		return historyWorkspaceSource{dataSource: model.HistoryDataSource{Mode: model.HistorySourceModeNone, DataStatus: model.HistoryDataStatusEmpty, Issues: []model.HistoryDataIssue{}}}, nil
	}
	snapshotID := uint64(0)
	revision := uint64(0)
	if found && stored.SnapshotID != nil {
		snapshotID = *stored.SnapshotID
		revision = stored.DataRevision
	} else if pathSourceFound {
		snapshotID = pathSource.SnapshotID
		revision = pathSource.Revision
	}
	if snapshotID == 0 {
		return historyWorkspaceSource{dataSource: model.HistoryDataSource{Mode: mode, DataStatus: model.HistoryDataStatusEmpty, Issues: []model.HistoryDataIssue{{Code: "HISTORY_OVERRIDE_MISSING", Message: "路径独立基础表单数据尚未保存", Blocking: true}}, Revision: revision}}, nil
	}
	snapshotValue, err := s.historyStore.GetSnapshot(ctx, planID, snapshotID)
	if err != nil {
		return historyWorkspaceSource{}, mapHistoryWorkspaceStoreError(err)
	}
	source := projectHistorySource(mode, snapshotValue, revision, currentSummary)
	return historyWorkspaceSource{dataSource: source, snapshot: &snapshotValue, dataStatus: source.DataStatus}, nil
}

// findWorkspacePath 按目标实际 choices 查找已有执行路径，禁止自动创建或按名称猜测。
func (s *PathConfigService) findWorkspacePath(ctx context.Context, planID uint64, choices []model.ExecutionPathChoice) (model.ExecutionPath, bool, error) {
	if matcher, ok := s.pathRepository.(repository.ExecutionPathChoiceMatcher); ok {
		path, found, err := matcher.FindByChoices(ctx, planID, choices)
		if err != nil {
			return model.ExecutionPath{}, false, &PathConfigError{Kind: PathConfigErrorStorage, Message: "执行路径存储暂不可用"}
		}
		return path, found, nil
	}
	paths, err := s.pathRepository.List(ctx, planID)
	if err != nil {
		return model.ExecutionPath{}, false, &PathConfigError{Kind: PathConfigErrorStorage, Message: "执行路径存储暂不可用"}
	}
	for _, candidate := range paths {
		full, getErr := s.pathRepository.Get(ctx, planID, candidate.ID)
		if getErr != nil {
			return model.ExecutionPath{}, false, &PathConfigError{Kind: PathConfigErrorStorage, Message: "执行路径存储暂不可用"}
		}
		if sameWorkspaceChoices(full.Choices, choices) {
			return full, true, nil
		}
	}
	return model.ExecutionPath{}, false, nil
}

// workspaceRecord 把服务端复验后的原始值编码为迁移 022 的独立数据列。
func workspaceRecord(snapshot target.PathConfigurationSnapshot, source historyWorkspaceSource, path model.ExecutionPath, current repository.HistoryPathConfigRecord, idempotencyKey string, input model.PathConfigurationDataInput, overlay branchoverlay.Result, dataStatus string, routeChanged bool) (repository.HistoryPathConfigRecord, error) {
	effective, err := json.Marshal(nonNilWorkspaceMap(overlay.Values))
	if err != nil {
		return repository.HistoryPathConfigRecord{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "目标原始表单数据无法编码"}
	}
	patches, err := json.Marshal(nonNilWorkspacePatches(overlay.Patches))
	if err != nil {
		return repository.HistoryPathConfigRecord{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "分支补丁无法编码"}
	}
	validation, err := json.Marshal(input.RuntimeValidation)
	if err != nil {
		return repository.HistoryPathConfigRecord{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "runtime 校验结果无法编码"}
	}
	issues := appendHistoryIssues(nil, input.RuntimeValidation.Issues)
	issueJSON, err := json.Marshal(issues)
	if err != nil {
		return repository.HistoryPathConfigRecord{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "表单问题无法编码"}
	}
	latest, err := json.Marshal(map[string]any{"idempotencyKey": idempotencyKey})
	if err != nil {
		return repository.HistoryPathConfigRecord{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "幂等结果无法编码"}
	}
	record := current
	record.PathID = path.ID
	record.IdempotencyKey = idempotencyKey
	record.SourceMode = source.dataSource.Mode
	record.RuntimeType = string(snapshot.RenderType)
	record.DataStatus = dataStatus
	record.EffectiveFormData, record.BranchPatches = effective, patches
	record.RuntimeValidation, record.Issues, record.LatestIdempotency = validation, issueJSON, latest
	if source.dataSource.Mode == model.HistorySourceModeOverride && source.snapshot != nil {
		id := source.snapshot.ID
		record.SnapshotID = &id
	} else {
		// default 来源必须保持动态继承，不能把当前默认快照 ID 固化到路径配置。
		record.SnapshotID = nil
	}
	if record.ConfigStatus == "" {
		record.ConfigStatus = "pending"
	}
	if record.NodeStatus == "" {
		record.NodeStatus = "pending"
	}
	if routeChanged {
		record.ConfigStatus, record.NodeStatus = "affected", "affected"
	}
	if len(record.UserActions) == 0 || string(record.UserActions) == `{}` {
		// 动作列按有序独立记录保存；空值必须是数组，避免后续场景编译把对象误判为损坏正文。
		record.UserActions = []byte(`[]`)
	}
	if record.PersonStrategies == nil {
		record.PersonStrategies = []byte(`{}`)
	}
	if record.CompiledSteps == nil {
		record.CompiledSteps = []byte(`[]`)
	}
	if record.ConfirmedNodeKeys == nil {
		record.ConfirmedNodeKeys = []byte(`[]`)
	}
	return record, nil
}

// workspaceDataResult 投影保存后的实际路径和 runtime 原始值，不返回目标内部标识。
func workspaceDataResult(snapshot target.PathConfigurationSnapshot, analysis ownedPathAnalysis, path model.ExecutionPath, stored repository.HistoryPathConfigRecord, routeChanged bool, validation model.HistoryRuntimeValidation, issues []model.HistoryDataIssue, overlay branchoverlay.Result) model.PathConfigurationDataResult {
	template, _ := workspaceRuntimeTemplate(snapshot)
	return model.PathConfigurationDataResult{
		Path: pathConfigPath(path), Revision: stored.Revision, DataRevision: stored.DataRevision, DataStatus: stored.DataStatus,
		RuntimeType: string(snapshot.RenderType), RuntimeTemplate: template, RuntimePage: projectVueCustomPage(snapshot.VuePage),
		RuntimePermissions: workspacePermissions(snapshot, analysis), RuntimeReadRequests: workspaceReadRequests(snapshot, template),
		EffectiveFormData: cloneWorkspaceMap(overlay.Values), BranchPatches: append([]model.HistoryBranchPatch(nil), overlay.Patches...), RuntimeValidation: validation,
		Issues: append([]model.HistoryDataIssue(nil), issues...), RouteChanged: routeChanged,
	}
}

// workspaceRouteChange 构造确认框所需的路径摘要与覆盖、人员、动作影响说明。
func workspaceRouteChange(from, to model.ExecutionPath, targetFound bool, target repository.HistoryPathConfigRecord) model.PathConfigurationRouteChange {
	affected := []model.PathConfigAffectedItem{
		{Kind: "person", Name: "目标路径人员", Reason: "换路后需要重新核对人员策略"},
		{Kind: "action", Name: "目标路径动作", Reason: "换路后需要重新核对动作编排"},
	}
	warning := "确认后将把当前编辑数据保存到实际命中的目标路径"
	if targetFound && len(target.EffectiveFormData) > 2 {
		warning = "目标路径已有业务表单数据，确认后将覆盖该数据，并把人员和动作标记为受影响"
	}
	return model.PathConfigurationRouteChange{From: pathConfigPath(from), To: pathConfigPath(to), OverwritesData: targetFound && len(target.EffectiveFormData) > 2, Affected: affected, Warning: warning}
}

// workspaceConfirmationToken 为当前计划、路径、正文摘要和双方修订生成一次性确认令牌。
func workspaceConfirmationToken(planID, fromPathID, toPathID uint64, values map[string]any, fromRevision, toRevision uint64) string {
	payload, _ := json.Marshal(values)
	sum := sha256.Sum256(payload)
	seed := fmt.Sprintf("f012-route-confirm:%d:%d:%d:%d:%d:%s", planID, fromPathID, toPathID, fromRevision, toRevision, hex.EncodeToString(sum[:]))
	digest := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(digest[:])
}

// workspaceDataStatus 只在 runtime 接受且目标路径复验通过时报告 ready。
func workspaceDataStatus(validation model.HistoryRuntimeValidation, issues []model.HistoryDataIssue) string {
	if !validation.Accepted || hasBlockingWorkspaceIssue(issues) {
		return model.HistoryDataStatusNeedsInput
	}
	return model.HistoryDataStatusReady
}

// hasBlockingWorkspaceIssue 检查 runtime 和分支复验返回的问题是否仍阻断保存完成。
func hasBlockingWorkspaceIssue(issues []model.HistoryDataIssue) bool {
	for _, issue := range issues {
		if issue.Blocking {
			return true
		}
	}
	return false
}

// workspaceRuntimeTemplate 只解析目标返回的 FormMaking 原文；自定义页面不在工具侧重建模板。
func workspaceRuntimeTemplate(snapshot target.PathConfigurationSnapshot) (map[string]any, []string) {
	if snapshot.RenderType != target.FormRenderTypeFormMaking {
		return map[string]any{}, []string{}
	}
	return runtimeTemplate(snapshot.Forms)
}

// workspacePermissions 将目标流程节点权限或目标页面原始权限交给复制的 runtime。
func workspacePermissions(snapshot target.PathConfigurationSnapshot, analysis ownedPathAnalysis) []model.PathFormPermission {
	if snapshot.RenderType == target.FormRenderTypeVueCustom {
		return vueCustomFormPermissions(snapshot.VuePage)
	}
	return formPermissions(snapshot.Tree, analysis.pathAnalysis.ReachableNodeIDs)
}

// workspaceReadRequests 仅保留目标运行时声明的只读请求，不把提交接口带入 iframe。
func workspaceReadRequests(snapshot target.PathConfigurationSnapshot, template map[string]any) []model.PathFormReadRequest {
	if snapshot.RenderType == target.FormRenderTypeVueCustom {
		requests := make([]model.PathFormReadRequest, 0)
		if snapshot.VuePage == nil {
			return requests
		}
		for _, request := range snapshot.VuePage.ReadRequests {
			if request.ReadOnly && strings.TrimSpace(request.Path) != "" {
				requests = append(requests, model.PathFormReadRequest{Method: strings.ToUpper(request.Method), Path: request.Path, Source: "target_runtime"})
			}
		}
		return requests
	}
	return projectPathFormReadRequests(snapshot, template)
}

// pathConfigPath 只投影路径序号和名称，避免数据工作区返回内部路径标识。
func pathConfigPath(path model.ExecutionPath) model.PathConfigPath {
	return model.PathConfigPath{SequenceNo: path.SequenceNo, Name: path.Name}
}

// defaultWorkspaceStatus 返回独立节点状态默认值。
func defaultWorkspaceStatus(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// sameWorkspaceChoices 按真实路由键比较 choices，忽略目标接口返回顺序差异。
func sameWorkspaceChoices(left, right []model.ExecutionPathChoice) bool {
	if len(left) != len(right) {
		return false
	}
	orderedLeft := append([]model.ExecutionPathChoice(nil), left...)
	orderedRight := append([]model.ExecutionPathChoice(nil), right...)
	sort.Slice(orderedLeft, func(i, j int) bool {
		if orderedLeft[i].RouteNodeID == orderedLeft[j].RouteNodeID {
			return orderedLeft[i].BranchID < orderedLeft[j].BranchID
		}
		return orderedLeft[i].RouteNodeID < orderedLeft[j].RouteNodeID
	})
	sort.Slice(orderedRight, func(i, j int) bool {
		if orderedRight[i].RouteNodeID == orderedRight[j].RouteNodeID {
			return orderedRight[i].BranchID < orderedRight[j].BranchID
		}
		return orderedRight[i].RouteNodeID < orderedRight[j].RouteNodeID
	})
	for index := range orderedLeft {
		if orderedLeft[index] != orderedRight[index] {
			return false
		}
	}
	return true
}

// mapHistoryWorkspaceStoreError 将新配置仓储错误收敛为稳定路径配置错误。
func mapHistoryWorkspaceStoreError(err error) error {
	switch {
	case errors.Is(err, repository.ErrHistoryPathConfigConflict), errors.Is(err, repository.ErrHistoryPathConfigIdempotency), errors.Is(err, repository.ErrHistoryRevisionConflict):
		return &PathConfigError{Kind: PathConfigErrorRevisionConflict, Message: "路径表单数据已被其他操作更新，请刷新后重试"}
	case errors.Is(err, repository.ErrExecutionPathNotFound):
		return &PathConfigError{Kind: PathConfigErrorNotFound, Message: "执行路径不存在"}
	case errors.Is(err, repository.ErrHistoryPathConfigDataInvalid):
		return &PathConfigError{Kind: PathConfigErrorStorage, Message: "路径表单数据异常，请重试"}
	default:
		return &PathConfigError{Kind: PathConfigErrorStorage, Message: "路径数据存储暂不可用"}
	}
}

// decodeWorkspaceMap 解码独立数据列；损坏正文返回空值并由上层标记存储异常。
func decodeWorkspaceMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	value, err := jsonvalues.DecodeObject(raw)
	if err != nil {
		return map[string]any{}
	}
	return value
}

// decodeWorkspacePatches 解码分支补丁列表并保持空数组稳定。
func decodeWorkspacePatches(raw []byte) []model.HistoryBranchPatch {
	var value []model.HistoryBranchPatch
	if err := jsonvalues.Decode(raw, &value); err != nil || value == nil {
		return []model.HistoryBranchPatch{}
	}
	return value
}

// decodeWorkspaceIssues 解码 runtime 问题列表并保持空数组稳定。
func decodeWorkspaceIssues(raw []byte) []model.HistoryDataIssue {
	var value []model.HistoryDataIssue
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return []model.HistoryDataIssue{}
	}
	return value
}

// decodeWorkspaceValidation 解码 runtime 结构化校验结果。
func decodeWorkspaceValidation(raw []byte) model.HistoryRuntimeValidation {
	var value model.HistoryRuntimeValidation
	if err := json.Unmarshal(raw, &value); err != nil {
		return model.HistoryRuntimeValidation{}
	}
	return value
}

// decodeWorkspaceActions 解码保存的有序用户动作，损坏正文不向页面伪造动作。
func decodeWorkspaceActions(raw []byte) []model.ConfiguredAction {
	var value []model.ConfiguredAction
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return []model.ConfiguredAction{}
	}
	return value
}

// decodeWorkspaceSteps 解码服务端已编译的动作步骤。
func decodeWorkspaceSteps(raw []byte) []model.CompiledActionStep {
	var value []model.CompiledActionStep
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return []model.CompiledActionStep{}
	}
	return value
}

// cloneWorkspaceMap 深复制 runtime 原始值，防止分支复验修改历史快照正文。
func cloneWorkspaceMap(value map[string]any) map[string]any {
	result, err := jsonvalues.DeepCopyObject(value)
	if err != nil {
		return map[string]any{}
	}
	return result
}

// nonNilWorkspaceMap 保证原始 JSON 对象编码为空对象而不是 null。
func nonNilWorkspaceMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

// nonNilWorkspacePatches 保证分支补丁编码为空数组而不是 null。
func nonNilWorkspacePatches(value []model.HistoryBranchPatch) []model.HistoryBranchPatch {
	if value == nil {
		return []model.HistoryBranchPatch{}
	}
	return value
}

// appendHistoryIssues 按 code、path 和 message 去重结构化问题。
func appendHistoryIssues(base []model.HistoryDataIssue, extra []model.HistoryDataIssue) []model.HistoryDataIssue {
	result := append([]model.HistoryDataIssue(nil), base...)
	seen := make(map[string]bool, len(result)+len(extra))
	for _, issue := range result {
		seen[issue.Code+"\x00"+issue.Path+"\x00"+issue.Message] = true
	}
	for _, issue := range extra {
		key := issue.Code + "\x00" + issue.Path + "\x00" + issue.Message
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, issue)
	}
	return result
}

// historyIssuesFromStrings 将目标运行时异常转换为可定位的历史数据问题。
func historyIssuesFromStrings(code string, messages []string) []model.HistoryDataIssue {
	result := make([]model.HistoryDataIssue, 0, len(messages))
	for _, message := range messages {
		if text := strings.TrimSpace(message); text != "" {
			result = append(result, model.HistoryDataIssue{Code: code, Message: text, Blocking: true})
		}
	}
	return result
}

// historyIssuesFromOverlay 把分支模块的原始条件问题投影为数据工作区问题。
func historyIssuesFromOverlay(issues []branchoverlay.Issue) []model.HistoryDataIssue {
	result := make([]model.HistoryDataIssue, 0, len(issues))
	for _, issue := range issues {
		result = append(result, model.HistoryDataIssue{Code: issue.Code, Path: issue.Path, Fields: append([]string(nil), issue.Fields...), Message: issue.Message, Blocking: true})
	}
	return result
}

// affectedFromOverlayIssues 将分支问题定位到路径或表单数据，而不公开目标内部标识。
func affectedFromOverlayIssues(issues []branchoverlay.Issue) []model.PathConfigAffectedItem {
	result := make([]model.PathConfigAffectedItem, 0, len(issues))
	for _, issue := range issues {
		name := "表单数据"
		if strings.TrimSpace(issue.Path) != "" {
			name = "条件字段"
		}
		result = append(result, model.PathConfigAffectedItem{Kind: "form", Name: name, Reason: issue.Message})
	}
	return result
}

// ClearAuditInfoValuesForTest 暴露审批意见清理，供 test 目录下的定向用例锁定行为。
func ClearAuditInfoValuesForTest(values map[string]any) map[string]any {
	return clearAuditInfoValues(values)
}

// KeyFieldLabelsForTest 暴露字段中文名称映射，供 test 目录下的定向用例锁定行为。
func KeyFieldLabelsForTest(snapshot target.PathConfigurationSnapshot) map[string]string {
	return keyFieldLabels(snapshot)
}

// retryTransientTargetRead 对目标平台读取做有界重试：目标服务重启或会话重登的短暂窗口内，
// 读取会以不可用、超时、会话失效甚至登录被拒失败（目标网关在互踢/重启窗口会短暂返回错误凭据类业务失败，
// 稍后同一凭据即可成功），立即暴露给用户就是一次"暂时无法读取"的无效阻断。只读调用最多重试
// attempts-1 次，间隔线性增长；持续故障仍在末次失败后原样返回，不被重试掩盖。
func retryTransientTargetRead(ctx context.Context, attempts int, call func(context.Context) error) error {
	if attempts < 1 {
		attempts = 1
	}
	var last error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(400*attempt) * time.Millisecond):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		last = call(ctx)
		if last == nil || !isTransientTargetReadError(last) {
			return last
		}
	}
	return last
}

// isTransientTargetReadError 判定目标读取错误是否属于可重试的瞬断窗口。
func isTransientTargetReadError(err error) bool {
	return target.IsKind(err, target.ErrorUnavailable) ||
		target.IsKind(err, target.ErrorTimeout) ||
		target.IsKind(err, target.ErrorSessionExpired) ||
		target.IsKind(err, target.ErrorLoginRejected)
}

// historyInstanceIdentifiers 是按实例绑定版本重读宿主配置所需的实例标识。
type historyInstanceIdentifiers struct {
	InstanceID   string
	FlowProxyID  string
	FormProxyIDs []string
}

// historyInstanceIdentifiersFromSummary 从快照实例摘要读取实例标识；旧快照没有这些键时返回 false，
// 保持按当前模板渲染的既有行为。
func historyInstanceIdentifiersFromSummary(summary map[string]any) (historyInstanceIdentifiers, bool) {
	if summary == nil {
		return historyInstanceIdentifiers{}, false
	}
	instanceID := strings.TrimSpace(templateString(summary["instanceId"]))
	flowProxyID := strings.TrimSpace(templateString(summary["flowProxyId"]))
	formProxyIDs := []string{}
	switch typed := summary["formProxyIds"].(type) {
	case []any:
		for _, item := range typed {
			if id := strings.TrimSpace(templateString(item)); id != "" {
				formProxyIDs = append(formProxyIDs, id)
			}
		}
	case []string:
		for _, id := range typed {
			if id = strings.TrimSpace(id); id != "" {
				formProxyIDs = append(formProxyIDs, id)
			}
		}
	}
	if instanceID == "" || flowProxyID == "" || len(formProxyIDs) == 0 {
		return historyInstanceIdentifiers{}, false
	}
	return historyInstanceIdentifiers{InstanceID: instanceID, FlowProxyID: flowProxyID, FormProxyIDs: formProxyIDs}, true
}

// resolveInstanceBoundSnapshot 在历史来源绑定时按快照携带的实例标识重读实例绑定的宿主配置。
// resolved=false 表示没有历史来源、缺少标识或非 FormMaking 运行时，保持当前模板行为；
// 读取失败返回错误，由调用方回落当前模板并记录问题。
func (s *PathConfigService) resolveInstanceBoundSnapshot(ctx context.Context, planID uint64, source historyWorkspaceSource) (target.PathConfigurationSnapshot, bool, error) {
	if source.snapshot == nil || !strings.EqualFold(strings.TrimSpace(source.snapshot.RuntimeType), string(target.FormRenderTypeFormMaking)) {
		return target.PathConfigurationSnapshot{}, false, nil
	}
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return target.PathConfigurationSnapshot{}, false, err
	}
	reader, readerOK := s.target.(interface {
		ReadProxyConfigurationForInstance(context.Context, string, string, []string, string) (target.PathConfigurationSnapshot, error)
		ResolveHistoryInstanceForSnapshot(context.Context, string, string, string, string, string) (target.HistoryInstance, error)
	})
	if !readerOK {
		return target.PathConfigurationSnapshot{}, false, &PathConfigError{Kind: PathConfigErrorStorage, Message: "目标配置读取服务暂不可用"}
	}
	identity, hasIdentity := historyInstanceIdentifiersFromSummary(source.snapshot.InstanceSummary)
	if !hasIdentity {
		// 旧快照没有实例标识：按候选键重新解析实例身份（与快照采集同一匹配规则），不要求用户重新绑定来源。
		resolved, resolveErr := reader.ResolveHistoryInstanceForSnapshot(ctx, plan.Account,
			source.snapshot.FlowCode, source.snapshot.FormName, source.snapshot.FlowName, source.snapshot.CandidateKey)
		if resolveErr != nil {
			return target.PathConfigurationSnapshot{}, false, resolveErr
		}
		identity = historyInstanceIdentifiers{
			InstanceID: strings.TrimSpace(resolved.ID), FlowProxyID: strings.TrimSpace(resolved.FlowProxyID),
			FormProxyIDs: resolved.FormProxyIDs,
		}
		if identity.InstanceID == "" || identity.FlowProxyID == "" || len(identity.FormProxyIDs) == 0 {
			return target.PathConfigurationSnapshot{}, false, nil
		}
	}
	var result target.PathConfigurationSnapshot
	if err := retryTransientTargetRead(ctx, 3, func(ctx context.Context) error {
		proxySnapshot, snapshotErr := reader.ReadProxyConfigurationForInstance(ctx, plan.Account, identity.FlowProxyID, identity.FormProxyIDs, identity.InstanceID)
		if snapshotErr != nil {
			return snapshotErr
		}
		result = proxySnapshot
		return nil
	}); err != nil {
		return target.PathConfigurationSnapshot{}, false, err
	}
	return result, true, nil
}
