package target_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
)

// TestActionValidationDoesNotReachTarget 验证动作本地校验失败时不会创建 trace 或发送 HTTP 请求。
func TestActionValidationDoesNotReachTarget(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount++
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := newF016Client(t, server.URL, 5*time.Second)
	tracking := true
	_, traceID, err := client.ExecuteActionWrite(context.Background(), target.Session{SID: "sid"}, target.ActionWriteRequest{
		Action: "follow", Tracking: &tracking,
	})
	var validationErr *target.RequestValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("本地参数错误应返回 RequestValidationError，实际 %T %v", err, err)
	}
	if requestCount != 0 || traceID != "" {
		t.Fatalf("本地校验失败不得发 HTTP 或生成 trace，requests=%d trace=%q", requestCount, traceID)
	}
	if got, want := target.UserFacingErrorMessage(target.WriteResponse{}, err), "流程实例 id不能为空，拒绝发送"; got != want {
		t.Fatalf("页面应显示原始校验原因，实际 %q，期望 %q", got, want)
	}
}

// TestActionPayloadsKeepEmptyFormContainer 验证所有会调用目标保存表单逻辑的动作即使没有字段也发送 data 空对象。
func TestActionPayloadsKeepEmptyFormContainer(t *testing.T) {
	tests := []struct {
		name  string
		build func() (map[string]any, error)
	}{
		{
			name: "提交",
			build: func() (map[string]any, error) {
				return target.BuildSubmitBody(target.SubmitFlowInstanceRequest{Name: "测试", FlowProxyID: "flow-1"}), nil
			},
		},
		{
			name: "同意",
			build: func() (map[string]any, error) {
				return target.BuildAuditBody(target.AuditCurrentTaskRequest{InstanceID: "instance-1", JobTaskID: "task-1", AuditStatus: "pass"}), nil
			},
		},
		{
			name: "重新提交",
			build: func() (map[string]any, error) {
				body, _, err := target.BuildActionBody(target.ActionWriteRequest{Action: "resubmit", InstanceID: "instance-1", FlowProxyID: "flow-1"})
				return body, err
			},
		},
		{
			name: "暂存",
			build: func() (map[string]any, error) {
				body, _, err := target.BuildActionBody(target.ActionWriteRequest{Action: "storage_form_data", InstanceID: "instance-1", NodeProxyID: "node-1"})
				return body, err
			},
		},
		{
			name: "不同意",
			build: func() (map[string]any, error) {
				body, _, err := target.BuildActionBody(target.ActionWriteRequest{Action: "reject", InstanceID: "instance-1", JobTaskID: "task-1", AuditStatus: "no_pass"})
				return body, err
			},
		},
		{
			name: "转发",
			build: func() (map[string]any, error) {
				body, _, err := target.BuildActionBody(target.ActionWriteRequest{Action: "forward", ReceiverID: "user-1", Name: "转发测试"})
				return body, err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body, err := test.build()
			if err != nil {
				t.Fatalf("构造%s载荷失败：%v", test.name, err)
			}
			formData, ok := body["formDataMongoVo"].(map[string]any)
			if !ok {
				t.Fatalf("%s缺少 formDataMongoVo：%v", test.name, body)
			}
			raw, ok := formData["data"].(json.RawMessage)
			if !ok || string(raw) != `{}` {
				t.Fatalf("%s空表单必须原样发送 data:{}，实际 %#v", test.name, formData["data"])
			}
		})
	}
}

// TestForwardEmptyFormUsesAtomicTargetWrite 验证空表单转发能穿过发送前校验并命中目标原子端点。
func TestForwardEmptyFormUsesAtomicTargetWrite(t *testing.T) {
	var path string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path = request.URL.Path
		data, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("读取转发请求失败：%v", err)
			return
		}
		if err := json.Unmarshal(data, &body); err != nil {
			t.Errorf("转发请求不是有效 JSON：%v，正文=%s", err, data)
			return
		}
		_, _ = writer.Write([]byte(`{"isSuccess":true,"data":{"id":"forward-1"}}`))
	}))
	defer server.Close()

	client := newF016Client(t, server.URL, 5*time.Second)
	_, _, err := client.ExecuteActionWrite(context.Background(), target.Session{SID: "sid"}, target.ActionWriteRequest{
		Action: "forward", ReceiverID: "user-1", Name: "转发测试",
	})
	if err != nil {
		t.Fatalf("空表单转发不应在本地校验阶段被拒绝：%v", err)
	}
	if path != target.WriteEndpointTranspond {
		t.Fatalf("转发写入端点错误：%s", path)
	}
	formData, ok := body["formDataMongoVo"].(map[string]any)
	if !ok {
		t.Fatalf("实际转发请求缺少 formDataMongoVo：%v", body)
	}
	if data, ok := formData["data"].(map[string]any); !ok || len(data) != 0 {
		t.Fatalf("实际转发请求空表单不正确：%v", formData)
	}
}

