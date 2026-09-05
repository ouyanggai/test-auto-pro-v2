package run_test

import (
	"testing"

	"test-auto-pro-v2/internal/model"
)

// TestF016PathRunStatusMachineOnlyAdvances 锁定路径运行状态机的合法前进表：
// 状态只前进不回退；核验中 -> 运行中是步骤循环的唯一“向后”通路；终态一律无出边。
func TestF016PathRunStatusMachineOnlyAdvances(t *testing.T) {
	legal := []struct {
		from model.PathRunStatus
		to   model.PathRunStatus
	}{
		{model.PathRunStatusNotStarted, model.PathRunStatusWaiting},
		{model.PathRunStatusWaiting, model.PathRunStatusRunning},
		{model.PathRunStatusWaiting, model.PathRunStatusCancelled},
		{model.PathRunStatusRunning, model.PathRunStatusVerifying},
		{model.PathRunStatusRunning, model.PathRunStatusPaused},
		{model.PathRunStatusRunning, model.PathRunStatusCompleted},
		{model.PathRunStatusRunning, model.PathRunStatusFailed},
		{model.PathRunStatusRunning, model.PathRunStatusAwaitingReconciliation},
		{model.PathRunStatusRunning, model.PathRunStatusStopped},
		{model.PathRunStatusRunning, model.PathRunStatusCancelled},
		{model.PathRunStatusVerifying, model.PathRunStatusRunning},
		{model.PathRunStatusVerifying, model.PathRunStatusCompleted},
		{model.PathRunStatusVerifying, model.PathRunStatusFailed},
		{model.PathRunStatusVerifying, model.PathRunStatusAwaitingReconciliation},
		{model.PathRunStatusVerifying, model.PathRunStatusStopped},
		{model.PathRunStatusVerifying, model.PathRunStatusCancelled},
		{model.PathRunStatusPaused, model.PathRunStatusRunning},
		{model.PathRunStatusPaused, model.PathRunStatusStopped},
		{model.PathRunStatusPaused, model.PathRunStatusCancelled},
		// 待对账 -> 运行中 是 F-018 显式扩展的唯一一条恢复通路：对账判「已生效」后确认前进、
		// 判「未生效」后重放本步，两者都必须先回到运行中。没有它，租约领不到、七阶段走不了，
		// 三个恢复动作全部不可达。
		{model.PathRunStatusAwaitingReconciliation, model.PathRunStatusRunning},
	}
	for _, item := range legal {
		if !model.CanAdvancePathRunStatus(item.from, item.to) {
			t.Fatalf("合法迁移被拒绝：%s -> %s", item.from, item.to)
		}
	}

	// 除核验中 -> 运行中外，一切从靠后状态回到靠前状态的迁移都非法。
	illegal := []struct {
		from model.PathRunStatus
		to   model.PathRunStatus
	}{
		{model.PathRunStatusWaiting, model.PathRunStatusNotStarted},
		{model.PathRunStatusRunning, model.PathRunStatusWaiting},
		{model.PathRunStatusRunning, model.PathRunStatusNotStarted},
		{model.PathRunStatusVerifying, model.PathRunStatusWaiting},
		{model.PathRunStatusVerifying, model.PathRunStatusPaused},
		{model.PathRunStatusCompleted, model.PathRunStatusRunning},
		{model.PathRunStatusCompleted, model.PathRunStatusVerifying},
		{model.PathRunStatusFailed, model.PathRunStatusRunning},
		// 待对账不得跳过对账直接进入核验中：核验是一次尝试的内部阶段，恢复只能从头走一遍。
		{model.PathRunStatusAwaitingReconciliation, model.PathRunStatusVerifying},
		{model.PathRunStatusStopped, model.PathRunStatusRunning},
		{model.PathRunStatusCancelled, model.PathRunStatusWaiting},
	}
	for _, item := range illegal {
		if model.CanAdvancePathRunStatus(item.from, item.to) {
			t.Fatalf("非法回退被放行：%s -> %s", item.from, item.to)
		}
	}
}

