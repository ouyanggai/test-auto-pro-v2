package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/formdata"
	"test-auto-pro-v2/internal/model"
)

const runInputSnapshotVersion = "p004-run-input/v1"

// PreflightRunInput 从当前目标和本地配置重建不可变输入，并纯编译目标请求预览。
func (s *PathConfigService) PreflightRunInput(ctx context.Context, planID, pathID uint64) (model.RunInputPreflightResult, error) {
	if planID == 0 || pathID == 0 {
		return model.RunInputPreflightResult{}, &PathConfigError{Kind: PathConfigErrorInvalidArgument, Message: "计划或路径 ID 不正确"}
	}
	path, err := s.ownedPath(ctx, planID, pathID)
	if err != nil {
		return model.RunInputPreflightResult{}, err
	}
	if err := s.validateConfigMutablePlan(ctx, planID); err != nil {
		return model.RunInputPreflightResult{}, err
	}
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return model.RunInputPreflightResult{}, err
	}
	stored, found, err := s.configRepository.FindByPath(ctx, pathID)
	if err != nil {
		return model.RunInputPreflightResult{}, mapPathConfigRepositoryError(err)
	}
	snapshot, err := s.readVerifiedSnapshot(ctx, planID)
	if err != nil {
		return model.RunInputPreflightResult{}, err
	}
	owned, err := s.analyzeOwnedPath(ctx, planID, snapshot, path)
	if err != nil {
		return model.RunInputPreflightResult{}, err
	}
	template, unsupported := runtimeTemplate(snapshot.Forms)
	if snapshot.RenderType == target.FormRenderTypeVueCustom {
		template = vueCustomTemplate(snapshot.VuePage)
	}
	issues := make([]model.RunInputPreflightIssue, 0)
	if !found {
		issues = appendRunInputIssue(issues, "configuration_missing", "path_configuration", "当前路径尚未保存完整配置", true, "")
		stored = model.StoredPathConfig{PathID: pathID, FormValues: map[string]any{}}
	}
	if len(unsupported) > 0 {
		issues = appendRunInputIssue(issues, "template_unsupported", "formmaking", strings.Join(unsupported, "；"), false, "")
	}
	if found && s.deriveStoredStatus(ctx, planID, path, snapshot, stored) != "configured" {
		issues = appendRunInputIssue(issues, "node_configuration_incomplete", "path_configuration", "路径节点人员或动作配置尚未完成", true, "")
	}
	form := projectPathForm(plan.FlowSource, snapshot, owned.pathAnalysis, path.Choices, stored, found)
	if form.Status != "valid" || !stored.FormValidated || stored.DataStatus != model.ExecutionPathDataConfirmed {
		message := "表单数据尚未确认或未通过当前规则复验"
		if len(form.Affected) > 0 {
			message = form.Affected[0].Reason
		}
		issues = appendRunInputIssue(issues, "form_configuration_invalid", "form_validation", message, true, "")
	}
	if snapshotRuleStatus(snapshot) != model.RuleReadinessReady {
		issues = appendRunInputIssue(issues, "rule_snapshot_not_ready", "rule_catalog", strings.Join(snapshotRuleIssues(snapshot), "；"), true, "")
	}
	issues = append(issues, s.refreshRunInputCandidateIssues(ctx, plan.Account, snapshot, template, stored.FormValues)...)

	identity, identityIssue := s.runInputIdentity(ctx, plan.Account)
	if identityIssue != nil {
		issues = append(issues, *identityIssue)
	}
	runSnapshot := buildRunInputSnapshot(plan, path, stored, snapshot, template, s.now().UTC())
	compiled := target.CompileFormSubmission(target.FormSubmissionCompileInput{
		FlowSource: plan.FlowSource, RenderType: snapshot.RenderType, Values: runSnapshot.FormValues,
		Template: template, Forms: snapshot.Forms, VuePage: snapshot.VuePage, Identity: identity,
	})
	for _, issue := range compiled.Issues {
		issues = appendRunInputIssue(issues, issue.Code, issue.Source, issue.Message, issue.CanRetry, issue.FieldPath)
	}
	status := model.RunInputPreflightReady
	if len(issues) > 0 || compiled.Status != target.FormSubmissionReady {
		status = model.RunInputPreflightBlocked
	}
	return model.RunInputPreflightResult{
		Status: status, Snapshot: runSnapshot,
		Target: model.TargetSubmissionPreview{
			Method: compiled.Method, Path: compiled.Path, PayloadKeys: append([]string(nil), compiled.PayloadKeys...),
			PayloadDigest: compiled.PayloadDigest, SuccessChecks: append([]string(nil), compiled.SuccessChecks...),
		},
		Issues: issues,
	}, nil
}

