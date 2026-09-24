package executor_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/engine/verdict"
	"test-auto-pro-v2/internal/model"
)

// taskActionFactsTarget 为回退和取回准备最小的目标事实；它故意不实现任务列表读取，
// 以锁定执行器不会再发送目标不支持的空 taskStatus 查询。
type taskActionFactsTarget struct {
	*fakeTarget
	snapshot     target.TaskSnapshot
	records      []target.AuditRecordSnapshot
	tree         *target.FlowNodeTemplate
	auditReads   int
	treeReads    int
	snapshotRead int
}

// FindTaskSnapshot 返回当前步骤现场的唯一任务身份。
func (t *taskActionFactsTarget) FindTaskSnapshot(context.Context, target.Session, string, string, string) (target.TaskSnapshot, error) {
	t.snapshotRead++
	return t.snapshot, nil
}

// ListTaskSnapshotsForUser 让事实发现路径复用本假件的任务快照与读取计数。
func (t *taskActionFactsTarget) ListTaskSnapshotsForUser(ctx context.Context, session target.Session, instanceID, status, queryUserID string) ([]target.TaskSnapshot, error) {
	if strings.TrimSpace(status) == "done" {
		t.snapshotRead++
		if t.snapshot.FlowNodeProxyID == "" || strings.TrimSpace(queryUserID) == t.snapshot.ExecutorID {
			return []target.TaskSnapshot{t.snapshot}, nil
		}
		return nil, nil
	}
	if strings.TrimSpace(status) != "pending" || strings.TrimSpace(queryUserID) != t.currentHandlerUserID() {
		return nil, nil
	}
	snapshot, err := t.FindTaskSnapshot(ctx, session, instanceID, "", status)
	if err != nil || strings.TrimSpace(snapshot.JobTaskID) == "" {
		return nil, err
	}
	return []target.TaskSnapshot{snapshot}, nil
}

// ListAuditRecords 返回前一任务或取回动作的审核记录事实。
func (t *taskActionFactsTarget) ListAuditRecords(context.Context, target.Session, string) ([]target.AuditRecordSnapshot, error) {
	t.auditReads++
	return append([]target.AuditRecordSnapshot(nil), t.records...), nil
}

// ReadProxyTree 返回实例当前流程代理树，供节点类型核验。
func (t *taskActionFactsTarget) ReadProxyTree(context.Context, target.Session, string) (*target.FlowNodeTemplate, error) {
	t.treeReads++
	return t.tree, nil
}

// retrieveExecutionTarget 模拟目标取回后把原已办任务改为新建 pending 任务的真实协议结果。
type retrieveExecutionTarget struct {
	*taskActionFactsTarget
	before       target.TaskSnapshot
	after        target.TaskSnapshot
	written      bool
	writeCalls   int
	writeAccount string
}

// rollbackExecutionTarget 模拟目标回退后写入回退审核记录，并切换到并行策略节点。
type rollbackExecutionTarget struct {
	*taskActionFactsTarget
	written    bool
	writeCalls int
}

// ListTaskSnapshotsForUser 与回退后的待办消失规则保持一致（复用本假件的 FindTaskSnapshot）。
func (t *rollbackExecutionTarget) ListTaskSnapshotsForUser(ctx context.Context, session target.Session, instanceID, status, queryUserID string) ([]target.TaskSnapshot, error) {
	if strings.TrimSpace(status) != "pending" || strings.TrimSpace(queryUserID) != t.currentHandlerUserID() {
		return nil, nil
	}
	snapshot, err := t.FindTaskSnapshot(ctx, session, instanceID, "", status)
	if err != nil || strings.TrimSpace(snapshot.JobTaskID) == "" {
		return nil, err
	}
	return []target.TaskSnapshot{snapshot}, nil
}

// ListAuditRecords 在回退后返回与原任务关联的目标回退审核记录。
func (t *rollbackExecutionTarget) ListAuditRecords(context.Context, target.Session, string) ([]target.AuditRecordSnapshot, error) {
	t.auditReads++
	records := append([]target.AuditRecordSnapshot(nil), t.records...)
	if t.written {
		records = append(records, target.AuditRecordSnapshot{
			FlowJobTaskID: t.snapshot.LinkID,
			AuditStatus:   "roll_back_the_previous_level",
		})
	}
	return records, nil
}

