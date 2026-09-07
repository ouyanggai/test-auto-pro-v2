package executor_test

import (
	"context"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/engine/verdict"
	"test-auto-pro-v2/internal/model"
)

// rotatingSessions 模拟缓存会话失效后强制登录得到新 SID。
type rotatingSessions struct {
	current      target.Session
	refreshed    target.Session
	refreshCalls int
}

// Current 返回执行开始时的缓存会话。
func (s *rotatingSessions) Current(context.Context, string) (target.Session, error) {
	return s.current, nil
}

// Refresh 返回目标明确拒绝旧会话后取得的新会话。
func (s *rotatingSessions) Refresh(context.Context, string) (target.Session, error) {
	s.refreshCalls++
	return s.refreshed, nil
}

// sessionContinuityTarget 只允许新 SID 完成写后重读，用来复现旧 SID 导致的长时间重读失败。
type sessionContinuityTarget struct {
	*fakeTarget
	staleSID        string
	freshSID        string
	submitSIDs      []string
	auditSIDs       []string
	dueTaskReadSIDs []string
	factReadSIDs    []string
}

// FindSubmittedFlow 在写已生效后拒绝旧 SID，保证用例能识别事实重读是否沿用成功写会话。
func (t *sessionContinuityTarget) FindSubmittedFlow(ctx context.Context, session target.Session, instanceID string) (string, []string, string, []string, bool, error) {
	t.factReadSIDs = append(t.factReadSIDs, session.SID)
	if (t.submitted || t.audited) && session.SID != t.freshSID {
		return "", nil, "", nil, false, target.NewError(target.ErrorSessionExpired, nil)
	}
	return t.fakeTarget.FindSubmittedFlow(ctx, session, instanceID)
}

// FindDueTaskID 模拟发送审批前旧 SID 失效，并记录重取任务时使用的会话。
func (t *sessionContinuityTarget) FindDueTaskID(ctx context.Context, session target.Session, instanceID, nodeProxyID string) (string, error) {
	t.dueTaskReadSIDs = append(t.dueTaskReadSIDs, session.SID)
	if session.SID == t.staleSID {
		return "", target.NewError(target.ErrorSessionExpired, nil)
	}
	return t.fakeTarget.FindDueTaskID(ctx, session, instanceID, nodeProxyID)
}

// SubmitFlowInstance 模拟旧 SID 在进入业务前被拒绝，新 SID 才真正完成发起。
func (t *sessionContinuityTarget) SubmitFlowInstance(ctx context.Context, session target.Session, request target.SubmitFlowInstanceRequest) (*target.SubmitFlowInstanceResult, target.WriteResponse, string, error) {
	t.submitSIDs = append(t.submitSIDs, session.SID)
	if session.SID == t.staleSID {
		return nil, target.WriteResponse{StatusCode: 401}, "trace-session-expired", target.NewError(target.ErrorSessionExpired, nil)
	}
	return t.fakeTarget.SubmitFlowInstance(ctx, session, request)
}

// AuditCurrentTask 记录审批真正发送时的 SID，并复用基础假件推进目标事实。
func (t *sessionContinuityTarget) AuditCurrentTask(ctx context.Context, session target.Session, request target.AuditCurrentTaskRequest) (*target.AuditCurrentTaskResult, target.WriteResponse, string, error) {
	t.auditSIDs = append(t.auditSIDs, session.SID)
	return t.fakeTarget.AuditCurrentTask(ctx, session, request)
}

