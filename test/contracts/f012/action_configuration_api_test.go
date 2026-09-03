package f012_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/api"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	"test-auto-pro-v2/internal/service"
)

// actionConfigurationAPIStub 记录 F-012 动作保存请求，不执行目标平台写操作。
type actionConfigurationAPIStub struct {
	api.PathConfigurationService
	planID, pathID uint64
	nodeKey        string
	input          model.ActionConfigurationInput
	idempotency    string
}

// actionTargetReader 仅用于组装路由；本契约请求不触发目标读取。
type actionTargetReader struct{ api.TargetReader }

// actionFlowGraphService 仅用于组装路由；本契约请求不读取流程图。
type actionFlowGraphService struct{ api.FlowGraphService }

// actionExecutionPathService 仅用于组装路由；本契约请求不读取执行路径。
type actionExecutionPathService struct{ api.ExecutionPathService }

// actionPathRequirementService 仅用于组装路由；本契约请求不读取路径要求。
type actionPathRequirementService struct{ api.PathRequirementService }

// actionPlanRepository 仅用于组装计划服务；动作契约请求不访问计划数据库。
type actionPlanRepository struct{ repository.PlanRepository }

// GetActionConfiguration 返回固定只读动作场景，供契约验证预览字段。
func (s *actionConfigurationAPIStub) GetActionConfiguration(_ context.Context, planID, pathID uint64) (model.ActionConfigurationResult, error) {
	return model.ActionConfigurationResult{Path: model.PathConfigPath{SequenceNo: 1, Name: "主路径"}, Revision: 4, NodeRevision: 2, ActionRevision: 2, Status: "configured", Actions: []model.ConfiguredAction{}, CompiledScenario: []model.CompiledActionStep{}}, nil
}

// GetCompiledScenario 返回固定编译结果，证明步骤由服务端读取而不是浏览器提交。
func (s *actionConfigurationAPIStub) GetCompiledScenario(_ context.Context, planID, pathID uint64) (model.ActionConfigurationResult, error) {
	s.planID, s.pathID = planID, pathID
	return model.ActionConfigurationResult{Path: model.PathConfigPath{SequenceNo: 1, Name: "主路径"}, Revision: 4, NodeRevision: 2, ActionRevision: 2, Status: "configured", CompiledScenario: []model.CompiledActionStep{{Sequence: 1, Source: model.ActionStepSourceNavigation, Action: model.ActionSystemAutomatic}}}, nil
}

// SaveActionConfiguration 记录语义动作和节点键，响应只返回服务端编译结果。
func (s *actionConfigurationAPIStub) SaveActionConfiguration(_ context.Context, planID, pathID uint64, nodeKey, idempotency string, input model.ActionConfigurationInput) (model.ActionConfigurationResult, error) {
	s.planID, s.pathID, s.nodeKey, s.idempotency, s.input = planID, pathID, nodeKey, idempotency, input
	return model.ActionConfigurationResult{Path: model.PathConfigPath{SequenceNo: 1, Name: "主路径"}, Revision: input.Revision + 1, NodeRevision: input.Revision + 1, ActionRevision: 1, Status: "configured", Actions: input.Actions, CompiledScenario: []model.CompiledActionStep{{Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionApprove}}}, nil
}

// TestActionConfigurationAPIUsesSemanticPayload 验证节点保存接收独立动作记录并由服务端返回编译结果。
func TestActionConfigurationAPIUsesSemanticPayload(t *testing.T) {
	stub := &actionConfigurationAPIStub{}
	handler := api.NewHandlerWithConfigurationServices(actionTargetReader{}, service.NewPlanService(actionPlanRepository{}), actionFlowGraphService{}, actionExecutionPathService{}, actionPathRequirementService{}, stub)
	body := `{"revision":4,"persons":[{"key":"person-token","strategy":"manual","seed":1,"selected":["candidate-token"]}],"actions":[{"key":"approve-1","action":"approve","scope":"task","nodeKey":"node-semantic","order":1,"actorPolicy":"current"}]}`
	request := httptest.NewRequest(http.MethodPut, "/api/plans/41/execution-paths/51/configuration/nodes/node-semantic", strings.NewReader(body))
	request.Header.Set("Idempotency-Key", "123e4567-e89b-12d3-a456-426614174701")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || stub.planID != 41 || stub.pathID != 51 || stub.nodeKey != "node-semantic" || stub.idempotency == "" {
		t.Fatalf("动作配置保存契约不正确：status=%d stub=%+v body=%s", response.Code, stub, response.Body.String())
	}
	if len(stub.input.Persons) != 1 || stub.input.Persons[0].Key != "person-token" || len(stub.input.Actions) != 1 || stub.input.Actions[0].Action != model.ActionApprove || stub.input.Actions[0].Parameters != nil {
		t.Fatalf("动作语义请求解析错误：%+v", stub.input)
	}
	if !strings.Contains(response.Body.String(), `"compiledScenario"`) || strings.Contains(response.Body.String(), "targetInstanceId") {
		t.Fatalf("动作保存响应缺少服务端编译结果或泄露目标身份：%s", response.Body.String())
	}
	for _, body := range []string{`{"revision":4,"compiledScenario":[],"actions":[]}`, `{"revision":4,"targetInstanceId":"target-1","actions":[]}`} {
		request = httptest.NewRequest(http.MethodPut, "/api/plans/41/execution-paths/51/configuration/nodes/node-semantic", strings.NewReader(body))
		request.Header.Set("Idempotency-Key", "123e4567-e89b-12d3-a456-426614174702")
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("动作配置接受了禁止字段：body=%s status=%d response=%s", body, response.Code, response.Body.String())
		}
	}
}

