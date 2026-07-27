package contracts_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/api"
	"test-auto-pro-v2/internal/config"
)

type stubTargetReader struct {
	verifyErr    error
	templatesErr error
	submittedErr error
	dueErr       error
}

func (s *stubTargetReader) Verify(_ context.Context, account string) (target.AccountSummary, error) {
	if s.verifyErr != nil {
		return target.AccountSummary{}, s.verifyErr
	}
	return target.AccountSummary{Account: account, DisplayName: "测试人员", CompanyName: "测试公司"}, nil
}

func (s *stubTargetReader) Templates(_ context.Context, _ string, _ string, page, pageSize int) (target.Page[target.FlowTemplate], error) {
	if s.templatesErr != nil {
		return target.Page[target.FlowTemplate]{}, s.templatesErr
	}
	return target.Page[target.FlowTemplate]{
		Items: []target.FlowTemplate{{ID: "template-id", FlowName: "测试流程", FlowStatus: "enable", StatusText: "正常"}},
		Page:  page, PageSize: pageSize, Total: 1,
	}, nil
}

func (s *stubTargetReader) Submitted(_ context.Context, _ string, _ string, page, pageSize int) (target.Page[target.SubmittedFlow], error) {
	if s.submittedErr != nil {
		return target.Page[target.SubmittedFlow]{}, s.submittedErr
	}
	return target.Page[target.SubmittedFlow]{
		Items: []target.SubmittedFlow{{ID: "submitted-id", Title: "已发流程", Status: "run"}},
		Page:  page, PageSize: pageSize, Total: 1,
	}, nil
}

func (s *stubTargetReader) Due(_ context.Context, _ string, _ string, page, pageSize int) (target.Page[target.DueFlow], error) {
	if s.dueErr != nil {
		return target.Page[target.DueFlow]{}, s.dueErr
	}
	return target.Page[target.DueFlow]{
		Items: []target.DueFlow{{FlowInstanceID: "due-id", Title: "待发流程", FlowStatus: "draft", StatusName: "草稿"}},
		Page:  page, PageSize: pageSize, Total: 1,
	}, nil
}

func TestTargetAPIContracts(t *testing.T) {
	handler := api.NewHandlerWithTargetReader(&stubTargetReader{})
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		fields []string
	}{
		{name: "验证账号", method: http.MethodPost, path: "/api/target/accounts/verify", body: `{"account":"account-a"}`, fields: []string{"verified", "displayName", "companyName"}},
		{name: "模板列表", method: http.MethodGet, path: "/api/target/flow-templates?account=account-a&query=test&page=1&pageSize=20", fields: []string{"flowName", "flowStatus", "hasMore"}},
		{name: "已发列表", method: http.MethodGet, path: "/api/target/flow-instances?account=account-a&source=submitted&page=1&pageSize=20", fields: []string{"submitted", "currentAuditUserNames", "hasMore"}},
		{name: "待发列表", method: http.MethodGet, path: "/api/target/flow-instances?account=account-a&source=due&page=1&pageSize=20", fields: []string{"due", "flowInstanceId", "statusName"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("状态码 = %d，响应不符合成功契约", recorder.Code)
			}
			body := recorder.Body.String()
			for _, field := range test.fields {
				if !strings.Contains(body, field) {
					t.Fatalf("响应缺少字段 %s", field)
				}
			}
			assertNoSensitiveJSONKeys(t, recorder.Body.Bytes())
		})
	}
}

