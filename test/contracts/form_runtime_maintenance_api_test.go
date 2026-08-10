package contracts_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/api"
	"test-auto-pro-v2/internal/formruntimemaintenance"
	"test-auto-pro-v2/internal/service"
)

type stubFormRuntimeMaintenance struct {
	source formruntimemaintenance.SourceState
	job    formruntimemaintenance.Job
	log    formruntimemaintenance.Log
	err    error
}

// InspectSource 返回维护契约的固定来源。
func (s *stubFormRuntimeMaintenance) InspectSource(context.Context) (formruntimemaintenance.SourceState, error) {
	return s.source, s.err
}

// CreateJob 返回维护契约的创建结果。
func (s *stubFormRuntimeMaintenance) CreateJob(context.Context) (formruntimemaintenance.Job, error) {
	return s.job, s.err
}

// GetJob 返回维护契约的任务详情。
func (s *stubFormRuntimeMaintenance) GetJob(context.Context, uint64) (formruntimemaintenance.Job, error) {
	return s.job, s.err
}

// LatestJob 返回维护契约的最新任务。
func (s *stubFormRuntimeMaintenance) LatestJob(context.Context) (formruntimemaintenance.Job, error) {
	return s.job, s.err
}

// GetJobLog 返回维护契约的日志尾部。
func (s *stubFormRuntimeMaintenance) GetJobLog(context.Context, uint64) (formruntimemaintenance.Log, error) {
	return s.log, s.err
}

// newMaintenanceHandler 组装包含维护 API 的完整路由。
func newMaintenanceHandler(maintenance api.FormRuntimeMaintenanceService) http.Handler {
	return api.NewHandlerWithMaintenanceServices(
		&stubTargetReader{}, service.NewPlanService(&contractPlanRepository{}), &stubFlowGraphService{},
		&stubExecutionPathService{}, &stubPathRequirementService{}, &stubPathConfigurationService{}, maintenance,
	)
}

// TestFormRuntimeMaintenanceAPIContract 验证固定来源、一键任务、阶段和日志契约不接受自定义命令。
func TestFormRuntimeMaintenanceAPIContract(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	stub := &stubFormRuntimeMaintenance{
		source: formruntimemaintenance.SourceState{Repository: "rsh-flow-components", Branch: "master", Head: strings.Repeat("a", 40), InspectedAt: now},
		job:    formruntimemaintenance.Job{ID: 3, Status: formruntimemaintenance.JobRunning, Stage: formruntimemaintenance.StageBuild, RecoveryStatus: formruntimemaintenance.RecoveryNotRequired, CreatedAt: now, UpdatedAt: now},
		log:    formruntimemaintenance.Log{Content: "[BUILD] pnpm", Truncated: true},
	}
	handler := newMaintenanceHandler(stub)

	for _, path := range []string{"/api/form-runtime-maintenance/source", "/api/form-runtime-maintenance/jobs/latest", "/api/form-runtime-maintenance/jobs/3", "/api/form-runtime-maintenance/jobs/3/log"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s 失败：status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, httptest.NewRequest(http.MethodPost, "/api/form-runtime-maintenance/jobs", nil))
	if created.Code != http.StatusAccepted || !strings.Contains(created.Body.String(), `"stage":"BUILD"`) {
		t.Fatalf("一键任务创建契约不正确：status=%d body=%s", created.Code, created.Body.String())
	}
	forged := httptest.NewRecorder()
	handler.ServeHTTP(forged, httptest.NewRequest(http.MethodPost, "/api/form-runtime-maintenance/jobs", strings.NewReader(`{"source":"/tmp","command":"rm"}`)))
	if forged.Code != http.StatusBadRequest || !strings.Contains(forged.Body.String(), "INVALID_ARGUMENT") {
		t.Fatalf("维护 API 接受了任意来源或命令：status=%d body=%s", forged.Code, forged.Body.String())
	}
}

// TestFormRuntimeMaintenanceAPIStableErrors 验证单活动任务和来源错误映射。
func TestFormRuntimeMaintenanceAPIStableErrors(t *testing.T) {
	for _, item := range []struct {
		err  error
		code string
	}{
		{err: formruntimemaintenance.ErrJobAlreadyActive, code: "FORM_RUNTIME_SYNC_ALREADY_ACTIVE"},
		{err: formruntimemaintenance.ErrSourceInvalid, code: "FORM_RUNTIME_SOURCE_INVALID"},
		{err: errors.New("storage failed"), code: "FORM_RUNTIME_MAINTENANCE_FAILED"},
	} {
		response := httptest.NewRecorder()
		newMaintenanceHandler(&stubFormRuntimeMaintenance{err: item.err}).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/form-runtime-maintenance/jobs", nil))
		if !strings.Contains(response.Body.String(), item.code) {
			t.Fatalf("错误 %v 未映射为 %s：%s", item.err, item.code, response.Body.String())
		}
	}
}
