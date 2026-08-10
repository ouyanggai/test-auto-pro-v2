package integration_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"test-auto-pro-v2/internal/formruntimemaintenance"
)

// TestFormRuntimeFixedSourceSnapshot 核对真实参考仓库的远端、分支、动态 HEAD、干净状态及实际运行源码内容。
func TestFormRuntimeFixedSourceSnapshot(t *testing.T) {
	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := formruntimemaintenance.LoadManifest(projectRoot, filepath.Join(projectRoot, "form-runtime", "sync-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	inspector, err := formruntimemaintenance.NewGitSourceInspector(projectRoot, manifest, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	source, err := inspector.Inspect(context.Background())
	if err != nil {
		t.Fatalf("固定参考仓库检查失败：%v", err)
	}
	if source.Dirty {
		t.Fatalf("固定参考仓库存在未提交修改：%+v", source.ChangedFiles)
	}
	// 直接执行项目保留的原生 sync-check，证明真正 dev/build 消费的 runtime-source 与当前来源一致。
	command := exec.CommandContext(context.Background(), "pnpm", "--dir", filepath.Join(projectRoot, "form-runtime"), "sync:check")
	command.Dir = projectRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("当前实际运行源码与固定来源不一致：%v\n%s", err, output)
	}
}
