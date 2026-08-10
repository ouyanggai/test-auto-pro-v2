package integration_test

import (
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"

	"test-auto-pro-v2/internal/formruntimemaintenance"
)

// TestFormRuntimeFixedSourceSnapshot 核对真实参考仓库的远端、分支、HEAD、干净状态及 upstream 原样区内容。
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
	syncer, err := formruntimemaintenance.NewSyncer(projectRoot, inspector.SourceRoot(), manifest)
	if err != nil {
		t.Fatal(err)
	}
	// 这里仅做只读 SYNC_CHECK；真正同步必须由持久化维护任务先完成来源快照复核和租约领取。
	if err := syncer.Check(context.Background(), io.Discard); err != nil {
		t.Fatalf("当前 upstream 原样区与固定来源不一致：%v", err)
	}
}
