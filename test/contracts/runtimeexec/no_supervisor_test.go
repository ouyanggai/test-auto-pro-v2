package runtimeexec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGoDoesNotSuperviseExecutorProcess 静态锁定：Go 启动路径不再托管执行服务，也不再启用能力门禁。
func TestGoDoesNotSuperviseExecutorProcess(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	files := []string{
		filepath.Join(root, "cmd", "server", "main.go"),
		filepath.Join(root, "internal", "config", "config.go"),
	}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		for _, needle := range []string{"StartExecutorSupervisor", "RUNTIME_EXECUTOR_MANAGED", "RUNTIME_EXEC_VERIFICATION", "CapabilityRegistry"} {
			if strings.Contains(text, needle) {
				t.Fatalf("%s 不得再包含 %s", filepath.Base(file), needle)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "runtimeexec", "supervisor.go")); !os.IsNotExist(err) {
		t.Fatal("必须删除 StartExecutorSupervisor")
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "runtimeexec", "capability.go")); !os.IsNotExist(err) {
		t.Fatal("必须删除能力登记执行门禁")
	}
}
