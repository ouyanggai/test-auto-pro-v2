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

func (r *memoryExecutionPathRepository) List(context.Context, uint64) ([]model.ExecutionPath, error) {
	return append([]model.ExecutionPath(nil), r.paths...), nil
}
func (r *memoryExecutionPathRepository) Create(_ context.Context, planID uint64, _ string, choices []model.ExecutionPathChoice, now time.Time) (model.ExecutionPath, bool, error) {
	r.createCalls++
	if r.createErr != nil {
		return model.ExecutionPath{}, false, r.createErr
	}
	path := model.ExecutionPath{ID: 1, PlanID: planID, SequenceNo: 1, Choices: choices, CreatedAt: now, UpdatedAt: now}
	r.paths = append(r.paths, path)
	return path, true, nil
}
func (r *memoryExecutionPathRepository) Update(_ context.Context, planID, pathID uint64, choices []model.ExecutionPathChoice, now time.Time) (model.ExecutionPath, error) {
	if r.updateErr != nil {
		return model.ExecutionPath{}, r.updateErr
	}
	return model.ExecutionPath{ID: pathID, PlanID: planID, SequenceNo: 1, Choices: choices, UpdatedAt: now}, nil
}
func (r *memoryExecutionPathRepository) Delete(context.Context, uint64, uint64, time.Time) error {
	return r.deleteErr
}

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
