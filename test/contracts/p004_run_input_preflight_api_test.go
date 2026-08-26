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

type p004PreflightAPIStub struct {
	stubPathConfigurationService
	planID uint64
	pathID uint64
}

// PreflightRunInput 记录只读预检目标并返回不含完整目标请求体的摘要。
func (s *p004PreflightAPIStub) PreflightRunInput(_ context.Context, planID, pathID uint64) (model.RunInputPreflightResult, error) {
	s.planID, s.pathID = planID, pathID
	return model.RunInputPreflightResult{
		Status:   model.RunInputPreflightReady,
		Snapshot: model.RunInputSnapshot{Version: "p004-run-input/v1", PlanID: planID, PathID: pathID, FormValues: map[string]any{"title": "值"}, PathChoices: []model.ExecutionPathChoice{}},
		Target:   model.TargetSubmissionPreview{Method: "POST", Path: "/web/flowInstanceApi/submit", PayloadKeys: []string{"data", "formDataMongoVo"}, PayloadDigest: "digest", SuccessChecks: []string{"isSuccess"}},
		Issues:   []model.RunInputPreflightIssue{},
	}, nil
}

// TestP004RunInputPreflightAPIIsReadOnlyPreview 验证 GET 端点只返回快照和摘要，不暴露可直接发送的 payload。
func TestP004RunInputPreflightAPIIsReadOnlyPreview(t *testing.T) {
	stub := &p004PreflightAPIStub{}
	handler := api.NewHandlerWithConfigurationServices(&stubTargetReader{}, service.NewPlanService(&contractPlanRepository{}), &stubFlowGraphService{}, &stubExecutionPathService{}, &stubPathRequirementService{}, stub)
	request := httptest.NewRequest(http.MethodGet, "/api/plans/7/execution-paths/31/run-input/preflight", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || stub.planID != 7 || stub.pathID != 31 {
		t.Fatalf("运行输入预检端点错误：status=%d plan=%d path=%d body=%s", response.Code, stub.planID, stub.pathID, response.Body.String())
	}
	body := response.Body.String()
	if !strings.Contains(body, `"snapshotDigest"`) || !strings.Contains(body, `"payloadDigest":"digest"`) || strings.Contains(body, `"payload":`) {
		t.Fatalf("预检响应未保持只读摘要边界：%s", body)
	}
}
