package service

import (
	"context"
	"encoding/json"
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/model"
)

// applyStoredTemplateRules 将规则目录的安全投影覆写到本次真实流程快照，禁止在计划页面扫描宿主源码。
func (s *PathConfigService) applyStoredTemplateRules(ctx context.Context, snapshot target.PathConfigurationSnapshot) (target.PathConfigurationSnapshot, error) {
	if s.templateRules == nil || strings.TrimSpace(snapshot.FlowCode) == "" {
		return snapshot, nil
	}
	item, found, err := s.templateRules.GetByFlowCode(ctx, snapshot.FlowCode)
	if err != nil {
		return target.PathConfigurationSnapshot{}, &PathConfigError{Kind: PathConfigErrorStorage, Message: "本地模板规则目录暂不可用，请重试"}
	}
	if !found || item.RenderType != model.TemplateRuleRenderVueCustom || snapshot.RenderType != target.FormRenderTypeVueCustom {
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
