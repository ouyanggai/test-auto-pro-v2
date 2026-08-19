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

const currentPathConfigVersion = 4

const currentPathFormConfigVersion = 2

const pathConfigActionCyclesStorageKey = "f008:action-cycles"

type pathFormRuntimeSessionReader interface {
	FormRuntimeSession(context.Context, string) (target.FormRuntimeSession, error)
}

type pathFormSampleReader interface {
	RecentFormSamples(context.Context, string, string, int) ([]map[string]any, error)
}

type pathFormIdentityReader interface {
	FormIdentityContext(context.Context, string) (target.FormIdentityContext, error)
}

type pathFormConditionProjection struct {
	Bindings       []model.PathFormConditionBinding
	Reviews        []string
	FieldRules     []model.PathFormFieldRule
	Constraints    []formdata.Constraint
	ProtectedPaths map[string]bool
}

type pathFormConditionFieldRef struct {
	Field formdata.Field
	Mode  string
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
		DepartmentID: active.DepartmentID, DepartmentName: active.DepartmentName,
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
		if recent, sampleErr := reader.RecentFormSamples(ctx, plan.Account, snapshot.FlowCode, 5); sampleErr == nil {
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
	conditions := buildPathFormConditionProjection(snapshot.Tree, path.Choices, template, base)
	generated := formdata.Generate(formdata.GenerateInput{
		Template: template, Base: base, Samples: samples, Seed: seed, Initiator: initiator,
		Constraints: conditions.Constraints, ManualOverridePaths: manualPaths, ProtectedPaths: conditions.ProtectedPaths,
		DateRangeBindings: dateRangeBindings,
		EditablePaths:     editableFormPathsFromPermissions(permissions),
		Identity:          identity,
	})
	generated.Unsupported = append(generated.Unsupported, unsupported...)
	solved := solveTargetPathValues(snapshot.Tree, path.Choices, template, generated.Values, seed)
	generated.Values = solved.values
	formdata.SynchronizeDateRangeBindings(generated.Values, dateRangeBindings, manualPaths)
	verificationReasons := validateTargetPathSelection(snapshot.Tree, path.Choices, generated.Values)
	dateReasons := formdata.ValidateDateRangeBindings(generated.Values, dateRangeBindings)
	matched := solved.matched && len(verificationReasons) == 0 && len(dateReasons) == 0
	issues := append(make([]model.PathFormGenerationIssue, 0, len(solved.issues)), solved.issues...)
	for _, review := range conditions.Reviews {
		issues = appendPathSolveIssue(issues, "当前路径条件", review, true)
	}
	for _, item := range uniquePublicStrings(generated.Unsupported) {
		issues = appendPathSolveIssue(issues, "表单字段", item, true)
	}
	for _, reason := range append(verificationReasons, dateReasons...) {
		issues = appendPathSolveIssue(issues, "当前路径条件", reason, true)
	}
	conditions = buildPathFormConditionProjection(snapshot.Tree, path.Choices, template, generated.Values)
	for _, review := range conditions.Reviews {
		issues = appendPathSolveIssue(issues, "当前路径条件", review, true)
	}
	if nextGroup && reflect.DeepEqual(base, generated.Values) {
		// 没有第二组候选属于可预期求解结果，返回 2xx 让页面直接展示原因。
		issues = appendPathSolveIssue(issues, "表单数据", "没有可切换的下一组有效候选，当前数据已保持不变", false)
	}
	generationState := "complete"
	if !matched || len(issues) > 0 {
		generationState = "partial"
		if len(generated.Values) == 0 {
			generationState = "blocked"
		}
	}
	verificationReason := solved.reason
	if len(verificationReasons) > 0 {
		verificationReason = verificationReasons[0]
	} else if len(dateReasons) > 0 {
		verificationReason = dateReasons[0]
	} else if matched {
		verificationReason = "生成数据已命中当前完整路径"
	}
	return model.PathFormGenerateResult{
		Revision: stored.FormRevision, Status: "draft", Values: generated.Values, Seed: seed,
		GeneratedFieldPaths: generated.GeneratedFieldPaths, ManualOverridePaths: generated.ManualOverridePaths,
		SampleSummary: model.PathFormSampleSummary{Saved: found && len(stored.FormValues) > 0, Defaults: generated.Defaults, Recent: generated.Recent, Fallback: generated.Fallback, Identity: generated.Identity},
		AutoFilled:    len(generated.GeneratedFieldPaths), ManualPending: generated.Pending,
		Unsupported:       uniquePublicStrings(generated.Unsupported),
		ConditionBindings: conditions.Bindings, ConditionReviews: conditions.Reviews, FieldRules: conditions.FieldRules,
		GenerationState: generationState, Issues: issues,
		RouteVerification: model.PathFormRouteVerification{Matched: matched, Reason: verificationReason},
	}, nil
}

// SaveNode 只合并当前节点的人员、动作和循环；循环与重复次数总在写入前按最新路径复验。
func (s *PathConfigService) SaveNode(ctx context.Context, planID, pathID uint64, nodeKey, idempotencyKey string, input model.PathNodeSaveInput) (model.PathConfigSaveResult, error) {
	idempotencyKey, nodeKey = strings.TrimSpace(idempotencyKey), strings.TrimSpace(nodeKey)
	if nodeKey == "" || !validUUID(idempotencyKey) {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalidArgument, Message: "节点或保存标识不正确"}
	}
	path, err := s.ownedPath(ctx, planID, pathID)
	if err != nil {
		return model.PathConfigSaveResult{}, err
	}
	if err = s.validateConfigMutablePlan(ctx, planID); err != nil {
		return model.PathConfigSaveResult{}, err
	}
	if existing, hit, findErr := s.configRepository.FindByPathAndKey(ctx, pathID, idempotencyKey); findErr != nil {
		return model.PathConfigSaveResult{}, mapPathConfigRepositoryError(findErr)
	} else if hit {
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
	_, validation, err := s.configAnalyzer.Analyze(owned.graph, snapshot.Tree, snapshot.FormFields, path, owned.pathAnalysis, snapshot.InstanceValues, map[string]map[string]string{}, stored.ActionValues, found)
	if err != nil {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前节点无法投影，请重新读取"}
	}
	nodeTarget, exists := validation.NodeTokens[nodeKey]
	if !exists {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalidArgument, Message: "当前节点没有可保存的动作配置"}
	}
	nodeValues, err := validatePathConfigNodeSubmission(nodeTarget, input)
	if err != nil {
		return model.PathConfigSaveResult{}, err
	}
	// 开发阶段直接清除旧动作计划和旧人员数组，避免任何双读或混合结构继续参与判断。
	values := copyPathConfigActionValues(stored.ActionValues)
	for key := range values {
		if strings.HasPrefix(key, "action-plan:") || strings.HasPrefix(key, "person:") {
			delete(values, key)
		}
	}
	for key, value := range nodeValues {
		values[key] = value
	}
	candidate, _, projectionErr := s.configAnalyzer.Analyze(owned.graph, snapshot.Tree, snapshot.FormFields, path, owned.pathAnalysis, snapshot.InstanceValues, map[string]map[string]string{}, values, true)
	if projectionErr != nil {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前路径循环无法核对，请重新读取"}
	}
	cycleInputs := decodePathConfigActionCycleInputs(values)
	if input.ActionCycles != nil {
		cycleInputs = input.ActionCycles
	}
	cycles, cycleErr := validatePathConfigActionCycles(candidate, cycleInputs)
	if cycleErr != nil {
		return model.PathConfigSaveResult{}, cycleErr
	}
	encodedCycles, marshalErr := json.Marshal(cycles)
	if marshalErr != nil {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorStorage, Message: "循环配置暂时无法保存，请重试"}
	}
	if len(cycleInputs) == 0 {
		delete(values, pathConfigActionCyclesStorageKey)
	} else {
		values[pathConfigActionCyclesStorageKey] = string(encodedCycles)
	}
	if _, valid := analyzer.CountStoredPathConfigActionExecutions(values); !valid {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "整条路径的动作总数不能超过 100 个"}
	}
	stored.ActionValues, stored.ConfirmedNodeKeys, stored.PathID = values, appendUnique(stored.ConfirmedNodeKeys, nodeKey), pathID
	stored.Revision, stored.NodeRevision, stored.IdempotencyKey, stored.ConfigVersion = stored.Revision+1, stored.NodeRevision+1, idempotencyKey, currentPathConfigVersion
	stored.Status = s.deriveStoredStatus(ctx, planID, path, snapshot, stored)
	if !found {
		stored.FormStatus = initialStoredFormStatus(snapshot)
		stored.DataStatus = initialStoredDataStatus(snapshot)
	}
	saved, err := s.configRepository.Save(ctx, stored, stored.Revision-1, s.now().UTC())
	if err != nil {
		return model.PathConfigSaveResult{}, mapPathConfigRepositoryError(err)
	}
	return pathConfigSaveResult(path, saved), nil
}

