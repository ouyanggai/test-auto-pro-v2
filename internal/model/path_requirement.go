package model

// RequirementStatus 是路径要求页面允许公开的四类中文状态。
type RequirementStatus string

const (
	RequirementPending   RequirementStatus = "待配置"
	RequirementAutomatic RequirementStatus = "目标平台自动确定"
	RequirementRuntime   RequirementStatus = "运行时确定"
	RequirementReview    RequirementStatus = "需要人工核对"
)

// PathRequirements 是单条已保存路径基于当前真实流程生成的只读要求。
type PathRequirements struct {
	Path    PathRequirementPath `json:"path"`
	Summary []RequirementCount  `json:"summary"`
	Groups  []RequirementGroup  `json:"groups"`
}

// PathRequirementPath 只公开稳定序号和名称，不公开本地或目标内部标识。
type PathRequirementPath struct {
	SequenceNo uint   `json:"sequenceNo"`
	Name       string `json:"name"`
}

// RequirementCount 是一种中文状态的要求项数量。
type RequirementCount struct {
	Status RequirementStatus `json:"status"`
	Count  int               `json:"count"`
}

// RequirementGroup 表示主线或一条并行分支的只读节点要求集合。
type RequirementGroup struct {
	Title string            `json:"title"`
	Kind  string            `json:"kind"`
	Nodes []RequirementNode `json:"nodes"`
}

// RequirementNode 使用业务名称和中文类型描述一个可达节点。
type RequirementNode struct {
	Name     string            `json:"name"`
	TypeName string            `json:"typeName"`
	Items    []RequirementItem `json:"items"`
}

// RequirementItem 是条件、人员、动作或约束的一条安全中文表达。
type RequirementItem struct {
	Category string            `json:"category"`
	Title    string            `json:"title"`
	Detail   string            `json:"detail"`
	Status   RequirementStatus `json:"status"`
}
