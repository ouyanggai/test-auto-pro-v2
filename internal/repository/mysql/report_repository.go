package mysql

import (
	"context"
	"database/sql"
	"strings"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// ReportRepository 是 F-042 报告中心的只读聚合仓储，只读运行记录六表，不写库、不读目标业务库。
type ReportRepository struct {
	db *sql.DB
}

// NewReportRepository 构造报告只读聚合仓储。
func NewReportRepository(db *sql.DB) *ReportRepository {
	return &ReportRepository{db: db}
}

// reportTerminalStatuses 是路径运行的已结束状态集合（用于成功率分母与结果分布）。
const reportTerminalStatuses = "'completed','failed','stopped','cancelled','awaiting_reconciliation'"

// reportSampleDefaultLimit 是样本行默认上限；reportSampleMaxLimit 是硬上限，防止一次拉全表。
const (
	reportSampleDefaultLimit = 500
	reportSampleMaxLimit     = 2000
)

// ReportCounts 用单条查询算出首页第一级计划情况计数（全量 COUNT，非采样）。
func (r *ReportRepository) ReportCounts(ctx context.Context) (repository.ReportCounts, error) {
	var counts repository.ReportCounts
	err := r.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM test_plans),
			(SELECT COUNT(DISTINCT plan_id) FROM runs),
			(SELECT COUNT(*) FROM runs),
			(SELECT COUNT(*) FROM runs WHERE status IN ('pending','running')),
			(SELECT COUNT(*) FROM path_runs)
	`).Scan(&counts.PlansTotal, &counts.PlansWithRuns, &counts.RunsTotal, &counts.RunsRunning, &counts.PathRunsTotal)
	if err != nil {
		return repository.ReportCounts{}, err
	}
	return counts, nil
}

// ReportPlanRollups 一次分组聚合出每个有运行记录计划的运行数、路径运行数、已结束数与成功数。
func (r *ReportRepository) ReportPlanRollups(ctx context.Context) ([]repository.ReportPlanRollup, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT r.plan_id,
		       COUNT(DISTINCT r.id) AS run_count,
		       COUNT(pr.id) AS path_run_count,
		       COALESCE(SUM(pr.status IN (`+reportTerminalStatuses+`)), 0) AS path_finished,
		       COALESCE(SUM(pr.result = 'succeeded'), 0) AS path_succeeded,
		       COALESCE(SUM(pr.result = 'failed'), 0) AS path_failed,
		       COALESCE(SUM(pr.status = 'stopped'), 0) AS path_stopped,
		       COALESCE(SUM(pr.status = 'cancelled'), 0) AS path_cancelled,
		       COALESCE(SUM(pr.status = 'awaiting_reconciliation'), 0) AS path_awaiting,
		       COUNT(DISTINCT pr.execution_path_id) AS covered_paths
		FROM runs r
		LEFT JOIN path_runs pr ON pr.run_id = r.id
		GROUP BY r.plan_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rollups := []repository.ReportPlanRollup{}
	for rows.Next() {
		var rollup repository.ReportPlanRollup
		if err := rows.Scan(&rollup.PlanID, &rollup.RunCount, &rollup.PathRunCount,
			&rollup.PathRunFinished, &rollup.PathRunSucceeded, &rollup.PathRunFailed,
			&rollup.PathRunStopped, &rollup.PathRunCancelled, &rollup.PathRunAwaiting,
			&rollup.CoveredPaths); err != nil {
			return nil, err
		}
		rollups = append(rollups, rollup)
	}
	return rollups, rows.Err()
}

// reportScopeClause 按范围拼装 WHERE 条件与参数：全局无条件，单计划按 plan_id，单路径再加 execution_path_id。
func reportScopeClause(scope repository.ReportScope) (string, []any) {
	clauses := []string{}
	args := []any{}
	if scope.PlanID != 0 {
		clauses = append(clauses, "r.plan_id = ?")
		args = append(args, scope.PlanID)
	}
	if scope.ExecutionPathID != 0 {
		clauses = append(clauses, "pr.execution_path_id = ?")
		args = append(args, scope.ExecutionPathID)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

// reportScopeLimit 归一化样本上限：非正值用默认，超硬上限截到硬上限，避免一次拉全表。
func reportScopeLimit(scope repository.ReportScope) int {
	if scope.Limit <= 0 {
		return reportSampleDefaultLimit
	}
	if scope.Limit > reportSampleMaxLimit {
		return reportSampleMaxLimit
	}
	return scope.Limit
}

// ReportPathRunRows 按范围有界拉取路径运行事实行（最新在前），供成功率、分布与实例列表使用。
func (r *ReportRepository) ReportPathRunRows(ctx context.Context, scope repository.ReportScope) ([]repository.ReportPathRunRow, error) {
	where, args := reportScopeClause(scope)
	args = append(args, reportScopeLimit(scope))
	rows, err := r.db.QueryContext(ctx, `
		SELECT pr.id, pr.run_id, r.run_no, r.plan_id, pr.execution_path_id, pr.status, pr.result, pr.failure_class,
		       pr.main_instance_ref, pr.total_steps, pr.started_at, pr.finished_at
		FROM path_runs pr JOIN runs r ON pr.run_id = r.id`+where+`
		ORDER BY pr.id DESC
		LIMIT ?
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []repository.ReportPathRunRow{}
	for rows.Next() {
		var row repository.ReportPathRunRow
		var status string
		var resultCode, failureClass, mainInstanceRef sql.NullString
		var totalSteps sql.NullInt64
		var startedAt, finishedAt sql.NullTime
		if err := rows.Scan(&row.PathRunID, &row.RunID, &row.RunNo, &row.PlanID, &row.ExecutionPathID,
			&status, &resultCode, &failureClass, &mainInstanceRef, &totalSteps, &startedAt, &finishedAt); err != nil {
			return nil, err
		}
		row.Status = model.PathRunStatus(status)
		if resultCode.Valid {
			value := model.RunResult(resultCode.String)
			row.Result = &value
		}
		if failureClass.Valid {
			value := model.FailureClass(failureClass.String)
			row.FailureClass = &value
		}
		row.MainInstanceRef = mainInstanceRef.String
		if totalSteps.Valid {
			value := int(totalSteps.Int64)
			row.TotalSteps = &value
		}
		if startedAt.Valid {
			value := startedAt.Time.UTC()
			row.StartedAt = &value
		}
		if finishedAt.Valid {
			value := finishedAt.Time.UTC()
			row.FinishedAt = &value
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// ReportAttemptRows 按范围有界拉取尝试事实行（最新在前），供步骤/执行器耗时分位与重试统计使用。
func (r *ReportRepository) ReportAttemptRows(ctx context.Context, scope repository.ReportScope) ([]repository.ReportAttemptRow, error) {
	where, args := reportScopeClause(scope)
	args = append(args, reportScopeLimit(scope))
	rows, err := r.db.QueryContext(ctx, `
		SELECT a.path_run_id, a.step_id, a.attempt_no, a.verdict, a.failure_class, a.duration_ms, a.executor_ms, a.is_replay
		FROM run_step_attempts a
		JOIN path_runs pr ON a.path_run_id = pr.id
		JOIN runs r ON pr.run_id = r.id`+where+`
		ORDER BY a.id DESC
		LIMIT ?
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []repository.ReportAttemptRow{}
	for rows.Next() {
		var row repository.ReportAttemptRow
		var failureClass sql.NullString
		var executorMs sql.NullInt64
		if err := rows.Scan(&row.PathRunID, &row.StepID, &row.AttemptNo, &row.Verdict, &failureClass,
			&row.DurationMs, &executorMs, &row.IsReplay); err != nil {
			return nil, err
		}
		if failureClass.Valid {
			value := model.FailureClass(failureClass.String)
			row.FailureClass = &value
		}
		if executorMs.Valid {
			row.ExecutorMs = executorMs.Int64
			row.ExecutorMsKnown = true
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
