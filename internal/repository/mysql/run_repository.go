package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	driver "github.com/go-sql-driver/mysql"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// runNumberRetryAttempts 是运行号分配在并发撞键时的重试上限。
// 运行号由 (plan_id, run_no) 唯一键兜底：读 MAX 后插入，撞键即重读重试，
// 本工具单用户场景下极少发生，3 次足够。
const runNumberRetryAttempts = 3

// RunRepository 是 RunStore 的 MySQL 实现：事实表只 INSERT，
// 聚合表状态列单向前进，且每次状态更新与事件行在同一事务内提交。
type RunRepository struct {
	db  *sql.DB
	now func() time.Time
}

// NewRunRepository 创建运行记录仓储；now 用于统一落库时间基准（UTC）。
func NewRunRepository(db *sql.DB) *RunRepository {
	return &RunRepository{db: db, now: func() time.Time { return time.Now().UTC() }}
}

// CreateRun 创建一次运行（等待运行）与它的唯一路径运行（等待运行），并在计划内单调递增分配运行号。
// 运行号、运行、路径运行与两条创建事件在同一事务内落库，保证“运行怎么来的”可回放。
func (r *RunRepository) CreateRun(ctx context.Context, planID uint64, executionPathID uint64, mode model.RunMode, trigger model.RunTriggerKind, maxConcurrency *int, now time.Time) (model.Run, model.PathRun, error) {
	now = now.UTC()
	for attempt := 0; attempt < runNumberRetryAttempts; attempt++ {
		run, pathRun, err := r.tryCreateRun(ctx, planID, executionPathID, mode, trigger, maxConcurrency, now)
		if err == nil {
			return run, pathRun, nil
		}
		if !isDuplicateKeyError(err) {
			return model.Run{}, model.PathRun{}, err
		}
	}
	return model.Run{}, model.PathRun{}, fmt.Errorf("运行号分配在并发冲突下重试 %d 次仍未成功", runNumberRetryAttempts)
}

// tryCreateRun 执行一次运行号分配与创建事务；计划内运行号取当前最大值加一。
func (r *RunRepository) tryCreateRun(ctx context.Context, planID uint64, executionPathID uint64, mode model.RunMode, trigger model.RunTriggerKind, maxConcurrency *int, now time.Time) (model.Run, model.PathRun, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Run{}, model.PathRun{}, err
	}
	defer tx.Rollback()

	var nextRunNo uint64
	if err := tx.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(run_no), 0) + 1 FROM runs WHERE plan_id = ?", planID).Scan(&nextRunNo); err != nil {
		return model.Run{}, model.PathRun{}, err
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO runs (plan_id, run_no, mode, trigger_kind, max_concurrency, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, planID, nextRunNo, string(mode), string(trigger), maxConcurrency, string(model.RunStatusPending), now, now)
	if err != nil {
		return model.Run{}, model.PathRun{}, err
	}
	runID, err := result.LastInsertId()
	if err != nil {
		return model.Run{}, model.PathRun{}, err
	}
	if err := appendRunEvent(ctx, tx, model.RunEvent{
		RunID:  uint64(runID),
		Kind:   "run_created",
		Label:  fmt.Sprintf("运行 %d 已创建（%s）", nextRunNo, model.RunModeName(mode)),
	}, now); err != nil {
		return model.Run{}, model.PathRun{}, err
	}

	pathResult, err := tx.ExecContext(ctx, `
		INSERT INTO path_runs (run_id, execution_path_id, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, runID, executionPathID, string(model.PathRunStatusWaiting), now, now)
	if err != nil {
		return model.Run{}, model.PathRun{}, err
	}
	pathRunID, err := pathResult.LastInsertId()
	if err != nil {
		return model.Run{}, model.PathRun{}, err
	}
	pathRunIDValue := uint64(pathRunID)
	if err := appendRunEvent(ctx, tx, model.RunEvent{
		RunID:     uint64(runID),
		PathRunID: &pathRunIDValue,
		Kind:      "path_run_created",
		Label:     "路径运行已创建，等待运行",
	}, now); err != nil {
		return model.Run{}, model.PathRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Run{}, model.PathRun{}, err
	}

	run, err := r.GetRun(ctx, uint64(runID))
	if err != nil {
		return model.Run{}, model.PathRun{}, err
	}
	pathRun, err := r.GetPathRun(ctx, uint64(pathRunID))
	if err != nil {
		return model.Run{}, model.PathRun{}, err
	}
	return run, pathRun, nil
}

// appendRunEvent 在给定事务内追加一行运行事件；只被聚合状态变更调用，保证事件与状态同事务。
func appendRunEvent(ctx context.Context, tx *sql.Tx, event model.RunEvent, now time.Time) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO run_events (run_id, path_run_id, kind, label, detail, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, event.RunID, nullableUint64(event.PathRunID), event.Kind, event.Label, emptyToNull(event.Detail), now)
	return err
}

// nullableUint64 把可空路径运行 ID 转为 database/sql 可识别的形态。
func nullableUint64(value *uint64) any {
	if value == nil {
		return nil
	}
	return *value
}

// emptyToNull 把空字符串细节转为 NULL，避免事实表里出现空串噪音。
func emptyToNull(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// GetRun 读取运行聚合，找不到返回 ErrRunNotFound。
func (r *RunRepository) GetRun(ctx context.Context, runID uint64) (model.Run, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, plan_id, run_no, mode, trigger_kind, max_concurrency, status, result, started_at, finished_at, created_at, updated_at
		FROM runs WHERE id = ?
	`, runID)
	run, err := scanRun(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Run{}, repository.ErrRunNotFound
		}
		return model.Run{}, err
	}
	return run, nil
}

