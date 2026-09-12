// F-020 运行级调度 Worker：串行按稳定序号推进、并行按最大并发补位、单次定时启动一次性消费。
// 边界（纲领第 4.1 节）：调度器只分配与收尾，绝不直接发目标请求；同一路径内部永远串行；
// 一条路径失败、待对账或停止只释放自己的槽位，其他路径继续（失败隔离）。
// Worker 形态沿用 formruntimemaintenance 已验证的「数据库状态 + 轮询 + 启动恢复」：
// 所有判断都基于数据库事实，进程重启后 Tick 天然恢复等待路径的调度，不依赖内存。
package schedule

import (
	"context"
	"log"
	"strings"
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
	// PlanAccountOf 返回计划的目标账号（调度器用于同账号串行排队）；读不到返回错误。
	PlanAccountOf(ctx context.Context, planID uint64) (string, error)
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
// F-030/T03：同账号真实串行排队 —— 同一账号的会话与目标操作在目标侧互踢，只能串行；
// 调度器在放行前按计划账号限制实际启动数：本轮 Tick 内某账号已有路径在运行（或刚被启动），
// 同账号后续路径继续等待，把并发机会让给不同账号的路径。这避免多条路径同时抢同一把
// 账号锁后长时间空转——先启动者持锁执行，后启动者不再白占执行现场等锁。
// 实现分两轮扫描：第一轮只收集「已有活跃路径」的账号（运行中/核验中/暂停都算占用，
// 暂停的路径随时会继续执行）；第二轮再补位启动。只做一轮会依赖运行的遍历顺序——
// 先处理全等待的运行会把同账号新路径启动起来，随后才看到另一个运行的同账号路径
// 正在运行，这正是要消除的无效排队（评审 P1）。
func (s *Scheduler) scheduleWaitingPaths(ctx context.Context) {
	runIDs, err := s.store.ListRunIDsNeedingScheduling(ctx)
	if err != nil {
		return
	}
	// busyAccounts 是本轮 Tick 内已确认占用的计划账号：已有活跃路径的运行与本轮启动成功的
	// 路径都计入（跨运行也生效：账号是全局资源不是运行资源）。
	busyAccounts := map[string]bool{}
	type runPlan struct {
		run        model.Run
		pathRuns   []model.PathRun
		planErr    error
		account    string
		active     int
		waitingCnt int
	}
	plans := make([]runPlan, 0, len(runIDs))
	for _, runID := range runIDs {
		run, err := s.store.GetRun(ctx, runID)
		if err != nil {
			continue
		}
		// 计划账号决定目标会话归属：读不到账号的运行无法判断账号约束，按容量补位（旧行为）。
		planAccount, planErr := s.plans.PlanAccountOf(ctx, run.PlanID)
		pathRuns, err := s.store.ListPathRunsByRun(ctx, runID)
		if err != nil {
			continue
		}
		entry := runPlan{run: run, pathRuns: pathRuns, planErr: planErr, account: planAccount}
		for _, pathRun := range pathRuns {
			switch pathRun.Status {
			case model.PathRunStatusRunning, model.PathRunStatusVerifying, model.PathRunStatusPaused:
				entry.active++
			case model.PathRunStatusWaiting:
				entry.waitingCnt++
			}
		}
		// 已有活跃路径即占用计划账号：同账号的其他运行（含本运行）本轮不再启动新路径。
		if entry.active > 0 && planErr == nil && strings.TrimSpace(planAccount) != "" {
			busyAccounts[strings.TrimSpace(planAccount)] = true
		}
		plans = append(plans, entry)
	}
	for _, entry := range plans {
		if entry.waitingCnt == 0 {
			continue
		}
		capacity := 1
		if entry.run.MaxConcurrency != nil && *entry.run.MaxConcurrency > 0 {
			capacity = *entry.run.MaxConcurrency
		}
		active := entry.active
		for _, pathRun := range entry.pathRuns {
			if pathRun.Status != model.PathRunStatusWaiting {
				continue
			}
			if active >= capacity {
				// 槽位已满：本运行的等待路径继续排队，处理下一个运行。
				break
			}
			// 同账号串行排队：本轮已有同账号路径在运行或被启动则不再启动（避免抢锁空转）。
			if entry.planErr == nil && busyAccounts[strings.TrimSpace(entry.account)] {
				break
			}
			log.Printf("[schedule] 启动路径运行 run=%d path_run=%d", entry.run.ID, pathRun.ID)
			if err := s.startPathRun(ctx, entry.run, pathRun); err != nil {
				// 启动失败如实交给下一轮 Tick：路径运行仍在等待，数据库状态没有被破坏；
				// 本运行停止补位，继续处理其他运行。
				log.Printf("[schedule] 启动失败 run=%d path_run=%d: %v", entry.run.ID, pathRun.ID, err)
				break
			}
			if entry.planErr == nil && strings.TrimSpace(entry.account) != "" {
				busyAccounts[strings.TrimSpace(entry.account)] = true
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
