package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	driver "github.com/go-sql-driver/mysql"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// HistoryReplayRepository 保存 F-012 历史快照、计划默认来源和路径来源。
type HistoryReplayRepository struct {
	db *sql.DB
}

// NewHistoryReplayRepository 创建历史快照仓储并复用计划数据库连接。
func NewHistoryReplayRepository(db *sql.DB) *HistoryReplayRepository {
	return &HistoryReplayRepository{db: db}
}

// NewHistoryReplayStore 创建满足历史回放存储接口的 MySQL 实现。
func NewHistoryReplayStore(db *sql.DB) repository.HistoryReplayStore {
	return NewHistoryReplayRepository(db)
}

// SaveSnapshot 按计划和候选键保存不可变目标原始数据，重复请求只返回相同摘要。
func (r *HistoryReplayRepository) SaveSnapshot(ctx context.Context, snapshot model.HistorySnapshot) (model.HistorySnapshot, error) {
	if r == nil || r.db == nil {
		return model.HistorySnapshot{}, errors.New("历史快照数据库未配置")
	}
	return saveHistorySnapshot(ctx, r.db, snapshot)
}

type historySnapshotAccess interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// saveHistorySnapshot 在调用方事务内保存不可变快照，重复候选只接受完全相同的来源摘要。
func saveHistorySnapshot(ctx context.Context, access historySnapshotAccess, snapshot model.HistorySnapshot) (model.HistorySnapshot, error) {
	instanceJSON, err := json.Marshal(nonNilJSONMap(snapshot.InstanceSummary))
	if err != nil {
		return model.HistorySnapshot{}, err
	}
	templateJSON, err := json.Marshal(nonNilJSONMap(snapshot.TemplateSummary))
	if err != nil {
		return model.HistorySnapshot{}, err
	}
	rawJSON, err := json.Marshal(nonNilJSONMap(snapshot.RawFormData))
	if err != nil {
		return model.HistorySnapshot{}, err
	}
	createdAt := snapshot.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	_, err = access.ExecContext(ctx, `
INSERT INTO test_history_data_snapshots (
  plan_id, source_account, candidate_key, flow_code, form_name, flow_name,
  render_type, instance_status, instance_summary, template_summary, raw_form_data,
  source_digest, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE id = id`,
		snapshot.PlanID, snapshot.SourceAccount, snapshot.CandidateKey, snapshot.FlowCode,
		snapshot.FormName, snapshot.FlowName, snapshot.RuntimeType, snapshot.InstanceStatus,
		string(instanceJSON), string(templateJSON), string(rawJSON), snapshot.SourceDigest, createdAt,
	)
	if err != nil {
		return model.HistorySnapshot{}, err
	}
	existing, found, err := findHistorySnapshot(ctx, access, snapshot.PlanID, snapshot.CandidateKey)
	if err != nil {
		return model.HistorySnapshot{}, err
	}
	if !found {
		return model.HistorySnapshot{}, repository.ErrHistorySnapshotNotFound
	}
	if strings.TrimSpace(snapshot.SourceDigest) != "" && strings.TrimSpace(existing.SourceDigest) != strings.TrimSpace(snapshot.SourceDigest) {
		// 同一候选键对应的目标正文变化时不能覆盖不可变快照，要求用户重新选择来源。
		return model.HistorySnapshot{}, repository.ErrHistoryRevisionConflict
	}
	return existing, nil
}

