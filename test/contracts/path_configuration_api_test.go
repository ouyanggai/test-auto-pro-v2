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

type stubPathConfigurationService struct {
	configuration model.PathConfiguration
	result        model.PathConfigSaveResult
	err           error
	planID        uint64
	pathID        uint64
	key           string
	revision      uint64
	fields        []model.PathConfigFieldValue
	actions       []model.PathConfigActionValue
}

// Get 返回契约测试预设的配置模型或稳定错误。
func (s *stubPathConfigurationService) Get(context.Context, uint64, uint64) (model.PathConfiguration, error) {
	return s.configuration, s.err
}

// Save 记录浏览器最小回写体并返回预设保存结果。
func (s *stubPathConfigurationService) Save(_ context.Context, planID, pathID uint64, key string, revision uint64, fields []model.PathConfigFieldValue, actions []model.PathConfigActionValue) (model.PathConfigSaveResult, error) {
	s.planID, s.pathID, s.key, s.revision = planID, pathID, key, revision
	s.fields, s.actions = fields, actions
	if strings.TrimSpace(key) == "" {
		return model.PathConfigSaveResult{}, &service.PathConfigError{Kind: service.PathConfigErrorInvalidArgument, Message: "保存标识不正确"}
	}
	return s.result, s.err
}

// newConfigurationHandler 组装包含路径配置端点的完整路由用于契约测试。
func newConfigurationHandler(configurations api.PathConfigurationService) http.Handler {
	return api.NewHandlerWithConfigurationServices(
		&stubTargetReader{}, service.NewPlanService(&contractPlanRepository{}), &stubFlowGraphService{},
		&stubExecutionPathService{}, &stubPathRequirementService{}, configurations,
	)
}

// TestPathConfigurationAPIGetAndPutContracts 验证读取与保存路由、修订号和公开字段边界。
func TestPathConfigurationAPIGetAndPutContracts(t *testing.T) {
	stub := &stubPathConfigurationService{
		configuration: model.PathConfiguration{
			Path: model.PathConfigPath{SequenceNo: 2, Name: "财务路径"}, Revision: 3, Status: "configured",
			Groups: []model.PathConfigGroup{{Title: "主线", Kind: "main", Nodes: []model.PathConfigNode{{
				Name: "财务审批", TypeName: "审批", Kind: "common",
				Fields: []model.PathConfigField{{
					Key: "opaque-field-key", Name: "申请金额", Type: "number", Required: true,
					Value: "2500", Options: []model.PathConfigOption{}, Editable: true,
				}},
				Actions: []model.PathConfigAction{{Key: "opaque-action-key", Kind: "agree_disagree", Label: "处理结果", Current: "agree", Default: "agree", Options: []model.PathConfigActionOption{{Value: "agree", Label: "同意"}, {Value: "disagree", Label: "不同意"}}}},
			}}}},
		},
		result: model.PathConfigSaveResult{Path: model.PathConfigPath{SequenceNo: 2, Name: "财务路径"}, Revision: 4, Status: "configured"},
	}
	handler := newConfigurationHandler(stub)
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/plans/7/execution-paths/31/configuration", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("配置读取状态不正确：%d %s", get.Code, get.Body.String())
	}
	getBody := get.Body.String()
	for _, want := range []string{`"sequenceNo":2`, `"name":"财务路径"`, `"revision":3`, `"opaque-field-key"`, `"申请金额"`, `"agree"`} {
		if !strings.Contains(getBody, want) {
			t.Fatalf("配置读取响应缺少 %s：%s", want, getBody)
		}
	}
	for _, forbidden := range []string{"englishName", "nodeId", "branchId", "flowProxyId", "formTemplateId", "sid", "password", "templateData", "formDataMongoVo"} {
		if strings.Contains(getBody, forbidden) {
			t.Fatalf("配置读取响应泄露禁止字段 %s：%s", forbidden, getBody)
		}
	}

	put := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/plans/7/execution-paths/31/configuration", strings.NewReader(`{"revision":3,"fields":[{"key":"opaque-field-key","value":"2600"}],"actions":[{"key":"opaque-action-key","action":"disagree"}]}`))
	request.Header.Set("Idempotency-Key", "123e4567-e89b-12d3-a456-426614174601")
	handler.ServeHTTP(put, request)
	if put.Code != http.StatusOK || !strings.Contains(put.Body.String(), `"revision":4`) {
		t.Fatalf("配置保存状态不正确：%d %s", put.Code, put.Body.String())
	}
	if stub.planID != 7 || stub.pathID != 31 || stub.key != "123e4567-e89b-12d3-a456-426614174601" || stub.revision != 3 || len(stub.fields) != 1 || stub.actions[0].Action != "disagree" {
		t.Fatalf("配置保存没有透传最小回写体：plan=%d path=%d key=%s revision=%d fields=%+v actions=%+v", stub.planID, stub.pathID, stub.key, stub.revision, stub.fields, stub.actions)
	}

	missingKey := httptest.NewRecorder()
	handler.ServeHTTP(missingKey, httptest.NewRequest(http.MethodPut, "/api/plans/7/execution-paths/31/configuration", strings.NewReader(`{"revision":3,"fields":[],"actions":[]}`)))
	if missingKey.Code != http.StatusBadRequest || !strings.Contains(missingKey.Body.String(), "INVALID_ARGUMENT") {
		t.Fatalf("缺少幂等键没有被拒绝：%d %s", missingKey.Code, missingKey.Body.String())
	}
	unknown := httptest.NewRecorder()
	handler.ServeHTTP(unknown, httptest.NewRequest(http.MethodPut, "/api/plans/7/execution-paths/31/configuration", strings.NewReader(`{"revision":3,"forged":true}`)))
	if unknown.Code != http.StatusBadRequest || !strings.Contains(unknown.Body.String(), "INVALID_ARGUMENT") {
		t.Fatalf("配置 API 接受了伪造字段：%d %s", unknown.Code, unknown.Body.String())
	}
}

