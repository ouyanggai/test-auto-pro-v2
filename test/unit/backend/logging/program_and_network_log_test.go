package logging_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/logging"
)

// readBucketFile 读取配置桶内指定日志文件的全部内容，缺失时直接失败。
func readBucketFile(t *testing.T, root, name string) string {
	t.Helper()
	path := filepath.Join(root, "config", fixedTime().Format("2006-01-02"), name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败：%v", name, err)
	}
	return string(content)
}

// TestErrorLogWritesClassChainSourceAndUserMessage 验证程序错误日志的八类分类、错误链、
// 来源定位与界面同源提示，并确认全局与桶内四个位置都写到了。
func TestErrorLogWritesClassChainSourceAndUserMessage(t *testing.T) {
	root := t.TempDir()
	logger := logging.NewLogger(logging.NewRouter(root, fixedTime), fixedTime)
	scope := logging.Scope{RequestID: "req-err"}
	inner := os.ErrPermission
	logger.Error(scope, logging.ErrorRecord{
		Message: "读取计划失败", Class: logging.ClassToolStorage, Err: inner,
		UserMessage: "暂时无法读取计划，请重试",
	})
	for _, name := range []string{"program.log", "program-error.log"} {
		content := readBucketFile(t, root, name)
		if !strings.Contains(content, "error_class=tool_storage") {
			t.Fatalf("%s 缺少错误分类：%s", name, content)
		}
		if !strings.Contains(content, "user_message=暂时无法读取计划，请重试") {
			t.Fatalf("%s 缺少界面同源提示：%s", name, content)
		}
		if !strings.Contains(content, "source=logging/logger.go:") && !strings.Contains(content, "source=logging/") {
			t.Fatalf("%s 缺少来源定位：%s", name, content)
		}
		if !strings.Contains(content, "permission_denied") {
			t.Fatalf("%s 缺少错误链：%s", name, content)
		}
	}
	// 全局日志按天分文件，配合保留期滚动删除，避免单文件无限增长。
	for _, name := range []string{"app.log", "app-error.log"} {
		daily := logging.DailyFileName(name, fixedTime())
		if !strings.Contains(daily, "2026-09-03") {
			t.Fatalf("全局日志没有按天命名：%s", daily)
		}
		if _, err := os.Stat(filepath.Join(root, daily)); err != nil {
			t.Fatalf("全局 %s 没有写入：%v", daily, err)
		}
	}
}

// TestUnknownClassFollowsToolIssue 验证无法判断的错误按工具问题跟进，并且仍然写 error 级别。
func TestUnknownClassFollowsToolIssue(t *testing.T) {
	root := t.TempDir()
	logging.NewLogger(logging.NewRouter(root, fixedTime), fixedTime).
		Error(logging.Scope{RequestID: "req-unknown"}, logging.ErrorRecord{Message: "未分类失败"})
	content := readBucketFile(t, root, "program-error.log")
	if !strings.Contains(content, "level=error") || !strings.Contains(content, "error_class=unknown") {
		t.Fatalf("未分类错误没有按 error 级别落盘：%s", content)
	}
	if !strings.Contains(content, "followup=按工具问题跟进") {
		t.Fatalf("未分类错误没有标注按工具问题跟进：%s", content)
	}
}

// TestPanicStackWritesBoundedBlock 验证 panic 栈按块包裹写入且长度被截断。
func TestPanicStackWritesBoundedBlock(t *testing.T) {
	root := t.TempDir()
	logging.NewLogger(logging.NewRouter(root, fixedTime), fixedTime).
		Error(logging.Scope{RequestID: "req-panic"}, logging.ErrorRecord{
			Message: "请求处理发生未预期错误", Class: logging.ClassToolBug,
			Stack: strings.Repeat("goroutine 1 [running]:\n", 2000),
		})
	content := readBucketFile(t, root, "program-error.log")
	if !strings.Contains(content, "--- begin stack trace_id=req-panic ---") || !strings.Contains(content, "--- end stack trace_id=req-panic ---") {
		t.Fatalf("panic 栈没有按块包裹：%s", content[:200])
	}
	if !strings.Contains(content, "栈已截断") {
		t.Fatalf("panic 栈没有被截断：长度 %d", len(content))
	}
}

// TestTargetKindClassCoversAllEightClasses 验证目标侧分类映射与八类取值集合。
func TestTargetKindClassCoversAllEightClasses(t *testing.T) {
	cases := map[string]string{
		"login_rejected":    logging.ClassTargetRuntime,
		"permission_denied": logging.ClassTargetRuntime,
		"session_expired":   logging.ClassTargetRuntime,
		"response_invalid":  logging.ClassTargetContract,
		"timeout":           logging.ClassNetwork,
		"unavailable":       logging.ClassNetwork,
		"":                  logging.ClassUnknown,
	}
	for kind, expected := range cases {
		if got := logging.TargetKindClass(kind); got != expected {
			t.Fatalf("目标分类 %q 映射错误：期望 %s 实际 %s", kind, expected, got)
		}
	}
	all := map[string]bool{
		logging.ClassToolBug: true, logging.ClassToolConfig: true, logging.ClassToolStorage: true,
		logging.ClassToolDependency: true, logging.ClassTargetContract: true, logging.ClassTargetRuntime: true,
		logging.ClassNetwork: true, logging.ClassUnknown: true,
	}
	if len(all) != 8 {
		t.Fatalf("错误分类不是固定八类：%d", len(all))
	}
}

