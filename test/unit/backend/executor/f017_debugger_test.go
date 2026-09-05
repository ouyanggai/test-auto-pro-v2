package executor_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/control"
	"test-auto-pro-v2/internal/engine/run"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
)

// TestF017RunControlsAppendOnlyBreakpointReplayAndIdempotentApprove 用真实 MySQL 验证：
// run_controls 只 INSERT；断点增删由事实回放得出；重复放行只走一步（条件写幂等）。
func TestF017RunControlsAppendOnlyBreakpointReplayAndIdempotentApprove(t *testing.T) {
	database := newF016ControlDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	store := planmysql.NewRunRepository(database.DB)
	runService := run.NewService(store, "worker-f017", time.Minute, time.Now)

	fakeTarget := &fakeTarget{
		afterSubmit:  &fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-audit"}, DueNodes: []string{"node-audit"}},
		afterAudit:   &fakeTargetView{Found: true, Status: "end", DueNodes: nil},
		dueTaskID:    "task-1",
		submitResult: &target.SubmitFlowInstanceResult{InstanceID: "instance-f017", Status: "run"},
		auditResult:  &target.AuditCurrentTaskResult{InstanceID: "instance-f017", Status: "end"},
	}
	executor := step.NewExecutor(fakeTarget, &fakeSessions{}, runService, store, fixedRunConfig(), nil)
	controller := control.NewService(runService, executor, store, time.Now)

	runCtx := newRunContext([]model.CompiledActionStep{submitStep(), approveStep()})
	started, err := controller.StartWithMode(ctx, runCtx, model.RunModeSingleStep, []control.Breakpoint{
		{Type: model.BreakpointNode, NodeKey: "node-audit"},
	})
	if err != nil {
		t.Fatalf("启动失败：%v", err)
	}

	// 断点回放：默认两项（首次写+路径偏离）+ 预置节点断点。
	list, err := controller.ListBreakpoints(ctx, started.PathRun.ID)
	if err != nil {
		t.Fatalf("断点回放失败：%v", err)
	}
	if len(list) != 3 {
		t.Fatalf("应回放 3 个生效断点：%+v", list)
	}

	// 单步执行一步：成功，停在同意之前。
	first, err := controller.ApproveWithCommand(ctx, started.PathRun.ID, model.CommandStep, 1, 1)
	if err != nil {
		t.Fatalf("放行失败：%v", err)
	}
	if first.NextPreview == nil {
		t.Fatalf("单步后应停在下一步之前")
	}
	// 重复放行（相同游标与版本）必须被条件写拒绝，不产生第二次写。
	if _, err := controller.ApproveWithCommand(ctx, started.PathRun.ID, model.CommandStep, 1, 1); !errors.Is(err, control.ErrCursorConflict) && !errors.Is(err, control.ErrVersionConflict) {
		t.Fatalf("重复放行必须被条件写拒绝，实际 err=%v", err)
	}
	if fakeTarget.auditCalls != 0 {
		t.Fatalf("重复放行不得触发第二次写，实际 audit=%d 次", fakeTarget.auditCalls)
	}

	// 控制事实只 INSERT：统计后再次动作，行数只增。
	var before, after int
	if err := database.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM run_controls WHERE path_run_id = ?", started.PathRun.ID).Scan(&before); err != nil {
		t.Fatalf("统计控制事实失败：%v", err)
	}
	if _, err := controller.ApproveWithCommand(ctx, started.PathRun.ID, model.CommandStep, first.NextPreview.StepNo, 2); err != nil {
		t.Fatalf("第二次放行失败：%v", err)
	}
	if err := database.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM run_controls WHERE path_run_id = ?", started.PathRun.ID).Scan(&after); err != nil || after <= before {
		t.Fatalf("控制事实应只增：before=%d after=%d err=%v", before, after, err)
	}
	// 事实回放与内存集合一致（节点断点仍生效）。
	replay, err := controller.ListBreakpoints(ctx, started.PathRun.ID)
	if err != nil {
		t.Fatalf("二次回放失败：%v", err)
	}
	if len(replay) != 3 {
		t.Fatalf("回放断点应仍为 3 个：%+v", replay)
	}
}