// TestIsFlowCreatorAcceptsBothTargetCreatorFields 验证创建人核验兼容目标实例列表已经使用的 initiatorId 和 createrId 字段。
func TestIsFlowCreatorAcceptsBothTargetCreatorFields(t *testing.T) {
	for _, payload := range []string{
		`[{"id":"instance-1","initiatorId":"user-1"}]`,
		`[{"id":"instance-1","createrId":"user-1"}]`,
	} {
		t.Run(payload, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/web/flowInstanceApi/list" {
					t.Errorf("创建人核验端点错误：%s", request.URL.Path)
				}
				_, _ = writer.Write([]byte(`{"isSuccess":true,"data":` + payload + `}`))
			}))
			defer server.Close()

			client := newF016Client(t, server.URL, 5*time.Second)
			isCreator, err := client.IsFlowCreator(context.Background(), target.Session{SID: "sid", UserID: "user-1"}, "instance-1")
			if err != nil || !isCreator {
				t.Fatalf("目标创建人字段应允许当前用户操作：isCreator=%v err=%v", isCreator, err)
			}
		})
	}
}

// TestAllAtomicActionsSendVerifiedTargetContracts 验证 15 个用户动作都经过各自已核对的原子端点，
// 并把目标服务实际要求的关键字段发送出去。测试覆盖真实 HTTP 出口，防止预览构造和最终写请求漂移。
func TestAllAtomicActionsSendVerifiedTargetContracts(t *testing.T) {
	var receivedPath string
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedPath = request.URL.Path
		if err := json.NewDecoder(request.Body).Decode(&receivedBody); err != nil {
			t.Errorf("目标写请求不是有效 JSON：%v", err)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Path == target.WriteEndpointSubmit || request.URL.Path == target.WriteEndpointAudit {
			_, _ = writer.Write([]byte(`{"isSuccess":true,"data":{"id":"instance-1","status":"run","batchNo":"batch-1"}}`))
			return
		}
		_, _ = writer.Write([]byte(`{"isSuccess":true,"data":{}}`))
	}))
	defer server.Close()

	client := newF016Client(t, server.URL, 5*time.Second)
	lookup := func(body map[string]any, path string) (any, bool) {
		var current any = body
		for _, key := range strings.Split(path, ".") {
			object, ok := current.(map[string]any)
			if !ok {
				return nil, false
			}
			current, ok = object[key]
			if !ok {
				return nil, false
			}
		}
		return current, true
	}
	assertFields := func(t *testing.T, fields map[string]any) {
		t.Helper()
		for path, want := range fields {
			got, ok := lookup(receivedBody, path)
			if !ok || !reflect.DeepEqual(got, want) {
				t.Fatalf("字段 %s = %#v，期望 %#v，完整正文=%v", path, got, want, receivedBody)
			}
		}
	}

	trackingOn, trackingOff := true, false
	formData := json.RawMessage(`{"amount":12}`)
	proxyTree := json.RawMessage(`{"id":"proxy-1","flowNodeTemplate":{"id":"node-1","flowNodeAuditConfig":{"flowNodeDetailConfigList":[{"bizId":"user-1","auditDetailType":"personnel"}]}}}`)
	type actionCase struct {
		name     string
		endpoint string
		send     func() error
		fields   map[string]any
	}
	cases := []actionCase{
		{
			name: "保存草稿", endpoint: "/web/flowInstanceApi/submit",
			send: func() error {
				_, _, _, err := client.SubmitFlowInstance(context.Background(), target.Session{SID: "sid"}, target.SubmitFlowInstanceRequest{
					Name: "草稿实例", Status: "draft", FlowProxyID: "proxy-1", CompanyID: "company-1", FormData: formData,
				})
				return err
			},
			fields: map[string]any{"data.name": "草稿实例", "data.status": "draft", "data.flowProxyId": "proxy-1", "data.companyId": "company-1", "formDataMongoVo.data.amount": float64(12)},
		},
		{
			name: "提交", endpoint: "/web/flowInstanceApi/submit",
			send: func() error {
				_, _, _, err := client.SubmitFlowInstance(context.Background(), target.Session{SID: "sid"}, target.SubmitFlowInstanceRequest{
					Name: "提交实例", FlowProxyID: "proxy-1", FormData: formData,
				})
				return err
			},
			fields: map[string]any{"data.name": "提交实例", "data.flowProxyId": "proxy-1", "formDataMongoVo.data.amount": float64(12)},
		},
		{
			name: "重新提交", endpoint: "/web/flowInstanceApi/reSubmit",
			send: func() error {
				_, _, err := client.ExecuteActionWrite(context.Background(), target.Session{SID: "sid"}, target.ActionWriteRequest{
					Action: "resubmit", InstanceID: "instance-1", FlowProxyID: "proxy-1", FormProxyID: "form-1", CompanyID: "company-1", FormData: formData,
				})
				return err
			},
			fields: map[string]any{"data.id": "instance-1", "data.formProxyId": "form-1", "data.companyId": "company-1", "formDataMongoVo.data.amount": float64(12)},
		},
		{
			name: "暂存当前表单", endpoint: "/web/flowInstanceApi/storageFormData",
			send: func() error {
				_, _, err := client.ExecuteActionWrite(context.Background(), target.Session{SID: "sid"}, target.ActionWriteRequest{
					Action: "storage_form_data", InstanceID: "instance-1", NodeProxyID: "node-1", ExecuteDesc: "暂存说明", FormData: formData,
				})
				return err
			},
			fields: map[string]any{"data.id": "instance-1", "data.currentNodeProxyId": "node-1", "data.auditRecord.executeDesc": "暂存说明", "formDataMongoVo.data.amount": float64(12)},
		},
		{
			name: "加签", endpoint: "/web/flowInstanceApi/updateFlowProxy",
			send: func() error {
				_, _, err := client.ExecuteActionWrite(context.Background(), target.Session{SID: "sid"}, target.ActionWriteRequest{
					Action: "add_sign", InstanceID: "instance-1", NodeProxyID: "node-1", UserIDs: []string{"user-2"}, FlowProxyTree: proxyTree,
				})
				return err
			},
			fields: map[string]any{"data.id": "instance-1", "flowProxyProtocol.data.id": "proxy-1", "flowProxyProtocol.data.flowNodeTemplate.id": "node-1"},
		},
		{
			name: "移交", endpoint: "/web/flowInstanceApi/approverAppend",
			send: func() error {
				_, _, err := client.ExecuteActionWrite(context.Background(), target.Session{SID: "sid"}, target.ActionWriteRequest{
					Action: "transfer", InstanceID: "instance-1", JobTaskID: "task-1", BatchNo: "batch-1", NodeProxyID: "node-1", AuditStatus: "transfer", ExecuteDesc: "移交说明", UserIDs: []string{"user-2"},
				})
				return err
			},
			fields: map[string]any{"data.id": "instance-1", "data.jobTaskId": "task-1", "data.batchNo": "batch-1", "data.auditRecord.auditStatus": "transfer", "approverAppendVo.flowNodeProxyId": "node-1", "approverAppendVo.userIds": []any{"user-2"}},
		},
		{
			name: "同意", endpoint: "/flowInstanceApi/audit",
			send: func() error {
				_, _, _, err := client.AuditCurrentTask(context.Background(), target.Session{SID: "sid"}, target.AuditCurrentTaskRequest{
					InstanceID: "instance-1", JobTaskID: "task-1", FlowProxyID: "proxy-1", AuditStatus: "pass", ExecuteDesc: "同意说明", FormData: formData,
				})
				return err
			},
			fields: map[string]any{"data.id": "instance-1", "data.jobTaskId": "task-1", "data.flowProxyId": "proxy-1", "data.auditRecord.auditStatus": "pass", "formDataMongoVo.data.amount": float64(12)},
		},
		{
			name: "不同意", endpoint: "/flowInstanceApi/audit",
			send: func() error {
				_, _, err := client.ExecuteActionWrite(context.Background(), target.Session{SID: "sid"}, target.ActionWriteRequest{
					Action: "reject", InstanceID: "instance-1", JobTaskID: "task-1", FlowProxyID: "proxy-1", AuditStatus: "no_pass", ExecuteDesc: "不同意说明", FormData: formData,
				})
				return err
			},
			fields: map[string]any{"data.id": "instance-1", "data.jobTaskId": "task-1", "data.auditRecord.auditStatus": "no_pass", "formDataMongoVo.data.amount": float64(12)},
		},
		{
			name: "回退上一节点", endpoint: "/web/flowInstanceApi/rollBackThePreviousLevel",
			send: func() error {
				_, _, err := client.ExecuteActionWrite(context.Background(), target.Session{SID: "sid"}, target.ActionWriteRequest{
					Action: "rollback_previous", InstanceID: "instance-1", JobTaskID: "task-1", ExecuteDesc: "回退说明",
				})
				return err
			},
			fields: map[string]any{"data.id": "instance-1", "data.jobTaskId": "task-1", "data.withdrawDesc": "回退说明"},
		},
		{
			name: "取回", endpoint: "/web/flowInstanceApi/retrieveProcess",
			send: func() error {
				_, _, err := client.ExecuteActionWrite(context.Background(), target.Session{SID: "sid"}, target.ActionWriteRequest{
					Action: "retrieve", InstanceID: "instance-1", JobTaskID: "done-task-1",
				})
				return err
			},
			fields: map[string]any{"data.id": "instance-1", "data.jobTaskId": "done-task-1"},
		},
		{
			name: "撤回", endpoint: "/web/flowInstanceApi/revocation",
			send: func() error {
				_, _, err := client.ExecuteActionWrite(context.Background(), target.Session{SID: "sid"}, target.ActionWriteRequest{
					Action: "withdraw", InstanceID: "instance-1", ExecuteDesc: "撤回说明",
				})
				return err
			},
			fields: map[string]any{"data.id": "instance-1", "data.withdrawDesc": "撤回说明"},
		},
		{
			name: "催办", endpoint: "/web/urgeHandleRecord/sendUrgeMessage",
			send: func() error {
				_, _, err := client.ExecuteActionWrite(context.Background(), target.Session{SID: "sid"}, target.ActionWriteRequest{Action: "urge", InstanceID: "instance-1"})
				return err
			},
			fields: map[string]any{"flowInstanceId": "instance-1"},
		},
		{
			name: "转发", endpoint: "/web/flowInstanceApi/transpond",
			send: func() error {
				_, _, err := client.ExecuteActionWrite(context.Background(), target.Session{SID: "sid"}, target.ActionWriteRequest{
					Action: "forward", ReceiverID: "user-2", Name: "转发实例", FormData: formData,
				})
				return err
			},
			fields: map[string]any{"data.name": "转发实例", "receiverId": "user-2", "formDataMongoVo.data.amount": float64(12)},
		},
		{
			name: "关注", endpoint: "/web/flowInstanceApi/flowTracking",
			send: func() error {
				_, _, err := client.ExecuteActionWrite(context.Background(), target.Session{SID: "sid"}, target.ActionWriteRequest{Action: "follow", InstanceID: "instance-1", Tracking: &trackingOn})
				return err
			},
			fields: map[string]any{"data.id": "instance-1", "tracking": true},
		},
		{
			name: "取消关注", endpoint: "/web/flowInstanceApi/flowTracking",
			send: func() error {
				_, _, err := client.ExecuteActionWrite(context.Background(), target.Session{SID: "sid"}, target.ActionWriteRequest{Action: "unfollow", InstanceID: "instance-1", Tracking: &trackingOff})
				return err
			},
			fields: map[string]any{"data.id": "instance-1", "tracking": false},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			receivedPath, receivedBody = "", nil
			if err := test.send(); err != nil {
				t.Fatalf("%s请求失败：%v", test.name, err)
			}
			if receivedPath != test.endpoint {
				t.Fatalf("%s端点 = %s，期望 %s", test.name, receivedPath, test.endpoint)
			}
			assertFields(t, test.fields)
		})
	}
}
