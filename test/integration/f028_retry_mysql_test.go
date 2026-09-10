package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// f028RecordStep 是测试辅助：按 F-028 场景落一行步骤事实与尝试事实（事实表只 INSERT）。
func f028RecordStep(t *testing.T, store f020Store, pathRunID uint64, stepNo int, action string, status model.RunStepStatus, now time.Time) {
	t.Helper()
	verdict := "confirmed_success"
	if status != model.RunStepSucceeded {
		verdict = "confirmed_failure"
	}
	if _, err := store.runs.RecordStepAttempt(context.Background(), model.RunStep{
		PathRunID: pathRunID, StepNo: stepNo, Source: "user", Action: action, NodeKey: "node-start",
		ActorSummary: "测试账号", Status: status, StartedAt: now, FinishedAt: now,
	}, model.RunStepAttempt{
		PathRunID: pathRunID, AttemptNo: 1, Verdict: verdict, SideEffect: "none",
		Transport: "responded", Initial: "success_claim", Reread: "advanced",
		Reason: "测试事实", Basis: "集成测试固定输入",
		TraceID: "trace-f028", CurlTraceID: "trace-f028", LogPath: "plans/x/runs/y/1/step.log", LogLine: 3, DurationMs: 42,
	}, now); err != nil {
		t.Fatalf("步骤事实落账失败：%v", err)
	}
}

