package executor_test

import (
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// TestF028ApproveCarriesChosenManualBranch 锁定同意跨越手动分支路由的代选规则（实测回归：
// 缺失分支选择被目标以「手动条件分支,请选择」拒绝，运行直接落入结果待确认终局）：
//   - 下一步正好是某条已选分支的入口时，同意必须以 nextAuditorList 首项携带该分支入口的
//     目标节点 ID（目标按 nodeProxyId 匹配分支节点）；
//   - 下一步不是分支入口时不携带，绝不能把路线第一条分支错传给后面的路由；
//   - 提交/重新提交保持既有语义：精确匹配不到时回落路线第一条分支（SubmitBranchTargetNodeID）。
func TestF028ApproveCarriesChosenManualBranch(t *testing.T) {
	session := target.Session{SID: "sid-test", CompanyID: "company-1"}

	newRunCtx := func() step.RunContext {
		return step.RunContext{
			Source:      "new",
			FlowProxyID: "flow-proxy-1",
			PathRun:     model.PathRun{MainInstanceRef: "instance-1"},
			Steps: []model.CompiledActionStep{
				{Sequence: 1, Action: model.ActionApprove, NodeKey: "node-audit1"},
				{Sequence: 2, Action: model.ActionApprove, NodeKey: "node-branch1"},
			},
			Nodes: map[string]step.NodeInfo{
				"node-audit1":  {Name: "审核人1", TargetNodeID: "audit1-proxy"},
				"node-branch1": {Name: "手动分支1", TargetNodeID: "branch1-proxy"},
				"node-plain":   {Name: "审核人2", TargetNodeID: "plain-proxy", AuditType: "run_node_choose"},
			},
			BranchSelections:         map[string]string{"route-1": "branch1-proxy"},
			SubmitBranchTargetNodeID: "route-1-first-proxy",
			NextNodeAuditors: map[string][]target.NextAuditor{
				// 链路上更靠后的自选审批人节点（实测审核人1同意被「未设置审批人」拒绝）：
				// 启动时已按保存的人员策略解析，必须随载荷一并携带。
				"node-plain": {{BizID: "user-2", Name: "审批人乙"}},
			},
			LastBeforeFactsKnown: true,
		}
	}

	t.Run("同意跨手动分支必须代选该分支入口", func(t *testing.T) {
		request, endpoint, _, err := step.BuildRequestForTest(newRunCtx(), model.CompiledActionStep{
			Sequence: 1, Action: model.ActionApprove, NodeKey: "node-audit1",
		}, session, []byte(`{}`), "node-branch1")
		if err != nil {
			t.Fatalf("构造同意请求失败：%v", err)
		}
		if endpoint != target.WriteEndpointAudit {
			t.Fatalf("端点应为 %s，实际 %s", target.WriteEndpointAudit, endpoint)
		}
		audit, ok := request.(*target.AuditCurrentTaskRequest)
		if !ok {
			t.Fatalf("请求类型错误：%T", request)
		}
		if len(audit.NextAuditors) == 0 || audit.NextAuditors[0].NodeProxyID != "branch1-proxy" {
			t.Fatalf("同意必须携带已选分支入口 branch1-proxy，实际 %+v", audit.NextAuditors)
		}
		// 链路后续自选节点的人员必须带上：目标按 nodeProxyId 匹配后设置真实审批人。
		found := false
		for _, auditor := range audit.NextAuditors {
			if auditor.NodeProxyID == "plain-proxy" {
				found = true
				if auditor.BizID != "user-2" || auditor.Name != "审批人乙" || auditor.AuditDetailTyp != "personnel" {
					t.Fatalf("链路后续人员条目不完整：%+v", auditor)
				}
			}
		}
		if !found {
			t.Fatalf("同意载荷必须携带链路后续自选节点（plain-proxy）的人员，实际 %+v", audit.NextAuditors)
		}
	})

	t.Run("同意不跨分支不得携带路线第一条分支", func(t *testing.T) {
		request, _, _, err := step.BuildRequestForTest(newRunCtx(), model.CompiledActionStep{
			Sequence: 1, Action: model.ActionApprove, NodeKey: "node-audit1",
		}, session, []byte(`{}`), "node-plain")
		if err != nil {
			t.Fatalf("构造同意请求失败：%v", err)
		}
		audit, ok := request.(*target.AuditCurrentTaskRequest)
		if !ok {
			t.Fatalf("请求类型错误：%T", request)
		}
		for _, auditor := range audit.NextAuditors {
			if auditor.NodeProxyID == "route-1-first-proxy" {
				t.Fatalf("不跨分支的同意不得携带路线第一条分支，实际 %+v", audit.NextAuditors)
			}
		}
	})

	t.Run("提交保持路线第一条分支回落语义", func(t *testing.T) {
		request, _, _, err := step.BuildRequestForTest(newRunCtx(), model.CompiledActionStep{
			Sequence: 1, Action: model.ActionSubmit, NodeKey: "node-audit1",
		}, session, []byte(`{}`), "node-plain")
		if err != nil {
			t.Fatalf("构造提交请求失败：%v", err)
		}
		submit, ok := request.(*target.SubmitFlowInstanceRequest)
		if !ok {
			t.Fatalf("请求类型错误：%T", request)
		}
		found := false
		for _, auditor := range submit.NextAuditors {
			if auditor.NodeProxyID == "route-1-first-proxy" {
				found = true
			}
		}
		if !found {
			t.Fatalf("提交应回落携带路线第一条分支，实际 %+v", submit.NextAuditors)
		}
	})
}
