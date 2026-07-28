package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

type PlanService interface {
	Create(context.Context, string, service.CreatePlanInput) (model.Plan, bool, error)
	List(context.Context, string, model.PlanStatus) ([]model.Plan, error)
	Get(context.Context, uint64) (model.Plan, error)
}

type createPlanRequest struct {
	Name               string  `json:"name"`
	Account            string  `json:"account"`
	AccountDisplayName string  `json:"accountDisplayName"`
	FlowSource         string  `json:"flowSource"`
	TargetObjectID     string  `json:"targetObjectId"`
	TargetObjectName   string  `json:"targetObjectName"`
	RunMode            string  `json:"runMode"`
	MaxConcurrency     *int    `json:"maxConcurrency"`
	ScheduledAt        *string `json:"scheduledAt"`
}

type planResponse struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Account            string  `json:"account"`
	AccountDisplayName string  `json:"accountDisplayName"`
	FlowSource         string  `json:"flowSource"`
	TargetObjectID     string  `json:"targetObjectId"`
	TargetObjectName   string  `json:"targetObjectName"`
	RunMode            string  `json:"runMode"`
	MaxConcurrency     *int    `json:"maxConcurrency"`
	ScheduledAt        *string `json:"scheduledAt"`
	Status             string  `json:"status"`
	PathCount          int     `json:"pathCount"`
	LastRunResult      string  `json:"lastRunResult"`
	CreatedAt          string  `json:"createdAt"`
	UpdatedAt          string  `json:"updatedAt"`
}

type planListResponse struct {
	Items []planResponse `json:"items"`
}

func registerPlanRoutes(mux *http.ServeMux, plans PlanService) {
	mux.HandleFunc("POST /api/plans", handleCreatePlan(plans))
	mux.HandleFunc("GET /api/plans", handleListPlans(plans))
	mux.HandleFunc("GET /api/plans/{id}", handleGetPlan(plans))
}

func handleCreatePlan(plans PlanService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		var input createPlanRequest
		decoder := json.NewDecoder(io.LimitReader(request.Body, maxAPIRequestBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "创建计划请求格式不正确", false)
			return
		}
		if err := ensureJSONEnd(decoder); err != nil {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "创建计划请求只能包含一个对象", false)
			return
		}
		scheduledAt, ok := parseOptionalRFC3339(response, input.ScheduledAt)
		if !ok {
			return
		}
		plan, created, err := plans.Create(request.Context(), strings.TrimSpace(request.Header.Get("Idempotency-Key")), service.CreatePlanInput{
			Name:               input.Name,
			Account:            input.Account,
			AccountDisplayName: input.AccountDisplayName,
			FlowSource:         input.FlowSource,
			TargetObjectID:     input.TargetObjectID,
			TargetObjectName:   input.TargetObjectName,
			RunMode:            input.RunMode,
			MaxConcurrency:     input.MaxConcurrency,
			ScheduledAt:        scheduledAt,
		})
		if err != nil {
			writePlanError(response, err)
			return
		}
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		writeJSON(response, status, apiSuccess{Success: true, Data: toPlanResponse(plan)})
	}
}

func handleListPlans(plans PlanService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		items, err := plans.List(
			request.Context(),
			request.URL.Query().Get("name"),
			model.PlanStatus(strings.TrimSpace(request.URL.Query().Get("status"))),
		)
		if err != nil {
			writePlanError(response, err)
			return
		}
		result := make([]planResponse, 0, len(items))
		for _, item := range items {
			result = append(result, toPlanResponse(item))
		}
		writeSuccess(response, planListResponse{Items: result})
	}
}

func handleGetPlan(plans PlanService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		id, err := strconv.ParseUint(request.PathValue("id"), 10, 64)
		if err != nil || id == 0 {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "计划 ID 不正确", false)
			return
		}
		plan, err := plans.Get(request.Context(), id)
		if err != nil {
			writePlanError(response, err)
			return
		}
		writeSuccess(response, toPlanResponse(plan))
	}
}

func parseOptionalRFC3339(response http.ResponseWriter, value *string) (*time.Time, bool) {
	if value == nil {
		return nil, true
	}
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(*value))
	if err != nil {
		writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "启动时间格式不正确", false)
		return nil, false
	}
	parsed = parsed.UTC()
	return &parsed, true
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	return errors.New("存在额外 JSON 内容")
}

func toPlanResponse(plan model.Plan) planResponse {
	var scheduledAt *string
	if plan.ScheduledAt != nil {
		value := plan.ScheduledAt.UTC().Format(time.RFC3339Nano)
		scheduledAt = &value
	}
	return planResponse{
		ID:                 strconv.FormatUint(plan.ID, 10),
		Name:               plan.Name,
		Account:            plan.Account,
		AccountDisplayName: plan.AccountDisplayName,
		FlowSource:         plan.FlowSource,
		TargetObjectID:     plan.TargetObjectID,
		TargetObjectName:   plan.TargetObjectName,
		RunMode:            plan.RunMode,
		MaxConcurrency:     plan.MaxConcurrency,
		ScheduledAt:        scheduledAt,
		Status:             string(plan.Status),
		PathCount:          0,
		LastRunResult:      "",
		CreatedAt:          plan.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:          plan.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func writePlanError(response http.ResponseWriter, err error) {
	switch {
	case service.IsPlanErrorKind(err, service.PlanErrorInvalidArgument):
		writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error(), false)
	case service.IsPlanErrorKind(err, service.PlanErrorNotFound):
		writeFailure(response, http.StatusNotFound, "PLAN_NOT_FOUND", "计划不存在", false)
	case service.IsPlanErrorKind(err, service.PlanErrorDataInvalid):
		writeFailure(response, http.StatusInternalServerError, "PLAN_DATA_INVALID", "计划数据异常，请联系维护人员", false)
	default:
		writeFailure(response, http.StatusServiceUnavailable, "PLAN_STORAGE_UNAVAILABLE", "计划存储暂不可用，请重试", true)
	}
}

type unavailablePlanService struct{}

func (unavailablePlanService) Create(context.Context, string, service.CreatePlanInput) (model.Plan, bool, error) {
	return model.Plan{}, false, &service.PlanError{Kind: service.PlanErrorStorage, Message: "计划存储暂不可用"}
}

func (unavailablePlanService) List(context.Context, string, model.PlanStatus) ([]model.Plan, error) {
	return nil, &service.PlanError{Kind: service.PlanErrorStorage, Message: "计划存储暂不可用"}
}

func (unavailablePlanService) Get(context.Context, uint64) (model.Plan, error) {
	return model.Plan{}, &service.PlanError{Kind: service.PlanErrorStorage, Message: "计划存储暂不可用"}
}
