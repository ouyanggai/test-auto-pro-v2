package history_replay_test

import (
	"context"
	"encoding/json"
	"reflect"
	"sync"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	"test-auto-pro-v2/internal/service"
)

type replayPlanRepo struct {
	repository.PlanRepository
	plan model.Plan
}

// Get 返回回放所属计划，身份由服务端计划事实提供。
func (r *replayPlanRepo) Get(_ context.Context, id uint64) (model.Plan, error) {
	if id != r.plan.ID {
		return model.Plan{}, repository.ErrPlanNotFound
	}
	return r.plan, nil
}

type replayPathRepo struct {
	repository.ExecutionPathRepository
	created model.ExecutionPath
	current model.ExecutionPath
}

// Get 返回当前路径，worker 通过它复验路径修订和真实 choices。
func (r *replayPathRepo) Get(_ context.Context, planID, pathID uint64) (model.ExecutionPath, error) {
	if r.current.PlanID != planID || r.current.ID != pathID {
		return model.ExecutionPath{}, repository.ErrExecutionPathNotFound
	}
	return r.current, nil
}

// GetMany 返回创建任务时明确勾选的路径快照。
func (r *replayPathRepo) GetMany(_ context.Context, planID uint64, pathIDs []uint64) ([]model.ExecutionPath, error) {
	if len(pathIDs) == 1 && pathIDs[0] == r.created.ID && r.created.PlanID == planID {
		return []model.ExecutionPath{r.created}, nil
	}
	return []model.ExecutionPath{}, nil
}

type replayTargetReader struct {
	snapshot target.PathConfigurationSnapshot
	calls    int
}

// PathConfigurationSnapshot 返回目标真实树和运行时类型，不读取 VuePage 映射。
func (r *replayTargetReader) PathConfigurationSnapshot(context.Context, string, string, string) (target.PathConfigurationSnapshot, error) {
	r.calls++
	return r.snapshot, nil
}

type replayValidator struct {
	mu     sync.Mutex
	calls  int
	render target.FormRenderType
	values map[string]any
}

// Validate 记录 form-runtime 收到的原始 map，并返回通过结果。
func (r *replayValidator) Validate(_ context.Context, render target.FormRenderType, values map[string]any) (model.HistoryRuntimeValidation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	r.render, r.values = render, values
	return model.HistoryRuntimeValidation{Accepted: true}, nil
}

type replayStore struct {
	repository.HistoryReplayStore
	mu        sync.Mutex
	nextID    uint64
	snapshot  model.HistorySnapshot
	defaultAt repository.HistoryDefaultRecord
	job       model.HistoryReplayJob
	items     []model.HistoryReplayItem
}

// GetPathSource 返回未设置独立覆盖，任务因此继承计划默认来源。
func (s *replayStore) GetPathSource(context.Context, uint64) (repository.HistoryPathSourceRecord, bool, error) {
	return repository.HistoryPathSourceRecord{}, false, nil
}

// GetDefault 返回计划默认快照绑定。
func (s *replayStore) GetDefault(_ context.Context, planID uint64) (repository.HistoryDefaultRecord, bool, error) {
	return s.defaultAt, s.defaultAt.PlanID == planID, nil
}

// GetSnapshot 返回不可变快照深复制，避免 worker 改写 fixture。
func (s *replayStore) GetSnapshot(_ context.Context, planID, snapshotID uint64) (model.HistorySnapshot, error) {
	if s.snapshot.PlanID != planID || s.snapshot.ID != snapshotID {
		return model.HistorySnapshot{}, repository.ErrHistorySnapshotNotFound
	}
	copy := s.snapshot
	copy.RawFormData = cloneReplayMap(copy.RawFormData)
	return copy, nil
}

// CreateReplay 保存一份任务检查点并返回持久化事实。
func (s *replayStore) CreateReplay(_ context.Context, job model.HistoryReplayJob, requested []model.HistoryReplayItem) (model.HistoryReplayJob, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job.ID != "" {
		return s.job, false, nil
	}
	job.Status, job.Total, job.Pending = model.HistoryReplayStatusQueued, len(requested), len(requested)
	job.CreatedAt, job.UpdatedAt = time.Now().UTC(), time.Now().UTC()
	s.job = job
	s.items = make([]model.HistoryReplayItem, len(requested))
	for i, item := range requested {
		item.ID, item.JobID, item.Status = s.nextID+1, job.ID, model.HistoryReplayItemStatusPending
		s.nextID = item.ID
		s.items[i] = item
	}
	return job, true, nil
}

