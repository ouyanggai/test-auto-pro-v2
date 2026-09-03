package f012_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/actioncatalog"
	"test-auto-pro-v2/internal/model"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
	"test-auto-pro-v2/internal/service"
)

// historyReadOnlyEndpoints 是 F-012 允许访问的目标只读端点；其余任何路径都视为越界调用。
var historyReadOnlyEndpoints = map[string]bool{
	"/web/user/api/login/user/login":          true,
	"/web/flowInstanceApi/list":               true,
	"/web/flowInstanceApi/getCurrentFromData": true,
	"/web/flowProxy/findById":                 true,
	"/web/formProxy/findById":                 true,
	"/web/flowJobTaskLink/list":               true,
	"/web/flowTemplateApi/list":               true,
	"/web/flowTemplateApi/findById":           true,
	"/web/formTemplateApi/findById":           true,
}

// recordingTargetGateway 记录 F-012 历史链路对目标平台发起的每一次请求。
type recordingTargetGateway struct {
	t     *testing.T
	mu    sync.Mutex
	paths []string
	raw   map[string]any
}

// handler 只实现历史读取协议：其他路径一律 404 并被记录，供越界断言使用。
func (g *recordingTargetGateway) handler(response http.ResponseWriter, request *http.Request) {
	g.mu.Lock()
	g.paths = append(g.paths, request.URL.Path)
	g.mu.Unlock()
	switch request.URL.Path {
	case "/web/user/api/login/user/login":
		writeRecordingJSON(response, map[string]any{"isSuccess": true, "sid": "sid-recording", "data": map[string]any{
			"user":      map[string]any{"id": "user-current", "name": "当前用户", "customerCode": "customer", "departmentId": "department"},
			"companyVo": map[string]any{"id": "company-current", "name": "测试公司", "customerCode": "customer"},
		}})
	case "/web/flowInstanceApi/list":
		writeRecordingJSON(response, map[string]any{"isSuccess": true, "total": 1, "pages": 1, "current": 1, "size": 100, "data": []any{
			map[string]any{"id": "history-end", "flowProxyId": "proxy-end", "formProxyId": "form-proxy-end",
				"flowCode": "expense-flow", "flowName": "费用审批", "formName": "费用单（测试公司）", "name": "已完成数据",
				"status": "end", "createDate": "2026-08-31 10:00:00", "createrId": "user-current", "companyId": "company-current"},
		}})
	case "/web/flowInstanceApi/getCurrentFromData":
		writeRecordingJSON(response, map[string]any{"isSuccess": true, "data": map[string]any{"data": g.raw}})
	case "/web/flowProxy/findById":
		writeRecordingJSON(response, map[string]any{"isSuccess": true, "data": map[string]any{
			"flowCode": "expense-flow", "flowName": "费用审批", "auditWay": "formmaking", "formExist": "form",
			"flowNodeTemplate": map[string]any{"id": "start", "name": "发起", "type": "start"},
			"formTemplateList": []any{map[string]any{"id": "form-proxy-end", "name": "费用单（测试公司）"}},
		}})
	case "/web/formProxy/findById":
		writeRecordingJSON(response, map[string]any{"isSuccess": true, "data": map[string]any{
			"id": "form-proxy-end", "name": "费用单（测试公司）", "fieldsTemplateList": []any{},
			"templateData": `{"list":[{"type":"input","model":"amount","name":"金额"}]}`,
		}})
	default:
		http.NotFound(response, request)
	}
}

