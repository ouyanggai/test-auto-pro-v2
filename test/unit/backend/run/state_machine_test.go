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
		// F-028 失败动作重试：失败 -> 运行中是唯一用户受控的终态出口（服务端校验确定失败后才允许）。
		{model.PathRunStatusFailed, model.PathRunStatusRunning},
		// 结果待确认已于 2026-09-06 随用户侧对账移除成为终局：不允许出现任何回到运行中的通路。
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
		// 结果待确认是终局：既不能回到运行中，也不得进入核验中等任何其他状态。
		{model.PathRunStatusAwaitingReconciliation, model.PathRunStatusVerifying},
		{model.PathRunStatusAwaitingReconciliation, model.PathRunStatusRunning},
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
		model.PathRunStatusVerifying:              "确认结果中",
		model.PathRunStatusPaused:                 "暂停",
		model.PathRunStatusCompleted:              "已完成",
		model.PathRunStatusFailed:                 "失败",
		model.PathRunStatusStopped:                "已停止",
		model.PathRunStatusCancelled:              "已取消",
		model.PathRunStatusAwaitingReconciliation: "结果待确认",
	}
	for status, name := range expected {
		if got := model.PathRunStatusName(status); got != name {
			t.Fatalf("状态 %s 的中文显示名应为 %s，实际 %s", status, name, got)
		}
	}
	if total := len(expected); total != 10 {
		t.Fatalf("路径运行状态应覆盖纲领九态加结果待确认，实际 %d 个", total)
	}
}

// TestF016TerminalPathRunStatuses 锁定停摆态集合：五个终态（含结果待确认）不会自行前进。
// F-028 起失败态多出一条用户显式重试的受控出口（失败 -> 运行中），其余终态仍一条出边都没有；
// 用户侧对账移除后（2026-09-06），写结果无法确认就是终局，不存在任何恢复动作或继续通路。
func TestF016TerminalPathRunStatuses(t *testing.T) {
	finished := []model.PathRunStatus{
		model.PathRunStatusCompleted,
		model.PathRunStatusFailed,
		model.PathRunStatusStopped,
		model.PathRunStatusCancelled,
		model.PathRunStatusAwaitingReconciliation,
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
		for _, next := range active {
			if model.CanAdvancePathRunStatus(status, next) {
				// 失败 -> 运行中是 F-028 唯一允许的终态出口，必须由用户显式重试触发，不在此禁止。
				if status == model.PathRunStatusFailed && next == model.PathRunStatusRunning {
					continue
				}
				t.Fatalf("终态 %s 不允许前进到 %s", status, next)
			}
		}
	}
	for _, status := range active {
		if model.IsTerminalPathRunStatus(status) {
			t.Fatalf("%s 不应为停摆态", status)
		}
	}
}
