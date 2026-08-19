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

// Error 返回面向调用方的稳定计划错误文案。
func (e *PlanError) Error() string { return e.Message }

// IsPlanErrorKind 判断错误是否属于指定计划错误类别。
func IsPlanErrorKind(err error, kind PlanErrorKind) bool {
	var planErr *PlanError
	return errors.As(err, &planErr) && planErr.Kind == kind
}

// CreatePlanInput 接收新计划已验证的业务字段。
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

// PlanService 管理计划持久化与公开三态边界。
type PlanService struct {
	repository repository.PlanRepository
	now        func() time.Time
}

// NewPlanService 创建计划服务。
func NewPlanService(planRepository repository.PlanRepository) *PlanService {
	return &PlanService{repository: planRepository, now: time.Now}
}

// Create 创建未运行计划，同一幂等键始终返回同一记录。
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
		Status:             model.PlanStatusNotStarted,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	createdPlan, created, err := s.repository.Create(ctx, createKey, plan)
	if err != nil {
		return model.Plan{}, false, mapRepositoryError(err)
	}
	return createdPlan, created, nil
}

// List 按名称和公开三态筛选计划。
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

// Get 按主键读取计划。
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

// Delete 删除尚未进入运行事实的开发计划，并依赖仓储外键清理路径与工具侧配置。
func (s *PlanService) Delete(ctx context.Context, id uint64) error {
	if id == 0 {
		return &PlanError{Kind: PlanErrorInvalidArgument, Message: "计划 ID 不正确"}
	}
	plan, err := s.repository.Get(ctx, id)
	if err != nil {
		return mapRepositoryError(err)
	}
	if plan.Status != model.PlanStatusNotStarted {
		return &PlanError{Kind: PlanErrorInvalidArgument, Message: "已有运行记录的计划不能删除"}
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return mapRepositoryError(err)
	}
	return nil
}

// normalizeCreateInput 清理计划文本并统一定时时区。
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

// validateCreateInput 校验计划创建边界并返回稳定中文错误。
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

// validUUID 校验幂等键是否为标准 UUID 文本。
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

// mapRepositoryError 把仓储错误收敛为脱敏计划错误。
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
