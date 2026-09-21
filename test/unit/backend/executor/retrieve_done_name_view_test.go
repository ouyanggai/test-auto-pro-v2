package executor_test

import (
	"context"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// 2026-09-21 运行 35/36 路径 6 第 11 步缺陷回归：本步节点的办结人只以姓名出现时，
// 取回必须以经人员目录反查出的那个账号发出，而不是退回计划账号。
//
// 现象：第 11 步取回以 `当前处理人_骆蒙恩（登录账号_）` 发出（登录账号为空），
// 写请求虽返回成功，实例没有回到本步节点，路径以「结果待确认」收尾。
//
// 根因：本步节点 b9fc434e… 的办结人在可用读取接口里只以姓名存在——
//   - /web/flowAuditRecord/list 没有该节点的任何记录（只有其它三个节点）；
//   - 该节点的已办行是 taskStatus=done、preAuditStatus=pass、preExecuteName=刘慧玲，
//     但完全没有 executorId 字段。
//
// 两条"按用户 ID 取办结人"的路径同时落空，取回于是退回计划账号身份。

// nameOnlyExecutorTarget 构造"审核记录不覆盖本步节点、已办行只给姓名"的目标。
type nameOnlyExecutorTarget struct {
	*fakeTarget
	// nameTasks 是按姓名反查后、以该用户 ID 视角能读到的已办行。
	nameTasks []target.TaskSnapshot
	// selfTasks 是计划账号自身视角能读到的已办行（本缺陷下应为空/不含本节点）。
	selfTasks []target.TaskSnapshot
	// viewCalls 记录视角查询使用的 queryUserId。
	viewCalls []string
	// matchCalls 记录人员目录反查收到的姓名。
	matchCalls []string
}

// FindTaskSnapshot 让假目标具备任务读取能力面：取回事实读取要求该能力存在，
// 否则会在解析办结人视角之前就提前返回（真实客户端始终具备这一能力）。
func (t *nameOnlyExecutorTarget) FindTaskSnapshot(_ context.Context, _ target.Session, instanceID, nodeID, status string) (target.TaskSnapshot, error) {
	rows, err := t.ListTaskSnapshots(context.Background(), target.Session{}, instanceID, status)
	if err != nil {
		return target.TaskSnapshot{}, err
	}
	for _, row := range rows {
		if strings.TrimSpace(row.FlowNodeProxyID) == strings.TrimSpace(nodeID) {
			return row, nil
		}
	}
	return target.TaskSnapshot{}, nil
}

// FindSubmittedFlowFacts 让实例事实可读：当前节点即本步节点。
func (t *nameOnlyExecutorTarget) FindSubmittedFlowFacts(_ context.Context, _ target.Session, _ string) (target.SubmittedFlowFacts, error) {
	return target.SubmittedFlowFacts{
		FlowProxyID: "flow-proxy-1", CurrentNodes: []string{"node-region"}, Status: "run", Found: true,
	}, nil
}

// ListAuditRecords 模拟"审核记录不覆盖本步节点"：只有别的节点的通过记录。
func (t *nameOnlyExecutorTarget) ListAuditRecords(_ context.Context, _ target.Session, _ string) ([]target.AuditRecordSnapshot, error) {
	return []target.AuditRecordSnapshot{
		{FlowJobTaskID: "link-a", FlowNodeProxyID: "node-other", ExecutorID: "user-other", AuditStatus: "pass"},
	}, nil
}

// ListTaskSnapshots 是计划账号自身视角的已办读取：看不到本步节点的已办。
func (t *nameOnlyExecutorTarget) ListTaskSnapshots(_ context.Context, _ target.Session, _, status string) ([]target.TaskSnapshot, error) {
	if status != "done" {
		return nil, nil
	}
	return t.selfTasks, nil
}

// ListTaskSnapshotsForUser 是带视角的已办读取：只有本节点真实办结人的视角能看到那一行。
func (t *nameOnlyExecutorTarget) ListTaskSnapshotsForUser(_ context.Context, _ target.Session, _, status, queryUserID string) ([]target.TaskSnapshot, error) {
	t.viewCalls = append(t.viewCalls, queryUserID)
	if status != "done" || queryUserID != "user-liuhuiling" {
		return nil, nil
	}
	return t.nameTasks, nil
}

// MatchHandlerAccounts 模拟按姓名反查人员目录：刘慧玲 → user-liuhuiling。
func (t *nameOnlyExecutorTarget) MatchHandlerAccounts(_ context.Context, _ target.Session, handler target.NodeCurrentHandler) ([]target.HandlerAccount, error) {
	for _, user := range handler.Users {
		t.matchCalls = append(t.matchCalls, user.Name)
		if user.Name == "刘慧玲" {
			return []target.HandlerAccount{{
				Key: "name:刘慧玲", UserID: "user-liuhuiling", Name: "刘慧玲", Account: "刘慧玲",
			}}, nil
		}
	}
	return nil, nil
}

// newNameOnlyRetrieveContext 构造一条取回步骤，节点目标 ID 为 node-region。
func newNameOnlyRetrieveContext() step.RunContext {
	runCtx := prepConsentRunContext()
	runCtx.Nodes["node-retrieve"] = step.NodeInfo{Name: "省区经理", Type: "common", TargetNodeID: "node-region"}
	return runCtx
}

// newNameOnlyRetrieveStep 构造取回步骤本身（已完成任务作用域，节点键 node-retrieve）。
func newNameOnlyRetrieveStep() model.CompiledActionStep {
	return model.CompiledActionStep{
		Sequence: 1, Action: model.ActionRetrieve,
		NodeKey: "node-retrieve",
		Scope:   model.ActionScopeCompletedTask,
	}
}

// TestRetrieveViewUserResolvedFromDoneRowName 锁定主修复路径：
// 审核记录不覆盖本步节点、已办行只给姓名时，取回视角必须由姓名反查出的用户 ID 承担，
// 并以该用户身份读取到本步节点的已办行、把该用户登记为取回身份。
func TestRetrieveViewUserResolvedFromDoneRowName(t *testing.T) {
	targetFake := &nameOnlyExecutorTarget{
		fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-region"}, DueNodes: []string{"node-region"}}},
		// 计划账号视角能看到本步节点的办结行，但目标在这行上只给姓名、不给 executorId
		// （2026-09-21 运行 36 实测形态：node=b9fc434e… preExecuteName=刘慧玲 execId 缺失）。
		selfTasks: []target.TaskSnapshot{{
			LinkID: "link-region", JobTaskID: "task-region", FlowNodeProxyID: "node-region",
			FlowProxyID: "flow-proxy-1", TaskStatus: "done", ExecutorName: "刘慧玲",
		}},
		// 反查出的用户视角同样能看到这一行，用于确认后续按视角读取也能命中。
		nameTasks: []target.TaskSnapshot{{
			LinkID: "link-region", JobTaskID: "task-region", FlowNodeProxyID: "node-region",
			FlowProxyID: "flow-proxy-1", TaskStatus: "done", ExecutorName: "刘慧玲",
		}},
	}
	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	facts, err := step.ReadActionTaskFactsForTest(context.Background(), executor, newNameOnlyRetrieveContext(), newNameOnlyRetrieveStep())
	if err != nil {
		t.Fatalf("取回事实读取失败：%v", err)
	}
	// 姓名必须真的被拿去反查过目录。
	if len(targetFake.matchCalls) == 0 {
		t.Fatalf("已办行只给姓名时应经人员目录反查账号，实际未发起反查")
	}
	// 视角查询必须使用反查出的用户 ID，而不是空视角（计划账号）。
	if len(targetFake.viewCalls) == 0 || targetFake.viewCalls[0] != "user-liuhuiling" {
		t.Fatalf("已办读取应使用反查出的办结人视角 user-liuhuiling，实际=%v", targetFake.viewCalls)
	}
	if !facts.CompletedTaskFound {
		t.Fatalf("按办结人视角应读到本步节点的已办行：%+v", facts)
	}
	if facts.CompletedTaskLinkID != "link-region" {
		t.Fatalf("已办关联行应为本步节点那一行，实际=%q", facts.CompletedTaskLinkID)
	}
	if facts.CompletedTaskExecutorID != "user-liuhuiling" {
		t.Fatalf("取回身份应为反查出的办结人，实际=%q", facts.CompletedTaskExecutorID)
	}
}

// TestRetrieveViewUserStaysEmptyWhenNameIsAmbiguous 锁定安全边界：
// 姓名在目录里对应多个用户时不得任选其一，取回身份保持为空（交由门禁按既有纪律裁决）。
func TestRetrieveViewUserStaysEmptyWhenNameIsAmbiguous(t *testing.T) {
	targetFake := &ambiguousNameTarget{
		nameOnlyExecutorTarget: &nameOnlyExecutorTarget{
			fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-region"}, DueNodes: []string{"node-region"}}},
			nameTasks: []target.TaskSnapshot{{
				LinkID: "link-region", JobTaskID: "task-region", FlowNodeProxyID: "node-region",
				FlowProxyID: "flow-proxy-1", TaskStatus: "done", ExecutorName: "张伟",
			}},
		},
	}
	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	facts, err := step.ReadActionTaskFactsForTest(context.Background(), executor, newNameOnlyRetrieveContext(), newNameOnlyRetrieveStep())
	if err != nil {
		t.Fatalf("取回事实读取失败：%v", err)
	}
	if facts.CompletedTaskExecutorID != "" {
		t.Fatalf("同名多人时不得任选一人作为取回身份，实际=%q", facts.CompletedTaskExecutorID)
	}
}

// ambiguousNameTarget 让姓名反查返回两个不同用户，模拟目标目录中的同名多人。
type ambiguousNameTarget struct {
	*nameOnlyExecutorTarget
}

// MatchHandlerAccounts 对同一姓名返回两个不同用户 ID。
func (t *ambiguousNameTarget) MatchHandlerAccounts(_ context.Context, _ target.Session, _ target.NodeCurrentHandler) ([]target.HandlerAccount, error) {
	return []target.HandlerAccount{
		{Key: "name:张伟", UserID: "user-zhangwei-1", Name: "张伟", Account: "zhangwei1"},
		{Key: "name:张伟", UserID: "user-zhangwei-2", Name: "张伟", Account: "zhangwei2"},
	}, nil
}
