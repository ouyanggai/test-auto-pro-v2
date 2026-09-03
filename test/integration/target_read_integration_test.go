package integration_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/api"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

type fakeTarget struct {
	t               *testing.T
	password        string
	loginCode       string
	mu              sync.Mutex
	loginCount      int
	templateCount   int
	formCount       int
	expireMode      string
	sessions        []string
	graphCalls      []string
	submittedStatus string
	duePaged        bool
	dueUnbounded    bool
	formFields      []any
	templateData    string
	directoryAudit  bool
	directoryFail   bool
	directoryFlags  []string
}

// newFakeTarget 创建不含固定凭证的假目标服务状态。
func newFakeTarget(t *testing.T) *fakeTarget {
	t.Helper()
	return &fakeTarget{t: t, password: runtimeValue(t, 12), loginCode: runtimeValue(t, 6), submittedStatus: "run"}
}

// handler 按已核实只读协议响应登录、列表和流程树请求。
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
			writeTargetJSON(response, map[string]any{"isSuccess": true, "data": []any{map[string]any{
				"id": "submitted-id", "flowProxyId": "proxy-submitted", "formProxyId": "form-proxy-submitted", "status": f.submittedStatus, "currentNodeProxyId": "end",
				"currentAuditUserInfo": map[string]any{"start": map[string]any{"userList": []any{}}},
			}}})
			return
		}
		name, _ := data["name"].(string)
		if data["useScope"] != "invest" || (name != "" && name != "sent" && name != "flow-test") || body["pagination"] != true {
			f.t.Error("已发流程请求参数不符合已核实协议")
		}
		writeTargetJSON(response, map[string]any{
			"isSuccess": true,
			"data": []any{
				map[string]any{"id": "submitted-run", "name": "真实已发流程", "flowCode": "flow-test", "formName": "备用标题", "status": "run", "createDate": "2026-07-27 10:00", "currentNodeName": "部门审批", "currentAuditUserInfo": map[string]any{"node-a": map[string]any{"userList": []any{map[string]any{"name": "处理人甲"}}}}},
				map[string]any{"id": "submitted-end", "name": "已完结流程", "flowCode": "flow-test", "status": "end"},
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
			if f.dueUnbounded {
				items := make([]any, 0, 100)
				for index := 0; index < 100; index++ {
					items = append(items, map[string]any{
						"flowInstanceId": "due-id", "flowProxyId": "proxy-due", "formProxyId": "form-proxy-due",
						"flowNodeProxyId": fmt.Sprintf("node-%d", index),
					})
				}
				writeTargetJSON(response, map[string]any{"isSuccess": true, "data": items, "current": body["pages"], "size": 100})
				return
			}
			if f.duePaged {
				page, _ := body["pages"].(float64)
				entryID := "start"
				if page == 2 {
					entryID = "end"
				}
				writeTargetJSON(response, map[string]any{
					"isSuccess": true, "pages": 2, "current": int(page), "size": 100,
					"data": []any{map[string]any{"flowInstanceId": "due-id", "flowProxyId": "proxy-due", "formProxyId": "form-proxy-due", "flowNodeProxyId": entryID}},
				})
				return
			}
			writeTargetJSON(response, map[string]any{"isSuccess": true, "data": []any{
				map[string]any{"flowInstanceId": "due-id", "flowProxyId": "proxy-due", "formProxyId": "form-proxy-due", "flowNodeProxyId": "start"},
				map[string]any{"flowInstanceId": "due-id", "flowProxyId": "proxy-due", "formProxyId": "form-proxy-due", "flowNodeProxyId": "end"},
			}})
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
	case "/web/formTemplateApi/findById":
		f.handleFormDetail(response, request, "form-template", "模板表单")
	case "/web/formProxy/findById":
		f.handleFormDetail(response, request, "", "代理表单")
	case "/web/flowInstanceApi/getCurrentFromData":
		f.handleInstanceCurrentData(response, request)
	case "/web/user/api/user/findByCompanyIdUserList":
		f.handleDirectoryResponse(response, map[string]any{"dataList": []any{map[string]any{"id": "person-1", "realName": "张三"}}})
	case "/web/user/api/company/children":
		body := f.requireSession(request)
		data, _ := body["data"].(map[string]any)
		flag := fmt.Sprint(data["flag"])
		f.mu.Lock()
		f.directoryFlags = append(f.directoryFlags, flag)
		f.mu.Unlock()
		byFlag := map[string]any{
			"2": []any{map[string]any{"id": "department-1", "name": "财务部"}},
			"3": []any{map[string]any{"id": "company-1", "name": "测试公司", "type": "1", "parentId": "", "childrenList": []any{
				map[string]any{"id": "department-1", "name": "财务部", "type": "2", "parentId": "company-1", "childrenList": []any{
					map[string]any{"id": "person-2", "name": "李四", "type": "5", "parentId": "department-1"},
				}},
				map[string]any{"id": "person-3", "name": "王五", "type": "5", "parentId": "company-1"},
			}}},
			"4": []any{map[string]any{"id": "position-1", "name": "财务主任"}},
			"7": []any{map[string]any{"id": "company-1", "name": "测试公司"}},
		}
		f.handleDirectoryResponse(response, byFlag[flag])
	case "/web/user/api/dutyLevel/list":
		f.handleDirectoryResponse(response, []any{map[string]any{"id": "level-1", "name": "二级岗"}})
	case "/web/flowRoleApi/list":
		f.handleDirectoryResponse(response, []any{map[string]any{"id": "role-1", "name": "财务审批角色"}})
	case "/web/user/api/expandAttr/list":
		f.handleDirectoryResponse(response, []any{map[string]any{"id": "attr-1", "name": "项目负责人属性"}})
	case "/web/flowRoleUserApi/list":
		f.handleDirectoryResponse(response, []any{map[string]any{"userVo": map[string]any{"id": "person-4", "realName": "赵六"}}})
	case "/web/user/api/user/getUserVosByBizIds":
		f.handleDirectoryResponse(response, []any{map[string]any{"id": "person-5", "realName": "孙七"}})
	default:
		http.NotFound(response, request)
	}
}

// recordGraphCall 线程安全记录精确核对与详情调用顺序。
func (f *fakeTarget) recordGraphCall(value string) {
	f.mu.Lock()
	f.graphCalls = append(f.graphCalls, value)
	f.mu.Unlock()
}

// handleFlowDetail 验证模板或代理树详情只能使用已核实的真实 ID。
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
	auditConfig := map[string]any{
		"auditType": "form_person", "type": "scramble", "formPersonFields": "amount",
		"userVoList": []any{map[string]any{"id": "candidate-1", "realName": "候选人甲"}},
	}
	if f.directoryAudit {
		auditConfig = map[string]any{
			"auditType": "run_node_choose", "type": "countersign", "countersignNum": 2,
			"flowNodeDetailConfigList": []any{
				map[string]any{"bizId": "person-1", "auditDetailType": "personnel"},
				map[string]any{"bizId": "position-1", "auditDetailType": "position"},
				map[string]any{"bizId": "level-1", "auditDetailType": "level"},
				map[string]any{"bizId": "role-1", "auditDetailType": "role"},
				map[string]any{"bizId": "department-1", "auditDetailType": "department"},
				map[string]any{"bizId": "company-1", "auditDetailType": "company"},
				map[string]any{"bizId": "attr-1", "auditDetailType": "extendedAttribute"},
			},
			"nodeAuditScopeList": []any{
				map[string]any{"bizId": "role-1", "type": "role"},
				map[string]any{"bizId": "position-1", "type": "position"},
				map[string]any{"bizId": "department-1", "type": "department"},
				map[string]any{"bizId": "company-1", "type": "company"},
				map[string]any{"bizId": "person-1", "type": "personnel"},
			},
		}
	}
	detailData := map[string]any{"flowCode": "flow-test", "auditWay": "contract_review", "flowNodeTemplate": map[string]any{
		"id": "start", "nodeName": "发起", "type": "start",
		"childFlowNodeTemplate": map[string]any{
			"id": "approval", "nodeName": "审批", "type": "common", "isSkip": true,
			"flowNodeAuditConfig":            auditConfig,
			"flowNodeFieldPowerTemplateList": []any{map[string]any{"formTemplateId": "form-template", "formFieldTemplateId": "field-amount", "formFieldTemplateEnglishName": "amount", "fieldPower": "only_read"}},
			"childFlowNodeTemplate":          map[string]any{"id": "end", "nodeName": "结束", "type": "end"},
		},
	}}
	if expectedID != "" {
		detailData["formTemplateList"] = []any{map[string]any{"id": "form-template", "name": "模板表单"}}
	}
	writeTargetJSON(response, map[string]any{
		"isSuccess": true,
		"data":      detailData,
	})
}

