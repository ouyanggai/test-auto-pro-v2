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

// PathConfigurationService 提供单条路径的 F-008 节点、循环、选择和表单配置能力。
type PathConfigurationService interface {
	Get(context.Context, uint64, uint64) (model.PathConfiguration, error)
	SaveNode(context.Context, uint64, uint64, string, string, model.PathNodeSaveInput) (model.PathConfigSaveResult, error)
	SaveSelection(context.Context, uint64, uint64, string, model.PathConfigSelectionInput) (model.PathConfigSaveResult, error)
	CopyCycles(context.Context, uint64, uint64, uint64, string) (model.PathConfigSaveResult, error)
	GenerateForm(context.Context, uint64, uint64, int64, map[string]any, []string, bool) (model.PathFormGenerateResult, error)
	SaveForm(context.Context, uint64, uint64, string, model.PathFormSaveInput) (model.PathConfigSaveResult, error)
	RuntimeSession(context.Context, uint64, uint64) (model.PathFormRuntimeSession, error)
}

type pathFormGenerateInput struct {
	Seed                int64          `json:"seed"`
	Values              map[string]any `json:"values"`
	ManualOverridePaths []string       `json:"manualOverridePaths"`
	NextGroup           bool           `json:"nextGroup"`
}

// registerPathConfigurationRoutes 注册同一计划下单条路径的配置读取与保存端点。
func registerPathConfigurationRoutes(mux *http.ServeMux, configurations PathConfigurationService) {
	mux.HandleFunc("GET /api/plans/{id}/execution-paths/{pathId}/configuration", handleGetPathConfiguration(configurations))
	mux.HandleFunc("PUT /api/plans/{id}/execution-paths/{pathId}/configuration/nodes/{nodeKey}", handleSavePathConfigurationNode(configurations))
	mux.HandleFunc("PUT /api/plans/{id}/execution-paths/{pathId}/configuration/selection", handleSavePathConfigurationSelection(configurations))
	mux.HandleFunc("POST /api/plans/{id}/execution-paths/{pathId}/configuration/cycles/copy", handleCopyPathConfigurationCycles(configurations))
	mux.HandleFunc("POST /api/plans/{id}/execution-paths/{pathId}/configuration/form/generate", handleGeneratePathConfigurationForm(configurations))
	mux.HandleFunc("PUT /api/plans/{id}/execution-paths/{pathId}/configuration/form", handleSavePathConfigurationForm(configurations))
	mux.HandleFunc("GET /api/plans/{id}/execution-paths/{pathId}/configuration/runtime-session", handlePathConfigurationRuntimeSession(configurations))
}

// handleCopyPathConfigurationCycles 只复制工具侧循环配置，并要求目标路径与来源路径结构签名完全一致。
func handleCopyPathConfigurationCycles(configurations PathConfigurationService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, targetPathID, ok := parsePathConfigurationIDs(response, request)
		if !ok {
			return
		}
		var input model.PathConfigCycleCopyInput
		decoder := json.NewDecoder(io.LimitReader(request.Body, maxAPIRequestBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || ensureJSONEnd(decoder) != nil {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "循环复制请求格式不正确", false)
			return
		}
		result, err := configurations.CopyCycles(request.Context(), planID, targetPathID, input.SourcePathID, strings.TrimSpace(request.Header.Get("Idempotency-Key")))
		if err != nil {
			writePathConfigError(response, err)
			return
		}
		writeSuccess(response, result)
	}
}

// handleSavePathConfigurationSelection 只保存后续运行范围选择，不触发节点或目标平台写操作。
func handleSavePathConfigurationSelection(configurations PathConfigurationService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, pathID, ok := parsePathConfigurationIDs(response, request)
		if !ok {
			return
		}
		var input model.PathConfigSelectionInput
		decoder := json.NewDecoder(io.LimitReader(request.Body, maxAPIRequestBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || ensureJSONEnd(decoder) != nil {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "本次测试路径请求格式不正确", false)
			return
		}
		result, err := configurations.SaveSelection(request.Context(), planID, pathID, strings.TrimSpace(request.Header.Get("Idempotency-Key")), input)
		if err != nil {
			writePathConfigError(response, err)
			return
		}
		writeSuccess(response, result)
	}
}

// parsePathConfigurationIDs 统一解析计划与路径 ID，所有分域端点沿用相同归属边界。
func parsePathConfigurationIDs(response http.ResponseWriter, request *http.Request) (uint64, uint64, bool) {
	planID, ok := parseExecutionPathID(response, request.PathValue("id"))
	if !ok {
		return 0, 0, false
	}
	pathID, ok := parseExecutionPathID(response, request.PathValue("pathId"))
	return planID, pathID, ok
}

// handleSavePathConfigurationNode 保存当前节点人员与动作，不覆盖其他节点或表单 values。
func handleSavePathConfigurationNode(configurations PathConfigurationService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, pathID, ok := parsePathConfigurationIDs(response, request)
		if !ok {
			return
		}
		var input model.PathNodeSaveInput
		decoder := json.NewDecoder(io.LimitReader(request.Body, maxAPIRequestBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || ensureJSONEnd(decoder) != nil {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "节点配置请求格式不正确", false)
			return
		}
		result, err := configurations.SaveNode(
			request.Context(), planID, pathID, strings.TrimSpace(request.PathValue("nodeKey")),
			strings.TrimSpace(request.Header.Get("Idempotency-Key")), input,
		)
		if err != nil {
			writePathConfigError(response, err)
			return
		}
		writeSuccess(response, result)
	}
}

