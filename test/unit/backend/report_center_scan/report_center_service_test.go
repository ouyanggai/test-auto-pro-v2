package report_center_scan_test

// F-042 本轮：报告中心服务层的性能性质与数据权威性门禁。
// 用桩仓储替换数据库，断言四件事：
//   - 一次 Overview 只发一次日志目录查询（不再有按路径运行的 N+1）；
//   - 接口统计来自真实 network.log 全量行（含 step_id=- 的登录/结构读取，旧口径漏计）；
//   - 三个接口都带 generatedAt（页面据此显示「数据截至 …」）；
//   - 仓储读取失败时降级为「无接口事实」，不让整页报告失败。
// 运行：go test ./test/unit/backend/report_center_scan/
import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	"test-auto-pro-v2/internal/service"
)

// fakePlanRepository 只实现报告中心用到的 List/Get，其余方法留空（报告不触达）。
type fakePlanRepository struct {
	plans []model.Plan
}

func (f *fakePlanRepository) Create(context.Context, string, model.Plan) (model.Plan, bool, error) {
	return model.Plan{}, false, errors.New("未使用")
}
func (f *fakePlanRepository) Update(context.Context, uint64, model.Plan) (model.Plan, error) {
	return model.Plan{}, errors.New("未使用")
}
func (f *fakePlanRepository) List(context.Context, model.PlanListFilter) ([]model.Plan, error) {
	return f.plans, nil
}
func (f *fakePlanRepository) Count(context.Context, model.PlanListFilter) (int, error) {
	return len(f.plans), nil
}
func (f *fakePlanRepository) Get(_ context.Context, id uint64) (model.Plan, error) {
	for _, plan := range f.plans {
		if plan.ID == id {
			return plan, nil
		}
	}
	return model.Plan{}, repository.ErrPlanNotFound
}
func (f *fakePlanRepository) Delete(context.Context, uint64) error { return errors.New("未使用") }
func (f *fakePlanRepository) BatchDelete(context.Context, []uint64) map[uint64]error {
	return nil
}

// fakeExecutionPathRepository 只实现报告中心用到的 List/Get。
type fakeExecutionPathRepository struct {
	paths map[uint64][]model.ExecutionPath
}

func (f *fakeExecutionPathRepository) List(_ context.Context, planID uint64) ([]model.ExecutionPath, error) {
	return f.paths[planID], nil
}
func (f *fakeExecutionPathRepository) Get(_ context.Context, planID, pathID uint64) (model.ExecutionPath, error) {
	for _, path := range f.paths[planID] {
		if path.ID == pathID {
			return path, nil
		}
	}
	return model.ExecutionPath{}, repository.ErrExecutionPathNotFound
}
func (f *fakeExecutionPathRepository) GetMany(context.Context, uint64, []uint64) ([]model.ExecutionPath, error) {
	return nil, errors.New("未使用")
}
func (f *fakeExecutionPathRepository) FindByCreateKey(context.Context, uint64, string) (model.ExecutionPath, bool, error) {
	return model.ExecutionPath{}, false, errors.New("未使用")
}
func (f *fakeExecutionPathRepository) Create(context.Context, uint64, string, string, []model.ExecutionPathChoice, time.Time) (model.ExecutionPath, bool, error) {
	return model.ExecutionPath{}, false, errors.New("未使用")
}
func (f *fakeExecutionPathRepository) Update(context.Context, uint64, uint64, string, []model.ExecutionPathChoice, time.Time) (model.ExecutionPath, error) {
	return model.ExecutionPath{}, errors.New("未使用")
}
func (f *fakeExecutionPathRepository) Delete(context.Context, uint64, uint64, time.Time) error {
	return errors.New("未使用")
}
func (f *fakeExecutionPathRepository) FindBatchByCreateKey(context.Context, uint64, string) (model.ExecutionPathBatchResult, bool, error) {
	return model.ExecutionPathBatchResult{}, false, errors.New("未使用")
}
func (f *fakeExecutionPathRepository) GeneratePathsBatch(context.Context, uint64, string, [][]model.ExecutionPathChoice, time.Time) (model.ExecutionPathBatchResult, bool, error) {
	return model.ExecutionPathBatchResult{}, false, errors.New("未使用")
}

