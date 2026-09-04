package run_readiness_test

import (
	"strings"
	"testing"

	"test-auto-pro-v2/internal/engine/assert"
	"test-auto-pro-v2/internal/model"
)

// savedAssertion 是一条已配置好的断言：到达"同意结束"，实例状态完结，第一次到达即算成功。
func savedAssertion() model.PathSuccessAssertion {
	return model.PathSuccessAssertion{
		EndNodeKey: "end-a", EndNodeName: "同意结束",
		ExpectedStatus: model.FlowInstanceStatusEnd, ArrivalOrdinal: 1,
	}
}

// readableFact 是一次成功读到的事实投影。
func readableFact(status string, arrived ...string) assert.Fact {
	return assert.Fact{
		Readable: true, InstanceStatusPresent: true,
		InstanceStatus: status, ArrivedEndNodeKeys: arrived,
	}
}

// TestAssertionHoldsOnlyWhenAllThreeMatch 验证结束节点、到达次数与期望状态三项全部相符才判成立。
func TestAssertionHoldsOnlyWhenAllThreeMatch(t *testing.T) {
	result := assert.Evaluate(savedAssertion(), readableFact(model.FlowInstanceStatusEnd, "end-a"))
	if result.Outcome != assert.OutcomeHolds {
		t.Fatalf("三项相符时应判成立：%+v", result)
	}
	if result.Reason == "" || result.Basis == "" {
		t.Fatalf("成立结论缺少中文原因或依据：%+v", result)
	}
	// 会到达多次的断言：只到达一次不足以成立。
	multiple := savedAssertion()
	multiple.ArrivalOrdinal = 2
	if once := assert.Evaluate(multiple, readableFact(model.FlowInstanceStatusEnd, "end-a")); once.Outcome != assert.OutcomeFails {
		t.Fatalf("到达次数不足应判不成立：%+v", once)
	}
	if twice := assert.Evaluate(multiple, readableFact(model.FlowInstanceStatusEnd, "end-a", "end-a")); twice.Outcome != assert.OutcomeHolds {
		t.Fatalf("到达次数满足配置应判成立：%+v", twice)
	}
}

// TestAssertionFailsOnlyAfterTerminal 验证只有实例已进终态且三项不全相符时才判不成立，
// 并且原因说清差在哪一项，不给一句笼统的失败。
func TestAssertionFailsOnlyAfterTerminal(t *testing.T) {
	cases := map[string]assert.Fact{
		"终态但从未到达配置的结束节点": readableFact(model.FlowInstanceStatusRejected),
		"到达了但状态与期望不符":    readableFact(model.FlowInstanceStatusWithdraw, "end-a"),
		"到达了别的结束节点":      readableFact(model.FlowInstanceStatusEnd, "end-b"),
	}
	for name, fact := range cases {
		result := assert.Evaluate(savedAssertion(), fact)
		if result.Outcome != assert.OutcomeFails {
			t.Fatalf("%s 应判不成立：%+v", name, result)
		}
		if result.Reason == "" {
			t.Fatalf("%s 缺少中文原因", name)
		}
		if !strings.Contains(result.Basis, "断言不成立") {
			t.Fatalf("%s 的依据必须说明这是断言不成立而不是执行出错：%s", name, result.Basis)
		}
	}
}