// handleGeneratePathConfigurationForm 生成或换一组表单草稿，不写数据库或目标平台。
func handleGeneratePathConfigurationForm(configurations PathConfigurationService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, pathID, ok := parsePathConfigurationIDs(response, request)
		if !ok {
			return
		}
		var input pathFormGenerateInput
		decoder := json.NewDecoder(io.LimitReader(request.Body, maxAPIRequestBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || ensureJSONEnd(decoder) != nil {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "智能生成请求格式不正确", false)
			return
		}
		result, err := configurations.GenerateForm(request.Context(), planID, pathID, input.Seed, input.Values, input.ManualOverridePaths, input.NextGroup)
		if err != nil {
			writePathConfigError(response, err)
			return
		}
		writeSuccess(response, result)
	}
}

// handleSavePathConfigurationForm 独立保存真实 getValues 结果和生成元数据。
func handleSavePathConfigurationForm(configurations PathConfigurationService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, pathID, ok := parsePathConfigurationIDs(response, request)
		if !ok {
			return
		}
		var input model.PathFormSaveInput
		decoder := json.NewDecoder(io.LimitReader(request.Body, maxAPIRequestBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || ensureJSONEnd(decoder) != nil {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "表单数据请求格式不正确", false)
			return
		}
		result, err := configurations.SaveForm(request.Context(), planID, pathID, strings.TrimSpace(request.Header.Get("Idempotency-Key")), input)
		if err != nil {
			writePathConfigError(response, err)
			return
		}
		writeSuccess(response, result)
	}
}

// handlePathConfigurationRuntimeSession 返回当前账号的短期 SID 会话，iframe 销毁后由前端清空。
func handlePathConfigurationRuntimeSession(configurations PathConfigurationService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, pathID, ok := parsePathConfigurationIDs(response, request)
		if !ok {
			return
		}
		result, err := configurations.RuntimeSession(request.Context(), planID, pathID)
		if err != nil {
			writePathConfigError(response, err)
			return
		}
		writeSuccess(response, result)
	}
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
	case service.IsExecutionPathErrorKind(err, service.ExecutionPathErrorNotFound):
		writeFailure(response, http.StatusNotFound, "EXECUTION_PATH_NOT_FOUND", "执行路径不存在", false)
	case service.IsExecutionPathErrorKind(err, service.ExecutionPathErrorLocked):
		writeFailure(response, http.StatusConflict, "PLAN_LOCKED", "计划已经不能修改路径配置", false)
	case service.IsExecutionPathErrorKind(err, service.ExecutionPathErrorStorage):
		writeFailure(response, http.StatusServiceUnavailable, "PLAN_STORAGE_UNAVAILABLE", "路径存储暂不可用，请重试", true)
	default:
		writeFlowGraphError(response, err)
	}
}

type unavailablePathConfigurationService struct{}

// Get 在未注入配置服务时返回稳定存储不可用错误。
func (unavailablePathConfigurationService) Get(context.Context, uint64, uint64) (model.PathConfiguration, error) {
	return model.PathConfiguration{}, &service.PathConfigError{Kind: service.PathConfigErrorStorage, Message: "路径配置服务暂不可用"}
}

// SaveNode 在未注入配置服务时拒绝节点保存。
func (unavailablePathConfigurationService) SaveNode(context.Context, uint64, uint64, string, string, model.PathNodeSaveInput) (model.PathConfigSaveResult, error) {
	return model.PathConfigSaveResult{}, &service.PathConfigError{Kind: service.PathConfigErrorStorage, Message: "路径配置服务暂不可用"}
}

// SaveSelection 在未注入配置服务时拒绝路径选择保存。
func (unavailablePathConfigurationService) SaveSelection(context.Context, uint64, uint64, string, model.PathConfigSelectionInput) (model.PathConfigSaveResult, error) {
	return model.PathConfigSaveResult{}, &service.PathConfigError{Kind: service.PathConfigErrorStorage, Message: "路径配置服务暂不可用"}
}

// CopyCycles 在未注入配置服务时拒绝循环复制。
func (unavailablePathConfigurationService) CopyCycles(context.Context, uint64, uint64, uint64, string) (model.PathConfigSaveResult, error) {
	return model.PathConfigSaveResult{}, &service.PathConfigError{Kind: service.PathConfigErrorStorage, Message: "路径配置服务暂不可用"}
}

// GenerateForm 在未注入配置服务时拒绝智能生成。
func (unavailablePathConfigurationService) GenerateForm(context.Context, uint64, uint64, int64, map[string]any, []string, bool) (model.PathFormGenerateResult, error) {
	return model.PathFormGenerateResult{}, &service.PathConfigError{Kind: service.PathConfigErrorStorage, Message: "表单生成服务暂不可用"}
}

// SaveForm 在未注入配置服务时拒绝表单保存。
func (unavailablePathConfigurationService) SaveForm(context.Context, uint64, uint64, string, model.PathFormSaveInput) (model.PathConfigSaveResult, error) {
	return model.PathConfigSaveResult{}, &service.PathConfigError{Kind: service.PathConfigErrorStorage, Message: "表单配置服务暂不可用"}
}

// RuntimeSession 在未注入配置服务时拒绝发放 SID 会话。
func (unavailablePathConfigurationService) RuntimeSession(context.Context, uint64, uint64) (model.PathFormRuntimeSession, error) {
	return model.PathFormRuntimeSession{}, &service.PathConfigError{Kind: service.PathConfigErrorStorage, Message: "表单运行时会话暂不可用"}
}
