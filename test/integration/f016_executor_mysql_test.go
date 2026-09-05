package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
)

// f016RunRecordTables 是 F-016 迁移 026 必须建立的六张运行记录表。
var f016RunRecordTables = []string{
	"runs",
	"path_runs",
	"run_steps",
	"run_step_attempts",
	"run_events",
	"run_controls",
}

// TestF016MigrationCreatesRunRecordTables 用真实 MySQL 验证迁移 026 被现有迁移执行器实际应用：
// 六张运行记录表必须存在，且重复启动（重连重跑迁移）不破坏已有记录。
func TestF016MigrationCreatesRunRecordTables(t *testing.T) {
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-016 MySQL 集成测试缺少配置名：%v", missing)
	}
	cfg.Name = temporaryPlanDatabaseName(t)
	t.Cleanup(func() { dropTemporaryPlanDatabase(t, cfg) })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	database, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("临时计划数据库迁移失败：%v", err)
	}
	found := map[string]bool{}
	rows, err := database.DB.QueryContext(ctx, "SHOW TABLES")
	if err != nil {
		t.Fatalf("无法读取临时库表清单：%v", err)
	}
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatalf("无法读取表名：%v", err)
		}
		found[table] = true
	}
	rows.Close()
	for _, table := range f016RunRecordTables {
		if !found[table] {
			t.Fatalf("迁移 026 未建立运行记录表 %s：实际表 %v", table, found)
		}
	}

	// 服务可启动等价于：迁移后的库能再次通过 OpenAndMigrate（幂等重放全部迁移且记录不被破坏）。
	var runCount int
	if err := database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM runs").Scan(&runCount); err != nil {
		t.Fatalf("runs 表不可查询：%v", err)
	}
	database.Close()

	reopened, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("重复迁移（模拟服务重启）失败：%v", err)
	}
	defer reopened.Close()
	var reopenedRunCount int
	if err := reopened.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM runs").Scan(&reopenedRunCount); err != nil {
		t.Fatalf("重连后 runs 表不可查询：%v", err)
	}
	if reopenedRunCount != runCount {
		t.Fatalf("重复迁移破坏了运行记录：before=%d after=%d", runCount, reopenedRunCount)
	}
}

// TestF016RunStateAllocatesMonotonicRunNumbersAndGuardsStatus 用真实 MySQL 验证：
// 运行号在计划内单调递增、跨计划互不影响；状态只前进且每次前进同事务追加事件行；
// 非法回退被拒绝且不产生事件行。
func TestF016RunStateAllocatesMonotonicRunNumbersAndGuardsStatus(t *testing.T) {
	database := newF016RunDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store := planmysql.NewRunRepository(database.DB)

	first, firstPath, err := store.CreateRun(ctx, 1, 11, model.RunModeSingleStep, model.RunTriggerManual, nil, time.Now())
	if err != nil {
		t.Fatalf("创建第一次运行失败：%v", err)
	}
	second, _, err := store.CreateRun(ctx, 1, 11, model.RunModeSingleStep, model.RunTriggerManual, nil, time.Now())
	if err != nil {
		t.Fatalf("创建第二次运行失败：%v", err)
	}
	otherPlan, _, err := store.CreateRun(ctx, 2, 22, model.RunModeSingleStep, model.RunTriggerManual, nil, time.Now())
	if err != nil {
		t.Fatalf("创建其他计划运行失败：%v", err)
	}
	if !(first.RunNo == 1 && second.RunNo == 2 && otherPlan.RunNo == 1) {
		t.Fatalf("运行号必须在计划内单调递增：plan1=%d,%d plan2=%d", first.RunNo, second.RunNo, otherPlan.RunNo)
	}

	if _, err := store.AdvancePathRunStatus(ctx, firstPath.ID,
		model.PathRunStatusWaiting, model.PathRunStatusRunning, model.RunEvent{}, time.Now()); err != nil {
		t.Fatalf("等待运行 -> 运行中应被允许：%v", err)
	}
	if _, err := store.AdvancePathRunStatus(ctx, firstPath.ID,
		model.PathRunStatusRunning, model.PathRunStatusVerifying, model.RunEvent{}, time.Now()); err != nil {
		t.Fatalf("运行中 -> 核验中应被允许：%v", err)
	}
	if _, err := store.AdvancePathRunStatus(ctx, firstPath.ID,
		model.PathRunStatusVerifying, model.PathRunStatusRunning, model.RunEvent{}, time.Now()); err != nil {
		t.Fatalf("核验中 -> 运行中（步骤循环）应被允许：%v", err)
	}
	if _, err := store.AdvancePathRunStatus(ctx, firstPath.ID,
		model.PathRunStatusRunning, model.PathRunStatusWaiting, model.RunEvent{}, time.Now()); !errors.Is(err, repository.ErrRunStatusConflict) {
		t.Fatalf("运行中 -> 等待运行必须被拒绝，实际 err=%v", err)
	}

	// 事件行只随成功迁移追加：1 条路径创建事件 + 3 次成功迁移，被拒的迁移不产生事件。
	var eventCount int
	if err := database.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM run_events WHERE path_run_id = ?", firstPath.ID).Scan(&eventCount); err != nil {
		t.Fatalf("统计路径运行事件失败：%v", err)
	}
	if eventCount != 4 {
		t.Fatalf("路径运行事件行数应为 4（1 创建 + 3 迁移），实际 %d", eventCount)
	}
}

