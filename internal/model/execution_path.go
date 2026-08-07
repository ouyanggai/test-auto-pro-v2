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
	Name       string
	// ConfigurationStatus 只表示本地路径配置表是否已有记录，不触发目标读取或完整配置分析。
	ConfigurationStatus string
	Choices             []ExecutionPathChoice
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type ExecutionPathBatchResult struct {
	TotalCount    int
	ExistingCount int
	CreatedCount  int
	Paths         []ExecutionPath
}

type ExecutionPathAnalysis struct {
	Complete            bool
	MissingRouteNodeIDs []string
	ReachableNodeIDs    []string
	ReachableEdgeIDs    []string
}