// FindTaskSnapshot 在回退后让原节点待办消失，避免测试依赖当前演员能否看到前一节点的新待办。
func (t *rollbackExecutionTarget) FindTaskSnapshot(context.Context, target.Session, string, string, string) (target.TaskSnapshot, error) {
	t.snapshotRead++
	if t.written {
		return target.TaskSnapshot{}, nil
	}
	return t.snapshot, nil
}

// ExecuteActionWrite 模拟目标接受回退并将主实例切换到并行策略节点。
func (t *rollbackExecutionTarget) ExecuteActionWrite(context.Context, target.Session, target.ActionWriteRequest) (target.WriteResponse, string, error) {
	t.writeCalls++
	t.written = true
	t.fakeTarget.instance = fakeTargetView{
		Found: true, Status: "run", CurrentNodes: []string{"node-parallel-strategy"}, DueNodes: []string{"node-previous"},
	}
	return target.WriteResponse{StatusCode: 200, IsSuccess: true, IsSuccessPresent: true}, "trace-rollback", nil
}

// addSignExecutionTarget 模拟 updateFlowProxy 成功后目标代理树写入新人员的真实效果。
type addSignExecutionTarget struct {
	*taskActionFactsTarget
	beforeDocument json.RawMessage
	afterDocument  json.RawMessage
	afterView      *fakeTargetView
	written        bool
	writeCalls     int
	request        target.ActionWriteRequest
}

// ReadFlowProxyDocument 在写前返回原代理，写后返回包含加签人员的目标当前代理。
func (t *addSignExecutionTarget) ReadFlowProxyDocument(context.Context, target.Session, string) (json.RawMessage, error) {
	if t.written {
		return append(json.RawMessage(nil), t.afterDocument...), nil
	}
	return append(json.RawMessage(nil), t.beforeDocument...), nil
}

// ExecuteActionWrite 模拟目标接受 updateFlowProxy 并返回重建后的代理和当前节点标识。
func (t *addSignExecutionTarget) ExecuteActionWrite(_ context.Context, _ target.Session, request target.ActionWriteRequest) (target.WriteResponse, string, error) {
	t.writeCalls++
	t.request = request
	t.written = true
	if t.afterView != nil {
		t.fakeTarget.instance = *t.afterView
	}
	return target.WriteResponse{
		StatusCode: 200, IsSuccess: true, IsSuccessPresent: true,
		Data: json.RawMessage(`{"flowProxyId":"proxy-private","currentNodeProxyId":"node-current"}`),
	}, "trace-add-sign", nil
}

// FindTaskSnapshot 在取回前返回当前账号的已办任务，取回后返回同节点的新待办任务。
func (t *retrieveExecutionTarget) FindTaskSnapshot(_ context.Context, _ target.Session, _ string, _ string, status string) (target.TaskSnapshot, error) {
	t.snapshotRead++
	switch status {
	case "done":
		if !t.written {
			return t.before, nil
		}
	case "pending":
		if t.written {
			return t.after, nil
		}
	}
	return target.TaskSnapshot{}, nil
}

// ExecuteActionWrite 模拟目标 retrieveProcess 成功，并让后续事实读取可见新的待办任务。
func (t *retrieveExecutionTarget) ExecuteActionWrite(_ context.Context, session target.Session, _ target.ActionWriteRequest) (target.WriteResponse, string, error) {
	t.writeCalls++
	t.writeAccount = session.Summary.Account
	t.written = true
	return target.WriteResponse{StatusCode: 200, IsSuccess: true, IsSuccessPresent: true}, "trace-retrieve", nil
}

