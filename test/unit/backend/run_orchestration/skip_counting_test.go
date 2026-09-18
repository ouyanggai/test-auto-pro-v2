package run_orchestration_test

import (
	"reflect"
	"testing"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// TestF036SkippedStepsSeparatedFromExecuted 锁定 T1 门禁 G8 的计数口径：
// 含目标自动跳过步的运行，聚合结果必须把「真实执行成功」与「目标自动跳过」分开计数，
// 并逐个暴露跳过步编号——run 2 那种「第 20 步没有 submit 行」的形态在新代码下
// 绝不能被表述成「20/20 步全部成功」。
func TestF036SkippedStepsSeparatedFromExecuted(t *testing.T) {
	// 复刻 run 2 的真实形态：18 步真实执行成功、第 19 步成功、第 20 步被目标自动跳过。
	steps := []model.RunStep{}
	for stepNo := 1; stepNo <= 19; stepNo++ {
		steps = append(steps, model.RunStep{StepNo: stepNo, Status: model.RunStepSucceeded})
	}
	steps = append(steps, model.RunStep{StepNo: 20, Status: model.RunStepSkipped})

	doneSteps, executedSteps, skippedStepNos := service.CountStepOutcomesForTest(steps)

	if doneSteps != 20 {
		t.Fatalf("进度分子应含跳过步，期望 20，实际 %d", doneSteps)
	}
	if executedSteps != 19 {
		t.Fatalf("真实执行成功步数应为 19（跳过步不得计入），实际 %d", executedSteps)
	}
	if !reflect.DeepEqual(skippedStepNos, []int{20}) {
		t.Fatalf("跳过步编号必须逐个暴露，期望 [20]，实际 %v", skippedStepNos)
	}
	// 核心断言：两类事实之和等于进度分子，且二者不相等——
	// 只要相等就说明跳过步被吞进了「全部成功」。
	if executedSteps+len(skippedStepNos) != doneSteps {
		t.Fatalf("真实执行 %d + 跳过 %d 应等于进度分子 %d", executedSteps, len(skippedStepNos), doneSteps)
	}
	if executedSteps == doneSteps {
		t.Fatalf("存在跳过步时真实执行步数不得等于进度分子，否则跳过步被伪装成成功")
	}
}

// TestF036NoSkippedStepsKeepsPlainCounting 锁定无跳过步时口径不变：
// 全部真实成功时两个计数相等，且跳过清单为空（不是 nil 之外的哨兵值）。
func TestF036NoSkippedStepsKeepsPlainCounting(t *testing.T) {
	steps := []model.RunStep{
		{StepNo: 1, Status: model.RunStepSucceeded},
		{StepNo: 2, Status: model.RunStepSucceeded},
	}

	doneSteps, executedSteps, skippedStepNos := service.CountStepOutcomesForTest(steps)

	if doneSteps != 2 || executedSteps != 2 {
		t.Fatalf("全部真实成功时两个计数都应为 2，实际 done=%d executed=%d", doneSteps, executedSteps)
	}
	if len(skippedStepNos) != 0 {
		t.Fatalf("无跳过步时清单应为空，实际 %v", skippedStepNos)
	}
}

// TestF036FailedAndUncertainStepsCountAsNeither 锁定失败与不确定步骤不进入任何成功计数：
// 否则失败步会被进度数字掩盖，把未完成的运行显示成已完成。
func TestF036FailedAndUncertainStepsCountAsNeither(t *testing.T) {
	steps := []model.RunStep{
		{StepNo: 1, Status: model.RunStepSucceeded},
		{StepNo: 2, Status: model.RunStepSkipped},
		{StepNo: 3, Status: model.RunStepFailed},
		{StepNo: 4, Status: model.RunStepUncertain},
	}

	doneSteps, executedSteps, skippedStepNos := service.CountStepOutcomesForTest(steps)

	if doneSteps != 2 || executedSteps != 1 {
		t.Fatalf("只有成功与跳过进入进度分子，期望 done=2 executed=1，实际 done=%d executed=%d", doneSteps, executedSteps)
	}
	if !reflect.DeepEqual(skippedStepNos, []int{2}) {
		t.Fatalf("跳过清单应只含第 2 步，实际 %v", skippedStepNos)
	}
}
