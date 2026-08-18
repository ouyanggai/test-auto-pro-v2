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
	// ConfigurationStatus 只表示本地路径配置完成度，不触发目标读取或完整配置分析。
	ConfigurationStatus string
	// ConfigurationDetail 是路径摘要中的用户可读完成原因，不暴露节点键或表单内部状态。
	ConfigurationDetail string
	// Included 表示用户是否已将该路径纳入本次运行准备，不会启动目标流程。
	Included  bool
	Choices   []ExecutionPathChoice
	CreatedAt time.Time
	UpdatedAt time.Time
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
