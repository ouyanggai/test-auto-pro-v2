package executor_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/engine/verdict"
	"test-auto-pro-v2/internal/model"
)

// fixedRunConfig 给出确定的重试预算：预算走配置，不在用例里散落魔法数。
func fixedRunConfig() config.RunConfig {
	return config.RunConfig{
		LeaseDuration:          time.Minute,
		ReadOnlyRetryAttempts:  3,
		ReadOnlyRetryBaseDelay: time.Millisecond,
		ReadOnlyRetryMaxDelay:  2 * time.Millisecond,
		StepProgressStaleAfter: time.Minute,
		StatusPollInterval:     time.Millisecond,
	}
}

// fakeRunState 记录状态机推进调用，供用例断言路径运行的状态路径。
type fakeRunState struct {
	claims        int
	markVerifying int
	backToRunning int
	released      int
	finishedTo    []model.PathRunStatus
	finishClasses []model.FailureClass
}

func (f *fakeRunState) SetMainInstanceRef(context.Context, uint64, string) error { return nil }

func (f *fakeRunState) ClaimExecution(context.Context, uint64) (uint64, error) {
	f.claims++
	return 7, nil
}

func (f *fakeRunState) RenewLease(context.Context, uint64, uint64) error { return nil }

func (f *fakeRunState) ReleaseExecution(context.Context, uint64, uint64) error {
	f.released++
	return nil
}

func (f *fakeRunState) MarkVerifying(context.Context, uint64) error {
	f.markVerifying++
	return nil
}

func (f *fakeRunState) BackToRunning(context.Context, uint64) error {
	f.backToRunning++
	return nil
}

func (f *fakeRunState) Finish(_ context.Context, _ uint64, to model.PathRunStatus, _ *model.RunResult, failureClass *model.FailureClass, _ string) (model.PathRun, error) {
	f.finishedTo = append(f.finishedTo, to)
	if failureClass != nil {
		f.finishClasses = append(f.finishClasses, *failureClass)
	}
	return model.PathRun{ID: 1, Status: to}, nil
}

// fakeFacts 记录落账的步骤与尝试事实。
type fakeFacts struct {
	steps    []model.RunStep
	attempts []model.RunStepAttempt
}

func (f *fakeFacts) RecordStepAttempt(_ context.Context, stepRecord model.RunStep, attempt model.RunStepAttempt, _ time.Time) (uint64, error) {
	f.steps = append(f.steps, stepRecord)
	f.attempts = append(f.attempts, attempt)
	return uint64(len(f.steps)), nil
}

// LatestStepAttempt 返回最近一次落账的步骤与尝试，供同一步恢复业务保存 id。
func (f *fakeFacts) LatestStepAttempt(_ context.Context, _ uint64) (model.RunStep, model.RunStepAttempt, error) {
	if len(f.steps) == 0 || len(f.attempts) == 0 {
		return model.RunStep{}, model.RunStepAttempt{}, sql.ErrNoRows
	}
	return f.steps[len(f.steps)-1], f.attempts[len(f.attempts)-1], nil
}

// fakeSessions 固定返回计划账号会话。
type fakeSessions struct {
	calls int
}

func (f *fakeSessions) Current(_ context.Context, account string) (target.Session, error) {
	f.calls++
	return target.Session{Summary: target.AccountSummary{Account: account, DisplayName: "测试账号"}}, nil
}

// Refresh 与 Current 同形：假件不区分缓存语义。
func (f *fakeSessions) Refresh(_ context.Context, account string) (target.Session, error) {
	f.calls++
	return target.Session{Summary: target.AccountSummary{Account: account, DisplayName: "测试账号"}}, nil
}

// fakeTargetView 是假件里一次目标事实读取的预设视图。
type fakeTargetView struct {
	Found        bool
	Status       string
	CurrentNodes []string
	DueNodes     []string
}

