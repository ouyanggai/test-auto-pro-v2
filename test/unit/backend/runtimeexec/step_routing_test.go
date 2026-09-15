// T012 七阶段执行器路由契约：
// ① 未配置 runtime → 全部动作保持 Go 写路径；
// ② 配置后仅七个表单动作走 runtime，Go 直连动作与系统节点不受影响；
// ③ 未登记能力的表单动作被阻塞且绝不回退 Go 写（无双写）；
// ④ runtime 已接管的动作对 TargetClient 的写调用为零。
package runtimeexec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/runtimeexec"
)

// recordingTarget 记录全部写调用：runtime 接管后必须保持零写入。
type recordingTarget struct {
	mu       sync.Mutex
	writes   []string
	template bool
	taskID   string
}

func (r *recordingTarget) FindSubmittedFlow(context.Context, target.Session, string) (string, []string, string, []string, bool, error) {
	return "", nil, "", nil, false, nil
}
func (r *recordingTarget) FindDueFlow(context.Context, target.Session, string) (string, []string, []string, bool, error) {
	return "", nil, nil, false, nil
}
func (r *recordingTarget) ReadInstanceCurrentData(context.Context, target.Session, string) (map[string]any, error) {
	return map[string]any{}, nil
}
func (r *recordingTarget) ReadFormTemplateData(context.Context, target.Session, string) (json.RawMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.template = true
	return json.RawMessage(`{"list":[],"config":{}}`), nil
}
func (r *recordingTarget) FindDueTaskID(context.Context, target.Session, string, string) (string, error) {
	return "", nil
}

// FindTaskSnapshot 提供任务快照能力面：返回预置任务号，供审批族动作的 runtime 前置刷新读取。
func (r *recordingTarget) FindTaskSnapshot(context.Context, target.Session, string, string, string) (target.TaskSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return target.TaskSnapshot{JobTaskID: r.taskID, FlowNodeProxyID: "node-proxy-1"}, nil
}
func (r *recordingTarget) SubmitFlowInstance(context.Context, target.Session, target.SubmitFlowInstanceRequest) (*target.SubmitFlowInstanceResult, target.WriteResponse, string, error) {
	r.mu.Lock()
	r.writes = append(r.writes, "submit")
	r.mu.Unlock()
	return nil, target.WriteResponse{}, "", errors.New("runtime 接管后不得调用")
}
func (r *recordingTarget) AuditCurrentTask(context.Context, target.Session, target.AuditCurrentTaskRequest) (*target.AuditCurrentTaskResult, target.WriteResponse, string, error) {
	r.mu.Lock()
	r.writes = append(r.writes, "audit")
	r.mu.Unlock()
	return nil, target.WriteResponse{}, "", errors.New("runtime 接管后不得调用")
}
func (r *recordingTarget) ExecuteActionWrite(context.Context, target.Session, target.ActionWriteRequest) (target.WriteResponse, string, error) {
	r.mu.Lock()
	r.writes = append(r.writes, "action")
	r.mu.Unlock()
	return target.WriteResponse{}, "", errors.New("runtime 接管后不得调用")
}
func (r *recordingTarget) SaveSpecialBusiness(context.Context, target.Session, target.SpecialBusinessSpec, json.RawMessage, string) (target.SpecialBusinessSaveResult, error) {
	return target.SpecialBusinessSaveResult{}, nil
}
func (r *recordingTarget) SaveNoFormFlow(context.Context, target.Session, target.NoFormFlowSpec, json.RawMessage, string) (target.SpecialBusinessSaveResult, error) {
	return target.SpecialBusinessSaveResult{}, nil
}

func (r *recordingTarget) writeCalls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.writes...)
}

// fakeRuntimeRunner 返回预置判定并记录调用；captures 非空时保存每次收到的请求事实。
// err 非空时 RunAction 返回该错误（用于连接断开二分分类的回归）。
type fakeRuntimeRunner struct {
	mu       sync.Mutex
	status   string
	err      error
	calls    int
	requests []runtimeexec.RuntimeRunRequest
}

func (f *fakeRuntimeRunner) RunAction(_ context.Context, request runtimeexec.RuntimeRunRequest) (runtimeexec.ActionRunResult, error) {
	f.mu.Lock()
	f.calls++
	f.requests = append(f.requests, request)
	f.mu.Unlock()
	if f.err != nil {
		return runtimeexec.ActionRunResult{Status: "failed", Message: "fake-error"}, f.err
	}
	return runtimeexec.ActionRunResult{Status: f.status, Message: "fake"}, nil
}

