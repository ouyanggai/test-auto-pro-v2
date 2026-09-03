package history_replay_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	"test-auto-pro-v2/internal/service"
)

type workspacePathRepository struct {
	repository.ExecutionPathRepository
	paths []model.ExecutionPath
}

// Get 返回指定计划下的完整路径选择，禁止使用列表摘要参与换路判断。
func (r *workspacePathRepository) Get(_ context.Context, planID, pathID uint64) (model.ExecutionPath, error) {
	for _, path := range r.paths {
		if path.PlanID == planID && path.ID == pathID {
			return path, nil
		}
	}
	return model.ExecutionPath{}, repository.ErrExecutionPathNotFound
}

// List 返回指定计划的全部路径，供没有专用匹配器的调用方回退读取。
func (r *workspacePathRepository) List(_ context.Context, planID uint64) ([]model.ExecutionPath, error) {
	result := make([]model.ExecutionPath, 0, len(r.paths))
	for _, path := range r.paths {
		if path.PlanID == planID {
			result = append(result, path)
		}
	}
	return result, nil
}

// FindByChoices 只按目标真实路由键寻找已存在路径，不自动创建新路径。
func (r *workspacePathRepository) FindByChoices(_ context.Context, planID uint64, choices []model.ExecutionPathChoice) (model.ExecutionPath, bool, error) {
	for _, path := range r.paths {
		if path.PlanID == planID && workspaceChoicesEqual(path.Choices, choices) {
			return path, true, nil
		}
	}
	return model.ExecutionPath{}, false, nil
}

type workspaceTargetReader struct {
	snapshot target.PathConfigurationSnapshot
}

// PathConfigurationSnapshot 返回固定的目标流程和原始表单运行时协议。
func (r workspaceTargetReader) PathConfigurationSnapshot(context.Context, string, string, string) (target.PathConfigurationSnapshot, error) {
	return r.snapshot, nil
}

type workspaceHistoryStore struct {
	*historyMemoryStore
	configs map[uint64]repository.HistoryPathConfigRecord
	writes  int
}

// newWorkspaceHistoryStore 创建带路径配置独立数据列的内存历史仓储。
func newWorkspaceHistoryStore() *workspaceHistoryStore {
	return &workspaceHistoryStore{historyMemoryStore: newHistoryMemoryStore(), configs: map[uint64]repository.HistoryPathConfigRecord{}}
}

// GetPathConfig 读取当前路径独立数据配置。
func (s *workspaceHistoryStore) GetPathConfig(_ context.Context, pathID uint64) (repository.HistoryPathConfigRecord, bool, error) {
	record, found := s.configs[pathID]
	return record, found, nil
}

// SavePathConfig 模拟带修订号和幂等键的单路径数据保存。
func (s *workspaceHistoryStore) SavePathConfig(_ context.Context, record repository.HistoryPathConfigRecord, expectedRevision uint64, now time.Time) (repository.HistoryPathConfigRecord, error) {
	return s.saveConfig(record, expectedRevision, now)
}

// SavePathData 模拟换路时来源与目标在一个事务内完成的单次写入。
func (s *workspaceHistoryStore) SavePathData(_ context.Context, _, _, targetPathID uint64, record repository.HistoryPathConfigRecord, expectedRevision uint64, now time.Time) (repository.HistoryPathConfigRecord, error) {
	if record.PathID != targetPathID {
		return repository.HistoryPathConfigRecord{}, repository.ErrExecutionPathNotFound
	}
	return s.saveConfig(record, expectedRevision, now)
}

