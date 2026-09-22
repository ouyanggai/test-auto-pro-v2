package action_orchestration_test

import "testing"

import "test-auto-pro-v2/internal/engine/scenario"

import "test-auto-pro-v2/internal/model"

// TestF026ConsecutiveDraftsProduceSingleFixedResubmit 验证连续草稿只在节点末尾生成一次固定重新提交，不提前推进也不创建第二实例。
func TestF026ConsecutiveDraftsProduceSingleFixedResubmit(t *testing.T) {
	result, err := scenario.Compile(scenario.Input{
		Actions: []model.ConfiguredAction{
			{Key: "draft-1", Action: model.ActionSaveDraft, Scope: model.ActionScopeInitiator, NodeKey: "start", Order: 1},
			{Key: "draft-2", Action: model.ActionSaveDraft, Scope: model.ActionScopeInitiator, NodeKey: "start", Order: 2},
			{Key: "submit-intent", Action: model.ActionSubmit, Scope: model.ActionScopeInitiator, NodeKey: "start", Order: 3},
			{Key: "approve-intent", Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: "review", Order: 4},
		},
		Nodes:        []model.FlowGraphNode{{ID: "start", Type: "start"}, {ID: "review", Type: "common"}, {ID: "end", Type: "end"}},
		NodeSequence: []string{"start", "review", "end"},
	})
	if err == nil {
	} else {
		t.Fatalf("连续草稿编译失败：%v", err)
	}
	wantActions := []model.ActionKey{model.ActionSaveDraft, model.ActionSaveDraft, model.ActionResubmit, model.ActionApprove}
	wantSources := []model.ActionStepSource{model.ActionStepSourceUser, model.ActionStepSourceUser, model.ActionStepSourceSystemDefault, model.ActionStepSourceSystemDefault}
	for index := range wantActions {
		if index >= len(result.Steps) {
			t.Fatalf("缺少步骤[%d]，实际=%+v", index, result.Steps)
		}
		step := result.Steps[index]
		if step.Action == wantActions[index] && step.Source == wantSources[index] {
			continue
		}
		t.Fatalf("步骤[%d] 错误：got(%s,%s) want(%s,%s)，全部=%+v", index, step.Action, step.Source, wantActions[index], wantSources[index], result.Steps)
	}
	if result.Steps[2].NodeKey == "start" && result.Steps[2].ReleaseRequired {
	} else {
		t.Fatalf("固定重新提交未作为发起节点独立动作组：%+v", result.Steps[2])
	}
	if findUserStep(result.Steps, model.ActionSubmit) == nil {
	} else {
		t.Fatal("显式提交意图不应再生成第二套用户步骤")
	}
}

// TestF026RejectTwiceBuildsRecoveryChain 验证两次不同意的每一次都带有“重提 + 前序人工节点同意”的完整恢复链。
func TestF026RejectTwiceBuildsRecoveryChain(t *testing.T) {
	result, err := scenario.Compile(scenario.Input{
		Actions: []model.ConfiguredAction{
			{Key: "reject-1", Action: model.ActionReject, Scope: model.ActionScopeTask, NodeKey: "review", Order: 1},
			{Key: "reject-2", Action: model.ActionReject, Scope: model.ActionScopeTask, NodeKey: "review", Order: 2},
		},
		Nodes: []model.FlowGraphNode{
			{ID: "start", Type: "start"}, {ID: "mid", Type: "common"},
			{ID: "review", Type: "common"}, {ID: "end", Type: "end"},
		},
		NodeSequence: []string{"start", "mid", "review", "end"},
	})
	if err == nil {
	} else {
		t.Fatalf("不同意 ×2 编译失败：%v", err)
	}
	want := []struct {
		source model.ActionStepSource
		action model.ActionKey
		node   string
	}{
		{model.ActionStepSourceSystemDefault, model.ActionSubmit, "start"},
		{model.ActionStepSourceSystemDefault, model.ActionApprove, "mid"},
		{model.ActionStepSourceUser, model.ActionReject, "review"},
		{model.ActionStepSourceRecovery, model.ActionResubmit, "start"},
		{model.ActionStepSourceRecovery, model.ActionApprove, "mid"},
		{model.ActionStepSourceUser, model.ActionReject, "review"},
		{model.ActionStepSourceRecovery, model.ActionResubmit, "start"},
		{model.ActionStepSourceRecovery, model.ActionApprove, "mid"},
		{model.ActionStepSourceSystemDefault, model.ActionApprove, "review"},
	}
	for index, item := range want {
		if index >= len(result.Steps) {
			t.Fatalf("缺少步骤[%d]，实际=%+v", index, result.Steps)
		}
		step := result.Steps[index]
		if step.Source == item.source && step.Action == item.action && step.NodeKey == item.node {
			continue
		}
		t.Fatalf("步骤[%d] 错误：got(%s,%s,%s) want(%s,%s,%s)，全部=%+v", index, step.Source, step.Action, step.NodeKey, item.source, item.action, item.node, result.Steps)
	}
	if result.Steps[2].ReleaseGroup == result.Steps[3].ReleaseGroup && result.Steps[3].ReleaseGroup == result.Steps[4].ReleaseGroup {
	} else {
		t.Fatalf("不同意后的恢复链没有归入同一动作组：%+v", result.Steps[2:5])
	}
	if result.Steps[3].ReleaseRequired || result.Steps[4].ReleaseRequired {
		t.Fatalf("恢复链内部步骤不应各自成为放行边界：%+v", result.Steps[3:5])
	}
	if result.Steps[5].ReleaseRequired == false {
		t.Fatal("第二次不同意应作为新动作组等待放行")
	}
}