// GetPathRun 读取路径运行聚合，找不到返回 ErrRunNotFound。
func (r *RunRepository) GetPathRun(ctx context.Context, pathRunID uint64) (model.PathRun, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, run_id, execution_path_id, status, result, failure_class, main_instance_ref,
		       final_target_summary, lease_owner, lease_expires_at, fencing_token,
		       started_at, finished_at, created_at, updated_at
		FROM path_runs WHERE id = ?
	`, pathRunID)
	pathRun, err := scanPathRun(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.PathRun{}, repository.ErrRunNotFound
		}
		return model.PathRun{}, err
	}
	return pathRun, nil
}

// GetPathRunByRun 读取一次运行下的唯一路径运行（本切片一次运行只跑一条路径）。
func (r *RunRepository) GetPathRunByRun(ctx context.Context, runID uint64) (model.PathRun, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, run_id, execution_path_id, status, result, failure_class, main_instance_ref,
		       final_target_summary, lease_owner, lease_expires_at, fencing_token,
		       started_at, finished_at, created_at, updated_at
		FROM path_runs WHERE run_id = ? ORDER BY id LIMIT 1
	`, runID)
	pathRun, err := scanPathRun(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.PathRun{}, repository.ErrRunNotFound
		}
		return model.PathRun{}, err
	}
	return pathRun, nil
}

// ListRunsByPlan 按计划列出运行，运行号倒序（最新在前）。
func (r *RunRepository) ListRunsByPlan(ctx context.Context, planID uint64, limit int) ([]model.Run, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, plan_id, run_no, mode, trigger_kind, max_concurrency, status, result, started_at, finished_at, created_at, updated_at
		FROM runs WHERE plan_id = ? ORDER BY run_no DESC LIMIT ?
	`, planID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := []model.Run{}
	for rows.Next() {
		run, err := scanRun(rows.Scan)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// AdvanceRunStatus 校验运行状态迁移后与事件行同事务落库；非法迁移返回 ErrRunStatusConflict 且不落任何行。
func (r *RunRepository) AdvanceRunStatus(ctx context.Context, runID uint64, from, to model.RunStatus, event model.RunEvent, now time.Time) (model.Run, error) {
	now = now.UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Run{}, err
	}
	defer tx.Rollback()

	var current string
	err = tx.QueryRowContext(ctx, "SELECT status FROM runs WHERE id = ? FOR UPDATE", runID).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Run{}, repository.ErrRunNotFound
	}
	if err != nil {
		return model.Run{}, err
	}
	if model.RunStatus(current) != from || !canAdvanceRunStatus(model.RunStatus(current), to) {
		return model.Run{}, fmt.Errorf("%w：%s -> %s", repository.ErrRunStatusConflict, model.RunStatusName(model.RunStatus(current)), model.RunStatusName(to))
	}
	finishedAt := any(nil)
	if isTerminalRunStatus(to) {
		finishedAt = now
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE runs SET status = ?, finished_at = COALESCE(?, finished_at), updated_at = ? WHERE id = ?
	`, string(to), finishedAt, now, runID); err != nil {
		return model.Run{}, err
	}
	event.RunID = runID
	if event.Kind == "" {
		event.Kind = "run_status_changed"
	}
	if event.Label == "" {
		event.Label = fmt.Sprintf("运行状态：%s -> %s", model.RunStatusName(from), model.RunStatusName(to))
	}
	if err := appendRunEvent(ctx, tx, event, now); err != nil {
		return model.Run{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Run{}, err
	}
	return r.GetRun(ctx, runID)
}