// saveConfig 执行内存仓储的幂等和修订校验，错误时不增加写计数。
func (s *workspaceHistoryStore) saveConfig(record repository.HistoryPathConfigRecord, expectedRevision uint64, now time.Time) (repository.HistoryPathConfigRecord, error) {
	current, found := s.configs[record.PathID]
	if found && current.IdempotencyKey == record.IdempotencyKey {
		if !reflect.DeepEqual(current.EffectiveFormData, record.EffectiveFormData) {
			return repository.HistoryPathConfigRecord{}, repository.ErrHistoryPathConfigIdempotency
		}
		return current, nil
	}
	if (!found && expectedRevision != 0) || (found && current.Revision != expectedRevision) {
		return repository.HistoryPathConfigRecord{}, repository.ErrHistoryPathConfigConflict
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	record.Revision = current.Revision + 1
	if record.DataRevision <= current.DataRevision {
		record.DataRevision = current.DataRevision + 1
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	s.configs[record.PathID] = record
	s.writes++
	return record, nil
}

// TestPathDataWorkspaceRequiresConfirmationBeforeRouteWrite 验证换路确认前绝不写入任何路径数据。
func TestPathDataWorkspaceRequiresConfirmationBeforeRouteWrite(t *testing.T) {
	plan := model.Plan{ID: 601, Account: "account-a", FlowSource: "new", TargetObjectID: "flow-a", Status: model.PlanStatusNotStarted}
	tree := workspaceConditionTree()
	paths := &workspacePathRepository{paths: []model.ExecutionPath{
		{ID: 611, PlanID: plan.ID, SequenceNo: 1, Name: "大额路径", Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "large"}}},
		{ID: 612, PlanID: plan.ID, SequenceNo: 2, Name: "普通路径", Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "fallback"}}},
	}}
	store := newWorkspaceHistoryStore()
	store.snapshots[701] = model.HistorySnapshot{
		ID: 701, PlanID: plan.ID, CandidateKey: "candidate-large", FlowCode: "flow-a", FormName: "申请单",
		FlowName: "申请流程", RuntimeType: string(target.FormRenderTypeFormMaking), InstanceStatus: "end",
		InstanceSummary: map[string]any{"instanceTitle": "历史申请", "initiator": "当前用户", "companyName": "测试公司", "createdAt": "2026-09-01", "statusName": "已结束"},
		TemplateSummary: map[string]any{}, RawFormData: map[string]any{"amount": 20, "nested": map[string]any{"memo": "keep"}},
	}
	store.defaults[plan.ID] = repository.HistoryDefaultRecord{PlanID: plan.ID, SnapshotID: 701, Revision: 1}
	store.configs[611] = repository.HistoryPathConfigRecord{PathID: 611, Revision: 4, DataRevision: 4, SourceMode: model.HistorySourceModeDefault, EffectiveFormData: []byte(`{"amount":20}`)}
	store.configs[612] = repository.HistoryPathConfigRecord{PathID: 612, Revision: 2, DataRevision: 2, SourceMode: model.HistorySourceModeDefault, EffectiveFormData: []byte(`{"amount":3}`)}
	reader := workspaceTargetReader{snapshot: target.PathConfigurationSnapshot{
		Tree: tree, EntryNodeIDs: []string{"route"}, FlowCode: "flow-a", FlowName: "申请流程", RenderType: target.FormRenderTypeFormMaking,
		Forms: []target.FormRuntimeTemplate{{Name: "申请单", TemplateData: `{"list":[{"type":"number","model":"amount"}]}`}},
	}}
	configService := service.NewPathConfigService(service.NewPlanService(&historyPlanRepository{plan: plan}), reader,
		analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathConfigAnalyzer(), paths)
	configService.SetHistoryWorkspaceStores(store, store)

	input := model.PathConfigurationDataInput{Revision: 4, Values: map[string]any{"amount": 3, "nested": map[string]any{"memo": "edited"}}, RuntimeValidation: model.HistoryRuntimeValidation{Accepted: true}}
	_, err := configService.SaveData(context.Background(), plan.ID, 611, "4b1a93f0-b3b1-49f8-9be7-3e86c1d8e001", input)
	if err == nil || !service.IsPathConfigErrorKind(err, service.PathConfigErrorRouteConfirmation) {
		t.Fatalf("实际路径变化未返回确认错误：%v", err)
	}
	if store.writes != 0 {
		t.Fatalf("换路确认前发生了写入：writes=%d", store.writes)
	}
	var configErr *service.PathConfigError
	if !errors.As(err, &configErr) || configErr.RouteChange == nil || configErr.ConfirmationToken == "" || !configErr.RouteChange.OverwritesData {
		t.Fatalf("换路覆盖影响或令牌缺失：%+v", configErr)
	}
	if configErr.RouteChange.From.SequenceNo != 1 || configErr.RouteChange.To.SequenceNo != 2 {
		t.Fatalf("换路路径摘要错误：%+v", configErr.RouteChange)
	}

	input.ConfirmationToken = configErr.ConfirmationToken
	result, err := configService.SaveData(context.Background(), plan.ID, 611, "4b1a93f0-b3b1-49f8-9be7-3e86c1d8e001", input)
	if err != nil {
		t.Fatalf("确认换路保存失败：%v", err)
	}
	if store.writes != 1 || !result.RouteChanged || result.Path.SequenceNo != 2 || result.DataStatus != model.HistoryDataStatusReady {
		t.Fatalf("确认换路结果或原子写入次数错误：writes=%d result=%+v", store.writes, result)
	}
	if string(store.configs[611].EffectiveFormData) != `{"amount":20}` {
		t.Fatalf("原路径配置被换路覆盖：%s", store.configs[611].EffectiveFormData)
	}
	if store.configs[612].NodeStatus != "affected" || store.configs[612].ConfigStatus != "affected" {
		t.Fatalf("目标路径人员和动作没有被标记 affected：%+v", store.configs[612])
	}
	if got := result.EffectiveFormData["nested"].(map[string]any)["memo"]; got != "edited" {
		t.Fatalf("目标原始嵌套数据没有透传：%v", got)
	}
}

