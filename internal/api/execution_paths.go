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
	Create(context.Context, uint64, string, []model.ExecutionPathChoice) (model.ExecutionPath, bool, error)
	Update(context.Context, uint64, uint64, []model.ExecutionPathChoice) (model.ExecutionPath, error)
	Delete(context.Context, uint64, uint64) error
}

type executionPathRequest struct {
	Choices []model.ExecutionPathChoice `json:"choices"`
}

type executionPathResponse struct {
	ID         string                      `json:"id"`
	SequenceNo uint                        `json:"sequenceNo"`
	Choices    []model.ExecutionPathChoice `json:"choices"`
	UpdatedAt  string                      `json:"updatedAt"`
}

type executionPathListResponse struct {
	Items []executionPathResponse `json:"items"`
}

func registerExecutionPathRoutes(mux *http.ServeMux, paths ExecutionPathService) {
	mux.HandleFunc("GET /api/plans/{id}/execution-paths", handleListExecutionPaths(paths))
	mux.HandleFunc("POST /api/plans/{id}/execution-paths", handleCreateExecutionPath(paths))
	mux.HandleFunc("PUT /api/plans/{id}/execution-paths/{pathId}", handleUpdateExecutionPath(paths))
	mux.HandleFunc("DELETE /api/plans/{id}/execution-paths/{pathId}", handleDeleteExecutionPath(paths))
}

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
		path, created, err := paths.Create(request.Context(), planID, strings.TrimSpace(request.Header.Get("Idempotency-Key")), input.Choices)
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
		path, err := paths.Update(request.Context(), planID, pathID, input.Choices)
		if err != nil {
			writeExecutionPathError(response, err)
			return
		}
		writeSuccess(response, toExecutionPathResponse(path))
	}
}

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

func parseExecutionPathID(response http.ResponseWriter, raw string) (uint64, bool) {
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "计划或路径 ID 不正确", false)
		return 0, false
	}
	return id, true
}

func toExecutionPathResponse(path model.ExecutionPath) executionPathResponse {
	return executionPathResponse{
		ID: strconv.FormatUint(path.ID, 10), SequenceNo: path.SequenceNo,
		Choices: nonNilSlice(path.Choices), UpdatedAt: path.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

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
	case service.IsExecutionPathErrorKind(err, service.ExecutionPathErrorLocked):
		writeFailure(response, http.StatusConflict, "PLAN_LOCKED", "计划已经不能修改执行路径", false)
	case service.IsExecutionPathErrorKind(err, service.ExecutionPathErrorStorage):
		writeFailure(response, http.StatusServiceUnavailable, "PLAN_STORAGE_UNAVAILABLE", "路径存储暂不可用，请重试", true)
	default:
		writeFlowGraphError(response, err)
	}
}

type unavailableExecutionPathService struct{}

func (unavailableExecutionPathService) List(context.Context, uint64) ([]model.ExecutionPath, error) {
	return nil, &service.ExecutionPathError{Kind: service.ExecutionPathErrorStorage, Message: "路径存储暂不可用"}
}
func (unavailableExecutionPathService) Create(context.Context, uint64, string, []model.ExecutionPathChoice) (model.ExecutionPath, bool, error) {
	return model.ExecutionPath{}, false, &service.ExecutionPathError{Kind: service.ExecutionPathErrorStorage, Message: "路径存储暂不可用"}
}
func (unavailableExecutionPathService) Update(context.Context, uint64, uint64, []model.ExecutionPathChoice) (model.ExecutionPath, error) {
	return model.ExecutionPath{}, &service.ExecutionPathError{Kind: service.ExecutionPathErrorStorage, Message: "路径存储暂不可用"}
}
func (unavailableExecutionPathService) Delete(context.Context, uint64, uint64) error {
	return &service.ExecutionPathError{Kind: service.ExecutionPathErrorStorage, Message: "路径存储暂不可用"}
}