// handleDirectoryResponse 返回固定只读目录或模拟稳定读取失败，不接收任何写语义。
func (f *fakeTarget) handleDirectoryResponse(response http.ResponseWriter, data any) {
	if f.directoryFail {
		writeTargetJSON(response, map[string]any{"isSuccess": false, "message": "directory unavailable"})
		return
	}
	writeTargetJSON(response, map[string]any{"isSuccess": true, "data": data})
}

// handleFormDetail 验证模板和代理表单详情使用已核实标识，并返回中文字段元数据。
func (f *fakeTarget) handleFormDetail(response http.ResponseWriter, request *http.Request, expectedID, formName string) {
	body := f.requireSession(request)
	data, _ := body["data"].(map[string]any)
	id, _ := data["id"].(string)
	if expectedID != "" && id != expectedID {
		f.t.Errorf("模板表单详情 ID 不正确：%s", id)
	}
	if expectedID == "" && id != "form-proxy-submitted" && id != "form-proxy-due" {
		f.t.Errorf("代理表单详情 ID 不正确：%s", id)
	}
	f.recordGraphCall("form-detail:" + id)
	f.mu.Lock()
	f.formCount++
	formCall := f.formCount
	mode := f.expireMode
	f.mu.Unlock()
	if mode == "form-session-once" && formCall == 1 {
		writeTargetJSON(response, map[string]any{"isSuccess": false, "code": "RESP401", "message": "session invalid"})
		return
	}
	fields := f.formFields
	if fields == nil {
		fields = []any{map[string]any{
			"id": "field-amount", "name": "申请金额", "englishName": "amount",
			"fieldType": "doubleType", "defaultValue": "1000", "valueOrigin": "fromUser", "fieldStatus": "enable",
		}}
	}
	templateData := f.templateData
	if templateData == "" {
		templateData = `{"list":[{"type":"number","model":"amount","name":"申请金额","options":{"defaultValue":1000,"required":true}}]}`
	}
	writeTargetJSON(response, map[string]any{"isSuccess": true, "data": map[string]any{
		"id": id, "name": formName,
		"fieldsTemplateList": fields,
		"templateData":       templateData,
	}})
}

