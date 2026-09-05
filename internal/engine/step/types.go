// Package step 实现一步的七阶段生命周期（纲领第 4.3 节）：
// plan 取步、gate 门禁复验、control 控制判定、prepare 演员与会话、submit 发写请求、
// verify 事实重读、settle 落账。本包不做多路径调度（属 schedule），
// 也不直接发目标请求——目标写只能由 internal/adapter/target 发出，本包给出语义意图。
package step

import (
	"context"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/model"
)

// TargetClient 是执行器需要的最小目标能力面。读写都经 internal/adapter/target，
// 写端点只有白名单内的两个；新增写端点必须同时更新 F-016 白名单契约。
type TargetClient interface {
	FindSubmittedFlow(ctx context.Context, active target.Session, instanceID string) (string, []string, string, []string, bool, error)
	FindDueFlow(ctx context.Context, active target.Session, instanceID string) (string, []string, []string, bool, error)
	FindDueTaskID(ctx context.Context, active target.Session, instanceID, nodeProxyID string) (string, error)
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
	// SubmitBranchTargetNodeID 是路线第一个手动分支所选的目标节点 ID，
	// 提交请求以 fixedExecuteNodeId 顶层字段携带（FlowSubmitServiceImpl 的 custom_choose 校验所需）。
	SubmitBranchTargetNodeID string
	// Steps 是编译场景（用户步骤），执行器按序号推进。
	Steps []model.CompiledActionStep
	// LastBeforeFacts 是最近一步写之前保存的目标事实基准（对账对照用，内存流转）。
	LastBeforeFacts InstanceFacts
	// EffectiveFormData 是路径生效表单数据的原始 JSON 文本。
	// 必须按原始字节透传到写请求，禁止先解码再重新序列化（数字字面量会被改写）。
	EffectiveFormData []byte
}

// InstanceFacts 是一次目标事实读取的快照，用于门禁复验与事实重读对照。
type InstanceFacts struct {
	ReadError    string
	Found        bool
	Status       string
	CurrentNodes []string
	DueNodes     []string
	// StepNodeKey 记录本次关心的是哪个节点上的待办（审批步骤对照用）。
	StepNodeKey string
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
	// Navigation 表示本步是只读导航步骤：不发出写请求，仅校验实例事实。
	Navigation bool
	// RequestPayload 是放行后将要发出的请求载荷（与预览同源），只在内存流转，含会话无关字段。
	RequestPayload map[string]any
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

// StepOutcome 是一步走完后的结果，供控制层决定路径去向。
type StepOutcome struct {
	Verdict string // confirmed_success / confirmed_failure / uncertain
	// NoMoreSteps 表示编译场景已走完，控制层应执行收尾重读。
	NoMoreSteps bool
	// MainInstanceRef 是本步之后的主实例引用（发起成功时写入）。
	MainInstanceRef string
	// DeviationDetected 表示核验重读的实际当前节点与已配置路径的下一个预期节点不一致
	//（纲领第 7.4 节：偏离是独立事实，停止在下一步阶段 3 生效）。
	DeviationDetected bool
}
