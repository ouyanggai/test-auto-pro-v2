package executor_test

import (
	"context"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/engine/verdict"
	"test-auto-pro-v2/internal/model"
)

// fixedRunConfig 给出确定的重试预算：预算走配置，不在用例里散落魔法数。
func fixedRunConfig() config.RunConfig {
	return config.RunConfig{
		LeaseDuration:          time.Minute,
		ReadOnlyRetryAttempts:  3,
		ReadOnlyRetryBaseDelay: time.Millisecond,
		ReadOnlyRetryMaxDelay:  2 * time.Millisecond,
		StepProgressStaleAfter: time.Minute,
		StatusPollInterval:     time.Millisecond,
	}
}

// fakeRunState 记录状态机推进调用，供用例断言路径运行的状态路径。
type fakeRunState struct {
	claims        int
	markVerifying int
	backToRunning int
	released      int
	finishedTo    []model.PathRunStatus
	finishClasses []model.FailureClass
}

func (f *fakeRunState) SetMainInstanceRef(context.Context, uint64, string) error { return nil }

func (f *fakeRunState) ClaimExecution(context.Context, uint64) (uint64, error) {
	f.claims++
	return 7, nil
}

func (f *fakeRunState) RenewLease(context.Context, uint64, uint64) error { return nil }

func (f *fakeRunState) ReleaseExecution(context.Context, uint64, uint64) error {
	f.released++
	return nil
}

func (f *fakeRunState) MarkVerifying(context.Context, uint64) error {
	f.markVerifying++
	return nil
}

func (f *fakeRunState) BackToRunning(context.Context, uint64) error {
	f.backToRunning++
	return nil
}

func (f *fakeRunState) Finish(_ context.Context, _ uint64, to model.PathRunStatus, _ *model.RunResult, failureClass *model.FailureClass, _ string) (model.PathRun, error) {
	f.finishedTo = append(f.finishedTo, to)
	if failureClass != nil {
		f.finishClasses = append(f.finishClasses, *failureClass)
	}
	return model.PathRun{ID: 1, Status: to}, nil
}

// fakeFacts 记录落账的步骤与尝试事实。
type fakeFacts struct {
	steps    []model.RunStep
	attempts []model.RunStepAttempt
}

func (f *fakeFacts) RecordStepAttempt(_ context.Context, stepRecord model.RunStep, attempt model.RunStepAttempt, _ time.Time) (uint64, error) {
	f.steps = append(f.steps, stepRecord)
	f.attempts = append(f.attempts, attempt)
	return uint64(len(f.steps)), nil
}

// fakeSessions 固定返回计划账号会话。
type fakeSessions struct {
	calls int
}

func (f *fakeSessions) Current(_ context.Context, account string) (target.Session, error) {
	f.calls++
	return target.Session{Summary: target.AccountSummary{Account: account, DisplayName: "测试账号"}}, nil
}

// Refresh 与 Current 同形：假件不区分缓存语义。
func (f *fakeSessions) Refresh(_ context.Context, account string) (target.Session, error) {
	f.calls++
	return target.Session{Summary: target.AccountSummary{Account: account, DisplayName: "测试账号"}}, nil
}

// fakeTargetView 是假件里一次目标事实读取的预设视图。
type fakeTargetView struct {
	Found        bool
	Status       string
	CurrentNodes []string
	DueNodes     []string
}

// fakeTarget 是目标能力的假件：读写调用计数与预设事实都可配置；
// afterSubmit 用于模拟发起成功后目标事实已经前进。
type fakeTarget struct {
	instance         fakeTargetView
	afterSubmit      *fakeTargetView
	afterAudit       *fakeTargetView
	submitted        bool
	audited          bool
	dueTaskID        string
	submitResult     *target.SubmitFlowInstanceResult
	submitErr        error
	auditResult      *target.AuditCurrentTaskResult
	auditErr         error
	submitCalls      int
	auditCalls       int
	findDueTaskCalls int
}

func (f *fakeTarget) FindSubmittedFlow(context.Context, target.Session, string) (string, []string, string, []string, bool, error) {
	view := f.currentView()
	return "flow-proxy-1", view.CurrentNodes, view.Status, nil, view.Found, nil
}

func (f *fakeTarget) FindDueFlow(context.Context, target.Session, string) (string, []string, []string, bool, error) {
	view := f.currentView()
	return "flow-proxy-1", view.DueNodes, nil, view.Found, nil
}

// currentView 返回当前应呈现的目标事实视图：按已发生的写动作切换到对应阶段。
func (f *fakeTarget) currentView() fakeTargetView {
	if f.audited && f.afterAudit != nil {
		return *f.afterAudit
	}
	if f.submitted && f.afterSubmit != nil {
		return *f.afterSubmit
	}
	return f.instance
}

