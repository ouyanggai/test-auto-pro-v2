package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

type ExecutionPathService interface {
	List(context.Context, uint64) ([]model.ExecutionPath, error)
	Create(context.Context, uint64, string, string, []model.ExecutionPathChoice) (model.ExecutionPath, bool, error)
	Update(context.Context, uint64, uint64, string, []model.ExecutionPathChoice) (model.ExecutionPath, error)
	Delete(context.Context, uint64, uint64) error
	GenerateAll(context.Context, uint64, string) (model.ExecutionPathBatchResult, bool, error)
}

type executionPathRequest struct {
	Name    string                      `json:"name"`
	Choices []model.ExecutionPathChoice `json:"choices"`
}

type executionPathResponse struct {
	ID         string                      `json:"id"`
	SequenceNo uint                        `json:"sequenceNo"`
	Name       string                      `json:"name"`
	Choices    []model.ExecutionPathChoice `json:"choices"`
	UpdatedAt  string                      `json:"updatedAt"`
}

type executionPathBatchResponse struct {
	TotalCount    int                     `json:"totalCount"`
	ExistingCount int                     `json:"existingCount"`
	CreatedCount  int                     `json:"createdCount"`
	Items         []executionPathResponse `json:"items"`
}

type executionPathListResponse struct {
	Items []executionPathResponse `json:"items"`
}

// registerExecutionPathRoutes 注册同一计划下的四个路径读写端点。
func registerExecutionPathRoutes(mux *http.ServeMux, paths ExecutionPathService) {
	mux.HandleFunc("GET /api/plans/{id}/execution-paths", handleListExecutionPaths(paths))
	mux.HandleFunc("POST /api/plans/{id}/execution-paths", handleCreateExecutionPath(paths))
	mux.HandleFunc("POST /api/plans/{id}/execution-paths/generate-all", handleGenerateAllExecutionPaths(paths))
	mux.HandleFunc("PUT /api/plans/{id}/execution-paths/{pathId}", handleUpdateExecutionPath(paths))
	mux.HandleFunc("DELETE /api/plans/{id}/execution-paths/{pathId}", handleDeleteExecutionPath(paths))
}

// handleListExecutionPaths 返回按稳定序号排列的最小路径 DTO。
func handleListExecutionPaths(paths ExecutionPathService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseExecutionPathID(response, request.PathValue("id"))
		if !ok {
			return
		}
		items, err := paths.List(request.Context(), planID)
		if err != nil {
			writeExecutionPathError(response, err)
			return
		}
		result := make([]executionPathResponse, 0, len(items))
		for _, item := range items {
			result = append(result, toExecutionPathResponse(item))
		}
		writeSuccess(response, executionPathListResponse{Items: result})
	}
}

// handleCreateExecutionPath 只接受 choices 与请求头幂等键，不接受浏览器伪造图身份。
func handleCreateExecutionPath(paths ExecutionPathService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseExecutionPathID(response, request.PathValue("id"))
		if !ok {
			return
		}
		input, ok := decodeExecutionPathRequest(response, request)
		if !ok {
			return
		}
		path, created, err := paths.Create(request.Context(), planID, strings.TrimSpace(request.Header.Get("Idempotency-Key")), input.Name, input.Choices)
		if err != nil {
			writeExecutionPathError(response, err)
			return
		}
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		writeJSON(response, status, apiSuccess{Success: true, Data: toExecutionPathResponse(path)})
	}
}

// handleUpdateExecutionPath 用完整 choices 原位替换属于该计划的路径。
func handleUpdateExecutionPath(paths ExecutionPathService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseExecutionPathID(response, request.PathValue("id"))
		if !ok {
			return
		}
		pathID, ok := parseExecutionPathID(response, request.PathValue("pathId"))
		if !ok {
			return
		}
		input, ok := decodeExecutionPathRequest(response, request)
		if !ok {
			return
		}
		path, err := paths.Update(request.Context(), planID, pathID, input.Name, input.Choices)
		if err != nil {
			writeExecutionPathError(response, err)
			return
		}
		writeSuccess(response, toExecutionPathResponse(path))
	}
}

// handleGenerateAllExecutionPaths 使用独立持久化幂等键触发新发起计划的全路径原子生成。
func handleGenerateAllExecutionPaths(paths ExecutionPathService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseExecutionPathID(response, request.PathValue("id"))
		if !ok {
			return
		}
		result, created, err := paths.GenerateAll(request.Context(), planID, strings.TrimSpace(request.Header.Get("Idempotency-Key")))
		if err != nil {
			writeExecutionPathError(response, err)
			return
		}
		items := make([]executionPathResponse, 0, len(result.Paths))
		for _, path := range result.Paths {
			items = append(items, toExecutionPathResponse(path))
		}
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		writeJSON(response, status, apiSuccess{Success: true, Data: executionPathBatchResponse{
			TotalCount: result.TotalCount, ExistingCount: result.ExistingCount,
			CreatedCount: result.CreatedCount, Items: items,
		}})
	}
}

// handleDeleteExecutionPath 删除本地路径记录并返回无响应体的 204。
func handleDeleteExecutionPath(paths ExecutionPathService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseExecutionPathID(response, request.PathValue("id"))
		if !ok {
			return
		}
		pathID, ok := parseExecutionPathID(response, request.PathValue("pathId"))
		if !ok {
			return
		}
		if err := paths.Delete(request.Context(), planID, pathID); err != nil {
			writeExecutionPathError(response, err)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	}
}

