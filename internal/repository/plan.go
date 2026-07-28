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
	List(context.Context, model.PlanListFilter) ([]model.Plan, error)
	Get(context.Context, uint64) (model.Plan, error)
}
