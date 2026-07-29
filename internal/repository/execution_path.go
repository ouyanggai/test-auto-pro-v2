package repository

import (
	"context"
	"errors"
	"time"

	"test-auto-pro-v2/internal/model"
)

var (
	ErrExecutionPathNotFound    = errors.New("执行路径不存在")
	ErrExecutionPathLimit       = errors.New("执行路径数量已达上限")
	ErrExecutionPathPlanLocked  = errors.New("计划已锁定")
	ErrExecutionPathDataInvalid = errors.New("执行路径数据异常")
	ErrExecutionPathSource      = errors.New("计划来源不支持批量生成路径")
)

type ExecutionPathRepository interface {
	List(context.Context, uint64) ([]model.ExecutionPath, error)
	// FindByCreateKey 只在指定计划内查询已成功的幂等记录。
	FindByCreateKey(context.Context, uint64, string) (model.ExecutionPath, bool, error)
	Create(context.Context, uint64, string, string, []model.ExecutionPathChoice, time.Time) (model.ExecutionPath, bool, error)
	Update(context.Context, uint64, uint64, string, []model.ExecutionPathChoice, time.Time) (model.ExecutionPath, error)
	Delete(context.Context, uint64, uint64, time.Time) error
	// FindBatchByCreateKey 只在指定计划内读取已经提交成功的批量幂等结果。
	FindBatchByCreateKey(context.Context, uint64, string) (model.ExecutionPathBatchResult, bool, error)
	GenerateAll(context.Context, uint64, string, [][]model.ExecutionPathChoice, time.Time) (model.ExecutionPathBatchResult, bool, error)
}
