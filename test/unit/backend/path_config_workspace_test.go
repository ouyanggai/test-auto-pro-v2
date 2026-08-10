package backend_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

type workspacePathConfigReader struct {
	*pathConfigReader
	samples      []map[string]any
	session      target.FormRuntimeSession
	sampleCalls  int
	sessionCalls int
}

// RecentFormSamples 返回预设近期样本并记录调用次数。
func (r *workspacePathConfigReader) RecentFormSamples(_ context.Context, _ string, _ int) ([]map[string]any, error) {
	r.sampleCalls++
	return r.samples, nil
}

// FormRuntimeSession 返回预设短期 SID 会话并记录调用次数。
func (r *workspacePathConfigReader) FormRuntimeSession(_ context.Context, _ string) (target.FormRuntimeSession, error) {
	r.sessionCalls++
	return r.session, nil
}

// TestPathConfigWorkspaceGeneratesAndPersistsFormSeparately 验证表单生成、完整 values 保存与逐节点确认互不冒充整条路径完成。
func TestPathConfigWorkspaceGeneratesAndPersistsFormSeparately(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration, Account: "account-a", FlowSource: "new", TargetObjectID: "template-a"}}
	baseReader := &pathConfigReader{snapshot: pathConfigWorkspaceSnapshot()}
	reader := &workspacePathConfigReader{
		pathConfigReader: baseReader,
		samples:          []map[string]any{{"amount": float64(3600), "type": "b", "note": "近期样本"}},
		session:          target.FormRuntimeSession{SID: "runtime-sid", BaseURL: "http://target.invalid/api", AccountName: "测试发起人"},
	}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{
		ID: 32, PlanID: 7, SequenceNo: 1, Name: "路径一",
		Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}},
	}}}
	configs := &memoryPathConfigRepository{}
	serviceUnderTest := newPathConfigService(t, plans, reader, paths, configs)

	initial, err := serviceUnderTest.Get(context.Background(), 7, 32)
	if err != nil || initial.Form.Status != "empty" || initial.Status != "pending" {
		t.Fatalf("首次表单或路径状态错误：configuration=%+v err=%v", initial, err)
	}
	generated, err := serviceUnderTest.GenerateForm(context.Background(), 7, 32, 73, nil, nil)
	if err != nil || generated.Status != "draft" || generated.Values["amount"] != float64(3600) || generated.SampleSummary.Recent == 0 {
		t.Fatalf("智能生成没有使用近期样本：generated=%+v err=%v", generated, err)
	}
	savedForm, err := serviceUnderTest.SaveForm(context.Background(), 7, 32, "123e4567-e89b-12d3-a456-426614174801", model.PathFormSaveInput{
		Revision: 0, Values: generated.Values, Seed: generated.Seed,
		GeneratedFieldPaths: generated.GeneratedFieldPaths, ManualOverridePaths: generated.ManualOverridePaths,
		SampleSummary: generated.SampleSummary, Validated: true,
	})
	if err != nil || savedForm.FormRevision != 1 || savedForm.Status != "pending" {
		t.Fatalf("表单独立保存错误地完成整条路径：saved=%+v err=%v", savedForm, err)
	}
	stored := configs.records[32]
	if stored.FormStatus != "valid" || !stored.FormValidated || stored.SampleSummary.Recent == 0 || !stored.SampleSummary.Saved {
		t.Fatalf("完整表单元数据没有持久化：%+v", stored)
	}

	afterForm, err := serviceUnderTest.Get(context.Background(), 7, 32)
	if err != nil || afterForm.Form.Status != "valid" || afterForm.Progress.Pending == 0 || afterForm.Status != "pending" {
		t.Fatalf("表单保存后节点进度或权威状态错误：configuration=%+v err=%v", afterForm, err)
	}
	for _, group := range afterForm.Groups {
		for _, node := range group.Nodes {
			if len(node.Actions) == 0 && !hasEditableWorkspacePerson(node) {
				continue
			}
			result, saveErr := serviceUnderTest.SaveNode(context.Background(), 7, 32, node.Key, nextWorkspaceSaveKey(configs.saveCalls), model.PathNodeSaveInput{
				Revision: configs.records[32].NodeRevision, Actions: workspaceNodeActions(node),
			})
			if saveErr != nil {
				t.Fatalf("逐节点保存失败：node=%s result=%+v err=%v", node.Name, result, saveErr)
			}
		}
	}
	complete, err := serviceUnderTest.Get(context.Background(), 7, 32)
	if err != nil || complete.Status != "configured" || complete.Progress.Pending != 0 {
		t.Fatalf("表单与必需节点全部完成后没有得到 configured：configuration=%+v err=%v", complete, err)
	}
}

