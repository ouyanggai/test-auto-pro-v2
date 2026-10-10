// F-042 报告中心真实度量：只读「总分总」三级聚合服务。
// 事实唯一来源是运行记录六表与运行目录 network.log（F-030 传输层 duration_s）；
// 分位/均值一律用成熟统计库 github.com/montanaflynn/stats 计算，不手写排序与插值。
// 接口耗时读取由 report_interface_scan.go 的快速聚合器按路径运行目录全量统计，
// 不再借用运行编排服务的逐尝试归属（那条路径要读 step.log 且会漏计登录/结构读取类请求）。
// 内网项目不脱敏：本服务只做聚合与安全统计，不额外搬运请求/响应正文，正文仍在既有日志与运行详情入口。
package service

import (
	"context"
	"math"
	"time"

	"github.com/montanaflynn/stats"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// interfaceLogSampleCap 是单次报告读取 network.log 的路径运行上限（采样上限保护）：
// 接口耗时是唯一需要逐路径运行读日志的指标，超过此上限只统计最近的样本并标记已截断。
const interfaceLogSampleCap = 200

// LatencyStats 是一组耗时样本的分位摘要（毫秒）。SampleCount=0 时前端显示「暂无足够样本」，不显示 0。
type LatencyStats struct {
	SampleCount int     `json:"sampleCount"`
	P50         float64 `json:"p50"`
	P90         float64 `json:"p90"`
	P95         float64 `json:"p95"`
	Mean        float64 `json:"mean"`
}

// ReportMetrics 是一个范围（全局/计划/路径）的专业测试指标聚合，全部来自真实运行事实。
type ReportMetrics struct {
	FinishedPathRuns  int     `json:"finishedPathRuns"`  // 已结束路径运行数（成功率分母）
	SucceededPathRuns int     `json:"succeededPathRuns"` // 结果为成功的路径运行数
	SuccessRate       float64 `json:"successRate"`       // 成功率 0-1
	FirstPassRuns     int     `json:"firstPassRuns"`     // 无重试即成功的路径运行数
	FirstPassRate     float64 `json:"firstPassRate"`     // 一次通过率 FPY 0-1
	TotalPaths        int     `json:"totalPaths"`        // 覆盖度分母：已保存路径总数
	CoveredPaths      int     `json:"coveredPaths"`      // 至少运行过一次的路径数
	PathCoverage      float64 `json:"pathCoverage"`      // 路径覆盖度 0-1

	StepLatencyMs           LatencyStats `json:"stepLatencyMs"`           // 步骤尝试耗时分位
	InterfaceLatencyMs      LatencyStats `json:"interfaceLatencyMs"`      // 目标接口传输耗时分位（口径独立于步骤）
	InterfaceKnown          bool         `json:"interfaceKnown"`          // 是否读到任何真实接口耗时
	ExecutorLatencyMs       LatencyStats `json:"executorLatencyMs"`       // 执行器墙钟耗时分位
	InterfaceWriteLatencyMs LatencyStats `json:"interfaceWriteLatencyMs"` // 写请求接口耗时分位

	ResultDistribution map[string]int `json:"resultDistribution"` // 中文结果/停机分类 -> 计数
	FailureClasses     map[string]int `json:"failureClasses"`     // 中文失败分类 -> 计数

	InterfaceCalls      int `json:"interfaceCalls"`      // 目标接口调用总次数
	InterfaceReadCalls  int `json:"interfaceReadCalls"`  // 读请求次数
	InterfaceWriteCalls int `json:"interfaceWriteCalls"` // 写请求次数

	RetryAttempts   int `json:"retryAttempts"`   // 重试尝试数（尝试序号>1 或重放）
	RetriedPathRuns int `json:"retriedPathRuns"` // 发生重试的路径运行数

	SampleCapped bool `json:"sampleCapped"` // 是否触达采样上限（接口耗时只统计了最近样本）
}

// ReportPlanCardDTO 是首页计划维度表的一行。
type ReportPlanCardDTO struct {
	PlanID             uint64         `json:"planId"`
	PlanName           string         `json:"planName"`
	RunCount           int            `json:"runCount"`
	PathRunCount       int            `json:"pathRunCount"`
	SucceededPathRuns  int            `json:"succeededPathRuns"`
	FinishedPathRuns   int            `json:"finishedPathRuns"`
	SuccessRate        float64        `json:"successRate"`
	ResultDistribution map[string]int `json:"resultDistribution"` // 中文结果/停机分类 -> 计数
	CoveredPaths       int            `json:"coveredPaths"`
	TotalPaths         int            `json:"totalPaths"`
	PathCoverage       float64        `json:"pathCoverage"`
	LastRunNo          *uint64        `json:"lastRunNo,omitempty"`
	LastRunResultName  string         `json:"lastRunResultName,omitempty"`
	LastRunFinishedAt  *time.Time     `json:"lastRunFinishedAt,omitempty"`
}

// ReportOverviewDTO 是 L0 首页：上半计划情况计数与计划表，下半全局专业指标。
type ReportOverviewDTO struct {
	Counts  repository.ReportCounts `json:"counts"`
	Plans   []ReportPlanCardDTO     `json:"plans"`
	Metrics ReportMetrics           `json:"metrics"`
	// GeneratedAt 是本次统计的服务端时刻，前端据此显示「数据截至 …」。
	GeneratedAt time.Time `json:"generatedAt"`
}

// ReportPathCardDTO 是计划详情里各执行路径的一行。
type ReportPathCardDTO struct {
	ExecutionPathID    uint64       `json:"executionPathId"`
	SequenceNo         int          `json:"sequenceNo"`
	Name               string       `json:"name"`
	PathRunCount       int          `json:"pathRunCount"`
	SucceededPathRuns  int          `json:"succeededPathRuns"`
	FinishedPathRuns   int          `json:"finishedPathRuns"`
	SuccessRate        float64      `json:"successRate"`
	AvgSteps           float64      `json:"avgSteps"`
	StepLatencyMs      LatencyStats `json:"stepLatencyMs"`
	InterfaceLatencyMs LatencyStats `json:"interfaceLatencyMs"`
	LastResultName     string       `json:"lastResultName"`
}

// ReportPlanDetailDTO 是 L1 计划详情：计划头部、计划级指标与各路径指标。
type ReportPlanDetailDTO struct {
	PlanID           uint64              `json:"planId"`
	PlanName         string              `json:"planName"`
	Account          string              `json:"account"`
	FlowSource       string              `json:"flowSource"`
	TargetObjectName string              `json:"targetObjectName"`
	TotalPaths       int                 `json:"totalPaths"`
	RunCount         int                 `json:"runCount"`
	Metrics          ReportMetrics       `json:"metrics"`
	Paths            []ReportPathCardDTO `json:"paths"`
	// GeneratedAt 是本次统计的服务端时刻，前端据此显示「数据截至 …」，让用户判断数字有多新。
	GeneratedAt time.Time `json:"generatedAt"`
}

// ReportRunInstanceDTO 是路径详情里一次运行实例的一行；点击进入现有运行详情页（L3）。
// 耗时三分解按 F-042 第 4.3 节口径：接口耗时(目标传输)/步骤总耗时/执行器耗时/工具端编排耗时(派生)。
// 派生的编排耗时在任一分量不可靠或相减为负时标记为未知，绝不硬凑负数或用 0 冒充。
type ReportRunInstanceDTO struct {
	RunID           uint64     `json:"runId"`
	RunNo           uint64     `json:"runNo"`
	PathRunID       uint64     `json:"pathRunId"`
	StatusName      string     `json:"statusName"`
	ResultName      string     `json:"resultName"`
	StepCount       int        `json:"stepCount"`
	TotalDurationMs int64      `json:"totalDurationMs"`
	InterfaceCalls  int        `json:"interfaceCalls"`
	MainInstanceRef string     `json:"mainInstanceRef"`
	StartedAt       *time.Time `json:"startedAt,omitempty"`
	// 耗时三分解（毫秒）；带 Known 布尔，前端据此显示「未知」而非 0。
	InterfaceDurationMs      float64 `json:"interfaceDurationMs"`
	InterfaceWriteDurationMs float64 `json:"interfaceWriteDurationMs"`
	InterfaceDurationKnown   bool    `json:"interfaceDurationKnown"`
	ExecutorDurationMs       int64   `json:"executorDurationMs"`
	ExecutorDurationKnown    bool    `json:"executorDurationKnown"`
	OrchestrationDurationMs  int64   `json:"orchestrationDurationMs"`
	OrchestrationKnown       bool    `json:"orchestrationKnown"`
}

// ReportPathDetailDTO 是 L2 路径详情：路径头部、路径级指标与运行实例列表。
type ReportPathDetailDTO struct {
	PlanID              uint64                 `json:"planId"`
	ExecutionPathID     uint64                 `json:"executionPathId"`
	SequenceNo          int                    `json:"sequenceNo"`
	Name                string                 `json:"name"`
	ConfigurationStatus string                 `json:"configurationStatus"`
	RunCount            int                    `json:"runCount"`
	Metrics             ReportMetrics          `json:"metrics"`
	Runs                []ReportRunInstanceDTO `json:"runs"`
	// GeneratedAt 是本次统计的服务端时刻，前端据此显示「数据截至 …」。
	GeneratedAt time.Time `json:"generatedAt"`
}

// ReportCenterService 组装报告中心的三级只读聚合。
type ReportCenterService struct {
	store   repository.ReportStore
	plans   *PlanService
	paths   repository.ExecutionPathRepository
	scanner *ReportInterfaceScanner
	now     func() time.Time
}

// NewReportCenterService 构造报告中心服务。
// logRoot 是日志根目录：接口耗时由快速聚合器直接读各路径运行目录的 network.log，
// 不再借用运行编排服务（那条路径要读 step.log 且会把登录/结构读取类请求漏计）。
func NewReportCenterService(store repository.ReportStore, plans *PlanService, paths repository.ExecutionPathRepository, logRoot string, now func() time.Time) *ReportCenterService {
	if now == nil {
		now = time.Now
	}
	return &ReportCenterService{
		store: store, plans: plans, paths: paths,
		scanner: NewReportInterfaceScanner(logRoot), now: now,
	}
}

// Overview 组装 L0 首页：全量计数、计划维度表与全局专业指标。
func (s *ReportCenterService) Overview(ctx context.Context) (ReportOverviewDTO, error) {
	counts, err := s.store.ReportCounts(ctx)
	if err != nil {
		return ReportOverviewDTO{}, err
	}
	rollups, err := s.store.ReportPlanRollups(ctx)
	if err != nil {
		return ReportOverviewDTO{}, err
	}
	rollupByPlan := map[uint64]repository.ReportPlanRollup{}
	for _, rollup := range rollups {
		rollupByPlan[rollup.PlanID] = rollup
	}
	plans, err := s.plans.List(ctx, "", "")
	if err != nil {
		return ReportOverviewDTO{}, err
	}
	totalPaths := 0
	cards := make([]ReportPlanCardDTO, 0, len(plans))
	for _, plan := range plans {
		totalPaths += plan.PathCount
		rollup := rollupByPlan[plan.ID]
		card := ReportPlanCardDTO{
			PlanID: plan.ID, PlanName: plan.Name,
			RunCount: rollup.RunCount, PathRunCount: rollup.PathRunCount,
			SucceededPathRuns: rollup.PathRunSucceeded, FinishedPathRuns: rollup.PathRunFinished,
			SuccessRate:        rate(rollup.PathRunSucceeded, rollup.PathRunFinished),
			ResultDistribution: rollupDistribution(rollup),
			CoveredPaths:       rollup.CoveredPaths,
			TotalPaths:         plan.PathCount,
			PathCoverage:       rate(rollup.CoveredPaths, plan.PathCount),
			LastRunNo:          plan.LastRunNo,
			LastRunResultName:  runResultName(plan.LastRunResult),
			LastRunFinishedAt:  plan.LastRunFinishedAt,
		}
		cards = append(cards, card)
	}
	pathRunRows, err := s.store.ReportPathRunRows(ctx, repository.ReportScope{})
	if err != nil {
		return ReportOverviewDTO{}, err
	}
	attemptRows, err := s.store.ReportAttemptRows(ctx, repository.ReportScope{})
	if err != nil {
		return ReportOverviewDTO{}, err
	}
	ifaceByPathRun, capped := s.interfaceStatsByPathRun(ctx, repository.ReportScope{})
	metrics := computeMetrics(pathRunRows, attemptRows, ifaceByPathRun, totalPaths, capped)
	return ReportOverviewDTO{Counts: counts, Plans: cards, Metrics: metrics, GeneratedAt: s.now()}, nil
}

// PlanReport 组装 L1 计划详情：计划头部、计划级指标与各路径指标。
func (s *ReportCenterService) PlanReport(ctx context.Context, planID uint64) (ReportPlanDetailDTO, error) {
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return ReportPlanDetailDTO{}, err
	}
	savedPaths, err := s.paths.List(ctx, planID)
	if err != nil {
		return ReportPlanDetailDTO{}, err
	}
	pathRunRows, err := s.store.ReportPathRunRows(ctx, repository.ReportScope{PlanID: planID})
	if err != nil {
		return ReportPlanDetailDTO{}, err
	}
	attemptRows, err := s.store.ReportAttemptRows(ctx, repository.ReportScope{PlanID: planID})
	if err != nil {
		return ReportPlanDetailDTO{}, err
	}
	// 接口耗时按计划范围取：一次查询拿日志目录，再有界并发扫描（只读 network.log）。
	ifaceByPathRun, capped := s.interfaceStatsByPathRun(ctx, repository.ReportScope{PlanID: planID})
	metrics := computeMetrics(pathRunRows, attemptRows, ifaceByPathRun, plan.PathCount, capped)

	rowsByPath := map[uint64][]repository.ReportPathRunRow{}
	for _, row := range pathRunRows {
		rowsByPath[row.ExecutionPathID] = append(rowsByPath[row.ExecutionPathID], row)
	}
	attemptsByPath := map[uint64][]repository.ReportAttemptRow{}
	pathRunToPath := map[uint64]uint64{}
	for _, row := range pathRunRows {
		pathRunToPath[row.PathRunID] = row.ExecutionPathID
	}
	for _, attempt := range attemptRows {
		if pathID, ok := pathRunToPath[attempt.PathRunID]; ok {
			attemptsByPath[pathID] = append(attemptsByPath[pathID], attempt)
		}
	}
	cards := make([]ReportPathCardDTO, 0, len(savedPaths))
	runCount := 0
	for _, path := range savedPaths {
		pathRows := rowsByPath[path.ID]
		pathMetrics := computeMetrics(pathRows, attemptsByPath[path.ID], ifaceByPathRun, 1, capped)
		runCount += len(pathRows)
		card := ReportPathCardDTO{
			ExecutionPathID: path.ID, SequenceNo: int(path.SequenceNo), Name: pathDisplayName(path),
			PathRunCount: len(pathRows), SucceededPathRuns: pathMetrics.SucceededPathRuns,
			FinishedPathRuns: pathMetrics.FinishedPathRuns, SuccessRate: pathMetrics.SuccessRate,
			AvgSteps: avgStepsOf(pathRows, attemptsByPath[path.ID]), StepLatencyMs: pathMetrics.StepLatencyMs,
			InterfaceLatencyMs: pathMetrics.InterfaceLatencyMs, LastResultName: lastResultName(pathRows),
		}
		cards = append(cards, card)
	}
	return ReportPlanDetailDTO{
		PlanID: plan.ID, PlanName: plan.Name, Account: plan.Account, FlowSource: plan.FlowSource,
		TargetObjectName: plan.TargetObjectName, TotalPaths: plan.PathCount, RunCount: runCount,
		Metrics: metrics, Paths: cards, GeneratedAt: s.now(),
	}, nil
}

