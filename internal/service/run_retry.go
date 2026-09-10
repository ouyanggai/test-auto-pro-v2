package service

import (
	"fmt"

	"test-auto-pro-v2/internal/model"
)

// RetryPlan 是重试装填前的校验结论（F-028 失败动作重试）；由校验函数产出、控制装填消费。
type RetryPlan struct {
	// StepNo 是失败步骤的序号（已落账事实里的最大步骤号）。
	StepNo int
	// CursorIndex 是失败步骤在重编译场景中的 0 基下标，重试预览从这里构建。
	CursorIndex int
	// ExecutedStepNos/ExecutedNodeKeys 是已成功步骤的序号与节点键集合，供控制现场
	// 做断点挂载校验与防重复执行护栏；失败步骤不在其中，放行才能再次执行它。
	ExecutedStepNos  map[int]bool
	ExecutedNodeKeys map[string]bool
}

// planFailedStepRetry 校验一次失败运行能否按当前编译场景重试（F-028）。
// compiledSteps 是按当前 user_actions 重编译的场景，totalSteps 是启动时冻结的总步骤数，
// factRows 是该路径运行按步骤序号升序的全部已落账步骤事实。
// 校验三件事：事实指向一个明确的失败步骤；已成功前缀与重编译场景逐步一致（动作、节点、来源）；
// 失败步骤本身也未被改配。冻结总步数不一致同样视为配置已变化。
// 任何不一致都意味着失败后改过配置，重试会执行与原运行不同的场景，必须拒绝并引导重新发起运行。
func planFailedStepRetry(compiledSteps []model.CompiledActionStep, totalSteps *int, factRows []model.RunStep) (RetryPlan, error) {
	if len(compiledSteps) == 0 {
		return RetryPlan{}, fmt.Errorf("编译场景为空，无法重试；请先完成动作编排")
	}
	if len(factRows) == 0 {
		return RetryPlan{}, fmt.Errorf("本次失败发生在启动阶段，没有已执行的动作可以重试；请从计划重新发起运行")
	}
	// 启动时冻结了总步骤数：重编译场景长度与冻结值不一致即配置已变化，
	// 不允许重试后半份场景——那会执行一条与原运行不同的路径。
	if totalSteps != nil && *totalSteps != len(compiledSteps) {
		return RetryPlan{}, fmt.Errorf("动作配置在失败后已被修改，无法继续本次运行；请从计划重新发起运行")
	}
	// 已落账事实里最大步骤号就是失败步骤：它之前的步骤必须全部成功，它自身的行必须全部失败。
	failedStepNo := 0
	succeededByStepNo := map[int]model.RunStep{}
	for _, row := range factRows {
		if row.StepNo > failedStepNo {
			failedStepNo = row.StepNo
		}
		if row.Status == model.RunStepSucceeded {
			succeededByStepNo[row.StepNo] = row
		}
	}
	for _, row := range factRows {
		if row.StepNo < failedStepNo && row.Status != model.RunStepSucceeded && succeededByStepNo[row.StepNo].ID == 0 {
			return RetryPlan{}, fmt.Errorf("运行事实不完整（第 %d 步没有成功记录），无法重试；请从计划重新发起运行", row.StepNo)
		}
	}
	failedRows := 0
	for _, row := range factRows {
		if row.StepNo == failedStepNo {
			failedRows++
			if row.Status != model.RunStepFailed {
				return RetryPlan{}, fmt.Errorf("运行事实不完整（第 %d 步状态不一致），无法重试；请从计划重新发起运行", failedStepNo)
			}
		}
	}
	if failedRows == 0 {
		return RetryPlan{}, fmt.Errorf("运行事实不完整（找不到失败步骤的记录），无法重试；请从计划重新发起运行")
	}
	// 失败步骤必须在重编译场景中，且已成功前缀与失败步骤本身都要与场景逐步一致；
	// 逐项比对动作、节点与来源，改过任何一处都视为配置已变化。
	cursorIndex := -1
	for index, compiled := range compiledSteps {
		if compiled.Sequence == failedStepNo {
			cursorIndex = index
			break
		}
	}
	if cursorIndex < 0 {
		return RetryPlan{}, fmt.Errorf("动作配置在失败后已被修改，无法继续本次运行；请从计划重新发起运行")
	}
	stepMatchesFact := func(compiled model.CompiledActionStep, row model.RunStep) bool {
		return compiled.Sequence == row.StepNo &&
			string(compiled.Action) == row.Action &&
			compiled.NodeKey == row.NodeKey &&
			string(compiled.Source) == row.Source
	}
	for index := 0; index < cursorIndex; index++ {
		row, ok := succeededByStepNo[compiledSteps[index].Sequence]
		if !ok || !stepMatchesFact(compiledSteps[index], row) {
			return RetryPlan{}, fmt.Errorf("动作配置在失败后已被修改，无法继续本次运行；请从计划重新发起运行")
		}
	}
	for _, row := range factRows {
		if row.StepNo == failedStepNo && !stepMatchesFact(compiledSteps[cursorIndex], row) {
			return RetryPlan{}, fmt.Errorf("动作配置在失败后已被修改，无法继续本次运行；请从计划重新发起运行")
		}
	}
	plan := RetryPlan{
		StepNo:           failedStepNo,
		CursorIndex:      cursorIndex,
		ExecutedStepNos:  map[int]bool{},
		ExecutedNodeKeys: map[string]bool{},
	}
	for index := 0; index < cursorIndex; index++ {
		plan.ExecutedStepNos[compiledSteps[index].Sequence] = true
		plan.ExecutedNodeKeys[compiledSteps[index].NodeKey] = true
	}
	return plan, nil
}

// PlanFailedStepRetryForTest 把 planFailedStepRetry 暴露给定向单测：测试与产品走同一份校验规则。
func PlanFailedStepRetryForTest(compiledSteps []model.CompiledActionStep, totalSteps *int, factRows []model.RunStep) (RetryPlan, error) {
	return planFailedStepRetry(compiledSteps, totalSteps, factRows)
}