// fakeReportStore 记录每个仓储方法的调用次数：本次门禁的核心是一次聚合只查一次日志目录。
type fakeReportStore struct {
	paths          []repository.ReportPathRunRow
	attempts       []repository.ReportAttemptRow
	dirs           []repository.ReportPathRunLogDir
	countCalls     int
	rollupCalls    int
	rowCalls       int
	attemptCalls   int
	dirCalls       int
	dirFail        bool
	now            time.Time
	planID         uint64
	executionPath  uint64
	pathRunCounter uint64
}

func (f *fakeReportStore) ReportCounts(context.Context) (repository.ReportCounts, error) {
	f.countCalls++
	return repository.ReportCounts{PlansTotal: 1, RunsTotal: 1, PathRunsTotal: 1}, nil
}

func (f *fakeReportStore) ReportPlanRollups(context.Context) ([]repository.ReportPlanRollup, error) {
	f.rollupCalls++
	return []repository.ReportPlanRollup{{PlanID: f.planID, RunCount: 1, PathRunCount: 1, CoveredPaths: 1}}, nil
}

func (f *fakeReportStore) ReportPathRunRows(context.Context, repository.ReportScope) ([]repository.ReportPathRunRow, error) {
	f.rowCalls++
	return f.paths, nil
}

func (f *fakeReportStore) ReportAttemptRows(context.Context, repository.ReportScope) ([]repository.ReportAttemptRow, error) {
	f.attemptCalls++
	return f.attempts, nil
}

func (f *fakeReportStore) ReportPathRunLogDirs(context.Context, repository.ReportScope) ([]repository.ReportPathRunLogDir, error) {
	f.dirCalls++
	if f.dirFail {
		return nil, errors.New("存储暂时不可用")
	}
	return f.dirs, nil
}

// newFakeRows 造一条已结束路径运行行与一条尝试行，用于驱动 computeMetrics。
func newFakeRows() ([]repository.ReportPathRunRow, []repository.ReportAttemptRow) {
	succeeded := model.RunResultSucceeded
	rows := []repository.ReportPathRunRow{{
		PathRunID: 901, RunID: 900, RunNo: 7, PlanID: 1, ExecutionPathID: 1,
		Status: model.PathRunStatusCompleted, Result: &succeeded,
	}}
	attempts := []repository.ReportAttemptRow{{
		PathRunID: 901, StepID: 1, AttemptNo: 1, DurationMs: 200, ExecutorMs: 80, ExecutorMsKnown: true,
	}}
	return rows, attempts
}

// TestReportOverviewQueriesLogDirsOnce 锁定性能门禁：一次首页读取只发一次日志目录查询，
// 不再按路径运行逐条查尝试（旧实现每条路径运行一次，是 N+1）。
func TestReportOverviewQueriesLogDirsOnce(t *testing.T) {
	root := t.TempDir()
	rows, attempts := newFakeRows()
	store := &fakeReportStore{paths: rows, attempts: attempts, planID: 1}
	// 日志行里带 step_id=-：旧口径会整行丢弃，新口径必须计入。
	dir := filepath.Join(root, "runs", "run1", "paths", "p1")
	if err := writeNetworkLogInto(t, dir, "901", networkLine("901", "/api/web/flowTemplateApi/list", "", "0.400")); err != nil {
		t.Fatalf("写日志失败：%v", err)
	}
	store.dirs = []repository.ReportPathRunLogDir{{
		PathRunID: 901, Status: model.PathRunStatusCompleted,
		LogPath: filepath.ToSlash(filepath.Join("runs", "run1", "paths", "p1", "step.log")),
	}}

	reports := service.NewReportCenterService(store,
		service.NewPlanService(&fakePlanRepository{plans: []model.Plan{{ID: 1, Name: "计划甲", PathCount: 1}}}),
		&fakeExecutionPathRepository{paths: map[uint64][]model.ExecutionPath{1: {{ID: 1, PlanID: 1, SequenceNo: 1, Name: "路径一"}}}},
		root, func() time.Time { return store.now })

	overview, err := reports.Overview(context.Background())
	if err != nil {
		t.Fatalf("Overview 失败：%v", err)
	}
	if store.dirCalls != 1 {
		t.Fatalf("一次首页读取应只查 1 次日志目录，实际 %d", store.dirCalls)
	}
	if overview.Metrics.InterfaceCalls != 1 {
		t.Fatalf("接口调用应为 1 次（含 step_id=- 的日志行），实际 %d", overview.Metrics.InterfaceCalls)
	}
	if overview.Metrics.InterfaceReadCalls != 1 || overview.Metrics.InterfaceWriteCalls != 0 {
		t.Fatalf("读写次数不符：%+v", overview.Metrics)
	}
	if !overview.Metrics.InterfaceKnown {
		t.Fatalf("读到真实 network.log，InterfaceKnown 必须为真")
	}
	if overview.Metrics.InterfaceLatencyMs.SampleCount != 1 || overview.Metrics.InterfaceLatencyMs.P50 != 400 {
		t.Fatalf("接口耗时分位样本应为 1 条 400ms，实际 %+v", overview.Metrics.InterfaceLatencyMs)
	}
}

