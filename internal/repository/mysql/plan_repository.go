package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	driver "github.com/go-sql-driver/mysql"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

type PlanRepository struct {
	db *sql.DB
}

// planDerivedStatusExpr 从 runs 运行事实派生计划公开状态（F-032）：
// 存在等待中/运行中记录则运行中；存在历史运行则已运行；否则保留存储列（未运行）。
// 读侧以运行事实为准，即使历史数据的状态列未同步也能自愈；存储列的同步由运行事务内的
// syncPlanRunStatus 负责维护运行事实冗余列，删除守卫等写侧判断仍读该存储列。
const planDerivedStatusExpr = `
CASE
  WHEN EXISTS(SELECT 1 FROM runs ar WHERE ar.plan_id = test_plans.id AND ar.status IN ('pending','running')) THEN 'running'
  WHEN EXISTS(SELECT 1 FROM runs ar WHERE ar.plan_id = test_plans.id) THEN 'completed'
  ELSE test_plans.status
END`

// planRunSummarySelect 是计划最近一次运行与活跃运行的事实聚合列（F-032）。
// 一次查询用关联子查询取计划内最大运行号的那次运行，避免列表页逐计划再查 runs 产生 N+1。
const planRunSummarySelect = `
(SELECT ar.run_no FROM runs ar WHERE ar.plan_id = test_plans.id ORDER BY ar.run_no DESC LIMIT 1) AS last_run_no,
(SELECT ar.status FROM runs ar WHERE ar.plan_id = test_plans.id ORDER BY ar.run_no DESC LIMIT 1) AS last_run_status,
(SELECT ar.result FROM runs ar WHERE ar.plan_id = test_plans.id ORDER BY ar.run_no DESC LIMIT 1) AS last_run_result,
(SELECT ar.finished_at FROM runs ar WHERE ar.plan_id = test_plans.id ORDER BY ar.run_no DESC LIMIT 1) AS last_run_finished_at,
EXISTS(SELECT 1 FROM runs ar WHERE ar.plan_id = test_plans.id AND ar.status IN ('pending','running')) AS has_active_run`

// NewPlanRepository 创建计划仓储并复用已迁移的计划数据库连接池。
func NewPlanRepository(db *sql.DB) *PlanRepository {
	return &PlanRepository{db: db}
}