// TestPathConfigurationAPIMapsStableErrors 验证配置读写稳定错误码与受影响项目详情。
func TestPathConfigurationAPIMapsStableErrors(t *testing.T) {
	tests := []struct {
		err    error
		status int
		code   string
	}{
		{err: &service.PathConfigError{Kind: service.PathConfigErrorNotFound}, status: 404, code: "EXECUTION_PATH_NOT_FOUND"},
		{err: &service.PathConfigError{Kind: service.PathConfigErrorLocked}, status: 409, code: "PLAN_LOCKED"},
		{err: &service.PathConfigError{Kind: service.PathConfigErrorRevisionConflict}, status: 409, code: "CONFIG_REVISION_CONFLICT"},
		{err: &service.PathConfigError{Kind: service.PathConfigErrorInvalid, Message: "配置不完整", Affected: []model.PathConfigAffectedItem{{Kind: "field", Name: "申请金额", Reason: "必填字段不能为空"}}}, status: 409, code: "CONFIG_INVALID"},
		{err: &service.PathConfigError{Kind: service.PathConfigErrorStorage}, status: 503, code: "PLAN_STORAGE_UNAVAILABLE"},
	}
	for _, test := range tests {
		stub := &stubPathConfigurationService{err: test.err}
		handler := newConfigurationHandler(stub)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/plans/7/execution-paths/31/configuration", nil))
		if recorder.Code != test.status || !strings.Contains(recorder.Body.String(), test.code) {
			t.Fatalf("配置稳定错误不正确：status=%d body=%s", recorder.Code, recorder.Body.String())
		}
		if test.code == "CONFIG_INVALID" && !strings.Contains(recorder.Body.String(), "必填字段不能为空") {
			t.Fatalf("配置错误没有携带受影响项目详情：%s", recorder.Body.String())
		}
	}

	unavailable := httptest.NewRecorder()
	handler := api.NewHandlerWithRequirementServices(
		&stubTargetReader{}, service.NewPlanService(&contractPlanRepository{}), &stubFlowGraphService{},
		&stubExecutionPathService{}, &stubPathRequirementService{},
	)
	handler.ServeHTTP(unavailable, httptest.NewRequest(http.MethodGet, "/api/plans/7/execution-paths/31/configuration", nil))
	if unavailable.Code != http.StatusServiceUnavailable {
		t.Fatalf("缺省配置服务没有返回存储不可用：%d", unavailable.Code)
	}
}
