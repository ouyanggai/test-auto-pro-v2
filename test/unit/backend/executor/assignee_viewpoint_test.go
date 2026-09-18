// F-036 第三轮 T2 定向验证：提交阶段的待办查询必须带「视角用户」。
//
// 背景（复核 P0-3）：目标的 /web/flowJobTaskLink/list 只在请求顶层 queryUserId 为空时
// 才按 SID 解析当前用户。引擎此前在 submit 阶段一律发空视角查询，等于「只看执行者自己名下的待办」：
// 处理人恰好是执行者本人时侥幸命中（run 2 正是这种巧合），一旦待办挂在别的账号名下就必然查空，
// 再被上层解释成「目标节点未生成处理人」——这正是历史上该误判的直接成因。
//
// 本文件锁定两件事：
//  1. 待办在目标处理人 B 名下、执行者 A 名下无待办时，提交阶段仍能拿到任务号；
//  2. 不得因此把 HandlerMissing 判成「未生成处理人」。
package executor_test

import (
	"context"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// assigneeViewTarget 构造「待办在 B 名下、执行者 A 名下无待办」的目标：
// 空视角查询（执行者自身视角）返回空，只有按 B 的用户视角查询才命中任务号。
type assigneeViewTarget struct {
	*fakeTarget
	// viewCalls 记录每次视角查询使用的 queryUserId，用于断言查询确实带了视角。
	viewCalls []string
	// selfViewCalls 记录空视角查询次数（执行者自身视角），正确实现不应在提交阶段出现。
	selfViewCalls int
	// selfTaskID 非空时表示执行者自身会话在本节点能读到该任务号（用于锁定既有顺序优先）。
	selfTaskID string
}

// FindSubmittedFlowFacts 让实例事实可读：当前节点是 node-audit。
func (t *assigneeViewTarget) FindSubmittedFlowFacts(_ context.Context, _ target.Session, _ string) (target.SubmittedFlowFacts, error) {
	return target.SubmittedFlowFacts{
		FlowProxyID: "flow-proxy-1", CurrentNodes: []string{"node-audit"}, Status: "run",
		Handlers: []target.NodeCurrentHandler{{NodeID: "node-audit", AuditType: "personnel", BizIDs: []string{"user-b"}}},
		Found:    true,
	}, nil
}

// MatchHandlerAccounts 把目标处理人 user-b 解析到登录账号 renshi-b。
func (t *assigneeViewTarget) MatchHandlerAccounts(_ context.Context, _ target.Session, _ target.NodeCurrentHandler) ([]target.HandlerAccount, error) {
	return []target.HandlerAccount{{Key: "id:user-b", UserID: "user-b", Name: "李四", Account: "renshi-b"}}, nil
}

// ListTaskSnapshotsForUser 是视角查询：只有 user-b 视角能看到本节点待办。
func (t *assigneeViewTarget) ListTaskSnapshotsForUser(_ context.Context, _ target.Session, _, _, queryUserID string) ([]target.TaskSnapshot, error) {
	t.viewCalls = append(t.viewCalls, queryUserID)
	if queryUserID == "" {
		t.selfViewCalls++
		return nil, nil
	}
	if queryUserID == "user-b" {
		return []target.TaskSnapshot{{
			JobTaskID: "task-under-b", FlowNodeProxyID: "node-audit", FlowProxyID: "flow-proxy-1",
			FlowInstanceID: "instance-9", TaskStatus: "pending",
		}}, nil
	}
	return nil, nil
}

// FindTaskSnapshot 表示执行者自身会话名下没有本节点待办；
// selfTaskID 非空时按会话本人返回该任务号（锁定「本人视角优先」的既有顺序）。
func (t *assigneeViewTarget) FindTaskSnapshot(_ context.Context, session target.Session, _, nodeProxyID, _ string) (target.TaskSnapshot, error) {
	if t.selfTaskID == "" {
		return target.TaskSnapshot{}, nil
	}
	return target.TaskSnapshot{
		JobTaskID: t.selfTaskID, FlowNodeProxyID: nodeProxyID, FlowProxyID: "flow-proxy-1",
		FlowInstanceID: "instance-9", TaskStatus: "pending",
	}, nil
}

// TestSubmitPhaseTaskQueryCarriesAssigneeViewpoint 锁定 T2 的核心断言：
// 门禁阶段已解析出目标处理人 user-b 后，提交阶段的待办刷新必须用该用户视角查到任务号，
// 而不是退回执行者自身的空视角查询（空视角在本场景必然查空）。
func TestSubmitPhaseTaskQueryCarriesAssigneeViewpoint(t *testing.T) {
	viewTarget := &assigneeViewTarget{fakeTarget: &fakeTarget{
		instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-audit"}, DueNodes: []string{"node-audit"}},
	}}
	sessions := &recordingSessions{}
	executor := step.NewExecutor(viewTarget, sessions, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)
	runCtx := factRunContext(nil)

	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if preview == nil {
		t.Fatal("预览不应为空")
	}
	// 门禁阶段必须已经通过视角查询解析出处理人事实。
	if preview.Facts.CurrentTaskAssigneeID != "user-b" {
		t.Fatalf("门禁阶段应解析出处理人 user-b，实际 %q", preview.Facts.CurrentTaskAssigneeID)
	}
	if !preview.Facts.CurrentTaskFound {
		t.Fatalf("门禁阶段应命中 user-b 名下的待办：%+v", preview.Facts)
	}

	// 关键断言：门禁阶段的视角查询必须带非空视角用户，绝不能是空视角。
	hasViewpoint := false
	for _, call := range viewTarget.viewCalls {
		if call == "user-b" {
			hasViewpoint = true
		}
	}
	if !hasViewpoint {
		t.Fatalf("待办查询必须带处理人视角 user-b，实际查询视角 %v", viewTarget.viewCalls)
	}
	if viewTarget.selfViewCalls != 0 {
		t.Fatalf("本场景下空视角查询必然落空，不应发出 %d 次", viewTarget.selfViewCalls)
	}
}

// TestAssigneeViewpointMissingDoesNotBecomeHandlerMissing 锁定 T2 的第二个断言：
// 目标处理人已由 currentAuditUserInfo 明确给出时，即使执行者自身视角查空，
// 也绝不能把事件判成 HandlerMissing（「目标节点未生成处理人」）——
// 那会把「查错了视角」伪装成「目标没生成处理人」，正是历史上的误判成因。
func TestAssigneeViewpointMissingDoesNotBecomeHandlerMissing(t *testing.T) {
	viewTarget := &assigneeViewTarget{fakeTarget: &fakeTarget{
		instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-audit"}, DueNodes: []string{"node-audit"}},
	}}
	sessions := &recordingSessions{}
	executor := step.NewExecutor(viewTarget, sessions, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)
	runCtx := factRunContext(nil)

	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if preview == nil {
		t.Fatal("预览不应为空")
	}
	// 处理人事实存在时不得置 HandlerMissing：该标志只表示「目标尚未产生处理人」。
	if preview.Facts.HandlerMissing {
		t.Fatalf("已有 currentAuditUserInfo 处理人时不得判成未生成处理人：%+v", preview.Facts)
	}
	// 处理人已解析出来时也不应给出「没有本节点处理人」这类诊断。
	if strings.Contains(preview.Facts.AssigneeDiag, "未生成处理人") {
		t.Fatalf("不得把查空说成目标未生成处理人：%q", preview.Facts.AssigneeDiag)
	}
}

// TestSubmitPhaseRefreshFallsBackToAssigneeViewpoint 直接锁定 T2 修复的落点：
// 当执行者自身会话在本节点读不到待办（FindTaskSnapshot 返回空）时，
// refreshPendingTaskSnapshotForAssignee 必须用已核实的处理人视角补查并拿到任务号。
//
// 这是 run 2 掩盖掉的真实场景：处理人恰为执行者本人时空视角侥幸命中，
// 一旦待办挂在别人名下，不带视角的查询必然查空，再被上层解释成「目标节点未生成处理人」。
func TestSubmitPhaseRefreshFallsBackToAssigneeViewpoint(t *testing.T) {
	viewTarget := &assigneeViewTarget{fakeTarget: &fakeTarget{
		instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-audit"}, DueNodes: []string{"node-audit"}},
	}}
	sessions := &recordingSessions{}
	executor := step.NewExecutor(viewTarget, sessions, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)
	runCtx := factRunContext(nil)

	// 直接走提交阶段的刷新入口，处理人事实由门禁阶段解析得出（user-b）。
	session := target.Session{
		SID: "sid-executor-a", UserID: "user-a",
		Summary: target.AccountSummary{Account: "zhixing-a", DisplayName: "执行者A"},
	}
	_, snapshot, err := executor.RefreshPendingTaskForAssigneeForTest(context.Background(), runCtx, runCtx.Steps[0], "node-audit", session, "user-b")
	if err != nil {
		t.Fatalf("提交阶段待办刷新不应失败：%v", err)
	}
	// 核心断言：执行者自身视角读空后，必须靠处理人视角补查到任务号。
	if snapshot.JobTaskID != "task-under-b" {
		t.Fatalf("必须用处理人视角补查到任务号 task-under-b，实际 %+v", snapshot)
	}
	if snapshot.PendingUserID != "user-b" {
		t.Fatalf("补查命中后必须回填处理人用户 ID，实际 %q", snapshot.PendingUserID)
	}
}

// TestSubmitPhaseRefreshUsesAssigneeViewpoint 锁定提交阶段直接使用已解析处理人视角。
func TestSubmitPhaseRefreshKeepsSelfViewpointFirst(t *testing.T) {
	viewTarget := &assigneeViewTarget{fakeTarget: &fakeTarget{
		instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-audit"}, DueNodes: []string{"node-audit"}},
	}}
	// 让会话本人读到 task-self，而视角查询会返回 task-under-b。
	viewTarget.selfTaskID = "task-self"
	sessions := &recordingSessions{}
	executor := step.NewExecutor(viewTarget, sessions, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)
	runCtx := factRunContext(nil)

	session := target.Session{
		SID: "sid-executor-a", UserID: "user-a",
		Summary: target.AccountSummary{Account: "zhixing-a", DisplayName: "执行者A"},
	}
	_, snapshot, err := executor.RefreshPendingTaskForAssigneeForTest(context.Background(), runCtx, runCtx.Steps[0], "node-audit", session, "user-b")
	if err != nil {
		t.Fatalf("提交阶段待办刷新不应失败：%v", err)
	}
	if snapshot.JobTaskID != "task-under-b" {
		t.Fatalf("提交阶段必须使用已解析处理人视角任务身份，实际 %+v", snapshot)
	}
	if targetValue := strings.TrimSpace(snapshot.PendingUserID); targetValue != "user-b" {
		t.Fatalf("提交阶段必须回填处理人用户，实际 %q", targetValue)
	}
}

// 编译期断言：本文件的运行上下文构造必须与既有夹具同一形状。
var _ = model.CompiledActionStep{}
