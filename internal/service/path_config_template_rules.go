package service

import (
	"context"
	"encoding/json"
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/model"
)

// applyStoredTemplateRules 将规则目录的安全投影覆写到本次真实流程快照，禁止在计划页面扫描宿主源码。
func (s *PathConfigService) applyStoredTemplateRules(ctx context.Context, account, source, targetObjectID string, snapshot target.PathConfigurationSnapshot) (target.PathConfigurationSnapshot, error) {
	if s.templateRules == nil || strings.TrimSpace(snapshot.FlowCode) == "" {
		return snapshot, nil
	}
	item, found, err := s.templateRules.GetByFlowCode(ctx, snapshot.FlowCode)
	if err != nil {
		return target.PathConfigurationSnapshot{}, &PathConfigError{Kind: PathConfigErrorStorage, Message: "本地模板规则目录暂不可用，请重试"}
	}
	if !found {
		return target.PathConfigurationSnapshot{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前流程尚未建立本地模板规则，请先在系统设置完成分析"}
	}
	if !CanInitiatorUseRule(account, source, targetObjectID, snapshot, item) {
		return target.PathConfigurationSnapshot{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前账号无权使用该流程模板"}
	}
	if item.Status == "failed" || item.RenderType != templateRuleRenderType(snapshot.RenderType) {
		return target.PathConfigurationSnapshot{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "当前模板规则不可用，请先在系统设置重新分析"}
	}
	if item.RenderType == model.TemplateRuleRenderFormMaking {
		encoded, encodeErr := json.Marshal(item.RuleData["template"])
		if encodeErr != nil {
			return target.PathConfigurationSnapshot{}, &PathConfigError{Kind: PathConfigErrorInvalid, Message: "本地 FormMaking 规则数据异常，请重新分析"}
		}
		snapshot.Forms = []target.FormRuntimeTemplate{{Name: item.FlowName, TemplateData: string(encoded)}}
		return snapshot, nil
	}
	if item.RenderType != model.TemplateRuleRenderVueCustom {
		return snapshot, nil
	}
	page, ok := decodeVueCustomPageRule(item.RuleData["page"])
	if !ok {
		snapshot.VuePage = &target.VueCustomPageRule{PageKey: snapshot.FlowCode, PageName: snapshot.FlowCode, Issues: []string{"本地 Vue 页面规则数据异常，请重新分析"}}
		return snapshot, nil
	}
	snapshot.VuePage = &page
	return snapshot, nil
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
