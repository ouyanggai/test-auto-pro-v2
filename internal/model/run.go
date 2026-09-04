package model

import (
	"fmt"
	"time"
)

// RunStatus 是一次运行（runs）的公开状态，对应纲领第 4.2 节：
// 等待运行 -> 运行中 -> 已完成 | 失败 | 已停止 | 已取消。暂停不是运行的独立状态。
type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"   // 等待运行
	RunStatusRunning   RunStatus = "running"   // 运行中
	RunStatusCompleted RunStatus = "completed" // 已完成
	RunStatusFailed    RunStatus = "failed"    // 失败
	RunStatusStopped   RunStatus = "stopped"   // 已停止
	RunStatusCancelled RunStatus = "cancelled" // 已取消
)

// RunStatusName 返回运行状态的中文显示名，界面与日志不直接使用英文键。
func RunStatusName(status RunStatus) string {
	switch status {
	case RunStatusPending:
		return "等待运行"
	case RunStatusRunning:
		return "运行中"
	case RunStatusCompleted:
		return "已完成"
	case RunStatusFailed:
		return "失败"
	case RunStatusStopped:
		return "已停止"
	case RunStatusCancelled:
		return "已取消"
	default:
		return string(status)
	}
}

// PathRunStatus 是一条路径运行（path_runs）的状态，即纲领已定义的九个中文状态之一。
// 状态只前进不回退：终态（已完成、失败、待对账、已停止、已取消）一旦进入不可离开；
// 运行中与核验中之间的前进表达步骤循环，不属于回退。
type PathRunStatus string

const (
	PathRunStatusNotStarted             PathRunStatus = "not_started"             // 未开始
	PathRunStatusWaiting                PathRunStatus = "waiting"                 // 等待运行
	PathRunStatusRunning                PathRunStatus = "running"                 // 运行中
	PathRunStatusVerifying              PathRunStatus = "verifying"               // 核验中
	PathRunStatusCompleted              PathRunStatus = "completed"               // 已完成
	PathRunStatusFailed                 PathRunStatus = "failed"                  // 失败
	PathRunStatusPaused                 PathRunStatus = "paused"                  // 暂停
	PathRunStatusAwaitingReconciliation PathRunStatus = "awaiting_reconciliation" // 待对账
	PathRunStatusStopped                PathRunStatus = "stopped"                 // 已停止
	PathRunStatusCancelled              PathRunStatus = "cancelled"               // 已取消
)

// pathRunTransitions 是路径运行的合法前进表。
// 核验中 -> 运行中表达“本步核验落账完毕、进入下一步”，是步骤循环的前进而非状态回退；
// 除这一条外任何从靠后状态回到靠前状态的迁移都非法，终态一律无出边。
// 暂停只在步骤的阶段 3（控制判定）生效，因此暂停只能从运行中进入。
var pathRunTransitions = map[PathRunStatus][]PathRunStatus{
	PathRunStatusNotStarted: {PathRunStatusWaiting},
	PathRunStatusWaiting:    {PathRunStatusRunning, PathRunStatusCancelled},
	PathRunStatusRunning: {
		PathRunStatusVerifying,
		PathRunStatusCompleted,
		PathRunStatusFailed,
		PathRunStatusPaused,
		PathRunStatusAwaitingReconciliation,
		PathRunStatusStopped,
		PathRunStatusCancelled,
	},
	PathRunStatusVerifying: {
		PathRunStatusRunning,
		PathRunStatusCompleted,
		PathRunStatusFailed,
		PathRunStatusAwaitingReconciliation,
		PathRunStatusStopped,
		PathRunStatusCancelled,
	},
	PathRunStatusPaused:                 {PathRunStatusRunning, PathRunStatusStopped, PathRunStatusCancelled},
	PathRunStatusAwaitingReconciliation: {},
	PathRunStatusCompleted:              {},
	PathRunStatusFailed:                 {},
	PathRunStatusStopped:                {},
	PathRunStatusCancelled:              {},
}

