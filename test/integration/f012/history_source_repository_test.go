package f012_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	driver "github.com/go-sql-driver/mysql"

	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
)

var historyTemporaryDatabasePattern = regexp.MustCompile(`^test_auto_pro_v2_f012_[0-9a-f]{12}$`)

// TestHistorySourceRepositoryPersistsRawPayloadsAndRollsBackConflicts 验证两类正文、动态继承、修订与无孤儿快照事务。
func TestHistorySourceRepositoryPersistsRawPayloadsAndRollsBackConflicts(t *testing.T) {
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Skipf("未配置测试数据库，跳过 F-012 来源事务验证：%v", missing)
	}
	cfg.Name = newHistoryTemporaryDatabaseName(t)
	t.Cleanup(func() { dropHistoryTemporaryDatabase(t, cfg) })
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	database, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("创建 F-012 临时数据库失败：%v", err)
	}
	defer database.Close()
	planID, pathID := insertHistoryPlanAndPath(t, database.DB)
	store := planmysql.NewHistoryReplayRepository(database.DB)
	now := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	formRaw := map[string]any{
		"amount": float64(128), "nested": map[string]any{"rows": []any{map[string]any{"virtual": true, "systemHelper": "kept"}}},
	}
	formSnapshot := repositoryHistorySnapshot(planID, strings.Repeat("a", 64), "formmaking", "form-digest", formRaw)
	persistedForm, defaultRecord, err := store.SaveDefaultWithSnapshot(ctx, formSnapshot, repository.HistoryDefaultRecord{
		PlanID: planID, IdempotencyKey: "123e4567-e89b-12d3-a456-426614174401",
	}, 0, now)
	if err != nil || persistedForm.ID == 0 || defaultRecord.Revision != 1 {
		t.Fatalf("默认来源与 FormMaking 快照事务失败：snapshot=%+v record=%+v err=%v", persistedForm, defaultRecord, err)
	}
	assertHistorySnapshotRaw(t, store, ctx, planID, persistedForm.ID, formRaw)

	customRaw := map[string]any{
		"pageState": map[string]any{"lines": []any{map[string]any{"amount": float64(88), "customComponentValue": map[string]any{"code": "X"}}}},
	}
	customSnapshot := repositoryHistorySnapshot(planID, strings.Repeat("b", 64), "vue_custom", "custom-digest", customRaw)
	if _, _, err := store.SaveDefaultWithSnapshot(ctx, customSnapshot, repository.HistoryDefaultRecord{
		PlanID: planID, IdempotencyKey: "123e4567-e89b-12d3-a456-426614174402",
	}, 0, now.Add(time.Minute)); !errors.Is(err, repository.ErrHistoryRevisionConflict) {
		t.Fatalf("错误修订号没有阻止默认来源更新：%v", err)
	}
	assertHistorySnapshotCount(t, database.DB, planID, 1)
	if _, _, err := store.SaveDefaultWithSnapshot(ctx, customSnapshot, repository.HistoryDefaultRecord{
		PlanID: planID, IdempotencyKey: defaultRecord.IdempotencyKey,
	}, 1, now.Add(time.Minute)); !errors.Is(err, repository.ErrHistoryRevisionConflict) {
		t.Fatalf("相同幂等键绑定不同候选未被拒绝：%v", err)
	}
	assertHistorySnapshotCount(t, database.DB, planID, 1)

	pathRecord, err := store.SavePathSource(ctx, planID, repository.HistoryPathSourceRecord{
		PathID: pathID, Mode: model.HistorySourceModeDefault, IdempotencyKey: "123e4567-e89b-12d3-a456-426614174403",
	}, 0, now.Add(2*time.Minute))
	if err != nil || pathRecord.Revision != 1 || pathRecord.SnapshotID != 0 {
		t.Fatalf("路径动态继承错误冻结快照：record=%+v err=%v", pathRecord, err)
	}
	if _, _, err := store.SavePathSourceWithSnapshot(ctx, planID, customSnapshot, repository.HistoryPathSourceRecord{
		PathID: pathID, Mode: model.HistorySourceModeOverride, IdempotencyKey: "123e4567-e89b-12d3-a456-426614174404",
	}, 0, now.Add(3*time.Minute)); !errors.Is(err, repository.ErrHistoryRevisionConflict) {
		t.Fatalf("路径错误修订号没有整体回滚独立快照：%v", err)
	}
	assertHistorySnapshotCount(t, database.DB, planID, 1)
	persistedCustom, overrideRecord, err := store.SavePathSourceWithSnapshot(ctx, planID, customSnapshot, repository.HistoryPathSourceRecord{
		PathID: pathID, Mode: model.HistorySourceModeOverride, IdempotencyKey: "123e4567-e89b-12d3-a456-426614174405",
	}, 1, now.Add(4*time.Minute))
	if err != nil || overrideRecord.Revision != 2 || overrideRecord.SnapshotID != persistedCustom.ID {
		t.Fatalf("路径独立自定义表单快照事务失败：snapshot=%+v record=%+v err=%v", persistedCustom, overrideRecord, err)
	}
	assertHistorySnapshotRaw(t, store, ctx, planID, persistedCustom.ID, customRaw)
	assertHistorySnapshotCount(t, database.DB, planID, 2)

	pathRecord, err = store.SavePathSource(ctx, planID, repository.HistoryPathSourceRecord{
		PathID: pathID, Mode: model.HistorySourceModeDefault, IdempotencyKey: "123e4567-e89b-12d3-a456-426614174406",
	}, 2, now.Add(5*time.Minute))
	if err != nil || pathRecord.Revision != 3 || pathRecord.SnapshotID != 0 {
		t.Fatalf("路径恢复动态继承失败：record=%+v err=%v", pathRecord, err)
	}
	_, defaultRecord, err = store.SaveDefaultWithSnapshot(ctx, customSnapshot, repository.HistoryDefaultRecord{
		PlanID: planID, IdempotencyKey: "123e4567-e89b-12d3-a456-426614174407",
	}, 1, now.Add(6*time.Minute))
	if err != nil || defaultRecord.Revision != 2 || defaultRecord.SnapshotID != persistedCustom.ID {
		t.Fatalf("替换计划默认来源失败：record=%+v err=%v", defaultRecord, err)
	}
	resolvedPath, found, err := store.GetPathSource(ctx, pathID)
	if err != nil || !found || resolvedPath.Mode != model.HistorySourceModeDefault || resolvedPath.SnapshotID != 0 || resolvedPath.Revision != 4 {
		t.Fatalf("默认来源替换后路径未保持动态继承或修订未受影响：record=%+v found=%v err=%v", resolvedPath, found, err)
	}
	var dataStatus string
	if err := database.DB.QueryRowContext(ctx, "SELECT data_status FROM test_execution_path_configs WHERE path_id = ?", pathID).Scan(&dataStatus); err != nil || dataStatus != model.HistoryDataStatusAffected {
		t.Fatalf("默认来源替换没有原子标记继承路径 affected：status=%q err=%v", dataStatus, err)
	}
	var replayJobs int
	if err := database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_history_replay_jobs").Scan(&replayJobs); err != nil || replayJobs != 0 {
		t.Fatalf("T02 错误创建了历史回放运行记录：count=%d err=%v", replayJobs, err)
	}
}

