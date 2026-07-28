package integration_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/api"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/service"
)

type fakeTarget struct {
	t             *testing.T
	password      string
	loginCode     string
	mu            sync.Mutex
	loginCount    int
	templateCount int
	expireMode    string
	sessions      []string
	graphCalls    []string
}

func newFakeTarget(t *testing.T) *fakeTarget {
	t.Helper()
	return &fakeTarget{t: t, password: runtimeValue(t, 12), loginCode: runtimeValue(t, 6)}
}

func (f *fakeTarget) handler(response http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case "/web/user/api/login/user/login":
		f.handleLogin(response, request)
	case "/web/flowTemplateApi/list":
		f.handleTemplates(response, request)
	case "/web/flowInstanceApi/list":
		body := f.requireSession(request)
		data, _ := body["data"].(map[string]any)
		if ids, ok := body["ids"].([]any); ok {
			if len(ids) != 1 || ids[0] != "submitted-id" || data["useScope"] != "invest" {
				f.t.Error("已发实例精确核对参数不正确")
			}
			f.recordGraphCall("submitted-list")
			writeTargetJSON(response, map[string]any{"isSuccess": true, "data": []any{map[string]any{"id": "submitted-id", "flowProxyId": "proxy-submitted"}}})
			return
		}
		if data["useScope"] != "invest" || data["name"] != "sent" || body["pagination"] != true {
			f.t.Error("已发流程请求参数不符合已核实协议")
		}
		writeTargetJSON(response, map[string]any{
			"isSuccess": true,
			"data": []any{
				map[string]any{"id": "submitted-run", "name": "真实已发流程", "formName": "备用标题", "status": "run", "createDate": "2026-07-27 10:00", "currentNodeName": "部门审批", "currentAuditUserInfo": map[string]any{"node-a": map[string]any{"userList": []any{map[string]any{"name": "处理人甲"}}}}},
				map[string]any{"id": "submitted-end", "name": "已完结流程", "status": "end"},
				map[string]any{"id": "submitted-rejected", "name": "已驳回流程", "status": "rejected"},
				map[string]any{"id": "submitted-withdraw", "name": "已撤销流程", "status": "withdraw"},
				map[string]any{"id": "submitted-await", "name": "待发流程", "status": "await_sent"},
				map[string]any{"id": "submitted-termination", "name": "终止流程", "status": "termination"},
				map[string]any{"id": "submitted-abandon", "name": "丢弃流程", "status": "abandon"},
				map[string]any{"id": "submitted-draft", "name": "草稿流程", "status": "draft"},
			},
			"total": 8, "pages": 1, "current": 1, "size": 20,
		})
	case "/web/flowJobTaskLink/list":
		body := f.requireSession(request)
		data, _ := body["data"].(map[string]any)
		if instanceID, ok := data["flowInstanceId"].(string); ok {
			if instanceID != "due-id" || data["taskStatus"] != "waiting_send" || data["useScope"] != "invest" {
				f.t.Error("待发实例精确核对参数不正确")
			}
			f.recordGraphCall("due-list")
			writeTargetJSON(response, map[string]any{"isSuccess": true, "data": []any{map[string]any{"flowInstanceId": "due-id", "flowProxyId": "proxy-due"}}})
			return
		}
		if data["taskStatus"] != "waiting_send" || data["useScope"] != "invest" || data["flowInstanceName"] != "due" || body["pagination"] != true {
			f.t.Error("待发流程请求参数不符合已核实协议")
		}
		writeTargetJSON(response, map[string]any{
			"isSuccess": true,
			"data": []any{map[string]any{
				"flowInstanceId": "due-id", "flowInstanceName": "真实待发流程", "formName": "备用标题",
				"flowStatus": "draft", "initiator": "发起人甲", "initiatorDate": "2026-07-27 09:00",
			}},
			"total": 1, "pages": 1, "current": 1, "size": 20,
		})
	case "/web/flowTemplateApi/findById":
		f.handleFlowDetail(response, request, "template-id", "template-detail")
	case "/web/flowProxy/findById":
		f.handleFlowDetail(response, request, "", "proxy-detail")
	default:
		http.NotFound(response, request)
	}
}