// handleInstanceCurrentData 验证实例现值读取使用精确实例 ID 并返回 formDataMongoVo.data。
func (f *fakeTarget) handleInstanceCurrentData(response http.ResponseWriter, request *http.Request) {
	body := f.requireSession(request)
	data, _ := body["data"].(map[string]any)
	id, _ := data["id"].(string)
	if id != "submitted-id" && id != "due-id" && !strings.HasPrefix(id, "submitted-") {
		f.t.Errorf("实例现值读取错误地使用了非精确实例 ID：%s", id)
	}
	f.recordGraphCall("instance-data:" + id)
	writeTargetJSON(response, map[string]any{"isSuccess": true, "data": map[string]any{
		"data": map[string]any{"amount": 2500.5},
	}})
}

// handleLogin 验证登录结构并生成仅存在于测试运行期的随机会话。
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
			"user":      map[string]any{"id": "person-2", "name": "测试人员", "customerCode": "tenant-code", "departmentId": "department-1"},
			"companyVo": map[string]any{"id": "company-1", "name": "测试公司"},
		},
	})
}

// handleTemplates 验证模板查询或顶层 ids 精确核对协议。
func (f *fakeTarget) handleTemplates(response http.ResponseWriter, request *http.Request) {
	body := f.requireSession(request)
	data, _ := body["data"].(map[string]any)
	_, hasFlowName := data["flowName"].(string)
	ids, hasIDs := body["ids"].([]any)
	if (!hasFlowName && !hasIDs) || data["useScope"] != "invest" || body["platformCode"] != "200001,999999" || body["pagination"] != true || body["ignoreFormTemplateBizRelevanceData"] != true {
		f.t.Error("流程模板请求参数不符合已核实协议")
	}
	if hasIDs {
		if len(ids) != 1 || ids[0] != "template-id" {
			f.t.Error("模板列表没有通过顶层 ids 精确核对保存的模板 ID")
		}
		if _, exists := data["id"]; exists {
			f.t.Error("模板列表错误地依赖 data.id，目标服务不会用它筛选")
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
	items := []any{
		map[string]any{
			"id": "template-id", "flowName": "真实流程模板", "code": "FLOW-CODE", "groupName": "业务流程",
			"flowStatus": "enable", "typeName": "经营管理", "updateDate": "2026-07-27 08:00",
			"remark": "用于验证采购审批", "formExist": "withForm", "auditWay": "contract_review",
			"formTemplateList": []any{map[string]any{"id": "form-a"}, map[string]any{"id": "form-b"}},
		},
	}
	if hasIDs {
		items = append([]any{map[string]any{"id": "other-template", "flowName": "其他模板"}}, items...)
	}
	if mode == "template-other-only" {
		items = []any{map[string]any{"id": "other-template", "flowName": "其他模板"}}
	}
	writeTargetJSON(response, map[string]any{
		"isSuccess": true, "data": items,
		"total": len(items), "pages": 1, "current": 1, "size": 20,
	})
}

// TestFlowTreeReadUsesExactSourceLookupBeforeDetails 验证三类来源先核对再读详情。
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
			if err != nil || tree == nil || tree.ID != "start" || tree.Child == nil || tree.Child.ID != "approval" || tree.Child.Child == nil || tree.Child.Child.ID != "end" {
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

// TestFlowRequirementSnapshotReadsSourceSpecificFormMetadata 验证三类来源读取对应模板或代理表单字段且保持调用顺序。
func TestFlowRequirementSnapshotReadsSourceSpecificFormMetadata(t *testing.T) {
	tests := []struct {
		name   string
		source string
		id     string
		calls  []string
	}{
		{name: "新发起模板表单", source: "new", id: "template-id", calls: []string{"template-list", "template-detail:template-id", "form-detail:form-template"}},
		{name: "已发代理表单", source: "started", id: "submitted-id", calls: []string{"submitted-list", "proxy-detail:proxy-submitted", "form-detail:form-proxy-submitted"}},
		{name: "待发代理表单", source: "pending", id: "due-id", calls: []string{"due-list", "proxy-detail:proxy-due", "form-detail:form-proxy-due"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := newFakeTarget(t)
			targetServer := httptest.NewServer(http.HandlerFunc(fake.handler))
			defer targetServer.Close()
			configureTargetEnv(t, targetServer.URL, fake.password, fake.loginCode, "2s")
			reader := service.NewTargetReadService(config.LoadTargetConfig())
			snapshot, err := reader.FlowRequirementSnapshot(context.Background(), "account-a", test.source, test.id)
			if err != nil || snapshot.Tree == nil || len(snapshot.FormFields) != 1 || snapshot.FormFields[0].Name != "申请金额" {
				t.Fatalf("路径要求目标快照读取失败：snapshot=%+v err=%v", snapshot, err)
			}
			if snapshot.Tree.Child == nil || snapshot.Tree.Child.AuditConfig == nil || snapshot.Tree.Child.AuditConfig.AuditType != "form_person" || len(snapshot.Tree.Child.FieldPowers) != 1 {
				t.Fatalf("流程节点要求元数据没有解码：%+v", snapshot.Tree.Child)
			}
			fake.mu.Lock()
			calls := append([]string(nil), fake.graphCalls...)
			fake.mu.Unlock()
			if strings.Join(calls, ",") != strings.Join(test.calls, ",") {
				t.Fatalf("要求读取调用顺序不正确：%v", calls)
			}
		})
	}
}

// TestFlowRequirementReadPreservesTimeoutCancellationAndResponseLimit 验证要求详情沿用目标客户端的超时、取消和响应上限。
func TestFlowRequirementReadPreservesTimeoutCancellationAndResponseLimit(t *testing.T) {
	t.Run("超时", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			select {
			case <-request.Context().Done():
			case <-time.After(100 * time.Millisecond):
			}
		}))
		defer server.Close()
		client, err := target.NewClient(target.ClientConfig{BaseURL: server.URL, Timeout: 20 * time.Millisecond})
		if err != nil {
			t.Fatalf("创建超时测试客户端失败：%v", err)
		}
		_, _, err = client.ReadTemplateRequirements(context.Background(), target.Session{SID: "runtime-session"}, "template")
		if !target.IsKind(err, target.ErrorTimeout) {
			t.Fatalf("要求详情超时未保持稳定分类：%v", err)
		}
	})

	t.Run("调用方取消", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			<-request.Context().Done()
		}))
		defer server.Close()
		client, err := target.NewClient(target.ClientConfig{BaseURL: server.URL, Timeout: time.Second})
		if err != nil {
			t.Fatalf("创建取消测试客户端失败：%v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, _, err = client.ReadTemplateRequirements(ctx, target.Session{SID: "runtime-session"}, "template")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("要求详情没有透传调用方取消：%v", err)
		}
	})

	t.Run("响应上限", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(strings.Repeat("x", 9<<20)))
		}))
		defer server.Close()
		client, err := target.NewClient(target.ClientConfig{BaseURL: server.URL, Timeout: time.Second})
		if err != nil {
			t.Fatalf("创建响应上限测试客户端失败：%v", err)
		}
		_, _, err = client.ReadTemplateRequirements(context.Background(), target.Session{SID: "runtime-session"}, "template")
		if !target.IsKind(err, target.ErrorResponseInvalid) {
			t.Fatalf("要求详情超大响应未稳定拒绝：%v", err)
		}
	})
}

