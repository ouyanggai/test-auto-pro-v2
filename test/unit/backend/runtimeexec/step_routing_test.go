// T011 七阶段执行器路由契约：
// ① 未配置 runtime → 全部动作保持 Go 写路径；
// ② 配置后仅七个表单动作走 runtime，Go 直连动作不受影响；
// ③ 执行服务不可用时写前阻塞且不回退 Go 写；
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

type fakeRuntimeRunner struct {
	mu        sync.Mutex
	status    string
	err       error
	healthErr error
	trace     json.RawMessage
	calls     int
	requests  []runtimeexec.RuntimeRunRequest
}

func (f *fakeRuntimeRunner) Health(context.Context) error {
	if f == nil {
		return errors.New("执行服务未配置")
	}
	return f.healthErr
}

func (f *fakeRuntimeRunner) RunAction(_ context.Context, request runtimeexec.RuntimeRunRequest) (runtimeexec.ActionRunResult, error) {
	f.mu.Lock()
	f.calls++
	f.requests = append(f.requests, request)
	f.mu.Unlock()
	if f.err != nil {
		return runtimeexec.ActionRunResult{Status: "failed", Message: "fake-error", WriteTraceIDs: []string{"trace-1"}}, f.err
	}
	return runtimeexec.ActionRunResult{Status: f.status, Message: "fake", RequestTrace: f.trace}, nil
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

func newExecutorForRouting(t *testing.T, view *recordingTarget, runner step.RuntimeRunner) *step.Executor {
	t.Helper()
	executor := step.NewExecutor(view, &fakeStepSessions{byAccount: map[string]target.Session{
		"plan-acct":  {UserID: "user-plan", Summary: target.AccountSummary{Account: "plan-acct", DisplayName: "发起人"}},
		"actor-acct": {UserID: "user-actor", Summary: target.AccountSummary{Account: "actor-acct", DisplayName: "处理人"}},
	}}, nil, nil, fixedRunConfigForRouting(), nil)
	if runner != nil {
		executor.EnableRuntimeExecution(runner, "")
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
	executor := newExecutorForRouting(t, &recordingTarget{}, nil)
	runCtx := runtimeRunCtx()
	for _, action := range []model.ActionKey{model.ActionSaveDraft, model.ActionSubmit, model.ActionApprove, model.ActionForward, model.ActionAddSign} {
		if executor.ShouldRunViaRuntimeForTest(runCtx, runtimeStep(action)) {
			t.Fatalf("未配置 runtime 时动作 %s 不应走 runtime", action)
		}
	}
}

// TestRoutingWithRunnerCoverSevenActions 配置后七个表单动作走 runtime，Go 直连动作不走。
func TestRoutingWithRunnerCoverSevenActions(t *testing.T) {
	executor := newExecutorForRouting(t, &recordingTarget{}, &fakeRuntimeRunner{status: "success"})
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
	// NoFormFlow 现在也走 runtime（继承原生能力，见 F-036 P1-8）。
	noForm := runtimeRunCtx()
	noForm.RenderType = string(target.FormRenderTypeNoFormFlow)
	if !executor.ShouldRunViaRuntimeForTest(noForm, runtimeStep(model.ActionSubmit)) {
		t.Fatalf("NoFormFlow 提交现在应该走 runtime")
	}
}

// TestRuntimeActionNeverTouchesTargetWrite runtime 已接管后 TargetClient 写调用必须为零。
func TestRuntimeActionNeverTouchesTargetWrite(t *testing.T) {
	view := &recordingTarget{}
	runner := &fakeRuntimeRunner{status: "success"}
	executor := newExecutorForRouting(t, view, runner)
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

// TestUnavailableExecutorBlocksSevenActionsWithoutFallback 执行服务不可用：写前阻塞且不回退 Go 写。
func TestUnavailableExecutorBlocksSevenActionsWithoutFallback(t *testing.T) {
	view := &recordingTarget{}
	runner := &fakeRuntimeRunner{status: "success", healthErr: errors.New("连接拒绝")}
	executor := newExecutorForRouting(t, view, runner)
	preview := &step.StepPreview{ActorAccount: "acct"}
	if !executor.RunViaRuntimeForTest(context.Background(), runtimeRunCtx(), runtimeStep(model.ActionSubmit), target.Session{}, preview, step.ApprovedStep{Attempt: 1}) {
		t.Fatalf("runtime 分支应接管（即便阻塞）")
	}
	if preview.WriteSent() {
		t.Fatalf("执行服务不可用不得发出写请求")
	}
	if !strings.Contains(preview.WriteErrForTest().Error(), "执行服务不可用") {
		t.Fatalf("阻塞原因必须说明执行服务不可用：%v", preview.WriteErrForTest())
	}
	if runner.callCount() != 0 {
		t.Fatalf("健康检查失败不应调用 RunAction：%d", runner.callCount())
	}
	if len(view.writeCalls()) != 0 {
		t.Fatalf("不得回退 Go 写路径：%v", view.writeCalls())
	}
}

// TestRuntimeDisconnectWithWritesIsUncertainNotZeroWrite 评审 P1 回归：
// runtime 连接断开且已有目标请求写出（ErrUncertain）必须按「已发出、结果未知」处理，
// 绝不允许判成零写入失败——那会把「可能已写出」标成「无副作用」，诱发重复执行真实业务。
func TestRuntimeDisconnectWithWritesIsUncertainNotZeroWrite(t *testing.T) {
	view := &recordingTarget{}
	runner := &fakeRuntimeRunner{err: fmt.Errorf("%w（ws closed）", runtimeexec.ErrUncertain)}
	executor := newExecutorForRouting(t, view, runner)
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
	runner := &fakeRuntimeRunner{err: fmt.Errorf("%w（ws closed）", runtimeexec.ErrFailedCleanly)}
	executor := newExecutorForRouting(t, view, runner)
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
		executor := newExecutorForRouting(t, view, &fakeRuntimeRunner{status: item.status})
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
	runner := &fakeRuntimeRunner{status: "success"}
	executor := newExecutorForRouting(t, view, runner)
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
	runner := &fakeRuntimeRunner{status: "success"}
	executor := newExecutorForRouting(t, view, runner)
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

// TestRunViaRuntimeInitiateDoesNotInjectTemplate v2 装载由原生页面自行读模板，Go 不得再注入 template。
func TestRunViaRuntimeInitiateDoesNotInjectTemplate(t *testing.T) {
	view := &recordingTarget{}
	runner := &fakeRuntimeRunner{status: "success"}
	executor := newExecutorForRouting(t, view, runner)
	preview := &step.StepPreview{ActorAccount: "plan-acct"}
	if !executor.RunViaRuntimeForTest(context.Background(), runtimeRunCtx(), runtimeStep(model.ActionSubmit), target.Session{}, preview, step.ApprovedStep{Attempt: 1}) {
		t.Fatalf("runtime 应接管本步")
	}
	if view.template {
		t.Fatalf("v2 不得由 Go 读取并注入表单模板")
	}
	request := runner.lastRequest()
	if request.JobTaskID != "" {
		t.Fatalf("发起动作不携带任务号，实际 %q", request.JobTaskID)
	}
}

// TestRunViaRuntimeBlocksWithoutFormProxy FormMaking 缺少表单代理 ID：装载无据，阻塞且不调用 runtime。
func TestRunViaRuntimeBlocksWithoutFormProxy(t *testing.T) {
	view := &recordingTarget{}
	runner := &fakeRuntimeRunner{status: "success"}
	executor := newExecutorForRouting(t, view, runner)
	runCtx := runtimeRunCtx()
	runCtx.FormProxyID = ""
	preview := &step.StepPreview{ActorAccount: "plan-acct"}
	executor.RunViaRuntimeForTest(context.Background(), runCtx, runtimeStep(model.ActionSubmit), target.Session{}, preview, step.ApprovedStep{Attempt: 1})
	if runner.callCount() != 0 || preview.WriteSent() {
		t.Fatalf("缺少表单代理 ID 必须阻塞，不得进入 runtime")
	}
}

// TestRunViaRuntimeRecordsDetailProjection 评审 P1-7 回归：runtime 动作的三阶段投影
// 必须随预览留存（尝试行据此落库），请求轨迹不再被丢弃；ErrUncertain 错误路径同样留存
// 已收集的写请求 trace，供尝试行与能力登记证据使用。
func TestRunViaRuntimeRecordsDetailProjection(t *testing.T) {
	view := &recordingTarget{}
	runner := &fakeRuntimeRunner{status: "success", trace: json.RawMessage(`[{"seq":1,"method":"POST","url":"https://t/web/flowInstanceApi/submit","state":"fulfilled","httpStatus":200,"durationMs":120}]`)}
	executor := newExecutorForRouting(t, view, runner)
	preview := &step.StepPreview{ActorAccount: "plan-acct"}
	executor.RunViaRuntimeForTest(context.Background(), runtimeRunCtx(), runtimeStep(model.ActionSubmit), target.Session{}, preview, step.ApprovedStep{Attempt: 1})
	detail := preview.RuntimeDetailForTest()
	if detail == "" {
		t.Fatalf("runtime 成功后必须留存三阶段投影")
	}
	for _, want := range []string{"表单业务处理", "流程动作", "提交后业务处理", "flowInstanceApi/submit"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("三阶段投影缺少 %s：%s", want, detail)
		}
	}
	// 错误路径：ErrUncertain 必须按已写出处理且保留首个写请求 trace。
	uncertain := &step.StepPreview{ActorAccount: "plan-acct"}
	runnerErr := &fakeRuntimeRunner{err: fmt.Errorf("%w（ws closed）", runtimeexec.ErrUncertain)}
	executorErr := newExecutorForRouting(t, &recordingTarget{}, runnerErr)
	executorErr.RunViaRuntimeForTest(context.Background(), runtimeRunCtx(), runtimeStep(model.ActionSubmit), target.Session{}, uncertain, step.ApprovedStep{Attempt: 1})
	if !uncertain.WriteSent() {
		t.Fatalf("ErrUncertain 必须按写已发出处理")
	}
	if uncertain.WriteTraceIDForTest() != "trace-1" {
		t.Fatalf("错误路径必须保留已收集的写请求 trace，实际 %q", uncertain.WriteTraceIDForTest())
	}
}

// TestRunViaRuntimeCarriesProxyNodeAndFormFacts 评审 P1-3 回归：runtime 请求必须携带
// 目标代理与节点事实——FlowDialog/审批组件的最终流程载荷直接消费这些字段，
// 缺失会让提交发送 flowProxyId:"" 或丢业务关联。
func TestRunViaRuntimeCarriesProxyNodeAndFormFacts(t *testing.T) {
	view := &recordingTarget{taskID: "task-9"}
	runner := &fakeRuntimeRunner{status: "success"}
	executor := newExecutorForRouting(t, view, runner)
	runCtx := runtimeRunCtx()
	runCtx.FlowProxyID = "flow-proxy-plan"
	runCtx.Nodes["node-1"] = step.NodeInfo{TargetNodeID: "target-node-1", Name: "审批节点", AuditType: "run_node_choose", EditableFields: []string{"reason"}}
	runCtx.Nodes["node-2"] = step.NodeInfo{TargetNodeID: "target-node-2", Name: "下一节点", AuditType: "company"}
	runCtx.Steps = []model.CompiledActionStep{
		{Action: model.ActionApprove, NodeKey: "node-1"},
		{Action: model.ActionApprove, NodeKey: "node-2"},
	}
	preview := &step.StepPreview{ActorAccount: "actor-acct"}
	preview.Facts = step.InstanceFacts{FormProxyID: "form-proxy-live", FlowProxyID: "flow-proxy-live",
		BizRelevance: []target.BizRelevance{{OtherBiz: "company", OtherBizID: "c-1"}}}
	preview.FormOverlaid = []string{"reason"}
	if !executor.RunViaRuntimeForTest(context.Background(), runCtx, runCtx.Steps[0], target.Session{}, preview, step.ApprovedStep{Attempt: 1}) {
		t.Fatalf("runtime 应接管本步")
	}
	request := runner.lastRequest()
	if request.FormProxyID != "form-proxy-live" {
		t.Fatalf("表单代理必须优先取实例当前事实，实际 %q", request.FormProxyID)
	}
	if request.FlowProxyID != "flow-proxy-live" {
		t.Fatalf("流程代理必须优先取实例当前事实，实际 %q", request.FlowProxyID)
	}
	if request.SelectFlowType != "employee_leave" {
		t.Fatalf("装载必须携带流程类型，实际 %q", request.SelectFlowType)
	}
	if request.FlowNodeType != "run_node_choose" {
		t.Fatalf("装载必须携带当前节点审批方式，实际 %q", request.FlowNodeType)
	}
	if request.NextNodeProxyID != "target-node-2" {
		t.Fatalf("下一节点代理应取真实目标标识，实际 %q", request.NextNodeProxyID)
	}
	if len(request.BizRelevance) != 1 || request.BizRelevance[0].OtherBizID != "c-1" {
		t.Fatalf("装载必须携带实例业务关联：%+v", request.BizRelevance)
	}
	if len(request.Permissions) != 1 || request.Permissions[0] != "reason" {
		t.Fatalf("装载必须携带当前节点可编辑字段：%+v", request.Permissions)
	}
	if len(request.ChangedFields) != 1 || request.ChangedFields[0] != "reason" {
		t.Fatalf("装载必须携带本步覆盖字段：%+v", request.ChangedFields)
	}
	if request.BranchNodeAuditTypes["target-node-1"] != "run_node_choose" || request.BranchNodeAuditTypes["target-node-2"] != "company" {
		t.Fatalf("选择闭环需要节点审批方式映射：%+v", request.BranchNodeAuditTypes)
	}
}

// TestRunViaRuntimeForwardRequiresReceiver 评审 P1-4 回归：转发未配置接收人必须
// 写前阻塞且不调用 runtime；配置接收人后请求必须携带转发事实。
func TestRunViaRuntimeForwardRequiresReceiver(t *testing.T) {
	view := &recordingTarget{}
	runner := &fakeRuntimeRunner{status: "success"}
	executor := newExecutorForRouting(t, view, runner)
	runCtx := runtimeRunCtx()
	runCtx.PathRun.MainInstanceRef = "instance-1"
	forwardStep := runtimeStep(model.ActionForward)
	// 未配置接收人：阻塞，零写入，不进入 runtime。
	blocked := &step.StepPreview{ActorAccount: "actor-acct"}
	executor.RunViaRuntimeForTest(context.Background(), runCtx, forwardStep, target.Session{}, blocked, step.ApprovedStep{Attempt: 1})
	if blocked.WriteSent() || runner.callCount() != 0 {
		t.Fatalf("无接收人的转发必须写前阻塞且不进入 runtime")
	}
	if !strings.Contains(blocked.WriteErrForTest().Error(), "接收人") {
		t.Fatalf("阻塞原因必须说明接收人缺失：%v", blocked.WriteErrForTest())
	}
	// 配置接收人：请求携带接收人与转发装载模式。
	runCtx.ActionPersonIDs = map[string][]string{
		step.ActionPersonIndex("node-1", model.ActionForward): {"receiver-1"},
	}
	preview := &step.StepPreview{ActorAccount: "actor-acct"}
	if !executor.RunViaRuntimeForTest(context.Background(), runCtx, forwardStep, target.Session{}, preview, step.ApprovedStep{Attempt: 1}) {
		t.Fatalf("runtime 应接管本步")
	}
	request := runner.lastRequest()
	if request.ReceiverID != "receiver-1" {
		t.Fatalf("转发请求必须携带接收人，实际 %q", request.ReceiverID)
	}
	if request.InstanceID != "instance-1" {
		t.Fatalf("转发请求必须携带主实例引用，实际 %q", request.InstanceID)
	}
}

// TestRunViaRuntimeValidatesBranchSelectionAgainstGraph 评审 P1-6 回归：
// 分支选择必须通过真实图边校验——入口不是分支节点的直接后继时写前阻塞，
// 绝不让目标返回的临时节点集合诱导工具发出与已保存配置无关的分支。
func TestRunViaRuntimeValidatesBranchSelectionAgainstGraph(t *testing.T) {
	view := &recordingTarget{taskID: "task-9"}
	runner := &fakeRuntimeRunner{status: "success"}
	executor := newExecutorForRouting(t, view, runner)
	runCtx := runtimeRunCtx()
	runCtx.BranchSelections = map[string]string{"branch-node-1": "entry-node-1"}
	runCtx.GraphEdges = map[string][]step.GraphEdgeInfo{
		"branch-node-1": {{Target: "other-entry"}},
	}
	preview := &step.StepPreview{ActorAccount: "actor-acct"}
	executor.RunViaRuntimeForTest(context.Background(), runCtx, runtimeStep(model.ActionApprove), target.Session{}, preview, step.ApprovedStep{Attempt: 1})
	if preview.WriteSent() || runner.callCount() != 0 {
		t.Fatalf("入口不属于分支节点后继时必须写前阻塞，不得进入 runtime")
	}
	if !strings.Contains(preview.WriteErrForTest().Error(), "直接后继") {
		t.Fatalf("阻塞原因必须说明图边校验失败：%v", preview.WriteErrForTest())
	}
	// 入口是真实后继：正常进入 runtime。
	runCtx.GraphEdges = map[string][]step.GraphEdgeInfo{
		"branch-node-1": {{Target: "entry-node-1"}},
	}
	previewOK := &step.StepPreview{ActorAccount: "actor-acct"}
	if !executor.RunViaRuntimeForTest(context.Background(), runCtx, runtimeStep(model.ActionApprove), target.Session{}, previewOK, step.ApprovedStep{Attempt: 1}) {
		t.Fatalf("runtime 应接管本步")
	}
	if runner.callCount() != 1 {
		t.Fatalf("图边校验通过后应进入 runtime，实际 %d 次", runner.callCount())
	}
}
