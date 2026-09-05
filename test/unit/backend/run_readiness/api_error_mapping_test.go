package run_readiness_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/api"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// stubReadinessService 按预设错误返回，用于验证 API 层的错误映射行为。
type stubReadinessService struct {
	err error
}

// PlanReadiness 直接返回预设错误或空结论。
func (s *stubReadinessService) PlanReadiness(context.Context, uint64, []uint64) (model.PlanRunReadiness, error) {
	if s.err != nil {
		return model.PlanRunReadiness{}, s.err
	}
	return model.PlanRunReadiness{Summary: "全部可以启动", TotalCount: 1}, nil
}

// serveReadiness 用与生产相同的路由模式注册 handler 并发起请求，返回响应与状态码。
// 必须经 ServeMux 匹配，handler 里的 PathValue("id") 才能取到计划 ID。
func serveReadiness(t *testing.T, readiness api.RunReadinessService, query string) (int, map[string]any) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/plans/{id}/run-readiness", api.PlanRunReadinessHandler(readiness))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/plans/1/run-readiness"+query, nil)
	mux.ServeHTTP(recorder, request)
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应体不是 JSON：%v", err)
	}
	return recorder.Code, body
}

// errorBody 取响应里的错误对象（code 与 message）。
func errorBody(t *testing.T, body map[string]any) (string, string) {
	t.Helper()
	data, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("响应缺少错误对象：%v", body)
	}
	code, _ := data["code"].(string)
	message, _ := data["message"].(string)
	return code, message
}

// TestAPIErrorMappingCoversAllKinds 三种错误语义必须在决定状态码的那一层被验证：
// 计划不存在 404、参数不合法 400、账号问题 401 且不可重试、目标故障 502 可重试、存储故障 503 可重试。
func TestAPIErrorMappingCoversAllKinds(t *testing.T) {
	cases := []struct {
		name         string
		err          error
		wantStatus   int
		wantCode     string
		wantRetryMsg bool
	}{
		{"计划不存在", &service.RunReadinessError{Kind: service.RunReadinessErrorNotFound, Message: "计划不存在"},
			http.StatusNotFound, "RUN_READINESS_NOT_FOUND", false},
		{"计划 ID 不正确", &service.RunReadinessError{Kind: service.RunReadinessErrorInvalid, Message: "计划 ID 不正确"},
			http.StatusBadRequest, "RUN_READINESS_INVALID", false},
		{"账号验证失败", &service.RunReadinessError{Kind: service.RunReadinessErrorAuth, Message: "账号验证失败，请核对计划账号后重新验证"},
			http.StatusUnauthorized, "RUN_READINESS_TARGET_AUTH", false},
		{"目标结构读取失败", &service.RunReadinessError{Kind: service.RunReadinessErrorTarget, Message: "读取目标流程超时，请稍后重试"},
			http.StatusBadGateway, "RUN_READINESS_TARGET_UNAVAILABLE", true},
		{"存储故障", &service.RunReadinessError{Kind: service.RunReadinessErrorStorage, Message: "暂时无法读取计划，请重试"},
			http.StatusServiceUnavailable, "RUN_READINESS_STORAGE_UNAVAILABLE", true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			status, body := serveReadiness(t, &stubReadinessService{err: testCase.err}, "?pathIds=1")
			if status != testCase.wantStatus {
				t.Fatalf("状态码 = %d，期望 %d，响应：%v", status, testCase.wantStatus, body)
			}
			code, message := errorBody(t, body)
			if code != testCase.wantCode {
				t.Fatalf("错误码 = %s，期望 %s", code, testCase.wantCode)
			}
			if strings.TrimSpace(message) == "" {
				t.Fatalf("错误文案为空，缺少中文提示")
			}
			if retryable, _ := body["error"].(map[string]any)["retryable"].(bool); retryable != testCase.wantRetryMsg {
				t.Fatalf("retryable = %v，期望 %v", retryable, testCase.wantRetryMsg)
			}
		})
	}
}

// TestAPIRejectsMalformedPathIDs 勾选参数不合法必须 400，不得静默按全部路径处理。
func TestAPIRejectsMalformedPathIDs(t *testing.T) {
	status, body := serveReadiness(t, &stubReadinessService{}, "?pathIds=abc")
	if status != http.StatusBadRequest {
		t.Fatalf("状态码 = %d，期望 400，响应：%v", status, body)
	}
	if code, _ := errorBody(t, body); code != "RUN_READINESS_INVALID" {
		t.Fatalf("错误码 = %s，期望 RUN_READINESS_INVALID", code)
	}
}

// TestAPIPassesSelectedPathIDs 正常勾选参数必须原样传到服务层，预检范围与勾选范围一致。
func TestAPIPassesSelectedPathIDs(t *testing.T) {
	var got []uint64
	svc := &captureStub{capture: func(ids []uint64) { got = ids }}
	serveReadiness(t, svc, "?pathIds=3,7")
	if len(got) != 2 || got[0] != 3 || got[1] != 7 {
		t.Fatalf("服务层收到的勾选 = %v，期望 [3 7]", got)
	}
}

// captureStub 记录服务层收到的勾选参数。
type captureStub struct {
	stubReadinessService
	capture func([]uint64)
}

// PlanReadiness 先记录勾选再返回空结论。
func (c *captureStub) PlanReadiness(_ context.Context, _ uint64, ids []uint64) (model.PlanRunReadiness, error) {
	c.capture(ids)
	return model.PlanRunReadiness{}, nil
}
