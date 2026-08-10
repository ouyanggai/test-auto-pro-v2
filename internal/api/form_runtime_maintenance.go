package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"test-auto-pro-v2/internal/formruntimemaintenance"
)

// FormRuntimeMaintenanceService 限定维护 API 只有固定来源检查、创建、查询和日志读取。
type FormRuntimeMaintenanceService interface {
	InspectSource(context.Context) (formruntimemaintenance.SourceState, error)
	CreateJob(context.Context) (formruntimemaintenance.Job, error)
	GetJob(context.Context, uint64) (formruntimemaintenance.Job, error)
	LatestJob(context.Context) (formruntimemaintenance.Job, error)
	GetJobLog(context.Context, uint64) (formruntimemaintenance.Log, error)
}

// registerFormRuntimeMaintenanceRoutes 注册固定的一键维护端点。
func registerFormRuntimeMaintenanceRoutes(mux *http.ServeMux, service FormRuntimeMaintenanceService) {
	mux.HandleFunc("GET /api/form-runtime-maintenance/source", handleFormRuntimeSource(service))
	mux.HandleFunc("POST /api/form-runtime-maintenance/jobs", handleCreateFormRuntimeJob(service))
	mux.HandleFunc("GET /api/form-runtime-maintenance/jobs/latest", handleLatestFormRuntimeJob(service))
	mux.HandleFunc("GET /api/form-runtime-maintenance/jobs/{jobId}", handleFormRuntimeJob(service))
	mux.HandleFunc("GET /api/form-runtime-maintenance/jobs/{jobId}/log", handleFormRuntimeJobLog(service))
}

// handleFormRuntimeSource 返回来源仓库、分支、HEAD、dirty 和检查时间。
func handleFormRuntimeSource(service FormRuntimeMaintenanceService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		source, err := service.InspectSource(request.Context())
		if err != nil {
			writeFormRuntimeMaintenanceError(response, err)
			return
		}
		writeSuccess(response, source)
	}
}

// handleCreateFormRuntimeJob 创建唯一活动任务，请求体不接受路径、分支或命令。
func handleCreateFormRuntimeJob(service FormRuntimeMaintenanceService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if request.ContentLength > 0 {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "一键同步不接受自定义参数", false)
			return
		}
		job, err := service.CreateJob(request.Context())
		if err != nil {
			writeFormRuntimeMaintenanceError(response, err)
			return
		}
		writeJSON(response, http.StatusAccepted, apiSuccess{Success: true, Data: job})
	}
}

// handleLatestFormRuntimeJob 返回最近任务；无任务时使用稳定 404。
func handleLatestFormRuntimeJob(service FormRuntimeMaintenanceService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		job, err := service.LatestJob(request.Context())
		if err != nil {
			writeFormRuntimeMaintenanceError(response, err)
			return
		}
		writeSuccess(response, job)
	}
}

// handleFormRuntimeJob 返回指定任务。
func handleFormRuntimeJob(service FormRuntimeMaintenanceService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		id, ok := parseMaintenanceJobID(response, request.PathValue("jobId"))
		if !ok {
			return
		}
		job, err := service.GetJob(request.Context(), id)
		if err != nil {
			writeFormRuntimeMaintenanceError(response, err)
			return
		}
		writeSuccess(response, job)
	}
}

// handleFormRuntimeJobLog 返回指定任务的有界日志尾部。
func handleFormRuntimeJobLog(service FormRuntimeMaintenanceService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		id, ok := parseMaintenanceJobID(response, request.PathValue("jobId"))
		if !ok {
			return
		}
		log, err := service.GetJobLog(request.Context(), id)
		if err != nil {
			writeFormRuntimeMaintenanceError(response, err)
			return
		}
		writeSuccess(response, log)
	}
}

// parseMaintenanceJobID 只接受正整数任务编号。
func parseMaintenanceJobID(response http.ResponseWriter, raw string) (uint64, bool) {
	id, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil || id == 0 {
		writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "维护任务编号不正确", false)
		return 0, false
	}
	return id, true
}

// writeFormRuntimeMaintenanceError 映射维护任务稳定错误，不公开文件内容或命令输出。
func writeFormRuntimeMaintenanceError(response http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, formruntimemaintenance.ErrJobAlreadyActive):
		writeFailure(response, http.StatusConflict, "FORM_RUNTIME_SYNC_ALREADY_ACTIVE", "已有表单运行时同步任务正在执行", false)
	case errors.Is(err, formruntimemaintenance.ErrJobNotFound), errors.Is(err, formruntimemaintenance.ErrLogNotFound):
		writeFailure(response, http.StatusNotFound, "FORM_RUNTIME_SYNC_NOT_FOUND", "表单运行时同步任务不存在", false)
	case errors.Is(err, formruntimemaintenance.ErrSourceInvalid), errors.Is(err, formruntimemaintenance.ErrTargetModified):
		writeFailure(response, http.StatusConflict, "FORM_RUNTIME_SOURCE_INVALID", "固定来源或上游原样区不符合安全同步条件", false)
	default:
		writeFailure(response, http.StatusInternalServerError, "FORM_RUNTIME_MAINTENANCE_FAILED", "表单运行时维护操作失败", true)
	}
}
