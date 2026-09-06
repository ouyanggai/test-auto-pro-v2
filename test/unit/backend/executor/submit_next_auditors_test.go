package executor_test

import (
	"encoding/json"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// TestSubmitNextAuditorListByTargetSemantics 锁定 2026-09-07 语义勘定（TARGET_SEMANTICS 1.8 补充）：
// 提交载荷的 nextAuditorList 按下一节点的审批方式生成——
//   - 下一节点审批方式 ∈ 目标 isSettingsPerson 集合（run_node_choose/level 等）时必须带
//     {nodeProxyId: 下一节点真实代理标识, name: 节点名}，缺失即被目标以「未设置审批人」拒绝；
//   - 下一节点是固定人员类（company/initiator/assign/role 等）时不传（目标自行解析）；
//   - 手动条件分支的显式选择（SubmitBranchTargetNodeID）仍然优先，两者可共存。
func TestSubmitNextAuditorListByTargetSemantics(t *testing.T) {
	session := target.Session{SID: "sid-test", CompanyID: "company-1"}

	buildBody := func(nextAuditType string) map[string]any {
		runCtx := step.RunContext{
			Source:      "new",
			FlowProxyID: "flow-proxy-1",
			Steps: []model.CompiledActionStep{
				{Sequence: 1, Action: model.ActionSubmit, NodeKey: "node-start"},
				{Sequence: 2, Action: model.ActionApprove, NodeKey: "node-audit"},
			},
			Nodes: map[string]step.NodeInfo{
				"node-start": {Name: "发起人", TargetNodeID: "start-proxy"},
				"node-audit": {Name: "审核人1", TargetNodeID: "audit-proxy", AuditType: nextAuditType},
			},
			LastBeforeFactsKnown: true,
		}
		request, endpoint, _, err := step.BuildRequestForTest(runCtx, model.CompiledActionStep{
			Sequence: 1, Action: model.ActionSubmit, NodeKey: "node-start",
		}, session, []byte(`{}`), "node-audit")
		if err != nil {
			t.Fatalf("构造提交请求失败：%v", err)
		}
		if endpoint != target.WriteEndpointSubmit {
			t.Fatalf("端点应为提交端点，实际 %s", endpoint)
		}
		submitRequest, ok := request.(*target.SubmitFlowInstanceRequest)
		if !ok {
			t.Fatalf("请求类型错误：%T", request)
		}
		return target.BuildSubmitBody(*submitRequest)
	}

	// run_node_choose：必须带下一节点人员指定项。
	body := buildBody("run_node_choose")
	auditors, ok := body["nextAuditorList"].([]target.NextAuditor)
	if !ok || len(auditors) != 1 {
		t.Fatalf("run_node_choose 下一节点必须携带 nextAuditorList：%+v", body["nextAuditorList"])
	}
	if auditors[0].NodeProxyID != "audit-proxy" {
		t.Fatalf("人员指定项应指向下一节点真实代理标识，实际 %+v", auditors[0])
	}
	if auditors[0].Name != "审核人1" {
		t.Fatalf("人员指定项应带节点名称，实际 %+v", auditors[0])
	}

	// level：同属 isSettingsPerson 集合，也必须带。
	if body := buildBody("level"); body["nextAuditorList"] == nil {
		t.Fatal("level 下一节点也必须携带 nextAuditorList（目标取人失败时按它兜底）")
	}

	// 固定人员类（company）：目标自行解析，不传。
	if body := buildBody("company"); body["nextAuditorList"] != nil {
		t.Fatalf("固定人员类下一节点不应携带 nextAuditorList：%+v", body["nextAuditorList"])
	}

	_ = json.Marshal
}
