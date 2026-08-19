package backend_test

import (
	"context"
	"errors"
	"strings"
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
	paths           []model.ExecutionPath
	createKeys      map[string]uint64
	createErr       error
	updateErr       error
	deleteErr       error
	findCalls       int
	createCalls     int
	createStored    chan struct{}
	createRelease   chan struct{}
	updateCalls     int
	batch           model.ExecutionPathBatchResult
	batchKey        string
	batchErr        error
	batchCalls      int
	listSummaryOnly bool
	getCalls        int
	getManyCalls    int
}

// GetMany 批量返回仍存在的测试路径详情，顺序与输入保持一致。
func (r *memoryExecutionPathRepository) GetMany(_ context.Context, planID uint64, pathIDs []uint64) ([]model.ExecutionPath, error) {
	r.getManyCalls++
	result := make([]model.ExecutionPath, 0, len(pathIDs))
	for _, pathID := range pathIDs {
		path, err := r.Get(context.Background(), planID, pathID)
		if errors.Is(err, repository.ErrExecutionPathNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		result = append(result, path)
	}
	return result, nil
}

// List 返回内存路径副本供服务单元测试使用。
func (r *memoryExecutionPathRepository) List(context.Context, uint64) ([]model.ExecutionPath, error) {
	paths := append([]model.ExecutionPath(nil), r.paths...)
	if r.listSummaryOnly {
		for index := range paths {
			paths[index].Choices = nil
		}
	}
	return paths, nil
}

// Get 返回计划内单条路径及 choices，模拟进入编辑态的按需读取。
func (r *memoryExecutionPathRepository) Get(_ context.Context, planID, pathID uint64) (model.ExecutionPath, error) {
	r.getCalls++
	for _, path := range r.paths {
		if path.PlanID == planID && path.ID == pathID {
			return path, nil
		}
	}
	return model.ExecutionPath{}, repository.ErrExecutionPathNotFound
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
func (r *memoryExecutionPathRepository) Create(_ context.Context, planID uint64, createKey, name string, choices []model.ExecutionPathChoice, now time.Time) (model.ExecutionPath, bool, error) {
	r.createCalls++
	if r.createErr != nil {
		return model.ExecutionPath{}, false, r.createErr
	}
	sequenceNo := uint(len(r.paths) + 1)
	if name == "" {
		name = "路径 " + string(rune('0'+sequenceNo))
	}
	path := model.ExecutionPath{ID: uint64(len(r.paths) + 1), PlanID: planID, SequenceNo: sequenceNo, Name: name, Choices: append([]model.ExecutionPathChoice(nil), choices...), CreatedAt: now, UpdatedAt: now}
	r.paths = append(r.paths, path)
	if r.createKeys == nil {
		r.createKeys = make(map[string]uint64)
	}
	r.createKeys[createKey] = path.ID
	if r.createStored != nil {
		// 用提交后、响应前的停顿模拟客户端超时；此时同键重试必须能观察到已落库事实，而不是再次读取目标图。
		close(r.createStored)
		<-r.createRelease
	}
	return path, true, nil
}

// Update 模拟原位替换选择并保留稳定序号。
func (r *memoryExecutionPathRepository) Update(_ context.Context, planID, pathID uint64, name string, choices []model.ExecutionPathChoice, now time.Time) (model.ExecutionPath, error) {
	r.updateCalls++
	if r.updateErr != nil {
		return model.ExecutionPath{}, r.updateErr
	}
	return model.ExecutionPath{ID: pathID, PlanID: planID, SequenceNo: 1, Name: name, Choices: choices, UpdatedAt: now}, nil
}

// FindBatchByCreateKey 返回内存中已经提交的批量幂等结果。
func (r *memoryExecutionPathRepository) FindBatchByCreateKey(_ context.Context, _ uint64, createKey string) (model.ExecutionPathBatchResult, bool, error) {
	if r.batchErr != nil {
		return model.ExecutionPathBatchResult{}, false, r.batchErr
	}
	if r.batchKey == createKey {
		return r.batch, true, nil
	}
	return model.ExecutionPathBatchResult{}, false, nil
}

// GeneratePathsBatch 模拟后台任务的批量原子写入并记录完整组合数。
func (r *memoryExecutionPathRepository) GeneratePathsBatch(_ context.Context, planID uint64, createKey string, candidates [][]model.ExecutionPathChoice, now time.Time) (model.ExecutionPathBatchResult, bool, error) {
	r.batchCalls++
	if r.batchErr != nil {
		return model.ExecutionPathBatchResult{}, false, r.batchErr
	}
	items := make([]model.ExecutionPath, 0, len(candidates))
	for index, choices := range candidates {
		items = append(items, model.ExecutionPath{ID: uint64(index + 1), PlanID: planID, SequenceNo: uint(index + 1), Name: "路径 " + string(rune('1'+index)), Choices: choices, CreatedAt: now, UpdatedAt: now})
	}
	r.batchKey = createKey
	r.batch = model.ExecutionPathBatchResult{TotalCount: len(candidates), CreatedCount: len(items), Paths: items}
	return r.batch, true, nil
}

// TestExecutionPathServiceRejectsChoicesAfterCurrentGraphChanges 验证保存前真实图变化会阻止旧选择写入。
func TestExecutionPathServiceRejectsChoicesAfterCurrentGraphChanges(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusNotStarted}}
	reader := &executionPathGraphReader{graph: selectableExecutionPathGraph()}
	repo := &memoryExecutionPathRepository{}
	serviceUnderTest := service.NewExecutionPathService(service.NewPlanService(plans), reader, analyzer.NewExecutionPathAnalyzer(), repo)
	choices := []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}
	created, _, err := serviceUnderTest.Create(context.Background(), 7, "123e4567-e89b-12d3-a456-426614174301", "", choices)
	if err != nil {
		t.Fatalf("准备已有路径失败：%v", err)
	}
	reader.graph.Edges = []model.FlowGraphEdge{
		{ID: "start-route", Source: "start", Target: "route", Kind: "sequence"},
		{ID: "branch-b-edge", Source: "route", Target: "end-b", Kind: "condition", BranchID: "branch-b"},
	}
	_, err = serviceUnderTest.Update(context.Background(), 7, created.ID, "自定义路径", choices)
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
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusNotStarted}}
	graph := selectableExecutionPathGraph()
	graphs := &executionPathGraphReader{graph: graph}
	repo := &memoryExecutionPathRepository{}
	serviceUnderTest := service.NewExecutionPathService(service.NewPlanService(plans), graphs, analyzer.NewExecutionPathAnalyzer(), repo)
	choices := []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}
	path, created, err := serviceUnderTest.Create(context.Background(), 7, "123e4567-e89b-12d3-a456-426614174301", "重点路径", choices)
	if err != nil || !created || path.SequenceNo != 1 || graphs.calls != 1 || repo.createCalls != 1 {
		t.Fatalf("创建路径没有重读并验证当前图：path=%+v created=%v calls=%d err=%v", path, created, graphs.calls, err)
	}
	if _, err := serviceUnderTest.Update(context.Background(), 7, path.ID, "重点路径", choices); err != nil || graphs.calls != 2 {
		t.Fatalf("更新路径没有再次读取当前图：calls=%d err=%v", graphs.calls, err)
	}
}

