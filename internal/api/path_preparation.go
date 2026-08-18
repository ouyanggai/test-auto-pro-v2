package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// PathPreparationService 提供 F-009 持久化批量准备任务能力。
type PathPreparationService interface {
	Create(context.Context, uint64, string) (model.PathPreparationJob, error)
	Get(context.Context, uint64, string) (model.PathPreparationJob, error)
	Active(context.Context, uint64) (model.PathPreparationJob, bool, error)
	Cancel(context.Context, uint64, string) (model.PathPreparationJob, error)
	Resume(context.Context, uint64, string) (model.PathPreparationJob, error)
	ListItems(context.Context, uint64, string, uint64, int) (model.PathPreparationItemPage, error)
}

// registerPathPreparationRoutes 注册独立批量准备任务及明细分页端点。
func registerPathPreparationRoutes(mux *http.ServeMux, preparations PathPreparationService) {
	mux.HandleFunc("POST /api/plans/{id}/path-preparations", handleCreatePathPreparation(preparations))
	mux.HandleFunc("GET /api/plans/{id}/path-preparations/active", handleActivePathPreparation(preparations))
	mux.HandleFunc("GET /api/plans/{id}/path-preparations/{jobId}", handleGetPathPreparation(preparations))
	mux.HandleFunc("GET /api/plans/{id}/path-preparations/{jobId}/items", handleListPathPreparationItems(preparations))
	mux.HandleFunc("POST /api/plans/{id}/path-preparations/{jobId}/cancel", handleCancelPathPreparation(preparations))
	mux.HandleFunc("POST /api/plans/{id}/path-preparations/{jobId}/resume", handleResumePathPreparation(preparations))
}

// handleCreatePathPreparation 只从服务端当前勾选事实创建任务。
func handleCreatePathPreparation(preparations PathPreparationService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseExecutionPathID(response, request.PathValue("id"))
		if !ok {
			return
		}
		job, err := preparations.Create(request.Context(), planID, strings.TrimSpace(request.Header.Get("Idempotency-Key")))
		if err != nil {
			writePathPreparationError(response, err)
			return
		}
		writeSuccess(response, job)
	}
}

// handleActivePathPreparation 返回刷新后需要继续展示的活动任务。
func handleActivePathPreparation(preparations PathPreparationService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseExecutionPathID(response, request.PathValue("id"))
		if !ok {
			return
		}
		job, found, err := preparations.Active(request.Context(), planID)
		if err != nil {
			writePathPreparationError(response, err)
			return
		}
		if !found {
			writeSuccess(response, nil)
			return
		}
		writeSuccess(response, job)
	}
}

// handleGetPathPreparation 返回任务真实聚合计数。
func handleGetPathPreparation(preparations PathPreparationService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseExecutionPathID(response, request.PathValue("id"))
		if !ok {
			return
		}
		job, err := preparations.Get(request.Context(), planID, strings.TrimSpace(request.PathValue("jobId")))
		if err != nil {
			writePathPreparationError(response, err)
			return
		}
		writeSuccess(response, job)
	}
}

// handleListPathPreparationItems 返回有界游标分页明细。
func handleListPathPreparationItems(preparations PathPreparationService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseExecutionPathID(response, request.PathValue("id"))
		if !ok {
			return
		}
		cursor, cursorErr := strconv.ParseUint(strings.TrimSpace(request.URL.Query().Get("cursor")), 10, 64)
		if request.URL.Query().Get("cursor") == "" {
			cursor, cursorErr = 0, nil
		}
		limit, limitErr := strconv.Atoi(strings.TrimSpace(request.URL.Query().Get("limit")))
		if request.URL.Query().Get("limit") == "" {
			limit, limitErr = 20, nil
		}
		if cursorErr != nil || limitErr != nil || limit < 1 || limit > 100 {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "任务明细分页参数不正确", false)
			return
		}
		page, err := preparations.ListItems(request.Context(), planID, strings.TrimSpace(request.PathValue("jobId")), cursor, limit)
		if err != nil {
			writePathPreparationError(response, err)
			return
		}
		writeSuccess(response, page)
	}
}

