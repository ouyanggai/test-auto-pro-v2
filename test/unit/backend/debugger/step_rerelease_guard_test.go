package debugger_test

import (
	"errors"
	"testing"

	"test-auto-pro-v2/internal/engine/control"
	"test-auto-pro-v2/internal/model"
)

// TestStepReReleaseGuardBlocksDuplicateRelease 锁定放行的防重复执行护栏：
// 真实运行中，游标停在已落账步骤（下一步预览因目标结构读取瞬断未建出，或加签写后重绑定停机）时，
// 再次放行会把同一步再跑一遍——只读导航步骤虚增进度，写步骤则重复写（实测加签因此插入了第二个
// 审核人节点并把运行判成待确认）。护栏要求：无论 step 还是 continue/next_node，只要游标所指步骤
// 已落账成功就必须拒绝；未执行步骤不受影响（连续放行时游标已前移到未执行步骤，不会误拦）。
func TestStepReReleaseGuardBlocksDuplicateRelease(t *testing.T) {
	executed := map[int]bool{3: true}
	for _, command := range []model.ControlCommand{model.CommandStep, model.CommandContinue, model.CommandNextNode} {
		if err := control.StepReReleaseGuard(command, executed, 3); err == nil {
			t.Fatalf("命令 %s 对已落账步骤的再次放行必须被拒绝", command)
		} else if !errors.Is(err, control.ErrStepAlreadyExecuted) {
			t.Fatalf("命令 %s 的拒绝错误必须是 ErrStepAlreadyExecuted：%v", command, err)
		}
	}
	if err := control.StepReReleaseGuard(model.CommandStep, executed, 4); err != nil {
		t.Fatalf("未执行步骤的放行不应被拒绝：%v", err)
	}
	if err := control.StepReReleaseGuard(model.CommandContinue, map[int]bool{}, 3); err != nil {
		t.Fatalf("未落账步骤的连续放行不应被拒绝：%v", err)
	}
}
