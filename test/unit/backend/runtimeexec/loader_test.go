package runtimeexec_test

import (
	"encoding/json"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/runtimeexec"
)

func TestBuildLoadPayload(t *testing.T) {
	values := json.RawMessage(`{"vacateType":"2","vacateType__virtualName":"产假"}`)
	payload, err := runtimeexec.BuildLoadPayload(runtimeexec.LoadInput{
		Mode:           runtimeexec.LoadModeInitiate,
		Session:        runtimeexec.SessionFacts{UserID: "u-1", Account: "acct", DisplayName: "骆蒙恩", CompanyID: "c-1", DepartmentID: "d-1", CustomerCode: "cc"},
		ToolFlowName:   "工具流程",
		SelectFlowType: "employee_leave",
		FlowTemplateID: "73eaa9b297b54364a3c320d3002a1b4f",
		Permissions:    []string{"reason"},
		BaseURL:        "https://target.example.com/",
	}, values)
	if err != nil {
		t.Fatalf("合法装载被拒绝：%v", err)
	}
	if payload.FlowTemplateID != "73eaa9b297b54364a3c320d3002a1b4f" {
		t.Fatalf("发起装载必须携带计划绑定模板 ID 专字段：%+v", payload)
	}
	if payload.CurrentOperator.UserID != "u-1" || payload.CurrentOperator.Name != "骆蒙恩" {
		t.Fatalf("当前操作人身份投影错误：%+v", payload.CurrentOperator)
	}
	if payload.BaseURL != "https://target.example.com" {
		t.Fatalf("装载基础字段错误：%+v", payload)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(raw)
	if strings.Contains(encoded, `"sid"`) && !strings.Contains(encoded, `"sid":""`) {
		t.Fatalf("BuildLoadPayload 自身不得携带 SID（运行时注入）：%s", raw)
	}
	if strings.Contains(encoded, `"template"`) {
		t.Fatalf("v2 装载不得携带渲染模板：%s", raw)
	}

	failures := []struct {
		name  string
		input runtimeexec.LoadInput
	}{
		{"未知模式", runtimeexec.LoadInput{Mode: "both", Session: runtimeexec.SessionFacts{UserID: "u"}, ToolFlowName: "t", SelectFlowType: "leave"}},
		{"缺工具流程名称", runtimeexec.LoadInput{Mode: runtimeexec.LoadModeInitiate, Session: runtimeexec.SessionFacts{UserID: "u"}, SelectFlowType: "leave", FlowTemplateID: "tpl-1"}},
		{"发起装载缺用户事实", runtimeexec.LoadInput{Mode: runtimeexec.LoadModeInitiate, ToolFlowName: "t", SelectFlowType: "leave", FlowTemplateID: "tpl-1"}},
		{"发起装载缺 auditWay", runtimeexec.LoadInput{Mode: runtimeexec.LoadModeInitiate, Session: runtimeexec.SessionFacts{UserID: "u"}, ToolFlowName: "t", FlowTemplateID: "tpl-1"}},
		{"发起装载缺模板 ID", runtimeexec.LoadInput{Mode: runtimeexec.LoadModeInitiate, Session: runtimeexec.SessionFacts{UserID: "u"}, ToolFlowName: "t", SelectFlowType: "leave"}},
		{"编辑装载缺实例 ID", runtimeexec.LoadInput{Mode: runtimeexec.LoadModeEdit, Session: runtimeexec.SessionFacts{UserID: "u"}, ToolFlowName: "t"}},
	}
	for _, item := range failures {
		if _, err := runtimeexec.BuildLoadPayload(item.input, values); err == nil {
			t.Fatalf("%s：非法装载通过了校验", item.name)
		}
	}
}

func TestEncodeLoadMessage(t *testing.T) {
	raw, err := runtimeexec.EncodeLoadMessage("sess-1", "req-1", "run-1", "pr-1", "step-1", &runtimeexec.LoadPayload{
		Mode:         runtimeexec.LoadModeInitiate,
		ToolFlowName: "工具流程",
	})
	if err != nil {
		t.Fatalf("编码失败：%v", err)
	}
	message, err := runtimeexec.ValidateMessage(raw)
	if err != nil {
		t.Fatalf("装载信封未通过协议校验：%v", err)
	}
	if message.Kind != runtimeexec.KindLoad || message.SessionID != "sess-1" {
		t.Fatalf("装载信封字段错误：%+v", message)
	}
}

func TestBuildLoadPayloadCarriesProxyAndForwardFacts(t *testing.T) {
	values := json.RawMessage(`{"reason":"x"}`)
	payload, err := runtimeexec.BuildLoadPayload(runtimeexec.LoadInput{
		Mode:                         runtimeexec.LoadModeForward,
		Session:                      runtimeexec.SessionFacts{UserID: "u-1", Account: "acct", DisplayName: "处理人"},
		ToolFlowName:                 "工具流程",
		InstanceID:                   "instance-1",
		JobTaskID:                    "task-1",
		FormProxyID:                  "form-proxy-1",
		FlowProxyID:                  "flow-proxy-1",
		SelectFlowType:               "employee_leave",
		FlowNodeType:                 "run_node_choose",
		NextNodeProxyID:              "next-node-1",
		FlowInstanceBizRelevanceList: []runtimeexec.BizRelevance{{OtherBiz: "company", OtherBizID: "c-1"}},
		ReceiverID:                   "receiver-1",
		ReceiverName:                 "接收人",
		ForwardCheckList:             []string{"fuyan", "yijian"},
		ForwardTab:                   "backlog",
	}, values)
	if err != nil {
		t.Fatalf("合法转发装载被拒绝：%v", err)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(raw)
	for _, want := range []string{
		`"formProxyId":"form-proxy-1"`, `"flowProxyId":"flow-proxy-1"`,
		`"selectFlowType":"employee_leave"`, `"flowNodeType":"run_node_choose"`,
		`"nextNodeProxyId":"next-node-1"`, `"flowInstanceBizRelevanceList"`,
		`"receiverId":"receiver-1"`, `"forwardCheckList":["fuyan","yijian"]`, `"forwardTab":"backlog"`,
	} {
		if !strings.Contains(encoded, want) {
			t.Fatalf("转发装载载荷缺少 %s：%s", want, encoded)
		}
	}

	forwardBase := runtimeexec.LoadInput{
		Mode: runtimeexec.LoadModeForward, ToolFlowName: "工具流程", InstanceID: "instance-1", JobTaskID: "task-1", ReceiverID: "receiver-1",
	}
	if _, err := runtimeexec.BuildLoadPayload(forwardBase, values); err != nil {
		t.Fatalf("带接收人的转发装载应通过：%v", err)
	}
	noReceiver := forwardBase
	noReceiver.ReceiverID = ""
	if _, err := runtimeexec.BuildLoadPayload(noReceiver, values); err == nil {
		t.Fatalf("缺接收人的转发装载必须被拒绝")
	}
	noInstance := forwardBase
	noInstance.InstanceID = ""
	if _, err := runtimeexec.BuildLoadPayload(noInstance, values); err == nil {
		t.Fatalf("缺实例的转发装载必须被拒绝")
	}
}