// fakeTarget 是目标能力的假件：读写调用计数与预设事实都可配置；
// afterSubmit 用于模拟发起成功后目标事实已经前进。
type fakeTarget struct {
	instance     fakeTargetView
	afterSubmit  *fakeTargetView
	afterAudit   *fakeTargetView
	flowProxyID  string
	formProxyID  string
	submitted    bool
	audited      bool
	dueTaskID    string
	submitResult *target.SubmitFlowInstanceResult
	submitErr    error
	submitDelay  time.Duration
	auditResult  *target.AuditCurrentTaskResult
	auditErr     error
	auditMessage string
	// actionResponse/actionErr 是统一原子动作出口的可控结果，供 F-019 非审批动作验证使用。
	actionResponse target.WriteResponse
	actionErr      error
	actionCalls    int
	submitCalls    int
	auditCalls     int
	// instanceFormData 是目标实例当前的完整表单数据；instanceDataReads 记录只读读取次数。
	instanceFormData  map[string]any
	instanceDataErr   error
	instanceDataReads int
	// dueTaskNodeID 记录待办读取实际收到的节点标识，用于锁定"发给目标的是真实标识"。
	dueTaskNodeID    string
	findDueTaskCalls int
	// handlerUserID/handlerAccount 是当前处理人事实发现路径的假件读数：
	// handlerUserID 是目标"已发"事实里该节点处理人的用户 ID，handlerAccount 是解析出的登录账号
	// （为空表示沿用当前会话，不额外登录）。默认值让既有用例在不改断言的前提下走真实的事实发现路径。
	handlerUserID  string
	handlerAccount string
	// 对账两个新增维度的假件读数（F-018 强证据规则）：
	// doneRecordFound 是本步节点是否已有已办记录，auditTraceFound 是是否已留下动作痕迹，
	// auditTraceTotal 只进依据说明；两个 Err 用于验证"读不到就按缺失降级"。
	doneRecordFound bool
	doneRecordErr   error
	auditTraceFound bool
	auditTraceTotal int
	auditTraceErr   error
	// auditAdvanceFromCall 表示从第几次同意调用起目标事实才切到 afterAudit，
	// 用于表达"重放那一次才真的生效"：0 表示只要发生过同意就切换。
	auditAdvanceFromCall int
	// businessSaveID 是前置业务保存返回的 id；businessSaveErr 注入失败。
	businessSaveID    string
	businessSaveErr   error
	specialSaveCalls  int
	noFormSaveCalls   int
	lastSpecialSpec   target.SpecialBusinessSpec
	lastNoFormSpec    target.NoFormFlowSpec
	lastSubmitRequest *target.SubmitFlowInstanceRequest
}

// FindDoneTaskOnNode 是对账「已办记录」维度的假件读取。
func (f *fakeTarget) FindDoneTaskOnNode(_ context.Context, _ target.Session, instanceID, _, _ string) (bool, error) {
	if f.doneRecordErr != nil {
		return false, f.doneRecordErr
	}
	if instanceID == "" {
		return false, nil
	}
	return f.doneRecordFound, nil
}

// FindAuditTraceOnNode 是对账「动作痕迹」维度的假件读取；第二个返回值是该实例的审核记录条数。
func (f *fakeTarget) FindAuditTraceOnNode(_ context.Context, _ target.Session, instanceID, _ string) (bool, int, error) {
	if f.auditTraceErr != nil {
		return false, 0, f.auditTraceErr
	}
	if instanceID == "" {
		return false, 0, nil
	}
	return f.auditTraceFound, f.auditTraceTotal, nil
}

func (f *fakeTarget) FindSubmittedFlow(context.Context, target.Session, string) (string, []string, string, []string, bool, error) {
	view := f.currentView()
	flowProxyID := f.flowProxyID
	if flowProxyID == "" {
		flowProxyID = "flow-proxy-1"
	}
	formProxyIDs := []string(nil)
	if f.formProxyID != "" {
		formProxyIDs = []string{f.formProxyID}
	}
	return flowProxyID, view.CurrentNodes, view.Status, formProxyIDs, view.Found, nil
}

func (f *fakeTarget) FindDueFlow(context.Context, target.Session, string) (string, []string, []string, bool, error) {
	view := f.currentView()
	return "flow-proxy-1", view.DueNodes, nil, view.Found, nil
}

// currentView 返回当前应呈现的目标事实视图：按已发生的写动作切换到对应阶段。
func (f *fakeTarget) currentView() fakeTargetView {
	if f.audited && f.afterAudit != nil && f.auditCalls >= f.auditAdvanceFromCall {
		return *f.afterAudit
	}
	if f.submitted && f.afterSubmit != nil {
		return *f.afterSubmit
	}
	return f.instance
}

// ReadInstanceCurrentData 返回预设的实例当前表单数据；未预设时按实例尚无数据处理。
func (f *fakeTarget) ReadInstanceCurrentData(context.Context, target.Session, string) (map[string]any, error) {
	f.instanceDataReads++
	if f.instanceDataErr != nil {
		return nil, f.instanceDataErr
	}
	return f.instanceFormData, nil
}

// currentHandlerUserID 返回假件为当前节点处理人分配的稳定用户 ID。
func (f *fakeTarget) currentHandlerUserID() string {
	if id := strings.TrimSpace(f.handlerUserID); id != "" {
		return id
	}
	return "user-handler"
}

// viewerTaskNodeID 返回假件在当前视图下待办所在的目标节点标识。
func (f *fakeTarget) viewerTaskNodeID() string {
	view := f.currentView()
	if len(view.DueNodes) > 0 {
		return strings.TrimSpace(view.DueNodes[0])
	}
	if len(view.CurrentNodes) > 0 {
		return strings.TrimSpace(view.CurrentNodes[0])
	}
	return strings.TrimSpace(f.dueTaskNodeID)
}

