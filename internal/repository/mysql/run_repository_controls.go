package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// 本文件承接运行记录仓储里与控制事实、对账结论与步骤落账相关的方法（纲领第 7.1、7.2 节），
// 与 run_repository.go 同属 mysql 包；拆分只为满足纲领第 10 节的单文件行数上限，不改任何行为。

// AppendRunControl 追加一行人工控制事实；kind/breakpoint/command 等列按事实类别填充。
func (r *RunRepository) AppendRunControl(ctx context.Context, control model.RunControl, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO run_controls (run_id, path_run_id, kind, action, source, breakpoint_type, object_kind, object_key, command, reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, control.RunID, control.PathRunID, string(control.Kind), string(control.Action), string(control.Source),
		nullableString(string(control.BreakpointType)), nullableString(control.ObjectKind), nullableString(control.ObjectKey),
		nullableString(string(control.Command)), nullableString(control.Reason), now.UTC())
	return err
}

// ListRunControls 按路径运行按时间正序列出控制事实。
func (r *RunRepository) ListRunControls(ctx context.Context, pathRunID uint64) ([]model.RunControl, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT run_id, path_run_id, kind, action, source, breakpoint_type, object_kind, object_key, command, reason, created_at
		FROM run_controls WHERE path_run_id = ? ORDER BY id ASC
	`, pathRunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	controls := []model.RunControl{}
	for rows.Next() {
		var control model.RunControl
		var kind, action, source, breakpointType, objectKind, objectKey, command, reason sql.NullString
		if err := rows.Scan(&control.RunID, &control.PathRunID, &kind, &action, &source, &breakpointType,
			&objectKind, &objectKey, &command, &reason, &control.CreatedAt); err != nil {
			return nil, err
		}
		control.Kind = model.ControlFactKind(kind.String)
		control.Action = model.RunControlAction(action.String)
		control.Source = model.RunControlSource(source.String)
		control.BreakpointType = model.BreakpointType(breakpointType.String)
		control.ObjectKind = objectKind.String
		control.ObjectKey = objectKey.String
		control.Command = model.ControlCommand(command.String)
		control.Reason = reason.String
		controls = append(controls, control)
	}
	return controls, rows.Err()
}

// AppendRunEvent 追加一行运行事件。
func (r *RunRepository) AppendRunEvent(ctx context.Context, event model.RunEvent, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := appendRunEvent(ctx, tx, event, now.UTC()); err != nil {
		return err
	}
	return tx.Commit()
}

// LatestStepAttempt 返回路径运行最近一次落账的步骤与尝试事实。
func (r *RunRepository) LatestStepAttempt(ctx context.Context, pathRunID uint64) (model.RunStep, model.RunStepAttempt, error) {
	var step model.RunStep
	var attempt model.RunStepAttempt
	err := r.db.QueryRowContext(ctx, `
		SELECT id, path_run_id, step_no, source, action, node_key, status FROM run_steps
		WHERE path_run_id = ? ORDER BY step_no DESC LIMIT 1
	`, pathRunID).Scan(&step.ID, &step.PathRunID, &step.StepNo, &step.Source, &step.Action, &step.NodeKey, &step.Status)
	if err != nil {
		return step, attempt, err
	}
	attempts, err := r.ListRunAttempts(ctx, pathRunID)
	if err != nil {
		return step, attempt, err
	}
	for index := len(attempts) - 1; index >= 0; index-- {
		if attempts[index].StepID == step.ID {
			return step, attempts[index], nil
		}
	}
	return step, attempt, sql.ErrNoRows
}

// RecordReconcileOutcome 把对账结论与恢复动作写回尝试行的对账三列。
// 这是事实表上唯一被允许的 UPDATE：仅覆盖纲领第 7.2 节明确归属本表的三个对账字段，不触碰任何既有事实。
func (r *RunRepository) RecordReconcileOutcome(ctx context.Context, attemptID uint64, verdict string, action string, isReplay bool, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE run_step_attempts SET reconcile_verdict = ?, recovery_action = ?, is_replay = ?
		WHERE id = ?
	`, verdict, action, isReplay, attemptID)
	return err
}