// TestF026InNodeRepeatsDoNotInsertRecovery 验证暂存、关注等不离开节点的重复动作不会被补入无关恢复链。
func TestF026InNodeRepeatsDoNotInsertRecovery(t *testing.T) {
	result, err := scenario.Compile(scenario.Input{
		Actions: []model.ConfiguredAction{
			{Key: "storage-1", Action: model.ActionStorageFormData, Scope: model.ActionScopeTask, NodeKey: "review", Order: 1},
			{Key: "storage-2", Action: model.ActionStorageFormData, Scope: model.ActionScopeTask, NodeKey: "review", Order: 2},
		},
		Nodes:        []model.FlowGraphNode{{ID: "start", Type: "start"}, {ID: "review", Type: "common"}, {ID: "end", Type: "end"}},
		NodeSequence: []string{"start", "review", "end"},
	})
	if err == nil {
	} else {
		t.Fatalf("暂存重复动作编译失败：%v", err)
	}
	var userStorage, recoveries int
	for _, step := range result.Steps {
		if step.Source == model.ActionStepSourceUser && step.Action == model.ActionStorageFormData {
			userStorage++
		}
		if step.Source == model.ActionStepSourceRecovery {
			recoveries++
		}
	}
	if userStorage != 2 {
		t.Fatalf("暂存重复动作应各生成一条用户步骤，实际=%d，全部=%+v", userStorage, result.Steps)
	}
	if recoveries == 0 {
	} else {
		t.Fatalf("不离开节点的动作不应插入恢复链：%+v", result.Steps)
	}
}

// TestF026AdvisoryIssuesDoNotBlockSave 验证实时目录变化只形成非阻断提醒，不阻止保存。
func TestF026AdvisoryIssuesDoNotBlockSave(t *testing.T) {
	result, err := scenario.Compile(scenario.Input{
		Actions: []model.ConfiguredAction{
			{Key: "storage", Action: model.ActionStorageFormData, Scope: model.ActionScopeTask, NodeKey: "review"},
		},
		Nodes:        []model.FlowGraphNode{{ID: "review", Type: "common"}},
		NodeSequence: []string{"review"},
		Catalog:      []model.ActionCatalogItem{{Action: model.ActionStorageFormData, Scope: model.ActionScopeTask, Enabled: false, DisabledReason: "目标实时待办已变化"}},
	})
	if err == nil {
	} else {
		t.Fatalf("运行时门禁变化不应阻止保存：%v", err)
	}
	if len(result.Issues) == 1 && result.Issues[0].Blocking == false && result.Issues[0].Message == "目标实时待办已变化" {
	} else {
		t.Fatalf("缺少非阻断提醒：%+v", result.Issues)
	}
}