// PathReport 组装 L2 路径详情：路径头部、路径级指标与运行实例列表。
func (s *ReportCenterService) PathReport(ctx context.Context, planID, executionPathID uint64) (ReportPathDetailDTO, error) {
	path, err := s.paths.Get(ctx, planID, executionPathID)
	if err != nil {
		return ReportPathDetailDTO{}, err
	}
	scope := repository.ReportScope{PlanID: planID, ExecutionPathID: executionPathID}
	pathRunRows, err := s.store.ReportPathRunRows(ctx, scope)
	if err != nil {
		return ReportPathDetailDTO{}, err
	}
	attemptRows, err := s.store.ReportAttemptRows(ctx, scope)
	if err != nil {
		return ReportPathDetailDTO{}, err
	}
	// 接口耗时按路径范围取：一次查询拿日志目录，再有界并发扫描（只读 network.log）。
	ifaceByPathRun, capped := s.interfaceStatsByPathRun(ctx, scope)
	coveredTotal := 1
	metrics := computeMetrics(pathRunRows, attemptRows, ifaceByPathRun, coveredTotal, capped)

	stepsByPathRun := map[uint64]map[uint64]struct{}{}
	durationByPathRun := map[uint64]int64{}
	executorByPathRun := map[uint64]int64{}
	executorKnownByPathRun := map[uint64]bool{}
	for _, attempt := range attemptRows {
		if stepsByPathRun[attempt.PathRunID] == nil {
			stepsByPathRun[attempt.PathRunID] = map[uint64]struct{}{}
		}
		stepsByPathRun[attempt.PathRunID][attempt.StepID] = struct{}{}
		durationByPathRun[attempt.PathRunID] += attempt.DurationMs
		if attempt.ExecutorMsKnown {
			executorByPathRun[attempt.PathRunID] += attempt.ExecutorMs
			executorKnownByPathRun[attempt.PathRunID] = true
		}
	}
	runs := make([]ReportRunInstanceDTO, 0, len(pathRunRows))
	for _, row := range pathRunRows {
		iface := ifaceByPathRun[row.PathRunID]
		stepDuration := durationByPathRun[row.PathRunID]
		interfaceDuration := round1(sumFloat(iface.DurationsMs))
		executorDuration := executorByPathRun[row.PathRunID]
		executorKnown := executorKnownByPathRun[row.PathRunID]
		// 工具端编排耗时 = 步骤总耗时 − 接口传输耗时 − 执行器耗时；
		// 任一分量不可靠或相减为负（runtime 步骤的执行器耗时天然包含接口等待，见 4.4）时标记未知。
		orchestration := stepDuration - int64(interfaceDuration) - executorDuration
		orchestrationKnown := iface.AnyKnown && executorKnown && orchestration >= 0
		runs = append(runs, ReportRunInstanceDTO{
			RunID: row.RunID, RunNo: row.RunNo, PathRunID: row.PathRunID,
			StatusName: model.PathRunStatusName(row.Status), ResultName: pathRunOutcomeLabel(row),
			StepCount: len(stepsByPathRun[row.PathRunID]), TotalDurationMs: stepDuration,
			InterfaceCalls: iface.ReadCount + iface.WriteCount, MainInstanceRef: row.MainInstanceRef,
			StartedAt:                row.StartedAt,
			InterfaceDurationMs:      interfaceDuration,
			InterfaceWriteDurationMs: round1(sumFloat(iface.WriteDurationsMs)),
			InterfaceDurationKnown:   iface.AnyKnown,
			ExecutorDurationMs:       executorDuration,
			ExecutorDurationKnown:    executorKnown,
			OrchestrationDurationMs:  orchestration,
			OrchestrationKnown:       orchestrationKnown,
		})
	}
	return ReportPathDetailDTO{
		PlanID: planID, ExecutionPathID: executionPathID, SequenceNo: int(path.SequenceNo),
		Name: pathDisplayName(path), ConfigurationStatus: path.ConfigurationStatus,
		RunCount: len(pathRunRows), Metrics: metrics, Runs: runs, GeneratedAt: s.now(),
	}, nil
}