// SaveSelection 保存本次测试纳入标记；它不要求节点已准备完成，也不调用目标平台。
func (s *PathConfigService) SaveSelection(ctx context.Context, planID, pathID uint64, idempotencyKey string, input model.PathConfigSelectionInput) (model.PathConfigSaveResult, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !validUUID(idempotencyKey) {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalidArgument, Message: "保存标识不正确，请重试"}
	}
	path, err := s.ownedPath(ctx, planID, pathID)
	if err != nil {
		return model.PathConfigSaveResult{}, err
	}
	if err = s.validateConfigMutablePlan(ctx, planID); err != nil {
		return model.PathConfigSaveResult{}, err
	}
	if existing, found, findErr := s.configRepository.FindByPathAndKey(ctx, pathID, idempotencyKey); findErr != nil {
		return model.PathConfigSaveResult{}, mapPathConfigRepositoryError(findErr)
	} else if found {
		return pathConfigSaveResult(path, existing), nil
	}
	stored, found, err := s.configRepository.FindByPath(ctx, pathID)
	if err != nil {
		return model.PathConfigSaveResult{}, mapPathConfigRepositoryError(err)
	}
	if found && input.Revision != stored.NodeRevision {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorRevisionConflict, Message: "配置已被其他操作更新，请刷新后重试"}
	}
	if stored.ActionValues == nil {
		stored.ActionValues = map[string]string{}
	}
	if input.Included {
		stored.ActionValues["f008:test-included"] = "true"
	} else {
		delete(stored.ActionValues, "f008:test-included")
	}
	stored.PathID, stored.Revision, stored.NodeRevision, stored.IdempotencyKey, stored.ConfigVersion = pathID, stored.Revision+1, stored.NodeRevision+1, idempotencyKey, currentPathConfigVersion
	if !found {
		stored.Status, stored.FormStatus, stored.DataStatus = "pending", "empty", "not_generated"
	}
	saved, err := s.configRepository.Save(ctx, stored, stored.Revision-1, s.now().UTC())
	if err != nil {
		return model.PathConfigSaveResult{}, mapPathConfigRepositoryError(err)
	}
	return pathConfigSaveResult(path, saved), nil
}