// GetReplay 返回任务聚合事实。
func (s *replayStore) GetReplay(_ context.Context, planID uint64, jobID string) (model.HistoryReplayJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job.PlanID != planID || s.job.ID != jobID {
		return model.HistoryReplayJob{}, repository.ErrHistoryReplayNotFound
	}
	return s.job, nil
}

// FindActiveReplay 返回内存任务中的唯一活动状态，模拟刷新页面后的恢复入口。
func (s *replayStore) FindActiveReplay(_ context.Context, planID uint64) (model.HistoryReplayJob, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job.PlanID != planID || (s.job.Status != model.HistoryReplayStatusQueued && s.job.Status != model.HistoryReplayStatusRunning) {
		return model.HistoryReplayJob{}, false, nil
	}
	return s.job, true, nil
}

// ListRecoverableReplays 返回仍需处理的内存任务，覆盖服务重启恢复入口。
func (s *replayStore) ListRecoverableReplays(context.Context) ([]model.HistoryReplayJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job.ID == "" || (s.job.Status != model.HistoryReplayStatusQueued && s.job.Status != model.HistoryReplayStatusRunning) {
		return []model.HistoryReplayJob{}, nil
	}
	return []model.HistoryReplayJob{s.job}, nil
}

// UpdateReplayStatus 执行取消与恢复状态转换，running 检查点退回 pending 但终态不重做。
func (s *replayStore) UpdateReplayStatus(_ context.Context, planID uint64, jobID, status string, now time.Time) (model.HistoryReplayJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job.PlanID != planID || s.job.ID != jobID {
		return model.HistoryReplayJob{}, repository.ErrHistoryReplayNotFound
	}
	if status == model.HistoryReplayStatusCancelled {
		if s.job.Status != model.HistoryReplayStatusQueued && s.job.Status != model.HistoryReplayStatusRunning {
			return model.HistoryReplayJob{}, repository.ErrHistoryReplayState
		}
		for index := range s.items {
			if s.items[index].Status == model.HistoryReplayItemStatusRunning {
				s.items[index].Status, s.items[index].LeaseOwner, s.items[index].LeaseExpiresAt = model.HistoryReplayItemStatusPending, "", nil
			}
		}
		s.job.Status = status
	} else if status == model.HistoryReplayStatusQueued {
		if s.job.Status != model.HistoryReplayStatusCancelled && s.job.Status != model.HistoryReplayStatusFailed {
			return model.HistoryReplayJob{}, repository.ErrHistoryReplayState
		}
		s.job.Status = status
	} else {
		return model.HistoryReplayJob{}, repository.ErrHistoryReplayState
	}
	s.recountLocked(now)
	return s.job, nil
}

// ClaimReplayItems 模拟数据库租约领取，只有一个 worker 能取得待处理明细。
func (s *replayStore) ClaimReplayItems(_ context.Context, jobID string, limit int, owner string, now time.Time) ([]model.HistoryReplayItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job.ID != jobID {
		return nil, repository.ErrHistoryReplayNotFound
	}
	if s.job.Status == model.HistoryReplayStatusQueued {
		s.job.Status = model.HistoryReplayStatusRunning
	}
	if s.job.Status != model.HistoryReplayStatusRunning {
		return nil, repository.ErrHistoryReplayState
	}
	claimed := make([]model.HistoryReplayItem, 0, limit)
	for index := range s.items {
		item := &s.items[index]
		if item.Status != model.HistoryReplayItemStatusPending && !(item.Status == model.HistoryReplayItemStatusRunning && item.LeaseExpiresAt != nil && !item.LeaseExpiresAt.After(now)) {
			continue
		}
		s.job.FencingToken++
		item.Status, item.LeaseOwner, item.FencingToken = model.HistoryReplayItemStatusRunning, owner, s.job.FencingToken
		expires := now.Add(2 * time.Minute)
		item.LeaseExpiresAt = &expires
		claimed = append(claimed, cloneReplayItem(*item))
		if len(claimed) >= limit {
			break
		}
	}
	if len(claimed) > 0 {
		s.job.Pending = 0
		s.job.Running = len(claimed)
		s.job.UpdatedAt = now
	}
	return claimed, nil
}