func (f *fakeTarget) recordGraphCall(value string) {
	f.mu.Lock()
	f.graphCalls = append(f.graphCalls, value)
	f.mu.Unlock()
}

func (f *fakeTarget) handleFlowDetail(response http.ResponseWriter, request *http.Request, expectedID, callName string) {
	body := f.requireSession(request)
	data, _ := body["data"].(map[string]any)
	id, _ := data["id"].(string)
	if expectedID != "" && id != expectedID {
		f.t.Error("模板详情没有使用保存的模板 ID")
	}
	if expectedID == "" && id != "proxy-submitted" && id != "proxy-due" {
		f.t.Errorf("实例详情错误地使用了实例 ID：%s", id)
	}
	f.recordGraphCall(callName + ":" + id)
	writeTargetJSON(response, map[string]any{
		"isSuccess": true,
		"data": map[string]any{"flowNodeTemplate": map[string]any{
			"id": "start", "nodeName": "发起", "type": "start",
			"childFlowNodeTemplate": map[string]any{"id": "end", "nodeName": "结束", "type": "end"},
		}},
	})
}

func (f *fakeTarget) handleLogin(response http.ResponseWriter, request *http.Request) {
	var body struct {
		Data struct {
			LoginType    string `json:"loginType"`
			Account      string `json:"account"`
			Password     string `json:"password"`
			PlatformCode string `json:"platformCode"`
			CustomerCode string `json:"customerCode"`
			Code         string `json:"code"`
		} `json:"data"`
	}
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		f.t.Error("登录请求体无法解析")
	}
	if body.Data.LoginType != "ACCOUNT" || body.Data.Account == "" || body.Data.Password == "" || body.Data.Password == f.password {
		f.t.Error("登录请求结构或密码加密不符合协议")
	}
	if body.Data.PlatformCode != "200001" || body.Data.Code != f.loginCode {
		f.t.Error("登录请求未采用运行时 code 或已批准平台代码")
	}
	if f.expireMode == "login-rejected" {
		writeTargetJSON(response, map[string]any{"isSuccess": false, "code": "LOGIN_REJECTED", "message": "login rejected"})
		return
	}
	if f.expireMode == "login-http-unauthorized" {
		response.WriteHeader(http.StatusUnauthorized)
		return
	}
	f.mu.Lock()
	f.loginCount++
	active := runtimeValue(f.t, 16)
	f.sessions = append(f.sessions, active)
	f.mu.Unlock()
	writeTargetJSON(response, map[string]any{
		"isSuccess": true,
		"sid":       active,
		"data": map[string]any{
			"user":      map[string]any{"name": "测试人员", "customerCode": "tenant-code"},
			"companyVo": map[string]any{"name": "测试公司"},
		},
	})
}

