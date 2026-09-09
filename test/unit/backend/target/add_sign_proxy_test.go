package target_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
)

// TestAppendAddSignUsersPreservesProxyTree 验证加签只修改指定节点人员明细，并保留代理文档未知字段。
func TestAppendAddSignUsersPreservesProxyTree(t *testing.T) {
	original := json.RawMessage(`{"id":"proxy-1","flowNodeTemplate":{"id":"node-1","flowNodeAuditConfig":{"userVoList":[{"id":"user-2","name":"用户二"}],"flowNodeDetailConfigList":[{"bizId":"user-1","id":"user-1","name":"已有人员","auditDetailType":"personnel"}]},"childFlowNodeTemplate":{"id":"node-2","flowNodeAuditConfig":{"flowNodeDetailConfigList":[]}}},"opaque":{"keep":true}}`)
	updated, err := target.AppendAddSignUsers(original, "node-1", []string{"user-2", "user-2"})
	if err != nil {
		t.Fatalf("追加加签人员失败：%v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(updated, &document); err != nil {
		t.Fatalf("更新后的代理树不是有效 JSON：%v", err)
	}
	if document["opaque"].(map[string]any)["keep"] != true {
		t.Fatalf("代理树未知字段未保留：%s", updated)
	}
	template := document["flowNodeTemplate"].(map[string]any)
	config := template["flowNodeAuditConfig"].(map[string]any)
	details := config["flowNodeDetailConfigList"].([]any)
	if len(details) != 2 {
		t.Fatalf("已有人员和新人员应各保留一次，实际 %d 条：%s", len(details), updated)
	}
	newDetail := details[1].(map[string]any)
	if newDetail["bizId"] != "user-2" || newDetail["id"] != "user-2" || newDetail["auditDetailType"] != "personnel" || newDetail["name"] != "用户二" {
		t.Fatalf("新增人员明细协议不正确：%v", newDetail)
	}
}

// TestAppendAddSignUsersRejectsAmbiguousNode 验证节点缺失或重复时不会生成可发送的覆盖载荷。
func TestAppendAddSignUsersRejectsAmbiguousNode(t *testing.T) {
	for name, tree := range map[string]string{
		"missing":   `{"flowNodeTemplate":{"id":"other","flowNodeAuditConfig":{"flowNodeDetailConfigList":[]}}}`,
		"ambiguous": `{"flowNodeTemplate":{"id":"node-1","flowNodeAuditConfig":{"flowNodeDetailConfigList":[]},"childFlowNodeTemplate":{"id":"node-1","flowNodeAuditConfig":{"flowNodeDetailConfigList":[]}}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := target.AppendAddSignUsers(json.RawMessage(tree), "node-1", []string{"user-1"}); err == nil {
				t.Fatal("节点结构不明确时应拒绝生成加签树")
			}
		})
	}
}

// TestHasAddSignUsersReadsPersistedPersonnel 验证加签写后只在目标完整代理树包含所有人员时确认成功。
func TestHasAddSignUsersReadsPersistedPersonnel(t *testing.T) {
	original := json.RawMessage(`{"id":"proxy-1","flowNodeTemplate":{"id":"node-1","flowNodeAuditConfig":{"flowNodeDetailConfigList":[{"bizId":"user-1","auditDetailType":"personnel"}]}}}`)
	updated, err := target.AppendAddSignUsers(original, "node-1", []string{"user-2"})
	if err != nil {
		t.Fatalf("准备加签代理树失败：%v", err)
	}
	present, err := target.HasAddSignUsers(updated, "node-1", []string{"user-1", "user-2"})
	if err != nil || !present {
		t.Fatalf("完整代理树应确认已写入人员：present=%v err=%v", present, err)
	}
	present, err = target.HasAddSignUsers(updated, "node-1", []string{"user-3"})
	if err != nil {
		t.Fatalf("核对缺失人员不应报结构错误：%v", err)
	}
	if present {
		t.Fatal("缺失人员不能被确认已写入")
	}
}

// TestBuildAddSignBodyUsesUpdateFlowProxy 验证加签正文使用完整代理协议，不再复用移交端点。
func TestBuildAddSignBodyUsesUpdateFlowProxy(t *testing.T) {
	tree := json.RawMessage(`{"id":"proxy-1","flowNodeTemplate":{"id":"node-1","flowNodeAuditConfig":{"flowNodeDetailConfigList":[]}}}`)
	body, endpoint, err := target.BuildActionBody(target.ActionWriteRequest{
		Action: "add_sign", InstanceID: "instance-1", JobTaskID: "task-1", BatchNo: "batch-1",
		NodeProxyID: "node-1", UserIDs: []string{"user-1"}, FlowProxyTree: tree,
	})
	if err != nil {
		t.Fatalf("构造加签正文失败：%v", err)
	}
	if endpoint != target.WriteEndpointUpdateFlowProxy {
		t.Fatalf("加签端点 = %q，期望 %q", endpoint, target.WriteEndpointUpdateFlowProxy)
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("序列化加签正文失败：%v", err)
	}
	if strings.Contains(string(encoded), "approverAppendVo") || strings.Contains(string(encoded), "jobTaskId") || strings.Contains(string(encoded), "batchNo") {
		t.Fatalf("加签正文混入移交字段：%s", encoded)
	}
	if !strings.Contains(string(encoded), `"flowProxyProtocol"`) || !strings.Contains(string(encoded), `"flowNodeTemplate"`) {
		t.Fatalf("加签正文缺少完整代理协议：%s", encoded)
	}
}

// TestReadFlowProxyDocumentReturnsRawDocument 验证读取流程代理时返回完整 data，而不是简化后的节点模型。
func TestReadFlowProxyDocumentReturnsRawDocument(t *testing.T) {
	const response = `{"isSuccess":true,"data":{"id":"proxy-1","opaque":{"number":1.20},"flowNodeTemplate":{"id":"node-1","flowNodeAuditConfig":{"flowNodeDetailConfigList":[]}}}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/web/flowProxy/findById" {
			t.Fatalf("读取代理使用了错误端点：%s", request.URL.Path)
		}
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()
	client := newF016Client(t, server.URL, 5*time.Second)
	document, err := client.ReadFlowProxyDocument(context.Background(), target.Session{SID: "sid"}, "proxy-1")
	if err != nil {
		t.Fatalf("读取完整流程代理失败：%v", err)
	}
	if string(document) != `{"id":"proxy-1","opaque":{"number":1.20},"flowNodeTemplate":{"id":"node-1","flowNodeAuditConfig":{"flowNodeDetailConfigList":[]}}}` {
		t.Fatalf("应保留目标原始代理文档，实际：%s", document)
	}
}
