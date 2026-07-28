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
	createErr   error
	updateErr   error
	deleteErr   error
	createCalls int
}

// List 返回内存路径副本供服务单元测试使用。
func (r *memoryExecutionPathRepository) List(context.Context, uint64) ([]model.ExecutionPath, error) {
	return append([]model.ExecutionPath(nil), r.paths...), nil
}

// Create 记录写入次数并模拟事务仓储创建结果。
func (r *memoryExecutionPathRepository) Create(_ context.Context, planID uint64, _ string, choices []model.ExecutionPathChoice, now time.Time) (model.ExecutionPath, bool, error) {
	r.createCalls++
	if r.createErr != nil {
		return model.ExecutionPath{}, false, r.createErr
	}
	path := model.ExecutionPath{ID: 1, PlanID: planID, SequenceNo: 1, Choices: choices, CreatedAt: now, UpdatedAt: now}
	r.paths = append(r.paths, path)
	return path, true, nil
}

// Update 模拟原位替换选择并保留稳定序号。
func (r *memoryExecutionPathRepository) Update(_ context.Context, planID, pathID uint64, choices []model.ExecutionPathChoice, now time.Time) (model.ExecutionPath, error) {
	if r.updateErr != nil {
		return model.ExecutionPath{}, r.updateErr
	}
	return model.ExecutionPath{ID: pathID, PlanID: planID, SequenceNo: 1, Choices: choices, UpdatedAt: now}, nil
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
