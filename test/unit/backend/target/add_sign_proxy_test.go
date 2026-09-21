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
	updated, err := target.InsertAddSignNode(original, "node-1", []string{"user-2", "user-2"}, nil)
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
			if _, err := target.InsertAddSignNode(json.RawMessage(tree), "node-1", []string{"user-1"}, nil); err == nil {
				t.Fatal("锚点结构不明确时应拒绝生成加签树")
			}
		})
	}
}

// TestInsertAddSignNodeKeepsPersonNameFromDirectory 锁定加签节点必须写上中文姓名这一条：
// 锚点节点自带的候选目录（userVoList）与加签用的公司人员目录是两套数据，加签的人通常不在锚点
// 候选里。只看锚点会把姓名全部丢掉，插入节点只落 ID、名称退化成通用「审核人」，
// 运行画布上也就没有审批人可显示。姓名必须由调用方按目标公司人员目录核实后传入。
func TestInsertAddSignNodeKeepsPersonNameFromDirectory(t *testing.T) {
	// 锚点候选目录里没有 user-3，模拟「加签的人不在锚点处理人范围」这一常见现场。
	original := json.RawMessage(`{"id":"proxy-1","flowNodeTemplate":{"id":"node-1","flowNodeAuditConfig":{"userVoList":[{"id":"user-9","name":"无关人员"}],"flowNodeDetailConfigList":[]},"childFlowNodeTemplate":{"id":"node-2"}}}`)
	updated, err := target.InsertAddSignNode(original, "node-1", []string{"user-3"}, map[string]string{"user-3": "张三"})
	if err != nil {
		t.Fatalf("插入加签节点失败：%v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(updated, &document); err != nil {
		t.Fatalf("更新后的代理树不是有效 JSON：%v", err)
	}
	inserted := document["flowNodeTemplate"].(map[string]any)["childFlowNodeTemplate"].(map[string]any)
	if inserted["nodeName"] != "张三" {
		t.Fatalf("插入节点应使用目录核实的中文姓名命名，实际 %v：%s", inserted["nodeName"], updated)
	}
	config := inserted["flowNodeAuditConfig"].(map[string]any)
	details := config["flowNodeDetailConfigList"].([]any)
	if len(details) != 1 {
		t.Fatalf("插入节点人员明细应只有一条，实际 %d 条：%s", len(details), updated)
	}
	detail := details[0].(map[string]any)
	if detail["name"] != "张三" {
		t.Fatalf("人员明细必须带上中文姓名（目标靠它渲染审批人），实际 %v：%s", detail["name"], updated)
	}
	if detail["bizId"] != "user-3" {
		t.Fatalf("人员明细 ID 应为加签目标人员，实际 %v", detail["bizId"])
	}
}

// TestInsertAddSignNodeFallsBackWhenNameMissing 锁定姓名取不到时的兜底：
// 调用方给不出姓名时保持目标协议最小字段（只落 ID），不编造姓名，也不因此拒绝发送。
func TestInsertAddSignNodeFallsBackWhenNameMissing(t *testing.T) {
	original := json.RawMessage(`{"id":"proxy-1","flowNodeTemplate":{"id":"node-1","flowNodeAuditConfig":{"flowNodeDetailConfigList":[]},"childFlowNodeTemplate":{"id":"node-2"}}}`)
	updated, err := target.InsertAddSignNode(original, "node-1", []string{"user-3"}, map[string]string{})
	if err != nil {
		t.Fatalf("姓名缺失不应阻断加签：%v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(updated, &document); err != nil {
		t.Fatalf("更新后的代理树不是有效 JSON：%v", err)
	}
	inserted := document["flowNodeTemplate"].(map[string]any)["childFlowNodeTemplate"].(map[string]any)
	details := inserted["flowNodeAuditConfig"].(map[string]any)["flowNodeDetailConfigList"].([]any)
	detail := details[0].(map[string]any)
	if _, hasName := detail["name"]; hasName {
		t.Fatalf("姓名未知时不得编造 name 字段：%v", detail)
	}
	if inserted["nodeName"] != "审核人" {
		t.Fatalf("姓名未知时节点名应退化为通用名，实际 %v", inserted["nodeName"])
	}
}

// TestHasAddSignNodeReadsInsertedNode 验证加签写后只在整个代理树恰好一个节点
// 以 personnel 明细包含全部加签人员时确认成功，其他类型命中不算。
func TestHasAddSignNodeReadsInsertedNode(t *testing.T) {
	original := json.RawMessage(`{"id":"proxy-1","flowNodeTemplate":{"id":"node-1","flowNodeAuditConfig":{"flowNodeDetailConfigList":[{"bizId":"user-1","auditDetailType":"personnel"},{"bizId":"user-2","auditDetailType":"initiator"}]},"childFlowNodeTemplate":{"id":"node-2","flowNodeAuditConfig":{"flowNodeDetailConfigList":[]}}}}`)
	updated, err := target.InsertAddSignNode(original, "node-1", []string{"user-2"}, nil)
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

// TestAddSignWalkersHandleBranchAliases 验证遍历严格沿流程树拓扑前进，不再盲目递归所有字段。
// 目标后端返回的节点对象同时带有 branchNodes / conditionNodes / parallelNodes 三组互为别名的
// 分支字段，指向同一批节点对象；旧实现按 object 的所有 key 递归会把同一节点计数多次，
// 导致加签锚点被误报"出现多次"、加签核对被误报"命中多个节点"。
func TestAddSignWalkersHandleBranchAliases(t *testing.T) {
	branch := `{"id":"cond-1","childFlowNodeTemplate":{"id":"node-branch","flowNodeAuditConfig":{"flowNodeDetailConfigList":[]}}}`
	tree := json.RawMessage(`{"id":"proxy-1","flowNodeTemplate":{"id":"node-1","flowNodeAuditConfig":{"flowNodeDetailConfigList":[]},"childFlowNodeTemplate":{"id":"node-2","flowNodeAuditConfig":{"flowNodeDetailConfigList":[]}},"conditionNodes":[` + branch + `],"parallelNodes":[` + branch + `],"branchNodes":[` + branch + `]}}`)
	// 别名分支不应让锚点被判为重复：这里必须能正常插入，且只在拓扑位置插入一次。
	updated, err := target.InsertAddSignNode(tree, "node-2", []string{"user-3"}, nil)
	if err != nil {
		t.Fatalf("分支别名不应导致锚点重复计数：%v", err)
	}
	if !strings.Contains(string(updated), "user-3") {
		t.Fatalf("插入的加签人员应落在代理树中：%s", updated)
	}
}

// TestAddSignWalkersTraverseConditionBranch 验证条件分支内的节点能被定位并命中人员核对：
// 分支节点的处理人属于路径末端节点，只有沿 conditionNodes 递归才能到达。
func TestAddSignWalkersTraverseConditionBranch(t *testing.T) {
	branch := `{"id":"cond-1","childFlowNodeTemplate":{"id":"node-branch","flowNodeAuditConfig":{"flowNodeDetailConfigList":[{"bizId":"user-7","auditDetailType":"personnel"}]}}}`
	tree := json.RawMessage(`{"id":"proxy-1","flowNodeTemplate":{"id":"node-1","flowNodeAuditConfig":{"flowNodeDetailConfigList":[]},"conditionNodes":[` + branch + `]}}`)
	matched, err := target.HasAddSignNode(tree, []string{"user-7"})
	if err != nil {
		t.Fatalf("条件分支核对失败：%v", err)
	}
	if !matched {
		t.Fatalf("条件分支内的加签人员应被核对命中：%s", tree)
	}
	// 锚点位于条件分支内时也必须能被找到，否则加签直接拒绝发送。
	if _, err := target.InsertAddSignNode(tree, "node-branch", []string{"user-8"}, nil); err != nil {
		t.Fatalf("条件分支内的锚点应可定位：%v", err)
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
