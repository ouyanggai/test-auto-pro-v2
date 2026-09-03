package repository

import (
	"context"
	"errors"
	"time"

	"test-auto-pro-v2/internal/model"
)

var (
	// ErrHistorySnapshotNotFound 表示候选快照不属于当前计划或已经不存在。
	ErrHistorySnapshotNotFound = errors.New("基础表单数据不存在")
	// ErrHistoryRevisionConflict 表示默认来源或路径来源发生并发修订。
	ErrHistoryRevisionConflict = errors.New("基础表单数据修订号冲突")
	// ErrHistoryReplayActive 表示同一计划已有活动历史回放任务。
	ErrHistoryReplayActive = errors.New("批量准备任务已经存在")
	// ErrHistoryReplayNotFound 表示任务不属于当前计划或已经不存在。
	ErrHistoryReplayNotFound = errors.New("批量准备任务不存在")
	// ErrHistoryReplayState 表示任务当前状态不能执行请求的转换。
	ErrHistoryReplayState = errors.New("批量准备任务状态不允许当前操作")
	// ErrHistoryReplayIdempotency 表示同一幂等键被复用于不同的路径集合。
	ErrHistoryReplayIdempotency = errors.New("批量准备幂等键不能复用于不同路径")
	// ErrHistoryPathConfigConflict 表示路径原始数据保存的修订号或幂等请求正文发生冲突。
	ErrHistoryPathConfigConflict = errors.New("路径业务表单数据修订号冲突")
	// ErrHistoryPathConfigIdempotency 表示同一幂等键被复用于不同的原始表单数据。
	ErrHistoryPathConfigIdempotency = errors.New("路径业务表单数据幂等键不能复用于不同正文")
	// ErrHistoryPathConfigDataInvalid 表示 F-012 路径配置 JSON 无法解析或缺少必要字段。
	ErrHistoryPathConfigDataInvalid = errors.New("路径业务配置数据异常")
)

// HistoryDefaultRecord 是计划默认来源的工具侧持久化记录。
type HistoryDefaultRecord struct {
	PlanID         uint64
	SnapshotID     uint64
	Revision       uint64
	IdempotencyKey string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// HistoryPathSourceRecord 是路径来源覆盖的持久化记录。
type HistoryPathSourceRecord struct {
	PathID         uint64
	Mode           string
	SnapshotID     uint64
	Revision       uint64
	IdempotencyKey string
	UpdatedAt      time.Time
}

// HistoryReplayStore 持久化 F-012 历史快照、来源和可恢复回放任务。
type HistoryReplayStore interface {
	SaveSnapshot(context.Context, model.HistorySnapshot) (model.HistorySnapshot, error)
	SaveDefaultWithSnapshot(context.Context, model.HistorySnapshot, HistoryDefaultRecord, uint64, time.Time) (model.HistorySnapshot, HistoryDefaultRecord, error)
	SavePathSourceWithSnapshot(context.Context, uint64, model.HistorySnapshot, HistoryPathSourceRecord, uint64, time.Time) (model.HistorySnapshot, HistoryPathSourceRecord, error)
	GetSnapshotByCandidate(context.Context, uint64, string) (model.HistorySnapshot, error)
	FindSnapshotByCandidate(context.Context, uint64, string) (model.HistorySnapshot, bool, error)
	GetSnapshot(context.Context, uint64, uint64) (model.HistorySnapshot, error)
	GetDefault(context.Context, uint64) (HistoryDefaultRecord, bool, error)
	SaveDefault(context.Context, HistoryDefaultRecord, uint64, time.Time) (HistoryDefaultRecord, error)
	GetPathSource(context.Context, uint64) (HistoryPathSourceRecord, bool, error)
	SavePathSource(context.Context, uint64, HistoryPathSourceRecord, uint64, time.Time) (HistoryPathSourceRecord, error)
	CreateReplay(context.Context, model.HistoryReplayJob, []model.HistoryReplayItem) (model.HistoryReplayJob, bool, error)
	GetReplay(context.Context, uint64, string) (model.HistoryReplayJob, error)
	FindActiveReplay(context.Context, uint64) (model.HistoryReplayJob, bool, error)
	ListRecoverableReplays(context.Context) ([]model.HistoryReplayJob, error)
	UpdateReplayStatus(context.Context, uint64, string, string, time.Time) (model.HistoryReplayJob, error)
	ClaimReplayItems(context.Context, string, int, string, time.Time) ([]model.HistoryReplayItem, error)
	CompleteReplayItem(context.Context, string, uint64, model.HistoryReplayItem, time.Time) error
	RecountReplay(context.Context, string, time.Time) (model.HistoryReplayJob, error)
	ListReplayItems(context.Context, uint64, string, uint64, int) (model.HistoryReplayItemPage, error)
}

// HistoryPathConfigRecord 是路径配置表新领域字段的结构化存储模型。
type HistoryPathConfigRecord struct {
	PathID            uint64
	Revision          uint64
	NodeRevision      uint64
	DataRevision      uint64
	ActionRevision    uint64
	IdempotencyKey    string
	ConfigStatus      string
	NodeStatus        string
	DataStatus        string
	SourceMode        string
	SnapshotID        *uint64
	RuntimeType       string
	PersonStrategies  []byte
	UserActions       []byte
	CompiledSteps     []byte
	ConfirmedNodeKeys []byte
	EffectiveFormData []byte
	BranchPatches     []byte
	RuntimeValidation []byte
	Issues            []byte
	LatestIdempotency []byte
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// HistoryPathConfigStore 保存路径人员、历史数据和动作编排的独立领域列。
type HistoryPathConfigStore interface {
	GetPathConfig(context.Context, uint64) (HistoryPathConfigRecord, bool, error)
	SavePathConfig(context.Context, HistoryPathConfigRecord, uint64, time.Time) (HistoryPathConfigRecord, error)
}

// HistoryPathDataStore 在同一数据库事务内保存原始表单数据，并在换路时锁定来源与目标路径。
// 单路径保存沿用 HistoryPathConfigStore；换路必须使用此接口，禁止服务层拆成两次写入。
type HistoryPathDataStore interface {
	HistoryPathConfigStore
	SavePathData(context.Context, uint64, uint64, uint64, HistoryPathConfigRecord, uint64, time.Time) (HistoryPathConfigRecord, error)
}
