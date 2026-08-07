package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// PathConfigurationRepository 是基于同一计划数据库的路径配置仓储实现。
type PathConfigurationRepository struct {
	db *sql.DB
}

// NewPathConfigurationRepository 创建路径配置仓储。
func NewPathConfigurationRepository(db *sql.DB) *PathConfigurationRepository {
	return &PathConfigurationRepository{db: db}
}

// FindByPath 读取指定路径的当前配置；未保存时返回 false，不把空记录误当配置。
func (r *PathConfigurationRepository) FindByPath(ctx context.Context, pathID uint64) (model.StoredPathConfig, bool, error) {
	return scanStoredPathConfig(r.db.QueryRowContext(ctx, "SELECT path_id, revision, idempotency_key, config_status, field_values, action_values, created_at, updated_at FROM test_execution_path_configs WHERE path_id = ?", pathID))
}

// FindByPathAndKey 只在指定路径内按幂等键读取已保存结果，避免跨路径键碰撞。
func (r *PathConfigurationRepository) FindByPathAndKey(ctx context.Context, pathID uint64, idempotencyKey string) (model.StoredPathConfig, bool, error) {
	return scanStoredPathConfig(r.db.QueryRowContext(ctx, "SELECT path_id, revision, idempotency_key, config_status, field_values, action_values, created_at, updated_at FROM test_execution_path_configs WHERE path_id = ? AND idempotency_key = ?", pathID, idempotencyKey))
}

// Save 在事务中锁定路径配置行，校验期望修订号后整份替换字段值与动作值并推进修订号。
// 期望修订号与当前记录不一致时返回 ErrPathConfigConflict，避免并发编辑互相覆盖。
func (r *PathConfigurationRepository) Save(ctx context.Context, record model.StoredPathConfig, expectedRevision uint64, now time.Time) (model.StoredPathConfig, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.StoredPathConfig{}, err
	}
	defer tx.Rollback()
	var currentRevision uint64
	exists := true
	if err := tx.QueryRowContext(ctx, "SELECT revision FROM test_execution_path_configs WHERE path_id = ? FOR UPDATE", record.PathID).Scan(&currentRevision); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return model.StoredPathConfig{}, err
		}
		exists = false
	}
	if exists && currentRevision != expectedRevision {
		// 行锁内二次核对修订号：服务层读取到保存之间的并发保存必须在此处被拒绝。
		return model.StoredPathConfig{}, repository.ErrPathConfigConflict
	}
	if !exists && expectedRevision != 0 {
		// 浏览器基于旧版本保存而目标记录已被删除或从未存在，不能静默建立新版本。
		return model.StoredPathConfig{}, repository.ErrPathConfigConflict
	}
	fieldJSON, err := encodePathConfigValues(record.FieldValues)
	if err != nil {
		return model.StoredPathConfig{}, err
	}
	actionJSON, err := encodeStringMap(record.ActionValues)
	if err != nil {
		return model.StoredPathConfig{}, err
	}
	if !exists {
		_, err = tx.ExecContext(ctx, "INSERT INTO test_execution_path_configs (path_id, revision, idempotency_key, config_status, field_values, action_values, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			record.PathID, record.Revision, record.IdempotencyKey, record.Status, fieldJSON, actionJSON, now.UTC(), now.UTC())
		if isDuplicateKeyError(err) {
			// 并发首次同键保存：另一请求已提交同一幂等键。读取胜出记录并直接返回，
			// 保证同键重试得到同一修订号，而不是把并发竞态误报为存储错误。
			existing, found, scanErr := scanStoredPathConfig(tx.QueryRowContext(ctx, "SELECT path_id, revision, idempotency_key, config_status, field_values, action_values, created_at, updated_at FROM test_execution_path_configs WHERE path_id = ? FOR UPDATE", record.PathID))
			if scanErr != nil {
				return model.StoredPathConfig{}, scanErr
			}
			if !found || existing.IdempotencyKey != record.IdempotencyKey {
				// 不同键抢占了同一路径行属于修订冲突，不能把其他保存结果当作本次幂等结果。
				return model.StoredPathConfig{}, repository.ErrPathConfigConflict
			}
			if err := tx.Commit(); err != nil {
				return model.StoredPathConfig{}, err
			}
			return existing, nil
		}
	} else {
		_, err = tx.ExecContext(ctx, "UPDATE test_execution_path_configs SET revision = ?, idempotency_key = ?, config_status = ?, field_values = ?, action_values = ?, updated_at = ? WHERE path_id = ?",
			record.Revision, record.IdempotencyKey, record.Status, fieldJSON, actionJSON, now.UTC(), record.PathID)
	}
	if err != nil {
		return model.StoredPathConfig{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.StoredPathConfig{}, err
	}
	record.CreatedAt = now.UTC()
	record.UpdatedAt = now.UTC()
	return record, nil
}

// isDuplicateKeyError 判断 MySQL 唯一键冲突错误，用于并发同键首次保存兜底。
func isDuplicateKeyError(err error) bool {
	var mysqlErr *mysqldriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

// scanStoredPathConfig 解析配置行并统一处理未找到与 JSON 数据损坏。
func scanStoredPathConfig(row *sql.Row) (model.StoredPathConfig, bool, error) {
	var record model.StoredPathConfig
	var fieldJSON string
	var actionJSON string
	err := row.Scan(&record.PathID, &record.Revision, &record.IdempotencyKey, &record.Status, &fieldJSON, &actionJSON, &record.CreatedAt, &record.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.StoredPathConfig{}, false, nil
	}
	if err != nil {
		return model.StoredPathConfig{}, false, err
	}
	record.FieldValues, err = decodePathConfigValues(fieldJSON)
	if err != nil {
		return model.StoredPathConfig{}, false, repository.ErrPathConfigDataInvalid
	}
	record.ActionValues, err = decodeStringMap(actionJSON)
	if err != nil {
		return model.StoredPathConfig{}, false, repository.ErrPathConfigDataInvalid
	}
	return record, true, nil
}

// encodePathConfigValues 把节点-字段-值两层映射编码为稳定 JSON。
func encodePathConfigValues(values map[string]map[string]string) (string, error) {
	if values == nil {
		values = map[string]map[string]string{}
	}
	data, err := json.Marshal(values)
	return string(data), err
}

// decodePathConfigValues 把配置 JSON 解码为节点-字段-值两层映射。
func decodePathConfigValues(raw string) (map[string]map[string]string, error) {
	result := make(map[string]map[string]string)
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// encodeStringMap 把节点-动作映射编码为稳定 JSON。
func encodeStringMap(values map[string]string) (string, error) {
	if values == nil {
		values = map[string]string{}
	}
	data, err := json.Marshal(values)
	return string(data), err
}

// decodeStringMap 把节点-动作 JSON 解码为普通字符串映射。
func decodeStringMap(raw string) (map[string]string, error) {
	result := make(map[string]string)
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, err
	}
	return result, nil
}
