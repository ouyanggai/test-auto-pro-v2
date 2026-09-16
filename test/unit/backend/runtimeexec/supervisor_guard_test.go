package runtimeexec_test

import (
	"os"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/runtimeexec"
)

// TestExecutorHostPortAcceptsLocalNetworkIP 锁定托管守卫的本机地址判定（2026-09-16 用户裁决）：
// 服务监听 0.0.0.0 后，RUNTIME_EXECUTOR_URL 允许写本机网卡 IP（如 http://192.168.1.212:19011），
// 不能再按“非环回即远端”拒绝，否则服务进程直接退出。
func TestExecutorHostPortAcceptsLocalNetworkIP(t *testing.T) {
	host, port, err := runtimeexec.ExecutorHostPortForTest("http://192.168.1.212:19011")
	if err != nil {
		t.Fatalf("本机网卡 IP 必须允许托管：%v", err)
	}
	if host != "192.168.1.212" || port != "19011" {
		t.Fatalf("解析结果不正确：host=%s port=%s", host, port)
	}
}

// TestExecutorHostPortStillRejectsRemoteIP 锁定远端守卫：真正不属于本机的 IP 仍拒绝托管。
func TestExecutorHostPortStillRejectsRemoteIP(t *testing.T) {
	// TEST-NET-3（203.0.113.0/24）不会分配给本机网卡，作为远端地址样本。
	if _, _, err := runtimeexec.ExecutorHostPortForTest("http://203.0.113.7:19011"); err == nil {
		t.Fatalf("远端 IP 必须拒绝托管")
	}
}

// TestSupervisorSourceDoesNotReferenceFormRuntimeExecutor 锁定 F-036 T001：
// Go 托管入口不再引用 form-runtime/executor 路径。
func TestSupervisorSourceDoesNotReferenceFormRuntimeExecutor(t *testing.T) {
	raw, err := os.ReadFile("../../../../internal/runtimeexec/supervisor.go")
	if err != nil {
		t.Fatalf("读取 supervisor.go 失败：%v", err)
	}
	text := string(raw)
	if strings.Contains(text, "form-runtime/executor") || strings.Contains(text, `"form-runtime", "executor"`) {
		t.Fatalf("supervisor.go 不得再引用 form-runtime/executor")
	}
	if !strings.Contains(text, "executor-service") {
		t.Fatalf("supervisor.go 必须引用独立 executor-service")
	}
}
