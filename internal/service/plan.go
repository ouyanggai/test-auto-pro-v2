package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

type PlanErrorKind string

const (
	PlanErrorInvalidArgument PlanErrorKind = "invalid_argument"
	PlanErrorNotFound        PlanErrorKind = "not_found"
	PlanErrorStorage         PlanErrorKind = "storage_unavailable"
	PlanErrorDataInvalid     PlanErrorKind = "data_invalid"
	defaultPlanListLimit                   = 200
)

type PlanError struct {
	Kind    PlanErrorKind
	Message string
}

func (e *PlanError) Error() string { return e.Message }

func IsPlanErrorKind(err error, kind PlanErrorKind) bool {
	var planErr *PlanError
	return errors.As(err, &planErr) && planErr.Kind == kind
}

type CreatePlanInput struct {
	Name               string
	Account            string
	AccountDisplayName string
	FlowSource         string
	TargetObjectID     string
	TargetObjectName   string
	RunMode            string
	MaxConcurrency     *int
	ScheduledAt        *time.Time
}

type PlanService struct {
	repository repository.PlanRepository
	now        func() time.Time
}

func NewPlanService(planRepository repository.PlanRepository) *PlanService {
	return &PlanService{repository: planRepository, now: time.Now}
}

func (s *PlanService) Create(ctx context.Context, createKey string, input CreatePlanInput) (model.Plan, bool, error) {
	input = normalizeCreateInput(input)
	if message := validateCreateInput(createKey, input, s.now().UTC()); message != "" {
		return model.Plan{}, false, &PlanError{Kind: PlanErrorInvalidArgument, Message: message}
	}
	now := s.now().UTC()
	plan := model.Plan{
		Name:               input.Name,
		Account:            input.Account,
		AccountDisplayName: input.AccountDisplayName,
		FlowSource:         input.FlowSource,
		TargetObjectID:     input.TargetObjectID,
		TargetObjectName:   input.TargetObjectName,
		RunMode:            input.RunMode,
		MaxConcurrency:     input.MaxConcurrency,
		ScheduledAt:        input.ScheduledAt,
		Status:             model.PlanStatusPendingConfiguration,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	createdPlan, created, err := s.repository.Create(ctx, createKey, plan)
	if err != nil {
		return model.Plan{}, false, mapRepositoryError(err)
	}
	return createdPlan, created, nil
}

func (s *PlanService) List(ctx context.Context, name string, status model.PlanStatus) ([]model.Plan, error) {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > 60 {
		return nil, &PlanError{Kind: PlanErrorInvalidArgument, Message: "计划名称筛选不能超过 60 个字符"}
	}
	if status != "" && !model.ValidPlanStatus(status) {
		return nil, &PlanError{Kind: PlanErrorInvalidArgument, Message: "计划状态不正确"}
	}
	plans, err := s.repository.List(ctx, model.PlanListFilter{Name: name, Status: status, Limit: defaultPlanListLimit})
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return plans, nil
}

func (s *PlanService) Get(ctx context.Context, id uint64) (model.Plan, error) {
	if id == 0 {
		return model.Plan{}, &PlanError{Kind: PlanErrorInvalidArgument, Message: "计划 ID 不正确"}
	}
	plan, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.Plan{}, mapRepositoryError(err)
	}
	return plan, nil
}

func normalizeCreateInput(input CreatePlanInput) CreatePlanInput {
	input.Name = strings.TrimSpace(input.Name)
	input.Account = strings.TrimSpace(input.Account)
	input.AccountDisplayName = strings.TrimSpace(input.AccountDisplayName)
	input.FlowSource = strings.TrimSpace(input.FlowSource)
	input.TargetObjectID = strings.TrimSpace(input.TargetObjectID)
	input.TargetObjectName = strings.TrimSpace(input.TargetObjectName)
	input.RunMode = strings.TrimSpace(input.RunMode)
	if input.ScheduledAt != nil {
		value := input.ScheduledAt.UTC()
		input.ScheduledAt = &value
	}
	return input
}

func validateCreateInput(createKey string, input CreatePlanInput, now time.Time) string {
	if !validUUID(createKey) {
		return "创建请求标识不正确，请重试"
	}
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 60 {
		return "计划名称应为 1 至 60 个字符"
	}
	if input.Account == "" || utf8.RuneCountInString(input.Account) > 100 {
		return "账号不能为空且不能超过 100 个字符"
	}
	if utf8.RuneCountInString(input.AccountDisplayName) > 100 {
		return "账号显示名称不能超过 100 个字符"
	}
	if input.FlowSource != "new" && input.FlowSource != "started" && input.FlowSource != "pending" {
		return "流程来源不正确"
	}
	if input.TargetObjectID == "" || utf8.RuneCountInString(input.TargetObjectID) > 100 {
		return "请选择有效流程"
	}
	if input.TargetObjectName == "" || utf8.RuneCountInString(input.TargetObjectName) > 255 {
		return "流程名称不能为空且不能超过 255 个字符"
	}
	if input.RunMode != "serial" && input.RunMode != "parallel" {
		return "运行方式不正确"
	}
	if input.RunMode == "serial" && input.MaxConcurrency != nil {
		return "串行运行不能设置并行最大并发数"
	}
	if input.RunMode == "parallel" && (input.MaxConcurrency == nil || *input.MaxConcurrency < 2 || *input.MaxConcurrency > 20) {
		return "并行最大并发数应为 2 至 20"
	}
	if input.ScheduledAt != nil && !input.ScheduledAt.After(now) {
		return "启动时间必须晚于当前时间"
	}
	return ""
}

func validUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if character != '-' {
				return false
			}
			continue
		}
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}

func mapRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrPlanNotFound):
		return &PlanError{Kind: PlanErrorNotFound, Message: "计划不存在"}
	case errors.Is(err, repository.ErrPlanDataInvalid):
		return &PlanError{Kind: PlanErrorDataInvalid, Message: "计划数据异常"}
	default:
		return &PlanError{Kind: PlanErrorStorage, Message: "计划存储暂不可用"}
	}
}
