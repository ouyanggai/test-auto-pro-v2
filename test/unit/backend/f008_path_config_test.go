package backend_test

import (
	"strings"
	"testing"

	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
)

// TestF008ActionConfigurationUsesOneArrivalPerAction 验证动作次数只表达真实再次到达，不生成旧动作计划结构。
func TestF008ActionConfigurationUsesOneArrivalPerAction(t *testing.T) {
	target := analyzer.PathConfigNodeTarget{NodeID: "approval-a", Name: "审批", ActionKinds: map[string]bool{"approve_pass": true, "reject_no_pass": true}}
	encoded, count, reason := analyzer.EncodePathConfigActions(target, []model.PathConfigConfiguredActionInput{{Key: "action-1", Kind: "approve_pass", Count: 2}})
	if reason != "" || count != 2 {
		t.Fatalf("动作配置编码失败：count=%d reason=%s", count, reason)
	}
	if !strings.Contains(encoded, `"version":1`) || strings.Contains(encoded, "arrivals") || strings.Contains(encoded, "target") {
		t.Fatalf("动作配置仍混入旧结构：%s", encoded)
	}
}

// TestF008ActionConfigurationRejectsUnsupportedInput 验证动作目录、次数和单节点上限由服务端约束。
func TestF008ActionConfigurationRejectsUnsupportedInput(t *testing.T) {
	target := analyzer.PathConfigNodeTarget{ActionKinds: map[string]bool{"approve_pass": true}}
	if _, _, reason := analyzer.EncodePathConfigActions(target, []model.PathConfigConfiguredActionInput{{Kind: "draft_save", Count: 1}}); reason == "" {
		t.Fatal("不允许的动作没有被拒绝")
	}
	if _, _, reason := analyzer.EncodePathConfigActions(target, []model.PathConfigConfiguredActionInput{{Kind: "approve_pass", Count: 11}}); reason == "" {
		t.Fatal("超出次数上限没有被拒绝")
	}
}

// TestF008ActionExecutionCountIgnoresLegacyKeys 验证旧 action-plan 键不会进入新动作执行量统计。
func TestF008ActionExecutionCountIgnoresLegacyKeys(t *testing.T) {
	values := map[string]string{"action-plan:approval-a": `{"version":1,"arrivals":[]}`}
	if count, valid := analyzer.CountStoredPathConfigActionExecutions(values); count != 0 || !valid {
		t.Fatalf("旧动作键错误影响新统计：count=%d valid=%v", count, valid)
	}
	encoded, _, _ := analyzer.EncodePathConfigActions(analyzer.PathConfigNodeTarget{ActionKinds: map[string]bool{"approve_pass": true}}, []model.PathConfigConfiguredActionInput{{Kind: "approve_pass", Count: 1}})
	values[analyzer.PathConfigActionConfigurationStorageKey("approval-a")] = encoded
	if count, valid := analyzer.CountStoredPathConfigActionExecutions(values); count != 1 || !valid {
		t.Fatalf("新动作执行量统计错误：count=%d valid=%v", count, valid)
	}
}
