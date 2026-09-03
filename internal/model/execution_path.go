package model

import "time"

const (
	ExecutionPathConfigurationConfigured = "configured"
)

// ExecutionPathChoice 记录一个真实路由节点选择的分支。
type ExecutionPathChoice struct {
	RouteNodeID string `json:"routeNodeId"`
	BranchID    string `json:"branchId"`
}

// ExecutionPath 保存计划下单条执行路径及其轻量准备状态。
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
	// ConfigurationRevision 冻结路径配置修订，供历史回放任务复验检查点。
	ConfigurationRevision uint64
	Choices               []ExecutionPathChoice
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// IsExecutionPathRunnable 只根据路径节点配置和数据准备状态判断未来运行资格。
func IsExecutionPathRunnable(path ExecutionPath) bool {
	if path.ConfigurationStatus != ExecutionPathConfigurationConfigured {
		return false
	}
	return path.DataStatus == HistoryDataStatusReady
}

// ExecutionPathBatchResult 汇总一次路径批量生成结果。
type ExecutionPathBatchResult struct {
	TotalCount    int
	ExistingCount int
	CreatedCount  int
	Paths         []ExecutionPath
}

// ExecutionPathAnalysis 描述选择集合在真实流程图中的完整性。
type ExecutionPathAnalysis struct {
	Complete            bool
	MissingRouteNodeIDs []string
	ReachableNodeIDs    []string
	ReachableEdgeIDs    []string
}
