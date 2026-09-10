// Package step 实现一步的七阶段生命周期（纲领第 4.3 节）：
// plan 取步、gate 门禁复验、control 控制判定、prepare 演员与会话、submit 发写请求、
// verify 事实重读、settle 落账。本包不做多路径调度（属 schedule），
// 也不直接发目标请求——目标写只能由 internal/adapter/target 发出，本包给出语义意图。
package step

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/model"
)

// TargetClient 是执行器需要的最小目标能力面。读写都经 internal/adapter/target，
// 写端点只能来自动作白名单；新增端点必须同步更新对应白名单契约。
type TargetClient interface {
	FindSubmittedFlow(ctx context.Context, active target.Session, instanceID string) (string, []string, string, []string, bool, error)
	FindDueFlow(ctx context.Context, active target.Session, instanceID string) (string, []string, []string, bool, error)
	FindDueTaskID(ctx context.Context, active target.Session, instanceID, nodeProxyID string) (string, error)
	// ReadInstanceCurrentData 读取实例当前的完整表单数据（只读，可重试）。
	// 目标保存表单数据是整份覆盖，写请求必须以这份数据为基线（语义清单第 16 条）。
	ReadInstanceCurrentData(ctx context.Context, active target.Session, instanceID string) (map[string]any, error)
	SubmitFlowInstance(ctx context.Context, session target.Session, request target.SubmitFlowInstanceRequest) (*target.SubmitFlowInstanceResult, target.WriteResponse, string, error)
	AuditCurrentTask(ctx context.Context, session target.Session, request target.AuditCurrentTaskRequest) (*target.AuditCurrentTaskResult, target.WriteResponse, string, error)
	// ExecuteActionWrite 是 F-019 全动作写出口：按动作分派端点与载荷。
	ExecuteActionWrite(ctx context.Context, session target.Session, request target.ActionWriteRequest) (target.WriteResponse, string, error)
}

// SessionProvider 取得指定账号的目标会话。登录与会话获取属只读阶段（纲领第 4.4.1 节），可安全重试。
type SessionProvider interface {
	Current(ctx context.Context, account string) (target.Session, error)
	// Refresh 作废缓存并强制重新登录；写步骤的 prepare 阶段必须用最新会话，
	// 因为 submit 一旦发出就没有任何重试或重登的机会。
	Refresh(ctx context.Context, account string) (target.Session, error)
}

// RunStateControl 是执行器对路径运行状态机的推进面（internal/engine/run.Service 满足）。
type RunStateControl interface {
	SetMainInstanceRef(ctx context.Context, pathRunID uint64, instanceRef string) error
	ClaimExecution(ctx context.Context, pathRunID uint64) (uint64, error)
	RenewLease(ctx context.Context, pathRunID uint64, fencingToken uint64) error
	ReleaseExecution(ctx context.Context, pathRunID uint64, fencingToken uint64) error
	MarkVerifying(ctx context.Context, pathRunID uint64) error
	BackToRunning(ctx context.Context, pathRunID uint64) error
	Finish(ctx context.Context, pathRunID uint64, to model.PathRunStatus, result *model.RunResult, failureClass *model.FailureClass, label string) (model.PathRun, error)
}

// RunFactsStore 是运行事实落账面：步骤与尝试事实只 INSERT。
type RunFactsStore interface {
	RecordStepAttempt(ctx context.Context, step model.RunStep, attempt model.RunStepAttempt, now time.Time) (uint64, error)
}

// NodeInfo 是执行器查表用的节点信息：目标节点名称与类型名（如「审批」，可被门禁归一化识别）。
type NodeInfo struct {
	Name string
	Type string
	// TargetNodeID 是这个节点在目标平台的真实节点标识。
	// 编译场景与配置快照用的 nodeKey 是工具侧的不透明键（哈希派生），不能直接发给目标：
	// 待办新鲜读取、按节点的写参数都必须用这个真实标识，否则永远匹配不上目标数据。
	TargetNodeID string
	// EditableFields 是目标在这个节点声明为可编辑（fieldPower=edit）的表单字段英文名。
	// 它是本节点写载荷唯一允许覆盖的字段集合（语义清单第 11 条）。
	EditableFields []string
	// AuditType 是目标在该节点配置的审批方式（run_node_choose/company/level 等）。
	// 提交载荷按它决定是否必须携带 nextAuditorList 人员指定项（语义清单 1.8 补充）。
	AuditType string
}

