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

// fakeRuntimeRunner 返回预置判定并记录调用。
type fakeRuntimeRunner struct {
	status string
	calls  int
}

func (f *fakeRuntimeRunner) RunAction(context.Context, runtimeexec.RuntimeRunRequest) (runtimeexec.ActionRunResult, error) {
	f.calls++
	return runtimeexec.ActionRunResult{Status: f.status, Message: "fake"}, nil
}

func newExecutorForRouting(t *testing.T, view *recordingTarget, runner step.RuntimeRunner, registry *runtimeexec.CapabilityRegistry) *step.Executor {
	t.Helper()
	executor := step.NewExecutor(view, nil, nil, nil, fixedRunConfigForRouting(), nil)
	if runner != nil {
		executor.EnableRuntimeExecution(runner, registry, "http://127.0.0.1:19011")
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
		EffectiveFormData: []byte(`{"reason":"x"}`),
		PathRun:           model.PathRun{ID: 1},
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
	if runner.calls != 0 {
		t.Fatalf("未登记能力不应调用 runtime：%d", runner.calls)
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
