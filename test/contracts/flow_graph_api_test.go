package contracts_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/api"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

type stubFlowGraphService struct {
	graph model.FlowGraph
	err   error
}

func (s *stubFlowGraphService) Get(context.Context, uint64) (model.FlowGraph, error) {
	return s.graph, s.err
}

func TestFlowGraphAPISuccessContractAndSafety(t *testing.T) {
	graphs := &stubFlowGraphService{graph: model.FlowGraph{
		PlanID: 41, TargetName: "采购流程", FlowSource: "new",
		Nodes: []model.FlowGraphNode{
			{ID: "start", Name: "发起", Type: "start", TypeName: "发起"},
			{ID: "route", Name: "条件", Type: "condition", TypeName: "条件", MergeTargetID: "merge"},
		},
		Edges:    []model.FlowGraphEdge{{ID: "start|route|sequence|", Source: "start", Target: "route", Kind: "sequence"}},
		Warnings: []string{},
	}}
	handler := apiHandlerWithGraph(graphs)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/plans/41/flow-graph", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("流程图成功状态码 = %d", recorder.Code)
	}
	body := recorder.Body.String()
	for _, field := range []string{`"planId":"41"`, `"targetName":"采购流程"`, `"flowSource":"new"`, `"typeName":"发起"`, `"mergeTargetId":"merge"`, `"warnings":[]`} {
		if !strings.Contains(body, field) {
			t.Fatalf("流程图响应缺少 %s", field)
		}
	}
	if strings.Contains(body, `"id":"start","name":"发起","type":"start","typeName":"发起","mergeTargetId"`) {
		t.Fatal("普通节点不应输出空汇合提示")
	}
	for _, forbidden := range []string{"flowProxyId", "sid", "password", "approval", "fieldPower", "customerCode", "platformCode"} {
		if strings.Contains(strings.ToLower(body), strings.ToLower(forbidden)) {
			t.Fatalf("流程图公开响应泄露禁止字段 %s", forbidden)
		}
	}
}

func TestFlowGraphAPIStableErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "计划不存在", err: &service.PlanError{Kind: service.PlanErrorNotFound}, status: 404, code: "PLAN_NOT_FOUND"},
		{name: "目标不可见", err: service.ErrTargetFlowNotFound, status: 404, code: "TARGET_FLOW_NOT_FOUND"},
		{name: "空结构", err: service.ErrTargetFlowStructureEmpty, status: 422, code: "TARGET_FLOW_STRUCTURE_EMPTY"},
		{name: "结构异常", err: analyzer.ErrFlowStructureInvalid, status: 502, code: "TARGET_FLOW_STRUCTURE_INVALID"},
		{name: "登录失败", err: target.NewError(target.ErrorLoginRejected, nil), status: 401, code: "TARGET_LOGIN_REJECTED"},
		{name: "会话失效", err: target.NewError(target.ErrorSessionExpired, nil), status: 401, code: "TARGET_SESSION_EXPIRED"},
		{name: "配置缺失", err: &config.MissingTargetConfigError{Names: []string{"TARGET_API_GATEWAY"}}, status: 503, code: "TARGET_CONFIG_MISSING"},
		{name: "不可用", err: target.NewError(target.ErrorUnavailable, nil), status: 503, code: "TARGET_UNAVAILABLE"},
		{name: "超时", err: target.NewError(target.ErrorTimeout, nil), status: 504, code: "TARGET_TIMEOUT"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := apiHandlerWithGraph(&stubFlowGraphService{err: test.err})
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/plans/41/flow-graph", nil))
			if recorder.Code != test.status || !strings.Contains(recorder.Body.String(), test.code) {
				t.Fatalf("稳定错误不正确：status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}

	invalid := httptest.NewRecorder()
	apiHandlerWithGraph(&stubFlowGraphService{}).ServeHTTP(invalid, httptest.NewRequest(http.MethodGet, "/api/plans/not-number/flow-graph", nil))
	if invalid.Code != http.StatusBadRequest || !strings.Contains(invalid.Body.String(), "INVALID_ARGUMENT") {
		t.Fatal("非法计划 ID 未返回稳定错误")
	}
}

func apiHandlerWithGraph(graphs *stubFlowGraphService) http.Handler {
	return api.NewHandlerWithFlowGraphServices(&stubTargetReader{}, service.NewPlanService(&contractPlanRepository{}), graphs)
}
