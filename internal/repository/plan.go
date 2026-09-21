package repository

import (
	"context"
	"errors"

	"test-auto-pro-v2/internal/model"
)

var (
	ErrPlanNotFound       = errors.New("计划不存在")
	ErrPlanDataInvalid    = errors.New("计划数据异常")
	ErrPlanStatusConflict = errors.New("计划状态冲突")
)

type PlanRepository interface {
	Create(context.Context, string, model.Plan) (model.Plan, bool, error)
	// Update 保存计划当前配置；账号或流程绑定变化时同步标记已有路径配置受影响。
	Update(context.Context, uint64, model.Plan) (model.Plan, error)
	List(context.Context, model.PlanListFilter) ([]model.Plan, error)
	// Count 返回与 List 同筛选条件下的计划总数，供分页展示。
	Count(context.Context, model.PlanListFilter) (int, error)
	Get(context.Context, uint64) (model.Plan, error)
	// Delete 删除计划及其全部运行记录；有活跃运行返回 ErrPlanStatusConflict。
	Delete(context.Context, uint64) error
	// BatchDelete 批量删除计划；返回失败映射（空映射表示全部成功）。
	BatchDelete(context.Context, []uint64) map[uint64]error
}