// TestF016LeaseMutexAndFencing 用真实 MySQL 验证租约互斥与 fencing token：
// 同一路径运行同时只有一个执行者；旧执行者凭旧 token 的续租与释放一律失效。
func TestF016LeaseMutexAndFencing(t *testing.T) {
	database := newF016RunDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store := planmysql.NewRunRepository(database.DB)
	now := time.Now()

	_, pathRun, err := store.CreateRun(ctx, 3, 33, model.RunModeSingleStep, model.RunTriggerManual, nil, now)
	if err != nil {
		t.Fatalf("创建路径运行失败：%v", err)
	}
	if _, err := store.AdvancePathRunStatus(ctx, pathRun.ID,
		model.PathRunStatusWaiting, model.PathRunStatusRunning, model.RunEvent{}, now); err != nil {
		t.Fatalf("进入运行中失败：%v", err)
	}

	tokenA, err := store.ClaimPathRunLease(ctx, pathRun.ID, "worker-a", time.Minute, now)
	if err != nil {
		t.Fatalf("worker-a 领取租约失败：%v", err)
	}
	if tokenA != 1 {
		t.Fatalf("首次领取后 fencing token 应为 1，实际 %d", tokenA)
	}
	if _, err := store.ClaimPathRunLease(ctx, pathRun.ID, "worker-b", time.Minute, now); !errors.Is(err, repository.ErrLeaseHeld) {
		t.Fatalf("worker-b 在有效租约期内领取必须被拒绝，实际 err=%v", err)
	}
	if err := store.RenewPathRunLease(ctx, pathRun.ID, "worker-a", tokenA+999, time.Minute, now); !errors.Is(err, repository.ErrStaleLease) {
		t.Fatalf("错误 token 的续租必须被拒绝，实际 err=%v", err)
	}
	if err := store.RenewPathRunLease(ctx, pathRun.ID, "worker-a", tokenA, time.Minute, now); err != nil {
		t.Fatalf("正确 token 续租失败：%v", err)
	}

	// 租约到期后 worker-b 可接管，fencing token 递增；worker-a 凭旧 token 释放必须失效。
	if _, err := store.ClaimPathRunLease(ctx, pathRun.ID, "worker-b", time.Minute, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("租约过期后接管失败：%v", err)
	}
	if err := store.ReleasePathRunLease(ctx, pathRun.ID, "worker-a", tokenA, now); !errors.Is(err, repository.ErrStaleLease) {
		t.Fatalf("旧执行者凭旧 token 释放必须被拒绝，实际 err=%v", err)
	}
	if err := store.ReleasePathRunLease(ctx, pathRun.ID, "worker-b", tokenA+1, now); err != nil {
		t.Fatalf("现执行者释放租约失败：%v", err)
	}
}