// TestPathDataWorkspaceRevisionConflictDoesNotPartiallyWrite 验证修订冲突不会留下半份配置或增加写计数。
func TestPathDataWorkspaceRevisionConflictDoesNotPartiallyWrite(t *testing.T) {
	plan := model.Plan{ID: 602, Account: "account-a", FlowSource: "new", TargetObjectID: "flow-b", Status: model.PlanStatusNotStarted}
	path := model.ExecutionPath{ID: 621, PlanID: plan.ID, SequenceNo: 1, Name: "直达路径"}
	store := newWorkspaceHistoryStore()
	store.snapshots[702] = model.HistorySnapshot{ID: 702, PlanID: plan.ID, CandidateKey: "candidate-straight", FlowCode: "flow-b", FormName: "申请单", FlowName: "申请流程", RuntimeType: string(target.FormRenderTypeFormMaking), TemplateSummary: map[string]any{}, RawFormData: map[string]any{"title": "old"}}
	store.defaults[plan.ID] = repository.HistoryDefaultRecord{PlanID: plan.ID, SnapshotID: 702, Revision: 1}
	store.configs[path.ID] = repository.HistoryPathConfigRecord{PathID: path.ID, Revision: 9, DataRevision: 9, SourceMode: model.HistorySourceModeDefault, EffectiveFormData: []byte(`{"title":"old"}`)}
	reader := workspaceTargetReader{snapshot: target.PathConfigurationSnapshot{
		Tree: &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{ID: "end", Type: "end"}}, EntryNodeIDs: []string{"start"},
		FlowCode: "flow-b", FlowName: "申请流程", RenderType: target.FormRenderTypeFormMaking,
		Forms: []target.FormRuntimeTemplate{{Name: "申请单", TemplateData: `{"list":[{"type":"input","model":"title"}]}`}},
	}}
	configService := service.NewPathConfigService(service.NewPlanService(&historyPlanRepository{plan: plan}), reader,
		analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathConfigAnalyzer(), &workspacePathRepository{paths: []model.ExecutionPath{path}})
	configService.SetHistoryWorkspaceStores(store, store)
	_, err := configService.SaveData(context.Background(), plan.ID, path.ID, "f497a5d1-7738-4d7d-94a8-80f4ecf2a002", model.PathConfigurationDataInput{Revision: 8, Values: map[string]any{"title": "new"}, RuntimeValidation: model.HistoryRuntimeValidation{Accepted: true}})
	if err == nil || !service.IsPathConfigErrorKind(err, service.PathConfigErrorRevisionConflict) {
		t.Fatalf("旧修订号没有被拒绝：%v", err)
	}
	if store.writes != 0 || string(store.configs[path.ID].EffectiveFormData) != `{"title":"old"}` {
		t.Fatalf("修订冲突产生了部分写入：writes=%d config=%+v", store.writes, store.configs[path.ID])
	}
}

