package action_orchestration_test

import (
	"testing"

	"test-auto-pro-v2/internal/engine/scenario"
	"test-auto-pro-v2/internal/model"
)

// TestScenarioCompilerKeepsIndependentOrderAndAddsRecoverySteps 验证重复动作逐条排序并保留恢复屏障。
func TestScenarioCompilerKeepsIndependentOrderAndAddsRecoverySteps(t *testing.T) {
	result, err := scenario.Compile(scenario.Input{
		Actions: []model.ConfiguredAction{
			{Key: "b", Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: "node-review", Order: 2},
			{Key: "a", Action: model.ActionStorageFormData, Scope: model.ActionScopeTask, NodeKey: "node-review", Order: 1},
			{Key: "c", Action: model.ActionStorageFormData, Scope: model.ActionScopeTask, NodeKey: "node-review", Order: 3},
		},
		Nodes:        []model.FlowGraphNode{{ID: "node-start", Type: "start"}, {ID: "node-review", Type: "common"}, {ID: "node-end", Type: "end"}},
		NodeSequence: []string{"node-start", "node-review", "node-end"},
		FinalNodeKey: "node-end",
	})
	if err != nil {
		t.Fatalf("合法动作编译失败：%v", err)
	}
	if len(result.Actions) != 3 || result.Actions[0].Key != "a" || result.Actions[1].Key != "b" || result.Actions[2].Key != "c" {
		t.Fatalf("动作未按用户顺序稳定排序：%+v", result.Actions)
	}
	for index, step := range result.Steps {
		if step.Source == model.ActionStepSourceUser && step.SourceActionKey == "" {
			t.Fatalf("用户步骤[%d] 缺少稳定来源动作键：%+v", index, step)
		}
	}
	if len(result.Steps) < 5 {
		t.Fatalf("缺少用户、同意后取回、系统导航或最终导航步骤：%+v", result.Steps)
	}
	if result.Steps[0].Source != model.ActionStepSourceUser || result.Steps[0].Action != model.ActionStorageFormData {
		t.Fatalf("首个用户步骤错误：%+v", result.Steps[0])
	}
	if !containsStep(result.Steps, model.ActionStepSourceRecovery, model.ActionRetrieve) {
		t.Fatal("同意后同节点动作未插入取回恢复步骤")
	}
	if !containsStep(result.Steps, model.ActionStepSourceNavigation, model.ActionSystemAutomatic) {
		t.Fatal("系统自动节点未投影为只读导航步骤")
	}
	if result.Steps[len(result.Steps)-1].Action != model.ActionApprove || result.Steps[len(result.Steps)-1].Source != model.ActionStepSourceNavigation {
		t.Fatalf("缺少最终导航同意：%+v", result.Steps[len(result.Steps)-1])
	}
}

// TestScenarioCompilerAddsDraftRecovery 验证草稿后仍有动作时重新提交同一主实例。
func TestScenarioCompilerAddsDraftRecovery(t *testing.T) {
	result, err := scenario.Compile(scenario.Input{
		Actions: []model.ConfiguredAction{
			{Key: "draft", Action: model.ActionSaveDraft, Scope: model.ActionScopeInitiator, NodeKey: "start", Order: 1},
			{Key: "submit", Action: model.ActionSubmit, Scope: model.ActionScopeInitiator, NodeKey: "start", Order: 2},
		},
		Nodes:        []model.FlowGraphNode{{ID: "start", Type: "start"}, {ID: "end", Type: "end"}},
		NodeSequence: []string{"start", "end"},
	})
	if err != nil {
		t.Fatalf("草稿后提交编译失败：%v", err)
	}
	var recovery *model.CompiledActionStep
	for index := range result.Steps {
		step := &result.Steps[index]
		if step.Source == model.ActionStepSourceRecovery && step.Action == model.ActionResubmit {
			recovery = step
			break
		}
	}
	if recovery == nil || recovery.SourceActionKey != "draft" {
		t.Fatalf("草稿恢复步骤缺少稳定来源键：%+v", result.Steps)
	}
}