// interfaceStatsByPathRun 为一个范围读取接口耗时事实：
// 先用一条查询取回范围内每条路径运行的日志定位，再交给快速聚合器有界并发扫描。
// 仓储读取失败时按「无接口事实」降级，不让整页报告失败（前端显示「暂无真实请求耗时」）。
func (s *ReportCenterService) interfaceStatsByPathRun(ctx context.Context, scope repository.ReportScope) (map[uint64]ReportInterfaceStats, bool) {
	out := map[uint64]ReportInterfaceStats{}
	if s.scanner == nil {
		return out, false
	}
	dirs, err := s.store.ReportPathRunLogDirs(ctx, scope)
	if err != nil {
		return out, false
	}
	return s.scanner.Aggregate(ctx, dirs)
}

// computeMetrics 从路径运行行、尝试行与接口耗时事实计算一个范围的专业测试指标。
// 全部指标只用真实事实：无样本时分位 SampleCount=0（前端显示暂无样本），接口无日志时 InterfaceKnown=false。
func computeMetrics(pathRunRows []repository.ReportPathRunRow, attemptRows []repository.ReportAttemptRow, ifaceByPathRun map[uint64]ReportInterfaceStats, totalPaths int, capped bool) ReportMetrics {
	metrics := ReportMetrics{
		ResultDistribution: map[string]int{},
		FailureClasses:     map[string]int{},
		TotalPaths:         totalPaths,
		SampleCapped:       capped,
	}
	// 每条路径运行是否发生过重试：尝试序号大于 1 或标记为重放即计入。
	retriedPathRun := map[uint64]bool{}
	for _, attempt := range attemptRows {
		if attempt.AttemptNo > 1 || attempt.IsReplay {
			retriedPathRun[attempt.PathRunID] = true
			metrics.RetryAttempts++
		}
	}
	metrics.RetriedPathRuns = len(retriedPathRun)

	coveredPaths := map[uint64]struct{}{}
	for _, row := range pathRunRows {
		coveredPaths[row.ExecutionPathID] = struct{}{}
		if !model.IsTerminalPathRunStatus(row.Status) {
			continue
		}
		metrics.FinishedPathRuns++
		metrics.ResultDistribution[pathRunOutcomeLabel(row)]++
		if row.FailureClass != nil {
			metrics.FailureClasses[model.FailureClassName(*row.FailureClass)]++
		}
		if pathRunSucceeded(row) {
			metrics.SucceededPathRuns++
			if !retriedPathRun[row.PathRunID] {
				metrics.FirstPassRuns++
			}
		}
	}
	metrics.CoveredPaths = len(coveredPaths)
	metrics.SuccessRate = rate(metrics.SucceededPathRuns, metrics.FinishedPathRuns)
	metrics.FirstPassRate = rate(metrics.FirstPassRuns, metrics.FinishedPathRuns)
	metrics.PathCoverage = rate(metrics.CoveredPaths, totalPaths)

	stepSamples := make([]float64, 0, len(attemptRows))
	executorSamples := make([]float64, 0, len(attemptRows))
	for _, attempt := range attemptRows {
		stepSamples = append(stepSamples, float64(attempt.DurationMs))
		if attempt.ExecutorMsKnown {
			executorSamples = append(executorSamples, float64(attempt.ExecutorMs))
		}
	}
	metrics.StepLatencyMs = latencyOf(stepSamples)
	metrics.ExecutorLatencyMs = latencyOf(executorSamples)

	interfaceSamples := []float64{}
	writeSamples := []float64{}
	for _, row := range pathRunRows {
		stat, ok := ifaceByPathRun[row.PathRunID]
		if !ok {
			continue
		}
		metrics.InterfaceReadCalls += stat.ReadCount
		metrics.InterfaceWriteCalls += stat.WriteCount
		if stat.AnyKnown {
			metrics.InterfaceKnown = true
		}
		interfaceSamples = append(interfaceSamples, stat.DurationsMs...)
		writeSamples = append(writeSamples, stat.WriteDurationsMs...)
	}
	metrics.InterfaceCalls = metrics.InterfaceReadCalls + metrics.InterfaceWriteCalls
	metrics.InterfaceLatencyMs = latencyOf(interfaceSamples)
	metrics.InterfaceWriteLatencyMs = latencyOf(writeSamples)
	return metrics
}