// TestNetworkLogSplitsSuccessAndFailureAndLinksCurl 验证目标请求日志分流与三处双向可查：
// 成功进 network.log，失败同时进 network-error.log，可重放命令按块写进 curl.log。
func TestNetworkLogSplitsSuccessAndFailureAndLinksCurl(t *testing.T) {
	root := t.TempDir()
	logger := logging.NewLogger(logging.NewRouter(root, fixedTime), fixedTime)
	scope := logging.Scope{RequestID: "req-net"}
	logger.Network(scope, logging.NetworkRecord{
		TraceID: "trace-ok", Method: "POST", Endpoint: "/web/flowProxy/findById", StatusCode: 200,
		Duration: 1500 * time.Millisecond, Result: "success", OutcomeKind: "business_success",
		Curl: "curl -sS -X POST 'https://target/web/flowProxy/findById'", ResponseBody: `{"isSuccess":true}`,
	})
	logger.Network(scope, logging.NetworkRecord{
		TraceID: "trace-bad", Method: "POST", Endpoint: "/web/flowInstanceApi/list",
		Duration: 2 * time.Second, Result: "failure", ErrorType: "timeout",
		Curl: "curl -sS -X POST 'https://target/web/flowInstanceApi/list'",
	})
	network := readBucketFile(t, root, "network.log")
	if !strings.Contains(network, "trace_id=trace-ok") || !strings.Contains(network, "duration_s=1.500") {
		t.Fatalf("network.log 缺少成功请求或耗时：%s", network)
	}
	if !strings.Contains(network, "request_class=read") || strings.Contains(network, "request_class=write") {
		t.Fatalf("当前切片写端点白名单为空，请求分类必须是 read：%s", network)
	}
	failure := readBucketFile(t, root, "network-error.log")
	if !strings.Contains(failure, "trace_id=trace-bad") || !strings.Contains(failure, "error_type=timeout") {
		t.Fatalf("network-error.log 缺少失败请求：%s", failure)
	}
	if strings.Contains(failure, "trace_id=trace-ok") {
		t.Fatalf("成功请求不应写进 network-error.log：%s", failure)
	}
	curl := readBucketFile(t, root, "curl.log")
	for _, traceID := range []string{"trace-ok", "trace-bad"} {
		if !strings.Contains(curl, "--- begin curl trace_id="+traceID+" ---") {
			t.Fatalf("curl.log 缺少 %s 的块：%s", traceID, curl)
		}
	}
	if !strings.Contains(curl, `{"isSuccess":true}`) {
		t.Fatalf("curl.log 缺少完整响应正文：%s", curl)
	}
}

// TestCurlCommandMatchesActualRequest 验证生成的命令逐字对应真实请求，含会话值且可直接重放。
func TestCurlCommandMatchesActualRequest(t *testing.T) {
	command := logging.CurlCommand("post", "https://target/web/user/api/login/user/login?platformCode=invest",
		map[string]string{"Content-Type": "application/json", "sid": "sid-real-value"}, `{"data":{"a":"b'c"}}`)
	for _, expected := range []string{
		"curl -sS -X POST 'https://target/web/user/api/login/user/login?platformCode=invest'",
		"-H 'Content-Type: application/json'",
		"-H 'sid: sid-real-value'",
		`--data-raw '{"data":{"a":"b'\''c"}}'`,
	} {
		if !strings.Contains(command, expected) {
			t.Fatalf("命令缺少 %s：%s", expected, command)
		}
	}
	repeat := logging.CurlCommand("post", "https://target/x",
		map[string]string{"sid": "s", "Content-Type": "application/json"}, "{}")
	again := logging.CurlCommand("post", "https://target/x",
		map[string]string{"Content-Type": "application/json", "sid": "s"}, "{}")
	if repeat != again {
		t.Fatalf("同一请求生成的命令不稳定：\n%s\n%s", repeat, again)
	}
}

// TestNetworkLogUsesContextScope 验证网络日志的关联键来自 context 作用域，不靠调用方逐处传参。
func TestNetworkLogUsesContextScope(t *testing.T) {
	root := t.TempDir()
	logger := logging.NewLogger(logging.NewRouter(root, fixedTime), fixedTime)
	ctx := logging.WithScope(context.Background(), logging.Scope{RequestID: "req-from-context"})
	logger.Network(logging.ScopeFrom(ctx), logging.NetworkRecord{
		Method: "POST", Endpoint: "/web/x", StatusCode: 200, Result: "success",
	})
	if content := readBucketFile(t, root, "network.log"); !strings.Contains(content, "request_id=req-from-context") {
		t.Fatalf("网络日志没有带上 context 作用域：%s", content)
	}
}