// CompleteReplayItem 提交单路径终态并从明细重新计算任务聚合，验证租约归属和 fencing token。
func (s *replayStore) CompleteReplayItem(_ context.Context, jobID string, itemID uint64, result model.HistoryReplayItem, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job.ID != jobID {
		return repository.ErrHistoryReplayNotFound
	}
	for index := range s.items {
		item := &s.items[index]
		if item.ID != itemID {
			continue
		}
		if item.Status != model.HistoryReplayItemStatusRunning || item.LeaseOwner != result.LeaseOwner || item.FencingToken != result.FencingToken {
			return repository.ErrHistoryReplayState
		}
		result.UpdatedAt, result.CompletedAt = now, timePtrReplay(now)
		result.LeaseOwner, result.LeaseExpiresAt = "", nil
		itemCopy := cloneReplayItem(result)
		*item = itemCopy
		s.recountLocked(now)
		return nil
	}
	return repository.ErrHistoryReplayNotFound
}

// RecountReplay 读取当前明细终态并更新聚合计数，模拟持久化层的真实重算。
func (s *replayStore) RecountReplay(_ context.Context, jobID string, now time.Time) (model.HistoryReplayJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job.ID != jobID {
		return model.HistoryReplayJob{}, repository.ErrHistoryReplayNotFound
	}
	s.recountLocked(now)
	return s.job, nil
}

// ListReplayItems 提供按明细自增 ID 的游标分页，测试公开响应不携带租约字段。
func (s *replayStore) ListReplayItems(_ context.Context, planID uint64, jobID string, cursor uint64, limit int) (model.HistoryReplayItemPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job.PlanID != planID || s.job.ID != jobID {
		return model.HistoryReplayItemPage{}, repository.ErrHistoryReplayNotFound
	}
	page := model.HistoryReplayItemPage{}
	for _, item := range s.items {
		if item.ID <= cursor {
			continue
		}
		page.Items = append(page.Items, cloneReplayItem(item))
		if len(page.Items) == limit {
			break
		}
	}
	if len(page.Items) == limit {
		for _, item := range s.items {
			if item.ID > page.Items[len(page.Items)-1].ID {
				page.NextCursor = page.Items[len(page.Items)-1].ID
				break
			}
		}
	}
	return page, nil
}

// recountLocked 让 fake 仓储的任务状态与明细真实状态保持一致。
func (s *replayStore) recountLocked(now time.Time) {
	s.job.Total = len(s.items)
	s.job.Pending, s.job.Running, s.job.Ready, s.job.NeedsInput, s.job.Affected, s.job.Failed, s.job.Cancelled = 0, 0, 0, 0, 0, 0, 0
	for _, item := range s.items {
		switch item.Status {
		case model.HistoryReplayItemStatusPending:
			s.job.Pending++
		case model.HistoryReplayItemStatusRunning:
			s.job.Running++
		case model.HistoryReplayItemStatusReady:
			s.job.Ready++
		case model.HistoryReplayItemStatusNeedsInput:
			s.job.NeedsInput++
		case model.HistoryReplayItemStatusAffected:
			s.job.Affected++
		case model.HistoryReplayItemStatusFailed:
			s.job.Failed++
		}
	}
	s.job.UpdatedAt = now
	if s.job.Status == model.HistoryReplayStatusRunning && s.job.Pending == 0 && s.job.Running == 0 {
		s.job.Status = model.HistoryReplayStatusCompleted
		s.job.CompletedAt = timePtrReplay(now)
	}
}

// cloneReplayMap 深复制原始表单 map，确保测试能够检测 runtime 是否收到同一份嵌套正文。
func cloneReplayMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = cloneReplayValue(value)
	}
	return result
}

// cloneReplayValue 递归复制 map 和数组，防止回放补丁改写来源快照。
func cloneReplayValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneReplayMap(typed)
	case []any:
		result := make([]any, len(typed))
		for index, element := range typed {
			result[index] = cloneReplayValue(element)
		}
		return result
	default:
		return value
	}
}

// cloneReplayItem 深复制明细中会被 worker 修改的切片和原始正文。
func cloneReplayItem(item model.HistoryReplayItem) model.HistoryReplayItem {
	item.Issues = append([]model.HistoryDataIssue(nil), item.Issues...)
	item.BranchPatches = append([]model.HistoryBranchPatch(nil), item.BranchPatches...)
	item.EffectiveFormData = cloneReplayMap(item.EffectiveFormData)
	return item
}