// ActionPersonIndex 返回运行上下文中动作人员解析结果的稳定索引。
func ActionPersonIndex(nodeKey string, action model.ActionKey) string {
	return strings.TrimSpace(nodeKey) + "\x00" + string(action)
}

// RunContext 是一次路径运行的静态上下文：执行期间不变的标识、场景与数据。
type RunContext struct {
	Run     model.Run
	PathRun model.PathRun
	// PlanName 与 PathName 是日志目录里使用的计划与执行路径显示名。
	PlanName string
	PathName string
	// PlanAccount 是计划账号：目标登录账号，同时是「新发起」流程的发起人。
	// 本切片的演员候选就是该账号；演员最终成立还必须通过目标待办/发起事实核验，绝不静默替换处理人。
	PlanAccount string
	// FlowProxyID 是发布流程代理 ID（计划指向的目标对象标识），发起请求的必填标识之一。
	FlowProxyID string
	// Source 是这条路径的流程来源（如“新发起”）；门禁投影与来源相关。
	Source string
	// Nodes 是路径配置快照里这条路径的目标节点表（键=目标代理节点 Key），
	// 供节点名称与节点类型查表。编译场景的 nodeKey 用的是这一套键，不是流程图节点 ID。
	Nodes map[string]NodeInfo
	// BranchSelections 是这条路径已保存的分支选择（分支节点 ID -> 所选分支的目标节点 ID）。
	// 手动条件分支在提交/审批时必须显式携带所选节点，否则目标以“手动条件分支,请选择”拒绝。
	BranchSelections map[string]string
	// SubmitBranchTargetNodeID 是路线第一个手动分支所选分支的目标节点 ID，
	// 随提交请求以 nextAuditorList[].nodeProxyId 传递（FlowOperateServiceImpl 按 nodeProxyId 匹配候选分支）；
	// 协议顶层的 fixedExecuteNodeId 是并行条件分支的另一机制，与此无关（语义清单第 15 条）。
	SubmitBranchTargetNodeID string
	// Steps 是编译场景（用户步骤），执行器按序号推进。
	Steps []model.CompiledActionStep
	// LastBeforeFacts 是最近一步写之前保存的目标事实基准（对账对照用）。
	// 执行时由控制现场填入，重启后由 run_step_attempts.before_facts 还原。
	LastBeforeFacts InstanceFacts
	// LastBeforeFactsKnown 表示上面那份基准是不是真的取到了。
	// 零值不等于"写之前实例不存在"：把丢失的基准当成确定事实用，会让对账把审批类写的
	// 实例状态维度说成"写之前实例不存在、现在存在"，凭空造出一条与事实相反的依据。
	LastBeforeFactsKnown bool
	// EffectiveFormData 是路径生效表单数据的原始 JSON 文本。
	// 以 json.Number 解码后再序列化，数字字面量保持原样（原生 JSON 列会改写数字的教训见迁移 023）。
	EffectiveFormData []byte
	// NodeEditableFields 是路线上每个节点声明的可编辑字段（键=编译场景的 nodeKey）。
	// 与 Nodes 里的同名字段同源，单独留一份是为了在没有节点信息时也能判断"这个字段属于哪个节点"。
	NodeEditableFields map[string][]string
	// ActionPersonIDs 是启动时按当前目标目录解析的动作人员 ID；只允许执行器内部使用，
	// 不向浏览器或持久化场景透传目标业务标识。
	ActionPersonIDs map[string][]string
	// NextNodeAuditors 是启动时为 run_node_choose 下一节点解析的真实处理人。
	// 仅在内存执行上下文中保存，提交、重新提交和同意都使用同一份结果构造目标 nextAuditorList。
	NextNodeAuditors map[string][]target.NextAuditor
	// FlowProxyRemapped 表示本次运行已经通过加签响应确认实例代理被重建；
	// 只有此时任务读取才允许在旧节点 ID 失配后按实例唯一任务恢复。
	FlowProxyRemapped bool
}