// TestHistoryDefaultFirstWriteIsSerialized 验证同计划两个首次写入只有一个提交且只留下一个快照。
func TestHistoryDefaultFirstWriteIsSerialized(t *testing.T) {
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Skipf("未配置测试数据库，跳过 F-012 并发来源验证：%v", missing)
	}
	cfg.Name = newHistoryTemporaryDatabaseName(t)
	t.Cleanup(func() { dropHistoryTemporaryDatabase(t, cfg) })
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	database, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("创建 F-012 并发临时数据库失败：%v", err)
	}
	defer database.Close()
	planID, _ := insertHistoryPlanAndPath(t, database.DB)
	store := planmysql.NewHistoryReplayRepository(database.DB)
	results := make(chan error, 2)
	for index, candidate := range []string{strings.Repeat("c", 64), strings.Repeat("d", 64)} {
		index, candidate := index, candidate
		go func() {
			snapshot := repositoryHistorySnapshot(planID, candidate, "formmaking", "digest-"+candidate[:1], map[string]any{"index": float64(index)})
			_, _, saveErr := store.SaveDefaultWithSnapshot(ctx, snapshot, repository.HistoryDefaultRecord{
				PlanID: planID, IdempotencyKey: []string{"123e4567-e89b-12d3-a456-426614174408", "123e4567-e89b-12d3-a456-426614174409"}[index],
			}, 0, time.Now().UTC())
			results <- saveErr
		}()
	}
	successes, conflicts := 0, 0
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, repository.ErrHistoryRevisionConflict):
			conflicts++
		default:
			t.Fatalf("首次来源并发返回非预期错误：%v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("首次来源并发没有串行化：success=%d conflict=%d", successes, conflicts)
	}
	assertHistorySnapshotCount(t, database.DB, planID, 1)
}

