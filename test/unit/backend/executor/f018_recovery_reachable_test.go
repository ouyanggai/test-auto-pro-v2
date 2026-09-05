package executor_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/control"
	"test-auto-pro-v2/internal/engine/reconcile"
	"test-auto-pro-v2/internal/engine/run"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
)

// f018Session 是 F-018 恢复链路用例的公共装配：真实 MySQL + 真实状态机 + 真实仓储，
// 只有目标读写是假件。场景固定为「发起 → 同意」两步：
// 第一步发起确定成功（拿到主实例引用，这是对账五维能读到东西的前提），
// 第二步同意返回成功声明但本步待办仍在（明确未变）→ 三值判定不确定 → 路径运行进入待对账。
//
// 为什么必须走两步而不是直接给一个同意步骤：主实例引用只在发起成功后才落库并回填内存现场，
// 恢复场景（对已有实例直接动手）的接线属 F-019，当前实现拿不到它。
func f018Session(t *testing.T) (*control.Service, *planmysql.RunRepository, uint64, *fakeTarget) {
	harness := f018Harness(t)
	return harness.controller, harness.store, harness.pathRunID, harness.fake
}

// f018Fixture 是 F-018 恢复链路用例的完整装配句柄：
// 除控制服务外还带上仓储与执行器，便于用例另建一个控制服务模拟"服务重启"。
type f018Fixture struct {
	controller *control.Service
	store      *planmysql.RunRepository
	runService *run.Service
	executor   *step.Executor
	// database 只给需要直接构造异常数据的用例用（如把写前基准列清空模拟旧版本数据）。
	database  *planmysql.Database
	pathRunID uint64
	fake      *fakeTarget
}

// f018Harness 把一条路径运行推进到「待对账」并返回完整装配句柄。
func f018Harness(t *testing.T) f018Fixture {
	t.Helper()
	database := newF016ControlDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	store := planmysql.NewRunRepository(database.DB)
	runService := run.NewService(store, "worker-f018", time.Minute, time.Now)

	fake := &fakeTarget{
		// 发起之前实例不可见；发起成功后实例运行中且本步节点上有待办。
		instance:    fakeTargetView{},
		afterSubmit: &fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-audit"}, DueNodes: []string{"node-audit"}},
		dueTaskID:   "task-1",
		submitResult: &target.SubmitFlowInstanceResult{
			InstanceID: "instance-9", Status: "run", CurrentNodeProxyIDs: []string{"node-audit"},
		},
		auditResult: &target.AuditCurrentTaskResult{InstanceID: "instance-9", Status: "run"},
		// 第一次同意不改变任何事实（成功声明但待办仍在）；只有第 2 次（重放那一次）才真的推进。
		afterAudit:           &fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-end"}, DueNodes: nil},
		auditAdvanceFromCall: 2,
		// 五维强证据：已办记录与动作痕迹都读到了，且都没有本次动作的痕迹。
		doneRecordFound: false,
		auditTraceFound: false,
		auditTraceTotal: 1,
	}
	executor := step.NewExecutor(fake, &fakeSessions{}, runService, store, fixedRunConfig(), time.Now)
	controller := control.NewService(runService, executor, store, time.Now)

	runCtx := newRunContext([]model.CompiledActionStep{submitStep(), approveStep()})
	started, err := controller.Start(ctx, runCtx)
	if err != nil {
		t.Fatalf("启动失败：%v", err)
	}
	pathRunID := started.PathRun.ID

	// 第一步：发起，确定成功。
	approveCurrentStep(t, ctx, controller, pathRunID)
	// 第二步：同意，成功声明 + 待办仍在 = 不确定。
	approveCurrentStep(t, ctx, controller, pathRunID)

	pathRun, err := store.GetPathRun(ctx, pathRunID)
	if err != nil {
		t.Fatalf("读取路径运行失败：%v", err)
	}
	if pathRun.Status != model.PathRunStatusAwaitingReconciliation {
		t.Fatalf("前提不成立：路径运行应进入待对账，实际 %s", model.PathRunStatusName(pathRun.Status))
	}
	if fake.auditCalls != 1 {
		t.Fatalf("一次尝试只允许一次写请求，实际同意调用 %d 次", fake.auditCalls)
	}
	return f018Fixture{
		controller: controller, store: store, runService: runService,
		executor: executor, database: database, pathRunID: pathRunID, fake: fake,
	}
}