// handleCancelPathPreparation 取消任务但保留已完成路径和未完成检查点。
func handleCancelPathPreparation(preparations PathPreparationService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseExecutionPathID(response, request.PathValue("id"))
		if !ok {
			return
		}
		job, err := preparations.Cancel(request.Context(), planID, strings.TrimSpace(request.PathValue("jobId")))
		if err != nil {
			writePathPreparationError(response, err)
			return
		}
		writeSuccess(response, job)
	}
}

// handleResumePathPreparation 从持久检查点恢复任务。
func handleResumePathPreparation(preparations PathPreparationService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseExecutionPathID(response, request.PathValue("id"))
		if !ok {
			return
		}
		job, err := preparations.Resume(request.Context(), planID, strings.TrimSpace(request.PathValue("jobId")))
		if err != nil {
			writePathPreparationError(response, err)
			return
		}
		writeSuccess(response, job)
	}
}

// writePathPreparationError 映射批量任务稳定错误，不把数据库或目标响应原文返回页面。
func writePathPreparationError(response http.ResponseWriter, err error) {
	switch {
	case service.IsPathPreparationErrorKind(err, service.PathPreparationErrorInvalid):
		writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error(), false)
	case service.IsPathPreparationErrorKind(err, service.PathPreparationErrorNotFound):
		writeFailure(response, http.StatusNotFound, "PATH_PREPARATION_NOT_FOUND", "批量准备任务不存在", false)
	case service.IsPathPreparationErrorKind(err, service.PathPreparationErrorState):
		writeFailure(response, http.StatusConflict, "PATH_PREPARATION_STATE_INVALID", err.Error(), false)
	case service.IsPathPreparationErrorKind(err, service.PathPreparationErrorStorage):
		writeFailure(response, http.StatusServiceUnavailable, "PLAN_STORAGE_UNAVAILABLE", "批量准备服务暂不可用，请重试", true)
	default:
		writePathConfigError(response, err)
	}
}

type unavailablePathPreparationService struct{}

// Create 在未注入服务时返回稳定不可用错误。
func (unavailablePathPreparationService) Create(context.Context, uint64, string) (model.PathPreparationJob, error) {
	return model.PathPreparationJob{}, &service.PathPreparationError{Kind: service.PathPreparationErrorStorage, Message: "批量准备服务暂不可用"}
}

// Get 在未注入服务时返回稳定不可用错误。
func (unavailablePathPreparationService) Get(context.Context, uint64, string) (model.PathPreparationJob, error) {
	return model.PathPreparationJob{}, &service.PathPreparationError{Kind: service.PathPreparationErrorStorage, Message: "批量准备服务暂不可用"}
}

// Active 在未注入服务时返回稳定不可用错误。
func (unavailablePathPreparationService) Active(context.Context, uint64) (model.PathPreparationJob, bool, error) {
	return model.PathPreparationJob{}, false, &service.PathPreparationError{Kind: service.PathPreparationErrorStorage, Message: "批量准备服务暂不可用"}
}

// Cancel 在未注入服务时返回稳定不可用错误。
func (unavailablePathPreparationService) Cancel(context.Context, uint64, string) (model.PathPreparationJob, error) {
	return model.PathPreparationJob{}, &service.PathPreparationError{Kind: service.PathPreparationErrorStorage, Message: "批量准备服务暂不可用"}
}

// Resume 在未注入服务时返回稳定不可用错误。
func (unavailablePathPreparationService) Resume(context.Context, uint64, string) (model.PathPreparationJob, error) {
	return model.PathPreparationJob{}, &service.PathPreparationError{Kind: service.PathPreparationErrorStorage, Message: "批量准备服务暂不可用"}
}

// ListItems 在未注入服务时返回稳定不可用错误。
func (unavailablePathPreparationService) ListItems(context.Context, uint64, string, uint64, int) (model.PathPreparationItemPage, error) {
	return model.PathPreparationItemPage{}, &service.PathPreparationError{Kind: service.PathPreparationErrorStorage, Message: "批量准备服务暂不可用"}
}
