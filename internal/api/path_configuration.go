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

// PathConfigurationDataService 提供 F-012 原始表单数据工作区读写，不接受生成器元数据。
type PathConfigurationDataService interface {
	GetData(context.Context, uint64, uint64) (model.PathConfigurationF012, error)
	SaveData(context.Context, uint64, uint64, string, model.PathConfigurationDataInput) (model.PathConfigurationDataResult, error)
}

// PathActionConfigurationService 提供 F-012 语义动作保存和同实例只读场景预览。
type PathActionConfigurationService interface {
	GetActionConfiguration(context.Context, uint64, uint64) (model.ActionConfigurationResult, error)
	GetCompiledScenario(context.Context, uint64, uint64) (model.ActionConfigurationResult, error)
	SaveActionConfiguration(context.Context, uint64, uint64, string, string, model.ActionConfigurationInput) (model.ActionConfigurationResult, error)
}

// PathRunInputPreflightService 提供 P4 只读运行输入快照和目标适配预检。
type PathRunInputPreflightService interface {
	PreflightRunInput(context.Context, uint64, uint64) (model.RunInputPreflightResult, error)
}

type pathFormGenerateInput struct {
	Seed                int64          `json:"seed"`
	Values              map[string]any `json:"values"`
	ManualOverridePaths []string       `json:"manualOverridePaths"`
	NextGroup           bool           `json:"nextGroup"`
}

// registerPathConfigurationRoutes 注册同一计划下单条路径的配置读取与保存端点。
// F-012 数据工作区必须由调用方显式注入；未注入时不注册兼容或兜底路由。
func registerPathConfigurationRoutes(mux *http.ServeMux, configurations PathConfigurationService, dataServices PathConfigurationDataService) {
	mux.HandleFunc("GET /api/plans/{id}/execution-paths/{pathId}/configuration", handleGetPathConfiguration(configurations))
	if actions, ok := configurations.(PathActionConfigurationService); ok {
		// F-012 节点保存端点只接收语义动作；未注入新服务时不注册旧接口或兼容路由。
		mux.HandleFunc("PUT /api/plans/{id}/execution-paths/{pathId}/configuration/nodes/{nodeKey}", handleSaveActionConfiguration(actions))
		mux.HandleFunc("GET /api/plans/{id}/execution-paths/{pathId}/configuration/compiled-scenario", handleGetCompiledScenario(actions))
	}
	mux.HandleFunc("PUT /api/plans/{id}/execution-paths/{pathId}/configuration/selection", handleSavePathConfigurationSelection(configurations))
	mux.HandleFunc("POST /api/plans/{id}/execution-paths/{pathId}/configuration/cycles/copy", handleCopyPathConfigurationCycles(configurations))
	mux.HandleFunc("POST /api/plans/{id}/execution-paths/{pathId}/configuration/form/generate", handleGeneratePathConfigurationForm(configurations))
	mux.HandleFunc("PUT /api/plans/{id}/execution-paths/{pathId}/configuration/form", handleSavePathConfigurationForm(configurations))
	mux.HandleFunc("GET /api/plans/{id}/execution-paths/{pathId}/configuration/runtime-session", handlePathConfigurationRuntimeSession(configurations))
	if dataServices != nil {
		mux.HandleFunc("GET /api/plans/{id}/execution-paths/{pathId}/configuration/data", handleGetPathConfigurationData(dataServices))
		mux.HandleFunc("PUT /api/plans/{id}/execution-paths/{pathId}/configuration/data", handleSavePathConfigurationData(dataServices))
	}
	preflights, ok := configurations.(PathRunInputPreflightService)
	if !ok {
		preflights = unavailablePathRunInputPreflightService{}
	}
	mux.HandleFunc("GET /api/plans/{id}/execution-paths/{pathId}/run-input/preflight", handlePathRunInputPreflight(preflights))
}

// handleGetCompiledScenario 返回服务端重编译的动作场景，不接受浏览器提交的步骤正文。
func handleGetCompiledScenario(actions PathActionConfigurationService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, pathID, ok := parsePathConfigurationIDs(response, request)
		if !ok {
			return
		}
		result, err := actions.GetCompiledScenario(request.Context(), planID, pathID)
		if err != nil {
			writePathConfigError(response, err)
			return
		}
		writeSuccess(response, result)
	}
}

// handleSaveActionConfiguration 保存当前语义节点动作并要求服务端重新编译完整主实例场景。
func handleSaveActionConfiguration(actions PathActionConfigurationService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, pathID, ok := parsePathConfigurationIDs(response, request)
		if !ok {
			return
		}
		var input model.ActionConfigurationInput
		decoder := json.NewDecoder(io.LimitReader(request.Body, maxAPIRequestBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || ensureJSONEnd(decoder) != nil {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "动作配置请求格式不正确", false)
			return
		}
		result, err := actions.SaveActionConfiguration(
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

// handleGetPathConfigurationData 返回目标原始表单数据和复制 runtime 的加载协议。
func handleGetPathConfigurationData(dataServices PathConfigurationDataService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, pathID, ok := parsePathConfigurationIDs(response, request)
		if !ok {
			return
		}
		result, err := dataServices.GetData(request.Context(), planID, pathID)
		if err != nil {
			writePathConfigError(response, err)
			return
		}
		writeSuccess(response, result)
	}
}

// handleSavePathConfigurationData 严格接收 runtime 捕获的原始 values，并在换路时返回确认令牌而不写入。
func handleSavePathConfigurationData(dataServices PathConfigurationDataService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, pathID, ok := parsePathConfigurationIDs(response, request)
		if !ok {
			return
		}
		var input model.PathConfigurationDataInput
		decoder := json.NewDecoder(io.LimitReader(request.Body, maxAPIRequestBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || ensureJSONEnd(decoder) != nil {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "表单数据请求格式不正确", false)
			return
		}
		result, err := dataServices.SaveData(request.Context(), planID, pathID, strings.TrimSpace(request.Header.Get("Idempotency-Key")), input)
		if err != nil {
			writePathConfigError(response, err)
			return
		}
		writeSuccess(response, result)
	}
}

// handlePathRunInputPreflight 返回不可变输入和目标请求摘要，不创建运行记录或发送目标请求。
func handlePathRunInputPreflight(preflights PathRunInputPreflightService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, pathID, ok := parsePathConfigurationIDs(response, request)
		if !ok {
			return
		}
		result, err := preflights.PreflightRunInput(request.Context(), planID, pathID)
		if err != nil {
			writePathConfigError(response, err)
			return
		}
		writeSuccess(response, result)
	}
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
	case service.IsPathConfigErrorKind(err, service.PathConfigErrorRouteConfirmation):
		var configErr *service.PathConfigError
		if !errors.As(err, &configErr) {
			configErr = &service.PathConfigError{}
		}
		writeJSON(response, http.StatusConflict, apiFailure{Success: false, Error: apiErrorDTO{
			Code: "PATH_ROUTE_CONFIRMATION_REQUIRED", Message: err.Error(), Retryable: false,
			Details: map[string]any{"routeChange": configErr.RouteChange, "confirmationToken": configErr.ConfirmationToken},
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

type unavailablePathRunInputPreflightService struct{}

// PreflightRunInput 在未注入预检服务时返回稳定存储不可用错误。
func (unavailablePathRunInputPreflightService) PreflightRunInput(context.Context, uint64, uint64) (model.RunInputPreflightResult, error) {
	return model.RunInputPreflightResult{}, &service.PathConfigError{Kind: service.PathConfigErrorStorage, Message: "运行输入预检服务暂不可用"}
}

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