func (f *fakeTarget) FindDueTaskID(context.Context, target.Session, string, string) (string, error) {
	f.findDueTaskCalls++
	return f.dueTaskID, nil
}

func (f *fakeTarget) SubmitFlowInstance(context.Context, target.Session, target.SubmitFlowInstanceRequest) (*target.SubmitFlowInstanceResult, target.WriteResponse, string, error) {
	f.submitCalls++
	f.submitted = true
	if f.submitErr != nil {
		return nil, target.WriteResponse{}, "trace-fail", f.submitErr
	}
	return f.submitResult, target.WriteResponse{StatusCode: 200, IsSuccess: true, IsSuccessPresent: true}, "trace-submit", nil
}

func (f *fakeTarget) ExecuteActionWrite(_ context.Context, _ target.Session, request target.ActionWriteRequest) (target.WriteResponse, string, error) {
	return target.WriteResponse{StatusCode: 200, IsSuccess: true, IsSuccessPresent: true}, "trace-action", nil
}

func (f *fakeTarget) AuditCurrentTask(context.Context, target.Session, target.AuditCurrentTaskRequest) (*target.AuditCurrentTaskResult, target.WriteResponse, string, error) {
	f.auditCalls++
	f.audited = true
	if f.auditErr != nil {
		return nil, target.WriteResponse{}, "trace-fail", f.auditErr
	}
	return f.auditResult, target.WriteResponse{StatusCode: 200, IsSuccess: true, IsSuccessPresent: true}, "trace-audit", nil
}

// newRunContext 构造一条「新发起」单步场景：发起后接同意。
func newRunContext(steps []model.CompiledActionStep) step.RunContext {
	return step.RunContext{
		Run:         model.Run{ID: 1, PlanID: 1, RunNo: 1},
		PathRun:     model.PathRun{ID: 11, RunID: 1, Status: model.PathRunStatusRunning},
		PathName:    "路径 1",
		PlanAccount: "oyg-test",
		FlowProxyID: "flow-proxy-1",
		Source:      "new",
		Nodes: map[string]step.NodeInfo{
			"node-start": {Name: "发起人", Type: "start"},
			"node-audit": {Name: "部门审批", Type: "审批"},
		},
		Steps:             steps,
		EffectiveFormData: []byte(`{"amount":"12.30"}`),
	}
}

// submitStep 与 approveStep 是两个最小编译步骤。
func submitStep() model.CompiledActionStep {
	return model.CompiledActionStep{Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionSubmit, Scope: model.ActionScopeInitiator, NodeKey: "node-start"}
}

func approveStep() model.CompiledActionStep {
	return model.CompiledActionStep{Sequence: 2, Source: model.ActionStepSourceUser, Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: "node-audit"}
}

// TestF016RereadClassification 锁定事实重读四值的判定口径。
func TestF016RereadClassification(t *testing.T) {
	cases := []struct {
		name    string
		action  string
		nodeKey string
		before  step.InstanceFacts
		after   step.InstanceFacts
		want    verdict.Reread
	}{
		{"发起后实例已运行", string(model.ActionSubmit), "", step.InstanceFacts{}, step.InstanceFacts{Found: true, Status: "run"}, verdict.RereadAdvanced},
		{"发起后实例仍不存在", string(model.ActionSubmit), "", step.InstanceFacts{}, step.InstanceFacts{}, verdict.RereadUnchanged},
		{"发起后落成草稿", string(model.ActionSubmit), "", step.InstanceFacts{}, step.InstanceFacts{Found: true, Status: "draft"}, verdict.RereadContradictory},
		{"重读失败", string(model.ActionApprove), "node-audit", step.InstanceFacts{DueNodes: []string{"node-audit"}}, step.InstanceFacts{ReadError: "目标抖动"}, verdict.RereadUnreadable},
		{"审批后待办仍在", string(model.ActionApprove), "node-audit", step.InstanceFacts{DueNodes: []string{"node-audit"}}, step.InstanceFacts{Found: true, Status: "run", DueNodes: []string{"node-audit"}}, verdict.RereadUnchanged},
		{"审批后待办消失", string(model.ActionApprove), "node-audit", step.InstanceFacts{DueNodes: []string{"node-audit"}}, step.InstanceFacts{Found: true, Status: "run", DueNodes: nil}, verdict.RereadAdvanced},
		{"审批后实例被撤回", string(model.ActionApprove), "node-audit", step.InstanceFacts{DueNodes: []string{"node-audit"}}, step.InstanceFacts{Found: true, Status: "withdraw", DueNodes: nil}, verdict.RereadContradictory},
	}
	for _, item := range cases {
		if got := step.ClassifyReread(item.action, item.nodeKey, item.before, item.after); got != item.want {
			t.Fatalf("%s：重读结论应为 %s，实际 %s", item.name, item.want, got)
		}
	}
}