// TestHistoryChainNeverCallsTargetWriteOperations 端到端验证候选、快照读取和真实落盘全过程只访问目标只读端点，
// 动作目录声明的全部目标写操作调用次数为零。
func TestHistoryChainNeverCallsTargetWriteOperations(t *testing.T) {
	database, _, ctx := openHistoryIntegrationDatabase(t, "目标写调用为零")
	store := planmysql.NewHistoryReplayRepository(database.DB)
	planID, _ := insertHistoryPlanWithPaths(t, database.DB, 20901, "template-a", "new", 1)

	gateway := &recordingTargetGateway{t: t, raw: map[string]any{
		"amount": json.Number("12.30"),
		"nested": map[string]any{"rows": []any{map[string]any{"tax": json.Number("0.010")}}},
	}}
	server := httptest.NewServer(http.HandlerFunc(gateway.handler))
	defer server.Close()
	client, err := target.NewClient(target.ClientConfig{
		BaseURL: server.URL, LoginPassword: "password", LoginAESKey: "0123456789abcdef", LoginCode: "code",
		PlatformCode: "invest", CustomerCode: "customer", Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("创建目标客户端失败：%v", err)
	}
	reader := service.NewTargetReadServiceWithClient(client, time.Hour)

	page, err := reader.HistoryCandidates(context.Background(), "account-a", "expense-flow", "费用单（测试公司）", "费用审批", "", 1, 20)
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("读取历史候选失败：err=%v items=%d", err, len(page.Items))
	}
	candidateKey := service.HistoryCandidateKey("account-a", page.Items[0])
	read, err := reader.ReadHistorySnapshot(context.Background(), "account-a", "expense-flow", "费用单（测试公司）", "费用审批", candidateKey)
	if err != nil {
		t.Fatalf("读取历史快照失败：%v", err)
	}

	saved, err := store.SaveSnapshot(ctx, model.HistorySnapshot{
		PlanID: planID, SourceAccount: "account-a", CandidateKey: candidateKey, FlowCode: "expense-flow",
		FormName: "费用单（测试公司）", FlowName: "费用审批", RuntimeType: string(read.RenderType), InstanceStatus: read.Instance.Status,
		InstanceSummary: map[string]any{"title": read.Instance.Title, "initiator": read.Instance.Initiator,
			"companyName": read.Instance.CompanyName, "createdAt": read.Instance.CreatedAt, "status": read.Instance.Status},
		TemplateSummary: read.TemplateSummary, RawFormData: read.RawFormData,
		SourceDigest: "digest-readonly", CreatedAt: time.Date(2026, 9, 1, 17, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("保存历史快照失败：%v", err)
	}
	loaded, err := store.GetSnapshot(ctx, planID, saved.ID)
	if err != nil {
		t.Fatalf("读取历史快照失败：%v", err)
	}
	if !reflect.DeepEqual(loaded.RawFormData, gateway.raw) {
		t.Fatalf("目标原始历史数据在落盘后被改写：want=%v got=%v", gateway.raw, loaded.RawFormData)
	}

	gateway.mu.Lock()
	paths := append([]string(nil), gateway.paths...)
	gateway.mu.Unlock()
	if len(paths) == 0 {
		t.Fatal("历史链路没有真实访问目标平台，写调用断言无效")
	}
	for _, path := range paths {
		if !historyReadOnlyEndpoints[path] {
			t.Fatalf("历史链路访问了未批准端点：%s", path)
		}
	}
	for _, operation := range assertCatalogTargetWriteOperations(t) {
		for _, path := range paths {
			if path == operation || strings.HasSuffix(operation, path) {
				t.Fatalf("历史链路调用了目标写操作：%s", operation)
			}
		}
	}
}

// TestTargetAdapterDeclaresNoWriteEndpoint 静态验证目标适配层没有实现任何动作目录声明的写端点，
// F-012 不可能因为调用点疏漏而向目标平台写入。
func TestTargetAdapterDeclaresNoWriteEndpoint(t *testing.T) {
	source, err := os.ReadFile("../../../internal/adapter/target/client.go")
	if err != nil {
		t.Fatalf("读取目标适配层源码失败：%v", err)
	}
	for _, operation := range assertCatalogTargetWriteOperations(t) {
		if strings.Contains(string(source), operation) {
			t.Fatalf("目标适配层仍然实现了写端点：%s", operation)
		}
	}
}

// assertCatalogTargetWriteOperations 取出动作目录写操作清单，并先证明清单非空，避免写调用断言变成空断言。
func assertCatalogTargetWriteOperations(t *testing.T) []string {
	t.Helper()
	operations := catalogTargetWriteOperations()
	var hasSubmit, hasAudit bool
	for _, operation := range operations {
		hasSubmit = hasSubmit || strings.HasSuffix(operation, "/submit")
		hasAudit = hasAudit || strings.HasSuffix(operation, "/audit")
	}
	if len(operations) == 0 || !hasSubmit || !hasAudit {
		t.Fatalf("动作目录没有给出目标写操作清单，写调用为零的断言会失效：%v", operations)
	}
	return operations
}

// catalogTargetWriteOperations 从真实动作目录取出全部目标写操作路径，避免测试维护一份会漂移的副本。
func catalogTargetWriteOperations() []string {
	contexts := []model.ActionContext{
		{FlowSource: "new", InstanceStatus: "draft", IsInitiator: true, CurrentNodeType: "start"},
		{FlowSource: "pending", InstanceStatus: "run", CurrentNodeType: "audit", HasCurrentTask: true, HasEditableProxy: true,
			HasCompletedTask: true, PreviousTaskExists: true, CanSwitchActor: true, HasPendingRecipient: true, InstanceVisible: true},
		{FlowSource: "done", InstanceStatus: "rejected", IsInitiator: true, CurrentNodeType: "audit", CurrentTaskDone: true},
	}
	operations := map[string]bool{}
	for _, actionContext := range contexts {
		for _, item := range actioncatalog.Build(actionContext) {
			if strings.TrimSpace(item.TargetOperation) != "" {
				operations[item.TargetOperation] = true
			}
		}
	}
	result := make([]string, 0, len(operations))
	for operation := range operations {
		result = append(result, operation)
	}
	return result
}

// writeRecordingJSON 按目标网关协议返回响应正文。
func writeRecordingJSON(response http.ResponseWriter, value any) {
	response.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(response).Encode(value)
}
