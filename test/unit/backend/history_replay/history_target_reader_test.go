package history_replay_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/service"
)

type targetHistoryFixture struct {
	t             *testing.T
	mu            sync.Mutex
	noForm        bool
	expireFirst   bool
	automationRow bool
	loginCount    int
	listCount     int
	paths         []string
	listBodies    []map[string]any
	raw           map[string]any
	formName      string
	flowName      string
	flowCode      string
	userID        string
	companyID     string
}

// newTargetHistoryFixture 创建仅实现目标只读历史协议的假网关。
func newTargetHistoryFixture(t *testing.T, noForm bool) *targetHistoryFixture {
	t.Helper()
	return &targetHistoryFixture{
		t: t, noForm: noForm, flowCode: "expense-flow", flowName: "费用审批", formName: "费用单（测试公司）",
		userID: "user-current", companyID: "company-current",
		raw: map[string]any{"nested": map[string]any{"rows": []any{map[string]any{"amount": json.Number("128.00"), "custom": "kept"}}}},
	}
}

// handler 响应登录、实例列表、实例原始数据和代理运行时详情，其他路径一律 404。
func (f *targetHistoryFixture) handler(response http.ResponseWriter, request *http.Request) {
	f.mu.Lock()
	f.paths = append(f.paths, request.URL.Path)
	f.mu.Unlock()
	switch request.URL.Path {
	case "/web/user/api/login/user/login":
		f.mu.Lock()
		f.loginCount++
		loginCount := f.loginCount
		f.mu.Unlock()
		writeHistoryTargetJSON(response, map[string]any{
			"isSuccess": true, "sid": "sid-private-" + string(rune('0'+loginCount)),
			"data": map[string]any{
				"user":      map[string]any{"id": f.userID, "name": "当前用户", "customerCode": "customer", "departmentId": "department"},
				"companyVo": map[string]any{"id": f.companyID, "name": "测试公司", "customerCode": "customer"},
			},
		})
	case "/web/flowInstanceApi/list":
		body := decodeHistoryTargetBody(f.t, request)
		f.mu.Lock()
		f.listCount++
		listCount := f.listCount
		f.listBodies = append(f.listBodies, body)
		f.mu.Unlock()
		if f.expireFirst && listCount == 1 {
			response.WriteHeader(http.StatusUnauthorized)
			return
		}
		data, _ := body["data"].(map[string]any)
		writeHistoryTargetJSON(response, f.historyListResponse(stringSlice(data["statusList"])))
	case "/web/flowInstanceApi/getCurrentFromData":
		body := decodeHistoryTargetBody(f.t, request)
		data, _ := body["data"].(map[string]any)
		if strings.TrimSpace(stringValue(data["id"])) == "" {
			f.t.Error("目标原始数据读取缺少实例 ID")
		}
		writeHistoryTargetJSON(response, map[string]any{"isSuccess": true, "data": map[string]any{"data": f.raw}})
	case "/web/flowProxy/findById":
		formExist := "form"
		forms := []any{map[string]any{"id": "form-proxy-end", "name": f.formName}}
		auditWay := "formmaking"
		if f.noForm {
			formExist, forms, auditWay = "noForm", []any{}, "NoFormFlow"
		}
		writeHistoryTargetJSON(response, map[string]any{"isSuccess": true, "data": map[string]any{
			"flowCode": f.flowCode, "flowName": f.flowName, "auditWay": auditWay, "formExist": formExist,
			"flowNodeTemplate": map[string]any{"id": "start", "name": "发起", "type": "start"},
			"formTemplateList": forms,
		}})
	case "/web/formProxy/findById":
		if f.noForm {
			f.t.Error("NoFormFlow 不应读取或构造 FormMaking 表单代理")
		}
		writeHistoryTargetJSON(response, map[string]any{"isSuccess": true, "data": map[string]any{
			"id": "form-proxy-end", "name": f.formName, "fieldsTemplateList": []any{},
			"templateData": `{"list":[{"type":"input","model":"amount","name":"金额"}]}`,
		}})
	default:
		http.NotFound(response, request)
	}
}

// historyListResponse 按目标 statusList 分组返回原始列表，并保留真实环境事实：
// 实例列表行大多不携带 flowCode，只能由目标返回的表单/流程名称构成身份。
func (f *targetHistoryFixture) historyListResponse(statuses []string) map[string]any {
	rows := f.historyListRows()
	matched := make([]any, 0, len(rows))
	for _, row := range rows {
		status := stringValue(row["status"])
		for _, value := range statuses {
			if value == status {
				matched = append(matched, row)
				break
			}
		}
	}
	return map[string]any{"isSuccess": true, "total": len(matched), "pages": 1, "current": 1, "size": 100, "data": matched}
}