func (f *fakeTarget) handleTemplates(response http.ResponseWriter, request *http.Request) {
	body := f.requireSession(request)
	data, _ := body["data"].(map[string]any)
	_, hasFlowName := data["flowName"].(string)
	templateID, hasTemplateID := data["id"].(string)
	if (!hasFlowName && !hasTemplateID) || data["useScope"] != "invest" || body["platformCode"] != "200001,999999" || body["pagination"] != true || body["ignoreFormTemplateBizRelevanceData"] != true {
		f.t.Error("流程模板请求参数不符合已核实协议")
	}
	if hasTemplateID {
		if templateID != "template-id" {
			f.t.Error("模板列表没有使用保存的模板 ID 精确核对")
		}
		f.recordGraphCall("template-list")
	}
	f.mu.Lock()
	f.templateCount++
	call := f.templateCount
	mode := f.expireMode
	f.mu.Unlock()
	if mode == "business-once" && call == 1 || mode == "business-always" {
		writeTargetJSON(response, map[string]any{"isSuccess": false, "code": "RESP401", "message": "session invalid"})
		return
	}
	if mode == "business-message-once" && call == 1 {
		writeTargetJSON(response, map[string]any{"isSuccess": false, "code": "ERROR_99999", "message": "用户会话已失效"})
		return
	}
	if mode == "business-minus-one-once" && call == 1 {
		writeTargetJSON(response, map[string]any{"isSuccess": false, "code": "-1", "message": "session invalid"})
		return
	}
	if mode == "http-once" && call == 1 || mode == "http-always" {
		response.WriteHeader(http.StatusUnauthorized)
		return
	}
	if mode == "empty" {
		writeTargetJSON(response, map[string]any{"isSuccess": true, "data": []any{}, "total": 0, "pages": 0, "current": 1, "size": 20})
		return
	}
	if mode == "invalid-json" {
		_, _ = response.Write([]byte("{"))
		return
	}
	if mode == "bad-pagination" {
		writeTargetJSON(response, map[string]any{"isSuccess": true, "data": []any{}, "total": -1})
		return
	}
	if mode == "unavailable" {
		response.WriteHeader(http.StatusInternalServerError)
		return
	}
	writeTargetJSON(response, map[string]any{
		"isSuccess": true,
		"data": []any{map[string]any{
			"id": "template-id", "flowName": "真实流程模板", "code": "FLOW-CODE", "groupName": "业务流程",
			"flowStatus": "enable", "typeName": "经营管理", "updateDate": "2026-07-27 08:00",
			"remark": "用于验证采购审批", "formExist": "withForm",
			"formTemplateList": []any{map[string]any{"id": "form-a"}, map[string]any{"id": "form-b"}},
		}},
		"total": 1, "pages": 1, "current": 1, "size": 20,
	})
}

func TestFlowTreeReadUsesExactSourceLookupBeforeDetails(t *testing.T) {
	tests := []struct {
		name   string
		source string
		id     string
		calls  []string
	}{
		{name: "新发起模板树", source: "new", id: "template-id", calls: []string{"template-list", "template-detail:template-id"}},
		{name: "已发实例代理树", source: "started", id: "submitted-id", calls: []string{"submitted-list", "proxy-detail:proxy-submitted"}},
		{name: "待发实例代理树", source: "pending", id: "due-id", calls: []string{"due-list", "proxy-detail:proxy-due"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := newFakeTarget(t)
			targetServer := httptest.NewServer(http.HandlerFunc(fake.handler))
			defer targetServer.Close()
			configureTargetEnv(t, targetServer.URL, fake.password, fake.loginCode, "2s")
			reader := service.NewTargetReadService(config.LoadTargetConfig())
			tree, err := reader.FlowTree(context.Background(), "account-a", test.source, test.id)
			if err != nil || tree == nil || tree.ID != "start" || tree.Child == nil || tree.Child.ID != "end" {
				t.Fatalf("真实流程树读取失败：%v", err)
			}
			fake.mu.Lock()
			calls := append([]string(nil), fake.graphCalls...)
			fake.mu.Unlock()
			if strings.Join(calls, ",") != strings.Join(test.calls, ",") {
				t.Fatalf("核对与详情请求顺序不正确：%v", calls)
			}
		})
	}
}

func (f *fakeTarget) requireSession(request *http.Request) map[string]any {
	f.mu.Lock()
	if len(f.sessions) == 0 {
		f.mu.Unlock()
		f.t.Error("业务请求发生在登录前")
		return nil
	}
	active := f.sessions[len(f.sessions)-1]
	f.mu.Unlock()
	var body map[string]any
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		f.t.Error("业务请求体无法解析")
		return nil
	}
	if request.URL.Query().Get("sid") != active || request.Header.Get("sid") != active || body["sid"] != active {
		f.t.Error("SID 未按 body、query、header 三处协议传递")
	}
	if request.URL.Query().Get("platformCode") != "200001" {
		f.t.Error("业务请求缺少平台代码")
	}
	return body
}

