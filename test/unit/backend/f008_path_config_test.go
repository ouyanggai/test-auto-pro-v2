package backend_test

import (
	"strings"
	"testing"

	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
)

// TestF008ActionConfigurationUsesOneArrivalPerAction 验证可配置动作按到达顺序保存，不包含系统基础动作。
func TestF008ActionConfigurationUsesOneArrivalPerAction(t *testing.T) {
	target := analyzer.PathConfigNodeTarget{NodeID: "approval-a", Name: "审批", ActionKinds: map[string]bool{"reject_no_pass": true}}
	encoded, count, reason := analyzer.EncodePathConfigActions(target, []model.PathConfigConfiguredActionInput{{Key: "action-1", Kind: "reject_no_pass", Count: 2}})
	if reason != "" || count != 2 {
		t.Fatalf("动作配置编码失败：count=%d reason=%s", count, reason)
	}
	if !strings.Contains(encoded, `"version":1`) || strings.Contains(encoded, "arrivals") || strings.Contains(encoded, "target") {
		t.Fatalf("动作配置仍混入旧结构：%s", encoded)
	}
}

// TestF008ActionConfigurationRejectsUnsupportedInput 验证动作目录、次数和单节点上限由服务端约束。
func TestF008ActionConfigurationRejectsUnsupportedInput(t *testing.T) {
	target := analyzer.PathConfigNodeTarget{ActionKinds: map[string]bool{"reject_no_pass": true}}
	if encoded, count, reason := analyzer.EncodePathConfigActions(target, nil); reason != "" || count != 0 || !strings.Contains(encoded, `"actions":[]`) {
		t.Fatalf("默认空动作配置不应失败：count=%d reason=%s encoded=%s", count, reason, encoded)
	}
	if _, _, reason := analyzer.EncodePathConfigActions(target, []model.PathConfigConfiguredActionInput{{Kind: "draft_save", Count: 1}}); reason == "" {
		t.Fatal("不允许的动作没有被拒绝")
	}
	if _, _, reason := analyzer.EncodePathConfigActions(target, []model.PathConfigConfiguredActionInput{{Kind: "reject_no_pass", Count: 1}, {Kind: "reject_no_pass", Count: 1}}); reason == "" {
		t.Fatal("重复动作没有被拒绝")
	}
	for _, kind := range []string{"approve_pass", "submit", "transfer_approver", "transpond"} {
		if _, _, reason := analyzer.EncodePathConfigActions(target, []model.PathConfigConfiguredActionInput{{Kind: kind, Count: 1}}); reason == "" {
			t.Fatalf("系统基础或错误动作没有被拒绝：%s", kind)
		}
	}
}

// TestF008ActionExecutionCountIgnoresLegacyKeys 验证旧 action-plan 键不会进入新动作执行量统计。
func TestF008ActionExecutionCountIgnoresLegacyKeys(t *testing.T) {
	values := map[string]string{"action-plan:approval-a": `{"version":1,"arrivals":[]}`}
	if count, valid := analyzer.CountStoredPathConfigActionExecutions(values); count != 0 || !valid {
		t.Fatalf("旧动作键错误影响新统计：count=%d valid=%v", count, valid)
	}
	encoded, _, _ := analyzer.EncodePathConfigActions(analyzer.PathConfigNodeTarget{ActionKinds: map[string]bool{"reject_no_pass": true}}, []model.PathConfigConfiguredActionInput{{Kind: "reject_no_pass", Count: 1}})
	values[analyzer.PathConfigActionConfigurationStorageKey("approval-a")] = encoded
	if count, valid := analyzer.CountStoredPathConfigActionExecutions(values); count != 1 || !valid {
		t.Fatalf("新动作执行量统计错误：count=%d valid=%v", count, valid)
	}
}
