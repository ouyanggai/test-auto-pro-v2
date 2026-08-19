package integration_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"testing"
	"time"

	driver "github.com/go-sql-driver/mysql"

	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/model"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
	"test-auto-pro-v2/internal/service"
)

var temporaryPlanDatabasePattern = regexp.MustCompile(`^test_auto_pro_v2_test_[a-f0-9]{12}$`)

// TestPlanMySQLConfiguredDatabaseIsIndependentAndMinimal 验证当前开发库仍使用独立计划库并包含 F-003 基础表。
func TestPlanMySQLConfiguredDatabaseIsIndependentAndMinimal(t *testing.T) {
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-003 本机计划数据库缺少配置名：%v", missing)
	}
	if cfg.Name != "test_auto_pro_v2" {
		t.Fatal("本机计划数据库没有使用已批准的独立库名")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	database, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("本机独立计划数据库迁移失败：%v", err)
	}
	defer database.Close()
	assertF003TablesPresent(t, database.DB)
}

// TestPlanMySQLMigrationCRUDIdempotencyAndRestartRead 验证计划迁移、增删改查幂等和重连读取。
func TestPlanMySQLMigrationCRUDIdempotencyAndRestartRead(t *testing.T) {
	baseConfig := config.LoadPlanDBConfig()
	if missing := baseConfig.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-003 MySQL 集成测试缺少配置名：%v", missing)
	}
	baseConfig.Name = temporaryPlanDatabaseName(t)
	if !temporaryPlanDatabasePattern.MatchString(baseConfig.Name) {
		t.Fatal("临时数据库名未通过严格前缀校验")
	}
	t.Cleanup(func() { dropTemporaryPlanDatabase(t, baseConfig) })

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	database, err := planmysql.OpenAndMigrate(ctx, baseConfig)
	if err != nil {
		t.Fatalf("临时计划数据库迁移失败：%v", err)
	}
	assertF003TablesPresent(t, database.DB)
	repository := planmysql.NewPlanRepository(database.DB)
	plans := service.NewPlanService(repository)

	concurrency := 4
	scheduledAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Millisecond)
	input := service.CreatePlanInput{
		Name: "MySQL 集成计划", Account: "integration-account", AccountDisplayName: "集成账号",
		FlowSource: "new", TargetObjectID: "integration-template", TargetObjectName: "集成流程",
		RunMode: "parallel", MaxConcurrency: &concurrency, ScheduledAt: &scheduledAt,
	}
	createKey := "123e4567-e89b-12d3-a456-426614174000"
	first, created, err := plans.Create(ctx, createKey, input)
	if err != nil || !created {
		t.Fatalf("首次写入失败：created=%v err=%v", created, err)
	}
	second, created, err := plans.Create(ctx, createKey, input)
	if err != nil || created || second.ID != first.ID {
		t.Fatal("MySQL 唯一键未保持创建幂等")
	}
	filtered, err := plans.List(ctx, "MySQL", model.PlanStatusNotStarted)
	if err != nil || len(filtered) != 1 || filtered[0].ID != first.ID {
		t.Fatal("MySQL 名称与状态筛选结果不正确")
	}
	var initialMigrationCount int
	if err := database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&initialMigrationCount); err != nil || initialMigrationCount < 1 {
		t.Fatalf("无法读取重连前迁移版本数量：count=%d err=%v", initialMigrationCount, err)
	}
	if err := database.Close(); err != nil {
		t.Fatal("关闭首次数据库连接池失败")
	}

	reopened, err := planmysql.OpenAndMigrate(ctx, baseConfig)
	if err != nil {
		t.Fatalf("重复迁移或重新连接失败：%v", err)
	}
	defer reopened.Close()
	stored, err := service.NewPlanService(planmysql.NewPlanRepository(reopened.DB)).Get(ctx, first.ID)
	if err != nil || stored.TargetObjectName != "集成流程" || stored.Status != model.PlanStatusNotStarted {
		t.Fatal("后端重启语义下未读取到同一条计划")
	}
	var migrationCount int
	if err := reopened.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&migrationCount); err != nil || migrationCount != initialMigrationCount {
		t.Fatalf("重连重复执行了已应用迁移：before=%d after=%d err=%v", initialMigrationCount, migrationCount, err)
	}
}

func temporaryPlanDatabaseName(t *testing.T) string {
	t.Helper()
	buffer := make([]byte, 6)
	if _, err := rand.Read(buffer); err != nil {
		t.Fatal("无法生成临时数据库随机后缀")
	}
	return "test_auto_pro_v2_test_" + hex.EncodeToString(buffer)
}

// assertF003TablesPresent 验证后续迁移没有破坏 F-003 的基础表，不禁止已批准切片增加新表。
func assertF003TablesPresent(t *testing.T, db *sql.DB) {
	t.Helper()
	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		t.Fatal("无法核对 F-003 表")
	}
	defer rows.Close()
	found := map[string]bool{}
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatal("无法读取 F-003 表名")
		}
		found[table] = true
	}
	if !found["schema_migrations"] || !found["test_plans"] {
		t.Fatalf("计划数据库缺少 F-003 基础表：%v", found)
	}
}

func dropTemporaryPlanDatabase(t *testing.T, cfg config.PlanDBConfig) {
	t.Helper()
	if !temporaryPlanDatabasePattern.MatchString(cfg.Name) || !config.ValidDatabaseName(cfg.Name) {
		t.Errorf("拒绝清理未通过严格校验的数据库")
		return
	}
	mysqlConfig := driver.NewConfig()
	mysqlConfig.User = cfg.User
	mysqlConfig.Passwd = cfg.Password
	mysqlConfig.Net = "tcp"
	mysqlConfig.Addr = net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	mysqlConfig.ParseTime = true
	mysqlConfig.Loc = time.UTC
	mysqlConfig.Timeout = 5 * time.Second
	db, err := sql.Open("mysql", mysqlConfig.FormatDSN())
	if err != nil {
		t.Errorf("无法准备临时数据库清理")
		return
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, fmt.Sprintf("DROP DATABASE `%s`", cfg.Name)); err != nil {
		t.Errorf("无法清理本次临时数据库")
	}
}