func TestTargetAPIParameterBoundaries(t *testing.T) {
	handler := api.NewHandlerWithTargetReader(&stubTargetReader{})
	requests := []*http.Request{
		httptest.NewRequest(http.MethodPost, "/api/target/accounts/verify", bytes.NewBufferString(`{"account":""}`)),
		httptest.NewRequest(http.MethodGet, "/api/target/flow-templates?account=account-a&page=0", nil),
		httptest.NewRequest(http.MethodGet, "/api/target/flow-templates?account=account-a&pageSize=101", nil),
		httptest.NewRequest(http.MethodGet, "/api/target/flow-instances?account=account-a&source=unknown", nil),
	}
	for _, request := range requests {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "INVALID_ARGUMENT") {
			t.Fatalf("非法参数未返回稳定契约：status=%d", recorder.Code)
		}
		assertNoSensitiveJSONKeys(t, recorder.Body.Bytes())
	}
}

func TestTargetAPIErrorMapping(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		status    int
		code      string
		retryable bool
	}{
		{name: "配置缺失", err: &config.MissingTargetConfigError{Names: []string{"TARGET_API_GATEWAY"}}, status: 503, code: "TARGET_CONFIG_MISSING"},
		{name: "登录拒绝", err: target.NewError(target.ErrorLoginRejected, nil), status: 401, code: "TARGET_LOGIN_REJECTED"},
		{name: "会话失效", err: target.NewError(target.ErrorSessionExpired, nil), status: 401, code: "TARGET_SESSION_EXPIRED", retryable: true},
		{name: "响应异常", err: target.NewError(target.ErrorResponseInvalid, nil), status: 502, code: "TARGET_RESPONSE_INVALID", retryable: true},
		{name: "不可用", err: target.NewError(target.ErrorUnavailable, nil), status: 502, code: "TARGET_UNAVAILABLE", retryable: true},
		{name: "超时", err: target.NewError(target.ErrorTimeout, nil), status: 504, code: "TARGET_TIMEOUT", retryable: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := api.NewHandlerWithTargetReader(&stubTargetReader{verifyErr: test.err})
			request := httptest.NewRequest(http.MethodPost, "/api/target/accounts/verify", bytes.NewBufferString(`{"account":"account-a"}`))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != test.status || !strings.Contains(recorder.Body.String(), test.code) {
				t.Fatalf("错误映射不正确：status=%d", recorder.Code)
			}
			var response struct {
				Error struct {
					Retryable bool `json:"retryable"`
				} `json:"error"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("错误响应不是 JSON：%v", err)
			}
			if response.Error.Retryable != test.retryable {
				t.Fatal("retryable 与稳定契约不一致")
			}
			assertNoSensitiveJSONKeys(t, recorder.Body.Bytes())
		})
	}
}

func TestMissingTargetConfigDoesNotBreakHealth(t *testing.T) {
	for _, name := range []string{"TARGET_API_GATEWAY", "TARGET_LOGIN_PASSWORD", "TARGET_LOGIN_AES_KEY", "TARGET_LOGIN_CODE"} {
		t.Setenv(name, "")
	}
	handler := api.NewHandler()

	healthRecorder := httptest.NewRecorder()
	handler.ServeHTTP(healthRecorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if healthRecorder.Code != http.StatusOK {
		t.Fatal("目标配置缺失影响了 health")
	}

	targetRecorder := httptest.NewRecorder()
	handler.ServeHTTP(targetRecorder, httptest.NewRequest(http.MethodPost, "/api/target/accounts/verify", strings.NewReader(`{"account":"account-a"}`)))
	if targetRecorder.Code != http.StatusServiceUnavailable || !strings.Contains(targetRecorder.Body.String(), "TARGET_CONFIG_MISSING") {
		t.Fatal("目标配置缺失未返回稳定错误")
	}
}

func assertNoSensitiveJSONKeys(t *testing.T, body []byte) {
	t.Helper()
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatalf("响应不是有效 JSON：%v", err)
	}
	forbidden := map[string]struct{}{
		"sid": {}, "password": {}, "aeskey": {}, "customercode": {}, "platformcode": {}, "codevalue": {},
	}
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			for key, child := range typed {
				if _, blocked := forbidden[strings.ToLower(key)]; blocked {
					t.Fatalf("公开响应包含敏感字段 %s", key)
				}
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
}
