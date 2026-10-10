package repository

import (
	"context"
	"time"

	"test-auto-pro-v2/internal/model"
)

// F-042 报告中心：只读聚合仓储面。全部方法只读运行事实，不写任何表、不调用目标平台。
// 计数类用 COUNT/GROUP BY 一次算出（非采样、无 N+1）；样本类（路径运行行、尝试行）按范围有界拉取，
// 由服务层计算分位与派生指标，避免逐路径运行发起查询。

// ReportScope 约束报告聚合范围：PlanID/ExecutionPathID 均为 0 表示全局；
// 只给 PlanID 表示单计划；两者都给表示单路径。Limit 是样本行上限（0 用默认值）。
type ReportScope struct {
	PlanID          uint64
	ExecutionPathID uint64
	Limit           int
}

// ReportCounts 是报告首页第一级计划情况计数，全部来自 COUNT 聚合，代表全量事实而非采样。
type ReportCounts struct {
	PlansTotal    int `json:"plansTotal"`
	PlansWithRuns int `json:"plansWithRuns"`
	RunsTotal     int `json:"runsTotal"`
	RunsRunning   int `json:"runsRunning"`
	PathRunsTotal int `json:"pathRunsTotal"`
}

// ReportPlanRollup 是单个计划的运行事实汇总，由一次分组聚合得到（避免逐计划查询）。
// 结果分布与覆盖度供首页计划表直接展示，全部来自 COUNT/SUM 聚合。
type ReportPlanRollup struct {
	PlanID           uint64
	RunCount         int
	PathRunCount     int
	PathRunFinished  int
	PathRunSucceeded int
	PathRunFailed    int
	PathRunStopped   int
	PathRunCancelled int
	PathRunAwaiting  int
	CoveredPaths     int
}

// ReportPathRunRow 是一条路径运行的轻量事实行，供成功率、分类分布与运行实例列表复用。
type ReportPathRunRow struct {
	PathRunID       uint64
	RunID           uint64
	RunNo           uint64
	PlanID          uint64
	ExecutionPathID uint64
	Status          model.PathRunStatus
	Result          *model.RunResult
	FailureClass    *model.FailureClass
	MainInstanceRef string
	TotalSteps      *int
	StartedAt       *time.Time
	FinishedAt      *time.Time
}

// ReportAttemptRow 是一条尝试的轻量事实行，供步骤/执行器耗时分位、重试与一次通过率计算。
type ReportAttemptRow struct {
	PathRunID       uint64
	StepID          uint64
	AttemptNo       int
	Verdict         string
	FailureClass    *model.FailureClass
	DurationMs      int64
	ExecutorMs      int64
	ExecutorMsKnown bool
	IsReplay        bool
}

// ReportPathRunLogDir 是一条路径运行的日志定位事实：报告中心据此直接读该路径运行目录的 network.log。
// 旧实现对每条路径运行各查一次尝试行（N+1），本类型让一次查询取回全部定位信息，报告侧不再碰尝试表。
type ReportPathRunLogDir struct {
	PathRunID uint64
	Status    model.PathRunStatus
	// LogPath 是该路径运行任一已落账尝试的 step.log 相对路径；空表示这次运行没有落过任何日志。
	// 与运行详情同口径：同一次路径运行的所有日志都在同一目录，取任一条即可定位目录。
	LogPath string
}

// ReportStore 是报告中心只读聚合仓储；实现只读运行记录六表，绝不写库或读目标平台业务库。
type ReportStore interface {
	// ReportCounts 返回首页第一级计划情况计数（全量 COUNT）。
	ReportCounts(ctx context.Context) (ReportCounts, error)
	// ReportPlanRollups 返回每个有运行记录的计划的运行/路径运行汇总（一次分组聚合）。
	ReportPlanRollups(ctx context.Context) ([]ReportPlanRollup, error)
	// ReportPathRunRows 按范围有界返回路径运行事实行（最新在前）。
	ReportPathRunRows(ctx context.Context, scope ReportScope) ([]ReportPathRunRow, error)
	// ReportAttemptRows 按范围有界返回尝试事实行（最新在前）。
	ReportAttemptRows(ctx context.Context, scope ReportScope) ([]ReportAttemptRow, error)
	// ReportPathRunLogDirs 按范围一次返回每条路径运行的日志定位（最新在前，分组聚合，无 N+1）。
	ReportPathRunLogDirs(ctx context.Context, scope ReportScope) ([]ReportPathRunLogDir, error)
}