// TestF016CrashRecoveryForcesAwaitingReconciliation 用真实 MySQL 验证崩溃恢复：
// 处于运行中/核验中的路径运行一律置为待对账并写事件行；运行聚合保持原状留给对账切片；
// 待对账是终态，任何继续推进都被拒绝。
func TestF016CrashRecoveryForcesAwaitingReconciliation(t *testing.T) {
	database := newF016RunDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store := planmysql.NewRunRepository(database.DB)
	now := time.Now()

	_, runningPath, err := store.CreateRun(ctx, 4, 44, model.RunModeSingleStep, model.RunTriggerManual, nil, now)
	if err != nil {
		t.Fatalf("创建运行中路径失败：%v", err)
	}
	if _, err := store.AdvanceRunStatus(ctx, runningPath.RunID,
		model.RunStatusPending, model.RunStatusRunning, model.RunEvent{}, now); err != nil {
		t.Fatalf("运行聚合进入运行中失败：%v", err)
	}
	if _, err := store.AdvancePathRunStatus(ctx, runningPath.ID,
		model.PathRunStatusWaiting, model.PathRunStatusRunning, model.RunEvent{}, now); err != nil {
		t.Fatalf("进入运行中失败：%v", err)
	}
	_, verifyingPath, err := store.CreateRun(ctx, 5, 55, model.RunModeSingleStep, model.RunTriggerManual, nil, now)
	if err != nil {
		t.Fatalf("创建核验中路径失败：%v", err)
	}
	if _, err := store.AdvanceRunStatus(ctx, verifyingPath.RunID,
		model.RunStatusPending, model.RunStatusRunning, model.RunEvent{}, now); err != nil {
		t.Fatalf("运行聚合进入运行中失败：%v", err)
	}
	if _, err := store.AdvancePathRunStatus(ctx, verifyingPath.ID,
		model.PathRunStatusWaiting, model.PathRunStatusRunning, model.RunEvent{}, now); err != nil {
		t.Fatalf("进入运行中失败：%v", err)
	}
	if _, err := store.AdvancePathRunStatus(ctx, verifyingPath.ID,
		model.PathRunStatusRunning, model.PathRunStatusVerifying, model.RunEvent{}, now); err != nil {
		t.Fatalf("进入核验中失败：%v", err)
	}

	recovered, err := store.RecoverInterruptedPathRuns(ctx, now)
	if err != nil {
		t.Fatalf("崩溃恢复失败：%v", err)
	}
	if len(recovered) != 2 {
		t.Fatalf("应恢复 2 条路径运行，实际 %v", recovered)
	}
	for _, pathRunID := range recovered {
		pathRun, err := store.GetPathRun(ctx, pathRunID)
		if err != nil {
			t.Fatalf("读取恢复后的路径运行失败：%v", err)
		}
		if pathRun.Status != model.PathRunStatusAwaitingReconciliation {
			t.Fatalf("路径运行 %d 应为待对账，实际 %s", pathRunID, pathRun.Status)
		}
		if pathRun.LeaseOwner != "" {
			t.Fatalf("恢复时应释放租约，实际 owner=%s", pathRun.LeaseOwner)
		}
		run, err := store.GetRun(ctx, pathRun.RunID)
		if err != nil {
			t.Fatalf("读取运行聚合失败：%v", err)
		}
		if run.Status != model.RunStatusRunning {
			t.Fatalf("运行聚合应保持运行中留给对账切片，实际 %s", run.Status)
		}
		// 待对账不得跳过对账直接进入核验中：核验属于一次尝试的内部阶段，
		// 恢复只能从"回到运行中重新走一遍"开始（F-018 的确认前进 / 重放）。
		if _, err := store.AdvancePathRunStatus(ctx, pathRunID,
			model.PathRunStatusAwaitingReconciliation, model.PathRunStatusVerifying, model.RunEvent{}, now); !errors.Is(err, repository.ErrRunStatusConflict) {
			t.Fatalf("待对账不得直接进入核验中，实际 err=%v", err)
		}
		// 关键的"不会自动继续"保证落在推进权上：停在待对账的路径运行领不到租约，
		// 任何执行者都无法在没经过对账的情况下继续发写请求。
		// （待对账 -> 运行中 的迁移本身是合法的，但只有 F-018 的恢复动作会发起它，见
		//  test/unit/backend/executor/f018_recovery_reachable_test.go。）
		if _, err := store.ClaimPathRunLease(ctx, pathRunID, "worker-recovery", time.Minute, now); !errors.Is(err, repository.ErrRunStatusConflict) {
			t.Fatalf("待对账的路径运行不得领到推进权，实际 err=%v", err)
		}
	}

	// 再次恢复是无操作：待对账是终态，重复恢复不产生新事实。
	recoveredAgain, err := store.RecoverInterruptedPathRuns(ctx, now)
	if err != nil || len(recoveredAgain) != 0 {
		t.Fatalf("重复恢复应为无操作：recovered=%v err=%v", recoveredAgain, err)
	}
}