// historyListRows 返回该流程下全部原始行，含应被精确过滤掉的其他表单和其他流程行。
func (f *targetHistoryFixture) historyListRows() []map[string]any {
	if f.noForm {
		return []map[string]any{
			{"id": "noform-end", "flowProxyId": "proxy-noform", "flowName": f.flowName, "formName": "", "name": "请款业务数据", "status": "end", "createDate": "2026-08-31 12:00:00", "createrId": f.userID, "companyId": f.companyID},
			{"id": "noform-mismatch", "flowProxyId": "proxy-other", "flowName": "其他页面", "formName": "", "name": "不匹配页面", "status": "end", "createDate": "2026-08-30 12:00:00", "createrId": f.userID, "companyId": f.companyID},
			{"id": "noform-missing-name", "flowProxyId": "proxy-missing", "flowName": "", "formName": "", "name": "名称证据缺失", "status": "run", "createTime": "2026-08-29 12:00:00", "createrId": f.userID, "companyId": f.companyID},
		}
	}
	rows := []map[string]any{
		{"id": "form-run", "flowProxyId": "proxy-run", "formProxyId": "form-proxy-run", "flowName": f.flowName, "formName": f.formName, "name": "运行中数据", "status": "run", "createDate": "2026-09-01 09:00:00", "createrId": f.userID, "companyId": f.companyID},
		{"id": "form-end", "flowProxyId": "proxy-end", "formProxyId": "form-proxy-end", "flowName": f.flowName, "formName": f.formName, "name": "已完成数据", "status": "end", "createDate": "2026-08-31 10:00:00", "createrId": f.userID, "companyId": f.companyID},
		{"id": "wrong-form", "flowProxyId": "proxy-wrong-form", "flowName": f.flowName, "formName": "费用单（其他公司）", "name": "错误表单", "status": "end", "createDate": "2026-08-30 10:00:00", "createrId": f.userID, "companyId": f.companyID},
		{"id": "wrong-flow", "flowProxyId": "proxy-wrong-flow", "flowCode": "other-flow", "flowName": "其他流程", "formName": f.formName, "name": "错误流程", "status": "end", "createDate": "2026-08-29 10:00:00", "createrId": f.userID, "companyId": f.companyID},
	}
	if f.automationRow {
		// 旧版本自动回归产生的实例：结构与当前目标表单不一致，必须被剔除。
		rows = append(rows, map[string]any{"id": "auto-regression", "flowProxyId": "proxy-auto", "formProxyId": "form-proxy-auto",
			"flowName": f.flowName, "formName": f.formName, "name": f.flowName + "-自动回归-分支01-动作链", "status": "end",
			"createDate": "2026-09-02 10:00:00", "createrId": f.userID, "companyId": f.companyID})
	}
	return rows
}

// newTargetHistoryReader 创建真实目标客户端与会话管理器，测试重登和读取边界。
func newTargetHistoryReader(t *testing.T, fixture *targetHistoryFixture) (*service.TargetReadService, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(fixture.handler))
	client, err := target.NewClient(target.ClientConfig{
		BaseURL: server.URL, LoginPassword: "password", LoginAESKey: "0123456789abcdef", LoginCode: "code",
		PlatformCode: "invest", CustomerCode: "customer", Timeout: 2 * time.Second,
	})
	if err != nil {
		server.Close()
		t.Fatalf("创建目标历史测试客户端失败：%v", err)
	}
	return service.NewTargetReadServiceWithClient(client, time.Hour), server
}

// TestTargetHistoryReaderFiltersSortsRelogsAndPreservesRawData 验证目标原字段过滤、完成优先、一次重登和正文透传。
func TestTargetHistoryReaderFiltersSortsRelogsAndPreservesRawData(t *testing.T) {
	fixture := newTargetHistoryFixture(t, false)
	fixture.expireFirst = true
	reader, server := newTargetHistoryReader(t, fixture)
	defer server.Close()
	page, err := reader.HistoryCandidates(context.Background(), "account-a", fixture.flowCode, fixture.formName, fixture.flowName, "", 1, 20)
	if err != nil {
		t.Fatalf("读取 FormMaking 历史候选失败：%v", err)
	}
	if len(page.Items) != 2 || page.Items[0].ID != "form-end" || page.Items[1].ID != "form-run" {
		t.Fatalf("目标精确过滤或完成优先排序错误：%+v", page.Items)
	}
	fixture.mu.Lock()
	loginCount := fixture.loginCount
	fixture.mu.Unlock()
	if loginCount != 2 {
		t.Fatalf("会话过期没有执行一次受控重登：loginCount=%d", loginCount)
	}
	key := service.HistoryCandidateKey("account-a", page.Items[0])
	snapshot, err := reader.ReadHistorySnapshot(context.Background(), "account-a", fixture.flowCode, fixture.formName, fixture.flowName, key)
	if err != nil {
		t.Fatalf("读取 FormMaking 原始历史数据失败：%v", err)
	}
	if snapshot.RenderType != target.FormRenderTypeFormMaking || !reflect.DeepEqual(snapshot.RawFormData, fixture.raw) {
		t.Fatalf("FormMaking 原始数据或 runtime 类型发生变化：%+v", snapshot)
	}
	if _, exists := snapshot.TemplateSummary["vuePage"]; exists || stringValue(snapshot.TemplateSummary["runtimeVersionDigest"]) == "" {
		t.Fatalf("历史模板摘要依赖旧 VuePage 或缺少目标原文版本：%+v", snapshot.TemplateSummary)
	}
	assertTargetHistoryReadOnlyCalls(t, fixture)
}