// TestScenarioCompilerRejectsTargetTemporaryParameter 验证浏览器不能把目标临时身份固化到动作配置。
func TestScenarioCompilerRejectsTargetTemporaryParameter(t *testing.T) {
	for _, parameters := range []map[string]any{{"jobTaskId": "target-123"}, {"auditRecord.id": "target-123"}, {"approverAppendVo.userIds": []any{"user-1"}}} {
		_, err := scenario.Compile(scenario.Input{
			Actions: []model.ConfiguredAction{{Key: "approve", Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: "review", Parameters: parameters}},
			Nodes:   []model.FlowGraphNode{{ID: "review", Type: "common"}}, NodeSequence: []string{"review"},
		})
		if err == nil {
			t.Fatalf("动作参数携带目标临时键却编译成功：%+v", parameters)
		}
		compileErr, ok := err.(*scenario.CompileError)
		if !ok || len(compileErr.Issues) == 0 || compileErr.Issues[0].Code != "ACTION_PARAMETER_TARGET_ID" {
			t.Fatalf("目标临时身份阻断未定位：%T %+v", err, err)
		}
	}
}

// TestScenarioCompilerHonorsCurrentCatalogGate 验证实时目录缺项或禁用项不能被动作保存绕过。
func TestScenarioCompilerHonorsCurrentCatalogGate(t *testing.T) {
	base := scenario.Input{
		Actions: []model.ConfiguredAction{{Key: "approve", Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: "review"}},
		Nodes:   []model.FlowGraphNode{{ID: "review", Type: "common"}}, NodeSequence: []string{"review"},
	}
	base.Catalog = []model.ActionCatalogItem{{Action: model.ActionApprove, Scope: model.ActionScopeTask, Enabled: false, DisabledReason: "当前待办已被其他演员处理"}}
	_, err := scenario.Compile(base)
	if err == nil {
		t.Fatal("实时目录禁用动作却编译成功")
	}
	compileErr, ok := err.(*scenario.CompileError)
	if !ok || len(compileErr.Issues) == 0 || compileErr.Issues[0].Code != "ACTION_DISABLED" || compileErr.Issues[0].Message != "当前待办已被其他演员处理" {
		t.Fatalf("目录禁用原因未透传：%T %+v", err, err)
	}
	base.Catalog = []model.ActionCatalogItem{{Action: model.ActionReject, Scope: model.ActionScopeTask, Enabled: true}}
	_, err = scenario.Compile(base)
	if err == nil {
		t.Fatal("实时目录缺少动作却编译成功")
	}
	compileErr, ok = err.(*scenario.CompileError)
	if !ok || len(compileErr.Issues) == 0 || compileErr.Issues[0].Code != "ACTION_NOT_IN_CATALOG" {
		t.Fatalf("目录缺项未阻断：%T %+v", err, err)
	}
	retrieve := scenario.Input{
		Actions: []model.ConfiguredAction{{Key: "retrieve", Action: model.ActionRetrieve, Scope: model.ActionScopeCompletedTask, NodeKey: "review"}},
		Nodes:   []model.FlowGraphNode{{ID: "review", Type: "common"}}, NodeSequence: []string{"review"},
		Catalog: []model.ActionCatalogItem{{Action: model.ActionRetrieve, Scope: model.ActionScopeCompletedTask, Enabled: false, DisabledReason: "会签或并行节点已有其他演员处理，不支持取回"}},
	}
	_, err = scenario.Compile(retrieve)
	if err == nil {
		t.Fatal("取回目录在会签或并行门禁失败时却编译成功")
	}
	compileErr, ok = err.(*scenario.CompileError)
	if !ok || len(compileErr.Issues) == 0 || compileErr.Issues[0].Code != "ACTION_DISABLED" || compileErr.Issues[0].Message != "会签或并行节点已有其他演员处理，不支持取回" {
		t.Fatalf("取回目录禁用原因未透传：%T %+v", err, err)
	}
}

