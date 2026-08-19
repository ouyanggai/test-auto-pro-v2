package backend_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// TestF009PlanStatusUsesOnlyPublicThreeStates 验证旧配置完成度不再进入计划公开状态。
func TestF009PlanStatusUsesOnlyPublicThreeStates(t *testing.T) {
	for _, status := range []model.PlanStatus{model.PlanStatusNotStarted, model.PlanStatusRunning, model.PlanStatusCompleted} {
		if !model.ValidPlanStatus(status) {
			t.Fatalf("合法计划状态被拒绝：%s", status)
		}
	}
	for _, status := range []model.PlanStatus{"pending_configuration", "ready", "unknown"} {
		if model.ValidPlanStatus(status) {
			t.Fatalf("旧计划状态仍被公开协议接受：%s", status)
		}
	}
}

// TestF009RunningPlanRejectsPathAndBatchMutations 验证运行中计划只读且不会启动后台工作。
func TestF009RunningPlanRejectsPathAndBatchMutations(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusRunning}}
	graph := &executionPathGraphReader{graph: selectableExecutionPathGraph()}
	paths := &memoryExecutionPathRepository{}
	executionPaths := service.NewExecutionPathService(service.NewPlanService(plans), graph, nil, paths)
	if _, err := executionPaths.StartGeneration(context.Background(), 7, "123e4567-e89b-12d3-a456-426614174901"); !service.IsExecutionPathErrorKind(err, service.ExecutionPathErrorLocked) {
		t.Fatalf("运行中计划仍可启动路径解析：%v", err)
	}
	if graph.calls != 0 {
		t.Fatalf("只读计划仍访问了真实流程：calls=%d", graph.calls)
	}

	config := service.NewPathConfigService(service.NewPlanService(plans), nil, nil, nil, nil, nil, nil)
	preparations := &f009BatchPreparationRepository{job: model.PathPreparationJob{ID: "job", PlanID: 7, Status: "cancelled"}}
	batch := service.NewPathPreparationService(config, preparations)
	if _, err := batch.Resume(context.Background(), 7, "job"); err == nil {
		t.Fatal("运行中计划仍可恢复批量准备任务")
	}
	if preparations.job.Status != "cancelled" {
		t.Fatalf("只读校验之后仍修改了任务：%+v", preparations.job)
	}
}

// TestF009PlanStatusMigrationIsSingleDevelopmentDataUpdate 验证迁移只直接收敛两个旧开发值。
func TestF009PlanStatusMigrationIsSingleDevelopmentDataUpdate(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "internal", "repository", "mysql", "migrations", "014_simplify_plan_status.sql"))
	if err != nil {
		t.Fatalf("读取计划状态迁移失败：%v", err)
	}
	sql := strings.TrimSpace(string(content))
	if strings.Count(sql, ";") != 1 || sql != "UPDATE test_plans SET status = 'not_started' WHERE status IN ('pending_configuration', 'ready');" {
		t.Fatalf("计划状态迁移必须保持单语句直接收敛：%s", sql)
	}
}

// TestF009ExecutionPathRunnableConsumesOnlyIndependentStatuses 验证未来运行资格只由路径双状态决定。
func TestF009ExecutionPathRunnableConsumesOnlyIndependentStatuses(t *testing.T) {
	tests := []struct {
		configurationStatus string
		dataStatus          string
		want                bool
	}{
		{configurationStatus: "configured", dataStatus: "not_required", want: true},
		{configurationStatus: "configured", dataStatus: "generated", want: true},
		{configurationStatus: "configured", dataStatus: "confirmed", want: true},
		{configurationStatus: "pending", dataStatus: "generated", want: false},
		{configurationStatus: "partial", dataStatus: "confirmed", want: false},
		{configurationStatus: "affected", dataStatus: "not_required", want: false},
		{configurationStatus: "configured", dataStatus: "not_generated", want: false},
		{configurationStatus: "configured", dataStatus: "needs_attention", want: false},
	}
	for _, test := range tests {
		path := model.ExecutionPath{ConfigurationStatus: test.configurationStatus, DataStatus: test.dataStatus}
		if got := model.IsExecutionPathRunnable(path); got != test.want {
			t.Fatalf("路径可运行判定不正确：configuration=%s data=%s got=%v", test.configurationStatus, test.dataStatus, got)
		}
	}
}
