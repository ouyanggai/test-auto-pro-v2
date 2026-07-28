package backend_test

import (
	"context"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

type capturedFlowTreeReader struct {
	account  string
	source   string
	targetID string
}

func (r *capturedFlowTreeReader) FlowTree(_ context.Context, account, source, targetID string) (*target.FlowNodeTemplate, error) {
	r.account, r.source, r.targetID = account, source, targetID
	return &target.FlowNodeTemplate{ID: "start", Name: "发起", Type: "start"}, nil
}

func TestFlowGraphServiceUsesOnlyPersistedPlanIdentity(t *testing.T) {
	repo := newMemoryPlanRepository()
	repo.plans = []model.Plan{{
		ID: 9, Name: "已保存计划", Account: "saved-account", FlowSource: "started",
		TargetObjectID: "saved-instance", TargetObjectName: "已发流程快照",
	}}
	reader := &capturedFlowTreeReader{}
	graphs := service.NewFlowGraphService(service.NewPlanService(repo), reader, analyzer.NewFlowGraphAnalyzer())
	graph, err := graphs.Get(context.Background(), 9)
	if err != nil {
		t.Fatalf("读取计划流程图失败：%v", err)
	}
	if reader.account != "saved-account" || reader.source != "started" || reader.targetID != "saved-instance" {
		t.Fatal("流程图没有只使用计划持久化的账号、来源和目标 ID")
	}
	if graph.PlanID != 9 || graph.TargetName != "已发流程快照" || graph.FlowSource != "started" {
		t.Fatal("流程图摘要没有来自已保存计划")
	}
}
