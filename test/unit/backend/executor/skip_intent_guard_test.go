// F-036 第三轮 T7：路径上配置了动作的节点不允许被跳过判定吞掉。
//
// 背景（T7.1 结论）：第三轮复核 P0-1 依据「run 2 第 20 步没有 submit 行」判定该步未执行。
// 复核该结论不正确——实测 step.log 中 step_id=20 完整具备
// plan/gate/control/prepare/submit/verify/settle 七个阶段，其中 phase=prepare 与 phase=submit
// 各有两行（会话就绪、协议摘要、runtime 收敛、目标接口返回成功），是一次真实执行的审批。
// 复核把「step 20 的 targetNodeId 与前一步解析出的当前节点相同」误读成未执行，
// 而前一步（step 19）是 system_navigation 只读导航步，本就不改变节点。
//
// 因此 T7.1 的答案属于「每一步都按各自节点真实执行」，不是「跳过判定放行」。
// 本文件锁定 T7.2 要防的那类真实风险：跳过判定绝不能把用户明确配置的写动作静默丢弃。
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

// TestF036ConfiguredActionNotSwallowedBySkipDecision 锁定 T7.2 的核心不变式：
// 本步是用户明确配置的写动作、且目标待办已越过本节点（跳过分型成立）时，
// 执行器必须阻塞而不是返回跳过预览——绝不静默丢弃用户要求的动作。
func TestF036ConfiguredActionNotSwallowedBySkipDecision(t *testing.T) {
	runCtx := f036SkipIntentRunContext(model.ActionStepSourceUser)
	// 目标事实：实例待办已越过 early 节点、落在路径后方的 late 节点。
	fake := &fakeTarget{instance: fakeTargetView{
		Found: true, Status: "run",
		CurrentNodes: []string{"late-target"}, DueNodes: []string{"late-target"},
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
	// 核心断言：用户配置的动作不得被跳过放行。
	if preview.TargetSkipped {
		t.Fatalf("用户明确配置的动作不得被跳过判定吞掉：%+v", preview)
	}
	if preview.GateAllowed {
		t.Fatalf("应降级为阻塞而不是放行：%+v", preview)
	}
	if !strings.Contains(preview.BlockReason, "配置了动作") {
		t.Fatalf("阻塞原因必须点明「本节点配置了动作」：%q", preview.BlockReason)
	}
	if !strings.Contains(preview.BlockReason, "跳过") {
		t.Fatalf("阻塞原因必须点明目标跳过判定：%q", preview.BlockReason)
	}
	// 阻塞必须是「门禁阻塞」分类，不能被当成工具缺陷。
	if preview.BlockFailureClass != model.FailureClassGateBlocked {
		t.Fatalf("应为门禁阻塞分类，实际 %q", preview.BlockFailureClass)
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
