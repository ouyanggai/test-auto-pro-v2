package backend_test

import (
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
)

// TestF010NoFormRenderTypeRequiresVueData 验证宿主 Vue 表单不会因缺少 FormMaking JSON 被归类为无需数据。
func TestF010NoFormRenderTypeRequiresVueData(t *testing.T) {
	for _, marker := range []string{"noForm", "notForm", "NOFORM"} {
		if got := target.NormalizeFormRenderType(marker, 0); got != target.FormRenderTypeVueCustom {
			t.Fatalf("formExist=%q 被错误归类为 %q", marker, got)
		}
	}
}

// TestF010FormMakingAndUnknownRenderTypes 验证 FormMaking 与未识别协议保持独立，未识别协议不得被当成无需数据。
func TestF010FormMakingAndUnknownRenderTypes(t *testing.T) {
	if got := target.NormalizeFormRenderType("", 1); got != target.FormRenderTypeFormMaking {
		t.Fatalf("存在 FormMaking 模板时得到 %q", got)
	}
	if got := target.NormalizeFormRenderType("", 0); got != target.FormRenderTypeUnknown {
		t.Fatalf("缺少协议和模板时得到 %q", got)
	}
}