// TestExecutionPathServiceIdempotentRetrySkipsChangedGraph 验证已成功请求重试不会再次依赖目标图。
func TestExecutionPathServiceIdempotentRetrySkipsChangedGraph(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusNotStarted}}
	graphs := &executionPathGraphReader{graph: selectableExecutionPathGraph()}
	repo := &memoryExecutionPathRepository{}
	serviceUnderTest := service.NewExecutionPathService(service.NewPlanService(plans), graphs, analyzer.NewExecutionPathAnalyzer(), repo)
	key := "123e4567-e89b-12d3-a456-426614174301"
	choices := []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}
	first, created, err := serviceUnderTest.Create(context.Background(), 7, key, "", choices)
	if err != nil || !created {
		t.Fatalf("首次创建路径失败：created=%v err=%v", created, err)
	}
	graphs.err = errors.New("目标平台随后不可用")
	retried, created, err := serviceUnderTest.Create(context.Background(), 7, key, "另一个名字", nil)
	if err != nil || created || retried.ID != first.ID || retried.SequenceNo != first.SequenceNo {
		t.Fatalf("幂等重试没有返回原路径：first=%+v retried=%+v created=%v err=%v", first, retried, created, err)
	}
	if graphs.calls != 1 || repo.createCalls != 1 || len(repo.paths) != 1 {
		t.Fatalf("幂等重试重复读取或写入：graphCalls=%d createCalls=%d paths=%d", graphs.calls, repo.createCalls, len(repo.paths))
	}
}

