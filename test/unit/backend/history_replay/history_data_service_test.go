package history_replay_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	"test-auto-pro-v2/internal/service"
)

type historyPlanRepository struct {
	repository.PlanRepository
	plan model.Plan
}

// Get 返回当前测试计划并保持账号与目标标识只在服务端读取。
func (r *historyPlanRepository) Get(_ context.Context, id uint64) (model.Plan, error) {
	if id != r.plan.ID {
		return model.Plan{}, repository.ErrPlanNotFound
	}
	return r.plan, nil
}

type historyPathRepository struct {
	repository.ExecutionPathRepository
	path model.ExecutionPath
}

// Get 按计划归属返回当前测试路径。
func (r *historyPathRepository) Get(_ context.Context, planID, pathID uint64) (model.ExecutionPath, error) {
	if planID != r.path.PlanID || pathID != r.path.ID {
		return model.ExecutionPath{}, repository.ErrExecutionPathNotFound
	}
	return r.path, nil
}

type historyTargetReader struct {
	identity   target.HistoryIdentity
	candidates []target.HistoryInstance
	sources    map[string]target.HistorySnapshotSource
	lastFlow   string
	lastForm   string
	lastPage   string
}

// HistoryIdentity 返回目标详情原字段构成的当前运行时身份。
func (r *historyTargetReader) HistoryIdentity(context.Context, string, string, string) (target.HistoryIdentity, error) {
	return r.identity, nil
}

// HistoryCandidates 记录精确身份过滤参数并返回目标候选摘要。
func (r *historyTargetReader) HistoryCandidates(_ context.Context, _ string, flowCode, formName, flowName string, page, pageSize int) (target.Page[target.HistoryInstance], error) {
	r.lastFlow, r.lastForm, r.lastPage = flowCode, formName, flowName
	return target.Page[target.HistoryInstance]{Items: append([]target.HistoryInstance(nil), r.candidates...), Page: page, PageSize: pageSize, Total: len(r.candidates)}, nil
}

// ReadHistorySnapshot 只按不透明候选键返回目标原始数据读取结果。
func (r *historyTargetReader) ReadHistorySnapshot(_ context.Context, _ string, flowCode, formName, flowName, candidateKey string) (target.HistorySnapshotSource, error) {
	r.lastFlow, r.lastForm, r.lastPage = flowCode, formName, flowName
	value, found := r.sources[candidateKey]
	if !found {
		return target.HistorySnapshotSource{}, service.ErrTargetFlowNotFound
	}
	return value, nil
}

type historyMemoryStore struct {
	repository.HistoryReplayStore
	nextID      uint64
	snapshots   map[uint64]model.HistorySnapshot
	byCandidate map[string]uint64
	defaults    map[uint64]repository.HistoryDefaultRecord
	paths       map[uint64]repository.HistoryPathSourceRecord
}

// newHistoryMemoryStore 创建仅覆盖 T02 来源事务的内存仓储。
func newHistoryMemoryStore() *historyMemoryStore {
	return &historyMemoryStore{
		nextID: 1, snapshots: map[uint64]model.HistorySnapshot{}, byCandidate: map[string]uint64{},
		defaults: map[uint64]repository.HistoryDefaultRecord{}, paths: map[uint64]repository.HistoryPathSourceRecord{},
	}
}

// SaveSnapshot 保存候选唯一快照并拒绝相同候选的正文变化。
func (s *historyMemoryStore) SaveSnapshot(_ context.Context, snapshot model.HistorySnapshot) (model.HistorySnapshot, error) {
	key := historyMemoryCandidateKey(snapshot.PlanID, snapshot.CandidateKey)
	if id, found := s.byCandidate[key]; found {
		existing := s.snapshots[id]
		if existing.SourceDigest != snapshot.SourceDigest {
			return model.HistorySnapshot{}, repository.ErrHistoryRevisionConflict
		}
		return existing, nil
	}
	snapshot.ID = s.nextID
	s.nextID++
	s.snapshots[snapshot.ID] = snapshot
	s.byCandidate[key] = snapshot.ID
	return snapshot, nil
}