// TestF019RollbackUsesAuditRecordForPreviousTask 验证回退从 pid 对应的审核记录定位前一节点，
// 不再尝试用空 taskStatus 查询整个任务链。
func TestF019RollbackUsesAuditRecordForPreviousTask(t *testing.T) {
	targetFake := &taskActionFactsTarget{
		fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-current"}, DueNodes: []string{"node-current"}}},
		snapshot: target.TaskSnapshot{
			LinkID: "current-link", ParentLinkID: "previous-link", JobTaskID: "current-task",
			FlowNodeProxyID: "node-current", FlowProxyID: "proxy-current", TaskStatus: "pending",
		},
		records: []target.AuditRecordSnapshot{{FlowJobTaskID: "previous-link", FlowNodeProxyID: "node-previous"}},
		tree: &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{
			ID: "node-previous", Type: "common", Child: &target.FlowNodeTemplate{ID: "node-current", Type: "common"},
		}},
	}
	runCtx := newRunContext([]model.CompiledActionStep{{
		Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionRollback,
		Scope: model.ActionScopeTask, NodeKey: "node-audit",
	}})
	runCtx.Source = "pending"
	runCtx.PathRun.MainInstanceRef = "instance-rollback"
	runCtx.Nodes["node-audit"] = step.NodeInfo{Name: "当前审批", Type: "common", TargetNodeID: "node-current"}
	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil || !preview.GateAllowed {
		t.Fatalf("回退前置事实应通过：finished=%v err=%v preview=%+v", finished, err, preview)
	}
	if targetFake.snapshotRead != 1 || targetFake.auditReads != 1 || targetFake.treeReads != 1 {
		t.Fatalf("回退应只读取任务快照、审核记录和代理树：snapshot=%d audit=%d tree=%d", targetFake.snapshotRead, targetFake.auditReads, targetFake.treeReads)
	}
}

// TestF019RollbackVerifiesRollbackAuditRecord 验证回退成功由与原待办关联的回退审核记录确认。
func TestF019RollbackVerifiesRollbackAuditRecord(t *testing.T) {
	targetFake := &rollbackExecutionTarget{taskActionFactsTarget: &taskActionFactsTarget{
		fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-current"}, DueNodes: []string{"node-current"}}},
		snapshot: target.TaskSnapshot{
			LinkID: "current-link", ParentLinkID: "previous-link", JobTaskID: "current-task",
			FlowNodeProxyID: "node-current", FlowProxyID: "proxy-current", TaskStatus: "pending",
		},
		records: []target.AuditRecordSnapshot{{FlowJobTaskID: "previous-link", FlowNodeProxyID: "node-previous"}},
		tree: &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{
			ID: "node-previous", Type: "common", Child: &target.FlowNodeTemplate{ID: "node-current", Type: "common"},
		}},
	}}
	runCtx := newRunContext([]model.CompiledActionStep{{
		Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionRollback,
		Scope: model.ActionScopeTask, NodeKey: "node-audit",
	}})
	runCtx.Source = "pending"
	runCtx.PathRun.MainInstanceRef = "instance-rollback"
	runCtx.Nodes["node-audit"] = step.NodeInfo{Name: "当前审批", Type: "common", TargetNodeID: "node-current"}
	state := &fakeRunState{}
	facts := &fakeFacts{}
	executor := step.NewExecutor(targetFake, &fakeSessions{}, state, facts, fixedRunConfig(), nil)

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil || !preview.GateAllowed {
		t.Fatalf("回退前置事实应通过：finished=%v err=%v preview=%+v", finished, err, preview)
	}
	outcome, _, err := executor.RunApprovedStep(context.Background(), step.ApprovedStep{RunCtx: runCtx, Preview: preview, NextIndex: 0})
	if err != nil {
		t.Fatalf("回退执行失败：%v", err)
	}
	if outcome.Verdict != string(verdict.OutcomeSucceeded) {
		t.Fatalf("回退应由回退审核记录确认成功，实际 verdict=%s", outcome.Verdict)
	}
	if targetFake.writeCalls != 1 {
		t.Fatalf("回退应只发送一次原子写请求，实际 %d 次", targetFake.writeCalls)
	}
	if len(facts.attempts) != 1 || facts.attempts[0].Reread != string(verdict.RereadAdvanced) {
		t.Fatalf("回退应记录回退审核记录核验，attempts=%+v", facts.attempts)
	}
}

