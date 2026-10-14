package executor_test

import (
	"context"
	"encoding/json"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/runtimeexec"
)

type forwardResultRunner struct {
	status     string
	calls      int
	instanceID string
}

type forwardControlSessions struct {
	fakeSessions
	runtimeReady bool
}

// Current 在控制闭环的转发阶段提供原发起人 ID，提交阶段保留原夹具的身份输入。
func (s *forwardControlSessions) Current(ctx context.Context, account string) (target.Session, error) {
	active, err := s.fakeSessions.Current(ctx, account)
	if s.runtimeReady {
		active.UserID = "uid-" + account
	}
	return active, err
}

// Refresh 与 Current 使用同一身份口径，避免测试把会话恢复变成另一个人。
func (s *forwardControlSessions) Refresh(ctx context.Context, account string) (target.Session, error) {
	return s.Current(ctx, account)
}

// Health 提供已就绪的运行时，当前用例只验证复合动作的落账与停止规则。
func (r *forwardResultRunner) Health(context.Context) error { return nil }

// RunAction 返回已创建新实例后的状态，不执行任何真实目标写请求。
func (r *forwardResultRunner) RunAction(context.Context, runtimeexec.RuntimeRunRequest) (runtimeexec.ActionRunResult, error) {
	r.calls++
	instanceID := r.instanceID
	if instanceID == "" {
		instanceID = "forward-created"
	}
	return runtimeexec.ActionRunResult{Status: r.status, ForwardInstanceID: instanceID, Message: "转发已创建，附言未完成；禁止重发主请求", BusinessSummary: json.RawMessage(`{"forwardInstanceId":"` + instanceID + `"}`)}, nil
}

// TestForwardPostscriptFailureStopsWithoutZeroWriteSkip 用真实七阶段入口验证后置失败不能被可选动作跳过，且保留已创建的实例。
func TestForwardPostscriptFailureStopsWithoutZeroWriteSkip(t *testing.T) {
	for _, status := range []string{"business_failed", "uncertain", "timeout"} {
		view := &fakeTarget{instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-audit"}, DueNodes: []string{"node-audit"}}, dueTaskID: "task-forward"}
		state, facts := &fakeRunState{}, &fakeFacts{}
		runner := &forwardResultRunner{status: status}
		executor := step.NewExecutor(view, &accountRecordingSessions{}, state, facts, fixedRunConfig(), nil)
		executor.EnableRuntimeExecution(runner, "")
		compiled := model.CompiledActionStep{Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionForward, Scope: model.ActionScopeInstance}
		runCtx := newRunContext([]model.CompiledActionStep{compiled})
		runCtx.PathRun.MainInstanceRef = "original-main"
		runCtx.ActionPersonIDs = map[string][]string{step.ActionPersonIndex("", model.ActionForward): {"receiver-1"}}
		preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
		if err != nil || preview == nil || !preview.GateAllowed {
			t.Fatalf("转发门禁未通过：%v，预览=%+v", err, preview)
		}
		outcome, _, err := executor.RunApprovedStep(context.Background(), step.ApprovedStep{RunCtx: runCtx, Preview: preview, NextIndex: 0, Attempt: 1})
		if err != nil || outcome.Verdict == "confirmed_success" || outcome.AuxiliaryInstanceRef != "forward-created" || runner.calls != 1 || view.actionCalls != 0 {
			t.Fatalf("%s 不得跳过、重发或丢失实例：%+v，错误=%v，运行时调用=%d，尝试=%+v", status, outcome, err, runner.calls, facts.attempts)
		}
		if len(facts.attempts) != 1 || facts.attempts[0].SideEffect != "possible" || len(state.finishedTo) != 1 {
			t.Fatalf("%s 必须保留可能有写入并停止：尝试=%+v，状态=%+v", status, facts.attempts, state.finishedTo)
		}
	}
}
