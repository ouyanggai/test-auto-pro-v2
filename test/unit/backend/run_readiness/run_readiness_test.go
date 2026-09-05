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
		"编译场景为空": {func(in *service.PathReadinessInput) { in.CompiledStepCount = 0 },
			model.RunReadinessCompiledScenarioEmpty},
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

// TestVerifiedRunnableActionsMatchesRealWriteEvidence 锁定纲领第 9 节的要求：
// 已验证动作集合必须与真实写证据一一对应。F-016 运行 8 登记了提交；
// 同意、驳回与撤回尚未在真实目标执行过，必须保持未登记，已配置动作的路径只提醒不阻塞。
func TestVerifiedRunnableActionsMatchesRealWriteEvidence(t *testing.T) {
	if !service.IsVerifiedRunnableAction(model.ActionSubmit) {
		t.Fatal("提交已按运行 8 的真实写证据登记，不得为空集")
	}
	for _, action := range []model.ActionKey{
		model.ActionApprove, model.ActionReject, model.ActionWithdraw,
	} {
		if service.IsVerifiedRunnableAction(action) {
			t.Fatalf("该动作尚未在真实目标执行过，不得登记：%s", action)
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
	if blocked != 0 {
		t.Fatalf("动作未验证只应提醒不应阻塞：%+v", readiness.Blocks)
	}
	reminded := 0
	for _, reminder := range readiness.Reminders {
		if reminder.Kind == model.RunReadinessActionNotVerified {
			reminded++
		}
	}
	// 提交已登记不再提醒；未登记的只有同意（重复出现已去重）。
	if reminded != 1 {
		t.Fatalf("未验证动作应去重后逐个提醒，期望 1 条实际 %d 条：%+v", reminded, readiness.Reminders)
	}
}

// TestAggregatePlanReadinessSummarizesAndSorts 验证计划级聚合按路径序号排序并给出一句中文总结论。
func TestAggregatePlanReadinessSummarizesAndSorts(t *testing.T) {
	blockedInput := readyInput()
	blockedInput.Path.ID, blockedInput.Path.SequenceNo, blockedInput.Path.Name = 14, 2, "路径 2"
	blockedInput.CompiledStepCount = 0
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

// TestNonActionableFactsAreRemindersNotBlocks 锁定人工验收反馈：
// 用户在预检里处理不了的事一律只提醒、不阻塞——否则会形成"要先能跑才能验证、阻塞又不让跑"的死锁。
// 未指定成功节点同样不阻塞：默认口径就是把流程走完。
func TestNonActionableFactsAreRemindersNotBlocks(t *testing.T) {
	cases := map[string]struct {
		mutate   func(*service.PathReadinessInput)
		wantKind string
	}{
		"动作尚未被真实写验证过": {func(in *service.PathReadinessInput) {
			// 提交已按 F-016 运行 8 登记为已验证；未登记的同意动作才会触发本提醒。
			in.ConfiguredActions = []model.ActionKey{model.ActionApprove}
		}, model.RunReadinessActionNotVerified},
		"目标行为尚未实测勘定": {func(in *service.PathReadinessInput) {
			in.PendingSemanticsEntries = []string{"回退语义"}
		}, model.RunReadinessSemanticsPending},
		"配置里的说明性提示": {func(in *service.PathReadinessInput) {
			in.ConfigNotices = []model.PathConfigAffectedItem{{Reason: "表单校验会在打开表单数据页时完成"}}
		}, model.RunReadinessConfigIssue},
	}
	for name, testCase := range cases {
		input := readyInput()
		testCase.mutate(&input)
		readiness := service.EvaluatePathReadiness(input)
		if !readiness.Runnable {
			t.Fatalf("%s 不应阻塞启动：%+v", name, readiness.Blocks)
		}
		found := false
		for _, reminder := range readiness.Reminders {
			if reminder.Kind == testCase.wantKind && reminder.Reason != "" {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s 没有作为提醒报出：%+v", name, readiness.Reminders)
		}
	}
}

// TestConfigUnreadableBlocksInsteadOfPassing 锁定人工复审的 P1：路径配置读取失败必须阻塞。
// 路径摘要恰好是「配置完成、数据就绪」时，如果把读失败当成「没有配置记录」，
// 这条其实无法判断的路径会被判成可以运行，这正是数据库故障时最危险的方向。
func TestConfigUnreadableBlocksInsteadOfPassing(t *testing.T) {
	input := readyInput()
	input.ConfigFound = false
	input.ConfigUnreadable = true
	input.CompiledStepCount = 0

	readiness := service.EvaluatePathReadiness(input)
	if readiness.Runnable {
		t.Fatalf("配置读取失败时不得判为可运行：%+v", readiness)
	}
	found := false
	for _, block := range readiness.Blocks {
		if block.Kind != model.RunReadinessConfigUnreadable {
			continue
		}
		found = true
		if block.Reason == "" || block.Anchor == "" {
			t.Fatalf("读取失败阻塞缺少中文原因或锚点：%+v", block)
		}
	}
	if !found {
		t.Fatalf("缺少配置读取失败这一类阻塞：%+v", readiness.Blocks)
	}
}

// TestConfigMissingIsNotReportedAsUnreadable 区分「确实还没配」与「读不到」两种含义，
// 避免修复 P1 时把没有配置记录的正常路径也标成读取失败。
func TestConfigMissingIsNotReportedAsUnreadable(t *testing.T) {
	input := readyInput()
	input.ConfigFound = false
	input.CompiledStepCount = 0

	for _, block := range service.EvaluatePathReadiness(input).Blocks {
		if block.Kind == model.RunReadinessConfigUnreadable {
			t.Fatalf("没有配置记录被误报为读取失败：%+v", block)
		}
	}
}