// TestF019RetrieveLeavesUnknownSuccessorToTarget 验证取回在没有全局后继读取能力时仍能发出正确原子请求，
// 由目标接口在实例锁内判断后继是否已经处理，而不是客户端伪造“后继已处理”。
func TestF019RetrieveLeavesUnknownSuccessorToTarget(t *testing.T) {
	targetFake := &taskActionFactsTarget{
		fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-next"}, DueNodes: []string{"node-next"}}},
		snapshot: target.TaskSnapshot{
			LinkID: "done-link", JobTaskID: "done-task", FlowNodeProxyID: "node-done",
			FlowProxyID: "proxy-current", TaskStatus: "done", ExecutorID: "user-handler",
		},
		records: []target.AuditRecordSnapshot{{
			FlowJobTaskID: "done-link", FlowNodeProxyID: "node-done",
			ExecutorID: "user-handler", AuditStatus: "pass",
		}},
		tree: &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{
			ID: "node-done", Type: "common", Child: &target.FlowNodeTemplate{ID: "node-next", Type: "common"},
		}},
	}
	runCtx := newRunContext([]model.CompiledActionStep{{
		Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionRetrieve,
		Scope: model.ActionScopeCompletedTask, NodeKey: "node-audit",
	}})
	runCtx.Source = "done"
	runCtx.PathRun.MainInstanceRef = "instance-retrieve"
	runCtx.Nodes["node-audit"] = step.NodeInfo{Name: "已办审批", Type: "common", TargetNodeID: "node-done"}
	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil || !preview.GateAllowed {
		t.Fatalf("取回应把未知后继交由目标核验：finished=%v err=%v preview=%+v", finished, err, preview)
	}
	if preview.Endpoint != target.WriteEndpointRetrieve {
		t.Fatalf("取回原子端点错误：%s", preview.Endpoint)
	}
	// 取回先按审核记录解析本节点的真实办结人作为已办读取视角，再用审核记录补齐已办事实，
	// 因此审核记录被读两次（视角解析一次、事实补齐一次）；这是 2026-09-21 路径 5 缺陷修复后的既定顺序。
	if targetFake.snapshotRead != 1 || targetFake.auditReads != 2 || targetFake.treeReads != 1 {
		t.Fatalf("取回应读取已办任务、审核记录（视角解析+事实补齐各一次）和代理树：snapshot=%d audit=%d tree=%d", targetFake.snapshotRead, targetFake.auditReads, targetFake.treeReads)
	}
}

// TestF019RetrieveVerifiesNewPendingTask 验证取回成功后，同节点的新 pending 任务也会被识别为真实前进。
// 目标以新 jobTaskId 标识新批次，不能只比较节点位置，否则执行器会把成功写误判成未变化并停止循环。
func TestF019RetrieveVerifiesNewPendingTask(t *testing.T) {
	targetFake := &retrieveExecutionTarget{
		taskActionFactsTarget: &taskActionFactsTarget{
			fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-done"}, DueNodes: []string{"node-done"}}},
			tree: &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{
				ID: "node-done", Type: "common", Child: &target.FlowNodeTemplate{ID: "node-next", Type: "common"},
			}},
			records: []target.AuditRecordSnapshot{{FlowJobTaskID: "done-link", FlowInstanceID: "instance-retrieve", FlowNodeProxyID: "node-done", ExecutorID: "user-handler", AuditStatus: "pass"}},
		},
		before: target.TaskSnapshot{
			LinkID: "done-link", JobTaskID: "done-task", FlowNodeProxyID: "node-done",
			FlowProxyID: "proxy-current", TaskStatus: "done",
		},
		after: target.TaskSnapshot{
			LinkID: "pending-link", JobTaskID: "retrieved-task", FlowNodeProxyID: "node-done",
			FlowProxyID: "proxy-current", TaskStatus: "pending",
		},
	}
	targetFake.fakeTarget.handlerAccount = "handler-account"
	runCtx := newRunContext([]model.CompiledActionStep{{
		Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionRetrieve,
		Scope: model.ActionScopeCompletedTask, NodeKey: "node-audit",
	}})
	runCtx.Source = "done"
	runCtx.PathRun.MainInstanceRef = "instance-retrieve"
	runCtx.Nodes["node-audit"] = step.NodeInfo{Name: "已办审批", Type: "common", TargetNodeID: "node-done"}
	state := &fakeRunState{}
	facts := &fakeFacts{}
	executor := step.NewExecutor(targetFake, &fakeSessions{}, state, facts, fixedRunConfig(), nil)

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil || !preview.GateAllowed {
		t.Fatalf("取回前置事实应通过：finished=%v err=%v preview=%+v", finished, err, preview)
	}
	outcome, _, err := executor.RunApprovedStep(context.Background(), step.ApprovedStep{RunCtx: runCtx, Preview: preview, NextIndex: 0})
	if err != nil {
		t.Fatalf("取回执行失败：%v", err)
	}
	if outcome.Verdict != string(verdict.OutcomeSucceeded) {
		t.Fatalf("取回应由新的 pending 任务确认成功，实际 verdict=%s", outcome.Verdict)
	}
	if targetFake.writeCalls != 1 {
		t.Fatalf("取回应只发送一次原子写请求，实际 %d 次", targetFake.writeCalls)
	}
	if targetFake.writeAccount != "handler-account" {
		t.Fatalf("取回写请求应使用实际审批人账号会话，实际 %q", targetFake.writeAccount)
	}
	if len(facts.attempts) != 1 || facts.attempts[0].Reread != string(verdict.RereadAdvanced) {
		t.Fatalf("取回应记录已前进的事实核验，attempts=%+v", facts.attempts)
	}
}

