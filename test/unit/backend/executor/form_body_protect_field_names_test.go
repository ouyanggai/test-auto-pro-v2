package executor_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/runtimeexec"
	"testing"
)

// TestRuntimeSnapshotRetainsNativeChanges 验证字段减少、清空、规范化和原生联动均保留实际快照，不把额外相等判断冒充目标业务失败。
func TestRuntimeSnapshotRetainsNativeChanges(t *testing.T) {
	for _, after := range []map[string]any{{}, {"contractSum": 0}, {"contractSum": 5000004, "currency": "CNY"}, {"legalUserName": `{"id":"user-1"}`}} {
		executor := runtimeProtectExecutor(after)
		runCtx := newRunContext([]model.CompiledActionStep{approveStep()})
		runCtx.PathRun.MainInstanceRef = "instance-1"
		before := map[string]any{"contractSum": 100, "contractAmountRmb": nil, "legalUserName": `{"id":"user-1","name":"姓名"}`}
		result := executor.CaptureRuntimeFormSnapshotForTest(context.Background(), runCtx, approveStep(), target.Session{}, runtimeProtectResult(t, before), 1)
		if result.Status != "success" {
			t.Fatalf("不得自行改判原生成功：%s", result.Message)
		}
		var summary map[string]json.RawMessage
		if err := json.Unmarshal(result.BusinessSummary, &summary); err != nil {
			t.Fatal(err)
		}
		expected, _ := json.Marshal(after)
		if string(summary["formAfter"]) != string(expected) || len(summary["formBefore"]) == 0 {
			t.Fatalf("必须保留实际前后快照：%s", result.BusinessSummary)
		}
	}
}

// failedSnapshotTarget 模拟技术读取失败，不能伪造业务拒绝或重发原动作。
type failedSnapshotTarget struct{ step.TargetClient }

// ReadInstanceCurrentData 返回明确技术错误，供写后快照失败证据测试使用。
func (failedSnapshotTarget) ReadInstanceCurrentData(context.Context, target.Session, string) (map[string]any, error) {
	return nil, errors.New("快照服务不可达")
}

// TestRuntimeSnapshotFailureRetainsWriteOutcome 验证已写成功后的快照失败单独记录，不诱导重复写入或隐藏技术错误。
func TestRuntimeSnapshotFailureRetainsWriteOutcome(t *testing.T) {
	executor := step.NewExecutor(failedSnapshotTarget{}, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)
	runCtx := newRunContext([]model.CompiledActionStep{approveStep()})
	runCtx.PathRun.MainInstanceRef = "instance-1"
	result := executor.CaptureRuntimeFormSnapshotForTest(context.Background(), runCtx, approveStep(), target.Session{}, runtimeexec.ActionRunResult{Status: "success"}, 1)
	if result.Status != "success" || !strings.Contains(string(result.BusinessSummary), "快照服务不可达") {
		t.Fatalf("技术错误须留证但不冒充业务拒绝：%+v", result)
	}
}

// TestRuntimeSnapshotPreservesNativeFailure 验证目标拒绝、超时和结果不明均不被快照阶段覆盖。
func TestRuntimeSnapshotPreservesNativeFailure(t *testing.T) {
	executor := runtimeProtectExecutor(nil)
	for _, status := range []string{"business_failed", "uncertain", "timeout"} {
		result := executor.CaptureRuntimeFormSnapshotForTest(context.Background(), newRunContext(nil), approveStep(), target.Session{}, runtimeexec.ActionRunResult{Status: status, Message: "原始结果"}, 1)
		if result.Status != status || result.Message != "原始结果" {
			t.Fatalf("目标失败不能被吞掉：%+v", result)
		}
	}
}
