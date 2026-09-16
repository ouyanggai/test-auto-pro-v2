// 执行服务托管（评审 P1-1 修复）：配置 RUNTIME_EXECUTOR_URL 后由 Go 服务把本机
// form-runtime/executor（无头 Chromium 执行服务）作为子进程拉起并做就绪探测，
// 随后端停止而停止——生产部署不再依赖"有人手动起 19011 服务"。
// 边界：只允许托管环回地址；远端执行服务必须由部署方自行启动（本进程无法跨机托管）。
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
//  3. 缺少 form-runtime 构建产物或 Node.js 时给出明确中文错误并让启动失败——
//     runtime 路由已显式启用，静默降级会让动作在执行中才失败。
func StartExecutorSupervisor(baseURL, workspaceRoot string) (*ExecutorSupervisor, error) {
	host, port, err := executorHostPort(baseURL)
	if err != nil {
		return nil, err
	}
	executorDir := filepath.Join(workspaceRoot, "form-runtime", "executor")
	distRoot := filepath.Join(workspaceRoot, "web", "dist", "form-runtime", "index.html")
	if _, statErr := os.Stat(distRoot); statErr != nil {
		return nil, fmt.Errorf("缺少 form-runtime 构建产物（%s），请先执行 pnpm --dir form-runtime build", distRoot)
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
			return nil, fmt.Errorf("执行服务启动后立即退出，请检查端口 %s 与 form-runtime 构建（stdout/stderr 有原始输出）", port)
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

// executorReady 探测执行服务是否可接受 HTTP 请求：任意响应（含 404）都算就绪，
// 因为服务的静态与 /runtime 路由在同一个监听器上，能应答就说明进程与端口已就绪。
func executorReady(host, port string) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://" + net.JoinHostPort(host, port) + "/")
	if err != nil {
		return false
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1))
	return true
}

// executorHostPort 校验并拆出托管目标的主机与端口：只允许环回地址且必须显式带端口。
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
	// 0.0.0.0 是本机全网卡监听，进程仍在本机托管浏览器，与 127.0.0.1 同等放行；
	// 其它具体 IP 视为远端服务，必须由部署方自行启动并保持本进程不托管。
	switch host {
	case "127.0.0.1", "localhost", "::1", "0.0.0.0":
	default:
		return "", "", fmt.Errorf("只允许托管环回地址的执行服务（%s），远端服务请由部署方自行启动并保持本进程不托管", host)
	}
	if _, convErr := strconv.Atoi(port); convErr != nil {
		return "", "", fmt.Errorf("执行服务端口不合法：%s", baseURL)
	}
	return host, port, nil
}