// TestF026StructuralErrorsStillBlock 验证路径、作用域和回退前驱等结构问题仍然阻止保存。
func TestF026StructuralErrorsStillBlock(t *testing.T) {
	cases := []struct {
		name    string
		actions []model.ConfiguredAction
		nodes   []model.FlowGraphNode
		seq     []string
		code    string
	}{
		{
			name:    "节点不属于路径",
			actions: []model.ConfiguredAction{{Key: "storage", Action: model.ActionStorageFormData, Scope: model.ActionScopeTask, NodeKey: "missing"}},
			nodes:   []model.FlowGraphNode{{ID: "review", Type: "common"}},
			seq:     []string{"review"},
			code:    "UNKNOWN_NODE",
		},
		{
			name:    "作用域错误",
			actions: []model.ConfiguredAction{{Key: "storage", Action: model.ActionStorageFormData, Scope: model.ActionScopeInitiator, NodeKey: "start"}},
			nodes:   []model.FlowGraphNode{{ID: "start", Type: "start"}},
			seq:     []string{"start"},
			code:    "ACTION_SCOPE_INVALID",
		},
		{
			name: "没有直接前一待办",
			actions: []model.ConfiguredAction{
				{Key: "rollback", Action: model.ActionRollback, Scope: model.ActionScopeTask, NodeKey: "review"},
			},
			nodes: []model.FlowGraphNode{{ID: "review", Type: "common"}},
			seq:   []string{"review"},
			code:  "ROLLBACK_PREVIOUS_MISSING",
		},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			_, err := scenario.Compile(scenario.Input{Actions: item.actions, Nodes: item.nodes, NodeSequence: item.seq})
			if err == nil {
				t.Fatalf("结构错误未阻止保存：%+v", item)
			}
			compileErr, ok := err.(*scenario.CompileError)
			if ok == false {
				t.Fatalf("错误类型不是 CompileError：%T %v", err, err)
			}
			if len(compileErr.Issues) > 0 && compileErr.Issues[0].Code == item.code {
				return
			}
			t.Fatalf("结构错误码错误：got=%+v want=%s", compileErr.Issues, item.code)
		})
	}
}