// canAdvanceRunStatus 判断运行聚合的状态迁移是否合法：等待运行 -> 运行中 -> 终态，终态无出边。
func canAdvanceRunStatus(from, to model.RunStatus) bool {
	switch from {
	case model.RunStatusPending:
		return to == model.RunStatusRunning || to == model.RunStatusCancelled
	case model.RunStatusRunning:
		switch to {
		case model.RunStatusCompleted, model.RunStatusFailed, model.RunStatusStopped, model.RunStatusCancelled:
			return true
		}
		return false
	default:
		return false
	}
}

// isTerminalRunStatus 判断运行聚合是否处于终态。
func isTerminalRunStatus(status model.RunStatus) bool {
	switch status {
	case model.RunStatusCompleted, model.RunStatusFailed, model.RunStatusStopped, model.RunStatusCancelled:
		return true
	default:
		return false
	}
}

// AdvancePathRunStatus 校验路径运行状态迁移后与事件行同事务落库；非法迁移返回 ErrRunStatusConflict 且不落任何行。
// 首次进入运行中时补记 started_at；核验中 -> 运行中是步骤循环的前进，不是状态回退。
func (r *RunRepository) AdvancePathRunStatus(ctx context.Context, pathRunID uint64, from, to model.PathRunStatus, event model.RunEvent, now time.Time) (model.PathRun, error) {
	now = now.UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.PathRun{}, err
	}
	defer tx.Rollback()

	var runID uint64
	var current string
	err = tx.QueryRowContext(ctx, "SELECT run_id, status FROM path_runs WHERE id = ? FOR UPDATE", pathRunID).Scan(&runID, &current)
	if errors.Is(err, sql.ErrNoRows) {
		return model.PathRun{}, repository.ErrRunNotFound
	}
	if err != nil {
		return model.PathRun{}, err
	}
	if model.PathRunStatus(current) != from || !model.CanAdvancePathRunStatus(model.PathRunStatus(current), to) {
		return model.PathRun{}, fmt.Errorf("%w：%s -> %s", repository.ErrRunStatusConflict, model.PathRunStatusName(model.PathRunStatus(current)), model.PathRunStatusName(to))
	}
	startedAt := any(nil)
	if to == model.PathRunStatusRunning {
		startedAt = now
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE path_runs SET status = ?, started_at = COALESCE(started_at, ?), updated_at = ? WHERE id = ?
	`, string(to), startedAt, now, pathRunID); err != nil {
		return model.PathRun{}, err
	}
	event.RunID = runID
	event.PathRunID = &pathRunID
	if event.Kind == "" {
		event.Kind = "path_run_status_changed"
	}
	if event.Label == "" {
		event.Label = fmt.Sprintf("路径运行状态：%s -> %s", model.PathRunStatusName(from), model.PathRunStatusName(to))
	}
	if err := appendRunEvent(ctx, tx, event, now); err != nil {
		return model.PathRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.PathRun{}, err
	}
	return r.GetPathRun(ctx, pathRunID)
}

// FinishPathRun 把路径运行推进到终态，落路径结果与失败分类，并镜像收尾运行聚合。
// 待对账是路径运行终态但不是运行终态：运行保持运行中，留给对账切片（F-018）唯一合法恢复。
func (r *RunRepository) FinishPathRun(ctx context.Context, pathRunID uint64, to model.PathRunStatus, result *model.RunResult, failureClass *model.FailureClass, event model.RunEvent, now time.Time) (model.PathRun, error) {
	if !model.IsTerminalPathRunStatus(to) {
		return model.PathRun{}, fmt.Errorf("%w：%s 不是路径运行终态", repository.ErrRunStatusConflict, model.PathRunStatusName(to))
	}
	now = now.UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.PathRun{}, err
	}
	defer tx.Rollback()

	var runID uint64
	var current string
	err = tx.QueryRowContext(ctx, "SELECT run_id, status FROM path_runs WHERE id = ? FOR UPDATE", pathRunID).Scan(&runID, &current)
	if errors.Is(err, sql.ErrNoRows) {
		return model.PathRun{}, repository.ErrRunNotFound
	}
	if err != nil {
		return model.PathRun{}, err
	}
	if !model.CanAdvancePathRunStatus(model.PathRunStatus(current), to) {
		return model.PathRun{}, fmt.Errorf("%w：%s -> %s", repository.ErrRunStatusConflict, model.PathRunStatusName(model.PathRunStatus(current)), model.PathRunStatusName(to))
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE path_runs
		SET status = ?, result = ?, failure_class = ?, finished_at = ?, lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
		WHERE id = ?
	`, string(to), nullableRunResult(result), nullableFailureClass(failureClass), now, now, pathRunID); err != nil {
		return model.PathRun{}, err
	}
	event.RunID = runID
	event.PathRunID = &pathRunID
	if event.Kind == "" {
		event.Kind = "path_run_finished"
	}
	if event.Label == "" {
		event.Label = fmt.Sprintf("路径运行结束：%s", model.PathRunStatusName(to))
	}
	if err := appendRunEvent(ctx, tx, event, now); err != nil {
		return model.PathRun{}, err
	}

	// 运行聚合镜像：只有路径运行进入运行级终态时才收尾运行；待对账保持运行中。
	if runTo, ok := runMirrorOf(to); ok {
		runFinishedAt := any(now)
		runResult := any(nil)
		if result != nil {
			runResult = string(*result)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE runs SET status = ?, result = ?, finished_at = COALESCE(?, finished_at), updated_at = ? WHERE id = ?
		`, string(runTo), runResult, runFinishedAt, now, runID); err != nil {
			return model.PathRun{}, err
		}
		if err := appendRunEvent(ctx, tx, model.RunEvent{
			RunID: runID,
			Kind:  "run_finished",
			Label: fmt.Sprintf("运行收尾：%s", model.RunStatusName(runTo)),
		}, now); err != nil {
			return model.PathRun{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.PathRun{}, err
	}
	return r.GetPathRun(ctx, pathRunID)
}

// runMirrorOf 给出路径运行终态对应的运行聚合终态；
// 待对账不镜像（第二返回值为 false）：运行整体保持运行中，等对账切片给出唯一合法恢复。
func runMirrorOf(to model.PathRunStatus) (model.RunStatus, bool) {
	switch to {
	case model.PathRunStatusCompleted:
		return model.RunStatusCompleted, true
	case model.PathRunStatusFailed:
		return model.RunStatusFailed, true
	case model.PathRunStatusStopped:
		return model.RunStatusStopped, true
	case model.PathRunStatusCancelled:
		return model.RunStatusCancelled, true
	default:
		return "", false
	}
}

// ClaimPathRunLease 领取路径运行的推进权：仅当未到终态且没有其他 Worker 的有效租约时成功。
// fencing token 随每次领取单调递增，旧 Worker 凭旧 token 的推进与续租都会落空。
func (r *RunRepository) ClaimPathRunLease(ctx context.Context, pathRunID uint64, workerID string, leaseDuration time.Duration, now time.Time) (uint64, error) {
	now = now.UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE path_runs
		SET lease_owner = ?, lease_expires_at = ?, fencing_token = fencing_token + 1, updated_at = ?
		WHERE id = ?
		  AND status IN ('waiting', 'running', 'verifying', 'paused')
		  AND (lease_owner IS NULL OR lease_owner = '' OR lease_expires_at IS NULL OR lease_expires_at <= ?)
	`, workerID, now.Add(leaseDuration), now, pathRunID, now)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if affected == 0 {
		// 未命中：要么已被其他 Worker 持有有效租约，要么路径运行已到终态。
		var status string
		statusErr := r.db.QueryRowContext(ctx, "SELECT status FROM path_runs WHERE id = ?", pathRunID).Scan(&status)
		if statusErr == nil && model.IsTerminalPathRunStatus(model.PathRunStatus(status)) {
			return 0, fmt.Errorf("%w：路径运行已处于 %s", repository.ErrRunStatusConflict, model.PathRunStatusName(model.PathRunStatus(status)))
		}
		return 0, repository.ErrLeaseHeld
	}
	var token uint64
	if err := r.db.QueryRowContext(ctx, "SELECT fencing_token FROM path_runs WHERE id = ?", pathRunID).Scan(&token); err != nil {
		return 0, err
	}
	return token, nil
}

