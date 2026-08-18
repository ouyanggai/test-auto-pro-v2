package contracts_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/api"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// stubPathConfigurationService 为其他 API 契约提供不触发目标平台的 F-008 配置服务桩。
type stubPathConfigurationService struct{}

// Get 返回空配置桩。
func (stubPathConfigurationService) Get(context.Context, uint64, uint64) (model.PathConfiguration, error) {
	return model.PathConfiguration{}, nil
}

// SaveNode 返回空保存结果桩。
func (stubPathConfigurationService) SaveNode(context.Context, uint64, uint64, string, string, model.PathNodeSaveInput) (model.PathConfigSaveResult, error) {
	return model.PathConfigSaveResult{}, nil
}

// SaveSelection 返回空保存结果桩。
func (stubPathConfigurationService) SaveSelection(context.Context, uint64, uint64, string, model.PathConfigSelectionInput) (model.PathConfigSaveResult, error) {
	return model.PathConfigSaveResult{}, nil
}

// CopyCycles 返回空复制结果桩。
func (stubPathConfigurationService) CopyCycles(context.Context, uint64, uint64, uint64, string) (model.PathConfigSaveResult, error) {
	return model.PathConfigSaveResult{}, nil
}

// GenerateForm 返回空生成结果桩。
func (stubPathConfigurationService) GenerateForm(context.Context, uint64, uint64, int64, map[string]any, []string, bool) (model.PathFormGenerateResult, error) {
	return model.PathFormGenerateResult{}, nil
}

// SaveForm 返回空保存结果桩。
func (stubPathConfigurationService) SaveForm(context.Context, uint64, uint64, string, model.PathFormSaveInput) (model.PathConfigSaveResult, error) {
	return model.PathConfigSaveResult{}, nil
}

// RuntimeSession 返回空运行时会话桩。
func (stubPathConfigurationService) RuntimeSession(context.Context, uint64, uint64) (model.PathFormRuntimeSession, error) {
	return model.PathFormRuntimeSession{}, nil
}

type f008CycleCopyStub struct {
	stubPathConfigurationService
	targetPathID uint64
	sourcePathID uint64
	idempotency  string
}

// CopyCycles 记录复制来源、目标和幂等键，证明端点不把循环成员交给浏览器。
func (s *f008CycleCopyStub) CopyCycles(_ context.Context, _, targetPathID, sourcePathID uint64, idempotency string) (model.PathConfigSaveResult, error) {
	s.targetPathID, s.sourcePathID, s.idempotency = targetPathID, sourcePathID, idempotency
	return model.PathConfigSaveResult{Revision: 2}, nil
}

// TestF008CycleCopyAPIContract 验证循环复制只透传来源路径并使用目标路径 URL。
func TestF008CycleCopyAPIContract(t *testing.T) {
	stub := &f008CycleCopyStub{}
	handler := api.NewHandlerWithConfigurationServices(&stubTargetReader{}, service.NewPlanService(&contractPlanRepository{}), &stubFlowGraphService{}, &stubExecutionPathService{}, &stubPathRequirementService{}, stub)
	request := httptest.NewRequest(http.MethodPost, "/api/plans/7/execution-paths/31/configuration/cycles/copy", strings.NewReader(`{"sourcePathId":12}`))
	request.Header.Set("Idempotency-Key", "01234567-89ab-cdef-0123-456789abcdef")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || stub.targetPathID != 31 || stub.sourcePathID != 12 || stub.idempotency == "" {
		t.Fatalf("循环复制契约不正确：status=%d target=%d source=%d key=%s body=%s", response.Code, stub.targetPathID, stub.sourcePathID, stub.idempotency, response.Body.String())
	}
}