// TestF016NineChinesePathRunStates 锁定九个中文节点运行态的显示名：不新增、不改名，
// 界面必须使用这些中文而不是颜色或英文键单独表意。
func TestF016NineChinesePathRunStates(t *testing.T) {
	expected := map[model.PathRunStatus]string{
		model.PathRunStatusNotStarted:             "未开始",
		model.PathRunStatusWaiting:                "等待运行",
		model.PathRunStatusRunning:                "运行中",
		model.PathRunStatusVerifying:              "核验中",
		model.PathRunStatusPaused:                 "暂停",
		model.PathRunStatusCompleted:              "已完成",
		model.PathRunStatusFailed:                 "失败",
		model.PathRunStatusStopped:                "已停止",
		model.PathRunStatusCancelled:              "已取消",
		model.PathRunStatusAwaitingReconciliation: "待对账",
	}
	for status, name := range expected {
		if got := model.PathRunStatusName(status); got != name {
			t.Fatalf("状态 %s 的中文显示名应为 %s，实际 %s", status, name, got)
		}
	}
	if total := len(expected); total != 10 {
		t.Fatalf("路径运行状态应覆盖纲领九态加待对账，实际 %d 个", total)
	}
}

// TestF016TerminalPathRunStatuses 锁定停摆态集合与「可恢复」的边界：
// 四个运行级终态一条出边都没有；待对账同样停摆（执行循环不会自行继续），
// 但它是唯一可恢复的停摆态——F-018 的对账动作能把它带回运行中。
// 两者必须用不同的判据区分，否则恢复动作会被「终态一律拒绝」的守卫全部挡掉。
func TestF016TerminalPathRunStatuses(t *testing.T) {
	finished := []model.PathRunStatus{
		model.PathRunStatusCompleted,
		model.PathRunStatusFailed,
		model.PathRunStatusStopped,
		model.PathRunStatusCancelled,
	}
	active := []model.PathRunStatus{
		model.PathRunStatusNotStarted,
		model.PathRunStatusWaiting,
		model.PathRunStatusRunning,
		model.PathRunStatusVerifying,
		model.PathRunStatusPaused,
	}
	for _, status := range finished {
		if !model.IsTerminalPathRunStatus(status) {
			t.Fatalf("%s 应为停摆态", status)
		}
		if model.CanRecoverPathRunStatus(status) {
			t.Fatalf("%s 已真正结束，不得接受恢复动作", status)
		}
		for _, next := range active {
			if model.CanAdvancePathRunStatus(status, next) {
				t.Fatalf("终态 %s 不允许前进到 %s", status, next)
			}
		}
	}
	// 待对账：停摆但可恢复，且只有一条出边（回到运行中）。
	if !model.IsTerminalPathRunStatus(model.PathRunStatusAwaitingReconciliation) {
		t.Fatal("待对账对执行循环而言应为停摆态")
	}
	if !model.CanRecoverPathRunStatus(model.PathRunStatusAwaitingReconciliation) {
		t.Fatal("待对账必须是可恢复状态，否则三个恢复动作全部不可达")
	}
	for _, next := range active {
		allowed := model.CanAdvancePathRunStatus(model.PathRunStatusAwaitingReconciliation, next)
		if next == model.PathRunStatusRunning && !allowed {
			t.Fatal("待对账必须允许回到运行中（对账后确认前进/重放的落点）")
		}
		if next != model.PathRunStatusRunning && allowed {
			t.Fatalf("待对账只允许回到运行中，不得前进到 %s", next)
		}
	}
	for _, status := range active {
		if model.IsTerminalPathRunStatus(status) {
			t.Fatalf("%s 不应为停摆态", status)
		}
		if model.CanRecoverPathRunStatus(status) {
			t.Fatalf("%s 不是待对账，不应被当成可恢复停摆态", status)
		}
	}
}
