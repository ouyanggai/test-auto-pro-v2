package service

import (
	"context"
	"encoding/json"
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/model"
)

const templateRuleStaleMessage = "模板已更新，请先到系统设置更新模板规则"

// applyStoredTemplateRules 将规则目录的安全投影覆写到本次真实流程快照，禁止在计划页面扫描宿主源码。
func (s *PathConfigService) applyStoredTemplateRules(ctx context.Context, account, source, targetObjectID string, snapshot target.PathConfigurationSnapshot) (target.PathConfigurationSnapshot, error) {
	snapshot.TemplateID = strings.TrimSpace(targetObjectID)
	if s.templateRules == nil || strings.TrimSpace(snapshot.FlowCode) == "" {
		snapshot.RuleStatus, snapshot.RuleIssues = snapshotRuleStatus(snapshot), snapshotRuleIssues(snapshot)
		return snapshot, nil
	}
	item, found, err := s.templateRules.GetByFlowCode(ctx, snapshot.FlowCode)
	if err != nil {
		return target.PathConfigurationSnapshot{}, &PathConfigError{Kind: PathConfigErrorStorage, Message: "本地模板规则目录暂不可用，请重试"}
	}
	if !found {
		snapshot.RuleStatus = model.RuleReadinessBlocked
		snapshot.RuleIssues = []string{"当前流程尚未建立本地模板规则，请先在系统设置完成分析"}
		return snapshot, nil
	}
	if !CanInitiatorUseRule(account, source, targetObjectID, snapshot, item) {
		return target.PathConfigurationSnapshot{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前账号无权使用该流程模板"}
	}
	if item.Status == "failed" || item.Status == "blocked" || item.RenderType != templateRuleRenderType(snapshot.RenderType) {
		snapshot.RuleStatus = model.RuleReadinessBlocked
		snapshot.RuleIssues = uniquePublicStrings(item.Issues)
		if len(snapshot.RuleIssues) == 0 {
			snapshot.RuleIssues = []string{"当前模板规则不可用，请先在系统设置重新分析"}
		}
		return snapshot, nil
	}
	completeness := model.ClassifyRuleIssues(item.Issues)
	snapshot.RuleStatus, snapshot.RuleIssues = completeness.Readiness, uniquePublicStrings(item.Issues)
	if item.RenderType == model.TemplateRuleRenderFormMaking {
		snapshot.RuleVersion = templateRuleVersion(item)
		formID := ""
		if len(snapshot.Forms) > 0 {
			formID = snapshot.Forms[0].ID
		}
		encoded, encodeErr := json.Marshal(item.RuleData["template"])
		if encodeErr != nil {
			snapshot.RuleStatus = model.RuleReadinessBlocked
			snapshot.RuleIssues = append(snapshot.RuleIssues, "本地 FormMaking 规则数据异常，请重新分析")
			return snapshot, nil
		}
		snapshot.Forms = []target.FormRuntimeTemplate{{ID: formID, Name: item.FlowName, TemplateData: string(encoded)}}
		markTemplateRuleStale(&snapshot, item.Stale)
		return snapshot, nil
	}
	if item.RenderType != model.TemplateRuleRenderVueCustom {
		return snapshot, nil
	}
	snapshot.RuleVersion = templateRuleVersion(item)
	page, ok := decodeVueCustomPageRule(item.RuleData["page"])
	if !ok {
		snapshot.RuleStatus = model.RuleReadinessBlocked
		snapshot.RuleIssues = append(snapshot.RuleIssues, "本地 Vue 页面规则数据异常，请重新分析")
		snapshot.VuePage = &target.VueCustomPageRule{Status: model.RuleReadinessBlocked, PageKey: snapshot.AuditWay, PageName: snapshot.AuditWay, Issues: []string{"本地 Vue 页面规则数据异常，请重新分析"}}
		return snapshot, nil
	}
	page.Status = vuePageRuleStatus(&page)
	if snapshot.RuleStatus == model.RuleReadinessReady && page.Status != model.RuleReadinessReady {
		snapshot.RuleStatus = page.Status
	}
	snapshot.VuePage = &page
	markTemplateRuleStale(&snapshot, item.Stale)
	return snapshot, nil
}

// markTemplateRuleStale 在旧规则已安全投影后覆盖为明确阻断，保留表单值和旧模板供人工查看。
func markTemplateRuleStale(snapshot *target.PathConfigurationSnapshot, stale bool) {
	if snapshot == nil || !stale {
		return
	}
	snapshot.RuleStatus = model.RuleReadinessBlocked
	snapshot.RuleIssues = []string{templateRuleStaleMessage}
	if snapshot.VuePage != nil {
		snapshot.VuePage.Status = model.RuleReadinessBlocked
		snapshot.VuePage.Issues = []string{templateRuleStaleMessage}
	}
}

// snapshotRuleStatus 汇总本次规则快照的完整性，目录未接入的测试边界仍按真实页面问题判定。
func snapshotRuleStatus(snapshot target.PathConfigurationSnapshot) string {
	if status := strings.TrimSpace(snapshot.RuleStatus); status != "" {
		return status
	}
	if snapshot.RenderType == target.FormRenderTypeVueCustom {
		return vuePageRuleStatus(snapshot.VuePage)
	}
	if snapshot.RenderType == target.FormRenderTypeUnknown {
		return model.RuleReadinessBlocked
	}
	return model.RuleReadinessReady
}

// snapshotRuleIssues 返回规则快照中可公开的字段级原因，并兼容直接注入 Vue 页面规则的测试快照。
func snapshotRuleIssues(snapshot target.PathConfigurationSnapshot) []string {
	issues := append([]string(nil), snapshot.RuleIssues...)
	if snapshot.VuePage != nil {
		issues = append(issues, snapshot.VuePage.Issues...)
	}
	if snapshot.RenderType == target.FormRenderTypeUnknown && len(issues) == 0 {
		issues = append(issues, "当前流程表单协议尚未完成分析")
	}
	return uniquePublicStrings(issues)
}

// vuePageRuleStatus 把 Vue 页面分析问题收敛为 complete、partial 或 blocked。
func vuePageRuleStatus(page *target.VueCustomPageRule) string {
	if page == nil {
		return model.RuleReadinessBlocked
	}
	status := strings.TrimSpace(page.Status)
	if status == model.RuleReadinessReady || status == model.RuleReadinessPartial || status == model.RuleReadinessBlocked {
		return status
	}
	return model.ClassifyRuleIssues(page.Issues).Readiness
}

// templateRuleVersion 以规则目录源指纹和分析器版本作为样本隔离版本，更新任一规则输入都会失效旧缓存。
func templateRuleVersion(item model.TemplateRuleCatalogItem) string {
	version := strings.TrimSpace(item.SourceFingerprint)
	if version == "" {
		version = strings.TrimSpace(item.AnalyzerVersion)
	}
	return version
}

// CanInitiatorUseRule 使用已由目标平台按计划账号验证的快照二次约束本地目录，目录来源账号不能扩大业务权限。
func CanInitiatorUseRule(account, source, targetObjectID string, snapshot target.PathConfigurationSnapshot, item model.TemplateRuleCatalogItem) bool {
	if strings.TrimSpace(account) == "" || strings.TrimSpace(snapshot.FlowCode) == "" || strings.TrimSpace(item.FlowCode) != strings.TrimSpace(snapshot.FlowCode) {
		return false
	}
	if strings.TrimSpace(source) == "new" && strings.TrimSpace(item.SourceTemplateID) != strings.TrimSpace(targetObjectID) {
		return false
	}
	return true
}

// decodeVueCustomPageRule 将 JSON 目录对象恢复为目标快照 DTO，并把空集合规范为稳定数组。
func decodeVueCustomPageRule(raw any) (target.VueCustomPageRule, bool) {
	data, err := json.Marshal(raw)
	if err != nil {
		return target.VueCustomPageRule{}, false
	}
	var page target.VueCustomPageRule
	if err := json.Unmarshal(data, &page); err != nil || strings.TrimSpace(page.PageKey) == "" {
		return target.VueCustomPageRule{}, false
	}
	if page.Fields == nil {
		page.Fields = []target.VueCustomFieldRule{}
	}
	if page.Issues == nil {
		page.Issues = []string{}
	}
	return page, true
}
