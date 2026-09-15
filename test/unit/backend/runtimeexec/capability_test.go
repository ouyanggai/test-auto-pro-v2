// T014 能力登记语义：只认 confirmed_success；同流程/同端点动作互不自动解锁。
package runtimeexec_test

import (
	"testing"

	"test-auto-pro-v2/internal/runtimeexec"
)

func TestCapabilityUnlockOnlyFromConfirmedSuccess(t *testing.T) {
	registry := runtimeexec.NewCapabilityRegistry()
	for _, verdict := range []string{"target_rejected", "uncertain", "business_failed", "timeout", "failed", ""} {
		if err := registry.UnlockFromVerifiedRun(runtimeexec.CapabilityEntry{
			FlowType: "employee_leave", Action: "submit", Evidence: "e", LogRef: "l",
		}, verdict); err == nil {
			t.Fatalf("判定 %s 不允许登记能力", verdict)
		}
	}
	if err := registry.UnlockFromVerifiedRun(runtimeexec.CapabilityEntry{
		FlowType: "employee_leave", Action: "submit", FormDigest: "digest-1",
		RuntimeVersion: "exec-engine/v1", TargetVersion: "target-1",
		Evidence: "实例 7bcf0c29 状态 run", LogRef: "logs/runs/运行_13",
	}, "confirmed_success"); err != nil {
		t.Fatalf("confirmed_success 应允许登记：%v", err)
	}
	if !registry.Registered("employee_leave", "submit", "digest-1") {
		t.Fatalf("精确键应命中")
	}
}

func TestCapabilityNoCrossUnlock(t *testing.T) {
	registry := runtimeexec.NewCapabilityRegistry()
	_ = registry.UnlockFromVerifiedRun(runtimeexec.CapabilityEntry{
		FlowType: "employee_leave", Action: "submit", RuntimeVersion: "v1", Evidence: "e", LogRef: "l",
	}, "confirmed_success")
	// 同流程其他动作不自动解锁。
	for _, action := range []string{"approve", "reject", "resubmit", "forward", "storage_form_data", "save_draft"} {
		if registry.Registered("employee_leave", action, "") {
			t.Fatalf("同流程动作 %s 不应被提交成功自动解锁", action)
		}
	}
	// 同端点不同动作不自动解锁：审批端点上同意成功不解锁不同意（不同动作键）。
	_ = registry.UnlockFromVerifiedRun(runtimeexec.CapabilityEntry{
		FlowType: "contract_review", Action: "approve", RuntimeVersion: "v1", Evidence: "e2", LogRef: "l2",
	}, "confirmed_success")
	if registry.Registered("contract_review", "reject", "") {
		t.Fatalf("同意成功不应自动解锁不同意")
	}
	// 其他流程不自动解锁。
	if registry.Registered("expense_budget", "submit", "") {
		t.Fatalf("其他流程不应被自动解锁")
	}
}
