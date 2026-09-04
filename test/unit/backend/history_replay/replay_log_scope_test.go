package history_replay_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/logging"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	"test-auto-pro-v2/internal/service"
)

// scopeCapturingReplayReader 记录后台批量回放读取目标配置时携带的日志作用域。
type scopeCapturingReplayReader struct {
	snapshot target.PathConfigurationSnapshot
	mu       sync.Mutex
	scope    logging.Scope
	seen     chan struct{}
	once     sync.Once
}

// PathConfigurationSnapshot 抓取后台协程实际拿到的作用域，用来证明计划与执行路径归属没有丢。
func (r *scopeCapturingReplayReader) PathConfigurationSnapshot(ctx context.Context, _, _, _ string) (target.PathConfigurationSnapshot, error) {
	r.mu.Lock()
	r.scope = logging.ScopeFrom(ctx)
	r.mu.Unlock()
	r.once.Do(func() { close(r.seen) })
	return r.snapshot, nil
}

// captured 等待后台 worker 真正读过一次目标配置，再返回它当时的作用域。
func (r *scopeCapturingReplayReader) captured(t *testing.T) logging.Scope {
	t.Helper()
	select {
	case <-r.seen:
	case <-time.After(5 * time.Second):
		t.Fatal("后台批量回放没有读取目标配置")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.scope
}

// newScopedReplayService 组装一个可观察后台作用域的批量回放服务，计划与执行路径都有真实显示名。
func newScopedReplayService() (*service.HistoryReplayService, *scopeCapturingReplayReader) {
	plan := model.Plan{
		ID: 91, Name: "员工请假单（集团）", Account: "tester", FlowSource: "flow-source",
		TargetObjectID: "target-1", Status: model.PlanStatusNotStarted,
	}
	path := model.ExecutionPath{
		ID: 101, PlanID: plan.ID, Name: "路径 1", ConfigurationRevision: 4,
		Choices: []model.ExecutionPathChoice{{RouteNodeID: "route-1", BranchID: "branch-a"}},
	}
	tree := conditionTreeID("route-1", []target.FlowBranchTemplate{
		{ID: "branch-a", Sort: 1, Conditions: []target.FlowCondition{{FieldA: "amount", ValueB: "2", Judge: "eq"}}},
		{ID: "branch-default", Sort: 2},
	})
	store := &replayStore{
		snapshot:  model.HistorySnapshot{ID: 9, PlanID: plan.ID, RuntimeType: string(target.FormRenderTypeFormMaking), RawFormData: map[string]any{"amount": 2}},
		defaultAt: repository.HistoryDefaultRecord{PlanID: plan.ID, SnapshotID: 9},
	}
	reader := &scopeCapturingReplayReader{
		snapshot: target.PathConfigurationSnapshot{Tree: tree, RenderType: target.FormRenderTypeFormMaking},
		seen:     make(chan struct{}),
	}
	replay := service.NewHistoryReplayService(
		service.NewPlanService(&replayPlanRepo{plan: plan}), &replayPathRepo{created: path, current: path}, reader, store,
	)
	replay.SetRuntimeValidator(&replayValidator{})
	return replay, reader
}

// TestReplayWorkerKeepsPlanAndPathLogScope 验证一键配置的后台 worker 继承请求作用域，
// 并在处理每条明细时补上该执行路径的归属，日志落进这条路径自己的目录。
func TestReplayWorkerKeepsPlanAndPathLogScope(t *testing.T) {
	replay, reader := newScopedReplayService()
	ctx := logging.WithScope(context.Background(), logging.Scope{
		RequestID: "req-replay", PlanID: "91", PlanName: "员工请假单（集团）",
	})
	if _, err := replay.Create(ctx, 91, model.HistoryReplayCreateInput{PathIDs: []uint64{101}}, "123e4567-e89b-12d3-a456-426614174910"); err != nil {
		t.Fatalf("创建批量回放任务失败：%v", err)
	}
	scope := reader.captured(t)
	if scope.PlanID != "91" || scope.PlanName != "员工请假单（集团）" || scope.RequestID != "req-replay" {
		t.Fatalf("后台 worker 丢失了计划归属：%+v", scope)
	}
	if scope.ExecutionPathID != "101" || scope.ExecutionPathName != "路径 1" {
		t.Fatalf("后台 worker 没有补上执行路径归属：%+v", scope)
	}
}

// TestReplayWorkerRecoversPlanScopeWithoutRequestContext 验证启动恢复这类没有请求作用域的场景，
// 后台 worker 仍从任务的计划 ID 与计划记录补上归属，不把业务日志降级到应用程序目录。
func TestReplayWorkerRecoversPlanScopeWithoutRequestContext(t *testing.T) {
	replay, reader := newScopedReplayService()
	if _, err := replay.Create(context.Background(), 91, model.HistoryReplayCreateInput{PathIDs: []uint64{101}}, "123e4567-e89b-12d3-a456-426614174911"); err != nil {
		t.Fatalf("创建批量回放任务失败：%v", err)
	}
	scope := reader.captured(t)
	if scope.PlanID != "91" || scope.PlanName != "员工请假单（集团）" {
		t.Fatalf("后台 worker 没有从计划记录补上归属：%+v", scope)
	}
	if !scope.HasPlan() {
		t.Fatalf("后台 worker 日志会降级到应用程序目录：%+v", scope)
	}
}
