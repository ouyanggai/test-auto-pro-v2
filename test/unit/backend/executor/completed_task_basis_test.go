// F-036 第三轮 T3 定向验证：「当前处理人已办」必须是分型事实，不能塌缩成布尔对。
//
// 背景（复核 P0-4）：原实现用 CompletedTaskRead/CompletedTaskFound 这对布尔表达核验依据，
// 但这对布尔只能说明「读没读」与「读没读到」，无法区分三种语义完全不同的情形：
//
//	· 确实按执行人核对过已办并命中（会签中间态的有效证据）；
//	· 核对过但目标上确实没有该已办任务（否定证据）；
//	· 根本没有做这项核对（占位，如处理人即计划账号本人时不切会话）。
//
// 把第三种的 false/false 与第二种混淆，会让判定建立在一个从未取证的占位布尔上，
// 一旦会签、委托或代批场景出现就会误判成功。
//
// 本文件锁定：只有 CheckedFound 才能把「待办仍在」判成已生效；占位与读取失败都不行。
package executor_test

import (
	"testing"

	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/engine/verdict"
	"test-auto-pro-v2/internal/model"
)

// approveFactsBase 构造同意动作重读后的公共事实：实例可读、本节点待办仍在。
func approveFactsBase() step.InstanceFacts {
	return step.InstanceFacts{
		Found:             true,
		Status:            "run",
		CurrentTaskRead:   true,
		CurrentTaskFound:  true,
		CurrentTaskNodeID: "node-audit",
		CurrentNodes:      []string{"node-audit"},
		DueNodes:          []string{"node-audit"},
	}
}

// TestF036CountersignAdvancedRequiresCheckedFound 锁定会签中间态的正面证据：
// 本节点待办仍在，但确实按执行人核对到本节点的已完成任务 → 判为已推进。
func TestF036CountersignAdvancedRequiresCheckedFound(t *testing.T) {
	before := step.InstanceFacts{Found: true, Status: "run", CurrentTaskFound: true, CurrentTaskNodeID: "node-audit"}
	after := approveFactsBase()
	after.CompletedTaskBasis = step.CompletedTaskBasisCheckedFound
	after.CompletedTaskNodeID = "node-audit"

	got := step.ClassifyReread(string(model.ActionApprove), "node-audit", before, after)
	if got != verdict.RereadAdvanced {
		t.Fatalf("已按执行人核对并命中本节点已办时应判为已推进，实际 %v", got)
	}
}

// TestF036CountersignNotAdvancedWhenBasisNotAttempted 锁定占位不得冒充证据：
// 待办仍在且「已办」这项核对根本没做（处理人即计划账号本人的场景）时，
// 必须判为「未见变化」，绝不能因为一个未取证的布尔就宣告成功。
func TestF036CountersignNotAdvancedWhenBasisNotAttempted(t *testing.T) {
	before := step.InstanceFacts{Found: true, Status: "run", CurrentTaskFound: true, CurrentTaskNodeID: "node-audit"}
	after := approveFactsBase()
	// 占位：未做该项核对，且为了构造「旧实现在这里会误判」的形态，把两个布尔都置 true。
	after.CompletedTaskBasis = step.CompletedTaskBasisNotAttempted
	after.CompletedTaskRead = true
	after.CompletedTaskFound = true
	after.CompletedTaskNodeID = "node-audit"

	got := step.ClassifyReread(string(model.ActionApprove), "node-audit", before, after)
	if got == verdict.RereadAdvanced {
		t.Fatalf("未做已办核对时不得把仍在的待办判成已生效（这正是 P0-4 的误判形态）")
	}
	if got != verdict.RereadUnchanged {
		t.Fatalf("待办仍在且无正面证据时应判为未见变化，实际 %v", got)
	}
}

// TestF036CountersignNotAdvancedWhenBasisCheckedAbsent 锁定否定证据不得被当作成功：
// 确实核对过、目标上就是没有该已办任务时，仍在的待办说明写未生效。
func TestF036CountersignNotAdvancedWhenBasisCheckedAbsent(t *testing.T) {
	before := step.InstanceFacts{Found: true, Status: "run", CurrentTaskFound: true, CurrentTaskNodeID: "node-audit"}
	after := approveFactsBase()
	after.CompletedTaskBasis = step.CompletedTaskBasisCheckedAbsent
	after.CompletedTaskRead = true

	got := step.ClassifyReread(string(model.ActionApprove), "node-audit", before, after)
	if got != verdict.RereadUnchanged {
		t.Fatalf("核对过但没有该已办时应判为未见变化，实际 %v", got)
	}
}

// TestF036CountersignNotAdvancedWhenBasisReadFailed 锁定读取失败不得被当作成功：
// 读取失败是「不知道」，既不是命中也不是没有；把不知道当成功会把不确定说成确定。
func TestF036CountersignNotAdvancedWhenBasisReadFailed(t *testing.T) {
	before := step.InstanceFacts{Found: true, Status: "run", CurrentTaskFound: true, CurrentTaskNodeID: "node-audit"}
	after := approveFactsBase()
	after.CompletedTaskBasis = step.CompletedTaskBasisReadFailed
	after.CompletedTaskRead = true

	got := step.ClassifyReread(string(model.ActionApprove), "node-audit", before, after)
	if got == verdict.RereadAdvanced {
		t.Fatalf("已办核对读取失败时不得判为已生效")
	}
}

// TestF036CountersignBasisMustMatchNode 锁定已办必须落在本节点：
// 命中的已办属于另一个节点时不能证明本步生效，否则跨节点的历史已办会顶替本步证据。
func TestF036CountersignBasisMustMatchNode(t *testing.T) {
	before := step.InstanceFacts{Found: true, Status: "run", CurrentTaskFound: true, CurrentTaskNodeID: "node-audit"}
	after := approveFactsBase()
	after.CompletedTaskBasis = step.CompletedTaskBasisCheckedFound
	// 命中的已办在别的节点上。
	after.CompletedTaskNodeID = "node-other"

	got := step.ClassifyReread(string(model.ActionApprove), "node-audit", before, after)
	if got == verdict.RereadAdvanced {
		t.Fatalf("已办不在本节点时不得判为已生效，实际 %v", got)
	}
}

// TestF036CompletedTaskBasisOfClassifiesFourStates 锁定分型函数的四态映射，
// 确保读取失败、未核对、核对到、核对不到四者互不塌缩。
func TestF036CompletedTaskBasisOfClassifiesFourStates(t *testing.T) {
	cases := []struct {
		name          string
		readAttempted bool
		found         bool
		readFailed    bool
		want          string
	}{
		{"读取失败优先于其余状态", true, true, true, step.CompletedTaskBasisReadFailed},
		{"未做核对是占位", false, false, false, step.CompletedTaskBasisNotAttempted},
		{"核对并命中", true, true, false, step.CompletedTaskBasisCheckedFound},
		{"核对但无该已办", true, false, false, step.CompletedTaskBasisCheckedAbsent},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			if got := step.CompletedTaskBasisOfForTest(item.readAttempted, item.found, item.readFailed); got != item.want {
				t.Fatalf("分型应为 %s，实际 %s", item.want, got)
			}
		})
	}
}
