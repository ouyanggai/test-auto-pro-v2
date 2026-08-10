package api

import (
	"context"
	"net/http"

	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/formruntimemaintenance"
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
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", health)
	registerTargetRoutes(mux, reader)
	registerPlanRoutes(mux, plans)
	registerFlowGraphRoute(mux, graphs)
	registerExecutionPathRoutes(mux, paths)
	registerPathRequirementRoute(mux, requirements)
	registerPathConfigurationRoutes(mux, configurations)
	registerFormRuntimeMaintenanceRoutes(mux, maintenance)
	return mux
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
