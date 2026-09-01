package api

import (
	"compress/gzip"
	"context"
	"net/http"
	"strings"

	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/formruntimemaintenance"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// 健康接口保持固定响应，供本地热更新探针和基础连通性检查使用。
const healthResponse = `{"status":"ok","service":"test-auto-pro","version":"dev"}`

// NewHandler 创建仅包含基础能力的默认处理器，业务存储缺失时返回稳定错误。
func NewHandler() http.Handler {
	return NewHandlerWithServices(service.NewTargetReadService(config.LoadTargetConfig()), unavailablePlanService{})
}

// NewHandlerWithTargetReader 为目标读取测试注入替代实现。
func NewHandlerWithTargetReader(reader TargetReader) http.Handler {
	return NewHandlerWithServices(reader, unavailablePlanService{})
}

// NewHandlerWithServices 为计划 API 测试注入目标和计划服务。
func NewHandlerWithServices(reader TargetReader, plans PlanService) http.Handler {
	return NewHandlerWithFlowGraphServices(reader, plans, unavailableFlowGraphService{})
}

// NewHandlerWithFlowGraphServices 为流程图范围注入真实图服务。
func NewHandlerWithFlowGraphServices(reader TargetReader, plans PlanService, graphs FlowGraphService) http.Handler {
	return NewHandlerWithExecutionPathServices(reader, plans, graphs, unavailableExecutionPathService{})
}

// NewHandlerWithExecutionPathServices 组装包含 F-005 路径端点的完整 HTTP 路由。
func NewHandlerWithExecutionPathServices(reader TargetReader, plans PlanService, graphs FlowGraphService, paths ExecutionPathService) http.Handler {
	return NewHandlerWithRequirementServices(reader, plans, graphs, paths, unavailablePathRequirementService{})
}

// NewHandlerWithRequirementServices 组装包含 F-006 只读路径要求端点的完整 HTTP 路由。
func NewHandlerWithRequirementServices(reader TargetReader, plans PlanService, graphs FlowGraphService, paths ExecutionPathService, requirements PathRequirementService) http.Handler {
	return NewHandlerWithConfigurationServices(reader, plans, graphs, paths, requirements, unavailablePathConfigurationService{})
}

// NewHandlerWithConfigurationServices 组装包含 F-007 路径配置读写端点的完整 HTTP 路由。
func NewHandlerWithConfigurationServices(reader TargetReader, plans PlanService, graphs FlowGraphService, paths ExecutionPathService, requirements PathRequirementService, configurations PathConfigurationService) http.Handler {
	return NewHandlerWithMaintenanceServices(reader, plans, graphs, paths, requirements, configurations, unavailableFormRuntimeMaintenanceService{})
}

// NewHandlerWithMaintenanceServices 组装 F-007 表单运行时维护端点，仍不允许请求指定来源或命令。
func NewHandlerWithMaintenanceServices(reader TargetReader, plans PlanService, graphs FlowGraphService, paths ExecutionPathService, requirements PathRequirementService, configurations PathConfigurationService, maintenance FormRuntimeMaintenanceService) http.Handler {
	return NewHandlerWithPreparationServices(reader, plans, graphs, paths, requirements, configurations, maintenance, unavailablePathPreparationService{})
}

// NewHandlerWithPreparationServices 组装 F-009 批量准备端点与既有全部能力。
func NewHandlerWithPreparationServices(reader TargetReader, plans PlanService, graphs FlowGraphService, paths ExecutionPathService, requirements PathRequirementService, configurations PathConfigurationService, maintenance FormRuntimeMaintenanceService, preparations PathPreparationService) http.Handler {
	return NewHandlerWithTemplateCatalogServices(reader, plans, graphs, paths, requirements, configurations, maintenance, preparations, unavailableTemplateCatalogService{})
}

// NewHandlerWithTemplateCatalogServices 组装 F-010 模板规则目录与既有全部能力。
func NewHandlerWithTemplateCatalogServices(reader TargetReader, plans PlanService, graphs FlowGraphService, paths ExecutionPathService, requirements PathRequirementService, configurations PathConfigurationService, maintenance FormRuntimeMaintenanceService, preparations PathPreparationService, catalog TemplateCatalogService) http.Handler {
	return NewHandlerWithHistoryDataServices(reader, plans, graphs, paths, requirements, configurations, maintenance, preparations, catalog, unavailableHistoryDataService{})
}

// NewHandlerWithHistoryDataServices 组装 F-012 历史候选与来源配置端点及既有全部能力。
func NewHandlerWithHistoryDataServices(reader TargetReader, plans PlanService, graphs FlowGraphService, paths ExecutionPathService, requirements PathRequirementService, configurations PathConfigurationService, maintenance FormRuntimeMaintenanceService, preparations PathPreparationService, catalog TemplateCatalogService, history HistoryDataService) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", health)
	registerTargetRoutes(mux, reader)
	registerPlanRoutes(mux, plans)
	registerFlowGraphRoute(mux, graphs)
	registerExecutionPathRoutes(mux, paths)
	registerPathRequirementRoute(mux, requirements)
	registerPathConfigurationRoutes(mux, configurations)
	registerFormRuntimeMaintenanceRoutes(mux, maintenance)
	registerPathPreparationRoutes(mux, preparations)
	registerTemplateCatalogRoutes(mux, catalog)
	registerHistoryDataRoutes(mux, history)
	return gzipResponses(mux)
}

