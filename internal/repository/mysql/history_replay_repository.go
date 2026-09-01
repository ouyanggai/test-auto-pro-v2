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

const (
	// historyReplayLeaseDuration 为后台 worker 提供有限租约；过期后同一明细可被新 worker 接管。
	historyReplayLeaseDuration = 2 * time.Minute
	// historyReplayDefaultPageSize 限制游标接口单次读取，避免把完整历史正文一次性加载到内存。
	historyReplayDefaultPageSize = 20
	// historyReplayMaxPageSize 是回放明细的服务端硬上限。
	historyReplayMaxPageSize = 100
)

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

// CreateReplay 在计划锁内一次建立任务及所有路径检查点，幂等重试只返回原任务。
func (r *HistoryReplayRepository) CreateReplay(ctx context.Context, job model.HistoryReplayJob, items []model.HistoryReplayItem) (model.HistoryReplayJob, bool, error) {
	if r == nil || r.db == nil || job.PlanID == 0 || strings.TrimSpace(job.ID) == "" || len(items) == 0 {
		return model.HistoryReplayJob{}, false, repository.ErrHistoryReplayState
	}
	// 任务 ID 由服务层直接采用请求幂等键；仓储直调时以同一 ID 作为幂等键，避免出现无法重试的半配置任务。
	if strings.TrimSpace(job.IdempotencyKey) == "" {
		job.IdempotencyKey = strings.TrimSpace(job.ID)
	}
	now := job.CreatedAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.HistoryReplayJob{}, false, err
	}
	defer tx.Rollback()
	if err := lockHistoryPlan(ctx, tx, job.PlanID); err != nil {
		return model.HistoryReplayJob{}, false, err
	}
	if existing, found, findErr := findHistoryReplayByIdempotency(ctx, tx, job.PlanID, job.IdempotencyKey); findErr != nil {
		return model.HistoryReplayJob{}, false, findErr
	} else if found {
		existingItems, itemErr := listHistoryReplayItemIDs(ctx, tx, existing.ID)
		if itemErr != nil {
			return model.HistoryReplayJob{}, false, itemErr
		}
		if !sameReplayPathSet(existingItems, items) {
			return model.HistoryReplayJob{}, false, repository.ErrHistoryReplayIdempotency
		}
		if err := tx.Commit(); err != nil {
			return model.HistoryReplayJob{}, false, err
		}
		return existing, false, nil
	}
	if _, found, activeErr := findActiveHistoryReplay(ctx, tx, job.PlanID); activeErr != nil {
		return model.HistoryReplayJob{}, false, activeErr
	} else if found {
		return model.HistoryReplayJob{}, false, repository.ErrHistoryReplayActive
	}
	if err := validateReplayPaths(ctx, tx, job.PlanID, items); err != nil {
		return model.HistoryReplayJob{}, false, err
	}
	job.Status = model.HistoryReplayStatusQueued
	job.Total = len(items)
	job.Pending = len(items)
	job.Running, job.Ready, job.NeedsInput, job.Affected, job.Failed, job.Cancelled = 0, 0, 0, 0, 0, 0
	job.CreatedAt, job.UpdatedAt = now, now
	job.CompletedAt = nil
	_, err = tx.ExecContext(ctx, `
INSERT INTO test_history_replay_jobs
  (id, plan_id, idempotency_key, status, total_count, pending_count, running_count,
   ready_count, needs_input_count, affected_count, failed_count, cancelled_count,
   lease_owner, lease_expires_at, fencing_token, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, 0, 0, 0, 0, 0, 0, NULL, NULL, 0, ?, ?)`,
		job.ID, job.PlanID, job.IdempotencyKey, job.Status, job.Total, job.Pending, now, now)
	if err != nil {
		return model.HistoryReplayJob{}, false, historyReplayCreateError(err)
	}
	for _, item := range items {
		issues, marshalErr := json.Marshal(nonNilHistoryIssues(item.Issues))
		if marshalErr != nil {
			return model.HistoryReplayJob{}, false, marshalErr
		}
		patches, marshalErr := json.Marshal(nonNilHistoryPatches(item.BranchPatches))
		if marshalErr != nil {
			return model.HistoryReplayJob{}, false, marshalErr
		}
		effective, marshalErr := json.Marshal(nonNilJSONMap(item.EffectiveFormData))
		if marshalErr != nil {
			return model.HistoryReplayJob{}, false, marshalErr
		}
		_, err = tx.ExecContext(ctx, `
INSERT INTO test_history_replay_items
  (job_id, path_id, path_revision, snapshot_id, status, data_status, issue,
   branch_patches, effective_form_data, lease_owner, lease_expires_at, updated_at, completed_at)
VALUES (?, ?, ?, ?, 'pending', 'empty', ?, ?, ?, NULL, NULL, ?, NULL)`,
			job.ID, item.PathID, item.PathRevision, nullableSnapshotPointer(item.SnapshotID),
			string(issues), string(patches), string(effective), now)
		if err != nil {
			return model.HistoryReplayJob{}, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.HistoryReplayJob{}, false, err
	}
	return job, true, nil
}

