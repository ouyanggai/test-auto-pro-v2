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

// TestInsertAddSignNodeSplicesApprovalNode 验证加签在锚点节点与其后继之间插入新审批节点，
// 且保留代理文档未知字段；新节点形状对齐目标手动加签设计器（assign 人员明细、加签标记、字段权限复制）。
func TestInsertAddSignNodeSplicesApprovalNode(t *testing.T) {
	original := json.RawMessage(`{"id":"proxy-1","flowNodeTemplate":{"id":"node-1","flowNodeFieldPowerTemplateList":[{"formFieldTemplateEnglishName":"vacateType"}],"flowNodeAuditConfig":{"userVoList":[{"id":"user-2","name":"用户二"}],"flowNodeDetailConfigList":[{"bizId":"user-1","id":"user-1","name":"已有人员","auditDetailType":"personnel"}]},"childFlowNodeTemplate":{"id":"node-2","flowNodeAuditConfig":{"flowNodeDetailConfigList":[]}}},"opaque":{"keep":true}}`)
	updated, err := target.InsertAddSignNode(original, "node-1", []string{"user-2", "user-2"})
	if err != nil {
		t.Fatalf("插入加签节点失败：%v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(updated, &document); err != nil {
		t.Fatalf("更新后的代理树不是有效 JSON：%v", err)
	}
	if document["opaque"].(map[string]any)["keep"] != true {
		t.Fatalf("代理树未知字段未保留：%s", updated)
	}
	anchor := document["flowNodeTemplate"].(map[string]any)
	if anchor["childFlowNodeTemplate"] == nil {
		t.Fatalf("锚点节点应指向新插入的加签节点：%s", updated)
	}
	// 锚点原有人员明细保持不变：加签不再修改既有节点的人员配置。
	anchorConfig := anchor["flowNodeAuditConfig"].(map[string]any)
	if got := len(anchorConfig["flowNodeDetailConfigList"].([]any)); got != 1 {
		t.Fatalf("锚点节点人员明细不应被修改，实际 %d 条：%s", got, updated)
	}
	inserted := anchor["childFlowNodeTemplate"].(map[string]any)
	if inserted["type"] != "common" || inserted["styleType"] != true || inserted["currentDelete"] != true {
		t.Fatalf("插入节点缺少加签审批节点标记：%v", inserted)
	}
	if inserted["nodeName"] != "用户二" {
		t.Fatalf("插入节点应按人员命名，实际 %v", inserted["nodeName"])
	}
	if _, hasID := inserted["id"]; hasID {
		t.Fatal("插入节点不得携带 ID，新 ID 由目标后端所有权复制时生成")
	}
	config := inserted["flowNodeAuditConfig"].(map[string]any)
	if config["auditType"] != "assign" || config["type"] != "scramble" {
		t.Fatalf("插入节点审批配置不正确：%v", config)
	}
	details := config["flowNodeDetailConfigList"].([]any)
	if len(details) != 1 {
		t.Fatalf("插入节点人员明细应去重后只有一条，实际 %d 条：%s", len(details), updated)
	}
	detail := details[0].(map[string]any)
	if detail["bizId"] != "user-2" || detail["id"] != "user-2" || detail["auditDetailType"] != "personnel" || detail["name"] != "用户二" {
		t.Fatalf("插入节点人员明细协议不正确：%v", detail)
	}
	power := inserted["flowNodeFieldPowerTemplateList"].([]any)
	if len(power) != 1 || power[0].(map[string]any)["formFieldTemplateEnglishName"] != "vacateType" {
		t.Fatalf("插入节点应复制锚点字段权限列表：%v", power)
	}
	next := inserted["childFlowNodeTemplate"].(map[string]any)
	if next["id"] != "node-2" {
		t.Fatalf("锚点原后继应成为插入节点的后继：%v", next)
	}
}

// TestInsertAddSignNodeRejectsAmbiguousAnchor 验证锚点缺失或重复时不会生成可发送的覆盖载荷。
func TestInsertAddSignNodeRejectsAmbiguousAnchor(t *testing.T) {
	for name, tree := range map[string]string{
		"missing":   `{"flowNodeTemplate":{"id":"other","flowNodeAuditConfig":{"flowNodeDetailConfigList":[]}}}`,
		"ambiguous": `{"flowNodeTemplate":{"id":"node-1","flowNodeAuditConfig":{"flowNodeDetailConfigList":[]},"childFlowNodeTemplate":{"id":"node-1","flowNodeAuditConfig":{"flowNodeDetailConfigList":[]}}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := target.InsertAddSignNode(json.RawMessage(tree), "node-1", []string{"user-1"}); err == nil {
				t.Fatal("锚点结构不明确时应拒绝生成加签树")
			}
		})
	}
}

// TestHasAddSignNodeReadsInsertedNode 验证加签写后只在整个代理树恰好一个节点
// 以 personnel 明细包含全部加签人员时确认成功，其他类型命中不算。
func TestHasAddSignNodeReadsInsertedNode(t *testing.T) {
	original := json.RawMessage(`{"id":"proxy-1","flowNodeTemplate":{"id":"node-1","flowNodeAuditConfig":{"flowNodeDetailConfigList":[{"bizId":"user-1","auditDetailType":"personnel"},{"bizId":"user-2","auditDetailType":"initiator"}]},"childFlowNodeTemplate":{"id":"node-2","flowNodeAuditConfig":{"flowNodeDetailConfigList":[]}}}}`)
	updated, err := target.InsertAddSignNode(original, "node-1", []string{"user-2"})
	if err != nil {
		t.Fatalf("准备加签代理树失败：%v", err)
	}
	matched, err := target.HasAddSignNode(updated, []string{"user-2"})
	if err != nil {
		t.Fatalf("核对加签节点失败：%v", err)
	}
	if !matched {
		t.Fatalf("插入节点包含加签人员时应确认成功：%s", updated)
	}
	// 锚点上虽有同 ID 用户但类型是 initiator，不构成 personnel 命中。
	if matched, err := target.HasAddSignNode(original, []string{"user-2"}); err != nil || matched {
		t.Fatalf("只有非 personnel 类型命中时不应确认成功：matched=%v err=%v", matched, err)
	}
	if matched, err := target.HasAddSignNode(original, []string{"user-9"}); err != nil || matched {
		t.Fatalf("人员缺失时不应确认成功：matched=%v err=%v", matched, err)
	}
}

// TestHasAddSignNodeRejectsAmbiguousMatches 验证多个节点同时包含全部人员时无法唯一核对。
func TestHasAddSignNodeRejectsAmbiguousMatches(t *testing.T) {
	tree := json.RawMessage(`{"flowNodeTemplate":{"id":"node-1","flowNodeAuditConfig":{"flowNodeDetailConfigList":[{"bizId":"user-1","auditDetailType":"personnel"}]},"childFlowNodeTemplate":{"id":"node-2","flowNodeAuditConfig":{"flowNodeDetailConfigList":[{"bizId":"user-1","auditDetailType":"personnel"}]}}}}`)
	if _, err := target.HasAddSignNode(tree, []string{"user-1"}); err == nil {
		t.Fatal("多个节点命中时无法唯一核对，应返回错误")
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