// TestReportGeneratedAtPresent 锁定数据权威性门禁：三个接口都带统计时刻，
// 页面据此显示「数据截至 …」，用户能判断数字有多新。
func TestReportGeneratedAtPresent(t *testing.T) {
	root := t.TempDir()
	rows, attempts := newFakeRows()
	at := time.Date(2026, 10, 10, 9, 30, 0, 0, time.UTC)
	store := &fakeReportStore{paths: rows, attempts: attempts, planID: 1, now: at}
	reports := service.NewReportCenterService(store,
		service.NewPlanService(&fakePlanRepository{plans: []model.Plan{{ID: 1, Name: "计划甲", PathCount: 1}}}),
		&fakeExecutionPathRepository{paths: map[uint64][]model.ExecutionPath{1: {{ID: 1, PlanID: 1, SequenceNo: 1, Name: "路径一"}}}},
		root, func() time.Time { return store.now })

	overview, err := reports.Overview(context.Background())
	if err != nil {
		t.Fatalf("Overview 失败：%v", err)
	}
	if !overview.GeneratedAt.Equal(at) {
		t.Fatalf("首页统计时刻应为 %v，实际 %v", at, overview.GeneratedAt)
	}
	planReport, err := reports.PlanReport(context.Background(), 1)
	if err != nil {
		t.Fatalf("PlanReport 失败：%v", err)
	}
	if !planReport.GeneratedAt.Equal(at) {
		t.Fatalf("计划详情统计时刻应为 %v，实际 %v", at, planReport.GeneratedAt)
	}
	// 计划详情不再重复整套指标面板，但仍要给出计划级核心指标：这里断言分母口径仍然正确。
	if planReport.Metrics.FinishedPathRuns != 1 || planReport.Metrics.SuccessRate != 1 {
		t.Fatalf("计划级成功率口径不符：%+v", planReport.Metrics)
	}
	pathReport, err := reports.PathReport(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("PathReport 失败：%v", err)
	}
	if !pathReport.GeneratedAt.Equal(at) {
		t.Fatalf("路径详情统计时刻应为 %v，实际 %v", at, pathReport.GeneratedAt)
	}
	if len(pathReport.Runs) != 1 {
		t.Fatalf("路径详情应有 1 次运行实例，实际 %d", len(pathReport.Runs))
	}
}

// TestReportInterfaceStatsFailureDegrades 锁定降级门禁：日志目录查询失败时，
// 接口相关指标全部按缺失处理（不用 0 冒充），其余指标照常返回。
func TestReportInterfaceStatsFailureDegrades(t *testing.T) {
	root := t.TempDir()
	rows, attempts := newFakeRows()
	store := &fakeReportStore{paths: rows, attempts: attempts, planID: 1, dirFail: true}
	reports := service.NewReportCenterService(store,
		service.NewPlanService(&fakePlanRepository{plans: []model.Plan{{ID: 1, Name: "计划甲", PathCount: 1}}}),
		&fakeExecutionPathRepository{paths: map[uint64][]model.ExecutionPath{1: {{ID: 1, PlanID: 1, SequenceNo: 1, Name: "路径一"}}}},
		root, time.Now)

	overview, err := reports.Overview(context.Background())
	if err != nil {
		t.Fatalf("日志目录查询失败不应让整页失败：%v", err)
	}
	if overview.Metrics.InterfaceKnown || overview.Metrics.InterfaceCalls != 0 {
		t.Fatalf("接口事实必须按缺失降级：%+v", overview.Metrics)
	}
	if overview.Metrics.InterfaceLatencyMs.SampleCount != 0 {
		t.Fatalf("无样本时分位 SampleCount 必须为 0，实际 %d", overview.Metrics.InterfaceLatencyMs.SampleCount)
	}
	// 与接口无关的指标不受影响：成功率仍然来自运行记录。
	if overview.Metrics.FinishedPathRuns != 1 || overview.Metrics.SuccessRate != 1 {
		t.Fatalf("降级不应影响运行记录指标：%+v", overview.Metrics)
	}
}