// TestF017AutoModeFirstWriteBreakpointStopsBeforeWrite 用真实 MySQL 验证自动运行的安全阀：
// 自动启动后连续执行，在第一个写步骤的阶段 3 被首次写断点拦下，写请求没有发出。
func TestF017AutoModeFirstWriteBreakpointStopsBeforeWrite(t *testing.T) {
	database := newF016ControlDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	store := planmysql.NewRunRepository(database.DB)
	runService := run.NewService(store, "worker-f017", time.Minute, time.Now)

	fakeTarget := &fakeTarget{submitResult: &target.SubmitFlowInstanceResult{InstanceID: "instance-auto", Status: "run"}}
	executor := step.NewExecutor(fakeTarget, &fakeSessions{}, runService, store, fixedRunConfig(), nil)
	controller := control.NewService(runService, executor, store, time.Now)

	runCtx := newRunContext([]model.CompiledActionStep{submitStep(), approveStep()})
	started, err := controller.StartWithMode(ctx, runCtx, model.RunModeAuto, nil)
	if err != nil {
		t.Fatalf("自动启动失败：%v", err)
	}
	// 等待后台循环停在首次写断点（环询直至循环结束，上限 5 秒）。
	deadline := time.Now().Add(5 * time.Second)
	view := controller.View(started.PathRun.ID)
	for view != nil && view.LoopRunning && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		view = controller.View(started.PathRun.ID)
	}
	if view == nil {
		t.Fatal("控制现场丢失")
	}
	if view.LoopRunning {
		t.Fatal("循环应已停在首次写断点")
	}
	if fakeTarget.submitCalls != 0 {
		t.Fatalf("首次写断点必须拦在写请求之前，实际已发出 %d 次", fakeTarget.submitCalls)
	}
	if view.StopReason == "" || !containsString(view.StopReason, "首次写断点") {
		t.Fatalf("停止原因应说明命中首次写断点：%s", view.StopReason)
	}
	if len(view.Commands) != 3 {
		t.Fatalf("自动模式暂停后应有三条命令：%v", view.Commands)
	}
}

// containsString 测试辅助：包含判断。
func containsString(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && (haystack == needle || indexOf(haystack, needle) >= 0)
}

// indexOf 返回子串位置，找不到返回 -1。
func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// TestF017PausedPathRunSurvivesRestart 用真实 MySQL 验证：暂停态路径运行在重启后保持暂停，
// 不自动继续也不被误判成待对账；运行中转待对账的既有行为不受干扰；暂停后继续合法。
func TestF017PausedPathRunSurvivesRestart(t *testing.T) {
	database := newF016ControlDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store := planmysql.NewRunRepository(database.DB)
	now := time.Now()

	_, pausedPath, err := store.CreateRun(ctx, 17, 171, model.RunModeAuto, model.RunTriggerManual, nil, now)
	if err != nil {
		t.Fatalf("创建运行失败：%v", err)
	}
	if _, err := store.AdvancePathRunStatus(ctx, pausedPath.ID,
		model.PathRunStatusWaiting, model.PathRunStatusRunning, model.RunEvent{}, now); err != nil {
		t.Fatalf("进入运行中失败：%v", err)
	}
	if _, err := store.AdvancePathRunStatus(ctx, pausedPath.ID,
		model.PathRunStatusRunning, model.PathRunStatusPaused, model.RunEvent{}, now); err != nil {
		t.Fatalf("进入暂停失败：%v", err)
	}
	_, runningPath, err := store.CreateRun(ctx, 18, 181, model.RunModeSingleStep, model.RunTriggerManual, nil, now)
	if err != nil {
		t.Fatalf("创建运行中路径失败：%v", err)
	}
	if _, err := store.AdvancePathRunStatus(ctx, runningPath.ID,
		model.PathRunStatusWaiting, model.PathRunStatusRunning, model.RunEvent{}, now); err != nil {
		t.Fatalf("进入运行中失败：%v", err)
	}

	if _, err := store.RecoverInterruptedPathRuns(ctx, now); err != nil {
		t.Fatalf("恢复失败：%v", err)
	}
	paused, err := store.GetPathRun(ctx, pausedPath.ID)
	if err != nil {
		t.Fatalf("读取暂停路径失败：%v", err)
	}
	if paused.Status != model.PathRunStatusPaused {
		t.Fatalf("暂停态应保持暂停，实际 %s", paused.Status)
	}
	running, err := store.GetPathRun(ctx, runningPath.ID)
	if err != nil {
		t.Fatalf("读取运行中路径失败：%v", err)
	}
	if running.Status != model.PathRunStatusAwaitingReconciliation {
		t.Fatalf("运行中应被置为待对账，实际 %s", running.Status)
	}
	// 暂停 -> 运行中 是暂停后继续，不属于状态回退。
	if _, err := store.AdvancePathRunStatus(ctx, pausedPath.ID,
		model.PathRunStatusPaused, model.PathRunStatusRunning, model.RunEvent{}, now); err != nil {
		t.Fatalf("暂停后继续应被允许：%v", err)
	}
}

