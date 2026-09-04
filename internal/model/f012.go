package model

import "time"

const (
	// HistoryDataStatusEmpty 表示路径尚未选择可用的历史来源。
	HistoryDataStatusEmpty = "empty"
	// HistoryDataStatusNeedsInput 表示数据或路径证据不足，需要用户补充或确认。
	HistoryDataStatusNeedsInput = "needs_input"
	// HistoryDataStatusReady 表示 runtime 校验和当前路径复验均已通过。
	HistoryDataStatusReady = "ready"
	// HistoryDataStatusAffected 表示来源、路径或目标运行时变化后需要重新核对。
	HistoryDataStatusAffected = "affected"
)

const (
	// HistorySourceModeNone 表示路径没有历史数据来源。
	HistorySourceModeNone = "none"
	// HistorySourceModeDefault 表示路径继承计划默认历史来源。
	HistorySourceModeDefault = "default"
	// HistorySourceModeOverride 表示路径使用独立历史来源。
	HistorySourceModeOverride = "override"
)

const (
	// HistoryReplayStatusQueued 表示回放任务已建立但尚未领取明细。
	HistoryReplayStatusQueued = "queued"
	// HistoryReplayStatusRunning 表示回放任务正在处理明细。
	HistoryReplayStatusRunning = "running"
	// HistoryReplayStatusCompleted 表示任务所有明细均已得到终态。
	HistoryReplayStatusCompleted = "completed"
	// HistoryReplayStatusCancelled 表示任务被取消并保留未完成检查点。
	HistoryReplayStatusCancelled = "cancelled"
	// HistoryReplayStatusFailed 表示任务无法继续但明细结果仍被保留。
	HistoryReplayStatusFailed = "failed"
)

const (
	// HistoryReplayItemStatusPending 表示路径尚未开始处理。
	HistoryReplayItemStatusPending = "pending"
	// HistoryReplayItemStatusRunning 表示路径已领取且正在处理。
	HistoryReplayItemStatusRunning = "running"
	// HistoryReplayItemStatusReady 表示路径数据回放已就绪。
	HistoryReplayItemStatusReady = "ready"
	// HistoryReplayItemStatusNeedsInput 表示路径无法被确定性回放。
	HistoryReplayItemStatusNeedsInput = "needs_input"
	// HistoryReplayItemStatusAffected 表示路径修订变化，需要重新读取。
	HistoryReplayItemStatusAffected = "affected"
	// HistoryReplayItemStatusFailed 表示该路径读取或 runtime 校验失败。
	HistoryReplayItemStatusFailed = "failed"
)

// HistoryCandidate 是目标历史实例的安全摘要，浏览器只持有不透明候选键。
type HistoryCandidate struct {
	CandidateKey      string `json:"candidateKey"`
	FlowCode          string `json:"flowCode"`
	FormName          string `json:"formName"`
	FlowName          string `json:"flowName"`
	RuntimeType       string `json:"runtimeType"`
	InstanceTitle     string `json:"instanceTitle"`
	BusinessSummary   string `json:"businessSummary"`
	Initiator         string `json:"initiator"`
	CompanyName       string `json:"companyName"`
	CreatedAt         string `json:"createdAt"`
	Status            string `json:"status"`
	StatusName        string `json:"statusName"`
	Completeness      string `json:"completeness"`
	IntegrityNotice   string `json:"integrityNotice"`
	SnapshotAvailable bool   `json:"snapshotAvailable"`
}

// HistoryCandidatePage 是历史候选的有界分页响应，不包含完整表单正文。
type HistoryCandidatePage struct {
	Items         []HistoryCandidate `json:"items"`
	Page          int                `json:"page"`
	PageSize      int                `json:"pageSize"`
	Total         int                `json:"total"`
	HasMore       bool               `json:"hasMore"`
	DefaultSource *HistoryDataSource `json:"defaultSource,omitempty"`
	PathSource    *HistoryDataSource `json:"pathSource,omitempty"`
}

// HistorySnapshotSummary 保存候选来源的业务摘要，避免向浏览器透传目标内部标识。
type HistorySnapshotSummary struct {
	CandidateKey    string `json:"candidateKey"`
	FlowCode        string `json:"flowCode"`
	FormName        string `json:"formName"`
	FlowName        string `json:"flowName"`
	InstanceTitle   string `json:"instanceTitle"`
	BusinessSummary string `json:"businessSummary"`
	Initiator       string `json:"initiator"`
	CompanyName     string `json:"companyName"`
	CreatedAt       string `json:"createdAt"`
	Status          string `json:"status"`
	StatusName      string `json:"statusName"`
	RuntimeType     string `json:"runtimeType"`
}