// FindSubmittedFlowFacts 返回带 currentAuditUserInfo 处理人的实例事实：
// 处理人挂在当前视图的待办节点上，执行器据此发现当前处理人（F-031/T03 的唯一首选来源）。
func (f *fakeTarget) FindSubmittedFlowFacts(_ context.Context, _ target.Session, _ string) (target.SubmittedFlowFacts, error) {
	view := f.currentView()
	flowProxyID := f.flowProxyID
	if flowProxyID == "" {
		flowProxyID = "flow-proxy-1"
	}
	formProxyIDs := []string(nil)
	if f.formProxyID != "" {
		formProxyIDs = []string{f.formProxyID}
	}
	handlers := make([]target.NodeCurrentHandler, 0, len(view.DueNodes))
	for _, nodeID := range view.DueNodes {
		handlers = append(handlers, target.NodeCurrentHandler{
			NodeID: strings.TrimSpace(nodeID), AuditType: "run_node_choose",
			BizIDs: []string{f.currentHandlerUserID()},
		})
	}
	return target.SubmittedFlowFacts{
		FlowProxyID: flowProxyID, CurrentNodes: view.CurrentNodes, Status: view.Status,
		FormProxyIDs: formProxyIDs, Handlers: handlers, Name: "测试实例", Found: view.Found,
	}, nil
}

// UserAccountsByID 是人员目录账号解析假件：默认把处理人解析为计划账号，
// 专用假件用 handlerAccount 表达"另一位处理人"的账号（actorSwitchTarget 另有自己的映射）。
func (f *fakeTarget) UserAccountsByID(_ context.Context, _ target.Session, ids []string) (map[string]string, error) {
	account := strings.TrimSpace(f.handlerAccount)
	if account == "" {
		account = "oyg-test"
	}
	result := make(map[string]string, len(ids))
	for _, id := range ids {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			result[trimmed] = account
		}
	}
	return result, nil
}

// MatchHandlerAccounts 把当前处理人解析为登录账号；账号为空表示沿用当前会话（不额外登录）。
func (f *fakeTarget) MatchHandlerAccounts(_ context.Context, _ target.Session, handler target.NodeCurrentHandler) ([]target.HandlerAccount, error) {
	result := []target.HandlerAccount{}
	for _, bizID := range handler.BizIDs {
		result = append(result, target.HandlerAccount{
			Key: "id:" + bizID, UserID: bizID, Name: "测试处理人", Account: strings.TrimSpace(f.handlerAccount),
		})
	}
	return result, nil
}

// ListTaskSnapshotsForUser 按视角用户返回本节点当前待办快照；
// 专用假件可覆盖本方法以表达各自的会话语义（例如按 SID 返回不同任务身份）。
func (f *fakeTarget) ListTaskSnapshotsForUser(_ context.Context, _ target.Session, _, status, queryUserID string) ([]target.TaskSnapshot, error) {
	if strings.TrimSpace(status) != "pending" || strings.TrimSpace(queryUserID) != f.currentHandlerUserID() {
		return nil, nil
	}
	if strings.TrimSpace(f.dueTaskID) == "" {
		return nil, nil
	}
	return []target.TaskSnapshot{{
		JobTaskID: f.dueTaskID, FlowNodeProxyID: f.viewerTaskNodeID(), FlowProxyID: "flow-proxy-1",
	}}, nil
}

func (f *fakeTarget) FindDueTaskID(_ context.Context, _ target.Session, _ string, nodeProxyID string) (string, error) {
	f.findDueTaskCalls++
	f.dueTaskNodeID = nodeProxyID
	return f.dueTaskID, nil
}

func (f *fakeTarget) SubmitFlowInstance(_ context.Context, _ target.Session, request target.SubmitFlowInstanceRequest) (*target.SubmitFlowInstanceResult, target.WriteResponse, string, error) {
	f.submitCalls++
	copied := request
	f.lastSubmitRequest = &copied
	if f.submitDelay > 0 {
		time.Sleep(f.submitDelay)
	}
	f.submitted = true
	if f.submitErr != nil {
		return nil, target.WriteResponse{}, "trace-fail", f.submitErr
	}
	return f.submitResult, target.WriteResponse{StatusCode: 200, IsSuccess: true, IsSuccessPresent: true}, "trace-submit", nil
}

// ExecuteActionWrite 模拟统一原子动作写出口，并保留目标响应 data 供动作专用结果确认。
func (f *fakeTarget) ExecuteActionWrite(_ context.Context, _ target.Session, _ target.ActionWriteRequest) (target.WriteResponse, string, error) {
	f.actionCalls++
	if f.actionErr != nil {
		return f.actionResponse, "trace-action", f.actionErr
	}
	if f.actionResponse.StatusCode != 0 || f.actionResponse.IsSuccessPresent || len(f.actionResponse.Data) > 0 {
		return f.actionResponse, "trace-action", nil
	}
	return target.WriteResponse{StatusCode: 200, IsSuccess: true, IsSuccessPresent: true}, "trace-action", nil
}