// TestF019AddSignVerifiesUpdatedProxyDocument 验证加签不只依赖响应字段，
// 而是读回目标完整代理树确认本次请求的人员已经存在。
func TestF019AddSignVerifiesUpdatedProxyDocument(t *testing.T) {
	beforeDocument := json.RawMessage(`{"id":"proxy-private","flowNodeTemplate":{"id":"start","flowNodeAuditConfig":{"flowNodeDetailConfigList":[]},"childFlowNodeTemplate":{"id":"node-current","flowNodeAuditConfig":{"flowNodeDetailConfigList":[]}}}}`)
	afterDocument := json.RawMessage(`{"id":"proxy-private","flowNodeTemplate":{"id":"start","flowNodeAuditConfig":{"flowNodeDetailConfigList":[]},"childFlowNodeTemplate":{"id":"node-current","flowNodeAuditConfig":{"flowNodeDetailConfigList":[{"bizId":"user-add","id":"user-add","auditDetailType":"personnel"}]}}}}`)
	targetFake := &addSignExecutionTarget{
		taskActionFactsTarget: &taskActionFactsTarget{
			fakeTarget: &fakeTarget{
				instance:    fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-current"}, DueNodes: []string{"node-current"}},
				flowProxyID: "proxy-private",
			},
			snapshot: target.TaskSnapshot{JobTaskID: "task-current", FlowNodeProxyID: "node-current", FlowProxyID: "proxy-private", TaskStatus: "pending"},
			tree: &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{
				ID: "node-current", Type: "common",
			}},
		},
		beforeDocument: beforeDocument,
		afterDocument:  afterDocument,
	}
	runCtx := newRunContext([]model.CompiledActionStep{{
		Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionAddSign,
		Scope: model.ActionScopeTask, NodeKey: "node-audit",
	}})
	runCtx.Source = "pending"
	runCtx.PathRun.MainInstanceRef = "instance-add-sign"
	runCtx.FlowProxyID = "proxy-private"
	runCtx.Nodes["node-audit"] = step.NodeInfo{Name: "当前审批", Type: "common", TargetNodeID: "node-current"}
	runCtx.ActionPersonIDs = map[string][]string{
		step.ActionPersonIndex("node-audit", model.ActionAddSign): {"user-add"},
	}
	state := &fakeRunState{}
	facts := &fakeFacts{}
	executor := step.NewExecutor(targetFake, &fakeSessions{}, state, facts, fixedRunConfig(), nil)

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil || !preview.GateAllowed {
		t.Fatalf("加签前置事实应通过：finished=%v err=%v preview=%+v", finished, err, preview)
	}
	outcome, _, err := executor.RunApprovedStep(context.Background(), step.ApprovedStep{RunCtx: runCtx, Preview: preview, NextIndex: 0})
	if err != nil {
		t.Fatalf("加签执行失败：%v", err)
	}
	if outcome.Verdict != string(verdict.OutcomeSucceeded) || targetFake.writeCalls != 1 {
		t.Fatalf("加签应在写后代理树确认成功：outcome=%+v writes=%d", outcome, targetFake.writeCalls)
	}
	if verified, verifyErr := target.HasAddSignNode(targetFake.request.FlowProxyTree, []string{"user-add"}); verifyErr != nil || !verified {
		t.Fatalf("发送的加签载荷没有插入包含目标人员的审批节点：verified=%v err=%v", verified, verifyErr)
	}
	if len(facts.attempts) != 1 || facts.attempts[0].Reread != string(verdict.RereadAdvanced) {
		t.Fatalf("加签应记录代理树已更新的核验结论，attempts=%+v", facts.attempts)
	}
}

