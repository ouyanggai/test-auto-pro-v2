package executorservice

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletionLocksWriteOnUncertainAndTimeout(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "executor-service", "src", "completion.ts"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, needle := range []string{
		"timeout",
		"uncertain",
		"writeGate?.lock",
		"后续写请求已被拒绝",
		"canSendForwardStep",
		"SID已失效",
		"business_failed",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("完成检测缺少 %s", needle)
		}
	}
}
