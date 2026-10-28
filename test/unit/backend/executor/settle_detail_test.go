// 回归结果待确认终态事件保留步骤业务上下文与结果原因；接口轨迹由动作详情单独展示。
package executor_test

import (
	"context"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// TestUncertainTerminalEventKeepsBusinessContext 锁定结果待确认终态事件的步骤、节点、动作、执行人与原因。
// 总览事件不重复动作详情中的逐接口轨迹，避免把事件流与接口明细混为一谈。
func TestUncertainTerminalEventKeepsBusinessContext(t *testing.T) {
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
		t.Fatalf("终态事件必须携带本步业务上下文和结果原因，实际为空")
	}
	for _, want := range []string{
		"第 2 步",
		"节点「部门审批」·同意",
		"执行人：测试账号",
		"目标接口已返回成功，但执行结果暂时无法确认",
	} {
		if !strings.Contains(detail, want) {
			t.Errorf("终态事件缺少业务上下文 %q，实际 %q", want, detail)
		}
	}
	if strings.Contains(detail, preview.Endpoint) {
		t.Fatalf("终态事件不应重复动作详情中的目标接口 %q，实际 %q", preview.Endpoint, detail)
	}
}
