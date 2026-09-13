package repository

import (
	"context"
	"errors"

	"test-auto-pro-v2/internal/model"
)

var (
	ErrPlanNotFound    = errors.New("计划不存在")
	ErrPlanDataInvalid = errors.New("计划数据异常")
)

type PlanRepository interface {
	Create(context.Context, string, model.Plan) (model.Plan, bool, error)
	// Update 保存计划当前配置；账号或流程绑定变化时同步标记已有路径配置受影响。
	Update(context.Context, uint64, model.Plan) (model.Plan, error)
	List(context.Context, model.PlanListFilter) ([]model.Plan, error)
	Get(context.Context, uint64) (model.Plan, error)
	// Delete 删除本系统计划；数据库外键同时清除其路径和工具侧配置。
	Delete(context.Context, uint64) error
}
