// T014 能力登记语义：只认 confirmed_success；同流程/同端点动作互不自动解锁；
// 登记字段缺一不得登记（评审 P2-1）；登记落盘可重启加载（评审 P1-2）。
package runtimeexec_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/runtimeexec"
)

// fullCapabilityEntry 构造一条字段齐全的登记：全部强制字段都有值。
func fullCapabilityEntry() runtimeexec.CapabilityEntry {
	return runtimeexec.CapabilityEntry{
		FlowType: "employee_leave", Action: "submit", FormDigest: "digest-1",
		RuntimeVersion: "exec-engine/v1", TargetVersion: "target-1",
		Evidence: "instance=7bcf0c29;trace=0715f0cd", LogRef: "logs/runs/运行_13",
	}
}

func TestCapabilityUnlockOnlyFromConfirmedSuccess(t *testing.T) {
	registry := runtimeexec.NewCapabilityRegistry()
	for _, verdict := range []string{"target_rejected", "uncertain", "business_failed", "timeout", "failed", ""} {
		if err := registry.UnlockFromVerifiedRun(fullCapabilityEntry(), verdict); err == nil {
			t.Fatalf("判定 %s 不允许登记能力", verdict)
		}
	}
	if err := registry.UnlockFromVerifiedRun(fullCapabilityEntry(), "confirmed_success"); err != nil {
		t.Fatalf("confirmed_success 应允许登记：%v", err)
	}
	if !registry.Registered("employee_leave", "submit", "digest-1") {
		t.Fatalf("精确键应命中")
	}
}

// TestCapabilityRegisterRequiresEveryField 评审 P2-1 回归：缺任一强制字段都不得登记。
func TestCapabilityRegisterRequiresEveryField(t *testing.T) {
	cases := map[string]func(*runtimeexec.CapabilityEntry){
		"流程类型":    func(e *runtimeexec.CapabilityEntry) { e.FlowType = "" },
		"动作":      func(e *runtimeexec.CapabilityEntry) { e.Action = "" },
		"表单摘要":    func(e *runtimeexec.CapabilityEntry) { e.FormDigest = "" },
		"runtime": func(e *runtimeexec.CapabilityEntry) { e.RuntimeVersion = "" },
		"目标版本":    func(e *runtimeexec.CapabilityEntry) { e.TargetVersion = "" },
		"证据":      func(e *runtimeexec.CapabilityEntry) { e.Evidence = "" },
		"日志引用":    func(e *runtimeexec.CapabilityEntry) { e.LogRef = "" },
	}
	for name, clear := range cases {
		registry := runtimeexec.NewCapabilityRegistry()
		entry := fullCapabilityEntry()
		clear(&entry)
		if err := registry.Register(entry); err == nil {
			t.Fatalf("缺少 %s 的登记必须被拒绝", name)
		}
	}
}

func TestCapabilityNoCrossUnlock(t *testing.T) {
	registry := runtimeexec.NewCapabilityRegistry()
	entry := fullCapabilityEntry()
	if err := registry.UnlockFromVerifiedRun(entry, "confirmed_success"); err != nil {
		t.Fatalf("登记失败：%v", err)
	}
	// 同流程其他动作不自动解锁。
	for _, action := range []string{"approve", "reject", "resubmit", "forward", "storage_form_data", "save_draft"} {
		if registry.Registered("employee_leave", action, "digest-1") {
			t.Fatalf("同流程动作 %s 不应被提交成功自动解锁", action)
		}
	}
	// 同端点不同动作不自动解锁：审批端点上同意成功不解锁不同意（不同动作键）。
	approve := fullCapabilityEntry()
	approve.FlowType = "contract_review"
	approve.Action = "approve"
	approve.FormDigest = "digest-2"
	approve.Evidence = "e2"
	approve.LogRef = "l2"
	if err := registry.UnlockFromVerifiedRun(approve, "confirmed_success"); err != nil {
		t.Fatalf("登记失败：%v", err)
	}
	if registry.Registered("contract_review", "reject", "digest-2") {
		t.Fatalf("同意成功不应自动解锁不同意")
	}
	// 其他流程不自动解锁。
	if registry.Registered("expense_budget", "submit", "digest-1") {
		t.Fatalf("其他流程不应被自动解锁")
	}
	// 摘要未知时不得放行：无摘要粒度的回落查询已被废除（评审 P2-1 同源修复）。
	if registry.Registered("employee_leave", "submit", "") {
		t.Fatalf("空表单摘要不得命中登记")
	}
}

// TestCapabilityRegistryPersistsAcrossReload 评审 P1-2 回归：登记落盘后重新打开
// 必须能读到同一批登记，Go 服务重启不再清空已验证能力。
func TestCapabilityRegistryPersistsAcrossReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capabilities.json")
	registry, err := runtimeexec.OpenCapabilityRegistry(path)
	if err != nil {
		t.Fatalf("空目录应得到空登记表：%v", err)
	}
	if len(registry.Snapshot()) != 0 {
		t.Fatalf("新登记表应为空")
	}
	if err := registry.Register(fullCapabilityEntry()); err != nil {
		t.Fatalf("登记失败：%v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("登记成功后必须落盘：%v", err)
	}
	reloaded, err := runtimeexec.OpenCapabilityRegistry(path)
	if err != nil {
		t.Fatalf("重启加载失败：%v", err)
	}
	if !reloaded.Registered("employee_leave", "submit", "digest-1") {
		t.Fatalf("重启后同组合必须仍然已登记")
	}
	snapshot := reloaded.Snapshot()
	if len(snapshot) != 1 || snapshot[0].Evidence != fullCapabilityEntry().Evidence {
		t.Fatalf("落盘内容必须与登记一致：%+v", snapshot)
	}
}

// TestCapabilityRegistryRejectsCorruptFile 损坏的登记文件必须显式报错，
// 不能带着残缺证据静默起跑。
func TestCapabilityRegistryRejectsCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capabilities.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("准备损坏文件失败：%v", err)
	}
	if _, err := runtimeexec.OpenCapabilityRegistry(path); err == nil {
		t.Fatalf("损坏的登记文件必须报错")
	} else if !strings.Contains(err.Error(), "JSON") {
		t.Fatalf("错误必须说明 JSON 问题：%v", err)
	}
}