func (f *fakeRuntimeRunner) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeRuntimeRunner) lastRequest() runtimeexec.RuntimeRunRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.requests) == 0 {
		return runtimeexec.RuntimeRunRequest{}
	}
	return f.requests[len(f.requests)-1]
}

// fakeStepSessions 是最小会话提供者：按账号返回预置会话，供 runtime 前置身份与任务刷新读取。
type fakeStepSessions struct {
	byAccount map[string]target.Session
}

func (s *fakeStepSessions) Current(_ context.Context, account string) (target.Session, error) {
	if session, ok := s.byAccount[account]; ok {
		return session, nil
	}
	return target.Session{}, errors.New("账号未登记")
}

func (s *fakeStepSessions) Refresh(_ context.Context, account string) (target.Session, error) {
	return s.Current(context.Background(), account)
}

func newExecutorForRouting(t *testing.T, view *recordingTarget, runner step.RuntimeRunner, registry *runtimeexec.CapabilityRegistry) *step.Executor {
	t.Helper()
	executor := step.NewExecutor(view, &fakeStepSessions{byAccount: map[string]target.Session{
		"plan-acct":  {UserID: "user-plan", Summary: target.AccountSummary{Account: "plan-acct", DisplayName: "发起人"}},
		"actor-acct": {UserID: "user-actor", Summary: target.AccountSummary{Account: "actor-acct", DisplayName: "处理人"}},
	}}, nil, nil, fixedRunConfigForRouting(), nil)
	// registry 非空即配置 runtime 能力面（runner 可为 nil：登记测试不经过路由）；
	// 两者都为空时保持未配置形态，锁定「未配置=全部动作走 Go 写路径」。
	if runner != nil || registry != nil {
		executor.EnableRuntimeExecution(runner, registry, "")
	}
	return executor
}

func fixedRunConfigForRouting() config.RunConfig {
	return config.RunConfig{}
}

func runtimeStep(action model.ActionKey) model.CompiledActionStep {
	return model.CompiledActionStep{Action: action, NodeKey: "node-1"}
}

func runtimeRunCtx() step.RunContext {
	return step.RunContext{
		FlowType:          "employee_leave",
		RenderType:        string(target.FormRenderTypeFormMaking),
		FormProxyID:       "form-proxy-1",
		PlanAccount:       "plan-acct",
		EffectiveFormData: []byte(`{"reason":"x"}`),
		PathRun:           model.PathRun{ID: 1},
		Nodes: map[string]step.NodeInfo{
			"node-1": {TargetNodeID: "target-node-1", Name: "审批节点"},
		},
	}
}

// TestRoutingWithoutRunnerKeepsGoPath 未配置 runtime：所有动作都不走 runtime（第一阶段兼容）。
func TestRoutingWithoutRunnerKeepsGoPath(t *testing.T) {
	executor := newExecutorForRouting(t, &recordingTarget{}, nil, nil)
	runCtx := runtimeRunCtx()
	for _, action := range []model.ActionKey{model.ActionSaveDraft, model.ActionSubmit, model.ActionApprove, model.ActionForward, model.ActionAddSign} {
		if executor.ShouldRunViaRuntimeForTest(runCtx, runtimeStep(action)) {
			t.Fatalf("未配置 runtime 时动作 %s 不应走 runtime", action)
		}
	}
}

// TestRoutingWithRunnerCoverSevenActions 配置后七个表单动作走 runtime，Go 直连动作不走。
func TestRoutingWithRunnerCoverSevenActions(t *testing.T) {
	executor := newExecutorForRouting(t, &recordingTarget{}, &fakeRuntimeRunner{status: "success"}, runtimeexec.NewCapabilityRegistry())
	runCtx := runtimeRunCtx()
	for _, action := range []model.ActionKey{model.ActionSaveDraft, model.ActionSubmit, model.ActionStorageFormData, model.ActionApprove, model.ActionReject, model.ActionResubmit, model.ActionForward} {
		if !executor.ShouldRunViaRuntimeForTest(runCtx, runtimeStep(action)) {
			t.Fatalf("表单动作 %s 应走 runtime", action)
		}
	}
	for _, action := range []model.ActionKey{model.ActionAddSign, model.ActionTransfer, model.ActionRollback, model.ActionRetrieve, model.ActionWithdraw, model.ActionUrge, model.ActionFollow, model.ActionUnfollow, model.ActionSystemAutomatic} {
		if executor.ShouldRunViaRuntimeForTest(runCtx, runtimeStep(action)) {
			t.Fatalf("动作 %s 不应走 runtime", action)
		}
	}
	// 无表单流程不经 runtime。
	noForm := runtimeRunCtx()
	noForm.RenderType = string(target.FormRenderTypeVueCustom)
	if executor.ShouldRunViaRuntimeForTest(noForm, runtimeStep(model.ActionSubmit)) {
		t.Fatalf("无表单流程不应走 runtime")
	}
}