// Create 按全局幂等键创建最小计划或返回已有计划。
func (r *PlanRepository) Create(ctx context.Context, createKey string, plan model.Plan) (model.Plan, bool, error) {
	result, err := r.db.ExecContext(ctx, `
INSERT INTO test_plans (
  create_key, name, account, account_display_name, flow_source,
  target_object_id, target_object_name, run_mode, max_concurrency,
  scheduled_at, status, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		createKey, plan.Name, plan.Account, plan.AccountDisplayName, plan.FlowSource,
		plan.TargetObjectID, plan.TargetObjectName, plan.RunMode, plan.MaxConcurrency,
		plan.ScheduledAt, plan.Status, plan.CreatedAt, plan.UpdatedAt,
	)
	if err == nil {
		id, idErr := result.LastInsertId()
		if idErr != nil {
			return model.Plan{}, false, idErr
		}
		if id < 1 {
			return model.Plan{}, false, errors.New("计划主键无效")
		}
		plan.ID = uint64(id)
		return plan, true, nil
	}
	var mysqlErr *driver.MySQLError
	if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1062 {
		return model.Plan{}, false, err
	}
	existing, selectErr := r.getByCreateKey(ctx, createKey)
	return existing, false, selectErr
}

// Update 原子保存计划配置；目标身份变化时将路径配置标记为受影响，避免静默复用旧事实。
func (r *PlanRepository) Update(ctx context.Context, id uint64, plan model.Plan) (model.Plan, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Plan{}, err
	}
	defer tx.Rollback()
	var oldAccount, oldSource, oldTargetID string
	var oldScheduledAt sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT account, flow_source, target_object_id, scheduled_at FROM test_plans WHERE id = ? FOR UPDATE`, id).Scan(&oldAccount, &oldSource, &oldTargetID, &oldScheduledAt); errors.Is(err, sql.ErrNoRows) {
		return model.Plan{}, repository.ErrPlanNotFound
	} else if err != nil {
		return model.Plan{}, err
	}
	scheduledAtChanged := (oldScheduledAt.Valid != (plan.ScheduledAt != nil)) || (oldScheduledAt.Valid && plan.ScheduledAt != nil && !oldScheduledAt.Time.UTC().Equal(plan.ScheduledAt.UTC()))
	if _, err := tx.ExecContext(ctx, `UPDATE test_plans SET name = ?, account = ?, account_display_name = ?, flow_source = ?, target_object_id = ?, target_object_name = ?, run_mode = ?, max_concurrency = ?, scheduled_at = ?, scheduled_consumed_at = CASE WHEN ? THEN NULL ELSE scheduled_consumed_at END, updated_at = ? WHERE id = ?`,
		plan.Name, plan.Account, plan.AccountDisplayName, plan.FlowSource, plan.TargetObjectID, plan.TargetObjectName, plan.RunMode, plan.MaxConcurrency, plan.ScheduledAt, scheduledAtChanged, plan.UpdatedAt, id); err != nil {
		return model.Plan{}, err
	}
	if oldAccount != plan.Account || oldSource != plan.FlowSource || oldTargetID != plan.TargetObjectID {
		if _, err := tx.ExecContext(ctx, `UPDATE test_execution_path_configs SET config_status = 'affected', node_status = 'affected', data_status = 'affected', updated_at = ? WHERE path_id IN (SELECT id FROM test_execution_paths WHERE plan_id = ?)`, plan.UpdatedAt, id); err != nil {
			return model.Plan{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.Plan{}, err
	}
	updated, err := r.Get(ctx, id)
	if err != nil {
		return model.Plan{}, err
	}
	return updated, nil
}

// List 按名称和状态筛选计划，从路径表实时统计 pathCount，并一次 SQL 聚合最近运行事实。
// 状态筛选与展示都用派生状态表达式，保证与运行事实一致。
func (r *PlanRepository) List(ctx context.Context, filter model.PlanListFilter) ([]model.Plan, error) {
	query := `
SELECT id, name, account, account_display_name, flow_source, target_object_id,
       target_object_name, run_mode, max_concurrency, scheduled_at,
       ` + planDerivedStatusExpr + ` AS status,
       ` + planRunSummarySelect + `,
       (SELECT COUNT(*) FROM test_execution_paths ep WHERE ep.plan_id = test_plans.id) AS path_count,
       created_at, updated_at
FROM test_plans
WHERE (? = '' OR name LIKE CONCAT('%', ?, '%'))
  AND (? = '' OR (` + planDerivedStatusExpr + `) = ?)
ORDER BY updated_at DESC, id DESC
LIMIT ? OFFSET ?`
	rows, err := r.db.QueryContext(ctx, query, filter.Name, filter.Name, filter.Status, filter.Status, filter.Limit, filter.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	plans := make([]model.Plan, 0)
	for rows.Next() {
		plan, scanErr := scanPlan(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		plans = append(plans, plan)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return plans, nil
}

// Count 返回与 List 同名称/状态筛选下的计划总数，供分页展示总页数。
func (r *PlanRepository) Count(ctx context.Context, filter model.PlanListFilter) (int, error) {
	query := `
SELECT COUNT(*) FROM test_plans
WHERE (? = '' OR name LIKE CONCAT('%', ?, '%'))
  AND (? = '' OR (` + planDerivedStatusExpr + `) = ?)`
	var total int
	if err := r.db.QueryRowContext(ctx, query, filter.Name, filter.Name, filter.Status, filter.Status).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// Get 按主键读取计划详情及真实路径数量。
func (r *PlanRepository) Get(ctx context.Context, id uint64) (model.Plan, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, name, account, account_display_name, flow_source, target_object_id,
       target_object_name, run_mode, max_concurrency, scheduled_at,
       `+planDerivedStatusExpr+` AS status,
       `+planRunSummarySelect+`,
       (SELECT COUNT(*) FROM test_execution_paths ep WHERE ep.plan_id = test_plans.id) AS path_count,
       created_at, updated_at
FROM test_plans WHERE id = ?`, id)
	plan, err := scanPlan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Plan{}, repository.ErrPlanNotFound
	}
	return plan, err
}

// Delete 删除一个计划及其全部运行记录；路径及其工具侧配置由外键级联清理，绝不访问目标平台。
func (r *PlanRepository) Delete(ctx context.Context, id uint64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := r.deletePlanWithRuns(ctx, tx, id); err != nil {
		return err
	}
	return tx.Commit()
}

// BatchDelete 批量删除计划及其运行记录；运行中的计划会跳过并返回错误映射。
func (r *PlanRepository) BatchDelete(ctx context.Context, ids []uint64) map[uint64]error {
	failures := make(map[uint64]error)
	for _, id := range ids {
		if err := r.Delete(ctx, id); err != nil {
			failures[id] = err
		}
	}
	return failures
}

// deletePlanWithRuns 在事务内删除计划及其全部运行记录子树。
func (r *PlanRepository) deletePlanWithRuns(ctx context.Context, tx *sql.Tx, planID uint64) error {
	// 1. 锁定计划行并检查存在性
	var exists bool
	if err := tx.QueryRowContext(ctx, "SELECT 1 FROM test_plans WHERE id = ? FOR UPDATE", planID).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return repository.ErrPlanNotFound
		}
		return err
	}

	// 2. 检查是否有活跃运行
	var activeCount int
	if err := tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM runs WHERE plan_id = ? AND status IN ('pending', 'running')",
		planID).Scan(&activeCount); err != nil {
		return err
	}
	if activeCount > 0 {
		return fmt.Errorf("%w: 这个计划还有 %d 次运行正在进行中；请先停止全部运行，再删除", repository.ErrPlanStatusConflict, activeCount)
	}

	// 3. 按依赖顺序删除运行子树
	stmts := []string{
		"DELETE FROM run_manual_conclusions WHERE path_run_id IN (SELECT id FROM path_runs WHERE run_id IN (SELECT id FROM runs WHERE plan_id = ?))",
		"DELETE FROM run_step_attempts WHERE path_run_id IN (SELECT id FROM path_runs WHERE run_id IN (SELECT id FROM runs WHERE plan_id = ?))",
		"DELETE FROM run_steps WHERE path_run_id IN (SELECT id FROM path_runs WHERE run_id IN (SELECT id FROM runs WHERE plan_id = ?))",
		"DELETE FROM run_controls WHERE run_id IN (SELECT id FROM runs WHERE plan_id = ?)",
		"DELETE FROM run_events WHERE run_id IN (SELECT id FROM runs WHERE plan_id = ?)",
		"DELETE FROM path_runs WHERE run_id IN (SELECT id FROM runs WHERE plan_id = ?)",
		"DELETE FROM plan_run_queue WHERE plan_id = ?",
		"DELETE FROM runs WHERE plan_id = ?",
		"DELETE FROM test_plans WHERE id = ?", // 级联清除 paths/batches/snapshots/history_* 等有 FK 的表
	}
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt, planID); err != nil {
			return err
		}
	}
	return nil
}

// getByCreateKey 读取计划创建幂等键对应的原记录。
func (r *PlanRepository) getByCreateKey(ctx context.Context, createKey string) (model.Plan, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, name, account, account_display_name, flow_source, target_object_id,
       target_object_name, run_mode, max_concurrency, scheduled_at,
       `+planDerivedStatusExpr+` AS status,
       `+planRunSummarySelect+`,
       (SELECT COUNT(*) FROM test_execution_paths ep WHERE ep.plan_id = test_plans.id) AS path_count,
       created_at, updated_at
FROM test_plans WHERE create_key = ?`, createKey)
	return scanPlan(row)
}

type rowScanner interface {
	Scan(...any) error
}

// scanPlan 将查询行转换为 UTC 计划模型并校验持久化状态。
func scanPlan(row rowScanner) (model.Plan, error) {
	var plan model.Plan
	var maxConcurrency sql.NullInt64
	var scheduledAt sql.NullTime
	var lastRunNo sql.NullInt64
	var lastRunStatus sql.NullString
	var lastRunResult sql.NullString
	var lastRunFinishedAt sql.NullTime
	var hasActiveRun bool
	if err := row.Scan(
		&plan.ID, &plan.Name, &plan.Account, &plan.AccountDisplayName, &plan.FlowSource,
		&plan.TargetObjectID, &plan.TargetObjectName, &plan.RunMode, &maxConcurrency,
		&scheduledAt, &plan.Status, &lastRunNo, &lastRunStatus, &lastRunResult,
		&lastRunFinishedAt, &hasActiveRun, &plan.PathCount, &plan.CreatedAt, &plan.UpdatedAt,
	); err != nil {
		return model.Plan{}, err
	}
	if maxConcurrency.Valid {
		value := int(maxConcurrency.Int64)
		plan.MaxConcurrency = &value
	}
	if scheduledAt.Valid {
		value := scheduledAt.Time.UTC()
		plan.ScheduledAt = &value
	}
	// 运行事实摘要：子查询为 NULL 表示计划从未运行过；结果与结束时间跟随最近运行可空。
	if lastRunNo.Valid {
		value := uint64(lastRunNo.Int64)
		plan.LastRunNo = &value
	}
	if lastRunStatus.Valid {
		plan.LastRunStatus = model.RunStatus(lastRunStatus.String)
	}
	if lastRunResult.Valid {
		value := model.RunResult(lastRunResult.String)
		plan.LastRunResult = &value
	}
	if lastRunFinishedAt.Valid {
		value := lastRunFinishedAt.Time.UTC()
		plan.LastRunFinishedAt = &value
	}
	plan.HasActiveRun = hasActiveRun
	plan.CreatedAt = plan.CreatedAt.UTC()
	plan.UpdatedAt = plan.UpdatedAt.UTC()
	if strings.TrimSpace(plan.Name) == "" || !model.ValidPlanStatus(plan.Status) {
		return model.Plan{}, repository.ErrPlanDataInvalid
	}
	return plan, nil
}
