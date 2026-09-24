package api

import (
	"context"
	"net/http"

	"test-auto-pro-v2/internal/service"
)

// ReportService 是报告中心三级只读聚合的服务面：首页总览、计划详情、路径详情。
// 全部只读运行事实，不写库、不触发目标平台，因此只暴露三个 GET 端点。
type ReportService interface {
	// Overview 返回 L0 首页：计划情况计数、计划维度表与全局专业指标。
	Overview(ctx context.Context) (service.ReportOverviewDTO, error)
	// PlanReport 返回 L1 计划详情：计划头部、计划级指标与各路径指标。
	PlanReport(ctx context.Context, planID uint64) (service.ReportPlanDetailDTO, error)
	// PathReport 返回 L2 路径详情：路径头部、路径级指标与运行实例列表。
	PathReport(ctx context.Context, planID, executionPathID uint64) (service.ReportPathDetailDTO, error)
}

// registerReportRoutes 注册报告中心三级只读端点。
func registerReportRoutes(mux *http.ServeMux, reports ReportService) {
	mux.HandleFunc("GET /api/reports/overview", handleReportOverview(reports))
	mux.HandleFunc("GET /api/reports/plans/{id}", handleReportPlan(reports))
	mux.HandleFunc("GET /api/reports/plans/{id}/paths/{pathId}", handleReportPath(reports))
}

// handleReportOverview 返回首页总览聚合。
func handleReportOverview(reports ReportService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		result, err := reports.Overview(request.Context())
		if err != nil {
			writePlanError(response, err)
			return
		}
		writeSuccess(response, result)
	}
}

// handleReportPlan 返回单计划详情聚合。
func handleReportPlan(reports ReportService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseExecutionPathID(response, request.PathValue("id"))
		if !ok {
			return
		}
		result, err := reports.PlanReport(request.Context(), planID)
		if err != nil {
			writePlanError(response, err)
			return
		}
		writeSuccess(response, result)
	}
}

// handleReportPath 返回单路径详情聚合（含运行实例列表，供跳转 L3 运行详情）。
func handleReportPath(reports ReportService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseExecutionPathID(response, request.PathValue("id"))
		if !ok {
			return
		}
		pathID, ok := parseExecutionPathID(response, request.PathValue("pathId"))
		if !ok {
			return
		}
		result, err := reports.PathReport(request.Context(), planID, pathID)
		if err != nil {
			writePlanError(response, err)
			return
		}
		writeSuccess(response, result)
	}
}

// NewHandlerWithReportCenter 在既有组装之上注入报告中心只读端点（F-042）。
// 沿用「在组装处包一层」的方式：未命中报告端点的请求原样交给既有 handler。
func NewHandlerWithReportCenter(base http.Handler, reports ReportService) http.Handler {
	if reports == nil {
		return base
	}
	mux := http.NewServeMux()
	registerReportRoutes(mux, reports)
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if _, pattern := mux.Handler(request); pattern != "" {
			mux.ServeHTTP(response, request)
			return
		}
		base.ServeHTTP(response, request)
	})
}
