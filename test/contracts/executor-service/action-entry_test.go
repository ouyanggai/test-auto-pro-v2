package executorservice

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeActionsDoNotClick(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "executor-service", "src", "native-actions.ts"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, forbidden := range []string{"locator(", "page.click", "page.fill"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("动作桥接不得出现 %s", forbidden)
		}
	}
	for _, method := range []string{"sumbitFlow", "storageFormData", "formMakingFormBusiness", "reSubmit", "confirmTranspond"} {
		if !strings.Contains(text, method) {
			t.Fatalf("缺少原生方法入口 %s", method)
		}
	}
}
