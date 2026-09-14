package protocol_test

import (
	"encoding/json"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
)

// F-035/T06 逐接口载荷契约：暂存、移交、回退、取回、撤回、转发、关注、催办的实际构造
// 必须落在协议矩阵登记的形状内（参考页面证据见 protocol_matrix.go 各行注释）。

// assertMatrixRequired 校验载荷满足矩阵中 required 字段的存在性。
func assertMatrixRequired(t *testing.T, endpoint string, body map[string]any) {
	t.Helper()
	flat := flattenBody(t, body)
	for field, presence := range target.ProtocolMatrix()[endpoint] {
		// sid/projectId 由统一出口在发送前注入（信封层），载荷构造器不负责；信封注入有独立契约用例。
		if field == "sid" || field == "projectId" {
			continue
		}
		_, exists := flat[field]
		switch presence {
		case target.PresenceRequired:
			if !exists {
				t.Fatalf("%s 缺少 required 字段 %s：%v", endpoint, field, flat)
			}
		case target.PresenceForbidden:
			if exists {
				t.Fatalf("%s 出现 forbidden 字段 %s：%v", endpoint, field, flat)
			}
		}
	}
}

// flattenBody 把 data.xxx / approverAppendVo.xxx 等嵌套路径打平，便于矩阵比对。
func flattenBody(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	var nested map[string]any
	if err := json.Unmarshal(encoded, &nested); err != nil {
		t.Fatal(err)
	}
	flat := map[string]any{}
	var walk func(prefix string, value any)
	walk = func(prefix string, value any) {
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				path := key
				if prefix != "" {
					path = prefix + "." + key
				}
				flat[path] = child
				walk(path, child)
			}
		default:
		}
	}
	walk("", nested)
	return flat
}

func buildAction(t *testing.T, request target.ActionWriteRequest) (string, map[string]any) {
	t.Helper()
	body, endpoint, err := target.BuildActionBody(request)
	if err != nil {
		t.Fatalf("构造 %s 载荷失败：%v", request.Action, err)
	}
	return endpoint, body
}

func TestActionBodiesMatchMatrix(t *testing.T) {
	trackingOn := true
	cases := []struct {
		name     string
		endpoint string
		request  target.ActionWriteRequest
	}{
		{
			name: "暂存", endpoint: target.WriteEndpointStorageForm,
			request: target.ActionWriteRequest{Action: "storage_form_data", InstanceID: "i-1", NodeProxyID: "node-1",
				ExecuteDesc: "暂存", FormData: json.RawMessage(`{"amount":1}`)},
		},
		{
			name: "移交", endpoint: target.WriteEndpointApproverAppend,
			request: target.ActionWriteRequest{Action: "transfer", InstanceID: "i-1", JobTaskID: "t-1", BatchNo: "b-1",
				AuditStatus: "transfer", ExecuteDesc: "移交", NodeProxyID: "node-1", UserIDs: []string{"user-1"}},
		},
		{
			name: "回退", endpoint: target.WriteEndpointRollBack,
			request: target.ActionWriteRequest{Action: "rollback_previous", InstanceID: "i-1", JobTaskID: "t-1", ExecuteDesc: "回退"},
		},
		{
			name: "取回", endpoint: target.WriteEndpointRetrieve,
			request: target.ActionWriteRequest{Action: "retrieve", InstanceID: "i-1", JobTaskID: "t-1"},
		},
		{
			name: "撤回", endpoint: target.WriteEndpointRevocation,
			request: target.ActionWriteRequest{Action: "withdraw", InstanceID: "i-1", ExecuteDesc: "撤回说明"},
		},
		{
			name: "催办", endpoint: target.WriteEndpointUrge,
			request: target.ActionWriteRequest{Action: "urge", InstanceID: "i-1"},
		},
		{
			name: "关注", endpoint: target.WriteEndpointFlowTracking,
			request: target.ActionWriteRequest{Action: "follow", InstanceID: "i-1", Tracking: &trackingOn},
		},
		{
			name: "转发", endpoint: target.WriteEndpointTranspond,
			request: target.ActionWriteRequest{Action: "forward", InstanceID: "i-1", ReceiverID: "user-2",
				Name: "转发名称", FormData: json.RawMessage(`{}`)},
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			endpoint, body := buildAction(t, test.request)
			if endpoint != test.endpoint {
				t.Fatalf("端点错误：%s，期望 %s", endpoint, test.endpoint)
			}
			assertMatrixRequired(t, endpoint, body)
		})
	}
}

// TestAddSignBodyMatchesMatrix 锁定加签形状：data.id + flowProxyProtocol.data（完整代理树）。
func TestAddSignBodyMatchesMatrix(t *testing.T) {
	endpoint, body := buildAction(t, target.ActionWriteRequest{
		Action: "add_sign", InstanceID: "i-1", FlowProxyTree: json.RawMessage(`{"flowNodeList":[]}`),
	})
	if endpoint != target.WriteEndpointUpdateFlowProxy {
		t.Fatalf("加签端点错误：%s", endpoint)
	}
	assertMatrixRequired(t, endpoint, body)
}