// TestF016SubmitHappyPath 验证发起步的七阶段闭环：一次写请求、状态推进到下一步、事实落账。
func TestF016SubmitHappyPath(t *testing.T) {
	fakeTarget := &fakeTarget{submitResult: &target.SubmitFlowInstanceResult{InstanceID: "instance-9", Status: "run"}, afterSubmit: &fakeTargetView{Found: true, Status: "run"}}
	sessions := &fakeSessions{}
	runState := &fakeRunState{}
	facts := &fakeFacts{}
	executor := step.NewExecutor(fakeTarget, sessions, runState, facts, fixedRunConfig(), nil)
	runCtx := newRunContext([]model.CompiledActionStep{submitStep(), approveStep()})

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil {
		t.Fatalf("预览构建失败：finished=%v err=%v", finished, err)
	}
	if !preview.GateAllowed {
		t.Fatalf("新发起的门禁应通过：%s", preview.BlockReason)
	}
	if preview.Endpoint != "/web/flowInstanceApi/submit" {
		t.Fatalf("端点应为发起白名单端点，实际 %s", preview.Endpoint)
	}
	if fakeTarget.submitCalls != 0 {
		t.Fatalf("预览阶段绝不允许发出写请求，实际 %d 次", fakeTarget.submitCalls)
	}

	outcome, _, err := executor.RunApprovedStep(context.Background(), step.ApprovedStep{RunCtx: runCtx, Preview: preview, NextIndex: 0})
	if err != nil {
		t.Fatalf("放行执行失败：%v", err)
	}
	if outcome.Verdict != string(verdict.OutcomeSucceeded) || outcome.MainInstanceRef != "instance-9" {
		t.Fatalf("发起步应确定成功并回填实例引用：%+v", outcome)
	}
	if fakeTarget.submitCalls != 1 {
		t.Fatalf("一次尝试最多一次写请求，实际 %d 次", fakeTarget.submitCalls)
	}
	if runState.markVerifying != 1 || runState.backToRunning != 1 || runState.released != 1 {
		t.Fatalf("状态推进应为核验中->运行中并释放租约：%+v", runState)
	}
	if len(facts.attempts) != 1 || facts.attempts[0].Verdict != string(verdict.OutcomeSucceeded) {
		t.Fatalf("尝试事实应落账且结论为确定成功：%+v", facts.attempts)
	}
	if facts.attempts[0].LogPath != "" || facts.attempts[0].TraceID == "" {
		t.Fatalf("尝试事实应携带日志引用与 trace_id：%+v", facts.attempts[0])
	}
}

// TestF016AuditUncertainStopsInAwaitingReconciliation 验证写结果不确定即停在待对账：
// 审批返回成功声明但待办仍在（明确未变），结论必须是不确定，路径停止且无第二次写请求。
func TestF016AuditUncertainStopsInAwaitingReconciliation(t *testing.T) {
	fakeTarget := &fakeTarget{
		instance:    fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-audit"}, DueNodes: []string{"node-audit"}},
		dueTaskID:   "task-1",
		auditResult: &target.AuditCurrentTaskResult{InstanceID: "instance-9", Status: "run"},
	}
	sessions := &fakeSessions{}
	runState := &fakeRunState{}
	facts := &fakeFacts{}
	executor := step.NewExecutor(fakeTarget, sessions, runState, facts, fixedRunConfig(), nil)
	runCtx := newRunContext([]model.CompiledActionStep{approveStep()})
	runCtx.PathRun.MainInstanceRef = "instance-9"

	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || preview == nil || !preview.GateAllowed {
		t.Fatalf("持有待办的审批步门禁应通过：err=%v block=%s", err, previewBlock(preview))
	}
	outcome, _, err := executor.RunApprovedStep(context.Background(), step.ApprovedStep{RunCtx: runCtx, Preview: preview, NextIndex: 0})
	if err != nil {
		t.Fatalf("放行执行失败：%v", err)
	}
	if outcome.Verdict != string(verdict.OutcomeUncertain) {
		t.Fatalf("成功声明+待办未变必须判不确定，实际 %s", outcome.Verdict)
	}
	if fakeTarget.auditCalls != 1 {
		t.Fatalf("不确定后绝不允许重发，实际 %d 次", fakeTarget.auditCalls)
	}
	if len(runState.finishedTo) != 1 || runState.finishedTo[0] != model.PathRunStatusAwaitingReconciliation {
		t.Fatalf("路径运行应进入待对账：%+v", runState.finishedTo)
	}
	if len(runState.finishClasses) != 1 || runState.finishClasses[0] != model.FailureClassWriteUncertain {
		t.Fatalf("失败分类应为写结果不确定：%+v", runState.finishClasses)
	}
}