// TestCompiledScenarioAPIDoesNotAcceptBrowserSteps 验证只读场景端点不接受浏览器伪造编译步骤。
func TestCompiledScenarioAPIDoesNotAcceptBrowserSteps(t *testing.T) {
	stub := &actionConfigurationAPIStub{}
	handler := api.NewHandlerWithConfigurationServices(actionTargetReader{}, service.NewPlanService(actionPlanRepository{}), actionFlowGraphService{}, actionExecutionPathService{}, actionPathRequirementService{}, stub)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/plans/41/execution-paths/51/configuration/compiled-scenario", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"source":"system_navigation"`) || stub.planID != 41 || stub.pathID != 51 {
		t.Fatalf("只读编译场景契约不正确：status=%d stub=%+v body=%s", response.Code, stub, response.Body.String())
	}
}

// TestCompiledScenarioAPIRejectsWriteMethods 验证只读编译场景端点不提供任何写方法，浏览器无法覆盖服务端步骤。
func TestCompiledScenarioAPIRejectsWriteMethods(t *testing.T) {
	stub := &actionConfigurationAPIStub{}
	handler := api.NewHandlerWithConfigurationServices(actionTargetReader{}, service.NewPlanService(actionPlanRepository{}), actionFlowGraphService{}, actionExecutionPathService{}, actionPathRequirementService{}, stub)
	for _, method := range []string{http.MethodPut, http.MethodPost, http.MethodPatch, http.MethodDelete} {
		body := `{"compiledScenario":[{"sequence":1,"source":"user","action":"approve","scope":"task"}]}`
		request := httptest.NewRequest(method, "/api/plans/41/execution-paths/51/configuration/compiled-scenario", strings.NewReader(body))
		request.Header.Set("Idempotency-Key", "123e4567-e89b-12d3-a456-426614174711")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code == http.StatusOK {
			t.Fatalf("只读编译场景端点接受了写方法：method=%s response=%s", method, response.Body.String())
		}
	}
	if stub.nodeKey != "" || len(stub.input.Actions) != 0 {
		t.Fatalf("只读编译场景端点写入了动作配置：%+v", stub)
	}
}

// TestInstanceActionConfigurationUsesInstanceContainer 验证实例作用域动作在实例容器保存且不绑定语义节点。
func TestInstanceActionConfigurationUsesInstanceContainer(t *testing.T) {
	stub := &actionConfigurationAPIStub{}
	handler := api.NewHandlerWithConfigurationServices(actionTargetReader{}, service.NewPlanService(actionPlanRepository{}), actionFlowGraphService{}, actionExecutionPathService{}, actionPathRequirementService{}, stub)
	instanceKey := analyzer.PathConfigInstanceActionKey()
	body := `{"revision":4,"actions":[{"key":"withdraw-1","action":"withdraw","scope":"instance","order":1},{"key":"urge-1","action":"urge","scope":"instance","order":2}]}`
	request := httptest.NewRequest(http.MethodPut, "/api/plans/41/execution-paths/51/configuration/nodes/"+instanceKey, strings.NewReader(body))
	request.Header.Set("Idempotency-Key", "123e4567-e89b-12d3-a456-426614174712")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || stub.nodeKey != instanceKey {
		t.Fatalf("实例动作容器保存契约不正确：status=%d nodeKey=%s response=%s", response.Code, stub.nodeKey, response.Body.String())
	}
	if len(stub.input.Actions) != 2 || stub.input.Actions[0].Action != model.ActionWithdraw || stub.input.Actions[0].Scope != model.ActionScopeInstance || stub.input.Actions[0].NodeKey != "" {
		t.Fatalf("实例动作请求解析错误：%+v", stub.input.Actions)
	}
}