// latencyOf 用统计库计算 P50/P90/P95 与均值；样本为空返回零值（SampleCount=0）。
func latencyOf(samples []float64) LatencyStats {
	if len(samples) == 0 {
		return LatencyStats{}
	}
	data := stats.Float64Data(samples)
	p50, _ := data.Percentile(50)
	p90, _ := data.Percentile(90)
	p95, _ := data.Percentile(95)
	mean, _ := data.Mean()
	return LatencyStats{SampleCount: len(samples), P50: round1(p50), P90: round1(p90), P95: round1(p95), Mean: round1(mean)}
}

// rate 计算比率，分母为 0 时返回 0（前端据 SampleCount/分母判断是否显示暂无）。
func rate(numerator, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return round4(float64(numerator) / float64(denominator))
}

// round1 保留一位小数（毫秒展示够用，避免浮点尾数噪音）。
func round1(value float64) float64 {
	return math.Round(value*10) / 10
}

// round4 保留四位小数（比率）。
func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}

// sumFloat 求和一组耗时样本（毫秒）。
func sumFloat(values []float64) float64 {
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total
}

// pathRunSucceeded 判断一次已结束路径运行是否成功：结果为成功，或已完成且未记结果。
func pathRunSucceeded(row repository.ReportPathRunRow) bool {
	if row.Result != nil {
		return *row.Result == model.RunResultSucceeded
	}
	return row.Status == model.PathRunStatusCompleted
}