// decodeExecutionPathRequest 严格拒绝未知字段和多余 JSON，缩小浏览器可信输入面。
func decodeExecutionPathRequest(response http.ResponseWriter, request *http.Request) (executionPathRequest, bool) {
	var input executionPathRequest
	decoder := json.NewDecoder(io.LimitReader(request.Body, maxAPIRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || ensureJSONEnd(decoder) != nil {
		writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "执行路径请求格式不正确", false)
		return executionPathRequest{}, false
	}
	if input.Choices == nil {
		input.Choices = []model.ExecutionPathChoice{}
	}
	return input, true
}

// parseExecutionPathID 解析计划或路径正整数标识并直接写出稳定参数错误。
func parseExecutionPathID(response http.ResponseWriter, raw string) (uint64, bool) {
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "计划或路径 ID 不正确", false)
		return 0, false
	}
	return id, true
}

// toExecutionPathResponse 仅公开选择所需标识、稳定序号和更新时间。
func toExecutionPathResponse(path model.ExecutionPath) executionPathResponse {
	return executionPathResponse{
		ID: strconv.FormatUint(path.ID, 10), SequenceNo: path.SequenceNo,
		Name:    path.Name,
		Choices: nonNilSlice(path.Choices), UpdatedAt: path.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

// writeExecutionPathError 将路径、计划和目标读取错误映射为批准的稳定契约。
func writeExecutionPathError(response http.ResponseWriter, err error) {
	switch {
	case service.IsExecutionPathErrorKind(err, service.ExecutionPathErrorInvalidArgument):
		writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error(), false)
	case service.IsExecutionPathErrorKind(err, service.ExecutionPathErrorNotFound):
		writeFailure(response, http.StatusNotFound, "EXECUTION_PATH_NOT_FOUND", "执行路径不存在", false)
	case service.IsExecutionPathErrorKind(err, service.ExecutionPathErrorInvalid):
		writeFailure(response, http.StatusConflict, "EXECUTION_PATH_INVALID", "执行路径选择不完整或已失效", false)
	case service.IsExecutionPathErrorKind(err, service.ExecutionPathErrorLimit):
		writeFailure(response, http.StatusConflict, "EXECUTION_PATH_LIMIT_REACHED", "当前计划最多只能保存一条执行路径", false)
	case service.IsExecutionPathErrorKind(err, service.ExecutionPathErrorEnumerationLimit):
		writeFailure(response, http.StatusConflict, "EXECUTION_PATH_ENUMERATION_LIMIT", "当前流程超过 128 条路径，请手动建立重点路径", false)
	case service.IsExecutionPathErrorKind(err, service.ExecutionPathErrorGenerateAll):
		writeFailure(response, http.StatusConflict, "EXECUTION_PATH_GENERATE_ALL_UNAVAILABLE", "只有新发起计划可以一键生成全部路径", false)
	case service.IsExecutionPathErrorKind(err, service.ExecutionPathErrorLocked):
		writeFailure(response, http.StatusConflict, "PLAN_LOCKED", "计划已经不能修改执行路径", false)
	case service.IsExecutionPathErrorKind(err, service.ExecutionPathErrorStorage):
		writeFailure(response, http.StatusServiceUnavailable, "PLAN_STORAGE_UNAVAILABLE", "路径存储暂不可用，请重试", true)
	default:
		writeFlowGraphError(response, err)
	}
}

type unavailableExecutionPathService struct{}

// List 在未注入路径存储时返回稳定不可用错误。
func (unavailableExecutionPathService) List(context.Context, uint64) ([]model.ExecutionPath, error) {
	return nil, &service.ExecutionPathError{Kind: service.ExecutionPathErrorStorage, Message: "路径存储暂不可用"}
}

// Create 在未注入路径存储时拒绝创建。
func (unavailableExecutionPathService) Create(context.Context, uint64, string, string, []model.ExecutionPathChoice) (model.ExecutionPath, bool, error) {
	return model.ExecutionPath{}, false, &service.ExecutionPathError{Kind: service.ExecutionPathErrorStorage, Message: "路径存储暂不可用"}
}

// Update 在未注入路径存储时拒绝更新。
func (unavailableExecutionPathService) Update(context.Context, uint64, uint64, string, []model.ExecutionPathChoice) (model.ExecutionPath, error) {
	return model.ExecutionPath{}, &service.ExecutionPathError{Kind: service.ExecutionPathErrorStorage, Message: "路径存储暂不可用"}
}

// Delete 在未注入路径存储时拒绝删除。
func (unavailableExecutionPathService) Delete(context.Context, uint64, uint64) error {
	return &service.ExecutionPathError{Kind: service.ExecutionPathErrorStorage, Message: "路径存储暂不可用"}
}

// GenerateAll 在未注入路径存储时拒绝批量生成。
func (unavailableExecutionPathService) GenerateAll(context.Context, uint64, string) (model.ExecutionPathBatchResult, bool, error) {
	return model.ExecutionPathBatchResult{}, false, &service.ExecutionPathError{Kind: service.ExecutionPathErrorStorage, Message: "路径存储暂不可用"}
}