// TestF016SubmitRereadsWithSuccessfulWriteSession 锁定会话恢复后的事实重读会话：
// 旧实现虽用新 SID 重发成功，却仍用旧 SID 重读，按完整退避耗尽后误入待对账。
func TestF016SubmitRereadsWithSuccessfulWriteSession(t *testing.T) {
	targetFake := &sessionContinuityTarget{
		fakeTarget: &fakeTarget{
			submitResult: &target.SubmitFlowInstanceResult{InstanceID: "instance-9", Status: "run"},
			afterSubmit:  &fakeTargetView{Found: true, Status: "run"},
		},
		staleSID: "sid-old",
		freshSID: "sid-new",
	}
	sessions := &rotatingSessions{
		current:   target.Session{SID: "sid-old", Summary: target.AccountSummary{Account: "oyg-test"}},
		refreshed: target.Session{SID: "sid-new", Summary: target.AccountSummary{Account: "oyg-test"}},
	}
	state := &fakeRunState{}
	facts := &fakeFacts{}
	executor := step.NewExecutor(targetFake, sessions, state, facts, fixedRunConfig(), nil)
	runCtx := newRunContext([]model.CompiledActionStep{submitStep()})

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil || preview.BlockReason != "" {
		t.Fatalf("发起预览构造失败：finished=%v err=%v block=%s", finished, err, previewBlock(preview))
	}
	outcome, _, err := executor.RunApprovedStep(context.Background(), step.ApprovedStep{RunCtx: runCtx, Preview: preview, NextIndex: 0})
	if err != nil {
		t.Fatalf("发起执行失败：%v", err)
	}
	if outcome.Verdict != string(verdict.OutcomeSucceeded) {
		t.Fatalf("新 SID 写成功且事实可读时应确定成功，实际 %+v", outcome)
	}
	if len(targetFake.submitSIDs) != 2 || targetFake.submitSIDs[0] != "sid-old" || targetFake.submitSIDs[1] != "sid-new" {
		t.Fatalf("应先收到旧会话拒绝，再用新会话完成写入：%v", targetFake.submitSIDs)
	}
	if len(targetFake.factReadSIDs) == 0 || targetFake.factReadSIDs[len(targetFake.factReadSIDs)-1] != "sid-new" {
		t.Fatalf("写后事实重读必须沿用成功写入的会话：%v", targetFake.factReadSIDs)
	}
	if sessions.refreshCalls != 1 || len(state.finishedTo) != 0 || len(facts.attempts) != 1 {
		t.Fatalf("会话只应刷新一次并正常落账：refresh=%d state=%+v attempts=%d", sessions.refreshCalls, state, len(facts.attempts))
	}
}

// TestF016AuditUsesRefreshedTaskSession 锁定审批发送前的任务新鲜读取与写后重读使用同一个新 SID。
func TestF016AuditUsesRefreshedTaskSession(t *testing.T) {
	targetFake := &sessionContinuityTarget{
		fakeTarget: &fakeTarget{
			instance:    fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-audit"}, DueNodes: []string{"node-audit"}},
			afterAudit:  &fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-next"}},
			dueTaskID:   "task-new",
			auditResult: &target.AuditCurrentTaskResult{InstanceID: "instance-9", Status: "run"},
		},
		staleSID: "sid-old",
		freshSID: "sid-new",
	}
	sessions := &rotatingSessions{
		current:   target.Session{SID: "sid-old", Summary: target.AccountSummary{Account: "oyg-test"}},
		refreshed: target.Session{SID: "sid-new", Summary: target.AccountSummary{Account: "oyg-test"}},
	}
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
		t.Fatalf("重取任务后应使用同一新会话完成审批与重读，实际 %+v", outcome)
	}
	if len(targetFake.dueTaskReadSIDs) != 2 || targetFake.dueTaskReadSIDs[0] != "sid-old" || targetFake.dueTaskReadSIDs[1] != "sid-new" {
		t.Fatalf("任务应在旧会话被拒后用新会话重取：%v", targetFake.dueTaskReadSIDs)
	}
	if len(targetFake.auditSIDs) != 1 || targetFake.auditSIDs[0] != "sid-new" {
		t.Fatalf("审批请求必须使用重取任务的同一个新会话：%v", targetFake.auditSIDs)
	}
	if len(targetFake.factReadSIDs) == 0 || targetFake.factReadSIDs[len(targetFake.factReadSIDs)-1] != "sid-new" {
		t.Fatalf("审批后的事实重读必须继续使用新会话：%v", targetFake.factReadSIDs)
	}
}
