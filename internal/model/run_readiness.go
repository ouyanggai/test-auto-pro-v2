package model

// 运行准备的阻塞与提醒来源，取值固定。界面按 Kind 分组显示，前端不自造来源。
const (
	// RunReadinessNodeConfiguration 节点人员与动作配置未完成。
	RunReadinessNodeConfiguration = "node_configuration"
	// RunReadinessFormData 基础表单数据未就绪。
	RunReadinessFormData = "form_data"
	// RunReadinessConfigIssue 路径配置里已记录的问题，原样透出不改写文案。
	RunReadinessConfigIssue = "config_issue"
	// RunReadinessConfigUnreadable 表示路径配置读取失败。读不到配置不等于没有配置，
	// 必须阻塞而不是放行，否则数据库故障会让一条其实无法运行的路径被判成可以运行。
	RunReadinessConfigUnreadable = "config_unreadable"
	// RunReadinessCompiledScenarioEmpty 编译场景为空，没有可执行步骤。
	RunReadinessCompiledScenarioEmpty = "compiled_scenario_empty"
	// RunReadinessActionNotVerified 路径包含尚未由真实写验证过的动作。
	RunReadinessActionNotVerified = "action_not_verified"
	// RunReadinessPersonNotUnique 人员策略解析不出唯一真实处理人。
	RunReadinessPersonNotUnique = "person_not_unique"
	// RunReadinessTopologyChanged 路径与当前真实流程结构已不一致。
	RunReadinessTopologyChanged = "topology_changed"
	// RunReadinessSemanticsPending 本路径动作涉及的目标语义条目仍待实测。
	RunReadinessSemanticsPending = "semantics_pending"
	// RunReadinessDeploymentNotice 目标部署差异，只提醒不阻塞。
	RunReadinessDeploymentNotice = "deployment_notice"
	// RunReadinessPlanNotice 计划已固化的只读提示，只提醒不阻塞。
	RunReadinessPlanNotice = "plan_notice"
)

// RunReadinessItem 是一条阻塞或提醒。Anchor 让界面能点进具体路径的具体面板。
type RunReadinessItem struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
	// Anchor 是目标面板锚点，例如节点配置面板或成功断言卡片；空串表示只能定位到路径本身。
	Anchor string `json:"anchor"`
}

// PathRunReadiness 是一条执行路径的运行准备结论。
// Runnable 为真表示当前没有阻塞；提醒不影响 Runnable，界面必须与阻塞分区显示。
type PathRunReadiness struct {
	PathID     uint64 `json:"pathId"`
	PathName   string `json:"pathName"`
	SequenceNo uint   `json:"sequenceNo"`
	Runnable   bool   `json:"runnable"`
	// Summary 是这条路径一句中文结论。
	Summary   string             `json:"summary"`
	Blocks    []RunReadinessItem `json:"blocks"`
	Reminders []RunReadinessItem `json:"reminders"`
}

// PlanRunReadiness 是一个计划的运行准备结论：一句总结论加逐条路径明细。
type PlanRunReadiness struct {
	Summary       string             `json:"summary"`
	TotalCount    int                `json:"totalCount"`
	RunnableCount int                `json:"runnableCount"`
	BlockedCount  int                `json:"blockedCount"`
	Paths         []PathRunReadiness `json:"paths"`
}
