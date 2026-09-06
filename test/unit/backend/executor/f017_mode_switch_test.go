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

// TestF017ModeSwitchAtSafeBoundaryAndIdempotent 锁定 2026-09-06 的运行中模式切换：
// 停在阶段 3 时立即生效；请求带控制版本，过期版本被拒绝；重复请求同一目标模式不产生第二条控制事实；
// 切换为单步后可用命令只剩「执行一步」，切换为自动后恢复三条命令。
func TestF017ModeSwitchAtSafeBoundaryAndIdempotent(t *testing.T) {
	database := newF016ControlDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	store := planmysql.NewRunRepository(database.DB)
	runService := run.NewService(store, "worker-mode-switch", time.Minute, time.Now)

	fakeTarget := &fakeTarget{
		afterSubmit:  &fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-audit"}, DueNodes: []string{"node-audit"}},
		dueTaskID:    "task-1",
		submitResult: &target.SubmitFlowInstanceResult{InstanceID: "instance-mode-switch", Status: "run"},
	}
	executor := step.NewExecutor(fakeTarget, &fakeSessions{}, runService, store, fixedRunConfig(), nil)
	controller := control.NewService(runService, executor, store, time.Now)

	runCtx := newRunContext([]model.CompiledActionStep{submitStep(), approveStep()})
	started, err := controller.StartWithMode(ctx, runCtx, model.RunModeSingleStep, nil)
	if err != nil {
		t.Fatalf("启动失败：%v", err)
	}
	pathRunID := started.PathRun.ID

	// 过期版本被拒绝：绝不基于旧状态切换。
	if _, err := controller.SwitchMode(ctx, pathRunID, model.RunModeAuto, 99); !errors.Is(err, control.ErrVersionConflict) {
		t.Fatalf("过期版本必须被拒绝，实际 err=%v", err)
	}

	var countFacts func() int
	countFacts = func() int {
		var count int
		if err := database.DB.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM run_controls WHERE path_run_id = ? AND kind IN ('mode_switch_req','mode_switched')", pathRunID).Scan(&count); err != nil {
			t.Fatalf("统计切换事实失败：%v", err)
		}
		return count
	}

	// 停在阶段 3（本步尚未开始）：切换立即生效，落「请求 + 生效」两条事实。
	view, err := controller.SwitchMode(ctx, pathRunID, model.RunModeAuto, 1)
	if err != nil {
		t.Fatalf("切换为自动失败：%v", err)
	}
	if view.Mode != model.RunModeAuto || view.PendingMode != nil {
		t.Fatalf("停在阶段 3 的切换应立即生效，实际 mode=%s pending=%v", view.Mode, view.PendingMode)
	}
	if got := countFacts(); got != 2 {
		t.Fatalf("应落 2 条切换事实（请求+生效），实际 %d", got)
	}
	if len(view.Commands) != 3 {
		t.Fatalf("自动模式下应有三条命令，实际 %v", view.Commands)
	}

	// 幂等：重复请求同一目标模式（即使携带旧版本语义上想再切一次）不产生新事实。
	view, err = controller.SwitchMode(ctx, pathRunID, model.RunModeAuto, 2)
	if err != nil {
		t.Fatalf("重复切换应作为幂等空操作成功：%v", err)
	}
	if got := countFacts(); got != 2 {
		t.Fatalf("重复请求不得产生新事实，实际 %d", got)
	}

	// 切回单步：命令集只剩执行一步。
	view, err = controller.SwitchMode(ctx, pathRunID, model.RunModeSingleStep, view.Version)
	if err != nil {
		t.Fatalf("切换回单步失败：%v", err)
	}
	if view.Mode != model.RunModeSingleStep {
		t.Fatalf("应已生效为单步，实际 %s", view.Mode)
	}
	if len(view.Commands) != 1 || view.Commands[0] != model.CommandStep {
		t.Fatalf("单步模式应只剩执行一步，实际 %v", view.Commands)
	}
	if got := countFacts(); got != 4 {
		t.Fatalf("两次切换应共 4 条事实，实际 %d", got)
	}
}