// projectPathConfigActionCycles 从 F-008 持久化输入与当前路径投影出公开摘要。
func projectPathConfigActionCycles(values map[string]string, configuration model.PathConfiguration) []model.PathConfigActionCycle {
	inputs := decodePathConfigActionCycleInputs(values)
	if inputs == nil {
		return []model.PathConfigActionCycle{}
	}
	cycles, err := derivePathConfigActionCycles(configuration, inputs)
	if err != nil {
		return []model.PathConfigActionCycle{}
	}
	return cycles
}

// validatePathConfigActionCycles 校验两种引擎真实回路及重复动作；静态循环不能包含暂存或加签。
func validatePathConfigActionCycles(configuration model.PathConfiguration, inputs []model.PathConfigActionCycleInput) ([]model.PathConfigActionCycleInput, *PathConfigError) {
	cycles, err := derivePathConfigActionCycles(configuration, inputs)
	if err != nil {
		return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "循环配置不合法", Affected: []model.PathConfigAffectedItem{{Kind: "cycle", Name: "循环配置", Reason: err.Error()}}}
	}
	plans := make(map[string][]model.PathConfigConfiguredAction)
	for _, node := range pathConfigCycleNodes(configuration) {
		plans[node.Key] = append([]model.PathConfigConfiguredAction(nil), node.ActionConfiguration.Actions...)
	}
	for _, cycle := range cycles {
		endActions := plans[cycleEndNodeKey(configuration, cycle)]
		if cycle.Type == "restart_from_initiator" && !pathConfigActionsContain(endActions, "reject_no_pass") {
			return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "重新发起循环缺少不同意动作", Affected: []model.PathConfigAffectedItem{{Kind: "cycle", Name: cycle.Label, Reason: "请先在循环最后一个节点配置不同意动作"}}}
		}
		if cycle.Type == "redo_previous_task" && !pathConfigActionsContain(endActions, "rollback_previous") {
			return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "上一节点返工缺少回退动作", Affected: []model.PathConfigAffectedItem{{Kind: "cycle", Name: cycle.Label, Reason: "请先在当前节点配置回退上一级动作"}}}
		}
	}
	for nodeKey, actions := range plans {
		if len(actions) == 0 {
			continue
		}
		contained := false
		for _, cycle := range cycles {
			for _, member := range cycle.Members {
				if member == pathConfigCycleNodeName(configuration, nodeKey) {
					contained = true
				}
			}
		}
		for _, action := range actions {
			if action.Count > 1 && !contained {
				return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "动作次数需要先建立真实循环", Affected: []model.PathConfigAffectedItem{{Kind: "action", Name: pathConfigCycleNodeName(configuration, nodeKey), Reason: "重复动作需要通过重新发起一整轮或上一节点返工再次到达该节点"}}}
			}
			if contained && (action.Kind == "draft_save" || action.Kind == "add_sign") {
				return nil, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "循环不能包含当前动作", Affected: []model.PathConfigAffectedItem{{Kind: "cycle", Name: pathConfigCycleNodeName(configuration, nodeKey), Reason: "暂存、加签不能加入静态循环"}}}
			}
		}
	}
	return inputs, nil
}

// decodePathConfigActionCycleInputs 只接受当前循环结构；无值与损坏值由调用方分别处理。
func decodePathConfigActionCycleInputs(values map[string]string) []model.PathConfigActionCycleInput {
	raw := strings.TrimSpace(values[pathConfigActionCyclesStorageKey])
	if raw == "" {
		return []model.PathConfigActionCycleInput{}
	}
	var inputs []model.PathConfigActionCycleInput
	if json.Unmarshal([]byte(raw), &inputs) != nil {
		return nil
	}
	return inputs
}

