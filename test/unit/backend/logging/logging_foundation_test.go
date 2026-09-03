package logging_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"test-auto-pro-v2/internal/logging"
)

// fixedTime 固定日志时间，避免用例依赖真实时钟。
func fixedTime() time.Time { return time.Date(2026, 9, 3, 18, 56, 31, 0, time.UTC) }

// TestWriterReturnsLineNumbersAndRotates 验证写入器返回行号、超限轮转并保留有界历史文件。
// 行号会被后续切片存进运行记录做日志深链，所以这里必须锁定。
func TestWriterReturnsLineNumbersAndRotates(t *testing.T) {
	root := t.TempDir()
	writer := logging.NewWriter(filepath.Join(root, "network.log"))
	writer.SetLimits(200, 2)
	if first := writer.WriteLine("time=1 level=info message=第一行"); first != 1 {
		t.Fatalf("首行行号应为 1，实际 %d", first)
	}
	if second := writer.WriteLine("time=2 level=info message=第二行"); second != 2 {
		t.Fatalf("第二行行号应为 2，实际 %d", second)
	}
	for index := 0; index < 6; index++ {
		writer.WriteLine("time=3 level=info message=填充内容需要足够长以触发轮转")
	}
	if _, err := os.Stat(filepath.Join(root, "network.log.1")); err != nil {
		t.Fatalf("超过容量上限没有轮转出 .1 文件：%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "network.log.3")); err == nil {
		t.Fatal("轮转保留份数超过上限")
	}
}

// TestWriterKeepsConcurrentLinesIntact 验证同一文件并发写入串行，不出现半行交错。
func TestWriterKeepsConcurrentLinesIntact(t *testing.T) {
	writer := logging.NewWriter(filepath.Join(t.TempDir(), "program.log"))
	group := sync.WaitGroup{}
	for index := 0; index < 40; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			writer.WriteLine("time=1 level=info message=并发写入的一整行内容")
		}()
	}
	group.Wait()
	content, err := os.ReadFile(writer.Path())
	if err != nil {
		t.Fatalf("读取日志失败：%v", err)
	}
	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	if len(lines) != 40 {
		t.Fatalf("并发写入行数不正确：%d", len(lines))
	}
	for _, line := range lines {
		if line != "time=1 level=info message=并发写入的一整行内容" {
			t.Fatalf("出现半行交错：%q", line)
		}
	}
}

// TestFormatLineUsesPlaceholderAndSingleLine 验证统一行格式：空值写占位符、值内空格换下划线、不出现换行。
func TestFormatLineUsesPlaceholderAndSingleLine(t *testing.T) {
	line := logging.FormatLine(fixedTime(), "error", []logging.Field{
		{Key: "request_id", Value: "abc"},
		{Key: "run_id", Value: ""},
		{Key: "user_message", Value: "读取流程超时，请重试\n继续"},
	})
	if !strings.HasPrefix(line, "time=2026-09-03_18:56:31 level=error ") {
		t.Fatalf("前两列不是固定的 time 与 level：%s", line)
	}
	if !strings.Contains(line, "run_id=-") {
		t.Fatalf("空值没有写占位符：%s", line)
	}
	if strings.Contains(line, "\n") || strings.Contains(line, "请重试 继续") {
		t.Fatalf("值没有收敛为单行 token：%s", line)
	}
}

// TestScopeFieldsInjectFromContext 验证作用域由 context 携带并按固定顺序展开，调用方不逐处传参。
func TestScopeFieldsInjectFromContext(t *testing.T) {
	ctx := logging.WithScope(context.Background(), logging.Scope{RequestID: "req-1", RunID: "run-1"})
	scope := logging.ScopeFrom(ctx)
	if scope.RequestID != "req-1" || scope.RunID != "run-1" {
		t.Fatalf("作用域没有从 context 读回：%+v", scope)
	}
	keys := make([]string, 0, 6)
	for _, field := range scope.Fields() {
		keys = append(keys, field.Key)
	}
	expected := []string{"request_id", "run_id", "path_run_id", "step_id", "attempt", "phase"}
	if strings.Join(keys, ",") != strings.Join(expected, ",") {
		t.Fatalf("关联键顺序不固定：%v", keys)
	}
	if empty := logging.ScopeFrom(context.Background()); empty.RequestID != "" {
		t.Fatalf("没有作用域时应返回零值：%+v", empty)
	}
}

// TestBucketRoutingSeparatesConfigAndRun 验证目录路由：运行作用域进运行目录，其余进配置桶，
// 目录段全部经过清洗，不接受目标返回的原始名称直接落盘。
func TestBucketRoutingSeparatesConfigAndRun(t *testing.T) {
	root := t.TempDir()
	router := logging.NewRouter(root, fixedTime)
	configDir := router.BucketDir(logging.Scope{RequestID: "req-1"})
	if configDir != filepath.Join(root, "config", "2026-09-03") {
		t.Fatalf("配置桶目录不正确：%s", configDir)
	}
	runDir := router.BucketDir(logging.Scope{
		RequestID: "req-1", PlanName: "oyg测试/001", PathName: "../路径 1", RunSeq: "run 7",
	})
	if strings.Contains(runDir, "..") || strings.Contains(runDir, " ") {
		t.Fatalf("运行目录没有清洗路径段：%s", runDir)
	}
	if !strings.HasPrefix(runDir, filepath.Join(root, "runs")) {
		t.Fatalf("运行作用域没有路由到运行目录：%s", runDir)
	}
}

// TestCleanupExpiredRemovesOnlyExpiredDirs 验证保留期清理只删过期目录，当天目录一律保留。
func TestCleanupExpiredRemovesOnlyExpiredDirs(t *testing.T) {
	root := t.TempDir()
	now := fixedTime()
	today := filepath.Join(root, "config", now.Format("2006-01-02"))
	expired := filepath.Join(root, "config", now.AddDate(0, 0, -20).Format("2006-01-02"))
	kept := filepath.Join(root, "config", now.AddDate(0, 0, -3).Format("2006-01-02"))
	unparsable := filepath.Join(root, "config", "not-a-date")
	for _, dir := range []string{today, expired, kept, unparsable} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("准备目录失败：%v", err)
		}
	}
	removed := logging.CleanupExpired(root, 14, now)
	if len(removed) != 1 || removed[0] != expired {
		t.Fatalf("只应删除过期目录：%v", removed)
	}
	for _, dir := range []string{today, kept, unparsable} {
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("目录被误删：%s", dir)
		}
	}
}
