package contracts_test

import (
	"context"

	"test-auto-pro-v2/internal/model"
)

// stubPathConfigurationService 为只读维护 API 契约提供不触发目标平台的路径配置桩。
type stubPathConfigurationService struct{}

// Get 返回空路径配置，保持维护 API 测试不依赖数据库或目标平台。
func (stubPathConfigurationService) Get(context.Context, uint64, uint64) (model.PathConfiguration, error) {
	return model.PathConfiguration{}, nil
}

// RuntimeSession 返回空运行时会话，维护 API 测试不会向目标平台发放真实会话。
func (stubPathConfigurationService) RuntimeSession(context.Context, uint64, uint64) (model.PathFormRuntimeSession, error) {
	return model.PathFormRuntimeSession{}, nil
}
