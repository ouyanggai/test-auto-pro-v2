// F-020 多路径调度与单次定时启动的编排层：
// 一次启动按勾选路径集合创建运行与全部路径运行，串行/并行调度由 engine/schedule 的 Worker 推进；
// 到点的单次定时计划在这里触发为自动模式的一次运行。调度器只分配，目标写请求仍只由适配层发出。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"test-auto-pro-v2/internal/engine/control"
	"test-auto-pro-v2/internal/engine/schedule"
	"test-auto-pro-v2/internal/logging"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// validRunModes 是启动允许的运行模式：模式在启动时确定，运行中不可切换（纲领第 5.1 节）。
func validRunModes() map[model.RunMode]bool {
	return map[model.RunMode]bool{
		model.RunModeSingleStep: true,
		model.RunModeAuto:       true,
		model.RunModeManual:     true,
	}
}

// RunStartDTO 是多路径启动的公开结果：运行身份、调度方式与每条路径运行的排队状态。
type RunStartDTO struct {
	RunID        uint64 `json:"runId"`
	RunNo        uint64 `json:"runNo"`
	ModeName     string `json:"modeName"`
	ScheduleName string `json:"scheduleName"`
	// ConcurrencyLabel 是中文并发说明：串行为「逐条依次运行」，并行为「最多 N 条同时运行」。
	ConcurrencyLabel string              `json:"concurrencyLabel"`
	Paths            []RunPathSummaryDTO `json:"paths"`
}

// RunEventDTO 是一条运行事件的公开形态（F-021 事件流时间线）。
type RunEventDTO struct {
	ID        uint64 `json:"id"`
	PathRunID uint64 `json:"pathRunId,omitempty"`
	Kind      string `json:"kind"`
	Label     string `json:"label"`
	CreatedAt string `json:"createdAt"`
}

// ListRunEvents 增量读取一次运行的事件流：afterEventID 为游标，只返回其后的事件。
// 只读，不触发目标调用，不改变任何运行事实。
func (s *RunOrchestrationService) ListRunEvents(ctx context.Context, runID uint64, afterEventID uint64, limit int, pathRunID uint64) ([]RunEventDTO, error) {
	events, err := s.store.ListRunEvents(ctx, runID, afterEventID, limit, pathRunID)
	if err != nil {
		return nil, err
	}
	dtos := make([]RunEventDTO, 0, len(events))
	for _, event := range events {
		dto := RunEventDTO{ID: event.ID, Kind: event.Kind, Label: event.Label}
		if event.PathRunID != nil {
			dto.PathRunID = *event.PathRunID
		}
		dto.CreatedAt = event.CreatedAt.Local().Format("2006-01-02 15:04:05")
		dtos = append(dtos, dto)
	}
	return dtos, nil
}

// RunPathSummaryDTO 是一条路径运行在运行级视图里的摘要。
type RunPathSummaryDTO struct {
	PathRunID  uint64 `json:"pathRunId"`
	PathID     uint64 `json:"pathId"`
	PathName   string `json:"pathName"`
	StatusName string `json:"statusName"`
	ResultName string `json:"resultName,omitempty"`
	// MainInstanceRef 是这条路径运行独占的真实主实例引用（不透明键）。
	MainInstanceRef string `json:"mainInstanceRef,omitempty"`
}

