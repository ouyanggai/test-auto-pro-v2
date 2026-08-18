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
	// DataStatus 独立表示表单数据准备，不参与节点人员与动作配置完成度。
	DataStatus string
	// DataDetail 是数据准备状态的用户可读原因，不暴露表单字段键或分支标识。
	DataDetail string
	// Included 表示用户是否已将该路径纳入本次运行准备，不会启动目标流程。
	Included bool
	// ConfigurationRevision 是轻量列表中保存纳入标记所需的节点配置修订号，不包含完整节点配置。
	ConfigurationRevision uint64
	Choices               []ExecutionPathChoice
	CreatedAt             time.Time
	UpdatedAt             time.Time
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
