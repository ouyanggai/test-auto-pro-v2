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

// 2026-09-21 运行 80/82 路径 1 第 43 步缺陷回归：转发装载的标签页口径。
//
// 现象：节点回退与会话切换都正确之后，装载仍然超时：
//
//	runtime 装载失败：执行服务返回 HTTP 500：page.waitForFunction: Timeout 60000ms exceeded
//
// 根因（原生页面产物取证）：列表页的「转发」入口只在非 dueout/timedout 标签页渲染，
// 且 transpond() 委托给当前激活标签页组件（this.$refs[this.activeName].transpond(...)）；
// dueout（待发）页既无入口也无该组件方法。而"可转发的任务"只出现在待办列表里。
//
// 修复口径：转发固定装载 backlog，不按实例来源分支。
// 这条测试锁定"不退回按来源选页"——那正是导致装载等不到可转发行的原缺陷。

// TestForwardTabIsAlwaysBacklog 锁定：转发装载标签恒为 backlog（待办），
// 不得因实例来源不同改选 dueout（待发）——dueout 页没有转发入口，装载必然超时。
func TestForwardTabIsAlwaysBacklog(t *testing.T) {
	if got := step.ForwardTabForAction(); got != "backlog" {
		t.Fatalf("转发装载标签必须是 backlog（待办），实际：%q", got)
	}
}

// 2026-09-21 运行 85 路径 1 第 43 步缺陷回归：实例级转发的执行者身份。
//
// 现象：节点回退、标签页与会话切换全部生效后，装载仍然超时，页面事实给出关键证据：
//
//	{"vueRoot":true,"listFound":true,"rowCount":10,"sameInstanceRows":[]}
//
// backlog 列表渲染了 10 行，却没有一行属于本实例——说明列表是按**计划账号**的身份查询的。
// 根因：prepare 阶段只在 task/completed_task 作用域解析并切换处理人身份，
// 实例级 forward 的 preview.ActorAccount 仍是计划账号，执行服务据此注入的身份也就是计划账号。
//
// 修复口径：凡"作用于本人当前待办"的作用域都必须切换执行者身份，含 instance。

// TestInstanceScopeActsOnCurrentTodo 锁定：实例级作用域属于"作用于本人当前待办"，
// 必须在 prepare 阶段切换执行者身份（这正是运行 85 第 43 步装载超时的根因）。
func TestInstanceScopeActsOnCurrentTodo(t *testing.T) {
	for _, scope := range []model.ActionScope{
		model.ActionScopeTask, model.ActionScopeCompletedTask, model.ActionScopeInstance,
	} {
		if !step.ScopeActsOnCurrentTodo(scope) {
			t.Fatalf("作用域 %q 作用于本人当前待办，必须切换执行者身份", scope)
		}
	}
}

// TestInitiatorScopeDoesNotActOnCurrentTodo 锁定安全边界：发起人身份动作不属于本人待办，
// 不得因为本次修复被误纳入身份切换范围（否则会拿处理人身份冒充发起人）。
func TestInitiatorScopeDoesNotActOnCurrentTodo(t *testing.T) {
	if step.ScopeActsOnCurrentTodo(model.ActionScopeInitiator) {
		t.Fatal("发起人作用域不作用于本人待办，不得纳入执行者身份切换")
	}
}

// TestForwardIsInstanceScopeActingOnCurrentTodo 锁定转发这条真实链路的两项口径同时成立：
// 作用域是 instance（配置里没有节点键），且该作用域必须触发执行者身份切换。
func TestForwardIsInstanceScopeActingOnCurrentTodo(t *testing.T) {
	compiled := model.CompiledActionStep{
		Source: model.ActionStepSourceUser, Action: model.ActionForward,
		Scope: model.ActionScopeInstance, NodeKey: "",
	}
	if compiled.Scope != model.ActionScopeInstance {
		t.Fatalf("转发的配置作用域应为 instance，实际：%q", compiled.Scope)
	}
	if !step.ScopeActsOnCurrentTodo(compiled.Scope) {
		t.Fatal("转发作用于本人当前待办，必须切换执行者身份")
	}
	if compiled.NodeKey != "" {
		t.Fatalf("实例级转发不应绑定配置节点键，实际：%q", compiled.NodeKey)
	}
}

// TestForwardTabIsNotSourceDependent 锁定：转发装载标签不随实例来源变化。
// 逐个来源断言构造出的写请求都能成功，并且装载标签始终等于恒定口径——
// 防止将来重新引入"按来源选标签页"的分支（dueout 页没有转发入口，会直接导致装载超时）。
func TestForwardTabIsNotSourceDependent(t *testing.T) {
	sources := []string{"create", "pending", "backlog", "dueout", "submitted", "", "unknown"}
	for _, source := range sources {
		runCtx := instanceForwardRunCtx()
		runCtx.Source = source
		compiled := model.CompiledActionStep{
			Source: model.ActionStepSourceUser, Action: model.ActionForward,
			Scope: model.ActionScopeInstance, NodeKey: "",
		}
		_, _, payload, err := step.BuildRequestForTest(runCtx, compiled, target.Session{CompanyID: "company-id", UserID: "user-id"}, json.RawMessage(`{}`), "")
		if err != nil {
			t.Fatalf("来源 %q 下构造实例级转发请求失败：%v", source, err)
		}
		raw, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			t.Fatalf("来源 %q 下序列化转发载荷失败：%v", source, marshalErr)
		}
		// 写载荷必须始终携带配置接收人：来源变化只影响装载页，绝不影响收件人。
		if !strings.Contains(string(raw), "receiver-user-id") {
			t.Fatalf("来源 %q 下转发载荷丢失配置接收人，实际载荷：%s", source, string(raw))
		}
		if got := step.ForwardTabForAction(); got != "backlog" {
			t.Fatalf("来源 %q 下转发装载标签偏离恒定口径：%q", source, got)
		}
	}
}