func (f *fakeTarget) AuditCurrentTask(context.Context, target.Session, target.AuditCurrentTaskRequest) (*target.AuditCurrentTaskResult, target.WriteResponse, string, error) {
	f.auditCalls++
	f.audited = true
	if f.auditErr != nil {
		return nil, target.WriteResponse{}, "trace-fail", f.auditErr
	}
	if f.auditMessage != "" {
		response := target.WriteResponse{StatusCode: 200, IsSuccessPresent: true, Message: f.auditMessage}
		return nil, response, fmt.Sprintf("trace-audit-%d", f.auditCalls), &target.BusinessRejection{Message: f.auditMessage}
	}
	// 每次写请求各自一个链路 ID：真实客户端也是每次调用新生成，重放必须能与首次尝试区分开。
	return f.auditResult, target.WriteResponse{StatusCode: 200, IsSuccess: true, IsSuccessPresent: true},
		fmt.Sprintf("trace-audit-%d", f.auditCalls), nil
}

// SaveSpecialBusiness 模拟 FormMaking 特殊业务前置保存，供执行器在主流程前调用。
func (f *fakeTarget) SaveSpecialBusiness(_ context.Context, _ target.Session, spec target.SpecialBusinessSpec, _ json.RawMessage, _ string) (target.SpecialBusinessSaveResult, error) {
	f.specialSaveCalls++
	f.lastSpecialSpec = spec
	if f.businessSaveErr != nil {
		return target.SpecialBusinessSaveResult{}, f.businessSaveErr
	}
	id := strings.TrimSpace(f.businessSaveID)
	if id == "" {
		id = "biz-" + spec.FlowType
	}
	return target.SpecialBusinessSaveResult{
		BusinessID: id,
		Response:   target.WriteResponse{StatusCode: 200, IsSuccess: true, IsSuccessPresent: true},
		TraceID:    "trace-special-save",
	}, nil
}

// SaveNoFormFlow 模拟无表单 mixin.saveData 前置保存。
func (f *fakeTarget) SaveNoFormFlow(_ context.Context, _ target.Session, spec target.NoFormFlowSpec, _ json.RawMessage, _ string) (target.SpecialBusinessSaveResult, error) {
	f.noFormSaveCalls++
	f.lastNoFormSpec = spec
	if f.businessSaveErr != nil {
		return target.SpecialBusinessSaveResult{}, f.businessSaveErr
	}
	id := strings.TrimSpace(f.businessSaveID)
	if id == "" {
		id = "biz-" + spec.PageKey
	}
	return target.SpecialBusinessSaveResult{
		BusinessID: id,
		Response:   target.WriteResponse{StatusCode: 200, IsSuccess: true, IsSuccessPresent: true},
		TraceID:    "trace-noform-save",
	}, nil
}

// TestF019StepReasonUsesTargetError 验证步骤记录直接保存目标接口原文，不再保存内部判定术语。
func TestF019StepReasonUsesTargetError(t *testing.T) {
	fakeTarget := &fakeTarget{
		instance:     fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-audit"}, DueNodes: []string{"node-audit"}},
		dueTaskID:    "task-1",
		auditMessage: "当前节点已被其他人处理",
	}
	facts := &fakeFacts{}
	executor := step.NewExecutor(fakeTarget, &fakeSessions{}, &fakeRunState{}, facts, fixedRunConfig(), nil)
	runCtx := newRunContext([]model.CompiledActionStep{approveStep()})
	runCtx.PathRun.MainInstanceRef = "instance-9"
	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || preview == nil || !preview.GateAllowed {
		t.Fatalf("审批步预览应通过：err=%v block=%s", err, previewBlock(preview))
	}
	if _, _, err := executor.RunApprovedStep(context.Background(), step.ApprovedStep{RunCtx: runCtx, Preview: preview, NextIndex: 0}); err != nil {
		t.Fatalf("放行执行失败：%v", err)
	}
	if len(facts.attempts) != 1 {
		t.Fatalf("应记录一次尝试，实际 %d 次", len(facts.attempts))
	}
	if got, want := facts.attempts[0].Reason, "执行失败：当前节点已被其他人处理"; got != want {
		t.Fatalf("步骤结果应展示目标原文，实际 %q，期望 %q", got, want)
	}
}

