package executor_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// f024RunContext 构造一条两节点路线：发起人只能改自己的字段，审批节点独占一批只有它能改的字段。
func f024RunContext(effective string) step.RunContext {
	runCtx := newRunContext([]model.CompiledActionStep{submitStep(), approveStep()})
	runCtx.EffectiveFormData = []byte(effective)
	runCtx.Nodes = map[string]step.NodeInfo{
		"node-start": {Name: "发起人", Type: "start", TargetNodeID: "real-start",
			EditableFields: []string{"contractSum", "classificationId"}},
		"node-audit": {Name: "部门审批", Type: "审批", TargetNodeID: "real-audit",
			EditableFields: []string{"accountantOpinion", "contractSum"}},
	}
	runCtx.NodeEditableFields = map[string][]string{
		"node-start": {"contractSum", "classificationId"},
		"node-audit": {"accountantOpinion", "contractSum"},
	}
	return runCtx
}

// decodeFormPayload 把构造出来的载荷按原始文本解析为键值对，便于逐字核对数字字面量。
func decodeFormPayload(t *testing.T, payload json.RawMessage) map[string]json.RawMessage {
	t.Helper()
	if len(payload) == 0 {
		return map[string]json.RawMessage{}
	}
	values := map[string]json.RawMessage{}
	if err := json.Unmarshal(payload, &values); err != nil {
		t.Fatalf("载荷不是合法 JSON 对象：%v", err)
	}
	return values
}

// TestF024SubmitDropsFieldsOnlyLaterNodesCanEdit 锁定发起载荷的构造规则：
// 只有后续节点才能编辑的字段不能出现在发起请求里——真实发起人填不出来那些值；
// 发起人可编辑字段、以及没有任何节点声明的表单自身伴生键照常提交。
func TestF024SubmitDropsFieldsOnlyLaterNodesCanEdit(t *testing.T) {
	runCtx := f024RunContext(`{"contractSum":12.30,"classificationId":["c-1"],"classificationId__virtualName":"施工类","classificationName":"施工类","accountantOpinion":"历史意见","initiatorId":"u-1"}`)
	plan, err := step.BuildNodeFormData(runCtx, submitStep(), nil)
	if err != nil {
		t.Fatalf("构造发起表单数据失败：%v", err)
	}
	values := decodeFormPayload(t, plan.Payload)
	if _, exists := values["accountantOpinion"]; exists {
		t.Fatalf("只有审批节点能编辑的字段不得出现在发起请求里：%v", values)
	}
	for _, key := range []string{"contractSum", "classificationId", "classificationId__virtualName", "classificationName", "initiatorId"} {
		if _, exists := values[key]; !exists {
			t.Fatalf("发起请求缺少应当携带的字段 %s：%v", key, values)
		}
	}
	if got := string(values["contractSum"]); got != "12.30" {
		t.Fatalf("数字字面量必须原样保留，实际 %s", got)
	}
	if !containsField(plan.Withheld, "accountantOpinion") {
		t.Fatalf("按权限未携带的字段必须如实记录：%v", plan.Withheld)
	}
	if plan.BaseFromInstance {
		t.Fatal("发起时没有实例，基线不能声称来自实例")
	}
}

// TestF024ApproveMergesInstanceDataAndOverlaysOnlyNodeEditable 锁定审批载荷的构造规则：
// 目标保存表单数据是整份覆盖，所以基线必须是实例当前完整数据；
// 只覆盖本节点声明可编辑的配置字段，上游处理人填过的内容一个都不许被历史快照盖掉。
func TestF024ApproveMergesInstanceDataAndOverlaysOnlyNodeEditable(t *testing.T) {
	runCtx := f024RunContext(`{"contractSum":500001,"accountantOpinion":"配置意见","classificationId":["c-1"],"classificationName":"施工类"}`)
	runCtx.PathRun.MainInstanceRef = "instance-1"
	instanceCurrent := map[string]any{
		"contractSum":        json.Number("10000"),
		"accountantOpinion":  "上游还没填",
		"classificationId":   []any{"c-1"},
		"classificationName": "施工类",
		"legalOpinion":       "法务已经填过的内容",
	}
	plan, err := step.BuildNodeFormData(runCtx, approveStep(), instanceCurrent)
	if err != nil {
		t.Fatalf("构造审批表单数据失败：%v", err)
	}
	if !plan.BaseFromInstance {
		t.Fatal("审批基线必须来自实例当前数据")
	}
	values := decodeFormPayload(t, plan.Payload)
	if got := string(values["legalOpinion"]); got != `"法务已经填过的内容"` {
		t.Fatalf("本节点不可编辑的字段必须保持实例现状，实际 %s", got)
	}
	if got := string(values["accountantOpinion"]); got != `"配置意见"` {
		t.Fatalf("本节点可编辑字段应按配置值覆盖，实际 %s", got)
	}
	if got := string(values["contractSum"]); got != "500001" {
		t.Fatalf("本节点同样可编辑的字段应按配置值覆盖，实际 %s", got)
	}
	if !containsField(plan.Overlaid, "accountantOpinion") || !containsField(plan.Overlaid, "contractSum") {
		t.Fatalf("覆盖字段必须如实记录：%v", plan.Overlaid)
	}
	if !containsField(plan.Withheld, "classificationId") {
		t.Fatalf("本节点不可编辑、实例已有的字段应记为未携带：%v", plan.Withheld)
	}
}

