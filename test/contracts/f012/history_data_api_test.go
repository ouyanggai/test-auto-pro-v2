package f012_test

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

type historyAPIStub struct {
	candidatePlanID uint64
	candidatePathID uint64
	page            int
	pageSize        int
	query           string
	defaultPlanID   uint64
	defaultInput    model.HistoryDefaultSaveInput
	pathPlanID      uint64
	pathID          uint64
	pathInput       model.HistoryPathSourceInput
	idempotencyKey  string
	defaultErr      error
}

// Candidates 记录服务端解析后的计划上下文并返回安全摘要。
func (s *historyAPIStub) Candidates(_ context.Context, planID, pathID uint64, query string, page, pageSize int) (model.HistoryCandidatePage, error) {
	s.candidatePlanID, s.candidatePathID, s.query, s.page, s.pageSize = planID, pathID, query, page, pageSize
	key := strings.Repeat("a", 64)
	return model.HistoryCandidatePage{
		Items: []model.HistoryCandidate{{
			CandidateKey: key, FlowCode: "expense-flow", FormName: "费用单（测试公司）", FlowName: "费用审批",
			RuntimeType: "formmaking", InstanceTitle: "差旅报销", BusinessSummary: "北京出差",
			Initiator: "张三", CompanyName: "测试公司", CreatedAt: "2026-08-31 10:00:00",
			Status: "end", StatusName: "已结束", Completeness: "complete", SnapshotAvailable: true,
		}},
		Page: page, PageSize: pageSize, Total: 1, HasMore: false,
		DefaultSource: &model.HistoryDataSource{
			Mode: model.HistorySourceModeDefault, SnapshotID: 9,
			Summary:    &model.HistorySnapshotSummary{CandidateKey: key, FormName: "费用单（测试公司）", RuntimeType: "formmaking"},
			DataStatus: model.HistoryDataStatusNeedsInput, Issues: []model.HistoryDataIssue{{Code: "HISTORY_REPLAY_REQUIRED", Message: "需要完成回放", Blocking: true}}, Revision: 2,
		},
	}, nil
}

// SaveDefault 记录最小请求体、修订号和幂等键。
func (s *historyAPIStub) SaveDefault(_ context.Context, planID uint64, input model.HistoryDefaultSaveInput, idempotencyKey string) (model.HistoryDataSource, error) {
	s.defaultPlanID, s.defaultInput, s.idempotencyKey = planID, input, idempotencyKey
	if s.defaultErr != nil {
		return model.HistoryDataSource{}, s.defaultErr
	}
	return model.HistoryDataSource{Mode: model.HistorySourceModeDefault, DataStatus: model.HistoryDataStatusNeedsInput, Issues: []model.HistoryDataIssue{}, Revision: input.Revision + 1}, nil
}

// SavePathSource 记录路径继承或独立覆盖的最小请求体。
func (s *historyAPIStub) SavePathSource(_ context.Context, planID, pathID uint64, input model.HistoryPathSourceInput, idempotencyKey string) (model.HistoryDataSource, error) {
	s.pathPlanID, s.pathID, s.pathInput, s.idempotencyKey = planID, pathID, input, idempotencyKey
	return model.HistoryDataSource{Mode: input.Mode, DataStatus: model.HistoryDataStatusNeedsInput, Issues: []model.HistoryDataIssue{}, Revision: input.Revision + 1}, nil
}

// TestHistoryCandidateAPIExposesOnlySafeSummary 验证候选分页与来源摘要不包含正文、SID 或目标内部 ID。
func TestHistoryCandidateAPIExposesOnlySafeSummary(t *testing.T) {
	stub := &historyAPIStub{}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/plans/41/history-data/candidates?pathId=51&query=北京&page=2&pageSize=25", nil)
	api.NewHistoryDataHandler(stub).ServeHTTP(response, request)
	if response.Code != http.StatusOK || stub.candidatePlanID != 41 || stub.candidatePathID != 51 || stub.page != 2 || stub.pageSize != 25 || stub.query != "北京" {
		t.Fatalf("历史候选分页契约不正确：status=%d stub=%+v body=%s", response.Code, stub, response.Body.String())
	}
	body := response.Body.String()
	for _, forbidden := range []string{"rawFormData", "targetInstanceId", "flowProxyId", "formProxyId", "snapshotId", "sid-private", "sourceAccount"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("历史候选响应泄露敏感字段 %s：%s", forbidden, body)
		}
	}
	for _, expected := range []string{"candidateKey", "费用单（测试公司）", "北京出差", "HISTORY_REPLAY_REQUIRED"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("历史候选响应缺少 %s：%s", expected, body)
		}
	}
}

// TestHistoryCandidateAPIRejectsBrowserIdentityOverrides 验证账号和目标身份不能通过查询参数覆盖计划事实。
func TestHistoryCandidateAPIRejectsBrowserIdentityOverrides(t *testing.T) {
	for _, query := range []string{"account=other", "targetObjectId=private", "flowCode=other", "formName=other"} {
		stub := &historyAPIStub{}
		response := httptest.NewRecorder()
		api.NewHistoryDataHandler(stub).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/plans/41/history-data/candidates?"+query, nil))
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "INVALID_ARGUMENT") || stub.candidatePlanID != 0 {
			t.Fatalf("浏览器身份覆盖参数未被拒绝：query=%s status=%d body=%s", query, response.Code, response.Body.String())
		}
	}
}