// TestFlowRequirementReadSessionExpiryReplaysWholeChainOnce 验证表单详情会话失效时整条核对链只重放一次。
func TestFlowRequirementReadSessionExpiryReplaysWholeChainOnce(t *testing.T) {
	fake := newFakeTarget(t)
	fake.expireMode = "form-session-once"
	targetServer := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer targetServer.Close()
	configureTargetEnv(t, targetServer.URL, fake.password, fake.loginCode, "2s")
	reader := service.NewTargetReadService(config.LoadTargetConfig())
	snapshot, err := reader.FlowRequirementSnapshot(context.Background(), "account-a", "new", "template-id")
	if err != nil || len(snapshot.FormFields) != 1 {
		t.Fatalf("表单详情会话失效后要求读取失败：snapshot=%+v err=%v", snapshot, err)
	}
	fake.mu.Lock()
	loginCount := fake.loginCount
	calls := append([]string(nil), fake.graphCalls...)
	fake.mu.Unlock()
	want := []string{
		"template-list", "template-detail:template-id", "form-detail:form-template",
		"template-list", "template-detail:template-id", "form-detail:form-template",
	}
	if loginCount != 2 || strings.Join(calls, ",") != strings.Join(want, ",") {
		t.Fatalf("要求核对链重放次数不正确：login=%d calls=%v", loginCount, calls)
	}
}

// TestFlowTreeSnapshotUsesSourceSpecificEntryNodes 验证根、活动节点和待发任务入口集合。
func TestFlowTreeSnapshotUsesSourceSpecificEntryNodes(t *testing.T) {
	tests := []struct {
		source string
		id     string
		want   string
	}{
		{source: "new", id: "template-id", want: "start"},
		{source: "started", id: "submitted-id", want: "start"},
		{source: "pending", id: "due-id", want: "start,end"},
	}
	for _, test := range tests {
		t.Run(test.source, func(t *testing.T) {
			fake := newFakeTarget(t)
			targetServer := httptest.NewServer(http.HandlerFunc(fake.handler))
			defer targetServer.Close()
			configureTargetEnv(t, targetServer.URL, fake.password, fake.loginCode, "2s")
			reader := service.NewTargetReadService(config.LoadTargetConfig())
			snapshot, err := reader.FlowTreeSnapshot(context.Background(), "account-a", test.source, test.id)
			if err != nil {
				t.Fatalf("读取流程入口失败：%v", err)
			}
			if strings.Join(snapshot.EntryNodeIDs, ",") != test.want {
				t.Fatalf("%s 入口 = %v，期望 %s", test.source, snapshot.EntryNodeIDs, test.want)
			}
		})
	}
}