// TestF037AddSignProxyRebuildDoesNotTriggerPathDeviation 验证加签写后代理节点重建时，
// 控制层尚未完成整树重绑定前不会拿旧节点 ID 误报路径偏离。
func TestF037AddSignProxyRebuildDoesNotTriggerPathDeviation(t *testing.T) {
	beforeDocument := json.RawMessage(`{"id":"proxy-private","flowNodeTemplate":{"id":"start","flowNodeAuditConfig":{"flowNodeDetailConfigList":[]},"childFlowNodeTemplate":{"id":"node-current","flowNodeAuditConfig":{"flowNodeDetailConfigList":[{"bizId":"user-add","id":"user-add","auditDetailType":"personnel"}]}}}}`)
	afterDocument := beforeDocument
	targetFake := &addSignExecutionTarget{
		taskActionFactsTarget: &taskActionFactsTarget{
			fakeTarget: &fakeTarget{
				instance:    fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-current"}, DueNodes: []string{"node-current"}},
				flowProxyID: "proxy-private",
			},
			snapshot: target.TaskSnapshot{JobTaskID: "task-current", FlowNodeProxyID: "node-current", FlowProxyID: "proxy-private", TaskStatus: "pending"},
			tree:     &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{ID: "node-current", Type: "common"}},
		},
		beforeDocument: beforeDocument,
		afterDocument:  afterDocument,
		afterView:      &fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-rebuilt"}, DueNodes: []string{"node-rebuilt"}},
	}
	runCtx := newRunContext([]model.CompiledActionStep{
		{Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionAddSign, Scope: model.ActionScopeTask, NodeKey: "node-audit"},
		{Sequence: 2, Source: model.ActionStepSourceUser, Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: "node-next"},
	})
	runCtx.Source = "pending"
	runCtx.PathRun.MainInstanceRef = "instance-add-sign"
	runCtx.FlowProxyID = "proxy-private"
	runCtx.Nodes["node-audit"] = step.NodeInfo{Name: "当前审批", Type: "common", TargetNodeID: "node-current"}
	runCtx.Nodes["node-next"] = step.NodeInfo{Name: "后续审批", Type: "common", TargetNodeID: "node-next-old"}
	runCtx.ActionPersonIDs = map[string][]string{
		step.ActionPersonIndex("node-audit", model.ActionAddSign): {"user-add"},
	}
	facts := &fakeFacts{}
	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, facts, fixedRunConfig(), nil)

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil || !preview.GateAllowed {
		t.Fatalf("加签前置事实应通过：finished=%v err=%v preview=%+v", finished, err, preview)
	}
	outcome, _, err := executor.RunApprovedStep(context.Background(), step.ApprovedStep{RunCtx: runCtx, Preview: preview, NextIndex: 0})
	if err != nil {
		t.Fatalf("加签执行失败：%v", err)
	}
	if outcome.Verdict != string(verdict.OutcomeSucceeded) {
		t.Fatalf("加签应确定成功，实际 outcome=%+v", outcome)
	}
	if outcome.DeviationDetected {
		t.Fatalf("加签后代理节点重建不应在旧 ID 比较阶段误报路径偏离：%+v", outcome)
	}
}

// transferAuditFallbackTarget 模拟移交成功但原处理人 pending 待办快照晚一拍仍返回同一任务的现场：
// 只能凭目标同事务写入的 transfer 审核记录（挂在原任务关联行）确认移交已生效。
type transferAuditFallbackTarget struct {
	*taskActionFactsTarget
	written    bool
	writeCalls int
}

// ListTaskSnapshotsForUser 让移交后的 pending 读取始终返回同一条陈旧待办，锁定「快照滞后」现场。
func (t *transferAuditFallbackTarget) ListTaskSnapshotsForUser(_ context.Context, _ target.Session, _, status, _ string) ([]target.TaskSnapshot, error) {
	if strings.TrimSpace(status) != "pending" {
		return nil, nil
	}
	return []target.TaskSnapshot{t.snapshot}, nil
}

