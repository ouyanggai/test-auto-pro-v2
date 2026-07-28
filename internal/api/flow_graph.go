package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

type FlowGraphService interface {
	Get(context.Context, uint64) (model.FlowGraph, error)
}

type flowGraphResponse struct {
	PlanID       string                `json:"planId"`
	TargetName   string                `json:"targetName"`
	FlowSource   string                `json:"flowSource"`
	EntryNodeIDs []string              `json:"entryNodeIds"`
	Nodes        []model.FlowGraphNode `json:"nodes"`
	Edges        []model.FlowGraphEdge `json:"edges"`
	Warnings     []string              `json:"warnings"`
}

// registerFlowGraphRoute 注册计划真实流程图只读端点。
func registerFlowGraphRoute(mux *http.ServeMux, graphs FlowGraphService) {
	mux.HandleFunc("GET /api/plans/{id}/flow-graph", func(response http.ResponseWriter, request *http.Request) {
		id, err := strconv.ParseUint(request.PathValue("id"), 10, 64)
		if err != nil || id == 0 {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "计划 ID 不正确", false)
			return
		}
		graph, err := graphs.Get(request.Context(), id)
		if err != nil {
			writeFlowGraphError(response, err)
			return
		}
		writeSuccess(response, flowGraphResponse{
			PlanID: strconv.FormatUint(graph.PlanID, 10), TargetName: graph.TargetName, FlowSource: graph.FlowSource,
			EntryNodeIDs: nonNilSlice(graph.EntryNodeIDs),
			Nodes:        nonNilSlice(graph.Nodes), Edges: nonNilSlice(graph.Edges), Warnings: nonNilSlice(graph.Warnings),
		})
	})
}

// writeFlowGraphError 将计划、目标会话和结构错误收敛为稳定公开响应。
func writeFlowGraphError(response http.ResponseWriter, err error) {
	var configErr *config.MissingTargetConfigError
	switch {
	case service.IsPlanErrorKind(err, service.PlanErrorInvalidArgument):
		writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "计划 ID 不正确", false)
	case service.IsPlanErrorKind(err, service.PlanErrorNotFound):
		writeFailure(response, http.StatusNotFound, "PLAN_NOT_FOUND", "计划不存在", false)
	case service.IsPlanErrorKind(err, service.PlanErrorStorage):
		writeFailure(response, http.StatusServiceUnavailable, "PLAN_STORAGE_UNAVAILABLE", "计划存储暂不可用，请重试", true)
	case errors.Is(err, service.ErrTargetFlowNotFound):
		writeFailure(response, http.StatusNotFound, "TARGET_FLOW_NOT_FOUND", "目标流程当前不可读取", false)
	case errors.Is(err, service.ErrTargetFlowStructureEmpty):
		writeFailure(response, http.StatusUnprocessableEntity, "TARGET_FLOW_STRUCTURE_EMPTY", "目标流程暂未配置节点", false)
	case errors.Is(err, service.ErrTargetFlowNotConfigurable):
		writeFailure(response, http.StatusConflict, "TARGET_FLOW_NOT_CONFIGURABLE", "当前流程已经不能配置执行路径", false)
	case errors.Is(err, analyzer.ErrFlowStructureInvalid):
		writeFailure(response, http.StatusBadGateway, "TARGET_FLOW_STRUCTURE_INVALID", "目标流程结构异常", false)
	case errors.As(err, &configErr):
		writeFailure(response, http.StatusServiceUnavailable, "TARGET_CONFIG_MISSING", "目标环境尚未配置完整", false)
	case target.IsKind(err, target.ErrorLoginRejected):
		writeFailure(response, http.StatusUnauthorized, "TARGET_LOGIN_REJECTED", "账号验证失败，请核对账号", false)
	case target.IsKind(err, target.ErrorSessionExpired):
		writeFailure(response, http.StatusUnauthorized, "TARGET_SESSION_EXPIRED", "账号会话已失效，请重新验证账号", true)
	case target.IsKind(err, target.ErrorResponseInvalid):
		writeFailure(response, http.StatusBadGateway, "TARGET_RESPONSE_INVALID", "流程数据格式异常", true)
	case target.IsKind(err, target.ErrorTimeout):
		writeFailure(response, http.StatusGatewayTimeout, "TARGET_TIMEOUT", "读取流程超时，请重试", true)
	default:
		writeFailure(response, http.StatusServiceUnavailable, "TARGET_UNAVAILABLE", "暂时无法读取流程，请重试", true)
	}
}

type unavailableFlowGraphService struct{}

// Get 在未注入图服务时返回稳定的计划存储不可用错误。
func (unavailableFlowGraphService) Get(context.Context, uint64) (model.FlowGraph, error) {
	return model.FlowGraph{}, &service.PlanError{Kind: service.PlanErrorStorage, Message: "计划存储暂不可用"}
}