// TestDueFlowSnapshotReadsAllWaitingSendPages 验证并行待发入口不会被第一页截断。
func TestDueFlowSnapshotReadsAllWaitingSendPages(t *testing.T) {
	fake := newFakeTarget(t)
	fake.duePaged = true
	targetServer := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer targetServer.Close()
	configureTargetEnv(t, targetServer.URL, fake.password, fake.loginCode, "2s")
	reader := service.NewTargetReadService(config.LoadTargetConfig())
	snapshot, err := reader.FlowTreeSnapshot(context.Background(), "account-a", "pending", "due-id")
	if err != nil || strings.Join(snapshot.EntryNodeIDs, ",") != "start,end" {
		t.Fatalf("待发分页入口没有完整读取：entries=%v err=%v", snapshot.EntryNodeIDs, err)
	}
	fake.mu.Lock()
	calls := append([]string(nil), fake.graphCalls...)
	fake.mu.Unlock()
	if strings.Join(calls, ",") != "due-list,due-list,proxy-detail:proxy-due" {
		t.Fatalf("待发分页读取调用顺序不正确：%v", calls)
	}
}

// TestDueFlowSnapshotStopsAtTwentyPagesWithoutMetadata 验证目标省略页数时也不会发生无界读取。
func TestDueFlowSnapshotStopsAtTwentyPagesWithoutMetadata(t *testing.T) {
	fake := newFakeTarget(t)
	fake.dueUnbounded = true
	targetServer := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer targetServer.Close()
	configureTargetEnv(t, targetServer.URL, fake.password, fake.loginCode, "2s")
	reader := service.NewTargetReadService(config.LoadTargetConfig())
	_, err := reader.FlowTreeSnapshot(context.Background(), "account-a", "pending", "due-id")
	if !target.IsKind(err, target.ErrorResponseInvalid) {
		t.Fatalf("待发无页数满页响应没有稳定停止：%v", err)
	}
	fake.mu.Lock()
	calls := append([]string(nil), fake.graphCalls...)
	fake.mu.Unlock()
	if len(calls) != 20 {
		t.Fatalf("待发分页上限调用次数 = %d，期望 20", len(calls))
	}
}

// TestSubmittedFinishedInstanceIsNotConfigurable 验证结束状态不能借 end 节点保存零选择路径。
func TestSubmittedFinishedInstanceIsNotConfigurable(t *testing.T) {
	for _, status := range []string{"end", "termination", "abandon", "rejected", "withdraw"} {
		t.Run(status, func(t *testing.T) {
			fake := newFakeTarget(t)
			fake.submittedStatus = status
			targetServer := httptest.NewServer(http.HandlerFunc(fake.handler))
			defer targetServer.Close()
			configureTargetEnv(t, targetServer.URL, fake.password, fake.loginCode, "2s")
			reader := service.NewTargetReadService(config.LoadTargetConfig())
			_, err := reader.FlowTreeSnapshot(context.Background(), "account-a", "started", "submitted-id")
			if !errors.Is(err, service.ErrTargetFlowNotConfigurable) {
				t.Fatalf("结束状态 %s 没有被拒绝：%v", status, err)
			}
			fake.mu.Lock()
			calls := append([]string(nil), fake.graphCalls...)
			fake.mu.Unlock()
			if strings.Join(calls, ",") != "submitted-list" {
				t.Fatalf("结束状态仍读取了代理树：%v", calls)
			}
		})
	}
}

// TestSubmittedAwaitSentInstanceRemainsConfigurable 验证有可映射入口的待发状态仍可配置。
func TestSubmittedAwaitSentInstanceRemainsConfigurable(t *testing.T) {
	fake := newFakeTarget(t)
	fake.submittedStatus = "await_sent"
	targetServer := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer targetServer.Close()
	configureTargetEnv(t, targetServer.URL, fake.password, fake.loginCode, "2s")
	reader := service.NewTargetReadService(config.LoadTargetConfig())
	snapshot, err := reader.FlowTreeSnapshot(context.Background(), "account-a", "started", "submitted-id")
	if err != nil || snapshot.Tree == nil || strings.Join(snapshot.EntryNodeIDs, ",") != "start" {
		t.Fatalf("待发状态且入口可映射时没有允许配置：snapshot=%+v err=%v", snapshot, err)
	}
}

