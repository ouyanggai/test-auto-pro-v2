package backend_test

import (
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

func TestServerAddressCanBeOverridden(t *testing.T) {
	t.Setenv("TEST_AUTO_PRO_SERVER_ADDR", "127.0.0.1:29080")
	if got := config.ServerAddress(); got != "127.0.0.1:29080" {
		t.Fatalf("覆盖后的监听地址 = %q", got)
	}
}