// buildRunInputSnapshot 深复制配置值和路径选择，并为形状与完整事实生成稳定摘要。
func buildRunInputSnapshot(plan model.Plan, path model.ExecutionPath, stored model.StoredPathConfig, snapshot target.PathConfigurationSnapshot, template map[string]any, capturedAt time.Time) model.RunInputSnapshot {
	values := cloneFormValues(stored.FormValues)
	choices := append([]model.ExecutionPathChoice(nil), path.Choices...)
	result := model.RunInputSnapshot{
		Version: runInputSnapshotVersion, PlanID: plan.ID, PathID: path.ID, SequenceNo: path.SequenceNo,
		AccountRef: plan.Account, FlowSource: plan.FlowSource, TargetObjectRef: plan.TargetObjectID,
		RenderType: string(snapshot.RenderType), TemplateRuleVersion: snapshot.RuleVersion,
		FormTemplateVersion: stored.FormTemplateVersion, ShapeDigest: runInputShapeDigest(snapshot.RenderType, template),
		ConfigVersion: stored.ConfigVersion, ConfigRevision: stored.Revision, NodeRevision: stored.NodeRevision, FormRevision: stored.FormRevision,
		PathChoices: choices, NodeFieldValues: cloneRunInputFieldValues(stored.FieldValues),
		ActionValues: copyPathConfigActionValues(stored.ActionValues), ConfirmedNodeKeys: append([]string(nil), stored.ConfirmedNodeKeys...),
		FormValues: values, CapturedAt: capturedAt,
	}
	result.SnapshotDigest = runInputSnapshotDigest(result)
	return result
}

// cloneRunInputFieldValues 深复制节点人员与字段配置，防止后续编辑改写已返回的运行输入。
func cloneRunInputFieldValues(values map[string]map[string]string) map[string]map[string]string {
	result := make(map[string]map[string]string, len(values))
	for nodeKey, fields := range values {
		clonedFields := make(map[string]string, len(fields))
		for fieldKey, value := range fields {
			clonedFields[fieldKey] = value
		}
		result[nodeKey] = clonedFields
	}
	return result
}

// runInputIdentity 读取当前账号短期会话中的非 SID 身份字段，编译结果不保留会话凭证。
func (s *PathConfigService) runInputIdentity(ctx context.Context, account string) (target.FormSubmissionIdentity, *model.RunInputPreflightIssue) {
	reader, ok := s.target.(pathFormRuntimeSessionReader)
	if !ok {
		issue := model.RunInputPreflightIssue{Code: "initiator_identity_unavailable", Source: "identity", Message: "当前账号身份读取能力不可用", CanRetry: true}
		return target.FormSubmissionIdentity{}, &issue
	}
	active, err := reader.FormRuntimeSession(ctx, account)
	if err != nil {
		issue := model.RunInputPreflightIssue{Code: "initiator_identity_unavailable", Source: "identity", Message: "当前账号身份读取失败", CanRetry: true}
		return target.FormSubmissionIdentity{}, &issue
	}
	return target.FormSubmissionIdentity{
		UserID: active.UserID, UserName: active.AccountName, CompanyID: active.CompanyID, CompanyName: active.CompanyName,
		DepartmentID: active.DepartmentID, DepartmentName: active.DepartmentName,
	}, nil
}

