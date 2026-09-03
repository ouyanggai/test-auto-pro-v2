package api

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"

	"test-auto-pro-v2/internal/jsonvalues"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// HistoryReplayService 提供 F-012 历史回放任务的创建、状态、取消、恢复和明细分页。
type HistoryReplayService interface {
	Create(context.Context, uint64, model.HistoryReplayCreateInput, string) (model.HistoryReplayJob, error)
	Active(context.Context, uint64) (model.HistoryReplayJob, bool, error)
	Get(context.Context, uint64, string) (model.HistoryReplayJob, error)
	ListItems(context.Context, uint64, string, uint64, int) (model.HistoryReplayItemPage, error)
	Cancel(context.Context, uint64, string) (model.HistoryReplayJob, error)
	Resume(context.Context, uint64, string) (model.HistoryReplayJob, error)
}

// NewHistoryReplayHandler 创建仅包含 F-012 历史回放任务端点的处理器，供契约测试复用。
func NewHistoryReplayHandler(replay HistoryReplayService) http.Handler {
	mux := http.NewServeMux()
	registerHistoryReplayRoutes(mux, replay)
	return gzipResponses(mux)
}

// registerHistoryReplayRoutes 注册回放任务聚合和明细端点，路径归属始终由服务层核对。
func registerHistoryReplayRoutes(mux *http.ServeMux, replay HistoryReplayService) {
	mux.HandleFunc("POST /api/plans/{id}/history-replays", handleCreateHistoryReplay(replay))
	mux.HandleFunc("GET /api/plans/{id}/history-replays/active", handleActiveHistoryReplay(replay))
	mux.HandleFunc("GET /api/plans/{id}/history-replays/{jobId}", handleGetHistoryReplay(replay))
	mux.HandleFunc("GET /api/plans/{id}/history-replays/{jobId}/items", handleListHistoryReplayItems(replay))
	mux.HandleFunc("POST /api/plans/{id}/history-replays/{jobId}/cancel", handleCancelHistoryReplay(replay))
	mux.HandleFunc("POST /api/plans/{id}/history-replays/{jobId}/resume", handleResumeHistoryReplay(replay))
}

// handleCreateHistoryReplay 只接受明确勾选路径和修订号，目标实例与快照由服务端解析。
func handleCreateHistoryReplay(replay HistoryReplayService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseHistoryPlanID(response, request.PathValue("id"))
		if !ok {
			return
		}
		var input model.HistoryReplayCreateInput
		if !decodeHistoryReplayJSON(response, request, &input, "历史回放请求格式不正确") {
			return
		}
		job, err := replay.Create(request.Context(), planID, input, strings.TrimSpace(request.Header.Get("Idempotency-Key")))
		if err != nil {
			writeHistoryReplayError(response, err)
			return
		}
		writeSuccess(response, job)
	}
}

// handleActiveHistoryReplay 返回刷新后仍处于排队或运行状态的任务。
func handleActiveHistoryReplay(replay HistoryReplayService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseHistoryPlanID(response, request.PathValue("id"))
		if !ok {
			return
		}
		job, found, err := replay.Active(request.Context(), planID)
		if err != nil {
			writeHistoryReplayError(response, err)
			return
		}
		if !found {
			writeSuccess(response, nil)
			return
		}
		writeSuccess(response, job)
	}
}

// handleGetHistoryReplay 返回任务真实聚合计数和可恢复状态。
func handleGetHistoryReplay(replay HistoryReplayService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseHistoryPlanID(response, request.PathValue("id"))
		if !ok {
			return
		}
		job, err := replay.Get(request.Context(), planID, strings.TrimSpace(request.PathValue("jobId")))
		if err != nil {
			writeHistoryReplayError(response, err)
			return
		}
		writeSuccess(response, job)
	}
}

// handleListHistoryReplayItems 返回有界游标分页结果，不把后台租约字段透传给页面。
func handleListHistoryReplayItems(replay HistoryReplayService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseHistoryPlanID(response, request.PathValue("id"))
		if !ok {
			return
		}
		cursor, limit, valid := parseHistoryReplayPagination(response, request)
		if !valid {
			return
		}
		page, err := replay.ListItems(request.Context(), planID, strings.TrimSpace(request.PathValue("jobId")), cursor, limit)
		if err != nil {
			writeHistoryReplayError(response, err)
			return
		}
		writeSuccess(response, page)
	}
}

// handleCancelHistoryReplay 取消任务并保留已经写入的路径结果。
func handleCancelHistoryReplay(replay HistoryReplayService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseHistoryPlanID(response, request.PathValue("id"))
		if !ok {
			return
		}
		job, err := replay.Cancel(request.Context(), planID, strings.TrimSpace(request.PathValue("jobId")))
		if err != nil {
			writeHistoryReplayError(response, err)
			return
		}
		writeSuccess(response, job)
	}
}

// handleResumeHistoryReplay 从未完成检查点恢复任务，不重做已经得到终态的路径。
func handleResumeHistoryReplay(replay HistoryReplayService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseHistoryPlanID(response, request.PathValue("id"))
		if !ok {
			return
		}
		job, err := replay.Resume(request.Context(), planID, strings.TrimSpace(request.PathValue("jobId")))
		if err != nil {
			writeHistoryReplayError(response, err)
			return
		}
		writeSuccess(response, job)
	}
}