// newF016RunDatabase 建立一次性临时计划数据库并应用全部迁移，用例结束后销毁。
func newF016RunDatabase(t *testing.T) *planmysql.Database {
	t.Helper()
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-016 MySQL 集成测试缺少配置名：%v", missing)
	}
	cfg.Name = temporaryPlanDatabaseName(t)
	t.Cleanup(func() { dropTemporaryPlanDatabase(t, cfg) })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("临时计划数据库迁移失败：%v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// TestF016StepFactsAreInsertOnlyAndInstanceRefExclusive 用真实 MySQL 验证：
// 步骤与尝试事实成对落账且只增不改；主实例引用首次落库后不可改绑（一条路径运行独占一个实例）。
func TestF016StepFactsAreInsertOnlyAndInstanceRefExclusive(t *testing.T) {
	database := newF016RunDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store := planmysql.NewRunRepository(database.DB)
	now := time.Now()

	_, pathRun, err := store.CreateRun(ctx, 6, 66, model.RunModeSingleStep, model.RunTriggerManual, nil, now)
	if err != nil {
		t.Fatalf("创建路径运行失败：%v", err)
	}
	stepID, err := store.RecordStepAttempt(ctx, model.RunStep{
		PathRunID: pathRun.ID, StepNo: 1, Source: "user", Action: "submit", NodeKey: "node-start",
		ActorSummary: "测试账号", Status: model.RunStepSucceeded, StartedAt: now, FinishedAt: now,
	}, model.RunStepAttempt{
		PathRunID: pathRun.ID, AttemptNo: 1, Verdict: "confirmed_success", SideEffect: "none",
		Transport: "responded", Initial: "success_claim", Reread: "advanced",
		Reason: "响应成功且实例已前进", Basis: "isSuccess=true;重读已前进",
		TraceID: "trace-1", CurlTraceID: "trace-1", LogPath: "plans/x/runs/y/1/step.log", LogLine: 3, DurationMs: 42,
	}, now)
	if err != nil {
		t.Fatalf("步骤事实落账失败：%v", err)
	}
	if stepID == 0 {
		t.Fatal("步骤行 ID 不应为 0")
	}
	// 第二步再落一次：事实表只增，两行并存。
	if _, err := store.RecordStepAttempt(ctx, model.RunStep{
		PathRunID: pathRun.ID, StepNo: 2, Source: "user", Action: "approve", NodeKey: "node-audit",
		Status: model.RunStepFailed, StartedAt: now, FinishedAt: now,
	}, model.RunStepAttempt{
		PathRunID: pathRun.ID, AttemptNo: 1, Verdict: "confirmed_failure", SideEffect: "none",
		Transport: "connect_refused", Reason: "连接阶段未完成", Basis: "dial 失败",
		TraceID: "trace-2", CurlTraceID: "trace-2", LogPath: "plans/x/runs/y/1/step.log", LogLine: 5, DurationMs: 7,
	}, now); err != nil {
		t.Fatalf("第二步事实落账失败：%v", err)
	}
	var stepCount, attemptCount int
	if err := database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM run_steps WHERE path_run_id = ?", pathRun.ID).Scan(&stepCount); err != nil || stepCount != 2 {
		t.Fatalf("应有 2 行步骤事实：count=%d err=%v", stepCount, err)
	}
	if err := database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM run_step_attempts WHERE path_run_id = ?", pathRun.ID).Scan(&attemptCount); err != nil || attemptCount != 2 {
		t.Fatalf("应有 2 行尝试事实：count=%d err=%v", attemptCount, err)
	}

	if err := store.SetMainInstanceRef(ctx, pathRun.ID, "instance-1", now); err != nil {
		t.Fatalf("首次落库主实例引用失败：%v", err)
	}
	if err := store.SetMainInstanceRef(ctx, pathRun.ID, "instance-2", now); !errors.Is(err, repository.ErrRunStatusConflict) {
		t.Fatalf("主实例引用不可改绑，实际 err=%v", err)
	}
	var ref string
	if err := database.DB.QueryRowContext(ctx, "SELECT main_instance_ref FROM path_runs WHERE id = ?", pathRun.ID).Scan(&ref); err != nil || ref != "instance-1" {
		t.Fatalf("主实例引用应保持首次落库值：ref=%s err=%v", ref, err)
	}
}