// timePtrReplay 返回独立时间指针，避免测试 fixture 共享可变地址。
func timePtrReplay(value time.Time) *time.Time {
	copy := value
	return &copy
}

// replayServiceFixture 构造一个包含默认历史快照和真实条件树的服务夹具。
func replayServiceFixture(pathRevision, currentRevision uint64, render target.FormRenderType) (*service.HistoryReplayService, *replayStore, *replayValidator, *replayTargetReader) {
	plan := model.Plan{ID: 91, Account: "tester", FlowSource: "flow-source", TargetObjectID: "target-1", Status: model.PlanStatusNotStarted}
	path := model.ExecutionPath{ID: 101, PlanID: plan.ID, ConfigurationRevision: pathRevision, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route-1", BranchID: "branch-a"}}}
	tree := conditionTreeID("route-1", []target.FlowBranchTemplate{
		{ID: "branch-a", Sort: 1, Conditions: []target.FlowCondition{{FieldA: "amount", ValueB: "2", Judge: "eq"}}},
		{ID: "branch-default", Sort: 2},
	})
	store := &replayStore{
		snapshot:  model.HistorySnapshot{ID: 9, PlanID: plan.ID, RuntimeType: string(render), RawFormData: map[string]any{"amount": 2, "nested": map[string]any{"kept": true}}},
		defaultAt: repository.HistoryDefaultRecord{PlanID: plan.ID, SnapshotID: 9},
	}
	paths := &replayPathRepo{created: path, current: path}
	paths.current.ConfigurationRevision = currentRevision
	validator := &replayValidator{}
	targetReader := &replayTargetReader{snapshot: target.PathConfigurationSnapshot{Tree: tree, RenderType: render}}
	plans := service.NewPlanService(&replayPlanRepo{plan: plan})
	replay := service.NewHistoryReplayService(plans, paths, targetReader, store)
	replay.SetRuntimeValidator(validator)
	return replay, store, validator, targetReader
}

// waitReplayCompletion 等待后台 worker 写入完成聚合，超时即说明任务没有可观察终态。
func waitReplayCompletion(t *testing.T, replay *service.HistoryReplayService, planID uint64, jobID string) model.HistoryReplayJob {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err := replay.Get(context.Background(), planID, jobID)
		if err != nil {
			t.Fatalf("读取回放任务失败：%v", err)
		}
		if job.Status == model.HistoryReplayStatusCompleted || job.Status == model.HistoryReplayStatusFailed || job.Status == model.HistoryReplayStatusCancelled {
			return job
		}
		time.Sleep(time.Millisecond * 5)
	}
	t.Fatalf("回放任务未在期限内完成")
	return model.HistoryReplayJob{}
}

// TestHistoryReplayServiceReplaysRawSnapshotAndRuntimeValidation 验证原始嵌套数据经过分支补丁后直接交给 runtime。
func TestHistoryReplayServiceReplaysRawSnapshotAndRuntimeValidation(t *testing.T) {
	replay, store, validator, targetReader := replayServiceFixture(4, 4, target.FormRenderTypeFormMaking)
	key := "123e4567-e89b-12d3-a456-426614174700"
	job, err := replay.Create(context.Background(), 91, model.HistoryReplayCreateInput{PathIDs: []uint64{101}}, key)
	if err != nil {
		t.Fatalf("创建回放任务失败：%v", err)
	}
	completed := waitReplayCompletion(t, replay, 91, job.ID)
	store.mu.Lock()
	debugItem := cloneReplayItem(store.items[0])
	store.mu.Unlock()
	if completed.Ready != 1 || completed.Total != 1 || completed.Pending != 0 || completed.Running != 0 {
		t.Fatalf("任务聚合计数不正确：job=%#v item=%#v", completed, debugItem)
	}
	store.mu.Lock()
	item := cloneReplayItem(store.items[0])
	store.mu.Unlock()
	if item.Status != model.HistoryReplayItemStatusReady || item.DataStatus != model.HistoryDataStatusReady || len(item.Issues) != 0 {
		t.Fatalf("路径终态不正确：%#v", item)
	}
	if !reflect.DeepEqual(item.EffectiveFormData["nested"], map[string]any{"kept": true}) {
		t.Fatalf("嵌套原始表单数据未保留：%#v", item.EffectiveFormData)
	}
	validator.mu.Lock()
	gotValues := cloneReplayMap(validator.values)
	gotRender, calls := validator.render, validator.calls
	validator.mu.Unlock()
	if calls != 1 || gotRender != target.FormRenderTypeFormMaking || !reflect.DeepEqual(gotValues["nested"], map[string]any{"kept": true}) {
		t.Fatalf("runtime 未收到原始 map：calls=%d render=%s values=%#v", calls, gotRender, gotValues)
	}
	if targetReader.calls != 1 {
		t.Fatalf("目标真实流程树读取次数不正确：%d", targetReader.calls)
	}
}

