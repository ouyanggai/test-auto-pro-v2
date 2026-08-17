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

// List 按名称和状态筛选计划，并从路径表实时统计 pathCount。
func (r *PlanRepository) List(ctx context.Context, filter model.PlanListFilter) ([]model.Plan, error) {
	query := `
SELECT id, name, account, account_display_name, flow_source, target_object_id,
       target_object_name, run_mode, max_concurrency, scheduled_at, status,
       (SELECT COUNT(*) FROM test_execution_paths ep WHERE ep.plan_id = test_plans.id) AS path_count,
       created_at, updated_at
FROM test_plans
WHERE (? = '' OR name LIKE CONCAT('%', ?, '%'))
  AND (? = '' OR status = ?)
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
       target_object_name, run_mode, max_concurrency, scheduled_at, status,
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
       target_object_name, run_mode, max_concurrency, scheduled_at, status,
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
	if err := row.Scan(
		&plan.ID, &plan.Name, &plan.Account, &plan.AccountDisplayName, &plan.FlowSource,
		&plan.TargetObjectID, &plan.TargetObjectName, &plan.RunMode, &maxConcurrency,
		&scheduledAt, &plan.Status, &plan.PathCount, &plan.CreatedAt, &plan.UpdatedAt,
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
	plan.CreatedAt = plan.CreatedAt.UTC()
	plan.UpdatedAt = plan.UpdatedAt.UTC()
	if strings.TrimSpace(plan.Name) == "" || !model.ValidPlanStatus(plan.Status) {
		return model.Plan{}, repository.ErrPlanDataInvalid
	}
	return plan, nil
}
