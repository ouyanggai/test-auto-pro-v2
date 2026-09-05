package debugger_test

import (
	"strings"
	"testing"

	"test-auto-pro-v2/internal/engine/control"
	"test-auto-pro-v2/internal/model"
)

// TestF017BreakpointHitMatrix 五类断点的命中矩阵（可穷举对照）。
func TestF017BreakpointHitMatrix(t *testing.T) {
	set := control.NewBreakpointSet()
	// 默认断点：首次写 + 路径偏离。
	writeStep := control.StepFacts{StepNo: 1, NodeKey: "n1", Action: "submit", IsWriteStep: true}
	readStep := control.StepFacts{StepNo: 2, NodeKey: "n2", Action: "approve"}
	deviated := control.StepFacts{StepNo: 3, NodeKey: "nX", Action: "approve", DeviationHit: true}

	if hits := control.EvaluateBreakpointHits(writeStep, set); len(hits) != 1 || hits[0].Breakpoint.Type != model.BreakpointFirstWrite {
		t.Fatalf("写步骤应命中首次写断点：%v", hits)
	}
	if hits := control.EvaluateBreakpointHits(readStep, set); len(hits) != 0 {
		t.Fatalf("非写步骤不应命中任何默认断点：%v", hits)
	}
	if hits := control.EvaluateBreakpointHits(deviated, set); len(hits) == 0 || hits[0].Breakpoint.Type != model.BreakpointPathDeviation {
		t.Fatalf("偏离应命中路径偏离断点且为主因：%v", hits)
	}

	// 挂载节点/动作/步骤断点后各自命中。
	set.Add(control.Breakpoint{Type: model.BreakpointNode, NodeKey: "n2"})
	set.Add(control.Breakpoint{Type: model.BreakpointAction, Action: "approve"})
	set.Add(control.Breakpoint{Type: model.BreakpointStep, StepNo: 5})
	if hits := control.EvaluateBreakpointHits(readStep, set); len(hits) != 2 {
		t.Fatalf("节点与动作断点应同时命中：%v", hits)
	}
	// 第 5 步同时配置了动作断点（approve）与步骤断点：两者都命中，主因是动作。
	hits5 := control.EvaluateBreakpointHits(control.StepFacts{StepNo: 5, Action: "approve"}, set)
	if len(hits5) != 2 || hits5[0].Breakpoint.Type != model.BreakpointAction {
		t.Fatalf("步骤断点与动作断点应同时命中且动作为主因：%v", hits5)
	}
}

// TestF017MultiHitPriority 多命中时主因按固定优先级：路径偏离、首次写、动作、节点、步骤。
func TestF017MultiHitPriority(t *testing.T) {
	set := control.NewBreakpointSet()
	set.Add(control.Breakpoint{Type: model.BreakpointNode, NodeKey: "n1"})
	set.Add(control.Breakpoint{Type: model.BreakpointAction, Action: "submit"})
	set.Add(control.Breakpoint{Type: model.BreakpointStep, StepNo: 1})
	facts := control.StepFacts{StepNo: 1, NodeKey: "n1", Action: "submit", IsWriteStep: true, DeviationHit: true}
	hits := control.EvaluateBreakpointHits(facts, set)
	if len(hits) != 5 {
		t.Fatalf("应命中全部 5 类断点：%v", hits)
	}
	wantOrder := []model.BreakpointType{
		model.BreakpointPathDeviation, model.BreakpointFirstWrite,
		model.BreakpointAction, model.BreakpointNode, model.BreakpointStep,
	}
	for index, hit := range hits {
		if hit.Breakpoint.Type != wantOrder[index] {
			t.Fatalf("主因顺序错误：第 %d 位应为 %s，实际 %s", index, wantOrder[index], hit.Breakpoint.Type)
		}
	}
}

