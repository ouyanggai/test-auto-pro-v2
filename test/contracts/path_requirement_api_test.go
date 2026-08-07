package contracts_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/api"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

type stubPathRequirementService struct {
	result model.PathRequirements
	err    error
}

// Get 返回契约测试预设的路径要求或稳定错误。
func (s *stubPathRequirementService) Get(context.Context, uint64, uint64) (model.PathRequirements, error) {
	return s.result, s.err
}

// TestPathRequirementAPIReadOnlyContractAndSafety 验证只读路由、中文状态和公开字段边界。
func TestPathRequirementAPIReadOnlyContractAndSafety(t *testing.T) {
	stub := &stubPathRequirementService{result: model.PathRequirements{
		Path:    model.PathRequirementPath{SequenceNo: 2, Name: "财务路径"},
		Summary: []model.RequirementCount{{Status: model.RequirementPending, Count: 2}, {Status: model.RequirementReview, Count: 1}},
		Groups: []model.RequirementGroup{{Title: "主线", Kind: "main", Nodes: []model.RequirementNode{{
			Name: "财务审批", TypeName: "审批", Items: []model.RequirementItem{{Category: "条件", Title: "大额", Detail: "申请金额大于 10000", Status: model.RequirementPending}},
		}}}},
	}}
	handler := api.NewHandlerWithRequirementServices(
		&stubTargetReader{}, service.NewPlanService(&contractPlanRepository{}), &stubFlowGraphService{}, &stubExecutionPathService{}, stub,
	)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/plans/7/execution-paths/31/requirements", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("路径要求状态不正确：%d %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, want := range []string{`"sequenceNo":2`, `"name":"财务路径"`, `"status":"待配置"`, `"title":"主线"`, `"detail":"申请金额大于 10000"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("路径要求响应缺少 %s：%s", want, body)
		}
	}
	for _, forbidden := range []string{"routeNodeId", "branchId", "flowProxyId", "formFieldTemplateEnglishName", "englishName", "auditType", "judge", "sid", "password", "raw"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("路径要求响应泄露禁止字段 %s：%s", forbidden, body)
		}
	}

	method := httptest.NewRecorder()
	handler.ServeHTTP(method, httptest.NewRequest(http.MethodPost, "/api/plans/7/execution-paths/31/requirements", nil))
	if method.Code != http.StatusMethodNotAllowed {
		t.Fatalf("路径要求错误开放写方法：%d", method.Code)
	}
}

// TestPathRequirementAPIMapsOwnershipAndInvalidPathErrors 验证归属错误和当前路径失效使用既有稳定契约。
func TestPathRequirementAPIMapsOwnershipAndInvalidPathErrors(t *testing.T) {
	tests := []struct {
		err    error
		status int
		code   string
	}{
		{err: &service.ExecutionPathError{Kind: service.ExecutionPathErrorNotFound}, status: http.StatusNotFound, code: "EXECUTION_PATH_NOT_FOUND"},
		{err: &service.ExecutionPathError{Kind: service.ExecutionPathErrorInvalid}, status: http.StatusConflict, code: "EXECUTION_PATH_INVALID"},
		{err: service.ErrTargetFlowNotConfigurable, status: http.StatusConflict, code: "TARGET_FLOW_NOT_CONFIGURABLE"},
	}
	for _, test := range tests {
		handler := api.NewHandlerWithRequirementServices(
			&stubTargetReader{}, service.NewPlanService(&contractPlanRepository{}), &stubFlowGraphService{}, &stubExecutionPathService{},
			&stubPathRequirementService{err: test.err},
		)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/plans/7/execution-paths/31/requirements", nil))
		if recorder.Code != test.status || !strings.Contains(recorder.Body.String(), test.code) {
			t.Fatalf("路径要求稳定错误不正确：status=%d body=%s", recorder.Code, recorder.Body.String())
		}
	}
}
