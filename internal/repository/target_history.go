package repository

import "context"

// TargetHistoryCandidateRow 是目标业务库中的候选关键列：只够列表展示与身份匹配，
// 不含任何表单正文；表单数据仍在用户选定候选后按目标只读协议单独读取。
type TargetHistoryCandidateRow struct {
	InstanceID    string
	Name          string
	FlowName      string
	FormName      string
	FlowCode      string
	FlowProxyID   string
	FormProxyID   string
	Status        string
	InitiatorName string
	CompanyName   string
	CreatedAt     string
}

// TargetHistoryCandidateFilter 是候选查询条件：只使用目标业务库原字段，不拼装新的身份键。
type TargetHistoryCandidateFilter struct {
	// FlowName 是目标流程详情返回的流程名称，用于锁定同一流程的实例。
	FlowName string
	// FlowCode 是目标流程模板编码；实例行没有写入编码时不据此排除，写入了不同编码时精确排除。
	FlowCode string
	// FormName 为空表示无表单流程；非空时只接受同名表单或没有表单代理的实例行。
	FormName string
	// Query 是用户输入的搜索词，按实例名称、发起人和公司名称模糊匹配。
	Query string
	// ExcludeNameKeywords 用于剔除历史自动化产生的实例名称，默认排除包含"自动"的旧回归数据。
	ExcludeNameKeywords []string
}

// TargetHistoryCandidateStore 是候选列表的只读快速来源，实现方只允许执行 SELECT。
type TargetHistoryCandidateStore interface {
	TargetHistoryCandidates(ctx context.Context, filter TargetHistoryCandidateFilter, page, pageSize int) ([]TargetHistoryCandidateRow, int, error)
}
