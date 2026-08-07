package api

import (
	"context"
	"net/http"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// PathRequirementService 读取单条计划路径的当前只读要求。
type PathRequirementService interface {
	Get(context.Context, uint64, uint64) (model.PathRequirements, error)
}

// registerPathRequirementRoute 注册计划与路径双重归属约束的只读要求端点。
func registerPathRequirementRoute(mux *http.ServeMux, requirements PathRequirementService) {
	mux.HandleFunc("GET /api/plans/{id}/execution-paths/{pathId}/requirements", handlePathRequirements(requirements))
}

// handlePathRequirements 解析计划和路径标识并返回安全中文要求 DTO。
func handlePathRequirements(requirements PathRequirementService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		planID, ok := parseExecutionPathID(response, request.PathValue("id"))
		if !ok {
			return
		}
		pathID, ok := parseExecutionPathID(response, request.PathValue("pathId"))
		if !ok {
			return
		}
		result, err := requirements.Get(request.Context(), planID, pathID)
		if err != nil {
			writeExecutionPathError(response, err)
			return
		}
		writeSuccess(response, result)
	}
}

type unavailablePathRequirementService struct{}

// Get 在未注入真实要求服务时返回稳定存储不可用错误。
func (unavailablePathRequirementService) Get(context.Context, uint64, uint64) (model.PathRequirements, error) {
	return model.PathRequirements{}, &service.ExecutionPathError{Kind: service.ExecutionPathErrorStorage, Message: "路径要求服务暂不可用"}
}