// TestExecutionPathServiceLateResponseRetryUsesCommittedPath 验证首次写入已完成但响应迟到时，同键重试直接返回原路径。
func TestExecutionPathServiceLateResponseRetryUsesCommittedPath(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusNotStarted}}
	graphs := &executionPathGraphReader{graph: selectableExecutionPathGraph()}
	repo := &memoryExecutionPathRepository{createStored: make(chan struct{}), createRelease: make(chan struct{})}
	serviceUnderTest := service.NewExecutionPathService(service.NewPlanService(plans), graphs, analyzer.NewExecutionPathAnalyzer(), repo)
	key := "123e4567-e89b-12d3-a456-426614174306"
	choices := []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}
	type createResult struct {
		path    model.ExecutionPath
		created bool
		err     error
	}
	firstResult := make(chan createResult, 1)
	go func() {
		path, created, err := serviceUnderTest.Create(context.Background(), 7, key, "迟到响应路径", choices)
		firstResult <- createResult{path: path, created: created, err: err}
	}()
	<-repo.createStored
	release := repo.createRelease
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()

	graphs.err = errors.New("首次提交后目标平台不可用")
	retried, created, err := serviceUnderTest.Create(context.Background(), 7, key, "重试名称", nil)
	if err != nil || created || retried.ID == 0 || retried.Name != "迟到响应路径" {
		t.Fatalf("迟到响应期间同键重试没有返回已提交路径：path=%+v created=%v err=%v", retried, created, err)
	}
	close(release)
	released = true
	first := <-firstResult
	if first.err != nil || !first.created || first.path.ID != retried.ID || repo.createCalls != 1 || graphs.calls != 1 || len(repo.paths) != 1 {
		t.Fatalf("迟到响应重试重复读取或写入：first=%+v retried=%+v graph=%d create=%d paths=%d", first, retried, graphs.calls, repo.createCalls, len(repo.paths))
	}
}

// TestExecutionPathServiceIdempotencyDoesNotLeakAcrossPlans 验证计划范围之外的相同键不能返回其他计划路径。
func TestExecutionPathServiceIdempotencyDoesNotLeakAcrossPlans(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{
		{ID: 7, Status: model.PlanStatusNotStarted},
		{ID: 8, Status: model.PlanStatusNotStarted},
	}
	graphs := &executionPathGraphReader{graph: selectableExecutionPathGraph()}
	repo := &memoryExecutionPathRepository{}
	serviceUnderTest := service.NewExecutionPathService(service.NewPlanService(plans), graphs, analyzer.NewExecutionPathAnalyzer(), repo)
	key := "123e4567-e89b-12d3-a456-426614174301"
	choices := []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}
	if _, _, err := serviceUnderTest.Create(context.Background(), 7, key, "", choices); err != nil {
		t.Fatalf("准备其他计划幂等记录失败：%v", err)
	}
	graphs.err = errors.New("用于证明未错误命中其他计划")
	other, _, err := serviceUnderTest.Create(context.Background(), 8, key, "", choices)
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
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusNotStarted}}
	for _, choices := range [][]model.ExecutionPathChoice{
		{},
		{{RouteNodeID: "route", BranchID: "missing"}},
		{{RouteNodeID: "route", BranchID: "branch-a"}, {RouteNodeID: "other", BranchID: "branch-x"}},
	} {
		repo := &memoryExecutionPathRepository{}
		serviceUnderTest := service.NewExecutionPathService(service.NewPlanService(plans), &executionPathGraphReader{graph: selectableExecutionPathGraph()}, analyzer.NewExecutionPathAnalyzer(), repo)
		_, _, err := serviceUnderTest.Create(context.Background(), 7, "123e4567-e89b-12d3-a456-426614174301", "", choices)
		if !service.IsExecutionPathErrorKind(err, service.ExecutionPathErrorInvalid) || repo.createCalls != 0 {
			t.Fatalf("无效选择进入了仓储：choices=%v err=%v", choices, err)
		}
	}
}

// TestExecutionPathServiceMapsRepositoryBoundaries 验证事务错误映射为稳定业务种类。
func TestExecutionPathServiceMapsRepositoryBoundaries(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusNotStarted}}
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
		_, _, err := serviceUnderTest.Create(context.Background(), 7, "123e4567-e89b-12d3-a456-426614174301", "", choices)
		if !service.IsExecutionPathErrorKind(err, test.kind) {
			t.Fatalf("仓储错误映射不正确：source=%v mapped=%v", test.err, err)
		}
	}
}

// TestExecutionPathServiceNormalizesPathName 验证路径名称去空格、长度边界和空值默认语义。
func TestExecutionPathServiceNormalizesPathName(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusNotStarted}}
	repo := &memoryExecutionPathRepository{}
	serviceUnderTest := service.NewExecutionPathService(service.NewPlanService(plans), &executionPathGraphReader{graph: selectableExecutionPathGraph()}, analyzer.NewExecutionPathAnalyzer(), repo)
	choices := []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}
	path, _, err := serviceUnderTest.Create(context.Background(), 7, "123e4567-e89b-12d3-a456-426614174301", "  重点线路  ", choices)
	if err != nil || path.Name != "重点线路" {
		t.Fatalf("自定义名称没有规范化：path=%+v err=%v", path, err)
	}
	_, _, err = serviceUnderTest.Create(context.Background(), 7, "123e4567-e89b-12d3-a456-426614174302", strings.Repeat("名", 51), choices)
	if !service.IsExecutionPathErrorKind(err, service.ExecutionPathErrorInvalidArgument) {
		t.Fatalf("超长名称没有被拒绝：%v", err)
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
