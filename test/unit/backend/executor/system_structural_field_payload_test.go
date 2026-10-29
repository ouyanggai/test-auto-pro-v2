// 系统标识只在协议顶层绑定；正文载荷剔除历史实例 ID，写后快照按目标原值留证，不添加字段不变性门禁。
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

// structuralFieldRunContext 构造一条带系统结构性字段的表单快照，节点可编辑一批业务字段。
func structuralFieldRunContext(effective string) step.RunContext {
	runCtx := newRunContext([]model.CompiledActionStep{submitStep(), approveStep()})
	runCtx.EffectiveFormData = []byte(effective)
	runCtx.Nodes = map[string]step.NodeInfo{
		"node-start": {Name: "发起人", Type: "start", TargetNodeID: "real-start",
			EditableFields: []string{"amount"}},
		"node-audit": {Name: "部门审批", Type: "审批", TargetNodeID: "real-audit",
			EditableFields: []string{"accountantOpinion"}},
	}
	runCtx.NodeEditableFields = map[string][]string{
		"node-start": {"amount"},
		"node-audit": {"accountantOpinion"},
	}
	return runCtx
}

// TestStructuralFieldDroppedFromInstancePayload 锁定实例分支：历史 formData 与实例基线里的
// flowInstanceId 都不得进入最终业务载荷，业务字段照常携带。
func TestStructuralFieldDroppedFromInstancePayload(t *testing.T) {
	runCtx := structuralFieldRunContext(`{"amount":100,"accountantOpinion":"配置意见","flowInstanceId":"cfg-instance"}`)
	runCtx.PathRun.MainInstanceRef = "instance-1"
	instanceCurrent := map[string]any{
		"amount":            json.Number("100"),
		"accountantOpinion": "上游未填",
		"legalOpinion":      "法务已填",
		"flowInstanceId":    "instance-1",
	}
	plan, err := step.BuildNodeFormData(runCtx, approveStep(), instanceCurrent, true)
	if err != nil {
		t.Fatalf("构造审批表单数据失败：%v", err)
	}
	values := decodeFormPayload(t, plan.Payload)
	if _, exists := values["flowInstanceId"]; exists {
		t.Fatalf("系统结构性字段 flowInstanceId 不得作为业务字段透传：%v", values)
	}
	if _, exists := values["legalOpinion"]; !exists {
		t.Fatalf("上游业务字段必须原样保留：%v", values)
	}
	if got := string(values["accountantOpinion"]); got != `"配置意见"` {
		t.Fatalf("本节点可编辑业务字段应按配置覆盖，实际 %s", got)
	}
	if containsField(plan.Decision.PreservedFields, "flowInstanceId") || containsField(plan.Decision.OverlaidFields, "flowInstanceId") {
		t.Fatalf("结构性字段不得出现在覆盖/保留清单：overlaid=%v preserved=%v", plan.Decision.OverlaidFields, plan.Decision.PreservedFields)
	}
}

// TestStructuralFieldDroppedFromInitiationPayload 锁定发起分支：历史配置里的 flowInstanceId
// 不得进入新发起载荷，新实例 ID 由目标提交时生成。
func TestStructuralFieldDroppedFromInitiationPayload(t *testing.T) {
	runCtx := structuralFieldRunContext(`{"amount":100,"flowInstanceId":"cfg-instance"}`)
	plan, err := step.BuildNodeFormData(runCtx, submitStep(), nil, false)
	if err != nil {
		t.Fatalf("构造发起表单数据失败：%v", err)
	}
	values := decodeFormPayload(t, plan.Payload)
	if _, exists := values["flowInstanceId"]; exists {
		t.Fatalf("发起载荷不得透传 flowInstanceId：%v", values)
	}
	if _, exists := values["amount"]; !exists {
		t.Fatalf("发起载荷应携带发起人可编辑业务字段：%v", values)
	}
}

// runtimeProtectExecutor 组装带实例当前数据的执行器，供 runtime 正文保护回归复用。
func runtimeProtectExecutor(after map[string]any) *step.Executor {
	fake := &fakeTarget{instance: fakeTargetView{Found: true, Status: "run"}, instanceFormData: after}
	return step.NewExecutor(fake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)
}

// runtimeProtectResult 组装一次成功的 runtime 结果，写前正文由 businessSummary.formBefore 提供。
func runtimeProtectResult(t *testing.T, before map[string]any) runtimeexec.ActionRunResult {
	t.Helper()
	summary, err := json.Marshal(map[string]any{"formBefore": before})
	if err != nil {
		t.Fatalf("构造业务摘要失败：%v", err)
	}
	return runtimeexec.ActionRunResult{Status: "success", BusinessSummary: summary}
}

// TestRuntimeProtectIgnoresStructuralFieldDrop 锁定 §3.1 场景②：写前正文含 flowInstanceId、
// 写后目标回读不含该键但业务字段完整时，正文保护保持 success，绝不误判 business_failed。
func TestRuntimeProtectIgnoresStructuralFieldDrop(t *testing.T) {
	after := map[string]any{"amount": json.Number("100"), "accountantOpinion": "已办意见"}
	executor := runtimeProtectExecutor(after)
	runCtx := newRunContext([]model.CompiledActionStep{approveStep()})
	runCtx.PathRun.MainInstanceRef = "instance-1"
	before := map[string]any{"amount": 100, "accountantOpinion": "已办意见", "flowInstanceId": "instance-1"}
	result := executor.CaptureRuntimeFormSnapshotForTest(context.Background(), runCtx, approveStep(), target.Session{}, runtimeProtectResult(t, before), 1)
	if result.Status != "success" {
		t.Fatalf("结构性字段回读缺失不得改判失败，实际 %q：%s", result.Status, result.Message)
	}
}

// TestRuntimeSnapshotKeepsTargetFieldRemoval 锁定 §3.1 场景③：真实业务字段在写后丢失时
// 保留实际快照而不新增字段存在性业务门禁。
func TestRuntimeSnapshotKeepsTargetFieldRemoval(t *testing.T) {
	after := map[string]any{"amount": json.Number("100")}
	executor := runtimeProtectExecutor(after)
	runCtx := newRunContext([]model.CompiledActionStep{approveStep()})
	runCtx.PathRun.MainInstanceRef = "instance-1"
	before := map[string]any{"amount": 100, "accountantOpinion": "已办意见", "flowInstanceId": "instance-1"}
	result := executor.CaptureRuntimeFormSnapshotForTest(context.Background(), runCtx, approveStep(), target.Session{}, runtimeProtectResult(t, before), 1)
	if result.Status != "success" {
		t.Fatalf("目标省略字段不能替代真实动作结果裁决，实际 %q", result.Status)
	}
}