// StartRunWithPaths 按勾选路径集合启动一次运行（F-020 手动启动入口）。
// 运行前检查对每条勾选路径复验：任何一条被阻塞则整次启动被拒绝（手动启动不允许半份启动）；
// 定时启动的「只跑可执行路径」口径由 StartScheduledRun 单独处理。
// pathDispatch / pathMaxConcurrency 是启动弹窗的本次选择（serial / parallel + 2~20），
// 仅作用于本次运行的路径并发，同时作为该计划的「上次选择」供下次弹窗回显。
// 计划层串行（plan.RunMode != parallel）：已有其他计划运行未到终态时，本次运行进入等待队列，
// 由调度器在上一轮计划运行终态后放行；并行计划不受限，直接开跑。
func (s *RunOrchestrationService) StartRunWithPaths(ctx context.Context, planID uint64, pathIDs []uint64,
	mode model.RunMode, presets []control.Breakpoint, idempotencyKey string,
	pathDispatch string, pathMaxConcurrency *int) (*RunStartDTO, error) {
	if !validRunModes()[mode] {
		return nil, &RunOrchestrationError{Kind: RunOrchestrationInvalid, Message: "运行模式不正确"}
	}
	if len(pathIDs) == 0 {
		return nil, &RunOrchestrationError{Kind: RunOrchestrationInvalid, Message: "请先勾选要运行的执行路径"}
	}
	// 每条勾选路径都要通过运行前检查：任何一条被阻塞，整次启动都不发生，绝不静默跳过。
	readiness, err := s.readiness.PlanReadiness(ctx, planID, pathIDs)
	if err != nil {
		return nil, err
	}
	byPath := map[uint64]*model.PathRunReadiness{}
	for i := range readiness.Paths {
		byPath[readiness.Paths[i].PathID] = &readiness.Paths[i]
	}
	blocked := make([]string, 0)
	for _, pathID := range pathIDs {
		pathReadiness := byPath[pathID]
		if pathReadiness == nil {
			blocked = append(blocked, "勾选的执行路径不存在或不属于该计划")
			continue
		}
		if !pathReadiness.Runnable {
			for _, block := range pathReadiness.Blocks {
				blocked = append(blocked, block.Name+"："+block.Reason)
			}
		}
	}
	if len(blocked) > 0 {
		return nil, &RunOrchestrationError{Kind: RunOrchestrationConflict,
			Message: "运行前检查未通过，不能启动：" + strings.Join(blocked, "；")}
	}
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return nil, mapPlanError(err)
	}
	// 计划内路径并发由启动弹窗决定：串行固定 1；并行用弹窗传入的最大并发（2~20，缺失/非法回退 1）。
	// 「上次选择」的回显从该计划最近一次运行的 path_dispatch / max_concurrency 反推，不再单独建偏好表。
	if pathDispatch != "parallel" {
		pathDispatch = "serial"
		pathMaxConcurrency = nil
	}
	capacity := 1
	if pathDispatch == "parallel" && pathMaxConcurrency != nil && *pathMaxConcurrency >= 2 && *pathMaxConcurrency <= 20 {
		capacity = *pathMaxConcurrency
	} else if pathDispatch == "parallel" {
		capacity = 1
	}
	presetJSON, err := json.Marshal(presets)
	if err != nil {
		return nil, err
	}
	created, pathRuns, err := s.store.CreateRunWithPaths(ctx, repository.CreateRunInput{
		PlanID: planID, ExecutionPathIDs: pathIDs, Mode: mode,
		Trigger: model.RunTriggerManual, MaxConcurrency: &capacity, PathDispatch: pathDispatch,
		IdempotencyKey: idempotencyKey, PresetBreakpoints: string(presetJSON),
	})
	if err != nil {
		return nil, err
	}
	// 幂等重试：运行已经在推进中（非等待启动）时原样返回那次运行，不再推进或重新调度。
	if created.Status == model.RunStatusPending {
		// 计划间串行：串行计划在已有非终态运行（含刚创建的自己之外）时入队等待。
		// CountActiveRuns 含自身，所以阈值是 2：除自己外还有别的运行在跑就必须排队。
		active, activeErr := s.store.CountActiveRuns(ctx)
		if activeErr != nil {
			return nil, activeErr
		}
		if plan.RunMode != "parallel" && active > 1 {
			if err := s.store.EnqueueRun(ctx, created.ID, planID, s.now()); err != nil {
				return nil, err
			}
			_ = s.store.AppendRunEvent(ctx, model.RunEvent{
				RunID: created.ID,
				Kind:  "run_queued",
				Label: "计划间串行：已有其他计划运行在进行，本次运行进入等待队列",
			}, s.now())
		} else {
			if _, err := s.runState.AdvanceRunStatus(ctx, created.ID,
				model.RunStatusPending, model.RunStatusRunning, model.RunEvent{
					Kind:  "run_started",
					Label: runStartedLabel(mode, plan, capacity, len(pathRuns)),
				}); err != nil {
				return nil, err
			}
		}
		s.kickScheduler(ctx)
	}
	return s.runStartDTO(ctx, created.ID)
}