// InstanceFacts 是一次目标事实读取的快照，用于门禁复验与事实重读对照。
type InstanceFacts struct {
	ReadError string `json:"readError,omitempty"`
	Found     bool   `json:"found"`
	Status    string `json:"status,omitempty"`
	// CreatorRead 表示本次实例读取已经核对创建人；没有该事实时不能把当前账号冒充创建人。
	CreatorRead bool `json:"creatorRead,omitempty"`
	// IsInitiator 表示当前会话账号是否为目标实例创建人。
	IsInitiator bool `json:"isInitiator,omitempty"`
	// FlowProxyID/FormProxyID 是目标实例当前返回的代理标识；重新提交必须使用这份实时事实，
	// 不能把计划里的流程代理或历史表单代理当成当前实例标识。
	FlowProxyID string `json:"flowProxyId,omitempty"`
	FormProxyID string `json:"formProxyId,omitempty"`
	// BizRelevance 是目标实例当前业务关联；重提和转发必须沿用它，避免辅助流程丢失业务归属。
	BizRelevance []target.BizRelevance `json:"bizRelevance,omitempty"`
	CurrentNodes []string              `json:"currentNodes,omitempty"`
	DueNodes     []string              `json:"dueNodes,omitempty"`
	// StepNodeKey 记录本次关心的是哪个节点上的待办（审批步骤对照用）。
	StepNodeKey string `json:"stepNodeKey,omitempty"`
	// StorageRead/StorageFound 是暂存检查点读取事实；检查点标识、说明和更新时间用于判断本次是否新增或更新。
	StorageRead       bool   `json:"storageRead,omitempty"`
	StorageFound      bool   `json:"storageFound,omitempty"`
	StorageDataID     string `json:"storageDataId,omitempty"`
	StorageAuditDesc  string `json:"storageAuditDesc,omitempty"`
	StorageUpdateDate string `json:"storageUpdateDate,omitempty"`
	// UrgeRecordsRead/UrgeRecordCount 是催办记录读取事实。
	UrgeRecordsRead bool `json:"urgeRecordsRead,omitempty"`
	UrgeRecordCount int  `json:"urgeRecordCount,omitempty"`
	// TrackingRead/Tracking 是当前登录用户的关注状态读取事实。
	TrackingRead bool `json:"trackingRead,omitempty"`
	Tracking     bool `json:"tracking,omitempty"`
	// ActionFactRead 表示本动作的专用结果接口已经成功读取，不能用实例节点未变化代替它。
	ActionFactRead bool `json:"actionFactRead,omitempty"`
	// CurrentTaskRead/CurrentTaskFound 是任务级动作的当前待办快照；门禁不得仅凭实例节点推断待办归属。
	CurrentTaskRead      bool   `json:"currentTaskRead,omitempty"`
	CurrentTaskFound     bool   `json:"currentTaskFound,omitempty"`
	CurrentTaskLinkID    string `json:"currentTaskLinkId,omitempty"`
	CurrentTaskParentID  string `json:"currentTaskParentId,omitempty"`
	CurrentTaskBatchNo   string `json:"currentTaskBatchNo,omitempty"`
	CurrentTaskFlowProxy string `json:"currentTaskFlowProxy,omitempty"`
	CurrentTaskNodeID    string `json:"currentTaskNodeId,omitempty"`
	// CurrentTaskAssigneeID/Name 是目标裁决的当前待办实际处理人（currentPendingUserId/Name）。
	// 任务级动作必须以该处理人身份登录并发出写请求；这是"人员配置驱动执行"的事实来源。
	CurrentTaskAssigneeID   string `json:"currentTaskAssigneeId,omitempty"`
	CurrentTaskAssigneeName string `json:"currentTaskAssigneeName,omitempty"`
	// AssigneeDiag 是当前处理人发现（已发事实提取+会话切换）没有命中时的中文原因，
	// 随门禁拒绝一并写入 step.log 与界面，让「当前待办已经处理」这类结论可解释。
	AssigneeDiag string `json:"assigneeDiag,omitempty"`
	// CurrentHandlers 是「已发」列表返回的各当前节点待办处理人信息（节点 ID → 审批方式
	// 与待处理人员）。会签节点在全部处理人审批完成前一直出现在这里，人员随审批进度递减；
	// 是发现「指定人员」类节点真实处理人的唯一稳定只读来源（待办列表按当前用户过滤）。
	CurrentHandlers []target.NodeCurrentHandler `json:"currentHandlers,omitempty"`
	// CompletedTask* 是取回动作的当前账号已办任务快照。
	CompletedTaskRead     bool   `json:"completedTaskRead,omitempty"`
	CompletedTaskFound    bool   `json:"completedTaskFound,omitempty"`
	CompletedTaskLinkID   string `json:"completedTaskLinkId,omitempty"`
	CompletedTaskNodeID   string `json:"completedTaskNodeId,omitempty"`
	CompletedTaskParentID string `json:"completedTaskParentId,omitempty"`
	CompletedTaskBatchNo  string `json:"completedTaskBatchNo,omitempty"`
	CompletedTaskAuditWay string `json:"completedTaskAuditWay,omitempty"`
	// 下面字段是动作门禁从目标任务链、代理树和审核记录得到的结论。
	EditableProxyRead   bool   `json:"editableProxyRead,omitempty"`
	ActorSwitchRead     bool   `json:"actorSwitchRead,omitempty"`
	PreviousTaskRead    bool   `json:"previousTaskRead,omitempty"`
	PreviousTaskExists  bool   `json:"previousTaskExists,omitempty"`
	PreviousNodeType    string `json:"previousNodeType,omitempty"`
	PreviousNodeIsStart bool   `json:"previousNodeIsStart,omitempty"`
	// SuccessorStateKnown 为 false 时，目标任务列表没有可用的全局后继视图，取回接口必须自行确认后继状态。
	SuccessorStateKnown       bool `json:"successorStateKnown,omitempty"`
	NextTaskProcessed         bool `json:"nextTaskProcessed,omitempty"`
	RetrieveAlreadyUsed       bool `json:"retrieveAlreadyUsed,omitempty"`
	RetrieveNodeIsStart       bool `json:"retrieveNodeIsStart,omitempty"`
	CurrentTaskHandledByOther bool `json:"currentTaskHandledByOther,omitempty"`
	CurrentTaskCountersign    bool `json:"currentTaskCountersign,omitempty"`
	CurrentTaskParallel       bool `json:"currentTaskParallel,omitempty"`
	// PendingTaskRead/PendingTaskFound 是实例级动作（如催办）读取到的全实例待办事实。
	PendingTaskRead  bool `json:"pendingTaskRead,omitempty"`
	PendingTaskFound bool `json:"pendingTaskFound,omitempty"`
}

