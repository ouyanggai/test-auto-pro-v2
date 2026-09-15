package history_replay_test

import (
	"context"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	"test-auto-pro-v2/internal/service"
)

// 计划 30 路径 4222 员工请假单真实样本——vacateType 控件 showLabel=false 且选项 label/value 错位，
// 分支最小补丁只改 vacateType__virtualName（事假→产假），服务端必须按目标渲染口径（显示名=value）
// 唯一匹配并回填绑定值，回放直接落最终 ready，不再依赖浏览器里的 form-runtime 选项协调。

// staticOptionReplayFixture 构造带静态下拉模板的回放服务夹具，快照值为“事假”，补丁目标为“产假”。
func staticOptionReplayFixture(t *testing.T, optionsJSON string, rawFormData map[string]any) (*service.HistoryReplayService, *replayStore) {
	return staticOptionReplayFixtureWithCondition(t, optionsJSON, rawFormData, "vacateType__virtualName", "产假")
}

// staticOptionReplayFixtureWithCondition 允许用例指定分支条件字段与期望值，覆盖不同控件的静态解析场景。
func staticOptionReplayFixtureWithCondition(t *testing.T, optionsJSON string, rawFormData map[string]any, conditionField, conditionValue string) (*service.HistoryReplayService, *replayStore) {
	t.Helper()
	plan := model.Plan{ID: 91, Account: "tester", FlowSource: "flow-source", TargetObjectID: "target-1", Status: model.PlanStatusNotStarted}
	path := model.ExecutionPath{ID: 101, PlanID: plan.ID, ConfigurationRevision: 4, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route-1", BranchID: "branch-a"}}}
	tree := conditionTreeID("route-1", []target.FlowBranchTemplate{
		{ID: "branch-a", Sort: 1, Conditions: []target.FlowCondition{{FieldA: conditionField, ValueB: conditionValue, Judge: "eq"}}},
		{ID: "branch-default", Sort: 2},
	})
	store := &replayStore{
		snapshot:  model.HistorySnapshot{ID: 9, PlanID: plan.ID, RuntimeType: string(target.FormRenderTypeFormMaking), RawFormData: rawFormData},
		defaultAt: repository.HistoryDefaultRecord{PlanID: plan.ID, SnapshotID: 9},
	}
	paths := &replayPathRepo{created: path, current: path}
	paths.current.ConfigurationRevision = 4
	reader := &replayTargetReader{snapshot: target.PathConfigurationSnapshot{
		Tree: tree, RenderType: target.FormRenderTypeFormMaking,
		Forms: []target.FormRuntimeTemplate{{Name: "员工请假单", TemplateData: optionsJSON}},
	}}
	replay := service.NewHistoryReplayService(service.NewPlanService(&replayPlanRepo{plan: plan}), paths, reader, store)
	return replay, store
}

func runStaticOptionReplay(t *testing.T, replay *service.HistoryReplayService, store *replayStore) model.HistoryReplayItem {
	t.Helper()
	job, err := replay.Create(context.Background(), 91, model.HistoryReplayCreateInput{PathIDs: []uint64{101}}, "123e4567-e89b-12d3-a456-426614174800")
	if err != nil {
		t.Fatalf("创建静态选项回放任务失败：%v", err)
	}
	completed := waitReplayCompletion(t, replay, 91, job.ID)
	if completed.Total != 1 {
		t.Fatalf("任务聚合计数不正确：%#v", completed)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	return cloneReplayItem(store.items[0])
}

// TestStaticOptionBindingResolvesMismatchedLabels 锁定错位样本：虚拟名“产假”必须回填绑定值“产假”并直接 ready。
func TestStaticOptionBindingResolvesMismatchedLabels(t *testing.T) {
	template := `{"list":[{"type":"select","model":"vacateType","options":{"showLabel":false,"remote":false,"options":[
		{"label":"设备材料采购","value":"事假"},{"label":"招标代理","value":"病假"},{"label":"全过程项目管理","value":"产假"}]}}]}`
	raw := map[string]any{"vacateType": "事假", "vacateType__virtualName": "事假"}
	replay, store := staticOptionReplayFixture(t, template, raw)
	item := runStaticOptionReplay(t, replay, store)
	if item.Status != model.HistoryReplayItemStatusReady || item.DataStatus != model.HistoryDataStatusReady {
		t.Fatalf("错位样本未直接 ready：%#v issues=%#v", item, item.Issues)
	}
	if item.EffectiveFormData["vacateType"] != "产假" || item.EffectiveFormData["vacateType__virtualName"] != "产假" {
		t.Fatalf("绑定值未按显示名回填：%#v", item.EffectiveFormData)
	}
}

// TestStaticOptionBindingKeepsShowLabelSemantics 锁定 showLabel=true 语义：显示名取 label，绑定值取 value。
func TestStaticOptionBindingKeepsShowLabelSemantics(t *testing.T) {
	template := `{"list":[{"type":"select","model":"companyId","options":{"showLabel":true,"remote":false,"options":[
		{"label":"甲公司","value":"c1"},{"label":"乙公司","value":"c2"}]}}]}`
	raw := map[string]any{"companyId": "c1", "companyId__virtualName": "甲公司"}
	replay, store := staticOptionReplayFixture(t, template, raw)
	item := runStaticOptionReplay(t, replay, store)
	if item.Status != model.HistoryReplayItemStatusNeedsInput {
		t.Fatalf("无条件变化时不应误写：%#v", item)
	}
}

// TestStaticOptionBindingBlocksAmbiguousDisplayName 锁定同名歧义：两个选项同名时必须阻断且保留历史值。
func TestStaticOptionBindingBlocksAmbiguousDisplayName(t *testing.T) {
	template := `{"list":[{"type":"select","model":"companyId","options":{"showLabel":true,"remote":false,"options":[
		{"label":"甲公司","value":"c1"},{"label":"同名公司","value":"c2"},{"label":"同名公司","value":"c3"}]}}]}`
	raw := map[string]any{"companyId": "c1", "companyId__virtualName": "旧公司"}
	replay, store := staticOptionReplayFixtureWithCondition(t, template, raw, "companyId__virtualName", "同名公司")
	item := runStaticOptionReplay(t, replay, store)
	found := false
	for _, issue := range item.Issues {
		if issue.Code == "STATIC_OPTION_AMBIGUOUS" && issue.Blocking {
			found = true
		}
	}
	if !found {
		t.Fatalf("同名歧义未落阻断问题：%#v", item.Issues)
	}
	if item.DataStatus != model.HistoryDataStatusNeedsInput {
		t.Fatalf("歧义时数据不得落 ready：%#v", item)
	}
	if item.EffectiveFormData["companyId"] != "c1" {
		t.Fatalf("歧义时必须保留历史绑定值：%#v", item.EffectiveFormData)
	}
}