// TestPathConfigWorkspaceFormIdempotencyReconcilesLostResponse 验证表单保存响应丢失后同键重试不再读取目标且返回同一事实。
func TestPathConfigWorkspaceFormIdempotencyReconcilesLostResponse(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration, Account: "account-a", FlowSource: "new", TargetObjectID: "template-a"}}
	reader := &pathConfigReader{snapshot: pathConfigWorkspaceSnapshot()}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}}}
	configs := &memoryPathConfigRepository{}
	serviceUnderTest := newPathConfigService(t, plans, reader, paths, configs)
	key := "123e4567-e89b-12d3-a456-426614174811"
	input := model.PathFormSaveInput{
		Values: map[string]any{"amount": float64(2800), "type": "a", "note": "已保存"},
		Seed:   9, GeneratedFieldPaths: []string{"amount", "type", "note"}, Validated: true,
	}
	first, err := serviceUnderTest.SaveForm(context.Background(), 7, 32, key, input)
	if err != nil || first.FormRevision != 1 || reader.calls != 1 || configs.saveCalls != 1 {
		t.Fatalf("首次表单保存失败：result=%+v calls=%d saves=%d err=%v", first, reader.calls, configs.saveCalls, err)
	}
	reader.err = errors.New("目标随后不可用")
	retried, err := serviceUnderTest.SaveForm(context.Background(), 7, 32, key, input)
	if err != nil || retried.FormRevision != 1 || reader.calls != 1 || configs.saveCalls != 1 {
		t.Fatalf("同键对账没有直接返回原表单事实：result=%+v calls=%d saves=%d err=%v", retried, reader.calls, configs.saveCalls, err)
	}
}

// TestPathConfigWorkspaceReadOnlyUsesInstanceValues 验证已发或待发只投影实例当前值且不提供表单写入。
func TestPathConfigWorkspaceReadOnlyUsesInstanceValues(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 7, Status: model.PlanStatusPendingConfiguration, Account: "account-a", FlowSource: "started", TargetObjectID: "instance-a"}}
	snapshot := pathConfigWorkspaceSnapshot()
	snapshot.InstanceValues = map[string]any{"amount": float64(9100), "type": "b", "note": "实例当前值"}
	reader := &pathConfigReader{snapshot: snapshot}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 32, PlanID: 7, Choices: []model.ExecutionPathChoice{{RouteNodeID: "route", BranchID: "branch-a"}}}}}
	serviceUnderTest := newPathConfigService(t, plans, reader, paths, &memoryPathConfigRepository{})
	configuration, err := serviceUnderTest.Get(context.Background(), 7, 32)
	if err != nil || !configuration.Form.ReadOnly || configuration.Form.Status != "valid" || configuration.Form.Values["note"] != "实例当前值" {
		t.Fatalf("已发实例当前值没有只读投影：configuration=%+v err=%v", configuration, err)
	}
	_, err = serviceUnderTest.SaveForm(context.Background(), 7, 32, "123e4567-e89b-12d3-a456-426614174821", model.PathFormSaveInput{Validated: true})
	if !service.IsPathConfigErrorKind(err, service.PathConfigErrorLocked) {
		t.Fatalf("已发表单写入没有被拒绝：%v", err)
	}
}

// pathConfigWorkspaceSnapshot 构造带完整目标 FormMaking 模板的当前路径快照。
func pathConfigWorkspaceSnapshot() target.PathConfigurationSnapshot {
	return target.PathConfigurationSnapshot{
		Tree: pathConfigTree(), EntryNodeIDs: []string{"start"}, FormFields: pathConfigFields(),
		Forms: []target.FormRuntimeTemplate{{Name: "申请表", TemplateData: `{"list":[{"type":"number","model":"amount","name":"申请金额","options":{"required":true}},{"type":"select","model":"type","name":"类型","options":{"required":true,"options":[{"label":"A","value":"a"},{"label":"B","value":"b"}]}},{"type":"input","model":"note","name":"备注","options":{}}],"config":{}}`}},
	}
}

// hasEditableWorkspacePerson 判断节点是否包含需要本轮保存的可编辑人员。
func hasEditableWorkspacePerson(node model.PathConfigNode) bool {
	for _, person := range node.Persons {
		if person.Editable {
			return true
		}
	}
	return false
}

// workspaceNodeActions 返回节点当前默认动作和人员选择，用于验证逐节点保存边界。
func workspaceNodeActions(node model.PathConfigNode) []model.PathConfigActionValue {
	result := make([]model.PathConfigActionValue, 0, len(node.Actions)+len(node.Persons))
	for _, action := range node.Actions {
		if action.Kind == "agree_disagree" {
			result = append(result, model.PathConfigActionValue{Key: action.Key, Action: action.Default})
		}
	}
	for _, person := range node.Persons {
		if person.Editable && len(person.Selected) > 0 {
			result = append(result, model.PathConfigActionValue{Key: person.Key, Action: `[]`})
		}
	}
	return result
}

// nextWorkspaceSaveKey 按保存次数生成不同且合法的测试幂等键。
func nextWorkspaceSaveKey(index int) string {
	return fmt.Sprintf("123e4567-e89b-12d3-a456-%012d", 426614174830+index)
}
