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

type f009PathPreparationStub struct {
	createdPlanID uint64
	createKey     string
	lastJobID     string
	lastCursor    uint64
	lastLimit     int
}

// Create 记录计划和幂等键并返回固定后台任务。
func (s *f009PathPreparationStub) Create(_ context.Context, planID uint64, createKey string) (model.PathPreparationJob, error) {
	s.createdPlanID, s.createKey = planID, createKey
	return f009PreparationJob(planID, createKey, "running"), nil
}

// Get 返回固定任务聚合计数。
func (s *f009PathPreparationStub) Get(_ context.Context, planID uint64, jobID string) (model.PathPreparationJob, error) {
	s.lastJobID = jobID
	return f009PreparationJob(planID, jobID, "running"), nil
}

// Active 模拟刷新页面后找到活动任务。
func (s *f009PathPreparationStub) Active(_ context.Context, planID uint64) (model.PathPreparationJob, bool, error) {
	return f009PreparationJob(planID, "123e4567-e89b-12d3-a456-426614174799", "running"), true, nil
}

// Cancel 返回保留检查点的取消状态。
func (s *f009PathPreparationStub) Cancel(_ context.Context, planID uint64, jobID string) (model.PathPreparationJob, error) {
	s.lastJobID = jobID
	return f009PreparationJob(planID, jobID, "cancelled"), nil
}

// Resume 返回从检查点重新排队的状态。
func (s *f009PathPreparationStub) Resume(_ context.Context, planID uint64, jobID string) (model.PathPreparationJob, error) {
	s.lastJobID = jobID
	return f009PreparationJob(planID, jobID, "queued"), nil
}

// ListItems 记录游标参数并返回一页中文原因。
func (s *f009PathPreparationStub) ListItems(_ context.Context, _ uint64, jobID string, cursor uint64, limit int) (model.PathPreparationItemPage, error) {
	s.lastJobID, s.lastCursor, s.lastLimit = jobID, cursor, limit
	return model.PathPreparationItemPage{
		Items:      []model.PathPreparationItem{{ID: cursor + 1, PathID: 9, SequenceNo: 2, PathName: "请假兜底路径", Status: "needs_attention", Reason: "条件需要人工核对", NeedsAttention: true}},
		NextCursor: cursor + 1,
	}, nil
}

// f009PreparationJob 构造带真实进度计数的任务响应。
func f009PreparationJob(planID uint64, jobID, status string) model.PathPreparationJob {
	now := time.Date(2026, 8, 18, 14, 0, 0, 0, time.UTC)
	return model.PathPreparationJob{
		ID: jobID, PlanID: planID, Status: status, Total: 5, Processed: 3,
		NodeConfigured: 2, DataGenerated: 1, NeedsAttention: 1, Failed: 1, PreservedManual: 1,
		CurrentPath: &model.PathPreparationCurrentPath{PathID: 9, SequenceNo: 2, PathName: "请假兜底路径", Status: "running"},
		CreatedAt:   now, UpdatedAt: now,
	}
}

// f009PreparationHandler 组装包含批量准备端点的真实 HTTP 路由。
func f009PreparationHandler(preparations api.PathPreparationService) http.Handler {
	return api.NewHandlerWithPreparationServices(
		&stubTargetReader{}, service.NewPlanService(&contractPlanRepository{}), &stubFlowGraphService{},
		&stubExecutionPathService{}, &stubPathRequirementService{}, &stubPathConfigurationService{},
		&stubFormRuntimeMaintenance{}, preparations,
	)
}

