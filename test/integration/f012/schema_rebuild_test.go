package f012_test

import (
	"testing"

	planmysql "test-auto-pro-v2/internal/repository/mysql"
)

// TestHistoryMigrationRebuildsToolDatabaseWithoutLegacyDomain 验证真实 MySQL 上的一次性清理与重建边界：
// 五张新表建立、schema_migrations 保留、运行时维护表保留但清空、旧领域表消失、路径拓扑表保留。
func TestHistoryMigrationRebuildsToolDatabaseWithoutLegacyDomain(t *testing.T) {
	database, _, _ := openHistoryIntegrationDatabase(t, "F-012 数据库重建验证")
	db := database.DB

	for _, table := range []string{
		"test_history_data_snapshots", "test_plan_history_data_defaults",
		"test_history_replay_jobs", "test_history_replay_items", "test_execution_path_configs",
	} {
		if !historyTableExists(t, db, table) {
			t.Fatalf("F-012 新表缺失：%s", table)
		}
	}

	for _, table := range []string{
		"test_plans", "test_execution_paths", "test_execution_path_choices",
		"test_execution_path_batches", "test_execution_path_batch_items",
	} {
		if !historyTableExists(t, db, table) {
			t.Fatalf("路径拓扑表被误删：%s", table)
		}
	}

	if !historyTableExists(t, db, "test_form_runtime_sync_jobs") {
		t.Fatal("运行时维护表被误删：test_form_runtime_sync_jobs")
	}
	if count := historyRowCount(t, db, "test_form_runtime_sync_jobs"); count != 0 {
		t.Fatalf("运行时维护任务数据未清空：%d", count)
	}

	for _, table := range []string{
		"test_path_preparation_items", "test_path_preparation_jobs",
		"test_template_rule_analysis_jobs", "test_template_rule_catalog",
	} {
		if historyTableExists(t, db, table) {
			t.Fatalf("旧领域表仍然存在：%s", table)
		}
	}

	if !historyTableExists(t, db, "schema_migrations") {
		t.Fatal("迁移历史表未保留：schema_migrations")
	}
	var migrated int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM schema_migrations WHERE version = ?", "022_rebuild_f012_history_replay.sql",
	).Scan(&migrated); err != nil {
		t.Fatalf("读取迁移记录失败：%v", err)
	}
	if migrated != 1 {
		t.Fatalf("F-012 迁移未按文件名登记：%d", migrated)
	}

	columns := historyTableColumns(t, db, "test_execution_path_configs")
	for _, column := range []string{
		"revision", "node_revision", "data_revision", "action_revision",
		"person_strategies", "user_actions", "compiled_steps", "confirmed_node_keys",
		"effective_form_data", "branch_patches", "runtime_validation", "issues", "latest_idempotency_result",
		"source_mode", "snapshot_id", "runtime_type", "config_status", "node_status", "data_status",
	} {
		if !columns[column] {
			t.Fatalf("路径配置表缺少 F-012 领域列：%s", column)
		}
	}
	for _, column := range []string{"generator", "constraints", "cycles", "rules", "preparation"} {
		if columns[column] {
			t.Fatalf("路径配置表仍保留旧生成链路列：%s", column)
		}
	}

	for _, key := range []string{
		"uk_history_snapshot_plan_candidate", "uk_history_replay_job_idempotency", "uk_history_replay_item_job_path",
	} {
		var found int
		if err := db.QueryRow(`
SELECT COUNT(*) FROM information_schema.statistics
WHERE table_schema = DATABASE() AND index_name = ? AND non_unique = 0`, key).Scan(&found); err != nil {
			t.Fatalf("读取唯一约束失败：%v", err)
		}
		if found == 0 {
			t.Fatalf("F-012 唯一约束缺失：%s", key)
		}
	}
}

// TestHistoryMigrationIsIdempotentAndKeepsExistingRows 验证再次启动不会重放 022 清理语句，已有业务数据不被二次删除。
func TestHistoryMigrationIsIdempotentAndKeepsExistingRows(t *testing.T) {
	database, cfg, ctx := openHistoryIntegrationDatabase(t, "F-012 迁移幂等验证")
	planID, pathIDs := insertHistoryPlanWithPaths(t, database.DB, 20501, "template-a", "new", 1)
	if planID == 0 || len(pathIDs) != 1 {
		t.Fatalf("集成计划准备失败：plan=%d paths=%v", planID, pathIDs)
	}

	reopened, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("重复执行迁移失败：%v", err)
	}
	defer reopened.Close()

	if count := historyRowCount(t, reopened.DB, "test_plans"); count != 1 {
		t.Fatalf("重复迁移清空了已有计划数据：%d", count)
	}
	if count := historyRowCount(t, reopened.DB, "test_execution_paths"); count != 1 {
		t.Fatalf("重复迁移清空了已有路径数据：%d", count)
	}
	var applied int
	if err := reopened.DB.QueryRow(
		"SELECT COUNT(*) FROM schema_migrations WHERE version = ?", "022_rebuild_f012_history_replay.sql",
	).Scan(&applied); err != nil {
		t.Fatalf("读取迁移记录失败：%v", err)
	}
	if applied != 1 {
		t.Fatalf("F-012 迁移被重复登记：%d", applied)
	}
}
