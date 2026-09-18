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
	// StructuralPath 是目标代理树递归遍历得到的真实结构位置，仅供后端绑定使用，不暴露给前端。
	StructuralPath string `json:"-"`
	// AuditPeople 是目标审批配置明细中的展示名称；加签人员属于节点配置，不是独立流程节点。
	AuditPeople []string `json:"auditPeople,omitempty"`
	// AuditType 是目标在该节点上配置的审批方式（run_node_choose/company/level 等）。
	// 仅供后端内部构造提交载荷（nextAuditorList）使用，json:"-" 不进入公开 DTO：
	// 界面禁止出现目标内部枚举值（纲领第 12.1 节）。
	AuditType string `json:"-"`
	// IsSkip 是目标模板声明的“无处理人时跳过该节点”（nil=未声明，语义为不允许跳过）。
	// F-034：执行器据此区分“目标可能跳过”与“目标必须有人”，跳过语义不能靠待办位置猜测。
	// 只供后端运行时使用，json:"-" 不进入公开 DTO。
	IsSkip *bool `json:"-"`
}

type FlowGraphEdge struct {
	ID       string `json:"id"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	Kind     string `json:"kind"`
	Label    string `json:"label"`
	BranchID string `json:"branchId"`
}
