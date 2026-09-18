package run_orchestration_test

import (
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// TestF037RuntimeProjectionUsesTargetTreePaths 验证嵌套条件分支没有直接汇合子节点时，
// 加签导致所有节点 ID 重建仍按目标树真实路径完成投影，不再从汇合图边猜测路径。
func TestF037RuntimeProjectionUsesTargetTreePaths(t *testing.T) {
	staticTree := projectionTree("old")
	dynamicTree := projectionTree("new")
	analyzerImpl := analyzer.NewFlowGraphAnalyzer()
	staticNodes, staticEdges, _, err := analyzerImpl.Analyze(staticTree)
	if err != nil {
		t.Fatalf("分析旧代理树失败：%v", err)
	}
	dynamicNodes, dynamicEdges, _, err := analyzerImpl.Analyze(dynamicTree)
	if err != nil {
		t.Fatalf("分析新代理树失败：%v", err)
	}
	staticGraph := model.FlowGraph{Nodes: staticNodes, Edges: staticEdges}
	dynamicGraph := model.FlowGraph{Nodes: dynamicNodes, Edges: dynamicEdges}
	projected, bindable := service.ProjectRuntimeGraphForTest(staticGraph, dynamicGraph)
	if !bindable {
		t.Fatal("合法的目标代理树重建不应判定为无法绑定")
	}
	if len(projected.Nodes) != len(staticNodes) {
		t.Fatalf("投影节点数 = %d，期望 %d", len(projected.Nodes), len(staticNodes))
	}
	for _, node := range projected.Nodes {
		if node.ID == "" || node.StructuralPath == "" {
			t.Fatalf("投影节点缺少结构事实：%+v", node)
		}
	}
}

// TestF037AuditPeopleComesFromTargetDetails 验证加签人员来自目标审批明细，而不是伪造流程节点。
func TestF037AuditPeopleComesFromTargetDetails(t *testing.T) {
	root := projectionTree("people")
	root.Child.ConditionNodes[0].Child.ConditionNodes[0].Child.AuditConfig = &target.FlowNodeAuditConfig{Details: []target.FlowAuditDetail{
		{Name: " 骆蒙恩 "}, {Name: "骆蒙恩"}, {Name: ""},
	}}
	nodes, _, _, err := analyzer.NewFlowGraphAnalyzer().Analyze(root)
	if err != nil {
		t.Fatalf("分析代理树失败：%v", err)
	}
	for _, node := range nodes {
		if node.ID == "people-audit" {
			if len(node.AuditPeople) != 1 || node.AuditPeople[0] != "骆蒙恩" {
				t.Fatalf("审批明细人员投影错误：%+v", node.AuditPeople)
			}
			return
		}
	}
	t.Fatalf("未找到审批节点：%+v", nodes)
}

// projectionTree 构造目标平台允许的嵌套条件：内层条件没有 child，由外层统一汇合。
func projectionTree(prefix string) *target.FlowNodeTemplate {
	return &target.FlowNodeTemplate{
		ID: prefix + "-root", Type: "start", Name: "发起人",
		Child: &target.FlowNodeTemplate{
			ID: prefix + "-outer", Type: "condition", Name: "外层条件",
			ConditionNodes: []target.FlowBranchTemplate{{
				ID: prefix + "-branch", Sort: 1, Name: "条件一",
				Child: &target.FlowNodeTemplate{
					ID: prefix + "-inner", Type: "condition", Name: "内层条件",
					ConditionNodes: []target.FlowBranchTemplate{{
						ID: prefix + "-inner-branch", Sort: 1,
						Child: &target.FlowNodeTemplate{ID: prefix + "-audit", Type: "common", Name: "审批人"},
					}},
				},
			}},
			Child: &target.FlowNodeTemplate{ID: prefix + "-end", Type: "end", Name: "结束"},
		},
	}
}
