package action_orchestration_test

import (
	"testing"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// TestRetrievePlacedOnIntermediateAndNotLastHumanNode 锁定取回放置规则（任务书 4.1/4.2）：
// 中间节点在直接后继人工待办未处理时可以取回；路径末节点没有后继人工待办窗口，目标后端必然拒绝，故排除。
func TestRetrievePlacedOnIntermediateAndNotLastHumanNode(t *testing.T) {
	configuration := model.PathConfiguration{Groups: []model.PathConfigGroup{
		{Nodes: []model.PathConfigNode{
			{Key: "start", Kind: "start"},
			{Key: "audit-1", Kind: "common"},
		}},
		{Nodes: []model.PathConfigNode{
			{Key: "synergy-1", Kind: "synergy"},
			{Key: "cond-1", Kind: "condition"},
			{Key: "audit-last", Kind: "common"},
			{Key: "end", Kind: "end"},
		}},
	}}
	if got := service.LastHumanNodeKeyForTest(configuration); got != "audit-last" {
		t.Fatalf("路径最后一个可执行审批节点应为 audit-last，实际 %q", got)
	}

	// 没有任何人工节点时不返回宿主。
	systemOnly := model.PathConfiguration{Groups: []model.PathConfigGroup{
		{Nodes: []model.PathConfigNode{{Key: "start", Kind: "start"}, {Key: "cond", Kind: "condition"}, {Key: "end", Kind: "end"}}},
	}}
	if got := service.LastHumanNodeKeyForTest(systemOnly); got != "" {
		t.Fatalf("没有可执行审批节点时不应有末节点，实际 %q", got)
	}
}
