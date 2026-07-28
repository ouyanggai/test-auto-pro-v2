package backend_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	"test-auto-pro-v2/internal/service"
)

type executionPathGraphReader struct {
	graph model.FlowGraph
	err   error
	calls int
}

// Get 记录服务重读次数并返回预设真实图。
func (r *executionPathGraphReader) Get(context.Context, uint64) (model.FlowGraph, error) {
	r.calls++
	return r.graph, r.err
}

type memoryExecutionPathRepository struct {
	paths       []model.ExecutionPath
	createKeys  map[string]uint64
	createErr   error
	updateErr   error
	deleteErr   error
	findCalls   int
	createCalls int
	updateCalls int
}

// List 返回内存路径副本供服务单元测试使用。
func (r *memoryExecutionPathRepository) List(context.Context, uint64) ([]model.ExecutionPath, error) {
	return append([]model.ExecutionPath(nil), r.paths...), nil
}

// FindByCreateKey 只返回同一计划内已成功写入的内存幂等记录。
func (r *memoryExecutionPathRepository) FindByCreateKey(_ context.Context, planID uint64, createKey string) (model.ExecutionPath, bool, error) {
	r.findCalls++
	pathID, found := r.createKeys[createKey]
	if !found {
		return model.ExecutionPath{}, false, nil
	}
	for _, path := range r.paths {
		if path.ID == pathID && path.PlanID == planID {
			path.Choices = append([]model.ExecutionPathChoice(nil), path.Choices...)
			return path, true, nil
		}
	}
	return model.ExecutionPath{}, false, nil
}

// Create 记录写入次数并模拟事务仓储创建结果。
func (r *memoryExecutionPathRepository) Create(_ context.Context, planID uint64, createKey string, choices []model.ExecutionPathChoice, now time.Time) (model.ExecutionPath, bool, error) {
	r.createCalls++
	if r.createErr != nil {
		return model.ExecutionPath{}, false, r.createErr
	}
	path := model.ExecutionPath{ID: uint64(len(r.paths) + 1), PlanID: planID, SequenceNo: uint(len(r.paths) + 1), Choices: append([]model.ExecutionPathChoice(nil), choices...), CreatedAt: now, UpdatedAt: now}
	r.paths = append(r.paths, path)
	if r.createKeys == nil {
		r.createKeys = make(map[string]uint64)
	}
	r.createKeys[createKey] = path.ID
	return path, true, nil
}

// Update 模拟原位替换选择并保留稳定序号。
func (r *memoryExecutionPathRepository) Update(_ context.Context, planID, pathID uint64, choices []model.ExecutionPathChoice, now time.Time) (model.ExecutionPath, error) {
	r.updateCalls++
	if r.updateErr != nil {
		return model.ExecutionPath{}, r.updateErr
	}
	return model.ExecutionPath{ID: pathID, PlanID: planID, SequenceNo: 1, Choices: choices, UpdatedAt: now}, nil
}

// TestExecutionPathServiceRejectsChoicesAfterCurrentGraphChanges 验证保存前真实图变化会阻止旧选择写入。
func TestExecutionPathServiceRejectsChoicesAfterCurrentGraphChanges(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration}}
	reader := &executionPathGraphReader{graph: selectableExecutionPathGraph()}
	repo := &memoryExecutionPathRepository{}
	serviceUnderTest := service.NewExecutionPathService(service.NewPlanService(plans), reader, analyzer.NewExecutionPathAnalyzer(), repo)
	choices := []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}
	created, _, err := serviceUnderTest.Create(context.Background(), 7, "123e4567-e89b-12d3-a456-426614174301", choices)
	if err != nil {
		t.Fatalf("准备已有路径失败：%v", err)
	}
	reader.graph.Edges = []model.FlowGraphEdge{
		{ID: "start-route", Source: "start", Target: "route", Kind: "sequence"},
		{ID: "branch-b-edge", Source: "route", Target: "end-b", Kind: "condition", BranchID: "branch-b"},
	}
	_, err = serviceUnderTest.Update(context.Background(), 7, created.ID, choices)
	if !service.IsExecutionPathErrorKind(err, service.ExecutionPathErrorInvalid) || repo.updateCalls != 0 {
		t.Fatalf("真实图变化后旧选择仍进入仓储：updates=%d err=%v", repo.updateCalls, err)
	}
}