// parseHistoryReplayPagination 校验任务明细游标和每页数量，拒绝未知分页参数。
func parseHistoryReplayPagination(response http.ResponseWriter, request *http.Request) (uint64, int, bool) {
	for key := range request.URL.Query() {
		if key != "cursor" && key != "limit" {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "任务明细分页参数不正确", false)
			return 0, 0, false
		}
	}
	var cursor uint64
	limit := 20
	var err error
	if raw := strings.TrimSpace(request.URL.Query().Get("cursor")); raw != "" {
		cursor, err = strconv.ParseUint(raw, 10, 63)
	}
	if err == nil {
		if raw := strings.TrimSpace(request.URL.Query().Get("limit")); raw != "" {
			limit, err = strconv.Atoi(raw)
		}
	}
	if err != nil || limit < 1 || limit > 100 {
		writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "任务明细分页参数不正确", false)
		return 0, 0, false
	}
	return cursor, limit, true
}

// decodeHistoryReplayJSON 严格解析回放请求并拒绝浏览器提交的派生或目标字段。
func decodeHistoryReplayJSON[T any](response http.ResponseWriter, request *http.Request, input *T, message string) bool {
	decoder := jsonvalues.NewDecoder(io.LimitReader(request.Body, maxAPIRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(input); err != nil || ensureJSONEnd(decoder) != nil {
		writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", message, false)
		return false
	}
	return true
}

// writeHistoryReplayError 将回放任务错误映射为稳定 HTTP 错误码。
func writeHistoryReplayError(response http.ResponseWriter, err error) {
	switch {
	case service.IsHistoryReplayErrorKind(err, service.HistoryReplayErrorInvalidArgument):
		writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error(), false)
	case service.IsHistoryReplayErrorKind(err, service.HistoryReplayErrorNotFound):
		writeFailure(response, http.StatusNotFound, "HISTORY_REPLAY_NOT_FOUND", err.Error(), false)
	case service.IsHistoryReplayErrorKind(err, service.HistoryReplayErrorConflict):
		writeFailure(response, http.StatusConflict, "HISTORY_REPLAY_CONFLICT", err.Error(), false)
	case service.IsHistoryReplayErrorKind(err, service.HistoryReplayErrorState):
		writeFailure(response, http.StatusConflict, "HISTORY_REPLAY_STATE_INVALID", err.Error(), false)
	case service.IsHistoryReplayErrorKind(err, service.HistoryReplayErrorTarget):
		writeFailure(response, http.StatusBadGateway, "TARGET_HISTORY_UNAVAILABLE", err.Error(), true)
	default:
		writeFailure(response, http.StatusServiceUnavailable, "HISTORY_REPLAY_STORAGE_UNAVAILABLE", "历史回放存储暂不可用，请重试", true)
	}
}

type unavailableHistoryReplayService struct{}

// Create 在默认处理器未注入回放服务时返回稳定不可用错误。
func (unavailableHistoryReplayService) Create(context.Context, uint64, model.HistoryReplayCreateInput, string) (model.HistoryReplayJob, error) {
	return model.HistoryReplayJob{}, &service.HistoryReplayError{Kind: service.HistoryReplayErrorStorage, Message: "历史回放服务暂不可用"}
}

// Active 在默认处理器未注入回放服务时返回稳定不可用错误。
func (unavailableHistoryReplayService) Active(context.Context, uint64) (model.HistoryReplayJob, bool, error) {
	return model.HistoryReplayJob{}, false, &service.HistoryReplayError{Kind: service.HistoryReplayErrorStorage, Message: "历史回放服务暂不可用"}
}

// Get 在默认处理器未注入回放服务时返回稳定不可用错误。
func (unavailableHistoryReplayService) Get(context.Context, uint64, string) (model.HistoryReplayJob, error) {
	return model.HistoryReplayJob{}, &service.HistoryReplayError{Kind: service.HistoryReplayErrorStorage, Message: "历史回放服务暂不可用"}
}

// ListItems 在默认处理器未注入回放服务时返回稳定不可用错误。
func (unavailableHistoryReplayService) ListItems(context.Context, uint64, string, uint64, int) (model.HistoryReplayItemPage, error) {
	return model.HistoryReplayItemPage{}, &service.HistoryReplayError{Kind: service.HistoryReplayErrorStorage, Message: "历史回放服务暂不可用"}
}

// Cancel 在默认处理器未注入回放服务时返回稳定不可用错误。
func (unavailableHistoryReplayService) Cancel(context.Context, uint64, string) (model.HistoryReplayJob, error) {
	return model.HistoryReplayJob{}, &service.HistoryReplayError{Kind: service.HistoryReplayErrorStorage, Message: "历史回放服务暂不可用"}
}

// Resume 在默认处理器未注入回放服务时返回稳定不可用错误。
func (unavailableHistoryReplayService) Resume(context.Context, uint64, string) (model.HistoryReplayJob, error) {
	return model.HistoryReplayJob{}, &service.HistoryReplayError{Kind: service.HistoryReplayErrorStorage, Message: "历史回放服务暂不可用"}
}