func TestRealReadProtocolAndThreeSourceMappings(t *testing.T) {
	fake := newFakeTarget(t)
	targetServer := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer targetServer.Close()
	configureTargetEnv(t, targetServer.URL, fake.password, fake.loginCode, "2s")
	app := api.NewHandler()

	responses := [][]byte{
		callApp(t, app, http.MethodPost, "/api/target/accounts/verify", `{"account":"account-a"}`, http.StatusOK),
		callApp(t, app, http.MethodGet, "/api/target/flow-templates?account=account-a&query=flow&page=1&pageSize=20", "", http.StatusOK),
		callApp(t, app, http.MethodGet, "/api/target/flow-instances?account=account-a&source=submitted&query=sent&page=1&pageSize=20", "", http.StatusOK),
		callApp(t, app, http.MethodGet, "/api/target/flow-instances?account=account-a&source=due&query=due&page=1&pageSize=20", "", http.StatusOK),
	}
	wants := []string{"displayName", "formTemplateCount\":2", "审批中", "真实待发流程"}
	for index, body := range responses {
		if !bytes.Contains(body, []byte(wants[index])) {
			t.Fatalf("第 %d 个真实读取响应缺少预期映射", index+1)
		}
		for _, active := range fake.sessions {
			if bytes.Contains(body, []byte(active)) {
				t.Fatal("公开响应泄露后端会话")
			}
		}
	}
	if fake.loginCount != 1 {
		t.Fatalf("三类读取未复用同一账号会话，登录次数 = %d", fake.loginCount)
	}
	for _, statusName := range []string{"待发", "审批中", "撤销", "终止", "丢弃", "驳回", "完结", "草稿"} {
		if !bytes.Contains(responses[2], []byte(statusName)) {
			t.Fatalf("已发流程响应缺少中文状态 %s", statusName)
		}
	}
}

func TestSessionExpiryRelogsAndReplaysOnce(t *testing.T) {
	for _, mode := range []string{"business-once", "business-message-once", "business-minus-one-once", "http-once"} {
		t.Run(mode, func(t *testing.T) {
			fake := newFakeTarget(t)
			fake.expireMode = mode
			targetServer := httptest.NewServer(http.HandlerFunc(fake.handler))
			defer targetServer.Close()
			configureTargetEnv(t, targetServer.URL, fake.password, fake.loginCode, "2s")
			body := callApp(t, api.NewHandler(), http.MethodGet, "/api/target/flow-templates?account=account-a", "", http.StatusOK)
			if !bytes.Contains(body, []byte("真实流程模板")) || fake.loginCount != 2 || fake.templateCount != 2 {
				t.Fatal("会话失效后未完成一次安全重登和只读重放")
			}
		})
	}
}

func TestLoginRejectionUsesStablePublicError(t *testing.T) {
	for _, mode := range []string{"login-rejected", "login-http-unauthorized"} {
		t.Run(mode, func(t *testing.T) {
			fake := newFakeTarget(t)
			fake.expireMode = mode
			targetServer := httptest.NewServer(http.HandlerFunc(fake.handler))
			defer targetServer.Close()
			configureTargetEnv(t, targetServer.URL, fake.password, fake.loginCode, "2s")
			body := callApp(t, api.NewHandler(), http.MethodPost, "/api/target/accounts/verify", `{"account":"account-a"}`, http.StatusUnauthorized)
			if !bytes.Contains(body, []byte("TARGET_LOGIN_REJECTED")) {
				t.Fatal("目标登录拒绝未映射为稳定公开错误")
			}
		})
	}
}

func TestSessionExpiryStopsAfterOneReplay(t *testing.T) {
	for _, mode := range []string{"business-always", "http-always"} {
		t.Run(mode, func(t *testing.T) {
			fake := newFakeTarget(t)
			fake.expireMode = mode
			targetServer := httptest.NewServer(http.HandlerFunc(fake.handler))
			defer targetServer.Close()
			configureTargetEnv(t, targetServer.URL, fake.password, fake.loginCode, "2s")
			body := callApp(t, api.NewHandler(), http.MethodGet, "/api/target/flow-templates?account=account-a", "", http.StatusUnauthorized)
			if !bytes.Contains(body, []byte("TARGET_SESSION_EXPIRED")) || fake.loginCount != 2 || fake.templateCount != 2 {
				t.Fatal("会话重登或重放超过一次边界")
			}
		})
	}
}