// TestF009PathPreparationAPIContract 验证创建、刷新恢复、计数、取消恢复和游标明细协议。
func TestF009PathPreparationAPIContract(t *testing.T) {
	stub := &f009PathPreparationStub{}
	handler := f009PreparationHandler(stub)
	jobID := "123e4567-e89b-12d3-a456-426614174798"
	created := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/plans/7/path-preparations", nil)
	request.Header.Set("Idempotency-Key", jobID)
	handler.ServeHTTP(created, request)
	if created.Code != http.StatusOK || stub.createdPlanID != 7 || stub.createKey != jobID || !strings.Contains(created.Body.String(), `"processed":3`) || !strings.Contains(created.Body.String(), `"currentPath":{"pathId":9,"sequenceNo":2,"pathName":"请假兜底路径","status":"running"}`) {
		t.Fatalf("批量准备创建契约不正确：status=%d plan=%d key=%s body=%s", created.Code, stub.createdPlanID, stub.createKey, created.Body.String())
	}

	for _, item := range []struct {
		method string
		path   string
		status string
	}{
		{http.MethodGet, "/api/plans/7/path-preparations/active", `"status":"running"`},
		{http.MethodGet, "/api/plans/7/path-preparations/" + jobID, `"dataGenerated":1`},
		{http.MethodPost, "/api/plans/7/path-preparations/" + jobID + "/cancel", `"status":"cancelled"`},
		{http.MethodPost, "/api/plans/7/path-preparations/" + jobID + "/resume", `"status":"queued"`},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(item.method, item.path, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), item.status) {
			t.Fatalf("%s %s 契约不正确：status=%d body=%s", item.method, item.path, response.Code, response.Body.String())
		}
	}

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/api/plans/7/path-preparations/"+jobID+"/items?cursor=11&limit=25", nil))
	if page.Code != http.StatusOK || stub.lastCursor != 11 || stub.lastLimit != 25 || !strings.Contains(page.Body.String(), "条件需要人工核对") {
		t.Fatalf("批量准备明细分页契约不正确：status=%d cursor=%d limit=%d body=%s", page.Code, stub.lastCursor, stub.lastLimit, page.Body.String())
	}
}

// TestF009PathPreparationAPIRejectsUnboundedPage 验证明细接口拒绝无界分页参数。
func TestF009PathPreparationAPIRejectsUnboundedPage(t *testing.T) {
	response := httptest.NewRecorder()
	f009PreparationHandler(&f009PathPreparationStub{}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/plans/7/path-preparations/job/items?limit=101", nil))
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "INVALID_ARGUMENT") {
		t.Fatalf("无界分页没有被拒绝：status=%d body=%s", response.Code, response.Body.String())
	}
}

type f009PartialGenerationStub struct{ stubPathConfigurationService }

// GenerateForm 返回可预期的部分求解结果，证明 HTTP 层不会转成 409。
func (f009PartialGenerationStub) GenerateForm(context.Context, uint64, uint64, int64, map[string]any, []string, bool) (model.PathFormGenerateResult, error) {
	return model.PathFormGenerateResult{
		Status: "draft", Values: map[string]any{"safeField": "已生成"}, GenerationState: "partial",
		Issues:            []model.PathFormGenerationIssue{{Field: "脚本条件", Reason: "动态条件需要人工核对", Blocking: true}},
		RouteVerification: model.PathFormRouteVerification{Matched: false, Reason: "当前完整路径仍需人工核对"},
	}, nil
}

// TestF009PartialGenerationReturnsBusinessResult 验证预期求解失败仍返回 2xx、部分值、问题和完整路径复验。
func TestF009PartialGenerationReturnsBusinessResult(t *testing.T) {
	handler := api.NewHandlerWithConfigurationServices(
		&stubTargetReader{}, service.NewPlanService(&contractPlanRepository{}), &stubFlowGraphService{},
		&stubExecutionPathService{}, &stubPathRequirementService{}, f009PartialGenerationStub{},
	)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/plans/7/execution-paths/9/configuration/form/generate", strings.NewReader(`{"seed":0,"values":{},"manualOverridePaths":[],"nextGroup":false}`))
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"generationState":"partial"`) || !strings.Contains(response.Body.String(), `"matched":false`) || !strings.Contains(response.Body.String(), "动态条件需要人工核对") {
		t.Fatalf("部分求解没有作为 2xx 业务结果返回：status=%d body=%s", response.Code, response.Body.String())
	}
}