type unavailableHistoryDataService struct{}

// Candidates 在默认测试处理器未注入历史数据服务时返回稳定不可用错误。
func (unavailableHistoryDataService) Candidates(context.Context, uint64, uint64, string, int, int) (model.HistoryCandidatePage, error) {
	return model.HistoryCandidatePage{}, &service.HistoryDataError{Kind: service.HistoryDataErrorStorage, Message: "历史数据存储暂不可用"}
}

// SaveDefault 在默认测试处理器未注入历史数据服务时返回稳定不可用错误。
func (unavailableHistoryDataService) SaveDefault(context.Context, uint64, model.HistoryDefaultSaveInput, string) (model.HistoryDataSource, error) {
	return model.HistoryDataSource{}, &service.HistoryDataError{Kind: service.HistoryDataErrorStorage, Message: "历史数据存储暂不可用"}
}

// SavePathSource 在默认测试处理器未注入历史数据服务时返回稳定不可用错误。
func (unavailableHistoryDataService) SavePathSource(context.Context, uint64, uint64, model.HistoryPathSourceInput, string) (model.HistoryDataSource, error) {
	return model.HistoryDataSource{}, &service.HistoryDataError{Kind: service.HistoryDataErrorStorage, Message: "历史数据存储暂不可用"}
}

// gzipResponses 为声明支持 gzip 的客户端压缩 JSON 响应，避免大路径摘要重复占用传输带宽。
func gzipResponses(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !strings.Contains(request.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(response, request)
			return
		}
		response.Header().Set("Content-Encoding", "gzip")
		response.Header().Add("Vary", "Accept-Encoding")
		writer := gzip.NewWriter(response)
		defer writer.Close()
		next.ServeHTTP(gzipResponseWriter{ResponseWriter: response, writer: writer}, request)
	})
}

// gzipResponseWriter 将 HTTP 写入委托给 gzip 流，状态码与其他头仍由原响应写入器处理。
type gzipResponseWriter struct {
	http.ResponseWriter
	writer *gzip.Writer
}

// Write 压缩响应体而不改变上层 JSON 编码和错误处理语义。
func (w gzipResponseWriter) Write(data []byte) (int, error) {
	return w.writer.Write(data)
}

type unavailableFormRuntimeMaintenanceService struct{}

// InspectSource 在默认测试处理器未注入维护服务时返回稳定不可用错误。
func (unavailableFormRuntimeMaintenanceService) InspectSource(context.Context) (formruntimemaintenance.SourceState, error) {
	return formruntimemaintenance.SourceState{}, formruntimemaintenance.ErrSourceInvalid
}

// CreateJob 在默认测试处理器未注入维护服务时返回稳定不可用错误。
func (unavailableFormRuntimeMaintenanceService) CreateJob(context.Context) (formruntimemaintenance.Job, error) {
	return formruntimemaintenance.Job{}, formruntimemaintenance.ErrSourceInvalid
}

// GetJob 在默认测试处理器未注入维护服务时返回稳定不可用错误。
func (unavailableFormRuntimeMaintenanceService) GetJob(context.Context, uint64) (formruntimemaintenance.Job, error) {
	return formruntimemaintenance.Job{}, formruntimemaintenance.ErrJobNotFound
}

// LatestJob 在默认测试处理器未注入维护服务时返回稳定不可用错误。
func (unavailableFormRuntimeMaintenanceService) LatestJob(context.Context) (formruntimemaintenance.Job, error) {
	return formruntimemaintenance.Job{}, formruntimemaintenance.ErrJobNotFound
}

// GetJobLog 在默认测试处理器未注入维护服务时返回稳定不可用错误。
func (unavailableFormRuntimeMaintenanceService) GetJobLog(context.Context, uint64) (formruntimemaintenance.Log, error) {
	return formruntimemaintenance.Log{}, formruntimemaintenance.ErrLogNotFound
}

// health 返回无时间戳的稳定健康响应。
func health(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write([]byte(healthResponse))
}