// AppendManualConclusion 登记人工核对结论事实（只 INSERT）。
func (r *RunRepository) AppendManualConclusion(ctx context.Context, conclusion model.RunManualConclusion, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO run_manual_conclusions (run_id, path_run_id, step_no, instance_status, current_node, note, reporter, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, conclusion.RunID, conclusion.PathRunID, conclusion.StepNo, conclusion.InstanceStatus,
		conclusion.CurrentNode, nullableString(conclusion.Note), nullableString(conclusion.Reporter), now.UTC())
	return err
}

// SetFinalTargetSummary 落库最终目标事实摘要；路径运行未绑定实例时不允许写摘要。
func (r *RunRepository) SetFinalTargetSummary(ctx context.Context, pathRunID uint64, summary string, now time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE path_runs SET final_target_summary = ?, updated_at = ? WHERE id = ?
	`, summary, now.UTC(), pathRunID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return repository.ErrRunNotFound
	}
	return nil
}

// SetMainInstanceRef 首次落库主实例引用；引用已存在且不同时拒绝，
// 保证一条路径运行自始至终只指向一个真实主实例。
func (r *RunRepository) SetMainInstanceRef(ctx context.Context, pathRunID uint64, instanceRef string, now time.Time) error {
	now = now.UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE path_runs SET main_instance_ref = ?, updated_at = ?
		WHERE id = ? AND (main_instance_ref IS NULL OR main_instance_ref = '')
	`, instanceRef, now, pathRunID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		var current string
		if err := r.db.QueryRowContext(ctx, "SELECT main_instance_ref FROM path_runs WHERE id = ?", pathRunID).Scan(&current); err != nil {
			return err
		}
		if current == instanceRef {
			return nil
		}
		return fmt.Errorf("%w：路径运行已绑定其他主实例", repository.ErrRunStatusConflict)
	}
	return nil
}

// RecordStepAttempt 把步骤事实与尝试事实在同一事务内 INSERT。
// 先插步骤行取得 ID，再插尝试行引用它；任何一步失败即整体回滚，保证事实只以完整形态存在。
func (r *RunRepository) RecordStepAttempt(ctx context.Context, step model.RunStep, attempt model.RunStepAttempt, now time.Time) (uint64, error) {
	now = now.UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	stepResult, err := tx.ExecContext(ctx, `
		INSERT INTO run_steps (path_run_id, step_no, source, action, node_key, actor_summary, gate_snapshot, status, started_at, finished_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, step.PathRunID, step.StepNo, step.Source, step.Action, step.NodeKey, nullableString(step.ActorSummary),
		nullableString(step.GateSnapshot), string(step.Status), step.StartedAt, step.FinishedAt, now)
	if err != nil {
		return 0, err
	}
	stepID, err := stepResult.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO run_step_attempts (path_run_id, step_id, attempt_no, verdict, side_effect, transport, status_code,
			initial, reread, failure_class, reason, basis, trace_id, curl_trace_id, log_path, log_line, duration_ms, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, step.PathRunID, stepID, attempt.AttemptNo, attempt.Verdict, attempt.SideEffect, attempt.Transport,
		nullableInt(attempt.StatusCode), attempt.Initial, attempt.Reread, nullableFailureClass(attempt.FailureClass),
		attempt.Reason, attempt.Basis, attempt.TraceID, attempt.CurlTraceID, attempt.LogPath, attempt.LogLine,
		attempt.DurationMs, now); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return uint64(stepID), nil
}

// nullableString 把空字符串转为 NULL，保持事实表没有空串噪音。
func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// nullableInt 把零值整数转为 NULL（HTTP 状态码缺省表示未收到状态行）。
func nullableInt(value int) any {
	if value == 0 {
		return nil
	}
	return value
}