// newRunContext 构造一条「新发起」单步场景：发起后接同意。
func newRunContext(steps []model.CompiledActionStep) step.RunContext {
	return step.RunContext{
		Run:         model.Run{ID: 1, PlanID: 1, RunNo: 1},
		PathRun:     model.PathRun{ID: 11, RunID: 1, Status: model.PathRunStatusRunning},
		PathName:    "路径 1",
		PlanAccount: "oyg-test",
		FlowProxyID: "flow-proxy-1",
		Source:      "new",
		// TargetNodeID 与键同值：真实装配里键是不透明派生键、TargetNodeID 是目标真实节点标识，
		// 用例只需要保证两者都存在，凡是与目标事实对照的地方都走 TargetNodeID。
		Nodes: map[string]step.NodeInfo{
			"node-start": {Name: "发起人", Type: "start", TargetNodeID: "node-start"},
			"node-audit": {Name: "部门审批", Type: "审批", TargetNodeID: "node-audit"},
		},
		Steps:             steps,
		EffectiveFormData: []byte(`{"amount":"12.30"}`),
	}
}

// submitStep 与 approveStep 是两个最小编译步骤。
func submitStep() model.CompiledActionStep {
	return model.CompiledActionStep{Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionSubmit, Scope: model.ActionScopeInitiator, NodeKey: "node-start"}
}

// resubmitStep 构造一条发起人重新提交步骤。
func resubmitStep() model.CompiledActionStep {
	return model.CompiledActionStep{Sequence: 1, Source: model.ActionStepSourceUser, Action: model.ActionResubmit, Scope: model.ActionScopeInitiator, NodeKey: "node-start"}
}

func approveStep() model.CompiledActionStep {
	return model.CompiledActionStep{Sequence: 2, Source: model.ActionStepSourceUser, Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: "node-audit"}
}

