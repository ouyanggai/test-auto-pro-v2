// F-030 评审 P2 定向验证：候选处理人扫描用同一计划会话按候选人用户 ID 窄查询，
// 只有命中者才登录其会话——消除「逐候选登录 + 各自完整分页扫描」的主要耗时。
package executor_test

import (
	"context"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// candidateViewTarget 支持 ListTaskSnapshotsForUser 视角查询：user-2 在 node-audit 有待办。
type candidateViewTarget struct {
	*fakeTarget
	viewCalls     []string // 每次视角查询的 queryUserID
	sessionLogins []string // 每次按账号取会话的账号（用于断言命中者才登录）
}

// ListTaskSnapshotsForUser 记录视角查询；user-2 命中 node-audit 的待办。
func (t *candidateViewTarget) ListTaskSnapshotsForUser(_ context.Context, _ target.Session, _, _, queryUserID string) ([]target.TaskSnapshot, error) {
	t.viewCalls = append(t.viewCalls, queryUserID)
	if queryUserID == "user-2" {
		return []target.TaskSnapshot{{JobTaskID: "task-9", FlowNodeProxyID: "node-audit", FlowProxyID: "proxy-1"}}, nil
	}
	return nil, nil
}

// FindTaskSnapshot 提供计划账号视角的任务读取：计划账号本人没有待办，候选发现由此触发。
func (t *candidateViewTarget) FindTaskSnapshot(_ context.Context, _ target.Session, _, _, _ string) (target.TaskSnapshot, error) {
	return target.TaskSnapshot{}, nil
}

// UserAccountsByID 提供候选账号目录：user-2 → lisi。
func (t *candidateViewTarget) UserAccountsByID(_ context.Context, _ target.Session, _ []string) (map[string]string, error) {
	return map[string]string{"user-1": "zhangsan", "user-2": "lisi"}, nil
}

// loginRecordingSessions 记录每次会话获取的账号。
type loginRecordingSessions struct {
	requested []string
}

// Current 记录账号并返回以账号派生 SID 的会话。
func (s *loginRecordingSessions) Current(_ context.Context, account string) (target.Session, error) {
	s.requested = append(s.requested, account)
	return target.Session{SID: "sid-" + account, Summary: target.AccountSummary{Account: account, DisplayName: account}}, nil
}

// Refresh 与 Current 同形。
func (s *loginRecordingSessions) Refresh(ctx context.Context, account string) (target.Session, error) {
	return s.Current(ctx, account)
}

// TestCandidateScanUsesViewQueryNotPerCandidateLogin 锁定候选扫描收敛：
// 两个候选（user-1 未命中、user-2 命中）只产生两次视角查询与一次命中者登录，
// 不再对每个候选都登录会话并完整扫描任务列表。
func TestCandidateScanUsesViewQueryNotPerCandidateLogin(t *testing.T) {
	viewTarget := &candidateViewTarget{
		fakeTarget: &fakeTarget{
			instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-audit"}, DueNodes: []string{"node-audit"}},
		},
	}
	sessions := &loginRecordingSessions{}
	executor := step.NewExecutor(viewTarget, sessions, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)
	runCtx := newRunContext([]model.CompiledActionStep{approveStep()})
	runCtx.PathRun.MainInstanceRef = "instance-9"
	runCtx.NextNodeAuditors = map[string][]target.NextAuditor{
		"node-audit": {
			{BizID: "user-1", Name: "张三"},
			{BizID: "user-2", Name: "李四"},
		},
	}
	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(viewTarget.viewCalls) != 2 {
		t.Fatalf("两个候选应各产生一次视角查询，实际 %v", viewTarget.viewCalls)
	}
	// 命中者（lisi）才登录；未命中者（zhangsan）不再登录。
	logins := map[string]bool{}
	for _, account := range sessions.requested {
		logins[account] = true
	}
	if logins["zhangsan"] {
		t.Fatalf("未命中候选不应登录会话，实际登录 %v", sessions.requested)
	}
	if !logins["lisi"] {
		t.Fatalf("命中候选应登录其会话，实际登录 %v", sessions.requested)
	}
	if preview == nil || preview.Facts.CurrentTaskAssigneeID != "user-2" || preview.Facts.CurrentTaskAssigneeName != "李四" {
		t.Fatalf("命中候选事实应回填到预览（处理人 ID/姓名），实际 %+v", preview.Facts)
	}
}