// CanAdvancePathRunStatus 判断路径运行状态迁移是否合法；未知状态一律拒绝。
func CanAdvancePathRunStatus(from, to PathRunStatus) bool {
	for _, next := range pathRunTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

// PathRunStatusName 返回九个中文节点运行态的显示名；名称与纲领一致，不新增、不改名。
func PathRunStatusName(status PathRunStatus) string {
	switch status {
	case PathRunStatusNotStarted:
		return "未开始"
	case PathRunStatusWaiting:
		return "等待运行"
	case PathRunStatusRunning:
		return "运行中"
	case PathRunStatusVerifying:
		return "核验中"
	case PathRunStatusCompleted:
		return "已完成"
	case PathRunStatusFailed:
		return "失败"
	case PathRunStatusPaused:
		return "暂停"
	case PathRunStatusAwaitingReconciliation:
		return "待对账"
	case PathRunStatusStopped:
		return "已停止"
	case PathRunStatusCancelled:
		return "已取消"
	default:
		return string(status)
	}
}

// IsTerminalPathRunStatus 判断路径运行是否已处于终态；终态不可再前进，也不可被任何恢复动作改写。
func IsTerminalPathRunStatus(status PathRunStatus) bool {
	switch status {
	case PathRunStatusAwaitingReconciliation, PathRunStatusCompleted,
		PathRunStatusFailed, PathRunStatusStopped, PathRunStatusCancelled:
		return true
	default:
		return false
	}
}

// RunMode 是运行的执行模式；本切片只交付单步模式，其余模式属于 F-017。
type RunMode string

const (
	// RunModeSingleStep 单步运行：每一步执行前都停下等待用户放行。
	RunModeSingleStep RunMode = "single_step"
)

// RunModeName 返回运行模式的中文显示名。
func RunModeName(mode RunMode) string {
	switch mode {
	case RunModeSingleStep:
		return "单步"
	default:
		return string(mode)
	}
}

// RunTriggerKind 是启动来源；本切片只支持手动启动。
type RunTriggerKind string

const RunTriggerManual RunTriggerKind = "manual"

// RunResult 是路径结果，只由执行事实得出（纲领第 7.4 节）：
// 全部步骤确定成功为成功；任一步确定失败为失败；出现写结果不确定为待对账。
type RunResult string

const (
	RunResultSucceeded         RunResult = "succeeded"               // 路径结果：成功
	RunResultFailed            RunResult = "failed"                  // 路径结果：失败
	RunResultAwaitingReconcile RunResult = "awaiting_reconciliation" // 路径结果：待对账
)

// FailureClass 是路径失败的中文可解释分类（纲领第 7.4 节），与日志 error_class 同名同义。
type FailureClass string

const (
	FailureClassGateBlocked     FailureClass = "gate_blocked"     // 门禁不通过
	FailureClassActorUnresolved FailureClass = "actor_unresolved" // 演员不可解析
	FailureClassTargetRejected  FailureClass = "target_rejected"  // 目标拒绝
	FailureClassWriteUncertain  FailureClass = "write_uncertain"  // 写结果不确定
	FailureClassToolBug         FailureClass = "tool_bug"         // 工具缺陷
)

// FailureClassName 返回失败分类的中文显示名。
func FailureClassName(class FailureClass) string {
	switch class {
	case FailureClassGateBlocked:
		return "门禁不通过"
	case FailureClassActorUnresolved:
		return "演员不可解析"
	case FailureClassTargetRejected:
		return "目标拒绝"
	case FailureClassWriteUncertain:
		return "写结果不确定"
	case FailureClassToolBug:
		return "工具缺陷"
	default:
		return string(class)
	}
}

// Run 是一次启动的运行聚合（runs 表）。
type Run struct {
	ID             uint64
	PlanID         uint64
	RunNo          uint64
	Mode           RunMode
	TriggerKind    RunTriggerKind
	MaxConcurrency *int
	Status         RunStatus
	Result         *RunResult
	StartedAt      *time.Time
	FinishedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// PathRun 是一条执行路径的运行聚合（path_runs 表）。
// 一条路径运行独占一个真实主实例：MainInstanceRef 只存不透明引用，
// 首步发起创建实例后写入，此后该路径运行的所有写动作都作用于同一实例。
type PathRun struct {
	ID                 uint64
	RunID              uint64
	ExecutionPathID    uint64
	Status             PathRunStatus
	Result             *RunResult
	FailureClass       *FailureClass
	MainInstanceRef    string
	FinalTargetSummary string
	LeaseOwner         string
	LeaseExpiresAt     *time.Time
	FencingToken       uint64
	StartedAt          *time.Time
	FinishedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// RunEvent 是运行事件流的一行（run_events 表）：聚合表每次状态前进在同一事务内追加一行。
type RunEvent struct {
	ID        uint64
	RunID     uint64
	PathRunID *uint64
	Kind      string
	Label     string
	Detail    string
	CreatedAt time.Time
}

// PathRunStatusChangeError 表示一次被拒绝的状态迁移，携带迁移双方便于中文说明。
type PathRunStatusChangeError struct {
	From PathRunStatus
	To   PathRunStatus
}

// Error 说明迁移被拒绝的原因：状态只前进不回退，终态不可离开。
func (e *PathRunStatusChangeError) Error() string {
	return fmt.Sprintf("路径运行状态不允许从 %s 前进到 %s", PathRunStatusName(e.From), PathRunStatusName(e.To))
}

// RunStepStatus 是步骤落账时的事实结论，与三值判定一一对应。
type RunStepStatus string

const (
	RunStepSucceeded RunStepStatus = "succeeded" // 确定成功
	RunStepFailed    RunStepStatus = "failed"    // 确定失败
	RunStepUncertain RunStepStatus = "uncertain" // 写结果不确定
)

// RunStep 是一个编译步骤的执行事实（run_steps 表），落账时一次性 INSERT。
type RunStep struct {
	ID           uint64
	PathRunID    uint64
	StepNo       int
	Source       string
	Action       string
	NodeKey      string
	ActorSummary string
	GateSnapshot string
	Status       RunStepStatus
	StartedAt    time.Time
	FinishedAt   time.Time
}

// RunStepAttempt 是一次尝试的判定事实（run_step_attempts 表），与所属步骤同事务 INSERT。
// trace_id 与 curl_trace_id 使本记录与 network.log/curl.log 双向可达；LogPath/LogLine 指向 step.log 具体行。
type RunStepAttempt struct {
	PathRunID    uint64
	StepID       uint64
	AttemptNo    int
	Verdict      string
	SideEffect   string
	Transport    string
	StatusCode   int
	Initial      string
	Reread       string
	FailureClass *FailureClass
	Reason       string
	Basis        string
	TraceID      string
	CurlTraceID  string
	LogPath      string
	LogLine      uint64
	DurationMs   int64
}

// RunControlAction 是人工控制事实的动作类别；本切片只承载放行与停止两类。
type RunControlAction string

const (
	RunControlApprove RunControlAction = "approve" // 放行
	RunControlStop    RunControlAction = "stop"    // 停止
)

// RunControlSource 是人工控制事实的来源。
type RunControlSource string

const RunControlSourceUI RunControlSource = "ui" // 界面按钮（放行与停止不绑单键快捷键，只接受明确点击）

// RunControl 是一次人工控制事实（run_controls 表），只 INSERT，可审计。
type RunControl struct {
	RunID     uint64
	PathRunID uint64
	Action    RunControlAction
	Source    RunControlSource
	CreatedAt time.Time
}
