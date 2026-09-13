package model

import "time"

// PlanStatus 表示计划是否已经进入真实运行事实，与路径配置完成度无关。
type PlanStatus string

const (
	PlanStatusNotStarted PlanStatus = "not_started"
	PlanStatusRunning    PlanStatus = "running"
	PlanStatusCompleted  PlanStatus = "completed"
)

// ValidPlanStatus 判断公开计划状态是否属于当前三态协议。
func ValidPlanStatus(value PlanStatus) bool {
	switch value {
	case PlanStatusNotStarted, PlanStatusRunning, PlanStatusCompleted:
		return true
	default:
		return false
	}
}

// Plan 保存测试计划的持久化身份和运行事实摘要。
type Plan struct {
	ID                 uint64
	Name               string
	Account            string
	AccountDisplayName string
	FlowSource         string
	TargetObjectID     string
	TargetObjectName   string
	RunMode            string
	MaxConcurrency     *int
	ScheduledAt        *time.Time
	// ScheduledConsumedAt 是单次定时启动的消费时刻：非空表示已触发过，绝不再扫描触发（F-020）。
	ScheduledConsumedAt *time.Time
	Status              PlanStatus
	PathCount           int
	// 以下五个字段是最近一次运行与活跃运行的事实摘要，由列表/详情查询从 runs 表一次 SQL 聚合（F-032）：
	// 计划状态公开口径的「已运行」以运行事实为准，不依赖 test_plans.status 是否被及时同步。
	// LastRunNo 为空表示计划从未运行过；HasActiveRun 表示存在等待中或运行中的运行记录。
	LastRunNo         *uint64
	LastRunStatus     RunStatus
	LastRunResult     *RunResult
	LastRunFinishedAt *time.Time
	HasActiveRun      bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// PlanListFilter 约束计划列表的名称、状态和数量。
type PlanListFilter struct {
	Name   string
	Status PlanStatus
	Limit  int
}
