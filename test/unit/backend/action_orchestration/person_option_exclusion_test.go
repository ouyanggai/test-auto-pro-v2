package action_orchestration_test

import (
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
	"testing"
)

// buildTokenOption 按真实投影方式构造人员候选选项：Value 是不透明令牌，RawID 是原始候选 ID。
// 令牌与原始 ID 必然不相等，用来锁定过滤只能按 RawID 比对。
func buildTokenOption(nodeID, rawID, name string) model.PathConfigPersonOption {
	return model.PathConfigPersonOption{
		Label: name,
		Value: analyzer.PathConfigPersonOptionToken(nodeID, rawID),
		RawID: rawID,
	}
}

// TestFilterPersonOptionsWhitelistKeepsByRawID 锁定关键回归：
// 选项 Value 是不透明令牌，白名单以候选原始 ID 为键；过滤必须按 RawID 比对。
// 曾经按 Value（令牌）比原始 ID，令牌永不在白名单内，导致「审批人自选」候选被整批清空、节点无法配置。
func TestFilterPersonOptionsWhitelistKeepsByRawID(t *testing.T) {
	const nodeID = "node-approver-1"
	person := &model.PathConfigPerson{
		Title:    "审批人自选",
		Editable: true,
		Required: true,
		MinCount: 1,
		Options: []model.PathConfigPersonOption{
			buildTokenOption(nodeID, "user-valid-1", "张三"),
			buildTokenOption(nodeID, "user-valid-2", "李四"),
			buildTokenOption(nodeID, "user-ghost-3", "幽灵"),
		},
	}
	// 白名单只含两个有效用户；幽灵用户不在白名单内应被剔除。excluded 为空。
	validSet := map[string]bool{"user-valid-1": true, "user-valid-2": true}

	service.FilterPersonOptionsForTest(person, nil, validSet)

	if len(person.Options) != 2 {
		t.Fatalf("按 RawID 白名单过滤后应保留 2 个候选，实际 %d 个（若为 0 说明又在拿令牌比原始 ID）", len(person.Options))
	}
	gotRaw := map[string]bool{}
	for _, opt := range person.Options {
		gotRaw[opt.RawID] = true
		// 令牌必须保持不透明，不能因为过滤退化成原始 ID。
		if opt.Value == opt.RawID {
			t.Fatalf("候选回写键不应等于原始 ID：%s", opt.Value)
		}
	}
	if !gotRaw["user-valid-1"] || !gotRaw["user-valid-2"] {
		t.Fatalf("白名单内的候选被误删：%v", gotRaw)
	}
	if gotRaw["user-ghost-3"] {
		t.Fatalf("白名单外的幽灵候选未被剔除")
	}
}

// TestFilterPersonOptionsExcludedAndSelectedByRawID 锁定：手动排除名单按 RawID 生效，
// 且已选人员 Selected 存的是令牌，也必须映射回原始 ID 后再判定，被排除者从已选集中剔除。
func TestFilterPersonOptionsExcludedAndSelectedByRawID(t *testing.T) {
	const nodeID = "node-approver-2"
	keepToken := analyzer.PathConfigPersonOptionToken(nodeID, "user-keep")
	dropToken := analyzer.PathConfigPersonOptionToken(nodeID, "user-drop")
	person := &model.PathConfigPerson{
		Title:    "审批人自选",
		Editable: true,
		Options: []model.PathConfigPersonOption{
			buildTokenOption(nodeID, "user-keep", "留用"),
			buildTokenOption(nodeID, "user-drop", "排除"),
		},
		Selected: []string{keepToken, dropToken},
	}
	excluded := map[string]bool{"user-drop": true}

	service.FilterPersonOptionsForTest(person, excluded, nil)

	if len(person.Options) != 1 || person.Options[0].RawID != "user-keep" {
		t.Fatalf("排除名单未按 RawID 生效，剩余候选：%+v", person.Options)
	}
	if len(person.Selected) != 1 || person.Selected[0] != keepToken {
		t.Fatalf("已选集中的被排除令牌未被剔除，剩余：%v", person.Selected)
	}
}