// EncodeInstanceFacts 把目标事实快照序列化为落库文本（run_step_attempts.before_facts）。
// 只在尝试行插入时写一次：它是"这次写之前目标什么样"的事实，不是可变状态。
// 序列化失败返回空串——宁可让对账按证据缺失降级，也不落一段解不开的文本。
func EncodeInstanceFacts(facts InstanceFacts) string {
	encoded, err := json.Marshal(facts)
	if err != nil {
		return ""
	}
	return string(encoded)
}

// DecodeInstanceFacts 从落库文本还原写之前的目标事实；空串或解析失败返回零值。
// 零值意味着"没有基准"，调用方必须按证据缺失处理（BeforeHadInstance=false 会让对账降级），
// 绝不能把解析失败当成"写之前实例不存在"这个确定事实来用。
func DecodeInstanceFacts(raw string) (InstanceFacts, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return InstanceFacts{}, false
	}
	var facts InstanceFacts
	if err := json.Unmarshal([]byte(trimmed), &facts); err != nil {
		return InstanceFacts{}, false
	}
	return facts, true
}

// StepPreview 是控制阶段停下时给用户的下一步预览。
type StepPreview struct {
	PathRunID  uint64
	StepNo     int
	TotalSteps int
	Action     model.ActionKey
	ActionName string
	NodeKey    string
	NodeName   string
	// TargetNodeID 是本步节点在目标平台的真实标识。NodeKey 是工具侧不透明键，
	// 凡是要与目标返回的节点集合对照（待办、当前节点、对账）都必须用这个字段。
	TargetNodeID string
	// ActorAccount 与 ActorName 是解析出的唯一真实演员；SID 绝不进入本结构或任何展示。
	ActorAccount string
	ActorName    string
	// ExpectedEffect 来自动作目录的预期效果中文说明。
	ExpectedEffect string
	// Endpoint 是即将发出的写请求端点（白名单内）。
	Endpoint string
	// RequestPreview 是即将发出的请求正文摘要 JSON（不含 SID 等会话敏感信息）。
	RequestPreview string
	// GateAllowed 为 false 时禁止放行，GateReason 给出中文原因。
	GateAllowed bool
	GateReason  string
	// GateItems 是门禁复验的逐项中文结论快照。
	GateItems []model.ActionPrecondition
	// Facts 是此刻的目标事实（发起前实例不存在则 Found=false）。
	Facts InstanceFacts
	// BlockReason 非空表示本步无法继续（门禁不通过/演员不可解析等），路径必须停止。
	BlockReason       string
	BlockFailureClass model.FailureClass
	// ReleaseGroup 是编译场景中的动作组标识；人工模式一次放行同一组内的物理步骤。
	ReleaseGroup string
	// ReleaseRequired 表示本步是动作组首个步骤，人工模式在此停下等待放行。
	ReleaseRequired bool
	// Navigation 表示本步是只读导航步骤：不发出写请求，仅校验实例事实。
	Navigation bool
	// TargetSkipped 表示本步节点已被目标自动跳过（模板约束「无处理人时跳过该节点」生效，
	// 实例待办已落到路径上更靠后的节点）：不发出写请求，放行后按「已跳过」落账并推进。
	// SkipReason 是给用户看的中文依据，必须说清为什么判定被跳过。
	TargetSkipped bool
	SkipReason    string
	// RequestPayload 是放行后将要发出的请求载荷（与预览同源），只在内存流转，含会话无关字段。
	RequestPayload map[string]any
	// FormOverlaid 与 FormWithheld 是本步表单数据按节点权限构造的结果：
	// 覆盖了哪些字段、按权限没带哪些字段。进日志与门禁快照，让"少带了什么"可追溯。
	FormOverlaid []string
	FormWithheld []string
	// request 是构造载荷的那份类型化请求本体；发送时直接使用它，保证预览与实际发出严格同源。
	request any

	// 以下字段由发送阶段回填，仅在内存流转，不进入任何公开 DTO：
	writeResult     any
	writeResponse   target.WriteResponse
	writeTraceID    string
	writeErr        error
	writeDurationMs int64
	// writeSent 表示写请求已经真正发出（或已尝试发出）。发送前的待办新鲜复验失败时保持 false：
	// 没有发出的请求不存在“写结果不确定”，绝不能进三值判定的写判定路径。
	writeSent bool
	// writeErrClass 是零写入失败的归属分类（演员/待办解析失败或工具缺陷），供落账时如实归类。
	writeErrClass model.FailureClass
}

