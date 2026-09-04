package logging_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
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

// TestWriterKeepsConcurrentBlocksIntact 验证多行块并发写入不会互相穿插。
// curl.log 与 panic 栈都是多行块，逐行加锁会让两个请求的 begin、正文、end 交错，把日志块写坏。
func TestWriterKeepsConcurrentBlocksIntact(t *testing.T) {
	writer := logging.NewWriter(filepath.Join(t.TempDir(), "curl.log"))
	const blocks, blockLines = 40, 5
	group := sync.WaitGroup{}
	for index := 0; index < blocks; index++ {
		group.Add(1)
		go func(id int) {
			defer group.Done()
			trace := "trace-" + strconv.Itoa(id)
			writer.WriteBlock(
				"--- begin curl trace_id="+trace+" ---",
				"curl -sS -X POST '"+trace+"'\n--- response ---\n{\"id\":\""+trace+"\"}",
				"--- end curl trace_id="+trace+" ---",
			)
		}(index)
	}
	group.Wait()
	content, err := os.ReadFile(writer.Path())
	if err != nil {
		t.Fatalf("读取日志失败：%v", err)
	}
	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	if len(lines) != blocks*blockLines {
		t.Fatalf("块总行数不正确：%d", len(lines))
	}
	seen := map[string]bool{}
	for index := 0; index < len(lines); index += blockLines {
		header := lines[index]
		if !strings.HasPrefix(header, "--- begin curl trace_id=") {
			t.Fatalf("第 %d 行不是块首，说明块被切开了：%q", index+1, header)
		}
		trace := strings.TrimSuffix(strings.TrimPrefix(header, "--- begin curl trace_id="), " ---")
		if seen[trace] {
			t.Fatalf("同一个块出现了两次：%s", trace)
		}
		seen[trace] = true
		for offset, expected := range []string{
			"curl -sS -X POST '" + trace + "'",
			"--- response ---",
			"{\"id\":\"" + trace + "\"}",
			"--- end curl trace_id=" + trace + " ---",
		} {
			if actual := lines[index+1+offset]; actual != expected {
				t.Fatalf("块 %s 被其他块穿插：第 %d 行是 %q，应为 %q", trace, index+offset+2, actual, expected)
			}
		}
	}
	if len(seen) != blocks {
		t.Fatalf("块数量不正确：%d", len(seen))
	}
	// 块首行号必须接着已写入的行继续，后续切片要用它做日志深链。
	if first := writer.WriteBlock("--- begin curl trace_id=trace-last ---", "curl", "--- end curl trace_id=trace-last ---"); first != blocks*blockLines+1 {
		t.Fatalf("块首行号不正确：%d", first)
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
// 业务归属键必须排在最前，方便按计划或执行路径直接 grep。
func TestScopeFieldsInjectFromContext(t *testing.T) {
	ctx := logging.WithScope(context.Background(), logging.Scope{RequestID: "req-1", RunID: "run-1"})
	scope := logging.ScopeFrom(ctx)
	if scope.RequestID != "req-1" || scope.RunID != "run-1" {
		t.Fatalf("作用域没有从 context 读回：%+v", scope)
	}
	keys := make([]string, 0, 10)
	for _, field := range scope.Fields() {
		keys = append(keys, field.Key)
	}
	expected := []string{
		"plan_id", "plan_name", "execution_path_id", "execution_path_name",
		"request_id", "run_id", "path_run_id", "step_id", "attempt", "phase",
	}
	if strings.Join(keys, ",") != strings.Join(expected, ",") {
		t.Fatalf("关联键顺序不固定：%v", keys)
	}
	if empty := logging.ScopeFrom(context.Background()); empty.RequestID != "" {
		t.Fatalf("没有作用域时应返回零值：%+v", empty)
	}
}

// TestWithScopeMergesInsteadOfOverwriting 验证补充计划与执行路径后 RequestID 等既有字段不丢失。
// 中间件先注入请求标识，随后才能从数据库拿到显示名，覆盖式写入会把请求标识丢掉。
func TestWithScopeMergesInsteadOfOverwriting(t *testing.T) {
	ctx := logging.WithScope(context.Background(), logging.Scope{RequestID: "req-1", RunID: "run-1"})
	ctx = logging.WithScope(ctx, logging.Scope{PlanID: "7", PlanName: "员工请假单（集团）", ExecutionPathID: "13"})
	merged := logging.ScopeFrom(ctx)
	if merged.RequestID != "req-1" || merged.RunID != "run-1" {
		t.Fatalf("合并作用域丢失了既有字段：%+v", merged)
	}
	if merged.PlanID != "7" || merged.PlanName != "员工请假单（集团）" || merged.ExecutionPathID != "13" {
		t.Fatalf("合并作用域没有补上业务归属：%+v", merged)
	}
	// 空值不得覆盖已有值，否则一次空补充就会把归属清掉。
	cleared := logging.ScopeFrom(logging.WithScope(ctx, logging.Scope{}))
	if cleared.PlanID != "7" || cleared.RequestID != "req-1" {
		t.Fatalf("空作用域覆盖了已有字段：%+v", cleared)
	}
}

// TestBucketRoutingSeparatesApplicationConfigurationAndRun 验证顶层只有 application 与 plans 两棵树，
// 业务日志先按计划与执行路径归档再按日期或运行号分层，只有确实无法归属业务对象的日志才进 application。
func TestBucketRoutingSeparatesApplicationConfigurationAndRun(t *testing.T) {
	root := t.TempDir()
	router := logging.NewRouter(root, fixedTime)
	applicationDir := router.BucketDir(logging.Scope{RequestID: "req-1"})
	if applicationDir != filepath.Join(root, "application", "2026-09-03") {
		t.Fatalf("无业务归属的日志目录不正确：%s", applicationDir)
	}
	configurationDir := router.BucketDir(logging.Scope{
		RequestID: "req-1", PlanID: "7", PlanName: "员工请假单（集团）-自动回归",
		ExecutionPathID: "13", ExecutionPathName: "执行路径 1",
	})
	expected := filepath.Join(root, "plans", "员工请假单（集团）-自动回归__plan-7", "configuration", "执行路径 1__path-13", "2026-09-03")
	if configurationDir != expected {
		t.Fatalf("配置阶段目录不正确：\n实际 %s\n期望 %s", configurationDir, expected)
	}
	runDir := router.BucketDir(logging.Scope{
		RequestID: "req-1", PlanID: "7", PlanName: "员工请假单（集团）-自动回归",
		ExecutionPathID: "13", ExecutionPathName: "执行路径 1", RunSeq: "run-1782741614477351000-1",
	})
	expectedRun := filepath.Join(root, "plans", "员工请假单（集团）-自动回归__plan-7", "runs", "执行路径 1__path-13", "run-1782741614477351000-1")
	if runDir != expectedRun {
		t.Fatalf("执行阶段目录不正确：\n实际 %s\n期望 %s", runDir, expectedRun)
	}
}

// TestSanitizePathSegmentKeepsChineseAndBlocksTraversal 验证目录段只替换斜杠、反斜杠、控制字符与路径穿越，
// 中文、括号、普通横线与空格必须原样保留，目录名要能对上界面上的名称。
func TestSanitizePathSegmentKeepsChineseAndBlocksTraversal(t *testing.T) {
	if kept := logging.SanitizePathSegment("员工请假单（集团）-自动回归 1"); kept != "员工请假单（集团）-自动回归 1" {
		t.Fatalf("中文、括号、横线与空格被误改：%s", kept)
	}
	for _, dangerous := range []string{"../路径 1", "oyg测试/001", `目录\子目录`, "时间 18:56"} {
		cleaned := logging.SanitizePathSegment(dangerous)
		for _, forbidden := range []string{"..", "/", "\\", ":"} {
			if strings.Contains(cleaned, forbidden) {
				t.Fatalf("清洗后仍包含 %q：%s -> %s", forbidden, dangerous, cleaned)
			}
		}
	}
	if empty := logging.SanitizePathSegment("  "); empty != "unknown" {
		t.Fatalf("空目录段没有回落占位：%s", empty)
	}
}

// TestCleanupExpiredRemovesExpiredPlanBuckets 验证保留期清理按新目录结构工作：
// 计划配置目录按日期删，运行目录按最后修改时间删，当天目录一律保留，并顺带收掉空掉的父目录。
func TestCleanupExpiredRemovesExpiredPlanBuckets(t *testing.T) {
	root := t.TempDir()
	now := fixedTime()
	pathDir := filepath.Join(root, "plans", "员工请假单（集团）__plan-7", "configuration", "执行路径 1__path-13")
	today := filepath.Join(pathDir, now.Format("2006-01-02"))
	expired := filepath.Join(pathDir, now.AddDate(0, 0, -9).Format("2006-01-02"))
	kept := filepath.Join(pathDir, now.AddDate(0, 0, -2).Format("2006-01-02"))
	runsPathDir := filepath.Join(root, "plans", "员工请假单（集团）__plan-7", "runs", "执行路径 1__path-13")
	expiredRun := filepath.Join(runsPathDir, "run-old")
	keptRun := filepath.Join(runsPathDir, "run-new")
	for _, dir := range []string{today, expired, kept, expiredRun, keptRun} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("准备目录失败：%v", err)
		}
	}
	// 运行目录名是运行号不含日期，只能按最后修改时间判断过期。
	if err := os.Chtimes(expiredRun, now.AddDate(0, 0, -20), now.AddDate(0, 0, -20)); err != nil {
		t.Fatalf("设置运行目录时间失败：%v", err)
	}
	removed := logging.CleanupExpired(root, logging.DefaultRetentionDays, now)
	if len(removed) != 2 {
		t.Fatalf("只应删除一个过期配置日期目录和一个过期运行目录：%v", removed)
	}
	for _, dir := range []string{today, kept, keptRun} {
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("目录被误删：%s", dir)
		}
	}
	for _, dir := range []string{expired, expiredRun} {
		if _, err := os.Stat(dir); err == nil {
			t.Fatalf("过期目录没有被删除：%s", dir)
		}
	}
}

