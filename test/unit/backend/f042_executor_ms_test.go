package backend_test

import (
	"encoding/json"
	"testing"

	"test-auto-pro-v2/internal/runtimeexec"
)

// TestActionRunResultDecodesExecutorMs 锁定执行器计时协议边界（F-042 选项 B）：
// executorMs 用指针区分「结果带回耗时」与「未回传」——存在即已知（含真实 0），缺失即未知。
// 若把字段退化为非指针 int64，缺失与真实 0 会被混淆，本用例即失败。
func TestActionRunResultDecodesExecutorMs(t *testing.T) {
	var present runtimeexec.ActionRunResult
	if err := json.Unmarshal([]byte(`{"status":"success","executorMs":1234}`), &present); err != nil {
		t.Fatalf("解码带 executorMs 的结果失败：%v", err)
	}
	if present.ExecutorMs == nil || *present.ExecutorMs != 1234 {
		t.Fatalf("executorMs=1234 应解码为已知值 1234，实际 %v", present.ExecutorMs)
	}

	// 真实 0 毫秒是合法值，必须与「未知」区分：存在即已知，即使值为 0。
	var zero runtimeexec.ActionRunResult
	if err := json.Unmarshal([]byte(`{"status":"success","executorMs":0}`), &zero); err != nil {
		t.Fatalf("解码 executorMs=0 失败：%v", err)
	}
	if zero.ExecutorMs == nil || *zero.ExecutorMs != 0 {
		t.Fatalf("executorMs=0 应解码为已知 0，实际 %v", zero.ExecutorMs)
	}

	// 缺失字段（连接断开等无结果场景）必须解码为未知（nil），报告据此显示「未知」。
	var absent runtimeexec.ActionRunResult
	if err := json.Unmarshal([]byte(`{"status":"success"}`), &absent); err != nil {
		t.Fatalf("解码无 executorMs 的结果失败：%v", err)
	}
	if absent.ExecutorMs != nil {
		t.Fatalf("缺失 executorMs 必须解码为未知（nil），实际 %v", *absent.ExecutorMs)
	}
}
