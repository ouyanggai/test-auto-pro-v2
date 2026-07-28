package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"

	"test-auto-pro-v2/internal/config"
)

const defaultOutput = ".env.local"

var targetNames = []string{
	"TARGET_API_GATEWAY",
	"TARGET_LOGIN_PASSWORD",
	"TARGET_LOGIN_AES_KEY",
	"TARGET_LOGIN_CODE",
	"TARGET_PLATFORM_CODE",
	"TARGET_CUSTOMER_CODE",
}

type v1Config struct {
	Target struct {
		APIGateway   string `yaml:"apiGateway"`
		PlatformCode string `yaml:"platformCode"`
		CustomerCode string `yaml:"customerCode"`
		LoginAESKey  string `yaml:"loginAesKey"`
	} `yaml:"target"`
}

func main() {
	source := flag.String("source", "", "V1 config.yaml 路径")
	output := flag.String("output", defaultOutput, "本机忽略配置输出路径")
	flag.Parse()

	if strings.TrimSpace(*source) == "" {
		fail("必须通过 -source 指定 V1 config.yaml")
	}
	if err := syncConfig(*source, *output); err != nil {
		fail(err.Error())
	}
	fmt.Println("本机目标配置已安全更新并通过完整性检查（未显示配置值）")
}

func syncConfig(source, output string) error {
	v1, err := readV1Config(source)
	if err != nil {
		return err
	}
	existing, err := readExisting(output)
	if err != nil {
		return err
	}
	values := map[string]string{
		"TARGET_API_GATEWAY":    strings.TrimSpace(v1.Target.APIGateway),
		"TARGET_LOGIN_PASSWORD": firstRuntimeOrExisting("TARGET_LOGIN_PASSWORD", existing),
		"TARGET_LOGIN_AES_KEY":  v1.Target.LoginAESKey,
		"TARGET_LOGIN_CODE":     firstRuntimeOrExisting("TARGET_LOGIN_CODE", existing),
		"TARGET_PLATFORM_CODE":  strings.TrimSpace(v1.Target.PlatformCode),
		"TARGET_CUSTOMER_CODE":  strings.TrimSpace(v1.Target.CustomerCode),
	}
	for _, name := range targetNames {
		if values[name] == "" {
			return fmt.Errorf("无法生成本机配置：缺少 %s", name)
		}
	}
	serialized, err := godotenv.Marshal(values)
	if err != nil {
		return fmt.Errorf("序列化本机配置失败")
	}
	if err := writePrivateFile(output, serialized+"\n"); err != nil {
		return err
	}
	return verifyWithLocalFile(output)
}

func readV1Config(path string) (v1Config, error) {
	var value v1Config
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return value, fmt.Errorf("读取 V1 配置失败")
	}
	defer file.Close()

	targetIndent := -1
	foundTarget := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if !foundTarget {
			if trimmed == "target:" {
				foundTarget = true
				targetIndent = indent
			}
			continue
		}
		if indent <= targetIndent {
			break
		}
		key, rawValue, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		fieldValue := yamlScalar(rawValue)
		switch strings.TrimSpace(key) {
		case "apiGateway":
			value.Target.APIGateway = fieldValue
		case "platformCode":
			value.Target.PlatformCode = fieldValue
		case "customerCode":
			value.Target.CustomerCode = fieldValue
		case "loginAesKey":
			value.Target.LoginAESKey = fieldValue
		}
	}
	if err := scanner.Err(); err != nil {
		return value, fmt.Errorf("读取 V1 配置失败")
	}
	if !foundTarget {
		return value, fmt.Errorf("解析 V1 配置失败：缺少 target 节")
	}
	return value, nil
}

// yamlScalar 只读取 V1 当前 target 节中的标量配置，避免维护命令依赖额外解析器。
func yamlScalar(raw string) string {
	value := strings.TrimSpace(raw)
	for index := 1; index < len(value); index++ {
		if value[index] == '#' && (value[index-1] == ' ' || value[index-1] == '\t') {
			value = strings.TrimSpace(value[:index])
			break
		}
	}
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		if unquoted, err := strconv.Unquote(value); err == nil {
			return unquoted
		}
	}
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		return strings.ReplaceAll(value[1:len(value)-1], "''", "'")
	}
	return value
}

func readExisting(path string) (map[string]string, error) {
	content, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("读取现有本机配置失败")
	}
	values, err := godotenv.Unmarshal(string(content))
	if err != nil {
		return nil, fmt.Errorf("现有本机配置格式不正确")
	}
	return values, nil
}

func firstRuntimeOrExisting(name string, existing map[string]string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return existing[name]
}

func writePrivateFile(path, content string) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("创建本机配置目录失败")
	}
	temporary, err := os.CreateTemp(directory, ".target-config-*")
	if err != nil {
		return fmt.Errorf("创建本机配置临时文件失败")
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("设置本机配置权限失败")
	}
	if _, err := temporary.WriteString(content); err != nil {
		temporary.Close()
		return fmt.Errorf("写入本机配置失败")
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("同步本机配置失败")
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("关闭本机配置失败")
	}
	if err := os.Rename(temporaryName, filepath.Clean(path)); err != nil {
		return fmt.Errorf("替换本机配置失败")
	}
	return nil
}

func verifyWithLocalFile(path string) error {
	for _, name := range targetNames {
		os.Unsetenv(name)
	}
	os.Setenv("TEST_AUTO_PRO_TARGET_ENV_FILE", path)
	loaded := config.LoadTargetConfig()
	if missing := loaded.MissingRequired(); len(missing) != 0 {
		return fmt.Errorf("本机配置完整性检查失败：%s", strings.Join(missing, "、"))
	}
	return nil
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
