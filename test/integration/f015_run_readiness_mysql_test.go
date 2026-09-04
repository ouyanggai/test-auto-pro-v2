package integration_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
)

// f015Fixture 在临时库里准备一个计划和两条路径，返回断言仓储与两条路径 ID。
// 断言表以 path_id 为主键并级联到路径，因此必须先有真实的计划与路径行。
func f015Fixture(t *testing.T) (*planmysql.PathSuccessAssertionRepository, *sql.DB, uint64, uint64, uint64) {
	t.Helper()
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-015 MySQL 集成测试要求测试数据库配置，缺失：%v", missing)
	}
	cfg.Name = temporaryPlanDatabaseName(t)
	t.Cleanup(func() { dropTemporaryPlanDatabase(t, cfg) })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("F-015 临时数据库迁移失败：%v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	now := time.Now().UTC()
	result, err := database.DB.ExecContext(ctx, `
INSERT INTO test_plans (create_key, name, account, flow_source, target_object_id, target_object_name, run_mode, status, created_at, updated_at)
VALUES (UUID(), 'F-015 断言计划', 'tester', 'new', 'target-1', '员工请假单', 'serial', 'not_started', ?, ?)`, now, now)
	if err != nil {
		t.Fatalf("准备计划失败：%v", err)
	}
	planID64, _ := result.LastInsertId()
	planID := uint64(planID64)
	pathIDs := make([]uint64, 0, 2)
	for sequence := 1; sequence <= 2; sequence++ {
		pathResult, pathErr := database.DB.ExecContext(ctx, `
INSERT INTO test_execution_paths (plan_id, sequence_no, create_key, name, created_at, updated_at)
VALUES (?, ?, UUID(), ?, ?, ?)`, planID, sequence, "路径 "+string(rune('0'+sequence)), now, now)
		if pathErr != nil {
			t.Fatalf("准备执行路径失败：%v", pathErr)
		}
		pathID64, _ := pathResult.LastInsertId()
		pathIDs = append(pathIDs, uint64(pathID64))
	}
	return planmysql.NewPathSuccessAssertionRepository(database.DB), database.DB, planID, pathIDs[0], pathIDs[1]
}

// f015Assertion 构造一条可落库的断言。
func f015Assertion(pathID uint64, status string, ordinal uint) model.PathSuccessAssertion {
	return model.PathSuccessAssertion{
		PathID: pathID, EndNodeKey: "end-a", EndNodeName: "同意结束",
		ExpectedStatus: status, ArrivalOrdinal: ordinal,
	}
}

// TestF015AssertionMigrationAndPersistence 验证迁移建表、首次写入、读回与计划归属校验。
func TestF015AssertionMigrationAndPersistence(t *testing.T) {
	assertions, db, planID, pathID, otherPathID := f015Fixture(t)
	ctx := context.Background()
	if tables := showTables(t, db); !tables["test_path_success_assertions"] {
		t.Fatal("迁移没有建出 test_path_success_assertions 表")
	}
	// 还没有断言时必须是明确的"不存在"，不是空值。
	if _, err := assertions.Get(ctx, planID, pathID); !errors.Is(err, repository.ErrPathSuccessAssertionNotFound) {
		t.Fatalf("未配置断言时应返回不存在：%v", err)
	}
	saved, err := assertions.Save(ctx, planID, f015Assertion(pathID, model.FlowInstanceStatusEnd, 1), 0, "key-1", time.Now().UTC())
	if err != nil {
		t.Fatalf("首次保存断言失败：%v", err)
	}
	if saved.Revision != 1 || saved.ExpectedStatusLabel != "完结" {
		t.Fatalf("首次保存结果不正确：%+v", saved)
	}
	read, err := assertions.Get(ctx, planID, pathID)
	if err != nil || read.EndNodeKey != "end-a" || read.ArrivalOrdinal != 1 {
		t.Fatalf("读回断言不一致：%+v %v", read, err)
	}
	// 归属校验：拿另一个计划 ID 读同一条路径必须被拒绝。
	if _, err := assertions.Get(ctx, planID+1000, pathID); !errors.Is(err, repository.ErrPathSuccessAssertionPathNotFound) {
		t.Fatalf("跨计划读取应被拒绝：%v", err)
	}
	// 批量读取按计划取齐，另一条路径还没有断言就不应出现在结果里。
	all, err := assertions.ListByPlan(ctx, planID)
	if err != nil || len(all) != 1 || all[pathID].EndNodeKey != "end-a" {
		t.Fatalf("按计划批量读取不正确：%+v %v", all, err)
	}
	if _, exists := all[otherPathID]; exists {
		t.Fatal("没有断言的路径不应出现在批量结果里")
	}
}