// SaveDefaultWithSnapshot 原子模拟默认来源修订与快照保存。
func (s *historyMemoryStore) SaveDefaultWithSnapshot(ctx context.Context, snapshot model.HistorySnapshot, record repository.HistoryDefaultRecord, expectedRevision uint64, now time.Time) (model.HistorySnapshot, repository.HistoryDefaultRecord, error) {
	current, found := s.defaults[record.PlanID]
	if found && current.IdempotencyKey == record.IdempotencyKey {
		existing := s.snapshots[current.SnapshotID]
		if existing.CandidateKey != snapshot.CandidateKey {
			return model.HistorySnapshot{}, repository.HistoryDefaultRecord{}, repository.ErrHistoryRevisionConflict
		}
		return existing, current, nil
	}
	if (!found && expectedRevision != 0) || (found && current.Revision != expectedRevision) {
		return model.HistorySnapshot{}, repository.HistoryDefaultRecord{}, repository.ErrHistoryRevisionConflict
	}
	persisted, err := s.SaveSnapshot(ctx, snapshot)
	if err != nil {
		return model.HistorySnapshot{}, repository.HistoryDefaultRecord{}, err
	}
	record.SnapshotID, record.UpdatedAt = persisted.ID, now
	if found {
		record.Revision, record.CreatedAt = current.Revision+1, current.CreatedAt
	} else {
		record.Revision, record.CreatedAt = 1, now
	}
	s.defaults[record.PlanID] = record
	return persisted, record, nil
}

// SavePathSourceWithSnapshot 原子模拟路径独立来源修订与快照保存。
func (s *historyMemoryStore) SavePathSourceWithSnapshot(ctx context.Context, _ uint64, snapshot model.HistorySnapshot, record repository.HistoryPathSourceRecord, expectedRevision uint64, now time.Time) (model.HistorySnapshot, repository.HistoryPathSourceRecord, error) {
	current, found := s.paths[record.PathID]
	if found && current.IdempotencyKey == record.IdempotencyKey {
		existing := s.snapshots[current.SnapshotID]
		if current.Mode != record.Mode || existing.CandidateKey != snapshot.CandidateKey {
			return model.HistorySnapshot{}, repository.HistoryPathSourceRecord{}, repository.ErrHistoryRevisionConflict
		}
		return existing, current, nil
	}
	if (!found && expectedRevision != 0) || (found && current.Revision != expectedRevision) {
		return model.HistorySnapshot{}, repository.HistoryPathSourceRecord{}, repository.ErrHistoryRevisionConflict
	}
	persisted, err := s.SaveSnapshot(ctx, snapshot)
	if err != nil {
		return model.HistorySnapshot{}, repository.HistoryPathSourceRecord{}, err
	}
	record.SnapshotID, record.UpdatedAt = persisted.ID, now
	if found {
		record.Revision = current.Revision + 1
	} else {
		record.Revision = 1
	}
	s.paths[record.PathID] = record
	return persisted, record, nil
}

// GetSnapshotByCandidate 读取计划内候选快照。
func (s *historyMemoryStore) GetSnapshotByCandidate(_ context.Context, planID uint64, candidateKey string) (model.HistorySnapshot, error) {
	id, found := s.byCandidate[historyMemoryCandidateKey(planID, candidateKey)]
	if !found {
		return model.HistorySnapshot{}, repository.ErrHistorySnapshotNotFound
	}
	return s.snapshots[id], nil
}

// FindSnapshotByCandidate 区分候选快照未找到与存储错误。
func (s *historyMemoryStore) FindSnapshotByCandidate(ctx context.Context, planID uint64, candidateKey string) (model.HistorySnapshot, bool, error) {
	snapshot, err := s.GetSnapshotByCandidate(ctx, planID, candidateKey)
	if errors.Is(err, repository.ErrHistorySnapshotNotFound) {
		return model.HistorySnapshot{}, false, nil
	}
	return snapshot, err == nil, err
}

// GetSnapshot 按计划归属读取快照。
func (s *historyMemoryStore) GetSnapshot(_ context.Context, planID, snapshotID uint64) (model.HistorySnapshot, error) {
	snapshot, found := s.snapshots[snapshotID]
	if !found || snapshot.PlanID != planID {
		return model.HistorySnapshot{}, repository.ErrHistorySnapshotNotFound
	}
	return snapshot, nil
}

// GetDefault 读取当前计划默认来源。
func (s *historyMemoryStore) GetDefault(_ context.Context, planID uint64) (repository.HistoryDefaultRecord, bool, error) {
	record, found := s.defaults[planID]
	return record, found, nil
}

