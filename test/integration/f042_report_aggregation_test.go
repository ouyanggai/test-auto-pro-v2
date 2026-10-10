package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
	"test-auto-pro-v2/internal/service"
)

// TestF042ReportAggregation 锁定报告中心只读聚合的真实事实口径（F-042）：
// 在真实 MySQL 上播种一个计划、两条路径、三次路径运行与若干尝试，
// 断言仓储层分组聚合 SQL 与服务层 L0/L1/L2 指标（成功率、一次通过率、覆盖度、
// 结果分布、执行器耗时分解）与播种事实一致；接口耗时读取器传 nil，验证「无真实请求耗时」降级。
func TestF042ReportAggregation(t *testing.T) {
	store := openF020Store(t)
	db := store.db.DB
	ctx := context.Background()
	now := time.Now().UTC()

	// 播种一个计划与两条执行路径；路径 A 承载两次运行，路径 B 承载一次未结束运行。
	planResult, err := db.ExecContext(ctx,
		"INSERT INTO test_plans (create_key, name, account, account_display_name, flow_source, target_object_id, target_object_name, run_mode, status, created_at, updated_at) VALUES ('k-f042-report', 'F042报告计划', 'acct', '账号', 'new', 'obj', '报告流程', 'serial', 'not_started', NOW(3), NOW(3))")
	if err != nil {
		t.Fatalf("播种计划失败：%v", err)
	}
	planID, err := planResult.LastInsertId()
	if err != nil {
		t.Fatalf("读取计划 ID 失败：%v", err)
	}
	insertPath := func(seq int, key, name string) uint64 {
		res, insErr := db.ExecContext(ctx,
			"INSERT INTO test_execution_paths (plan_id, sequence_no, create_key, name, created_at, updated_at) VALUES (?, ?, ?, ?, NOW(3), NOW(3))",
			planID, seq, key, name)
		if insErr != nil {
			t.Fatalf("播种路径 %s 失败：%v", name, insErr)
		}
		id, idErr := res.LastInsertId()
		if idErr != nil {
			t.Fatalf("读取路径 ID 失败：%v", idErr)
		}
		return uint64(id)
	}
	pathA := insertPath(1, "path-f042-a", "路径甲")
	pathB := insertPath(2, "path-f042-b", "路径乙")

	// createRun 建一次运行并返回其唯一路径运行 ID。
	createRun := func(pathID uint64) uint64 {
		_, pathRuns, runErr := store.runs.CreateRunWithPaths(ctx, repository.CreateRunInput{
			PlanID: uint64(planID), ExecutionPathIDs: []uint64{pathID},
			Mode: model.RunModeAuto, Trigger: model.RunTriggerManual, MaxConcurrency: f020MaxConcurrency(1),
		})
		if runErr != nil {
			t.Fatalf("创建运行失败：%v", runErr)
		}
		return pathRuns[0].ID
	}
	// recordAttempt 落一条步骤与尝试，携带执行器真实计时与重放标记。
	recordAttempt := func(pathRunID uint64, stepNo, attemptNo int, durationMs, executorMs int64, executorKnown, replay bool) {
		if _, recErr := store.runs.RecordStepAttempt(ctx, model.RunStep{
			PathRunID: pathRunID, StepNo: stepNo, Source: "user", Action: "submit", NodeKey: "node",
			Status: model.RunStepSucceeded, StartedAt: now, FinishedAt: now,
		}, model.RunStepAttempt{
			PathRunID: pathRunID, AttemptNo: attemptNo, Verdict: "confirmed_success", TraceID: "t", LogPath: "logs/x", LogLine: 1,
			DurationMs: durationMs, ExecutorMs: executorMs, ExecutorMsKnown: executorKnown, IsReplay: replay,
		}, now); recErr != nil {
			t.Fatalf("落账尝试失败：%v", recErr)
		}
	}
	// finalizePathRun 直接把路径运行置为终态（仅测试播种用，不经业务状态机）。
	finalizePathRun := func(pathRunID uint64, status, result, failureClass string) {
		var resultArg, failureArg any
		if result != "" {
			resultArg = result
		}
		if failureClass != "" {
			failureArg = failureClass
		}
		if _, upErr := db.ExecContext(ctx,
			"UPDATE path_runs SET status = ?, result = ?, failure_class = ?, finished_at = NOW(3), updated_at = NOW(3) WHERE id = ?",
			status, resultArg, failureArg, pathRunID); upErr != nil {
			t.Fatalf("置终态失败：%v", upErr)
		}
	}

	// 运行一：路径甲，成功但发生过重放（非一次通过）；两条尝试各带真实执行器耗时。
	pathRun1 := createRun(pathA)
	recordAttempt(pathRun1, 1, 1, 100, 80, true, false)
	recordAttempt(pathRun1, 2, 2, 120, 90, true, true)
	finalizePathRun(pathRun1, string(model.PathRunStatusCompleted), string(model.RunResultSucceeded), "")

	// 运行二：路径甲，确定失败（门禁不通过）。
	pathRun2 := createRun(pathA)
	recordAttempt(pathRun2, 1, 1, 200, 0, false, false)
	finalizePathRun(pathRun2, string(model.PathRunStatusFailed), string(model.RunResultFailed), string(model.FailureClassGateBlocked))

	// 运行三：路径乙，未结束（不计入成功率分母，但计入覆盖度）。
	createRun(pathB)

	// —— 仓储层聚合 SQL 断言 ——
	reportStore := planmysql.NewReportRepository(db)
	counts, err := reportStore.ReportCounts(ctx)
	if err != nil {
		t.Fatalf("读取计数失败：%v", err)
	}
	if counts.PlansTotal != 1 || counts.PlansWithRuns != 1 || counts.RunsTotal != 3 || counts.PathRunsTotal != 3 {
		t.Fatalf("计数不符：%+v", counts)
	}
	rollups, err := reportStore.ReportPlanRollups(ctx)
	if err != nil {
		t.Fatalf("读取计划汇总失败：%v", err)
	}
	if len(rollups) != 1 {
		t.Fatalf("应得 1 条计划汇总，实际 %d", len(rollups))
	}
	rollup := rollups[0]
	if rollup.RunCount != 3 || rollup.PathRunCount != 3 || rollup.PathRunFinished != 2 ||
		rollup.PathRunSucceeded != 1 || rollup.PathRunFailed != 1 || rollup.CoveredPaths != 2 {
		t.Fatalf("计划汇总不符：%+v", rollup)
	}
	attemptRows, err := reportStore.ReportAttemptRows(ctx, repository.ReportScope{PlanID: uint64(planID)})
	if err != nil {
		t.Fatalf("读取尝试行失败：%v", err)
	}
	if len(attemptRows) != 3 {
		t.Fatalf("应得 3 条尝试行，实际 %d", len(attemptRows))
	}

	// —— 服务层三级聚合断言（日志根指向空目录：接口耗时降级为未知）——
	reports := service.NewReportCenterService(reportStore, service.NewPlanService(planmysql.NewPlanRepository(db)),
		planmysql.NewExecutionPathRepository(db), t.TempDir(), func() time.Time { return now })

	overview, err := reports.Overview(ctx)
	if err != nil {
		t.Fatalf("Overview 失败：%v", err)
	}
	if len(overview.Plans) != 1 {
		t.Fatalf("首页应有 1 个计划，实际 %d", len(overview.Plans))
	}
	if overview.Metrics.FinishedPathRuns != 2 || overview.Metrics.SucceededPathRuns != 1 {
		t.Fatalf("全局成功指标不符：%+v", overview.Metrics)
	}
	if overview.Metrics.SuccessRate != 0.5 || overview.Metrics.FirstPassRuns != 0 {
		t.Fatalf("成功率/一次通过不符：rate=%v fpy=%d", overview.Metrics.SuccessRate, overview.Metrics.FirstPassRuns)
	}
	if overview.Metrics.CoveredPaths != 2 || overview.Metrics.TotalPaths != 2 || overview.Metrics.PathCoverage != 1 {
		t.Fatalf("覆盖度不符：%+v", overview.Metrics)
	}
	if overview.Metrics.RetriedPathRuns != 1 {
		t.Fatalf("重试路径运行数应为 1，实际 %d", overview.Metrics.RetriedPathRuns)
	}
	if overview.Metrics.InterfaceKnown {
		t.Fatalf("未提供 network.log，接口耗时必须降级为未知")
	}
	if overview.Metrics.ResultDistribution["成功"] != 1 || overview.Metrics.ResultDistribution["失败"] != 1 {
		t.Fatalf("结果分布不符：%v", overview.Metrics.ResultDistribution)
	}

	planReport, err := reports.PlanReport(ctx, uint64(planID))
	if err != nil {
		t.Fatalf("PlanReport 失败：%v", err)
	}
	if len(planReport.Paths) != 2 {
		t.Fatalf("计划应有 2 条路径卡片，实际 %d", len(planReport.Paths))
	}

	pathReport, err := reports.PathReport(ctx, uint64(planID), pathA)
	if err != nil {
		t.Fatalf("PathReport 失败：%v", err)
	}
	if len(pathReport.Runs) != 2 {
		t.Fatalf("路径甲应有 2 次运行实例，实际 %d", len(pathReport.Runs))
	}
	for _, run := range pathReport.Runs {
		if run.PathRunID != pathRun1 {
			continue
		}
		// 运行一：两步共 220ms；执行器耗时 80+90=170ms（真实已知）；接口未知 → 编排耗时未知，不硬凑。
		if run.TotalDurationMs != 220 {
			t.Fatalf("运行一步骤总耗时应为 220，实际 %d", run.TotalDurationMs)
		}
		if !run.ExecutorDurationKnown || run.ExecutorDurationMs != 170 {
			t.Fatalf("运行一执行器耗时应为 170/已知，实际 %d/%v", run.ExecutorDurationMs, run.ExecutorDurationKnown)
		}
		if run.InterfaceDurationKnown {
			t.Fatalf("运行一接口耗时无 network.log，必须为未知")
		}
		if run.OrchestrationKnown {
			t.Fatalf("接口分量未知时编排耗时必须为未知，不硬凑")
		}
	}
}

