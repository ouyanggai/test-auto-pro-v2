package model

import "time"

type PlanStatus string

const (
	PlanStatusPendingConfiguration PlanStatus = "pending_configuration"
	PlanStatusReady                PlanStatus = "ready"
	PlanStatusRunning              PlanStatus = "running"
	PlanStatusCompleted            PlanStatus = "completed"
)

func ValidPlanStatus(value PlanStatus) bool {
	switch value {
	case PlanStatusPendingConfiguration, PlanStatusReady, PlanStatusRunning, PlanStatusCompleted:
		return true
	default:
		return false
	}
}

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
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type PlanListFilter struct {
	Name   string
	Status PlanStatus
	Limit  int
}