// SaveDefault 保存已存在快照对应的默认来源修订。
func (s *historyMemoryStore) SaveDefault(_ context.Context, record repository.HistoryDefaultRecord, expectedRevision uint64, now time.Time) (repository.HistoryDefaultRecord, error) {
	current, found := s.defaults[record.PlanID]
	if (!found && expectedRevision != 0) || (found && current.Revision != expectedRevision) {
		return repository.HistoryDefaultRecord{}, repository.ErrHistoryRevisionConflict
	}
	record.UpdatedAt = now
	if found {
		record.Revision, record.CreatedAt = current.Revision+1, current.CreatedAt
	} else {
		record.Revision, record.CreatedAt = 1, now
	}
	s.defaults[record.PlanID] = record
	return record, nil
}

// GetPathSource 读取当前路径来源配置。
func (s *historyMemoryStore) GetPathSource(_ context.Context, pathID uint64) (repository.HistoryPathSourceRecord, bool, error) {
	record, found := s.paths[pathID]
	return record, found, nil
}

// SavePathSource 保存路径动态继承配置且不冻结默认快照。
func (s *historyMemoryStore) SavePathSource(_ context.Context, _ uint64, record repository.HistoryPathSourceRecord, expectedRevision uint64, now time.Time) (repository.HistoryPathSourceRecord, error) {
	current, found := s.paths[record.PathID]
	if (!found && expectedRevision != 0) || (found && current.Revision != expectedRevision) {
		return repository.HistoryPathSourceRecord{}, repository.ErrHistoryRevisionConflict
	}
	record.UpdatedAt = now
	if found {
		record.Revision = current.Revision + 1
	} else {
		record.Revision = 1
	}
	s.paths[record.PathID] = record
	return record, nil
}

// historyMemoryCandidateKey 构造内存仓储的计划内唯一键。
func historyMemoryCandidateKey(planID uint64, candidateKey string) string {
	return string(rune(planID)) + ":" + candidateKey
}

// newHistoryServiceFixture 创建 FormMaking 默认计划、路径、目标读取器和仓储。
func newHistoryServiceFixture() (*service.HistoryDataManager, *historyTargetReader, *historyMemoryStore, target.HistoryInstance) {
	plan := model.Plan{ID: 41, Account: "account-a", FlowSource: "new", TargetObjectID: "template-a", Status: model.PlanStatusNotStarted}
	path := model.ExecutionPath{ID: 51, PlanID: plan.ID, Name: "审批路径"}
	instance := target.HistoryInstance{
		ID: "target-instance-private", FlowCode: "leave-flow", FlowName: "请假审批", FormName: "请假单（测试公司）",
		Title: "张三请假", BusinessSummary: "年假 2 天", Initiator: "张三", CompanyName: "测试公司",
		CreatedAt: "2026-08-31 10:00:00", Status: "end", StatusName: "已结束",
	}
	reader := &historyTargetReader{
		identity: target.HistoryIdentity{
			FlowCode: instance.FlowCode, FormName: instance.FormName, FlowName: instance.FlowName,
			RenderType: target.FormRenderTypeFormMaking, TemplateSummary: map[string]any{"runtimeVersionDigest": "version-current"},
		},
		candidates: []target.HistoryInstance{instance}, sources: map[string]target.HistorySnapshotSource{},
	}
	store := newHistoryMemoryStore()
	manager := service.NewHistoryDataService(
		service.NewPlanService(&historyPlanRepository{plan: plan}),
		&historyPathRepository{path: path}, reader, store,
	)
	return manager, reader, store, instance
}

// TestHistoryCandidatesExposeOnlyOpaqueSummary 验证候选响应不泄露目标 ID 或完整正文，并标记非完成状态。
func TestHistoryCandidatesExposeOnlyOpaqueSummary(t *testing.T) {
	manager, reader, _, completed := newHistoryServiceFixture()
	terminated := completed
	terminated.ID, terminated.Status, terminated.StatusName = "target-termination-private", "termination", "已终止"
	reader.candidates = []target.HistoryInstance{completed, terminated}
	page, err := manager.Candidates(context.Background(), 41, 0, "", 1, 20)
	if err != nil {
		t.Fatalf("读取历史候选失败：%v", err)
	}
	if reader.lastFlow != completed.FlowCode || reader.lastForm != completed.FormName || reader.lastPage != completed.FlowName {
		t.Fatalf("候选没有沿用目标原始身份字段：flow=%q form=%q page=%q", reader.lastFlow, reader.lastForm, reader.lastPage)
	}
	if len(page.Items) != 2 || len(page.Items[0].CandidateKey) != 64 || page.Items[0].CandidateKey == completed.ID {
		t.Fatalf("候选不透明键不正确：%+v", page.Items)
	}
	if page.Items[0].Completeness != "complete" || page.Items[1].Completeness != "partial" || page.Items[1].IntegrityNotice == "" {
		t.Fatalf("完成与非完成状态提示不正确：%+v", page.Items)
	}
	encoded, _ := json.Marshal(page)
	if string(encoded) == "" || containsAny(string(encoded), completed.ID, terminated.ID, "rawFormData", "sid") {
		t.Fatalf("候选响应泄露目标内部字段：%s", encoded)
	}
}

