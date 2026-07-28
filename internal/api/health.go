package api

import (
	"net/http"

	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/service"
)

// 健康接口保持固定响应，供本地热更新探针=初始和基础连通性检查使用。
const healthResponse = `{"status":"ok","service":"test-auto-pro","version":"dev"}`

func NewHandler() http.Handler {
	return NewHandlerWithServices(service.NewTargetReadService(config.LoadTargetConfig()), unavailablePlanService{})
}

func NewHandlerWithTargetReader(reader TargetReader) http.Handler {
	return NewHandlerWithServices(reader, unavailablePlanService{})
}

func NewHandlerWithServices(reader TargetReader, plans PlanService) http.Handler {
	return NewHandlerWithFlowGraphServices(reader, plans, unavailableFlowGraphService{})
}

func NewHandlerWithFlowGraphServices(reader TargetReader, plans PlanService, graphs FlowGraphService) http.Handler {
	return NewHandlerWithExecutionPathServices(reader, plans, graphs, unavailableExecutionPathService{})
}

func NewHandlerWithExecutionPathServices(reader TargetReader, plans PlanService, graphs FlowGraphService, paths ExecutionPathService) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", health)
	registerTargetRoutes(mux, reader)
	registerPlanRoutes(mux, plans)
	registerFlowGraphRoute(mux, graphs)
	registerExecutionPathRoutes(mux, paths)
	return mux
}

func health(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write([]byte(healthResponse))
}