// TestTargetNoFormHistoryUsesRawIdentityWithoutVuePageMapping 验证 NoFormFlow 只用目标流程/页面原字段并保留自定义页面数据。
func TestTargetNoFormHistoryUsesRawIdentityWithoutVuePageMapping(t *testing.T) {
	fixture := newTargetHistoryFixture(t, true)
	fixture.flowName = "NoFormFlow 请款页"
	reader, server := newTargetHistoryReader(t, fixture)
	defer server.Close()
	page, err := reader.HistoryCandidates(context.Background(), "account-a", fixture.flowCode, "", fixture.flowName, "", 1, 20)
	if err != nil {
		t.Fatalf("读取 NoFormFlow 历史候选失败：%v", err)
	}
	if len(page.Items) != 2 || page.Items[0].ID != "noform-end" || page.Items[1].ID != "noform-missing-name" {
		t.Fatalf("NoFormFlow 没有按目标页面名称过滤或保留证据缺失候选：%+v", page.Items)
	}
	if page.Items[1].CreatedAt != "2026-08-29 12:00:00" {
		t.Fatalf("历史候选没有兼容目标返回的 createTime 原字段：%+v", page.Items[1])
	}
	key := service.HistoryCandidateKey("account-a", page.Items[0])
	snapshot, err := reader.ReadHistorySnapshot(context.Background(), "account-a", fixture.flowCode, "", fixture.flowName, key)
	if err != nil {
		t.Fatalf("读取 NoFormFlow 原始历史数据失败：%v", err)
	}
	if snapshot.RenderType != target.FormRenderTypeVueCustom || !reflect.DeepEqual(snapshot.RawFormData, fixture.raw) {
		t.Fatalf("NoFormFlow 原始数据没有按既有 runtime 协议透传：%+v", snapshot)
	}
	if _, exists := snapshot.TemplateSummary["vuePage"]; exists || stringValue(snapshot.TemplateSummary["pageKey"]) != "NoFormFlow" {
		t.Fatalf("NoFormFlow 摘要使用了旧页面映射或丢失目标 pageKey：%+v", snapshot.TemplateSummary)
	}
	assertTargetHistoryReadOnlyCalls(t, fixture)
}

// assertTargetHistoryReadOnlyCalls 核对 T02 只访问已批准的目标读取端点与全状态原始过滤字段。
func assertTargetHistoryReadOnlyCalls(t *testing.T, fixture *targetHistoryFixture) {
	t.Helper()
	fixture.mu.Lock()
	paths := append([]string(nil), fixture.paths...)
	bodies := append([]map[string]any(nil), fixture.listBodies...)
	fixture.mu.Unlock()
	allowed := map[string]bool{
		"/web/user/api/login/user/login": true, "/web/flowInstanceApi/list": true,
		"/web/flowInstanceApi/getCurrentFromData": true, "/web/flowProxy/findById": true,
		"/web/formProxy/findById": true,
	}
	for _, path := range paths {
		if !allowed[path] {
			t.Fatalf("历史链路调用了未批准或写入端点：%s", path)
		}
	}
	covered := map[string]bool{}
	for _, body := range bodies {
		data, _ := body["data"].(map[string]any)
		// 实例列表按目标自己使用的 flowName 过滤；flowCode 在实例行上不可靠，不能作为查询字段。
		if stringValue(data["flowName"]) != fixture.flowName || data["flowCode"] != nil || data["name"] != nil {
			t.Fatalf("业务数据候选没有只用目标 flowName 原字段：%+v", data)
		}
		for _, status := range stringSlice(data["statusList"]) {
			covered[status] = true
		}
	}
	statuses := make([]string, 0, len(covered))
	for status := range covered {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)
	expected := []string{"abandon", "await_sent", "draft", "end", "rejected", "run", "termination", "withdraw"}
	if !reflect.DeepEqual(statuses, expected) {
		t.Fatalf("业务数据候选没有覆盖目标全部实例状态：%v", statuses)
	}
}

// decodeHistoryTargetBody 解析假目标请求并保留数字和数组原结构。
func decodeHistoryTargetBody(t *testing.T, request *http.Request) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		t.Fatalf("解析目标历史请求失败：%v", err)
	}
	return body
}

// writeHistoryTargetJSON 输出目标网关成功包络。
func writeHistoryTargetJSON(response http.ResponseWriter, value any) {
	response.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(response).Encode(value)
}

// stringValue 读取假目标请求或响应中的字符串字段。
func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

// stringSlice 把 JSON 解码后的状态数组转换为字符串列表。
func stringSlice(value any) []string {
	raw, _ := value.([]any)
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if text := stringValue(item); text != "" {
			result = append(result, text)
		}
	}
	return result
}
