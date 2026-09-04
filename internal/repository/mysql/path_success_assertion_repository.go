package mysql

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// PathSuccessAssertionRepository 读写 test_path_success_assertions。
// 断言只有一行一路径，修订与路径配置解耦，因此这里不碰 test_execution_path_configs 的任何列。
type PathSuccessAssertionRepository struct {
	db *sql.DB
}

// NewPathSuccessAssertionRepository 基于工具侧计划数据库连接创建断言仓储。
func NewPathSuccessAssertionRepository(db *sql.DB) *PathSuccessAssertionRepository {
	return &PathSuccessAssertionRepository{db: db}
}

// pathSuccessAssertionColumns 固定列顺序，读取与扫描共用同一份定义。
const pathSuccessAssertionColumns = `path_id, end_node_key, end_node_name, expected_status, arrival_ordinal, revision, updated_at`

// Get 读取单条路径的断言；先按计划核对路径归属，再读断言。
func (r *PathSuccessAssertionRepository) Get(ctx context.Context, planID, pathID uint64) (model.PathSuccessAssertion, error) {
	if r == nil || r.db == nil {
		return model.PathSuccessAssertion{}, repository.ErrExecutionPathDataInvalid
	}
	if err := r.requirePathInPlan(ctx, planID, pathID); err != nil {
		return model.PathSuccessAssertion{}, err
	}
	assertion, err := scanPathSuccessAssertion(r.db.QueryRowContext(ctx,
		`SELECT `+pathSuccessAssertionColumns+` FROM test_path_success_assertions WHERE path_id = ?`, pathID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.PathSuccessAssertion{}, repository.ErrPathSuccessAssertionNotFound
	}
	return assertion, err
}

// ListByPlan 一次取齐该计划下全部路径的断言，供运行准备聚合使用。
func (r *PathSuccessAssertionRepository) ListByPlan(ctx context.Context, planID uint64) (map[uint64]model.PathSuccessAssertion, error) {
	if r == nil || r.db == nil {
		return nil, repository.ErrExecutionPathDataInvalid
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT a.path_id, a.end_node_key, a.end_node_name, a.expected_status, a.arrival_ordinal, a.revision, a.updated_at
FROM test_path_success_assertions a
JOIN test_execution_paths p ON p.id = a.path_id
WHERE p.plan_id = ?`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	assertions := map[uint64]model.PathSuccessAssertion{}
	for rows.Next() {
		assertion, scanErr := scanPathSuccessAssertion(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		assertions[assertion.PathID] = assertion
	}
	return assertions, rows.Err()
}

// Save 在一次事务内完成归属校验、幂等判断、修订检查与写入。
// 用 FOR UPDATE 锁住断言行，保证并发保存要么串行成功要么明确返回修订冲突，不出现修订跳号或覆盖。
func (r *PathSuccessAssertionRepository) Save(ctx context.Context, planID uint64, assertion model.PathSuccessAssertion, expectedRevision uint64, idempotencyKey string, now time.Time) (model.PathSuccessAssertion, error) {
	if r == nil || r.db == nil {
		return model.PathSuccessAssertion{}, repository.ErrExecutionPathDataInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.PathSuccessAssertion{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var owned uint64
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM test_execution_paths WHERE id = ? AND plan_id = ? FOR UPDATE`,
		assertion.PathID, planID).Scan(&owned); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.PathSuccessAssertion{}, repository.ErrPathSuccessAssertionPathNotFound
		}
		return model.PathSuccessAssertion{}, err
	}
	var currentRevision uint64
	var currentKey string
	scanErr := tx.QueryRowContext(ctx,
		`SELECT revision, idempotency_key FROM test_path_success_assertions WHERE path_id = ? FOR UPDATE`,
		assertion.PathID).Scan(&currentRevision, &currentKey)
	if scanErr != nil && !errors.Is(scanErr, sql.ErrNoRows) {
		return model.PathSuccessAssertion{}, scanErr
	}
	exists := scanErr == nil
	trimmedKey := strings.TrimSpace(idempotencyKey)
	if exists && trimmedKey != "" && trimmedKey == currentKey {
		// 同一次保存请求重复到达：直接回放已有结果，不产生第二次修订。
		saved, readErr := scanPathSuccessAssertion(tx.QueryRowContext(ctx,
			`SELECT `+pathSuccessAssertionColumns+` FROM test_path_success_assertions WHERE path_id = ?`, assertion.PathID))
		if readErr != nil {
			return model.PathSuccessAssertion{}, readErr
		}
		return saved, tx.Commit()
	}
	if exists && expectedRevision != currentRevision {
		return model.PathSuccessAssertion{}, repository.ErrPathSuccessAssertionRevisionConflict
	}
	if !exists && expectedRevision != 0 {
		return model.PathSuccessAssertion{}, repository.ErrPathSuccessAssertionRevisionConflict
	}
	nextRevision := currentRevision + 1
	if _, err := tx.ExecContext(ctx, `
INSERT INTO test_path_success_assertions
  (path_id, end_node_key, end_node_name, expected_status, arrival_ordinal, revision, idempotency_key, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  end_node_key = VALUES(end_node_key), end_node_name = VALUES(end_node_name),
  expected_status = VALUES(expected_status), arrival_ordinal = VALUES(arrival_ordinal),
  revision = VALUES(revision), idempotency_key = VALUES(idempotency_key), updated_at = VALUES(updated_at)`,
		assertion.PathID, assertion.EndNodeKey, assertion.EndNodeName, assertion.ExpectedStatus,
		assertion.ArrivalOrdinal, nextRevision, trimmedKey, now, now); err != nil {
		return model.PathSuccessAssertion{}, err
	}
	saved, readErr := scanPathSuccessAssertion(tx.QueryRowContext(ctx,
		`SELECT `+pathSuccessAssertionColumns+` FROM test_path_success_assertions WHERE path_id = ?`, assertion.PathID))
	if readErr != nil {
		return model.PathSuccessAssertion{}, readErr
	}
	return saved, tx.Commit()
}

// requirePathInPlan 核对路径存在且属于该计划；按 ID 读取本身就是归属校验。
func (r *PathSuccessAssertionRepository) requirePathInPlan(ctx context.Context, planID, pathID uint64) error {
	var id uint64
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM test_execution_paths WHERE id = ? AND plan_id = ?`, pathID, planID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return repository.ErrPathSuccessAssertionPathNotFound
	}
	return err
}

// scanPathSuccessAssertion 按固定列顺序扫描一行断言，并补上中文状态标签。
func scanPathSuccessAssertion(scanner rowScanner) (model.PathSuccessAssertion, error) {
	var assertion model.PathSuccessAssertion
	if err := scanner.Scan(&assertion.PathID, &assertion.EndNodeKey, &assertion.EndNodeName,
		&assertion.ExpectedStatus, &assertion.ArrivalOrdinal, &assertion.Revision, &assertion.UpdatedAt); err != nil {
		return model.PathSuccessAssertion{}, err
	}
	assertion.ExpectedStatusLabel = model.FlowInstanceStatusLabel(assertion.ExpectedStatus)
	return assertion, nil
}