// TestScenarioCompilerAddsPreparationBeforeRetrieveAndResubmit 验证取回准备、驳回重提和旁支动作语义。
func TestScenarioCompilerAddsPreparationBeforeRetrieveAndResubmit(t *testing.T) {
	result, err := scenario.Compile(scenario.Input{
		Actions: []model.ConfiguredAction{
			{Key: "reject", Action: model.ActionReject, Scope: model.ActionScopeTask, NodeKey: "review", Order: 1},
			{Key: "forward", Action: model.ActionForward, Scope: model.ActionScopeInstance, Order: 2},
			{Key: "retrieve", Action: model.ActionRetrieve, Scope: model.ActionScopeCompletedTask, NodeKey: "review", Order: 3},
		},
		Nodes:        []model.FlowGraphNode{{ID: "start", Type: "start"}, {ID: "review", Type: "common"}, {ID: "end", Type: "end"}},
		NodeSequence: []string{"start", "review", "end"},
		FinalNodeKey: "end",
	})
	if err != nil {
		t.Fatalf("驳回、转发和取回编译失败：%v", err)
	}
	if !containsStep(result.Steps, model.ActionStepSourceRecovery, model.ActionResubmit) {
		t.Fatal("驳回后没有插入重新提交恢复步骤")
	}
	for _, step := range result.Steps {
		if step.Source == model.ActionStepSourceRecovery && step.Action == model.ActionResubmit && step.SourceActionKey != "reject" {
			t.Fatalf("重新提交恢复步骤来源键错误：%+v", step)
		}
	}
	if !containsStep(result.Steps, model.ActionStepSourceRecovery, model.ActionApprove) {
		t.Fatal("取回前没有插入准备同意恢复步骤")
	}
	forward := findUserStep(result.Steps, model.ActionForward)
	if forward == nil || forward.ReloadRequired == false || forward.ExpectedEffect == "" {
		t.Fatalf("转发旁支缺少只读预期或重读屏障：%+v", forward)
	}
	explicitResubmit, err := scenario.Compile(scenario.Input{
		Actions: []model.ConfiguredAction{
			{Key: "reject-explicit", Action: model.ActionReject, Scope: model.ActionScopeTask, NodeKey: "review", Order: 1},
			{Key: "resubmit-explicit", Action: model.ActionResubmit, Scope: model.ActionScopeInitiator, NodeKey: "start", Order: 2},
		},
		Nodes:        []model.FlowGraphNode{{ID: "start", Type: "start"}, {ID: "review", Type: "common"}},
		NodeSequence: []string{"start", "review"},
	})
	if err != nil || findUserStep(explicitResubmit.Steps, model.ActionResubmit) == nil {
		t.Fatalf("驳回后显式重提不应被路径回跳阻断：err=%v result=%+v", err, explicitResubmit)
	}
}

