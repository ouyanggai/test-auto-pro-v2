package executor_test

import (
	"context"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/engine/verdict"
	"test-auto-pro-v2/internal/model"
)

// actorSwitchTarget 模拟目标返回实时待办的真实处理人（currentPendingUserId），
// 并记录任务读取与写请求分别使用的会话 SID。
type actorSwitchTarget struct {
	*fakeTarget
	ownerID       string
	ownerName     string
	accounts      map[string]string
	accountsRead  bool
	taskReadSIDs  []string
	writeSIDs     []string
	writeAccounts []string
}

// FindTaskSnapshot 每次任务读取都返回带实际处理人的任务快照；审批完成后待办消失。
func (t *actorSwitchTarget) FindTaskSnapshot(_ context.Context, session target.Session, _, _, _ string) (target.TaskSnapshot, error) {
	t.taskReadSIDs = append(t.taskReadSIDs, session.SID)
	if t.fakeTarget.audited {
		return target.TaskSnapshot{}, nil
	}
	return target.TaskSnapshot{
		JobTaskID: "task-1", FlowNodeProxyID: "node-audit", FlowProxyID: "proxy-1",
		PendingUserID: t.ownerID, PendingUserName: t.ownerName,
	}, nil
}

// UserAccountsByID 是人员目录账号解析假件：ownerID → zhangsan。
func (t *actorSwitchTarget) UserAccountsByID(_ context.Context, _ target.Session, ids []string) (map[string]string, error) {
	t.accountsRead = true
	out := map[string]string{}
	for _, id := range ids {
		if account, ok := t.accounts[id]; ok {
			out[id] = account
		}
	}
	return out, nil
}

// AuditCurrentTask 记录审批写请求使用的会话与账号。
func (t *actorSwitchTarget) AuditCurrentTask(ctx context.Context, session target.Session, request target.AuditCurrentTaskRequest) (*target.AuditCurrentTaskResult, target.WriteResponse, string, error) {
	t.writeSIDs = append(t.writeSIDs, session.SID)
	t.writeAccounts = append(t.writeAccounts, session.Summary.Account)
	return t.fakeTarget.AuditCurrentTask(ctx, session, request)
}

// accountRecordingSessions 记录每一次按账号取会话的请求。
type accountRecordingSessions struct {
	requested []string
}

// Current 返回以账号派生 SID 的会话，便于断言写请求用的是谁的会话。
func (s *accountRecordingSessions) Current(_ context.Context, account string) (target.Session, error) {
	s.requested = append(s.requested, account)
	return target.Session{SID: "sid-" + account, Summary: target.AccountSummary{Account: account, DisplayName: account}}, nil
}

// Refresh 与 Current 同形。
func (s *accountRecordingSessions) Refresh(_ context.Context, account string) (target.Session, error) {
	return s.Current(nil, account)
}

// TestApproveRunsAsRealtimeTaskAssignee 锁定人员配置驱动执行的核心契约：
// 审批写请求必须以目标实时待办的真实处理人（currentPendingUserId → 登录账号）会话发出，
// 不能冒用计划账号审批他人任务。这是"节点人员配置真正驱动执行"的最小事实标准。
func TestApproveRunsAsRealtimeTaskAssignee(t *testing.T) {
	targetFake := &actorSwitchTarget{
		fakeTarget: &fakeTarget{
			instance:    fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-audit"}, DueNodes: []string{"node-audit"}},
			afterAudit:  &fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-next"}},
			auditResult: &target.AuditCurrentTaskResult{InstanceID: "instance-9", Status: "run"},
		},
		ownerID:   "user-b",
		ownerName: "张三",
		accounts:  map[string]string{"user-b": "zhangsan"},
	}
	sessions := &accountRecordingSessions{}
	executor := step.NewExecutor(targetFake, sessions, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)
	runCtx := newRunContext([]model.CompiledActionStep{approveStep()})
	runCtx.PathRun.MainInstanceRef = "instance-9"

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil || preview.BlockReason != "" {
		t.Fatalf("审批预览构造失败：finished=%v err=%v block=%s", finished, err, previewBlock(preview))
	}
	outcome, _, err := executor.RunApprovedStep(context.Background(), step.ApprovedStep{RunCtx: runCtx, Preview: preview, NextIndex: 0})
	if err != nil {
		t.Fatalf("审批执行失败：%v", err)
	}
	if outcome.Verdict != string(verdict.OutcomeSucceeded) {
		t.Fatalf("审批应确定成功，实际 %+v", outcome)
	}
	if !targetFake.accountsRead {
		t.Fatal("没有调用人员目录解析实际处理人账号")
	}
	foundActor := false
	for _, account := range sessions.requested {
		if account == "zhangsan" {
			foundActor = true
		}
	}
	if !foundActor {
		t.Fatalf("应按实际处理人账号取得会话：%v", sessions.requested)
	}
	if len(targetFake.writeSIDs) != 1 || targetFake.writeSIDs[0] != "sid-zhangsan" {
		t.Fatalf("写请求必须用实际处理人的会话发出：%v", targetFake.writeSIDs)
	}
	if len(targetFake.writeAccounts) != 1 || targetFake.writeAccounts[0] != "zhangsan" {
		t.Fatalf("写请求账号必须是实际处理人：writes=%v accounts=%v taskReads=%v requested=%v verdict=%s", targetFake.writeSIDs, targetFake.writeAccounts, targetFake.taskReadSIDs, sessions.requested, outcome.Verdict)
	}
}
