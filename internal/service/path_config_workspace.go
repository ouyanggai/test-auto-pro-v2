package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/formdata"
	"test-auto-pro-v2/internal/model"
)

const currentPathConfigVersion = 3

const currentPathFormConfigVersion = 2

type pathFormRuntimeSessionReader interface {
	FormRuntimeSession(context.Context, string) (target.FormRuntimeSession, error)
}

type pathFormSampleReader interface {
	RecentFormSamples(context.Context, string, int) ([]map[string]any, error)
}

type pathFormIdentityReader interface {
	FormIdentityContext(context.Context, string) (target.FormIdentityContext, error)
}

// RuntimeSession 校验计划与路径归属后返回当前账号缓存的短期 iframe 会话。
func (s *PathConfigService) RuntimeSession(ctx context.Context, planID, pathID uint64) (model.PathFormRuntimeSession, error) {
	if _, err := s.ownedPath(ctx, planID, pathID); err != nil {
		return model.PathFormRuntimeSession{}, err
	}
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return model.PathFormRuntimeSession{}, err
	}
	reader, ok := s.target.(pathFormRuntimeSessionReader)
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
	}, nil
}

// GenerateForm 按当前真实模板、近期样本、发起人和路径条件生成可复现草稿。
func (s *PathConfigService) GenerateForm(ctx context.Context, planID, pathID uint64, seed int64, current map[string]any, manualPaths []string, nextGroup bool) (model.PathFormGenerateResult, error) {
	path, err := s.ownedPath(ctx, planID, pathID)
	if err != nil {
		return model.PathFormGenerateResult{}, err
	}
	if err := s.validateConfigMutablePlan(ctx, planID); err != nil {
		return model.PathFormGenerateResult{}, err
	}
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return model.PathFormGenerateResult{}, err
	}
	if plan.FlowSource != "new" {
		return model.PathFormGenerateResult{}, &PathConfigError{Kind: PathConfigErrorLocked, Message: "已发或待发表单只能查看实例当前值"}
	}
	snapshot, err := s.readVerifiedSnapshot(ctx, planID)
	if err != nil {
		return model.PathFormGenerateResult{}, err
	}
	owned, err := s.analyzeOwnedPath(ctx, planID, snapshot, path)
	if err != nil {
		return model.PathFormGenerateResult{}, err
	}
	template, unsupported := runtimeTemplate(snapshot.Forms)
	if seed == 0 {
		// 首次种子只依赖路径与当前修订，重复读取会得到同一组；换一组由浏览器显式推进种子。
		seed = int64(path.ID*1000003 + uint64(len(path.Choices))*97 + 1)
	}
	stored, found, err := s.configRepository.FindByPath(ctx, pathID)
	if err != nil {
		return model.PathFormGenerateResult{}, mapPathConfigRepositoryError(err)
	}
	base := current
	if len(base) == 0 && found {
		base = stored.FormValues
	}
	samples := make([]map[string]any, 0)
	if reader, ok := s.target.(pathFormSampleReader); ok {
		// 样本读取失败不阻断安全兜底，摘要保持 recent=0，页面会明确提示样本不足。
		if recent, sampleErr := reader.RecentFormSamples(ctx, plan.Account, 5); sampleErr == nil {
			samples = recent
		}
	}
	initiator := plan.Account
	if reader, ok := s.target.(pathFormRuntimeSessionReader); ok {
		if active, sessionErr := reader.FormRuntimeSession(ctx, plan.Account); sessionErr == nil && strings.TrimSpace(active.AccountName) != "" {
			initiator = active.AccountName
		}
	}
	identity := formdata.IdentityContext{}
	if reader, ok := s.target.(pathFormIdentityReader); ok {
		// 身份目录读取失败只影响选人/选公司自动填充，不阻断其他基础字段生成。
		if active, identityErr := reader.FormIdentityContext(ctx, plan.Account); identityErr == nil {
			identity = formdataIdentityContext(active)
		}
	}
	permissions := formPermissions(snapshot.Tree, formPermissionNodeIDs(plan.FlowSource, snapshot, owned.pathAnalysis.ReachableNodeIDs))
	dateRangeBindings := buildPathDateRangeBindings(snapshot.Tree, path.Choices, template)
	generated := formdata.Generate(formdata.GenerateInput{
		Template: template, Base: base, Samples: samples, Seed: seed, Initiator: initiator,
		Constraints: buildPathConstraints(snapshot.Tree, path.Choices), ManualOverridePaths: manualPaths,
		DateRangeBindings: dateRangeBindings,
		EditablePaths:     editableFormPathsFromPermissions(permissions),
		Identity:          identity,
	})
	generated.Unsupported = append(generated.Unsupported, unsupported...)
	if reasons := validateTargetPathSelection(snapshot.Tree, path.Choices, generated.Values); len(reasons) > 0 {
		return model.PathFormGenerateResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "生成数据无法命中当前已选分支", Affected: affectedFromStrings("form", reasons)}
	}
	if reasons := formdata.ValidateDateRangeBindings(generated.Values, dateRangeBindings); len(reasons) > 0 {
		return model.PathFormGenerateResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "生成日期区间无法满足当前条件天数", Affected: affectedFromStrings("form", reasons)}
	}
	hints := buildPathConditionHints(snapshot.Tree, path.Choices, template, permissions, generated.Values)
	if nextGroup && reflect.DeepEqual(base, generated.Values) {
		// “换一组”只能在当前模板、样本和路径约束能证明产生新值时成功，避免旧值被伪装成新候选。
		return model.PathFormGenerateResult{}, &PathConfigError{Kind: PathConfigErrorInvalidArgument, Message: "没有可切换的下一组有效候选，当前数据已保持不变"}
	}
	return model.PathFormGenerateResult{
		Revision: stored.FormRevision, Status: "draft", Values: generated.Values, Seed: seed,
		GeneratedFieldPaths: generated.GeneratedFieldPaths, ManualOverridePaths: generated.ManualOverridePaths,
		SampleSummary: model.PathFormSampleSummary{Saved: found && len(stored.FormValues) > 0, Defaults: generated.Defaults, Recent: generated.Recent, Fallback: generated.Fallback, Identity: generated.Identity},
		AutoFilled:    len(generated.GeneratedFieldPaths), ManualPending: generated.Pending,
		Unsupported:    uniquePublicStrings(generated.Unsupported),
		ConditionHints: hints, FieldRules: buildPathFormFieldRules(hints),
	}, nil
}

