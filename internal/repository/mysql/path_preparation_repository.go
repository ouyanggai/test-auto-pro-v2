package mysql

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	driver "github.com/go-sql-driver/mysql"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// PathPreparationRepository 是独立批量准备任务的 MySQL 检查点仓储。
type PathPreparationRepository struct {
	db *sql.DB
}

// NewPathPreparationRepository 创建批量准备任务仓储。
func NewPathPreparationRepository(db *sql.DB) *PathPreparationRepository {
	return &PathPreparationRepository{db: db}
}

// Create 按幂等键或同计划活动任务返回既有任务，否则快照当前勾选路径建立明细。
func (r *PathPreparationRepository) Create(ctx context.Context, planID uint64, createKey string, now time.Time) (model.PathPreparationJob, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.PathPreparationJob{}, false, err
	}
	defer tx.Rollback()
	// 锁定计划行后再判断活动任务，使同一计划的并发创建在数据库事务内严格串行。
	var lockedPlanID uint64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM test_plans WHERE id = ? FOR UPDATE", planID).Scan(&lockedPlanID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.PathPreparationJob{}, false, repository.ErrPathPreparationNotFound
		}
		return model.PathPreparationJob{}, false, err
	}
	if job, found, findErr := findPathPreparationByCreateKey(ctx, tx, createKey); findErr != nil {
		return model.PathPreparationJob{}, false, findErr
	} else if found {
		if job.PlanID != planID {
			return model.PathPreparationJob{}, false, repository.ErrPathPreparationState
		}
		return job, false, tx.Commit()
	}
	if job, found, findErr := findActivePathPreparation(ctx, tx, planID); findErr != nil {
		return model.PathPreparationJob{}, false, findErr
	} else if found {
		return job, false, tx.Commit()
	}
	job := model.PathPreparationJob{ID: createKey, PlanID: planID, Status: "queued", CreatedAt: now.UTC(), UpdatedAt: now.UTC()}
	_, err = tx.ExecContext(ctx, `INSERT INTO test_path_preparation_jobs
(id, plan_id, create_key, status, total_count, created_at, updated_at)
VALUES (?, ?, ?, 'queued', 0, ?, ?)`, job.ID, planID, createKey, now.UTC(), now.UTC())
	if err != nil {
		var mysqlErr *driver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			// 极小概率的跨计划幂等键碰撞需要结束当前快照后再读取胜出的任务。
			_ = tx.Rollback()
			if existing, found, findErr := findPathPreparationByCreateKey(ctx, r.db, createKey); findErr == nil && found {
				if existing.PlanID != planID {
					return model.PathPreparationJob{}, false, repository.ErrPathPreparationState
				}
				return existing, false, nil
			}
		}
		return model.PathPreparationJob{}, false, err
	}
	// 由数据库直接快照勾选路径，创建阶段不会把数百条路径累积到 Go 堆内存。
	inserted, err := tx.ExecContext(ctx, `INSERT INTO test_path_preparation_items
(job_id, path_id, sequence_no, path_name, status, updated_at)
SELECT ?, path.id, path.sequence_no, path.name, 'pending', ?
FROM test_execution_paths path
JOIN test_execution_path_configs config ON config.path_id = path.id
WHERE path.plan_id = ?
  AND COALESCE(JSON_UNQUOTE(JSON_EXTRACT(config.action_values, '$."f008:test-included"')) = 'true', FALSE)
ORDER BY path.sequence_no`, job.ID, now.UTC(), planID)
	if err != nil {
		return model.PathPreparationJob{}, false, err
	}
	count, err := inserted.RowsAffected()
	if err != nil {
		return model.PathPreparationJob{}, false, err
	}
	if count == 0 {
		return model.PathPreparationJob{}, false, repository.ErrPathPreparationEmpty
	}
	job.Total = int(count)
	if _, err := tx.ExecContext(ctx, "UPDATE test_path_preparation_jobs SET total_count = ? WHERE id = ?", job.Total, job.ID); err != nil {
		return model.PathPreparationJob{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.PathPreparationJob{}, false, err
	}
	return job, true, nil
}

