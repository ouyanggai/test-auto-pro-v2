package action_orchestration_test

import (
	"strings"
	"testing"

	"test-auto-pro-v2/internal/engine/scenario"
	"test-auto-pro-v2/internal/model"
)

// tailPath 是一条 发起→审批→结束 的最小人工路径输入，供固定尾同意条件化断言复用。
func tailPath(actions []model.ConfiguredAction) scenario.Input {
	return scenario.Input{
		Actions: actions,
		Nodes: []model.FlowGraphNode{
			{ID: "start", Type: "start"}, {ID: "review", Type: "common"}, {ID: "end", Type: "end"},
		},
		NodeSequence: []string{"start", "review", "end"},
	}
}

// systemDefaultApproveCount 统计编译结果里系统默认同意步骤数量。
func systemDefaultApproveCount(steps []model.CompiledActionStep) int {
	count := 0
	for _, step := range steps {
		if step.Source == model.ActionStepSourceSystemDefault && step.Action == model.ActionApprove {
			count++
		}
	}
	return count
}

// TestFixedTailConsentFollowsUserActionAsSeparateGroup 验证普通审批节点在用户动作之后仍生成一次
// 固定尾同意，且尾同意作为独立放行动作组，携带条件化门禁事实，不与用户动作混在同一次放行。
func TestFixedTailConsentFollowsUserActionAsSeparateGroup(t *testing.T) {
	result, err := scenario.Compile(tailPath([]model.ConfiguredAction{
		{Key: "storage-1", Action: model.ActionStorageFormData, Scope: model.ActionScopeTask, NodeKey: "review", Order: 1},
	}))
	if err != nil {
		t.Fatalf("暂存 + 固定尾同意编译失败：%v", err)
	}
	// 期望顺序：提交@start、暂存@review(user)、同意@review(system_default)、end 导航。
	var user, tail *model.CompiledActionStep
	for index := range result.Steps {
		step := &result.Steps[index]
		if step.Source == model.ActionStepSourceUser && step.Action == model.ActionStorageFormData {
			user = step
		}
		if step.Source == model.ActionStepSourceSystemDefault && step.Action == model.ActionApprove && step.NodeKey == "review" {
			tail = step
		}
	}
	if user == nil || tail == nil {
		t.Fatalf("应同时存在用户暂存动作和 review 固定尾同意：%+v", result.Steps)
	}
	if user.Sequence >= tail.Sequence {
		t.Fatalf("固定尾同意应排在用户动作之后：user=%d tail=%d", user.Sequence, tail.Sequence)
	}
	if user.ReleaseGroup == tail.ReleaseGroup {
		t.Fatalf("固定尾同意必须与用户动作分属不同放行组：user=%s tail=%s", user.ReleaseGroup, tail.ReleaseGroup)
	}
	if !tail.ReleaseRequired {
		t.Fatal("固定尾同意应作为独立放行边界，人工模式在此停下等待放行")
	}
	// 条件化门禁事实必须写进 Precondition：写后核验、活动待办归属、动态插入节点。
	if !strings.Contains(tail.Precondition, "写后核验") || !strings.Contains(tail.Precondition, "动态插入节点") {
		t.Fatalf("固定尾同意缺少条件化门禁事实：%q", tail.Precondition)
	}
	if !strings.Contains(tail.StopOnFailure, "命令集合为空") {
		t.Fatalf("固定尾同意未声明不满足时命令集合为空：%q", tail.StopOnFailure)
	}
}

// TestInstanceActionDoesNotAddFixedTailConsent 验证关注等实例级动作排在节点动作之后时，
// 不会因此多生成一次固定尾同意——固定尾只按人工节点生成，实例级动作不承载节点同意。
func TestInstanceActionDoesNotAddFixedTailConsent(t *testing.T) {
	result, err := scenario.Compile(tailPath([]model.ConfiguredAction{
		{Key: "storage-1", Action: model.ActionStorageFormData, Scope: model.ActionScopeTask, NodeKey: "review", Order: 1},
		{Key: "follow-1", Action: model.ActionFollow, Scope: model.ActionScopeInstance, Order: 2},
	}))
	if err != nil {
		t.Fatalf("实例级关注编译失败：%v", err)
	}
	if got := systemDefaultApproveCount(result.Steps); got != 1 {
		t.Fatalf("固定尾同意数量 = %d，期望恰好 1（仅 review 节点）：%+v", got, result.Steps)
	}
	// 关注动作本身必须以用户动作出现，不能被套成同意。
	foundFollow := false
	for _, step := range result.Steps {
		if step.Action == model.ActionFollow {
			if step.Source != model.ActionStepSourceUser {
				t.Fatalf("关注动作不应被改写为系统步骤：%+v", step)
			}
			foundFollow = true
		}
	}
	if !foundFollow {
		t.Fatalf("缺少用户关注步骤：%+v", result.Steps)
	}
}

// TestAddSignFixedTailStaysSeparateGroup 验证加签用户动作与其后的固定尾同意分属不同放行组，
// 使插入节点审批（T05）不会被原节点固定尾同意吞并。
func TestAddSignFixedTailStaysSeparateGroup(t *testing.T) {
	result, err := scenario.Compile(tailPath([]model.ConfiguredAction{
		{Key: "addsign-1", Action: model.ActionAddSign, Scope: model.ActionScopeTask, NodeKey: "review", Order: 1},
	}))
	if err != nil {
		t.Fatalf("加签 + 固定尾同意编译失败：%v", err)
	}
	var addSign, tail *model.CompiledActionStep
	for index := range result.Steps {
		switch result.Steps[index].Action {
		case model.ActionAddSign:
			addSign = &result.Steps[index]
		case model.ActionApprove:
			if result.Steps[index].Source == model.ActionStepSourceSystemDefault {
				tail = &result.Steps[index]
			}
		}
	}
	if addSign == nil || tail == nil {
		t.Fatalf("应同时存在加签用户步骤和固定尾同意：%+v", result.Steps)
	}
	if addSign.ReleaseGroup == tail.ReleaseGroup {
		t.Fatalf("加签与原节点固定尾同意必须分属不同放行组：addSign=%s tail=%s", addSign.ReleaseGroup, tail.ReleaseGroup)
	}
	if !tail.ReleaseRequired {
		t.Fatal("加签后的固定尾同意应作为独立放行边界")
	}
}