// approveCurrentStep 放行当前等待放行的一步；取不到现场即失败，不静默跳过。
func approveCurrentStep(t *testing.T, ctx context.Context, controller *control.Service, pathRunID uint64) {
	t.Helper()
	preview := controller.CurrentPreview(pathRunID)
	if preview == nil {
		t.Fatal("控制现场丢失，没有等待放行的步骤")
	}
	if preview.BlockReason != "" {
		t.Fatalf("第 %d 步被阻塞，无法放行：%s", preview.StepNo, preview.BlockReason)
	}
	view := controller.View(pathRunID)
	if _, err := controller.ApproveWithCommand(ctx, pathRunID, model.CommandStep, preview.StepNo, view.Version); err != nil {
		t.Fatalf("放行第 %d 步失败：%v", preview.StepNo, err)
	}
}

// TestF018NotEffectiveLeadsToReplay 锁定 F-018 的核心承诺链路：
// 写结果不确定 → 路径运行停在待对账且控制现场保留 → 自动只读对账在五维齐备且全部未变时判「未生效」
// → 唯一动作是重放 → 重放是 attempt 递增的新尝试、重走七阶段、只发一次写请求。
//
// 这条链路此前整段不可达：不确定落账后内存现场被清空（对账报"没有等待放行的步骤"），
// 待对账又被当成不可离开的终态（三个恢复动作一律被拒），租约也不接受待对账。
func TestF018NotEffectiveLeadsToReplay(t *testing.T) {
	controller, store, pathRunID, fake := f018Session(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// 现场必须还在：这是自动对账能跑起来的前提。
	if controller.CurrentPreview(pathRunID) == nil {
		t.Fatal("待对账后控制现场被清空，只读对账无法进行")
	}

	view, err := controller.ReconcileNow(ctx, pathRunID)
	if err != nil {
		t.Fatalf("只读对账失败：%v", err)
	}
	t.Logf("对账结论=%s 动作=%s 已用重放=%d/%d 结论=%s", view.Verdict, view.Action, view.ReplaysUsed, view.ReplaysMax, view.Headline)
	for _, reason := range view.Reasons {
		t.Logf("  依据：%s", reason)
	}
	if view.Verdict != string(reconcile.VerdictNotEffective) {
		t.Fatalf("五维齐备且全部未变时应判未生效，实际 %s", view.Verdict)
	}
	if view.Action != string(reconcile.ActionReplay) {
		t.Fatalf("未生效的唯一合法动作是重放，实际 %s", view.Action)
	}

	// 结论与动作必须一一对应：拿未生效去做确认前进要被拒。
	if err := controller.RecoveryAction(ctx, pathRunID, reconcile.ActionAdvance, model.RunManualConclusion{}); err == nil {
		t.Fatal("未生效结论下不允许执行确认前进")
	}

	// 重放这一步：第 2 次同意（重放那一次）让待办消失，本步确定成功。
	if err := controller.RecoveryAction(ctx, pathRunID, reconcile.ActionReplay, model.RunManualConclusion{}); err != nil {
		t.Fatalf("对账给出的唯一动作必须可执行，实际被拒绝：%v", err)
	}
	if fake.auditCalls != 2 {
		t.Fatalf("重放应当只多发一次写请求（共 2 次），实际同意调用 %d 次", fake.auditCalls)
	}
	// 重放成功后现场必须真的离开待对账：本场景第 2 步是最后一步，因此路径运行收尾为已完成。
	// 这一条同时锁住"重放后忘记清掉待对账标记"这个卡死缺陷——那会让后续步骤永远等不到放行入口。
	pathRun, err := store.GetPathRun(ctx, pathRunID)
	if err != nil {
		t.Fatalf("读取路径运行失败：%v", err)
	}
	if pathRun.Status != model.PathRunStatusCompleted {
		t.Fatalf("重放确定成功且已是最后一步，路径运行应收尾为已完成，实际 %s", model.PathRunStatusName(pathRun.Status))
	}

	// 尝试事实：同一步两次尝试，第二次标记为重放；第一次尝试行落了对账结论与恢复动作。
	attempts, err := store.ListRunAttempts(ctx, pathRunID)
	if err != nil {
		t.Fatalf("读取尝试事实失败：%v", err)
	}
	var first, replay *model.RunStepAttempt
	for index := range attempts {
		attempt := attempts[index]
		if attempt.AttemptNo == 1 && attempt.Verdict == "uncertain" {
			first = &attempts[index]
		}
		if attempt.AttemptNo == 2 {
			replay = &attempts[index]
		}
	}
	if first == nil {
		t.Fatalf("找不到那次不确定的首次尝试：%+v", attempts)
	}
	if first.ReconcileVerdict != string(reconcile.VerdictNotEffective) || first.RecoveryAction != string(reconcile.ActionReplay) {
		t.Fatalf("首次尝试行的对账三列没有落库：结论=%q 动作=%q", first.ReconcileVerdict, first.RecoveryAction)
	}
	if first.IsReplay {
		t.Fatal("首次尝试不是重放，is_replay 不能为真")
	}
	if replay == nil {
		t.Fatalf("重放没有产生第 2 次尝试：%+v", attempts)
	}
	if !replay.IsReplay {
		t.Fatal("重放尝试的 is_replay 必须为真")
	}
	if replay.TraceID == "" || replay.TraceID == first.TraceID {
		t.Fatalf("重放必须是新的一次写（新 trace）：首次=%q 重放=%q", first.TraceID, replay.TraceID)
	}
	t.Logf("首次尝试 verdict=%s 三列=%s/%s；重放尝试 verdict=%s is_replay=%v", first.Verdict,
		first.ReconcileVerdict, first.RecoveryAction, replay.Verdict, replay.IsReplay)
}

// TestF018ReplayLimitIsEnforced 锁定「默认最多重放一次」真的生效：
// 重放后仍然不确定、证据仍然全部未变时，服务端不再提供重放，唯一动作降级为登记人工核对结论。
// 此前这条上限是空的——对账每次都把"已用重放次数"重置为 0，判断永远不成立。
func TestF018ReplayLimitIsEnforced(t *testing.T) {
	controller, _, pathRunID, fake := f018Session(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	// 让任何一次同意都不推进事实：重放后依然是"未生效"。
	fake.auditAdvanceFromCall = 99

	first, err := controller.ReconcileNow(ctx, pathRunID)
	if err != nil {
		t.Fatalf("只读对账失败：%v", err)
	}
	if first.Action != string(reconcile.ActionReplay) || first.ReplaysUsed != 0 {
		t.Fatalf("首次对账应给出重放且已用次数为 0，实际动作=%s 已用=%d", first.Action, first.ReplaysUsed)
	}
	if err := controller.RecoveryAction(ctx, pathRunID, reconcile.ActionReplay, model.RunManualConclusion{}); err != nil {
		t.Fatalf("第一次重放应当可执行：%v", err)
	}
	if fake.auditCalls != 2 {
		t.Fatalf("第一次重放后写请求应为 2 次，实际 %d 次", fake.auditCalls)
	}

	second, err := controller.ReconcileNow(ctx, pathRunID)
	if err != nil {
		t.Fatalf("重放后只读对账失败：%v", err)
	}
	t.Logf("重放后对账：结论=%s 动作=%s 已用重放=%d/%d 用完=%v", second.Verdict, second.Action,
		second.ReplaysUsed, second.ReplaysMax, second.ReplayExhausted)
	if second.Verdict != string(reconcile.VerdictNotEffective) {
		t.Fatalf("证据没变，结论仍应如实是未生效，实际 %s", second.Verdict)
	}
	if second.ReplaysUsed != 1 || !second.ReplayExhausted {
		t.Fatalf("已用重放次数应为 1 且标记用完，实际 已用=%d 用完=%v", second.ReplaysUsed, second.ReplayExhausted)
	}
	if second.Action != string(reconcile.ActionManualEnd) {
		t.Fatalf("重放用完后唯一动作应降级为登记人工结论，实际 %s", second.Action)
	}
	if err := controller.RecoveryAction(ctx, pathRunID, reconcile.ActionReplay, model.RunManualConclusion{}); err == nil {
		t.Fatal("重放次数用完后不允许再次重放")
	}
	if fake.auditCalls != 2 {
		t.Fatalf("被拒绝的重放绝不允许发出写请求，实际写请求 %d 次", fake.auditCalls)
	}
	if err := controller.RecoveryAction(ctx, pathRunID, reconcile.ActionManualEnd, model.RunManualConclusion{
		InstanceStatus: "run", CurrentNode: "部门审批", Reporter: "自动化用例", Note: "重放用完后的人工登记",
	}); err != nil {
		t.Fatalf("重放用完后必须能登记人工结论：%v", err)
	}
}

// TestF018EffectiveLeadsToAdvanceOnly 锁定「已生效 → 只能确认并前进」：
// 五维全部变化时结论为已生效，唯一动作是确认前进，且执行后路径运行回到运行中、游标只推进一次。
func TestF018EffectiveLeadsToAdvanceOnly(t *testing.T) {
	controller, store, pathRunID, fake := f018Session(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	// 事实改为「写其实已经生效」：实例已推进、本步待办消失、已办与动作痕迹都出现了。
	fake.afterAudit = &fakeTargetView{Found: true, Status: "end", CurrentNodes: []string{"node-end"}, DueNodes: nil}
	fake.auditAdvanceFromCall = 1
	fake.doneRecordFound = true
	fake.auditTraceFound = true

	view, err := controller.ReconcileNow(ctx, pathRunID)
	if err != nil {
		t.Fatalf("只读对账失败：%v", err)
	}
	t.Logf("对账结论=%s 动作=%s 结论=%s", view.Verdict, view.Action, view.Headline)
	if view.Verdict != string(reconcile.VerdictEffective) || view.Action != string(reconcile.ActionAdvance) {
		t.Fatalf("五维全部变化应判已生效且只给确认前进，实际 结论=%s 动作=%s", view.Verdict, view.Action)
	}
	if err := controller.RecoveryAction(ctx, pathRunID, reconcile.ActionReplay, model.RunManualConclusion{}); err == nil {
		t.Fatal("已生效结论下不允许重放")
	}
	if err := controller.RecoveryAction(ctx, pathRunID, reconcile.ActionAdvance, model.RunManualConclusion{}); err != nil {
		t.Fatalf("确认前进必须可执行：%v", err)
	}
	if fake.auditCalls != 1 {
		t.Fatalf("确认前进不得发出任何写请求，实际同意调用 %d 次", fake.auditCalls)
	}
	pathRun, err := store.GetPathRun(ctx, pathRunID)
	if err != nil {
		t.Fatalf("读取路径运行失败：%v", err)
	}
	// 场景只有两步，第二步确认前进后场景走完，路径运行收尾为已完成。
	if pathRun.Status != model.PathRunStatusCompleted {
		t.Fatalf("两步场景确认前进后应收尾为已完成，实际 %s", model.PathRunStatusName(pathRun.Status))
	}
}

// TestF018MissingDimensionDegradesToManualEnd 锁定强证据规则的降级方向：
// 五维里只要有一维读不到，结论必须降级为仍无法判定，唯一动作是登记人工核对结论，绝不给重放。
// 「未生效」是唯一会导致再写一次的结论，它的门槛不允许被任何读取失败绕过。
func TestF018MissingDimensionDegradesToManualEnd(t *testing.T) {
	controller, store, pathRunID, fake := f018Session(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	// 动作痕迹这一维读不到（目标抖动），其余四维仍然明确未变。
	fake.auditTraceErr = target.NewError(target.ErrorTimeout, nil)

	view, err := controller.ReconcileNow(ctx, pathRunID)
	if err != nil {
		t.Fatalf("只读对账失败：%v", err)
	}
	t.Logf("对账结论=%s 动作=%s 结论=%s", view.Verdict, view.Action, view.Headline)
	if view.Verdict != string(reconcile.VerdictIndeterminate) {
		t.Fatalf("有维度读不到时必须降级为仍无法判定，实际 %s", view.Verdict)
	}
	if view.Action != string(reconcile.ActionManualEnd) {
		t.Fatalf("仍无法判定的唯一动作是登记人工结论，实际 %s", view.Action)
	}
	if err := controller.RecoveryAction(ctx, pathRunID, reconcile.ActionReplay, model.RunManualConclusion{}); err == nil {
		t.Fatal("证据不完整时不允许重放")
	}
	if fake.auditCalls != 1 {
		t.Fatalf("被拒绝的重放绝不允许发出写请求，实际写请求 %d 次", fake.auditCalls)
	}
	if err := controller.RecoveryAction(ctx, pathRunID, reconcile.ActionManualEnd, model.RunManualConclusion{
		InstanceStatus: "run", CurrentNode: "部门审批", Reporter: "自动化用例", Note: "目标读不到动作痕迹，人工核对后登记",
	}); err != nil {
		t.Fatalf("登记人工结论必须可执行：%v", err)
	}
	// 人工结论是 append-only 事实，路径运行留在待对账作为最终归宿。
	pathRun, err := store.GetPathRun(ctx, pathRunID)
	if err != nil {
		t.Fatalf("读取路径运行失败：%v", err)
	}
	if pathRun.Status != model.PathRunStatusAwaitingReconciliation {
		t.Fatalf("登记人工结论后路径运行应留在待对账，实际 %s", model.PathRunStatusName(pathRun.Status))
	}
	// 运行聚合必须收尾：待对账时故意保持"运行中"是为了把出路留给对账，
	// 人工结论一登记就再没有后续动作，运行还挂在运行中会让运行列表长期显示陈旧状态。
	runAggregate, err := store.GetRun(ctx, pathRun.RunID)
	if err != nil {
		t.Fatalf("读取运行聚合失败：%v", err)
	}
	if runAggregate.Status != model.RunStatusStopped {
		t.Fatalf("登记人工结论后运行聚合应收尾为已停止，实际 %s", model.RunStatusName(runAggregate.Status))
	}
	// 现场已作废：登记之后不再有任何恢复入口。
	if err := controller.RecoveryAction(ctx, pathRunID, reconcile.ActionManualEnd, model.RunManualConclusion{
		InstanceStatus: "run", CurrentNode: "部门审批", Reporter: "自动化用例",
	}); err == nil {
		t.Fatal("人工结论登记后不应再接受恢复动作")
	}
}

// TestF018AwaitingReconciliationSurvivesRestart 锁定根治项：服务重启后，
// 停在待对账的路径运行仍然能对账、仍然能执行对账给出的唯一动作。
//
// 此前对账现场（本步预览、步骤游标、写之前的基准事实、已用重放次数）只在进程内存里，
// 重启即丢失，这条路径运行就永远没有出路了——界面上待对账区域直接消失。
// 现在这些事实都在库里：本用例用「另建一个控制服务」表达重启，全程不碰第一个服务的内存。
func TestF018AwaitingReconciliationSurvivesRestart(t *testing.T) {
	harness := f018Harness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// 写之前的基准必须已经随尝试行落库，否则重启后无从比较。
	attempts, err := harness.store.ListRunAttempts(ctx, harness.pathRunID)
	if err != nil {
		t.Fatalf("读取尝试事实失败：%v", err)
	}
	uncertain := model.RunStepAttempt{}
	for _, attempt := range attempts {
		if attempt.Verdict == "uncertain" {
			uncertain = attempt
		}
	}
	if uncertain.ID == 0 {
		t.Fatalf("前提不成立：没有不确定的尝试行：%+v", attempts)
	}
	if uncertain.BeforeFacts == "" {
		t.Fatal("写之前的目标事实必须随尝试行落库，否则重启后对账没有基准")
	}
	t.Logf("落库的写前基准：%s", uncertain.BeforeFacts)

	// 模拟重启：另建一个控制服务，内存现场是空的。
	restarted := control.NewService(harness.runService, harness.executor, harness.store, time.Now)
	if restarted.HasSession(harness.pathRunID) {
		t.Fatal("新建的控制服务不应该已经有现场")
	}
	if _, err := restarted.ReconcileNow(ctx, harness.pathRunID); err == nil {
		t.Fatal("未重建现场时对账应当失败，不能凭空对账")
	}

	// 按运行事实重建现场：调用方只提供计划与路径配置装配出的执行上下文。
	runCtx := newRunContext([]model.CompiledActionStep{submitStep(), approveStep()})
	runCtx.PathRun.ID = harness.pathRunID
	if err := restarted.Rehydrate(ctx, runCtx); err != nil {
		t.Fatalf("按运行事实重建对账现场失败：%v", err)
	}
	if !restarted.HasSession(harness.pathRunID) {
		t.Fatal("重建后必须有现场")
	}
	preview := restarted.CurrentPreview(harness.pathRunID)
	if preview == nil || preview.StepNo != approveStep().Sequence {
		t.Fatalf("重建出的游标必须指向那个不确定的步骤，实际 %+v", preview)
	}

	// 五维证据必须与重启前一致：基准是从库里还原的，不是猜的。
	view, err := restarted.ReconcileNow(ctx, harness.pathRunID)
	if err != nil {
		t.Fatalf("重建后只读对账失败：%v", err)
	}
	t.Logf("重建后对账：结论=%s 动作=%s", view.Verdict, view.Action)
	for _, reason := range view.Reasons {
		t.Logf("  依据：%s", reason)
	}
	if view.Verdict != string(reconcile.VerdictNotEffective) || view.Action != string(reconcile.ActionReplay) {
		t.Fatalf("重启前五维齐备且全部未变，重启后必须得到同样的结论：结论=%s 动作=%s", view.Verdict, view.Action)
	}

	// 对账给出的唯一动作必须真的可执行：重放为新尝试，并且只多发一次写请求。
	if err := restarted.RecoveryAction(ctx, harness.pathRunID, reconcile.ActionReplay, model.RunManualConclusion{}); err != nil {
		t.Fatalf("重建后重放必须可执行：%v", err)
	}
	if harness.fake.auditCalls != 2 {
		t.Fatalf("重放应当只多发一次写请求（共 2 次），实际 %d 次", harness.fake.auditCalls)
	}
	after, err := harness.store.ListRunAttempts(ctx, harness.pathRunID)
	if err != nil {
		t.Fatalf("读取尝试事实失败：%v", err)
	}
	replayed := false
	for _, attempt := range after {
		if attempt.AttemptNo == 2 && attempt.IsReplay {
			replayed = true
		}
	}
	if !replayed {
		t.Fatalf("重建后的重放必须落成 attempt=2 且标记重放的新尝试：%+v", after)
	}
}

// TestF018RestartWithoutBaselineDegradesHonestly 锁定基准缺失时的诚实降级：
// 尝试行没有写前基准（早于基准落库版本，或崩溃在落账之前）时，
// 由实例事实派生的三个维度必须按缺失处理，结论只能是仍无法判定——
// 绝不允许把零值基准读成「写之前实例不存在」，那会凭空造出一条与事实相反的依据。
func TestF018RestartWithoutBaselineDegradesHonestly(t *testing.T) {
	harness := f018Harness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// 模拟"这条尝试是旧版本落的"：把基准列清空（只有本用例这样做，产品代码永不更新该列）。
	if _, err := harness.database.DB.ExecContext(ctx,
		"UPDATE run_step_attempts SET before_facts = NULL WHERE path_run_id = ?", harness.pathRunID); err != nil {
		t.Fatalf("构造基准缺失失败：%v", err)
	}

	restarted := control.NewService(harness.runService, harness.executor, harness.store, time.Now)
	runCtx := newRunContext([]model.CompiledActionStep{submitStep(), approveStep()})
	runCtx.PathRun.ID = harness.pathRunID
	if err := restarted.Rehydrate(ctx, runCtx); err != nil {
		t.Fatalf("重建对账现场失败：%v", err)
	}
	view, err := restarted.ReconcileNow(ctx, harness.pathRunID)
	if err != nil {
		t.Fatalf("只读对账失败：%v", err)
	}
	t.Logf("基准缺失时对账：结论=%s 动作=%s", view.Verdict, view.Action)
	for _, reason := range view.Reasons {
		t.Logf("  依据：%s", reason)
	}
	if view.Verdict != string(reconcile.VerdictIndeterminate) || view.Action != string(reconcile.ActionManualEnd) {
		t.Fatalf("基准缺失必须降级为仍无法判定且只给人工登记：结论=%s 动作=%s", view.Verdict, view.Action)
	}
	joined := strings.Join(view.Reasons, "；")
	if !strings.Contains(joined, "写之前的目标事实基准没有落库") {
		t.Fatalf("降级理由必须点明基准缺失，而不是含糊其辞：%v", view.Reasons)
	}
	if strings.Contains(joined, "写之前实例不存在") {
		t.Fatalf("基准缺失时绝不能声称写之前实例不存在：%v", view.Reasons)
	}
	if err := restarted.RecoveryAction(ctx, harness.pathRunID, reconcile.ActionReplay, model.RunManualConclusion{}); err == nil {
		t.Fatal("基准缺失时不允许重放")
	}
	if harness.fake.auditCalls != 1 {
		t.Fatalf("被拒绝的重放绝不允许发出写请求，实际 %d 次", harness.fake.auditCalls)
	}
}

// TestF018AwaitingReconciliationOffersNoApprovalCommand 锁定待对账现场不得出现放行入口。
// 现场在不确定写之后被刻意保留（对账要用它），但保留现场不等于还能放行：
// 界面上出现「执行一步」会与「一个结论只对应一个动作」直接冲突，点下去也必然被状态守卫拒绝，
// 属纲领第 12 节的陈旧文案。重建出来的现场同样如此。
func TestF018AwaitingReconciliationOffersNoApprovalCommand(t *testing.T) {
	harness := f018Harness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	live := harness.controller.View(harness.pathRunID)
	if live == nil {
		t.Fatal("待对账后现场必须保留，否则无法对账")
	}
	if len(live.Commands) != 0 {
		t.Fatalf("待对账现场不得提供任何放行类命令，实际 %v", live.Commands)
	}
	if live.PauseState != control.PauseStateUncertain {
		t.Fatalf("待对账现场的控制状态应为写结果不确定，实际 %s", live.PauseState)
	}
	if live.StopReason == "" {
		t.Fatal("待对账必须给出「为什么停在这里」的中文原因")
	}
	t.Logf("待对账现场：命令=%v 状态=%s 原因=%s", live.Commands, live.PauseState, live.StopReason)

	// 直接尝试放行必须被拒绝，且不产生任何写请求。
	preview := harness.controller.CurrentPreview(harness.pathRunID)
	if preview == nil {
		t.Fatal("待对账现场应当仍能给出本步预览")
	}
	if _, err := harness.controller.ApproveWithCommand(ctx, harness.pathRunID,
		model.CommandStep, preview.StepNo, live.Version); err == nil {
		t.Fatal("待对账的路径运行不允许放行")
	}
	if harness.fake.auditCalls != 1 {
		t.Fatalf("被拒绝的放行绝不允许发出写请求，实际 %d 次", harness.fake.auditCalls)
	}

	// 重建出来的现场同样不给放行入口。
	restarted := control.NewService(harness.runService, harness.executor, harness.store, time.Now)
	runCtx := newRunContext([]model.CompiledActionStep{submitStep(), approveStep()})
	runCtx.PathRun.ID = harness.pathRunID
	if err := restarted.Rehydrate(ctx, runCtx); err != nil {
		t.Fatalf("重建对账现场失败：%v", err)
	}
	rebuilt := restarted.View(harness.pathRunID)
	if rebuilt == nil || len(rebuilt.Commands) != 0 || rebuilt.PauseState != control.PauseStateUncertain {
		t.Fatalf("重建现场不得提供放行类命令：%+v", rebuilt)
	}
}