// TestF017ContinueLoopSurvivesRequestContextCancel 复核自动运行与「继续运行」的连续执行：
// 控制 API 是立即返回、循环在后台 goroutine 里跑的，而 HTTP 请求上下文在处理器返回后就被取消。
// 如果循环直接沿用请求上下文，第一步的目标调用会立刻拿到 context.Canceled，
// 传输档归类不出来即按不确定处理，于是"点继续运行"会把路径推进待对账，一次写都没发出。
func TestF017ContinueLoopSurvivesRequestContextCancel(t *testing.T) {
	database := newF016ControlDatabase(t)
	setupCtx, setupCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer setupCancel()
	store := planmysql.NewRunRepository(database.DB)
	runService := run.NewService(store, "worker-f017-loop", time.Minute, time.Now)

	fake := &fakeTarget{
		submitResult: &target.SubmitFlowInstanceResult{InstanceID: "instance-loop", Status: "run"},
		afterSubmit:  &fakeTargetView{Found: true, Status: "run"},
	}
	executor := step.NewExecutor(fake, &fakeSessions{}, runService, store, fixedRunConfig(), nil)
	controller := control.NewService(runService, executor, store, time.Now)

	runCtx := newRunContext([]model.CompiledActionStep{submitStep()})
	started, err := controller.StartWithMode(setupCtx, runCtx, model.RunModeAuto, nil)
	if err != nil {
		t.Fatalf("自动启动失败：%v", err)
	}
	pathRunID := started.PathRun.ID
	waitLoopIdle(t, controller, pathRunID)
	if fake.submitCalls != 0 {
		t.Fatalf("首次写断点应先拦住写请求，实际已发出 %d 次", fake.submitCalls)
	}

	// 用户看过预览后删除首次写断点并点「继续运行」：这是自动模式的正常用法。
	if _, err := controller.RemoveBreakpoint(setupCtx, pathRunID, control.Breakpoint{Type: model.BreakpointFirstWrite}); err != nil {
		t.Fatalf("删除首次写断点失败：%v", err)
	}
	view := controller.View(pathRunID)
	preview := controller.CurrentPreview(pathRunID)
	if view == nil || preview == nil {
		t.Fatal("控制现场丢失")
	}

	// 复刻真实调用形态：处理器返回即取消请求上下文。
	requestCtx, requestCancel := context.WithCancel(context.Background())
	if _, err := controller.ApproveWithCommand(requestCtx, pathRunID, model.CommandContinue, preview.StepNo, view.Version); err != nil {
		requestCancel()
		t.Fatalf("继续运行命令失败：%v", err)
	}
	requestCancel()
	waitLoopIdle(t, controller, pathRunID)

	pathRun, err := store.GetPathRun(setupCtx, pathRunID)
	if err != nil {
		t.Fatalf("读取路径运行失败：%v", err)
	}
	if fake.submitCalls != 1 {
		t.Fatalf("继续运行必须真实执行这一步，实际发出 %d 次写请求（路径运行状态 %s）",
			fake.submitCalls, model.PathRunStatusName(pathRun.Status))
	}
	if pathRun.Status == model.PathRunStatusAwaitingReconciliation {
		t.Fatalf("零写入或请求上下文取消不得把路径推进待对账：%s", model.PathRunStatusName(pathRun.Status))
	}
}

// waitLoopIdle 等待后台连续执行循环结束，上限 5 秒。
func waitLoopIdle(t *testing.T, controller *control.Service, pathRunID uint64) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		view := controller.View(pathRunID)
		if view == nil || !view.LoopRunning {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("后台循环 5 秒内未结束")
}