// TestHistoryDefaultAPIUsesOpaqueKeyRevisionAndIdempotency 验证默认来源写入只接收候选键和修订号。
func TestHistoryDefaultAPIUsesOpaqueKeyRevisionAndIdempotency(t *testing.T) {
	stub := &historyAPIStub{}
	key := strings.Repeat("b", 64)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/plans/41/history-data/default", strings.NewReader(`{"candidateKey":"`+key+`","revision":3}`))
	request.Header.Set("Idempotency-Key", "123e4567-e89b-12d3-a456-426614174301")
	api.NewHistoryDataHandler(stub).ServeHTTP(response, request)
	if response.Code != http.StatusOK || stub.defaultPlanID != 41 || stub.defaultInput.CandidateKey != key || stub.defaultInput.Revision != 3 || stub.idempotencyKey != request.Header.Get("Idempotency-Key") {
		t.Fatalf("计划默认来源写入契约不正确：status=%d stub=%+v body=%s", response.Code, stub, response.Body.String())
	}
}

// TestHistoryWriteAPIRejectsSensitiveAndComputedFields 验证浏览器不能提交账号、目标 ID、正文或计算结果。
func TestHistoryWriteAPIRejectsSensitiveAndComputedFields(t *testing.T) {
	requests := []struct {
		path string
		body string
	}{
		{"/api/plans/41/history-data/default", `{"candidateKey":"` + strings.Repeat("c", 64) + `","revision":0,"account":"other"}`},
		{"/api/plans/41/history-data/default", `{"candidateKey":"` + strings.Repeat("c", 64) + `","revision":0,"targetInstanceId":"private"}`},
		{"/api/plans/41/execution-paths/51/configuration/data/source", `{"mode":"override","candidateKey":"` + strings.Repeat("c", 64) + `","revision":0,"rawFormData":{"private":true}}`},
		{"/api/plans/41/execution-paths/51/configuration/data/source", `{"mode":"default","revision":0,"compiledSteps":[]}`},
	}
	for _, test := range requests {
		stub := &historyAPIStub{}
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, test.path, strings.NewReader(test.body))
		request.Header.Set("Idempotency-Key", "123e4567-e89b-12d3-a456-426614174302")
		api.NewHistoryDataHandler(stub).ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "INVALID_ARGUMENT") || stub.defaultPlanID != 0 || stub.pathPlanID != 0 {
			t.Fatalf("敏感或计算字段未被拒绝：path=%s status=%d body=%s", test.path, response.Code, response.Body.String())
		}
	}
}

// TestHistoryPathSourceAPIKeepsDefaultAndOverrideDistinct 验证路径继承不携带候选键，独立覆盖只携带不透明键。
func TestHistoryPathSourceAPIKeepsDefaultAndOverrideDistinct(t *testing.T) {
	for _, test := range []struct {
		body          string
		wantMode      string
		wantCandidate string
	}{
		{`{"mode":"default","revision":2}`, model.HistorySourceModeDefault, ""},
		{`{"mode":"override","candidateKey":"` + strings.Repeat("d", 64) + `","revision":2}`, model.HistorySourceModeOverride, strings.Repeat("d", 64)},
	} {
		stub := &historyAPIStub{}
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/api/plans/41/execution-paths/51/configuration/data/source", strings.NewReader(test.body))
		request.Header.Set("Idempotency-Key", "123e4567-e89b-12d3-a456-426614174303")
		api.NewHistoryDataHandler(stub).ServeHTTP(response, request)
		if response.Code != http.StatusOK || stub.pathPlanID != 41 || stub.pathID != 51 || stub.pathInput.Mode != test.wantMode || stub.pathInput.CandidateKey != test.wantCandidate || stub.pathInput.Revision != 2 {
			t.Fatalf("路径来源模式契约不正确：status=%d stub=%+v body=%s", response.Code, stub, response.Body.String())
		}
	}
}

// TestHistoryAPIMapsRevisionConflict 验证来源修订冲突保持稳定 409 契约。
func TestHistoryAPIMapsRevisionConflict(t *testing.T) {
	stub := &historyAPIStub{defaultErr: &service.HistoryDataError{Kind: service.HistoryDataErrorConflict, Message: "历史数据来源已更新"}}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/plans/41/history-data/default", strings.NewReader(`{"candidateKey":"`+strings.Repeat("e", 64)+`","revision":1}`))
	request.Header.Set("Idempotency-Key", "123e4567-e89b-12d3-a456-426614174304")
	api.NewHistoryDataHandler(stub).ServeHTTP(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "HISTORY_REVISION_CONFLICT") {
		t.Fatalf("历史来源修订冲突契约不正确：status=%d body=%s", response.Code, response.Body.String())
	}
}
