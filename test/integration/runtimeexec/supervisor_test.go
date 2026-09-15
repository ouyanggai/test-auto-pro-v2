// 评审 P1-1 集成回归：配置执行服务地址后，Go 服务可以托管拉起本机执行服务子进程
// （真实无头 Chromium），就绪探测可用，随后端停止而停止；地址校验拒绝远端与缺端口形式。
// 依赖与既有 Go 集成用例一致：Node.js ≥ 22 与 form-runtime 构建产物（先 pnpm --dir form-runtime build）。
package runtimeexec_test

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/runtimeexec"
)

// TestManagedExecutorSupervisorStartsAndStops 托管启动：真实拉起执行服务并就绪，
// 停止后端口释放；复用模式（服务已在运行）不拉起新子进程。
func TestManagedExecutorSupervisorStartsAndStops(t *testing.T) {
	if reason := executorEnvMissing(); reason != "" {
		t.Skipf("执行服务运行条件缺失：%s", reason)
	}
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatalf("解析工作区根失败：%v", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "web", "dist", "form-runtime", "index.html")); statErr != nil {
		t.Skipf("缺少 form-runtime 构建产物（先执行 pnpm --dir form-runtime build）")
	}
	port := freePort(t)
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	supervisor, err := runtimeexec.StartExecutorSupervisor(base, root)
	if err != nil {
		t.Fatalf("托管启动失败：%v", err)
	}
	if !supervisor.Started() {
		t.Fatalf("空闲端口应真实拉起执行服务子进程")
	}
	// 就绪探测已通过：状态查询端点必须可应答（会话不存在返回 404，也是服务就绪事实）。
	statusClient := &http.Client{Timeout: 5 * time.Second}
	response, err := statusClient.Get(base + "/runtime/session/none/status")
	if err != nil {
		t.Fatalf("托管启动后服务不可达：%v", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	runtimeexec.StopExecutorSupervisor(supervisor)
	// 停止后进程必须退出：端口在短时间内不可再监听（给执行服务 10 秒优雅退出窗口）。
	stoppedAt := time.Now().Add(12 * time.Second)
	for time.Now().Before(stoppedAt) {
		conn, dialErr := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
		if dialErr != nil {
			return
		}
		_ = conn.Close()
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("托管停止后执行服务端口 %d 仍在监听", port)
}

// TestManagedExecutorRejectsRemoteAndPortlessAddresses 地址校验（评审 P1-1 纪律）：
// 远端地址不由本进程托管；缺端口的地址无法托管，必须在启动阶段显式失败。
func TestManagedExecutorRejectsRemoteAndPortlessAddresses(t *testing.T) {
	root := "../../../"
	cases := []struct {
		name    string
		baseURL string
		want    string
	}{
		{"远端地址", "http://10.0.0.8:19011", "环回"},
		{"缺端口", "http://127.0.0.1", "端口"},
		{"纯主机名", "executor.example.internal", "端口"},
	}
	for _, item := range cases {
		if _, err := runtimeexec.StartExecutorSupervisor(item.baseURL, root); err == nil {
			t.Fatalf("%s：必须拒绝托管 %s", item.name, item.baseURL)
		} else if !containsAll(err.Error(), item.want) {
			t.Fatalf("%s：错误必须说明 %s，实际 %v", item.name, item.want, err)
		}
	}
}

// containsAll 判断文本是否包含全部关键词。
func containsAll(text string, wants ...string) bool {
	for _, want := range wants {
		if !strings.Contains(text, want) {
			return false
		}
	}
	return true
}