// HistorySnapshot 是工具侧不可变的目标原始表单数据副本。
type HistorySnapshot struct {
	ID              uint64         `json:"id"`
	PlanID          uint64         `json:"-"`
	SourceAccount   string         `json:"-"`
	CandidateKey    string         `json:"candidateKey"`
	FlowCode        string         `json:"flowCode"`
	FormName        string         `json:"formName"`
	FlowName        string         `json:"flowName"`
	RuntimeType     string         `json:"runtimeType"`
	InstanceStatus  string         `json:"instanceStatus"`
	InstanceSummary map[string]any `json:"instanceSummary"`
	TemplateSummary map[string]any `json:"templateSummary"`
	RawFormData     map[string]any `json:"rawFormData"`
	SourceDigest    string         `json:"sourceDigest"`
	CreatedAt       time.Time      `json:"createdAt"`
}

// HistoryDataSource 是路径历史来源的公开投影，正文只在服务端或 runtime 会话内流转。
type HistoryDataSource struct {
	Mode       string                  `json:"mode"`
	SnapshotID uint64                  `json:"-"`
	Summary    *HistorySnapshotSummary `json:"summary,omitempty"`
	DataStatus string                  `json:"dataStatus"`
	Issues     []HistoryDataIssue      `json:"issues"`
	Revision   uint64                  `json:"revision"`
}

// HistoryDataIssue 是历史数据差异、必填缺口或运行时拒绝的结构化说明。
type HistoryDataIssue struct {
	Code     string   `json:"code"`
	Path     string   `json:"path,omitempty"`
	Fields   []string `json:"fields,omitempty"`
	Message  string   `json:"message"`
	Blocking bool     `json:"blocking"`
}

// HistoryDefaultSaveInput 是计划默认历史来源的最小回写体。
type HistoryDefaultSaveInput struct {
	CandidateKey string `json:"candidateKey"`
	Revision     uint64 `json:"revision"`
}

// HistoryPathSourceInput 设置路径继承计划默认值或覆盖为独立候选。
type HistoryPathSourceInput struct {
	Mode         string `json:"mode"`
	CandidateKey string `json:"candidateKey,omitempty"`
	Revision     uint64 `json:"revision"`
}

