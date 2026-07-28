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

const (
	defaultOutput       = ".env.local"
	defaultDatabaseName = "test_auto_pro_v2"
)

var planDBNames = []string{
	"PLAN_DB_HOST",
	"PLAN_DB_PORT",
	"PLAN_DB_USER",
	"PLAN_DB_PASSWORD",
	"PLAN_DB_NAME",
}

type runnerDBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

func main() {
	source := flag.String("source", "", "V1 config.yaml 路径")
	output := flag.String("output", defaultOutput, "本机忽略配置输出路径")
	databaseName := flag.String("database", defaultDatabaseName, "独立计划数据库名")
	flag.Parse()

	if strings.TrimSpace(*source) == "" {
		fail("必须通过 -source 指定 V1 config.yaml")
	}
	if err := syncConfig(*source, *output, *databaseName); err != nil {
		fail(err.Error())
	}
	fmt.Println("本机计划数据库配置已安全更新并通过完整性检查（未显示配置值）")
}

func syncConfig(source, output, databaseName string) error {
	databaseName = strings.TrimSpace(databaseName)
	if !config.ValidDatabaseName(databaseName) {
		return fmt.Errorf("独立计划数据库名不合法")
	}
	runnerDB, err := readRunnerDBConfig(source)
	if err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(runnerDB.Name), databaseName) {
		return fmt.Errorf("独立计划数据库名不得复用 V1 runnerDb 原数据库")
	}
	existing, err := readExisting(output)
	if err != nil {
		return err
	}
	existing["PLAN_DB_HOST"] = strings.TrimSpace(runnerDB.Host)
	existing["PLAN_DB_PORT"] = strings.TrimSpace(runnerDB.Port)
	existing["PLAN_DB_USER"] = strings.TrimSpace(runnerDB.User)
	existing["PLAN_DB_PASSWORD"] = runnerDB.Password
	existing["PLAN_DB_NAME"] = databaseName
	for _, name := range planDBNames {
		if existing[name] == "" {
			return fmt.Errorf("无法生成本机配置：缺少 %s", name)
		}
	}
	serialized, err := godotenv.Marshal(existing)
	if err != nil {
		return fmt.Errorf("序列化本机配置失败")
	}
	if err := writePrivateFile(output, serialized+"\n"); err != nil {
		return err
	}
	return verifyWithLocalFile(output)
}

func readRunnerDBConfig(path string) (runnerDBConfig, error) {
	var value runnerDBConfig
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return value, fmt.Errorf("读取 V1 配置失败")
	}
	defer file.Close()

	sectionIndent := -1
	foundSection := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if !foundSection {
			if trimmed == "runnerDb:" {
				foundSection = true
				sectionIndent = indent
			}
			continue
		}
		if indent <= sectionIndent {
			break
		}
		key, rawValue, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		fieldValue := yamlScalar(rawValue)
		switch strings.TrimSpace(key) {
		case "host":
			value.Host = fieldValue
		case "port":
			value.Port = fieldValue
		case "user":
			value.User = fieldValue
		case "password":
			value.Password = fieldValue
		case "name":
			value.Name = fieldValue
		}
	}
	if err := scanner.Err(); err != nil {
		return value, fmt.Errorf("读取 V1 配置失败")
	}
	if !foundSection {
		return value, fmt.Errorf("解析 V1 配置失败：缺少 runnerDb 节")
	}
	return value, nil
}

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

func writePrivateFile(path, content string) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("创建本机配置目录失败")
	}
	temporary, err := os.CreateTemp(directory, ".plan-db-config-*")
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
	return os.Chmod(filepath.Clean(path), 0o600)
}

func verifyWithLocalFile(path string) error {
	for _, name := range planDBNames {
		_ = os.Unsetenv(name)
	}
	if err := os.Setenv("TEST_AUTO_PRO_PLAN_DB_ENV_FILE", path); err != nil {
		return fmt.Errorf("本机配置完整性检查失败")
	}
	loaded := config.LoadPlanDBConfig()
	if missing := loaded.MissingRequired(); len(missing) != 0 {
		return fmt.Errorf("本机配置完整性检查失败：%s", strings.Join(missing, "、"))
	}
	return nil
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
