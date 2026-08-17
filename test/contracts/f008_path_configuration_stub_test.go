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

type f008PresetStub struct {
	stubPathConfigurationService
	scope string
}

// PreviewPreset 记录预览范围并返回逐节点处理结果。
func (s *f008PresetStub) PreviewPreset(_ context.Context, _, _ uint64, scope string) (model.PathConfigPresetPreview, error) {
	s.scope = scope
	return model.PathConfigPresetPreview{Scope: scope, Paths: []model.PathConfigPresetPath{{Path: model.PathConfigPath{SequenceNo: 1, Name: "路径"}, Items: []model.PathConfigPresetNodeItem{{NodeKey: "node", NodeName: "审批", Status: "write", Detail: "将写入安全默认动作"}}}}}, nil
}

// ApplyPreset 记录确认范围并返回安全写入统计。
func (s *f008PresetStub) ApplyPreset(_ context.Context, _, _ uint64, scope string) (model.PathConfigPresetApplyResult, error) {
	s.scope = scope
	return model.PathConfigPresetApplyResult{Written: 1}, nil
}

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

// PreviewPreset 返回空预览桩。
func (stubPathConfigurationService) PreviewPreset(context.Context, uint64, uint64, string) (model.PathConfigPresetPreview, error) {
	return model.PathConfigPresetPreview{}, nil
}

// ApplyPreset 返回空写入结果桩。
func (stubPathConfigurationService) ApplyPreset(context.Context, uint64, uint64, string) (model.PathConfigPresetApplyResult, error) {
	return model.PathConfigPresetApplyResult{}, nil
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

// TestF008PresetAPIContract 验证预览与确认分别走独立端点并透传三个受限范围之一。
func TestF008PresetAPIContract(t *testing.T) {
	stub := &f008PresetStub{}
	handler := api.NewHandlerWithConfigurationServices(&stubTargetReader{}, service.NewPlanService(&contractPlanRepository{}), &stubFlowGraphService{}, &stubExecutionPathService{}, &stubPathRequirementService{}, stub)
	preview := httptest.NewRecorder()
	handler.ServeHTTP(preview, httptest.NewRequest(http.MethodPost, "/api/plans/7/execution-paths/31/configuration/preset/preview", strings.NewReader(`{"scope":"compatible"}`)))
	if preview.Code != http.StatusOK || stub.scope != "compatible" || !strings.Contains(preview.Body.String(), `"status":"write"`) {
		t.Fatalf("预设预览契约不正确：status=%d scope=%s body=%s", preview.Code, stub.scope, preview.Body.String())
	}
	apply := httptest.NewRecorder()
	handler.ServeHTTP(apply, httptest.NewRequest(http.MethodPost, "/api/plans/7/execution-paths/31/configuration/preset/apply", strings.NewReader(`{"scope":"selected"}`)))
	if apply.Code != http.StatusOK || stub.scope != "selected" || !strings.Contains(apply.Body.String(), `"written":1`) {
		t.Fatalf("预设应用契约不正确：status=%d scope=%s body=%s", apply.Code, stub.scope, apply.Body.String())
	}
}
