package repository

import (
	"context"
	"errors"
	"time"

	"test-auto-pro-v2/internal/model"
)

var (
	// ErrPathPreparationNotFound 表示任务不属于指定计划或已经不存在。
	ErrPathPreparationNotFound = errors.New("批量路径准备任务不存在")
	// ErrPathPreparationEmpty 表示当前计划没有勾选任何路径。
	ErrPathPreparationEmpty = errors.New("没有勾选需要准备的路径")
	// ErrPathPreparationState 表示当前任务状态不能执行请求的转换。
	ErrPathPreparationState = errors.New("批量路径准备任务状态不允许当前操作")
)

// PathPreparationRepository 持久化独立任务、逐路径检查点与真实计数。
type PathPreparationRepository interface {
	Create(context.Context, uint64, string, time.Time) (model.PathPreparationJob, bool, error)
	Get(context.Context, uint64, string) (model.PathPreparationJob, error)
	FindActive(context.Context, uint64) (model.PathPreparationJob, bool, error)
	ListRecoverable(context.Context) ([]model.PathPreparationJob, error)
	Start(context.Context, uint64, string, time.Time) error
	ClaimBatch(context.Context, uint64, string, int, time.Time) ([]model.PathPreparationItem, error)
	CompleteItem(context.Context, uint64, string, uint64, model.PathPreparationItemResult, time.Time) error
	Finish(context.Context, uint64, string, time.Time) (model.PathPreparationJob, error)
	Cancel(context.Context, uint64, string, time.Time) (model.PathPreparationJob, error)
	Resume(context.Context, uint64, string, time.Time) (model.PathPreparationJob, error)
	Fail(context.Context, uint64, string, string, time.Time) error
	ListItems(context.Context, uint64, string, uint64, int) (model.PathPreparationItemPage, error)
}
