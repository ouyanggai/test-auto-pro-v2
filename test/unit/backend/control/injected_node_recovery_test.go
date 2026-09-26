package control_test

import (
	"testing"

	"test-auto-pro-v2/internal/engine/control"
	"test-auto-pro-v2/internal/model"
)

// TestInsertedNodeApprovalsInheritRecoveryTrigger 锁定恢复锚点后动态加签步骤的触发依赖。
// 触发驳回被目标跳过时，插入节点也必须随恢复链标记为不适用，不能单独阻塞运行。
func TestInsertedNodeApprovalsInheritRecoveryTrigger(t *testing.T) {
	steps := control.InsertedNodeApprovalsForTest(model.CompiledActionStep{
		Source:          model.ActionStepSourceRecovery,
		SourceActionKey: "reject-action-1",
		Action:          model.ActionApprove,
		NodeKey:         "anchor-node",
	}, []string{control.AddSignInsertedNodeKey("inserted-node")})
	if len(steps) != 1 {
		t.Fatalf("应构造一条插入节点审批步骤，实际 %d 条", len(steps))
	}
	if steps[0].Source != model.ActionStepSourceRecovery || steps[0].SourceActionKey != "reject-action-1" {
		t.Fatalf("插入节点审批必须继承恢复触发依赖：%+v", steps[0])
	}
}

// TestInsertedNodeApprovalsKeepNormalAnchorAsSystemDefault 锁定普通锚点仍使用默认系统步骤来源。
func TestInsertedNodeApprovalsKeepNormalAnchorAsSystemDefault(t *testing.T) {
	steps := control.InsertedNodeApprovalsForTest(model.CompiledActionStep{
		Source:  model.ActionStepSourceSystemDefault,
		Action:  model.ActionApprove,
		NodeKey: "anchor-node",
	}, []string{control.AddSignInsertedNodeKey("inserted-node")})
	if len(steps) != 1 {
		t.Fatalf("应构造一条插入节点审批步骤，实际 %d 条", len(steps))
	}
	if steps[0].Source != model.ActionStepSourceSystemDefault || steps[0].SourceActionKey != "" {
		t.Fatalf("普通锚点不应携带恢复触发依赖：%+v", steps[0])
	}
}