// StartScheduledRun 触发到点计划的一次定时启动（调度器已原子领取消费标记）。
// 口径（功能文档未决事项默认裁决）：只创建可执行路径的运行，被阻塞路径记为阻塞事实；
// 一条可执行路径都没有时不创建运行，把失败事实写进计划配置日志目录，可回放、不重复触发。
// 定时启动强制自动模式（纲领第 4.5 节），并默认开启首次写断点作为连续真实写的安全阀。
func (s *RunOrchestrationService) StartScheduledRun(ctx context.Context, plan model.Plan) error {
	paths, err := s.paths.List(ctx, plan.ID)
	if err != nil {
		return s.recordScheduleFailure(ctx, plan, "无法读取执行路径："+err.Error())
	}
	selected := make([]uint64, 0, len(paths))
	blocked := make([]string, 0)
	readiness, err := s.readiness.PlanReadiness(ctx, plan.ID, nil)
	if err != nil {
		return s.recordScheduleFailure(ctx, plan, "运行前检查失败："+err.Error())
	}
	readyByID := map[uint64]model.PathRunReadiness{}
	for _, item := range readiness.Paths {
		readyByID[item.PathID] = item
	}
	for _, path := range paths {
		item, ok := readyByID[path.ID]
		if ok && item.Runnable {
			selected = append(selected, path.ID)
			continue
		}
		name := path.Name
		if strings.TrimSpace(name) == "" {
			name = "路径 " + strconv.FormatUint(uint64(path.SequenceNo), 10)
		}
		if ok && len(item.Blocks) > 0 {
			name += "（" + item.Blocks[0].Reason + "）"
		}
		blocked = append(blocked, name)
	}
	if len(selected) == 0 {
		return s.recordScheduleFailure(ctx, plan, "定时启动未执行：没有可执行的路径。"+strings.Join(blocked, "；"))
	}
	// 定时触发按计划身份生成确定性幂等键：即使消费标记出现竞态，同键重试也只得到同一次运行。
	// 路径调度方式沿用计划保存的上次选择：并行计划用其最大并发，串行计划串行。
	scheduledDispatch := "serial"
	var scheduledConcurrency *int
	if plan.RunMode == "parallel" {
		scheduledDispatch = "parallel"
		scheduledConcurrency = plan.MaxConcurrency
	}
	result, err := s.StartRunWithPaths(ctx, plan.ID, selected, model.RunModeAuto,
		[]control.Breakpoint{{Type: model.BreakpointFirstWrite}}, "scheduled-"+strconv.FormatUint(plan.ID, 10),
		scheduledDispatch, scheduledConcurrency)
	if err != nil {
		return s.recordScheduleFailure(ctx, plan, "定时启动失败："+err.Error())
	}
	summary := "定时启动已创建运行 " + strconv.FormatUint(result.RunNo, 10) + "（自动模式，" + result.ConcurrencyLabel + "）"
	if len(blocked) > 0 {
		summary += "；以下路径被阻塞未纳入：" + strings.Join(blocked, "；")
	}
	s.writeScheduleFact(ctx, plan.ID, summary)
	return nil
}