// TestHistoryReplayServiceMarksPathRevisionChangeAffected 验证路径修订变化落确定受影响终态，不伪造数据。
func TestHistoryReplayServiceMarksPathRevisionChangeAffected(t *testing.T) {
	replay, store, _, _ := replayServiceFixture(8, 9, target.FormRenderTypeVueCustom)
	job, err := replay.Create(context.Background(), 91, model.HistoryReplayCreateInput{PathIDs: []uint64{101}}, "123e4567-e89b-12d3-a456-426614174701")
	if err != nil {
		t.Fatalf("创建修订变化回放任务失败：%v", err)
	}
	completed := waitReplayCompletion(t, replay, 91, job.ID)
	if completed.Affected != 1 || completed.Ready != 0 {
		t.Fatalf("修订变化聚合不正确：%#v", completed)
	}
	store.mu.Lock()
	item := cloneReplayItem(store.items[0])
	store.mu.Unlock()
	if item.Status != model.HistoryReplayItemStatusAffected || item.DataStatus != model.HistoryDataStatusAffected || len(item.EffectiveFormData) != 0 {
		t.Fatalf("修订变化明细未落确定终态：%#v", item)
	}
	if len(item.Issues) != 1 || item.Issues[0].Code != "HISTORY_PATH_REVISION_CHANGED" {
		t.Fatalf("修订变化问题不明确：%#v", item.Issues)
	}
}

// TestHistoryReplayServiceReplaysVueCustomRawSnapshot 验证 NoFormFlow 自定义页面沿用同一原始 map 和任务状态机。
func TestHistoryReplayServiceReplaysVueCustomRawSnapshot(t *testing.T) {
	replay, store, validator, targetReader := replayServiceFixture(6, 6, target.FormRenderTypeVueCustom)
	store.mu.Lock()
	store.snapshot.RawFormData = map[string]any{
		"pageState": map[string]any{"rows": []any{map[string]any{"amount": json.Number("2"), "customValue": map[string]any{"code": "X"}}}},
	}
	store.mu.Unlock()
	targetReader.snapshot.Tree.ConditionNodes[0].Conditions[0].FieldA = "pageState.rows[].amount"
	job, err := replay.Create(context.Background(), 91, model.HistoryReplayCreateInput{PathIDs: []uint64{101}}, "123e4567-e89b-12d3-a456-426614174702")
	if err != nil {
		t.Fatalf("创建自定义页面回放任务失败：%v", err)
	}
	completed := waitReplayCompletion(t, replay, 91, job.ID)
	if completed.Ready != 1 || completed.NeedsInput != 0 {
		t.Fatalf("自定义页面任务聚合不正确：%#v", completed)
	}
	validator.mu.Lock()
	gotRender, gotValues, calls := validator.render, cloneReplayMap(validator.values), validator.calls
	validator.mu.Unlock()
	if calls != 1 || gotRender != target.FormRenderTypeVueCustom {
		t.Fatalf("自定义页面 runtime 协议不正确：calls=%d render=%s", calls, gotRender)
	}
	pageState, ok := gotValues["pageState"].(map[string]any)
	rows, rowsOK := pageState["rows"].([]any)
	row, rowOK := func() (map[string]any, bool) {
		if len(rows) != 1 {
			return nil, false
		}
		value, ok := rows[0].(map[string]any)
		return value, ok
	}()
	customValue, customOK := row["customValue"].(map[string]any)
	amount, amountOK := row["amount"].(json.Number)
	if !ok || !rowsOK || !rowOK || !amountOK || amount != json.Number("2") || !customOK || customValue["code"] != "X" {
		t.Fatalf("自定义页面原始正文未完整透传：ok=%v rows=%v row=%v amount=%#v(%T) custom=%v customCode=%#v values=%#v", ok, rowsOK, rowOK, row["amount"], row["amount"], customOK, customValue["code"], gotValues)
	}
	if targetReader.calls != 1 {
		t.Fatalf("自定义页面目标结构读取次数不正确：%d", targetReader.calls)
	}
}