// TestF024CompanionKeysFollowTheirOwnControl 锁定伴生键跟随本体：
// 目标只对控件本体声明权限，而条件求值读的是 classificationId__virtualName（语义清单第 15 条），
// 伴生键漏掉分支就算不出来，因此本体可编辑时伴生键必须一起提交。
func TestF024CompanionKeysFollowTheirOwnControl(t *testing.T) {
	runCtx := f024RunContext(`{"classificationId":["c-2"],"classificationId__virtualName":"施工类","classificationName":"施工类","accountantUserName":"张三","accountantUserName__formPersonId":"u-9"}`)
	runCtx.Nodes["node-start"] = step.NodeInfo{Name: "发起人", Type: "start", TargetNodeID: "real-start",
		EditableFields: []string{"classificationId"}}
	runCtx.NodeEditableFields["node-start"] = []string{"classificationId"}
	runCtx.NodeEditableFields["node-audit"] = []string{"accountantUserName"}
	plan, err := step.BuildNodeFormData(runCtx, submitStep(), nil)
	if err != nil {
		t.Fatalf("构造发起表单数据失败：%v", err)
	}
	values := decodeFormPayload(t, plan.Payload)
	for _, key := range []string{"classificationId", "classificationId__virtualName", "classificationName"} {
		if _, exists := values[key]; !exists {
			t.Fatalf("控件本体可编辑时伴生键必须一起提交，缺少 %s：%v", key, values)
		}
	}
	for _, key := range []string{"accountantUserName", "accountantUserName__formPersonId"} {
		if _, exists := values[key]; exists {
			t.Fatalf("只有审批节点能编辑的控件及其伴生键不得出现在发起请求里：%s", key)
		}
	}
}

// TestF024ApproveUsesRealTargetNodeID 锁定待办新鲜读取用目标真实节点标识：
// 编译场景的 nodeKey 是工具侧不透明派生键，发给目标永远匹配不上任何待办。
func TestF024ApproveUsesRealTargetNodeID(t *testing.T) {
	fake := &fakeTarget{
		instance:    fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"real-audit"}, DueNodes: []string{"real-audit"}},
		dueTaskID:   "task-1",
		auditResult: nil,
	}
	runCtx := f024RunContext(`{"accountantOpinion":"配置意见"}`)
	runCtx.PathRun.MainInstanceRef = "instance-1"
	runCtx.Steps = []model.CompiledActionStep{approveStep()}
	executor := step.NewExecutor(fake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), func() time.Time { return time.Unix(0, 0).UTC() })
	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil {
		t.Fatalf("构造预览失败：%v", err)
	}
	if preview.BlockReason != "" {
		t.Fatalf("门禁应通过，实际被阻塞：%s", preview.BlockReason)
	}
	if preview.TargetNodeID != "real-audit" {
		t.Fatalf("预览必须携带目标真实节点标识，实际 %q", preview.TargetNodeID)
	}
	if _, _, err := executor.RunApprovedStep(context.Background(), step.ApprovedStep{RunCtx: runCtx, Preview: preview, NextIndex: 0}); err != nil {
		t.Fatalf("放行失败：%v", err)
	}
	if fake.dueTaskNodeID != "real-audit" {
		t.Fatalf("待办读取应传目标真实节点标识，实际 %q", fake.dueTaskNodeID)
	}
	if fake.instanceDataReads == 0 {
		t.Fatal("携带表单数据的动作必须先读实例当前数据作为基线")
	}
}

// TestF024MissingTargetNodeIDBlocksStep 锁定安全兜底：解析不出目标真实节点标识时拒绝执行，
// 而不是拿空标识去比对待办——那会把没生效的写误判成已前进。
func TestF024MissingTargetNodeIDBlocksStep(t *testing.T) {
	fake := &fakeTarget{instance: fakeTargetView{Found: true, Status: "run", DueNodes: []string{"real-audit"}}, dueTaskID: "task-1"}
	runCtx := f024RunContext(`{"accountantOpinion":"配置意见"}`)
	runCtx.PathRun.MainInstanceRef = "instance-1"
	runCtx.Steps = []model.CompiledActionStep{approveStep()}
	runCtx.Nodes["node-audit"] = step.NodeInfo{Name: "部门审批", Type: "审批", EditableFields: []string{"accountantOpinion"}}
	executor := step.NewExecutor(fake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)
	preview, _, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil {
		t.Fatalf("构造预览失败：%v", err)
	}
	if preview.BlockReason == "" || !strings.Contains(preview.BlockReason, "真实标识") {
		t.Fatalf("缺少节点真实标识必须阻塞并说明原因，实际 %q", preview.BlockReason)
	}
	if fake.auditCalls != 0 {
		t.Fatalf("阻塞的步骤绝不允许发出写请求，实际 %d 次", fake.auditCalls)
	}
}

// containsField 判断字段清单是否包含目标字段。
func containsField(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
