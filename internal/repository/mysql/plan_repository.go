package mysql

import (
	"context"
	"database/sql"
	"errors"
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
// syncPlanRunStatus 负责（路径配置锁定等写侧判断仍读存储列）。
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
LIMIT ?`
	rows, err := r.db.QueryContext(ctx, query, filter.Name, filter.Name, filter.Status, filter.Status, filter.Limit)
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

// Get 按主键读取计划详情及真实路径数量。
func (r *PlanRepository) Get(ctx context.Context, id uint64) (model.Plan, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, name, account, account_display_name, flow_source, target_object_id,
       target_object_name, run_mode, max_concurrency, scheduled_at,
       ` + planDerivedStatusExpr + ` AS status,
       ` + planRunSummarySelect + `,
       (SELECT COUNT(*) FROM test_execution_paths ep WHERE ep.plan_id = test_plans.id) AS path_count,
       created_at, updated_at
FROM test_plans WHERE id = ?`, id)
	plan, err := scanPlan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Plan{}, repository.ErrPlanNotFound
	}
	return plan, err
}

// Delete 删除一个计划；路径及其工具侧配置由外键级联清理，绝不访问目标平台。
func (r *PlanRepository) Delete(ctx context.Context, id uint64) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM test_plans WHERE id = ?", id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return repository.ErrPlanNotFound
	}
	return nil
}

// getByCreateKey 读取计划创建幂等键对应的原记录。
func (r *PlanRepository) getByCreateKey(ctx context.Context, createKey string) (model.Plan, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, name, account, account_display_name, flow_source, target_object_id,
       target_object_name, run_mode, max_concurrency, scheduled_at,
       ` + planDerivedStatusExpr + ` AS status,
       ` + planRunSummarySelect + `,
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