// Get 按计划归属读取任务真实计数。
func (r *PathPreparationRepository) Get(ctx context.Context, planID uint64, jobID string) (model.PathPreparationJob, error) {
	job, err := scanPathPreparationJob(r.db.QueryRowContext(ctx, pathPreparationJobSelect+" WHERE plan_id = ? AND id = ?", planID, jobID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.PathPreparationJob{}, repository.ErrPathPreparationNotFound
	}
	return job, err
}

// FindActive 返回同计划排队或运行中的任务。
func (r *PathPreparationRepository) FindActive(ctx context.Context, planID uint64) (model.PathPreparationJob, bool, error) {
	return findActivePathPreparation(ctx, r.db, planID)
}

// ListRecoverable 返回服务重启后需要继续的持久化任务。
func (r *PathPreparationRepository) ListRecoverable(ctx context.Context) ([]model.PathPreparationJob, error) {
	rows, err := r.db.QueryContext(ctx, pathPreparationJobSelect+" WHERE status IN ('queued', 'running') ORDER BY created_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]model.PathPreparationJob, 0)
	for rows.Next() {
		job, scanErr := scanPathPreparationJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, job)
	}
	return result, rows.Err()
}

// Start 把排队任务置为运行中，并在进程恢复时把遗留 running 明细退回待处理检查点。
func (r *PathPreparationRepository) Start(ctx context.Context, planID uint64, jobID string, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status string
	if err := tx.QueryRowContext(ctx, "SELECT status FROM test_path_preparation_jobs WHERE plan_id = ? AND id = ? FOR UPDATE", planID, jobID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.ErrPathPreparationNotFound
		}
		return err
	}
	if status != "queued" && status != "running" {
		return repository.ErrPathPreparationState
	}
	if status == "running" {
		if _, err := tx.ExecContext(ctx, "UPDATE test_path_preparation_items SET status = 'pending', updated_at = ? WHERE job_id = ? AND status = 'running'", now.UTC(), jobID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE test_path_preparation_jobs SET status = 'running', failure_reason = '', updated_at = ? WHERE plan_id = ? AND id = ?", now.UTC(), planID, jobID); err != nil {
		return err
	}
	return tx.Commit()
}

// ClaimBatch 锁定并领取一批待处理明细，批次大小由服务端资源边界控制。
func (r *PathPreparationRepository) ClaimBatch(ctx context.Context, planID uint64, jobID string, limit int, now time.Time) ([]model.PathPreparationItem, error) {
	if limit < 1 || limit > 100 {
		return nil, repository.ErrPathPreparationState
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var status string
	if err := tx.QueryRowContext(ctx, "SELECT status FROM test_path_preparation_jobs WHERE plan_id = ? AND id = ? FOR UPDATE", planID, jobID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrPathPreparationNotFound
		}
		return nil, err
	}
	if status != "running" {
		return []model.PathPreparationItem{}, nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, job_id, path_id, sequence_no, path_name, status, reason,
node_configured, data_generated, needs_attention, preserved_manual
FROM test_path_preparation_items WHERE job_id = ? AND status = 'pending' ORDER BY id LIMIT ? FOR UPDATE`, jobID, limit)
	if err != nil {
		return nil, err
	}
	items := make([]model.PathPreparationItem, 0, limit)
	for rows.Next() {
		item, scanErr := scanPathPreparationItem(rows)
		if scanErr != nil {
			rows.Close()
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, "UPDATE test_path_preparation_items SET status = 'running', updated_at = ? WHERE id = ?", now.UTC(), item.ID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	for index := range items {
		items[index].Status = "running"
	}
	return items, nil
}

// SetCurrent 在真正处理单条路径前持久化当前路径；已取消任务拒绝继续执行。
func (r *PathPreparationRepository) SetCurrent(ctx context.Context, planID uint64, jobID string, item model.PathPreparationItem, now time.Time) error {
	if item.PathID == 0 || item.SequenceNo == 0 || strings.TrimSpace(item.PathName) == "" {
		return repository.ErrPathPreparationState
	}
	result, err := r.db.ExecContext(ctx, `UPDATE test_path_preparation_jobs
SET current_path_id = ?, current_sequence_no = ?, current_path_name = ?, current_item_status = 'running', updated_at = ?
WHERE plan_id = ? AND id = ? AND status = 'running'`, item.PathID, item.SequenceNo, item.PathName, now.UTC(), planID, jobID)
	if err != nil {
		return err
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return repository.ErrPathPreparationState
	}
	return nil
}

// CompleteItem 原子提交单条终态并累加任务真实计数；取消后迟到结果不会覆盖检查点。
func (r *PathPreparationRepository) CompleteItem(ctx context.Context, planID uint64, jobID string, itemID uint64, outcome model.PathPreparationItemResult, now time.Time) error {
	if !validPathPreparationItemStatus(outcome.Status) {
		return repository.ErrPathPreparationState
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var jobStatus string
	if err := tx.QueryRowContext(ctx, "SELECT status FROM test_path_preparation_jobs WHERE plan_id = ? AND id = ? FOR UPDATE", planID, jobID).Scan(&jobStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.ErrPathPreparationNotFound
		}
		return err
	}
	if jobStatus != "running" {
		return nil
	}
	var itemStatus, pathName string
	var pathID uint64
	var sequenceNo uint
	if err := tx.QueryRowContext(ctx, `SELECT status, path_id, sequence_no, path_name
FROM test_path_preparation_items WHERE id = ? AND job_id = ? FOR UPDATE`, itemID, jobID).Scan(&itemStatus, &pathID, &sequenceNo, &pathName); err != nil {
		return err
	}
	if itemStatus != "running" {
		return tx.Commit()
	}
	reason := strings.TrimSpace(outcome.Reason)
	if len([]rune(reason)) > 500 {
		reason = string([]rune(reason)[:500])
	}
	_, err = tx.ExecContext(ctx, `UPDATE test_path_preparation_items SET status = ?, reason = ?, node_configured = ?,
data_generated = ?, needs_attention = ?, preserved_manual = ?, updated_at = ? WHERE id = ?`,
		outcome.Status, reason, outcome.NodeConfigured, outcome.DataGenerated, outcome.NeedsAttention, outcome.PreservedManual, now.UTC(), itemID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE test_path_preparation_jobs SET processed_count = processed_count + 1,
node_configured_count = node_configured_count + ?, data_generated_count = data_generated_count + ?,
needs_attention_count = needs_attention_count + ?, failed_count = failed_count + ?,
	preserved_manual_count = preserved_manual_count + ?, current_path_id = ?, current_sequence_no = ?,
	current_path_name = ?, current_item_status = ?, updated_at = ? WHERE id = ?`,
		boolCount(outcome.NodeConfigured), boolCount(outcome.DataGenerated), boolCount(outcome.NeedsAttention),
		boolCount(outcome.Status == "failed"), boolCount(outcome.PreservedManual), pathID, sequenceNo, pathName,
		outcome.Status, now.UTC(), jobID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Finish 在没有待处理或运行中明细时完成任务，否则保持运行态。
func (r *PathPreparationRepository) Finish(ctx context.Context, planID uint64, jobID string, now time.Time) (model.PathPreparationJob, error) {
	var remaining int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM test_path_preparation_items item
JOIN test_path_preparation_jobs job ON job.id = item.job_id
WHERE job.plan_id = ? AND job.id = ? AND item.status IN ('pending', 'running')`, planID, jobID).Scan(&remaining); err != nil {
		return model.PathPreparationJob{}, err
	}
	if remaining == 0 {
		if _, err := r.db.ExecContext(ctx, "UPDATE test_path_preparation_jobs SET status = 'completed', completed_at = ?, updated_at = ? WHERE plan_id = ? AND id = ? AND status = 'running'", now.UTC(), now.UTC(), planID, jobID); err != nil {
			return model.PathPreparationJob{}, err
		}
	}
	return r.Get(ctx, planID, jobID)
}

// Cancel 取消活动任务并把已领取但未完成明细退回检查点。
func (r *PathPreparationRepository) Cancel(ctx context.Context, planID uint64, jobID string, now time.Time) (model.PathPreparationJob, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.PathPreparationJob{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE test_path_preparation_jobs
SET status = 'cancelled', current_item_status = IF(current_path_id IS NULL, '', 'pending'), updated_at = ?
WHERE plan_id = ? AND id = ? AND status IN ('queued', 'running')`, now.UTC(), planID, jobID)
	if err != nil {
		return model.PathPreparationJob{}, err
	}
	if changed, _ := result.RowsAffected(); changed > 0 {
		if _, err := tx.ExecContext(ctx, "UPDATE test_path_preparation_items SET status = 'pending', updated_at = ? WHERE job_id = ? AND status = 'running'", now.UTC(), jobID); err != nil {
			return model.PathPreparationJob{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.PathPreparationJob{}, err
	}
	return r.Get(ctx, planID, jobID)
}

// Resume 恢复已取消或失败任务，并从未完成明细继续。
func (r *PathPreparationRepository) Resume(ctx context.Context, planID uint64, jobID string, now time.Time) (model.PathPreparationJob, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.PathPreparationJob{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE test_path_preparation_jobs
SET status = 'queued', failure_reason = '', completed_at = NULL,
    current_item_status = IF(current_path_id IS NULL, '', 'pending'), updated_at = ?
WHERE plan_id = ? AND id = ? AND status IN ('cancelled', 'failed')`, now.UTC(), planID, jobID)
	if err != nil {
		return model.PathPreparationJob{}, err
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return model.PathPreparationJob{}, repository.ErrPathPreparationState
	}
	if _, err := tx.ExecContext(ctx, "UPDATE test_path_preparation_items SET status = 'pending', updated_at = ? WHERE job_id = ? AND status = 'running'", now.UTC(), jobID); err != nil {
		return model.PathPreparationJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.PathPreparationJob{}, err
	}
	return r.Get(ctx, planID, jobID)
}

// Fail 记录任务级基础读取失败并保留未完成明细供恢复。
func (r *PathPreparationRepository) Fail(ctx context.Context, planID uint64, jobID, reason string, now time.Time) error {
	reason = strings.TrimSpace(reason)
	if len([]rune(reason)) > 500 {
		reason = string([]rune(reason)[:500])
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE test_path_preparation_jobs
SET status = 'failed', failure_reason = ?, current_item_status = IF(current_path_id IS NULL, '', 'pending'), updated_at = ?
WHERE plan_id = ? AND id = ? AND status IN ('queued', 'running')`, reason, now.UTC(), planID, jobID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE test_path_preparation_items SET status = 'pending', updated_at = ? WHERE job_id = ? AND status = 'running'", now.UTC(), jobID); err != nil {
		return err
	}
	return tx.Commit()
}

// ListItems 按稳定明细 ID 游标分页返回中文结果。
func (r *PathPreparationRepository) ListItems(ctx context.Context, planID uint64, jobID string, cursor uint64, limit int) (model.PathPreparationItemPage, error) {
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if _, err := r.Get(ctx, planID, jobID); err != nil {
		return model.PathPreparationItemPage{}, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, job_id, path_id, sequence_no, path_name, status, reason,
node_configured, data_generated, needs_attention, preserved_manual
FROM test_path_preparation_items WHERE job_id = ? AND id > ? ORDER BY id LIMIT ?`, jobID, cursor, limit+1)
	if err != nil {
		return model.PathPreparationItemPage{}, err
	}
	defer rows.Close()
	items := make([]model.PathPreparationItem, 0, limit+1)
	for rows.Next() {
		item, scanErr := scanPathPreparationItem(rows)
		if scanErr != nil {
			return model.PathPreparationItemPage{}, scanErr
		}
		items = append(items, item)
	}
	page := model.PathPreparationItemPage{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		page.NextCursor = items[limit-1].ID
	}
	return page, rows.Err()
}

const pathPreparationJobSelect = `SELECT id, plan_id, status, total_count, processed_count,
	node_configured_count, data_generated_count, needs_attention_count, failed_count,
	preserved_manual_count, current_path_id, current_sequence_no, current_path_name, current_item_status,
	failure_reason, created_at, updated_at, completed_at
FROM test_path_preparation_jobs`

type pathPreparationScanner interface {
	Scan(...any) error
}

type pathPreparationQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// scanPathPreparationJob 解析任务计数并统一 UTC 时间。
func scanPathPreparationJob(scanner pathPreparationScanner) (model.PathPreparationJob, error) {
	var job model.PathPreparationJob
	var completed sql.NullTime
	var currentPathID, currentSequenceNo sql.NullInt64
	var currentPathName, currentItemStatus string
	err := scanner.Scan(&job.ID, &job.PlanID, &job.Status, &job.Total, &job.Processed, &job.NodeConfigured,
		&job.DataGenerated, &job.NeedsAttention, &job.Failed, &job.PreservedManual,
		&currentPathID, &currentSequenceNo, &currentPathName, &currentItemStatus, &job.Error,
		&job.CreatedAt, &job.UpdatedAt, &completed)
	if err != nil {
		return model.PathPreparationJob{}, err
	}
	job.CreatedAt, job.UpdatedAt = job.CreatedAt.UTC(), job.UpdatedAt.UTC()
	if currentPathID.Valid && currentSequenceNo.Valid {
		job.CurrentPath = &model.PathPreparationCurrentPath{
			PathID: uint64(currentPathID.Int64), SequenceNo: uint(currentSequenceNo.Int64),
			PathName: currentPathName, Status: currentItemStatus,
		}
	}
	if completed.Valid {
		value := completed.Time.UTC()
		job.CompletedAt = &value
	}
	return job, nil
}

// scanPathPreparationItem 解析单条任务明细。
func scanPathPreparationItem(scanner pathPreparationScanner) (model.PathPreparationItem, error) {
	var item model.PathPreparationItem
	err := scanner.Scan(&item.ID, &item.JobID, &item.PathID, &item.SequenceNo, &item.PathName, &item.Status,
		&item.Reason, &item.NodeConfigured, &item.DataGenerated, &item.NeedsAttention, &item.PreservedManual)
	return item, err
}

// findPathPreparationByCreateKey 在同一事务中读取幂等任务。
func findPathPreparationByCreateKey(ctx context.Context, queryer pathPreparationQueryer, createKey string) (model.PathPreparationJob, bool, error) {
	job, err := scanPathPreparationJob(queryer.QueryRowContext(ctx, pathPreparationJobSelect+" WHERE create_key = ?", createKey))
	if errors.Is(err, sql.ErrNoRows) {
		return model.PathPreparationJob{}, false, nil
	}
	return job, err == nil, err
}

// findActivePathPreparation 返回同计划唯一活动任务。
func findActivePathPreparation(ctx context.Context, queryer pathPreparationQueryer, planID uint64) (model.PathPreparationJob, bool, error) {
	job, err := scanPathPreparationJob(queryer.QueryRowContext(ctx, pathPreparationJobSelect+" WHERE plan_id = ? AND status IN ('queued', 'running') ORDER BY created_at LIMIT 1", planID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.PathPreparationJob{}, false, nil
	}
	return job, err == nil, err
}

// validPathPreparationItemStatus 限制 Worker 只能提交批准的终态。
func validPathPreparationItemStatus(status string) bool {
	return status == "completed" || status == "needs_attention" || status == "failed"
}

// boolCount 把布尔结果转换为数据库计数增量。
func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}