// refreshRunInputCandidateIssues 强制刷新当前规则候选，并确认配置中的外部对象仍属于当前账号可选集合。
func (s *PathConfigService) refreshRunInputCandidateIssues(ctx context.Context, account string, snapshot target.PathConfigurationSnapshot, template, values map[string]any) []model.RunInputPreflightIssue {
	if s.candidateCache == nil {
		return []model.RunInputPreflightIssue{}
	}
	types := componentCandidateTypes(template)
	if len(types) == 0 {
		return []model.RunInputPreflightIssue{}
	}
	set, _ := s.candidateCache.RefreshCandidateSet(ctx, account, snapshot.FlowCode, snapshot.TemplateID, snapshot.RuleVersion, types)
	byField := buildComponentCandidatesMap(template, set)
	fieldIssues := buildComponentCandidateIssues(template, set)
	issues := make([]model.RunInputPreflightIssue, 0)
	for fieldPath, message := range fieldIssues {
		if !runInputValueEmpty(runInputValueAt(values, fieldPath)) {
			issues = appendRunInputIssue(issues, "candidate_refresh_failed", "component_candidate", message, true, fieldPath)
		}
	}
	for fieldPath, candidates := range byField {
		selectedIDs := runInputCandidateIDs(runInputValueAt(values, fieldPath))
		if len(selectedIDs) == 0 {
			continue
		}
		allowed := make(map[string]bool)
		for _, candidate := range candidates {
			for _, id := range runInputCandidateIDs(candidate) {
				allowed[id] = true
			}
		}
		for _, id := range selectedIDs {
			if !allowed[id] {
				issues = appendRunInputIssue(issues, "candidate_no_longer_available", "component_candidate", "已保存外部对象不再属于当前账号候选", true, fieldPath)
				break
			}
		}
	}
	sort.Slice(issues, func(left, right int) bool { return issues[left].FieldPath < issues[right].FieldPath })
	return issues
}

// appendRunInputIssue 按代码与字段去重预检问题，避免同一根因重复阻断。
func appendRunInputIssue(issues []model.RunInputPreflightIssue, code, source, message string, canRetry bool, fieldPath string) []model.RunInputPreflightIssue {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "当前运行输入无法完成预检"
	}
	for _, issue := range issues {
		if issue.Code == code && issue.FieldPath == fieldPath {
			return issues
		}
	}
	return append(issues, model.RunInputPreflightIssue{Code: code, Source: source, FieldPath: fieldPath, Message: message, CanRetry: canRetry})
}

// runInputValueAt 按字段点路径读取保存值。
func runInputValueAt(values map[string]any, path string) any {
	var current any = values
	for _, part := range strings.Split(strings.TrimSpace(path), ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = object[part]
	}
	return current
}

// runInputValueEmpty 判断候选字段是否已有需要复验的真实值。
func runInputValueEmpty(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

// runInputCandidateIDs 递归提取对象、JSON 字符串和集合中的真实候选 ID。
func runInputCandidateIDs(value any) []string {
	result := make([]string, 0)
	seen := make(map[string]bool)
	var visit func(any)
	visit = func(current any) {
		switch typed := current.(type) {
		case string:
			var decoded any
			if json.Unmarshal([]byte(typed), &decoded) == nil {
				visit(decoded)
			}
		case map[string]any:
			if id := strings.TrimSpace(fmt.Sprint(typed["id"])); id != "" && id != "<nil>" && !seen[id] {
				seen[id] = true
				result = append(result, id)
			}
		case []any:
			for _, item := range typed {
				visit(item)
			}
		}
	}
	visit(value)
	sort.Strings(result)
	return result
}

// runInputShapeDigest 绑定渲染类型和当前规则模板形状。
func runInputShapeDigest(renderType target.FormRenderType, template map[string]any) string {
	return runInputDigest(map[string]any{"renderType": renderType, "templateVersion": formdata.TemplateVersion(template)})
}

// runInputSnapshotDigest 对不含捕获时间的运行事实生成稳定摘要，重复预检相同配置得到同一值。
func runInputSnapshotDigest(snapshot model.RunInputSnapshot) string {
	return runInputDigest(map[string]any{
		"version": snapshot.Version, "planId": snapshot.PlanID, "pathId": snapshot.PathID,
		"sequenceNo": snapshot.SequenceNo, "accountRef": snapshot.AccountRef, "flowSource": snapshot.FlowSource, "targetObjectRef": snapshot.TargetObjectRef,
		"renderType": snapshot.RenderType, "ruleVersion": snapshot.TemplateRuleVersion,
		"templateVersion": snapshot.FormTemplateVersion, "shapeDigest": snapshot.ShapeDigest,
		"configVersion": snapshot.ConfigVersion, "configRevision": snapshot.ConfigRevision, "nodeRevision": snapshot.NodeRevision, "formRevision": snapshot.FormRevision,
		"pathChoices": snapshot.PathChoices, "nodeFieldValues": snapshot.NodeFieldValues, "actionValues": snapshot.ActionValues,
		"confirmedNodeKeys": snapshot.ConfirmedNodeKeys, "formValues": snapshot.FormValues,
	})
}

// runInputDigest 使用 Go 稳定 JSON 键顺序生成 SHA-256 摘要。
func runInputDigest(value any) string {
	encoded, _ := json.Marshal(value)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}
