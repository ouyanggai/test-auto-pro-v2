package executor_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/runtimeexec"
)

// TestRuntimeProtectReportsBusinessFieldNames 锁定写后正文保护的诊断粒度：
// 目标登录上下文字段不参与缺失计数，真正丢失的业务字段名必须出现在阻塞消息中。
func TestRuntimeProtectReportsBusinessFieldNames(t *testing.T) {
	after := map[string]any{"contractId": "123"}
	executor := runtimeProtectExecutor(after)
	runCtx := newRunContext([]model.CompiledActionStep{approveStep()})
	runCtx.PathRun.MainInstanceRef = "instance-1"
	before := map[string]any{
		"contractId":     "123",
		"contractSum":    100,
		"currentDepName": "综合管理部",
	}
	summary, err := json.Marshal(map[string]any{"formBefore": before})
	if err != nil {
		t.Fatal(err)
	}
	result := executor.ProtectRuntimeFormBodyForTest(
		context.Background(), runCtx, approveStep(), target.Session{},
		runtimeexec.ActionRunResult{Status: "success", BusinessSummary: summary}, 1)
	if result.Status != "business_failed" {
		t.Fatalf("真实业务字段丢失必须判 business_failed，实际 %q", result.Status)
	}
	if !strings.Contains(result.Message, "contractSum") {
		t.Fatalf("阻塞消息必须列出缺失业务字段名：%s", result.Message)
	}
	if strings.Contains(result.Message, "currentDepName") {
		t.Fatalf("目标上下文字段不应进入缺失诊断：%s", result.Message)
	}
}

// TestRuntimeProtectAllowsPersonPickerCompanionCleanup 锁定运行 8 路径 3：
// 主人员字段仍存在时，目标清洗 __formPersonId 伴生字段不代表业务正文丢失。
func TestRuntimeProtectAllowsPersonPickerCompanionCleanup(t *testing.T) {
	after := map[string]any{
		"legalUserName":            `{"id":"user-1","name":"孙乐"}`,
		"legalUserName__condition": "孙乐",
	}
	executor := runtimeProtectExecutor(after)
	runCtx := newRunContext([]model.CompiledActionStep{approveStep()})
	runCtx.PathRun.MainInstanceRef = "instance-1"
	before := map[string]any{
		"legalUserName":               `{"id":"user-1","name":"孙乐"}`,
		"legalUserName__condition":    "孙乐",
		"legalUserName__formPersonId": "user-1",
	}
	result := executor.ProtectRuntimeFormBodyForTest(
		context.Background(), runCtx, approveStep(), target.Session{}, runtimeProtectResult(t, before), 1)
	if result.Status != "success" {
		t.Fatalf("主人员字段仍在时伴生字段清洗不得改判失败，实际 %q：%s", result.Status, result.Message)
	}
}

// TestRuntimeProtectBlocksPersonPickerPrimaryFieldLoss 锁定正文保护下界：
// 主人员字段和伴生字段一起丢失时仍必须阻塞，不能借伴生字段容差吞掉真实业务值。
func TestRuntimeProtectBlocksPersonPickerPrimaryFieldLoss(t *testing.T) {
	after := map[string]any{}
	executor := runtimeProtectExecutor(after)
	runCtx := newRunContext([]model.CompiledActionStep{approveStep()})
	runCtx.PathRun.MainInstanceRef = "instance-1"
	before := map[string]any{
		"legalUserName":               `{"id":"user-1","name":"孙乐"}`,
		"legalUserName__formPersonId": "user-1",
	}
	result := executor.ProtectRuntimeFormBodyForTest(
		context.Background(), runCtx, approveStep(), target.Session{}, runtimeProtectResult(t, before), 1)
	if result.Status != "business_failed" || !strings.Contains(result.Message, "legalUserName") {
		t.Fatalf("主人员字段丢失必须继续阻塞，实际 %q：%s", result.Status, result.Message)
	}
}