// TestRuntimeActionNeverTouchesTargetWrite runtime 已接管后 TargetClient 写调用必须为零。
func TestRuntimeActionNeverTouchesTargetWrite(t *testing.T) {
	view := &recordingTarget{}
	registry := runtimeexec.NewCapabilityRegistry()
	_ = registry.Register(runtimeexec.CapabilityEntry{FlowType: "employee_leave", Action: "submit", RuntimeVersion: "exec-engine/v1", Evidence: "t012", LogRef: "test"})
	runner := &fakeRuntimeRunner{status: "success"}
	executor := newExecutorForRouting(t, view, runner, registry)
	runCtx := runtimeRunCtx()
	preview := &step.StepPreview{ActorAccount: "acct", ActionName: "提交"}
	approved := step.ApprovedStep{Attempt: 1}
	if !executor.RunViaRuntimeForTest(context.Background(), runCtx, runtimeStep(model.ActionSubmit), target.Session{}, preview, approved) {
		t.Fatalf("runtime 应接管本步")
	}
	if len(view.writeCalls()) != 0 {
		t.Fatalf("runtime 接管后目标写调用必须为零：%v", view.writeCalls())
	}
	if preview.WriteSent() != true || preview.WriteResponseForTest().IsSuccess != true {
		t.Fatalf("成功判定应回填写事实：sent=%v resp=%v", preview.WriteSent(), preview.WriteResponseForTest())
	}
}

// TestUnregisteredCapabilityBlocksStepWithoutFallback 能力未登记：阻塞且不回退 Go 写。
func TestUnregisteredCapabilityBlocksStepWithoutFallback(t *testing.T) {
	view := &recordingTarget{}
	registry := runtimeexec.NewCapabilityRegistry()
	runner := &fakeRuntimeRunner{status: "success"}
	executor := newExecutorForRouting(t, view, runner, registry)
	runCtx := runtimeRunCtx()
	preview := &step.StepPreview{ActorAccount: "acct"}
	if !executor.RunViaRuntimeForTest(context.Background(), runCtx, runtimeStep(model.ActionApprove), target.Session{}, preview, step.ApprovedStep{Attempt: 1}) {
		t.Fatalf("runtime 分支应接管（即便阻塞）")
	}
	if preview.WriteSent() {
		t.Fatalf("未登记能力不得发出写请求")
	}
	if !strings.Contains(preview.WriteErrForTest().Error(), "能力未登记") {
		t.Fatalf("阻塞原因必须说明能力未登记：%v", preview.WriteErrForTest())
	}
	if runner.callCount() != 0 {
		t.Fatalf("未登记能力不应调用 runtime：%d", runner.callCount())
	}
}

// TestVerificationModePassesUnregisteredOnce 验证模式（T015）：未登记组合放行执行一次以取得登记证据。
func TestVerificationModePassesUnregisteredOnce(t *testing.T) {
	view := &recordingTarget{taskID: "task-9"}
	registry := runtimeexec.NewCapabilityRegistry()
	runner := &fakeRuntimeRunner{status: "success"}
	executor := newExecutorForRouting(t, view, runner, registry)
	executor.EnableRuntimeVerification()
	preview := &step.StepPreview{ActorAccount: "actor-acct"}
	if !executor.RunViaRuntimeForTest(context.Background(), runtimeRunCtx(), runtimeStep(model.ActionApprove), target.Session{}, preview, step.ApprovedStep{Attempt: 1}) {
		t.Fatalf("验证模式下 runtime 应接管本步")
	}
	if runner.callCount() != 1 {
		t.Fatalf("验证模式应放行未登记组合调用 runtime，实际 %d 次", runner.callCount())
	}
	if !preview.WriteSent() {
		t.Fatalf("验证模式放行后应进入写路径")
	}
	if preview.RuntimeFormDigestForTest() == "" {
		t.Fatalf("放行时必须记录登记用表单摘要")
	}
}

