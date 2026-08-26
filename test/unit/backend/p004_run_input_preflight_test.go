package backend_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/formdata"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// TestP004FormMakingCompilerBuildsCompletePayloadWithoutMutation 验证完整 values 被深复制到真实新发起外壳且编译不发送请求。
func TestP004FormMakingCompilerBuildsCompletePayloadWithoutMutation(t *testing.T) {
	values := map[string]any{"title": "已确认标题", "rows": []any{map[string]any{"id": "row-1"}}, "project": `{"id":"p-1","name":"项目一"}`, "project__virtualName": "项目一"}
	compiled := target.CompileFormSubmission(target.FormSubmissionCompileInput{
		FlowSource: "new", RenderType: target.FormRenderTypeFormMaking, Values: values,
		Template: map[string]any{"list": []any{}, "config": map[string]any{}},
		Forms:    []target.FormRuntimeTemplate{{ID: "form-proxy-1", TemplateData: `{"list":[]}`}},
		Identity: target.FormSubmissionIdentity{UserID: "user-1", UserName: "发起人", CompanyID: "company-1", CompanyName: "测试公司", DepartmentID: "dep-1", DepartmentName: "研发部"},
	})
	if compiled.Status != target.FormSubmissionReady || compiled.Method != "POST" || compiled.Path != "/web/flowInstanceApi/submit" || compiled.PayloadDigest == "" {
		t.Fatalf("FormMaking 请求体未编译完成：%+v", compiled)
	}
	formData := compiled.Payload["formDataMongoVo"].(map[string]any)["data"].(map[string]any)
	if !reflect.DeepEqual(formData["rows"], []any{map[string]any{"id": "row-1"}}) || formData["project__virtualName"] != "项目一" {
		t.Fatalf("完整集合或虚拟字段在编译时丢失：%+v", formData)
	}
	identity := formData["global_user_basic_information"].(map[string]any)
	if identity["userId"] != "user-1" || identity["companyId"] != "company-1" {
		t.Fatalf("发起人身份外壳错误：%+v", identity)
	}
	values["title"] = "后续修改"
	if formData["title"] != "已确认标题" {
		t.Fatalf("编译结果与来源 values 共享了可变引用：%+v", formData)
	}
}

// TestP004FormMakingCompilerBlocksDynamicHooks 验证无法证明副作用的表单业务钩子不会被伪装成可运行请求。
func TestP004FormMakingCompilerBlocksDynamicHooks(t *testing.T) {
	compiled := target.CompileFormSubmission(target.FormSubmissionCompileInput{
		FlowSource: "new", RenderType: target.FormRenderTypeFormMaking, Values: map[string]any{"title": "值"},
		Template: map[string]any{"list": []any{}, "config": map[string]any{"beforeSubmitAndDraft": "saveBusiness()"}},
		Forms:    []target.FormRuntimeTemplate{{ID: "form-proxy-1"}},
		Identity: target.FormSubmissionIdentity{UserID: "user-1", CompanyID: "company-1"},
	})
	if compiled.Status != target.FormSubmissionBlocked || len(compiled.Issues) != 1 || compiled.Issues[0].Code != "dynamic_submit_hook" || len(compiled.Payload) != 0 {
		t.Fatalf("动态业务钩子未被明确阻断：%+v", compiled)
	}
}

// TestP004VueCompilerRequiresSubmitAndJavaEvidence 验证 Vue data 外壳只有在页面提交规则和 Java 路由同时证明时可编译。
func TestP004VueCompilerRequiresSubmitAndJavaEvidence(t *testing.T) {
	page := &target.VueCustomPageRule{
		Fields: []target.VueCustomFieldRule{{Path: "contractName", Required: true}},
		Submit: &target.VueCustomSubmitRule{Method: "POST", Path: "/web/contract/api/save", Payload: []string{"data"}, SuccessChecks: []string{"isSuccess"}, Blocked: true, Issues: []string{}},
		Java:   &target.JavaPageRule{Routes: []target.JavaRouteRule{{Method: "POST", Path: "/web/contract/api/save", Request: "ContractSaveRequest", Response: "BaseResponseProtocol"}}},
		Issues: []string{},
	}
	compiled := target.CompileFormSubmission(target.FormSubmissionCompileInput{FlowSource: "new", RenderType: target.FormRenderTypeVueCustom, Values: map[string]any{"contractName": "合同一"}, VuePage: page})
	if compiled.Status != target.FormSubmissionReady || compiled.Path != "/web/contract/api/save" || !reflect.DeepEqual(compiled.Payload, map[string]any{"data": map[string]any{"contractName": "合同一"}}) {
		t.Fatalf("Vue 保存协议未按双重证据编译：%+v", compiled)
	}
	page.Java = nil
	blocked := target.CompileFormSubmission(target.FormSubmissionCompileInput{FlowSource: "new", RenderType: target.FormRenderTypeVueCustom, Values: map[string]any{"contractName": "合同一"}, VuePage: page})
	if blocked.Status != target.FormSubmissionBlocked || blocked.Issues[0].Code != "java_submit_route_unverified" {
		t.Fatalf("缺少 Java 路由证据的 Vue 请求未阻断：%+v", blocked)
	}
}

// TestP004SubmissionCompilerCannotSendTargetWrite 固化 P4 只能纯编译、不得持有目标客户端或发起网络请求的边界。
func TestP004SubmissionCompilerCannotSendTargetWrite(t *testing.T) {
	sourcePath := filepath.Join(f010ProjectRoot(t), "internal", "adapter", "target", "form_submission.go")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("读取 P4 目标请求编译器失败：%v", err)
	}
	for _, forbidden := range []string{"func (c *Client)", "c.call(", "net/http", "http."} {
		if strings.Contains(string(source), forbidden) {
			t.Fatalf("P4 目标请求编译器出现写请求能力 %q，必须保持纯函数边界", forbidden)
		}
	}
}