// HistoryReplayJob 是多路径回放任务的真实聚合状态。
type HistoryReplayJob struct {
	ID     string `json:"id"`
	PlanID uint64 `json:"-"`
	// IdempotencyKey 只在服务端和仓储事务内使用，浏览器不能把它当作任务身份。
	IdempotencyKey string     `json:"-"`
	Status         string     `json:"status"`
	Total          int        `json:"total"`
	Pending        int        `json:"pending"`
	Running        int        `json:"running"`
	Ready          int        `json:"ready"`
	NeedsInput     int        `json:"needsInput"`
	Affected       int        `json:"affected"`
	Failed         int        `json:"failed"`
	Cancelled      int        `json:"cancelled"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	CompletedAt    *time.Time `json:"completedAt,omitempty"`
	// LeaseOwner/LeaseExpiresAt/FencingToken 只用于后台租约与过期 worker 隔离。
	LeaseOwner     string     `json:"-"`
	LeaseExpiresAt *time.Time `json:"-"`
	FencingToken   uint64     `json:"-"`
}

// HistoryReplayItem 是任务内单路径检查点和回放结果。
type HistoryReplayItem struct {
	ID                uint64               `json:"id"`
	JobID             string               `json:"-"`
	PathID            uint64               `json:"pathId"`
	PathRevision      uint64               `json:"pathRevision"`
	SnapshotID        *uint64              `json:"snapshotId,omitempty"`
	Status            string               `json:"status"`
	DataStatus        string               `json:"dataStatus"`
	Issues            []HistoryDataIssue   `json:"issues"`
	BranchPatches     []HistoryBranchPatch `json:"branchPatches"`
	EffectiveFormData map[string]any       `json:"effectiveFormData,omitempty"`
	UpdatedAt         time.Time            `json:"updatedAt"`
	CompletedAt       *time.Time           `json:"completedAt,omitempty"`
	// LeaseOwner/LeaseExpiresAt 不向页面暴露，仅由仓储核对完成写入者仍持有租约。
	LeaseOwner     string     `json:"-"`
	LeaseExpiresAt *time.Time `json:"-"`
	FencingToken   uint64     `json:"-"`
	// RuntimeType/RuntimeValidation 只在回放完成事务内更新当前路径配置，不能由浏览器提交或伪造。
	RuntimeType       string                   `json:"-"`
	RuntimeValidation HistoryRuntimeValidation `json:"-"`
}

// HistoryReplayItemPage 是明细游标分页结果。
type HistoryReplayItemPage struct {
	Items      []HistoryReplayItem `json:"items"`
	NextCursor uint64              `json:"nextCursor,omitempty"`
}

// HistoryReplayCreateInput 指定本次明确勾选的路径，不接受目标内部 ID。
type HistoryReplayCreateInput struct {
	PathIDs  []uint64 `json:"pathIds"`
	Revision uint64   `json:"revision"`
}

// HistoryBranchPatch 记录系统对条件驱动字段所做的最小修改。
type HistoryBranchPatch struct {
	Path      string `json:"path"`
	Before    any    `json:"before"`
	After     any    `json:"after"`
	Reason    string `json:"reason"`
	BranchKey string `json:"branchKey"`
}

// HistoryRuntimeValidation 是复制的 form-runtime 返回的结构化校验摘要。
type HistoryRuntimeValidation struct {
	Accepted bool               `json:"accepted"`
	Issues   []HistoryDataIssue `json:"issues"`
}

// PathConfigurationF012 是 F-012 配置读取接口的统一领域视图。
// HistoryKeyField 是决定当前执行路径的条件字段投影：只包含目标条件声明的真实字段路径、
// 现值、目标真实候选值和它影响的分支，不包含目标内部标识。
type HistoryKeyField struct {
	Path string `json:"path"`
	// Label 是目标表单字段的中文名称，缺失时前端回退显示字段路径。
	Label      string   `json:"label,omitempty"`
	HasCurrent bool     `json:"hasCurrent"`
	Current    any      `json:"current,omitempty"`
	Candidates []any    `json:"candidates,omitempty"`
	Operators  []string `json:"operators,omitempty"`
	Branches   []string `json:"branches,omitempty"`
	Decisive   bool     `json:"decisive"`
}

type PathConfigurationF012 struct {
	Path           PathConfigPath    `json:"path"`
	Revision       uint64            `json:"revision"`
	NodeRevision   uint64            `json:"nodeRevision"`
	DataRevision   uint64            `json:"dataRevision"`
	ActionRevision uint64            `json:"actionRevision"`
	NodeStatus     string            `json:"nodeStatus"`
	DataStatus     string            `json:"dataStatus"`
	HistorySource  HistoryDataSource `json:"historySource"`
	RuntimeType    string            `json:"runtimeType"`
	// RuntimeTemplate/RuntimePage/RuntimePermissions/RuntimeReadRequests 直接对应复制 form-runtime 的既有加载协议，不承载工具侧字段映射。
	RuntimeTemplate     map[string]any           `json:"template"`
	RuntimePage         *PathVueCustomPageRule   `json:"vuePage,omitempty"`
	RuntimePermissions  []PathFormPermission     `json:"permissions"`
	RuntimeReadRequests []PathFormReadRequest    `json:"readRequests"`
	EffectiveFormData   map[string]any           `json:"effectiveFormData,omitempty"`
	BranchPatches       []HistoryBranchPatch     `json:"branchPatches"`
	RuntimeValidation   HistoryRuntimeValidation `json:"runtimeValidation"`
	Issues              []HistoryDataIssue       `json:"issues"`
	// KeyFields 是决定当前执行路径的条件字段，供界面提示用户优先核对哪些字段。
	KeyFields        []HistoryKeyField    `json:"keyFields"`
	Actions          []ConfiguredAction   `json:"actions"`
	CompiledScenario []CompiledActionStep `json:"compiledScenario"`
}

// PathConfigurationDataInput 是复制 form-runtime 捕获的原始目标表单数据保存请求。
// Values 必须保持目标接口原始 JSON 结构；服务端只根据流程条件重算路径，不接受生成元数据或计算结果。
type PathConfigurationDataInput struct {
	Revision          uint64                   `json:"revision"`
	Values            map[string]any           `json:"values"`
	RuntimeValidation HistoryRuntimeValidation `json:"runtimeValidation"`
	ConfirmationToken string                   `json:"confirmationToken,omitempty"`
}

// PathConfigurationRouteChange 描述保存前后实际路径变化及目标路径覆盖影响。
type PathConfigurationRouteChange struct {
	From           PathConfigPath           `json:"from"`
	To             PathConfigPath           `json:"to"`
	OverwritesData bool                     `json:"overwritesData"`
	Affected       []PathConfigAffectedItem `json:"affected"`
	Warning        string                   `json:"warning"`
}

// PathConfigurationDataResult 是原始数据保存后的权威结果；需要确认时只返回令牌和影响摘要，不写入目标路径。
type PathConfigurationDataResult struct {
	Path                 PathConfigPath                `json:"path"`
	Revision             uint64                        `json:"revision"`
	DataRevision         uint64                        `json:"dataRevision"`
	DataStatus           string                        `json:"dataStatus"`
	RuntimeType          string                        `json:"runtimeType"`
	RuntimeTemplate      map[string]any                `json:"template"`
	RuntimePage          *PathVueCustomPageRule        `json:"vuePage,omitempty"`
	RuntimePermissions   []PathFormPermission          `json:"permissions"`
	RuntimeReadRequests  []PathFormReadRequest         `json:"readRequests"`
	EffectiveFormData    map[string]any                `json:"effectiveFormData,omitempty"`
	BranchPatches        []HistoryBranchPatch          `json:"branchPatches"`
	RuntimeValidation    HistoryRuntimeValidation      `json:"runtimeValidation"`
	Issues               []HistoryDataIssue            `json:"issues"`
	RouteChanged         bool                          `json:"routeChanged"`
	RequiresConfirmation bool                          `json:"requiresConfirmation"`
	ConfirmationToken    string                        `json:"confirmationToken,omitempty"`
	RouteChange          *PathConfigurationRouteChange `json:"routeChange,omitempty"`
}