// TestDefaultRetentionIsSevenDays 锁定默认保留七天，避免日志目录无限增长。
func TestDefaultRetentionIsSevenDays(t *testing.T) {
	if logging.DefaultRetentionDays != 7 {
		t.Fatalf("默认保留天数应为 7，实际 %d", logging.DefaultRetentionDays)
	}
	t.Setenv(logging.RetentionDaysEnv, "3")
	if days := logging.RetentionDays(); days != 3 {
		t.Fatalf("环境变量没有覆盖保留天数：%d", days)
	}
	t.Setenv(logging.RetentionDaysEnv, "abc")
	if days := logging.RetentionDays(); days != logging.DefaultRetentionDays {
		t.Fatalf("非法保留天数没有回落默认值：%d", days)
	}
}

// TestCleanupExpiredRemovesOnlyExpiredApplicationDirs 验证应用程序日志按日期目录清理，
// 当天目录与解析不出日期的目录一律保留。
func TestCleanupExpiredRemovesOnlyExpiredApplicationDirs(t *testing.T) {
	root := t.TempDir()
	now := fixedTime()
	today := filepath.Join(root, "application", now.Format("2006-01-02"))
	expired := filepath.Join(root, "application", now.AddDate(0, 0, -20).Format("2006-01-02"))
	kept := filepath.Join(root, "application", now.AddDate(0, 0, -3).Format("2006-01-02"))
	unparsable := filepath.Join(root, "application", "not-a-date")
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
