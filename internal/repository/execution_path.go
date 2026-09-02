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
	Get(context.Context, uint64, uint64) (model.ExecutionPath, error)
	// GetMany 在一个批次内读取路径及 choices，禁止后台任务逐路径查询。
	GetMany(context.Context, uint64, []uint64) ([]model.ExecutionPath, error)
	// FindByCreateKey 只在指定计划内查询已成功的幂等记录。
	FindByCreateKey(context.Context, uint64, string) (model.ExecutionPath, bool, error)
	Create(context.Context, uint64, string, string, []model.ExecutionPathChoice, time.Time) (model.ExecutionPath, bool, error)
	Update(context.Context, uint64, uint64, string, []model.ExecutionPathChoice, time.Time) (model.ExecutionPath, error)
	Delete(context.Context, uint64, uint64, time.Time) error
	// FindBatchByCreateKey 只在指定计划内读取已经提交成功的批量幂等结果。
	FindBatchByCreateKey(context.Context, uint64, string) (model.ExecutionPathBatchResult, bool, error)
	GeneratePathsBatch(context.Context, uint64, string, [][]model.ExecutionPathChoice, time.Time) (model.ExecutionPathBatchResult, bool, error)
}

// ExecutionPathChoiceMatcher 按目标真实分支选择查找已存在路径，供 F-012 换路确认复用。
// 该接口独立于路径 CRUD，避免把路径匹配实现塞入表单数据服务。
type ExecutionPathChoiceMatcher interface {
	FindByChoices(context.Context, uint64, []model.ExecutionPathChoice) (model.ExecutionPath, bool, error)
}