// TestScenarioCompilerRejectsUnrecoverableOrder 定位没有回退支撑的非法回跳和隐式演员切换。
func TestScenarioCompilerRejectsUnrecoverableOrder(t *testing.T) {
	_, err := scenario.Compile(scenario.Input{
		Actions: []model.ConfiguredAction{
			{Key: "first", Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: "second", Order: 1},
			{Key: "back", Action: model.ActionStorageFormData, Scope: model.ActionScopeTask, NodeKey: "first", Order: 2},
		},
		Nodes:        []model.FlowGraphNode{{ID: "first", Type: "common"}, {ID: "second", Type: "common"}},
		NodeSequence: []string{"first", "second"},
	})
	if err == nil {
		t.Fatal("没有恢复步骤支撑的前序节点回跳却编译成功")
	}
	compileErr, ok := err.(*scenario.CompileError)
	if !ok || len(compileErr.Issues) == 0 || compileErr.Issues[0].Index != 1 || compileErr.Issues[0].Message == "" {
		t.Fatalf("非法顺序未定位首条阻断动作：%T %+v", err, err)
	}

	_, err = scenario.Compile(scenario.Input{
		Actions: []model.ConfiguredAction{
			{Key: "transfer", Action: model.ActionTransfer, Scope: model.ActionScopeTask, NodeKey: "review", ActorPolicy: "actor:next", Order: 1},
			{Key: "approve", Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: "review", Order: 2},
		},
		Nodes:        []model.FlowGraphNode{{ID: "review", Type: "common"}},
		NodeSequence: []string{"review"},
	})
	if err == nil {
		t.Fatal("移交后缺少演员策略却编译成功")
	}

	result, err := scenario.Compile(scenario.Input{
		Actions: []model.ConfiguredAction{
			{Key: "rollback", Action: model.ActionRollback, Scope: model.ActionScopeTask, NodeKey: "second", Order: 1},
			{Key: "redo", Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: "first", ActorPolicy: "actor:previous", Order: 2},
		},
		Nodes:        []model.FlowGraphNode{{ID: "first", Type: "common"}, {ID: "second", Type: "common"}},
		NodeSequence: []string{"first", "second"},
	})
	if err != nil || !containsStep(result.Steps, model.ActionStepSourceRecovery, model.ActionApprove) {
		t.Fatalf("回退后重走前驱节点未生成恢复步骤：err=%v result=%+v", err, result)
	}
	result, err = scenario.Compile(scenario.Input{
		Actions: []model.ConfiguredAction{
			{Key: "rollback-auto", Action: model.ActionRollback, Scope: model.ActionScopeTask, NodeKey: "second", Order: 1},
			{Key: "redo-auto", Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: "first", ActorPolicy: "actor:previous", Order: 2},
		},
		Nodes:        []model.FlowGraphNode{{ID: "first", Type: "common"}, {ID: "route", Type: "condition"}, {ID: "second", Type: "common"}},
		NodeSequence: []string{"first", "route", "second"},
	})
	if err != nil {
		t.Fatalf("回退时跳过自动节点后重走前驱失败：%v", err)
	}

	_, err = scenario.Compile(scenario.Input{
		Actions:      []model.ConfiguredAction{{Key: "rollback", Action: model.ActionRollback, Scope: model.ActionScopeTask, NodeKey: "review", Order: 1}},
		Nodes:        []model.FlowGraphNode{{ID: "start", Type: "start"}, {ID: "review", Type: "common"}},
		NodeSequence: []string{"start", "review"},
	})
	if err == nil {
		t.Fatal("直接前一节点是发起节点却允许回退")
	}
	compileErr, ok = err.(*scenario.CompileError)
	if !ok || len(compileErr.Issues) == 0 || compileErr.Issues[0].Code != "ROLLBACK_PREVIOUS_START" {
		t.Fatalf("发起节点回退未定位阻断：%T %+v", err, err)
	}

	_, err = scenario.Compile(scenario.Input{
		Actions: []model.ConfiguredAction{{Key: "rollback-first", Action: model.ActionRollback, Scope: model.ActionScopeTask, NodeKey: "review", Order: 1}},
		Nodes:   []model.FlowGraphNode{{ID: "review", Type: "common"}}, NodeSequence: []string{"review"},
	})
	if err == nil {
		t.Fatal("路径首节点没有前驱却允许回退")
	}
	compileErr, ok = err.(*scenario.CompileError)
	if !ok || len(compileErr.Issues) == 0 || compileErr.Issues[0].Code != "ROLLBACK_PREVIOUS_MISSING" {
		t.Fatalf("首节点回退未定位阻断：%T %+v", err, err)
	}
}

// containsStep 判断只读步骤集合中是否出现来源和动作的组合。
func containsStep(steps []model.CompiledActionStep, source model.ActionStepSource, action model.ActionKey) bool {
	for _, step := range steps {
		if step.Source == source && step.Action == action {
			return true
		}
	}
	return false
}

// findUserStep 返回指定用户动作的编译步骤。
func findUserStep(steps []model.CompiledActionStep, action model.ActionKey) *model.CompiledActionStep {
	for index := range steps {
		if steps[index].Source == model.ActionStepSourceUser && steps[index].Action == action {
			return &steps[index]
		}
	}
	return nil
}