// ListAuditRecords 移交后追加一条挂在原任务关联行的 transfer 审核记录。
func (t *transferAuditFallbackTarget) ListAuditRecords(context.Context, target.Session, string) ([]target.AuditRecordSnapshot, error) {
	t.auditReads++
	records := append([]target.AuditRecordSnapshot(nil), t.records...)
	if t.written {
		records = append(records, target.AuditRecordSnapshot{
			FlowJobTaskID: t.snapshot.LinkID, FlowNodeProxyID: t.snapshot.FlowNodeProxyID, AuditStatus: "transfer",
		})
	}
	return records, nil
}

// ExecuteActionWrite 模拟目标接受移交，但当前处理人待办快照尚未刷新。
func (t *transferAuditFallbackTarget) ExecuteActionWrite(context.Context, target.Session, target.ActionWriteRequest) (target.WriteResponse, string, error) {
	t.writeCalls++
	t.written = true
	return target.WriteResponse{StatusCode: 200, IsSuccess: true, IsSuccessPresent: true}, "trace-transfer", nil
}

// retrieveAuditFallbackTarget 模拟取回成功但新建 pending 待办快照晚一拍尚不可见的现场：
// 只能凭目标同事务写入的 retrieve 审核记录（挂在原办结关联行）确认取回已生效。
type retrieveAuditFallbackTarget struct {
	*taskActionFactsTarget
	before     target.TaskSnapshot
	written    bool
	writeCalls int
}

// FindTaskSnapshot 取回前返回本节点已办任务；取回后新建待办尚未可见，返回空。
func (t *retrieveAuditFallbackTarget) FindTaskSnapshot(_ context.Context, _ target.Session, _ string, _ string, status string) (target.TaskSnapshot, error) {
	t.snapshotRead++
	if status == "done" && !t.written {
		return t.before, nil
	}
	return target.TaskSnapshot{}, nil
}

// ListAuditRecords 取回后追加一条挂在原办结关联行的 retrieve 审核记录。
func (t *retrieveAuditFallbackTarget) ListAuditRecords(context.Context, target.Session, string) ([]target.AuditRecordSnapshot, error) {
	t.auditReads++
	records := append([]target.AuditRecordSnapshot(nil), t.records...)
	if t.written {
		records = append(records, target.AuditRecordSnapshot{
			FlowJobTaskID: t.before.LinkID, FlowNodeProxyID: t.before.FlowNodeProxyID, AuditStatus: "retrieve",
		})
	}
	return records, nil
}

// ExecuteActionWrite 模拟目标 retrieveProcess 成功，但新建待办尚未在快照中可见。
func (t *retrieveAuditFallbackTarget) ExecuteActionWrite(context.Context, target.Session, target.ActionWriteRequest) (target.WriteResponse, string, error) {
	t.writeCalls++
	t.written = true
	return target.WriteResponse{StatusCode: 200, IsSuccess: true, IsSuccessPresent: true}, "trace-retrieve", nil
}

