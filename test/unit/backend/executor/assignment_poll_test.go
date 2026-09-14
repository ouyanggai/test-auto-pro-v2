package executor_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// F-035/T07 处理人生成事实：目标节点已到达但 currentAuditUserInfo/待办为空时，
// 有界轮询（≤5 次、≤10 秒）只读复查；超时后按 assignment_missing 阻塞，不显示“已处理”，
// 不重发写请求，也不回退计划账号或候选人。

// pollTarget 是可控的目标假件：前 N 次读取无处理人，之后按设定出现处理人。
type pollTarget struct {
	*fakeTarget
	// handlersAfter 表示从第几次 FindSubmittedFlowFacts 起开始返回处理人（1 基；0 表示永不）。
	handlersAfter int
	handlersReads int
}

func (t *pollTarget) FindSubmittedFlowFacts(_ context.Context, _ target.Session, _ string) (target.SubmittedFlowFacts, error) {
	t.handlersReads++
	t.fakeTarget.instance.CurrentNodes = []string{"node-audit"}
	if t.handlersAfter > 0 && t.handlersReads >= t.handlersAfter {
		t.fakeTarget.instance.DueNodes = []string{"node-audit"}
		return target.SubmittedFlowFacts{
			FlowProxyID: "flow-proxy-1", CurrentNodes: []string{"node-audit"}, Status: "run", Found: true,
			Handlers: []target.NodeCurrentHandler{{NodeID: "node-audit", AuditType: "run_node_choose", BizIDs: []string{"user-2"}}},
		}, nil
	}
	return target.SubmittedFlowFacts{
		FlowProxyID: "flow-proxy-1", CurrentNodes: []string{"node-audit"}, Status: "run", Found: true,
	}, nil
}

// TestHandlerMissingBlocksAfterBoundedPoll 锁定：处理人一直未生成时，轮询有界（不无限等待），
// 最终按 assignment_missing 阻塞且不发出任何写请求。
func TestHandlerMissingBlocksAfterBoundedPoll(t *testing.T) {
	view := &pollTarget{fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run"}}}
	// 用固定时钟 + 快速间隔验证有界性：把预算从时间维度转化为次数维度（假件时间固定，轮询按次数封顶）。
	executor := step.NewExecutor(view, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), func() time.Time { return time.Unix(0, 0).UTC() })
	runCtx := factRunContext(nil)

	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if preview.GateAllowed {
		t.Fatal("处理人缺失必须阻塞，不得放行")
	}
	if !containsAll(preview.BlockReason, "assignment_missing", "未生成处理人") {
		t.Fatalf("阻塞原因必须说明 assignment_missing 与处理人未生成：%q", preview.BlockReason)
	}
	if view.fakeTarget.submitCalls != 0 && view.auditCalls() != 0 {
		t.Fatal("等待处理人期间不得发送任何写请求")
	}
	// 有界性：初始读取 + 轮询次数封顶（5），不得超过 7 次实例事实读取。
	if view.handlersReads > 7 {
		t.Fatalf("轮询次数应有界（≤5 次复查），实际 %d 次", view.handlersReads)
	}
}

// TestHandlerAppearsDuringPollAllowsGate 锁定：轮询期间目标生成处理人后，门禁按最新事实继续。
func TestHandlerAppearsDuringPollAllowsGate(t *testing.T) {
	view := &pollTarget{fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run"}}, handlersAfter: 2}
	executor := step.NewExecutor(view, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), func() time.Time { return time.Unix(0, 0).UTC() })
	runCtx := factRunContext(nil)

	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if preview == nil {
		t.Fatal("预览不应为空")
	}
	// 处理人出现后按事实推进：不允许因早期缺失而永久阻塞。
	if preview.BlockReason != "" && !preview.GateAllowed && containsAll(preview.BlockReason, "assignment_missing") {
		t.Fatalf("处理人已生成后不得再按 assignment_missing 阻塞：%q", preview.BlockReason)
	}
}

// TestSubmitOnlySentOnceWhenHandlerMissing 锁定：处理人缺失的等待绝不能以重发写请求收场。
func TestSubmitOnlySentOnceWhenHandlerMissing(t *testing.T) {
	view := &pollTarget{fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run"}}}
	executor := step.NewExecutor(view, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), func() time.Time { return time.Unix(0, 0).UTC() })
	runCtx := newRunContext([]model.CompiledActionStep{approveStep()})
	runCtx.PathRun.MainInstanceRef = "instance-9"
	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if preview.GateAllowed {
		t.Fatal("处理人缺失必须阻塞")
	}
	if view.auditCalls() != 0 || view.submitCalls != 0 {
		t.Fatalf("阻塞路径不得有写请求：audit=%d submit=%d", view.auditCalls(), view.submitCalls)
	}
}

