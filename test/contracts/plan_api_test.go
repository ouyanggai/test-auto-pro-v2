package contracts_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/api"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	"test-auto-pro-v2/internal/service"
)

type contractPlanRepository struct {
	plan  model.Plan
	key   string
	err   error
	found bool
}

func (r *contractPlanRepository) Create(_ context.Context, key string, plan model.Plan) (model.Plan, bool, error) {
	if r.err != nil {
		return model.Plan{}, false, r.err
	}
	if r.found && r.key == key {
		return r.plan, false, nil
	}
	plan.ID = 41
	r.plan, r.key, r.found = plan, key, true
	return plan, true, nil
}

func (r *contractPlanRepository) List(_ context.Context, filter model.PlanListFilter) ([]model.Plan, error) {
	if r.err != nil {
		return nil, r.err
	}
	if !r.found {
		return []model.Plan{}, nil
	}
	if filter.Status != "" && filter.Status != r.plan.Status {
		return []model.Plan{}, nil
	}
	return []model.Plan{r.plan}, nil
}

func (r *contractPlanRepository) Get(_ context.Context, id uint64) (model.Plan, error) {
	if r.err != nil {
		return model.Plan{}, r.err
	}
	if !r.found || r.plan.ID != id {
		return model.Plan{}, repository.ErrPlanNotFound
	}
	return r.plan, nil
}

// Delete 删除夹具中的计划，供 DELETE 契约验证。
func (r *contractPlanRepository) Delete(_ context.Context, id uint64) error {
	if r.err != nil {
		return r.err
	}
	if !r.found || r.plan.ID != id {
		return repository.ErrPlanNotFound
	}
	r.found = false
	return nil
}

// TestPlanAPIContractsAndIdempotency 验证计划接口三态协议和创建幂等。
func TestPlanAPIContractsAndIdempotency(t *testing.T) {
	repo := &contractPlanRepository{}
	handler := api.NewHandlerWithServices(&stubTargetReader{}, service.NewPlanService(repo))
	body := `{"name":"采购回归","account":"tester01","accountDisplayName":"测试专员","flowSource":"new","targetObjectId":"template-id","targetObjectName":"采购流程","runMode":"serial","maxConcurrency":null,"scheduledAt":null}`
	for attempt, expectedStatus := range []int{http.StatusCreated, http.StatusOK} {
		request := httptest.NewRequest(http.MethodPost, "/api/plans", strings.NewReader(body))
		request.Header.Set("Idempotency-Key", "123e4567-e89b-12d3-a456-426614174000")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != expectedStatus {
			t.Fatalf("第 %d 次创建状态码 = %d", attempt+1, recorder.Code)
		}
		responseBody := recorder.Body.String()
		for _, field := range []string{"\"id\":\"41\"", "not_started", "targetObjectName", "pathCount", "lastRunResult"} {
			if !strings.Contains(responseBody, field) {
				t.Fatalf("创建响应缺少 %s", field)
			}
		}
		assertPlanResponseSafe(t, recorder.Body.Bytes())
	}

	for _, path := range []string{"/api/plans?status=not_started", "/api/plans/41"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "采购回归") {
			t.Fatalf("读取计划契约失败：%s status=%d", path, recorder.Code)
		}
		assertPlanResponseSafe(t, recorder.Body.Bytes())
	}
}

// TestPlanAPIDeletesDevelopmentPlan 验证删除只影响系统计划数据，重复删除稳定返回不存在。
func TestPlanAPIDeletesDevelopmentPlan(t *testing.T) {
	repo := &contractPlanRepository{plan: model.Plan{ID: 41, Status: model.PlanStatusNotStarted}, found: true}
	handler := api.NewHandlerWithServices(&stubTargetReader{}, service.NewPlanService(repo))
	request := httptest.NewRequest(http.MethodDelete, "/api/plans/41", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"deleted":true`) {
		t.Fatalf("删除计划契约不正确：%d %s", recorder.Code, recorder.Body.String())
	}
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodDelete, "/api/plans/41", nil))
	if second.Code != http.StatusNotFound || !strings.Contains(second.Body.String(), "PLAN_NOT_FOUND") {
		t.Fatalf("重复删除没有稳定返回不存在：%d %s", second.Code, second.Body.String())
	}
}

// TestPlanAPIParameterAndStableErrorContracts 验证非法输入和存储错误保持稳定脱敏协议。
func TestPlanAPIParameterAndStableErrorContracts(t *testing.T) {
	repo := &contractPlanRepository{}
	handler := api.NewHandlerWithServices(&stubTargetReader{}, service.NewPlanService(repo))
	requests := []*http.Request{
		httptest.NewRequest(http.MethodPost, "/api/plans", bytes.NewBufferString(`{"name":"计划"}`)),
		httptest.NewRequest(http.MethodPost, "/api/plans", bytes.NewBufferString(`{} {}`)),
		httptest.NewRequest(http.MethodGet, "/api/plans?status=unknown", nil),
		httptest.NewRequest(http.MethodGet, "/api/plans/not-a-number", nil),
	}
	for _, request := range requests {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "INVALID_ARGUMENT") {
			t.Fatalf("非法参数未返回稳定错误：path=%s status=%d", request.URL.Path, recorder.Code)
		}
		assertPlanResponseSafe(t, recorder.Body.Bytes())
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/plans/99", nil))
	if recorder.Code != http.StatusNotFound || !strings.Contains(recorder.Body.String(), "PLAN_NOT_FOUND") {
		t.Fatal("不存在计划未返回稳定错误")
	}

	repo.err = errors.New("dsn and sql details")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/plans", nil))
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "PLAN_STORAGE_UNAVAILABLE") {
		t.Fatal("仓储失败未返回稳定错误")
	}
	if strings.Contains(recorder.Body.String(), "dsn") || strings.Contains(recorder.Body.String(), "sql") {
		t.Fatal("公开错误泄露数据库细节")
	}
}

// assertPlanResponseSafe 验证计划响应不会泄露内部存储和凭证字段。
func assertPlanResponseSafe(t *testing.T, body []byte) {
	t.Helper()
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatalf("响应不是 JSON：%v", err)
	}
	for _, forbidden := range []string{"create_key", "createKey", "sid", "password", "dsn", "customerCode", "platformCode"} {
		if strings.Contains(strings.ToLower(string(body)), strings.ToLower(forbidden)) {
			t.Fatalf("计划响应包含禁止字段 %s", forbidden)
		}
	}
}