// TestTransferVerifiedByAuditRecordWhenPendingLags 验证移交成功但原处理人 pending 待办快照滞后时，
// 由目标同事务写入的 transfer 审核记录确认移交已生效，不落「未变化」->暂时无法确认。
func TestTransferVerifiedByAuditRecordWhenPendingLags(t *testing.T) {
	targetFake := &transferAuditFallbackTarget{taskActionFactsTarget: &taskActionFactsTarget{
		fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-current"}, DueNodes: []string{"node-current"}}},
		snapshot: target.TaskSnapshot{
			LinkID: "current-link", JobTaskID: "current-task", BatchNo: "batch-current",
			FlowNodeProxyID: "node-current", FlowProxyID: "proxy-current", TaskStatus: "pending",
		},
		tree: &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{ID: "node-current", Type: "common"}},
	}}
	runCtx := newRunContext([]model.CompiledActionStep{{
		Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionTransfer,
		Scope: model.ActionScopeTask, NodeKey: "node-audit",
	}})
	runCtx.Source = "pending"
	runCtx.PathRun.MainInstanceRef = "instance-transfer"
	runCtx.Nodes["node-audit"] = step.NodeInfo{Name: "当前审批", Type: "common", TargetNodeID: "node-current"}
	runCtx.ActionPersonIDs = map[string][]string{
		step.ActionPersonIndex("node-audit", model.ActionTransfer): {"receiver-1"},
	}
	state := &fakeRunState{}
	facts := &fakeFacts{}
	executor := step.NewExecutor(targetFake, &fakeSessions{}, state, facts, fixedRunConfig(), nil)

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil || !preview.GateAllowed {
		t.Fatalf("移交前置事实应通过：finished=%v err=%v preview=%+v", finished, err, preview)
	}
	outcome, _, err := executor.RunApprovedStep(context.Background(), step.ApprovedStep{RunCtx: runCtx, Preview: preview, NextIndex: 0})
	if err != nil {
		t.Fatalf("移交执行失败：%v", err)
	}
	if outcome.Verdict != string(verdict.OutcomeSucceeded) {
		t.Fatalf("原处理人待办快照滞后时移交应由 transfer 审核记录确认成功，实际 verdict=%s", outcome.Verdict)
	}
	if targetFake.writeCalls != 1 {
		t.Fatalf("移交应只发送一次原子写请求，实际 %d 次", targetFake.writeCalls)
	}
	if len(facts.attempts) != 1 || facts.attempts[0].Reread != string(verdict.RereadAdvanced) {
		t.Fatalf("移交应由审核记录记录为已前进，attempts=%+v", facts.attempts)
	}
}

// TestRetrieveVerifiedByAuditRecordWhenPendingLags 验证取回成功但新建 pending 待办快照滞后时，
// 由目标同事务写入的 retrieve 审核记录确认取回已生效，不落「未变化」->暂时无法确认。
func TestRetrieveVerifiedByAuditRecordWhenPendingLags(t *testing.T) {
	targetFake := &retrieveAuditFallbackTarget{
		taskActionFactsTarget: &taskActionFactsTarget{
			fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-done"}, DueNodes: []string{"node-done"}}},
			tree: &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{
				ID: "node-done", Type: "common", Child: &target.FlowNodeTemplate{ID: "node-next", Type: "common"},
			}},
			records: []target.AuditRecordSnapshot{{FlowJobTaskID: "done-link", FlowInstanceID: "instance-retrieve", FlowNodeProxyID: "node-done", ExecutorID: "user-handler", AuditStatus: "pass"}},
		},
		before: target.TaskSnapshot{
			LinkID: "done-link", JobTaskID: "done-task", FlowNodeProxyID: "node-done",
			FlowProxyID: "proxy-current", TaskStatus: "done",
		},
	}
	targetFake.fakeTarget.handlerAccount = "handler-account"
	runCtx := newRunContext([]model.CompiledActionStep{{
		Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionRetrieve,
		Scope: model.ActionScopeCompletedTask, NodeKey: "node-audit",
	}})
	runCtx.Source = "done"
	runCtx.PathRun.MainInstanceRef = "instance-retrieve"
	runCtx.Nodes["node-audit"] = step.NodeInfo{Name: "已办审批", Type: "common", TargetNodeID: "node-done"}
	state := &fakeRunState{}
	facts := &fakeFacts{}
	executor := step.NewExecutor(targetFake, &fakeSessions{}, state, facts, fixedRunConfig(), nil)

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil || !preview.GateAllowed {
		t.Fatalf("取回前置事实应通过：finished=%v err=%v preview=%+v", finished, err, preview)
	}
	outcome, _, err := executor.RunApprovedStep(context.Background(), step.ApprovedStep{RunCtx: runCtx, Preview: preview, NextIndex: 0})
	if err != nil {
		t.Fatalf("取回执行失败：%v", err)
	}
	if outcome.Verdict != string(verdict.OutcomeSucceeded) {
		t.Fatalf("新建待办快照滞后时取回应由 retrieve 审核记录确认成功，实际 verdict=%s", outcome.Verdict)
	}
	if targetFake.writeCalls != 1 {
		t.Fatalf("取回应只发送一次原子写请求，实际 %d 次", targetFake.writeCalls)
	}
	if len(facts.attempts) != 1 || facts.attempts[0].Reread != string(verdict.RereadAdvanced) {
		t.Fatalf("取回应由审核记录记录为已前进，attempts=%+v", facts.attempts)
	}
}
