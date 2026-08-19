package backend_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

type f009BatchSnapshotReader struct {
	snapshot target.PathConfigurationSnapshot
	mu       sync.Mutex
	calls    int
}

// PathConfigurationSnapshot 返回同一份目标快照并记录读取次数。
func (r *f009BatchSnapshotReader) PathConfigurationSnapshot(context.Context, string, string, string) (target.PathConfigurationSnapshot, error) {
	r.mu.Lock()
	r.calls++
	r.mu.Unlock()
	return r.snapshot, nil
}

type f009BatchConfigRepository struct {
	mu           sync.Mutex
	configs      map[uint64]model.StoredPathConfig
	findManyCall int
}

// FindByPath 返回单条路径的持久配置快照。
func (r *f009BatchConfigRepository) FindByPath(_ context.Context, pathID uint64) (model.StoredPathConfig, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, found := r.configs[pathID]
	return value, found, nil
}

// FindByPaths 一次返回一批配置，验证批处理不逐路径访问配置仓储。
func (r *f009BatchConfigRepository) FindByPaths(_ context.Context, pathIDs []uint64) (map[uint64]model.StoredPathConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.findManyCall++
	result := make(map[uint64]model.StoredPathConfig, len(pathIDs))
	for _, id := range pathIDs {
		if value, ok := r.configs[id]; ok {
			result[id] = value
		}
	}
	return result, nil
}

// FindByPathAndKey 返回不存在的幂等记录，交由批量任务生成新结果。
func (r *f009BatchConfigRepository) FindByPathAndKey(context.Context, uint64, string) (model.StoredPathConfig, bool, error) {
	return model.StoredPathConfig{}, false, nil
}

// Save 保存批量任务生成的路径数据。
func (r *f009BatchConfigRepository) Save(_ context.Context, value model.StoredPathConfig, _ uint64, _ time.Time) (model.StoredPathConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configs[value.PathID] = value
	return value, nil
}

type f009BatchPreparationRepository struct {
	mu          sync.Mutex
	job         model.PathPreparationJob
	items       []model.PathPreparationItem
	claimed     bool
	completedCh chan model.PathPreparationItemResult
}

// Create 返回当前计划的明细检查点。
func (r *f009BatchPreparationRepository) Create(_ context.Context, planID uint64, createKey string, now time.Time) (model.PathPreparationJob, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.job = model.PathPreparationJob{ID: createKey, PlanID: planID, Status: "queued", Total: len(r.items), CreatedAt: now, UpdatedAt: now}
	return r.job, true, nil
}

// Get 返回当前任务计数。
func (r *f009BatchPreparationRepository) Get(context.Context, uint64, string) (model.PathPreparationJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.job, nil
}

// FindActive 返回无活动任务，避免测试重复启动 Worker。
func (r *f009BatchPreparationRepository) FindActive(context.Context, uint64) (model.PathPreparationJob, bool, error) {
	return model.PathPreparationJob{}, false, nil
}

// ListRecoverable 返回空列表，恢复语义由 MySQL 检查点测试覆盖。
func (r *f009BatchPreparationRepository) ListRecoverable(context.Context) ([]model.PathPreparationJob, error) {
	return nil, nil
}

// Start 将任务置为运行中。
func (r *f009BatchPreparationRepository) Start(_ context.Context, _ uint64, _ string, now time.Time) error {
	r.mu.Lock()
	r.job.Status, r.job.UpdatedAt = "running", now
	r.mu.Unlock()
	return nil
}

// ClaimBatch 一次交付固定批次，避免逐路径详情读取。
func (r *f009BatchPreparationRepository) ClaimBatch(_ context.Context, _ uint64, _ string, _ int, _ time.Time) ([]model.PathPreparationItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.claimed {
		return []model.PathPreparationItem{}, nil
	}
	r.claimed = true
	return append([]model.PathPreparationItem(nil), r.items...), nil
}

// CompleteItem 收集每条路径结果，验证单条失败不阻断其他路径。
func (r *f009BatchPreparationRepository) CompleteItem(_ context.Context, _ uint64, _ string, _ uint64, outcome model.PathPreparationItemResult, now time.Time) error {
	r.mu.Lock()
	r.job.Processed++
	if outcome.DataGenerated {
		r.job.DataGenerated++
	}
	if outcome.NeedsAttention {
		r.job.NeedsAttention++
	}
	if outcome.PreservedManual {
		r.job.PreservedManual++
	}
	if outcome.Status == "failed" {
		r.job.Failed++
	}
	r.job.UpdatedAt = now
	r.completedCh <- outcome
	r.mu.Unlock()
	return nil
}

// Finish 将所有明细完成的任务标为完成。
func (r *f009BatchPreparationRepository) Finish(_ context.Context, _ uint64, _ string, now time.Time) (model.PathPreparationJob, error) {
	r.mu.Lock()
	r.job.Status, r.job.UpdatedAt = "completed", now
	value := r.job
	r.mu.Unlock()
	return value, nil
}

