package run_orchestration

import (
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
	"testing"
)

// TestF052StepSourceProjection 保证接口保留持久化来源和状态，前端不再靠“同意”名称误判导航或自动跳过。
func TestF052StepSourceProjection(t *testing.T) {
	rows := []model.RunStep{
		{ID: 1, Source: "system_navigation", Action: "approve", Status: model.RunStepSkipped},
		{ID: 2, Source: "system_navigation", Action: "approve", Status: model.RunStepSucceeded},
		{ID: 3, Source: "user_action", Action: "approve", Status: model.RunStepSucceeded},
	}
	dtos := service.BuildStepDTOsForTest(rows, nil)
	for i, row := range rows {
		if dtos[i].Source != row.Source || dtos[i].Status != row.Status {
			t.Fatalf("步骤来源或状态丢失：%+v", dtos[i])
		}
	}
}