// TestRegisterRuntimeCapabilityAfterVerifiedSuccess 核验确认成功后按流程+动作+表单摘要登记能力，
// 之后同组合路由直接命中登记；其他动作、其他表单摘要不自动解锁（T014 验收口径）。
func TestRegisterRuntimeCapabilityAfterVerifiedSuccess(t *testing.T) {
	view := &recordingTarget{}
	registry := runtimeexec.NewCapabilityRegistry()
	executor := newExecutorForRouting(t, view, nil, registry)
	runCtx := runtimeRunCtx()
	stepItem := runtimeStep(model.ActionSubmit)
	preview := &step.StepPreview{ActorAccount: "plan-acct"}
	preview.SetRuntimeFormDigestForTest("digest-abcdef")
	preview.SetRuntimeHandledForTest(true)
	executor.RegisterRuntimeCapabilityForTest(runCtx, stepItem, preview)
	if !registry.Registered(runCtx.FlowType, "submit", "digest-abcdef") {
		t.Fatalf("核验确认成功后同组合应已登记")
	}
	if registry.Registered(runCtx.FlowType, "approve", "digest-abcdef") {
		t.Fatalf("同流程其他动作不得被自动解锁")
	}
	if registry.Registered(runCtx.FlowType, "submit", "other-digest") {
		t.Fatalf("其他表单摘要不得被自动解锁")
	}
}

// TestRuntimeDisconnectWithWritesIsUncertainNotZeroWrite 评审 P1 回归：
// runtime 连接断开且已有目标请求写出（ErrUncertain）必须按「已发出、结果未知」处理，
// 绝不允许判成零写入失败——那会把「可能已写出」标成「无副作用」，诱发重复执行真实业务。
func TestRuntimeDisconnectWithWritesIsUncertainNotZeroWrite(t *testing.T) {
	view := &recordingTarget{}
	registry := runtimeexec.NewCapabilityRegistry()
	_ = registry.Register(runtimeexec.CapabilityEntry{FlowType: "employee_leave", Action: "submit", RuntimeVersion: "exec-engine/v1", Evidence: "e", LogRef: "l"})
	runner := &fakeRuntimeRunner{err: fmt.Errorf("%w（ws closed）", runtimeexec.ErrUncertain)}
	executor := newExecutorForRouting(t, view, runner, registry)
	preview := &step.StepPreview{ActorAccount: "acct"}
	executor.RunViaRuntimeForTest(context.Background(), runtimeRunCtx(), runtimeStep(model.ActionSubmit), target.Session{}, preview, step.ApprovedStep{Attempt: 1})
	if !preview.WriteSent() {
		t.Fatalf("ErrUncertain 必须按「写已发出」处理，不得判零写入")
	}
	var targetErr *target.Error
	if !errors.As(preview.WriteErrForTest(), &targetErr) || targetErr.Transport != target.TransportInterrupted {
		t.Fatalf("ErrUncertain 应映射为传输中断错误，实际 %v", preview.WriteErrForTest())
	}
	if preview.RuntimeHandledForTest() != true {
		t.Fatalf("runtime 仍应标记接管")
	}
}

// TestRuntimeDisconnectWithoutWritesIsCleanZeroWrite 评审 P1 对偶回归：
// 连接断开且没有任何目标请求写出（ErrFailedCleanly）保持零写入失败，不得虚报「可能已写出」。
func TestRuntimeDisconnectWithoutWritesIsCleanZeroWrite(t *testing.T) {
	view := &recordingTarget{}
	registry := runtimeexec.NewCapabilityRegistry()
	_ = registry.Register(runtimeexec.CapabilityEntry{FlowType: "employee_leave", Action: "submit", RuntimeVersion: "exec-engine/v1", Evidence: "e", LogRef: "l"})
	runner := &fakeRuntimeRunner{err: fmt.Errorf("%w（ws closed）", runtimeexec.ErrFailedCleanly)}
	executor := newExecutorForRouting(t, view, runner, registry)
	preview := &step.StepPreview{ActorAccount: "acct"}
	executor.RunViaRuntimeForTest(context.Background(), runtimeRunCtx(), runtimeStep(model.ActionSubmit), target.Session{}, preview, step.ApprovedStep{Attempt: 1})
	if preview.WriteSent() {
		t.Fatalf("ErrFailedCleanly 是零写入失败，不得标记写已发出")
	}
	if preview.WriteErrForTest() == nil || !strings.Contains(preview.WriteErrForTest().Error(), "没有任何目标请求写出") {
		t.Fatalf("失败原因必须保留连接断开事实：%v", preview.WriteErrForTest())
	}
}

