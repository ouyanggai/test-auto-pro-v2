package integration_test

import (
	"context"
	"testing"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// 2026-09-06 产品裁决：用户侧对账功能整体移除。本文件锁定移除后的行为契约：
// 写结果无法确认是终局、运行聚合随之收尾、服务重启后的恢复同样收尾，绝不留下僵尸运行。

// TestF018UncertainWriteClosesRunAggregation 用真实 MySQL 锁定「不确定写 → 结果待确认 → 运行收尾」：
// 路径运行停在结果待确认后没有任何继续通路，全部路径闭合时运行聚合必须收尾为已停止并写事件行。
func TestF018UncertainWriteClosesRunAggregation(t *testing.T) {
	store := openF020Store(t)
	ctx := context.Background()
	now := time.Now().UTC()

	runRow, pathRuns, err := store.runs.CreateRunWithPaths(ctx, repository.CreateRunInput{
		PlanID: 61, ExecutionPathIDs: []uint64{611},
		Mode: model.RunModeSingleStep, Trigger: model.RunTriggerManual, MaxConcurrency: nil,
	})
	if err != nil {
		t.Fatalf("创建运行失败：%v", err)
	}
	if _, err := store.runs.AdvanceRunStatus(ctx, runRow.ID, model.RunStatusPending, model.RunStatusRunning,
		model.RunEvent{Kind: "run_started", Label: "测试"}, now); err != nil {
		t.Fatalf("推进运行失败：%v", err)
	}
	if _, err := store.runs.AdvancePathRunStatus(ctx, pathRuns[0].ID,
		model.PathRunStatusWaiting, model.PathRunStatusRunning,
		model.RunEvent{Kind: "path_run_started", Label: "开始"}, now); err != nil {
		t.Fatalf("启动路径失败：%v", err)
	}
	uncertain := model.FailureClassWriteUncertain
	if _, err := store.runs.FinishPathRun(ctx, pathRuns[0].ID,
		model.PathRunStatusAwaitingReconciliation, &[]model.RunResult{model.RunResultAwaitingReconcile}[0], &uncertain,
		model.RunEvent{Kind: "path_run_finished", Label: "写结果无法确认"}, now); err != nil {
		t.Fatalf("结果待确认落库失败：%v", err)
	}
	closed, err := store.runs.FinishRunIfAllPathsClosed(ctx, runRow.ID, now)
	if err != nil {
		t.Fatalf("聚合收尾失败：%v", err)
	}
	if !closed {
		t.Fatal("结果待确认是终局：单路径运行应立即收尾")
	}
	final, err := store.runs.GetRun(ctx, runRow.ID)
	if err != nil {
		t.Fatalf("读取运行失败：%v", err)
	}
	if final.Status != model.RunStatusStopped {
		t.Fatalf("不确定收尾的运行应聚合为已停止，实际 %s", model.RunStatusName(final.Status))
	}
	if final.FinishedAt == nil {
		t.Fatal("收尾后的运行必须带结束时间")
	}
}

// TestF018RecoveredRunsCloseAggregatesAfterRestart 锁定服务重启后的行为：
// 运行中/核验中的路径运行被恢复为结果待确认后，全部闭合的运行聚合必须在启动阶段收尾，
// 运行列表不得出现永远「运行中」的僵尸运行。
func TestF018RecoveredRunsCloseAggregatesAfterRestart(t *testing.T) {
	store := openF020Store(t)
	ctx := context.Background()
	now := time.Now().UTC()

	runRow, pathRuns, err := store.runs.CreateRunWithPaths(ctx, repository.CreateRunInput{
		PlanID: 62, ExecutionPathIDs: []uint64{621},
		Mode: model.RunModeAuto, Trigger: model.RunTriggerManual, MaxConcurrency: nil,
	})
	if err != nil {
		t.Fatalf("创建运行失败：%v", err)
	}
	if _, err := store.runs.AdvanceRunStatus(ctx, runRow.ID, model.RunStatusPending, model.RunStatusRunning,
		model.RunEvent{Kind: "run_started", Label: "测试"}, now); err != nil {
		t.Fatalf("推进运行失败：%v", err)
	}
	if _, err := store.runs.AdvancePathRunStatus(ctx, pathRuns[0].ID,
		model.PathRunStatusWaiting, model.PathRunStatusRunning,
		model.RunEvent{Kind: "path_run_started", Label: "开始"}, now); err != nil {
		t.Fatalf("启动路径失败：%v", err)
	}
	if _, err := store.runs.AdvancePathRunStatus(ctx, pathRuns[0].ID,
		model.PathRunStatusRunning, model.PathRunStatusVerifying,
		model.RunEvent{Kind: "path_run_verifying", Label: "模拟崩溃时正在核验"}, now); err != nil {
		t.Fatalf("推进核验中失败：%v", err)
	}

	recovered, err := store.runs.RecoverInterruptedPathRuns(ctx, now)
	if err != nil {
		t.Fatalf("启动恢复失败：%v", err)
	}
	found := false
	for _, id := range recovered {
		if id == pathRuns[0].ID {
			found = true
		}
	}
	if !found {
		t.Fatal("核验中的路径运行必须被启动恢复置为结果待确认")
	}
	pathRun, err := store.runs.GetPathRun(ctx, pathRuns[0].ID)
	if err != nil {
		t.Fatalf("读取路径运行失败：%v", err)
	}
	if pathRun.Status != model.PathRunStatusAwaitingReconciliation {
		t.Fatalf("恢复后的状态应为结果待确认，实际 %s", model.PathRunStatusName(pathRun.Status))
	}
	// 与 cmd/server/main.go 的启动收尾一致：恢复出的每条路径运行触发一次聚合收尾检查。
	if _, err := store.runs.FinishRunIfAllPathsClosed(ctx, runRow.ID, now); err != nil {
		t.Fatalf("启动收尾失败：%v", err)
	}
	final, err := store.runs.GetRun(ctx, runRow.ID)
	if err != nil {
		t.Fatalf("读取运行失败：%v", err)
	}
	if final.Status == model.RunStatusRunning {
		t.Fatal("重启恢复后全部闭合的运行不得停留在运行中")
	}
	if final.Status != model.RunStatusStopped {
		t.Fatalf("重启恢复后的运行应聚合为已停止，实际 %s", model.RunStatusName(final.Status))
	}
}