// beginPathRunOnScheduler 是注入调度器的路径启动回调：构建执行上下文并停在阶段 3（自动模式进循环）。
// 构建失败（结构读取失败、编译场景为空等）按失败隔离处理：只把这一条路径运行置为失败，其他路径继续。
func (s *RunOrchestrationService) beginPathRunOnScheduler(ctx context.Context, runRow model.Run, pathRun model.PathRun) error {
	// 运行日志作用域必须在这里注入：调度链路没有 HTTP 请求中间件，不注入的话
	// step.log 与 network/curl 日志会落错目录（实测落到 configuration/ 下，运行目录里什么都没有）。
	if scope, scopeErr := s.runLogScope(ctx, pathRun.ID); scopeErr == nil {
		ctx = logging.WithScope(ctx, scope)
	}
	// 原子领取：先把等待运行条件推进为运行中。并行补位循环与定时 Tick 并发时，
	// 两个调度方可能同时读到同一条等待路径；不在这里抢先领取就会重复构建执行上下文、
	// 重复启动同一条路径（实测 run=89 每条路径都被启动了两次）。输了领取的一方直接退出。
	if _, err := s.runState.AdvancePathRun(ctx, pathRun.ID,
		model.PathRunStatusWaiting, model.PathRunStatusRunning, model.RunEvent{
			Kind:  "path_run_started",
			Label: "调度器领取路径运行，准备第一步预览",
		}); err != nil {
		if errors.Is(err, repository.ErrRunStatusConflict) {
			return nil
		}
		return err
	}
	runCtx, err := s.buildRunContext(ctx, runRow.PlanID, pathRun.ExecutionPathID)
	if err != nil {
		s.finishPathRunFailed(ctx, pathRun.ID, "路径运行启动失败："+err.Error())
		return nil
	}
	if len(runCtx.Steps) == 0 {
		s.finishPathRunFailed(ctx, pathRun.ID, "编译场景为空，路径运行置为失败")
		return nil
	}
	// buildRunContext 留下的 PathRun/Run 只有占位身份（创建前的 ID 未知），
	// 调度时真实身份已知，必须先填进上下文，否则 BeginPathRun 会按 0 号 ID 推进（实测踩坑）。
	// 上面已用条件推进原子领取（等待运行→运行中）：这里同步内存副本的状态，
	// 否则 BeginPathRun 会按旧的等待运行再推进一次，撞上状态冲突后整个启动被吞掉，
	// 路径卡在「运行中」但没有任何执行现场（实测缺陷）。
	pathRun.Status = model.PathRunStatusRunning
	runCtx.Run = runRow
	runCtx.PathRun = pathRun
	// 待发/已发来源的路径运行创建即绑定计划指向的真实实例（2026-09-07 交付验收修复）：
	// 门禁复验与事实重读都依赖主实例引用；此前它只在提交成功后落库，
	// 草稿实例的 save_draft 门禁因读不到实例状态被误拒。
	if runCtx.Source != "" && runCtx.Source != "new" && runCtx.PathRun.MainInstanceRef == "" {
		// 待发/已发场景：计划目标对象就是真实实例 ID（buildRunContext 已把它放进 FlowProxyID），
		// 门禁与事实重读据此读取实例状态；提交成功后 SetMainInstanceRef 的独占绑定不受影响
		// （值相同，且「首次落库不可改写」语义仍然成立）。
		runCtx.PathRun.MainInstanceRef = runCtx.FlowProxyID
	}
	// 预置断点随运行落库：每条路径开始时重放同一预置，重启恢复后同样生效。
	presets := decodePresetBreakpoints(runRow.PresetBreakpoints)
	if _, err := s.control.BeginPathRun(ctx, runCtx, runRow.Mode, presets); err != nil {
		log.Printf("[schedule] BeginPathRun 失败 run=%d path_run=%d: %v", runRow.ID, pathRun.ID, err)
		// 状态冲突说明并发调度已把它启动或它已到终态，不算失败；其余错误交回调度器下一轮重试。
		if !errors.Is(err, repository.ErrRunStatusConflict) {
			return err
		}
	}
	return nil
}

// finishPathRunFailed 把一条路径运行置为失败终态（失败隔离：只影响这一条路径）。
func (s *RunOrchestrationService) finishPathRunFailed(ctx context.Context, pathRunID uint64, reason string) {
	class := model.FailureClassToolBug
	result := model.RunResultFailed
	_, _ = s.runState.Finish(ctx, pathRunID, model.PathRunStatusFailed, &result, &class, reason)
}

// NewScheduler 组装运行级调度 Worker：路径启动与定时触发回调都指向本服务。
// 调用方负责在后台协程里运行返回值的 Run 方法。
func (s *RunOrchestrationService) NewScheduler(interval time.Duration) *schedule.Scheduler {
	return schedule.NewScheduler(s.store, s.store, interval,
		s.beginPathRunOnScheduler, s.StartScheduledRun, s.now)
}

// kickScheduler 立即做一轮调度：Tick 内的数据库条件写保证并发安全，多做一轮不会重复启动。
// 必须与请求上下文脱钩：请求返回会取消 ctx，若定时触发在消费标记已写、运行未建之间被打断，
// 一次性定时就被静默吞掉且没有任何事实（评审 P2）。
func (s *RunOrchestrationService) kickScheduler(ctx context.Context) {
	detached, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	go func() {
		defer cancel()
		s.NewScheduler(s.runConfig.StatusPollInterval).Tick(detached)
	}()
}