// TestProxyRemapOrderBlocksActionsAfterAddSign 锁定 2026-09-21 夜间路径 10 实测缺陷：
// 加签（updateFlowProxy）会重建实例流程代理并为节点生成新标识，同一节点上排在加签之后的动作
// 再按原节点读待办必然为空，运行时报「目标上已无当前处理人在本节点的待办任务」而停止整条路径。
// 该顺序约束必须在配置阶段阻断；把加签排到最后（或换节点）则正常通过。
func TestProxyRemapOrderBlocksActionsAfterAddSign(t *testing.T) {
	nodes := []model.FlowGraphNode{
		{ID: "start", Type: "start"}, {ID: "review", Type: "common"}, {ID: "end", Type: "end"},
	}
	seq := []string{"start", "review", "end"}

	result, err := scenario.Compile(scenario.Input{
		Actions: []model.ConfiguredAction{
			{Key: "sign", Action: model.ActionAddSign, Scope: model.ActionScopeTask, NodeKey: "review", Order: 1},
			{Key: "storage", Action: model.ActionStorageFormData, Scope: model.ActionScopeTask, NodeKey: "review", Order: 2},
		},
		Nodes: nodes, NodeSequence: seq,
	})
	if err == nil {
		t.Fatalf("加签之后仍排布同节点动作必须在配置阶段阻断，实际编译成功：%+v", result.Steps)
	}
	compileErr, ok := err.(*scenario.CompileError)
	if !ok {
		t.Fatalf("错误类型不是 CompileError：%T %v", err, err)
	}
	if len(compileErr.Issues) == 0 || compileErr.Issues[0].Code != "ACTION_AFTER_PROXY_REMAP" {
		t.Fatalf("结构错误码错误：got=%+v want=ACTION_AFTER_PROXY_REMAP", compileErr.Issues)
	}

	// 反向顺序（暂存在前、加签在后）是合法排布，不能被这条规则误伤。
	if _, err := scenario.Compile(scenario.Input{
		Actions: []model.ConfiguredAction{
			{Key: "storage", Action: model.ActionStorageFormData, Scope: model.ActionScopeTask, NodeKey: "review", Order: 1},
			{Key: "sign", Action: model.ActionAddSign, Scope: model.ActionScopeTask, NodeKey: "review", Order: 2},
		},
		Nodes: nodes, NodeSequence: seq,
	}); err != nil {
		t.Fatalf("加签排在最后是合法排布，不应被阻断：%v", err)
	}
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

// TestF026WithdrawTwiceBuildsRecoveryChain 验证两次撤回后都从发起节点重提并逐节点同意返回原节点。
func TestF026WithdrawTwiceBuildsRecoveryChain(t *testing.T) {
	result, err := scenario.Compile(scenario.Input{
		Actions: []model.ConfiguredAction{
			{Key: "withdraw-1", Action: model.ActionWithdraw, Scope: model.ActionScopeInstance, Order: 1},
			{Key: "withdraw-2", Action: model.ActionWithdraw, Scope: model.ActionScopeInstance, Order: 2},
		},
		Nodes:        []model.FlowGraphNode{{ID: "start", Type: "start"}, {ID: "mid", Type: "common"}, {ID: "review", Type: "common"}, {ID: "end", Type: "end"}},
		NodeSequence: []string{"start", "mid", "review", "end"},
	})
	if err == nil {
	} else {
		t.Fatalf("撤回 ×2 编译失败：%v", err)
	}
	want := []struct {
		source model.ActionStepSource
		action model.ActionKey
		node   string
	}{
		{model.ActionStepSourceSystemDefault, model.ActionSubmit, "start"},
		{model.ActionStepSourceSystemDefault, model.ActionApprove, "mid"},
		{model.ActionStepSourceUser, model.ActionWithdraw, ""},
		{model.ActionStepSourceRecovery, model.ActionResubmit, "start"},
		{model.ActionStepSourceRecovery, model.ActionApprove, "mid"},
		{model.ActionStepSourceUser, model.ActionWithdraw, ""},
		{model.ActionStepSourceRecovery, model.ActionResubmit, "start"},
		{model.ActionStepSourceRecovery, model.ActionApprove, "mid"},
		{model.ActionStepSourceSystemDefault, model.ActionApprove, "review"},
	}
	for index, item := range want {
		if index >= len(result.Steps) {
			t.Fatalf("缺少步骤[%d]，实际=%+v", index, result.Steps)
		}
		step := result.Steps[index]
		if step.Source == item.source && step.Action == item.action && step.NodeKey == item.node {
			continue
		}
		t.Fatalf("步骤[%d] 错误：got(%s,%s,%s) want(%s,%s,%s)，全部=%+v", index, step.Source, step.Action, step.NodeKey, item.source, item.action, item.node, result.Steps)
	}
}

// TestF026RetrieveTwiceRepeatsApproveThenRetrieve 验证没有已办时取回前先同意，重复取回重复生成该循环。
func TestF026RetrieveTwiceRepeatsApproveThenRetrieve(t *testing.T) {
	result, err := scenario.Compile(scenario.Input{
		Actions: []model.ConfiguredAction{
			{Key: "retrieve-1", Action: model.ActionRetrieve, Scope: model.ActionScopeCompletedTask, NodeKey: "review", Order: 1},
			{Key: "retrieve-2", Action: model.ActionRetrieve, Scope: model.ActionScopeCompletedTask, NodeKey: "review", Order: 2},
		},
		Nodes:        []model.FlowGraphNode{{ID: "start", Type: "start"}, {ID: "mid", Type: "common"}, {ID: "review", Type: "common"}, {ID: "end", Type: "end"}},
		NodeSequence: []string{"start", "mid", "review", "end"},
	})
	if err == nil {
	} else {
		t.Fatalf("取回 ×2 编译失败：%v", err)
	}
	want := []struct {
		source model.ActionStepSource
		action model.ActionKey
		node   string
	}{
		{model.ActionStepSourceSystemDefault, model.ActionSubmit, "start"},
		{model.ActionStepSourceSystemDefault, model.ActionApprove, "mid"},
		{model.ActionStepSourceRecovery, model.ActionApprove, "review"},
		{model.ActionStepSourceUser, model.ActionRetrieve, "review"},
		{model.ActionStepSourceRecovery, model.ActionApprove, "review"},
		{model.ActionStepSourceUser, model.ActionRetrieve, "review"},
		{model.ActionStepSourceSystemDefault, model.ActionApprove, "review"},
	}
	for index, item := range want {
		if index >= len(result.Steps) {
			t.Fatalf("缺少步骤[%d]，实际=%+v", index, result.Steps)
		}
		step := result.Steps[index]
		if step.Source == item.source && step.Action == item.action && step.NodeKey == item.node {
			continue
		}
		t.Fatalf("步骤[%d] 错误：got(%s,%s,%s) want(%s,%s,%s)，全部=%+v", index, step.Source, step.Action, step.NodeKey, item.source, item.action, item.node, result.Steps)
	}
}

// TestF026RollbackTwiceWalksBackThroughPreviousManualNode 验证每次回退后从真实前驱人工节点重新同意返回原节点。
func TestF026RollbackTwiceWalksBackThroughPreviousManualNode(t *testing.T) {
	result, err := scenario.Compile(scenario.Input{
		Actions: []model.ConfiguredAction{
			{Key: "rollback-1", Action: model.ActionRollback, Scope: model.ActionScopeTask, NodeKey: "review", Order: 1},
			{Key: "rollback-2", Action: model.ActionRollback, Scope: model.ActionScopeTask, NodeKey: "review", Order: 2},
		},
		Nodes:        []model.FlowGraphNode{{ID: "start", Type: "start"}, {ID: "mid", Type: "common"}, {ID: "review", Type: "common"}, {ID: "end", Type: "end"}},
		NodeSequence: []string{"start", "mid", "review", "end"},
	})
	if err == nil {
	} else {
		t.Fatalf("回退 ×2 编译失败：%v", err)
	}
	want := []struct {
		source model.ActionStepSource
		action model.ActionKey
		node   string
	}{
		{model.ActionStepSourceSystemDefault, model.ActionSubmit, "start"},
		{model.ActionStepSourceSystemDefault, model.ActionApprove, "mid"},
		{model.ActionStepSourceUser, model.ActionRollback, "review"},
		{model.ActionStepSourceRecovery, model.ActionApprove, "mid"},
		{model.ActionStepSourceUser, model.ActionRollback, "review"},
		{model.ActionStepSourceRecovery, model.ActionApprove, "mid"},
		{model.ActionStepSourceSystemDefault, model.ActionApprove, "review"},
	}
	for index, item := range want {
		if index >= len(result.Steps) {
			t.Fatalf("缺少步骤[%d]，实际=%+v", index, result.Steps)
		}
		step := result.Steps[index]
		if step.Source == item.source && step.Action == item.action && step.NodeKey == item.node {
			continue
		}
		t.Fatalf("步骤[%d] 错误：got(%s,%s,%s) want(%s,%s,%s)，全部=%+v", index, step.Source, step.Action, step.NodeKey, item.source, item.action, item.node, result.Steps)
	}
}
// TestForwardBeforeSubmitBlocked 验证流程处于草稿且尚未提交进入后续审批运行时，禁止执行转发动作。
func TestForwardBeforeSubmitBlocked(t *testing.T) {
	_, err := scenario.Compile(scenario.Input{
		Actions: []model.ConfiguredAction{
			{Key: "draft-1", Action: model.ActionSaveDraft, Scope: model.ActionScopeInitiator, NodeKey: "start", Order: 1},
			{Key: "forward-1", Action: model.ActionForward, Scope: model.ActionScopeInstance, NodeKey: "", Order: 2},
		},
		Nodes:        []model.FlowGraphNode{{ID: "start", Type: "start"}, {ID: "end", Type: "end"}},
		NodeSequence: []string{"start", "end"},
	})
	if err == nil {
		t.Fatal("流程未发起前执行转发必须被阻断，实际未报错")
	}
	compileErr, ok := err.(*scenario.CompileError)
	if !ok || len(compileErr.Issues) == 0 || compileErr.Issues[0].Code != "ACTION_FORWARD_BEFORE_SUBMIT" {
		t.Fatalf("阻断原因应为 ACTION_FORWARD_BEFORE_SUBMIT，实际报错：%v", err)
	}
}

// TestForwardAfterDraftAndResubmitSucceeds 验证首节点保存草稿后重新提交进入后续审批节点，转发作为实例动作可成功编译。
func TestForwardAfterDraftAndResubmitSucceeds(t *testing.T) {
	result, err := scenario.Compile(scenario.Input{
		Actions: []model.ConfiguredAction{
			{Key: "draft-1", Action: model.ActionSaveDraft, Scope: model.ActionScopeInitiator, NodeKey: "start", Order: 1},
			{Key: "forward-1", Action: model.ActionForward, Scope: model.ActionScopeInstance, NodeKey: "", Order: 2},
		},
		Nodes:        []model.FlowGraphNode{{ID: "start", Type: "start"}, {ID: "review", Type: "common"}, {ID: "end", Type: "end"}},
		NodeSequence: []string{"start", "review", "end"},
	})
	if err != nil {
		t.Fatalf("草稿后重新提交流转到审批节点的转发应编译成功，实际报错：%v", err)
	}
	if len(result.Steps) != 5 {
		t.Fatalf("期望 5 个步骤，实际步骤数=%d：%+v", len(result.Steps), result.Steps)
	}
	if result.Steps[0].Action != model.ActionSaveDraft || result.Steps[1].Action != model.ActionResubmit || result.Steps[2].Action != model.ActionForward || result.Steps[3].Action != model.ActionApprove || result.Steps[4].Action != model.ActionSystemAutomatic {
		t.Fatalf("步骤序列不符合预期：全部步骤=%+v", result.Steps)
	}
}
