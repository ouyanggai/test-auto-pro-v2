package executor_test

import (
	"strings"
	"testing"

	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// F-034 评审修复 #2：isSkip 与审批类型进入执行器后的跳过/阻塞分型锁定。
// 三种场景都不发写请求：分型只决定“跳过落账”还是“发送前阻塞”。

// f034SkipFacts 构造“实例待办已越过本步节点”的目标事实：待办落在路径后方节点。
func f034SkipFacts() step.InstanceFacts {
	return step.InstanceFacts{
		Found:      true,
		Status:     "run",
		CurrentNodes: []string{"late-target"},
		DueNodes:     []string{"late-target"},
	}
}

// f034SkipContext 构造两步场景：本步在 early 节点（位置 0），路径后方 late 节点（位置 1）。
func f034SkipContext(isSkip *bool, auditType string) step.RunContext {
	return step.RunContext{
		Nodes: map[string]step.NodeInfo{
			"early": {Name: "待跳过节点", Type: "common", TargetNodeID: "early-target", IsSkip: isSkip, AuditType: auditType},
			"late":  {Name: "后方节点", Type: "common", TargetNodeID: "late-target"},
		},
		Steps: []model.CompiledActionStep{
			{Source: model.ActionStepSourceUser, Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: "early"},
			{Source: model.ActionStepSourceUser, Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: "late"},
		},
	}
}

// f034SkipStep 构造本步审批动作。
func f034SkipStep() model.CompiledActionStep {
	return model.CompiledActionStep{Source: model.ActionStepSourceUser, Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: "early"}
}

// TestF034SkipDeclaredTrueSkipsWithoutWrite 锁定：isSkip=true 且待办越过本节点 → 目标已跳过语义。
func TestF034SkipDeclaredTrueSkipsWithoutWrite(t *testing.T) {
	action, reason := step.TargetSkippedClassificationForTest(f034SkipContext(f034PtrBool(true), "company"), f034SkipStep(), f034SkipFacts())
	if action != "skip" {
		t.Fatalf("isSkip=true 应解析为跳过：%s（%s）", action, reason)
	}
	if !strings.Contains(reason, "自动跳过") {
		t.Fatalf("跳过原因应说明目标自动跳过：%s", reason)
	}
}

// TestF034SkipUndeclaredBlocksWithoutWrite 锁定：isSkip 未声明（目标不允许跳过）且待办越过 → 阻塞。
func TestF034SkipUndeclaredBlocksWithoutWrite(t *testing.T) {
	for name, isSkip := range map[string]*bool{"未声明": nil, "显式false": f034PtrBool(false)} {
		action, reason := step.TargetSkippedClassificationForTest(f034SkipContext(isSkip, "company"), f034SkipStep(), f034SkipFacts())
		if action != "block" {
			t.Fatalf("%s：不允许跳过时必须阻塞，实际 %s", name, action)
		}
		if !strings.Contains(reason, "无处理人时跳过") {
			t.Fatalf("%s：阻塞原因应说明模板不允许跳过：%s", name, reason)
		}
	}
}

// TestF034RunNodeChooseBlocksEvenWhenSkipDeclared 锁定：run_node_choose 自选节点缺失人员必须阻塞，
// 即使模板声明了 isSkip=true（目标 validateNodeRunNodeChooseIsExistPersonnel 直接抛“未设置审批人”）。
func TestF034RunNodeChooseBlocksEvenWhenSkipDeclared(t *testing.T) {
	action, reason := step.TargetSkippedClassificationForTest(f034SkipContext(f034PtrBool(true), "run_node_choose"), f034SkipStep(), f034SkipFacts())
	if action != "block" {
		t.Fatalf("run_node_choose 越过时必须阻塞，实际 %s", action)
	}
	if !strings.Contains(reason, "自选") {
		t.Fatalf("阻塞原因应说明自选审批节点：%s", reason)
	}
}

// TestF034PendingNotAdvancedNeitherSkipsNorBlocks 锁定：待办未越过本节点时维持既有门禁，不猜测。
func TestF034PendingNotAdvancedNeitherSkipsNorBlocks(t *testing.T) {
	facts := step.InstanceFacts{Found: true, Status: "run", CurrentNodes: []string{"early-target"}, DueNodes: []string{"early-target"}}
	action, _ := step.TargetSkippedClassificationForTest(f034SkipContext(f034PtrBool(true), "company"), f034SkipStep(), facts)
	if action != "" {
		t.Fatalf("待办未越过时不应做出跳过/阻塞判定：%s", action)
	}
}

// f034PtrBool 是布尔指针辅助。
func f034PtrBool(value bool) *bool { return &value }