// RenewPathRunLease 校验 owner 与 fencing token 后续租；未命中返回 ErrStaleLease。
// 命中判定放在事务内的 SELECT ... FOR UPDATE，而不是依赖 UPDATE 受影响行数：
// MySQL 默认统计“被更改的行”而不是“匹配的行”，当新到期时间与现值完全相同（同一毫秒内连续续租）
// 时受影响行数为 0，会把一次合法续租误判成租约失效。
func (r *RunRepository) RenewPathRunLease(ctx context.Context, pathRunID uint64, workerID string, fencingToken uint64, leaseDuration time.Duration, now time.Time) error {
	now = now.UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var matched uint64
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM path_runs
		WHERE id = ? AND lease_owner = ? AND fencing_token = ? AND lease_expires_at > ?
		  AND status IN ('waiting', 'running', 'verifying', 'paused')
		FOR UPDATE
	`, pathRunID, workerID, fencingToken, now).Scan(&matched)
	if errors.Is(err, sql.ErrNoRows) {
		return repository.ErrStaleLease
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE path_runs SET lease_expires_at = ?, updated_at = ? WHERE id = ?
	`, now.Add(leaseDuration), now, matched); err != nil {
		return err
	}
	return tx.Commit()
}

// ReleasePathRunLease 在一步走完落账后释放租约；未命中说明租约已被接管，返回 ErrStaleLease。
func (r *RunRepository) ReleasePathRunLease(ctx context.Context, pathRunID uint64, workerID string, fencingToken uint64, now time.Time) error {
	now = now.UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE path_runs SET lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
		WHERE id = ? AND lease_owner = ? AND fencing_token = ?
	`, now, pathRunID, workerID, fencingToken)
	if err != nil {
		return err
	}
	return requireOneRowUpdated(result, nil)
}

// RecoverInterruptedPathRuns 把处于运行中/核验中的路径运行一律置为待对账并写事件行。
// 这是纲领第 4.2 节的不可破坏约束：崩溃前可能已经发出过一次写请求，重启后绝不自动继续。
// 每条路径运行独立处理（一条失败不影响其余），运行聚合保持原状留给对账切片。
func (r *RunRepository) RecoverInterruptedPathRuns(ctx context.Context, now time.Time) ([]uint64, error) {
	now = now.UTC()
	rows, err := r.db.QueryContext(ctx, `
		SELECT id FROM path_runs WHERE status IN ('running', 'verifying') ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	var ids []uint64
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	recovered := []uint64{}
	for _, id := range ids {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		var runID uint64
		var status string
		err = tx.QueryRowContext(ctx, "SELECT run_id, status FROM path_runs WHERE id = ? FOR UPDATE", id).Scan(&runID, &status)
		if errors.Is(err, sql.ErrNoRows) {
			tx.Rollback()
			continue
		}
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		if model.PathRunStatus(status) != model.PathRunStatusRunning && model.PathRunStatus(status) != model.PathRunStatusVerifying {
			// 已被并发逻辑处理，跳过。
			tx.Rollback()
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE path_runs
			SET status = ?, finished_at = ?, lease_owner = NULL, lease_expires_at = NULL, updated_at = ?
			WHERE id = ?
		`, string(model.PathRunStatusAwaitingReconciliation), now, now, id); err != nil {
			tx.Rollback()
			return nil, err
		}
		if err := appendRunEvent(ctx, tx, model.RunEvent{
			RunID:     runID,
			PathRunID: &id,
			Kind:      "crash_recovered",
			Label:     "进程重启时发现路径运行未完成，已置为待对账；写是否已生效需要人工确认",
		}, now); err != nil {
			tx.Rollback()
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		recovered = append(recovered, id)
	}
	return recovered, nil
}

// isDuplicateKeyError 判断错误是否为 MySQL 唯一键冲突，用于运行号分配的撞键重试。
func isDuplicateKeyError(err error) bool {
	var driverErr *driver.MySQLError
	return errors.As(err, &driverErr) && driverErr.Number == 1062
}

// requireOneRowUpdated 把租约类 UPDATE 的未命中统一映射为租约已失效。
func requireOneRowUpdated(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return repository.ErrStaleLease
	}
	return nil
}

// scanRun 把运行聚合行组装为模型；可空列逐个判空。
func scanRun(scan func(dest ...any) error) (model.Run, error) {
	var run model.Run
	var mode, trigger, status string
	var result sql.NullString
	var maxConcurrency sql.NullInt64
	var startedAt, finishedAt sql.NullTime
	err := scan(&run.ID, &run.PlanID, &run.RunNo, &mode, &trigger, &maxConcurrency, &status, &result, &startedAt, &finishedAt, &run.CreatedAt, &run.UpdatedAt)
	if err != nil {
		return model.Run{}, err
	}
	run.Mode = model.RunMode(mode)
	run.TriggerKind = model.RunTriggerKind(trigger)
	run.Status = model.RunStatus(status)
	if maxConcurrency.Valid {
		value := int(maxConcurrency.Int64)
		run.MaxConcurrency = &value
	}
	if result.Valid {
		value := model.RunResult(result.String)
		run.Result = &value
	}
	if startedAt.Valid {
		value := startedAt.Time
		run.StartedAt = &value
	}
	if finishedAt.Valid {
		value := finishedAt.Time
		run.FinishedAt = &value
	}
	return run, nil
}

// scanPathRun 把路径运行聚合行组装为模型；可空列逐个判空。
func scanPathRun(scan func(dest ...any) error) (model.PathRun, error) {
	var pathRun model.PathRun
	var status string
	var result, failureClass, mainInstanceRef, finalTargetSummary, leaseOwner sql.NullString
	var leaseExpiresAt, startedAt, finishedAt sql.NullTime
	err := scan(&pathRun.ID, &pathRun.RunID, &pathRun.ExecutionPathID, &status, &result, &failureClass,
		&mainInstanceRef, &finalTargetSummary, &leaseOwner, &leaseExpiresAt, &pathRun.FencingToken,
		&startedAt, &finishedAt, &pathRun.CreatedAt, &pathRun.UpdatedAt)
	if err != nil {
		return model.PathRun{}, err
	}
	pathRun.Status = model.PathRunStatus(status)
	if result.Valid {
		value := model.RunResult(result.String)
		pathRun.Result = &value
	}
	if failureClass.Valid {
		value := model.FailureClass(failureClass.String)
		pathRun.FailureClass = &value
	}
	pathRun.MainInstanceRef = mainInstanceRef.String
	pathRun.FinalTargetSummary = finalTargetSummary.String
	pathRun.LeaseOwner = leaseOwner.String
	if leaseExpiresAt.Valid {
		value := leaseExpiresAt.Time
		pathRun.LeaseExpiresAt = &value
	}
	if startedAt.Valid {
		value := startedAt.Time
		pathRun.StartedAt = &value
	}
	if finishedAt.Valid {
		value := finishedAt.Time
		pathRun.FinishedAt = &value
	}
	return pathRun, nil
}

// nullableRunResult 把可空路径结果转为落库形态。
func nullableRunResult(result *model.RunResult) any {
	if result == nil {
		return nil
	}
	return string(*result)
}

// nullableFailureClass 把可空失败分类转为落库形态。
func nullableFailureClass(class *model.FailureClass) any {
	if class == nil {
		return nil
	}
	return string(*class)
}
