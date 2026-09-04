package target_test

import (
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
)

// TestResolveVueCustomPageUsesTargetAuditWay 验证无表单流程把目标审批方式交给复制运行时页面注册表。
func TestResolveVueCustomPageUsesTargetAuditWay(t *testing.T) {
	page := target.ResolveVueCustomPage(target.FormRenderTypeVueCustom, "contract_review", "合同评审表")
	if page == nil {
		t.Fatal("无表单流程没有生成运行时页面规则")
	}
	if page.ComponentName != "contract_review" || page.Route != "contract_review" || page.PageName != "合同评审表" {
		t.Fatalf("目标页面入口没有保留 auditWay：%+v", page)
	}
	if page.Status != "complete" || len(page.Issues) != 0 {
		t.Fatalf("完整页面入口被错误标记为阻塞：%+v", page)
	}
}

// TestResolveVueCustomPageKeepsFormMakingUnchanged 验证普通 FormMaking 配置不会生成自定义页面入口。
func TestResolveVueCustomPageKeepsFormMakingUnchanged(t *testing.T) {
	if page := target.ResolveVueCustomPage(target.FormRenderTypeFormMaking, "contract_review", "合同评审表"); page != nil {
		t.Fatalf("普通 FormMaking 被错误转换为自定义页面：%+v", page)
	}
}

// TestResolveVueCustomPageFallsBackToGenericPage 验证目标只返回无表单类型时仍生成可渲染入口，并提供常见中文字段标签。
func TestResolveVueCustomPageFallsBackToGenericPage(t *testing.T) {
	page := target.ResolveVueCustomPage(target.FormRenderTypeVueCustom, "", "普通流程")
	if page == nil || page.ComponentName != "NoFormFlow" || page.Status != "complete" {
		t.Fatalf("无 auditWay 的无表单流程没有回落到通用页面：%+v", page)
	}
	found := false
	for _, field := range page.Fields {
		if field.Path == "userInfo" && field.Name == "用户姓名" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("通用无表单页面缺少 userInfo 中文标签")
	}
}
