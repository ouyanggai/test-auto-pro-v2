package f012_test

import (
	"encoding/json"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/engine/actioncatalog"
	"test-auto-pro-v2/internal/model"
)

// TestActionCatalogContractUsesRealTargetFields 验证动作目录公开的是目标协议字段，而不是工具侧数据包装。
func TestActionCatalogContractUsesRealTargetFields(t *testing.T) {
	items := actioncatalog.Build(model.ActionContext{FlowSource: "pending", InstanceStatus: "run", CurrentNodeType: "common", HasCurrentTask: true, HasEditableProxy: true, CanSwitchActor: true, PreviousTaskExists: true, PreviousNodeType: "common"})
	for _, item := range items {
		if item.Action == model.ActionSystemAutomatic {
			continue
		}
		if item.TargetOperation == "" || len(item.Parameters) == 0 || len(item.Preconditions) == 0 || item.ExpectedEffect == "" || !item.RequiresReload || len(item.ReloadRequirements) == 0 {
			t.Fatalf("动作目录项缺少完整契约字段：%+v", item)
		}
		if strings.Contains(strings.ToLower(item.TargetOperation), "write") {
			t.Fatalf("动作目录使用了非目标协议操作：%s", item.TargetOperation)
		}
		for _, parameter := range item.Parameters {
			if strings.Contains(parameter, "=") || strings.Contains(parameter, "HistoricalDataPayload") || strings.Contains(parameter, "generated") || strings.Contains(parameter, "manualOverride") {
				t.Fatalf("动作参数不是目标原字段或混入工具侧旧语义：%q", parameter)
			}
		}
	}
	approve := findAction(items, model.ActionApprove)
	reject := findAction(items, model.ActionReject)
	if !hasParameterValue(approve, "auditRecord.auditStatus", "pass") || !hasParameterValue(reject, "auditRecord.auditStatus", "no_pass") {
		t.Fatalf("同意/不同意未投影目标审计结果参数：approve=%+v reject=%+v", approve, reject)
	}
	if hasParameterValue(approve, "status", "draft") || hasParameterValue(reject, "status", "draft") {
		t.Fatal("审批动作错误复用了保存草稿参数")
	}
}

// TestActionCatalogContractJSONIsSafeAndDeterministic 验证 JSON 字段稳定、没有目标内部标识或不可恢复动作。
func TestActionCatalogContractJSONIsSafeAndDeterministic(t *testing.T) {
	context := model.ActionContext{FlowSource: "done", InstanceStatus: "run", CurrentNodeType: "common", HasCompletedTask: true, InstanceVisible: true}
	first, err := json.Marshal(actioncatalog.Build(context))
	if err != nil {
		t.Fatalf("动作目录 JSON 编码失败：%v", err)
	}
	second, err := json.Marshal(actioncatalog.Build(context))
	if err != nil || string(first) != string(second) {
		t.Fatalf("动作目录不是确定性输出：first=%s second=%s err=%v", first, second, err)
	}
	body := string(first)
	for _, forbidden := range []string{"sid", "HistoricalDataPayload", "ConstraintIR", "delete", "terminate", "abandon"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("动作目录 JSON 暴露内部标识或不可恢复能力 %q：%s", forbidden, body)
		}
	}
	if !strings.Contains(body, "disabledReason") || !strings.Contains(body, "preconditions") || !strings.Contains(body, "reloadRequirements") {
		t.Fatalf("动作目录 JSON 缺少解释性字段：%s", body)
	}
}

// TestActionCatalogContractRejectsUnknownAndForwardedContexts 验证未知节点、转发辅助流程不会被误判为可操作待办。
func TestActionCatalogContractRejectsUnknownAndForwardedContexts(t *testing.T) {
	unknown := actioncatalog.Build(model.ActionContext{FlowSource: "pending", InstanceStatus: "run", CurrentNodeType: "future_node", HasCurrentTask: true})
	for _, key := range []model.ActionKey{model.ActionStorageFormData, model.ActionApprove, model.ActionReject, model.ActionAddSign, model.ActionTransfer} {
		item := findAction(unknown, key)
		if item.Enabled || item.DisabledReason == "" {
			t.Fatalf("未知节点仍允许动作 %q：%+v", key, item)
		}
	}
	forwarded := actioncatalog.Build(model.ActionContext{FlowSource: "pending", InstanceStatus: "run", CurrentNodeType: "common", HasCurrentTask: true, ForwardedContext: true})
	for _, key := range []model.ActionKey{model.ActionStorageFormData, model.ActionApprove, model.ActionReject} {
		item := findAction(forwarded, key)
		if item.Enabled || item.DisabledReason != "转发辅助流程不提供主实例审批动作" {
			t.Fatalf("转发辅助流程动作门禁错误 %q：%+v", key, item)
		}
	}
	unknownSource := actioncatalog.Build(model.ActionContext{FlowSource: "unknown", InstanceStatus: "run", InstanceVisible: true})
	forward := findAction(unknownSource, model.ActionForward)
	if forward.Enabled || forward.DisabledReason != "当前上下文不是目标页面开放的待办、已发或已办实例" {
		t.Fatalf("未知来源仍允许转发：%+v", forward)
	}
}

// findAction 在契约测试中定位单个动作，不依赖目录内部实现。
func findAction(items []model.ActionCatalogItem, action model.ActionKey) model.ActionCatalogItem {
	for _, item := range items {
		if item.Action == action {
			return item
		}
	}
	return model.ActionCatalogItem{}
}

// hasParameterValue 判断目录参数是否包含目标协议的固定字段值。
func hasParameterValue(item model.ActionCatalogItem, name, value string) bool {
	for _, parameter := range item.ParameterDetails {
		if parameter.Name == name && parameter.Value == value {
			return true
		}
	}
	return false
}