// SaveNode 只合并当前节点的人员与动作，并用节点确认键推进权威进度。
func (s *PathConfigService) SaveNode(ctx context.Context, planID, pathID uint64, nodeKey, idempotencyKey string, input model.PathNodeSaveInput) (model.PathConfigSaveResult, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	nodeKey = strings.TrimSpace(nodeKey)
	if nodeKey == "" || !validUUID(idempotencyKey) {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalidArgument, Message: "节点或保存标识不正确"}
	}
	path, err := s.ownedPath(ctx, planID, pathID)
	if err != nil {
		return model.PathConfigSaveResult{}, err
	}
	if err := s.validateConfigMutablePlan(ctx, planID); err != nil {
		return model.PathConfigSaveResult{}, err
	}
	if existing, found, err := s.configRepository.FindByPathAndKey(ctx, pathID, idempotencyKey); err != nil {
		return model.PathConfigSaveResult{}, mapPathConfigRepositoryError(err)
	} else if found {
		return pathConfigSaveResult(path, existing), nil
	}
	stored, found, err := s.configRepository.FindByPath(ctx, pathID)
	if err != nil {
		return model.PathConfigSaveResult{}, mapPathConfigRepositoryError(err)
	}
	if input.Revision != stored.NodeRevision {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorRevisionConflict, Message: "节点配置已被其他操作更新，请刷新后重试"}
	}
	snapshot, err := s.readVerifiedSnapshot(ctx, planID)
	if err != nil {
		return model.PathConfigSaveResult{}, err
	}
	owned, err := s.analyzeOwnedPath(ctx, planID, snapshot, path)
	if err != nil {
		return model.PathConfigSaveResult{}, err
	}
	_, validation, err := s.configAnalyzer.Analyze(
		owned.graph, snapshot.Tree, snapshot.FormFields, path, owned.pathAnalysis,
		snapshot.InstanceValues, map[string]map[string]string{}, stored.ActionValues, found,
	)
	if err != nil {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前节点无法投影，请重新读取"}
	}
	nodeValidation := analyzer.PathConfigValidation{FieldTokens: map[string]analyzer.PathConfigFieldTarget{}, ActionTokens: map[string]analyzer.PathConfigActionTarget{}, NodeTokens: map[string]analyzer.PathConfigNodeTarget{}}
	for key, action := range validation.ActionTokens {
		if analyzer.PathConfigNodeToken(action.NodeID) == nodeKey {
			nodeValidation.ActionTokens[key] = action
		}
	}
	if nodeTarget, exists := validation.NodeTokens[nodeKey]; exists {
		nodeValidation.NodeTokens[nodeKey] = nodeTarget
	}
	if len(nodeValidation.ActionTokens) == 0 && len(nodeValidation.NodeTokens) == 0 {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalidArgument, Message: "当前节点没有可保存配置"}
	}
	nodeActions := make(map[string]string)
	if strings.TrimSpace(input.ActionPlan.Result.Kind) != "" || len(input.ActionPlan.Actions) > 0 || input.ActionPlan.CombinationCount > 0 || len(input.ActionPlan.AddSignNodes) > 0 || len(input.Persons) > 0 {
		nodeTarget, exists := nodeValidation.NodeTokens[nodeKey]
		if !exists {
			return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前节点动作目录已变化，请重新读取"}
		}
		nodeActions, err = validatePathConfigNodeSubmission(nodeTarget, input)
		if err != nil {
			return model.PathConfigSaveResult{}, err
		}
	} else {
		// 兼容已部署页面的旧最小动作数组；新页面只使用人员策略和动作列表。
		_, nodeActions, err = validatePathConfigSubmission(nodeValidation, nil, input.Actions)
		if err != nil {
			return model.PathConfigSaveResult{}, err
		}
	}
	if stored.ActionValues == nil {
		stored.ActionValues = map[string]string{}
	}
	for storageKey, value := range nodeActions {
		stored.ActionValues[storageKey] = value
	}
	if strings.TrimSpace(input.ActionPlan.Result.Kind) != "" || len(input.ActionPlan.Actions) > 0 || input.ActionPlan.CombinationCount > 0 || len(input.ActionPlan.AddSignNodes) > 0 || len(input.Persons) > 0 {
		nodeTarget := nodeValidation.NodeTokens[nodeKey]
		// 新格式成为当前节点的权威值后移除旧键，刷新时不会出现两套语义竞争。
		delete(stored.ActionValues, nodeTarget.NodeID)
		delete(stored.ActionValues, analyzer.PathConfigPersonStorageKey(nodeTarget.NodeID))
	}
	if _, valid := analyzer.CountStoredPathConfigActionSteps(stored.ActionValues); !valid {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "整条路径的动作总数不能超过 100 个"}
	}
	stored.ConfirmedNodeKeys = appendUnique(stored.ConfirmedNodeKeys, nodeKey)
	stored.PathID = pathID
	stored.Revision++
	stored.NodeRevision++
	stored.IdempotencyKey = idempotencyKey
	stored.ConfigVersion = currentPathConfigVersion
	stored.Status = s.deriveStoredStatus(ctx, planID, path, snapshot, stored)
	if !found {
		stored.FormStatus = initialStoredFormStatus(snapshot)
	}
	saved, err := s.configRepository.Save(ctx, stored, stored.Revision-1, s.now().UTC())
	if err != nil {
		return model.PathConfigSaveResult{}, mapPathConfigRepositoryError(err)
	}
	return pathConfigSaveResult(path, saved), nil
}

