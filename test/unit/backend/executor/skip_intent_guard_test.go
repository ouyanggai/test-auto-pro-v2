// F-036 第三轮 T7：目标已自动越过无处理人节点时，节点上的配置动作随节点整体跳过。
// 用户保存过动作不代表目标仍有可执行待办；此时既不能凭空补处理人，也不能再发写请求。
package executor_test

import (
	"context"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// f036SkipIntentRunContext 构造两步路径：本步在 early 节点，目标待办已越过到 late 节点，
// 模板声明 isSkip=true（即跳过分型会成立）。source 决定本步是否为用户明确配置。
func f036SkipIntentRunContext(source model.ActionStepSource) step.RunContext {
	isSkip := true
	return step.RunContext{
		Run:         model.Run{ID: 1, PlanID: 1, RunNo: 1},
		PathRun:     model.PathRun{ID: 11, RunID: 1, Status: model.PathRunStatusRunning, MainInstanceRef: "instance-9"},
		PathName:    "路径 1",
		PlanAccount: "oyg-test",
		FlowProxyID: "flow-proxy-1",
		Source:      "new",
		Nodes: map[string]step.NodeInfo{
			"early": {Name: "已配置动作节点", Type: "common", TargetNodeID: "early-target", IsSkip: &isSkip, AuditType: "company"},
			"late":  {Name: "后方节点", Type: "common", TargetNodeID: "late-target"},
		},
		Steps: []model.CompiledActionStep{
			{Sequence: 1, Source: source, Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: "early"},
			{Sequence: 2, Source: model.ActionStepSourceUser, Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: "late"},
		},
		EffectiveFormData: []byte(`{"amount":"12.30"}`),
	}
}

// TestF036ConfiguredActionFollowsTargetSkip 锁定目标平台的节点级语义：
// 用户明确配置的写动作、且目标待办已越过本节点（isSkip=true）时，
// 执行器必须返回可放行的跳过预览，不发任何写请求。
func TestF036ConfiguredActionFollowsTargetSkip(t *testing.T) {
	runCtx := f036SkipIntentRunContext(model.ActionStepSourceUser)
	// 目标事实：实例待办已越过 early 节点、落在路径后方的 late 节点。
	fake := &fakeTarget{instance: fakeTargetView{
		Found: true, Status: "run",
		// 目标已把实例推进到后续节点，但当前节点没有生成待办；这正是不能先进入处理人轮询的形状。
		CurrentNodes: []string{"late-target"},
	}}
	executor := step.NewExecutor(fake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	// 前置条件：跳过分型本身成立（目标确实越过了本节点）。
	action, _ := step.TargetSkippedClassificationForTest(runCtx, runCtx.Steps[0], step.InstanceFacts{
		Found: true, Status: "run", CurrentNodes: []string{"late-target"}, DueNodes: []string{"late-target"},
	})
	if action != "skip" {
		t.Fatalf("前置条件不成立：本场景应被跳过分型判为 skip，实际 %q", action)
	}

	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if preview == nil {
		t.Fatal("预览不应为空")
	}
	if !preview.TargetSkipped {
		t.Fatalf("用户配置动作应随目标节点整体跳过：%+v", preview)
	}
	if !preview.GateAllowed {
		t.Fatalf("目标跳过预览应放行：%+v", preview)
	}
	if strings.TrimSpace(preview.BlockReason) != "" {
		t.Fatalf("跳过预览不应带阻塞原因：%q", preview.BlockReason)
	}
	if !strings.Contains(preview.SkipReason, "没有可执行的审批动作") {
		t.Fatalf("跳过原因必须说明节点动作未执行：%q", preview.SkipReason)
	}
	if fake.auditCalls != 0 || fake.actionCalls != 0 || fake.submitCalls != 0 {
		t.Fatalf("目标已跳过时不得发任何写请求：audit=%d action=%d submit=%d", fake.auditCalls, fake.actionCalls, fake.submitCalls)
	}
}

// TestF036SystemDefaultStepStillSkippable 锁定约束范围：
// 系统默认步（非用户明确配置）在目标确实越过时仍可跳过，
// 避免修复把 F-034 已验收的跳过能力一并取消。
func TestF036SystemDefaultStepStillSkippable(t *testing.T) {
	runCtx := f036SkipIntentRunContext(model.ActionStepSourceSystemDefault)
	fake := &fakeTarget{instance: fakeTargetView{
		Found: true, Status: "run",
		CurrentNodes: []string{"late-target"}, DueNodes: []string{"late-target"},
	}}
	executor := step.NewExecutor(fake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if preview == nil {
		t.Fatal("预览不应为空")
	}
	if !preview.TargetSkipped {
		t.Fatalf("系统默认步在目标越过时仍应可跳过：%+v", preview)
	}
	if !preview.GateAllowed {
		t.Fatalf("跳过预览应放行：%+v", preview)
	}
	if strings.TrimSpace(preview.SkipReason) == "" {
		t.Fatalf("跳过必须带目标事实原因：%+v", preview)
	}
}

// TestF036ConfiguredActionAtPathEndExecutes 锁定 T7.3 的组合之一：
// 路径最后一个节点已配置动作、且目标待办仍在该节点时，本步必须走正常执行路径
// （不判跳过、正常放行），证明「末节点已配置动作」不会被跳过判定吞掉。
func TestF036ConfiguredActionAtPathEndExecutes(t *testing.T) {
	runCtx := f036SkipIntentRunContext(model.ActionStepSourceUser)
	// 目标待办就在本步节点上（未越过）：这是最典型的末节点待执行形态。
	fake := &fakeTarget{
		instance:      fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"early-target"}, DueNodes: []string{"early-target"}},
		handlerUserID: "user-2", handlerAccount: "lisi",
	}
	executor := step.NewExecutor(fake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if preview == nil {
		t.Fatal("预览不应为空")
	}
	if preview.TargetSkipped {
		t.Fatalf("待办仍在本节点时不得判跳过：%+v", preview)
	}
	// 本步应进入正常门禁流程；是否放行取决于门禁条件，但绝不能是跳过。
	if preview.Action != model.ActionApprove {
		t.Fatalf("末节点动作应保持为配置的同意动作，实际 %q", preview.Action)
	}
	if strings.Contains(preview.BlockReason, "跳过") {
		t.Fatalf("待办在本节点时阻塞原因不应提到跳过：%q", preview.BlockReason)
	}
}

// TestF036ConfiguredActionWhenInstanceEndedBlocks 锁定 T7.3 的组合之二：
// 路径最后一个节点已配置动作、而目标实例已到 end 时，本步不得被判成「目标自动跳过」，
// 必须以可解释的阻塞或失败收口，让运行报告不会出现「成功但没执行」的步。
func TestF036ConfiguredActionWhenInstanceEndedBlocks(t *testing.T) {
	runCtx := f036SkipIntentRunContext(model.ActionStepSourceUser)
	// 实例已结束：当前节点仍报本节点，但状态是 end。
	fake := &fakeTarget{instance: fakeTargetView{
		Found: true, Status: "end", CurrentNodes: []string{"early-target"}, DueNodes: []string{},
	}}
	executor := step.NewExecutor(fake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if preview == nil {
		t.Fatal("预览不应为空")
	}
	// 实例已结束时，用户配置的动作绝不能被跳过放行成「目标自动跳过」。
	if preview.TargetSkipped {
		t.Fatalf("实例已 end 时不得把用户配置的动作判成目标自动跳过：%+v", preview)
	}
	if preview.GateAllowed {
		t.Fatalf("实例已 end 且无待办时不应直接放行写请求：%+v", preview)
	}
	if strings.TrimSpace(preview.BlockReason) == "" {
		t.Fatalf("必须给出可解释的阻塞原因：%+v", preview)
	}
}