// Delete 返回预设仓储错误以覆盖删除边界。
func (r *memoryExecutionPathRepository) Delete(context.Context, uint64, uint64, time.Time) error {
	return r.deleteErr
}

// TestExecutionPathServiceRereadsAndValidatesCurrentGraph 验证每次创建和更新都重读当前图。
func TestExecutionPathServiceRereadsAndValidatesCurrentGraph(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration}}
	graph := selectableExecutionPathGraph()
	graphs := &executionPathGraphReader{graph: graph}
	repo := &memoryExecutionPathRepository{}
	serviceUnderTest := service.NewExecutionPathService(service.NewPlanService(plans), graphs, analyzer.NewExecutionPathAnalyzer(), repo)
	choices := []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}
	path, created, err := serviceUnderTest.Create(context.Background(), 7, "123e4567-e89b-12d3-a456-426614174301", choices)
	if err != nil || !created || path.SequenceNo != 1 || graphs.calls != 1 || repo.createCalls != 1 {
		t.Fatalf("创建路径没有重读并验证当前图：path=%+v created=%v calls=%d err=%v", path, created, graphs.calls, err)
	}
	if _, err := serviceUnderTest.Update(context.Background(), 7, path.ID, choices); err != nil || graphs.calls != 2 {
		t.Fatalf("更新路径没有再次读取当前图：calls=%d err=%v", graphs.calls, err)
	}
}

// TestExecutionPathServiceIdempotentRetrySkipsChangedGraph 验证已成功请求重试不会再次依赖目标图。
func TestExecutionPathServiceIdempotentRetrySkipsChangedGraph(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration}}
	graphs := &executionPathGraphReader{graph: selectableExecutionPathGraph()}
	repo := &memoryExecutionPathRepository{}
	serviceUnderTest := service.NewExecutionPathService(service.NewPlanService(plans), graphs, analyzer.NewExecutionPathAnalyzer(), repo)
	key := "123e4567-e89b-12d3-a456-426614174301"
	choices := []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}
	first, created, err := serviceUnderTest.Create(context.Background(), 7, key, choices)
	if err != nil || !created {
		t.Fatalf("首次创建路径失败：created=%v err=%v", created, err)
	}
	graphs.err = errors.New("目标平台随后不可用")
	retried, created, err := serviceUnderTest.Create(context.Background(), 7, key, nil)
	if err != nil || created || retried.ID != first.ID || retried.SequenceNo != first.SequenceNo {
		t.Fatalf("幂等重试没有返回原路径：first=%+v retried=%+v created=%v err=%v", first, retried, created, err)
	}
	if graphs.calls != 1 || repo.createCalls != 1 || len(repo.paths) != 1 {
		t.Fatalf("幂等重试重复读取或写入：graphCalls=%d createCalls=%d paths=%d", graphs.calls, repo.createCalls, len(repo.paths))
	}
}

