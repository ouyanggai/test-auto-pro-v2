package model

import "time"

type ExecutionPathChoice struct {
	RouteNodeID string `json:"routeNodeId"`
	BranchID    string `json:"branchId"`
}

type ExecutionPath struct {
	ID         uint64
	PlanID     uint64
	SequenceNo uint
	Choices    []ExecutionPathChoice
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ExecutionPathAnalysis struct {
	Complete            bool
	MissingRouteNodeIDs []string
	ReachableNodeIDs    []string
	ReachableEdgeIDs    []string
}
