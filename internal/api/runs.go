package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"test-auto-pro-v2/internal/engine/control"
	"test-auto-pro-v2/internal/model"
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

// RunOrchestrator 是运行主线的处理器的服务面：启动（模式与断点）、详情、放行命令、断点、暂停、停止、列表。
type RunOrchestrator interface {
	StartRun(ctx context.Context, input service.StartRunInput) (*service.PathRunDetailDTO, error)
	StartRunWithMode(ctx context.Context, input service.StartRunInput, mode model.RunMode, breakpoints []control.Breakpoint) (*service.PathRunDetailDTO, error)
	RunDetail(ctx context.Context, runID uint64) (*service.PathRunDetailDTO, error)
	ApproveWithCommand(ctx context.Context, pathRunID uint64, command model.ControlCommand, cursor int, version int64) (*service.PathRunDetailDTO, error)
	SetBreakpoint(ctx context.Context, pathRunID uint64, bp control.Breakpoint) ([]control.Breakpoint, error)
	RemoveBreakpoint(ctx context.Context, pathRunID uint64, bp control.Breakpoint) ([]control.Breakpoint, error)
	ListBreakpoints(ctx context.Context, pathRunID uint64) ([]control.Breakpoint, error)
	RequestPause(ctx context.Context, pathRunID uint64) error
	ReconcileNow(ctx context.Context, pathRunID uint64) (*service.ReconcileViewDTO, error)
	RecoveryAction(ctx context.Context, pathRunID uint64, action string, manual model.RunManualConclusion) (*service.PathRunDetailDTO, error)
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
	mux.HandleFunc("POST /api/runs/{runId}/breakpoints", handleSetBreakpoint(orchestrator))
	mux.HandleFunc("DELETE /api/runs/{runId}/breakpoints", handleRemoveBreakpoint(orchestrator))
	mux.HandleFunc("POST /api/runs/{runId}/pause", handlePause(orchestrator))
	mux.HandleFunc("POST /api/runs/{runId}/reconcile", handleReconcile(orchestrator))
	mux.HandleFunc("POST /api/runs/{runId}/recovery", handleRecoveryAction(orchestrator))
}

// startRunRequest 是启动请求体：模式三选一（默认单步）+ 启动前断点预置。
type startRunRequest struct {
	PlanID          uint64            `json:"planId"`
	ExecutionPathID uint64            `json:"executionPathId"`
	Mode            string            `json:"mode"`
	Breakpoints     []breakpointInput `json:"breakpoints"`
}

// breakpointInput 是断点预置/增删请求的最小体。
type breakpointInput struct {
	Type    string `json:"type"`
	StepNo  int    `json:"stepNo,omitempty"`
	NodeKey string `json:"nodeKey,omitempty"`
	Action  string `json:"action,omitempty"`
}

// toBreakpoint 转换为控制层断点；非法类型返回错误。
func toBreakpoint(input breakpointInput) (control.Breakpoint, error) {
	switch model.BreakpointType(input.Type) {
	case model.BreakpointFirstWrite:
		return control.Breakpoint{Type: model.BreakpointFirstWrite}, nil
	case model.BreakpointStep:
		return control.Breakpoint{Type: model.BreakpointStep, StepNo: input.StepNo}, nil
	case model.BreakpointNode:
		return control.Breakpoint{Type: model.BreakpointNode, NodeKey: input.NodeKey}, nil
	case model.BreakpointAction:
		return control.Breakpoint{Type: model.BreakpointAction, Action: input.Action}, nil
	case model.BreakpointPathDeviation:
		return control.Breakpoint{Type: model.BreakpointPathDeviation}, nil
	default:
		return control.Breakpoint{}, fmt.Errorf("未知的断点类型：%s", input.Type)
	}
}

// handleStartRun 启动一次运行。模式三选一（默认单步——最保守的默认值），
// 启动前由服务层复验 F-015 的运行准备结论，未通过直接拒绝并给中文原因。
func handleStartRun(orchestrator RunOrchestrator) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseExecutionPathID(response, request.PathValue("planId"))
		if !ok {
			return
		}
		var body startRunRequest
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
		mode := model.RunModeSingleStep
		if body.Mode != "" {
			mode = model.RunMode(body.Mode)
		}
		breakpoints := make([]control.Breakpoint, 0, len(body.Breakpoints))
		for _, input := range body.Breakpoints {
			bp, err := toBreakpoint(input)
			if err != nil {
				writeFailure(response, http.StatusBadRequest, "RUN_BREAKPOINT_INVALID", err.Error(), false)
				return
			}
			breakpoints = append(breakpoints, bp)
		}
		detail, err := orchestrator.StartRunWithMode(request.Context(), service.StartRunInput{
			PlanID: body.PlanID, ExecutionPathID: body.ExecutionPathID,
		}, mode, breakpoints)
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

// approveRunRequest 是放行命令请求体：命令种类 + 步游标 + 控制版本（条件写、幂等）。
type approveRunRequest struct {
	Command      string `json:"command"`
	Cursor       int    `json:"cursor"`
	ControlVersion int64 `json:"controlVersion"`
}

