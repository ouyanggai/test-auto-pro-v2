package backend_test

import (
	"context"
	"errors"
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
	entries  []string
	calls    int
}

// FlowTreeSnapshotWithoutRelogin 与同步读取同形状：测试假件无需区分，直接复用。
func (r *capturedFlowTreeReader) FlowTreeSnapshotWithoutRelogin(ctx context.Context, account, source, targetID string) (target.FlowTreeSnapshot, error) {
	return r.FlowTreeSnapshot(ctx, account, source, targetID)
}

// FlowTreeSnapshot 记录计划持久化身份并返回预设真实入口。
func (r *capturedFlowTreeReader) FlowTreeSnapshot(_ context.Context, account, source, targetID string) (target.FlowTreeSnapshot, error) {
	r.calls++
	r.account, r.source, r.targetID = account, source, targetID
	entries := r.entries
	if entries == nil {
		entries = []string{"start"}
	}
	return target.FlowTreeSnapshot{Tree: &target.FlowNodeTemplate{ID: "start", Name: "发起", Type: "start"}, EntryNodeIDs: entries}, nil
}

// TestFlowGraphServiceInvalidatesCacheAfterPlanBindingChange 验证编辑目标身份后不会继续给路径编辑或预检返回旧流程图。
func TestFlowGraphServiceInvalidatesCacheAfterPlanBindingChange(t *testing.T) {
	repo := newMemoryPlanRepository()
	repo.plans = []model.Plan{{
		ID: 9, Name: "计划", Account: "old-account", FlowSource: "new", TargetObjectID: "old-template", TargetObjectName: "旧流程", RunMode: "serial",
	}}
	plans := service.NewPlanService(repo)
	reader := &capturedFlowTreeReader{}
	graphs := service.NewFlowGraphService(plans, reader, analyzer.NewFlowGraphAnalyzer())
	if _, err := graphs.Get(context.Background(), 9); err != nil {
		t.Fatalf("首次读取流程图失败：%v", err)
	}
	if reader.calls != 1 || reader.targetID != "old-template" {
		t.Fatalf("首次图读取身份不正确：calls=%d target=%s", reader.calls, reader.targetID)
	}
	if _, err := plans.Update(context.Background(), 9, service.UpdatePlanInput{
		Name: "计划", Account: "new-account", FlowSource: "new", TargetObjectID: "new-template", TargetObjectName: "新流程", RunMode: "serial",
	}); err != nil {
		t.Fatalf("编辑计划目标身份失败：%v", err)
	}
	graph, err := graphs.Get(context.Background(), 9)
	if err != nil {
		t.Fatalf("编辑后读取流程图失败：%v", err)
	}
	if reader.calls != 2 || reader.account != "new-account" || reader.targetID != "new-template" {
		t.Fatalf("编辑后仍命中旧流程图缓存：calls=%d account=%s target=%s", reader.calls, reader.account, reader.targetID)
	}
	if graph.TargetName != "新流程" {
		t.Fatalf("编辑后流程图摘要未更新：%+v", graph)
	}
}

// TestFlowGraphServiceRejectsMissingOrForeignEntries 验证空入口和跨图入口均不可配置。
func TestFlowGraphServiceRejectsMissingOrForeignEntries(t *testing.T) {
	for _, entries := range [][]string{{}, {"other"}} {
		repo := newMemoryPlanRepository()
		repo.plans = []model.Plan{{ID: 9, Account: "account", FlowSource: "started", TargetObjectID: "instance"}}
		reader := &capturedFlowTreeReader{entries: entries}
		_, err := service.NewFlowGraphService(service.NewPlanService(repo), reader, analyzer.NewFlowGraphAnalyzer()).Get(context.Background(), 9)
		if !errors.Is(err, service.ErrTargetFlowNotConfigurable) {
			t.Fatalf("非法运行入口没有被拒绝：entries=%v err=%v", entries, err)
		}
	}
}

// TestFlowGraphServiceUsesOnlyPersistedPlanIdentity 验证图读取只采用计划保存身份。
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
	if len(graph.EntryNodeIDs) != 1 || graph.EntryNodeIDs[0] != "start" {
		t.Fatal("流程图没有保留真实运行入口")
	}
}