// SaveForm 在真实运行时与服务端双重校验后独立保存完整 getValues 数据和生成元数据。
func (s *PathConfigService) SaveForm(ctx context.Context, planID, pathID uint64, idempotencyKey string, input model.PathFormSaveInput) (model.PathConfigSaveResult, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !validUUID(idempotencyKey) || !input.Validated {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalidArgument, Message: "表单尚未通过真实运行时校验"}
	}
	path, err := s.ownedPath(ctx, planID, pathID)
	if err != nil {
		return model.PathConfigSaveResult{}, err
	}
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return model.PathConfigSaveResult{}, err
	}
	if plan.FlowSource != "new" {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorLocked, Message: "已发或待发表单只能查看实例当前值"}
	}
	if existing, found, err := s.configRepository.FindByPathAndKey(ctx, pathID, idempotencyKey); err != nil {
		return model.PathConfigSaveResult{}, mapPathConfigRepositoryError(err)
	} else if found {
		return pathConfigSaveResult(path, existing), nil
	}
	if unsupported := uniquePublicStrings(input.Unsupported); len(unsupported) > 0 {
		// 组件注册表只存在于真实 rsh-flow-components 运行时；幂等命中后再校验，既不重写成功事实，也不另建易漂移白名单。
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前表单含未适配组件，不能保存为可执行配置", Affected: affectedFromStrings("form", unsupported)}
	}
	stored, _, err := s.configRepository.FindByPath(ctx, pathID)
	if err != nil {
		return model.PathConfigSaveResult{}, mapPathConfigRepositoryError(err)
	}
	if input.Revision != stored.FormRevision {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorRevisionConflict, Message: "表单数据已被其他操作更新，请刷新后重试"}
	}
	snapshot, err := s.readVerifiedSnapshot(ctx, planID)
	if err != nil {
		return model.PathConfigSaveResult{}, err
	}
	owned, err := s.analyzeOwnedPath(ctx, planID, snapshot, path)
	if err != nil {
		return model.PathConfigSaveResult{}, err
	}
	template, unsupported := runtimeTemplate(snapshot.Forms)
	if len(unsupported) > 0 {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前表单结构无法载入", Affected: affectedFromStrings("form", unsupported)}
	}
	if reasons := formdata.ValidateEditable(template, input.Values, buildPathConstraints(snapshot.Tree, path.Choices), editableFormPaths(snapshot.Tree, formPermissionNodeIDs(plan.FlowSource, snapshot, owned.pathAnalysis.ReachableNodeIDs))); len(reasons) > 0 {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "表单数据不符合当前模板或路径条件", Affected: affectedFromStrings("form", reasons)}
	}
	if reasons := validateTargetPathSelection(snapshot.Tree, path.Choices, input.Values); len(reasons) > 0 {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "表单数据无法命中当前已选分支", Affected: affectedFromStrings("form", reasons)}
	}
	if reasons := formdata.ValidateDateRangeBindings(input.Values, buildPathDateRangeBindings(snapshot.Tree, path.Choices, template)); len(reasons) > 0 {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "日期区间与当前条件天数不一致", Affected: affectedFromStrings("form", reasons)}
	}
	stored.PathID = pathID
	stored.Revision++
	stored.FormRevision++
	stored.IdempotencyKey = idempotencyKey
	stored.ConfigVersion = currentPathConfigVersion
	stored.FormValues = cloneFormValues(input.Values)
	stored.FormStatus = "valid"
	stored.FormValidated = true
	stored.FormSeed = input.Seed
	stored.GeneratedFieldPaths = uniquePublicStrings(input.GeneratedFieldPaths)
	stored.ManualOverridePaths = uniquePublicStrings(input.ManualOverridePaths)
	stored.SampleSummary = input.SampleSummary
	stored.FormTemplateVersion = formdata.TemplateVersion(template)
	stored.SampleSummary.Saved = true
	stored.Status = s.deriveStoredStatus(ctx, planID, path, snapshot, stored)
	saved, err := s.configRepository.Save(ctx, stored, stored.Revision-1, s.now().UTC())
	if err != nil {
		return model.PathConfigSaveResult{}, mapPathConfigRepositoryError(err)
	}
	return pathConfigSaveResult(path, saved), nil
}

