package assignment_test

import (
	"strings"
	"testing"

	"test-auto-pro-v2/internal/engine/actioncatalog"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// F-035/T07 处理人事实：目标节点已到达但没有 currentAuditUserInfo/待办时，
// 目录必须显示“阻塞/未生成处理人”，绝不能显示“当前待办已经处理”；
// 也不允许用计划账号或候选人回退成可执行。

// TestHandlerMissingIsBlockedNotProcessed 锁定处理人缺失显示阻塞而不是已处理。
func TestHandlerMissingIsBlockedNotProcessed(t *testing.T) {
	items := actioncatalog.Build(model.ActionContext{
		FlowSource:      "existing",
		InstanceStatus:  "run",
		InstanceVisible: true,
		CurrentNodeKey:  "node-audit",
		CurrentNodeType: "审批",
		HasCurrentTask:  false,
		CurrentTaskDone: false,
		HandlerMissing:  true,
	})
	approve := catalogAction(t, items, model.ActionApprove)
	if approve.Enabled {
		t.Fatal("处理人缺失时同意不得启用")
	}
	if !strings.Contains(approve.DisabledReason, "assignment_missing") {
		t.Fatalf("禁用原因必须带 assignment_missing：%q", approve.DisabledReason)
	}
	if strings.Contains(approve.DisabledReason, "已经处理") {
		t.Fatalf("处理人缺失不得显示已处理：%q", approve.DisabledReason)
	}
}

// TestHandlerMissingDoesNotUsePlanAccountFallback 锁定发起人身份不能顶替缺失处理人。
func TestHandlerMissingDoesNotUsePlanAccountFallback(t *testing.T) {
	items := actioncatalog.Build(model.ActionContext{
		FlowSource:      "new",
		InstanceStatus:  "run",
		IsInitiator:     true,
		InstanceVisible: true,
		CurrentNodeKey:  "node-audit",
		CurrentNodeType: "审批",
		HasCurrentTask:  false,
		HandlerMissing:  true,
	})
	approve := catalogAction(t, items, model.ActionApprove)
	if approve.Enabled {
		t.Fatal("发起人身份不能顶替缺失的当前处理人")
	}
}

// TestPersistedFormVerifyIssueSurvivesDecode 锁定写后核对结论能从落库 JSON 还原。
func TestPersistedFormVerifyIssueSurvivesDecode(t *testing.T) {
	encoded := step.EncodeInstanceFacts(step.InstanceFacts{
		FormDataVerifyIssue:       "字段 reason 发送后未出现在目标实例数据中",
		FormDataVerifyAt:          "2026-09-14T10:00:00Z",
		FormDataVerifyFingerprint: "abc",
		FormDataVerifyResults:     []string{"字段 reason 发送后未出现在目标实例数据中"},
		BusinessSaveID:            "biz-1",
		BusinessSaveOtherBiz:      "cost_funds_transactions",
	})
	facts, ok := step.DecodeInstanceFacts(encoded)
	if !ok {
		t.Fatal("写后核对结论必须能从落库 JSON 还原")
	}
	if facts.FormDataVerifyIssue == "" || facts.BusinessSaveID != "biz-1" {
		t.Fatalf("持久化核对/业务 id 丢失：%+v", facts)
	}
}

// catalogAction 取出目录中指定动作项。
func catalogAction(t *testing.T, items []model.ActionCatalogItem, action model.ActionKey) model.ActionCatalogItem {
	t.Helper()
	for _, item := range items {
		if item.Action == action {
			return item
		}
	}
	t.Fatalf("目录缺少动作 %s", action)
	return model.ActionCatalogItem{}
}
