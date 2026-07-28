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
)

type ExecutionPathRepository interface {
	List(context.Context, uint64) ([]model.ExecutionPath, error)
	// FindByCreateKey 只在指定计划内查询已成功的幂等记录。
	FindByCreateKey(context.Context, uint64, string) (model.ExecutionPath, bool, error)
	Create(context.Context, uint64, string, []model.ExecutionPathChoice, time.Time) (model.ExecutionPath, bool, error)
	Update(context.Context, uint64, uint64, []model.ExecutionPathChoice, time.Time) (model.ExecutionPath, error)
	Delete(context.Context, uint64, uint64, time.Time) error
}
