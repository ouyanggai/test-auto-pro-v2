package backend

import (
	"testing"

	"test-auto-pro-v2/internal/model"
)

// TestClassifyRuleIssues_Blocking 验证未知脚本和渲染协议都会阻断，其他未登记问题至少进入 partial。
func TestClassifyRuleIssues_Blocking(t *testing.T) {
	issues := []string{
		"表单渲染协议尚未识别",
		"动态脚本需要人工核对：requestFunc",
		"宿主页面缺少渲染标记",
	}

	completeness := model.ClassifyRuleIssues(issues)

	if len(completeness.Blocking) != 2 {
		t.Errorf("阻断问题数量应为 2，实际 %d", len(completeness.Blocking))
	}

	if len(completeness.Warning) != 1 {
		t.Errorf("需核对问题数量应为 1，实际 %d", len(completeness.Warning))
	}

	if len(completeness.Info) != 0 {
		t.Errorf("规则问题不能降为纯信息，实际 %d", len(completeness.Info))
	}

	if completeness.Readiness != model.RuleReadinessBlocked {
		t.Errorf("规则状态应为 blocked，实际 %s", completeness.Readiness)
	}
}

// TestClassifyRuleIssues_Warning 验证动态脚本按批准规则阻断，其余可选数据源问题保持需核对。
func TestClassifyRuleIssues_Warning(t *testing.T) {
	issues := []string{
		"动态脚本需要人工核对：requestFunc",
		"字段级错误路径未证明",
		"数据源无可用记录",
	}

	completeness := model.ClassifyRuleIssues(issues)

	if len(completeness.Blocking) != 1 {
		t.Errorf("阻断问题数量应为 1，实际 %d", len(completeness.Blocking))
	}

	if len(completeness.Warning) != 2 {
		t.Errorf("需核对问题数量应为 2，实际 %d", len(completeness.Warning))
	}

	if completeness.Readiness != model.RuleReadinessBlocked {
		t.Errorf("规则状态应为 blocked，实际 %s", completeness.Readiness)
	}
}

// TestClassifyRuleIssues_Ready 验证存在未分类问题时不能误报 complete。
func TestClassifyRuleIssues_Ready(t *testing.T) {
	issues := []string{
		"宿主页面缺少渲染标记",
	}

	completeness := model.ClassifyRuleIssues(issues)

	if len(completeness.Blocking) != 0 {
		t.Errorf("Expected 0 blocking issues, got %d", len(completeness.Blocking))
	}

	if len(completeness.Warning) != 1 {
		t.Errorf("需核对问题数量应为 1，实际 %d", len(completeness.Warning))
	}

	if len(completeness.Info) != 0 {
		t.Errorf("规则问题不能降为纯信息，实际 %d", len(completeness.Info))
	}

	if completeness.Readiness != model.RuleReadinessPartial {
		t.Errorf("规则状态应为 partial，实际 %s", completeness.Readiness)
	}
}

// TestClassifyRuleIssues_Empty 验证无问题的规则返回 complete。
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

// TestClassifyRuleIssues_UnknownComponent 验证未知组件保持阻断。
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

// TestClassifyRuleIssues_MixedSeverity 验证混合问题取最高严重级别并保留全部分组。
func TestClassifyRuleIssues_MixedSeverity(t *testing.T) {
	issues := []string{
		"表单渲染协议尚未识别",
		"Vue 请求常量「Api.test.getData」未在宿主 API 表中识别",
		"宿主页面缺少渲染标记",
		"动态脚本需要人工核对：responseFunc",
	}

	completeness := model.ClassifyRuleIssues(issues)

	if len(completeness.Blocking) != 2 {
		t.Errorf("阻断问题数量应为 2，实际 %d", len(completeness.Blocking))
	}

	if len(completeness.Warning) != 2 {
		t.Errorf("Expected 2 warning issues, got %d", len(completeness.Warning))
	}

	if len(completeness.Info) != 0 {
		t.Errorf("规则问题不能降为纯信息，实际 %d", len(completeness.Info))
	}

	if completeness.Readiness != model.RuleReadinessBlocked {
		t.Errorf("Expected blocked readiness, got %s", completeness.Readiness)
	}
}