// GetReplay 按计划归属读取任务聚合计数，防止跨计划猜测任务存在性。
func (r *HistoryReplayRepository) GetReplay(ctx context.Context, planID uint64, jobID string) (model.HistoryReplayJob, error) {
	if r == nil || r.db == nil || planID == 0 || strings.TrimSpace(jobID) == "" {
		return model.HistoryReplayJob{}, repository.ErrHistoryReplayNotFound
	}
	job, err := scanHistoryReplayJob(r.db.QueryRowContext(ctx, historyReplayJobSelect+" WHERE plan_id = ? AND id = ?", planID, jobID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.HistoryReplayJob{}, repository.ErrHistoryReplayNotFound
	}
	return job, err
}

// FindActiveReplay 返回同一计划唯一排队或运行中的任务，不把取消任务误报为活动任务。
func (r *HistoryReplayRepository) FindActiveReplay(ctx context.Context, planID uint64) (model.HistoryReplayJob, bool, error) {
	if r == nil || r.db == nil || planID == 0 {
		return model.HistoryReplayJob{}, false, repository.ErrHistoryReplayState
	}
	return findActiveHistoryReplay(ctx, r.db, planID)
}

// ListRecoverableReplays 返回服务重启后仍可继续处理的任务，过期租约由后续领取事务接管。
func (r *HistoryReplayRepository) ListRecoverableReplays(ctx context.Context) ([]model.HistoryReplayJob, error) {
	if r == nil || r.db == nil {
		return nil, repository.ErrHistoryReplayState
	}
	rows, err := r.db.QueryContext(ctx, historyReplayJobSelect+" WHERE status IN ('queued', 'running') ORDER BY created_at, id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := make([]model.HistoryReplayJob, 0)
	for rows.Next() {
		job, scanErr := scanHistoryReplayJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return jobs, nil
}

// UpdateReplayStatus 执行排队、运行、取消、失败和恢复的有限状态转换，并清理取消时的租约。
func (r *HistoryReplayRepository) UpdateReplayStatus(ctx context.Context, planID uint64, jobID, status string, now time.Time) (model.HistoryReplayJob, error) {
	if r == nil || r.db == nil || planID == 0 || strings.TrimSpace(jobID) == "" {
		return model.HistoryReplayJob{}, repository.ErrHistoryReplayNotFound
	}
	now = now.UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.HistoryReplayJob{}, err
	}
	defer tx.Rollback()
	job, err := scanHistoryReplayJob(tx.QueryRowContext(ctx, historyReplayJobSelect+" WHERE plan_id = ? AND id = ? FOR UPDATE", planID, jobID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.HistoryReplayJob{}, repository.ErrHistoryReplayNotFound
	}
	if err != nil {
		return model.HistoryReplayJob{}, err
	}
	if !validHistoryReplayStatus(status) || !historyReplayTransitionAllowed(job.Status, status) {
		return model.HistoryReplayJob{}, repository.ErrHistoryReplayState
	}
	if status == model.HistoryReplayStatusCancelled || status == model.HistoryReplayStatusFailed {
		// 取消/失败只保留已完成明细；被中断的 running 明细退回 pending，恢复时可重新领取。
		if _, err := tx.ExecContext(ctx, `UPDATE test_history_replay_items
SET status = 'pending', lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
WHERE job_id = ? AND status = 'running'`, now, jobID); err != nil {
			return model.HistoryReplayJob{}, err
		}
	}
	if status == model.HistoryReplayStatusQueued {
		if _, err := tx.ExecContext(ctx, `UPDATE test_history_replay_items
SET status = 'pending', lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
WHERE job_id = ? AND status = 'running'`, now, jobID); err != nil {
			return model.HistoryReplayJob{}, err
		}
	}
	completedAt := any(nil)
	if status == model.HistoryReplayStatusCompleted {
		completedAt = now
	}
	_, err = tx.ExecContext(ctx, `UPDATE test_history_replay_jobs
SET status = ?, lease_owner = NULL, lease_expires_at = NULL, completed_at = ?, updated_at = ?
WHERE plan_id = ? AND id = ?`, status, completedAt, now, planID, jobID)
	if err != nil {
		return model.HistoryReplayJob{}, err
	}
	job, err = recountHistoryReplayLocked(ctx, tx, jobID, now)
	if err != nil {
		return model.HistoryReplayJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.HistoryReplayJob{}, err
	}
	return job, nil
}

// ClaimReplayItems 领取未完成明细并更新任务租约；过期 worker 由 fencing token 隔离。
func (r *HistoryReplayRepository) ClaimReplayItems(ctx context.Context, jobID string, limit int, workerID string, now time.Time) ([]model.HistoryReplayItem, error) {
	if r == nil || r.db == nil || strings.TrimSpace(jobID) == "" || strings.TrimSpace(workerID) == "" || limit < 1 || limit > historyReplayMaxPageSize {
		return nil, repository.ErrHistoryReplayState
	}
	now = now.UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	job, err := scanHistoryReplayJob(tx.QueryRowContext(ctx, historyReplayJobSelect+" WHERE id = ? FOR UPDATE", jobID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrHistoryReplayNotFound
	}
	if err != nil {
		return nil, err
	}
	if job.Status == model.HistoryReplayStatusQueued {
		if _, err := tx.ExecContext(ctx, "UPDATE test_history_replay_jobs SET status = 'running', updated_at = ? WHERE id = ?", now, jobID); err != nil {
			return nil, err
		}
		job.Status = model.HistoryReplayStatusRunning
	}
	if job.Status != model.HistoryReplayStatusRunning {
		return []model.HistoryReplayItem{}, tx.Commit()
	}
	if job.LeaseOwner != "" && job.LeaseOwner != workerID && job.LeaseExpiresAt != nil && job.LeaseExpiresAt.After(now) {
		return []model.HistoryReplayItem{}, tx.Commit()
	}
	leaseUntil := now.Add(historyReplayLeaseDuration)
	token := job.FencingToken + 1
	if _, err := tx.ExecContext(ctx, `UPDATE test_history_replay_jobs
SET lease_owner = ?, lease_expires_at = ?, fencing_token = ?, updated_at = ? WHERE id = ?`, workerID, leaseUntil, token, now, jobID); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, job_id, path_id, path_revision, snapshot_id, status, data_status,
issue, branch_patches, effective_form_data, lease_owner, lease_expires_at, updated_at, completed_at
FROM test_history_replay_items
WHERE job_id = ? AND (status = 'pending' OR (status = 'running' AND lease_expires_at IS NOT NULL AND lease_expires_at <= ?))
ORDER BY id LIMIT ? FOR UPDATE`, jobID, now, limit)
	if err != nil {
		return nil, err
	}
	items := make([]model.HistoryReplayItem, 0, limit)
	for rows.Next() {
		item, scanErr := scanHistoryReplayItem(rows)
		if scanErr != nil {
			rows.Close()
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range items {
		if _, err := tx.ExecContext(ctx, `UPDATE test_history_replay_items
SET status = 'running', lease_owner = ?, lease_expires_at = ?, updated_at = ? WHERE id = ?`, workerID, leaseUntil, now, items[index].ID); err != nil {
			return nil, err
		}
		items[index].Status = model.HistoryReplayItemStatusRunning
		items[index].LeaseOwner = workerID
		items[index].LeaseExpiresAt = timePtr(leaseUntil)
		items[index].FencingToken = token
		items[index].UpdatedAt = now
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return items, nil
}

// CompleteReplayItem 仅允许仍持有明细租约的 worker 写入一个确定终态。
func (r *HistoryReplayRepository) CompleteReplayItem(ctx context.Context, jobID string, itemID uint64, item model.HistoryReplayItem, now time.Time) error {
	if r == nil || r.db == nil || strings.TrimSpace(jobID) == "" || itemID == 0 || strings.TrimSpace(item.LeaseOwner) == "" || !validHistoryReplayItemStatus(item.Status) {
		return repository.ErrHistoryReplayState
	}
	now = now.UTC()
	issues, err := json.Marshal(nonNilHistoryIssues(item.Issues))
	if err != nil {
		return err
	}
	patches, err := json.Marshal(nonNilHistoryPatches(item.BranchPatches))
	if err != nil {
		return err
	}
	effective, err := json.Marshal(nonNilJSONMap(item.EffectiveFormData))
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	job, err := scanHistoryReplayJob(tx.QueryRowContext(ctx, historyReplayJobSelect+" WHERE id = ? FOR UPDATE", jobID))
	if errors.Is(err, sql.ErrNoRows) {
		return repository.ErrHistoryReplayNotFound
	}
	if err != nil {
		return err
	}
	if job.Status != model.HistoryReplayStatusRunning || item.FencingToken != 0 && item.FencingToken != job.FencingToken {
		return repository.ErrHistoryReplayState
	}
	var currentStatus, currentOwner string
	var currentExpires sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT status, lease_owner, lease_expires_at FROM test_history_replay_items WHERE id = ? AND job_id = ? FOR UPDATE`, itemID, jobID).Scan(&currentStatus, &currentOwner, &currentExpires); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.ErrHistoryReplayNotFound
		}
		return err
	}
	if currentStatus != model.HistoryReplayItemStatusRunning || currentOwner != item.LeaseOwner || !currentExpires.Valid || currentExpires.Time.Before(now) {
		return repository.ErrHistoryReplayState
	}
	completedAt := now
	result, err := tx.ExecContext(ctx, `UPDATE test_history_replay_items
SET status = ?, data_status = ?, issue = ?, branch_patches = ?, effective_form_data = ?,
    lease_owner = NULL, lease_expires_at = NULL, updated_at = ?, completed_at = ?
WHERE id = ? AND job_id = ? AND status = 'running' AND lease_owner = ?`,
		item.Status, normalizeReplayDataStatus(item.DataStatus, item.Status), string(issues), string(patches), string(effective), now, completedAt, itemID, jobID, item.LeaseOwner)
	if err != nil {
		return err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		if rowsErr != nil {
			return rowsErr
		}
		return repository.ErrHistoryReplayState
	}
	// 明细终态与当前路径配置必须在同一事务内落盘；否则任务已完成而列表仍显示旧 dataStatus，刷新后会丢失回放结果。
	runtimeValidation, err := json.Marshal(item.RuntimeValidation)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE test_execution_path_configs
SET data_revision = data_revision + 1,
    data_status = ?,
    runtime_type = CASE WHEN ? <> '' THEN ? ELSE runtime_type END,
    effective_form_data = ?, branch_patches = ?, runtime_validation = ?, issues = ?, updated_at = ?
WHERE path_id = ?`, normalizeReplayDataStatus(item.DataStatus, item.Status), item.RuntimeType, item.RuntimeType,
		string(effective), string(patches), string(runtimeValidation), string(issues), now, item.PathID); err != nil {
		return err
	}
	if _, err := recountHistoryReplayLocked(ctx, tx, jobID, now); err != nil {
		return err
	}
	return tx.Commit()
}

// RecountReplay 从明细真实状态重算任务聚合计数，并在没有未完成项时落为 completed。
func (r *HistoryReplayRepository) RecountReplay(ctx context.Context, jobID string, now time.Time) (model.HistoryReplayJob, error) {
	if r == nil || r.db == nil || strings.TrimSpace(jobID) == "" {
		return model.HistoryReplayJob{}, repository.ErrHistoryReplayNotFound
	}
	now = now.UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.HistoryReplayJob{}, err
	}
	defer tx.Rollback()
	if _, err := scanHistoryReplayJob(tx.QueryRowContext(ctx, historyReplayJobSelect+" WHERE id = ? FOR UPDATE", jobID)); errors.Is(err, sql.ErrNoRows) {
		return model.HistoryReplayJob{}, repository.ErrHistoryReplayNotFound
	} else if err != nil {
		return model.HistoryReplayJob{}, err
	}
	job, err := recountHistoryReplayLocked(ctx, tx, jobID, now)
	if err != nil {
		return model.HistoryReplayJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.HistoryReplayJob{}, err
	}
	return job, nil
}

// ListReplayItems 按自增明细 ID 执行有界游标分页，不把租约内部字段返回给调用方。
func (r *HistoryReplayRepository) ListReplayItems(ctx context.Context, planID uint64, jobID string, cursor uint64, limit int) (model.HistoryReplayItemPage, error) {
	if r == nil || r.db == nil || planID == 0 || strings.TrimSpace(jobID) == "" || limit < 1 || limit > historyReplayMaxPageSize {
		return model.HistoryReplayItemPage{}, repository.ErrHistoryReplayState
	}
	if _, err := r.GetReplay(ctx, planID, jobID); err != nil {
		return model.HistoryReplayItemPage{}, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, job_id, path_id, path_revision, snapshot_id, status, data_status,
issue, branch_patches, effective_form_data, lease_owner, lease_expires_at, updated_at, completed_at
FROM test_history_replay_items WHERE job_id = ? AND id > ? ORDER BY id LIMIT ?`, jobID, cursor, limit+1)
	if err != nil {
		return model.HistoryReplayItemPage{}, err
	}
	defer rows.Close()
	items := make([]model.HistoryReplayItem, 0, limit)
	for rows.Next() {
		item, scanErr := scanHistoryReplayItem(rows)
		if scanErr != nil {
			return model.HistoryReplayItemPage{}, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return model.HistoryReplayItemPage{}, err
	}
	page := model.HistoryReplayItemPage{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		page.NextCursor = page.Items[len(page.Items)-1].ID
	}
	return page, nil
}

const historyReplayJobSelect = `SELECT id, plan_id, idempotency_key, status, total_count, pending_count,
running_count, ready_count, needs_input_count, affected_count, failed_count, cancelled_count,
lease_owner, lease_expires_at, fencing_token, created_at, updated_at, completed_at
FROM test_history_replay_jobs`

// historyReplayItemSelect 是任务明细的统一列顺序，所有读取都按同一 JSON 解码边界处理。
const historyReplayItemSelect = `SELECT id, job_id, path_id, path_revision, snapshot_id, status, data_status,
issue, branch_patches, effective_form_data, lease_owner, lease_expires_at, updated_at, completed_at
FROM test_history_replay_items`

// findHistoryReplayByIdempotency 在事务内读取同计划幂等任务，避免请求标识跨计划复用。
func findHistoryReplayByIdempotency(ctx context.Context, access historyReplayAccess, planID uint64, key string) (model.HistoryReplayJob, bool, error) {
	job, err := scanHistoryReplayJob(access.QueryRowContext(ctx, historyReplayJobSelect+" WHERE plan_id = ? AND idempotency_key = ? FOR UPDATE", planID, key))
	if errors.Is(err, sql.ErrNoRows) {
		return model.HistoryReplayJob{}, false, nil
	}
	return job, err == nil, err
}

// findActiveHistoryReplay 查询计划内唯一排队或运行任务，调用方需在计划锁内使用它。
func findActiveHistoryReplay(ctx context.Context, access historyReplayAccess, planID uint64) (model.HistoryReplayJob, bool, error) {
	job, err := scanHistoryReplayJob(access.QueryRowContext(ctx, historyReplayJobSelect+" WHERE plan_id = ? AND status IN ('queued', 'running') ORDER BY created_at, id LIMIT 1", planID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.HistoryReplayJob{}, false, nil
	}
	return job, err == nil, err
}

// historyReplayAccess 抽象事务和连接的单行查询，保持状态机读取逻辑可复用。
type historyReplayAccess interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// validateReplayPaths 在任务创建事务内核对所有路径仍属于当前计划且没有重复明细。
func validateReplayPaths(ctx context.Context, tx *sql.Tx, planID uint64, items []model.HistoryReplayItem) error {
	seen := make(map[uint64]struct{}, len(items))
	for _, item := range items {
		if item.PathID == 0 {
			return repository.ErrExecutionPathNotFound
		}
		if _, exists := seen[item.PathID]; exists {
			return repository.ErrHistoryReplayIdempotency
		}
		seen[item.PathID] = struct{}{}
		var storedID uint64
		if err := tx.QueryRowContext(ctx, "SELECT id FROM test_execution_paths WHERE id = ? AND plan_id = ? FOR UPDATE", item.PathID, planID).Scan(&storedID); errors.Is(err, sql.ErrNoRows) {
			return repository.ErrExecutionPathNotFound
		} else if err != nil {
			return err
		}
	}
	return nil
}

// listHistoryReplayItemIDs 读取幂等任务的路径集合，不展开历史正文。
func listHistoryReplayItemIDs(ctx context.Context, tx *sql.Tx, jobID string) ([]model.HistoryReplayItem, error) {
	rows, err := tx.QueryContext(ctx, "SELECT path_id, path_revision FROM test_history_replay_items WHERE job_id = ?", jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]model.HistoryReplayItem, 0)
	for rows.Next() {
		var item model.HistoryReplayItem
		if err := rows.Scan(&item.PathID, &item.PathRevision); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// sameReplayPathSet 判断幂等重试是否仍指向相同路径和修订快照。
func sameReplayPathSet(existing, requested []model.HistoryReplayItem) bool {
	if len(existing) != len(requested) {
		return false
	}
	left := make(map[uint64]uint64, len(existing))
	for _, item := range existing {
		left[item.PathID] = item.PathRevision
	}
	for _, item := range requested {
		revision, found := left[item.PathID]
		if !found || revision != item.PathRevision {
			return false
		}
		delete(left, item.PathID)
	}
	return len(left) == 0
}

// historyReplayCreateError 将任务唯一键竞争转换为可重试的业务错误。
func historyReplayCreateError(err error) error {
	var mysqlErr *driver.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return repository.ErrHistoryReplayActive
	}
	return err
}

// validHistoryReplayStatus 判断状态机只接受公开的五种任务状态。
func validHistoryReplayStatus(status string) bool {
	switch status {
	case model.HistoryReplayStatusQueued, model.HistoryReplayStatusRunning, model.HistoryReplayStatusCompleted, model.HistoryReplayStatusCancelled, model.HistoryReplayStatusFailed:
		return true
	default:
		return false
	}
}

// historyReplayTransitionAllowed 约束取消、恢复和 worker 状态变更不能越过终态。
func historyReplayTransitionAllowed(from, to string) bool {
	if from == to {
		return true
	}
	switch from {
	case model.HistoryReplayStatusQueued:
		return to == model.HistoryReplayStatusRunning || to == model.HistoryReplayStatusCancelled || to == model.HistoryReplayStatusFailed
	case model.HistoryReplayStatusRunning:
		return to == model.HistoryReplayStatusCancelled || to == model.HistoryReplayStatusFailed || to == model.HistoryReplayStatusCompleted
	case model.HistoryReplayStatusCancelled, model.HistoryReplayStatusFailed:
		return to == model.HistoryReplayStatusQueued
	default:
		return false
	}
}

// validHistoryReplayItemStatus 判断明细只允许写入四类确定终态。
func validHistoryReplayItemStatus(status string) bool {
	switch status {
	case model.HistoryReplayItemStatusReady, model.HistoryReplayItemStatusNeedsInput, model.HistoryReplayItemStatusAffected, model.HistoryReplayItemStatusFailed:
		return true
	default:
		return false
	}
}

// normalizeReplayDataStatus 让明细状态与数据状态保持同一确定终态，避免聚合出现不可解释组合。
func normalizeReplayDataStatus(dataStatus, itemStatus string) string {
	switch dataStatus {
	case model.HistoryDataStatusReady, model.HistoryDataStatusNeedsInput, model.HistoryDataStatusAffected:
		return dataStatus
	}
	switch itemStatus {
	case model.HistoryReplayItemStatusReady:
		return model.HistoryDataStatusReady
	case model.HistoryReplayItemStatusAffected:
		return model.HistoryDataStatusAffected
	default:
		return model.HistoryDataStatusNeedsInput
	}
}

// scanHistoryReplayJob 将任务行的租约和可空完成时间转换为服务端内部字段。
func scanHistoryReplayJob(row interface{ Scan(...any) error }) (model.HistoryReplayJob, error) {
	var job model.HistoryReplayJob
	var leaseOwner sql.NullString
	var leaseExpires, completed sql.NullTime
	err := row.Scan(&job.ID, &job.PlanID, &job.IdempotencyKey, &job.Status, &job.Total, &job.Pending, &job.Running, &job.Ready, &job.NeedsInput, &job.Affected, &job.Failed, &job.Cancelled, &leaseOwner, &leaseExpires, &job.FencingToken, &job.CreatedAt, &job.UpdatedAt, &completed)
	if err != nil {
		return model.HistoryReplayJob{}, err
	}
	job.LeaseOwner = leaseOwner.String
	if leaseExpires.Valid {
		job.LeaseExpiresAt = timePtr(leaseExpires.Time.UTC())
	}
	if completed.Valid {
		job.CompletedAt = timePtr(completed.Time.UTC())
	}
	job.CreatedAt, job.UpdatedAt = job.CreatedAt.UTC(), job.UpdatedAt.UTC()
	return job, nil
}

// scanHistoryReplayItem 将明细 JSON 和可空租约解码为原始数据结构。
func scanHistoryReplayItem(row interface{ Scan(...any) error }) (model.HistoryReplayItem, error) {
	var item model.HistoryReplayItem
	var snapshotID sql.NullInt64
	var issueJSON, patchJSON, effectiveJSON string
	var leaseOwner sql.NullString
	var leaseExpires, completed sql.NullTime
	err := row.Scan(&item.ID, &item.JobID, &item.PathID, &item.PathRevision, &snapshotID, &item.Status, &item.DataStatus, &issueJSON, &patchJSON, &effectiveJSON, &leaseOwner, &leaseExpires, &item.UpdatedAt, &completed)
	if err != nil {
		return model.HistoryReplayItem{}, err
	}
	if snapshotID.Valid {
		value := uint64(snapshotID.Int64)
		item.SnapshotID = &value
	}
	if err := decodeHistoryIssues(issueJSON, &item.Issues); err != nil {
		return model.HistoryReplayItem{}, repository.ErrHistoryReplayState
	}
	if err := decodeHistoryPatches(patchJSON, &item.BranchPatches); err != nil {
		return model.HistoryReplayItem{}, repository.ErrHistoryReplayState
	}
	if err := decodeJSONMap(effectiveJSON, &item.EffectiveFormData); err != nil {
		return model.HistoryReplayItem{}, repository.ErrHistoryReplayState
	}
	item.LeaseOwner = leaseOwner.String
	if leaseExpires.Valid {
		item.LeaseExpiresAt = timePtr(leaseExpires.Time.UTC())
	}
	if completed.Valid {
		item.CompletedAt = timePtr(completed.Time.UTC())
	}
	item.UpdatedAt = item.UpdatedAt.UTC()
	return item, nil
}

// decodeHistoryIssues 从 JSON 列恢复结构化问题并拒绝错误类型。
func decodeHistoryIssues(raw string, target *[]model.HistoryDataIssue) error {
	if strings.TrimSpace(raw) == "" {
		*target = []model.HistoryDataIssue{}
		return nil
	}
	var values []model.HistoryDataIssue
	if err := json.Unmarshal([]byte(raw), &values); err != nil || values == nil {
		return errors.New("历史回放问题 JSON 格式不正确")
	}
	*target = values
	return nil
}

// decodeHistoryPatches 从 JSON 列恢复分支补丁列表并拒绝错误类型。
func decodeHistoryPatches(raw string, target *[]model.HistoryBranchPatch) error {
	if strings.TrimSpace(raw) == "" {
		*target = []model.HistoryBranchPatch{}
		return nil
	}
	var values []model.HistoryBranchPatch
	if err := json.Unmarshal([]byte(raw), &values); err != nil || values == nil {
		return errors.New("历史回放补丁 JSON 格式不正确")
	}
	*target = values
	return nil
}

// nonNilHistoryIssues 保证问题列表写为 JSON 数组而不是 SQL NULL。
func nonNilHistoryIssues(values []model.HistoryDataIssue) []model.HistoryDataIssue {
	if values == nil {
		return []model.HistoryDataIssue{}
	}
	return values
}

// nonNilHistoryPatches 保证补丁列表写为 JSON 数组而不是 SQL NULL。
func nonNilHistoryPatches(values []model.HistoryBranchPatch) []model.HistoryBranchPatch {
	if values == nil {
		return []model.HistoryBranchPatch{}
	}
	return values
}

// recountHistoryReplayLocked 在任务行锁内从明细状态重算计数和终态。
func recountHistoryReplayLocked(ctx context.Context, tx *sql.Tx, jobID string, now time.Time) (model.HistoryReplayJob, error) {
	job, err := scanHistoryReplayJob(tx.QueryRowContext(ctx, historyReplayJobSelect+" WHERE id = ? FOR UPDATE", jobID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.HistoryReplayJob{}, repository.ErrHistoryReplayNotFound
	}
	if err != nil {
		return model.HistoryReplayJob{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE test_history_replay_items
SET status = 'pending', lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
WHERE job_id = ? AND status = 'running' AND lease_expires_at IS NOT NULL AND lease_expires_at <= ?`, now, jobID, now); err != nil {
		return model.HistoryReplayJob{}, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT status, COUNT(*) FROM test_history_replay_items WHERE job_id = ? GROUP BY status", jobID)
	if err != nil {
		return model.HistoryReplayJob{}, err
	}
	counts := map[string]int{}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			rows.Close()
			return model.HistoryReplayJob{}, err
		}
		counts[status] = count
	}
	if err := rows.Close(); err != nil {
		return model.HistoryReplayJob{}, err
	}
	if err := rows.Err(); err != nil {
		return model.HistoryReplayJob{}, err
	}
	total := 0
	for _, count := range counts {
		total += count
	}
	pending, running := counts[model.HistoryReplayItemStatusPending], counts[model.HistoryReplayItemStatusRunning]
	ready, needsInput := counts[model.HistoryReplayItemStatusReady], counts[model.HistoryReplayItemStatusNeedsInput]
	affected, failed := counts[model.HistoryReplayItemStatusAffected], counts[model.HistoryReplayItemStatusFailed]
	cancelled := counts["cancelled"]
	status := job.Status
	completedAt := job.CompletedAt
	if status != model.HistoryReplayStatusCancelled && pending+running == 0 {
		status = model.HistoryReplayStatusCompleted
		completedAt = timePtr(now)
	} else if status == model.HistoryReplayStatusCompleted {
		status = model.HistoryReplayStatusRunning
		completedAt = nil
	}
	_, err = tx.ExecContext(ctx, `UPDATE test_history_replay_jobs SET status = ?, total_count = ?, pending_count = ?, running_count = ?,
ready_count = ?, needs_input_count = ?, affected_count = ?, failed_count = ?, cancelled_count = ?,
updated_at = ?, completed_at = ? WHERE id = ?`, status, total, pending, running, ready, needsInput, affected, failed, cancelled, now, nullableTime(completedAt), jobID)
	if err != nil {
		return model.HistoryReplayJob{}, err
	}
	job.Status, job.Total, job.Pending, job.Running = status, total, pending, running
	job.Ready, job.NeedsInput, job.Affected, job.Failed, job.Cancelled = ready, needsInput, affected, failed, cancelled
	job.UpdatedAt, job.CompletedAt = now, completedAt
	return job, nil
}

// timePtr 返回独立时间副本，避免把数据库扫描变量地址泄露到后续逻辑。
func timePtr(value time.Time) *time.Time {
	value = value.UTC()
	return &value
}

// nullableTime 把可空时间转换为 SQL NULL。
func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
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

// nullableSnapshotPointer 将明细可选快照指针转换为 SQL NULL，避免伪造零值主键。
func nullableSnapshotPointer(id *uint64) any {
	if id == nil || *id == 0 {
		return nil
	}
	return *id
}
