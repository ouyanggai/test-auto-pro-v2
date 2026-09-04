package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"test-auto-pro-v2/internal/engine/control"
	"test-auto-pro-v2/internal/repository"
	"test-auto-pro-v2/internal/service"
)

// decodeRunBody 解析 JSON 请求体；空请求体按空对象处理（放行与停止没有请求体）。
func decodeRunBody(request *http.Request, target any) error {
	content, err := io.ReadAll(io.LimitReader(request.Body, 1<<20))
	if err != nil {
		return err
	}
	if len(content) == 0 {
		return nil
	}
	return json.Unmarshal(content, target)
}

// RunOrchestrator 是运行主线的处理器的服务面：启动、详情、放行、停止、列表。
type RunOrchestrator interface {
	StartRun(ctx context.Context, input service.StartRunInput) (*service.PathRunDetailDTO, error)
	RunDetail(ctx context.Context, runID uint64) (*service.PathRunDetailDTO, error)
	Approve(ctx context.Context, pathRunID uint64) (*service.PathRunDetailDTO, error)
	Stop(ctx context.Context, pathRunID uint64) (*service.PathRunDetailDTO, error)
	ListRuns(ctx context.Context, planID uint64) ([]service.RunSummaryDTO, error)
}

// registerRunControlRoutes 注册启动、详情、放行与停止端点。
// 详情与控制端点以运行 ID 寻址（一次运行只跑一条路径），列表与启动挂在计划下，
// 与 F-013 的日志作用域中间件的 /api/plans/{planId}/... 约定一致。
func registerRunControlRoutes(mux *http.ServeMux, orchestrator RunOrchestrator) {
	mux.HandleFunc("POST /api/plans/{planId}/runs", handleStartRun(orchestrator))
	mux.HandleFunc("GET /api/plans/{planId}/runs", handleListRuns(orchestrator))
	mux.HandleFunc("GET /api/runs/{runId}", handleRunDetail(orchestrator))
	mux.HandleFunc("POST /api/runs/{runId}/approve", handleApproveRun(orchestrator))
	mux.HandleFunc("POST /api/runs/{runId}/stop", handleStopRun(orchestrator))
}

// handleStartRun 启动一次单步运行。模式固定为单步（不是可切换控件），
// 启动前由服务层复验 F-015 的运行准备结论，未通过直接拒绝并给中文原因。
func handleStartRun(orchestrator RunOrchestrator) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseExecutionPathID(response, request.PathValue("planId"))
		if !ok {
			return
		}
		var body service.StartRunInput
		if err := decodeRunBody(request, &body); err != nil {
			writeFailure(response, http.StatusBadRequest, "RUN_START_INVALID", "启动请求体格式不正确", false)
			return
		}
		if body.PlanID == 0 {
			body.PlanID = planID
		}
		if body.PlanID != planID || body.ExecutionPathID == 0 {
			writeFailure(response, http.StatusBadRequest, "RUN_START_INVALID", "启动请求必须指明该计划下的执行路径", false)
			return
		}
		detail, err := orchestrator.StartRun(request.Context(), body)
		if err != nil {
			writeRunControlError(response, err)
			return
		}
		writeSuccess(response, detail)
	}
}

// handleListRuns 列出计划下的运行，供运行列表页进入详情。
func handleListRuns(orchestrator RunOrchestrator) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseExecutionPathID(response, request.PathValue("planId"))
		if !ok {
			return
		}
		items, err := orchestrator.ListRuns(request.Context(), planID)
		if err != nil {
			writeRunControlError(response, err)
			return
		}
		writeSuccess(response, items)
	}
}

// handleRunDetail 返回路径运行详情：运行事实、节点状态、当前预览与最终目标事实。
func handleRunDetail(orchestrator RunOrchestrator) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		runID, ok := parseExecutionPathID(response, request.PathValue("runId"))
		if !ok {
			return
		}
		detail, err := orchestrator.RunDetail(request.Context(), runID)
		if err != nil {
			writeRunControlError(response, err)
			return
		}
		writeSuccess(response, detail)
	}
}

// handleApproveRun 放行当前步：只作用于这一条路径运行，没有批量入口。
func handleApproveRun(orchestrator RunOrchestrator) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		runID, ok := parseExecutionPathID(response, request.PathValue("runId"))
		if !ok {
			return
		}
		detail, err := orchestrator.Approve(request.Context(), runID)
		if err != nil {
			writeRunControlError(response, err)
			return
		}
		writeSuccess(response, detail)
	}
}

// handleStopRun 停止路径运行；本步执行中时延迟生效并如实告知。
func handleStopRun(orchestrator RunOrchestrator) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		runID, ok := parseExecutionPathID(response, request.PathValue("runId"))
		if !ok {
			return
		}
		detail, err := orchestrator.Stop(request.Context(), runID)
		if err != nil {
			writeRunControlError(response, err)
			return
		}
		writeSuccess(response, detail)
	}
}

// writeRunControlError 把运行控制类错误映射为状态码与稳定错误码，中文文案与日志同源。
func writeRunControlError(response http.ResponseWriter, err error) {
	var orchestrationErr *service.RunOrchestrationError
	switch {
	case errors.As(err, &orchestrationErr):
		switch orchestrationErr.Kind {
		case service.RunOrchestrationNotFound:
			writeFailure(response, http.StatusNotFound, "RUN_NOT_FOUND", orchestrationErr.Error(), false)
		case service.RunOrchestrationStorage:
			writeFailure(response, http.StatusServiceUnavailable, "RUN_TARGET_UNAVAILABLE", orchestrationErr.Error(), true)
		default:
			writeFailure(response, http.StatusConflict, "RUN_CONFLICT", orchestrationErr.Error(), false)
		}
	case errors.Is(err, control.ErrStopDeferred):
		writeFailure(response, http.StatusConflict, "RUN_STOP_DEFERRED", err.Error(), false)
	case errors.Is(err, control.ErrNoActiveStep), errors.Is(err, control.ErrRunAlreadyFinished):
		writeFailure(response, http.StatusConflict, "RUN_CONFLICT", err.Error(), false)
	case service.IsRunReadinessErrorKind(err, service.RunReadinessErrorNotFound):
		writeFailure(response, http.StatusNotFound, "RUN_START_NOT_FOUND", err.Error(), false)
	case service.IsRunReadinessErrorKind(err, service.RunReadinessErrorInvalid):
		writeFailure(response, http.StatusBadRequest, "RUN_START_INVALID", err.Error(), false)
	case service.IsRunReadinessErrorKind(err, service.RunReadinessErrorTarget):
		writeFailure(response, http.StatusBadGateway, "RUN_TARGET_UNAVAILABLE", err.Error(), true)
	case errors.Is(err, repository.ErrRunNotFound):
		writeFailure(response, http.StatusNotFound, "RUN_NOT_FOUND", "运行记录不存在", false)
	case errors.Is(err, repository.ErrRunStatusConflict), errors.Is(err, repository.ErrLeaseHeld), errors.Is(err, repository.ErrStaleLease):
		writeFailure(response, http.StatusConflict, "RUN_STATE_CONFLICT", err.Error(), false)
	default:
		writeFailure(response, http.StatusServiceUnavailable, "RUN_STORAGE_UNAVAILABLE", "运行服务暂不可用，请重试", true)
	}
}
