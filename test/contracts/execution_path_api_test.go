package contracts_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/api"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

type stubExecutionPathService struct {
	items   []model.ExecutionPath
	path    model.ExecutionPath
	created bool
	err     error
	choices []model.ExecutionPathChoice
}

// List 返回契约测试预设的路径集合或错误。
func (s *stubExecutionPathService) List(context.Context, uint64) ([]model.ExecutionPath, error) {
	return s.items, s.err
}

// Create 记录浏览器提交的最小 choices 并返回预设创建结果。
func (s *stubExecutionPathService) Create(_ context.Context, _ uint64, _ string, choices []model.ExecutionPathChoice) (model.ExecutionPath, bool, error) {
	s.choices = choices
	return s.path, s.created, s.err
}

// Update 记录完整替换 choices 并返回预设路径。
func (s *stubExecutionPathService) Update(_ context.Context, _, _ uint64, choices []model.ExecutionPathChoice) (model.ExecutionPath, error) {
	s.choices = choices
	return s.path, s.err
}

// Delete 返回预设删除错误以覆盖稳定映射。
func (s *stubExecutionPathService) Delete(context.Context, uint64, uint64) error { return s.err }

// TestExecutionPathAPIFourOperationsAndSafety 验证四个端点和公开字段安全边界。
func TestExecutionPathAPIFourOperationsAndSafety(t *testing.T) {
	now := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	path := model.ExecutionPath{ID: 31, PlanID: 7, SequenceNo: 2, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route-a", BranchID: "branch-a"}}, UpdatedAt: now}
	stub := &stubExecutionPathService{items: []model.ExecutionPath{path}, path: path, created: true}
	handler := api.NewHandlerWithExecutionPathServices(&stubTargetReader{}, service.NewPlanService(&contractPlanRepository{}), &stubFlowGraphService{}, stub)

	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/plans/7/execution-paths", nil))
	assertExecutionPathResponse(t, list, http.StatusOK)

	createRequest := httptest.NewRequest(http.MethodPost, "/api/plans/7/execution-paths", strings.NewReader(`{"choices":[{"routeNodeId":"route-a","branchId":"branch-a"}]}`))
	createRequest.Header.Set("Idempotency-Key", "123e4567-e89b-12d3-a456-426614174301")
	create := httptest.NewRecorder()
	handler.ServeHTTP(create, createRequest)
	assertExecutionPathResponse(t, create, http.StatusCreated)

	update := httptest.NewRecorder()
	handler.ServeHTTP(update, httptest.NewRequest(http.MethodPut, "/api/plans/7/execution-paths/31", strings.NewReader(`{"choices":[]}`)))
	assertExecutionPathResponse(t, update, http.StatusOK)

	remove := httptest.NewRecorder()
	handler.ServeHTTP(remove, httptest.NewRequest(http.MethodDelete, "/api/plans/7/execution-paths/31", nil))
	if remove.Code != http.StatusNoContent || remove.Body.Len() != 0 {
		t.Fatalf("删除路径契约不正确：status=%d body=%s", remove.Code, remove.Body.String())
	}
}

// TestExecutionPathAPIRejectsUnknownFieldsAndMapsStableErrors 验证伪造字段拒绝和稳定错误码。
func TestExecutionPathAPIRejectsUnknownFieldsAndMapsStableErrors(t *testing.T) {
	tests := []struct {
		err    error
		status int
		code   string
	}{
		{err: &service.ExecutionPathError{Kind: service.ExecutionPathErrorNotFound}, status: 404, code: "EXECUTION_PATH_NOT_FOUND"},
		{err: &service.ExecutionPathError{Kind: service.ExecutionPathErrorInvalid}, status: 409, code: "EXECUTION_PATH_INVALID"},
		{err: &service.ExecutionPathError{Kind: service.ExecutionPathErrorLimit}, status: 409, code: "EXECUTION_PATH_LIMIT_REACHED"},
		{err: service.ErrTargetFlowNotConfigurable, status: 409, code: "TARGET_FLOW_NOT_CONFIGURABLE"},
		{err: &service.ExecutionPathError{Kind: service.ExecutionPathErrorLocked}, status: 409, code: "PLAN_LOCKED"},
		{err: &service.ExecutionPathError{Kind: service.ExecutionPathErrorStorage}, status: 503, code: "PLAN_STORAGE_UNAVAILABLE"},
	}
	for _, test := range tests {
		stub := &stubExecutionPathService{err: test.err}
		handler := api.NewHandlerWithExecutionPathServices(&stubTargetReader{}, service.NewPlanService(&contractPlanRepository{}), &stubFlowGraphService{}, stub)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/plans/7/execution-paths", nil))
		if recorder.Code != test.status || !strings.Contains(recorder.Body.String(), test.code) {
			t.Fatalf("路径稳定错误不正确：status=%d body=%s", recorder.Code, recorder.Body.String())
		}
	}

	handler := api.NewHandlerWithExecutionPathServices(&stubTargetReader{}, service.NewPlanService(&contractPlanRepository{}), &stubFlowGraphService{}, &stubExecutionPathService{})
	unknown := httptest.NewRecorder()
	handler.ServeHTTP(unknown, httptest.NewRequest(http.MethodPost, "/api/plans/7/execution-paths", strings.NewReader(`{"choices":[],"entryNodeIds":["forged"]}`)))
	if unknown.Code != http.StatusBadRequest || !strings.Contains(unknown.Body.String(), "INVALID_ARGUMENT") {
		t.Fatal("路径 API 接受了浏览器伪造入口")
	}
}

// TestPlanAPIExposesRealPathCount 验证计划公开响应采用持久化真实路径计数。
func TestPlanAPIExposesRealPathCount(t *testing.T) {
	repo := &contractPlanRepository{found: true, plan: model.Plan{
		ID: 7, Name: "路径计划", Account: "account", FlowSource: "new",
		TargetObjectID: "target", TargetObjectName: "流程", RunMode: "serial",
		Status: model.PlanStatusPendingConfiguration, PathCount: 3,
	}}
	handler := api.NewHandlerWithServices(&stubTargetReader{}, service.NewPlanService(repo))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/plans/7", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"pathCount":3`) {
		t.Fatalf("计划响应没有返回真实路径数量：status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "next_path_sequence_no") {
		t.Fatal("计划响应泄露内部稳定序号计数器")
	}
}

// assertExecutionPathResponse 核对路径响应只包含批准的公开字段。
func assertExecutionPathResponse(t *testing.T, recorder *httptest.ResponseRecorder, status int) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("路径 API 状态码 = %d，响应=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, want := range []string{`"id":"31"`, `"sequenceNo":2`, `"routeNodeId":"route-a"`, `"branchId":"branch-a"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("路径响应缺少 %s：%s", want, body)
		}
	}
	for _, forbidden := range []string{"create_key", "createKey", "entryNodeIds", "flowProxyId", "sid", "password", "targetObjectId"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("路径响应泄露禁止字段 %s", forbidden)
		}
	}
}