// TestRuntimeResultMappingIsThreeValued 判定映射：rejected/failed 收敛到既有三值判定输入。
func TestRuntimeResultMappingIsThreeValued(t *testing.T) {
	cases := []struct {
		status      string
		expectSent  bool
		expectUncer bool
	}{
		{status: "target_rejected", expectSent: true},
		{status: "business_failed", expectSent: true},
		{status: "uncertain", expectSent: true, expectUncer: true},
		{status: "failed", expectSent: false},
	}
	for _, item := range cases {
		view := &recordingTarget{}
		registry := runtimeexec.NewCapabilityRegistry()
		_ = registry.Register(runtimeexec.CapabilityEntry{FlowType: "employee_leave", Action: "submit"})
		executor := newExecutorForRouting(t, view, &fakeRuntimeRunner{status: item.status}, registry)
		preview := &step.StepPreview{ActorAccount: "acct"}
		executor.RunViaRuntimeForTest(context.Background(), runtimeRunCtx(), runtimeStep(model.ActionSubmit), target.Session{}, preview, step.ApprovedStep{Attempt: 1})
		if preview.WriteSent() != item.expectSent {
			t.Fatalf("%s：写事实回填不符 sent=%v", item.status, preview.WriteSent())
		}
		if item.expectUncer && preview.WriteErrForTest() == nil {
			t.Fatalf("%s：应携带待确认错误", item.status)
		}
	}
}

// TestRunViaRuntimeCarriesSelectionTaskAndIdentityFacts runtime 请求必须携带写前事实：
// 审批动作带现场任务号；分支选择按「分支节点 ID → 入口节点 ID」映射；
// 发起人是计划账号的真实用户事实（绝不是实例 ID）；节点人员按目标节点代理 ID 展开。
func TestRunViaRuntimeCarriesSelectionTaskAndIdentityFacts(t *testing.T) {
	view := &recordingTarget{taskID: "task-9"}
	registry := runtimeexec.NewCapabilityRegistry()
	_ = registry.Register(runtimeexec.CapabilityEntry{FlowType: "employee_leave", Action: "approve"})
	runner := &fakeRuntimeRunner{status: "success"}
	executor := newExecutorForRouting(t, view, runner, registry)
	runCtx := runtimeRunCtx()
	runCtx.BranchSelections = map[string]string{"branch-node-1": "entry-node-1", "branch-node-2": "entry-node-2"}
	runCtx.NextNodeAuditors = map[string][]target.NextAuditor{
		"node-1": {{BizID: "biz-1", Name: "张三"}, {BizID: "", Name: "空人员"}},
	}
	preview := &step.StepPreview{ActorAccount: "actor-acct"}
	if !executor.RunViaRuntimeForTest(context.Background(), runCtx, runtimeStep(model.ActionApprove), target.Session{}, preview, step.ApprovedStep{Attempt: 1}) {
		t.Fatalf("runtime 应接管本步")
	}
	if runner.callCount() != 1 {
		t.Fatalf("runtime 应被调用一次，实际 %d", runner.callCount())
	}
	request := runner.lastRequest()
	if request.JobTaskID != "task-9" {
		t.Fatalf("审批动作必须携带现场任务号，实际 %q", request.JobTaskID)
	}
	if len(request.BranchSelections) != 2 {
		t.Fatalf("两条分支选择都应带入，实际 %d", len(request.BranchSelections))
	}
	for _, selection := range request.BranchSelections {
		if selection.NodeProxyID == selection.EntryNodeID {
			t.Fatalf("分支节点与入口节点不得混用同一 ID：%+v", selection)
		}
		if selection.NodeProxyID != "branch-node-1" && selection.NodeProxyID != "branch-node-2" {
			t.Fatalf("分支节点归属错误：%+v", selection)
		}
	}
	if request.Initiator.UserID != "user-plan" || request.Initiator.Name != "发起人" {
		t.Fatalf("发起人必须是计划账号用户事实，实际 %+v", request.Initiator)
	}
	if request.Initiator.UserID == request.InstanceID {
		t.Fatalf("发起人用户 ID 不得用主实例 ID 冒充：%q", request.Initiator.UserID)
	}
	if request.NodeProxyID != "target-node-1" {
		t.Fatalf("节点代理 ID 应取目标真实标识，实际 %q", request.NodeProxyID)
	}
	if len(request.NextNodeAuditors) != 1 || request.NextNodeAuditors[0].BizID != "biz-1" || request.NextNodeAuditors[0].AuditDetailType != "personnel" {
		t.Fatalf("空人员必须被剔除且带 personnel 类型：%+v", request.NextNodeAuditors)
	}
}

