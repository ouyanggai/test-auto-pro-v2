package model

type FlowGraph struct {
	PlanID     uint64
	TargetName string
	FlowSource string
	Nodes      []FlowGraphNode
	Edges      []FlowGraphEdge
	Warnings   []string
}

type FlowGraphNode struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	TypeName      string `json:"typeName"`
	MergeTargetID string `json:"mergeTargetId,omitempty"`
}

type FlowGraphEdge struct {
	ID       string `json:"id"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	Kind     string `json:"kind"`
	Label    string `json:"label"`
	BranchID string `json:"branchId"`
}
