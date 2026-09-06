package model

type FlowGraph struct {
	PlanID       uint64
	TargetName   string
	FlowSource   string
	EntryNodeIDs []string
	Nodes        []FlowGraphNode
	Edges        []FlowGraphEdge
	Warnings     []string
}

type FlowGraphNode struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	TypeName      string `json:"typeName"`
	MergeTargetID string `json:"mergeTargetId,omitempty"`
	// AuditType 是目标在该节点上配置的审批方式（run_node_choose/company/level 等）。
	// 仅供后端内部构造提交载荷（nextAuditorList）使用，json:"-" 不进入公开 DTO：
	// 界面禁止出现目标内部枚举值（纲领第 12.1 节）。
	AuditType string `json:"-"`
}

type FlowGraphEdge struct {
	ID       string `json:"id"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	Kind     string `json:"kind"`
	Label    string `json:"label"`
	BranchID string `json:"branchId"`
}
