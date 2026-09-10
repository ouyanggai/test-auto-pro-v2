package run_orchestration_test

import (
	"strings"
	"testing"

	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// f028CompiledSteps 构造两步编译场景：第 1 步用户提交（发起人节点），第 2 步导航。
func f028CompiledSteps() []model.CompiledActionStep {
	startToken := analyzer.PathConfigNodeToken("node-start")
	auditToken := analyzer.PathConfigNodeToken("node-audit")
	return []model.CompiledActionStep{
		{Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionSubmit,
			Scope: model.ActionScopeInitiator, NodeKey: startToken},
		{Sequence: 2, Source: model.ActionStepSourceNavigation, Action: model.ActionSystemAutomatic,
			Scope: model.ActionScopeTask, NodeKey: auditToken},
	}
}

// f028FactRows 构造已落账事实：第 1 步成功、第 2 步确定失败（与场景一致）。
func f028FactRows() []model.RunStep {
	startToken := analyzer.PathConfigNodeToken("node-start")
	auditToken := analyzer.PathConfigNodeToken("node-audit")
	return []model.RunStep{
		{ID: 1, StepNo: 1, Source: string(model.ActionStepSourceUser), Action: string(model.ActionSubmit),
			NodeKey: startToken, Status: model.RunStepSucceeded},
		{ID: 2, StepNo: 2, Source: string(model.ActionStepSourceNavigation), Action: string(model.ActionSystemAutomatic),
			NodeKey: auditToken, Status: model.RunStepFailed},
	}
}

// TestF028RetryPlanArmsCursorAtFailedStep 锁定重试装填的基本结论：
// 游标落在失败步骤、已执行集合只含成功前缀（失败步骤绝不能进护栏集合，否则放行被误拒）。
func TestF028RetryPlanArmsCursorAtFailedStep(t *testing.T) {
	compiled := f028CompiledSteps()
	total := 2
	plan, err := service.PlanFailedStepRetryForTest(compiled, &total, f028FactRows())
	if err != nil {
		t.Fatalf("一致的场景与事实应允许重试：%v", err)
	}
	if plan.StepNo != 2 || plan.CursorIndex != 1 {
		t.Fatalf("游标应装填在第 2 步（下标 1），实际 stepNo=%d cursorIndex=%d", plan.StepNo, plan.CursorIndex)
	}
	if !plan.ExecutedStepNos[1] || plan.ExecutedStepNos[2] {
		t.Fatalf("已执行集合应只含第 1 步，实际 %v", plan.ExecutedStepNos)
	}
}

// TestF028RetryPlanRejectsChangedConfig 锁定前缀一致性校验：
// 失败后改过配置（总步数、前缀动作、失败步骤本身）一律拒绝重试并给出「配置已变化」结论。
func TestF028RetryPlanRejectsChangedConfig(t *testing.T) {
	total := 2

	// 冻结总步数与重编译场景长度不一致：配置已变化。
	changedTotal := 3
	if _, err := service.PlanFailedStepRetryForTest(f028CompiledSteps(), &changedTotal, f028FactRows()); err == nil {
		t.Fatalf("总步数变化必须拒绝重试")
	} else if !strings.Contains(err.Error(), "配置") {
		t.Fatalf("拒绝结论应说明配置已变化，实际 %v", err)
	}

	// 前缀动作被改：第 1 步从提交变成保存草稿。
	compiled := f028CompiledSteps()
	compiled[0].Action = model.ActionSaveDraft
	if _, err := service.PlanFailedStepRetryForTest(compiled, &total, f028FactRows()); err == nil {
		t.Fatalf("前缀动作变化必须拒绝重试")
	}

	// 失败步骤被改：第 2 步节点换到别的节点。
	recompiled := f028CompiledSteps()
	recompiled[1].NodeKey = analyzer.PathConfigNodeToken("node-other")
	if _, err := service.PlanFailedStepRetryForTest(recompiled, &total, f028FactRows()); err == nil {
		t.Fatalf("失败步骤变化必须拒绝重试")
	}

	// 失败步骤不在场景中（被删除）。
	shortened := f028CompiledSteps()[:1]
	if _, err := service.PlanFailedStepRetryForTest(shortened, &total, f028FactRows()); err == nil {
		t.Fatalf("失败步骤被删除必须拒绝重试")
	}
}

// TestF028RetryPlanRejectsStartupFailureAndBrokenFacts 锁定范围边界与事实完整性：
// 启动阶段失败（无任何已落账步骤）不在重试范围；前缀缺成功记录、失败步骤缺失败记录都拒绝。
func TestF028RetryPlanRejectsStartupFailureAndBrokenFacts(t *testing.T) {
	total := 2

	if _, err := service.PlanFailedStepRetryForTest(f028CompiledSteps(), &total, nil); err == nil {
		t.Fatalf("启动阶段失败（无事实行）必须拒绝重试")
	} else if !strings.Contains(err.Error(), "启动阶段") {
		t.Fatalf("拒绝结论应指出失败发生在启动阶段，实际 %v", err)
	}

	// 前缀第 1 步只有失败记录、没有成功记录：事实不完整。
	broken := f028FactRows()
	broken[0].Status = model.RunStepFailed
	if _, err := service.PlanFailedStepRetryForTest(f028CompiledSteps(), &total, broken); err == nil {
		t.Fatalf("前缀缺成功记录必须拒绝重试")
	}

	// 失败步骤的记录不是失败结论：事实与失败态矛盾。
	misleading := f028FactRows()
	misleading[1].Status = model.RunStepSucceeded
	if _, err := service.PlanFailedStepRetryForTest(f028CompiledSteps(), &total, misleading); err == nil {
		t.Fatalf("失败步骤无失败记录必须拒绝重试")
	}
}
