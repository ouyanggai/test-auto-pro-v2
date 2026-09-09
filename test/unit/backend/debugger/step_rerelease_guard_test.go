package debugger_test

import (
	"errors"
	"testing"

	"test-auto-pro-v2/internal/engine/control"
	"test-auto-pro-v2/internal/model"
)

// TestStepReReleaseGuardBlocksDuplicateRelease 锁定单步放行的防重复执行护栏：
// 真实运行中，下一步预览因目标结构读取瞬断而构建失败时，游标仍停在已落账的步骤上；
// 此时再次放行曾把同一个只读导航步骤重复执行落账（doneSteps 虚增），写步骤则有重复写风险。
// 护栏要求：已落账步骤的 step 命令必须被拒绝；未执行步骤与连续执行命令不受影响。
func TestStepReReleaseGuardBlocksDuplicateRelease(t *testing.T) {
	executed := map[int]bool{3: true}
	if err := control.StepReReleaseGuard(model.CommandStep, executed, 3); err == nil {
		t.Fatal("已落账步骤的再次放行必须被拒绝")
	} else if !errors.Is(err, control.ErrStepAlreadyExecuted) {
		t.Fatalf("拒绝错误必须是 ErrStepAlreadyExecuted：%v", err)
	}
	if err := control.StepReReleaseGuard(model.CommandStep, executed, 4); err != nil {
		t.Fatalf("未执行步骤的放行不应被拒绝：%v", err)
	}
	if err := control.StepReReleaseGuard(model.CommandStep, map[int]bool{}, 3); err != nil {
		t.Fatalf("未落账步骤的放行不应被拒绝：%v", err)
	}
	if err := control.StepReReleaseGuard(model.CommandContinue, executed, 3); err != nil {
		t.Fatalf("护栏只约束单步命令，不影响连续执行命令：%v", err)
	}
}
