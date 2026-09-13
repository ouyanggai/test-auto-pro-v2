package executor_test

import (
	"encoding/json"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// F-034 评审修复 #2/#6：isSkip 三种场景的跳过/阻塞分型与分支解析失败的无写入验证。
// 全部为纯构造测试：不发出任何目标请求（无 HTTP、无 session）。

// f034Bool 是布尔指针辅助。
func f034Bool(value bool) *bool { return &value }

// f034Step 构造一个审批同意步骤。
func f034Step(nodeKey string) model.CompiledActionStep {
	return model.CompiledActionStep{
		Source: model.ActionStepSourceUser, Action: model.ActionApprove,
		Scope: model.ActionScopeTask, NodeKey: nodeKey,
	}
}

// f034SkipRunContext 构造含手动分支路由、空入口与业务节点的图上下文（路径 8 形状）。
func f034SkipRunContext(currentNodeKey string, isSkip *bool, auditType string) step.RunContext {
	const (
		routeNodeID = "route-target"
		emptyNodeID = "empty-target"
		businessID  = "business-target"
	)
	return step.RunContext{
		Nodes: map[string]step.NodeInfo{
			"route":    {Name: "分支节点", Type: "manual", TargetNodeID: routeNodeID, IsSkip: isSkip, AuditType: auditType},
			"empty":    {Name: "空节点", Type: "empty", TargetNodeID: emptyNodeID},
			"business": {Name: "审核人", Type: "common", TargetNodeID: businessID},
		},
		BranchSelections: map[string]string{routeNodeID: emptyNodeID},
		GraphEdges: map[string][]step.GraphEdgeInfo{
			"route-target":   {{Target: emptyNodeID}},
			"empty-target":   {{Target: businessID}},
			"business-target": {},
		},
		GraphNodeTypes: map[string]string{routeNodeID: "manual", emptyNodeID: "empty", businessID: "common"},
	}
}

// TestF034SkipDeclaredAllowsSkipResolution 锁定 isSkip=true 的解析结果：解析为“目标已跳过”语义。
func TestF034SkipDeclaredAllowsSkipResolution(t *testing.T) {
	runCtx := f034SkipRunContext("route", f034Bool(true), "company")
	runCtx.Nodes["route"] = step.NodeInfo{Name: "分支节点", Type: "manual", TargetNodeID: "route-target", IsSkip: f034Bool(true)}
	// 节点声明允许跳过时，解析器层面不做跳过判定（跳过由待办位置 + isSkip 共同决定）；
	// 这里锁定解析器不阻塞、正常携带空入口。
	entries, blockReason := step.BranchEntriesForTransitionForTest("route", "business", runCtx)
	if blockReason != "" {
		t.Fatalf("isSkip=true 且分支已保存时不应阻塞：%s", blockReason)
	}
	if len(entries) != 1 || entries[0] != "empty-target" {
		t.Fatalf("应携带空入口 nodeProxyId：%v", entries)
	}
}

// TestF034BuildRequestNoFallbackWithoutSelection 锁定评审 #6：
// 分支存在但无法解析入口时 BuildRequest 必须返回阻塞错误，绝不回落第一条分支，
// 更不能构造出 /flowInstanceApi/audit 载荷（无写入）。
func TestF034BuildRequestNoFallbackWithoutSelection(t *testing.T) {
	for _, tc := range []struct {
		name   string
		action model.ActionKey
	}{
		{"提交", model.ActionSubmit},
		{"同意", model.ActionApprove},
		{"重新提交", model.ActionResubmit},
	} {
		runCtx := f034SkipRunContext("route", nil, "company")
		// 分支节点没有已保存选择：解析器必须阻塞。
		delete(runCtx.BranchSelections, "route-target")
		stepRec := f034Step("route")
		stepRec.Action = tc.action
		_, _, _, err := step.BuildRequestForTest(runCtx, stepRec, target.Session{CompanyID: "c-1"}, json.RawMessage(`{}`), "business")
		if err == nil {
			t.Fatalf("%s：分支无法解析时必须阻塞，不得构造载荷", tc.name)
		}
		if !strings.Contains(err.Error(), "入口") {
			t.Fatalf("%s：阻塞原因应说明找不到入口：%v", tc.name, err)
		}
	}
}

// TestF034BuildRequestCrossEmptyNodeCarriesEntry 锁定评审 #2 的正向场景：
// 分支已保存且入口是空节点时，载荷携带空入口 nodeProxyId 且不伪造 bizId/人员。
func TestF034BuildRequestCrossEmptyNodeCarriesEntry(t *testing.T) {
	runCtx := f034SkipRunContext("route", f034Bool(true), "company")
	runCtx.Nodes["route"] = step.NodeInfo{Name: "分支节点", Type: "manual", TargetNodeID: "route-target", AuditType: "company"}
	stepRec := f034Step("route")
	_, endpoint, body, err := step.BuildRequestForTest(runCtx, stepRec, target.Session{CompanyID: "c-1"}, json.RawMessage(`{}`), "business")
	if err != nil {
		t.Fatalf("分支已保存且入口可解析时不应阻塞：%v", err)
	}
	if endpoint != target.WriteEndpointAudit {
		t.Fatalf("同意应走审批端点：%s", endpoint)
	}
	auditors, ok := body["nextAuditorList"].([]target.NextAuditor)
	if !ok || len(auditors) != 1 || auditors[0].NodeProxyID != "empty-target" {
		t.Fatalf("应携带空入口 nodeProxyId 且不伪造人员：%v", body["nextAuditorList"])
	}
}
