// F-020 多路径调度与单次定时启动的存储面：批量创建路径运行、按运行列出路径运行、
// 计划行聚合收尾与定时消费标记。纪律与迁移 026/030 一致：
// 事实表只 INSERT；聚合表状态单向前进且同事务追加事件行；绝不触碰目标平台数据。
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

// CreateRunWithPaths 创建一次运行并按勾选路径集合批量创建路径运行（全部等待运行）。
// 幂等：键非空且命中已有运行时，返回那次运行与它的全部路径运行（created=false 语义由调用方经数量判断）。
func (r *RunRepository) CreateRunWithPaths(ctx context.Context, input repository.CreateRunInput) (model.Run, []model.PathRun, error) {
	if len(input.ExecutionPathIDs) == 0 {
		return model.Run{}, nil, fmt.Errorf("%w：至少勾选一条执行路径才能启动", repository.ErrRunInvalidInput)
	}
	if input.IdempotencyKey != "" {
		if existing, pathRuns, found, err := r.findRunByIdempotencyKey(ctx, input.PlanID, input.IdempotencyKey); err != nil {
			return model.Run{}, nil, err
		} else if found {
			return existing, pathRuns, nil
		}
	}
	now := time.Now().UTC()
	for attempt := 0; attempt < runNumberRetryAttempts; attempt++ {
		run, pathRuns, err := r.tryCreateRunWithPaths(ctx, input, now)
		if err == nil {
			return run, pathRuns, nil
		}
		if !isDuplicateKeyError(err) {
			return model.Run{}, nil, err
		}
	}
	return model.Run{}, nil, fmt.Errorf("运行号分配在并发冲突下重试 %d 次仍未成功", runNumberRetryAttempts)
}

// findRunByIdempotencyKey 按幂等键查找已有运行；命中时连同全部路径运行一起返回。
func (r *RunRepository) findRunByIdempotencyKey(ctx context.Context, planID uint64, key string) (model.Run, []model.PathRun, bool, error) {
	var id uint64
	err := r.db.QueryRowContext(ctx,
		"SELECT id FROM runs WHERE plan_id = ? AND idempotency_key = ?", planID, key).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Run{}, nil, false, nil
	}
	if err != nil {
		return model.Run{}, nil, false, err
	}
	run, err := r.getRunWithMeta(ctx, id)
	if err != nil {
		return model.Run{}, nil, false, err
	}
	pathRuns, err := r.ListPathRunsByRun(ctx, id)
	if err != nil {
		return model.Run{}, nil, false, err
	}
	return run, pathRuns, true, nil
}

// getRunWithMeta 读取运行聚合并带出幂等键与预置断点两列元数据（共享 scanRun 不含这两列）。
func (r *RunRepository) getRunWithMeta(ctx context.Context, runID uint64) (model.Run, error) {
	var run model.Run
	var mode, trigger, status string
	var result sql.NullString
	var maxConcurrency sql.NullInt64
	var startedAt, finishedAt sql.NullTime
	var idempotencyKey, presetBreakpoints sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT id, plan_id, run_no, mode, trigger_kind, max_concurrency, status, result, started_at, finished_at, created_at, updated_at,
		       idempotency_key, preset_breakpoints
		FROM runs WHERE id = ?
	`, runID).Scan(&run.ID, &run.PlanID, &run.RunNo, &mode, &trigger, &maxConcurrency, &status, &result,
		&startedAt, &finishedAt, &run.CreatedAt, &run.UpdatedAt, &idempotencyKey, &presetBreakpoints)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Run{}, repository.ErrRunNotFound
	}
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
	run.IdempotencyKey = idempotencyKey.String
	run.PresetBreakpoints = presetBreakpoints.String
	return run, nil
}

// tryCreateRunWithPaths 执行一次运行号分配与批量创建事务；计划内运行号取当前最大值加一。
func (r *RunRepository) tryCreateRunWithPaths(ctx context.Context, input repository.CreateRunInput, now time.Time) (model.Run, []model.PathRun, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Run{}, nil, err
	}
	defer tx.Rollback()

	var nextRunNo uint64
	if err := tx.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(run_no), 0) + 1 FROM runs WHERE plan_id = ?", input.PlanID).Scan(&nextRunNo); err != nil {
		return model.Run{}, nil, err
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO runs (plan_id, run_no, mode, trigger_kind, max_concurrency, idempotency_key, preset_breakpoints, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, input.PlanID, nextRunNo, string(input.Mode), string(input.Trigger), input.MaxConcurrency,
		nullableString(input.IdempotencyKey), nullableString(input.PresetBreakpoints),
		string(model.RunStatusPending), now, now)
	if err != nil {
		return model.Run{}, nil, err
	}
	runID, err := result.LastInsertId()
	if err != nil {
		return model.Run{}, nil, err
	}
	if err := appendRunEvent(ctx, tx, model.RunEvent{
		RunID: uint64(runID),
		Kind:  "run_created",
		Label: fmt.Sprintf("运行 %d 已创建（%s，共 %d 条路径）", nextRunNo, model.RunModeName(input.Mode), len(input.ExecutionPathIDs)),
	}, now); err != nil {
		return model.Run{}, nil, err
	}
	created := make([]model.PathRun, 0, len(input.ExecutionPathIDs))
	for _, pathID := range input.ExecutionPathIDs {
		pathResult, err := tx.ExecContext(ctx, `
			INSERT INTO path_runs (run_id, execution_path_id, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
		`, runID, pathID, string(model.PathRunStatusWaiting), now, now)
		if err != nil {
			return model.Run{}, nil, err
		}
		pathRunID, err := pathResult.LastInsertId()
		if err != nil {
			return model.Run{}, nil, err
		}
		pathRunIDValue := uint64(pathRunID)
		if err := appendRunEvent(ctx, tx, model.RunEvent{
			RunID:     uint64(runID),
			PathRunID: &pathRunIDValue,
			Kind:      "path_run_created",
			Label:     "路径运行已创建，等待调度",
		}, now); err != nil {
			return model.Run{}, nil, err
		}
		created = append(created, model.PathRun{
			ID: pathRunIDValue, RunID: uint64(runID), ExecutionPathID: pathID,
			Status: model.PathRunStatusWaiting, CreatedAt: now, UpdatedAt: now,
		})
	}
	if err := tx.Commit(); err != nil {
		return model.Run{}, nil, err
	}
	run, err := r.GetRun(ctx, uint64(runID))
	if err != nil {
		return model.Run{}, nil, err
	}
	return run, created, nil
}

// ListPathRunsByRun 按运行列出全部路径运行（按 ID 升序，即创建顺序）。
func (r *RunRepository) ListPathRunsByRun(ctx context.Context, runID uint64) ([]model.PathRun, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, run_id, execution_path_id, status, result, failure_class,
		       main_instance_ref, final_target_summary, lease_owner, lease_expires_at, fencing_token,
		       started_at, finished_at, created_at, updated_at
		FROM path_runs WHERE run_id = ? ORDER BY id ASC
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	pathRuns := make([]model.PathRun, 0)
	for rows.Next() {
		pathRun, scanErr := scanPathRun(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		pathRuns = append(pathRuns, pathRun)
	}
	return pathRuns, rows.Err()
}

// ListRunEvents 读取一次运行的事件流（F-021 增量读取）：afterID 为游标，只返回其后的事件。
// 按数据库自增键升序，前端轮询只追加，历史不重排。
func (r *RunRepository) ListRunEvents(ctx context.Context, runID uint64, afterID uint64, limit int) ([]model.RunEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, run_id, path_run_id, kind, label, detail, created_at
		FROM run_events WHERE run_id = ? AND id > ? ORDER BY id ASC LIMIT ?
	`, runID, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]model.RunEvent, 0)
	for rows.Next() {
		var event model.RunEvent
		var pathRunID sql.NullInt64
		var detail sql.NullString
		if err := rows.Scan(&event.ID, &event.RunID, &pathRunID, &event.Kind, &event.Label, &detail, &event.CreatedAt); err != nil {
			return nil, err
		}
		if pathRunID.Valid {
			value := uint64(pathRunID.Int64)
			event.PathRunID = &value
		}
		event.Detail = detail.String
		events = append(events, event)
	}
	return events, rows.Err()
}