// projectPathForm 把当前真实模板、权限、实例值和已保存元数据投影为 iframe 工作区。
func projectPathForm(source string, snapshot target.PathConfigurationSnapshot, analysis model.ExecutionPathAnalysis, choices []model.ExecutionPathChoice, stored model.StoredPathConfig, found bool) model.PathFormConfig {
	template, unsupported := runtimeTemplate(snapshot.Forms)
	form := model.PathFormConfig{
		Revision: stored.FormRevision, Status: "empty", StatusName: "待配置",
		ReadOnly: source != "new", Template: template, Permissions: formPermissions(snapshot.Tree, formPermissionNodeIDs(source, snapshot, analysis.ReachableNodeIDs)),
		Values: map[string]any{}, GeneratedFieldPaths: []string{}, ManualOverridePaths: []string{},
		Unsupported: uniquePublicStrings(unsupported), Affected: []model.PathConfigAffectedItem{},
		ConditionHints: []model.PathFormConditionHint{},
	}
	if found && len(stored.FormValues) > 0 {
		form.Values = cloneFormValues(stored.FormValues)
		form.Seed = stored.FormSeed
		form.GeneratedFieldPaths = append([]string(nil), stored.GeneratedFieldPaths...)
		form.ManualOverridePaths = append([]string(nil), stored.ManualOverridePaths...)
		form.SampleSummary = stored.SampleSummary
		form.Validated = stored.FormValidated
		form.Status = stored.FormStatus
	}
	if len(form.Values) == 0 && len(snapshot.InstanceValues) > 0 {
		form.Values = cloneFormValues(snapshot.InstanceValues)
	}
	// 提示和禁用必须基于当前实际 values 复算；未生成或无法证明命中时仅保留普通条件说明。
	form.ConditionHints = buildPathConditionHints(snapshot.Tree, choices, template, form.Permissions, form.Values)
	form.FieldRules = buildPathFormFieldRules(form.ConditionHints)
	if len(snapshot.Forms) == 0 {
		form.Status, form.StatusName, form.Validated = "valid", "已配置", true
		return form
	}
	if len(form.Unsupported) > 0 {
		form.Status, form.StatusName = "unsupported", "待配置"
		form.Affected = affectedFromStrings("form", form.Unsupported)
		return form
	}
	version := formdata.TemplateVersion(template)
	if found && stored.ConfigVersion < currentPathFormConfigVersion {
		form.Status, form.StatusName = "affected", "部分配置"
		form.Affected = []model.PathConfigAffectedItem{{Kind: "form", Name: "表单数据", Reason: "旧配置未保存完整 FormMaking values"}}
		return form
	}
	if found && stored.FormTemplateVersion != "" && stored.FormTemplateVersion != version {
		form.Status, form.StatusName = "affected", "部分配置"
		form.Affected = []model.PathConfigAffectedItem{{Kind: "form", Name: "表单数据", Reason: "目标表单模板已变化，需要重新校验"}}
		return form
	}
	if form.ReadOnly {
		form.Status, form.StatusName, form.Validated = "valid", "已配置", true
		return form
	}
	switch form.Status {
	case "valid":
		form.StatusName = "已配置"
	case "draft":
		form.StatusName = "部分配置"
	case "affected":
		form.StatusName = "部分配置"
	default:
		form.Status, form.StatusName = "empty", "待配置"
	}
	return form
}

// formPermissionNodeIDs 为新发起只选择真实入口节点权限；下游审批节点的 edit 不能提前开放到发起表单。
func formPermissionNodeIDs(source string, snapshot target.PathConfigurationSnapshot, reachable []string) []string {
	if strings.TrimSpace(source) != "new" {
		return reachable
	}
	if len(snapshot.EntryNodeIDs) > 0 {
		return append([]string(nil), snapshot.EntryNodeIDs...)
	}
	if snapshot.Tree != nil && strings.TrimSpace(snapshot.Tree.ID) != "" {
		return []string{snapshot.Tree.ID}
	}
	return []string{}
}

