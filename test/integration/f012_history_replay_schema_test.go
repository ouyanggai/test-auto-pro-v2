package integration_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"test-auto-pro-v2/internal/config"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
)

// TestF012HistoryReplaySchemaRebuild 验证 F-012 迁移只保留运行时维护表并重建五类业务表。
func TestF012HistoryReplaySchemaRebuild(t *testing.T) {
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Skipf("未配置测试数据库，跳过 F-012 MySQL 迁移验证：%v", missing)
	}
	cfg.Name = temporaryPlanDatabaseName(t)
	t.Cleanup(func() { dropTemporaryPlanDatabase(t, cfg) })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	database, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("F-012 临时数据库迁移失败：%v", err)
	}
	defer database.Close()

	tables := showTables(t, database.DB)
	for _, required := range []string{
		"schema_migrations",
		"test_plans",
		"test_execution_paths",
		"test_execution_path_choices",
		"test_execution_path_configs",
		"test_history_data_snapshots",
		"test_plan_history_data_defaults",
		"test_history_replay_jobs",
		"test_history_replay_items",
		"test_form_runtime_sync_jobs",
	} {
		if !tables[required] {
			t.Fatalf("F-012 迁移缺少表：%s", required)
		}
	}
	for _, removed := range []string{
		"test_path_preparation_jobs",
		"test_path_preparation_items",
		"test_template_rule_catalog",
		"test_template_rule_analysis_jobs",
		"test_execution_path_batches",
		"test_execution_path_batch_items",
	} {
		if tables[removed] {
			t.Fatalf("F-012 迁移仍保留旧业务表：%s", removed)
		}
	}
	assertF012Columns(t, database.DB, "test_history_data_snapshots", []string{"candidate_key", "render_type", "raw_form_data", "source_digest"})
	assertF012Columns(t, database.DB, "test_execution_path_configs", []string{"source_mode", "snapshot_id", "runtime_type", "person_strategies", "user_actions", "compiled_steps", "branch_patches", "runtime_validation"})
}

// showTables 读取临时工具库的表签名，供迁移清理边界断言使用。
func showTables(t *testing.T, db *sql.DB) map[string]bool {
	t.Helper()
	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		t.Fatalf("读取迁移后的表清单失败：%v", err)
	}
	defer rows.Close()
	result := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("读取迁移后的表名失败：%v", err)
		}
		result[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("遍历迁移后的表清单失败：%v", err)
	}
	return result
}

// assertF012Columns 检查新领域表包含不可缺失的原始数据与编排列。
func assertF012Columns(t *testing.T, db *sql.DB, table string, expected []string) {
	t.Helper()
	rows, err := db.Query("SHOW COLUMNS FROM " + table)
	if err != nil {
		t.Fatalf("读取 %s 列失败：%v", table, err)
	}
	defer rows.Close()
	found := map[string]bool{}
	for rows.Next() {
		var field, columnType, nullable, key, defaultValue, extra sql.NullString
		if err := rows.Scan(&field, &columnType, &nullable, &key, &defaultValue, &extra); err != nil {
			t.Fatalf("读取 %s 列详情失败：%v", table, err)
		}
		found[field.String] = true
	}
	for _, column := range expected {
		if !found[column] {
			t.Fatalf("%s 缺少列 %s", table, column)
		}
	}
}
