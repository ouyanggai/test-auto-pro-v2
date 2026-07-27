package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultServerAddress = "127.0.0.1:19080"

func ServerAddress() string {
	if address := os.Getenv("TEST_AUTO_PRO_SERVER_ADDR"); address != "" {
		return address
	}
	return defaultServerAddress
}

const (
	defaultTargetPlatformCode          = "200001"
	defaultTargetTemplatePlatformCodes = "200001,999999"
	defaultTargetSessionTTL            = 8 * time.Hour
	defaultTargetHTTPTimeout           = 120 * time.Second
)

// TargetConfig 只在 Go 后端保存目标平台连接与登录配置。
type TargetConfig struct {
	APIGateway            string
	LoginPassword         string
	LoginAESKey           string
	LoginCode             string
	PlatformCode          string
	TemplatePlatformCodes string
	CustomerCode          string
	SessionTTL            time.Duration
	HTTPTimeout           time.Duration
}

// LoadTargetConfig 从进程环境读取目标平台配置；敏感值没有代码默认值。
func LoadTargetConfig() TargetConfig {
	return TargetConfig{
		APIGateway:            strings.TrimSpace(os.Getenv("TARGET_API_GATEWAY")),
		LoginPassword:         os.Getenv("TARGET_LOGIN_PASSWORD"),
		LoginAESKey:           os.Getenv("TARGET_LOGIN_AES_KEY"),
		LoginCode:             os.Getenv("TARGET_LOGIN_CODE"),
		PlatformCode:          firstNonEmpty(strings.TrimSpace(os.Getenv("TARGET_PLATFORM_CODE")), defaultTargetPlatformCode),
		TemplatePlatformCodes: firstNonEmpty(strings.TrimSpace(os.Getenv("TARGET_TEMPLATE_PLATFORM_CODES")), defaultTargetTemplatePlatformCodes),
		CustomerCode:          strings.TrimSpace(os.Getenv("TARGET_CUSTOMER_CODE")),
		SessionTTL:            durationFromEnv("TARGET_SESSION_TTL", defaultTargetSessionTTL),
		HTTPTimeout:           durationFromEnv("TARGET_HTTP_TIMEOUT", defaultTargetHTTPTimeout),
	}
}

// MissingRequired 返回缺失或格式不合法的配置名，不返回配置值。
func (c TargetConfig) MissingRequired() []string {
	missing := make([]string, 0, 4)
	if strings.TrimSpace(c.APIGateway) == "" {
		missing = append(missing, "TARGET_API_GATEWAY")
	}
	if c.LoginPassword == "" {
		missing = append(missing, "TARGET_LOGIN_PASSWORD")
	}
	if c.LoginAESKey == "" {
		missing = append(missing, "TARGET_LOGIN_AES_KEY")
	} else if size := len([]byte(c.LoginAESKey)); size != 16 && size != 24 && size != 32 {
		missing = append(missing, "TARGET_LOGIN_AES_KEY")
	}
	if c.LoginCode == "" {
		missing = append(missing, "TARGET_LOGIN_CODE")
	}
	return missing
}

func durationFromEnv(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
		return parsed
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// MissingTargetConfigError 可安全写入错误响应，只包含配置项名称。
type MissingTargetConfigError struct {
	Names []string
}

func (e *MissingTargetConfigError) Error() string {
	return fmt.Sprintf("目标环境配置不完整：%s", strings.Join(e.Names, "、"))
}
