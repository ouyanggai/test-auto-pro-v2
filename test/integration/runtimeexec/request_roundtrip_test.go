// T007 集成测试：拉起真实 form-runtime 执行服务（无头 Chromium）与真实 Go 客户端，
// 走通「创建会话 → 装载固定路由 → WS 命令往返（capture）→ 销毁会话」的完整闭环。
// 目标出口使用假件：本测试不访问真实目标平台（真实目标链路由 T015 人工闭环验证）。
package runtimeexec_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/runtimeexec"
)

// executorEnvMissing 返回本机缺少执行服务运行条件的原因；齐全时返回空串。
func executorEnvMissing() string {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		return err.Error()
	}
	if _, err := os.Stat(filepath.Join(root, "form-runtime", "executor", "node_modules")); err != nil {
		return "缺少 form-runtime/executor/node_modules，先执行 pnpm install"
	}
	if _, err := os.Stat(filepath.Join(root, "web", "dist", "form-runtime", "index.html")); err != nil {
		return "缺少 form-runtime 构建产物，先执行 pnpm --dir form-runtime build"
	}
	return ""
}

// freePort 申请一个可用端口。
func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("分配端口失败：%v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// fakeForwarderInt 是集成测试的假目标出口：不访问真实目标平台。
type fakeForwarderInt struct{}

func (f *fakeForwarderInt) ForwardRuntimeRequest(_ context.Context, _ target.Session, _ target.PageForwardRequest) (*target.PageForwardResponse, error) {
	return &target.PageForwardResponse{StatusCode: 200, Headers: map[string]string{"content-type": "application/json"}, Body: []byte(`{"isSuccess":true}`)}, nil
}

// fakeSessionsInt 提供固定会话。
type fakeSessionsInt struct{}

func (f *fakeSessionsInt) Current(_ context.Context, _ string) (target.Session, error) {
	return target.Session{SID: "sid-int", Summary: target.AccountSummary{Account: "acct"}}, nil
}

func (f *fakeSessionsInt) Refresh(_ context.Context, _ string) (target.Session, error) {
	return target.Session{SID: "sid-int", Summary: target.AccountSummary{Account: "acct"}}, nil
}

// waitForExecutor 等待执行服务端口可连。
func waitForExecutor(t *testing.T, port int) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("执行服务 30 秒内未就绪")
}

// TestExecutorServiceRequestRoundtrip 真实服务闭环：创建会话 → 装载 → capture 命令往返 → 销毁。
func TestExecutorServiceRequestRoundtrip(t *testing.T) {
	if reason := executorEnvMissing(); reason != "" {
		t.Skipf("执行服务运行条件缺失：%s", reason)
	}
	port := freePort(t)
	cmd := exec.Command("node", "--experimental-strip-types", "src/server.ts")
	cmd.Dir = filepath.Join("..", "..", "..", "form-runtime", "executor")
	cmd.Env = append(os.Environ(), fmt.Sprintf("EXECUTOR_PORT=%d", port))
	stderr := &strings.Builder{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("启动执行服务失败：%v", err)
	}
	defer func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan struct{})
		go func() { _, _ = cmd.Process.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(15 * time.Second):
			_ = cmd.Process.Kill()
		}
	}()
	waitForExecutor(t, port)

	client, err := runtimeexec.NewClient(fmt.Sprintf("http://127.0.0.1:%d", port), &fakeForwarderInt{}, &fakeSessionsInt{})
	if err != nil {
		t.Fatalf("构造客户端失败：%v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// ① 创建会话：真实无头 Chromium Context + 固定路由装载 + 桥接注册。
	handle, err := client.CreateSession(ctx, runtimeexec.SessionSpec{AccountKey: "acct-int", RunID: "run-int", PathRunID: "pr-int", StepID: "step-int"})
	if err != nil {
		t.Fatalf("创建会话失败：%v", err)
	}

	// ② 装载（发起模式、空模板）：走真实页面桥接 loadInitiate → App load 链路。
	loadEnvelope := runtimeexec.Message{
		Kind: runtimeexec.KindLoad, RequestID: "req-load", SessionID: handle.SessionID,
		RunID: "run-int", PathRunID: "pr-int", StepID: "step-int",
		Payload: json.RawMessage(`{"mode":"initiate","renderType":"formmaking","template":{"list":[],"config":{}},"values":{},"permissions":[]}`),
	}
	loadResult, err := client.Load(ctx, handle.SessionID, loadEnvelope)
	if err != nil {
		t.Fatalf("装载失败：%v", err)
	}
	if !strings.Contains(string(loadResult), `"ready":true`) {
		t.Fatalf("装载结果应为 ready：true：%s", loadResult)
	}

	// ③ WS 命令往返：capture 走真实页面 getValues；空表单也应返回 success 结果。
	var events int
	var mu sync.Mutex
	result, err := client.Execute(ctx, handle.SessionID, "acct-int", runtimeexec.Message{
		Kind: runtimeexec.KindExecute, RequestID: "req-exec", SessionID: handle.SessionID,
		RunID: "run-int", PathRunID: "pr-int", StepID: "step-int",
		Payload: json.RawMessage(`{"action":"capture"}`),
	}, runtimeexec.ExecuteCallbacks{OnEvent: func(_ *runtimeexec.Message) {
		mu.Lock()
		events++
		mu.Unlock()
	}})
	if err != nil {
		if !errors.Is(err, runtimeexec.ErrUncertain) && !errors.Is(err, runtimeexec.ErrFailedCleanly) {
			t.Fatalf("capture 往返失败：%v（stderr：%s）", err, stderr.String())
		}
		t.Fatalf("capture 往返失败：%v（stderr：%s）", err, stderr.String())
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(result.Payload, &payload); err != nil {
		t.Fatalf("结果载荷解析失败：%v（%s）", err, result.Payload)
	}
	if payload.Status != "success" {
		t.Fatalf("capture 应返回 success，实际 %s（%s）", payload.Status, result.Payload)
	}

	// ④ 销毁会话。
	if err := client.Destroy(ctx, handle.SessionID); err != nil {
		t.Fatalf("销毁会话失败：%v", err)
	}
	if _, err := client.Load(ctx, handle.SessionID, loadEnvelope); err == nil {
		t.Fatalf("销毁后的会话不允许再装载")
	}
}

// TestExecutorServiceStatusEndpoint 状态查询只读：返回在途与轨迹统计，不触发目标请求。
func TestExecutorServiceStatusEndpoint(t *testing.T) {
	if reason := executorEnvMissing(); reason != "" {
		t.Skipf("执行服务运行条件缺失：%s", reason)
	}
	port := freePort(t)
	cmd := exec.Command("node", "--experimental-strip-types", "src/server.ts")
	cmd.Dir = filepath.Join("..", "..", "..", "form-runtime", "executor")
	cmd.Env = append(os.Environ(), fmt.Sprintf("EXECUTOR_PORT=%d", port))
	if err := cmd.Start(); err != nil {
		t.Fatalf("启动执行服务失败：%v", err)
	}
	defer func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		time.Sleep(500 * time.Millisecond)
	}()
	waitForExecutor(t, port)
	// 未创建会话时查询返回 404：证明状态端点不创建任何现场。
	response, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/runtime/session/none/status", port))
	if err != nil {
		t.Fatalf("状态查询失败：%v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("未知会话应返回 404，实际 %d", response.StatusCode)
	}
}
