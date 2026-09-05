package executor_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/engine/control"
	"test-auto-pro-v2/internal/engine/run"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
)

// newF016ControlDatabase 建立一次性临时计划数据库（真实迁移），用例结束后销毁。
func newF016ControlDatabase(t *testing.T) *planmysql.Database {
	t.Helper()
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		// 这些用例的真实语义是集成用例（真实 MySQL）；run-f016.sh 带配置实跑、禁止跳过，
		// 只有脱离脚本单独跑包时才允许跳过，不再以 FAIL 干扰纯单测结果（评审低优先级 16）。
		t.Skipf("控制闭环集成测试缺少数据库配置名 %v，已跳过；请通过 test/run-f016.sh 实跑", missing)
	}
	buffer := make([]byte, 6)
	if _, err := rand.Read(buffer); err != nil {
		t.Fatal("无法生成临时数据库随机后缀")
	}
	cfg.Name = "test_auto_pro_v2_test_" + hex.EncodeToString(buffer)
	t.Cleanup(func() {
		if !config.ValidDatabaseName(cfg.Name) {
			t.Errorf("拒绝清理未通过校验的数据库名")
			return
		}
		database, err := planmysql.OpenAndMigrate(context.Background(), cfg)
		if err != nil {
			return
		}
		defer database.Close()
		_, _ = database.DB.Exec("DROP DATABASE IF EXISTS `" + cfg.Name + "`")
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("临时计划数据库迁移失败：%v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// TestF016SingleStepControlLoop 用真实 MySQL 走完整单步闭环：
// 启动停在第一步之前 -> 放行发起 -> 停在同意之前 -> 放行同意 -> 场景走完收尾重读并完成。
// 每次放行与停止都落 run_controls 事实；最终目标事实摘要与路径结果分开落库。
func TestF016SingleStepControlLoop(t *testing.T) {
	database := newF016ControlDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	store := planmysql.NewRunRepository(database.DB)
	runService := run.NewService(store, "worker-test", time.Minute, time.Now)

	fakeTarget := &fakeTarget{
		instance:     fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-audit"}, DueNodes: []string{"node-audit"}},
		afterSubmit:  &fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-audit"}, DueNodes: []string{"node-audit"}},
		afterAudit:   &fakeTargetView{Found: true, Status: "end", CurrentNodes: nil, DueNodes: nil},
		dueTaskID:    "task-1",
		submitResult: &target.SubmitFlowInstanceResult{InstanceID: "instance-77", Status: "run"},
		auditResult:  &target.AuditCurrentTaskResult{InstanceID: "instance-77", Status: "end"},
	}
	executor := step.NewExecutor(fakeTarget, &fakeSessions{}, runService, store, fixedRunConfig(), nil)
	controller := control.NewService(runService, executor, store, time.Now)

	runCtx := newRunContext([]model.CompiledActionStep{submitStep(), approveStep()})

	started, err := controller.Start(ctx, runCtx)
	if err != nil {
		t.Fatalf("启动失败：%v", err)
	}
	if started.PathFinished || started.Preview == nil {
		t.Fatalf("启动后应停在第一步之前：finished=%v", started.PathFinished)
	}
	if started.Run.Status != model.RunStatusRunning || started.PathRun.Status != model.PathRunStatusRunning {
		t.Fatalf("运行与路径运行都应处于运行中：%s / %s", started.Run.Status, started.PathRun.Status)
	}
	if fakeTarget.submitCalls != 0 {
		t.Fatalf("启动绝不允许发出写请求，实际 %d 次", fakeTarget.submitCalls)
	}

	// 第一次放行：发起，停在同意之前。
	first, err := controller.ApproveWithCommand(ctx, started.PathRun.ID, model.CommandStep, started.Preview.StepNo, 1)
	if err != nil {
		t.Fatalf("第一次放行失败：%v", err)
	}
	if fakeTarget.submitCalls != 1 || fakeTarget.auditCalls != 0 {
		t.Fatalf("第一步只应发出一次发起请求：submit=%d audit=%d", fakeTarget.submitCalls, fakeTarget.auditCalls)
	}
	if first.PathFinished || first.NextPreview == nil {
		t.Fatalf("发起后应停在同意之前：finished=%v", first.PathFinished)
	}

	// 第二次放行：同意，场景走完并收尾重读。游标与版本取自第一步之后的现场。
	secondCursor := first.NextPreview.StepNo
	secondVersion := int64(2)
	second, err := controller.ApproveWithCommand(ctx, started.PathRun.ID, model.CommandStep, secondCursor, secondVersion)
	if err != nil {
		t.Fatalf("第二次放行失败：%v", err)
	}
	if !second.PathFinished || second.FinalFacts == nil {
		t.Fatalf("场景走完应给出收尾重读：%+v", second)
	}
	if second.FinalFacts.StatusName != "已结束" || second.FinalFacts.InstanceRef != "instance-77" {
		t.Fatalf("最终目标事实应如实呈现：%+v", second.FinalFacts)
	}
	if fakeTarget.auditCalls != 1 {
		t.Fatalf("同意只应发出一次写请求，实际 %d 次", fakeTarget.auditCalls)
	}

	pathRun, err := store.GetPathRun(ctx, started.PathRun.ID)
	if err != nil {
		t.Fatalf("读取路径运行失败：%v", err)
	}
	if pathRun.Status != model.PathRunStatusCompleted {
		t.Fatalf("路径运行应已完成：%s", pathRun.Status)
	}
	if pathRun.Result == nil || *pathRun.Result != model.RunResultSucceeded {
		t.Fatalf("路径结果应为成功：%+v", pathRun.Result)
	}
	if pathRun.MainInstanceRef != "instance-77" {
		t.Fatalf("主实例引用应已落库：%s", pathRun.MainInstanceRef)
	}
	if pathRun.FinalTargetSummary == "" {
		t.Fatal("最终目标事实摘要应已落库")
	}
	finishedRun, err := store.GetRun(ctx, started.Run.ID)
	if err != nil {
		t.Fatalf("读取运行失败：%v", err)
	}
	if finishedRun.Status != model.RunStatusCompleted {
		t.Fatalf("运行应已完成：%s", finishedRun.Status)
	}
	var controlCount int
	// F-017 起控制事实含模式选定：模式选定 1 行 + 两次放行（带命令）2 行。
	if err := database.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM run_controls WHERE path_run_id = ? AND kind IN ('mode_selected','approved')", started.PathRun.ID).Scan(&controlCount); err != nil || controlCount != 3 {
		t.Fatalf("模式选定与两次放行应有三行控制事实：count=%d err=%v", controlCount, err)
	}
	var stepCount int
	if err := database.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM run_steps WHERE path_run_id = ?", started.PathRun.ID).Scan(&stepCount); err != nil || stepCount != 2 {
		t.Fatalf("两步应有两行步骤事实：count=%d err=%v", stepCount, err)
	}

	// 场景已走完：再放行必须被拒绝。
	if _, err := controller.ApproveWithCommand(ctx, started.PathRun.ID, model.CommandStep, 1, 1); !errors.Is(err, control.ErrNoActiveStep) && err == nil {
		t.Fatalf("场景走完后放行应被拒绝，实际 err=%v", err)
	}
}

// TestF016StopControl 用真实 MySQL 验证停止语义：
// 运行中可停止（终态、事实保留）；核验中停止延迟生效；终态不可再停止。
func TestF016StopControl(t *testing.T) {
	database := newF016ControlDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	store := planmysql.NewRunRepository(database.DB)
	runService := run.NewService(store, "worker-test", time.Minute, time.Now)

	fakeTarget := &fakeTarget{
		afterSubmit:  &fakeTargetView{Found: true, Status: "run", DueNodes: []string{"node-audit"}},
		submitResult: &target.SubmitFlowInstanceResult{InstanceID: "instance-88", Status: "run"},
	}
	executor := step.NewExecutor(fakeTarget, &fakeSessions{}, runService, store, fixedRunConfig(), nil)
	controller := control.NewService(runService, executor, store, time.Now)

	runCtx := newRunContext([]model.CompiledActionStep{submitStep(), approveStep()})
	started, err := controller.Start(ctx, runCtx)
	if err != nil {
		t.Fatalf("启动失败：%v", err)
	}

	// 运行中（停在第一步之前）停止：终态成立。
	stopped, err := controller.Stop(ctx, started.PathRun.ID)
	if err != nil {
		t.Fatalf("运行中停止失败：%v", err)
	}
	if stopped.Status != model.PathRunStatusStopped {
		t.Fatalf("路径运行应为已停止：%s", stopped.Status)
	}
	var controlCount int
	// F-017 起停止落「请求停止+停止生效」两行事实。
	if err := database.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM run_controls WHERE path_run_id = ? AND kind IN ('stop_requested','stopped')", started.PathRun.ID).Scan(&controlCount); err != nil || controlCount != 2 {
		t.Fatalf("停止应有两行控制事实：count=%d err=%v", controlCount, err)
	}
	if _, err := controller.Stop(ctx, started.PathRun.ID); !errors.Is(err, control.ErrRunAlreadyFinished) {
		t.Fatalf("终态不可再停止，实际 err=%v", err)
	}

	// 核验中停止延迟生效：直接把路径运行置为核验中模拟提交进行中。
	_, verifyingPath, err := store.CreateRun(ctx, 9, 99, model.RunModeSingleStep, model.RunTriggerManual, nil, time.Now())
	if err != nil {
		t.Fatalf("创建核验中路径失败：%v", err)
	}
	if _, err := store.AdvancePathRunStatus(ctx, verifyingPath.ID,
		model.PathRunStatusWaiting, model.PathRunStatusRunning, model.RunEvent{}, time.Now()); err != nil {
		t.Fatalf("进入运行中失败：%v", err)
	}
	if _, err := store.AdvancePathRunStatus(ctx, verifyingPath.ID,
		model.PathRunStatusRunning, model.PathRunStatusVerifying, model.RunEvent{}, time.Now()); err != nil {
		t.Fatalf("进入核验中失败：%v", err)
	}
	if _, err := controller.Stop(ctx, verifyingPath.ID); !errors.Is(err, control.ErrStopDeferred) {
		t.Fatalf("核验中停止应延迟生效，实际 err=%v", err)
	}
	var deferredCount int
	if err := database.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM run_controls WHERE path_run_id = ?", verifyingPath.ID).Scan(&deferredCount); err != nil || deferredCount != 0 {
		t.Fatalf("延迟生效的停止不得先落控制事实：count=%d err=%v", deferredCount, err)
	}
}
