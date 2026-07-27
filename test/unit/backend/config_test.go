package backend_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/config"
)

func TestServerAddressDefaultsToProjectPort(t *testing.T) {
	t.Setenv("TEST_AUTO_PRO_SERVER_ADDR", "")
	if got := config.ServerAddress(); got != "127.0.0.1:19080" {
		t.Fatalf("默认监听地址 = %q", got)
	}
}

func TestTargetConfigUsesV1NonSensitiveDefaults(t *testing.T) {
	disableLocalTargetConfig(t)
	for _, name := range []string{
		"TARGET_API_GATEWAY", "TARGET_LOGIN_PASSWORD", "TARGET_LOGIN_AES_KEY", "TARGET_LOGIN_CODE",
		"TARGET_PLATFORM_CODE", "TARGET_TEMPLATE_PLATFORM_CODES", "TARGET_CUSTOMER_CODE",
		"TARGET_SESSION_TTL", "TARGET_HTTP_TIMEOUT",
	} {
		t.Setenv(name, "")
	}

	cfg := config.LoadTargetConfig()
	if cfg.PlatformCode != "200001" || cfg.TemplatePlatformCodes != "200001,999999" {
		t.Fatal("未采用已批准的 V1 非敏感平台代码默认值")
	}
	if cfg.SessionTTL != 8*time.Hour || cfg.HTTPTimeout != 120*time.Second {
		t.Fatal("未采用已批准的 V1 超时或会话 TTL 默认值")
	}
	missing := strings.Join(cfg.MissingRequired(), ",")
	for _, name := range []string{"TARGET_API_GATEWAY", "TARGET_LOGIN_PASSWORD", "TARGET_LOGIN_AES_KEY", "TARGET_LOGIN_CODE"} {
		if !strings.Contains(missing, name) {
			t.Fatalf("缺失项未包含 %s", name)
		}
	}
}

func TestTargetConfigReadsSensitiveValuesWithoutEchoingThem(t *testing.T) {
	disableLocalTargetConfig(t)
	password := generatedValue(t, 12)
	aesKey := generatedValue(t, 8)
	loginCode := generatedValue(t, 6)
	t.Setenv("TARGET_API_GATEWAY", "https://target.example.invalid/gateway")
	t.Setenv("TARGET_LOGIN_PASSWORD", password)
	t.Setenv("TARGET_LOGIN_AES_KEY", aesKey)
	t.Setenv("TARGET_LOGIN_CODE", loginCode)
	t.Setenv("TARGET_SESSION_TTL", "30m")
	t.Setenv("TARGET_HTTP_TIMEOUT", "45s")

	cfg := config.LoadTargetConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("完整配置被判断为缺失：%v", missing)
	}
	if cfg.SessionTTL != 30*time.Minute || cfg.HTTPTimeout != 45*time.Second {
		t.Fatal("时长环境变量未生效")
	}
	message := (&config.MissingTargetConfigError{Names: []string{"TARGET_LOGIN_PASSWORD"}}).Error()
	for _, sensitive := range []string{password, aesKey, loginCode} {
		if strings.Contains(message, sensitive) {
			t.Fatal("配置错误消息泄露敏感值")
		}
	}
}

func TestTargetConfigLoadsInjectedLocalFile(t *testing.T) {
	localFile := filepath.Join(t.TempDir(), "target.env")
	password := generatedValue(t, 12)
	aesKey := generatedValue(t, 8)
	loginCode := generatedValue(t, 6)
	content := fmt.Sprintf(
		"TARGET_API_GATEWAY=https://target.example.invalid/local\nTARGET_LOGIN_PASSWORD=%s\nTARGET_LOGIN_AES_KEY=%s\nTARGET_LOGIN_CODE=%s\nTARGET_PLATFORM_CODE=local-platform\nTARGET_SESSION_TTL=45m\n",
		password, aesKey, loginCode,
	)
	if err := os.WriteFile(localFile, []byte(content), 0o600); err != nil {
		t.Fatal("无法写入测试期本地配置")
	}
	unsetTargetEnvironment(t)
	t.Setenv("TEST_AUTO_PRO_TARGET_ENV_FILE", localFile)

	cfg := config.LoadTargetConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("注入的本地配置未被完整读取：%v", missing)
	}
	if cfg.APIGateway != "https://target.example.invalid/local" || cfg.PlatformCode != "local-platform" {
		t.Fatal("本地配置字段映射不正确")
	}
	if cfg.LoginPassword != password || cfg.LoginAESKey != aesKey || cfg.LoginCode != loginCode {
		t.Fatal("本地登录配置未被读取")
	}
	if cfg.SessionTTL != 45*time.Minute {
		t.Fatal("本地时长配置未被读取")
	}
}

