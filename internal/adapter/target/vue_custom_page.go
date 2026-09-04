package target

import "strings"

// ResolveVueCustomPage 将目标无表单审批方式转换为复制运行时可解析的页面入口。
// auditWay 是目标页面注册表使用的稳定键；工具不根据流程名称猜测组件，也不重建页面字段。
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
	rule := &VueCustomPageRule{
		Status:        "complete",
		PageKey:       pageKey,
		PageName:      pageName,
		ComponentName: pageKey,
		Route:         pageKey,
		Fields:        vueCustomFieldRules(),
		Dependencies:  []VueCustomDependencyRule{},
		ReadRequests:  []VueCustomRequestRule{},
		Issues:        []string{},
	}
	return rule
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
