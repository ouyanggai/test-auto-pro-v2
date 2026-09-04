package run_readiness_test

import (
	"testing"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// singleArrival 与 multipleArrival 是两种候选形态：只到达一次与会到达多次。
func singleArrival() []model.SuccessAssertionEndNodeCandidate {
	return []model.SuccessAssertionEndNodeCandidate{{NodeKey: "end-a", Name: "同意结束", ArrivalCount: 1}}
}

func multipleArrival() []model.SuccessAssertionEndNodeCandidate {
	return []model.SuccessAssertionEndNodeCandidate{{NodeKey: "end-all", Name: "汇合结束", ArrivalCount: 2}}
}

// TestValidateAcceptsRealCandidateAndStatus 验证合法输入能落库，并带上目标自己的中文状态标签。
func TestValidateAcceptsRealCandidateAndStatus(t *testing.T) {
	assertion, reason := service.ValidateSuccessAssertion(singleArrival(), service.SuccessAssertionInput{
		EndNodeKey: "end-a", ExpectedStatus: model.FlowInstanceStatusEnd,
	})
	if reason != "" {
		t.Fatalf("合法输入被拒绝：%s", reason)
	}
	if assertion.EndNodeKey != "end-a" || assertion.EndNodeName != "同意结束" {
		t.Fatalf("断言没有带上候选的真实名称：%+v", assertion)
	}
	if assertion.ExpectedStatusLabel != "完结" {
		t.Fatalf("期望状态没有带上目标中文标签：%+v", assertion)
	}
	if assertion.ArrivalOrdinal != 1 {
		t.Fatalf("只到达一次时序号应固定为 1：%d", assertion.ArrivalOrdinal)
	}
}

// TestValidateRejectsInputOutsideRealFacts 验证四类非法输入各自给出中文原因，且都不自动修正。
func TestValidateRejectsInputOutsideRealFacts(t *testing.T) {
	cases := []struct {
		name       string
		candidates []model.SuccessAssertionEndNodeCandidate
		input      service.SuccessAssertionInput
	}{
		{"候选为空", nil, service.SuccessAssertionInput{EndNodeKey: "end-a", ExpectedStatus: model.FlowInstanceStatusEnd}},
		{"未选结束节点", singleArrival(), service.SuccessAssertionInput{ExpectedStatus: model.FlowInstanceStatusEnd}},
		{"结束节点不在真实线路上", singleArrival(),
			service.SuccessAssertionInput{EndNodeKey: "end-x", ExpectedStatus: model.FlowInstanceStatusEnd}},
		{"未选期望状态", singleArrival(), service.SuccessAssertionInput{EndNodeKey: "end-a"}},
		{"期望状态是自造取值", singleArrival(),
			service.SuccessAssertionInput{EndNodeKey: "end-a", ExpectedStatus: "finished"}},
	}
	for _, testCase := range cases {
		assertion, reason := service.ValidateSuccessAssertion(testCase.candidates, testCase.input)
		if reason == "" {
			t.Fatalf("%s 应被拒绝", testCase.name)
		}
		if assertion.EndNodeKey != "" || assertion.ExpectedStatus != "" {
			t.Fatalf("%s 被拒绝时不应返回可落库的断言：%+v", testCase.name, assertion)
		}
	}
}

// TestArrivalOrdinalFollowsRealArrivalCount 验证第几次到达完全按真实到达次数强制或禁止。
func TestArrivalOrdinalFollowsRealArrivalCount(t *testing.T) {
	// 只到达一次：不填与填 1 都接受，填 2 必须被拒绝。
	for _, ordinal := range []uint{0, 1} {
		if _, reason := service.ValidateSuccessAssertion(singleArrival(), service.SuccessAssertionInput{
			EndNodeKey: "end-a", ExpectedStatus: model.FlowInstanceStatusEnd, ArrivalOrdinal: ordinal,
		}); reason != "" {
			t.Fatalf("只到达一次时序号 %d 应被接受：%s", ordinal, reason)
		}
	}
	if _, reason := service.ValidateSuccessAssertion(singleArrival(), service.SuccessAssertionInput{
		EndNodeKey: "end-a", ExpectedStatus: model.FlowInstanceStatusEnd, ArrivalOrdinal: 2,
	}); reason == "" {
		t.Fatal("只到达一次时填第 2 次必须被拒绝")
	}
	// 会到达多次：不填必须被拒绝，越界必须被拒绝，范围内接受。
	if _, reason := service.ValidateSuccessAssertion(multipleArrival(), service.SuccessAssertionInput{
		EndNodeKey: "end-all", ExpectedStatus: model.FlowInstanceStatusEnd,
	}); reason == "" {
		t.Fatal("会到达多次时必须要求指定第几次到达")
	}
	if _, reason := service.ValidateSuccessAssertion(multipleArrival(), service.SuccessAssertionInput{
		EndNodeKey: "end-all", ExpectedStatus: model.FlowInstanceStatusEnd, ArrivalOrdinal: 3,
	}); reason == "" {
		t.Fatal("越界的第几次到达必须被拒绝")
	}
	assertion, reason := service.ValidateSuccessAssertion(multipleArrival(), service.SuccessAssertionInput{
		EndNodeKey: "end-all", ExpectedStatus: model.FlowInstanceStatusEnd, ArrivalOrdinal: 2,
	})
	if reason != "" || assertion.ArrivalOrdinal != 2 {
		t.Fatalf("范围内的第几次到达应被接受：%s %+v", reason, assertion)
	}
}

// TestRevalidateReportsWithoutFixing 验证只读复验只报问题不自动修正，三类失效各有中文原因。
func TestRevalidateReportsWithoutFixing(t *testing.T) {
	gone := service.RevalidateSuccessAssertion(singleArrival(), model.PathSuccessAssertion{
		EndNodeKey: "end-removed", EndNodeName: "已删除结束", ExpectedStatus: model.FlowInstanceStatusEnd, ArrivalOrdinal: 1,
	})
	if len(gone) != 1 || gone[0].Kind != "success_assertion" {
		t.Fatalf("结束节点已不在路径上时应报一条问题：%+v", gone)
	}
	overflow := service.RevalidateSuccessAssertion(multipleArrival(), model.PathSuccessAssertion{
		EndNodeKey: "end-all", ExpectedStatus: model.FlowInstanceStatusEnd, ArrivalOrdinal: 5,
	})
	if len(overflow) != 1 {
		t.Fatalf("到达序号越界时应报一条问题：%+v", overflow)
	}
	badStatus := service.RevalidateSuccessAssertion(singleArrival(), model.PathSuccessAssertion{
		EndNodeKey: "end-a", ExpectedStatus: "finished", ArrivalOrdinal: 1,
	})
	if len(badStatus) != 1 {
		t.Fatalf("期望状态不在目标取值内时应报一条问题：%+v", badStatus)
	}
	healthy := service.RevalidateSuccessAssertion(singleArrival(), model.PathSuccessAssertion{
		EndNodeKey: "end-a", ExpectedStatus: model.FlowInstanceStatusEnd, ArrivalOrdinal: 1,
	})
	if len(healthy) != 0 {
		t.Fatalf("站得住的断言不应报问题：%+v", healthy)
	}
}
