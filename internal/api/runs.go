package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

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
	// StartRunWithPaths 按勾选路径集合启动（F-020）：多路径一次运行，串并方式来自计划配置。
	StartRunWithPaths(ctx context.Context, planID uint64, pathIDs []uint64, mode model.RunMode, breakpoints []control.Breakpoint, idempotencyKey string) (*service.RunStartDTO, error)
	RunDetail(ctx context.Context, runID uint64) (*service.PathRunDetailDTO, error)
	RunDetailByRunAndPathRun(ctx context.Context, runID, pathRunID uint64) (*service.PathRunDetailDTO, error)
	// 控制端点以运行 ID 寻址；多路径运行必须携带路径运行身份（pathRunId），单路径可省略。
	ApproveWithCommand(ctx context.Context, runID uint64, pathRunID uint64, command model.ControlCommand, cursor int, version int64) (*service.PathRunDetailDTO, error)
	SetBreakpoint(ctx context.Context, runID uint64, pathRunID uint64, bp control.Breakpoint) ([]control.Breakpoint, error)
	RemoveBreakpoint(ctx context.Context, runID uint64, pathRunID uint64, bp control.Breakpoint) ([]control.Breakpoint, error)
	ListBreakpoints(ctx context.Context, runID uint64, pathRunID uint64) ([]control.Breakpoint, error)
	RequestPause(ctx context.Context, runID uint64, pathRunID uint64) error
	SwitchMode(ctx context.Context, runID uint64, pathRunID uint64, mode model.RunMode, version int64) (*service.PathRunDetailDTO, error)
	Stop(ctx context.Context, runID uint64, pathRunID uint64) (*service.PathRunDetailDTO, error)
	// 运行记录层级（2026-09-06）：跨计划列表（一行一次运行）、二级路径页与整次运行删除。
	ListAllRuns(ctx context.Context, status string) ([]service.RunSummaryDTO, error)
	RunPaths(ctx context.Context, runID uint64) (*service.RunPathsDTO, error)
	DeleteRun(ctx context.Context, runID uint64) error
	ListRunEvents(ctx context.Context, runID uint64, afterEventID uint64, limit int, pathRunID uint64) ([]service.RunEventDTO, error)
}

// registerRunControlRoutes 注册启动、详情、放行与停止端点。
// 详情与控制端点以运行 ID 寻址（一次运行只跑一条路径），列表与启动挂在计划下，
// 与 F-013 的日志作用域中间件的 /api/plans/{planId}/... 约定一致。
func registerRunControlRoutes(mux *http.ServeMux, orchestrator RunOrchestrator) {
	mux.HandleFunc("POST /api/plans/{planId}/runs", handleStartRun(orchestrator))
	// 运行记录层级（2026-09-06）：列表挂在全局 /api/runs 下（一行只对应一次计划运行），
	// 二级路径页与删除挂在单次运行下，与详情、控制端点同一寻址约定。
	mux.HandleFunc("GET /api/runs", handleListAllRuns(orchestrator))
	mux.HandleFunc("GET /api/runs/{runId}", handleRunDetail(orchestrator))
	mux.HandleFunc("GET /api/runs/{runId}/paths", handleRunPaths(orchestrator))
	mux.HandleFunc("DELETE /api/runs/{runId}", handleDeleteRun(orchestrator))
	mux.HandleFunc("POST /api/runs/{runId}/approve", handleApproveRun(orchestrator))
	mux.HandleFunc("POST /api/runs/{runId}/stop", handleStopRun(orchestrator))
	mux.HandleFunc("POST /api/runs/{runId}/breakpoints", handleSetBreakpoint(orchestrator))
	mux.HandleFunc("DELETE /api/runs/{runId}/breakpoints", handleRemoveBreakpoint(orchestrator))
	mux.HandleFunc("POST /api/runs/{runId}/pause", handlePause(orchestrator))
	// 运行中模式切换（2026-09-06）：自动/单步互切，安全边界生效，条件写幂等。
	mux.HandleFunc("POST /api/runs/{runId}/mode", handleSwitchMode(orchestrator))
	// F-021 事件流时间线：只读增量读取，afterEventID 为游标。
	mux.HandleFunc("GET /api/runs/{runId}/events", handleRunEvents(orchestrator))
}

// handleListAllRuns 跨计划列出运行（一行一次运行）；status 查询参数可选筛选。
func handleListAllRuns(orchestrator RunOrchestrator) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		status := strings.TrimSpace(request.URL.Query().Get("status"))
		items, err := orchestrator.ListAllRuns(request.Context(), status)
		if err != nil {
			writeRunControlError(response, err)
			return
		}
		writeSuccess(response, items)
	}
}

// handleRunPaths 返回一次运行的二级路径页数据：每条路径的名称、状态、当前节点与准确进度。
func handleRunPaths(orchestrator RunOrchestrator) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		runID, ok := parseExecutionPathID(response, request.PathValue("runId"))
		if !ok {
			return
		}
		paths, err := orchestrator.RunPaths(request.Context(), runID)
		if err != nil {
			writeRunControlError(response, err)
			return
		}
		writeSuccess(response, paths)
	}
}

// handleDeleteRun 删除整次工具侧运行及其全部子记录；运行中的记录必须先停止再删除。
func handleDeleteRun(orchestrator RunOrchestrator) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		runID, ok := parseExecutionPathID(response, request.PathValue("runId"))
		if !ok {
			return
		}
		if err := orchestrator.DeleteRun(request.Context(), runID); err != nil {
			writeRunControlError(response, err)
			return
		}
		writeSuccess(response, map[string]any{"deleted": true})
	}
}