func TestTargetConfigEnvironmentOverridesLocalFile(t *testing.T) {
	localFile := filepath.Join(t.TempDir(), "target.env")
	localKey := generatedValue(t, 8)
	if err := os.WriteFile(localFile, []byte("TARGET_API_GATEWAY=https://target.example.invalid/local\nTARGET_LOGIN_AES_KEY="+localKey+"\n"), 0o600); err != nil {
		t.Fatal("无法写入测试期本地配置")
	}
	t.Setenv("TEST_AUTO_PRO_TARGET_ENV_FILE", localFile)
	t.Setenv("TARGET_API_GATEWAY", "https://target.example.invalid/environment")
	environmentKey := generatedValue(t, 8)
	t.Setenv("TARGET_LOGIN_AES_KEY", environmentKey)

	cfg := config.LoadTargetConfig()
	if cfg.APIGateway != "https://target.example.invalid/environment" || cfg.LoginAESKey != environmentKey {
		t.Fatal("进程环境没有覆盖本地配置")
	}
}

func TestTargetConfigRejectsMalformedLocalFileWithoutLeakingContent(t *testing.T) {
	localFile := filepath.Join(t.TempDir(), "target.env")
	sensitive := generatedValue(t, 16)
	if err := os.WriteFile(localFile, []byte("TARGET_LOGIN_PASSWORD=\""+sensitive), 0o600); err != nil {
		t.Fatal("无法写入测试期非法配置")
	}
	unsetTargetEnvironment(t)
	t.Setenv("TEST_AUTO_PRO_TARGET_ENV_FILE", localFile)

	cfg := config.LoadTargetConfig()
	missing := strings.Join(cfg.MissingRequired(), ",")
	if !strings.Contains(missing, "TARGET_LOCAL_CONFIG") {
		t.Fatal("非法本地配置未被稳定标记为缺失")
	}
	if strings.Contains(missing, sensitive) {
		t.Fatal("非法本地配置内容泄露到缺失项")
	}
}

func TestServerAddressCanBeOverridden(t *testing.T) {
	t.Setenv("TEST_AUTO_PRO_SERVER_ADDR", "127.0.0.1:29080")
	if got := config.ServerAddress(); got != "127.0.0.1:29080" {
		t.Fatalf("覆盖后的监听地址 = %q", got)
	}
}

func disableLocalTargetConfig(t *testing.T) {
	t.Helper()
	t.Setenv("TEST_AUTO_PRO_TARGET_ENV_FILE", "")
}

func unsetTargetEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"TARGET_API_GATEWAY", "TARGET_LOGIN_PASSWORD", "TARGET_LOGIN_AES_KEY", "TARGET_LOGIN_CODE",
		"TARGET_PLATFORM_CODE", "TARGET_TEMPLATE_PLATFORM_CODES", "TARGET_CUSTOMER_CODE",
		"TARGET_SESSION_TTL", "TARGET_HTTP_TIMEOUT",
	} {
		value, exists := os.LookupEnv(name)
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("无法隔离测试环境变量 %s", name)
		}
		restoreName, restoreValue, restoreExists := name, value, exists
		t.Cleanup(func() {
			if restoreExists {
				_ = os.Setenv(restoreName, restoreValue)
			} else {
				_ = os.Unsetenv(restoreName)
			}
		})
	}
}
