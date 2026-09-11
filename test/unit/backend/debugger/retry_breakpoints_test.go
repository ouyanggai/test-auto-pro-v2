package debugger_test

import (
	"testing"

	"test-auto-pro-v2/internal/engine/control"
	"test-auto-pro-v2/internal/model"
)

// TestF028RetryBreakpointSetReplaysFactsAndDisarmsFirstWrite 锁定重试装填的断点规则（F-028）：
// 生效断点按控制事实回放，路径偏离断点始终强制开启；首次写断点已不默认开启，
// 只有事实回放显式设置过才会出现，重试装填不再特殊处理安全阀。
func TestF028RetryBreakpointSetReplaysFactsAndDisarmsFirstWrite(t *testing.T) {
	controls := []model.RunControl{
		{Kind: model.ControlFactBreakpointSet, BreakpointType: model.BreakpointStep, ObjectKind: "step", ObjectKey: "5"},
		{Kind: model.ControlFactBreakpointSet, BreakpointType: model.BreakpointNode, ObjectKind: "node", ObjectKey: "node-audit"},
		{Kind: model.ControlFactBreakpointRemove, BreakpointType: model.BreakpointNode, ObjectKind: "node", ObjectKey: "node-audit"},
	}

	// 从失败的第 1 步重试：事实回放的步骤断点生效。
	fromFirst := control.RetryBreakpointSet(controls, 0)
	if fromFirst.Contains(control.Breakpoint{Type: model.BreakpointFirstWrite}) {
		t.Fatal("首次写断点未显式设置时不应出现在重试断点集合")
	}
	if !fromFirst.Contains(control.Breakpoint{Type: model.BreakpointStep, StepNo: 5}) {
		t.Fatal("事实回放的步骤断点应在重试后继续生效")
	}
	if !fromFirst.Contains(control.Breakpoint{Type: model.BreakpointPathDeviation}) {
		t.Fatal("路径偏离断点必须始终强制开启")
	}

	// 从失败的第 3 步重试（前缀已有成功步骤）：断点照常回放。
	fromThird := control.RetryBreakpointSet(controls, 2)
	if fromThird.Contains(control.Breakpoint{Type: model.BreakpointFirstWrite}) {
		t.Fatal("首次写断点未显式设置时不应出现在重试断点集合")
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
