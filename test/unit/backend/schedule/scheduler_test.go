package schedule_test

import (
	"context"
	"testing"
	"time"

	"test-auto-pro-v2/internal/engine/schedule"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// fakeSchedulerStore 以最小实现驱动调度 Tick：只覆盖调度器读取的三条查询。
// 其余方法内嵌接口空值，被调用即 panic（说明调度器越界读了不该读的数据）。
type fakeSchedulerStore struct {
	repository.RunStore
	runs          map[uint64]model.Run
	pathRuns      map[uint64][]model.PathRun
	needingIDs    []uint64
	started       []uint64
	consumedPlans []uint64
}

// ListRunIDsNeedingScheduling 返回待调度的运行集合。
func (f *fakeSchedulerStore) ListRunIDsNeedingScheduling(context.Context) ([]uint64, error) {
	return f.needingIDs, nil
}

// DequeueRun 默认队列为空：计划间串行排队单独用例覆盖。
func (f *fakeSchedulerStore) DequeueRun(context.Context, time.Time) (uint64, error) {
	return 0, nil
}

// GetRun 读取运行聚合。
func (f *fakeSchedulerStore) GetRun(_ context.Context, runID uint64) (model.Run, error) {
	return f.runs[runID], nil
}

// ListPathRunsByRun 读取一次运行下的全部路径运行。
func (f *fakeSchedulerStore) ListPathRunsByRun(_ context.Context, runID uint64) ([]model.PathRun, error) {
	return f.pathRuns[runID], nil
}

// ListDueScheduledPlans / ClaimScheduledPlan 定时扫描未在本用例覆盖，返回空与未领取。
func (f *fakeSchedulerStore) ListDueScheduledPlans(context.Context, time.Time) ([]model.Plan, error) {
	return nil, nil
}

func (f *fakeSchedulerStore) ClaimScheduledPlan(context.Context, uint64, time.Time) (bool, error) {
	return false, nil
}

// waiting 构造等待路径运行。
func waiting(id, pathID uint64) model.PathRun {
	return model.PathRun{ID: id, RunID: 1, ExecutionPathID: pathID, Status: model.PathRunStatusWaiting}
}

// TestSchedulerSerialStartsOneAtATime 锁定串行语义：并发上限 1 时一轮 Tick 只启动第一条等待路径，
// 第二条继续排队；已有活跃路径时（占满唯一槽位）一条都不再启动。
func TestSchedulerSerialStartsOneAtATime(t *testing.T) {
	store := &fakeSchedulerStore{
		runs: map[uint64]model.Run{1: {ID: 1, PlanID: 1, Status: model.RunStatusRunning}},
		pathRuns: map[uint64][]model.PathRun{1: {
			waiting(102, 2), waiting(103, 3),
		}},
		needingIDs: []uint64{1},
	}
	started := make([]uint64, 0)
	scheduler := schedule.NewScheduler(store, store, time.Hour,
		func(_ context.Context, _ model.Run, pathRun model.PathRun) error {
			started = append(started, pathRun.ID)
			return nil
		}, func(context.Context, model.Plan) error { return nil }, nil)
	scheduler.Tick(context.Background())
	if len(started) != 1 || started[0] != 102 {
		t.Fatalf("串行一轮只应按序启动一条等待路径，实际 %v", started)
	}
}

// running 构造运行中路径运行。
func running(id, pathID uint64) model.PathRun {
	return model.PathRun{ID: id, RunID: 1, ExecutionPathID: pathID, Status: model.PathRunStatusRunning}
}

// TestSchedulerStartFailureDoesNotBlockRun 锁定失败隔离与重试边界：
// 启动回调报错时调度器本轮收手（等待路径仍在，下一轮重试），不把错误吞成“已启动”。
func TestSchedulerStartFailureDoesNotBlockRun(t *testing.T) {
	store := &fakeSchedulerStore{
		runs:       map[uint64]model.Run{1: {ID: 1, PlanID: 1, Status: model.RunStatusRunning}},
		pathRuns:   map[uint64][]model.PathRun{1: {waiting(101, 1)}},
		needingIDs: []uint64{1},
	}
	calls := 0
	scheduler := schedule.NewScheduler(store, store, time.Hour,
		func(context.Context, model.Run, model.PathRun) error {
			calls++
			return context.DeadlineExceeded
		}, func(context.Context, model.Plan) error { return nil }, nil)
	scheduler.Tick(context.Background())
	if calls != 1 {
		t.Fatalf("失败后本轮不应继续尝试其他路径，实际调用 %d 次", calls)
	}
}

// TestSchedulerParallelStartsUpToCap 锁定方案 A（用户裁决）：并行上限 2 时一轮补足两个槽位，
// 同账号路径也同时启动——同账号互踢的串行约束由账号锁在请求粒度消化，
// 总耗时趋近 max(各路径)；锁等待由 [session] 日志单独可见。
func TestSchedulerParallelStartsUpToCap(t *testing.T) {
	capacity := 2
	store := &fakeSchedulerStore{
		runs: map[uint64]model.Run{1: {ID: 1, PlanID: 1, Status: model.RunStatusRunning, MaxConcurrency: &capacity}},
		pathRuns: map[uint64][]model.PathRun{1: {
			waiting(101, 1), waiting(102, 2), waiting(103, 3),
		}},
		needingIDs: []uint64{1},
	}
	started := make([]uint64, 0)
	scheduler := schedule.NewScheduler(store, store, time.Hour,
		func(_ context.Context, _ model.Run, pathRun model.PathRun) error {
			started = append(started, pathRun.ID)
			return nil
		}, func(context.Context, model.Plan) error { return nil }, nil)
	scheduler.Tick(context.Background())
	if len(started) != 2 || started[0] != 101 || started[1] != 102 {
		t.Fatalf("并行一轮应按序补足两个槽位（同账号也并发启动），实际 %v", started)
	}
}