// TestF019ResubmitUsesLiveProxyFacts 验证重新提交门禁和载荷都使用目标当前实例返回的代理标识。
func TestF019ResubmitUsesLiveProxyFacts(t *testing.T) {
	fakeTarget := &fakeTarget{
		instance:    fakeTargetView{Found: true, Status: "rejected"},
		flowProxyID: "flow-live",
		formProxyID: "form-live",
	}
	runCtx := newRunContext([]model.CompiledActionStep{resubmitStep()})
	runCtx.PathRun.MainInstanceRef = "instance-9"
	executor := step.NewExecutor(fakeTarget, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || preview == nil || !preview.GateAllowed {
		t.Fatalf("驳回实例的重新提交应通过门禁：err=%v preview=%+v", err, preview)
	}
	if preview.Endpoint != target.WriteEndpointReSubmit {
		t.Fatalf("重新提交端点错误：%s", preview.Endpoint)
	}
	data, ok := preview.RequestPayload["data"].(map[string]any)
	if !ok {
		t.Fatalf("重新提交载荷缺少 data：%v", preview.RequestPayload)
	}
	// F-035 代理互斥：FormMaking 重提只发实时的 formProxyId，不再同时携带 flowProxyId。
	if data["formProxyId"] != "form-live" || data["flowProxyId"] != nil {
		t.Fatalf("必须且只能使用目标实代表单代理标识：%v", data)
	}
}

// TestF016RereadClassification 锁定事实重读四值的判定口径。
func TestF016RereadClassification(t *testing.T) {
	cases := []struct {
		name    string
		action  string
		nodeKey string
		before  step.InstanceFacts
		after   step.InstanceFacts
		want    verdict.Reread
	}{
		{"发起后实例已运行", string(model.ActionSubmit), "", step.InstanceFacts{}, step.InstanceFacts{Found: true, Status: "run"}, verdict.RereadAdvanced},
		{"发起后实例仍不存在", string(model.ActionSubmit), "", step.InstanceFacts{}, step.InstanceFacts{}, verdict.RereadUnchanged},
		{"发起后落成草稿", string(model.ActionSubmit), "", step.InstanceFacts{}, step.InstanceFacts{Found: true, Status: "draft"}, verdict.RereadContradictory},
		{"重读失败", string(model.ActionApprove), "node-audit", step.InstanceFacts{DueNodes: []string{"node-audit"}}, step.InstanceFacts{ReadError: "目标抖动"}, verdict.RereadUnreadable},
		{"审批后待办仍在", string(model.ActionApprove), "node-audit", step.InstanceFacts{DueNodes: []string{"node-audit"}}, step.InstanceFacts{Found: true, Status: "run", DueNodes: []string{"node-audit"}}, verdict.RereadUnchanged},
		{"审批后待办消失", string(model.ActionApprove), "node-audit", step.InstanceFacts{DueNodes: []string{"node-audit"}}, step.InstanceFacts{Found: true, Status: "run", DueNodes: nil}, verdict.RereadAdvanced},
		{"审批后实例被撤回", string(model.ActionApprove), "node-audit", step.InstanceFacts{DueNodes: []string{"node-audit"}}, step.InstanceFacts{Found: true, Status: "withdraw", DueNodes: nil}, verdict.RereadContradictory},
		{"暂存后流程事实稳定", string(model.ActionStorageFormData), "node-audit", step.InstanceFacts{Found: true, Status: "run", DueNodes: []string{"node-audit"}, ActionFactRead: true, StorageFound: true, StorageDataID: "data-1", StorageAuditDesc: "同一说明", StorageUpdateDate: "2026-09-07 10:00:00"}, step.InstanceFacts{Found: true, Status: "run", DueNodes: []string{"node-audit"}, ActionFactRead: true, StorageFound: true, StorageDataID: "data-1", StorageAuditDesc: "同一说明", StorageUpdateDate: "2026-09-07 10:00:00"}, verdict.RereadUnchanged},
		{"暂存后检查点已更新", string(model.ActionStorageFormData), "node-audit", step.InstanceFacts{Found: true, Status: "run", ActionFactRead: true, StorageFound: true, StorageDataID: "data-1"}, step.InstanceFacts{Found: true, Status: "run", ActionFactRead: true, StorageFound: true, StorageDataID: "data-2"}, verdict.RereadAdvanced},
		{"催办记录已增加", string(model.ActionUrge), "", step.InstanceFacts{Found: true, Status: "run", DueNodes: []string{"node-audit"}, ActionFactRead: true, UrgeRecordCount: 1}, step.InstanceFacts{Found: true, Status: "run", DueNodes: []string{"node-audit"}, ActionFactRead: true, UrgeRecordCount: 2}, verdict.RereadAdvanced},
		{"关注状态已开启", string(model.ActionFollow), "", step.InstanceFacts{Found: true, Status: "run", ActionFactRead: true, Tracking: false}, step.InstanceFacts{Found: true, Status: "run", ActionFactRead: true, Tracking: true}, verdict.RereadAdvanced},
	}
	for _, item := range cases {
		if got := step.ClassifyReread(item.action, item.nodeKey, item.before, item.after); got != item.want {
			t.Fatalf("%s：重读结论应为 %s，实际 %s", item.name, item.want, got)
		}
	}
}

// TestF019StableActionSuccessDoesNotStopLoop 验证不推进流程节点的目标动作成功后不会被误判为不确定。
func TestF019StableActionSuccessDoesNotStopLoop(t *testing.T) {
	result := verdict.Evaluate(verdict.Observation{
		Action:             string(model.ActionStorageFormData),
		Endpoint:           "/web/flowInstanceApi/storageFormData",
		Transport:          verdict.TransportResponded,
		StatusCode:         200,
		Response:           &verdict.Response{IsSuccess: true, IsSuccessPresent: true},
		Reread:             verdict.RereadUnchanged,
		ActionFactVerified: true,
	})
	if result.Outcome != verdict.OutcomeSucceeded {
		t.Fatalf("暂存成功且流程事实稳定时应继续动作循环，实际：%+v", result)
	}
}

// TestF016SubmitHappyPath 验证发起步的七阶段闭环：一次写请求、状态推进到下一步、事实落账。
func TestF016SubmitHappyPath(t *testing.T) {
	fakeTarget := &fakeTarget{submitResult: &target.SubmitFlowInstanceResult{InstanceID: "instance-9", Status: "run"}, afterSubmit: &fakeTargetView{Found: true, Status: "run"}}
	sessions := &fakeSessions{}
	runState := &fakeRunState{}
	facts := &fakeFacts{}
	executor := step.NewExecutor(fakeTarget, sessions, runState, facts, fixedRunConfig(), nil)
	runCtx := newRunContext([]model.CompiledActionStep{submitStep(), approveStep()})

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil {
		t.Fatalf("预览构建失败：finished=%v err=%v", finished, err)
	}
	if !preview.GateAllowed {
		t.Fatalf("新发起的门禁应通过：%s", preview.BlockReason)
	}
	if preview.Endpoint != "/web/flowInstanceApi/submit" {
		t.Fatalf("端点应为发起白名单端点，实际 %s", preview.Endpoint)
	}
	if fakeTarget.submitCalls != 0 {
		t.Fatalf("预览阶段绝不允许发出写请求，实际 %d 次", fakeTarget.submitCalls)
	}

	outcome, _, err := executor.RunApprovedStep(context.Background(), step.ApprovedStep{RunCtx: runCtx, Preview: preview, NextIndex: 0})
	if err != nil {
		t.Fatalf("放行执行失败：%v", err)
	}
	if outcome.Verdict != string(verdict.OutcomeSucceeded) || outcome.MainInstanceRef != "instance-9" {
		t.Fatalf("发起步应确定成功并回填实例引用：%+v", outcome)
	}
	if fakeTarget.submitCalls != 1 {
		t.Fatalf("一次尝试最多一次写请求，实际 %d 次", fakeTarget.submitCalls)
	}
	if runState.markVerifying != 1 || runState.backToRunning != 1 || runState.released != 1 {
		t.Fatalf("状态推进应为核验中->运行中并释放租约：%+v", runState)
	}
	if len(facts.attempts) != 1 || facts.attempts[0].Verdict != string(verdict.OutcomeSucceeded) {
		t.Fatalf("尝试事实应落账且结论为确定成功：%+v", facts.attempts)
	}
	if facts.attempts[0].LogPath != "" || facts.attempts[0].TraceID == "" {
		t.Fatalf("单测假件未接装配层的相对路径注入，LogPath 应为空、由装配层填充；trace_id 必须存在：%+v", facts.attempts[0])
	}
}

// TestF016AuditUncertainStopsInAwaitingReconciliation 验证写结果不确定即停在待对账：
// 审批返回成功声明但待办仍在（明确未变），结论必须是不确定，路径停止且无第二次写请求。
func TestF016AuditUncertainStopsInAwaitingReconciliation(t *testing.T) {
	fakeTarget := &fakeTarget{
		instance:    fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-audit"}, DueNodes: []string{"node-audit"}},
		dueTaskID:   "task-1",
		auditResult: &target.AuditCurrentTaskResult{InstanceID: "instance-9", Status: "run"},
	}
	sessions := &fakeSessions{}
	runState := &fakeRunState{}
	facts := &fakeFacts{}
	executor := step.NewExecutor(fakeTarget, sessions, runState, facts, fixedRunConfig(), nil)
	runCtx := newRunContext([]model.CompiledActionStep{approveStep()})
	runCtx.PathRun.MainInstanceRef = "instance-9"

	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || preview == nil || !preview.GateAllowed {
		t.Fatalf("持有待办的审批步门禁应通过：err=%v block=%s", err, previewBlock(preview))
	}
	outcome, _, err := executor.RunApprovedStep(context.Background(), step.ApprovedStep{RunCtx: runCtx, Preview: preview, NextIndex: 0})
	if err != nil {
		t.Fatalf("放行执行失败：%v", err)
	}
	if outcome.Verdict != string(verdict.OutcomeUncertain) {
		t.Fatalf("成功声明+待办未变必须判不确定，实际 %s", outcome.Verdict)
	}
	if fakeTarget.auditCalls != 1 {
		t.Fatalf("不确定后绝不允许重发，实际 %d 次", fakeTarget.auditCalls)
	}
	if len(runState.finishedTo) != 1 || runState.finishedTo[0] != model.PathRunStatusAwaitingReconciliation {
		t.Fatalf("路径运行应进入待对账：%+v", runState.finishedTo)
	}
	if len(runState.finishClasses) != 1 || runState.finishClasses[0] != model.FailureClassWriteUncertain {
		t.Fatalf("失败分类应为写结果不确定：%+v", runState.finishClasses)
	}
}

// TestF016GateDenialBlocksApproval 验证门禁不通过绝不放行：
// 实例事实读不到待办时，审批步被门禁阻塞，放行请求把路径置为失败而不是带病发送写请求。
func TestF016GateDenialBlocksApproval(t *testing.T) {
	fakeTarget := &fakeTarget{instance: fakeTargetView{Found: true, Status: "run", DueNodes: nil}}
	executor := step.NewExecutor(fakeTarget, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)
	runCtx := newRunContext([]model.CompiledActionStep{approveStep()})
	runCtx.PathRun.MainInstanceRef = "instance-9"

	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || preview == nil {
		t.Fatalf("预览构建失败：%v", err)
	}
	if preview.GateAllowed || preview.BlockReason == "" {
		t.Fatalf("无待办的审批步必须被门禁阻塞：%+v", preview)
	}
	runState := &fakeRunState{}
	executor2 := step.NewExecutor(fakeTarget, &fakeSessions{}, runState, &fakeFacts{}, fixedRunConfig(), nil)
	if _, _, err := executor2.RunApprovedStep(context.Background(), step.ApprovedStep{RunCtx: runCtx, Preview: preview, NextIndex: 0}); err != nil {
		t.Fatalf("被阻塞步骤的放行应优雅拒绝：%v", err)
	}
	if fakeTarget.auditCalls != 0 {
		t.Fatalf("门禁不通过绝不允许发出写请求，实际 %d 次", fakeTarget.auditCalls)
	}
	if len(runState.finishedTo) != 1 || runState.finishedTo[0] != model.PathRunStatusFailed {
		t.Fatalf("路径运行应置为失败：%+v", runState.finishedTo)
	}
}

// TestF016RetryBudgetOnReadOnlyPhases 验证只读阶段的有界重试：可重试错误按预算退避，
// 预算耗尽如实失败；submit 阶段没有重试机制可依赖。
func TestF016RetryBudgetOnReadOnlyPhases(t *testing.T) {
	attempts := 0
	sleeps := []time.Duration{}
	policy := step.RetryPolicy{Attempts: 3, BaseDelay: 10 * time.Millisecond, MaxDelay: 40 * time.Millisecond, Sleep: func(d time.Duration) {
		sleeps = append(sleeps, d)
	}}
	result, err := step.RunWithRetry(context.Background(), policy, "测试操作", func() (int, error) {
		attempts++
		if attempts < 3 {
			return 0, &target.Error{Kind: target.ErrorTimeout}
		}
		return attempts, nil
	}, nil)
	if err != nil || result != 3 {
		t.Fatalf("第 3 次应成功：result=%d err=%v", result, err)
	}
	if len(sleeps) != 2 || sleeps[0] != 10*time.Millisecond || sleeps[1] != 20*time.Millisecond {
		t.Fatalf("指数退避应为 10ms、20ms：%v", sleeps)
	}

	attempts = 0
	_, err = step.RunWithRetry(context.Background(), policy, "测试操作", func() (int, error) {
		attempts++
		return 0, &target.Error{Kind: target.ErrorLoginRejected}
	}, nil)
	if attempts != 1 || err == nil {
		t.Fatalf("登录被拒是确定性失败，不应重试且必须返回错误：attempts=%d err=%v", attempts, err)
	}

	// 预算耗尽：3 次全部失败后如实返回最后一次错误。
	budget := step.RetryPolicy{Attempts: 3, BaseDelay: time.Millisecond, Sleep: func(time.Duration) {}}
	exhausted := 0
	_, err = step.RunWithRetry(context.Background(), budget, "测试操作", func() (int, error) {
		exhausted++
		return 0, &target.Error{Kind: target.ErrorUnavailable}
	}, nil)
	if exhausted != 3 || err == nil {
		t.Fatalf("预算耗尽必须如实失败：exhausted=%d err=%v", exhausted, err)
	}
}

// TestWriteConnectRetryStopsAtTransportBoundary 验证写请求只在明确未建立连接时重试，
// 请求发出后的响应丢失与完整业务拒绝都只调用一次。
func TestWriteConnectRetryStopsAtTransportBoundary(t *testing.T) {
	policy := step.RetryPolicy{
		WriteConnectAttempts:  3,
		WriteConnectBaseDelay: time.Millisecond,
		WriteConnectMaxDelay:  2 * time.Millisecond,
		Sleep:                 func(time.Duration) {},
	}
	connectCalls := 0
	_, _, _, err, attempted := step.RunWithWriteConnectRetry(context.Background(), policy,
		func(context.Context) (int, target.WriteResponse, string, error) {
			connectCalls++
			if connectCalls < 3 {
				failure := target.NewError(target.ErrorUnavailable, nil).(*target.Error)
				failure.Transport = target.TransportConnectFailed
				return 0, target.WriteResponse{}, "", failure
			}
			return 3, target.WriteResponse{}, "trace-success", nil
		}, nil)
	if err != nil || !attempted || connectCalls != 3 {
		t.Fatalf("连接阶段应重试至成功：calls=%d attempted=%v err=%v", connectCalls, attempted, err)
	}

	interruptedCalls := 0
	_, _, _, err, attempted = step.RunWithWriteConnectRetry(context.Background(), policy,
		func(context.Context) (int, target.WriteResponse, string, error) {
			interruptedCalls++
			failure := target.NewError(target.ErrorTimeout, nil).(*target.Error)
			failure.Transport = target.TransportInterrupted
			return 0, target.WriteResponse{}, "trace-lost", failure
		}, nil)
	if err == nil || !attempted || interruptedCalls != 1 {
		t.Fatalf("响应丢失写请求不得重发：calls=%d attempted=%v err=%v", interruptedCalls, attempted, err)
	}

	businessCalls := 0
	_, _, _, err, attempted = step.RunWithWriteConnectRetry(context.Background(), policy,
		func(context.Context) (int, target.WriteResponse, string, error) {
			businessCalls++
			return 0, target.WriteResponse{Code: "FLOW_409", Message: "业务拒绝"}, "trace-business", &target.BusinessRejection{Code: "FLOW_409", Message: "业务拒绝"}
		}, nil)
	if err == nil || !attempted || businessCalls != 1 {
		t.Fatalf("业务拒绝不得重试：calls=%d attempted=%v err=%v", businessCalls, attempted, err)
	}
}

// previewBlock 安全取出预览的阻塞原因（测试辅助）。
func previewBlock(preview *step.StepPreview) string {
	if preview == nil {
		return "<nil>"
	}
	return preview.BlockReason
}

// CurrentUserIdentity 为 fakeTarget 提供实时身份读取（F-035 评审补充）：
// 假件返回派生自会话账号的完整身份（含岗位），让既有用例继续走生产同一条身份覆盖路径。
func (f *fakeTarget) CurrentUserIdentity(_ context.Context, active target.Session) (target.UserIdentity, error) {
	account := strings.TrimSpace(active.Summary.Account)
	if account == "" {
		account = "plan-account"
	}
	return target.UserIdentity{
		UserID: "uid-" + account, UserName: "姓名-" + account,
		CompanyID: "company-" + account, CompanyName: "公司-" + account,
		DepartmentID: "dept-" + account, DepartmentName: "部门-" + account,
		DutyID: "duty-" + account, DutyName: "岗位-" + account,
	}, nil
}
