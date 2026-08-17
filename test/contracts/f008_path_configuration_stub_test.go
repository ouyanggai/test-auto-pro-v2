package contracts_test

import (
	"context"

	"test-auto-pro-v2/internal/model"
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
