package f012_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/model"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
)

// openHistoryIntegrationDatabase 在一次性临时工具库上执行真实迁移，边界与来源事务用例保持一致：
// 只允许 test_auto_pro_v2_f012_* 库名，绝不连接真实工具库或目标平台数据库。
func openHistoryIntegrationDatabase(t *testing.T, reason string) (*planmysql.Database, config.PlanDBConfig, context.Context) {
	t.Helper()
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Skipf("未配置测试数据库，跳过 %s：%v", reason, missing)
	}
	cfg.Name = newHistoryTemporaryDatabaseName(t)
	t.Cleanup(func() { dropHistoryTemporaryDatabase(t, cfg) })
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	database, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("创建 F-012 临时数据库失败：%v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database, cfg, ctx
}

// historyIntegrationKey 生成稳定且互不冲突的创建键与幂等键，避免多个用例共享同一 UUID 尾号。
func historyIntegrationKey(sequence int) string {
	return fmt.Sprintf("123e4567-e89b-12d3-a456-4266141%05d", sequence)
}

// insertHistoryPlanWithPaths 插入一个计划及指定数量的执行路径父记录，供多路径任务与换路用例复用。
func insertHistoryPlanWithPaths(t *testing.T, db *sql.DB, keySeed int, targetObjectID, flowSource string, pathCount int) (uint64, []uint64) {
	t.Helper()
	now := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	planResult, err := db.Exec(`
INSERT INTO test_plans (
  create_key, name, account, account_display_name, flow_source, target_object_id,
  target_object_name, run_mode, max_concurrency, scheduled_at, status, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		historyIntegrationKey(keySeed), fmt.Sprintf("F-012 集成计划 %d", keySeed), "account-a", "当前用户",
		flowSource, targetObjectID, "费用审批", "serial", nil, nil, model.PlanStatusNotStarted, now, now,
	)
	if err != nil {
		t.Fatalf("插入 F-012 集成计划失败：%v", err)
	}
	planIDValue, _ := planResult.LastInsertId()
	pathIDs := make([]uint64, 0, pathCount)
	for index := 1; index <= pathCount; index++ {
		pathResult, pathErr := db.Exec(`
INSERT INTO test_execution_paths (plan_id, sequence_no, create_key, name, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)`,
			uint64(planIDValue), index, historyIntegrationKey(keySeed+index), fmt.Sprintf("路径 %d", index), now, now)
		if pathErr != nil {
			t.Fatalf("插入 F-012 集成路径失败：%v", pathErr)
		}
		pathIDValue, _ := pathResult.LastInsertId()
		pathIDs = append(pathIDs, uint64(pathIDValue))
	}
	return uint64(planIDValue), pathIDs
}

// historyTableExists 按当前临时库判断表是否存在，不使用可能命中其他库的模糊查询。
func historyTableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var count int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", table,
	).Scan(&count); err != nil {
		t.Fatalf("读取表结构失败：%v", err)
	}
	return count != 0
}

// historyTableColumns 返回当前临时库指定表的列名集合。
func historyTableColumns(t *testing.T, db *sql.DB, table string) map[string]bool {
	t.Helper()
	rows, err := db.Query(
		"SELECT column_name FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ?", table)
	if err != nil {
		t.Fatalf("读取列结构失败：%v", err)
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("读取列名失败：%v", err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("遍历列结构失败：%v", err)
	}
	return columns
}

// historyRowCount 读取当前临时库内指定表的真实行数。
func historyRowCount(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM `" + table + "`").Scan(&count); err != nil {
		t.Fatalf("统计 %s 行数失败：%v", table, err)
	}
	return count
}
