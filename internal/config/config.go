package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const defaultServerAddress = "127.0.0.1:19080"

const (
	defaultTargetLocalEnvFile = ".env.local"
	targetLocalEnvFileEnv     = "TEST_AUTO_PRO_TARGET_ENV_FILE"
)

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
	localConfigInvalid    bool
}

// LoadTargetConfig 优先读取进程环境，其次读取项目根目录的本机忽略配置。
// 本地文件只被解析到内存，不写入进程环境，避免影响测试和其他组件。
func LoadTargetConfig() TargetConfig {
	localValues, localErr := loadTargetLocalValues(targetLocalEnvFile())
	return TargetConfig{
		APIGateway:            strings.TrimSpace(targetConfigValue("TARGET_API_GATEWAY", localValues)),
		LoginPassword:         targetConfigValue("TARGET_LOGIN_PASSWORD", localValues),
		LoginAESKey:           targetConfigValue("TARGET_LOGIN_AES_KEY", localValues),
		LoginCode:             targetConfigValue("TARGET_LOGIN_CODE", localValues),
		PlatformCode:          firstNonEmpty(strings.TrimSpace(targetConfigValue("TARGET_PLATFORM_CODE", localValues)), defaultTargetPlatformCode),
		TemplatePlatformCodes: firstNonEmpty(strings.TrimSpace(targetConfigValue("TARGET_TEMPLATE_PLATFORM_CODES", localValues)), defaultTargetTemplatePlatformCodes),
		CustomerCode:          strings.TrimSpace(targetConfigValue("TARGET_CUSTOMER_CODE", localValues)),
		SessionTTL:            durationFromValue(targetConfigValue("TARGET_SESSION_TTL", localValues), defaultTargetSessionTTL),
		HTTPTimeout:           durationFromValue(targetConfigValue("TARGET_HTTP_TIMEOUT", localValues), defaultTargetHTTPTimeout),
		localConfigInvalid:    localErr != nil,
	}
}

// MissingRequired 返回缺失或格式不合法的配置名，不返回配置值。
func (c TargetConfig) MissingRequired() []string {
	missing := make([]string, 0, 5)
	if c.localConfigInvalid {
		missing = append(missing, "TARGET_LOCAL_CONFIG")
	}
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

func durationFromValue(value string, fallback time.Duration) time.Duration {
	value = strings.TrimSpace(value)
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

func targetLocalEnvFile() string {
	if path, ok := os.LookupEnv(targetLocalEnvFileEnv); ok {
		return strings.TrimSpace(path)
	}
	return defaultTargetLocalEnvFile
}

func loadTargetLocalValues(path string) (map[string]string, error) {
	if path == "" {
		return map[string]string{}, nil
	}
	content, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return map[string]string{}, err
	}
	values, err := godotenv.Unmarshal(string(content))
	if err != nil {
		return map[string]string{}, err
	}
	return values, nil
}

func targetConfigValue(name string, localValues map[string]string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return localValues[name]
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