// copyPathConfigActionValues 复制当前路径的工具侧配置，候选校验失败时不会污染已保存记录。
func copyPathConfigActionValues(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

// cycleEndNodeKey 从服务端派生循环取得不透明终点键，避免用展示名称在重复节点中猜测。
func cycleEndNodeKey(configuration model.PathConfiguration, cycle model.PathConfigActionCycle) string {
	return cycle.EndNodeKey
}

// pathConfigActionsContain 判断节点动作草稿是否包含循环所需的状态迁移动作。
func pathConfigActionsContain(actions []model.PathConfigConfiguredAction, kind string) bool {
	for _, action := range actions {
		if action.Kind == kind {
			return true
		}
	}
	return false
}

// derivePathConfigActionCycles 仅按当前已保存路径的业务节点顺序派生循环成员，不接受浏览器提供成员列表。
func derivePathConfigActionCycles(configuration model.PathConfiguration, inputs []model.PathConfigActionCycleInput) ([]model.PathConfigActionCycle, error) {
	nodes := pathConfigCycleNodes(configuration)
	start := -1
	for index, node := range nodes {
		if node.Kind == "start" {
			start = index
			break
		}
	}
	if start < 0 {
		return nil, fmt.Errorf("当前路径缺少发起节点")
	}
	seen := map[string]bool{}
	result := make([]model.PathConfigActionCycle, 0, len(inputs))
	for index, input := range inputs {
		if input.Count != 1 {
			return nil, fmt.Errorf("循环固定执行一次")
		}
		key := strings.TrimSpace(input.Key)
		if key == "" {
			key = fmt.Sprintf("cycle-%d", index+1)
		}
		if seen[key] {
			return nil, fmt.Errorf("循环不能重复保存")
		}
		seen[key] = true
		end := -1
		for nodeIndex, node := range nodes {
			if node.Key == strings.TrimSpace(input.EndNodeKey) {
				end = nodeIndex
				break
			}
		}
		if end < 0 {
			return nil, fmt.Errorf("循环节点已不属于当前路径")
		}
		members := make([]string, 0)
		switch input.Type {
		case "restart_from_initiator":
			if end <= start || (nodes[end].Kind != "common" && nodes[end].Kind != "synergy") {
				return nil, fmt.Errorf("重新发起一整轮必须以路径上的审批或协同节点不同意结束")
			}
			for _, node := range nodes[start : end+1] {
				members = append(members, node.Name)
			}
			result = append(result, model.PathConfigActionCycle{Key: key, Type: input.Type, EndNodeKey: input.EndNodeKey, Label: "重新发起一整轮", Count: input.Count, Members: members, Summary: "终点不同意后回到发起人；重新提交时将重新解析条件、并行和人员"})
		case "redo_previous_task":
			if end < 2 || nodes[end-1].Kind == "start" {
				return nil, fmt.Errorf("上一节点返工不能回退到发起节点，请使用不同意后重新提交")
			}
			members = []string{nodes[end-1].Name, nodes[end].Name}
			result = append(result, model.PathConfigActionCycle{Key: key, Type: input.Type, EndNodeKey: input.EndNodeKey, Label: "上一节点返工", Count: input.Count, Members: members, Summary: "引擎只会回到真实上一个待办，不能指定其他目标"})
		default:
			return nil, fmt.Errorf("循环方式不属于当前引擎支持范围")
		}
	}
	return result, nil
}

// pathConfigCycleNodes 按路径投影顺序返回业务节点，路由和结束节点不参与循环。
func pathConfigCycleNodes(configuration model.PathConfiguration) []model.PathConfigNode {
	result := make([]model.PathConfigNode, 0)
	for _, group := range configuration.Groups {
		for _, node := range group.Nodes {
			if node.Kind == "start" || node.Kind == "common" || node.Kind == "synergy" {
				result = append(result, node)
			}
		}
	}
	return result
}

// pathConfigCycleNodeName 将不透明节点键转换为当前可读名称，错误信息不泄露内部标识。
func pathConfigCycleNodeName(configuration model.PathConfiguration, key string) string {
	for _, node := range pathConfigCycleNodes(configuration) {
		if node.Key == key {
			return node.Name
		}
	}
	return "当前节点"
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
	conditions := buildPathFormConditionProjection(snapshot.Tree, path.Choices, template, input.Values)
	if len(conditions.Reviews) > 0 {
		return model.PathConfigSaveResult{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前路径条件需要人工核对", Affected: affectedFromStrings("form", conditions.Reviews)}
	}
	if reasons := formdata.ValidateEditable(template, input.Values, conditions.Constraints, editableFormPaths(snapshot.Tree, formPermissionNodeIDs(plan.FlowSource, snapshot, owned.pathAnalysis.ReachableNodeIDs))); len(reasons) > 0 {
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
	stored.DataStatus = "confirmed"
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
		ConditionBindings: []model.PathFormConditionBinding{}, ConditionReviews: []string{},
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
	if len(snapshot.Forms) == 0 {
		form.Status, form.StatusName, form.Validated = "valid", "已配置", true
		return form
	}
	// 条件投影是提示、字段锁定、生成和保存复验的共同来源，任何无法精确对应的条件都不能放行保存。
	conditions := buildPathFormConditionProjection(snapshot.Tree, choices, template, form.Values)
	form.ConditionBindings = conditions.Bindings
	form.ConditionReviews = conditions.Reviews
	form.FieldRules = conditions.FieldRules
	if len(conditions.Reviews) > 0 {
		form.Status, form.StatusName = "affected", "部分配置"
		form.Affected = affectedFromStrings("form", conditions.Reviews)
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
			for _, action := range node.ActionConfiguration.Catalog {
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

// derivePathConfigurationStatus 只按节点人员和动作配置派生完成度，表单数据状态独立维护。
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
	return derivePathConfigurationStatus(configuration)
}

// runtimeTemplate 合并同一流程关联的全部 FormMaking 模板，重复字段模型和坏模板进入人工核对而不静默丢表单。
func runtimeTemplate(forms []target.FormRuntimeTemplate) (map[string]any, []string) {
	if len(forms) == 0 {
		return map[string]any{"list": []any{}, "config": map[string]any{}}, nil
	}
	unsupported := make([]string, 0)
	template := map[string]any{"list": []any{}, "config": map[string]any{}}
	seenModels := map[string]bool{}
	for index, form := range forms {
		fragment := make(map[string]any)
		if err := json.Unmarshal([]byte(form.TemplateData), &fragment); err != nil {
			unsupported = append(unsupported, fmt.Sprintf("第 %d 个 FormMaking 模板无法解析", index+1))
			continue
		}
		list, ok := fragment["list"].([]any)
		if !ok {
			unsupported = append(unsupported, fmt.Sprintf("第 %d 个 FormMaking 模板缺少字段列表", index+1))
			continue
		}
		template["list"] = append(template["list"].([]any), list...)
		if index == 0 {
			if config, configOK := fragment["config"].(map[string]any); configOK {
				template["config"] = config
			}
		}
		fields, _ := formdata.ParseTemplate(fragment)
		for _, field := range fields {
			if field.Path == "" || strings.Contains(field.Path, "[]") {
				continue
			}
			if seenModels[field.Path] {
				unsupported = append(unsupported, "多个表单包含重复字段模型「"+field.Path+"」，需要人工核对")
			}
			seenModels[field.Path] = true
		}
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

// buildPathDateRangeBindings 仅在模板与路径条件形成唯一一对一结构关系时声明天数区间绑定。
// 这避免按“请假天数”等名称猜测字段，模板存在多个数值或日期区间时宁可不自动绑定。
func buildPathDateRangeBindings(tree *target.FlowNodeTemplate, choices []model.ExecutionPathChoice, template map[string]any) []formdata.DateRangeBinding {
	selected := make(map[string]string, len(choices))
	for _, choice := range choices {
		selected[choice.RouteNodeID] = choice.BranchID
	}
	conditionFields := make(map[string]bool)
	visitSelectedPathConditionNodes(tree, selected, func(node *target.FlowNodeTemplate) {
		for _, branch := range orderedTargetBranches(node.ConditionNodes) {
			for _, condition := range branch.Conditions {
				field := normalizeFormFieldPath(condition.FieldA)
				if field != "" && strings.TrimSpace(condition.FieldB) == "" && isNumericConditionJudge(condition.Judge) {
					conditionFields[field] = true
				}
			}
			// 日期联动必须覆盖当前分支及所有更靠前分支，兜底路径才能构造避开前置条件的天数。
			if selected[node.ID] == branch.ID {
				break
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
	visited := make(map[string]bool)
	selected := make(map[string]string, len(choices))
	for _, choice := range choices {
		selected[choice.RouteNodeID] = choice.BranchID
	}
	var visit func(*target.FlowNodeTemplate)
	visit = func(node *target.FlowNodeTemplate) {
		if node == nil || visited[node.ID] {
			return
		}
		visited[node.ID] = true
		branches := orderedTargetBranches(node.ConditionNodes)
		if len(branches) == 0 {
			if len(node.ParallelNodes) > 0 {
				if strings.TrimSpace(node.Type) == "parallel" {
					// 并行路由的所有分支都属于同一执行路径，必须逐支复验嵌套条件。
					for index := range node.ParallelNodes {
						visit(node.ParallelNodes[index].Child)
					}
				} else {
					for index := range node.ParallelNodes {
						if node.ParallelNodes[index].ID == selected[node.ID] {
							visit(node.ParallelNodes[index].Child)
							break
						}
					}
				}
			}
			visit(node.Child)
			return
		}
		if isTargetManualBranchNode(node) {
			for _, branch := range branches {
				if branch.ID == selected[node.ID] {
					actual[node.ID] = branch.ID
					visit(branch.Child)
					visit(node.Child)
					return
				}
			}
			reasons = append(reasons, "当前路径缺少手动分支选择")
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
		visit(node.Child)
	}
	visit(tree)
	return actual, uniquePublicStrings(reasons)
}

// isTargetManualBranchNode 识别目标平台以 condition/custom_choose 表示的人工分支，人工选择不由表单值求解。
func isTargetManualBranchNode(node *target.FlowNodeTemplate) bool {
	return node != nil && strings.TrimSpace(node.Type) == "condition" && strings.TrimSpace(node.BranchExecuteType) == "custom_choose"
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
		leftNumber, leftOK := targetComparableNumber(left)
		rightNumber, rightOK := targetComparableNumber(right)
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
	for _, rawPart := range strings.Split(strings.TrimSpace(path), ".") {
		part := strings.TrimSuffix(rawPart, "[]")
		isCollection := strings.HasSuffix(rawPart, "[]")
		object, ok := current.(map[string]any)
		if !ok {
			if list, listOK := current.([]any); listOK && len(list) > 0 {
				current = list[0]
				object, ok = current.(map[string]any)
			}
		}
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
		if isCollection {
			list, listOK := current.([]any)
			if !listOK || len(list) == 0 {
				return nil, false
			}
			current = list[0]
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
	case "contains":
		return "contains"
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

// buildPathFormConditionProjection 生成条件提示、字段锁定、智能生成约束和保存复验共用的单一投影。
func buildPathFormConditionProjection(tree *target.FlowNodeTemplate, choices []model.ExecutionPathChoice, template map[string]any, values map[string]any) pathFormConditionProjection {
	fields, _ := formdata.ParseTemplate(template)
	selected := make(map[string]string, len(choices))
	for _, choice := range choices {
		selected[choice.RouteNodeID] = choice.BranchID
	}
	active, _ := resolveTargetConditionBranches(tree, values, choices)
	projection := pathFormConditionProjection{Bindings: []model.PathFormConditionBinding{}, Reviews: []string{}, FieldRules: []model.PathFormFieldRule{}, Constraints: []formdata.Constraint{}, ProtectedPaths: map[string]bool{}}
	ruleHints := make(map[string][]string)
	reviewSet := map[string]bool{}
	visitSelectedPathConditionNodes(tree, selected, func(node *target.FlowNodeTemplate) {
		branchID := selected[node.ID]
		branch := selectedTargetConditionBranch(node.ConditionNodes, branchID)
		if branch == nil {
			branch = selectedTargetConditionBranch(node.ParallelNodes, branchID)
		}
		if branch == nil {
			return
		}
		branchName := strings.TrimSpace(branch.Name)
		if branchName == "" {
			branchName = "当前路径分支"
		}
		actualBranch, activeKnown := active[node.ID]
		verified := activeKnown && actualBranch == branch.ID
		if len(branch.Conditions) == 0 {
			projection.Bindings = append(projection.Bindings, model.PathFormConditionBinding{
				Key: conditionHintKey(node.ID, branch.ID, 0), NodeName: node.Name, BranchName: branchName,
				Expression: "当前路径在此处不依赖表单条件；由流程的人工/兜底规则决定", Selected: true, Verified: verified,
			})
			return
		}
		orGroup := 0
		for index, condition := range branch.Conditions {
			left, leftOK := resolvePathFormConditionField(condition.FieldA, fields)
			right, rightOK := resolvePathFormConditionField(condition.FieldB, fields)
			judge, judgeOK := pathConditionJudgeText(condition.Judge)
			if !judgeOK {
				judge = "未知比较方式"
			}
			leftText := conditionFieldText(condition.FieldA, left, leftOK)
			rightText := pathConditionDisplayText(pathConditionValue(condition.ValueB))
			if strings.TrimSpace(condition.FieldB) != "" {
				rightText = conditionFieldText(condition.FieldB, right, rightOK)
			}
			expression := fmt.Sprintf("%s %s %s", leftText, judge, rightText)
			refs := make([]pathFormConditionFieldRef, 0, 2)
			if leftOK {
				refs = append(refs, left)
			}
			if rightOK && strings.TrimSpace(condition.FieldB) != "" {
				refs = append(refs, right)
			}
			needsReview := !leftOK || !judgeOK || (strings.TrimSpace(condition.FieldB) != "" && !rightOK)
			constraint := formdata.Constraint{}
			if !needsReview {
				orGroup = conditionORGroup(branch.Conditions, index, orGroup)
				constraint, needsReview = buildPathFormConstraint(condition, left, right, rightOK, orGroup)
			}
			locked := !needsReview && len(refs) > 0
			fieldLabels := make([]string, 0, len(refs))
			for _, ref := range refs {
				fieldLabels = append(fieldLabels, ref.Field.Name)
			}
			binding := model.PathFormConditionBinding{
				Key: conditionHintKey(node.ID, branch.ID, index), NodeName: node.Name, BranchName: branchName,
				Expression: expression, Fields: uniquePublicStrings(fieldLabels), Selected: true, Locked: locked, NeedsReview: needsReview, Verified: verified,
			}
			projection.Bindings = append(projection.Bindings, binding)
			if needsReview {
				reason := fmt.Sprintf("节点「%s」的当前路径分支条件需要人工核对", node.Name)
				if !reviewSet[reason] {
					projection.Reviews = append(projection.Reviews, reason)
					reviewSet[reason] = true
				}
				continue
			}
			for _, ref := range refs {
				projection.ProtectedPaths[ref.Field.Path] = true
				projection.ProtectedPaths[ref.Field.Path+"__virtualName"] = true
				ruleHints[ref.Field.Path] = append(ruleHints[ref.Field.Path], expression)
			}
			if constraint.Field != "" {
				projection.Constraints = append(projection.Constraints, constraint)
				// 虚拟条件键同样是路径事实，不能让浏览器把它当成普通人工覆盖字段。
				projection.ProtectedPaths[constraint.Field] = true
			}
		}
		branches := node.ConditionNodes
		if len(branches) == 0 {
			branches = node.ParallelNodes
		}
		for _, referenceBranch := range branches {
			if referenceBranch.ID == branch.ID {
				continue
			}
			projection.Bindings = append(projection.Bindings, buildPathConditionReferenceBindings(node, referenceBranch, fields)...)
		}
	})
	for field, hints := range ruleHints {
		projection.FieldRules = append(projection.FieldRules, model.PathFormFieldRule{Field: field, Disabled: true, ConditionKeys: uniquePublicStrings(hints)})
	}
	sort.Slice(projection.FieldRules, func(left, right int) bool {
		return projection.FieldRules[left].Field < projection.FieldRules[right].Field
	})
	return normalizePathFormConditionProjection(projection)
}

// normalizePathFormConditionProjection 将公开条件 DTO 的切片统一为空数组，禁止 JSON null 穿透到前端渲染层。
func normalizePathFormConditionProjection(projection pathFormConditionProjection) pathFormConditionProjection {
	if projection.Bindings == nil {
		projection.Bindings = []model.PathFormConditionBinding{}
	}
	if projection.Reviews == nil {
		projection.Reviews = []string{}
	}
	if projection.FieldRules == nil {
		projection.FieldRules = []model.PathFormFieldRule{}
	}
	for index := range projection.Bindings {
		if projection.Bindings[index].Fields == nil {
			projection.Bindings[index].Fields = []string{}
		}
	}
	for index := range projection.FieldRules {
		if projection.FieldRules[index].ConditionKeys == nil {
			projection.FieldRules[index].ConditionKeys = []string{}
		}
	}
	return projection
}

// buildPathConditionReferenceBindings 生成未选分支的只读对照项，不参与当前路径的锁定、生成或保存约束。
func buildPathConditionReferenceBindings(node *target.FlowNodeTemplate, branch target.FlowBranchTemplate, fields []formdata.Field) []model.PathFormConditionBinding {
	branchName := strings.TrimSpace(branch.Name)
	if branchName == "" {
		branchName = "其他分支"
	}
	if len(branch.Conditions) == 0 {
		return []model.PathFormConditionBinding{{
			Key: conditionHintKey(node.ID, branch.ID, 0), NodeName: node.Name, BranchName: branchName,
			Expression: "其他条件均不满足时进入此分支", Fields: []string{}, Selected: false, Verified: false,
		}}
	}
	bindings := make([]model.PathFormConditionBinding, 0, len(branch.Conditions))
	for index, condition := range branch.Conditions {
		left, leftOK := resolvePathFormConditionField(condition.FieldA, fields)
		right, rightOK := resolvePathFormConditionField(condition.FieldB, fields)
		judge, judgeOK := pathConditionJudgeText(condition.Judge)
		if !judgeOK {
			judge = "未知比较方式"
		}
		leftText := conditionFieldText(condition.FieldA, left, leftOK)
		rightText := pathConditionDisplayText(pathConditionValue(condition.ValueB))
		if strings.TrimSpace(condition.FieldB) != "" {
			rightText = conditionFieldText(condition.FieldB, right, rightOK)
		}
		fieldsForBinding := make([]string, 0, 2)
		if leftOK {
			fieldsForBinding = append(fieldsForBinding, left.Field.Name)
		}
		if rightOK && strings.TrimSpace(condition.FieldB) != "" {
			fieldsForBinding = append(fieldsForBinding, right.Field.Name)
		}
		bindings = append(bindings, model.PathFormConditionBinding{
			Key: conditionHintKey(node.ID, branch.ID, index), NodeName: node.Name, BranchName: branchName,
			Expression: fmt.Sprintf("%s %s %s", leftText, judge, rightText), Fields: uniquePublicStrings(fieldsForBinding),
			Selected: false, Locked: false, NeedsReview: !leftOK || !judgeOK || (strings.TrimSpace(condition.FieldB) != "" && !rightOK), Verified: false,
		})
	}
	return bindings
}

// resolvePathFormConditionField 按目标已定义的模型键和虚拟键后缀精确定位 FormMaking 字段，不使用显示名称猜测。
func resolvePathFormConditionField(raw string, fields []formdata.Field) (pathFormConditionFieldRef, bool) {
	path := normalizeFormFieldPath(raw)
	for _, field := range fields {
		if path == field.Path {
			return pathFormConditionFieldRef{Field: field, Mode: "direct"}, true
		}
		if path == field.Path+"__virtualName" && (field.Type == "select" || field.Type == "radio" || field.Type == "cascader") {
			if field.OptionVirtualUsesValue {
				return pathFormConditionFieldRef{Field: field, Mode: "option-value"}, true
			}
			return pathFormConditionFieldRef{Field: field, Mode: "option-label"}, true
		}
		if path == field.Path+"__condition" && field.Type == "infoSelect" {
			return pathFormConditionFieldRef{Field: field, Mode: "info-condition"}, true
		}
	}
	return pathFormConditionFieldRef{}, false
}

// conditionFieldText 将精确匹配的模板字段转换为用户可读名称，未匹配字段只显示稳定中文提示。
func conditionFieldText(raw string, ref pathFormConditionFieldRef, ok bool) string {
	if !ok || strings.TrimSpace(raw) == "" {
		return "需要人工核对的字段"
	}
	if strings.TrimSpace(ref.Field.Name) != "" {
		return ref.Field.Name
	}
	return "表单字段"
}

// conditionORGroup 按目标条件顺序将相邻“或”条件归入同一生成约束组。
func conditionORGroup(conditions []target.FlowCondition, index, current int) int {
	previousOR := index > 0 && strings.EqualFold(strings.TrimSpace(conditions[index-1].ConditionType), "or")
	currentOR := strings.EqualFold(strings.TrimSpace(conditions[index].ConditionType), "or")
	if previousOR || currentOR {
		if !previousOR {
			current++
		}
		return current
	}
	return 0
}

// buildPathFormConstraint 把目标条件映射为生成器可执行约束，并拒绝无法安全反投影的字段语义。
func buildPathFormConstraint(condition target.FlowCondition, left, right pathFormConditionFieldRef, rightOK bool, group int) (formdata.Constraint, bool) {
	op := normalizeConditionJudge(condition.Judge)
	if left.Mode == "info-condition" {
		return formdata.Constraint{Field: normalizeFormFieldPath(condition.FieldA), FieldType: left.Field.Type, Op: op, Value: pathConditionValue(condition.ValueB), Group: group}, false
	}
	if strings.TrimSpace(condition.FieldB) != "" {
		if !rightOK || left.Mode != "direct" || right.Mode != "direct" {
			return formdata.Constraint{}, true
		}
		return formdata.Constraint{Field: left.Field.Path, FieldType: left.Field.Type, Op: op, ValueField: right.Field.Path, Group: group}, false
	}
	value := pathConditionValue(condition.ValueB)
	if left.Mode == "option-label" {
		mapped, ok := optionConditionValue(left.Field, value, op)
		if !ok {
			return formdata.Constraint{}, true
		}
		value = mapped
	}
	if left.Mode == "option-value" && !pathConditionValueExistsInOptions(left.Field, value, op) {
		return formdata.Constraint{}, true
	}
	if left.Field.Type == "cascader" && left.Mode == "direct" {
		var ok bool
		value, ok = cascaderConstraintValue(left.Field, value, op)
		if !ok {
			return formdata.Constraint{}, true
		}
	}
	return formdata.Constraint{Field: left.Field.Path, FieldType: left.Field.Type, Op: op, Value: value, Group: group}, false
}

// cascaderConstraintValue 把级联条件叶值映射为目标组件要求的完整根到叶路径。
func cascaderConstraintValue(field formdata.Field, value any, op string) (any, bool) {
	if len(field.OptionPaths) == 0 {
		return nil, false
	}
	if op == "in" {
		items, ok := value.([]any)
		if !ok {
			return nil, false
		}
		for _, item := range items {
			for _, path := range field.OptionPaths {
				if len(path) > 0 && targetValuesEqual(path[len(path)-1], item) {
					return path, true
				}
			}
		}
		return nil, false
	}
	if candidate, ok := value.([]any); ok {
		for _, path := range field.OptionPaths {
			if targetValuesEqual(path, candidate) {
				return path, true
			}
		}
		return nil, false
	}
	for _, path := range field.OptionPaths {
		if len(path) > 0 && targetValuesEqual(path[len(path)-1], value) {
			return path, true
		}
	}
	return nil, false
}

// pathConditionValueExistsInOptions 验证目标条件常量确实属于模板选项值，不按显示名猜测。
func pathConditionValueExistsInOptions(field formdata.Field, value any, op string) bool {
	values := []any{value}
	if op == "in" {
		if list, ok := value.([]any); ok {
			values = list
		}
	}
	for _, expected := range values {
		found := false
		for _, option := range field.Options {
			if targetValuesEqual(option, expected) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// optionConditionValue 将目标保存的选项显示值反向映射为 FormMaking 的模型值。
func optionConditionValue(field formdata.Field, value any, op string) (any, bool) {
	lookup := func(item any) (any, bool) {
		for _, option := range field.Options {
			if fmt.Sprint(item) == field.OptionNames[fmt.Sprint(option)] {
				// 保留模板定义的原始值类型；数值选项被转成字符串会导致真实表单校验失败。
				return option, true
			}
		}
		return nil, false
	}
	if op == "in" {
		items, ok := value.([]any)
		if !ok {
			return nil, false
		}
		mapped := make([]any, 0, len(items))
		for _, item := range items {
			converted, found := lookup(item)
			if !found {
				return nil, false
			}
			mapped = append(mapped, converted)
		}
		return mapped, true
	}
	return lookup(value)
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
	visited := make(map[string]bool)
	var visit func(*target.FlowNodeTemplate)
	visit = func(node *target.FlowNodeTemplate) {
		if node == nil || visited[node.ID] {
			return
		}
		visited[node.ID] = true
		if len(node.ConditionNodes) > 0 {
			call(node)
			branchID := selected[node.ID]
			for _, branch := range node.ConditionNodes {
				if branch.ID == branchID {
					visit(branch.Child)
					break
				}
			}
			visit(node.Child)
			return
		}
		if len(node.ParallelNodes) > 0 {
			if strings.TrimSpace(node.Type) == "parallel" {
				for _, branch := range node.ParallelNodes {
					visit(branch.Child)
				}
			} else {
				call(node)
				for _, branch := range node.ParallelNodes {
					if branch.ID == selected[node.ID] {
						visit(branch.Child)
						break
					}
				}
			}
			visit(node.Child)
			return
		}
		visit(node.Child)
	}
	visit(tree)
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

// initialStoredDataStatus 根据真实表单存在性建立独立数据准备初态。
func initialStoredDataStatus(snapshot target.PathConfigurationSnapshot) string {
	if len(snapshot.Forms) == 0 {
		return "not_required"
	}
	return "not_generated"
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
