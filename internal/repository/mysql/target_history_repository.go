package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/repository"
)

// TargetHistoryRepository 只读查询目标业务库中的实例关键列。
// 目标只读 API 的实例列表需要逐页拉取并逐行解析人员目录，账号历史达到数千条时明显变慢；
// 这里用一次带索引的联表查询取回列表所需的最少列，表单正文仍只能通过目标只读协议读取。
type TargetHistoryRepository struct {
	db             *sql.DB
	schema         string
	userCenter     string
	customerCode   string
	verifiedTables bool
}

// NewTargetHistoryRepository 打开目标业务库只读连接，并核对必需表存在。
// 库名与用户中心库名都必须通过标识符白名单，避免任意 SQL 片段拼入查询。
func NewTargetHistoryRepository(ctx context.Context, cfg config.TargetBizDBConfig, customerCode string) (*TargetHistoryRepository, error) {
	if !cfg.Enabled() {
		return nil, errors.New("目标业务库只读配置未启用")
	}
	if strings.TrimSpace(customerCode) == "" {
		return nil, errors.New("目标业务库查询缺少客户编码，拒绝跨租户读取")
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=false&charset=utf8mb4&loc=Local&timeout=5s&readTimeout=15s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	repo := &TargetHistoryRepository{
		db: db, schema: cfg.Name, userCenter: cfg.UserCenterName, customerCode: strings.TrimSpace(customerCode),
	}
	if err := repo.verifyTables(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return repo, nil
}

// Close 释放目标业务库只读连接池。
func (r *TargetHistoryRepository) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

// verifyTables 证明连接的确是目标工作流库，缺表时拒绝启用快速候选查询。
func (r *TargetHistoryRepository) verifyTables(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, `
SELECT table_name FROM information_schema.tables
WHERE table_schema = ? AND table_name IN ('t_flow_instance', 't_flow_proxy', 't_form_proxy')`, r.schema)
	if err != nil {
		return err
	}
	defer rows.Close()
	found := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		found[strings.ToLower(name)] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, required := range []string{"t_flow_instance", "t_flow_proxy", "t_form_proxy"} {
		if !found[required] {
			return fmt.Errorf("目标业务库缺少只读候选查询所需的表：%s", required)
		}
	}
	return nil
}

// TargetHistoryCandidates 按流程身份分页返回候选关键列与真实总数，不限制发起人。
func (r *TargetHistoryRepository) TargetHistoryCandidates(ctx context.Context, filter repository.TargetHistoryCandidateFilter, page, pageSize int) ([]repository.TargetHistoryCandidateRow, int, error) {
	if r == nil || r.db == nil {
		return nil, 0, errors.New("目标业务库只读连接不可用")
	}
	flowName := strings.TrimSpace(filter.FlowName)
	if flowName == "" {
		return []repository.TargetHistoryCandidateRow{}, 0, nil
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	where, args := r.candidateConditions(filter, flowName)
	total, err := r.candidateTotal(ctx, where, args)
	if err != nil || total == 0 {
		return []repository.TargetHistoryCandidateRow{}, total, err
	}
	// 已完成实例整体优先，其余按发起时间倒序；排序完全来自目标业务库原字段。
	query := fmt.Sprintf(`
SELECT i.id, COALESCE(i.name, ''), COALESCE(p.flow_name, ''), COALESCE(f.name, ''), COALESCE(NULLIF(i.flow_code, ''), COALESCE(p.code, '')),
       COALESCE(i.flow_proxy_id, ''), COALESCE(i.form_proxy_id, ''), COALESCE(i.status, ''),
       COALESCE(u.name, ''), COALESCE(c.name, ''), COALESCE(DATE_FORMAT(i.create_date, '%%Y-%%m-%%d %%H:%%i:%%s'), '')
FROM %[1]s.t_flow_instance i
JOIN %[1]s.t_flow_proxy p ON p.id = i.flow_proxy_id
LEFT JOIN %[1]s.t_form_proxy f ON f.id = i.form_proxy_id
LEFT JOIN %[2]s.t_user u ON u.id = i.creater_id
LEFT JOIN %[2]s.t_company c ON c.id = i.company_id
WHERE %[3]s
ORDER BY (i.status = 'end') DESC, i.create_date DESC, i.id DESC
LIMIT ? OFFSET ?`, r.schema, r.userCenter, where)
	rows, err := r.db.QueryContext(ctx, query, append(append([]any{}, args...), pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := make([]repository.TargetHistoryCandidateRow, 0, pageSize)
	for rows.Next() {
		var row repository.TargetHistoryCandidateRow
		if err := rows.Scan(&row.InstanceID, &row.Name, &row.FlowName, &row.FormName, &row.FlowCode,
			&row.FlowProxyID, &row.FormProxyID, &row.Status, &row.InitiatorName, &row.CompanyName, &row.CreatedAt); err != nil {
			return nil, 0, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return result, total, nil
}

// candidateConditions 组装只使用目标原字段的查询条件；所有用户输入都走占位符。
func (r *TargetHistoryRepository) candidateConditions(filter repository.TargetHistoryCandidateFilter, flowName string) (string, []any) {
	conditions := []string{"i.is_delete = 0", "i.customer_code = ?", "p.flow_name = ?"}
	args := []any{r.customerCode, flowName}
	if flowCode := strings.TrimSpace(filter.FlowCode); flowCode != "" {
		// 实例或其私有代理写入了不同流程编码时精确排除；两处都没有编码时只能依靠流程名称身份。
		conditions = append(conditions, "(COALESCE(NULLIF(i.flow_code, ''), COALESCE(p.code, '')) = '' OR COALESCE(NULLIF(i.flow_code, ''), COALESCE(p.code, '')) = ?)")
		args = append(args, flowCode)
	}
	if formName := strings.TrimSpace(filter.FormName); formName != "" {
		// 有表单时只接受同名表单；该实例没有表单代理时退回流程名称身份。
		conditions = append(conditions, "(COALESCE(f.name, '') = '' OR COALESCE(f.name, '') = ?)")
		args = append(args, formName)
	}
	for _, keyword := range filter.ExcludeNameKeywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		conditions = append(conditions, "COALESCE(i.name, '') NOT LIKE ?")
		args = append(args, "%"+escapeTargetHistoryLike(keyword)+"%")
	}
	if query := strings.TrimSpace(filter.Query); query != "" {
		conditions = append(conditions, "(COALESCE(i.name, '') LIKE ? OR COALESCE(u.name, '') LIKE ? OR COALESCE(c.name, '') LIKE ?)")
		pattern := "%" + escapeTargetHistoryLike(query) + "%"
		args = append(args, pattern, pattern, pattern)
	}
	return strings.Join(conditions, " AND "), args
}

// candidateTotal 返回与列表同一条件下的真实总数，供分页器显示。
func (r *TargetHistoryRepository) candidateTotal(ctx context.Context, where string, args []any) (int, error) {
	query := fmt.Sprintf(`
SELECT COUNT(*)
FROM %[1]s.t_flow_instance i
JOIN %[1]s.t_flow_proxy p ON p.id = i.flow_proxy_id
LEFT JOIN %[1]s.t_form_proxy f ON f.id = i.form_proxy_id
LEFT JOIN %[2]s.t_user u ON u.id = i.creater_id
LEFT JOIN %[2]s.t_company c ON c.id = i.company_id
WHERE %[3]s`, r.schema, r.userCenter, where)
	var total int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// escapeTargetHistoryLike 转义用户搜索词中的通配符，避免把 % 和 _ 当作模式。
func escapeTargetHistoryLike(value string) string {
	replaced := strings.ReplaceAll(value, "\\", "\\\\")
	replaced = strings.ReplaceAll(replaced, "%", "\\%")
	return strings.ReplaceAll(replaced, "_", "\\_")
}
