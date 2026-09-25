// Plan 59 / Run 169 复审 §3.2 回归：会签/并行「本步节点与后续节点并存」快照的推进判定，
// 必须依赖本步执行人的完成事实，不能只看节点集合，也不能被别的并行分支推进误判成本步成功。
//
// 背景：advancedByNodePosition 原先只要本步节点仍在 CurrentNodes 就立即返回 false，不检查同一快照里
// 是否已出现合法后续节点；会签中间态下这会漏判本人已完成的写。修复后并存快照需「本步执行人完成证据」，
// 该证据优先按稳定任务关联行 ID 绑定（不受目标改写已办行节点字段影响），回退按「确实核对并命中本节点已办」。
package executor_test

import (
	"testing"

	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/engine/verdict"
	"test-auto-pro-v2/internal/model"
)

// coexistForward 是本步之后的合法节点集合。
var coexistForward = []string{"node-next"}

// TestCoexistUnchangedWhenOnlyStepNodePresentNoEvidence 场景①：只剩本步节点、本人待办仍在、无完成证据 → 未见变化。
func TestCoexistUnchangedWhenOnlyStepNodePresentNoEvidence(t *testing.T) {
	before := step.InstanceFacts{CurrentTaskLinkID: "link-me"}
	after := step.InstanceFacts{
		Found: true, Status: "run",
		CurrentNodes:       []string{"node-audit"},
		CurrentTaskRead:    true,
		CurrentTaskFound:   true,
		CompletedTaskBasis: step.CompletedTaskBasisCheckedAbsent,
	}
	got := step.ClassifyReread(string(model.ActionApprove), "node-audit", coexistForward, before, after)
	if got != verdict.RereadUnchanged {
		t.Fatalf("只剩本步节点且无完成证据时应判未见变化，实际 %v", got)
	}
}

// TestCoexistAdvancedWhenActorLinkRowConfirmed 场景②：本步节点与后续节点并存，
// 本步执行人的关联行已确认（即使目标改写了已办行节点，节点等值比较失败）→ 已推进。
func TestCoexistAdvancedWhenActorLinkRowConfirmed(t *testing.T) {
	before := step.InstanceFacts{CurrentTaskLinkID: "link-me"}
	after := step.InstanceFacts{
		Found: true, Status: "run",
		CurrentNodes:         []string{"node-audit", "node-next"},
		CurrentTaskRead:      true,
		CurrentTaskFound:     false,
		CompletedTaskBasis:   step.CompletedTaskBasisCheckedAbsent, // 已办行节点被改写，按节点过滤读空
		CompletedTaskLinkIDs: []string{"link-me"},                 // 但稳定关联行 ID 命中本人写前待办
	}
	got := step.ClassifyReread(string(model.ActionApprove), "node-audit", coexistForward, before, after)
	if got != verdict.RereadAdvanced {
		t.Fatalf("本步执行人关联行已确认时应判已推进，实际 %v", got)
	}
}

// TestCoexistAdvancedWhenActorDoneNodeMatched 场景②回退路径：无关联行匹配，但确实核对并命中本节点已办 → 已推进。
func TestCoexistAdvancedWhenActorDoneNodeMatched(t *testing.T) {
	before := step.InstanceFacts{}
	after := step.InstanceFacts{
		Found: true, Status: "run",
		CurrentNodes:        []string{"node-audit", "node-next"},
		CompletedTaskBasis:  step.CompletedTaskBasisCheckedFound,
		CompletedTaskNodeID: "node-audit",
	}
	got := step.ClassifyReread(string(model.ActionApprove), "node-audit", coexistForward, before, after)
	if got != verdict.RereadAdvanced {
		t.Fatalf("命中本节点已办时应判已推进，实际 %v", got)
	}
}

// TestCoexistNotAdvancedWhenOnlyOtherBranchCompleted 场景③：并存快照下只有别的并行分支完成、
// 本步执行人没有完成证据（本人待办仍在）→ 不得判本步成功。
func TestCoexistNotAdvancedWhenOnlyOtherBranchCompleted(t *testing.T) {
	before := step.InstanceFacts{CurrentTaskLinkID: "link-me"}
	after := step.InstanceFacts{
		Found: true, Status: "run",
		CurrentNodes:         []string{"node-audit", "node-next"},
		CurrentTaskRead:      true,
		CurrentTaskFound:     true,                       // 本人这一签还没审批
		CompletedTaskBasis:   step.CompletedTaskBasisCheckedAbsent,
		CompletedTaskLinkIDs: []string{"link-other"},     // 只有别人的已办
	}
	got := step.ClassifyReread(string(model.ActionApprove), "node-audit", coexistForward, before, after)
	if got == verdict.RereadAdvanced {
		t.Fatalf("只有别的并行分支推进时不得判本步成功，实际 %v", got)
	}
	if got != verdict.RereadUnchanged {
		t.Fatalf("本人待办仍在且无本步完成证据时应判未见变化，实际 %v", got)
	}
}

// TestCoexistCrossNodeStillAdvancesWhenStepNodeGone 场景④：本步节点已消失、命中后续合法节点 → 保留既有跨节点成功判断。
func TestCoexistCrossNodeStillAdvancesWhenStepNodeGone(t *testing.T) {
	before := step.InstanceFacts{}
	after := step.InstanceFacts{
		Found: true, Status: "run",
		CurrentNodes: []string{"node-next"},
	}
	got := step.ClassifyReread(string(model.ActionApprove), "node-audit", coexistForward, before, after)
	if got != verdict.RereadAdvanced {
		t.Fatalf("本步节点已消失且命中后续合法节点时应判已推进，实际 %v", got)
	}
}