func TestEmptyBadPaginationBadJSONAndTargetUnavailable(t *testing.T) {
	tests := []struct {
		mode   string
		status int
		code   string
	}{
		{mode: "empty", status: http.StatusOK, code: `"items":[]`},
		{mode: "invalid-json", status: http.StatusBadGateway, code: "TARGET_RESPONSE_INVALID"},
		{mode: "bad-pagination", status: http.StatusBadGateway, code: "TARGET_RESPONSE_INVALID"},
		{mode: "unavailable", status: http.StatusBadGateway, code: "TARGET_UNAVAILABLE"},
	}
	for _, test := range tests {
		t.Run(test.mode, func(t *testing.T) {
			fake := newFakeTarget(t)
			fake.expireMode = test.mode
			targetServer := httptest.NewServer(http.HandlerFunc(fake.handler))
			defer targetServer.Close()
			configureTargetEnv(t, targetServer.URL, fake.password, fake.loginCode, "2s")
			body := callApp(t, api.NewHandler(), http.MethodGet, "/api/target/flow-templates?account=account-a", "", test.status)
			if !bytes.Contains(body, []byte(test.code)) {
				t.Fatal("目标异常未映射为预期稳定结果")
			}
		})
	}
}

func TestTargetTimeoutAndContextCancellation(t *testing.T) {
	started := make(chan struct{})
	var startOnce sync.Once
	targetServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		startOnce.Do(func() { close(started) })
		select {
		case <-request.Context().Done():
		case <-time.After(100 * time.Millisecond):
		}
	}))
	defer targetServer.Close()
	password := runtimeValue(t, 12)
	loginCode := runtimeValue(t, 6)
	configureTargetEnv(t, targetServer.URL, password, loginCode, "20ms")
	body := callApp(t, api.NewHandler(), http.MethodPost, "/api/target/accounts/verify", `{"account":"account-a"}`, http.StatusGatewayTimeout)
	if !bytes.Contains(body, []byte("TARGET_TIMEOUT")) {
		t.Fatal("目标超时未返回稳定错误")
	}
	<-started

	client, err := target.NewClient(target.ClientConfig{
		BaseURL: targetServer.URL, LoginPassword: password, LoginAESKey: runtimeValue(t, 8),
		LoginCode: loginCode, PlatformCode: "200001", TemplatePlatformCodes: "200001,999999", Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("创建目标客户端失败：%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Login(ctx, "account-a"); !errors.Is(err, context.Canceled) {
		t.Fatal("目标请求未透传 context 取消")
	}
}

func configureTargetEnv(t *testing.T, baseURL, password, loginCode, timeout string) {
	t.Helper()
	t.Setenv("TARGET_API_GATEWAY", baseURL)
	t.Setenv("TARGET_LOGIN_PASSWORD", password)
	t.Setenv("TARGET_LOGIN_AES_KEY", runtimeValue(t, 8))
	t.Setenv("TARGET_LOGIN_CODE", loginCode)
	t.Setenv("TARGET_PLATFORM_CODE", "")
	t.Setenv("TARGET_TEMPLATE_PLATFORM_CODES", "")
	t.Setenv("TARGET_CUSTOMER_CODE", "")
	t.Setenv("TARGET_SESSION_TTL", "1h")
	t.Setenv("TARGET_HTTP_TIMEOUT", timeout)
}

func callApp(t *testing.T, handler http.Handler, method, path, body string, expectedStatus int) []byte {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != expectedStatus {
		t.Fatalf("状态码 = %d，期望 %d", recorder.Code, expectedStatus)
	}
	responseBody, err := io.ReadAll(recorder.Result().Body)
	if err != nil {
		t.Fatal("读取应用响应失败")
	}
	return responseBody
}

func runtimeValue(t *testing.T, byteCount int) string {
	t.Helper()
	data := make([]byte, byteCount)
	if _, err := rand.Read(data); err != nil {
		t.Fatal("无法生成测试期临时值")
	}
	return hex.EncodeToString(data)
}

func writeTargetJSON(response http.ResponseWriter, value any) {
	response.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(response).Encode(value)
}
