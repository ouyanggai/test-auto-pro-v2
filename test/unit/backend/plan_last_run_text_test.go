package backend

import (
	"testing"
	"time"

	"test-auto-pro-v2/internal/api"
	"test-auto-pro-v2/internal/model"
)

// TestPlanLastRunText 验证「最近运行结果」按运行事实生成展示文案（F-032）。
func TestPlanLastRunText(t *testing.T) {
	now := time.Now().UTC()
	runNo := uint64(7)
	resultSucceeded := model.RunResultSucceeded
	resultFailed := model.RunResultFailed
	cases := []struct {
		name string
		plan model.Plan
		want string
	}{
		{"从未运行", model.Plan{}, "暂无运行记录"},
		{
			"活跃运行中",
			model.Plan{LastRunNo: &runNo, LastRunStatus: model.RunStatusRunning, HasActiveRun: true},
			"第 7 次运行中",
		},
		{
			"等待中也是活跃",
			model.Plan{LastRunNo: &runNo, LastRunStatus: model.RunStatusPending, HasActiveRun: true},
			"第 7 次运行中",
		},
		{
			"最近一次成功",
			model.Plan{LastRunNo: &runNo, LastRunStatus: model.RunStatusCompleted, LastRunResult: &resultSucceeded, LastRunFinishedAt: &now},
			"第 7 次：成功",
		},
		{
			"最近一次失败",
			model.Plan{LastRunNo: &runNo, LastRunStatus: model.RunStatusFailed, LastRunResult: &resultFailed},
			"第 7 次：失败",
		},
		{
			"停止无结果",
			model.Plan{LastRunNo: &runNo, LastRunStatus: model.RunStatusStopped},
			"第 7 次：已停止",
		},
		{
			"取消无结果",
			model.Plan{LastRunNo: &runNo, LastRunStatus: model.RunStatusCancelled},
			"第 7 次：已取消",
		},
	}
	for _, tc := range cases {
		if got := api.PlanLastRunText(tc.plan); got != tc.want {
			t.Fatalf("%s：got=%q want=%q", tc.name, got, tc.want)
		}
	}
}