// handleApproveRun 按命令放行：只作用于这一条路径运行，没有批量入口，不绑单键快捷键。
func handleApproveRun(orchestrator RunOrchestrator) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		runID, ok := parseExecutionPathID(response, request.PathValue("runId"))
		if !ok {
			return
		}
		var body approveRunRequest
		if err := decodeRunBody(request, &body); err != nil {
			writeFailure(response, http.StatusBadRequest, "RUN_APPROVE_INVALID", "放行请求体格式不正确", false)
			return
		}
		command := model.ControlCommand(body.Command)
		if command == "" {
			command = model.CommandStep
		}
		detail, err := orchestrator.ApproveWithCommand(request.Context(), runID, command, body.Cursor, body.ControlVersion)
		if err != nil {
			writeRunControlError(response, err)
			return
		}
		writeSuccess(response, detail)
	}
}

// handleSetBreakpoint 运行中增加断点。
func handleSetBreakpoint(orchestrator RunOrchestrator) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		runID, ok := parseExecutionPathID(response, request.PathValue("runId"))
		if !ok {
			return
		}
		var body breakpointInput
		if err := decodeRunBody(request, &body); err != nil {
			writeFailure(response, http.StatusBadRequest, "RUN_BREAKPOINT_INVALID", "断点请求体格式不正确", false)
			return
		}
		bp, err := toBreakpoint(body)
		if err != nil {
			writeFailure(response, http.StatusBadRequest, "RUN_BREAKPOINT_INVALID", err.Error(), false)
			return
		}
		breakpoints, err := orchestrator.SetBreakpoint(request.Context(), runID, bp)
		if err != nil {
			writeRunControlError(response, err)
			return
		}
		writeSuccess(response, breakpointsToDTO(breakpoints))
	}
}

// handleRemoveBreakpoint 运行中删除断点；路径偏离断点被拒绝。
func handleRemoveBreakpoint(orchestrator RunOrchestrator) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		runID, ok := parseExecutionPathID(response, request.PathValue("runId"))
		if !ok {
			return
		}
		var body breakpointInput
		if err := decodeRunBody(request, &body); err != nil {
			writeFailure(response, http.StatusBadRequest, "RUN_BREAKPOINT_INVALID", "断点请求体格式不正确", false)
			return
		}
		bp, err := toBreakpoint(body)
		if err != nil {
			writeFailure(response, http.StatusBadRequest, "RUN_BREAKPOINT_INVALID", err.Error(), false)
			return
		}
		breakpoints, err := orchestrator.RemoveBreakpoint(request.Context(), runID, bp)
		if err != nil {
			writeRunControlError(response, err)
			return
		}
		writeSuccess(response, breakpointsToDTO(breakpoints))
	}
}

// handleReconcile 触发只读对账（可重复调用，安全）。
func handleReconcile(orchestrator RunOrchestrator) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		runID, ok := parseExecutionPathID(response, request.PathValue("runId"))
		if !ok {
			return
		}
		view, err := orchestrator.ReconcileNow(request.Context(), runID)
		if err != nil {
			writeRunControlError(response, err)
			return
		}
		writeSuccess(response, view)
	}
}

// recoveryRequest 是恢复动作请求体：动作名 + 人工结论登记（仅 manual_end 需要）。
type recoveryRequest struct {
	Action         string `json:"action"`
	InstanceStatus string `json:"instanceStatus"`
	CurrentNode    string `json:"currentNode"`
	Note           string `json:"note"`
	Reporter       string `json:"reporter"`
}

// handleRecoveryAction 执行对账给出的唯一合法动作；重复/过期请求返回当前真实状态。
func handleRecoveryAction(orchestrator RunOrchestrator) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		runID, ok := parseExecutionPathID(response, request.PathValue("runId"))
		if !ok {
			return
		}
		var body recoveryRequest
		if err := decodeRunBody(request, &body); err != nil {
			writeFailure(response, http.StatusBadRequest, "RUN_RECOVERY_INVALID", "恢复请求体格式不正确", false)
			return
		}
		manual := model.RunManualConclusion{
			InstanceStatus: body.InstanceStatus, CurrentNode: body.CurrentNode,
			Note: body.Note, Reporter: body.Reporter,
		}
		detail, err := orchestrator.RecoveryAction(request.Context(), runID, body.Action, manual)
		if err != nil {
			writeRunControlError(response, err)
			return
		}
		writeSuccess(response, detail)
	}
}

// breakpointsToDTO 把断点列表转为公开形态。
func breakpointsToDTO(breakpoints []control.Breakpoint) []map[string]any {
	result := make([]map[string]any, 0, len(breakpoints))
	for _, bp := range breakpoints {
		result = append(result, map[string]any{
			"type":   string(bp.Type),
			"stepNo": bp.StepNo,
			"nodeKey": bp.NodeKey,
			"action": bp.Action,
		})
	}
	return result
}

// handlePause 提交暂停请求：随时可提交、只在阶段 3 生效。
func handlePause(orchestrator RunOrchestrator) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		runID, ok := parseExecutionPathID(response, request.PathValue("runId"))
		if !ok {
			return
		}
		if err := orchestrator.RequestPause(request.Context(), runID); err != nil {
			writeRunControlError(response, err)
			return
		}
		writeSuccess(response, map[string]any{"paused": true})
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
	case errors.Is(err, control.ErrLoopRunning), errors.Is(err, control.ErrVersionConflict),
		errors.Is(err, control.ErrCursorConflict), errors.Is(err, control.ErrCommandNotAllowed):
		writeFailure(response, http.StatusConflict, "RUN_CONTROL_CONFLICT", err.Error(), false)
	case errors.Is(err, control.ErrNotRunnable):
		writeFailure(response, http.StatusConflict, "RUN_NOT_RUNNABLE", err.Error(), false)
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
