package run_readiness_test

import (
	"testing"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// readyPath 是一条各项都齐备的路径，用它做基线，逐类破坏来验证每一类阻塞。
func readyPath() model.ExecutionPath {
	return model.ExecutionPath{
		ID: 13, SequenceNo: 1, Name: "路径 1",
		ConfigurationStatus: model.ExecutionPathConfigurationConfigured,
		DataStatus:          model.HistoryDataStatusReady,
	}
}

// readyInput 是没有任何阻塞的输入基线：断言已配置、编译场景非空、各类复验都没问题。
func readyInput() service.PathReadinessInput {
	return service.PathReadinessInput{
		Path: readyPath(), ConfigFound: true, CompiledStepCount: 3,
		Assertion: &model.PathSuccessAssertion{
			EndNodeKey: "end-a", EndNodeName: "同意结束",
			ExpectedStatus: model.FlowInstanceStatusEnd, ArrivalOrdinal: 1,
		},
	}
}

// TestReadyPathHasNoBlocks 验证基线路径可以启动，并给出一句中文结论。
func TestReadyPathHasNoBlocks(t *testing.T) {
	readiness := service.EvaluatePathReadiness(readyInput())
	if !readiness.Runnable || len(readiness.Blocks) != 0 {
		t.Fatalf("基线路径应可启动：%+v", readiness)
	}
	if readiness.Summary == "" || readiness.PathName != "路径 1" {
		t.Fatalf("路径结论缺少中文说明：%+v", readiness)
	}
}

// TestEachBlockSourceIsReported 验证十类阻塞每一类都能单独触发，且都带中文原因与可点击锚点。
func TestEachBlockSourceIsReported(t *testing.T) {
	issue := []model.PathConfigAffectedItem{{Kind: "x", Name: "节点 A", Reason: "解析不出唯一真实处理人"}}
	cases := map[string]struct {
		mutate   func(*service.PathReadinessInput)
		wantKind string
	}{
		"节点配置未完成": {func(in *service.PathReadinessInput) {
			in.Path.ConfigurationStatus = "pending"
			in.Path.ConfigurationDetail = "还有节点没有配置动作"
		}, model.RunReadinessNodeConfiguration},
		"基础表单数据未就绪": {func(in *service.PathReadinessInput) {
			in.Path.DataStatus = model.HistoryDataStatusNeedsInput
			in.Path.DataDetail = "表单数据需要人工确认"
		}, model.RunReadinessFormData},
		"配置已记录的问题": {func(in *service.PathReadinessInput) { in.ConfigIssues = issue }, model.RunReadinessConfigIssue},
		"人员解析不唯一":  {func(in *service.PathReadinessInput) { in.PersonIssues = issue }, model.RunReadinessPersonNotUnique},
		"路径拓扑已变":   {func(in *service.PathReadinessInput) { in.TopologyIssues = issue }, model.RunReadinessTopologyChanged},
		"成功断言缺失":   {func(in *service.PathReadinessInput) { in.Assertion = nil }, model.RunReadinessAssertionMissing},
		"成功断言失效": {func(in *service.PathReadinessInput) { in.AssertionIssues = issue },
			model.RunReadinessAssertionInvalid},
		"编译场景为空": {func(in *service.PathReadinessInput) { in.CompiledStepCount = 0 },
			model.RunReadinessCompiledScenarioEmpty},
		"动作尚未验证可执行": {func(in *service.PathReadinessInput) {
			in.ConfiguredActions = []model.ActionKey{model.ActionSubmit}
		}, model.RunReadinessActionNotVerified},
		"语义条目待实测": {func(in *service.PathReadinessInput) {
			in.PendingSemanticsEntries = []string{"回退语义"}
		}, model.RunReadinessSemanticsPending},
	}
	for name, testCase := range cases {
		input := readyInput()
		testCase.mutate(&input)
		readiness := service.EvaluatePathReadiness(input)
		if readiness.Runnable {
			t.Fatalf("%s 时不应判为可启动：%+v", name, readiness)
		}
		found := false
		for _, block := range readiness.Blocks {
			if block.Kind != testCase.wantKind {
				continue
			}
			found = true
			if block.Reason == "" || block.Anchor == "" {
				t.Fatalf("%s 的阻塞缺少中文原因或锚点：%+v", name, block)
			}
		}
		if !found {
			t.Fatalf("%s 没有报出 %s 类阻塞：%+v", name, testCase.wantKind, readiness.Blocks)
		}
	}
}

// TestRemindersNeverBlock 验证提醒不影响能否启动，也不会被混进阻塞列表。
func TestRemindersNeverBlock(t *testing.T) {
	input := readyInput()
	input.Reminders = []model.RunReadinessItem{
		{Kind: model.RunReadinessDeploymentNotice, Name: "审批方式迁移", Reason: "目标环境已执行 20260828 迁移"},
		{Kind: model.RunReadinessPlanNotice, Name: "计划提示", Reason: "计划已固化为串行运行"},
	}
	readiness := service.EvaluatePathReadiness(input)
	if !readiness.Runnable {
		t.Fatalf("只有提醒时必须仍可启动：%+v", readiness)
	}
	if len(readiness.Blocks) != 0 {
		t.Fatalf("提醒不得混进阻塞：%+v", readiness.Blocks)
	}
	if len(readiness.Reminders) != 2 {
		t.Fatalf("提醒条目丢失：%+v", readiness.Reminders)
	}
}

// TestVerifiedRunnableActionsIsEmptyBeforeFirstRealWrite 锁定纲领第 9 节的要求：
// F-016 之前没有任何动作被真实写验证过，因此已配置动作的路径必须直接阻塞，不做静默降级。
func TestVerifiedRunnableActionsIsEmptyBeforeFirstRealWrite(t *testing.T) {
	for _, action := range []model.ActionKey{
		model.ActionSubmit, model.ActionApprove, model.ActionReject, model.ActionWithdraw,
	} {
		if service.IsVerifiedRunnableAction(action) {
			t.Fatalf("F-016 之前不应有任何已验证动作：%s", action)
		}
	}
	input := readyInput()
	input.ConfiguredActions = []model.ActionKey{model.ActionSubmit, model.ActionSubmit, model.ActionApprove}
	readiness := service.EvaluatePathReadiness(input)
	blocked := 0
	for _, block := range readiness.Blocks {
		if block.Kind == model.RunReadinessActionNotVerified {
			blocked++
		}
	}
	if blocked != 2 {
		t.Fatalf("重复动作应去重后逐个报出，期望 2 条实际 %d 条：%+v", blocked, readiness.Blocks)
	}
}

// TestAggregatePlanReadinessSummarizesAndSorts 验证计划级聚合按路径序号排序并给出一句中文总结论。
func TestAggregatePlanReadinessSummarizesAndSorts(t *testing.T) {
	blockedInput := readyInput()
	blockedInput.Path.ID, blockedInput.Path.SequenceNo, blockedInput.Path.Name = 14, 2, "路径 2"
	blockedInput.Assertion = nil
	plan := service.AggregatePlanReadiness([]model.PathRunReadiness{
		service.EvaluatePathReadiness(blockedInput),
		service.EvaluatePathReadiness(readyInput()),
	})
	if plan.TotalCount != 2 || plan.RunnableCount != 1 || plan.BlockedCount != 1 {
		t.Fatalf("计划级计数不正确：%+v", plan)
	}
	if plan.Paths[0].SequenceNo != 1 || plan.Paths[1].SequenceNo != 2 {
		t.Fatalf("路径没有按序号排序：%+v", plan.Paths)
	}
	if plan.Summary == "" {
		t.Fatal("计划级结论缺少中文总结")
	}
	empty := service.AggregatePlanReadiness(nil)
	if empty.Summary == "" || empty.TotalCount != 0 {
		t.Fatalf("没有路径时也要给出中文结论：%+v", empty)
	}
}