// handleRunEvents 增量读取一次运行的事件流：afterEventID 之后的按数据库顺序返回。
func handleRunEvents(orchestrator RunOrchestrator) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		runID, ok := parseExecutionPathID(response, request.PathValue("runId"))
		if !ok {
			return
		}
		afterID, _ := strconv.ParseUint(strings.TrimSpace(request.URL.Query().Get("afterEventId")), 10, 64)
		limit, _ := strconv.Atoi(strings.TrimSpace(request.URL.Query().Get("limit")))
		pathRunID, _ := strconv.ParseUint(strings.TrimSpace(request.URL.Query().Get("pathRunId")), 10, 64)
		events, err := orchestrator.ListRunEvents(request.Context(), runID, afterID, limit, pathRunID)
		if err != nil {
			writeRunControlError(response, err)
			return
		}
		writeSuccess(response, events)
	}
}

// startRunRequest 是启动请求体：勾选路径集合（F-020 多路径）+ 模式三选一（默认单步）+ 启动前断点预置。
type startRunRequest struct {
	PlanID      uint64            `json:"planId"`
	PathIDs     []uint64          `json:"pathIds"`
	Mode        string            `json:"mode"`
	Breakpoints []breakpointInput `json:"breakpoints"`
	// IdempotencyKey 是本次启动的幂等键：同键重试返回同一次运行，绝不创建第二个运行。
	IdempotencyKey string `json:"idempotencyKey"`
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
		if body.PlanID != planID || len(body.PathIDs) == 0 {
			writeFailure(response, http.StatusBadRequest, "RUN_START_INVALID", "启动请求必须指明该计划下要运行的执行路径", false)
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
		result, err := orchestrator.StartRunWithPaths(request.Context(), body.PlanID, body.PathIDs, mode, breakpoints, body.IdempotencyKey)
		if err != nil {
			writeRunControlError(response, err)
			return
		}
		writeSuccess(response, result)
	}
}

// parsePathRunIDQuery 解析 pathRunId 查询参数（F-020 多路径运行的控制寻址）；缺省返回 0。
func parsePathRunIDQuery(request *http.Request) uint64 {
	value, err := strconv.ParseUint(strings.TrimSpace(request.URL.Query().Get("pathRunId")), 10, 64)
	if err != nil {
		return 0
	}
	return value
}

// handleRunDetail 返回路径运行详情：运行事实、节点状态、当前预览与最终目标事实。
// 多路径运行可用 pathRunId 查询参数选择要查看的路径运行。
func handleRunDetail(orchestrator RunOrchestrator) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		runID, ok := parseExecutionPathID(response, request.PathValue("runId"))
		if !ok {
			return
		}
		pathRunID := parsePathRunIDQuery(request)
		var (
			detail *service.PathRunDetailDTO
			err    error
		)
		if pathRunID != 0 {
			detail, err = orchestrator.RunDetailByRunAndPathRun(request.Context(), runID, pathRunID)
		} else {
			detail, err = orchestrator.RunDetail(request.Context(), runID)
		}
		if err != nil {
			writeRunControlError(response, err)
			return
		}
		writeSuccess(response, detail)
	}
}

// approveRunRequest 是放行命令请求体：命令种类 + 步游标 + 控制版本（条件写、幂等）。
type approveRunRequest struct {
	Command        string `json:"command"`
	Cursor         int    `json:"cursor"`
	ControlVersion int64  `json:"controlVersion"`
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
		detail, err := orchestrator.ApproveWithCommand(request.Context(), runID, parsePathRunIDQuery(request), command, body.Cursor, body.ControlVersion)
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
		breakpoints, err := orchestrator.SetBreakpoint(request.Context(), runID, parsePathRunIDQuery(request), bp)
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
		breakpoints, err := orchestrator.RemoveBreakpoint(request.Context(), runID, parsePathRunIDQuery(request), bp)
		if err != nil {
			writeRunControlError(response, err)
			return
		}
		writeSuccess(response, breakpointsToDTO(breakpoints))
	}
}

// breakpointsToDTO 把断点列表转为公开形态。
func breakpointsToDTO(breakpoints []control.Breakpoint) []map[string]any {
	result := make([]map[string]any, 0, len(breakpoints))
	for _, bp := range breakpoints {
		result = append(result, map[string]any{
			"type":    string(bp.Type),
			"stepNo":  bp.StepNo,
			"nodeKey": bp.NodeKey,
			"action":  bp.Action,
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
		if err := orchestrator.RequestPause(request.Context(), runID, parsePathRunIDQuery(request)); err != nil {
			writeRunControlError(response, err)
			return
		}
		writeSuccess(response, map[string]any{"paused": true})
	}
}

// switchModeRequest 是模式切换请求体：目标模式 + 控制版本（条件写、幂等）。
type switchModeRequest struct {
	Mode           string `json:"mode"`
	ControlVersion int64  `json:"controlVersion"`
}

// handleSwitchMode 运行中切换自动/单步；重复点击只产生一次控制事实。
func handleSwitchMode(orchestrator RunOrchestrator) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		runID, ok := parseExecutionPathID(response, request.PathValue("runId"))
		if !ok {
			return
		}
		var body switchModeRequest
		if err := decodeRunBody(request, &body); err != nil {
			writeFailure(response, http.StatusBadRequest, "RUN_MODE_INVALID", "模式切换请求体格式不正确", false)
			return
		}
		detail, err := orchestrator.SwitchMode(request.Context(), runID, parsePathRunIDQuery(request), model.RunMode(body.Mode), body.ControlVersion)
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
		detail, err := orchestrator.Stop(request.Context(), runID, parsePathRunIDQuery(request))
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
	case errors.Is(err, control.ErrLoopRunning), errors.Is(err, control.ErrStepInFlight), errors.Is(err, control.ErrVersionConflict),
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