// TestHistoryDefaultSnapshotPreservesDeepRawData 验证 FormMaking 深层对象、数组和辅助字段原样进入不可变快照。
func TestHistoryDefaultSnapshotPreservesDeepRawData(t *testing.T) {
	manager, reader, store, instance := newHistoryServiceFixture()
	raw := map[string]any{
		"leaveDays":    float64(2),
		"nested":       map[string]any{"children": []any{map[string]any{"code": "A", "virtual": true}}},
		"systemHelper": map[string]any{"privateValue": "preserve-me"},
	}
	key := service.HistoryCandidateKey("account-a", instance)
	reader.sources[key] = target.HistorySnapshotSource{
		Instance: instance, RenderType: target.FormRenderTypeFormMaking,
		TemplateSummary: map[string]any{"runtimeVersionDigest": "version-current"}, RawFormData: raw,
	}
	result, err := manager.SaveDefault(context.Background(), 41, model.HistoryDefaultSaveInput{CandidateKey: key}, "123e4567-e89b-12d3-a456-426614174201")
	if err != nil {
		t.Fatalf("保存计划默认历史来源失败：%v", err)
	}
	stored := store.snapshots[result.SnapshotID]
	if !reflect.DeepEqual(stored.RawFormData, raw) {
		t.Fatalf("目标原始数据结构发生变化：got=%#v want=%#v", stored.RawFormData, raw)
	}
	nested := raw["nested"].(map[string]any)["children"].([]any)[0].(map[string]any)
	nested["code"] = "MUTATED"
	storedNested := stored.RawFormData["nested"].(map[string]any)["children"].([]any)[0].(map[string]any)
	if storedNested["code"] != "A" {
		t.Fatalf("快照仍引用目标读取对象：%#v", stored.RawFormData)
	}
	encoded, _ := json.Marshal(result)
	if containsAny(string(encoded), "preserve-me", instance.ID, "rawFormData") {
		t.Fatalf("来源响应泄露完整正文或目标 ID：%s", encoded)
	}
	if result.DataStatus != model.HistoryDataStatusNeedsInput || !hasHistoryIssue(result.Issues, "HISTORY_REPLAY_REQUIRED") {
		t.Fatalf("来源选择在 runtime 校验前错误宣称就绪：%+v", result)
	}
}

// TestHistoryDefaultReportsRuntimeVersionDifference 验证历史模板正文摘要与当前目标版本不同会标记 affected。
func TestHistoryDefaultReportsRuntimeVersionDifference(t *testing.T) {
	manager, reader, _, instance := newHistoryServiceFixture()
	key := service.HistoryCandidateKey("account-a", instance)
	reader.sources[key] = target.HistorySnapshotSource{
		Instance: instance, RenderType: target.FormRenderTypeFormMaking,
		TemplateSummary: map[string]any{"runtimeVersionDigest": "version-history"}, RawFormData: map[string]any{"value": "kept"},
	}
	result, err := manager.SaveDefault(context.Background(), 41, model.HistoryDefaultSaveInput{CandidateKey: key}, "123e4567-e89b-12d3-a456-426614174202")
	if err != nil {
		t.Fatalf("保存不同版本历史来源失败：%v", err)
	}
	if result.DataStatus != model.HistoryDataStatusAffected || !hasHistoryIssue(result.Issues, "HISTORY_RUNTIME_VERSION_CHANGED") {
		t.Fatalf("目标版本差异没有形成结构化摘要：%+v", result)
	}
}