// TestF016GateDenialBlocksApproval 验证门禁不通过绝不放行：
// 实例事实读不到待办时，审批步被门禁阻塞，放行请求把路径置为失败而不是带病发送写请求。
func TestF016GateDenialBlocksApproval(t *testing.T) {
	fakeTarget := &fakeTarget{instance: fakeTargetView{Found: true, Status: "run", DueNodes: nil}}
	executor := step.NewExecutor(fakeTarget, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)
	runCtx := newRunContext([]model.CompiledActionStep{approveStep()})
	runCtx.PathRun.MainInstanceRef = "instance-9"

	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || preview == nil {
		t.Fatalf("预览构建失败：%v", err)
	}
	if preview.GateAllowed || preview.BlockReason == "" {
		t.Fatalf("无待办的审批步必须被门禁阻塞：%+v", preview)
	}
	runState := &fakeRunState{}
	executor2 := step.NewExecutor(fakeTarget, &fakeSessions{}, runState, &fakeFacts{}, fixedRunConfig(), nil)
	if _, _, err := executor2.RunApprovedStep(context.Background(), step.ApprovedStep{RunCtx: runCtx, Preview: preview, NextIndex: 0}); err != nil {
		t.Fatalf("被阻塞步骤的放行应优雅拒绝：%v", err)
	}
	if fakeTarget.auditCalls != 0 {
		t.Fatalf("门禁不通过绝不允许发出写请求，实际 %d 次", fakeTarget.auditCalls)
	}
	if len(runState.finishedTo) != 1 || runState.finishedTo[0] != model.PathRunStatusFailed {
		t.Fatalf("路径运行应置为失败：%+v", runState.finishedTo)
	}
}

// TestF016RetryBudgetOnReadOnlyPhases 验证只读阶段的有界重试：可重试错误按预算退避，
// 预算耗尽如实失败；submit 阶段没有重试机制可依赖。
func TestF016RetryBudgetOnReadOnlyPhases(t *testing.T) {
	attempts := 0
	sleeps := []time.Duration{}
	policy := step.RetryPolicy{Attempts: 3, BaseDelay: 10 * time.Millisecond, MaxDelay: 40 * time.Millisecond, Sleep: func(d time.Duration) {
		sleeps = append(sleeps, d)
	}}
	result, err := step.RunWithRetry(context.Background(), policy, "测试操作", func() (int, error) {
		attempts++
		if attempts < 3 {
			return 0, &target.Error{Kind: target.ErrorTimeout}
		}
		return attempts, nil
	}, nil)
	if err != nil || result != 3 {
		t.Fatalf("第 3 次应成功：result=%d err=%v", result, err)
	}
	if len(sleeps) != 2 || sleeps[0] != 10*time.Millisecond || sleeps[1] != 20*time.Millisecond {
		t.Fatalf("指数退避应为 10ms、20ms：%v", sleeps)
	}

	attempts = 0
	_, err = step.RunWithRetry(context.Background(), policy, "测试操作", func() (int, error) {
		attempts++
		return 0, &target.Error{Kind: target.ErrorLoginRejected}
	}, nil)
	if attempts != 1 || err == nil {
		t.Fatalf("登录被拒是确定性失败，不应重试且必须返回错误：attempts=%d err=%v", attempts, err)
	}

	// 预算耗尽：3 次全部失败后如实返回最后一次错误。
	budget := step.RetryPolicy{Attempts: 3, BaseDelay: time.Millisecond, Sleep: func(time.Duration) {}}
	exhausted := 0
	_, err = step.RunWithRetry(context.Background(), budget, "测试操作", func() (int, error) {
		exhausted++
		return 0, &target.Error{Kind: target.ErrorUnavailable}
	}, nil)
	if exhausted != 3 || err == nil {
		t.Fatalf("预算耗尽必须如实失败：exhausted=%d err=%v", exhausted, err)
	}
}

// previewBlock 安全取出预览的阻塞原因（测试辅助）。
func previewBlock(preview *step.StepPreview) string {
	if preview == nil {
		return "<nil>"
	}
	return preview.BlockReason
}
