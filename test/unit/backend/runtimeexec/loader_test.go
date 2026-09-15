// T008 装载载荷契约：身份投影、模板/表单数据校验与 load 信封编码。
package runtimeexec_test

import (
	"encoding/json"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/runtimeexec"
)

// TestBuildLoadPayload 装载载荷必须携带两套身份与工具流程名称，SID 绝不出现在载荷里。
func TestBuildLoadPayload(t *testing.T) {
	template := json.RawMessage(`{"list":[{"type":"select","model":"vacateType"}],"config":{}}`)
	values := json.RawMessage(`{"vacateType":"2","vacateType__virtualName":"产假"}`)
	payload, err := runtimeexec.BuildLoadPayload(runtimeexec.LoadInput{
		Mode:         runtimeexec.LoadModeInitiate,
		Session:      runtimeexec.SessionFacts{UserID: "u-1", Account: "acct", DisplayName: "骆蒙恩", CompanyID: "c-1", DepartmentID: "d-1", CustomerCode: "cc"},
		ToolFlowName: "工具流程",
		Permissions:  []string{"reason"},
		BaseURL:      "https://target.example.com/",
	}, template, values)
	if err != nil {
		t.Fatalf("合法装载被拒绝：%v", err)
	}
	if payload.CurrentOperator.UserID != "u-1" || payload.CurrentOperator.Name != "骆蒙恩" {
		t.Fatalf("当前操作人身份投影错误：%+v", payload.CurrentOperator)
	}
	if payload.RenderType != "formmaking" || payload.BaseURL != "https://target.example.com" {
		t.Fatalf("装载基础字段错误：%+v", payload)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "SID") || strings.Contains(string(raw), `"sid"`) {
		t.Fatalf("装载载荷不允许携带会话标识：%s", raw)
	}

	failures := []struct {
		name  string
		input runtimeexec.LoadInput
	}{
		{"未知模式", runtimeexec.LoadInput{Mode: "both", Session: runtimeexec.SessionFacts{UserID: "u"}, ToolFlowName: "t"}},
		{"缺工具流程名称", runtimeexec.LoadInput{Mode: runtimeexec.LoadModeInitiate, Session: runtimeexec.SessionFacts{UserID: "u"}}},
		{"发起装载缺用户事实", runtimeexec.LoadInput{Mode: runtimeexec.LoadModeInitiate, ToolFlowName: "t"}},
	}
	for _, item := range failures {
		if _, err := runtimeexec.BuildLoadPayload(item.input, template, values); err == nil {
			t.Fatalf("%s：非法装载通过了校验", item.name)
		}
	}
	if _, err := runtimeexec.BuildLoadPayload(runtimeexec.LoadInput{Mode: runtimeexec.LoadModeInitiate, Session: runtimeexec.SessionFacts{UserID: "u"}, ToolFlowName: "t"}, nil, values); err == nil {
		t.Fatalf("缺模板必须被拒绝")
	}
	if _, err := runtimeexec.BuildLoadPayload(runtimeexec.LoadInput{Mode: runtimeexec.LoadModeInitiate, Session: runtimeexec.SessionFacts{UserID: "u"}, ToolFlowName: "t"}, json.RawMessage(`{bad`), values); err == nil {
		t.Fatalf("非法模板 JSON 必须被拒绝")
	}
}

// TestEncodeLoadMessage 装载信封必须通过协议校验且版本完整。
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

// TestBuildLoadPayloadCarriesProxyAndForwardFacts 评审 P1-3/P1-4 回归：
// 装载载荷必须携带目标代理与节点事实（目标组件的最终流程载荷直接消费），
// 转发装载必须携带接收人事实，且缺接收人/缺实例的转发在装载阶段就被拒绝。
func TestBuildLoadPayloadCarriesProxyAndForwardFacts(t *testing.T) {
	template := json.RawMessage(`{"list":[],"config":{}}`)
	values := json.RawMessage(`{"reason":"x"}`)
	payload, err := runtimeexec.BuildLoadPayload(runtimeexec.LoadInput{
		Mode:                         runtimeexec.LoadModeForward,
		Session:                      runtimeexec.SessionFacts{UserID: "u-1", Account: "acct", DisplayName: "处理人"},
		ToolFlowName:                 "工具流程",
		InstanceID:                   "instance-1",
		FormProxyID:                  "form-proxy-1",
		FlowProxyID:                  "flow-proxy-1",
		SelectFlowType:               "employee_leave",
		FlowNodeType:                 "run_node_choose",
		NextNodeProxyID:              "next-node-1",
		FlowInstanceBizRelevanceList: []runtimeexec.BizRelevance{{OtherBiz: "company", OtherBizID: "c-1"}},
		ReceiverID:                   "receiver-1",
		ReceiverName:                 "接收人",
		ForwardCheckList:             []string{"fuyan", "yijian"},
	}, template, values)
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
		`"receiverId":"receiver-1"`, `"forwardCheckList":["fuyan","yijian"]`,
	} {
		if !strings.Contains(encoded, want) {
			t.Fatalf("转发装载载荷缺少 %s：%s", want, encoded)
		}
	}

	forwardBase := runtimeexec.LoadInput{
		Mode: runtimeexec.LoadModeForward, ToolFlowName: "工具流程", InstanceID: "instance-1", ReceiverID: "receiver-1",
	}
	if _, err := runtimeexec.BuildLoadPayload(forwardBase, template, values); err != nil {
		t.Fatalf("带接收人的转发装载应通过：%v", err)
	}
	noReceiver := forwardBase
	noReceiver.ReceiverID = ""
	if _, err := runtimeexec.BuildLoadPayload(noReceiver, template, values); err == nil {
		t.Fatalf("缺接收人的转发装载必须被拒绝")
	}
	noInstance := forwardBase
	noInstance.InstanceID = ""
	if _, err := runtimeexec.BuildLoadPayload(noInstance, template, values); err == nil {
		t.Fatalf("缺实例的转发装载必须被拒绝")
	}
}
