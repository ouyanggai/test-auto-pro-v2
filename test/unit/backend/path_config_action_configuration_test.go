package backend_test

import (
	"strings"
	"testing"

	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
)

// TestPathConfigActionConfigurationEncodesSeparateCounts 验证动作次数和组合循环次数作为配置期 JSON 独立保存。
func TestPathConfigActionConfigurationEncodesSeparateCounts(t *testing.T) {
	target := analyzer.PathConfigNodeTarget{ActionKinds: map[string]bool{"approve_pass": true}}
	encoded, _, reason := analyzer.EncodePathConfigActionPlan(target, model.PathConfigActionPlanInput{
		Actions:          []model.PathConfigConfiguredActionInput{{Key: "action-1", Kind: "approve_pass", Count: 2}},
		CombinationCount: 3,
		Result:           model.PathConfigActionStepInput{Kind: "approve_pass"},
	})
	if reason != "" || !strings.Contains(encoded, `"combinationCount":3`) || !strings.Contains(encoded, `"actions":[{"kind":"approve_pass","count":2}`) {
		t.Fatalf("动作配置没有保存独立次数：encoded=%s reason=%s", encoded, reason)
	}
	if _, _, reason := analyzer.EncodePathConfigActionPlan(target, model.PathConfigActionPlanInput{CombinationCount: 11, Result: model.PathConfigActionStepInput{Kind: "approve_pass"}}); reason == "" {
		t.Fatal("超过上限的组合循环次数没有被拒绝")
	}
}
