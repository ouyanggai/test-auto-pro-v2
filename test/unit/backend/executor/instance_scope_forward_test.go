package executor_test

import (
	"encoding/json"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// 2026-09-21 运行 78 路径 1 第 43 步缺陷回归：实例级转发必须能按实例当前待办取到节点。
//
// 现象：路径 1 前 40 个编译步骤全部成功（executedSteps=40、totalSteps=40），
// 只剩收尾追加的实例级 forward 失败：
//
//	目标上已无当前处理人在本节点的待办任务，无法经 runtime 执行forward
//
// 根因：forward 的作用域是 instance，配置里没有节点键（NodeKey 为空）。
// 写前刷新任务快照时按 runCtx.Nodes[""] 取目标节点 ID 必然是空串，按空节点查待办必然查空，
// 于是每一次转发都在写前被自己挡住——与目标实际状态无关。
//
// 修复口径：实例级动作的节点标识回退到实例事实里已核实的**当前活动待办节点**。
// 只回退标识，不放宽待办要求：仍然必须由写前刷新真实读到 jobTaskId。

// instanceForwardRunCtx 构造只含真实节点、不含实例动作节点键的运行上下文，
// 模拟真实编译产物形状：实例级 forward 不绑定节点。
func instanceForwardRunCtx() step.RunContext {
	return step.RunContext{
		Run:     model.Run{PlanID: 21},
		PathRun: model.PathRun{ID: 1, MainInstanceRef: "instance-ref"},
		Nodes: map[string]step.NodeInfo{
			"last-synergy-node": {Name: "发起人考勤管理", Type: "synergy", TargetNodeID: "target-synergy-id"},
		},
		ActionPersonIDs: map[string][]string{
			step.ActionPersonIndex("", model.ActionForward): {"receiver-user-id"},
		},
	}
}

// TestInstanceScopeForwardBuildsRequestWithoutNodeKey 锁定：NodeKey 为空的实例级转发
// 必须能构造出写请求，而不是因为"空节点取不到目标节点"在写前被挡。
// 这正是运行 78 第 43 步失败的路径。
func TestInstanceScopeForwardBuildsRequestWithoutNodeKey(t *testing.T) {
	runCtx := instanceForwardRunCtx()
	compiled := model.CompiledActionStep{
		Source: model.ActionStepSourceUser, Action: model.ActionForward,
		Scope: model.ActionScopeInstance, NodeKey: "",
	}
	request, endpoint, _, err := step.BuildRequestForTest(runCtx, compiled, target.Session{CompanyID: "company-id", UserID: "user-id"}, json.RawMessage(`{}`), "")
	if err != nil {
		t.Fatalf("实例级转发不应因缺少配置节点键而构造失败，实际错误：%v", err)
	}
	if request == nil {
		t.Fatal("实例级转发必须构造出写请求")
	}
	if strings.TrimSpace(endpoint) == "" {
		t.Fatal("实例级转发必须给出目标端点")
	}
}

// TestInstanceScopeForwardCarriesConfiguredReceiver 锁定：构造出的转发请求必须带上
// 配置里选定的人员，不能因为节点键为空而丢掉接收人。
func TestInstanceScopeForwardCarriesConfiguredReceiver(t *testing.T) {
	runCtx := instanceForwardRunCtx()
	compiled := model.CompiledActionStep{
		Source: model.ActionStepSourceUser, Action: model.ActionForward,
		Scope: model.ActionScopeInstance, NodeKey: "",
	}
	_, _, payload, err := step.BuildRequestForTest(runCtx, compiled, target.Session{CompanyID: "company-id", UserID: "user-id"}, json.RawMessage(`{}`), "")
	if err != nil {
		t.Fatalf("构造实例级转发请求失败：%v", err)
	}
	encoded, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		t.Fatalf("序列化转发载荷失败：%v", marshalErr)
	}
	if !strings.Contains(string(encoded), "receiver-user-id") {
		t.Fatalf("转发载荷必须携带配置选定的人员，实际载荷：%s", string(encoded))
	}
}
