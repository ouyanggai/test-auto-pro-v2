package backend_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/logging"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// scopeCapturingGraphReader 记录后台路径解析读取真实图时携带的日志作用域。
type scopeCapturingGraphReader struct {
	graph model.FlowGraph
	mu    sync.Mutex
	scope logging.Scope
	seen  chan struct{}
	once  sync.Once
}

// Get 抓取后台协程实际拿到的作用域，用来证明计划归属没有在起协程时丢掉。
func (r *scopeCapturingGraphReader) Get(ctx context.Context, _ uint64) (model.FlowGraph, error) {
	r.mu.Lock()
	r.scope = logging.ScopeFrom(ctx)
	r.mu.Unlock()
	r.once.Do(func() { close(r.seen) })
	return r.graph, nil
}

// captured 等待后台任务真正读过一次图，再返回它当时的作用域。
func (r *scopeCapturingGraphReader) captured(t *testing.T) logging.Scope {
	t.Helper()
	select {
	case <-r.seen:
	case <-time.After(5 * time.Second):
		t.Fatal("后台路径解析没有读取真实图")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.scope
}

// newScopedGenerationService 组装一个可观察后台作用域的路径解析服务。
func newScopedGenerationService(planName string) (*service.ExecutionPathService, *scopeCapturingGraphReader) {
	plans := newMemoryPlanRepository()
	// 只有新发起计划允许自动解析全部路径，夹具必须给出真实的 FlowSource。
	plans.plans = []model.Plan{{ID: 7, Name: planName, FlowSource: "new", Status: model.PlanStatusNotStarted}}
	graphs := &scopeCapturingGraphReader{graph: selectableExecutionPathGraph(), seen: make(chan struct{})}
	return service.NewExecutionPathService(
		service.NewPlanService(plans), graphs, analyzer.NewExecutionPathAnalyzer(), &memoryExecutionPathRepository{},
	), graphs
}

// TestPathGenerationWorkerKeepsRequestLogScope 验证后台全路径解析继承请求的日志作用域。
// 后台协程用 context.Background() 起，只传 planID 会让它产生的目标请求日志掉进应用程序目录。
func TestPathGenerationWorkerKeepsRequestLogScope(t *testing.T) {
	serviceUnderTest, graphs := newScopedGenerationService("员工请假单（集团）")
	ctx := logging.WithScope(context.Background(), logging.Scope{
		RequestID: "req-generation", PlanID: "7", PlanName: "员工请假单（集团）",
	})
	const jobKey = "123e4567-e89b-12d3-a456-426614174901"
	if _, err := serviceUnderTest.StartGeneration(ctx, 7, jobKey); err != nil {
		t.Fatalf("启动后台路径解析失败：%v", err)
	}
	defer func() { _ = serviceUnderTest.CancelGeneration(context.Background(), 7, jobKey) }()
	scope := graphs.captured(t)
	if scope.PlanID != "7" || scope.PlanName != "员工请假单（集团）" {
		t.Fatalf("后台任务丢失了计划归属：%+v", scope)
	}
	if scope.RequestID != "req-generation" {
		t.Fatalf("后台任务丢失了发起请求标识：%+v", scope)
	}
	if !scope.HasPlan() {
		t.Fatalf("后台任务日志会降级到应用程序目录：%+v", scope)
	}
}

// TestPathGenerationWorkerRecoversPlanNameWithoutRequestScope 验证请求作用域缺失时
// 后台任务仍从计划记录补上计划归属，日志不会因为上下文没接通而落错目录。
func TestPathGenerationWorkerRecoversPlanNameWithoutRequestScope(t *testing.T) {
	serviceUnderTest, graphs := newScopedGenerationService("同名计划")
	const jobKey = "123e4567-e89b-12d3-a456-426614174902"
	if _, err := serviceUnderTest.StartGeneration(context.Background(), 7, jobKey); err != nil {
		t.Fatalf("启动后台路径解析失败：%v", err)
	}
	defer func() { _ = serviceUnderTest.CancelGeneration(context.Background(), 7, jobKey) }()
	scope := graphs.captured(t)
	if scope.PlanID != "7" || scope.PlanName != "同名计划" {
		t.Fatalf("后台任务没有从计划记录补上归属：%+v", scope)
	}
}
