package target

import "strings"

// ResolveVueCustomPage 将目标无表单审批方式转换为复制运行时可解析的页面入口。
// auditWay 是目标页面注册表使用的稳定键；工具不根据流程名称猜测组件，也不重建页面字段。
// 已实现的专用链路（noFormFlowRegistry 中 implemented）标记 complete，不再用“尚未逐页面实现”阻断配置；
// 未登记或未实现入口仍标记 partial 并携带阻断说明，禁止把通用字段清单伪装成完整协议。
func ResolveVueCustomPage(renderType FormRenderType, auditWay, flowName string) *VueCustomPageRule {
	if renderType != FormRenderTypeVueCustom {
		return nil
	}
	pageKey := strings.TrimSpace(auditWay)
	if pageKey == "" {
		// 目标旧接口有时只返回 no-form 类型而不带 auditWay；仍使用通用无表单入口，避免工作区空白。
		pageKey = "NoFormFlow"
	}
	pageName := strings.TrimSpace(flowName)
	componentName := pageKey
	status := "partial"
	issues := []string{"目标无表单页面「" + pageKey + "」存在专用业务链路（项目/业务关联、initiatorRange、并行或手动分支选人等），本工具尚未逐页面实现；发起/审批前会阻塞并要求人工在目标平台处理"}
	if spec, ok := LookupNoFormFlow(pageKey); ok {
		if spec.Status == SpecialBusinessImplemented && spec.Chain != NoFormFlowChainBlocked {
			status = "complete"
			issues = nil
			if name := strings.TrimSpace(spec.ComponentName); name != "" {
				componentName = name
			}
		} else if spec.UnimplementedWhy != "" {
			issues = []string{NoFormFlowBlockReason(pageKey, "submit")}
		}
	}
	return &VueCustomPageRule{
		Status:        status,
		PageKey:       pageKey,
		PageName:      pageName,
		ComponentName: componentName,
		Route:         pageKey,
		Fields:        vueCustomFieldRules(),
		Dependencies:  []VueCustomDependencyRule{},
		ReadRequests:  []VueCustomRequestRule{},
		Issues:        issues,
	}
}

// vueCustomFieldRules 返回无表单页面常见业务字段的中文标签，供路径条件提示和通用页面渲染复用。
func vueCustomFieldRules() []VueCustomFieldRule {
	labels := map[string]string{
		"userInfo": "用户姓名", "userName": "姓名", "companyName": "公司", "departmentName": "部门",
		"deptName": "部门", "projectName": "项目名称", "projectCode": "项目编号", "contractName": "合同名称",
		"contractNumber": "合同编号", "year": "年度",
	}
	fields := make([]VueCustomFieldRule, 0, len(labels))
	for path, name := range labels {
		fields = append(fields, VueCustomFieldRule{Path: path, Name: name, ValueType: "string", Evidence: "复制运行时公共字段标签"})
	}
	return fields
}
