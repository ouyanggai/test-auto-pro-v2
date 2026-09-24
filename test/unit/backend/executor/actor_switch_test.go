package executor_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/engine/verdict"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/runtimeexec"
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

// currentHandlerUserID 用本假件配置的真实处理人覆盖基础默认值，
// 事实发现路径（currentAuditUserInfo → 视角查询）因此针对的就是这位处理人。
func (t *actorSwitchTarget) currentHandlerUserID() string { return strings.TrimSpace(t.ownerID) }

// FindSubmittedFlowFacts 用本假件配置的真实处理人构造 currentAuditUserInfo：
// 基础假件的方法体按静态分派取默认处理人 ID，无法表达「待办属于另一位处理人」。
func (t *actorSwitchTarget) FindSubmittedFlowFacts(_ context.Context, _ target.Session, _ string) (target.SubmittedFlowFacts, error) {
	view := t.currentView()
	flowProxyID := t.flowProxyID
	if flowProxyID == "" {
		flowProxyID = "flow-proxy-1"
	}
	handlers := make([]target.NodeCurrentHandler, 0, len(view.DueNodes))
	for _, nodeID := range view.DueNodes {
		handlers = append(handlers, target.NodeCurrentHandler{
			NodeID: strings.TrimSpace(nodeID), AuditType: "run_node_choose",
			BizIDs: []string{t.currentHandlerUserID()},
		})
	}
	return target.SubmittedFlowFacts{
		FlowProxyID: flowProxyID, CurrentNodes: view.CurrentNodes, Status: view.Status,
		Handlers: handlers, Name: "测试实例", Found: view.Found,
	}, nil
}

// ListTaskSnapshotsForUser 让事实发现路径复用本假件的任务语义：命中处理人视角时返回实时待办。
func (t *actorSwitchTarget) ListTaskSnapshotsForUser(ctx context.Context, session target.Session, instanceID, status, queryUserID string) ([]target.TaskSnapshot, error) {
	if strings.TrimSpace(status) != "pending" || strings.TrimSpace(queryUserID) != t.currentHandlerUserID() {
		return nil, nil
	}
	snapshot, err := t.FindTaskSnapshot(ctx, session, instanceID, "", status)
	if err != nil || strings.TrimSpace(snapshot.JobTaskID) == "" {
		return nil, err
	}
	return []target.TaskSnapshot{snapshot}, nil
}