// runStartDTO 汇总一次运行的调度视图。
func (s *RunOrchestrationService) runStartDTO(ctx context.Context, runID uint64) (*RunStartDTO, error) {
	runRow, err := s.store.GetRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	pathRuns, err := s.store.ListPathRunsByRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	dto := &RunStartDTO{
		RunID:        runRow.ID,
		RunNo:        runRow.RunNo,
		ModeName:     model.RunModeName(runRow.Mode),
		ScheduleName: runScheduleName(runRow),
		Paths:        make([]RunPathSummaryDTO, 0, len(pathRuns)),
	}
	capacity := runCapacityOf(runRow)
	if capacity <= 1 {
		dto.ConcurrencyLabel = "逐条依次运行"
	} else {
		dto.ConcurrencyLabel = "最多同时运行 " + strconv.Itoa(capacity) + " 条路径"
	}
	for _, pathRun := range pathRuns {
		summary := RunPathSummaryDTO{
			PathRunID:  pathRun.ID,
			PathID:     pathRun.ExecutionPathID,
			PathName:   pathNameOf(ctx, s.paths, runRow.PlanID, pathRun.ExecutionPathID, ""),
			StatusName: model.PathRunStatusName(pathRun.Status),
		}
		if pathRun.Result != nil {
			summary.ResultName = resultName(*pathRun.Result)
		}
		summary.MainInstanceRef = pathRun.MainInstanceRef
		dto.Paths = append(dto.Paths, summary)
	}
	return dto, nil
}

// runCapacityOf 返回运行的并发上限；空或非法按 1（串行）处理。
func runCapacityOf(runRow model.Run) int {
	if runRow.MaxConcurrency != nil && *runRow.MaxConcurrency > 0 {
		return *runRow.MaxConcurrency
	}
	return 1
}

// runStartedLabel 生成运行开始的中文事件。
func runStartedLabel(mode model.RunMode, plan model.Plan, capacity int, pathCount int) string {
	scheduleName := "串行调度"
	if capacity > 1 {
		scheduleName = "并行调度，最大并发 " + strconv.Itoa(capacity)
	}
	if plan.ScheduledAt != nil {
		scheduleName += "，定时触发"
	}
	return "运行开始（" + model.RunModeName(mode) + "，" + scheduleName + "，共 " + strconv.Itoa(pathCount) + " 条路径）"
}

// runScheduleName 返回运行级的调度方式中文名。
func runScheduleName(runRow model.Run) string {
	if runCapacityOf(runRow) <= 1 {
		return "串行"
	}
	return "并行"
}

// decodePresetBreakpoints 解码运行行上的预置断点；损坏或为空返回空集合（首次写安全阀由启动链路自带）。
func decodePresetBreakpoints(raw string) []control.Breakpoint {
	if raw == "" {
		return nil
	}
	var decoded []control.Breakpoint
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil
	}
	return decoded
}

// recordScheduleFailure 把定时启动的失败事实写进该计划的配置日志目录（可回放），不创建运行。
func (s *RunOrchestrationService) recordScheduleFailure(ctx context.Context, plan model.Plan, reason string) error {
	s.writeScheduleFact(ctx, plan.ID, reason)
	return nil
}

// writeScheduleFact 把定时启动事实按计划作用域写进配置日志目录，界面之外同样可回放。
func (s *RunOrchestrationService) writeScheduleFact(ctx context.Context, planID uint64, message string) {
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return
	}
	scope := logging.Scope{PlanID: strconv.FormatUint(plan.ID, 10), PlanName: plan.Name}
	writer := s.router.Bucket(scope, "operation.log")
	if writer == nil {
		return
	}
	writer.WriteLine("time=" + s.now().Format("2006-01-02_15:04:05") + " level=info message=" + message)
}

// RunDispatchDefault 返回计划上次启动使用的路径调度方式与并发数（无历史运行返回空串与 nil）。
// 「记住上次选择」直接从最近一次运行行反推，不另建偏好表：上一次怎么跑的，下次默认怎么跑。
func (s *RunOrchestrationService) RunDispatchDefault(ctx context.Context, planID uint64) (string, *int, error) {
	runs, err := s.store.ListRunsByPlan(ctx, planID, 1)
	if err != nil {
		return "", nil, err
	}
	if len(runs) == 0 {
		return "", nil, nil
	}
	dispatch := runs[0].PathDispatch
	if dispatch == "" {
		// 迁移前的历史运行没有 path_dispatch：按并发数反推，避免回显成空。
		if runs[0].MaxConcurrency != nil && *runs[0].MaxConcurrency > 1 {
			dispatch = "parallel"
		} else {
			dispatch = "serial"
		}
	}
	return dispatch, runs[0].MaxConcurrency, nil
}