// SaveDefaultWithSnapshot 在同一事务中保存快照并切换计划默认来源，修订冲突不会留下孤儿快照。
func (r *HistoryReplayRepository) SaveDefaultWithSnapshot(ctx context.Context, snapshot model.HistorySnapshot, record repository.HistoryDefaultRecord, expectedRevision uint64, now time.Time) (model.HistorySnapshot, repository.HistoryDefaultRecord, error) {
	if r == nil || r.db == nil || snapshot.PlanID == 0 || snapshot.PlanID != record.PlanID {
		return model.HistorySnapshot{}, repository.HistoryDefaultRecord{}, repository.ErrHistorySnapshotNotFound
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.HistorySnapshot{}, repository.HistoryDefaultRecord{}, err
	}
	defer tx.Rollback()
	if err := lockHistoryPlan(ctx, tx, record.PlanID); err != nil {
		return model.HistorySnapshot{}, repository.HistoryDefaultRecord{}, err
	}
	current, found, err := getHistoryDefaultForUpdate(ctx, tx, record.PlanID)
	if err != nil {
		return model.HistorySnapshot{}, repository.HistoryDefaultRecord{}, err
	}
	if found && current.IdempotencyKey == record.IdempotencyKey {
		existing, snapshotErr := getHistorySnapshot(ctx, tx, record.PlanID, current.SnapshotID)
		if snapshotErr != nil {
			return model.HistorySnapshot{}, repository.HistoryDefaultRecord{}, snapshotErr
		}
		if existing.CandidateKey != snapshot.CandidateKey {
			return model.HistorySnapshot{}, repository.HistoryDefaultRecord{}, repository.ErrHistoryRevisionConflict
		}
		if strings.TrimSpace(snapshot.SourceDigest) != "" && strings.TrimSpace(existing.SourceDigest) != strings.TrimSpace(snapshot.SourceDigest) {
			// 相同幂等键重试也必须绑定同一份不可变正文，避免请求标识被复用于变更后的目标数据。
			return model.HistorySnapshot{}, repository.HistoryDefaultRecord{}, repository.ErrHistoryRevisionConflict
		}
		if err := tx.Commit(); err != nil {
			return model.HistorySnapshot{}, repository.HistoryDefaultRecord{}, err
		}
		return existing, current, nil
	}
	if (!found && expectedRevision != 0) || (found && (expectedRevision == 0 || current.Revision != expectedRevision)) {
		return model.HistorySnapshot{}, repository.HistoryDefaultRecord{}, repository.ErrHistoryRevisionConflict
	}
	persisted, err := saveHistorySnapshot(ctx, tx, snapshot)
	if err != nil {
		return model.HistorySnapshot{}, repository.HistoryDefaultRecord{}, err
	}
	record.SnapshotID = persisted.ID
	if found {
		record.Revision = current.Revision + 1
		record.CreatedAt = current.CreatedAt.UTC()
		record.UpdatedAt = now.UTC()
		_, err = tx.ExecContext(ctx, `
UPDATE test_plan_history_data_defaults
SET snapshot_id = ?, revision = ?, idempotency_key = ?, updated_at = ?
WHERE plan_id = ?`, record.SnapshotID, record.Revision, record.IdempotencyKey, record.UpdatedAt, record.PlanID)
	} else {
		record.Revision = 1
		record.CreatedAt = now.UTC()
		record.UpdatedAt = now.UTC()
		_, err = tx.ExecContext(ctx, `
INSERT INTO test_plan_history_data_defaults
  (plan_id, snapshot_id, revision, idempotency_key, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)`, record.PlanID, record.SnapshotID, record.Revision, record.IdempotencyKey, record.CreatedAt, record.UpdatedAt)
	}
	if err != nil {
		return model.HistorySnapshot{}, repository.HistoryDefaultRecord{}, historyRevisionError(err)
	}
	if found && current.SnapshotID != record.SnapshotID {
		// 动态继承路径不冻结快照 ID；默认来源变化时必须在同一事务清空旧回放派生值并标记受影响。
		_, err = tx.ExecContext(ctx, `
UPDATE test_execution_path_configs config
JOIN test_execution_paths path ON path.id = config.path_id
SET config.revision = config.revision + 1,
    config.data_revision = config.data_revision + 1,
    config.data_status = 'affected',
    config.runtime_type = 'unknown',
    config.effective_form_data = JSON_OBJECT(),
    config.branch_patches = JSON_ARRAY(),
    config.runtime_validation = JSON_OBJECT(),
    config.issues = JSON_ARRAY(),
    config.updated_at = ?
WHERE path.plan_id = ? AND config.source_mode = 'default'`, now.UTC(), record.PlanID)
		if err != nil {
			return model.HistorySnapshot{}, repository.HistoryDefaultRecord{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.HistorySnapshot{}, repository.HistoryDefaultRecord{}, err
	}
	return persisted, record, nil
}

// SavePathSourceWithSnapshot 在同一事务中保存独立快照和路径来源，任何修订冲突都会整体回滚。
func (r *HistoryReplayRepository) SavePathSourceWithSnapshot(ctx context.Context, planID uint64, snapshot model.HistorySnapshot, record repository.HistoryPathSourceRecord, expectedRevision uint64, now time.Time) (model.HistorySnapshot, repository.HistoryPathSourceRecord, error) {
	if r == nil || r.db == nil || snapshot.PlanID == 0 || snapshot.PlanID != planID {
		return model.HistorySnapshot{}, repository.HistoryPathSourceRecord{}, repository.ErrHistorySnapshotNotFound
	}
	if strings.TrimSpace(record.Mode) != model.HistorySourceModeOverride {
		return model.HistorySnapshot{}, repository.HistoryPathSourceRecord{}, repository.ErrHistoryReplayState
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.HistorySnapshot{}, repository.HistoryPathSourceRecord{}, err
	}
	defer tx.Rollback()
	if err := lockHistoryPlan(ctx, tx, planID); err != nil {
		return model.HistorySnapshot{}, repository.HistoryPathSourceRecord{}, err
	}
	if err := lockHistoryPath(ctx, tx, planID, record.PathID); err != nil {
		return model.HistorySnapshot{}, repository.HistoryPathSourceRecord{}, err
	}
	current, found, err := getHistoryPathSourceForUpdate(ctx, tx, record.PathID)
	if err != nil {
		return model.HistorySnapshot{}, repository.HistoryPathSourceRecord{}, err
	}
	if found && current.IdempotencyKey == record.IdempotencyKey {
		if current.Mode != record.Mode || current.SnapshotID == 0 {
			return model.HistorySnapshot{}, repository.HistoryPathSourceRecord{}, repository.ErrHistoryRevisionConflict
		}
		existing, snapshotErr := getHistorySnapshot(ctx, tx, planID, current.SnapshotID)
		if snapshotErr != nil {
			return model.HistorySnapshot{}, repository.HistoryPathSourceRecord{}, snapshotErr
		}
		if existing.CandidateKey != snapshot.CandidateKey {
			return model.HistorySnapshot{}, repository.HistoryPathSourceRecord{}, repository.ErrHistoryRevisionConflict
		}
		if strings.TrimSpace(snapshot.SourceDigest) != "" && strings.TrimSpace(existing.SourceDigest) != strings.TrimSpace(snapshot.SourceDigest) {
			// 幂等重试不允许悄悄接受同候选键但正文摘要不同的快照。
			return model.HistorySnapshot{}, repository.HistoryPathSourceRecord{}, repository.ErrHistoryRevisionConflict
		}
		if err := tx.Commit(); err != nil {
			return model.HistorySnapshot{}, repository.HistoryPathSourceRecord{}, err
		}
		return existing, current, nil
	}
	if (!found && expectedRevision != 0) || (found && (expectedRevision == 0 || current.Revision != expectedRevision)) {
		return model.HistorySnapshot{}, repository.HistoryPathSourceRecord{}, repository.ErrHistoryRevisionConflict
	}
	persisted, err := saveHistorySnapshot(ctx, tx, snapshot)
	if err != nil {
		return model.HistorySnapshot{}, repository.HistoryPathSourceRecord{}, err
	}
	record.SnapshotID = persisted.ID
	record, err = saveHistoryPathSourceLocked(ctx, tx, record, current, found, now)
	if err != nil {
		return model.HistorySnapshot{}, repository.HistoryPathSourceRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.HistorySnapshot{}, repository.HistoryPathSourceRecord{}, err
	}
	return persisted, record, nil
}

// GetSnapshotByCandidate 读取计划归属的历史快照，不跨计划接受候选键。
func (r *HistoryReplayRepository) GetSnapshotByCandidate(ctx context.Context, planID uint64, candidateKey string) (model.HistorySnapshot, error) {
	snapshot, found, err := r.findSnapshot(ctx, planID, candidateKey)
	if err != nil {
		return model.HistorySnapshot{}, err
	}
	if !found {
		return model.HistorySnapshot{}, repository.ErrHistorySnapshotNotFound
	}
	return snapshot, nil
}

// FindSnapshotByCandidate 查询计划下历史快照并区分未找到与数据库故障。
func (r *HistoryReplayRepository) FindSnapshotByCandidate(ctx context.Context, planID uint64, candidateKey string) (model.HistorySnapshot, bool, error) {
	return r.findSnapshot(ctx, planID, candidateKey)
}

// GetSnapshot 按计划归属读取指定快照，避免跨计划使用同一目标数据。
func (r *HistoryReplayRepository) GetSnapshot(ctx context.Context, planID, snapshotID uint64) (model.HistorySnapshot, error) {
	return getHistorySnapshot(ctx, r.db, planID, snapshotID)
}

// getHistorySnapshot 在数据库或调用方事务内按计划归属读取快照。
func getHistorySnapshot(ctx context.Context, access historySnapshotAccess, planID, snapshotID uint64) (model.HistorySnapshot, error) {
	row := access.QueryRowContext(ctx, `
SELECT id, plan_id, source_account, candidate_key, flow_code, form_name, flow_name,
       render_type, instance_status, instance_summary, template_summary, raw_form_data,
       source_digest, created_at
FROM test_history_data_snapshots WHERE plan_id = ? AND id = ?`, planID, snapshotID)
	snapshot, found, err := scanHistorySnapshot(row)
	if err != nil {
		return model.HistorySnapshot{}, err
	}
	if !found {
		return model.HistorySnapshot{}, repository.ErrHistorySnapshotNotFound
	}
	return snapshot, nil
}

// findSnapshot 查询计划和候选键的唯一快照行。
func (r *HistoryReplayRepository) findSnapshot(ctx context.Context, planID uint64, candidateKey string) (model.HistorySnapshot, bool, error) {
	return findHistorySnapshot(ctx, r.db, planID, candidateKey)
}

// findHistorySnapshot 在数据库或调用方事务内查询候选唯一快照。
func findHistorySnapshot(ctx context.Context, access historySnapshotAccess, planID uint64, candidateKey string) (model.HistorySnapshot, bool, error) {
	row := access.QueryRowContext(ctx, `
SELECT id, plan_id, source_account, candidate_key, flow_code, form_name, flow_name,
       render_type, instance_status, instance_summary, template_summary, raw_form_data,
       source_digest, created_at
FROM test_history_data_snapshots WHERE plan_id = ? AND candidate_key = ?`, planID, candidateKey)
	return scanHistorySnapshot(row)
}

// scanHistorySnapshot 解析快照 JSON 并把损坏数据收敛为仓储数据错误。
func scanHistorySnapshot(row interface{ Scan(...any) error }) (model.HistorySnapshot, bool, error) {
	var snapshot model.HistorySnapshot
	var instanceJSON, templateJSON, rawJSON string
	err := row.Scan(
		&snapshot.ID, &snapshot.PlanID, &snapshot.SourceAccount, &snapshot.CandidateKey,
		&snapshot.FlowCode, &snapshot.FormName, &snapshot.FlowName, &snapshot.RuntimeType,
		&snapshot.InstanceStatus, &instanceJSON, &templateJSON, &rawJSON,
		&snapshot.SourceDigest, &snapshot.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.HistorySnapshot{}, false, nil
	}
	if err != nil {
		return model.HistorySnapshot{}, false, err
	}
	if err := decodeJSONMap(instanceJSON, &snapshot.InstanceSummary); err != nil {
		return model.HistorySnapshot{}, false, repository.ErrHistorySnapshotNotFound
	}
	if err := decodeJSONMap(templateJSON, &snapshot.TemplateSummary); err != nil {
		return model.HistorySnapshot{}, false, repository.ErrHistorySnapshotNotFound
	}
	if err := decodeJSONMap(rawJSON, &snapshot.RawFormData); err != nil {
		return model.HistorySnapshot{}, false, repository.ErrHistorySnapshotNotFound
	}
	snapshot.CreatedAt = snapshot.CreatedAt.UTC()
	return snapshot, true, nil
}

// GetDefault 读取计划当前默认历史来源。
func (r *HistoryReplayRepository) GetDefault(ctx context.Context, planID uint64) (repository.HistoryDefaultRecord, bool, error) {
	return scanHistoryDefault(r.db.QueryRowContext(ctx, `
SELECT plan_id, snapshot_id, revision, idempotency_key, created_at, updated_at
FROM test_plan_history_data_defaults WHERE plan_id = ?`, planID))
}

// getHistoryDefaultForUpdate 在计划行锁内读取当前默认来源。
func getHistoryDefaultForUpdate(ctx context.Context, tx *sql.Tx, planID uint64) (repository.HistoryDefaultRecord, bool, error) {
	return scanHistoryDefault(tx.QueryRowContext(ctx, `
SELECT plan_id, snapshot_id, revision, idempotency_key, created_at, updated_at
FROM test_plan_history_data_defaults WHERE plan_id = ? FOR UPDATE`, planID))
}

// scanHistoryDefault 解析默认来源并统一 UTC 时间。
func scanHistoryDefault(row rowScanner) (repository.HistoryDefaultRecord, bool, error) {
	var record repository.HistoryDefaultRecord
	err := row.Scan(&record.PlanID, &record.SnapshotID, &record.Revision, &record.IdempotencyKey, &record.CreatedAt, &record.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return repository.HistoryDefaultRecord{}, false, nil
	}
	if err != nil {
		return repository.HistoryDefaultRecord{}, false, err
	}
	record.CreatedAt = record.CreatedAt.UTC()
	record.UpdatedAt = record.UpdatedAt.UTC()
	return record, true, nil
}

// SaveDefault 按修订号和幂等键更新计划默认历史来源。
func (r *HistoryReplayRepository) SaveDefault(ctx context.Context, record repository.HistoryDefaultRecord, expectedRevision uint64, now time.Time) (repository.HistoryDefaultRecord, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return repository.HistoryDefaultRecord{}, err
	}
	defer tx.Rollback()
	// 计划主键行提供稳定的串行化锁，避免“默认来源尚不存在”时并发插入产生半成品。
	if err := lockHistoryPlan(ctx, tx, record.PlanID); err != nil {
		return repository.HistoryDefaultRecord{}, err
	}
	current, found, err := getHistoryDefaultForUpdate(ctx, tx, record.PlanID)
	if err != nil {
		return repository.HistoryDefaultRecord{}, err
	}
	if !found {
		if expectedRevision != 0 {
			return repository.HistoryDefaultRecord{}, repository.ErrHistoryRevisionConflict
		}
		record.Revision = 1
		record.CreatedAt = now.UTC()
		record.UpdatedAt = now.UTC()
		_, err = tx.ExecContext(ctx, `
INSERT INTO test_plan_history_data_defaults
  (plan_id, snapshot_id, revision, idempotency_key, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)`, record.PlanID, record.SnapshotID, record.Revision, record.IdempotencyKey, record.CreatedAt, record.UpdatedAt)
		if err != nil {
			return repository.HistoryDefaultRecord{}, historyRevisionError(err)
		}
	} else {
		if current.IdempotencyKey == record.IdempotencyKey {
			if current.SnapshotID != record.SnapshotID {
				return repository.HistoryDefaultRecord{}, repository.ErrHistoryRevisionConflict
			}
			if err := tx.Commit(); err != nil {
				return repository.HistoryDefaultRecord{}, err
			}
			return current, nil
		}
		if expectedRevision == 0 || current.Revision != expectedRevision {
			return repository.HistoryDefaultRecord{}, repository.ErrHistoryRevisionConflict
		}
		record.Revision = current.Revision + 1
		record.CreatedAt = current.CreatedAt
		record.UpdatedAt = now.UTC()
		_, err = tx.ExecContext(ctx, `
UPDATE test_plan_history_data_defaults
SET snapshot_id = ?, revision = ?, idempotency_key = ?, updated_at = ?
WHERE plan_id = ?`, record.SnapshotID, record.Revision, record.IdempotencyKey, record.UpdatedAt, record.PlanID)
		if err != nil {
			return repository.HistoryDefaultRecord{}, historyRevisionError(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return repository.HistoryDefaultRecord{}, err
	}
	return record, nil
}

// GetPathSource 读取路径的历史来源模式和快照绑定。
func (r *HistoryReplayRepository) GetPathSource(ctx context.Context, pathID uint64) (repository.HistoryPathSourceRecord, bool, error) {
	return scanHistoryPathSource(r.db.QueryRowContext(ctx, `
SELECT path_id, source_mode, snapshot_id, data_revision, idempotency_key, updated_at
FROM test_execution_path_configs WHERE path_id = ?`, pathID))
}

// getHistoryPathSourceForUpdate 在路径主键行锁内读取当前来源配置。
func getHistoryPathSourceForUpdate(ctx context.Context, tx *sql.Tx, pathID uint64) (repository.HistoryPathSourceRecord, bool, error) {
	return scanHistoryPathSource(tx.QueryRowContext(ctx, `
SELECT path_id, source_mode, snapshot_id, data_revision, idempotency_key, updated_at
FROM test_execution_path_configs WHERE path_id = ? FOR UPDATE`, pathID))
}

// scanHistoryPathSource 解析路径来源的可空快照与数据域修订号。
func scanHistoryPathSource(row rowScanner) (repository.HistoryPathSourceRecord, bool, error) {
	var record repository.HistoryPathSourceRecord
	var snapshotID sql.NullInt64
	err := row.Scan(&record.PathID, &record.Mode, &snapshotID, &record.Revision, &record.IdempotencyKey, &record.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return repository.HistoryPathSourceRecord{}, false, nil
	}
	if err != nil {
		return repository.HistoryPathSourceRecord{}, false, err
	}
	if snapshotID.Valid {
		record.SnapshotID = uint64(snapshotID.Int64)
	}
	record.UpdatedAt = record.UpdatedAt.UTC()
	return record, true, nil
}

// SavePathSource 按路径修订号更新来源字段，不覆盖人员、动作和有效数据列。
func (r *HistoryReplayRepository) SavePathSource(ctx context.Context, planID uint64, record repository.HistoryPathSourceRecord, expectedRevision uint64, now time.Time) (repository.HistoryPathSourceRecord, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return repository.HistoryPathSourceRecord{}, err
	}
	defer tx.Rollback()
	// 计划到路径的固定加锁顺序避免默认来源替换与路径覆盖并发时形成循环等待。
	if err := lockHistoryPlan(ctx, tx, planID); err != nil {
		return repository.HistoryPathSourceRecord{}, err
	}
	if err := lockHistoryPath(ctx, tx, planID, record.PathID); err != nil {
		return repository.HistoryPathSourceRecord{}, err
	}
	current, found, err := getHistoryPathSourceForUpdate(ctx, tx, record.PathID)
	if err != nil {
		return repository.HistoryPathSourceRecord{}, err
	}
	if found && current.IdempotencyKey == record.IdempotencyKey {
		if current.Mode != record.Mode || current.SnapshotID != record.SnapshotID {
			return repository.HistoryPathSourceRecord{}, repository.ErrHistoryRevisionConflict
		}
		if err := tx.Commit(); err != nil {
			return repository.HistoryPathSourceRecord{}, err
		}
		return current, nil
	}
	if (!found && expectedRevision != 0) || (found && (expectedRevision == 0 || current.Revision != expectedRevision)) {
		return repository.HistoryPathSourceRecord{}, repository.ErrHistoryRevisionConflict
	}
	record, err = saveHistoryPathSourceLocked(ctx, tx, record, current, found, now)
	if err != nil {
		return repository.HistoryPathSourceRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return repository.HistoryPathSourceRecord{}, err
	}
	return record, nil
}

// saveHistoryPathSourceLocked 在已锁定路径事务中插入或更新来源，不覆盖人员与动作领域 JSON。
func saveHistoryPathSourceLocked(ctx context.Context, tx *sql.Tx, record, current repository.HistoryPathSourceRecord, found bool, now time.Time) (repository.HistoryPathSourceRecord, error) {
	record.UpdatedAt = now.UTC()
	if !found {
		record.Revision = 1
		_, err := tx.ExecContext(ctx, `
INSERT INTO test_execution_path_configs (
  path_id, revision, node_revision, data_revision, action_revision, idempotency_key,
  config_status, node_status, data_status, source_mode, snapshot_id, runtime_type,
  person_strategies, user_actions, compiled_steps, confirmed_node_keys,
  effective_form_data, branch_patches, runtime_validation, issues, latest_idempotency_result,
  created_at, updated_at
) VALUES (?, 1, 0, 1, 0, ?, 'pending', 'pending', 'empty', ?, ?, 'unknown',
  JSON_OBJECT(), JSON_OBJECT(), JSON_ARRAY(), JSON_ARRAY(), JSON_OBJECT(),
  JSON_ARRAY(), JSON_OBJECT(), JSON_ARRAY(), JSON_OBJECT(), ?, ?)`,
			record.PathID, record.IdempotencyKey, record.Mode, nullableSnapshotID(record.SnapshotID), record.UpdatedAt, record.UpdatedAt)
		if err != nil {
			return repository.HistoryPathSourceRecord{}, historyRevisionError(err)
		}
		return record, nil
	}
	record.Revision = current.Revision + 1
	_, err := tx.ExecContext(ctx, `
UPDATE test_execution_path_configs
SET revision = revision + 1, data_revision = ?, source_mode = ?, snapshot_id = ?, idempotency_key = ?,
    data_status = 'empty', runtime_type = 'unknown', effective_form_data = JSON_OBJECT(),
    branch_patches = JSON_ARRAY(), runtime_validation = JSON_OBJECT(), issues = JSON_ARRAY(), updated_at = ?
WHERE path_id = ?`, record.Revision, record.Mode, nullableSnapshotID(record.SnapshotID),
		record.IdempotencyKey, record.UpdatedAt, record.PathID)
	if err != nil {
		return repository.HistoryPathSourceRecord{}, historyRevisionError(err)
	}
	return record, nil
}

// lockHistoryPlan 锁定工具计划主键，串行化同计划首次默认来源创建。
func lockHistoryPlan(ctx context.Context, tx *sql.Tx, planID uint64) error {
	var lockedID uint64
	err := tx.QueryRowContext(ctx, "SELECT id FROM test_plans WHERE id = ? FOR UPDATE", planID).Scan(&lockedID)
	if errors.Is(err, sql.ErrNoRows) {
		return repository.ErrPlanNotFound
	}
	return err
}

// lockHistoryPath 锁定工具路径主键并可选核对计划归属，禁止跨计划绑定快照。
func lockHistoryPath(ctx context.Context, tx *sql.Tx, planID, pathID uint64) error {
	var storedPlanID uint64
	err := tx.QueryRowContext(ctx, "SELECT plan_id FROM test_execution_paths WHERE id = ? FOR UPDATE", pathID).Scan(&storedPlanID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && planID != 0 && storedPlanID != planID) {
		return repository.ErrExecutionPathNotFound
	}
	return err
}

// historyRevisionError 将唯一键并发竞争收敛为修订冲突，其余数据库错误保持原样。
func historyRevisionError(err error) error {
	var mysqlErr *driver.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return repository.ErrHistoryRevisionConflict
	}
	return err
}

// CreateReplay 在 T04 接入历史回放任务事务；T02 禁止提前创建运行事实。
func (r *HistoryReplayRepository) CreateReplay(context.Context, model.HistoryReplayJob, []model.HistoryReplayItem) (model.HistoryReplayJob, bool, error) {
	return model.HistoryReplayJob{}, false, repository.ErrHistoryReplayState
}

// GetReplay 在 T04 接入历史回放任务读取；T02 不创建或读取运行任务。
func (r *HistoryReplayRepository) GetReplay(context.Context, uint64, string) (model.HistoryReplayJob, error) {
	return model.HistoryReplayJob{}, repository.ErrHistoryReplayState
}

// FindActiveReplay 在 T04 接入单活动任务查询；T02 不创建运行任务。
func (r *HistoryReplayRepository) FindActiveReplay(context.Context, uint64) (model.HistoryReplayJob, bool, error) {
	return model.HistoryReplayJob{}, false, repository.ErrHistoryReplayState
}

// UpdateReplayStatus 在 T04 接入任务状态机；T02 不改变任何运行状态。
func (r *HistoryReplayRepository) UpdateReplayStatus(context.Context, uint64, string, string, time.Time) (model.HistoryReplayJob, error) {
	return model.HistoryReplayJob{}, repository.ErrHistoryReplayState
}

// ClaimReplayItems 在 T04 接入租约领取；T02 不领取任何路径明细。
func (r *HistoryReplayRepository) ClaimReplayItems(context.Context, string, int, string, time.Time) ([]model.HistoryReplayItem, error) {
	return nil, repository.ErrHistoryReplayState
}

// CompleteReplayItem 在 T04 接入单路径终态；T02 不写入回放结果。
func (r *HistoryReplayRepository) CompleteReplayItem(context.Context, string, uint64, model.HistoryReplayItem, time.Time) error {
	return repository.ErrHistoryReplayState
}

// RecountReplay 在 T04 接入真实计数；T02 不聚合回放任务。
func (r *HistoryReplayRepository) RecountReplay(context.Context, string, time.Time) (model.HistoryReplayJob, error) {
	return model.HistoryReplayJob{}, repository.ErrHistoryReplayState
}

// ListReplayItems 在 T04 接入游标分页；T02 不读取回放明细。
func (r *HistoryReplayRepository) ListReplayItems(context.Context, uint64, string, uint64, int) (model.HistoryReplayItemPage, error) {
	return model.HistoryReplayItemPage{}, repository.ErrHistoryReplayState
}

// nonNilJSONMap 保证 JSON 列保存对象而不是 SQL NULL。
func nonNilJSONMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

// decodeJSONMap 从 MySQL JSON 列读取对象并拒绝数组或空正文。
func decodeJSONMap(raw string, target *map[string]any) error {
	if strings.TrimSpace(raw) == "" {
		*target = map[string]any{}
		return nil
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil || decoded == nil {
		return fmt.Errorf("历史快照 JSON 不是对象")
	}
	*target = decoded
	return nil
}

// nullableSnapshotID 把零值来源转换为 SQL NULL，避免伪造目标快照主键。
func nullableSnapshotID(id uint64) any {
	if id == 0 {
		return nil
	}
	return id
}
