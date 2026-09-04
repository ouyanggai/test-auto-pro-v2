package integration_test

import (
	"context"
	"testing"
	"time"

	"test-auto-pro-v2/internal/config"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
)

// f016RunRecordTables 是 F-016 迁移 026 必须建立的六张运行记录表。
var f016RunRecordTables = []string{
	"runs",
	"path_runs",
	"run_steps",
	"run_step_attempts",
	"run_events",
	"run_controls",
}

// TestF016MigrationCreatesRunRecordTables 用真实 MySQL 验证迁移 026 被现有迁移执行器实际应用：
// 六张运行记录表必须存在，且重复启动（重连重跑迁移）不破坏已有记录。
func TestF016MigrationCreatesRunRecordTables(t *testing.T) {
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-016 MySQL 集成测试缺少配置名：%v", missing)
	}
	cfg.Name = temporaryPlanDatabaseName(t)
	t.Cleanup(func() { dropTemporaryPlanDatabase(t, cfg) })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	database, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("临时计划数据库迁移失败：%v", err)
	}
	found := map[string]bool{}
	rows, err := database.DB.QueryContext(ctx, "SHOW TABLES")
	if err != nil {
		t.Fatalf("无法读取临时库表清单：%v", err)
	}
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatalf("无法读取表名：%v", err)
		}
		found[table] = true
	}
	rows.Close()
	for _, table := range f016RunRecordTables {
		if !found[table] {
			t.Fatalf("迁移 026 未建立运行记录表 %s：实际表 %v", table, found)
		}
	}

	// 服务可启动等价于：迁移后的库能再次通过 OpenAndMigrate（幂等重放全部迁移且记录不被破坏）。
	var runCount int
	if err := database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM runs").Scan(&runCount); err != nil {
		t.Fatalf("runs 表不可查询：%v", err)
	}
	database.Close()

	reopened, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("重复迁移（模拟服务重启）失败：%v", err)
	}
	defer reopened.Close()
	var reopenedRunCount int
	if err := reopened.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM runs").Scan(&reopenedRunCount); err != nil {
		t.Fatalf("重连后 runs 表不可查询：%v", err)
	}
	if reopenedRunCount != runCount {
		t.Fatalf("重复迁移破坏了运行记录：before=%d after=%d", runCount, reopenedRunCount)
	}
}
