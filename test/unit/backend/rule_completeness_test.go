package backend

import (
	"testing"

	"test-auto-pro-v2/internal/model"
)

func TestClassifyRuleIssues_Blocking(t *testing.T) {
	issues := []string{
		"表单渲染协议尚未识别",
		"动态脚本需要人工核对：requestFunc",
		"宿主页面缺少渲染标记",
	}

	completeness := model.ClassifyRuleIssues(issues)

	if len(completeness.Blocking) != 1 {
		t.Errorf("Expected 1 blocking issue, got %d", len(completeness.Blocking))
	}

	if len(completeness.Warning) != 1 {
		t.Errorf("Expected 1 warning issue, got %d", len(completeness.Warning))
	}

	if len(completeness.Info) != 1 {
		t.Errorf("Expected 1 info issue, got %d", len(completeness.Info))
	}

	if completeness.Readiness != model.RuleReadinessBlocked {
		t.Errorf("Expected blocked readiness, got %s", completeness.Readiness)
	}
}

func TestClassifyRuleIssues_Warning(t *testing.T) {
	issues := []string{
		"动态脚本需要人工核对：requestFunc",
		"字段级错误路径未证明",
		"数据源无可用记录",
	}

	completeness := model.ClassifyRuleIssues(issues)

	if len(completeness.Blocking) != 0 {
		t.Errorf("Expected 0 blocking issues, got %d", len(completeness.Blocking))
	}

	if len(completeness.Warning) != 3 {
		t.Errorf("Expected 3 warning issues, got %d", len(completeness.Warning))
	}

	if completeness.Readiness != model.RuleReadinessPartial {
		t.Errorf("Expected partial readiness, got %s", completeness.Readiness)
	}
}

func TestClassifyRuleIssues_Ready(t *testing.T) {
	issues := []string{
		"宿主页面缺少渲染标记",
	}

	completeness := model.ClassifyRuleIssues(issues)

	if len(completeness.Blocking) != 0 {
		t.Errorf("Expected 0 blocking issues, got %d", len(completeness.Blocking))
	}

	if len(completeness.Warning) != 0 {
		t.Errorf("Expected 0 warning issues, got %d", len(completeness.Warning))
	}

	if len(completeness.Info) != 1 {
		t.Errorf("Expected 1 info issue, got %d", len(completeness.Info))
	}

	if completeness.Readiness != model.RuleReadinessReady {
		t.Errorf("Expected ready readiness, got %s", completeness.Readiness)
	}
}

func TestClassifyRuleIssues_Empty(t *testing.T) {
	issues := []string{}

	completeness := model.ClassifyRuleIssues(issues)

	if len(completeness.Blocking) != 0 {
		t.Errorf("Expected 0 blocking issues, got %d", len(completeness.Blocking))
	}

	if len(completeness.Warning) != 0 {
		t.Errorf("Expected 0 warning issues, got %d", len(completeness.Warning))
	}

	if len(completeness.Info) != 0 {
		t.Errorf("Expected 0 info issues, got %d", len(completeness.Info))
	}

	if completeness.Readiness != model.RuleReadinessReady {
		t.Errorf("Expected ready readiness, got %s", completeness.Readiness)
	}
}

func TestClassifyRuleIssues_UnknownComponent(t *testing.T) {
	issues := []string{
		"未知自定义组件：custom-unknown",
	}

	completeness := model.ClassifyRuleIssues(issues)

	if len(completeness.Blocking) != 1 {
		t.Errorf("Expected 1 blocking issue, got %d", len(completeness.Blocking))
	}

	if completeness.Readiness != model.RuleReadinessBlocked {
		t.Errorf("Expected blocked readiness, got %s", completeness.Readiness)
	}
}

func TestClassifyRuleIssues_MixedSeverity(t *testing.T) {
	issues := []string{
		"表单渲染协议尚未识别",
		"Vue 请求常量「Api.test.getData」未在宿主 API 表中识别",
		"宿主页面缺少渲染标记",
		"动态脚本需要人工核对：responseFunc",
	}

	completeness := model.ClassifyRuleIssues(issues)

	if len(completeness.Blocking) != 1 {
		t.Errorf("Expected 1 blocking issue, got %d", len(completeness.Blocking))
	}

	if len(completeness.Warning) != 2 {
		t.Errorf("Expected 2 warning issues, got %d", len(completeness.Warning))
	}

	if len(completeness.Info) != 1 {
		t.Errorf("Expected 1 info issue, got %d", len(completeness.Info))
	}

	if completeness.Readiness != model.RuleReadinessBlocked {
		t.Errorf("Expected blocked readiness, got %s", completeness.Readiness)
	}
}