// TestFlowTreeReadSessionExpiryReplaysWholeChainOnce 验证会话失效后整条只读链仅重放一次。
func TestFlowTreeReadSessionExpiryReplaysWholeChainOnce(t *testing.T) {
	fake := newFakeTarget(t)
	fake.expireMode = "business-once"
	targetServer := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer targetServer.Close()
	configureTargetEnv(t, targetServer.URL, fake.password, fake.loginCode, "2s")
	reader := service.NewTargetReadService(config.LoadTargetConfig())
	tree, err := reader.FlowTree(context.Background(), "account-a", "new", "template-id")
	if err != nil || tree == nil {
		t.Fatalf("会话失效后读取流程树失败：%v", err)
	}
	if fake.loginCount != 2 || fake.templateCount != 2 {
		t.Fatalf("整条核对与详情链没有只重放一次：login=%d list=%d", fake.loginCount, fake.templateCount)
	}
	fake.mu.Lock()
	calls := append([]string(nil), fake.graphCalls...)
	fake.mu.Unlock()
	if strings.Join(calls, ",") != "template-list,template-list,template-detail:template-id" {
		t.Fatalf("会话重放顺序不正确：%v", calls)
	}
}

// TestFlowTreeReadRejectsUnmatchedTemplateEvenWhenListHasOtherItems 验证列表其他项不能冒充保存模板。
func TestFlowTreeReadRejectsUnmatchedTemplateEvenWhenListHasOtherItems(t *testing.T) {
	fake := newFakeTarget(t)
	fake.expireMode = "template-other-only"
	targetServer := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer targetServer.Close()
	configureTargetEnv(t, targetServer.URL, fake.password, fake.loginCode, "2s")
	reader := service.NewTargetReadService(config.LoadTargetConfig())
	_, err := reader.FlowTree(context.Background(), "account-a", "new", "template-id")
	if !errors.Is(err, service.ErrTargetFlowNotFound) {
		t.Fatalf("未精确匹配保存模板 ID 时没有拒绝读取：%v", err)
	}
	fake.mu.Lock()
	calls := append([]string(nil), fake.graphCalls...)
	fake.mu.Unlock()
	if strings.Join(calls, ",") != "template-list" {
		t.Fatalf("未匹配模板仍继续读取详情：%v", calls)
	}
}

// requireSession 验证 SID 只在后端按目标协议传递并解析请求体。
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

// TestRealReadProtocolAndThreeSourceMappings 验证三类只读列表的协议与公开映射。
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
	wants := []string{"displayName", "\"auditWay\":\"contract_review\"", "审批中", "真实待发流程"}
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

// TestSessionExpiryRelogsAndReplaysOnce 验证列表读取会话失效后只重登一次。
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

// TestLoginRejectionUsesStablePublicError 验证登录拒绝不泄露目标原文。
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

// TestSessionExpiryStopsAfterOneReplay 验证持续失效时禁止无限重登。
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

// TestEmptyBadPaginationBadJSONAndTargetUnavailable 验证空、坏响应和不可用边界。
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

// TestTargetTimeoutAndContextCancellation 验证超时和调用方取消保持可区分。
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

// configureTargetEnv 为单个假目标测试隔离运行期配置。
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

// callApp 调用测试 HTTP 处理器并核对状态码。
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

// runtimeValue 生成不会写入源码或日志的随机测试值。
func runtimeValue(t *testing.T, byteCount int) string {
	t.Helper()
	data := make([]byte, byteCount)
	if _, err := rand.Read(data); err != nil {
		t.Fatal("无法生成测试期临时值")
	}
	return hex.EncodeToString(data)
}

// writeTargetJSON 写出假目标 JSON 响应。
func writeTargetJSON(response http.ResponseWriter, value any) {
	response.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(response).Encode(value)
}