// TestUndecidableIsNeverMergedIntoFails 验证六类拿不准的情况全部落无法判定，绝不合并进不成立。
// 这是与 F-014 同一条底线：拿不准就停。
func TestUndecidableIsNeverMergedIntoFails(t *testing.T) {
	cases := map[string]struct {
		assertion model.PathSuccessAssertion
		fact      assert.Fact
	}{
		"事实自相矛盾": {savedAssertion(), assert.Fact{
			Readable: true, InstanceStatusPresent: true, InstanceStatus: model.FlowInstanceStatusEnd,
			Contradictory: true, ContradictionReason: "状态是完结但流程仍在推进",
		}},
		"事实读不到（含会话失效）": {savedAssertion(), assert.Fact{
			Readable: false, UnreadableReason: "目标会话已失效，重读没有拿到实例事实",
		}},
		"实例没有返回状态字段":   {savedAssertion(), assert.Fact{Readable: true, ArrivedEndNodeKeys: []string{"end-a"}}},
		"状态取值超出目标真实集合": {savedAssertion(), readableFact("finished", "end-a")},
		"实例仍未进入终态":     {savedAssertion(), readableFact(model.FlowInstanceStatusRun, "end-a")},
	}
	for name, testCase := range cases {
		result := assert.Evaluate(testCase.assertion, testCase.fact)
		if result.Outcome != assert.OutcomeUndecidable {
			t.Fatalf("%s 应判无法判定，实际 %s：%+v", name, result.Outcome, result)
		}
		if result.Reason == "" || result.Basis == "" {
			t.Fatalf("%s 缺少中文原因或依据：%+v", name, result)
		}
	}
}

// TestTerminalStatusSetIsExplicit 验证终态集合按目标状态名义划分，且集合外取值不被猜成终态。
func TestTerminalStatusSetIsExplicit(t *testing.T) {
	terminal := []string{
		model.FlowInstanceStatusWithdraw, model.FlowInstanceStatusTermination,
		model.FlowInstanceStatusAbandon, model.FlowInstanceStatusRejected, model.FlowInstanceStatusEnd,
	}
	for _, status := range terminal {
		if !assert.IsTerminalStatus(status) {
			t.Fatalf("%s 应属终态", status)
		}
	}
	for _, status := range []string{
		model.FlowInstanceStatusAwaitSent, model.FlowInstanceStatusDraft, model.FlowInstanceStatusRun, "finished", "",
	} {
		if assert.IsTerminalStatus(status) {
			t.Fatalf("%s 不应被当成终态", status)
		}
	}
}

// TestDefaultAssertionIsRunToCompletion 锁定人工验收反馈：没有指定成功节点时默认按"把流程走完"判定。
// 指定成功节点是可选能力（像断点一样），不是必填项，因此这里不能判无法判定。
func TestDefaultAssertionIsRunToCompletion(t *testing.T) {
	none := model.PathSuccessAssertion{}
	// 流程走完（完结）即成立，不要求到达任何指定节点。
	holds := assert.Evaluate(none, readableFact(model.FlowInstanceStatusEnd))
	if holds.Outcome != assert.OutcomeHolds {
		t.Fatalf("流程走完应判成立：%+v", holds)
	}
	// 进了终态但不是完结（撤销、驳回一类）说明没跑完，判不成立并提示可以指定成功节点。
	for _, status := range []string{
		model.FlowInstanceStatusWithdraw, model.FlowInstanceStatusRejected,
		model.FlowInstanceStatusAbandon, model.FlowInstanceStatusTermination,
	} {
		result := assert.Evaluate(none, readableFact(status))
		if result.Outcome != assert.OutcomeFails {
			t.Fatalf("%s 不是跑完流程，应判不成立：%+v", status, result)
		}
		if !strings.Contains(result.Reason, "指定成功节点") {
			t.Fatalf("%s 的原因应提示可以指定成功节点：%s", status, result.Reason)
		}
	}
	// 仍未进终态、事实读不到这些情况依然是无法判定。
	if running := assert.Evaluate(none, readableFact(model.FlowInstanceStatusRun)); running.Outcome != assert.OutcomeUndecidable {
		t.Fatalf("未进终态应判无法判定：%+v", running)
	}
	if unreadable := assert.Evaluate(none, assert.Fact{Readable: false, UnreadableReason: "会话失效"}); unreadable.Outcome != assert.OutcomeUndecidable {
		t.Fatalf("事实读不到应判无法判定：%+v", unreadable)
	}
}
