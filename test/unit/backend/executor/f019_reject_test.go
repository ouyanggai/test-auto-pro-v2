package executor_test

import (
	"encoding/json"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
)

// TestF019RejectHasWritePayload 锁定不同意动作可执行：不同意与同意共用 /flowInstanceApi/audit，
// 差别只在 auditRecord.auditStatus=no_pass。此前统一动作载荷构造器里没有 reject 分支，
// 会落到 default 返回 UNSUPPORTED_ACTION——失败安全但动作全集并未齐备。
func TestF019RejectHasWritePayload(t *testing.T) {
	body, endpoint, err := target.BuildActionBody(target.ActionWriteRequest{
		Action: "reject", InstanceID: "i-1", JobTaskID: "task-1", FlowProxyID: "flow-1",
		AuditStatus: "no_pass", ExecuteDesc: "不同意说明",
		FormData: []byte(`{"amount":"12.30"}`),
	})
	if err != nil {
		t.Fatalf("不同意动作必须有载荷分支：%v", err)
	}
	if endpoint != target.WriteEndpointAudit {
		t.Fatalf("不同意必须走审批端点，实际 %s", endpoint)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("载荷缺少 data 容器：%v", body)
	}
	if data["id"] != "i-1" || data["jobTaskId"] != "task-1" {
		t.Fatalf("实例与待办标识必须携带：%v", data)
	}
	auditRecord, ok := data["auditRecord"].(map[string]any)
	if !ok || auditRecord["auditStatus"] != "no_pass" {
		t.Fatalf("不同意必须以 auditStatus=no_pass 表达：%v", data["auditRecord"])
	}
	if auditRecord["executeDesc"] != "不同意说明" {
		t.Fatalf("不同意说明必须携带：%v", auditRecord)
	}
	form, ok := body["formDataMongoVo"].(map[string]any)
	if !ok {
		t.Fatalf("不同意同样整份提交表单数据（目标保存是整份覆盖）：%v", body)
	}
	if got := string(form["data"].(json.RawMessage)); got != `{"amount":"12.30"}` {
		t.Fatalf("表单数据必须按原始文本透传，实际 %s", got)
	}
}