// TestPathConfigurationSnapshotResolvesConcreteAuditDirectories 验证人员、岗位、岗级、角色、部门、公司和扩展属性均由目标只读目录解析。
func TestPathConfigurationSnapshotResolvesConcreteAuditDirectories(t *testing.T) {
	fake := newFakeTarget(t)
	fake.directoryAudit = true
	targetServer := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer targetServer.Close()
	configureTargetEnv(t, targetServer.URL, fake.password, fake.loginCode, "2s")
	snapshot, err := service.NewTargetReadService(config.LoadTargetConfig()).PathConfigurationSnapshot(context.Background(), "account-a", "new", "template-id")
	if err != nil || snapshot.Tree == nil || snapshot.Tree.Child == nil || snapshot.Tree.Child.AuditConfig == nil {
		t.Fatalf("人员目录配置读取失败：snapshot=%+v err=%v", snapshot, err)
	}
	audit := snapshot.Tree.Child.AuditConfig
	wantNames := map[string]bool{"张三": true, "财务主任": true, "二级岗": true, "财务审批角色": true, "财务部": true, "测试公司": true, "项目负责人属性": true}
	for _, detail := range audit.Details {
		delete(wantNames, detail.Name)
	}
	if len(wantNames) != 0 || len(audit.ResolutionIssues) != 0 {
		t.Fatalf("固定人员对象没有完整解析：missing=%v audit=%+v", wantNames, audit)
	}
	wantScopes := map[string]bool{"财务审批角色": true, "财务主任": true, "财务部": true, "测试公司": true, "张三": true}
	for _, scope := range audit.Scopes {
		delete(wantScopes, scope.Name)
	}
	if len(wantScopes) != 0 || len(audit.Candidates) != 5 {
		t.Fatalf("运行节点范围或合法候选没有完整解析：missing=%v scopes=%+v candidates=%+v", wantScopes, audit.Scopes, audit.Candidates)
	}
	wantCandidates := map[string]bool{"张三": true, "李四": true, "王五": true, "赵六": true, "孙七": true}
	for _, candidate := range audit.Candidates {
		delete(wantCandidates, candidate.Name)
	}
	if len(wantCandidates) != 0 {
		t.Fatalf("人员、部门和公司范围没有按真实目录候选解析：missing=%v candidates=%+v", wantCandidates, audit.Candidates)
	}
	if len(snapshot.Tree.Child.AddSignIssues) != 0 || len(snapshot.Tree.Child.AddSignCandidates) != 1 || snapshot.Tree.Child.AddSignCandidates[0].Name != "张三" {
		t.Fatalf("加签节点人员目录没有独立按公司人员响应解析：candidates=%+v issues=%+v", snapshot.Tree.Child.AddSignCandidates, snapshot.Tree.Child.AddSignIssues)
	}
	fake.mu.Lock()
	flags := append([]string(nil), fake.directoryFlags...)
	fake.mu.Unlock()
	if strings.Count(strings.Join(flags, ","), "3") != 1 {
		t.Fatalf("部门和公司候选必须复用一次 flag=3 人员树：flags=%v", flags)
	}
	graphNodes, graphEdges, warnings, err := analyzer.NewFlowGraphAnalyzer().Analyze(snapshot.Tree)
	if err != nil {
		t.Fatalf("目录测试流程图分析失败：%v", err)
	}
	graph := model.FlowGraph{EntryNodeIDs: snapshot.EntryNodeIDs, Nodes: graphNodes, Edges: graphEdges, Warnings: warnings}
	pathAnalysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, nil)
	if err != nil {
		t.Fatalf("目录测试路径分析失败：%v", err)
	}
	configuration, _, err := analyzer.NewPathConfigAnalyzer().Analyze(graph, snapshot.Tree, snapshot.FormFields, model.ExecutionPath{}, pathAnalysis, nil, nil, nil)
	if err != nil {
		t.Fatalf("目录测试配置投影失败：%v", err)
	}
	encoded, _ := json.Marshal(configuration)
	for _, forbidden := range []string{"person-1", "position-1", "level-1", "role-1", "department-1", "company-1", "attr-1"} {
		if bytes.Contains(encoded, []byte(forbidden)) {
			t.Fatalf("公开配置泄露目标目录 ID %s：%s", forbidden, encoded)
		}
	}
}

// TestPathConfigurationSnapshotClassifiesAuditDirectoryFailure 验证目标目录失败形成明确阻塞项而不是伪装成运行时确定。
func TestPathConfigurationSnapshotClassifiesAuditDirectoryFailure(t *testing.T) {
	fake := newFakeTarget(t)
	fake.directoryAudit = true
	fake.directoryFail = true
	targetServer := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer targetServer.Close()
	configureTargetEnv(t, targetServer.URL, fake.password, fake.loginCode, "2s")
	snapshot, err := service.NewTargetReadService(config.LoadTargetConfig()).PathConfigurationSnapshot(context.Background(), "account-a", "new", "template-id")
	if err != nil || snapshot.Tree == nil || snapshot.Tree.Child == nil || snapshot.Tree.Child.AuditConfig == nil {
		t.Fatalf("目录失败快照不应丢失流程结构：snapshot=%+v err=%v", snapshot, err)
	}
	if len(snapshot.Tree.Child.AuditConfig.ResolutionIssues) == 0 {
		t.Fatalf("目录读取失败没有形成稳定分类：%+v", snapshot.Tree.Child.AuditConfig)
	}
	graphNodes, graphEdges, warnings, err := analyzer.NewFlowGraphAnalyzer().Analyze(snapshot.Tree)
	if err != nil {
		t.Fatalf("目录失败流程图分析失败：%v", err)
	}
	graph := model.FlowGraph{EntryNodeIDs: snapshot.EntryNodeIDs, Nodes: graphNodes, Edges: graphEdges, Warnings: warnings}
	pathAnalysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, nil)
	if err != nil {
		t.Fatalf("目录失败路径分析失败：%v", err)
	}
	configuration, validation, err := analyzer.NewPathConfigAnalyzer().Analyze(graph, snapshot.Tree, snapshot.FormFields, model.ExecutionPath{}, pathAnalysis, nil, nil, nil)
	if err != nil {
		t.Fatalf("目录失败配置投影失败：%v", err)
	}
	approval := configuration.Groups[0].Nodes[1]
	if len(approval.Persons) != 1 || approval.Persons[0].Mode != "review" || len(validation.Blockers) == 0 || !strings.Contains(approval.Persons[0].Detail, "读取失败") {
		t.Fatalf("目录失败被错误降级：person=%+v blockers=%+v", approval.Persons, validation.Blockers)
	}
}

