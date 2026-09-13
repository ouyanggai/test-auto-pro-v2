package executor_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
)

// TestF019ApproveHasWritePayload 锁定同意动作可执行：同意与不同意共用 /flowInstanceApi/audit，
// 差别只在 auditRecord.auditStatus=pass。此前统一动作载荷构造器里没有 approve 分支，
// 会落到 default 返回 UNSUPPORTED_ACTION——失败安全但审批路径永远无法推进。
func TestF019ApproveHasWritePayload(t *testing.T) {
	body, endpoint, err := target.BuildActionBody(target.ActionWriteRequest{
		Action: "approve", InstanceID: "i-1", JobTaskID: "task-1", FlowProxyID: "flow-1",
		AuditStatus: "pass", ExecuteDesc: "同意说明",
		FormData:     []byte(`{"amount":"12.30"}`),
		NextAuditors: []target.NextAuditor{{Name: "下一节点处理人", BizID: "user-9"}},
	})
	if err != nil {
		t.Fatalf("同意动作必须有载荷分支：%v", err)
	}
	if endpoint != target.WriteEndpointAudit {
		t.Fatalf("同意必须走审批端点，实际 %s", endpoint)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("载荷缺少 data 容器：%v", body)
	}
	if data["id"] != "i-1" || data["jobTaskId"] != "task-1" {
		t.Fatalf("实例与待办标识必须携带：%v", data)
	}
	if data["flowProxyId"] != "flow-1" {
		t.Fatalf("同意必须携带流程代理 id（下一节点选人依赖）：%v", data)
	}
	auditRecord, ok := data["auditRecord"].(map[string]any)
	if !ok || auditRecord["auditStatus"] != "pass" {
		t.Fatalf("同意必须以 auditStatus=pass 表达：%v", data["auditRecord"])
	}
	if auditRecord["executeDesc"] != "同意说明" {
		t.Fatalf("同意说明必须携带：%v", auditRecord)
	}
	auditors, ok := body["nextAuditorList"].([]target.NextAuditor)
	if !ok || len(auditors) != 1 || auditors[0].BizID != "user-9" {
		t.Fatalf("下一节点选人参数必须透传：%v", body["nextAuditorList"])
	}
	form, ok := body["formDataMongoVo"].(map[string]any)
	if !ok {
		t.Fatalf("同意同样整份提交表单数据（目标保存是整份覆盖）：%v", body)
	}
	if got := string(form["data"].(json.RawMessage)); got != `{"amount":"12.30"}` {
		t.Fatalf("表单数据必须按原始文本透传，实际 %s", got)
	}
}

// TestF019ApproveValidateRejectsWrongAuditStatus 锁定同意动作的发送前校验：
// 实例与待办标识缺失、auditStatus 不是 pass 都必须在发出前拒绝，
// 与 reject 的 no_pass 校验同源，防止把空枚举发给目标形成默认行为。
func TestF019ApproveValidateRejectsWrongAuditStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("校验失败时不得发出任何请求")
	}))
	defer server.Close()
	client, err := target.NewClient(target.ClientConfig{
		BaseURL:       server.URL,
		LoginPassword: "test-password",
		LoginAESKey:   "0123456789abcdef0123456789abcdef",
		Timeout:       5 * time.Second,
	})
	if err != nil {
		t.Fatalf("构造客户端失败：%v", err)
	}
	session := target.Session{SID: "sid-test"}
	cases := []struct {
		name    string
		request target.ActionWriteRequest
	}{
		{"缺少待办任务", target.ActionWriteRequest{Action: "approve", InstanceID: "i-1", AuditStatus: "pass"}},
		{"auditStatus为空", target.ActionWriteRequest{Action: "approve", InstanceID: "i-1", JobTaskID: "task-1"}},
		{"auditStatus不是pass", target.ActionWriteRequest{Action: "approve", InstanceID: "i-1", JobTaskID: "task-1", AuditStatus: "no_pass"}},
	}
	for _, tc := range cases {
		_, _, err := client.ExecuteActionWrite(nil, session, tc.request)
		if err == nil {
			t.Fatalf("%s：必须被发送前校验拒绝", tc.name)
		}
		var invalid *target.RequestValidationError
		if !errors.As(err, &invalid) {
			t.Fatalf("%s：拒绝必须是参数校验错误，实际 %v", tc.name, err)
		}
	}
}
