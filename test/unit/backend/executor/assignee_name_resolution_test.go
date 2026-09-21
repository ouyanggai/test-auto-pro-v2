package executor_test

import (
	"context"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// assigneeNameTarget 用于测试处理人账号与姓名解析的假件。
type assigneeNameTarget struct {
	*fakeTarget
	userInfo map[string]target.UserAccount
}

// UserAccountsByID 返回假件配置的用户登录账号与中文姓名。
func (t *assigneeNameTarget) UserAccountsByID(_ context.Context, _ target.Session, ids []string) (map[string]target.UserAccount, error) {
	out := map[string]target.UserAccount{}
	for _, id := range ids {
		if u, ok := t.userInfo[id]; ok {
			out[id] = u
		}
	}
	return out, nil
}

// TestAssigneeNameResolutionNeverShowsUUID 验证审批步骤解析当前处理人时必须保留并展示中文姓名，
// 绝不能将 32 位 hex / UUID 暴露在 ActorName 或步骤摘要中。
func TestAssigneeNameResolutionNeverShowsUUID(t *testing.T) {
	const realUUID = "0546c9d3aa80455ab9045440a9991137"
	const expectedName = "刘慧玲"
	const expectedAccount = "liuhuiling"

	targetFake := &assigneeNameTarget{
		fakeTarget: &fakeTarget{
			instance: fakeTargetView{
				Found:        true,
				Status:       "run",
				CurrentNodes: []string{"node-audit"},
				DueNodes:     []string{"node-audit"},
			},
			handlerAccount: expectedAccount,
		},
		userInfo: map[string]target.UserAccount{
			realUUID: {
				Account: expectedAccount,
				Name:    expectedName,
			},
		},
	}

	runCtx := newRunContext([]model.CompiledActionStep{{
		Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionApprove,
		Scope: model.ActionScopeTask, NodeKey: "node-audit",
	}})
	runCtx.Nodes["node-audit"] = step.NodeInfo{Name: "部门审批", Type: "common", TargetNodeID: "node-audit"}
	runCtx.PathRun.MainInstanceRef = "inst-1"

	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	// 构造待办事实：目标返回的当前处理人 ID 为 32 位 UUID
	preview := &step.StepPreview{
		PathRunID: runCtx.PathRun.ID, StepNo: 1, TotalSteps: 1,
		Action: model.ActionApprove, NodeKey: "node-audit",
		ActorAccount: runCtx.PlanAccount,
		ActorName:    "发起人",
		Facts: step.InstanceFacts{
			CurrentTaskAssigneeID: realUUID,
		},
	}

	// 模拟执行步骤放行准备，断言 ActorName 最终为真实中文姓名，绝非 32 位 UUID
	outcome, _, err := executor.RunApprovedStep(context.Background(), step.ApprovedStep{
		RunCtx:    runCtx,
		Preview:   preview,
		NextIndex: 0,
	})
	if err != nil {
		t.Fatalf("执行步骤失败：%v", err)
	}
	_ = outcome

	if preview.ActorName == realUUID {
		t.Fatalf("ActorName 绝不能被覆盖为 32 位 UUID，实际 %q", preview.ActorName)
	}
	if preview.ActorName != expectedName {
		t.Fatalf("ActorName 应解析为真实姓名 %q，实际 %q", expectedName, preview.ActorName)
	}
	if strings.TrimSpace(preview.ActorAccount) != expectedAccount {
		t.Fatalf("ActorAccount 应解析为 %q，实际 %q", expectedAccount, preview.ActorAccount)
	}
}