// TestF042ReportInterfaceStatsFromNetworkLog 锁定本轮口径修正与耗时分解口径（F-042）：
// 在真实 MySQL 播种的运行记录之外，造一份真实 network.log，断言报告的接口调用次数按
// 路径运行全量计（含 step_id=- 的登录/结构读取行，旧口径整行漏计），并且
// 接口耗时/执行器耗时/工具端编排耗时的派生值与播种事实逐项对得上。
func TestF042ReportInterfaceStatsFromNetworkLog(t *testing.T) {
	store := openF020Store(t)
	db := store.db.DB
	ctx := context.Background()
	now := time.Now().UTC()

	planResult, err := db.ExecContext(ctx,
		"INSERT INTO test_plans (create_key, name, account, account_display_name, flow_source, target_object_id, target_object_name, run_mode, status, created_at, updated_at) VALUES ('k-f042-netlog', 'F042网络日志计划', 'acct', '账号', 'new', 'obj', '报告流程', 'serial', 'not_started', NOW(3), NOW(3))")
	if err != nil {
		t.Fatalf("播种计划失败：%v", err)
	}
	planID, err := planResult.LastInsertId()
	if err != nil {
		t.Fatalf("读取计划 ID 失败：%v", err)
	}
	pathResult, err := db.ExecContext(ctx,
		"INSERT INTO test_execution_paths (plan_id, sequence_no, create_key, name, created_at, updated_at) VALUES (?, 1, 'path-f042-netlog', '路径甲', NOW(3), NOW(3))",
		planID)
	if err != nil {
		t.Fatalf("播种路径失败：%v", err)
	}
	pathID, err := pathResult.LastInsertId()
	if err != nil {
		t.Fatalf("读取路径 ID 失败：%v", err)
	}
	_, pathRuns, err := store.runs.CreateRunWithPaths(ctx, repository.CreateRunInput{
		PlanID: uint64(planID), ExecutionPathIDs: []uint64{uint64(pathID)},
		Mode: model.RunModeAuto, Trigger: model.RunTriggerManual, MaxConcurrency: f020MaxConcurrency(1),
	})
	if err != nil {
		t.Fatalf("创建运行失败：%v", err)
	}
	pathRunID := pathRuns[0].ID
	// 两步尝试：共 220ms，执行器真实计时 170ms（两步都已知）。
	attemptRows := []struct {
		step, number  int
		duration      int64
		executor      int64
		executorKnown bool
	}{
		{1, 1, 100, 80, true},
		{2, 1, 120, 90, true},
	}
	for _, attempt := range attemptRows {
		if _, recErr := store.runs.RecordStepAttempt(ctx, model.RunStep{
			PathRunID: pathRunID, StepNo: attempt.step, Source: "user", Action: "submit", NodeKey: "node",
			Status: model.RunStepSucceeded, StartedAt: now, FinishedAt: now,
		}, model.RunStepAttempt{
			PathRunID: pathRunID, AttemptNo: attempt.number, Verdict: "confirmed_success", TraceID: "t",
			LogPath: "logs/netlog/step.log", LogLine: 1,
			DurationMs: attempt.duration, ExecutorMs: attempt.executor, ExecutorMsKnown: attempt.executorKnown,
		}, now); recErr != nil {
			t.Fatalf("落账尝试失败：%v", recErr)
		}
	}
	succeeded := string(model.RunResultSucceeded)
	if _, upErr := db.ExecContext(ctx,
		"UPDATE path_runs SET status = 'completed', result = ?, finished_at = NOW(3), updated_at = NOW(3) WHERE id = ?",
		succeeded, pathRunID); upErr != nil {
		t.Fatalf("置终态失败：%v", upErr)
	}

	// 真实 network.log：4 行里有 2 行 step_id=-（模板读取与待办轮询，旧口径会整行漏计），
	// 其中 1 行还缺 duration_s（计入次数、不计耗时样本）。
	logRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(logRoot, "logs", "netlog"), 0o755); err != nil {
		t.Fatalf("建日志目录失败：%v", err)
	}
	line := func(stepID, endpoint, class, duration string) string {
		classField := "request_class=" + class + " "
		if class == "" {
			classField = ""
		}
		return "time=2026-10-10_10:00:00 level=info path_run_id=" + strconv.FormatUint(pathRunID, 10) +
			" step_id=" + stepID + " attempt=1 endpoint=" + endpoint + " " + classField +
			" status_code=200 duration_s=" + duration + " result=success\n"
	}
	body := line("-", "/api/web/flowTemplateApi/list", "", "0.005") +
		line("1", "/api/web/flowInstanceApi/submit", "write", "0.010") +
		line("2", "/api/web/flowInstanceApi/audit", "write", "0.020") +
		line("-", "/api/web/flowJobTaskLink/list", "read", "-")
	if err := os.WriteFile(filepath.Join(logRoot, "logs", "netlog", "network.log"), []byte(body), 0o644); err != nil {
		t.Fatalf("写 network.log 失败：%v", err)
	}

	reportStore := planmysql.NewReportRepository(db)
	reports := service.NewReportCenterService(reportStore, service.NewPlanService(planmysql.NewPlanRepository(db)),
		planmysql.NewExecutionPathRepository(db), logRoot, func() time.Time { return now })

	pathReport, err := reports.PathReport(ctx, uint64(planID), uint64(pathID))
	if err != nil {
		t.Fatalf("PathReport 失败：%v", err)
	}
	if len(pathReport.Runs) != 1 {
		t.Fatalf("应有 1 次运行实例，实际 %d", len(pathReport.Runs))
	}
	run := pathReport.Runs[0]
	// 接口调用按路径运行全量计：4 行全部计入（旧口径只算能归属到步骤尝试的行，会得 2）。
	if run.InterfaceCalls != 4 {
		t.Fatalf("接口调用应为 4 次（含 step_id=- 两行），实际 %d", run.InterfaceCalls)
	}
	// 耗时分解：接口 35ms（5+10+20，缺 duration_s 的读取不计样本）+ 执行器 170ms，
	// 工具端编排 = 步骤总耗时 220 − 接口 35 − 执行器 170 = 15ms。
	if !run.InterfaceDurationKnown || run.InterfaceDurationMs != 35 {
		t.Fatalf("接口耗时应为 35ms/已知，实际 %v/%v", run.InterfaceDurationMs, run.InterfaceDurationKnown)
	}
	if run.InterfaceWriteDurationMs != 30 {
		t.Fatalf("写请求耗时合计应为 30ms，实际 %v", run.InterfaceWriteDurationMs)
	}
	if !run.OrchestrationKnown || run.OrchestrationDurationMs != 15 {
		t.Fatalf("工具端编排耗时应为 15ms/已知，实际 %v/%v", run.OrchestrationDurationMs, run.OrchestrationKnown)
	}
	// 计划级指标同样按全量口径聚合，且带统计时刻。
	planReport, err := reports.PlanReport(ctx, uint64(planID))
	if err != nil {
		t.Fatalf("PlanReport 失败：%v", err)
	}
	if planReport.Metrics.InterfaceCalls != 4 || planReport.Metrics.InterfaceReadCalls != 2 ||
		planReport.Metrics.InterfaceWriteCalls != 2 {
		t.Fatalf("计划级接口统计不符：%+v", planReport.Metrics)
	}
	if planReport.Metrics.InterfaceLatencyMs.SampleCount != 3 {
		t.Fatalf("接口耗时样本应为 3 条（缺 duration_s 的那行不计），实际 %d", planReport.Metrics.InterfaceLatencyMs.SampleCount)
	}
	if !planReport.GeneratedAt.Equal(now) {
		t.Fatalf("计划详情统计时刻应为 %v，实际 %v", now, planReport.GeneratedAt)
	}
}