// applyConfirmedNodeState 用节点确认事实覆盖旧的“一行存在即全部完成”投影语义。
func applyConfirmedNodeState(configuration *model.PathConfiguration, confirmedKeys []string) {
	confirmed := make(map[string]bool, len(confirmedKeys))
	for _, key := range confirmedKeys {
		confirmed[key] = true
	}
	for groupIndex := range configuration.Groups {
		for nodeIndex := range configuration.Groups[groupIndex].Nodes {
			node := &configuration.Groups[groupIndex].Nodes[nodeIndex]
			if node.Status == "affected" || node.Status == "partial" || node.LineBlocked {
				continue
			}
			requiresSave := false
			for _, action := range node.ActionPlan.Catalog {
				if action.Enabled {
					requiresSave = true
					break
				}
			}
			for _, person := range node.Persons {
				requiresSave = requiresSave || person.Editable
			}
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

// derivePathConfigurationStatus 只在表单有效且所有必需节点完成时返回 configured。
func derivePathConfigurationStatus(configuration model.PathConfiguration) string {
	if configuration.Form.Status == "affected" || configuration.Form.Status == "unsupported" {
		return "affected"
	}
	for _, group := range configuration.Groups {
		for _, node := range group.Nodes {
			if node.Status == "affected" {
				return "affected"
			}
		}
	}
	if configuration.Form.Status == "valid" && configuration.Progress.Pending == 0 {
		return "configured"
	}
	return "pending"
}

// deriveStoredStatus 用当前真实投影重新计算路径列表的轻量本地状态。
func (s *PathConfigService) deriveStoredStatus(ctx context.Context, planID uint64, path model.ExecutionPath, snapshot target.PathConfigurationSnapshot, stored model.StoredPathConfig) string {
	owned, err := s.analyzeOwnedPath(ctx, planID, snapshot, path)
	if err != nil {
		return "affected"
	}
	configuration, _, err := s.configAnalyzer.Analyze(
		owned.graph, snapshot.Tree, snapshot.FormFields, path, owned.pathAnalysis,
		snapshot.InstanceValues, stored.FieldValues, stored.ActionValues, true,
	)
	if err != nil {
		return "affected"
	}
	applyConfirmedNodeState(&configuration, stored.ConfirmedNodeKeys)
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return "affected"
	}
	configuration.Form = projectPathForm(plan.FlowSource, snapshot, owned.pathAnalysis, path.Choices, stored, true)
	return derivePathConfigurationStatus(configuration)
}

// runtimeTemplate 只解析单一完整模板；组件兼容性必须以真实 rsh-flow-components 注册表为准。
func runtimeTemplate(forms []target.FormRuntimeTemplate) (map[string]any, []string) {
	if len(forms) == 0 {
		return map[string]any{"list": []any{}, "config": map[string]any{}}, nil
	}
	unsupported := make([]string, 0)
	if len(forms) > 1 {
		unsupported = append(unsupported, "当前路径关联多个表单，尚未建立独立多表单适配")
	}
	template := make(map[string]any)
	if err := json.Unmarshal([]byte(forms[0].TemplateData), &template); err != nil {
		return map[string]any{"list": []any{}, "config": map[string]any{}}, append(unsupported, "目标 FormMaking 模板无法解析")
	}
	return template, uniquePublicStrings(unsupported)
}

// formPermissions 合并当前路径字段权限；任一可达节点允许编辑时保持 edit，否则只读或隐藏。
func formPermissions(tree *target.FlowNodeTemplate, reachable []string) []model.PathFormPermission {
	reachableSet := make(map[string]bool, len(reachable))
	for _, id := range reachable {
		reachableSet[id] = true
	}
	powers := make(map[string]string)
	visitTargetTree(tree, map[string]bool{}, func(node *target.FlowNodeTemplate) {
		if !reachableSet[node.ID] {
			return
		}
		for _, power := range node.FieldPowers {
			field := normalizeFormFieldPath(power.EnglishName)
			if field == "" {
				continue
			}
			current := powers[field]
			if power.Power == "edit" || current == "" {
				powers[field] = power.Power
			}
		}
	})
	result := make([]model.PathFormPermission, 0, len(powers))
	for field, power := range powers {
		if power != "edit" && power != "hide" {
			power = "only_read"
		}
		result = append(result, model.PathFormPermission{Field: field, Power: power})
	}
	return result
}

// editableFormPaths 提取当前路径任一节点明确授予 edit 的字段，未授权字段由真实运行时保持只读或隐藏。
func editableFormPaths(tree *target.FlowNodeTemplate, reachable []string) map[string]bool {
	return editableFormPathsFromPermissions(formPermissions(tree, reachable))
}

// editableFormPathsFromPermissions 从已投影权限中提取可编辑字段，保证生成、校验和运行时使用相同字段路径规范。
func editableFormPathsFromPermissions(permissions []model.PathFormPermission) map[string]bool {
	result := make(map[string]bool)
	for _, permission := range permissions {
		if permission.Power == "edit" {
			result[permission.Field] = true
		}
	}
	return result
}

// buildPathConstraints 把当前选择的条件分支转为生成和服务端复验共用的基本约束。
func buildPathConstraints(tree *target.FlowNodeTemplate, choices []model.ExecutionPathChoice) []formdata.Constraint {
	selected := make(map[string]string, len(choices))
	for _, choice := range choices {
		selected[choice.RouteNodeID] = choice.BranchID
	}
	constraints := make([]formdata.Constraint, 0)
	visitTargetTree(tree, map[string]bool{}, func(node *target.FlowNodeTemplate) {
		branchID := selected[node.ID]
		if branchID == "" {
			return
		}
		branches := node.ConditionNodes
		if len(branches) == 0 {
			branches = node.ParallelNodes
		}
		for _, branch := range branches {
			if branch.ID != branchID {
				continue
			}
			if len(branch.Conditions) == 0 {
				// 无显式条件的分支不能反向猜测其他分支的字段值；实际选支只能由后续完整复验得出。
				continue
			}
			orGroup := 0
			for index, condition := range branch.Conditions {
				if strings.TrimSpace(condition.FieldA) == "" || strings.TrimSpace(condition.FieldB) != "" {
					continue
				}
				group := 0
				previousOR := index > 0 && strings.EqualFold(strings.TrimSpace(branch.Conditions[index-1].ConditionType), "or")
				currentOR := strings.EqualFold(strings.TrimSpace(condition.ConditionType), "or")
				if previousOR || currentOR {
					if !previousOR {
						orGroup++
					}
					group = orGroup
				}
				constraints = append(constraints, formdata.Constraint{
					Field: normalizeFormFieldPath(condition.FieldA), Op: normalizeConditionJudge(condition.Judge), Value: pathConditionValue(condition.ValueB), Group: group,
				})
			}
		}
	})
	return constraints
}

// buildPathDateRangeBindings 仅在模板与路径条件形成唯一一对一结构关系时声明天数区间绑定。
// 这避免按“请假天数”等名称猜测字段，模板存在多个数值或日期区间时宁可不自动绑定。
func buildPathDateRangeBindings(tree *target.FlowNodeTemplate, choices []model.ExecutionPathChoice, template map[string]any) []formdata.DateRangeBinding {
	selected := make(map[string]string, len(choices))
	for _, choice := range choices {
		selected[choice.RouteNodeID] = choice.BranchID
	}
	conditionFields := make(map[string]bool)
	visitTargetTree(tree, map[string]bool{}, func(node *target.FlowNodeTemplate) {
		for _, branch := range node.ConditionNodes {
			if selected[node.ID] != branch.ID {
				continue
			}
			for _, condition := range branch.Conditions {
				field := normalizeFormFieldPath(condition.FieldA)
				if field != "" && strings.TrimSpace(condition.FieldB) == "" && isNumericConditionJudge(condition.Judge) {
					conditionFields[field] = true
				}
			}
		}
	})
	fields, _ := formdata.ParseTemplate(template)
	numbers, ranges := make([]string, 0), make([]string, 0)
	for _, field := range fields {
		path := normalizeFormFieldPath(field.Path)
		if field.Type == "number" && conditionFields[path] {
			numbers = append(numbers, path)
		}
		if field.Type == "date" && field.Mode == "daterange" {
			ranges = append(ranges, path)
		}
	}
	if len(numbers) != 1 || len(ranges) != 1 {
		return nil
	}
	return []formdata.DateRangeBinding{{DurationField: numbers[0], RangeField: ranges[0]}}
}

// isNumericConditionJudge 判断目标比较码是否可以稳定投影为天数约束。
func isNumericConditionJudge(judge string) bool {
	switch normalizeConditionJudge(judge) {
	case "gt", "gte", "lt", "lte", "eq":
		return true
	default:
		return false
	}
}

// validateTargetPathSelection 按目标平台有序策略和完整条件表达式复验 values 的实际选支。
func validateTargetPathSelection(tree *target.FlowNodeTemplate, choices []model.ExecutionPathChoice, values map[string]any) []string {
	expected := make(map[string]string, len(choices))
	for _, choice := range choices {
		expected[choice.RouteNodeID] = choice.BranchID
	}
	actual, reasons := resolveTargetConditionBranches(tree, values, choices)
	for routeNodeID, branchID := range expected {
		if actualBranchID, exists := actual[routeNodeID]; exists && actualBranchID != branchID {
			reasons = append(reasons, "表单数据实际会命中其他排序更靠前的条件分支")
		}
	}
	return uniquePublicStrings(reasons)
}

// resolveTargetConditionBranches 复刻目标条件节点的有序策略选择，返回每个条件节点真正会进入的分支。
func resolveTargetConditionBranches(tree *target.FlowNodeTemplate, values map[string]any, choices []model.ExecutionPathChoice) (map[string]string, []string) {
	actual := make(map[string]string)
	reasons := make([]string, 0)
	selected := make(map[string]string, len(choices))
	for _, choice := range choices {
		selected[choice.RouteNodeID] = choice.BranchID
	}
	var visit func(*target.FlowNodeTemplate)
	visit = func(node *target.FlowNodeTemplate) {
		if node == nil {
			return
		}
		branches := orderedTargetBranches(node.ConditionNodes)
		if len(branches) == 0 {
			visit(node.Child)
			return
		}
		var selectedChild *target.FlowNodeTemplate
		for index, branch := range branches {
			// 目标源码在前序策略均未成立时仍会选择最后一项；这里只复现运行结果，绝不据此生成或改写字段。
			if index == len(branches)-1 {
				actual[node.ID] = branch.ID
				selectedChild = branch.Child
				break
			}
			matched, evaluable := targetBranchMatches(values, branch.Conditions)
			if !evaluable {
				reasons = append(reasons, "当前表单值无法完整计算条件分支")
				break
			}
			if matched {
				actual[node.ID] = branch.ID
				selectedChild = branch.Child
				break
			}
		}
		if selected[node.ID] == "" || actual[node.ID] == selected[node.ID] {
			visit(selectedChild)
		}
	}
	visit(tree)
	return actual, uniquePublicStrings(reasons)
}

// orderedTargetBranches 按目标策略 sort 保持稳定排序，数据库同序号仍保留读取顺序。
func orderedTargetBranches(branches []target.FlowBranchTemplate) []target.FlowBranchTemplate {
	ordered := append([]target.FlowBranchTemplate(nil), branches...)
	sort.SliceStable(ordered, func(left, right int) bool { return ordered[left].Sort < ordered[right].Sort })
	return ordered
}

// targetBranchMatches 复刻目标 isLastResult 的顺序 and/or 聚合，不把跨分支条件错误并成全局阈值。
func targetBranchMatches(values map[string]any, conditions []target.FlowCondition) (bool, bool) {
	if len(conditions) == 0 {
		return false, true
	}
	lastResult := false
	var previousResult *bool
	previousType := ""
	for _, condition := range conditions {
		current, evaluable := targetConditionMatches(values, condition)
		if !evaluable {
			return false, false
		}
		conditionType := strings.ToLower(strings.TrimSpace(condition.ConditionType))
		if conditionType == "" && previousType == "" {
			return current, true
		}
		switch previousType {
		case "and":
			if previousResult != nil && current && *previousResult && conditionType == "" {
				lastResult = true
			}
			merged := previousResult != nil && current && *previousResult
			previousResult = &merged
		case "or":
			if previousResult != nil && (current || *previousResult) && conditionType == "" {
				lastResult = true
			}
			merged := previousResult != nil && (current || *previousResult)
			previousResult = &merged
		default:
			previousResult = &current
		}
		previousType = conditionType
	}
	return lastResult, true
}

// targetConditionMatches 用目标条件的字段 A、固定值或字段 B 执行基础比较，未知比较码不伪造命中。
func targetConditionMatches(values map[string]any, condition target.FlowCondition) (bool, bool) {
	left, exists := pathFormValue(values, normalizeFormFieldPath(condition.FieldA))
	if !exists {
		return false, false
	}
	right := pathConditionValue(condition.ValueB)
	if strings.TrimSpace(condition.ValueB) == "" {
		var found bool
		right, found = pathFormValue(values, normalizeFormFieldPath(condition.FieldB))
		if !found {
			return false, false
		}
	}
	switch normalizeConditionJudge(condition.Judge) {
	case "eq":
		return targetValuesEqual(left, right), true
	case "neq":
		return !targetValuesEqual(left, right), true
	case "gt", "gte", "lt", "lte":
		leftNumber, leftOK := targetNumber(left)
		rightNumber, rightOK := targetNumber(right)
		if !leftOK || !rightOK {
			return false, false
		}
		switch normalizeConditionJudge(condition.Judge) {
		case "gt":
			return leftNumber > rightNumber, true
		case "gte":
			return leftNumber >= rightNumber, true
		case "lt":
			return leftNumber < rightNumber, true
		default:
			return leftNumber <= rightNumber, true
		}
	case "in":
		list, ok := right.([]any)
		if !ok {
			return targetValuesEqual(left, right), true
		}
		for _, value := range list {
			if targetValuesEqual(left, value) {
				return true, true
			}
		}
		return false, true
	case "contains":
		return strings.Contains(fmt.Sprint(left), fmt.Sprint(right)), true
	default:
		return false, false
	}
}

// pathFormValue 读取点分隔表单字段键，不跨数组或显示名称猜测字段。
func pathFormValue(values map[string]any, path string) (any, bool) {
	current := any(values)
	for _, part := range strings.Split(strings.TrimSpace(path), ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// targetValuesEqual 先以数字比较兼容 JSON 数字与条件字符串，再回退到目标展示值比较。
func targetValuesEqual(left, right any) bool {
	leftNumber, leftOK := targetNumber(left)
	rightNumber, rightOK := targetNumber(right)
	if leftOK && rightOK {
		return leftNumber == rightNumber
	}
	return fmt.Sprint(left) == fmt.Sprint(right)
}

// targetNumber 把目标运行时可能出现的 JSON 数值或数值文本转换为比较值。
func targetNumber(value any) (float64, bool) {
	result, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
	return result, err == nil
}

// pathConditionValue 把目标条件常量恢复为 JSON 标量或列表，无法解析时保留原字符串。
func pathConditionValue(raw string) any {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}
	var value any
	if json.Unmarshal([]byte(text), &value) == nil {
		return value
	}
	if strings.Contains(text, ",") {
		parts := strings.Split(text, ",")
		values := make([]any, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				values = append(values, trimmed)
			}
		}
		return values
	}
	return text
}

// normalizeConditionJudge 归一目标比较码为生成器支持的基本运算。
func normalizeConditionJudge(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "=", "eq", "equal", "equals":
		return "eq"
	case "!=", "neq", "not_equal":
		return "neq"
	case ">", "gt":
		return "gt"
	case ">=", "gte":
		return "gte"
	case "<", "lt":
		return "lt"
	case "<=", "lte":
		return "lte"
	case "in":
		return "in"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

// formdataIdentityContext 把目标目录身份节点转换为生成器输入，不暴露任何内部 ID 到公开响应。
func formdataIdentityContext(identity target.FormIdentityContext) formdata.IdentityContext {
	return formdata.IdentityContext{
		Company: formdata.IdentityNode{
			ID: identity.Company.ID, Name: identity.Company.Name, Type: identity.Company.Type,
			ParentID: identity.Company.ParentID, CompanyID: identity.Company.CompanyID,
		},
		Department: formdata.IdentityNode{
			ID: identity.Department.ID, Name: identity.Department.Name, Type: identity.Department.Type,
			ParentID: identity.Department.ParentID, CompanyID: identity.Department.CompanyID,
		},
		User: formdata.IdentityNode{
			ID: identity.User.ID, Name: identity.User.Name, Type: identity.User.Type,
			ParentID: identity.User.ParentID, CompanyID: identity.User.CompanyID,
		},
	}
}

// buildPathConditionHints 只投影当前路径实际选择的条件节点和分支；无法复验实际值时仍保护精确映射字段。
func buildPathConditionHints(tree *target.FlowNodeTemplate, choices []model.ExecutionPathChoice, template map[string]any, permissions []model.PathFormPermission, values map[string]any) []model.PathFormConditionHint {
	fields, _ := formdata.ParseTemplate(template)
	labelByModel := make(map[string]string, len(fields))
	for _, field := range fields {
		if normalized := normalizeFormFieldPath(field.Path); normalized != "" && strings.TrimSpace(field.Name) != "" {
			labelByModel[normalized] = field.Name
		}
	}
	_ = permissions // 字段权限由 iframe 同时应用；提示的命中事实不能因原字段已只读而丢失。
	selected := make(map[string]string, len(choices))
	for _, choice := range choices {
		selected[choice.RouteNodeID] = choice.BranchID
	}
	hints := make([]model.PathFormConditionHint, 0)
	active, _ := resolveTargetConditionBranches(tree, values, choices)
	visitSelectedPathConditionNodes(tree, selected, func(node *target.FlowNodeTemplate) {
		branchID := selected[node.ID]
		if branchID == "" {
			return
		}
		branch := selectedTargetConditionBranch(node.ConditionNodes, branchID)
		if branch == nil {
			return
		}
		branchName := strings.TrimSpace(branch.Name)
		if branchName == "" {
			branchName = "已选条件分支"
		}
		actualBranch, activeKnown := active[node.ID]
		isActive := activeKnown && actualBranch == branch.ID
		// 当前路径已选分支是生成和保存必须满足的约束。运行时值不完整时不能撤销已建立的精确字段保护。
		protected := isActive || !activeKnown
		if len(branch.Conditions) == 0 {
			hints = append(hints, model.PathFormConditionHint{
				Key: conditionHintKey(node.ID, branch.ID, 0), NodeName: node.Name, BranchName: branchName,
				Text:   fmt.Sprintf("节点「%s」的当前路径选择「%s」没有显式条件", node.Name, branchName),
				Active: isActive, ActiveKnown: activeKnown, Mapped: false,
			})
			return
		}
		for index, condition := range branch.Conditions {
			leftField := normalizeFormFieldPath(condition.FieldA)
			rightField := normalizeFormFieldPath(condition.FieldB)
			fields := uniquePublicStrings([]string{leftField, rightField})
			mapped := len(fields) > 0
			for _, field := range fields {
				_, exists := labelByModel[field]
				if !exists {
					mapped = false
					continue
				}
			}
			judge, ok := pathConditionJudgeText(condition.Judge)
			if !ok {
				judge = "使用未识别的比较方式"
			}
			leftText := "未识别字段"
			if leftField != "" {
				if label, exists := labelByModel[leftField]; exists {
					leftText = label
				}
			}
			rightText := pathConditionDisplayText(pathConditionValue(condition.ValueB))
			if rightField != "" {
				if label, exists := labelByModel[rightField]; exists {
					rightText = label
				} else {
					rightText = "未识别字段"
				}
			}
			text := fmt.Sprintf("节点「%s」· 当前路径选择「%s」：%s %s %s", node.Name, branchName, leftText, judge, rightText)
			if !mapped {
				text += "（条件字段无法精确映射，保持可编辑）"
			}
			field := ""
			if len(fields) > 0 {
				field = fields[0]
			}
			hints = append(hints, model.PathFormConditionHint{
				Key: conditionHintKey(node.ID, branch.ID, index), NodeName: node.Name, BranchName: branchName,
				Field: field, Fields: fields, Text: text, Protected: protected && mapped,
				Active: isActive, ActiveKnown: activeKnown, Mapped: mapped,
			})
		}
	})
	return hints
}

// selectedTargetConditionBranch 按保存的真实分支标识定位当前条件节点的已选分支。
func selectedTargetConditionBranch(branches []target.FlowBranchTemplate, branchID string) *target.FlowBranchTemplate {
	for index := range branches {
		if strings.TrimSpace(branches[index].ID) == strings.TrimSpace(branchID) {
			return &branches[index]
		}
	}
	return nil
}

// conditionHintKey 为当前节点、分支和条件序号生成不暴露目标 ID 的稳定公开键。
func conditionHintKey(nodeID, branchID string, index int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("path-condition-hint:%s:%s:%d", nodeID, branchID, index)))
	return hex.EncodeToString(sum[:16])
}

// visitSelectedPathConditionNodes 只沿已保存路径的分支子树投影条件，避免把其他路径节点的提示带进当前表单。
func visitSelectedPathConditionNodes(tree *target.FlowNodeTemplate, selected map[string]string, call func(*target.FlowNodeTemplate)) {
	var visit func(*target.FlowNodeTemplate)
	visit = func(node *target.FlowNodeTemplate) {
		if node == nil {
			return
		}
		if len(node.ConditionNodes) > 0 {
			call(node)
			branchID := selected[node.ID]
			for _, branch := range node.ConditionNodes {
				if branch.ID == branchID {
					visit(branch.Child)
					return
				}
			}
			return
		}
		if len(node.ParallelNodes) > 0 {
			call(node)
			for _, branch := range node.ParallelNodes {
				if branch.ID == selected[node.ID] {
					visit(branch.Child)
					return
				}
			}
			return
		}
		visit(node.Child)
	}
	visit(tree)
}

// buildPathFormFieldRules 合并同一字段的命中条件，供 iframe 在真实组件创建前设置 disabled。
func buildPathFormFieldRules(hints []model.PathFormConditionHint) []model.PathFormFieldRule {
	byField := make(map[string]*model.PathFormFieldRule)
	for _, hint := range hints {
		if !hint.Mapped || !hint.Protected {
			continue
		}
		fields := hint.Fields
		if len(fields) == 0 && strings.TrimSpace(hint.Field) != "" {
			fields = []string{hint.Field}
		}
		for _, field := range fields {
			if strings.TrimSpace(field) == "" {
				continue
			}
			rule := byField[field]
			if rule == nil {
				rule = &model.PathFormFieldRule{Field: field, ConditionHints: []string{}}
				byField[field] = rule
			}
			rule.Disabled = true
			rule.ConditionHints = append(rule.ConditionHints, hint.Text)
		}
	}
	result := make([]model.PathFormFieldRule, 0, len(byField))
	for _, rule := range byField {
		rule.ConditionHints = uniquePublicStrings(rule.ConditionHints)
		result = append(result, *rule)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Field < result[right].Field })
	return result
}