// WriteSent 报告本次预览是否已进入目标写请求阶段。
// 控制层据此在写后本地落账失败时封存现场，禁止用户再次放行造成重复业务操作。
func (p *StepPreview) WriteSent() bool {
	return p != nil && p.writeSent
}

// StepVerdictTargetSkipped 是目标自动跳过步骤的尝试结论：不是三值判定对象（没有写请求），
// 单独取值以便事实行与界面如实区分「成功」与「被目标跳过」。
const StepVerdictTargetSkipped = "target_skipped"

// StepOutcome 是一步走完后的结果，供控制层决定路径去向。
type StepOutcome struct {
	Verdict string // confirmed_success / confirmed_failure / uncertain
	// NoMoreSteps 表示编译场景已走完，控制层应执行收尾重读。
	NoMoreSteps bool
	// MainInstanceRef 是本步之后的主实例引用（发起成功时写入）。
	MainInstanceRef string
	// FlowProxyID 是加签更新后目标返回的新实例私有流程代理标识。
	FlowProxyID string
	// CurrentNodeProxyID 是加签更新后目标返回的当前节点代理标识。
	CurrentNodeProxyID string
	// AuxiliaryInstanceRef 是转发动作创建的辅助实例引用，供运行记录与后续状态确认使用。
	AuxiliaryInstanceRef string
	// DeviationDetected 表示核验重读的实际当前节点与已配置路径的下一个预期节点不一致
	//（纲领第 7.4 节：偏离是独立事实，停止在下一步阶段 3 生效）。
	DeviationDetected bool
}