// Cancel 返回取消状态。
func (r *f009BatchPreparationRepository) Cancel(context.Context, uint64, string, time.Time) (model.PathPreparationJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.job.Status = "cancelled"
	return r.job, nil
}

// Resume 返回排队状态。
func (r *f009BatchPreparationRepository) Resume(context.Context, uint64, string, time.Time) (model.PathPreparationJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.job.Status = "queued"
	return r.job, nil
}

// Fail 记录任务级失败状态。
func (r *f009BatchPreparationRepository) Fail(_ context.Context, _ uint64, _ string, reason string, _ time.Time) error {
	r.mu.Lock()
	r.job.Status, r.job.Error = "failed", reason
	r.mu.Unlock()
	return nil
}

// ListItems 返回当前明细页，游标边界由 MySQL 仓储覆盖。
func (r *f009BatchPreparationRepository) ListItems(context.Context, uint64, string, uint64, int) (model.PathPreparationItemPage, error) {
	return model.PathPreparationItemPage{Items: r.items}, nil
}

// TestF009BatchPreparationReadsSharedAssetsOnceAndIsolatesFailure 验证目标快照只读一次、路径配置批量读取且单条失败不阻断其他路径。
func TestF009BatchPreparationReadsSharedAssetsOnceAndIsolatesFailure(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 501, Account: "account", FlowSource: "new", TargetObjectID: "template", TargetObjectName: "请假流程", Status: model.PlanStatusNotStarted}}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{
		{ID: 601, PlanID: 501, SequenceNo: 1, Name: "兜底路径", Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "fallback"}}},
		{ID: 602, PlanID: 501, SequenceNo: 2, Name: "失效路径", Choices: []model.ExecutionPathChoice{{RouteNodeID: "removed", BranchID: "missing"}}},
		{ID: 603, PlanID: 501, SequenceNo: 3, Name: "人工数据路径", Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "fallback"}}},
	}}
	reader := &f009BatchSnapshotReader{snapshot: target.PathConfigurationSnapshot{Tree: f009SortedRouteTree(), EntryNodeIDs: []string{"start"}, Forms: []target.FormRuntimeTemplate{{Name: "请假表单", TemplateData: f009LeaveTemplate()}}}}
	configs := &f009BatchConfigRepository{configs: map[uint64]model.StoredPathConfig{
		603: {
			PathID: 603, Revision: 4, FormRevision: 2, Status: "pending", FormStatus: "valid", DataStatus: "confirmed",
			FieldValues: map[string]map[string]string{}, ActionValues: map[string]string{}, FormValues: map[string]any{"vacateDayNum": 100},
			ConfirmedNodeKeys: []string{}, GeneratedFieldPaths: []string{}, ManualOverridePaths: []string{"vacateDayNum"},
		},
	}}
	preparations := &f009BatchPreparationRepository{items: []model.PathPreparationItem{
		{ID: 1, PathID: 601, SequenceNo: 1, PathName: "兜底路径"},
		{ID: 2, PathID: 602, SequenceNo: 2, PathName: "失效路径"},
		{ID: 3, PathID: 603, SequenceNo: 3, PathName: "人工数据路径"},
	}, completedCh: make(chan model.PathPreparationItemResult, 3)}
	configService := service.NewPathConfigService(service.NewPlanService(plans), reader, analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathConfigAnalyzer(), paths, configs)
	preparationService := service.NewPathPreparationService(configService, preparations)
	job, err := preparationService.Create(context.Background(), 501, "123e4567-e89b-12d3-a456-426614174501")
	if err != nil || job.Total != 3 {
		t.Fatalf("批量任务创建失败：job=%+v err=%v", job, err)
	}
	<-preparations.completedCh
	<-preparations.completedCh
	<-preparations.completedCh
	deadline := time.Now().Add(2 * time.Second)
	var completed model.PathPreparationJob
	for {
		completed, err = preparationService.Get(context.Background(), 501, job.ID)
		if err == nil && completed.Status == "completed" {
			break
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil || completed.Status != "completed" || completed.Processed != 3 || completed.DataGenerated != 1 || completed.NeedsAttention != 2 || completed.PreservedManual != 1 {
		t.Fatalf("单条失败没有隔离或任务计数错误：job=%+v err=%v", completed, err)
	}
	reader.mu.Lock()
	snapshotCalls := reader.calls
	reader.mu.Unlock()
	configs.mu.Lock()
	configManyCalls := configs.findManyCall
	configs.mu.Unlock()
	if snapshotCalls != 1 || paths.getManyCalls != 1 || configManyCalls != 1 {
		t.Fatalf("批量任务重复读取目标资产或逐路径读取：snapshot=%d getMany=%d configMany=%d", snapshotCalls, paths.getManyCalls, configManyCalls)
	}
	configs.mu.Lock()
	preserved := configs.configs[603]
	configs.mu.Unlock()
	if preserved.DataStatus != "needs_attention" || preserved.FormValues["vacateDayNum"] != 100 {
		t.Fatalf("批量任务覆盖了人工确认值或没有标记冲突：stored=%+v", preserved)
	}
}
