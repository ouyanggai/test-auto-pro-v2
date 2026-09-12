// F-030 评审 P1 定向验证：执行器必须把 step_id/attempt/phase 注入目标请求上下文，
// network.log 传输层据此稳定归属每条请求；不注入时只能按秒级时间窗口猜测归属。
package executor_test

import (
	"context"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/logging"
	"test-auto-pro-v2/internal/model"
)

// scopeRecordingTarget 记录每次目标读取调用收到的上下文里的日志作用域。
type scopeRecordingTarget struct {
	*fakeTarget
	scopes []logging.Scope
}

// FindSubmittedFlow 记录实例事实读取时的作用域。
func (t *scopeRecordingTarget) FindSubmittedFlow(ctx context.Context, _ target.Session, _ string) (string, []string, string, []string, bool, error) {
	t.scopes = append(t.scopes, logging.ScopeFrom(ctx))
	return t.fakeTarget.FindSubmittedFlow(ctx, target.Session{}, "")
}

// FindDueFlow 记录待办节点读取时的作用域。
func (t *scopeRecordingTarget) FindDueFlow(ctx context.Context, _ target.Session, _ string) (string, []string, []string, bool, error) {
	t.scopes = append(t.scopes, logging.ScopeFrom(ctx))
	return t.fakeTarget.FindDueFlow(ctx, target.Session{}, "")
}

// TestBuildPreviewInjectsStepScopeIntoTargetRequests 锁定预览阶段的注入契约：
// 所有目标读取请求的上下文必须带本步 step_id、attempt=1 与当前阶段名。
func TestBuildPreviewInjectsStepScopeIntoTargetRequests(t *testing.T) {
	targetFake := &scopeRecordingTarget{
		fakeTarget: &fakeTarget{
			instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-audit"}, DueNodes: []string{"node-audit"}},
		},
	}
	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)
	runCtx := newRunContext([]model.CompiledActionStep{approveStep()})
	runCtx.PathRun.MainInstanceRef = "instance-9"
	progress := make([][2]string, 0)
	preview, _, err := executor.BuildPreviewWithProgress(context.Background(), runCtx, 0, func(phase, note string) {
		progress = append(progress, [2]string{phase, note})
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview == nil || len(targetFake.scopes) == 0 {
		t.Fatalf("预览应产生目标读取请求，实际作用域记录 %d 条", len(targetFake.scopes))
	}
	for _, scope := range targetFake.scopes {
		// approveStep 的场景序号是 2：断言跟随场景步骤号，不硬编码步骤号。
		if scope.StepID != "2" || scope.Attempt != "1" || scope.Phase == "" {
			t.Fatalf("目标请求上下文缺少步骤作用域（step_id=%q attempt=%q phase=%q）", scope.StepID, scope.Attempt, scope.Phase)
		}
	}
	if len(progress) == 0 {
		t.Fatal("预览阶段应上报实时进度说明，页面等待时才能看到正在检查什么")
	}
	for _, item := range progress {
		if item[1] == "" {
			t.Fatalf("阶段 %s 的上报说明为空", item[0])
		}
	}
}
