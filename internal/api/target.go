package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/config"
)

const maxAPIRequestBytes = 1 << 20

type TargetReader interface {
	Verify(context.Context, string) (target.AccountSummary, error)
	Templates(context.Context, string, string, int, int) (target.Page[target.FlowTemplate], error)
	Submitted(context.Context, string, string, int, int) (target.Page[target.SubmittedFlow], error)
	Due(context.Context, string, string, int, int) (target.Page[target.DueFlow], error)
}

type apiSuccess struct {
	Success bool `json:"success"`
	Data    any  `json:"data"`
}

type apiFailure struct {
	Success bool        `json:"success"`
	Error   apiErrorDTO `json:"error"`
}

type apiErrorDTO struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type verifyRequest struct {
	Account string `json:"account"`
}

type verifyResponse struct {
	Verified bool                  `json:"verified"`
	Account  target.AccountSummary `json:"account"`
}

type templatePageResponse struct {
	Account  string                `json:"account"`
	Items    []target.FlowTemplate `json:"items"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"pageSize"`
	Total    int                   `json:"total"`
	HasMore  bool                  `json:"hasMore"`
}

type submittedPageResponse struct {
	Account  string                 `json:"account"`
	Source   string                 `json:"source"`
	Items    []target.SubmittedFlow `json:"items"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"pageSize"`
	Total    int                    `json:"total"`
	HasMore  bool                   `json:"hasMore"`
}

type duePageResponse struct {
	Account  string           `json:"account"`
	Source   string           `json:"source"`
	Items    []target.DueFlow `json:"items"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
	Total    int              `json:"total"`
	HasMore  bool             `json:"hasMore"`
}

func registerTargetRoutes(mux *http.ServeMux, reader TargetReader) {
	mux.HandleFunc("POST /api/target/accounts/verify", handleVerifyAccount(reader))
	mux.HandleFunc("GET /api/target/flow-templates", handleFlowTemplates(reader))
	mux.HandleFunc("GET /api/target/flow-instances", handleFlowInstances(reader))
}

func handleVerifyAccount(reader TargetReader) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		var input verifyRequest
		decoder := json.NewDecoder(io.LimitReader(request.Body, maxAPIRequestBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "账号验证请求格式不正确", false)
			return
		}
		account := strings.TrimSpace(input.Account)
		if account == "" {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "请输入真实账号", false)
			return
		}
		summary, err := reader.Verify(request.Context(), account)
		if err != nil {
			writeTargetError(response, err)
			return
		}
		writeSuccess(response, verifyResponse{Verified: true, Account: summary})
	}
}

func handleFlowTemplates(reader TargetReader) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		account := strings.TrimSpace(request.URL.Query().Get("account"))
		if account == "" {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "缺少真实账号", false)
			return
		}
		page, pageSize, ok := parsePagination(response, request)
		if !ok {
			return
		}
		result, err := reader.Templates(request.Context(), account, request.URL.Query().Get("query"), page, pageSize)
		if err != nil {
			writeTargetError(response, err)
			return
		}
		writeSuccess(response, templatePageResponse{
			Account: account, Items: nonNilSlice(result.Items), Page: result.Page,
			PageSize: result.PageSize, Total: result.Total, HasMore: result.HasMore,
		})
	}
}

func handleFlowInstances(reader TargetReader) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		account := strings.TrimSpace(request.URL.Query().Get("account"))
		if account == "" {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "缺少真实账号", false)
			return
		}
		source := strings.TrimSpace(request.URL.Query().Get("source"))
		if source != "submitted" && source != "due" {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "流程实例来源必须是 submitted 或 due", false)
			return
		}
		page, pageSize, ok := parsePagination(response, request)
		if !ok {
			return
		}
		query := request.URL.Query().Get("query")
		if source == "submitted" {
			result, err := reader.Submitted(request.Context(), account, query, page, pageSize)
			if err != nil {
				writeTargetError(response, err)
				return
			}
			writeSuccess(response, submittedPageResponse{
				Account: account, Source: source, Items: nonNilSlice(result.Items), Page: result.Page,
				PageSize: result.PageSize, Total: result.Total, HasMore: result.HasMore,
			})
			return
		}
		result, err := reader.Due(request.Context(), account, query, page, pageSize)
		if err != nil {
			writeTargetError(response, err)
			return
		}
		writeSuccess(response, duePageResponse{
			Account: account, Source: source, Items: nonNilSlice(result.Items), Page: result.Page,
			PageSize: result.PageSize, Total: result.Total, HasMore: result.HasMore,
		})
	}
}

func parsePagination(response http.ResponseWriter, request *http.Request) (int, int, bool) {
	page, ok := parsePositiveInt(request.URL.Query().Get("page"), 1)
	if !ok {
		writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "page 必须是正整数", false)
		return 0, 0, false
	}
	pageSize, ok := parsePositiveInt(request.URL.Query().Get("pageSize"), 20)
	if !ok || pageSize > 100 {
		writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "pageSize 必须是 1 至 100 的整数", false)
		return 0, 0, false
	}
	return page, pageSize, true
}

func parsePositiveInt(value string, fallback int) (int, bool) {
	if strings.TrimSpace(value) == "" {
		return fallback, true
	}
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil && parsed > 0
}

func writeTargetError(response http.ResponseWriter, err error) {
	var configErr *config.MissingTargetConfigError
	if errors.As(err, &configErr) {
		writeFailure(response, http.StatusServiceUnavailable, "TARGET_CONFIG_MISSING", "目标环境尚未配置完整", false)
		return
	}
	switch {
	case target.IsKind(err, target.ErrorLoginRejected):
		writeFailure(response, http.StatusUnauthorized, "TARGET_LOGIN_REJECTED", "目标平台拒绝登录，请核对账号", false)
	case target.IsKind(err, target.ErrorSessionExpired):
		writeFailure(response, http.StatusUnauthorized, "TARGET_SESSION_EXPIRED", "目标平台会话已失效，请重新验证账号", true)
	case target.IsKind(err, target.ErrorResponseInvalid):
		writeFailure(response, http.StatusBadGateway, "TARGET_RESPONSE_INVALID", "目标平台返回的数据格式异常", true)
	case target.IsKind(err, target.ErrorTimeout):
		writeFailure(response, http.StatusGatewayTimeout, "TARGET_TIMEOUT", "目标平台响应超时，请重试", true)
	default:
		writeFailure(response, http.StatusBadGateway, "TARGET_UNAVAILABLE", "暂时无法连接目标平台，请重试", true)
	}
}

func writeSuccess(response http.ResponseWriter, data any) {
	writeJSON(response, http.StatusOK, apiSuccess{Success: true, Data: data})
}

func writeFailure(response http.ResponseWriter, status int, code, message string, retryable bool) {
	writeJSON(response, status, apiFailure{Success: false, Error: apiErrorDTO{Code: code, Message: message, Retryable: retryable}})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func nonNilSlice[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}