// TestHistoryPathDefaultFollowsLatestPlanDefault 验证路径继承只保存模式并实时解析最新计划默认快照。
func TestHistoryPathDefaultFollowsLatestPlanDefault(t *testing.T) {
	manager, reader, store, first := newHistoryServiceFixture()
	firstKey := service.HistoryCandidateKey("account-a", first)
	reader.sources[firstKey] = target.HistorySnapshotSource{Instance: first, RenderType: target.FormRenderTypeFormMaking, TemplateSummary: map[string]any{"runtimeVersionDigest": "version-current"}, RawFormData: map[string]any{"value": "first"}}
	if _, err := manager.SaveDefault(context.Background(), 41, model.HistoryDefaultSaveInput{CandidateKey: firstKey}, "123e4567-e89b-12d3-a456-426614174203"); err != nil {
		t.Fatalf("保存第一个默认来源失败：%v", err)
	}
	pathSource, err := manager.SavePathSource(context.Background(), 41, 51, model.HistoryPathSourceInput{Mode: model.HistorySourceModeDefault}, "123e4567-e89b-12d3-a456-426614174204")
	if err != nil || pathSource.Summary == nil || pathSource.Summary.CandidateKey != firstKey {
		t.Fatalf("保存路径动态继承失败：source=%+v err=%v", pathSource, err)
	}
	if store.paths[51].SnapshotID != 0 {
		t.Fatalf("路径继承错误冻结了默认快照：%+v", store.paths[51])
	}
	second := first
	second.ID, second.Title, second.CreatedAt = "target-instance-second", "李四请假", "2026-09-01 09:00:00"
	secondKey := service.HistoryCandidateKey("account-a", second)
	reader.sources[secondKey] = target.HistorySnapshotSource{Instance: second, RenderType: target.FormRenderTypeFormMaking, TemplateSummary: map[string]any{"runtimeVersionDigest": "version-current"}, RawFormData: map[string]any{"value": "second"}}
	if _, err := manager.SaveDefault(context.Background(), 41, model.HistoryDefaultSaveInput{CandidateKey: secondKey, Revision: 1}, "123e4567-e89b-12d3-a456-426614174205"); err != nil {
		t.Fatalf("替换计划默认来源失败：%v", err)
	}
	reader.candidates = []target.HistoryInstance{second}
	page, err := manager.Candidates(context.Background(), 41, 51, "", 1, 20)
	if err != nil || page.PathSource == nil || page.PathSource.Summary == nil || page.PathSource.Summary.CandidateKey != secondKey {
		t.Fatalf("路径没有实时继承最新默认来源：page=%+v err=%v", page, err)
	}
}

// TestNoFormHistorySnapshotDoesNotRequireVuePageMapping 验证 NoFormFlow 原始页面数据不依赖旧 VuePage 规则也能快照。
func TestNoFormHistorySnapshotDoesNotRequireVuePageMapping(t *testing.T) {
	manager, reader, store, instance := newHistoryServiceFixture()
	instance.FormName, instance.FlowName, instance.ID = "", "NoFormFlow 请款页", "target-noform-private"
	reader.identity = target.HistoryIdentity{
		FlowCode: instance.FlowCode, FlowName: instance.FlowName, RenderType: target.FormRenderTypeVueCustom,
		TemplateSummary: map[string]any{"runtimeVersionDigest": "noform-page-version"},
	}
	key := service.HistoryCandidateKey("account-a", instance)
	raw := map[string]any{"pageState": map[string]any{"lines": []any{map[string]any{"amount": float64(88), "custom": "kept"}}}}
	reader.sources[key] = target.HistorySnapshotSource{
		Instance: instance, RenderType: target.FormRenderTypeVueCustom,
		TemplateSummary: map[string]any{"runtimeVersionDigest": "noform-page-version"}, RawFormData: raw,
	}
	result, err := manager.SaveDefault(context.Background(), 41, model.HistoryDefaultSaveInput{CandidateKey: key}, "123e4567-e89b-12d3-a456-426614174206")
	if err != nil {
		t.Fatalf("保存 NoFormFlow 历史来源失败：%v", err)
	}
	stored := store.snapshots[result.SnapshotID]
	if stored.RuntimeType != string(target.FormRenderTypeVueCustom) || !reflect.DeepEqual(stored.RawFormData, raw) {
		t.Fatalf("NoFormFlow 原始数据未按既有 runtime 协议保存：%+v", stored)
	}
	if _, exists := stored.TemplateSummary["vuePage"]; exists {
		t.Fatalf("NoFormFlow 快照错误依赖旧 VuePage 映射：%+v", stored.TemplateSummary)
	}
}

// hasHistoryIssue 判断来源问题是否包含指定稳定代码。
func hasHistoryIssue(issues []model.HistoryDataIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

// containsAny 判断文本是否包含任一敏感片段。
func containsAny(value string, fragments ...string) bool {
	for _, fragment := range fragments {
		if fragment != "" && stringContains(value, fragment) {
			return true
		}
	}
	return false
}

// stringContains 使用标准字节查找避免测试引入额外依赖。
func stringContains(value, fragment string) bool {
	if len(fragment) > len(value) {
		return false
	}
	for index := 0; index+len(fragment) <= len(value); index++ {
		if value[index:index+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
