// Plan 59 / Run 169 复审 §3.3 回归：runtime 身份读取失败必须在写前阻塞，绝不静默回退会话缓存。
//
// 背景：sessionFactsForRuntime 曾忽略 CurrentUserIdentity 错误，继续用会话缓存里的身份装载 runtime，
// 于是 runtime 可能拿着不完整或过期的公司/部门/岗位事实继续写目标。协议合约要求任何身份读取失败
// 都由调用方阻塞（protocol_matrix.CurrentUserIdentity），nodeFormData 已有同口径，runtime 不能例外。
package runtimeexec

import (
	"context"
	"errors"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// identityErrTarget 实现实时身份读取能力，但读取一律失败：验证读取失败时写前阻塞。
type identityErrTarget struct {
	*recordingTarget
}

// CurrentUserIdentity 覆盖内嵌假件：始终返回读取错误，模拟实时身份接口失败。
func (t *identityErrTarget) CurrentUserIdentity(context.Context, target.Session) (target.UserIdentity, error) {
	return target.UserIdentity{}, errors.New("目录服务超时")
}

// noIdentityTarget 只暴露 TargetClient 方法集，不实现 runtimeIdentityReader：验证能力缺失时阻塞。
type noIdentityTarget struct {
	step.TargetClient
}

// TestRuntimeIdentityReadErrorBlocksBeforeWrite 锁定 §3.3 场景①：实时身份读取返回错误时，
// 写前阻塞，不发出任何目标写请求，也不调用 runtime RunAction。
func TestRuntimeIdentityReadErrorBlocksBeforeWrite(t *testing.T) {
	view := &recordingTarget{}
	runner := &fakeRuntimeRunner{status: "success"}
	executor := step.NewExecutor(&identityErrTarget{recordingTarget: view}, &fakeStepSessions{byAccount: map[string]target.Session{
		"plan-acct": {UserID: "user-plan", Summary: target.AccountSummary{Account: "plan-acct", DisplayName: "发起人"}},
	}}, nil, nil, fixedRunConfigForRouting(), nil)
	executor.EnableRuntimeExecution(runner, "")
	preview := &step.StepPreview{ActorAccount: "plan-acct"}
	if !executor.RunViaRuntimeForTest(context.Background(), runtimeRunCtx(), runtimeStep(model.ActionSubmit), target.Session{}, preview, step.ApprovedStep{Attempt: 1}) {
		t.Fatalf("runtime 分支应接管（即便阻塞）")
	}
	if preview.WriteSent() {
		t.Fatalf("身份读取失败不得发出写请求")
	}
	if runner.callCount() != 0 {
		t.Fatalf("身份门禁阻塞时不应调用 RunAction：%d", runner.callCount())
	}
	if len(view.writeCalls()) != 0 {
		t.Fatalf("身份门禁阻塞时目标写调用必须为零：%v", view.writeCalls())
	}
	if !strings.Contains(preview.WriteErrForTest().Error(), "实时身份") {
		t.Fatalf("阻塞原因必须说明实时身份读取失败：%v", preview.WriteErrForTest())
	}
}

// TestRuntimeIdentityCapabilityMissingBlocks 锁定 §3.3 场景③：目标不实现实时身份读取能力时，
// 按现行约定明确阻塞，绝不静默用会话缓存冒充实时事实。
func TestRuntimeIdentityCapabilityMissingBlocks(t *testing.T) {
	view := &recordingTarget{}
	runner := &fakeRuntimeRunner{status: "success"}
	executor := step.NewExecutor(noIdentityTarget{TargetClient: view}, &fakeStepSessions{byAccount: map[string]target.Session{
		"plan-acct": {UserID: "user-plan", Summary: target.AccountSummary{Account: "plan-acct", DisplayName: "发起人"}},
	}}, nil, nil, fixedRunConfigForRouting(), nil)
	executor.EnableRuntimeExecution(runner, "")
	preview := &step.StepPreview{ActorAccount: "plan-acct"}
	executor.RunViaRuntimeForTest(context.Background(), runtimeRunCtx(), runtimeStep(model.ActionSubmit), target.Session{}, preview, step.ApprovedStep{Attempt: 1})
	if preview.WriteSent() {
		t.Fatalf("缺少实时身份读取能力不得发出写请求")
	}
	if runner.callCount() != 0 {
		t.Fatalf("能力缺失时不应调用 RunAction：%d", runner.callCount())
	}
	if !strings.Contains(preview.WriteErrForTest().Error(), "不支持实时身份读取") {
		t.Fatalf("阻塞原因必须说明缺少实时身份读取能力：%v", preview.WriteErrForTest())
	}
}

// TestRuntimeReceivesCurrentIdentity 锁定 §3.3 场景②：实时身份完整时，runtime 收到的
// 公司/部门/岗位/用户与当前读取结果一致，客户码/账号沿用会话固有事实。
func TestRuntimeReceivesCurrentIdentity(t *testing.T) {
	view := &recordingTarget{}
	runner := &fakeRuntimeRunner{status: "success"}
	executor := newExecutorForRouting(t, view, runner)
	preview := &step.StepPreview{ActorAccount: "plan-acct"}
	session := target.Session{CustomerCode: "cust-9", Summary: target.AccountSummary{Account: "actor-acct"}}
	if !executor.RunViaRuntimeForTest(context.Background(), runtimeRunCtx(), runtimeStep(model.ActionSubmit), session, preview, step.ApprovedStep{Attempt: 1}) {
		t.Fatalf("runtime 应接管本步")
	}
	got := runner.lastRequest().Session
	if got.UserID != "user-actor" || got.DisplayName != "处理人" {
		t.Fatalf("用户身份应来自实时读取：%+v", got)
	}
	if got.CompanyName != "公司一" || got.DepartmentName != "部门一" || got.DutyName != "岗位一" {
		t.Fatalf("公司/部门/岗位应来自实时读取：%+v", got)
	}
	if got.CompanyID != "company-1" || got.DepartmentID != "dept-1" || got.DutyID != "duty-1" {
		t.Fatalf("组织标识应来自实时读取：%+v", got)
	}
	if got.CustomerCode != "cust-9" || got.Account != "actor-acct" {
		t.Fatalf("客户码/账号应沿用会话固有事实：%+v", got)
	}
}