// TestRunViaRuntimeBlocksWithoutTaskFact 审批动作读不到任务事实：阻塞且不调用 runtime，绝不带空任务号执行。
func TestRunViaRuntimeBlocksWithoutTaskFact(t *testing.T) {
	view := &recordingTarget{taskID: ""}
	registry := runtimeexec.NewCapabilityRegistry()
	_ = registry.Register(runtimeexec.CapabilityEntry{FlowType: "employee_leave", Action: "approve"})
	runner := &fakeRuntimeRunner{status: "success"}
	executor := newExecutorForRouting(t, view, runner, registry)
	preview := &step.StepPreview{ActorAccount: "actor-acct"}
	if !executor.RunViaRuntimeForTest(context.Background(), runtimeRunCtx(), runtimeStep(model.ActionApprove), target.Session{}, preview, step.ApprovedStep{Attempt: 1}) {
		t.Fatalf("runtime 分支应接管（即便阻塞）")
	}
	if runner.callCount() != 0 {
		t.Fatalf("无任务事实不得调用 runtime，实际 %d 次", runner.callCount())
	}
	if preview.WriteSent() {
		t.Fatalf("无任务事实不得产生写请求")
	}
	if preview.WriteErrForTest() == nil || !strings.Contains(preview.WriteErrForTest().Error(), "待办任务") {
		t.Fatalf("阻塞原因必须说明待办任务缺失：%v", preview.WriteErrForTest())
	}
}

// TestRunViaRuntimeInitiateCarriesRealTemplate 发起模式同样必须携带真实表单模板：
// 装载载荷没有 flowId 自装载通道，空模板会装载出一张空表单并把空数据写进目标。
func TestRunViaRuntimeInitiateCarriesRealTemplate(t *testing.T) {
	view := &recordingTarget{}
	registry := runtimeexec.NewCapabilityRegistry()
	_ = registry.Register(runtimeexec.CapabilityEntry{FlowType: "employee_leave", Action: "submit"})
	runner := &fakeRuntimeRunner{status: "success"}
	executor := newExecutorForRouting(t, view, runner, registry)
	preview := &step.StepPreview{ActorAccount: "plan-acct"}
	if !executor.RunViaRuntimeForTest(context.Background(), runtimeRunCtx(), runtimeStep(model.ActionSubmit), target.Session{}, preview, step.ApprovedStep{Attempt: 1}) {
		t.Fatalf("runtime 应接管本步")
	}
	if !view.template {
		t.Fatalf("发起模式必须读取实例绑定表单模板")
	}
	request := runner.lastRequest()
	if !strings.Contains(string(request.Template), "list") {
		t.Fatalf("发起装载必须携带真实模板，实际 %s", request.Template)
	}
	if request.JobTaskID != "" {
		t.Fatalf("发起动作不携带任务号，实际 %q", request.JobTaskID)
	}
}

// TestRunViaRuntimeBlocksWithoutFormProxy FormMaking 缺少表单代理 ID：装载无据，阻塞且不调用 runtime。
func TestRunViaRuntimeBlocksWithoutFormProxy(t *testing.T) {
	view := &recordingTarget{}
	registry := runtimeexec.NewCapabilityRegistry()
	_ = registry.Register(runtimeexec.CapabilityEntry{FlowType: "employee_leave", Action: "submit"})
	runner := &fakeRuntimeRunner{status: "success"}
	executor := newExecutorForRouting(t, view, runner, registry)
	runCtx := runtimeRunCtx()
	runCtx.FormProxyID = ""
	preview := &step.StepPreview{ActorAccount: "plan-acct"}
	executor.RunViaRuntimeForTest(context.Background(), runCtx, runtimeStep(model.ActionSubmit), target.Session{}, preview, step.ApprovedStep{Attempt: 1})
	if runner.callCount() != 0 || preview.WriteSent() {
		t.Fatalf("缺少表单代理 ID 必须阻塞，不得进入 runtime")
	}
}
