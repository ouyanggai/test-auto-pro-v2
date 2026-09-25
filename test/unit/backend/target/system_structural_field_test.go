package target_test

import (
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
)

// TestIsSystemStructuralFieldExcludesInstanceID 锁定关键回归：flowInstanceId 是目标引擎管理的实例自身 ID，
// 属系统结构性字段，绝不能纳入表单写后核对。曾经把它当业务字段核对，读回缺失即报
// 「字段 flowInstanceId 发送后未出现在目标实例数据中」，把整条路径误判阻塞停止。
func TestIsSystemStructuralFieldExcludesInstanceID(t *testing.T) {
	if !target.IsSystemStructuralField("flowInstanceId") {
		t.Fatalf("flowInstanceId 必须识别为系统结构性字段，不参与业务写后核对")
	}
	// 前后空白也应识别，避免因空白绕过排除。
	if !target.IsSystemStructuralField("  flowInstanceId  ") {
		t.Fatalf("带空白的 flowInstanceId 也应识别为系统结构性字段")
	}
}

// TestIsSystemStructuralFieldKeepsBusinessFields 锁定边界：真实业务字段不得被误判为系统字段而跳过核对，
// 否则会漏掉真正的写入丢失。
func TestIsSystemStructuralFieldKeepsBusinessFields(t *testing.T) {
	for _, field := range []string{"amount", "reason", "applyUserName", "flowInstance", "instanceId", ""} {
		if target.IsSystemStructuralField(field) {
			t.Fatalf("业务字段 %q 不应被判为系统结构性字段", field)
		}
	}
}
