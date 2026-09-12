// F-031/T03 定向验证：当前节点处理人只能由「已发」列表的 currentAuditUserInfo 事实确定。
// 配置里的 NextNodeAuditors 只表示下一次提交/审批要发送的选人参数，不是当前节点已经产生的
// 待办处理人；它绝不能被当成当前处理人，也不能触发对配置候选人的登录与任务扫描。
package executor_test

import (
	"context"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// handlerFactTarget 提供实例事实（含 currentAuditUserInfo 处理人）与视角查询能力：
// 计划账号自己没有待办，node-audit 的当前待办属于 user-2。
type handlerFactTarget struct {
	*fakeTarget
	// handlers 是实例事实里的当前节点处理人（来自 currentAuditUserInfo）。
	handlers []target.NodeCurrentHandler
	// viewCalls 记录每次视角查询使用的 queryUserId。
	viewCalls []string
	// snapshotCalls 记录按会话直接读取任务快照的次数（计划账号视角）。
	snapshotCalls int
}

// FindSubmittedFlowFacts 返回带 currentAuditUserInfo 处理人的实例事实。
func (t *handlerFactTarget) FindSubmittedFlowFacts(_ context.Context, _ target.Session, _ string) (target.SubmittedFlowFacts, error) {
	return target.SubmittedFlowFacts{
		FlowProxyID: "flow-proxy-1", CurrentNodes: []string{"node-audit"}, Status: "run",
		Handlers: t.handlers, Found: true,
	}, nil
}

// MatchHandlerAccounts 解析待处理人员的登录账号：user-2 → lisi。
func (t *handlerFactTarget) MatchHandlerAccounts(_ context.Context, _ target.Session, handler target.NodeCurrentHandler) ([]target.HandlerAccount, error) {
	result := []target.HandlerAccount{}
	for _, bizID := range handler.BizIDs {
		if strings.TrimSpace(bizID) == "user-2" {
			// 同一处理人同时命中用户 ID 与姓名两条键：调用方必须按用户 ID 去重后再查任务。
			result = append(result,
				target.HandlerAccount{Key: "id:user-2", UserID: "user-2", Name: "李四", Account: "lisi"},
				target.HandlerAccount{Key: "name:李四", UserID: "user-2", Name: "李四", Account: "lisi"},
			)
		}
	}
	return result, nil
}

// ListTaskSnapshotsForUser 按视角用户返回待办：只有 user-2 在 node-audit 上有任务。
func (t *handlerFactTarget) ListTaskSnapshotsForUser(_ context.Context, _ target.Session, _, _, queryUserID string) ([]target.TaskSnapshot, error) {
	t.viewCalls = append(t.viewCalls, queryUserID)
	if queryUserID == "user-2" {
		return []target.TaskSnapshot{{
			JobTaskID: "task-fact", FlowNodeProxyID: "node-audit", FlowProxyID: "flow-proxy-1",
			FlowInstanceID: "instance-9", TaskStatus: "pending",
		}}, nil
	}
	return nil, nil
}

// FindTaskSnapshot 表示计划账号本人没有待办；正确实现不得把这次落空当成需要扫描配置候选的信号。
func (t *handlerFactTarget) FindTaskSnapshot(_ context.Context, _ target.Session, _, _, _ string) (target.TaskSnapshot, error) {
	t.snapshotCalls++
	return target.TaskSnapshot{}, nil
}

// recordingSessions 记录每次会话获取的账号，用于断言只有命中者才登录。
type recordingSessions struct {
	requested []string
}

// Current 记录账号并返回以账号派生 SID 的会话。
func (s *recordingSessions) Current(_ context.Context, account string) (target.Session, error) {
	s.requested = append(s.requested, account)
	return target.Session{SID: "sid-" + account, UserID: "user-2", Summary: target.AccountSummary{Account: account, DisplayName: account}}, nil
}

// Refresh 与 Current 同形。
func (s *recordingSessions) Refresh(ctx context.Context, account string) (target.Session, error) {
	return s.Current(ctx, account)
}

// factRunContext 构造一步审批的上下文，并按需带上配置的下一节点候选人。
func factRunContext(candidates []target.NextAuditor) step.RunContext {
	runCtx := newRunContext([]model.CompiledActionStep{approveStep()})
	runCtx.PathRun.MainInstanceRef = "instance-9"
	if len(candidates) > 0 {
		runCtx.NextNodeAuditors = map[string][]target.NextAuditor{"node-audit": candidates}
	}
	return runCtx
}

// TestCurrentHandlerComesFromFactsNotConfiguredCandidates 锁定 F-031/T03 的查询顺序：
// 事实里的当前处理人（user-2）优先；配置的下一节点候选人（user-1）既不查询也不登录，
// 同一处理人的多条匹配键只产生一次任务查询。
func TestCurrentHandlerComesFromFactsNotConfiguredCandidates(t *testing.T) {
	viewTarget := &handlerFactTarget{
		fakeTarget: &fakeTarget{
			instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-audit"}, DueNodes: []string{"node-audit"}},
		},
		handlers: []target.NodeCurrentHandler{{NodeID: "node-audit", AuditType: "run_node_choose", BizIDs: []string{"user-2"}}},
	}
	sessions := &recordingSessions{}
	executor := step.NewExecutor(viewTarget, sessions, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)
	runCtx := factRunContext([]target.NextAuditor{{BizID: "user-1", Name: "张三"}})

	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if preview == nil {
		t.Fatal("预览不应为空")
	}
	// 只查事实里的真实处理人：配置候选人与重复匹配键都不产生任务查询。
	if len(viewTarget.viewCalls) != 1 || viewTarget.viewCalls[0] != "user-2" {
		t.Fatalf("只应按事实处理人查询一次待办，实际 %v", viewTarget.viewCalls)
	}
	logins := strings.Join(sessions.requested, ",")
	if strings.Contains(logins, "lisi") == false {
		t.Fatalf("命中处理人必须登录以供后续读取，实际 %v", sessions.requested)
	}
	if strings.Contains(logins, "zhangsan") {
		t.Fatalf("配置候选人不得登录，实际 %v", sessions.requested)
	}
	if preview.Facts.CurrentTaskAssigneeID != "user-2" || preview.Facts.CurrentTaskAssigneeName != "李四" {
		t.Fatalf("当前处理人事实没有回填到预览：%+v", preview.Facts)
	}
	if preview.Facts.CurrentTaskRead != true || preview.Facts.CurrentTaskFound != true {
		t.Fatalf("当前待办应读到并命中：%+v", preview.Facts)
	}
}

// TestMissingCurrentAuditUserInfoStopsWithExplanation 锁定事实缺失时的行为：
// 目标响应缺少 currentAuditUserInfo（没有本节点处理人）时给出可解释原因并停在本步，
// 绝不用配置的下一节点候选人冒充当前处理人。
func TestMissingCurrentAuditUserInfoStopsWithExplanation(t *testing.T) {
	viewTarget := &handlerFactTarget{
		fakeTarget: &fakeTarget{
			instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-audit"}, DueNodes: []string{"node-audit"}},
		},
		handlers: nil,
	}
	sessions := &recordingSessions{}
	executor := step.NewExecutor(viewTarget, sessions, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)
	runCtx := factRunContext([]target.NextAuditor{{BizID: "user-1", Name: "张三"}})

	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if preview == nil {
		t.Fatal("预览不应为空")
	}
	if len(viewTarget.viewCalls) != 0 {
		t.Fatalf("事实缺失时不得按配置候选人扫描待办，实际 %v", viewTarget.viewCalls)
	}
	if strings.Contains(strings.Join(sessions.requested, ","), "zhangsan") {
		t.Fatalf("事实缺失时不得登录配置候选人，实际 %v", sessions.requested)
	}
	if preview.Facts.CurrentTaskFound {
		t.Fatalf("没有处理人事实时不应声称找到当前待办：%+v", preview.Facts)
	}
	if !strings.Contains(preview.Facts.AssigneeDiag, "currentAuditUserInfo") {
		t.Fatalf("缺失字段必须有可解释原因，实际 %q", preview.Facts.AssigneeDiag)
	}
	if strings.TrimSpace(preview.BlockReason) == "" && preview.GateAllowed {
		t.Fatalf("事实缺失必须停在当前步骤而不是放行：%+v", preview)
	}
}
