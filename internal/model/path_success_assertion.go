package model

import "time"

// 目标平台真实存在的流程实例状态取值，来自目标 FlowInstanceStatusEnum。
// 只做取值登记与中文标签映射，不合并同义状态、不自造状态；界面显示目标自己的中文名。
const (
	// FlowInstanceStatusAwaitSent 待发：实例已创建但还没提交。
	FlowInstanceStatusAwaitSent = "await_sent"
	// FlowInstanceStatusDraft 草稿。
	FlowInstanceStatusDraft = "draft"
	// FlowInstanceStatusRun 运行中。
	FlowInstanceStatusRun = "run"
	// FlowInstanceStatusWithdraw 撤销。
	FlowInstanceStatusWithdraw = "withdraw"
	// FlowInstanceStatusTermination 终止。
	FlowInstanceStatusTermination = "termination"
	// FlowInstanceStatusAbandon 废弃。
	FlowInstanceStatusAbandon = "abandon"
	// FlowInstanceStatusRejected 驳回。
	FlowInstanceStatusRejected = "rejected"
	// FlowInstanceStatusEnd 完结。
	FlowInstanceStatusEnd = "end"
)

// flowInstanceStatusLabels 是目标状态取值到目标自己的中文标签的映射。
var flowInstanceStatusLabels = map[string]string{
	FlowInstanceStatusAwaitSent:   "待发",
	FlowInstanceStatusDraft:       "草稿",
	FlowInstanceStatusRun:         "运行中",
	FlowInstanceStatusWithdraw:    "撤销",
	FlowInstanceStatusTermination: "终止",
	FlowInstanceStatusAbandon:     "废弃",
	FlowInstanceStatusRejected:    "驳回",
	FlowInstanceStatusEnd:         "完结",
}

// FlowInstanceStatusOptions 按固定顺序返回目标真实状态取值与中文标签，供界面下拉与校验共用同一份来源。
func FlowInstanceStatusOptions() []SuccessAssertionStatusOption {
	order := []string{
		FlowInstanceStatusAwaitSent, FlowInstanceStatusDraft, FlowInstanceStatusRun,
		FlowInstanceStatusWithdraw, FlowInstanceStatusTermination, FlowInstanceStatusAbandon,
		FlowInstanceStatusRejected, FlowInstanceStatusEnd,
	}
	options := make([]SuccessAssertionStatusOption, 0, len(order))
	for _, value := range order {
		options = append(options, SuccessAssertionStatusOption{Value: value, Label: flowInstanceStatusLabels[value]})
	}
	return options
}

// IsFlowInstanceStatus 判断取值是否属于目标真实状态集合；集合外一律拒绝，不做兜底放行。
func IsFlowInstanceStatus(value string) bool {
	_, ok := flowInstanceStatusLabels[value]
	return ok
}

// FlowInstanceStatusLabel 返回目标状态的中文标签；集合外返回空串，由调用方决定如何提示。
func FlowInstanceStatusLabel(value string) string {
	return flowInstanceStatusLabels[value]
}

// SuccessAssertionStatusOption 是界面可选的期望状态项。
type SuccessAssertionStatusOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// PathSuccessAssertion 是一条执行路径的成功断言：跑到哪个结束节点、实例状态是什么、第几次到达。
// 一条路径只有一个断言；判定发生在运行时的事实重读之后，本类型只承载配置。
type PathSuccessAssertion struct {
	PathID uint64 `json:"-"`
	// EndNodeKey 是工具侧语义节点键，只能取自该路径真实线路上的结束节点。
	EndNodeKey string `json:"endNodeKey"`
	// EndNodeName 只用于目标不可读时的界面回显，不参与判定。
	EndNodeName string `json:"endNodeName"`
	// ExpectedStatus 是目标平台真实实例状态取值。
	ExpectedStatus string `json:"expectedStatus"`
	// ExpectedStatusLabel 是该状态在目标平台的中文标签，随响应下发避免前端自建映射。
	ExpectedStatusLabel string `json:"expectedStatusLabel"`
	// ArrivalOrdinal 是第几次到达该结束节点；只出现一次时固定为 1。
	ArrivalOrdinal uint `json:"arrivalOrdinal"`
	// Revision 是断言自己的修订号，与路径配置修订互不影响。
	Revision  uint64    `json:"revision"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// SuccessAssertionEndNodeCandidate 是断言可选的结束节点，全部由真实线路与真实流程结构推导。
type SuccessAssertionEndNodeCandidate struct {
	NodeKey string `json:"nodeKey"`
	Name    string `json:"name"`
	// ArrivalCount 是该结束节点在本路径上的出现次数；大于 1 时必须指定第几次到达。
	ArrivalCount uint `json:"arrivalCount"`
}

// SuccessAssertionWorkspace 是断言卡片一次读取所需的全部内容：候选、可选状态与已保存断言。
type SuccessAssertionWorkspace struct {
	EndNodeCandidates []SuccessAssertionEndNodeCandidate `json:"endNodeCandidates"`
	StatusOptions     []SuccessAssertionStatusOption     `json:"statusOptions"`
	// Assertion 为空表示这条路径还没有配置成功断言。
	Assertion *PathSuccessAssertion `json:"assertion,omitempty"`
	// Issues 是只读复验发现的问题，例如已保存的结束节点不再在路径上；不自动修正。
	Issues []PathConfigAffectedItem `json:"issues"`
}
