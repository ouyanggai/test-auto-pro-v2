package f012_test

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

type historyReplayAPIStub struct {
	planID        uint64
	input         model.HistoryReplayCreateInput
	idempotency   string
	jobID         string
	cursor, limit uint64
	pageLimit     int
	conflict      bool
}

// Create 记录明确路径、修订和幂等键，响应只返回任务聚合字段。
func (s *historyReplayAPIStub) Create(_ context.Context, planID uint64, input model.HistoryReplayCreateInput, key string) (model.HistoryReplayJob, error) {
	s.planID, s.input, s.idempotency = planID, input, key
	if s.conflict {
		return model.HistoryReplayJob{}, &service.HistoryReplayError{Kind: service.HistoryReplayErrorConflict, Message: "当前计划已有历史回放任务"}
	}
	return historyReplayAPIJob(planID, key, model.HistoryReplayStatusQueued), nil
}

// Active 返回刷新页面时可见的唯一活动任务。
func (s *historyReplayAPIStub) Active(_ context.Context, planID uint64) (model.HistoryReplayJob, bool, error) {
	return historyReplayAPIJob(planID, "123e4567-e89b-12d3-a456-426614174710", model.HistoryReplayStatusRunning), true, nil
}

// Get 记录任务身份并返回真实计数。
func (s *historyReplayAPIStub) Get(_ context.Context, planID uint64, jobID string) (model.HistoryReplayJob, error) {
	s.planID, s.jobID = planID, jobID
	return historyReplayAPIJob(planID, jobID, model.HistoryReplayStatusCompleted), nil
}

// ListItems 记录游标参数，验证分页不会退回页码或无界读取。
func (s *historyReplayAPIStub) ListItems(_ context.Context, planID uint64, jobID string, cursor uint64, limit int) (model.HistoryReplayItemPage, error) {
	s.planID, s.jobID, s.cursor, s.pageLimit = planID, jobID, cursor, limit
	return model.HistoryReplayItemPage{Items: []model.HistoryReplayItem{{ID: cursor + 1, PathID: 51, Status: model.HistoryReplayItemStatusReady, DataStatus: model.HistoryDataStatusReady}}, NextCursor: cursor + 1}, nil
}

// Cancel 返回取消后保留检查点的任务。
func (s *historyReplayAPIStub) Cancel(_ context.Context, planID uint64, jobID string) (model.HistoryReplayJob, error) {
	s.planID, s.jobID = planID, jobID
	return historyReplayAPIJob(planID, jobID, model.HistoryReplayStatusCancelled), nil
}

// Resume 返回从未完成检查点重新排队的任务。
func (s *historyReplayAPIStub) Resume(_ context.Context, planID uint64, jobID string) (model.HistoryReplayJob, error) {
	s.planID, s.jobID = planID, jobID
	return historyReplayAPIJob(planID, jobID, model.HistoryReplayStatusQueued), nil
}

// historyReplayAPIJob 构造稳定时间和真实状态计数，避免契约测试依赖当前时钟。
func historyReplayAPIJob(planID uint64, jobID, status string) model.HistoryReplayJob {
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	return model.HistoryReplayJob{ID: jobID, PlanID: planID, Status: status, Total: 2, Pending: 1, Ready: 1, CreatedAt: now, UpdatedAt: now}
}

// TestHistoryReplayAPIContract 验证回放创建、活动刷新、状态、取消、恢复和游标明细协议。
func TestHistoryReplayAPIContract(t *testing.T) {
	stub := &historyReplayAPIStub{}
	handler := api.NewHistoryReplayHandler(stub)
	key := "123e4567-e89b-12d3-a456-426614174711"
	created := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/plans/41/history-replays", strings.NewReader(`{"pathIds":[51,52],"revision":4}`))
	request.Header.Set("Idempotency-Key", key)
	handler.ServeHTTP(created, request)
	if created.Code != http.StatusOK || stub.planID != 41 || stub.idempotency != key || len(stub.input.PathIDs) != 2 || stub.input.Revision != 4 || !strings.Contains(created.Body.String(), `"status":"queued"`) {
		t.Fatalf("历史回放创建契约不正确：status=%d stub=%+v body=%s", created.Code, stub, created.Body.String())
	}

	for _, requestPath := range []string{
		"/api/plans/41/history-replays/active",
		"/api/plans/41/history-replays/" + key,
		"/api/plans/41/history-replays/" + key + "/cancel",
		"/api/plans/41/history-replays/" + key + "/resume",
	} {
		method := http.MethodGet
		if strings.HasSuffix(requestPath, "/cancel") || strings.HasSuffix(requestPath, "/resume") {
			method = http.MethodPost
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(method, requestPath, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("回放路径 %s 契约不正确：status=%d body=%s", requestPath, response.Code, response.Body.String())
		}
	}

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/api/plans/41/history-replays/"+key+"/items?cursor=11&limit=25", nil))
	if page.Code != http.StatusOK || stub.cursor != 11 || stub.pageLimit != 25 || !strings.Contains(page.Body.String(), `"pathId":51`) {
		t.Fatalf("回放明细游标契约不正确：status=%d cursor=%d limit=%d body=%s", page.Code, stub.cursor, stub.pageLimit, page.Body.String())
	}
}

// TestHistoryReplayAPIRejectsDerivedFieldsAndUnboundedPagination 验证浏览器不能提交派生数据，也不能绕过分页上限。
func TestHistoryReplayAPIRejectsDerivedFieldsAndUnboundedPagination(t *testing.T) {
	stub := &historyReplayAPIStub{}
	handler := api.NewHistoryReplayHandler(stub)
	request := httptest.NewRequest(http.MethodPost, "/api/plans/41/history-replays", strings.NewReader(`{"pathIds":[51],"rawFormData":{"secret":true}}`))
	request.Header.Set("Idempotency-Key", "123e4567-e89b-12d3-a456-426614174712")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "INVALID_ARGUMENT") || stub.planID != 0 {
		t.Fatalf("派生字段没有被拒绝：status=%d body=%s", response.Code, response.Body.String())
	}

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/api/plans/41/history-replays/job/items?limit=101", nil))
	if page.Code != http.StatusBadRequest || !strings.Contains(page.Body.String(), "INVALID_ARGUMENT") {
		t.Fatalf("无界明细分页没有被拒绝：status=%d body=%s", page.Code, page.Body.String())
	}
}

// TestHistoryReplayAPIMapsConflict 验证同计划单活动任务冲突保持稳定 409 契约。
func TestHistoryReplayAPIMapsConflict(t *testing.T) {
	response := httptest.NewRecorder()
	stub := &historyReplayAPIStub{conflict: true}
	request := httptest.NewRequest(http.MethodPost, "/api/plans/41/history-replays", strings.NewReader(`{"pathIds":[51]}`))
	request.Header.Set("Idempotency-Key", "123e4567-e89b-12d3-a456-426614174713")
	api.NewHistoryReplayHandler(stub).ServeHTTP(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "HISTORY_REPLAY_CONFLICT") {
		t.Fatalf("活动任务冲突契约不正确：status=%d body=%s", response.Code, response.Body.String())
	}
}
