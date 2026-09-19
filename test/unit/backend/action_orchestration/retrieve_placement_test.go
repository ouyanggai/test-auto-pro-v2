package action_orchestration_test

import (
	"testing"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// TestRetrievePlacedOnLastExecutableHumanNode 锁定用户裁决与目标跨节点取回硬规则：
// 一键配置里「取回」只能落在路径最后一个可执行审批/协同节点上——它之后没有别的人工节点会推进其后继，
// 取回在运行时才可执行。中间的审批节点、以及系统/条件/结束节点都不是取回的合法宿主。
func TestRetrievePlacedOnLastExecutableHumanNode(t *testing.T) {
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
		t.Fatalf("取回宿主应为最后一个可执行审批节点 audit-last，实际 %q", got)
	}

	// 没有任何人工节点时不返回宿主（不给取回找一个必然失败的落点）。
	systemOnly := model.PathConfiguration{Groups: []model.PathConfigGroup{
		{Nodes: []model.PathConfigNode{{Key: "start", Kind: "start"}, {Key: "cond", Kind: "condition"}, {Key: "end", Kind: "end"}}},
	}}
	if got := service.LastHumanNodeKeyForTest(systemOnly); got != "" {
		t.Fatalf("没有可执行审批节点时不应给取回找落点，实际 %q", got)
	}
}