// TestCustomWorkspacePassesRawValuesWithoutPageMapping 验证 NoFormFlow 不依赖 VuePage 映射也能原样进入数据工作区。
func TestCustomWorkspacePassesRawValuesWithoutPageMapping(t *testing.T) {
	plan := model.Plan{ID: 603, Account: "account-a", FlowSource: "new", TargetObjectID: "flow-custom", Status: model.PlanStatusNotStarted}
	path := model.ExecutionPath{ID: 631, PlanID: plan.ID, SequenceNo: 1, Name: "自定义页路径"}
	store := newWorkspaceHistoryStore()
	raw := map[string]any{"custom": map[string]any{"rows": []any{map[string]any{"code": "A", "amount": float64(7)}}}, "virtual": "keep"}
	store.snapshots[703] = model.HistorySnapshot{ID: 703, PlanID: plan.ID, CandidateKey: "candidate-custom", FlowCode: "flow-custom", FlowName: "NoFormFlow 申请页", RuntimeType: string(target.FormRenderTypeVueCustom), TemplateSummary: map[string]any{"pageKey": "NoFormFlow"}, RawFormData: raw}
	store.defaults[plan.ID] = repository.HistoryDefaultRecord{PlanID: plan.ID, SnapshotID: 703, Revision: 1}
	reader := workspaceTargetReader{snapshot: target.PathConfigurationSnapshot{
		Tree: &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{ID: "end", Type: "end"}}, EntryNodeIDs: []string{"start"},
		FlowCode: "flow-custom", FlowName: "NoFormFlow 申请页", AuditWay: "NoFormFlow", RenderType: target.FormRenderTypeVueCustom,
	}}
	configService := service.NewPathConfigService(service.NewPlanService(&historyPlanRepository{plan: plan}), reader,
		analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathConfigAnalyzer(), &workspacePathRepository{paths: []model.ExecutionPath{path}})
	configService.SetHistoryWorkspaceStores(store, store)
	configuration, err := configService.GetData(context.Background(), plan.ID, path.ID)
	if err != nil {
		t.Fatalf("读取自定义表单数据工作区失败：%v", err)
	}
	if configuration.RuntimeType != string(target.FormRenderTypeVueCustom) || configuration.RuntimePage != nil || !reflect.DeepEqual(configuration.EffectiveFormData, raw) {
		t.Fatalf("NoFormFlow 数据被映射或丢失：%+v", configuration)
	}
	configuration.EffectiveFormData["custom"].(map[string]any)["rows"].([]any)[0].(map[string]any)["code"] = "changed"
	if raw["custom"].(map[string]any)["rows"].([]any)[0].(map[string]any)["code"] != "A" {
		t.Fatal("工作区读取没有深复制历史原始数据")
	}
}

// workspaceConditionTree 创建带自动条件和末分支兜底的最小目标流程树。
func workspaceConditionTree() *target.FlowNodeTemplate {
	return &target.FlowNodeTemplate{ID: "route", Type: "condition", ConditionNodes: []target.FlowBranchTemplate{
		{ID: "large", Sort: 1, Conditions: []target.FlowCondition{{FieldA: "amount", ValueB: "10", Judge: "gt"}}, Child: &target.FlowNodeTemplate{ID: "large-end", Type: "end"}},
		{ID: "fallback", Sort: 2, Child: &target.FlowNodeTemplate{ID: "fallback-end", Type: "end"}},
	}}
}

// workspaceChoicesEqual 比较执行路径的真实路由键并忽略目标返回顺序差异。
func workspaceChoicesEqual(left, right []model.ExecutionPathChoice) bool {
	if len(left) != len(right) {
		return false
	}
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftJSON) == string(rightJSON)
}

// TestWorkspacePermissionsUseInitiatorNodeOnly 锁定配置阶段的表单权限只来自发起节点：
// 审批节点放开的编辑权限不能提前生效，否则发起时不可填的字段会被渲染成可填。
func TestWorkspacePermissionsUseInitiatorNodeOnly(t *testing.T) {
	audit := &target.FlowNodeTemplate{ID: "audit", Type: "common", FieldPowers: []target.FlowNodeFieldPower{
		{EnglishName: "amount", Power: "edit"},
		{EnglishName: "auditRemark", Power: "edit"},
	}}
	start := &target.FlowNodeTemplate{ID: "start", Type: "start", Child: audit, FieldPowers: []target.FlowNodeFieldPower{
		{EnglishName: "amount", Power: "only_read"},
		{EnglishName: "leaveType", Power: "edit"},
	}}
	permissions := service.ProjectFormPermissionsForTest(start, []string{"start", "audit"})
	powers := map[string]string{}
	for _, permission := range permissions {
		powers[permission.Field] = permission.Power
	}
	if powers["amount"] != "only_read" {
		t.Fatalf("审批节点的编辑权限提前生效了：%v", powers)
	}
	if powers["leaveType"] != "edit" {
		t.Fatalf("发起节点声明的可填字段丢失：%v", powers)
	}
	if _, exists := powers["auditRemark"]; exists {
		t.Fatalf("审核节点专用字段不应进入发起态权限：%v", powers)
	}
}
