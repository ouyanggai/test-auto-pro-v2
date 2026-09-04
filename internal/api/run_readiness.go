package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// RunReadinessService 是成功断言与运行准备的只读服务面。
// 本切片不启动运行，因此这里只有读取、保存断言与读取运行准备结论三个动作。
type RunReadinessService interface {
	PlanReadiness(ctx context.Context, planID uint64, selectedPathIDs []uint64) (model.PlanRunReadiness, error)
}

// registerRunReadinessRoutes 注册成功断言与运行准备端点。
// 路径形如 /api/plans/{id}/execution-paths/{pathId}/...，与 F-013 的日志作用域中间件约定一致，
// 因此这些请求产生的日志会自动落进对应计划与执行路径的目录。
func registerRunReadinessRoutes(mux *http.ServeMux, readiness RunReadinessService) {
	mux.HandleFunc("GET /api/plans/{id}/run-readiness", handlePlanRunReadiness(readiness))
}

// handlePlanRunReadiness 返回一个计划的运行准备结论：一句总结论加逐条路径的阻塞与提醒。
func handlePlanRunReadiness(readiness RunReadinessService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseExecutionPathID(response, request.PathValue("id"))
		if !ok {
			return
		}
		selected, ok := parseSelectedPathIDs(response, request.URL.Query().Get("pathIds"))
		if !ok {
			return
		}
		result, err := readiness.PlanReadiness(request.Context(), planID, selected)
		if err != nil {
			writeRunReadinessError(response, err)
			return
		}
		writeSuccess(response, result)
	}
}

// parseSelectedPathIDs 解析 pathIds 查询参数：逗号分隔的路径主键，为空表示检查全部路径。
// 运行只运行勾选路径，预检也只检查勾选路径，因此这个参数决定本次检查范围。
func parseSelectedPathIDs(response http.ResponseWriter, raw string) ([]uint64, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, true
	}
	parts := strings.Split(trimmed, ",")
	selected := make([]uint64, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.ParseUint(strings.TrimSpace(part), 10, 64)
		if err != nil || value == 0 {
			writeFailure(response, http.StatusBadRequest, "RUN_READINESS_INVALID", "勾选的执行路径标识不正确", false)
			return nil, false
		}
		selected = append(selected, value)
	}
	return selected, true
}

// writeRunReadinessError 把服务层稳定错误映射为状态码与稳定错误码，中文文案与日志同源。
func writeRunReadinessError(response http.ResponseWriter, err error) {
	switch {
	case service.IsRunReadinessErrorKind(err, service.RunReadinessErrorNotFound):
		writeFailure(response, http.StatusNotFound, "RUN_READINESS_NOT_FOUND", err.Error(), false)
	case service.IsRunReadinessErrorKind(err, service.RunReadinessErrorInvalid):
		writeFailure(response, http.StatusBadRequest, "RUN_READINESS_INVALID", err.Error(), false)
	case service.IsRunReadinessErrorKind(err, service.RunReadinessErrorConflict):
		writeFailure(response, http.StatusConflict, "RUN_READINESS_CONFLICT", err.Error(), false)
	case service.IsRunReadinessErrorKind(err, service.RunReadinessErrorTarget):
		writeFailure(response, http.StatusBadGateway, "RUN_READINESS_TARGET_UNAVAILABLE", err.Error(), true)
	default:
		writeFailure(response, http.StatusServiceUnavailable, "RUN_READINESS_STORAGE_UNAVAILABLE",
			firstNonEmptyMessage(err, "运行准备服务暂不可用，请重试"), true)
	}
}

// firstNonEmptyMessage 取错误自带的中文提示，缺失时用稳定兜底文案。
func firstNonEmptyMessage(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	if message := strings.TrimSpace(err.Error()); message != "" {
		return message
	}
	return fallback
}

// unavailableRunReadinessService 在未注入真实服务时返回稳定不可用错误，不静默放行。
type unavailableRunReadinessService struct{}

// PlanReadiness 返回稳定不可用错误。
func (unavailableRunReadinessService) PlanReadiness(context.Context, uint64, []uint64) (model.PlanRunReadiness, error) {
	return model.PlanRunReadiness{}, &service.RunReadinessError{
		Kind: service.RunReadinessErrorStorage, Message: "运行准备服务暂不可用"}
}
