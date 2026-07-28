package backend_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/config"
)

func TestPlanDBConfigLoadsLocalFileAndEnvironmentOverrides(t *testing.T) {
	localFile := filepath.Join(t.TempDir(), "plan-db.env")
	password := generatedValue(t, 12)
	content := fmt.Sprintf(
		"PLAN_DB_HOST=db.local\nPLAN_DB_PORT=3307\nPLAN_DB_USER=local-user\nPLAN_DB_PASSWORD=%s\nPLAN_DB_NAME=test_auto_pro_v2\n",
		password,
	)
	if err := os.WriteFile(localFile, []byte(content), 0o600); err != nil {
		t.Fatal("无法写入测试期计划数据库配置")
	}
	unsetPlanDBEnvironment(t)
	t.Setenv("TEST_AUTO_PRO_PLAN_DB_ENV_FILE", localFile)
	t.Setenv("PLAN_DB_HOST", "db.override")

	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("完整计划数据库配置被判断为缺失：%v", missing)
	}
	if cfg.Host != "db.override" || cfg.Port != 3307 || cfg.User != "local-user" || cfg.Password != password || cfg.Name != "test_auto_pro_v2" {
		t.Fatal("计划数据库配置优先级或字段映射不正确")
	}
}

func TestPlanDBConfigDefaultsAndSafeErrors(t *testing.T) {
	unsetPlanDBEnvironment(t)
	t.Setenv("TEST_AUTO_PRO_PLAN_DB_ENV_FILE", "")
	cfg := config.LoadPlanDBConfig()
	if cfg.Port != 3306 || cfg.Name != "test_auto_pro_v2" {
		t.Fatal("计划数据库非敏感默认值不正确")
	}
	missing := strings.Join(cfg.MissingRequired(), ",")
	for _, name := range []string{"PLAN_DB_HOST", "PLAN_DB_USER", "PLAN_DB_PASSWORD"} {
		if !strings.Contains(missing, name) {
			t.Fatalf("缺失项未包含 %s", name)
		}
	}
	secret := generatedValue(t, 16)
	message := (&config.MissingPlanDBConfigError{Names: []string{"PLAN_DB_PASSWORD"}}).Error()
	if strings.Contains(message, secret) {
		t.Fatal("计划数据库配置错误泄露敏感值")
	}
}

func TestPlanDBConfigRejectsInvalidPortDatabaseAndMalformedLocalFile(t *testing.T) {
	tests := []struct {
		name     string
		port     string
		database string
		missing  string
	}{
		{name: "非法端口", port: "70000", database: "test_auto_pro_v2", missing: "PLAN_DB_PORT"},
		{name: "注入式库名", port: "3306", database: "test_auto_pro_v2`;DROP", missing: "PLAN_DB_NAME"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			unsetPlanDBEnvironment(t)
			t.Setenv("TEST_AUTO_PRO_PLAN_DB_ENV_FILE", "")
			t.Setenv("PLAN_DB_HOST", "db.local")
			t.Setenv("PLAN_DB_PORT", test.port)
			t.Setenv("PLAN_DB_USER", "user")
			t.Setenv("PLAN_DB_PASSWORD", generatedValue(t, 12))
			t.Setenv("PLAN_DB_NAME", test.database)
			if missing := strings.Join(config.LoadPlanDBConfig().MissingRequired(), ","); !strings.Contains(missing, test.missing) {
				t.Fatalf("非法配置未标记 %s", test.missing)
			}
		})
	}

	localFile := filepath.Join(t.TempDir(), "broken.env")
	if err := os.WriteFile(localFile, []byte("PLAN_DB_PASSWORD=\"broken"), 0o600); err != nil {
		t.Fatal("无法写入测试期非法配置")
	}
	unsetPlanDBEnvironment(t)
	t.Setenv("TEST_AUTO_PRO_PLAN_DB_ENV_FILE", localFile)
	if missing := strings.Join(config.LoadPlanDBConfig().MissingRequired(), ","); !strings.Contains(missing, "PLAN_DB_LOCAL_CONFIG") {
		t.Fatal("非法本机配置未被稳定标记")
	}
}

func TestValidDatabaseName(t *testing.T) {
	for _, valid := range []string{"test_auto_pro_v2", "test_auto_pro_v2_ab12", "DB1"} {
		if !config.ValidDatabaseName(valid) {
			t.Fatalf("合法数据库名被拒绝：%s", valid)
		}
	}
	for _, invalid := range []string{"", "test-auto", "test auto", "db`x", strings.Repeat("a", 65)} {
		if config.ValidDatabaseName(invalid) {
			t.Fatalf("非法数据库名被接受：%q", invalid)
		}
	}
}

func unsetPlanDBEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{"PLAN_DB_HOST", "PLAN_DB_PORT", "PLAN_DB_USER", "PLAN_DB_PASSWORD", "PLAN_DB_NAME"} {
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
