package debugger_test

import (
	"testing"

	"test-auto-pro-v2/internal/engine/control"
	"test-auto-pro-v2/internal/model"
)

// TestF028RetryBreakpointSetReplaysFactsAndDisarmsFirstWrite 锁定重试装填的断点规则（F-028）：
// 生效断点按控制事实回放，路径偏离断点始终强制开启；
// 本运行已有成功步骤（cursorIndex > 0）时移除首次写断点——首个写请求早已被放行过，
// 安全阀只对「从失败的第一步重新开始」有意义；从第 1 步重试时安全阀保持开启。
func TestF028RetryBreakpointSetReplaysFactsAndDisarmsFirstWrite(t *testing.T) {
	controls := []model.RunControl{
		{Kind: model.ControlFactBreakpointSet, BreakpointType: model.BreakpointStep, ObjectKind: "step", ObjectKey: "5"},
		{Kind: model.ControlFactBreakpointSet, BreakpointType: model.BreakpointNode, ObjectKind: "node", ObjectKey: "node-audit"},
		{Kind: model.ControlFactBreakpointRemove, BreakpointType: model.BreakpointNode, ObjectKind: "node", ObjectKey: "node-audit"},
	}

	// 从失败的第 1 步重试：首次写安全阀保持开启，事实回放的步骤断点生效。
	fromFirst := control.RetryBreakpointSet(controls, 0)
	if !fromFirst.Contains(control.Breakpoint{Type: model.BreakpointFirstWrite}) {
		t.Fatal("从第 1 步重试时应保留首次写断点")
	}
	if !fromFirst.Contains(control.Breakpoint{Type: model.BreakpointStep, StepNo: 5}) {
		t.Fatal("事实回放的步骤断点应在重试后继续生效")
	}
	if !fromFirst.Contains(control.Breakpoint{Type: model.BreakpointPathDeviation}) {
		t.Fatal("路径偏离断点必须始终强制开启")
	}

	// 从失败的第 3 步重试（前缀已有成功步骤）：首次写安全阀解除，其余断点照常回放。
	fromThird := control.RetryBreakpointSet(controls, 2)
	if fromThird.Contains(control.Breakpoint{Type: model.BreakpointFirstWrite}) {
		t.Fatal("前缀已有成功步骤时应移除首次写断点，避免自动重试被安全阀二次拦停")
	}
	if !fromThird.Contains(control.Breakpoint{Type: model.BreakpointStep, StepNo: 5}) {
		t.Fatal("步骤断点应跨重试保留")
	}
	if fromThird.Contains(control.Breakpoint{Type: model.BreakpointNode, NodeKey: "node-audit"}) {
		t.Fatal("删除事实必须在重试回放后继续生效")
	}
	if !fromThird.Contains(control.Breakpoint{Type: model.BreakpointPathDeviation}) {
		t.Fatal("路径偏离断点必须始终强制开启")
	}
}
