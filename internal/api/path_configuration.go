package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// PathConfigurationService 提供单条路径配置的读取与整份保存能力。
type PathConfigurationService interface {
	Get(context.Context, uint64, uint64) (model.PathConfiguration, error)
	Save(context.Context, uint64, uint64, string, uint64, []model.PathConfigFieldValue, []model.PathConfigActionValue) (model.PathConfigSaveResult, error)
}

// pathConfigurationUpdate 是浏览器最小可信回写体，只包含修订号和不透明键值。
type pathConfigurationUpdate struct {
	Revision uint64                        `json:"revision"`
	Fields   []model.PathConfigFieldValue  `json:"fields"`
	Actions  []model.PathConfigActionValue `json:"actions"`
}

// registerPathConfigurationRoutes 注册同一计划下单条路径的配置读取与保存端点。
func registerPathConfigurationRoutes(mux *http.ServeMux, configurations PathConfigurationService) {
	mux.HandleFunc("GET /api/plans/{id}/execution-paths/{pathId}/configuration", handleGetPathConfiguration(configurations))
	mux.HandleFunc("PUT /api/plans/{id}/execution-paths/{pathId}/configuration", handleSavePathConfiguration(configurations))
}

// handleGetPathConfiguration 返回当前路径的可操作配置工作台模型。
func handleGetPathConfiguration(configurations PathConfigurationService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseExecutionPathID(response, request.PathValue("id"))
		if !ok {
			return
		}
		pathID, ok := parseExecutionPathID(response, request.PathValue("pathId"))
		if !ok {
			return
		}
		configuration, err := configurations.Get(request.Context(), planID, pathID)
		if err != nil {
			writePathConfigError(response, err)
			return
		}
		writeSuccess(response, configuration)
	}
}

// handleSavePathConfiguration 校验修订号与幂等键后整份保存字段值和动作值。
func handleSavePathConfiguration(configurations PathConfigurationService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseExecutionPathID(response, request.PathValue("id"))
		if !ok {
			return
		}
		pathID, ok := parseExecutionPathID(response, request.PathValue("pathId"))
		if !ok {
			return
		}
		var input pathConfigurationUpdate
		decoder := json.NewDecoder(io.LimitReader(request.Body, maxAPIRequestBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || ensureJSONEnd(decoder) != nil {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "路径配置请求格式不正确", false)
			return
		}
		result, err := configurations.Save(request.Context(), planID, pathID, strings.TrimSpace(request.Header.Get("Idempotency-Key")), input.Revision, input.Fields, input.Actions)
		if err != nil {
			writePathConfigError(response, err)
			return
		}
		writeSuccess(response, result)
	}
}

// writePathConfigError 把路径配置错误映射为批准的稳定契约。
func writePathConfigError(response http.ResponseWriter, err error) {
	switch {
	case service.IsPathConfigErrorKind(err, service.PathConfigErrorInvalidArgument):
		writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error(), false)
	case service.IsPathConfigErrorKind(err, service.PathConfigErrorNotFound):
		writeFailure(response, http.StatusNotFound, "EXECUTION_PATH_NOT_FOUND", "执行路径不存在", false)
	case service.IsPathConfigErrorKind(err, service.PathConfigErrorLocked):
		writeFailure(response, http.StatusConflict, "PLAN_LOCKED", "计划已经不能修改路径配置", false)
	case service.IsPathConfigErrorKind(err, service.PathConfigErrorRevisionConflict):
		writeFailure(response, http.StatusConflict, "CONFIG_REVISION_CONFLICT", "配置已被其他操作更新，请刷新后重试", false)
	case service.IsPathConfigErrorKind(err, service.PathConfigErrorInvalid):
		var configErr *service.PathConfigError
		if !errors.As(err, &configErr) {
			configErr = &service.PathConfigError{}
		}
		writeJSON(response, http.StatusConflict, apiFailure{Success: false, Error: apiErrorDTO{
			Code: "CONFIG_INVALID", Message: err.Error(), Retryable: false,
			Details: nonNilSlice(configErr.Affected),
		}})
	case service.IsPathConfigErrorKind(err, service.PathConfigErrorStorage):
		writeFailure(response, http.StatusServiceUnavailable, "PLAN_STORAGE_UNAVAILABLE", "路径配置存储暂不可用，请重试", true)
	default:
		writeFlowGraphError(response, err)
	}
}

type unavailablePathConfigurationService struct{}

// Get 在未注入配置服务时返回稳定存储不可用错误。
func (unavailablePathConfigurationService) Get(context.Context, uint64, uint64) (model.PathConfiguration, error) {
	return model.PathConfiguration{}, &service.PathConfigError{Kind: service.PathConfigErrorStorage, Message: "路径配置服务暂不可用"}
}

// Save 在未注入配置服务时拒绝保存。
func (unavailablePathConfigurationService) Save(context.Context, uint64, uint64, string, uint64, []model.PathConfigFieldValue, []model.PathConfigActionValue) (model.PathConfigSaveResult, error) {
	return model.PathConfigSaveResult{}, &service.PathConfigError{Kind: service.PathConfigErrorStorage, Message: "路径配置服务暂不可用"}
}