// TestFormIdentityContextResolvesCompanyDepartmentUser 验证账号身份在 flag=3 目录树中定位为公司、部门与本人节点。
func TestFormIdentityContextResolvesCompanyDepartmentUser(t *testing.T) {
	fake := newFakeTarget(t)
	targetServer := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer targetServer.Close()
	configureTargetEnv(t, targetServer.URL, fake.password, fake.loginCode, "2s")
	reader := service.NewTargetReadService(config.LoadTargetConfig())
	runtimeSession, err := reader.FormRuntimeSession(context.Background(), "account-a")
	if err != nil {
		t.Fatalf("预热已验证运行时会话失败：%v", err)
	}
	identity, err := reader.FormIdentityContext(context.Background(), target.Session{
		SID: runtimeSession.SID, UserID: runtimeSession.UserID, CompanyID: runtimeSession.CompanyID,
		DepartmentID: runtimeSession.DepartmentID, CustomerCode: runtimeSession.CustomerCode,
	})
	if err != nil {
		t.Fatalf("身份目录解析失败：%v", err)
	}
	if identity.Company.ID != "company-1" || identity.Company.Name != "测试公司" || identity.Company.Type != "1" {
		t.Fatalf("公司节点解析不正确：%+v", identity.Company)
	}
	if identity.Department.ID != "department-1" || identity.Department.Name != "财务部" || identity.Department.Type != "2" || identity.Department.CompanyID != "company-1" {
		t.Fatalf("部门节点解析不正确：%+v", identity.Department)
	}
	if identity.User.ID != "person-2" || identity.User.Name != "李四" || identity.User.Type != "5" || identity.User.ParentID != "department-1" {
		t.Fatalf("本人节点解析不正确：%+v", identity.User)
	}
}

// TestPathConfigurationSnapshotReadsFormMakingOptions 验证选项、必填和默认值来自 FormMaking 组件配置。
func TestPathConfigurationSnapshotReadsFormMakingOptions(t *testing.T) {
	fake := newFakeTarget(t)
	targetServer := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer targetServer.Close()
	configureTargetEnv(t, targetServer.URL, fake.password, fake.loginCode, "2s")
	reader := service.NewTargetReadService(config.LoadTargetConfig())
	snapshot, err := reader.PathConfigurationSnapshot(context.Background(), "account-a", "new", "template-id")
	if err != nil || len(snapshot.FormFields) != 1 {
		t.Fatalf("选项读取失败：snapshot=%+v err=%v", snapshot, err)
	}
	field := snapshot.FormFields[0]
	if field.DefaultValue != "1000" {
		t.Fatalf("组件默认值没有覆盖字段默认值：%+v", field)
	}
	if !field.Required {
		t.Fatalf("组件必填状态没有读取：%+v", field)
	}
}

// TestPathConfigurationSnapshotRecursesFormMakingContainers 验证栅格、报表、表格列和嵌套复杂组件不会因不在顶层 list 而漏读或伪造成文本。
func TestPathConfigurationSnapshotRecursesFormMakingContainers(t *testing.T) {
	fake := newFakeTarget(t)
	fake.formFields = []any{
		map[string]any{"id": "field-grid", "name": "栅格字段", "englishName": "gridValue", "fieldType": "stringType"},
		map[string]any{"id": "field-report", "name": "报表字段", "englishName": "reportValue", "fieldType": "stringType"},
		map[string]any{"id": "field-table", "name": "表格字段", "englishName": "tableValue", "fieldType": "stringType"},
		map[string]any{"id": "field-complex", "name": "复杂组件", "englishName": "complexValue", "fieldType": "listType"},
	}
	fake.templateData = `{"list":[{"type":"grid","columns":[{"type":"col","list":[{"type":"input","model":"gridValue","name":"栅格字段","options":{"required":true}}]}]},{"type":"report","rows":[{"columns":[{"type":"td","list":[{"type":"input","model":"reportValue","name":"报表字段"}]}]}]},{"type":"table","tableColumns":[{"type":"input","model":"tableValue","name":"表格字段"}]},{"type":"custom","el":"custome-info-select","model":"complexValue","name":"通用信息选择","options":{}}]}`
	targetServer := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer targetServer.Close()
	configureTargetEnv(t, targetServer.URL, fake.password, fake.loginCode, "2s")
	reader := service.NewTargetReadService(config.LoadTargetConfig())
	snapshot, err := reader.PathConfigurationSnapshot(context.Background(), "account-a", "new", "template-id")
	if err != nil || len(snapshot.FormFields) != 4 {
		t.Fatalf("递归表单字段读取失败：fields=%+v err=%v", snapshot.FormFields, err)
	}
	byName := make(map[string]target.FormFieldDetail, len(snapshot.FormFields))
	for _, field := range snapshot.FormFields {
		byName[field.EnglishName] = field
	}
	for _, key := range []string{"gridValue", "reportValue", "tableValue"} {
		if byName[key].Name == "" || byName[key].ComponentType != "input" {
			t.Fatalf("嵌套基础字段没有按真实组件解析：%s %+v", key, byName[key])
		}
	}
	if byName["complexValue"].ComponentType != "custom" || byName["complexValue"].ComponentName != "custome-info-select" {
		t.Fatalf("自定义组件真实 el 没有保留：%+v", byName["complexValue"])
	}
}