// pathRunOutcomeLabel 把一次路径运行归入中文结果/停机分类：已停止/已取消看状态，其余看结果。
func pathRunOutcomeLabel(row repository.ReportPathRunRow) string {
	switch row.Status {
	case model.PathRunStatusStopped:
		return "已停止"
	case model.PathRunStatusCancelled:
		return "已取消"
	case model.PathRunStatusAwaitingReconciliation:
		return "结果待确认"
	}
	if name := runResultName(row.Result); name != "" {
		return name
	}
	switch row.Status {
	case model.PathRunStatusCompleted:
		return "成功"
	case model.PathRunStatusFailed:
		return "失败"
	default:
		return model.PathRunStatusName(row.Status)
	}
}

// runResultName 返回路径结果的中文名；nil 返回空串。
func runResultName(result *model.RunResult) string {
	if result == nil {
		return ""
	}
	switch *result {
	case model.RunResultSucceeded:
		return "成功"
	case model.RunResultFailed:
		return "失败"
	case model.RunResultAwaitingReconcile:
		return "结果待确认"
	default:
		return string(*result)
	}
}

// rollupDistribution 把计划汇总的结果/停机计数拼成中文分类映射，只保留非零项供表格展示。
func rollupDistribution(rollup repository.ReportPlanRollup) map[string]int {
	distribution := map[string]int{}
	for label, count := range map[string]int{
		"成功":    rollup.PathRunSucceeded,
		"失败":    rollup.PathRunFailed,
		"已停止":   rollup.PathRunStopped,
		"已取消":   rollup.PathRunCancelled,
		"结果待确认": rollup.PathRunAwaiting,
	} {
		if count > 0 {
			distribution[label] = count
		}
	}
	return distribution
}

// lastResultName 返回该路径最新一次路径运行的中文结果（行按最新在前）。
func lastResultName(rows []repository.ReportPathRunRow) string {
	if len(rows) == 0 {
		return ""
	}
	return pathRunOutcomeLabel(rows[0])
}

// avgStepsOf 计算该路径每次运行的平均步骤数（去重 step_id 后按路径运行数平均）。
func avgStepsOf(rows []repository.ReportPathRunRow, attempts []repository.ReportAttemptRow) float64 {
	if len(rows) == 0 {
		return 0
	}
	steps := map[uint64]map[uint64]struct{}{}
	for _, attempt := range attempts {
		if steps[attempt.PathRunID] == nil {
			steps[attempt.PathRunID] = map[uint64]struct{}{}
		}
		steps[attempt.PathRunID][attempt.StepID] = struct{}{}
	}
	total := 0
	for _, set := range steps {
		total += len(set)
	}
	return round1(float64(total) / float64(len(rows)))
}