// TestF028ReopenFailedPathRunForRetry 在真实 MySQL 上锁定重试装填的存储语义（F-028）：
// 失败态路径运行重开为运行中时清空聚合结论与结束时间、释放残留租约并追加事件行；
// 已落账事实一行不丢；运行聚合同步重开，重试后的再次收尾仍按既有镜像规则闭合。
func TestF028ReopenFailedPathRunForRetry(t *testing.T) {
	store := openF020Store(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	createdRun, pathRuns, err := store.runs.CreateRunWithPaths(ctx, repository.CreateRunInput{
		PlanID: 28, ExecutionPathIDs: []uint64{281},
		Mode: model.RunModeManual, Trigger: model.RunTriggerManual,
		MaxConcurrency: f028One(), PresetBreakpoints: `[{"type":"first_write"}]`,
	})
	if err != nil {
		t.Fatalf("创建运行失败：%v", err)
	}
	pathRun := pathRuns[0]
	// 推进到运行中后模拟「第 1 步成功、第 2 步确定失败、路径运行收尾为失败」的完整事实链。
	if _, err := store.runs.AdvanceRunStatus(ctx, createdRun.ID, model.RunStatusPending, model.RunStatusRunning,
		model.RunEvent{Kind: "run_started", Label: "运行开始"}, now); err != nil {
		t.Fatalf("运行推进失败：%v", err)
	}
	if _, err := store.runs.AdvancePathRunStatus(ctx, pathRun.ID, model.PathRunStatusWaiting, model.PathRunStatusRunning,
		model.RunEvent{Kind: "path_run_started", Label: "路径运行开始"}, now); err != nil {
		t.Fatalf("路径运行推进失败：%v", err)
	}
	// 运行中先领取租约，模拟「失败时残留一条过期租约行」的真实终态。
	if _, err := store.runs.ClaimPathRunLease(ctx, pathRun.ID, "stale-worker", time.Minute, now); err != nil {
		t.Fatalf("预置残留租约失败：%v", err)
	}
	f028RecordStep(t, store, pathRun.ID, 1, "submit", model.RunStepSucceeded, now)
	f028RecordStep(t, store, pathRun.ID, 2, "approve", model.RunStepFailed, now)
	failedClass := model.FailureClassTargetRejected
	failedResult := model.RunResultFailed
	if _, err := store.runs.FinishPathRun(ctx, pathRun.ID, model.PathRunStatusFailed, &failedResult, &failedClass,
		model.RunEvent{Kind: "path_run_finished", Label: "路径运行结束：失败"}, now); err != nil {
		t.Fatalf("失败收尾失败：%v", err)
	}

	reopenedPathRun, reopenedRun, err := store.runs.ReopenPathRunForRetry(ctx, pathRun.ID,
		model.RunEvent{Kind: "path_run_retry", Label: "用户重试失败动作，路径运行从第 2 步重新装填"}, now)
	if err != nil {
		t.Fatalf("重试装填失败：%v", err)
	}
	if reopenedPathRun.Status != model.PathRunStatusRunning {
		t.Fatalf("重开后路径运行应为运行中，实际 %s", reopenedPathRun.Status)
	}
	if reopenedPathRun.Result != nil || reopenedPathRun.FailureClass != nil || reopenedPathRun.FinishedAt != nil {
		t.Fatalf("重开后聚合结论与结束时间必须清空：%v %v %v", reopenedPathRun.Result, reopenedPathRun.FailureClass, reopenedPathRun.FinishedAt)
	}
	if reopenedPathRun.LeaseOwner != "" || reopenedPathRun.LeaseExpiresAt != nil {
		t.Fatalf("重开后残留租约必须释放：%s %v", reopenedPathRun.LeaseOwner, reopenedPathRun.LeaseExpiresAt)
	}
	if reopenedRun.Status != model.RunStatusRunning || reopenedRun.FinishedAt != nil {
		t.Fatalf("运行聚合同步重开为运行中并清空结束时间：status=%s finishedAt=%v", reopenedRun.Status, reopenedRun.FinishedAt)
	}

	// 事实只追加：两条步骤事实在重开后原样可读。
	steps, err := store.runs.ListRunSteps(ctx, pathRun.ID)
	if err != nil || len(steps) != 2 {
		t.Fatalf("重开不得改写事实：rows=%d err=%v", len(steps), err)
	}
	// 事件流必须留下重试与聚合重开的痕迹。
	events, err := store.runs.ListRunEvents(ctx, createdRun.ID, 0, 100, 0)
	if err != nil {
		t.Fatalf("读取事件流失败：%v", err)
	}
	kinds := map[string]bool{}
	for _, event := range events {
		kinds[event.Kind] = true
	}
	if !kinds["path_run_retry"] || !kinds["run_reopened"] {
		t.Fatalf("事件流缺少重试事件：%v", kinds)
	}

	// 重试后的执行再次收尾：聚合按既有镜像规则闭合，不需要任何特殊通路。
	succeeded := model.RunResultSucceeded
	if _, err := store.runs.FinishPathRun(ctx, pathRun.ID, model.PathRunStatusCompleted, &succeeded, nil,
		model.RunEvent{Kind: "path_run_finished", Label: "路径运行结束：已完成"}, now); err != nil {
		t.Fatalf("重试后收尾失败：%v", err)
	}
	finalRun, err := store.runs.GetRun(ctx, createdRun.ID)
	if err != nil || finalRun.Status != model.RunStatusCompleted {
		t.Fatalf("重试后的收尾应把运行聚合适回已完成：status=%s err=%v", finalRun.Status, err)
	}
}

// TestF028ReopenRejectsIllegalStates 锁定重试装填的拒绝边界：
// 非失败态拒绝；同一运行还有进行中的兄弟路径时拒绝；事实完整性由服务层校验，仓储层只守状态边界。
func TestF028ReopenRejectsIllegalStates(t *testing.T) {
	store := openF020Store(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	// 场景一：等待运行中的路径运行不是失败态，拒绝重开。
	_, waitingPaths, err := store.runs.CreateRunWithPaths(ctx, repository.CreateRunInput{
		PlanID: 282, ExecutionPathIDs: []uint64{2821},
		Mode: model.RunModeSingleStep, Trigger: model.RunTriggerManual, MaxConcurrency: f028One(),
	})
	if err != nil {
		t.Fatalf("创建等待运行失败：%v", err)
	}
	if _, _, err := store.runs.ReopenPathRunForRetry(ctx, waitingPaths[0].ID, model.RunEvent{}, now); !errors.Is(err, repository.ErrRunStatusConflict) {
		t.Fatalf("非失败态必须拒绝重开，实际 err=%v", err)
	}

	// 场景二：一条路径失败、另一条仍在运行中，拒绝重试以避免绕过并发语义。
	_, siblingPaths, err := store.runs.CreateRunWithPaths(ctx, repository.CreateRunInput{
		PlanID: 283, ExecutionPathIDs: []uint64{2831, 2832},
		Mode: model.RunModeSingleStep, Trigger: model.RunTriggerManual, MaxConcurrency: f028One(),
	})
	if err != nil {
		t.Fatalf("创建多路径运行失败：%v", err)
	}
	for _, pathRun := range siblingPaths {
		if _, err := store.runs.AdvancePathRunStatus(ctx, pathRun.ID, model.PathRunStatusWaiting, model.PathRunStatusRunning,
			model.RunEvent{Kind: "path_run_started", Label: "路径运行开始"}, now); err != nil {
			t.Fatalf("路径运行推进失败：%v", err)
		}
	}
	failedClass := model.FailureClassTargetRejected
	failedResult := model.RunResultFailed
	if _, err := store.runs.FinishPathRun(ctx, siblingPaths[0].ID, model.PathRunStatusFailed, &failedResult, &failedClass,
		model.RunEvent{Kind: "path_run_finished", Label: "路径运行结束：失败"}, now); err != nil {
		t.Fatalf("失败收尾失败：%v", err)
	}
	if _, _, err := store.runs.ReopenPathRunForRetry(ctx, siblingPaths[0].ID, model.RunEvent{}, now); !errors.Is(err, repository.ErrRunStatusConflict) {
		t.Fatalf("存在进行中兄弟路径时必须拒绝重开，实际 err=%v", err)
	}

	// 兄弟路径结束后同一条失败路径可以重开：证明拒绝只来自并发边界，不来自路径本身。
	if _, err := store.runs.AdvancePathRunStatus(ctx, siblingPaths[1].ID, model.PathRunStatusRunning, model.PathRunStatusCompleted,
		model.RunEvent{Kind: "path_run_finished", Label: "路径运行结束：已完成"}, now); err != nil {
		t.Fatalf("兄弟路径收尾失败：%v", err)
	}
	if _, _, err := store.runs.ReopenPathRunForRetry(ctx, siblingPaths[0].ID,
		model.RunEvent{Kind: "path_run_retry", Label: "用户重试失败动作"}, now); err != nil {
		t.Fatalf("并发边界解除后应允许重开：%v", err)
	}
}

// f028One 返回并发上限 1 的指针形态（F-028 用例固定单并发）。
func f028One() *int {
	value := 1
	return &value
}
