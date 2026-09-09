package executor_test

import (
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// TestInitiationNextAuditorListByTargetSemantics 锁定 2026-09-07 语义勘定：
// 提交和重新提交的 nextAuditorList 都按下一节点的审批方式生成——
//   - run_node_choose 必须带启动时按当前目录解析出的真实 {bizId,name,auditDetailType,nodeProxyId}；
//   - level 等由目标按组织/表单上下文解析的审批方式不传伪造人员（由目标自行解析）；
//   - 下一节点是固定人员类（company/initiator/assign/role 等）时不传（目标自行解析）；
//   - 手动条件分支的显式选择（SubmitBranchTargetNodeID）仍然优先，两者可共存。
func TestInitiationNextAuditorListByTargetSemantics(t *testing.T) {
	session := target.Session{SID: "sid-test", CompanyID: "company-1"}

	buildBody := func(action model.ActionKey, nextAuditType string) map[string]any {
		runCtx := step.RunContext{
			Source:      "new",
			FlowProxyID: "flow-proxy-1",
			PathRun:     model.PathRun{MainInstanceRef: "instance-1"},
			Steps: []model.CompiledActionStep{
				{Sequence: 1, Action: action, NodeKey: "node-start"},
				{Sequence: 2, Action: model.ActionApprove, NodeKey: "node-audit"},
			},
			Nodes: map[string]step.NodeInfo{
				"node-start": {Name: "发起人", TargetNodeID: "start-proxy"},
				"node-audit": {Name: "审核人1", TargetNodeID: "audit-proxy", AuditType: nextAuditType},
			},
			NextNodeAuditors: map[string][]target.NextAuditor{
				"node-audit": {{BizID: "user-1", Name: "审批人甲", AuditDetailTyp: "personnel"}},
			},
			LastBeforeFactsKnown: true,
		}
		request, endpoint, _, err := step.BuildRequestForTest(runCtx, model.CompiledActionStep{
			Sequence: 1, Action: action, NodeKey: "node-start",
		}, session, []byte(`{}`), "node-audit")
		if err != nil {
			t.Fatalf("构造%s请求失败：%v", action, err)
		}
		expectedEndpoint := target.WriteEndpointSubmit
		if action == model.ActionResubmit {
			expectedEndpoint = target.WriteEndpointReSubmit
		}
		if endpoint != expectedEndpoint {
			t.Fatalf("端点应为 %s，实际 %s", expectedEndpoint, endpoint)
		}
		switch typed := request.(type) {
		case *target.SubmitFlowInstanceRequest:
			return target.BuildSubmitBody(*typed)
		case *target.ActionWriteRequest:
			body, _, buildErr := target.BuildActionBody(*typed)
			if buildErr != nil {
				t.Fatalf("构造%s载荷失败：%v", action, buildErr)
			}
			return body
		default:
			t.Fatalf("请求类型错误：%T", request)
			return nil
		}
	}

	for _, action := range []model.ActionKey{model.ActionSubmit, model.ActionResubmit} {
		t.Run(string(action), func(t *testing.T) {
			// run_node_choose：必须带下一节点人员指定项。
			body := buildBody(action, "run_node_choose")
			auditors, ok := body["nextAuditorList"].([]target.NextAuditor)
			if !ok || len(auditors) != 1 {
				t.Fatalf("run_node_choose 下一节点必须携带 nextAuditorList：%+v", body["nextAuditorList"])
			}
			if auditors[0].NodeProxyID != "audit-proxy" || auditors[0].BizID != "user-1" || auditors[0].AuditDetailTyp != "personnel" {
				t.Fatalf("人员指定项应带下一节点真实人员和节点标识，实际 %+v", auditors[0])
			}
			if auditors[0].Name != "审批人甲" {
				t.Fatalf("人员指定项应带真实人员名称，实际 %+v", auditors[0])
			}

			// level：目标自行按组织层级解析，不能发送只有节点名的伪造项。
			if body := buildBody(action, "level"); body["nextAuditorList"] != nil {
				t.Fatalf("level 不应携带伪造 nextAuditorList：%+v", body["nextAuditorList"])
			}

			// 固定人员类（company）：目标自行解析，不传。
			if body := buildBody(action, "company"); body["nextAuditorList"] != nil {
				t.Fatalf("固定人员类下一节点不应携带 nextAuditorList：%+v", body["nextAuditorList"])
			}
		})
	}

}