// TestF017BreakpointTargetValidation 断点只能挂在尚未执行的对象上。
func TestF017BreakpointTargetValidation(t *testing.T) {
	executedSteps := map[int]bool{1: true, 2: true}
	executedNodes := map[string]bool{"n-done": true}
	if err := control.ValidateBreakpointTarget(control.Breakpoint{Type: model.BreakpointStep, StepNo: 1}, executedSteps, executedNodes); err == nil {
		t.Fatal("对已执行步骤设断点必须被拒绝")
	} else if !strings.Contains(err.Error(), "尚未执行") {
		t.Fatalf("拒绝原因必须是中文且说明边界：%v", err)
	}
	if err := control.ValidateBreakpointTarget(control.Breakpoint{Type: model.BreakpointNode, NodeKey: "n-done"}, executedSteps, executedNodes); err == nil {
		t.Fatal("对已执行节点设断点必须被拒绝")
	}
	if err := control.ValidateBreakpointTarget(control.Breakpoint{Type: model.BreakpointStep, StepNo: 3}, executedSteps, executedNodes); err != nil {
		t.Fatalf("未执行步骤应可挂断点：%v", err)
	}
	if err := control.ValidateBreakpointTarget(control.Breakpoint{Type: model.BreakpointFirstWrite}, executedSteps, executedNodes); err != nil {
		t.Fatalf("首次写断点无挂载对象限制：%v", err)
	}
}

// TestF017ReplayBreakpointsFromFacts 当前生效断点集合由控制事实回放得出：
// 增删是事实不是改写；路径偏离断点的删除事实无效。
func TestF017ReplayBreakpointsFromFacts(t *testing.T) {
	controls := []model.RunControl{
		{Kind: model.ControlFactBreakpointSet, BreakpointType: model.BreakpointNode, ObjectKind: "node", ObjectKey: "n1"},
		{Kind: model.ControlFactBreakpointSet, BreakpointType: model.BreakpointAction, ObjectKind: "action", ObjectKey: "approve"},
		{Kind: model.ControlFactBreakpointSet, BreakpointType: model.BreakpointStep, ObjectKind: "step", ObjectKey: "3"},
		{Kind: model.ControlFactBreakpointRemove, BreakpointType: model.BreakpointNode, ObjectKind: "node", ObjectKey: "n1"},
		{Kind: model.ControlFactBreakpointRemove, BreakpointType: model.BreakpointPathDeviation},
	}
	set := control.ReplayBreakpoints(controls)
	list := set.List()
	if len(list) != 4 { // 默认两项（首次写+路径偏离，删除无效）+ 动作 + 步骤
		t.Fatalf("回放结果应为 4 个生效断点：%+v", list)
	}
	hasAction, hasNode, hasDeviation := false, false, false
	for _, bp := range list {
		switch bp.Type {
		case model.BreakpointAction:
			hasAction = true
		case model.BreakpointNode:
			if bp.NodeKey == "n1" {
				hasNode = true // 被删除的节点断点不应回放出来
			}
		case model.BreakpointPathDeviation:
			hasDeviation = true // 删除事实无效，路径偏离断点仍在
		}
	}
	if !hasAction || hasNode || !hasDeviation {
		t.Fatalf("回放结果与事实不符：hasAction=%v hasNode(已删)=%v hasDeviation=%v", hasAction, hasNode, hasDeviation)
	}
}

// TestF017CommandSets 模式与命令集映射：单步只有执行一步；自动/人工暂停时三条；偏离与终态无命令。
func TestF017CommandSets(t *testing.T) {
	cases := []struct {
		mode  model.RunMode
		state control.PauseState
		want  int
	}{
		{model.RunModeSingleStep, control.PauseStateWaiting, 1},
		{model.RunModeAuto, control.PauseStateWaiting, 3},
		{model.RunModeManual, control.PauseStateWaiting, 3},
		{model.RunModeAuto, control.PauseStateDeviation, 0},
		{model.RunModeManual, control.PauseStateUncertain, 0},
		{model.RunModeSingleStep, control.PauseStateFinished, 0},
	}
	for _, item := range cases {
		got := control.AvailableCommands(item.mode, item.state)
		if len(got) != item.want {
			t.Fatalf("%s @ %s：可用命令应为 %d 条，实际 %v", item.mode, item.state, item.want, got)
		}
	}
	if commands := control.AvailableCommands(model.RunModeSingleStep, control.PauseStateWaiting); commands[0] != model.CommandStep {
		t.Fatalf("单步运行唯一命令应是执行一步：%v", commands)
	}
}

// TestF017NextNodeBoundary 执行到下一节点的边界判定：语义节点变化即到达。
func TestF017NextNodeBoundary(t *testing.T) {
	if !control.NextNodeBoundary("n1", "n2") {
		t.Fatal("语义节点变化应判到达边界")
	}
	if control.NextNodeBoundary("n1", "n1") {
		t.Fatal("同一节点的后续步骤未到边界")
	}
}