// TestExecutionPathServiceIdempotencyDoesNotLeakAcrossPlans 验证计划范围之外的相同键不能返回其他计划路径。
func TestExecutionPathServiceIdempotencyDoesNotLeakAcrossPlans(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{
		{ID: 7, Status: model.PlanStatusPendingConfiguration},
		{ID: 8, Status: model.PlanStatusPendingConfiguration},
	}
	graphs := &executionPathGraphReader{graph: selectableExecutionPathGraph()}
	repo := &memoryExecutionPathRepository{}
	serviceUnderTest := service.NewExecutionPathService(service.NewPlanService(plans), graphs, analyzer.NewExecutionPathAnalyzer(), repo)
	key := "123e4567-e89b-12d3-a456-426614174301"
	choices := []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}
	if _, _, err := serviceUnderTest.Create(context.Background(), 7, key, choices); err != nil {
		t.Fatalf("准备其他计划幂等记录失败：%v", err)
	}
	graphs.err = errors.New("用于证明未错误命中其他计划")
	other, _, err := serviceUnderTest.Create(context.Background(), 8, key, choices)
	if err == nil || other.ID != 0 {
		t.Fatalf("其他计划泄露了幂等记录：path=%+v err=%v", other, err)
	}
	if graphs.calls != 2 {
		t.Fatalf("其他计划错误命中幂等记录：graphCalls=%d", graphs.calls)
	}
}

// TestExecutionPathServiceRejectsIncompleteAndExtraSelections 验证无效选择不会进入仓储。
func TestExecutionPathServiceRejectsIncompleteAndExtraSelections(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration}}
	for _, choices := range [][]model.ExecutionPathChoice{
		{},
		{{RouteNodeID: "route", BranchID: "missing"}},
		{{RouteNodeID: "route", BranchID: "branch-a"}, {RouteNodeID: "other", BranchID: "branch-x"}},
	} {
		repo := &memoryExecutionPathRepository{}
		serviceUnderTest := service.NewExecutionPathService(service.NewPlanService(plans), &executionPathGraphReader{graph: selectableExecutionPathGraph()}, analyzer.NewExecutionPathAnalyzer(), repo)
		_, _, err := serviceUnderTest.Create(context.Background(), 7, "123e4567-e89b-12d3-a456-426614174301", choices)
		if !service.IsExecutionPathErrorKind(err, service.ExecutionPathErrorInvalid) || repo.createCalls != 0 {
			t.Fatalf("无效选择进入了仓储：choices=%v err=%v", choices, err)
		}
	}
}

// TestExecutionPathServiceMapsRepositoryBoundaries 验证事务错误映射为稳定业务种类。
func TestExecutionPathServiceMapsRepositoryBoundaries(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration}}
	choices := []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}
	tests := []struct {
		err  error
		kind service.ExecutionPathErrorKind
	}{
		{err: repository.ErrExecutionPathLimit, kind: service.ExecutionPathErrorLimit},
		{err: repository.ErrExecutionPathPlanLocked, kind: service.ExecutionPathErrorLocked},
		{err: errors.New("database unavailable"), kind: service.ExecutionPathErrorStorage},
	}
	for _, test := range tests {
		repo := &memoryExecutionPathRepository{createErr: test.err}
		serviceUnderTest := service.NewExecutionPathService(service.NewPlanService(plans), &executionPathGraphReader{graph: selectableExecutionPathGraph()}, analyzer.NewExecutionPathAnalyzer(), repo)
		_, _, err := serviceUnderTest.Create(context.Background(), 7, "123e4567-e89b-12d3-a456-426614174301", choices)
		if !service.IsExecutionPathErrorKind(err, test.kind) {
			t.Fatalf("仓储错误映射不正确：source=%v mapped=%v", test.err, err)
		}
	}
}

// selectableExecutionPathGraph 返回包含一个条件路由的最小当前图。
func selectableExecutionPathGraph() model.FlowGraph {
	return model.FlowGraph{
		EntryNodeIDs: []string{"start"},
		Nodes: []model.FlowGraphNode{
			{ID: "start", Type: "start"}, {ID: "route", Type: "condition"},
			{ID: "end-a", Type: "end"}, {ID: "end-b", Type: "end"},
		},
		Edges: []model.FlowGraphEdge{
			{ID: "start-route", Source: "start", Target: "route", Kind: "sequence"},
			{ID: "branch-a-edge", Source: "route", Target: "end-a", Kind: "condition", BranchID: "branch-a"},
			{ID: "branch-b-edge", Source: "route", Target: "end-b", Kind: "condition", BranchID: "branch-b"},
		},
	}
}
