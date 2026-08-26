package backend_test

import (
	"context"
	"reflect"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// TestP003FormConfigurationProjectsReadManifest 验证规则版本和模板只读数据源进入 iframe 清单且去重、去查询串。
func TestP003FormConfigurationProjectsReadManifest(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 301, Account: "account", FlowSource: "new", TargetObjectID: "template", TargetObjectName: "清单表单", Status: model.PlanStatusNotStarted}}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 302, PlanID: 301, SequenceNo: 1, Name: "路径 1"}}}
	tree := &target.FlowNodeTemplate{ID: "start", Name: "发起", Type: "start", Child: &target.FlowNodeTemplate{ID: "end", Name: "结束", Type: "end"}}
	template := `{"list":[
		{"type":"select","model":"project","options":{"requestURL":"http://target.test/web/project/options?company=1","requestMethod":"post"}},
		{"type":"select","model":"projectCopy","options":{"url":"/web/project/options?company=2","method":"POST"}},
		{"type":"cascader","model":"company","options":{"url":"/api/web/company/tree","method":"GET"}},
		{"type":"custom","el":"custome-select-project","model":"linkedProject","options":{}}
	]}`
	serviceUnderTest := service.NewPathConfigService(
		service.NewPlanService(plans),
		pathConfigSnapshotReader{snapshot: target.PathConfigurationSnapshot{
			Tree: tree, EntryNodeIDs: []string{"start"}, RuleVersion: "rule-p003", RenderType: target.FormRenderTypeFormMaking,
			Forms: []target.FormRuntimeTemplate{{Name: "申请表", TemplateData: template}},
		}},
		analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathConfigAnalyzer(), paths, emptyPathConfigRepository{},
	)
	configuration, err := serviceUnderTest.Get(context.Background(), 301, 302)
	if err != nil {
		t.Fatalf("读取表单只读请求清单失败：%v", err)
	}
	want := []model.PathFormReadRequest{
		{Method: "GET", Path: "/api/web/company/tree", Source: "formmaking_template"},
		{Method: "POST", Path: "/web/project/api/findById", Source: "component_capability"},
		{Method: "POST", Path: "/web/project/api/getProjectVosOfCompanyAndGroup", Source: "component_capability"},
		{Method: "POST", Path: "/web/project/options", Source: "formmaking_template"},
		{Method: "POST", Path: "/web/user/api/company/children", Source: "component_capability"},
	}
	if configuration.Form.RuleVersion != "rule-p003" || !reflect.DeepEqual(configuration.Form.ReadRequests, want) {
		t.Fatalf("规则版本或只读清单投影错误：version=%q requests=%+v", configuration.Form.RuleVersion, configuration.Form.ReadRequests)
	}
}