// repositoryHistorySnapshot 构造仓储事务使用的不可变原始数据快照。
func repositoryHistorySnapshot(planID uint64, candidateKey, runtimeType, digest string, raw map[string]any) model.HistorySnapshot {
	return model.HistorySnapshot{
		PlanID: planID, SourceAccount: "account-a", CandidateKey: candidateKey, FlowCode: "expense-flow",
		FormName: "费用单（测试公司）", FlowName: "费用审批", RuntimeType: runtimeType, InstanceStatus: "end",
		InstanceSummary: map[string]any{"instanceTitle": "真实历史实例"},
		TemplateSummary: map[string]any{"runtimeVersionDigest": digest}, RawFormData: raw,
		SourceDigest: digest, CreatedAt: time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC),
	}
}

// assertHistorySnapshotRaw 使用 JSON 语义比较持久化前后的深层原始正文。
func assertHistorySnapshotRaw(t *testing.T, store *planmysql.HistoryReplayRepository, ctx context.Context, planID, snapshotID uint64, expected map[string]any) {
	t.Helper()
	stored, err := store.GetSnapshot(ctx, planID, snapshotID)
	if err != nil {
		t.Fatalf("读取持久化历史快照失败：%v", err)
	}
	actualJSON, _ := json.Marshal(stored.RawFormData)
	expectedJSON, _ := json.Marshal(expected)
	if string(actualJSON) != string(expectedJSON) {
		t.Fatalf("历史原始数据持久化结构变化：got=%s want=%s", actualJSON, expectedJSON)
	}
}

// assertHistorySnapshotCount 断言计划内快照数量，证明冲突事务没有留下孤儿记录。
func assertHistorySnapshotCount(t *testing.T, db *sql.DB, planID uint64, expected int) {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM test_history_data_snapshots WHERE plan_id = ?", planID).Scan(&count); err != nil || count != expected {
		t.Fatalf("历史快照数量不正确：got=%d want=%d err=%v", count, expected, err)
	}
}

// insertHistoryPlanAndPath 插入当前临时库唯一的计划与路径父记录。
func insertHistoryPlanAndPath(t *testing.T, db *sql.DB) (uint64, uint64) {
	t.Helper()
	now := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	planResult, err := db.Exec(`
INSERT INTO test_plans (
  create_key, name, account, account_display_name, flow_source, target_object_id,
  target_object_name, run_mode, max_concurrency, scheduled_at, status, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"123e4567-e89b-12d3-a456-426614174410", "F-012 集成计划", "account-a", "当前用户", "new", "template-a",
		"费用审批", "serial", nil, nil, model.PlanStatusNotStarted, now, now,
	)
	if err != nil {
		t.Fatalf("插入 F-012 集成计划失败：%v", err)
	}
	planIDValue, _ := planResult.LastInsertId()
	pathResult, err := db.Exec(`
INSERT INTO test_execution_paths (plan_id, sequence_no, create_key, name, created_at, updated_at)
VALUES (?, 1, ?, ?, ?, ?)`, uint64(planIDValue), "123e4567-e89b-12d3-a456-426614174411", "审批路径", now, now)
	if err != nil {
		t.Fatalf("插入 F-012 集成路径失败：%v", err)
	}
	pathIDValue, _ := pathResult.LastInsertId()
	return uint64(planIDValue), uint64(pathIDValue)
}

// newHistoryTemporaryDatabaseName 生成受严格前缀和长度约束的临时工具库名。
func newHistoryTemporaryDatabaseName(t *testing.T) string {
	t.Helper()
	buffer := make([]byte, 6)
	if _, err := rand.Read(buffer); err != nil {
		t.Fatalf("生成 F-012 临时数据库后缀失败：%v", err)
	}
	return "test_auto_pro_v2_f012_" + hex.EncodeToString(buffer)
}

// dropHistoryTemporaryDatabase 仅删除本测试创建且通过严格签名校验的临时数据库。
func dropHistoryTemporaryDatabase(t *testing.T, cfg config.PlanDBConfig) {
	t.Helper()
	if !historyTemporaryDatabasePattern.MatchString(cfg.Name) || !config.ValidDatabaseName(cfg.Name) {
		t.Errorf("拒绝清理未通过严格校验的 F-012 临时数据库")
		return
	}
	mysqlConfig := driver.NewConfig()
	mysqlConfig.User, mysqlConfig.Passwd = cfg.User, cfg.Password
	mysqlConfig.Net, mysqlConfig.Addr = "tcp", net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	mysqlConfig.ParseTime, mysqlConfig.Loc, mysqlConfig.Timeout = true, time.UTC, 5*time.Second
	db, err := sql.Open("mysql", mysqlConfig.FormatDSN())
	if err != nil {
		t.Errorf("准备 F-012 临时数据库清理失败")
		return
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, fmt.Sprintf("DROP DATABASE `%s`", cfg.Name)); err != nil {
		t.Errorf("清理 F-012 临时数据库失败")
	}
}
