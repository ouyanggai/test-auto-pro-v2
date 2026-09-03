package f012_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"test-auto-pro-v2/internal/api"
)

// TestF012RemovedAPIsReturnNotFound 验证已删除旧接口不会转发到新协议或返回兼容载荷。
func TestF012RemovedAPIsReturnNotFound(t *testing.T) {
	handler := api.NewHandler()
	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/plans/1/execution-paths/2/configuration/form/generate"},
		{http.MethodPut, "/api/plans/1/execution-paths/2/configuration/form"},
		{http.MethodPut, "/api/plans/1/execution-paths/2/configuration/selection"},
		{http.MethodPost, "/api/plans/1/execution-paths/2/configuration/cycles/copy"},
		{http.MethodGet, "/api/plans/1/execution-paths/2/run-input/preflight"},
		{http.MethodPost, "/api/plans/1/execution-paths/2/path-preparations"},
		{http.MethodGet, "/api/plans/1/path-preparations/active"},
		{http.MethodGet, "/api/template-rules"},
		{http.MethodGet, "/api/template-rules/analysis-jobs/active"},
	}
	for _, testCase := range cases {
		request := httptest.NewRequest(testCase.method, testCase.path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("旧接口未返回 404：%s %s status=%d body=%s", testCase.method, testCase.path, response.Code, response.Body.String())
		}
	}
}
