package backend_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	"test-auto-pro-v2/internal/service"
)

const testCreateKey = "123e4567-e89b-12d3-a456-426614174000"

type memoryPlanRepository struct {
	nextID      uint64
	byCreateKey map[string]model.Plan
	plans       []model.Plan
	err         error
}

func newMemoryPlanRepository() *memoryPlanRepository {
	return &memoryPlanRepository{nextID: 1, byCreateKey: map[string]model.Plan{}}
}

func (r *memoryPlanRepository) Create(_ context.Context, createKey string, plan model.Plan) (model.Plan, bool, error) {
	if r.err != nil {
		return model.Plan{}, false, r.err
	}
	if existing, ok := r.byCreateKey[createKey]; ok {
		return existing, false, nil
	}
	plan.ID = r.nextID
	r.nextID++
	r.byCreateKey[createKey] = plan
	r.plans = append(r.plans, plan)
	return plan, true, nil
}

func (r *memoryPlanRepository) List(_ context.Context, filter model.PlanListFilter) ([]model.Plan, error) {
	if r.err != nil {
		return nil, r.err
	}
	return append([]model.Plan(nil), r.plans...), nil
}

func (r *memoryPlanRepository) Get(_ context.Context, id uint64) (model.Plan, error) {
	if r.err != nil {
		return model.Plan{}, r.err
	}
	for _, plan := range r.plans {
		if plan.ID == id {
			return plan, nil
		}
	}
	return model.Plan{}, repository.ErrPlanNotFound
}

// Delete 从内存夹具中移除计划，模拟数据库级联清理后的计划不可再读取状态。
func (r *memoryPlanRepository) Delete(_ context.Context, id uint64) error {
	if r.err != nil {
		return r.err
	}
	for index, plan := range r.plans {
		if plan.ID == id {
			r.plans = append(r.plans[:index], r.plans[index+1:]...)
			return nil
		}
	}
	return repository.ErrPlanNotFound
}

func TestPlanServiceCreatesPendingPlanAndReusesIdempotencyKey(t *testing.T) {
	repo := newMemoryPlanRepository()
	plans := service.NewPlanService(repo)
	concurrency := 3
	scheduledAt := time.Now().UTC().Add(24 * time.Hour)
	input := service.CreatePlanInput{
		Name: "  采购回归  ", Account: " tester01 ", AccountDisplayName: " 测试专员 ",
		FlowSource: "new", TargetObjectID: "template-id", TargetObjectName: "采购流程",
		RunMode: "parallel", MaxConcurrency: &concurrency, ScheduledAt: &scheduledAt,
	}
	first, created, err := plans.Create(context.Background(), testCreateKey, input)
	if err != nil || !created {
		t.Fatalf("首次创建失败：created=%v err=%v", created, err)
	}
	second, created, err := plans.Create(context.Background(), testCreateKey, input)
	if err != nil || created || first.ID != second.ID || len(repo.plans) != 1 {
		t.Fatal("同一幂等键未返回同一条计划")
	}
	if first.Name != "采购回归" || first.Account != "tester01" || first.Status != model.PlanStatusPendingConfiguration {
		t.Fatal("计划标准化或初始状态不正确")
	}
	if first.ScheduledAt == nil || first.ScheduledAt.Location() != time.UTC {
		t.Fatal("定时时间未统一为 UTC")
	}
}

// TestPlanServiceDeletesDevelopmentPlan 验证待配置计划可以删除且后续读取返回不存在。
func TestPlanServiceDeletesDevelopmentPlan(t *testing.T) {
	repo := newMemoryPlanRepository()
	repo.plans = []model.Plan{{ID: 9, Status: model.PlanStatusPendingConfiguration}}
	plans := service.NewPlanService(repo)
	if err := plans.Delete(context.Background(), 9); err != nil {
		t.Fatalf("删除开发计划失败：%v", err)
	}
	if _, err := plans.Get(context.Background(), 9); !service.IsPlanErrorKind(err, service.PlanErrorNotFound) {
		t.Fatalf("删除后计划仍可读取：%v", err)
	}
}

func TestPlanServiceValidatesCreateBoundaries(t *testing.T) {
	parallelOne := 1
	serialValue := 2
	past := time.Now().UTC().Add(-time.Minute)
	valid := service.CreatePlanInput{
		Name: "计划", Account: "tester01", FlowSource: "new", TargetObjectID: "template-id",
		TargetObjectName: "采购流程", RunMode: "serial",
	}
	tests := []struct {
		name string
		key  string
		edit func(*service.CreatePlanInput)
	}{
		{name: "幂等键缺失", key: ""},
		{name: "计划名缺失", key: testCreateKey, edit: func(input *service.CreatePlanInput) { input.Name = "" }},
		{name: "来源非法", key: testCreateKey, edit: func(input *service.CreatePlanInput) { input.FlowSource = "other" }},
		{name: "流程缺失", key: testCreateKey, edit: func(input *service.CreatePlanInput) { input.TargetObjectID = "" }},
		{name: "串行含并发数", key: testCreateKey, edit: func(input *service.CreatePlanInput) { input.MaxConcurrency = &serialValue }},
		{name: "并行并发越界", key: testCreateKey, edit: func(input *service.CreatePlanInput) { input.RunMode = "parallel"; input.MaxConcurrency = &parallelOne }},
		{name: "定时已过期", key: testCreateKey, edit: func(input *service.CreatePlanInput) { input.ScheduledAt = &past }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := valid
			if test.edit != nil {
				test.edit(&input)
			}
			_, _, err := service.NewPlanService(newMemoryPlanRepository()).Create(context.Background(), test.key, input)
			if !service.IsPlanErrorKind(err, service.PlanErrorInvalidArgument) {
				t.Fatalf("非法创建未返回参数错误：%v", err)
			}
		})
	}
}

func TestPlanServiceMapsRepositoryErrorsAndListStatus(t *testing.T) {
	repo := newMemoryPlanRepository()
	repo.err = errors.New("database secret should not escape")
	plans := service.NewPlanService(repo)
	_, err := plans.List(context.Background(), "", "")
	if !service.IsPlanErrorKind(err, service.PlanErrorStorage) || err.Error() != "计划存储暂不可用" {
		t.Fatal("仓储错误未映射为稳定脱敏错误")
	}

	repo.err = nil
	_, err = plans.List(context.Background(), "", model.PlanStatus("unknown"))
	if !service.IsPlanErrorKind(err, service.PlanErrorInvalidArgument) {
		t.Fatal("非法列表状态未返回参数错误")
	}
	_, err = plans.Get(context.Background(), 99)
	if !service.IsPlanErrorKind(err, service.PlanErrorNotFound) {
		t.Fatal("不存在计划未返回稳定错误")
	}
}
