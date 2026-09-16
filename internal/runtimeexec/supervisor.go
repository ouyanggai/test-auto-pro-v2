// 执行服务托管：配置 RUNTIME_EXECUTOR_URL 后由 Go 把本机 executor-service 作为子进程拉起。
// F-036 T001 已把工作目录改为根目录 executor-service/；T011 将删除本托管。
// 边界：只允许托管本机地址（环回、0.0.0.0 或本机网卡 IP）；远端执行服务必须由部署方自行启动。

package runtimeexec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// 托管启动的默认预算：执行服务要启动无头 Chromium，冷启动留足 60 秒。
const (
	executorReadyTimeout = 60 * time.Second
	executorReadyPoll    = 300 * time.Millisecond
)

// ExecutorSupervisor 是一个被托管执行服务子进程的句柄；Stop 随后端退出回收进程。
type ExecutorSupervisor struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
	done   chan struct{}
	// started 为 false 表示探测到服务已在运行，本进程没有拉起新子进程，Stop 是空操作。
	started bool
}

// Started 报告本进程是否真的拉起了执行服务子进程（复用模式为 false）。
func (s *ExecutorSupervisor) Started() bool {
	return s != nil && s.started
}

// StartExecutorSupervisor 托管启动本机执行服务：
//  1. baseURL 必须指向环回地址且带端口，否则拒绝托管（远端服务不由本进程负责）；
//  2. 端口已被监听时视为服务已在运行，直接复用（人工启动的现场不被破坏）；
//  3. 缺少 executor-service 源码或 Node.js 时给出明确中文错误并让启动失败。
func StartExecutorSupervisor(baseURL, workspaceRoot string) (*ExecutorSupervisor, error) {
	host, port, err := executorHostPort(baseURL)
	if err != nil {
		return nil, err
	}
	executorDir := filepath.Join(workspaceRoot, "executor-service")
	if _, statErr := os.Stat(filepath.Join(executorDir, "src", "server.ts")); statErr != nil {
		return nil, fmt.Errorf("缺少独立执行服务源码（%s），请确认 executor-service/src/server.ts 存在", executorDir)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		return nil, errors.New("未找到 node 可执行文件，无法托管执行服务（需要 Node.js ≥ 22）")
	}

	// 服务已在运行：直接复用，不重复拉起（与人工按清单启动的流程兼容）。
	if executorReady(host, port) {
		return &ExecutorSupervisor{}, nil
	}
	_, cancel := context.WithCancel(context.Background())
	cmd := exec.Command(node, "--experimental-strip-types", "src/server.ts")
	cmd.Dir = executorDir
	cmd.Env = append(os.Environ(), "EXECUTOR_PORT="+port)
	// 执行服务的 JSON 启动行与页面异常直接进后端标准输出，内网日志原样落盘。
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("启动执行服务失败：%w", err)
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	deadline := time.Now().Add(executorReadyTimeout)
	for time.Now().Before(deadline) {
		// 子进程先行退出（端口占用、产物缺失、Chromium 启动失败）：带着真实退出原因报错。
		select {
		case <-done:
			cancel()
			return nil, fmt.Errorf("执行服务启动后立即退出，请检查端口 %s 与 executor-service 源码（stdout/stderr 有原始输出）", port)

		default:
		}
		if executorReady(host, port) {
			return &ExecutorSupervisor{cmd: cmd, cancel: cancel, done: done, started: true}, nil
		}
		time.Sleep(executorReadyPoll)
	}
	StopExecutorSupervisor(&ExecutorSupervisor{cmd: cmd, cancel: cancel, done: done, started: true})
	return nil, fmt.Errorf("执行服务在 %s 内未就绪（%s:%s）", executorReadyTimeout, host, port)
}

// StopExecutorSupervisor 停止被托管执行服务：先 SIGTERM 让它销毁全部会话与浏览器，
// 10 秒后强制杀掉；复用模式下（未拉起子进程）是空操作，绝不误伤人工启动的服务。
func StopExecutorSupervisor(supervisor *ExecutorSupervisor) {
	if supervisor == nil || !supervisor.started || supervisor.cmd == nil || supervisor.cmd.Process == nil {
		return
	}
	_ = supervisor.cmd.Process.Signal(os.Interrupt)
	select {
	case <-supervisor.done:
	case <-time.After(10 * time.Second):
		_ = supervisor.cmd.Process.Kill()
	}
	supervisor.cancel()
}

// executorReady 探测执行服务是否可接受 HTTP 请求：优先打 /health。
// 任意 HTTP 应答都算端口就绪；连接失败才视为未启动。
func executorReady(host, port string) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://" + net.JoinHostPort(host, port) + "/health")
	if err != nil {
		return false
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1))
	return true
}

// isLocalExecutorHost 判断执行服务地址是否指向本机：环回名、全网卡监听或本机任一网卡 IP 均算本机。
// Go 进程只能托管本机子进程，远端 IP 一律拒绝托管；网卡地址在每次调用时实时读取，插拔网卡后无需改配置。
func isLocalExecutorHost(host string) bool {
	switch host {
	case "127.0.0.1", "localhost", "::1", "0.0.0.0":
		return true
	}
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, address := range addresses {
		if ipNet, ok := address.(*net.IPNet); ok && ipNet.IP.String() == host {
			return true
		}
	}
	return false
}

func executorHostPort(baseURL string) (string, string, error) {
	trimmed := strings.TrimSpace(baseURL)
	parsed, parseErr := url.Parse(trimmed)
	if parseErr != nil || parsed.Host == "" {
		// 允许省略协议的 host:port 写法。
		if !strings.Contains(trimmed, "://") {
			parsed, parseErr = url.Parse("http://" + trimmed)
		}
		if parseErr != nil || parsed.Host == "" {
			return "", "", fmt.Errorf("执行服务地址不合法：%s", baseURL)
		}
	}
	host := parsed.Hostname()
	port := parsed.Port()
	if host == "" || port == "" {
		return "", "", fmt.Errorf("执行服务地址必须携带端口：%s", baseURL)
	}
	// 托管的前提是执行服务跑在本机：环回地址、全网卡监听 0.0.0.0 或本机任一网卡 IP 都满足；
	// 解析不到的本机地址才视为远端服务，必须由部署方自行启动并保持本进程不托管。
	if !isLocalExecutorHost(host) {
		return "", "", fmt.Errorf("执行服务地址 %s 不在本机上，远端服务请由部署方自行启动并保持本进程不托管", host)
	}
	if _, convErr := strconv.Atoi(port); convErr != nil {
		return "", "", fmt.Errorf("执行服务端口不合法：%s", baseURL)
	}
	return host, port, nil
}

// ExecutorHostPortForTest 暴露托管守卫的本机地址判定，供单元测试锁定本机 IP 放行与远端拒绝口径。
func ExecutorHostPortForTest(baseURL string) (string, string, error) {
	return executorHostPort(baseURL)
}
