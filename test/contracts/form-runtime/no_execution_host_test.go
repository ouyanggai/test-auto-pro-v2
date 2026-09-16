package formruntime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFormRuntimeHasNoExecutionHost 配置侧不得保留执行宿主、执行桥接或 exec-engine 协议。
func TestFormRuntimeHasNoExecutionHost(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	runtimeDir := filepath.Join(root, "form-runtime")
	forbiddenFiles := []string{
		filepath.Join(runtimeDir, "src", "ExecutionHostInitiate.vue"),
		filepath.Join(runtimeDir, "src", "ExecutionHostApprove.vue"),
		filepath.Join(runtimeDir, "src", "runtime", "executionBridge.js"),
		filepath.Join(runtimeDir, "executor", "src", "server.ts"),
	}
	for _, file := range forbiddenFiles {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Fatalf("配置侧不得保留执行宿主：%s", file)
		}
	}
	needles := []string{"__runtimeBridge__", "ExecutionHost", "exec-engine", "/test-auto-form/exec"}
	err = filepath.Walk(runtimeDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			base := info.Name()
			if base == "node_modules" || base == "runtime-source" || base == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".js" && ext != ".vue" && ext != ".ts" && ext != ".json" {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		text := string(raw)
		for _, needle := range needles {
			if strings.Contains(text, needle) {
				t.Errorf("%s 仍包含执行宿主痕迹 %s", path, needle)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