// normalizeFormFieldPath 只做目标已定义的嵌套分隔符归一，不按字段名称或相似度猜测映射。
func normalizeFormFieldPath(value string) string {
	return strings.TrimSpace(strings.ReplaceAll(value, "_$$_", "."))
}

// pathConditionJudgeText 把比较码翻译为气泡可读中文；未知码返回 false。
func pathConditionJudgeText(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "eq":
		return "等于", true
	case "neq":
		return "不等于", true
	case "gt":
		return "大于", true
	case "gte":
		return "大于等于", true
	case "lt":
		return "小于", true
	case "lte":
		return "小于等于", true
	case "in":
		return "属于", true
	case "contains":
		return "包含", true
	default:
		return "", false
	}
}

// pathConditionDisplayText 把条件常量收敛为气泡可读文本，数组与对象只保留简短摘要。
func pathConditionDisplayText(value any) string {
	switch typed := value.(type) {
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, fmt.Sprint(item))
		}
		return strings.Join(parts, "、")
	case map[string]any:
		if text, ok := typed["name"].(string); ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
		return "目标配置值"
	default:
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" {
			return "（未填）"
		}
		return text
	}
}

// visitTargetTree 遍历一次真实流程树，重复或循环节点只处理一次。
func visitTargetTree(node *target.FlowNodeTemplate, visited map[string]bool, call func(*target.FlowNodeTemplate)) {
	if node == nil || visited[node.ID] {
		return
	}
	visited[node.ID] = true
	call(node)
	for _, branch := range node.ConditionNodes {
		visitTargetTree(branch.Child, visited, call)
	}
	for _, branch := range node.ParallelNodes {
		visitTargetTree(branch.Child, visited, call)
	}
	visitTargetTree(node.Child, visited, call)
}