// TestF015AssertionRevisionAndIdempotency 验证修订推进、幂等回放与并发修订冲突。
func TestF015AssertionRevisionAndIdempotency(t *testing.T) {
	assertions, _, planID, pathID, _ := f015Fixture(t)
	ctx := context.Background()
	first, err := assertions.Save(ctx, planID, f015Assertion(pathID, model.FlowInstanceStatusEnd, 1), 0, "key-1", time.Now().UTC())
	if err != nil {
		t.Fatalf("首次保存失败：%v", err)
	}
	// 同一幂等键重复到达：直接回放已有结果，不产生第二次修订。
	replay, err := assertions.Save(ctx, planID, f015Assertion(pathID, model.FlowInstanceStatusRejected, 1), 0, "key-1", time.Now().UTC())
	if err != nil {
		t.Fatalf("幂等回放失败：%v", err)
	}
	if replay.Revision != first.Revision || replay.ExpectedStatus != model.FlowInstanceStatusEnd {
		t.Fatalf("同一幂等键不应产生新修订也不应改写取值：%+v", replay)
	}
	// 带上正确修订的新键：修订推进一格。
	second, err := assertions.Save(ctx, planID, f015Assertion(pathID, model.FlowInstanceStatusRejected, 1), first.Revision, "key-2", time.Now().UTC())
	if err != nil || second.Revision != first.Revision+1 || second.ExpectedStatus != model.FlowInstanceStatusRejected {
		t.Fatalf("按修订保存不正确：%+v %v", second, err)
	}
	// 用过期修订保存必须明确冲突，不允许覆盖。
	if _, err := assertions.Save(ctx, planID, f015Assertion(pathID, model.FlowInstanceStatusEnd, 1), first.Revision, "key-3", time.Now().UTC()); !errors.Is(err, repository.ErrPathSuccessAssertionRevisionConflict) {
		t.Fatalf("过期修订应返回冲突：%v", err)
	}
	// 断言已存在却按"首次创建"保存，同样是冲突。
	if _, err := assertions.Save(ctx, planID, f015Assertion(pathID, model.FlowInstanceStatusEnd, 1), 0, "key-4", time.Now().UTC()); !errors.Is(err, repository.ErrPathSuccessAssertionRevisionConflict) {
		t.Fatalf("已存在断言按首次创建保存应返回冲突：%v", err)
	}
}

// TestF015AssertionCascadesWithPath 验证路径被删除时断言随外键级联清除，不留孤儿行。
func TestF015AssertionCascadesWithPath(t *testing.T) {
	assertions, db, planID, pathID, _ := f015Fixture(t)
	ctx := context.Background()
	if _, err := assertions.Save(ctx, planID, f015Assertion(pathID, model.FlowInstanceStatusEnd, 1), 0, "key-1", time.Now().UTC()); err != nil {
		t.Fatalf("保存断言失败：%v", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM test_execution_paths WHERE id = ?`, pathID); err != nil {
		t.Fatalf("删除执行路径失败：%v", err)
	}
	var remaining int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM test_path_success_assertions WHERE path_id = ?`, pathID).Scan(&remaining); err != nil {
		t.Fatalf("统计断言残留失败：%v", err)
	}
	if remaining != 0 {
		t.Fatalf("路径删除后断言应级联清除，实际残留 %d 行", remaining)
	}
}