type p004PreflightTarget struct {
	snapshot target.PathConfigurationSnapshot
}

// PathConfigurationSnapshot 返回当前目标模板快照，预检每次都从这里重读。
func (r *p004PreflightTarget) PathConfigurationSnapshot(context.Context, string, string, string) (target.PathConfigurationSnapshot, error) {
	return r.snapshot, nil
}

// FormRuntimeSession 返回不含持久化副作用的当前身份会话。
func (r *p004PreflightTarget) FormRuntimeSession(context.Context, string) (target.FormRuntimeSession, error) {
	return target.FormRuntimeSession{SID: "not-returned", AccountName: "发起人", UserID: "user-1", CompanyID: "company-1", CompanyName: "测试公司", DepartmentID: "dep-1", DepartmentName: "研发部"}, nil
}

// TestP004ServiceSnapshotIsImmutableAndStaleBlocks 验证后续配置编辑不覆盖旧快照，模板变化会在运行前阻断。
func TestP004ServiceSnapshotIsImmutableAndStaleBlocks(t *testing.T) {
	plans := newMemoryPlanRepository()
	plans.plans = []model.Plan{{ID: 401, Account: "account", FlowSource: "new", TargetObjectID: "template-1", TargetObjectName: "测试流程", Status: model.PlanStatusNotStarted}}
	paths := &memoryExecutionPathRepository{paths: []model.ExecutionPath{{ID: 402, PlanID: 401, SequenceNo: 1, Name: "路径 1"}}}
	tree := &target.FlowNodeTemplate{ID: "start", Name: "发起", Type: "start", Child: &target.FlowNodeTemplate{ID: "end", Name: "结束", Type: "end"}}
	templateJSON := `{"list":[{"type":"input","model":"title","name":"标题","options":{"required":true}}],"config":{}}`
	template := map[string]any{}
	if err := json.Unmarshal([]byte(templateJSON), &template); err != nil {
		t.Fatal(err)
	}
	repository := &f010StoredPathConfigRepository{stored: model.StoredPathConfig{
		PathID: 402, Revision: 5, NodeRevision: 2, FormRevision: 3, ConfigVersion: 4,
		FieldValues: map[string]map[string]string{"audit": {"assignee": "user-1"}}, ActionValues: map[string]string{"audit-action": "approve"}, ConfirmedNodeKeys: []string{"audit"},
		FormValues: map[string]any{"title": "首个快照"}, FormStatus: "valid", DataStatus: model.ExecutionPathDataConfirmed,
		FormValidated: true, FormTemplateVersion: formdata.TemplateVersion(template),
	}}
	reader := &p004PreflightTarget{snapshot: target.PathConfigurationSnapshot{
		Tree: tree, EntryNodeIDs: []string{"start"}, TemplateID: "template-1", RuleVersion: "rule-p004", RuleStatus: model.RuleReadinessReady,
		RenderType: target.FormRenderTypeFormMaking, Forms: []target.FormRuntimeTemplate{{ID: "form-proxy-1", Name: "表单", TemplateData: templateJSON}},
	}}
	serviceUnderTest := service.NewPathConfigService(service.NewPlanService(plans), reader, analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathConfigAnalyzer(), paths, repository)
	first, err := serviceUnderTest.PreflightRunInput(context.Background(), 401, 402)
	if err != nil || first.Status != model.RunInputPreflightReady {
		t.Fatalf("已确认 FormMaking 配置未通过运行输入预检：result=%+v err=%v", first, err)
	}
	if first.Snapshot.SnapshotDigest == "" || first.Snapshot.ShapeDigest == "" || first.Target.PayloadDigest == "" || first.Target.Path != "/web/flowInstanceApi/submit" {
		t.Fatalf("运行输入摘要或目标预览不完整：%+v", first)
	}
	repository.stored.FormValues["title"] = "后续编辑"
	repository.stored.FieldValues["audit"]["assignee"] = "user-2"
	repository.stored.ActionValues["audit-action"] = "reject"
	repository.stored.ConfirmedNodeKeys[0] = "changed"
	second, err := serviceUnderTest.PreflightRunInput(context.Background(), 401, 402)
	if err != nil || first.Snapshot.FormValues["title"] != "首个快照" || second.Snapshot.FormValues["title"] != "后续编辑" || first.Snapshot.SnapshotDigest == second.Snapshot.SnapshotDigest {
		t.Fatalf("快照未隔离后续配置编辑：first=%+v second=%+v err=%v", first.Snapshot, second.Snapshot, err)
	}
	if first.Snapshot.NodeFieldValues["audit"]["assignee"] != "user-1" || first.Snapshot.ActionValues["audit-action"] != "approve" || first.Snapshot.ConfirmedNodeKeys[0] != "audit" {
		t.Fatalf("节点人员或动作快照与来源配置共享了可变引用：%+v", first.Snapshot)
	}
	repository.stored.FormTemplateVersion = "stale-template-version"
	stale, err := serviceUnderTest.PreflightRunInput(context.Background(), 401, 402)
	if err != nil || stale.Status != model.RunInputPreflightBlocked || !hasP004Issue(stale.Issues, "form_configuration_invalid") {
		t.Fatalf("模板变化未阻断运行输入：result=%+v err=%v", stale, err)
	}
}

// hasP004Issue 判断预检结果是否包含指定稳定代码。
func hasP004Issue(issues []model.RunInputPreflightIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