// UserAccountsByID 是人员目录账号解析假件：ownerID → zhangsan。
func (t *actorSwitchTarget) UserAccountsByID(_ context.Context, _ target.Session, ids []string) (map[string]target.UserAccount, error) {
	t.accountsRead = true
	out := map[string]target.UserAccount{}
	for _, id := range ids {
		if account, ok := t.accounts[id]; ok {
			name := t.ownerName
			if name == "" {
				name = account
			}
			out[id] = target.UserAccount{Account: account, Name: name}
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
// UserID 必须有值：runtime 装载按计划账号会话解析原发起人身份，缺 UserID 会在写前阻塞。
func (s *accountRecordingSessions) Current(_ context.Context, account string) (target.Session, error) {
	s.requested = append(s.requested, account)
	return target.Session{SID: "sid-" + account, UserID: "user-plan", Summary: target.AccountSummary{Account: account, DisplayName: account}}, nil
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

// recordingRuntimeRunner 记录引擎下发到 runtime 的每次动作请求，供账号字段断言使用。
type recordingRuntimeRunner struct {
	requests []runtimeexec.RuntimeRunRequest
	// onRun 在每次 RunAction 时回调：测试用它翻转目标假件的已审批事实。
	onRun func()
}

// Health 恒定可用：用例只关心请求字段，不关心执行服务连通性。
func (r *recordingRuntimeRunner) Health(context.Context) error { return nil }

// RunAction 记录请求并返回成功判定，让流程继续走到 Go 侧事实核验。
func (r *recordingRuntimeRunner) RunAction(_ context.Context, request runtimeexec.RuntimeRunRequest) (runtimeexec.ActionRunResult, error) {
	r.requests = append(r.requests, request)
	if r.onRun != nil {
		r.onRun()
	}
	return runtimeexec.ActionRunResult{Status: "success", RequestTrace: json.RawMessage(`[]`)}, nil
}

// TestApproveAsSelfAssigneeKeepsRuntimeAccountKey 锁定 2026-09-18 单路径真实运行的失败根因：
// 当前待办的实际处理人就是计划账号本人时，runtime 会话创建的 accountKey 不能留空——
// 执行服务按 accountKey/runId/pathRunId/stepId 强校验，空账号会让协同发起人节点的同意
// 以「工具缺陷」失败。修复后处理人等于计划账号也要回填 preview.ActorAccount。
func TestApproveAsSelfAssigneeKeepsRuntimeAccountKey(t *testing.T) {
	targetFake := &actorSwitchTarget{
		fakeTarget: &fakeTarget{
			instance:    fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-audit"}, DueNodes: []string{"node-audit"}},
			afterAudit:  &fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-next"}},
			auditResult: &target.AuditCurrentTaskResult{InstanceID: "instance-9", Status: "run"},
		},
		// 实时待办处理人解析出的登录账号 == 计划账号 oyg-test：触发「处理人=计划账号」分支。
		ownerID:   "user-self",
		ownerName: "骆蒙恩",
		accounts:  map[string]string{"user-self": "oyg-test"},
	}
	sessions := &accountRecordingSessions{}
	runtime := &recordingRuntimeRunner{onRun: func() { targetFake.fakeTarget.audited = true }}
	executor := step.NewExecutor(targetFake, sessions, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)
	executor.EnableRuntimeExecution(runtime, "")
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
		t.Fatalf("审批应确定成功，实际 %+v actorAccount=%q writeErr=%v runtimeReq=%+v", outcome, preview.ActorAccount, preview.WriteErrForTest(), runtime.requests)
	}
	if len(runtime.requests) != 1 {
		t.Fatalf("审批应恰好下发一次 runtime 动作，实际 %d 次", len(runtime.requests))
	}
	if got := strings.TrimSpace(runtime.requests[0].Account); got != "oyg-test" {
		t.Fatalf("处理人=计划账号时 runtime 请求必须携带账号，实际 %q", got)
	}
	if got := strings.TrimSpace(preview.ActorAccount); got != "oyg-test" {
		t.Fatalf("preview 执行者账号必须回填计划账号，实际 %q", got)
	}
	// 处理人就是计划账号本人：不得触发第二账号登录，写会话保持计划账号会话。
	for _, account := range sessions.requested {
		if account != "oyg-test" {
			t.Fatalf("处理人=计划账号时不应登录其他账号：%v", sessions.requested)
		}
	}
}

// failingAccountSessions 支持按账号模拟部分账号登录失败。
type failingAccountSessions struct {
	accountRecordingSessions
	failAccounts map[string]error
}

func (s *failingAccountSessions) Current(ctx context.Context, account string) (target.Session, error) {
	if err, ok := s.failAccounts[account]; ok {
		return target.Session{}, err
	}
	return s.accountRecordingSessions.Current(ctx, account)
}

func (s *failingAccountSessions) Refresh(ctx context.Context, account string) (target.Session, error) {
	return s.Current(ctx, account)
}

// multiHandlerTarget 模拟多处理人/抢签节点返回多个待处理人。
type multiHandlerTarget struct {
	*actorSwitchTarget
	allHandlers []string
}

func (t *multiHandlerTarget) FindSubmittedFlowFacts(_ context.Context, _ target.Session, _ string) (target.SubmittedFlowFacts, error) {
	view := t.currentView()
	flowProxyID := t.flowProxyID
	if flowProxyID == "" {
		flowProxyID = "flow-proxy-1"
	}
	handlers := make([]target.NodeCurrentHandler, 0, len(view.DueNodes))
	for _, nodeID := range view.DueNodes {
		handlers = append(handlers, target.NodeCurrentHandler{
			NodeID: strings.TrimSpace(nodeID), AuditType: "run_node_choose",
			BizIDs: t.allHandlers,
		})
	}
	return target.SubmittedFlowFacts{
		FlowProxyID: flowProxyID, CurrentNodes: view.CurrentNodes, Status: view.Status,
		Handlers: handlers, Name: "测试实例", Found: view.Found,
	}, nil
}

func (t *multiHandlerTarget) ListTaskSnapshotsForUser(ctx context.Context, session target.Session, instanceID, status, queryUserID string) ([]target.TaskSnapshot, error) {
	if strings.TrimSpace(status) != "pending" {
		return nil, nil
	}
	for _, h := range t.allHandlers {
		if queryUserID == h {
			return []target.TaskSnapshot{{
				JobTaskID: "task-" + h, FlowNodeProxyID: "node-audit", FlowProxyID: "proxy-1",
				PendingUserID: h, PendingUserName: h,
			}}, nil
		}
	}
	return nil, nil
}
func (t *multiHandlerTarget) MatchHandlerAccounts(_ context.Context, _ target.Session, handler target.NodeCurrentHandler) ([]target.HandlerAccount, error) {
	result := []target.HandlerAccount{}
	for _, bizID := range handler.BizIDs {
		if acc, ok := t.accounts[bizID]; ok {
			result = append(result, target.HandlerAccount{
				Key: "id:" + bizID, UserID: bizID, Name: bizID, Account: acc,
			})
		}
	}
	return result, nil
}


// TestSwitchToCurrentHandlerSkipsUnloginableCandidateAndPicksNext 验证：
// 当抢签/多处理人节点的第一个候选人无法登录（例如无权限、账号停用）时，
// 执行器不会认死该候选人导致整条路径停机，而是自动跳过并尝试下一个可用候选人推进流程。
func TestSwitchToCurrentHandlerSkipsUnloginableCandidateAndPicksNext(t *testing.T) {
	targetFake := &multiHandlerTarget{
		actorSwitchTarget: &actorSwitchTarget{
			fakeTarget: &fakeTarget{
				instance:    fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-audit"}, DueNodes: []string{"node-audit"}},
				afterAudit:  &fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-next"}},
				auditResult: &target.AuditCurrentTaskResult{InstanceID: "instance-multi", Status: "run"},
			},
			accounts: map[string]string{
				"user-unloginable": "zhaofei-bad",
				"user-available":   "libai-good",
			},
		},
		allHandlers: []string{"user-unloginable", "user-available"},
	}
	sessions := &failingAccountSessions{
		failAccounts: map[string]error{
			"zhaofei-bad": errors.New("无权限!"),
		},
	}
	executor := step.NewExecutor(targetFake, sessions, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)
	runCtx := newRunContext([]model.CompiledActionStep{approveStep()})
	runCtx.PathRun.MainInstanceRef = "instance-multi"

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil || preview.BlockReason != "" {
		t.Fatalf("审批预览应成功放行，实际 finished=%v err=%v block=%s", finished, err, previewBlock(preview))
	}
	if preview.ActorName != "user-available" {
		t.Fatalf("应自动跳过无法登录的 zhaofei-bad 并选用 user-available，实际选中：%q", preview.ActorName)
	}
	outcome, _, err := executor.RunApprovedStep(context.Background(), step.ApprovedStep{RunCtx: runCtx, Preview: preview, NextIndex: 0})
	if err != nil {
		t.Fatalf("审批执行应成功：%v", err)
	}
	if outcome.Verdict != string(verdict.OutcomeSucceeded) {
		t.Fatalf("审批应确定成功，实际 outcome=%+v", outcome)
	}
	if len(targetFake.writeAccounts) != 1 || targetFake.writeAccounts[0] != "libai-good" {
		t.Fatalf("写请求必须以可用候选人 libai-good 发出，实际：%v", targetFake.writeAccounts)
	}
}