// TestHistoryReplayServiceIdempotencyAndResumeCheckpoint 验证重复请求不创建第二任务，取消后可从同一检查点恢复。
func TestHistoryReplayServiceIdempotencyAndResumeCheckpoint(t *testing.T) {
	replay, store, _, _ := replayServiceFixture(6, 6, target.FormRenderTypeVueCustom)
	key := "123e4567-e89b-12d3-a456-426614174703"
	first, err := replay.Create(context.Background(), 91, model.HistoryReplayCreateInput{PathIDs: []uint64{101}}, key)
	if err != nil {
		t.Fatalf("创建幂等回放任务失败：%v", err)
	}
	completed := waitReplayCompletion(t, replay, 91, first.ID)
	second, err := replay.Create(context.Background(), 91, model.HistoryReplayCreateInput{PathIDs: []uint64{101}}, key)
	if err != nil || second.ID != first.ID || completed.Status != model.HistoryReplayStatusCompleted {
		t.Fatalf("重复幂等请求未返回原任务：first=%#v second=%#v completed=%#v err=%v", first, second, completed, err)
	}
	if _, err := replay.Cancel(context.Background(), 91, first.ID); err == nil {
		t.Fatalf("已完成任务不应允许取消")
	}
	store.mu.Lock()
	createdItems := len(store.items)
	store.mu.Unlock()
	if createdItems != 1 {
		t.Fatalf("幂等请求创建了重复明细：%d", createdItems)
	}
}

// TestHistoryReplayServiceCancelAndResumeKeepsCheckpoint 验证取消不会清除检查点，恢复只重新领取未完成路径。
func TestHistoryReplayServiceCancelAndResumeKeepsCheckpoint(t *testing.T) {
	replay, store, _, _ := replayServiceFixture(6, 6, target.FormRenderTypeVueCustom)
	started := make(chan struct{})
	release := make(chan struct{})
	var startedOnce sync.Once
	replay.SetRuntimeValidator(service.HistoryReplayRuntimeValidatorFunc(func(ctx context.Context, _ target.FormRenderType, _ map[string]any) (model.HistoryRuntimeValidation, error) {
		startedOnce.Do(func() { close(started) })
		select {
		case <-release:
		case <-ctx.Done():
			return model.HistoryRuntimeValidation{}, ctx.Err()
		}
		return model.HistoryRuntimeValidation{Accepted: true}, nil
	}))
	key := "123e4567-e89b-12d3-a456-426614174704"
	job, err := replay.Create(context.Background(), 91, model.HistoryReplayCreateInput{PathIDs: []uint64{101}}, key)
	if err != nil {
		t.Fatalf("创建可取消回放任务失败：%v", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatalf("worker 未领取路径并进入 runtime")
	}
	cancelled, err := replay.Cancel(context.Background(), 91, job.ID)
	if err != nil || cancelled.Status != model.HistoryReplayStatusCancelled || cancelled.Pending != 1 {
		t.Fatalf("取消未保留待处理检查点：job=%#v err=%v", cancelled, err)
	}
	close(release)
	time.Sleep(10 * time.Millisecond)
	resumed, err := replay.Resume(context.Background(), 91, job.ID)
	if err != nil || resumed.Status != model.HistoryReplayStatusQueued {
		t.Fatalf("恢复任务未重新排队：job=%#v err=%v", resumed, err)
	}
	completed := waitReplayCompletion(t, replay, 91, job.ID)
	if completed.Status != model.HistoryReplayStatusCompleted || completed.Ready != 1 {
		t.Fatalf("恢复后任务未完成：%#v", completed)
	}
	store.mu.Lock()
	item := cloneReplayItem(store.items[0])
	store.mu.Unlock()
	if item.Status != model.HistoryReplayItemStatusReady {
		t.Fatalf("恢复后明细未复用原检查点：%#v", item)
	}
}
