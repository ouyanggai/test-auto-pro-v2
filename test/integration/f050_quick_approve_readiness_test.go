package integration_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/model"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
	"test-auto-pro-v2/internal/service"
)

// f050TargetReader 只提供当前目标规则的固定投影，不执行真实目标提交或审批。
type f050TargetReader struct{ tree *target.FlowNodeTemplate }

// PathConfigurationSnapshot 返回支持默认提交和固定人员同意的最小流程。
func (r f050TargetReader) PathConfigurationSnapshot(context.Context, string, string, string) (target.PathConfigurationSnapshot, error) {
	return target.PathConfigurationSnapshot{Tree: r.tree, EntryNodeIDs: []string{"start"}, RenderType: target.FormRenderTypeFormMaking, FlowCode: "flow-a", FlowName: "默认同意"}, nil
}

// f050GraphReader 复用同一真实分析器生成的图，保证配置和运行前检查核对相同路径。
type f050GraphReader struct{ graph model.FlowGraph }

// Get 返回本测试已核实的路径图，不引入目标平台网络或内存配置标记。
func (r f050GraphReader) Get(context.Context, uint64) (model.FlowGraph, error) { return r.graph, nil }

// TestF050QuickApprovePersistsAcrossReconnect 验证零额外动作经过真实 MySQL 保存、列表读取和重连预检后可运行。
func TestF050QuickApprovePersistsAcrossReconnect(t *testing.T) {
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) > 0 {
		t.Fatalf("快速同意集成测试缺少配置名：%v", missing)
	}
	cfg.Name = temporaryPlanDatabaseName(t)
	t.Cleanup(func() { dropTemporaryPlanDatabase(t, cfg) })
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	database, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	plans := service.NewPlanService(planmysql.NewPlanRepository(database.DB))
	plan := createPathTestPlan(t, ctx, plans, "new", "123e4567-e89b-12d3-a456-426614175050")
	paths := planmysql.NewExecutionPathRepository(database.DB)
	path, created, err := paths.Create(ctx, plan.ID, "123e4567-e89b-12d3-a456-426614175051", "全同意路径", nil, time.Now().UTC())
	if err != nil || !created {
		t.Fatalf("创建路径失败：%v", err)
	}
	tree := &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{
		ID: "review", Type: "common", AuditConfig: &target.FlowNodeAuditConfig{AuditType: "assign", Mode: "scramble", Candidates: []target.FlowAuditCandidate{{ID: "user-a", Name: "用户甲"}}},
		Child: &target.FlowNodeTemplate{ID: "end", Type: "end"},
	}}
	flowAnalyzer := analyzer.NewFlowGraphAnalyzer()
	nodes, edges, _, err := flowAnalyzer.Analyze(tree)
	if err != nil {
		t.Fatal(err)
	}
	graph := model.FlowGraph{Nodes: nodes, Edges: edges, EntryNodeIDs: []string{"start"}}
	store := planmysql.NewHistoryReplayRepository(database.DB)
	pathConfig := service.NewPathConfigService(plans, f050TargetReader{tree}, flowAnalyzer, analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathConfigAnalyzer(), paths)
	pathConfig.SetHistoryWorkspaceStores(store, store)
	if err := pathConfig.AutoConfigurePathActions(ctx, plan.ID, path.ID, model.HistoryReplayActionModeApproveOnly); err != nil {
		t.Fatal(err)
	}
	record, found, err := store.GetPathConfig(ctx, path.ID)
	if err != nil || !found {
		t.Fatalf("读取准备配置失败：%v", err)
	}
	var actions []model.ConfiguredAction
	var steps []model.CompiledActionStep
	if err := json.Unmarshal(record.UserActions, &actions); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(record.CompiledSteps, &steps); err != nil {
		t.Fatal(err)
	}
	if len(actions) != 0 || len(steps) != 3 || record.NodeStatus != "configured" {
		t.Fatalf("默认场景必须真实落盘且不冒充额外动作：actions=%+v steps=%+v status=%s", actions, steps, record.NodeStatus)
	}
	// 数据独立准备：这里只替换临时库里的测试数据状态，不连接目标表单或提交业务数据。
	record.DataStatus, record.DataRevision = model.HistoryDataStatusBaseReady, record.DataRevision+1
	record.EffectiveFormData = []byte(`{"title":"临时库测试数据"}`)
	if _, err := store.SavePathConfig(ctx, record, record.Revision, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	paths = planmysql.NewExecutionPathRepository(reopened.DB)
	listed, err := paths.List(ctx, plan.ID)
	if err != nil || len(listed) != 1 || listed[0].ConfigurationStatus != "configured" || listed[0].DataStatus != "base_ready" {
		t.Fatalf("列表状态必须从真实落盘事实恢复：paths=%+v err=%v", listed, err)
	}
	readiness := service.NewRunReadinessService(service.NewPlanService(planmysql.NewPlanRepository(reopened.DB)), paths, f050GraphReader{graph}, planmysql.NewHistoryReplayRepository(reopened.DB), analyzer.NewExecutionPathAnalyzer())
	result, err := readiness.PlanReadiness(ctx, plan.ID, []uint64{path.ID})
	if err != nil || result.RunnableCount != 1 || result.BlockedCount != 0 {
		t.Fatalf("重连后的预检不应把纯同意当成待配置：result=%+v err=%v", result, err)
	}
}