// ListRunsFiltered 按状态筛选计划下的运行（最新在前，游标分页：beforeID 为上一页最后一行运行 ID）。
func (r *RunRepository) ListRunsFiltered(ctx context.Context, planID uint64, status string, beforeID uint64, limit int) ([]model.Run, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if beforeID == 0 {
		beforeID = ^uint64(0)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, plan_id, run_no, mode, trigger_kind, max_concurrency, status, result, started_at, finished_at, created_at, updated_at
		FROM runs
		WHERE plan_id = ? AND (? = '' OR status = ?) AND id < ?
		ORDER BY id DESC LIMIT ?
	`, planID, status, status, beforeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := make([]model.Run, 0)
	for rows.Next() {
		run, scanErr := scanRun(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// ClaimScheduledPlan 原子领取到点的单次定时启动：只有 scheduled_consumed_at 仍为空的行会置为已消费。
// 返回是否领取成功：并发扫描或重启后的重复扫描只有一个赢家，其余全部落空（一次性消费）。
func (r *RunRepository) ClaimScheduledPlan(ctx context.Context, planID uint64, now time.Time) (bool, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE test_plans SET scheduled_consumed_at = ?, updated_at = ?
		WHERE id = ? AND scheduled_at IS NOT NULL AND scheduled_consumed_at IS NULL
	`, now, now, planID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

// ListDueScheduledPlans 列出到点尚未消费的计划（数据库时间为准，不依赖应用进程时钟）。
func (r *RunRepository) ListDueScheduledPlans(ctx context.Context, now time.Time) ([]model.Plan, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, account, account_display_name, flow_source, target_object_id,
		       target_object_name, run_mode, max_concurrency, scheduled_at, scheduled_consumed_at, status,
		       (SELECT COUNT(*) FROM test_execution_paths ep WHERE ep.plan_id = test_plans.id) AS path_count,
		       created_at, updated_at
		FROM test_plans
		WHERE scheduled_at IS NOT NULL AND scheduled_consumed_at IS NULL AND scheduled_at <= ?
		ORDER BY scheduled_at ASC
	`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	plans := make([]model.Plan, 0)
	for rows.Next() {
		var plan model.Plan
		var maxConcurrency sql.NullInt64
		var scheduledAt, consumedAt sql.NullTime
		if err := rows.Scan(&plan.ID, &plan.Name, &plan.Account, &plan.AccountDisplayName, &plan.FlowSource,
			&plan.TargetObjectID, &plan.TargetObjectName, &plan.RunMode, &maxConcurrency, &scheduledAt, &consumedAt, &plan.Status,
			&plan.PathCount, &plan.CreatedAt, &plan.UpdatedAt); err != nil {
			return nil, err
		}
		if maxConcurrency.Valid {
			value := int(maxConcurrency.Int64)
			plan.MaxConcurrency = &value
		}
		if scheduledAt.Valid {
			value := scheduledAt.Time.UTC()
			plan.ScheduledAt = &value
		}
		if consumedAt.Valid {
			value := consumedAt.Time.UTC()
			plan.ScheduledConsumedAt = &value
		}
		plans = append(plans, plan)
	}
	return plans, rows.Err()
}