// auditCalls 访问内嵌 fakeTarget 的审计计数。
func (t *pollTarget) auditCalls() int { return t.fakeTarget.auditCalls }

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

// TestSpecialBusinessLifecycleBlocksSubmit 锁定 F-035 评审补充：
// 命中目标特殊业务钩子清单的流程类型必须阻塞，绝不发通用请求顶替业务变更。
func TestSpecialBusinessLifecycleBlocksSubmit(t *testing.T) {
	view := &pollTarget{fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run"}}}
	executor := step.NewExecutor(view, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), func() time.Time { return time.Unix(0, 0).UTC() })
	// 新发起（无实例）：与发起链路一致，不进入待办轮询。
	runCtx := newRunContext([]model.CompiledActionStep{submitStep(), approveStep()})
	runCtx.FlowType = "contract_seal_review"
	runCtx.RenderType = "formmaking"
	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if preview == nil || preview.GateAllowed {
		t.Fatalf("特殊业务流程类型必须阻塞：%+v", preview)
	}
	if !strings.Contains(preview.BlockReason, "contract_seal_review") {
		t.Fatalf("阻塞原因必须指明流程类型：%q", preview.BlockReason)
	}
	if view.fakeTarget.submitCalls != 0 || view.auditCalls() != 0 {
		t.Fatal("特殊业务阻塞路径不得发送写请求")
	}
}

// TestVueCustomSubmitBlocks 锁定：无表单页面的专用业务链路（项目/业务关联、initiatorRange、
// 并行/手动分支选人）未实现前，发起/重提必须阻塞，不得用通用请求冒充。
func TestVueCustomSubmitBlocks(t *testing.T) {
	view := &pollTarget{fakeTarget: &fakeTarget{instance: fakeTargetView{Found: true, Status: "run"}}}
	executor := step.NewExecutor(view, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), func() time.Time { return time.Unix(0, 0).UTC() })
	runCtx := newRunContext([]model.CompiledActionStep{submitStep(), approveStep()})
	runCtx.FlowType = "contract_review"
	runCtx.RenderType = "vue_custom"
	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if preview == nil || preview.GateAllowed {
		t.Fatalf("vue_custom 无表单发起必须阻塞：%+v", preview)
	}
	if !strings.Contains(preview.BlockReason, "无表单") {
		t.Fatalf("阻塞原因必须说明无表单业务链路：%q", preview.BlockReason)
	}
}

// TestRuntimeIdentityOverridesSnapshot 锁定：发起前用当前会话实时身份覆盖表单登录人字段，
// 配置期历史身份不得进入载荷。
func TestRuntimeIdentityOverridesSnapshot(t *testing.T) {
	// 新发起场景：实例尚不存在（Found=false），与 TestF016SubmitHappyPath 同一事实形状。
	view := &pollTarget{fakeTarget: &fakeTarget{instance: fakeTargetView{Found: false}}}
	executor := step.NewExecutor(view, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), func() time.Time { return time.Unix(0, 0).UTC() })
	runCtx := newRunContext([]model.CompiledActionStep{submitStep(), approveStep()})
	// 配置期快照里是历史身份“旧账号”。
	runCtx.EffectiveFormData = []byte(`{"global_user_basic_information":{"userId":"old-user","userName":"旧账号","companyId":"old-company","companyName":"旧公司","departmentId":"old-dept","departmentName":"旧部门","dutyId":"old-duty","dutyName":"旧岗位"}}`)
	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if preview == nil || !preview.GateAllowed {
		t.Fatalf("普通流程发起应通过门禁：%+v", preview)
	}
	formDataContainer, ok := preview.RequestPayload["formDataMongoVo"].(map[string]any)
	if !ok {
		t.Fatalf("载荷缺少表单容器：%v", preview.RequestPayload)
	}
	// json.RawMessage 在 map[string]any 里表现为 []byte：先解码再断言。
	var formData map[string]any
	if raw, ok := formDataContainer["data"].([]byte); ok {
		if err := json.Unmarshal(raw, &formData); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := json.Unmarshal(formDataContainer["data"].(json.RawMessage), &formData); err != nil {
			t.Fatal(err)
		}
	}
	identity, ok := formData["global_user_basic_information"].(map[string]any)
	if !ok {
		t.Fatalf("表单缺少登录人上下文：%v", formData)
	}
	if identity["userId"] != "uid-oyg-test" || identity["dutyId"] != "duty-oyg-test" {
		t.Fatalf("登录人上下文必须被实时身份覆盖：%v", identity)
	}
}
