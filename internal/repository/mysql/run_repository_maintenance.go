package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// 本文件承接 2026-09-06 交付验收新增的运行维护方法：跨计划运行列表、总步骤冻结与整次运行删除。
// 与 run_repository.go 同属 mysql 包；拆分只为满足单文件行数上限，不改任何既有行为。

// SetPathRunTotalSteps 冻结本次运行的总步骤数（进度分母）。
// 只允许写一次：已冻结或历史运行（列非 NULL）一律保持原值，配置后续变化不得改变已运行进度的分母。
func (r *RunRepository) SetPathRunTotalSteps(ctx context.Context, pathRunID uint64, total int, now time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE path_runs SET total_steps = ?, updated_at = ? WHERE id = ? AND total_steps IS NULL
	`, total, now.UTC(), pathRunID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		// 已冻结（幂等重复调用）或路径运行不存在：区分这两种情况，不把不存在静默当成已冻结。
		var exists uint64
		if err := r.db.QueryRowContext(ctx, "SELECT id FROM path_runs WHERE id = ?", pathRunID).Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return repository.ErrRunNotFound
			}
			return err
		}
		return nil
	}
	return nil
}

// ListAllRunsFiltered 跨计划列出运行（运行号倒序、游标分页），status 非空时按状态筛选。
// 供运行记录列表「一行只对应一次计划运行」使用；每次返回附带计划 ID 由服务层补计划名称。
func (r *RunRepository) ListAllRunsFiltered(ctx context.Context, status string, beforeID uint64, limit int) ([]model.Run, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := `
		SELECT id, plan_id, run_no, mode, trigger_kind, max_concurrency, status, result, started_at, finished_at, created_at, updated_at
		FROM runs
		WHERE (? = '' OR status = ?) AND (? = 0 OR id < ?)
		ORDER BY id DESC LIMIT ?
	`
	rows, err := r.db.QueryContext(ctx, query, status, status, beforeID, beforeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := []model.Run{}
	for rows.Next() {
		run, scanErr := scanRun(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// ListRunIDsByAwaitingPaths 列出含结果待确认路径的运行 ID（去重、升序）。
// 供启动阶段的历史清扫调用：这些运行的用户侧对账通路已移除，全部闭合时应收尾聚合。
func (r *RunRepository) ListRunIDsByAwaitingPaths(ctx context.Context) ([]uint64, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT run_id FROM path_runs WHERE status = 'awaiting_reconciliation' ORDER BY run_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []uint64{}
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DeleteRun 在同一事务内删除整次工具侧运行及其全部子记录：路径运行、步骤、尝试、事件与控制事实。
// 边界：
//   - 绝不触碰目标平台实例或业务数据，删除范围只有本工具数据库的运行事实；
//   - 仍有未闭合路径（等待/运行/核验/暂停）时拒绝并给中文原因——运行中的记录必须先停止再删除；
//   - 全部删除在同一事务内提交或回滚，失败不留半删状态。
func (r *RunRepository) DeleteRun(ctx context.Context, runID uint64, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists uint64
	err = tx.QueryRowContext(ctx, "SELECT id FROM runs WHERE id = ? FOR UPDATE", runID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return repository.ErrRunNotFound
	}
	if err != nil {
		return err
	}
	var active int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM path_runs
		WHERE run_id = ? AND status IN ('waiting', 'running', 'verifying', 'paused')
	`, runID).Scan(&active); err != nil {
		return err
	}
	if active > 0 {
		return fmt.Errorf("%w：这次运行还有 %d 条未结束的路径；请先停止全部运行中的路径，再删除", repository.ErrRunStatusConflict, active)
	}
	for _, table := range []string{"run_step_attempts", "run_steps"} {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(
			"DELETE FROM %s WHERE path_run_id IN (SELECT id FROM path_runs WHERE run_id = ?)", table), runID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM run_controls WHERE run_id = ?", runID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM run_events WHERE run_id = ?", runID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM path_runs WHERE run_id = ?", runID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM runs WHERE id = ?", runID); err != nil {
		return err
	}
	return tx.Commit()
}
