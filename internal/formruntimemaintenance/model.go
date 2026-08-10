package formruntimemaintenance

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	ErrJobNotFound      = errors.New("表单运行时同步任务不存在")
	ErrJobAlreadyActive = errors.New("已有表单运行时同步任务正在执行")
	ErrJobNotReady      = errors.New("没有可领取的表单运行时同步任务")
	ErrStaleLease       = errors.New("表单运行时同步任务租约已失效")
	ErrLogNotFound      = errors.New("表单运行时同步日志不存在")
	ErrSourceInvalid    = errors.New("表单运行时固定来源不符合约束")
	ErrTargetModified   = errors.New("表单运行时实际源码存在同步任务之外的修改")
)

// JobStatus 表示维护任务的持久化生命周期。
type JobStatus string

const (
	JobPending   JobStatus = "PENDING"
	JobRunning   JobStatus = "RUNNING"
	JobSucceeded JobStatus = "SUCCEEDED"
	JobFailed    JobStatus = "FAILED"
)

// Stage 表示可恢复维护流水线中的确定阶段。
type Stage string

const (
	StageQueued    Stage = "QUEUED"
	StageInspect   Stage = "INSPECT"
	StageSync      Stage = "SYNC"
	StageCheck     Stage = "SYNC_CHECK"
	StageBuild     Stage = "BUILD"
	StageRestart   Stage = "RESTART"
	StageVerify    Stage = "VERIFY"
	StageCompleted Stage = "COMPLETED"
)

// RecoveryStatus 表示候选切换失败后 previous 版本的恢复结果。
type RecoveryStatus string

const (
	RecoveryNotRequired RecoveryStatus = "NOT_REQUIRED"
	RecoverySucceeded   RecoveryStatus = "SUCCEEDED"
	RecoveryFailed      RecoveryStatus = "FAILED"
	RecoveryUnknown     RecoveryStatus = "UNKNOWN"
)

// RecoveryError 携带候选失败和自动回退的权威结果。
type RecoveryError struct {
	Cause   error
	Status  RecoveryStatus
	Message string
}

// Error 返回原始候选或回退失败原因。
func (e *RecoveryError) Error() string {
	if e == nil || e.Cause == nil {
		return "表单运行时恢复失败"
	}
	return e.Cause.Error()
}

// Unwrap 允许错误映射读取底层原因。
func (e *RecoveryError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// ChangedFile 是公开的来源脏文件摘要，不包含文件内容。
type ChangedFile struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

// SourceState 是任务创建与执行前两次核对的固定来源快照。
type SourceState struct {
	Repository   string        `json:"repository"`
	Branch       string        `json:"branch"`
	Head         string        `json:"head"`
	Dirty        bool          `json:"dirty"`
	ChangedFiles []ChangedFile `json:"changedFiles"`
	InspectedAt  time.Time     `json:"inspectedAt"`
}

// Job 保存状态机、租约、候选与回退所需的最小持久事实。
type Job struct {
	ID              uint64         `json:"id"`
	Status          JobStatus      `json:"status"`
	Stage           Stage          `json:"stage"`
	Source          SourceState    `json:"source"`
	FailureReason   string         `json:"failureReason,omitempty"`
	Candidate       string         `json:"candidateVersion,omitempty"`
	Previous        string         `json:"previousVersion,omitempty"`
	RecoveryStatus  RecoveryStatus `json:"recoveryStatus"`
	RecoveryMessage string         `json:"recoveryMessage,omitempty"`
	AttemptCount    uint32         `json:"attemptCount"`
	CreatedAt       time.Time      `json:"createdAt"`
	StartedAt       *time.Time     `json:"startedAt,omitempty"`
	UpdatedAt       time.Time      `json:"updatedAt"`
	CompletedAt     *time.Time     `json:"completedAt,omitempty"`
	LeaseOwner      string         `json:"-"`
	LeaseExpiresAt  *time.Time     `json:"-"`
	FencingToken    uint64         `json:"-"`
}

// Log 是有上限的在线日志视图。
type Log struct {
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}

// Claim 描述 Worker 获取或接管过期任务所需的租约。
type Claim struct {
	WorkerID      string
	LeaseDuration time.Duration
}

// Progress 描述阶段推进，并以 fencing token 阻断旧 Worker。
type Progress struct {
	ID            uint64
	WorkerID      string
	FencingToken  uint64
	LeaseDuration time.Duration
	Stage         Stage
	Candidate     string
	Previous      string
}

// LeaseRenewal 描述长阶段执行中的租约续期。
type LeaseRenewal struct {
	ID            uint64
	WorkerID      string
	FencingToken  uint64
	LeaseDuration time.Duration
}

// Completion 描述任务最终结果及回退事实。
type Completion struct {
	ID              uint64
	WorkerID        string
	FencingToken    uint64
	FailureReason   string
	RecoveryStatus  RecoveryStatus
	RecoveryMessage string
}

// Store 抽象任务持久化，生产使用 MySQL、单元测试使用内存实现。
type Store interface {
	Create(context.Context, SourceState) (Job, error)
	Get(context.Context, uint64) (Job, error)
	Latest(context.Context) (Job, error)
	ClaimNext(context.Context, Claim) (Job, error)
	RenewLease(context.Context, LeaseRenewal) error
	UpdateProgress(context.Context, Progress) error
	Complete(context.Context, Completion) error
}

// SourceInspector 只允许读取固定参考仓库的来源状态。
type SourceInspector interface {
	Inspect(context.Context) (SourceState, error)
}

// LogStore 提供追加写和有界读取，不允许任务日志进入数据库或 API 请求体。
type LogStore interface {
	Open(context.Context, uint64) (io.WriteCloser, error)
	Read(context.Context, uint64) (Log, error)
}

// RuntimeOperator 把旧 V2 Docker 镜像语义适配为 pnpm 候选构建与版本目录切换。
type RuntimeOperator interface {
	Sync(context.Context, uint64, SourceState, io.Writer) error
	SyncCheck(context.Context, uint64, SourceState, io.Writer) error
	BuildCandidate(context.Context, uint64, io.Writer) (string, error)
	CurrentVersion(context.Context) (string, error)
	Restart(context.Context, string, string, io.Writer) error
	Verify(context.Context, string, string, io.Writer) error
}
