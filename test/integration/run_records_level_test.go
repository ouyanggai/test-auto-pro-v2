package integration_test

import (
	"context"
	"testing"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// 2026-09-06 交付验收：运行记录层级与安全删除。本文件用真实 MySQL 锁定：
// 运行中不可删除（先停止再删除）、删除整次运行的级联范围、总步骤数只冻结一次、跨计划列表。

// TestRunRecordsDeleteRequiresAllPathsClosed 锁定删除守卫：仍有未闭合路径时拒绝删除；
// 全部路径闭合后删除成功，且运行、路径、步骤、尝试、事件与控制事实全部消失（不留半删状态）。
func TestRunRecordsDeleteRequiresAllPathsClosed(t *testing.T) {
	store := openF020Store(t)
	ctx := context.Background()
	now := time.Now().UTC()

	runRow, pathRuns, err := store.runs.CreateRunWithPaths(ctx, repository.CreateRunInput{
		PlanID: 71, ExecutionPathIDs: []uint64{711, 712},
		Mode: model.RunModeAuto, Trigger: model.RunTriggerManual, MaxConcurrency: f020MaxConcurrency(1),
	})
	if err != nil {
		t.Fatalf("创建运行失败：%v", err)
	}
	if _, err := store.runs.AdvanceRunStatus(ctx, runRow.ID, model.RunStatusPending, model.RunStatusRunning,
		model.RunEvent{Kind: "run_started", Label: "测试"}, now); err != nil {
		t.Fatalf("推进运行失败：%v", err)
	}
	for _, pathRun := range pathRuns {
		if _, err := store.runs.AdvancePathRunStatus(ctx, pathRun.ID,
			model.PathRunStatusWaiting, model.PathRunStatusRunning,
			model.RunEvent{Kind: "path_run_started", Label: "开始"}, now); err != nil {
			t.Fatalf("启动路径失败：%v", err)
		}
	}
	// 落一条步骤/尝试与一条控制事实，用于验证级联删除的范围。
	stepID, err := store.runs.RecordStepAttempt(ctx, model.RunStep{
		PathRunID: pathRuns[0].ID, StepNo: 1, Source: "user", Action: "submit", NodeKey: "node-1",
		Status: model.RunStepSucceeded, StartedAt: now, FinishedAt: now,
	}, model.RunStepAttempt{
		PathRunID: pathRuns[0].ID, AttemptNo: 1, Verdict: "confirmed_success", TraceID: "t-1", LogPath: "logs/x", LogLine: 1,
	}, now)
	if err != nil {
		t.Fatalf("落账步骤失败：%v", err)
	}
	if err := store.runs.AppendRunControl(ctx, model.RunControl{
		RunID: runRow.ID, PathRunID: pathRuns[0].ID, Kind: model.ControlFactModeSelected,
		Source: model.RunControlSourceUI, CreatedAt: now,
	}, now); err != nil {
		t.Fatalf("落控制事实失败：%v", err)
	}

	// 运行中的记录必须先停止再删除。
	if err := store.runs.DeleteRun(ctx, runRow.ID, now); err == nil {
		t.Fatal("仍有运行中路径时删除必须被拒绝")
	}

	// 全部路径停止后删除成功。
	for _, pathRun := range pathRuns {
		if _, err := store.runs.FinishPathRun(ctx, pathRun.ID, model.PathRunStatusStopped, nil, nil,
			model.RunEvent{Kind: "path_run_finished", Label: "用户停止"}, now); err != nil {
			t.Fatalf("停止路径失败：%v", err)
		}
	}
	if err := store.runs.DeleteRun(ctx, runRow.ID, now); err != nil {
		t.Fatalf("全部闭合后删除失败：%v", err)
	}
	if _, err := store.runs.GetRun(ctx, runRow.ID); err == nil {
		t.Fatal("删除后运行必须不存在")
	}
	// 步骤与尝试按路径运行关联：路径行已删，用原路径运行 ID 与尝试 ID 反查确认没有孤儿行。
	for _, check := range []struct {
		table string
		query string
		arg   any
	}{
		{"path_runs", "SELECT COUNT(*) FROM path_runs WHERE run_id = ?", runRow.ID},
		{"run_steps", "SELECT COUNT(*) FROM run_steps WHERE path_run_id = ?", pathRuns[0].ID},
		{"run_step_attempts", "SELECT COUNT(*) FROM run_step_attempts WHERE step_id = ?", stepID},
		{"run_events", "SELECT COUNT(*) FROM run_events WHERE run_id = ?", runRow.ID},
		{"run_controls", "SELECT COUNT(*) FROM run_controls WHERE run_id = ?", runRow.ID},
	} {
		var count int
		if err := store.db.DB.QueryRowContext(ctx, check.query, check.arg).Scan(&count); err != nil {
			t.Fatalf("核对 %s 失败：%v", check.table, err)
		}
		if count != 0 {
			t.Fatalf("删除后 %s 仍有 %d 行记录", check.table, count)
		}
	}
}

// TestRunRecordsTotalStepsFreezeOnce 锁定进度分母的冻结语义：只允许写一次，
// 重复写入与后续变化都不改变已冻结的值（进度分母来自启动时快照，不随配置漂移）。
func TestRunRecordsTotalStepsFreezeOnce(t *testing.T) {
	store := openF020Store(t)
	ctx := context.Background()
	now := time.Now().UTC()

	runRow, pathRuns, err := store.runs.CreateRunWithPaths(ctx, repository.CreateRunInput{
		PlanID: 72, ExecutionPathIDs: []uint64{721},
		Mode: model.RunModeSingleStep, Trigger: model.RunTriggerManual, MaxConcurrency: nil,
	})
	if err != nil {
		t.Fatalf("创建运行失败：%v", err)
	}
	if err := store.runs.SetPathRunTotalSteps(ctx, pathRuns[0].ID, 7, now); err != nil {
		t.Fatalf("冻结总步骤失败：%v", err)
	}
	// 幂等重复调用与「配置变化后的新值」都不得改写冻结值。
	if err := store.runs.SetPathRunTotalSteps(ctx, pathRuns[0].ID, 9, now); err != nil {
		t.Fatalf("重复冻结调用失败：%v", err)
	}
	pathRun, err := store.runs.GetPathRun(ctx, pathRuns[0].ID)
	if err != nil {
		t.Fatalf("读取路径运行失败：%v", err)
	}
	if pathRun.TotalSteps == nil || *pathRun.TotalSteps != 7 {
		t.Fatalf("总步骤应冻结为 7，实际 %v", pathRun.TotalSteps)
	}
	_ = runRow
}

// TestRunRecordsListAllRunsAcrossPlans 锁定跨计划列表：不同计划的运行按最新在前返回，状态筛选生效。
func TestRunRecordsListAllRunsAcrossPlans(t *testing.T) {
	store := openF020Store(t)
	ctx := context.Background()
	now := time.Now().UTC()

	for _, planID := range []uint64{81, 82} {
		runRow, _, err := store.runs.CreateRunWithPaths(ctx, repository.CreateRunInput{
			PlanID: planID, ExecutionPathIDs: []uint64{planID * 10},
			Mode: model.RunModeSingleStep, Trigger: model.RunTriggerManual, MaxConcurrency: nil,
		})
		if err != nil {
			t.Fatalf("创建运行失败：%v", err)
		}
		if _, err := store.runs.AdvanceRunStatus(ctx, runRow.ID, model.RunStatusPending, model.RunStatusRunning,
			model.RunEvent{Kind: "run_started", Label: "测试"}, now); err != nil {
			t.Fatalf("推进运行失败：%v", err)
		}
	}
	all, err := store.runs.ListAllRunsFiltered(ctx, "", 0, 100)
	if err != nil {
		t.Fatalf("跨计划列表失败：%v", err)
	}
	if len(all) != 2 {
		t.Fatalf("应列出 2 次运行，实际 %d", len(all))
	}
	if all[0].ID <= all[1].ID {
		t.Fatal("列表必须按最新在前排序")
	}
	completed, err := store.runs.ListAllRunsFiltered(ctx, "completed", 0, 100)
	if err != nil {
		t.Fatalf("状态筛选失败：%v", err)
	}
	if len(completed) != 0 {
		t.Fatalf("没有已完成运行时筛选结果应为空，实际 %d", len(completed))
	}
}
