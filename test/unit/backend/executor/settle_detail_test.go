// Plan 59 / Run 169 复审 §3.4 回归：终态事件必须携带本步落账明细（节点·动作·目标写接口·结果），
// 不能只写用户可见的泛化 label。原先 Finish 只接 label、不接 detail，事件流看不到终态节点的接口和结果。
package executor_test

import (
	"context"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// TestSettleDetailCarriedIntoTerminalEvent 锁定结果待确认终态：路径运行收尾事件带本步节点接口明细。
// 场景与 TestF016AuditUncertainStopsInAwaitingReconciliation 相同（成功声明 + 待办未变 → 不确定），
// 但断言点是终态事件的 detail：必须非空且包含本步实际调用的目标写接口。
func TestSettleDetailCarriedIntoTerminalEvent(t *testing.T) {
	fakeTarget := &fakeTarget{
		instance:    fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-audit"}, DueNodes: []string{"node-audit"}},
		dueTaskID:   "task-1",
		auditResult: &target.AuditCurrentTaskResult{InstanceID: "instance-9", Status: "run"},
	}
	runState := &fakeRunState{}
	executor := step.NewExecutor(fakeTarget, &fakeSessions{}, runState, &fakeFacts{}, fixedRunConfig(), nil)
	runCtx := newRunContext([]model.CompiledActionStep{approveStep()})
	runCtx.PathRun.MainInstanceRef = "instance-9"

	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || preview == nil || !preview.GateAllowed {
		t.Fatalf("审批步门禁应通过：err=%v block=%s", err, previewBlock(preview))
	}
	if _, _, err := executor.RunApprovedStep(context.Background(), step.ApprovedStep{RunCtx: runCtx, Preview: preview, NextIndex: 0}); err != nil {
		t.Fatalf("放行执行失败：%v", err)
	}
	if len(runState.finishDetails) != 1 || runState.finishedTo[0] != model.PathRunStatusAwaitingReconciliation {
		t.Fatalf("应恰好写一次结果待确认终态事件：to=%+v details=%+v", runState.finishedTo, runState.finishDetails)
	}
	detail := runState.finishDetails[0]
	if strings.TrimSpace(detail) == "" {
		t.Fatalf("终态事件必须携带本步落账明细（节点·动作·接口·结果），实际为空")
	}
	if preview.Endpoint != "" && !strings.Contains(detail, preview.Endpoint) {
		t.Fatalf("终态明细应包含本步目标写接口 %q，实际 %q", preview.Endpoint, detail)
	}
}