// pathConfigSaveResult 返回节点与表单分域修订号和整条路径权威状态。
func pathConfigSaveResult(path model.ExecutionPath, stored model.StoredPathConfig) model.PathConfigSaveResult {
	return model.PathConfigSaveResult{
		Path:     model.PathConfigPath{SequenceNo: path.SequenceNo, Name: path.Name},
		Revision: stored.Revision, NodeRevision: stored.NodeRevision, FormRevision: stored.FormRevision, Status: stored.Status,
	}
}

// initialStoredFormStatus 为首次节点保存建立不冒充完成的表单状态。
func initialStoredFormStatus(snapshot target.PathConfigurationSnapshot) string {
	if len(snapshot.Forms) == 0 {
		return "valid"
	}
	return "empty"
}

// affectedFromStrings 把内部原因收敛为不含目标标识的公开受影响项。
func affectedFromStrings(kind string, reasons []string) []model.PathConfigAffectedItem {
	result := make([]model.PathConfigAffectedItem, 0, len(reasons))
	for _, reason := range uniquePublicStrings(reasons) {
		result = append(result, model.PathConfigAffectedItem{Kind: kind, Name: "表单数据", Reason: reason})
	}
	return result
}

// uniquePublicStrings 去重公开摘要并保持首次出现顺序。
func uniquePublicStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

// appendUnique 在节点确认列表中保持稳定顺序且不重复。
func appendUnique(values []string, value string) []string {
	for _, current := range values {
		if current == value {
			return values
		}
	}
	return append(values, value)
}

// cloneFormValues 通过 JSON 深复制完整 values，避免服务层修改仓储或目标快照。
func cloneFormValues(values map[string]any) map[string]any {
	if values == nil {
		return map[string]any{}
	}
	data, _ := json.Marshal(values)
	result := make(map[string]any)
	_ = json.Unmarshal(data, &result)
	return result
}
