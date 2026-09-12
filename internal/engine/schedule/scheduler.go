// F-020 运行级调度 Worker：串行按稳定序号推进、并行按最大并发补位、单次定时启动一次性消费。
// 边界（纲领第 4.1 节）：调度器只分配与收尾，绝不直接发目标请求；同一路径内部永远串行；
// 一条路径失败、待对账或停止只释放自己的槽位，其他路径继续（失败隔离）。
// Worker 形态沿用 formruntimemaintenance 已验证的「数据库状态 + 轮询 + 启动恢复」：
// 所有判断都基于数据库事实，进程重启后 Tick 天然恢复等待路径的调度，不依赖内存。
package schedule

import (
	"context"
	"log"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// ScheduledPlanStore 是定时扫描需要的最小计划存储面（由 MySQL 仓储实现）。
type ScheduledPlanStore interface {
	// ListDueScheduledPlans 列出到点尚未消费的计划（数据库时间为准）。
	ListDueScheduledPlans(ctx context.Context, now time.Time) ([]model.Plan, error)
	// ClaimScheduledPlan 原子领取到点计划的一次性消费标记；返回是否领取成功。
	ClaimScheduledPlan(ctx context.Context, planID uint64, now time.Time) (bool, error)
}

// Scheduler 是运行级调度器。
type Scheduler struct {
	store repository.RunStore
	plans ScheduledPlanStore
	now   func() time.Time
	// interval 是调度轮询间隔：路径运行终态到下一条路径开始的最大延迟上界。
	interval time.Duration
	// startPathRun 由编排层注入：把一条等待路径运行推进到执行中。
	// 返回错误只记录不中断：下一轮 Tick 会对仍处等待的路径重试。
	startPathRun func(ctx context.Context, run model.Run, pathRun model.PathRun) error
	// startScheduledRun 由编排层注入：触发一次到点计划的定时启动（调用前已完成一次性消费领取）。
	startScheduledRun func(ctx context.Context, plan model.Plan) error
}

// NewScheduler 组装调度器。startPathRun / startScheduledRun 不允许为空：调度器自己不碰目标平台。
func NewScheduler(store repository.RunStore, plans ScheduledPlanStore, interval time.Duration,
	startPathRun func(ctx context.Context, run model.Run, pathRun model.PathRun) error,
	startScheduledRun func(ctx context.Context, plan model.Plan) error, now func() time.Time) *Scheduler {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Scheduler{store: store, plans: plans, now: now, interval: interval,
		startPathRun: startPathRun, startScheduledRun: startScheduledRun}
}

// Run 启动调度循环，直到上下文取消。立刻先做一轮调度（启动恢复：重启后等待路径照常排队）。
func (s *Scheduler) Run(ctx context.Context) {
	s.Tick(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Tick(ctx)
		}
	}
}

// Kick 立即做一轮调度：启动后不等轮询间隔，第一条路径立刻开始。
// 并发安全：Tick 内的推进由数据库状态迁移的条件写保证，两个 Tick 同时跑也只有一个能推进同一路径。
func (s *Scheduler) Kick(ctx context.Context) {
	s.Tick(ctx)
}

// Tick 执行一轮调度：先按计划间串行规则放行队列首的等待运行，再补位运行中运行的等待路径，
// 最后扫描到点的单次定时计划。
func (s *Scheduler) Tick(ctx context.Context) {
	s.promoteQueuedRun(ctx)
	s.scheduleWaitingPaths(ctx)
	s.triggerDuePlans(ctx)
}

// promoteQueuedRun 放行计划间串行队列的队首（若有）：DequeueRun 内部原子判定
// 「没有其他非终态运行」才领取并置运行中；领到的运行随后的 scheduleWaitingPaths 立即启动路径。
func (s *Scheduler) promoteQueuedRun(ctx context.Context) {
	runID, err := s.store.DequeueRun(ctx, s.now())
	if err != nil {
		log.Printf("[schedule] 计划间串行队列放行失败: %v", err)
		return
	}
	if runID != 0 {
		log.Printf("[schedule] 队列放行运行 run=%d", runID)
	}
}

// scheduleWaitingPaths 对每个仍在运行中且有等待路径的运行按调度策略补位。
// 并发上限取运行行的 max_concurrency（串行运行为 1）；活跃数=运行中+核验中+暂停。
// F-030/T03（方案 A，用户裁决）：路径级并发直接放开——同账号路径也同时启动，
// 目标同账号互踢的串行约束由账号使用锁在请求粒度上排队消化；总耗时趋近
// max(各路径)，而不是串行 sum。锁等待在 [session] 日志可见，与目标接口耗时分开。
func (s *Scheduler) scheduleWaitingPaths(ctx context.Context) {
	runIDs, err := s.store.ListRunIDsNeedingScheduling(ctx)
	if err != nil {
		return
	}
	for _, runID := range runIDs {
		run, err := s.store.GetRun(ctx, runID)
		if err != nil {
			continue
		}
		pathRuns, err := s.store.ListPathRunsByRun(ctx, runID)
		if err != nil {
			continue
		}
		capacity := 1
		if run.MaxConcurrency != nil && *run.MaxConcurrency > 0 {
			capacity = *run.MaxConcurrency
		}
		active := 0
		waiting := make([]model.PathRun, 0)
		for _, pathRun := range pathRuns {
			switch pathRun.Status {
			case model.PathRunStatusRunning, model.PathRunStatusVerifying, model.PathRunStatusPaused:
				active++
			case model.PathRunStatusWaiting:
				waiting = append(waiting, pathRun)
			}
		}
		for _, pathRun := range waiting {
			if active >= capacity {
				// 槽位已满：本运行的等待路径继续排队，处理下一个运行。
				break
			}
			log.Printf("[schedule] 启动路径运行 run=%d path_run=%d", run.ID, pathRun.ID)
			if err := s.startPathRun(ctx, run, pathRun); err != nil {
				// 启动失败如实交给下一轮 Tick：路径运行仍在等待，数据库状态没有被破坏；
				// 本运行停止补位，继续处理其他运行。
				log.Printf("[schedule] 启动失败 run=%d path_run=%d: %v", run.ID, pathRun.ID, err)
				break
			}
			active++
		}
	}
}

// triggerDuePlans 扫描到点的单次定时计划并逐个原子领取；领取成功才触发，
// 服务重启或并发扫描都只有一个赢家，绝不重复创建运行（一次性消费）。
func (s *Scheduler) triggerDuePlans(ctx context.Context) {
	plans, err := s.plans.ListDueScheduledPlans(ctx, s.now())
	if err != nil {
		return
	}
	for _, plan := range plans {
		claimed, err := s.plans.ClaimScheduledPlan(ctx, plan.ID, s.now())
		if err != nil || !claimed {
			continue
		}
		s.startScheduledRun(ctx, plan)
	}
}
