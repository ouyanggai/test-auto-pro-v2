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
	Status             PlanStatus
	PathCount          int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// PlanListFilter 约束计划列表的名称、状态和数量。
type PlanListFilter struct {
	Name   string
	Status PlanStatus
	Limit  int
}
