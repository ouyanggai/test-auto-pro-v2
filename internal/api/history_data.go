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

// HistoryDataService 提供历史候选、计划默认来源和路径来源配置接口。
type HistoryDataService interface {
	Candidates(context.Context, uint64, uint64, string, int, int) (model.HistoryCandidatePage, error)
	SaveDefault(context.Context, uint64, model.HistoryDefaultSaveInput, string) (model.HistoryDataSource, error)
	SavePathSource(context.Context, uint64, uint64, model.HistoryPathSourceInput, string) (model.HistoryDataSource, error)
}

// NewHistoryDataHandler 创建仅包含 F-012 历史候选与来源配置端点的处理器，供定向契约验证复用。
func NewHistoryDataHandler(history HistoryDataService) http.Handler {
	mux := http.NewServeMux()
	registerHistoryDataRoutes(mux, history)
	return gzipResponses(mux)
}

// registerHistoryDataRoutes 注册 F-012 历史数据只读候选与来源写入端点。
func registerHistoryDataRoutes(mux *http.ServeMux, history HistoryDataService) {
	mux.HandleFunc("GET /api/plans/{id}/history-data/candidates", handleHistoryCandidates(history))
	mux.HandleFunc("PUT /api/plans/{id}/history-data/default", handleSaveHistoryDefault(history))
	mux.HandleFunc("PUT /api/plans/{id}/execution-paths/{pathId}/configuration/data/source", handleSaveHistoryPathSource(history))
}

// handleHistoryCandidates 返回目标原始历史实例摘要，不包含完整表单数据或目标内部标识。
func handleHistoryCandidates(history HistoryDataService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if !validateHistoryCandidateQuery(response, request) {
			return
		}
		planID, ok := parseHistoryPlanID(response, request.PathValue("id"))
		if !ok {
			return
		}
		page, pageSize, ok := parsePagination(response, request)
		if !ok {
			return
		}
		pathID, ok := parseOptionalHistoryPathID(response, request.URL.Query().Get("pathId"))
		if !ok {
			return
		}
		result, err := history.Candidates(request.Context(), planID, pathID, request.URL.Query().Get("query"), page, pageSize)
		if err != nil {
			writeHistoryDataError(response, err)
			return
		}
		writeSuccess(response, result)
	}
}

// validateHistoryCandidateQuery 拒绝账号、目标 ID 或其他未声明查询参数，身份只能由计划服务端解析。
func validateHistoryCandidateQuery(response http.ResponseWriter, request *http.Request) bool {
	allowed := map[string]bool{"pathId": true, "query": true, "page": true, "pageSize": true}
	for key := range request.URL.Query() {
		if !allowed[key] {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "业务数据查询参数不正确", false)
			return false
		}
	}
	return true
}

// parseOptionalHistoryPathID 解析候选弹窗可选路径上下文，零值表示计划默认来源场景。
func parseOptionalHistoryPathID(response http.ResponseWriter, raw string) (uint64, bool) {
	if strings.TrimSpace(raw) == "" {
		return 0, true
	}
	return parseHistoryPlanID(response, raw)
}

// handleSaveHistoryDefault 保存计划级默认候选，并要求请求头携带幂等键。
func handleSaveHistoryDefault(history HistoryDataService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseHistoryPlanID(response, request.PathValue("id"))
		if !ok {
			return
		}
		var input model.HistoryDefaultSaveInput
		if !decodeHistoryJSON(response, request, &input, "计划默认基础表单数据请求格式不正确") {
			return
		}
		result, err := history.SaveDefault(request.Context(), planID, input, strings.TrimSpace(request.Header.Get("Idempotency-Key")))
		if err != nil {
			writeHistoryDataError(response, err)
			return
		}
		writeSuccess(response, result)
	}
}

// handleSaveHistoryPathSource 保存路径继承或独立来源，不接受浏览器提交的账号和目标内部 ID。
func handleSaveHistoryPathSource(history HistoryDataService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseHistoryPlanID(response, request.PathValue("id"))
		if !ok {
			return
		}
		pathID, ok := parseHistoryPlanID(response, request.PathValue("pathId"))
		if !ok {
			return
		}
		var input model.HistoryPathSourceInput
		if !decodeHistoryJSON(response, request, &input, "路径基础表单数据请求格式不正确") {
			return
		}
		result, err := history.SavePathSource(request.Context(), planID, pathID, input, strings.TrimSpace(request.Header.Get("Idempotency-Key")))
		if err != nil {
			writeHistoryDataError(response, err)
			return
		}
		writeSuccess(response, result)
	}
}

// parseHistoryPlanID 统一校验历史接口中的计划或路径主键。
func parseHistoryPlanID(response http.ResponseWriter, raw string) (uint64, bool) {
	id, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil || id == 0 {
		writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "计划或路径 ID 不正确", false)
		return 0, false
	}
	return id, true
}

// decodeHistoryJSON 严格解析一个历史来源对象并拒绝未知字段或尾随 JSON。
func decodeHistoryJSON[T any](response http.ResponseWriter, request *http.Request, input *T, message string) bool {
	decoder := jsonvalues.NewDecoder(io.LimitReader(request.Body, maxAPIRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(input); err != nil || ensureJSONEnd(decoder) != nil {
		writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", message, false)
		return false
	}
	return true
}

// writeHistoryDataError 将历史服务错误映射为稳定公开契约。
func writeHistoryDataError(response http.ResponseWriter, err error) {
	switch {
	case service.IsHistoryDataErrorKind(err, service.HistoryDataErrorInvalidArgument):
		writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error(), false)
	case service.IsHistoryDataErrorKind(err, service.HistoryDataErrorNotFound):
		writeFailure(response, http.StatusNotFound, "HISTORY_NOT_FOUND", err.Error(), false)
	case service.IsHistoryDataErrorKind(err, service.HistoryDataErrorConflict):
		writeFailure(response, http.StatusConflict, "HISTORY_REVISION_CONFLICT", err.Error(), false)
	case service.IsHistoryDataErrorKind(err, service.HistoryDataErrorTarget):
		writeFailure(response, http.StatusBadGateway, "TARGET_HISTORY_UNAVAILABLE", err.Error(), true)
	default:
		writeFailure(response, http.StatusServiceUnavailable, "HISTORY_STORAGE_UNAVAILABLE", "业务数据存储暂不可用，请重试", true)
	}
}
