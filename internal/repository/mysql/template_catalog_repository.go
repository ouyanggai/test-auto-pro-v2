package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// TemplateCatalogRepository 是本地全模板规则目录的 MySQL 实现。
type TemplateCatalogRepository struct {
	db *sql.DB
}

// NewTemplateCatalogRepository 创建模板规则目录仓储。
func NewTemplateCatalogRepository(db *sql.DB) *TemplateCatalogRepository {
	return &TemplateCatalogRepository{db: db}
}

// Upsert 保存同一目标模板的最新分析结果，规则正文和覆盖统计始终整份替换。
func (r *TemplateCatalogRepository) Upsert(ctx context.Context, item model.TemplateRuleCatalogItem) (model.TemplateRuleCatalogItem, error) {
	ruleData, err := encodeTemplateCatalogJSON(item.RuleData, map[string]any{})
	if err != nil {
		return model.TemplateRuleCatalogItem{}, err
	}
	coverage, err := encodeTemplateCatalogJSON(item.Coverage, map[string]any{})
	if err != nil {
		return model.TemplateRuleCatalogItem{}, err
	}
	issues, err := encodeTemplateCatalogJSON(item.Issues, []string{})
	if err != nil {
		return model.TemplateRuleCatalogItem{}, err
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = item.CreatedAt
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO test_template_rule_catalog
(source_template_id, flow_code, flow_name, template_type, form_exist, render_type, source_account,
 source_version, target_digest, formmaking_digest, vue_source_digest, java_source_digest, component_digest,
 source_fingerprint, analyzer_version, status, stale, rule_data, coverage, issues, analyzed_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE flow_code = VALUES(flow_code), flow_name = VALUES(flow_name), template_type = VALUES(template_type),
 form_exist = VALUES(form_exist), render_type = VALUES(render_type), source_account = VALUES(source_account),
 source_version = VALUES(source_version), target_digest = VALUES(target_digest), formmaking_digest = VALUES(formmaking_digest),
 vue_source_digest = VALUES(vue_source_digest), java_source_digest = VALUES(java_source_digest), component_digest = VALUES(component_digest),
 source_fingerprint = VALUES(source_fingerprint), analyzer_version = VALUES(analyzer_version),
 status = VALUES(status), stale = VALUES(stale), rule_data = VALUES(rule_data), coverage = VALUES(coverage), issues = VALUES(issues),
 analyzed_at = VALUES(analyzed_at), updated_at = VALUES(updated_at)`,
		strings.TrimSpace(item.SourceTemplateID), strings.TrimSpace(item.FlowCode), strings.TrimSpace(item.FlowName),
		strings.TrimSpace(item.TemplateType), strings.TrimSpace(item.FormExist), string(item.RenderType), strings.TrimSpace(item.SourceAccount),
		strings.TrimSpace(item.SourceVersion), strings.TrimSpace(item.TargetDigest), strings.TrimSpace(item.FormMakingDigest),
		strings.TrimSpace(item.VueSourceDigest), strings.TrimSpace(item.JavaSourceDigest), strings.TrimSpace(item.ComponentDigest),
		strings.TrimSpace(item.SourceFingerprint), strings.TrimSpace(item.AnalyzerVersion),
		strings.TrimSpace(item.Status), item.Stale, ruleData, coverage, issues, item.AnalyzedAt, item.CreatedAt.UTC(), item.UpdatedAt.UTC())
	if err != nil {
		return model.TemplateRuleCatalogItem{}, err
	}
	stored, found, err := r.GetBySourceTemplateID(ctx, item.SourceTemplateID)
	if err != nil {
		return model.TemplateRuleCatalogItem{}, err
	}
	if !found {
		return model.TemplateRuleCatalogItem{}, repository.ErrTemplateCatalogNotFound
	}
	return stored, nil
}

// GetByFlowCode 按真实流程编码读取最新规则，供路径配置和批量生成复用。
func (r *TemplateCatalogRepository) GetByFlowCode(ctx context.Context, flowCode string) (model.TemplateRuleCatalogItem, bool, error) {
	row := r.db.QueryRowContext(ctx, templateCatalogSelect+" WHERE flow_code = ? ORDER BY updated_at DESC, id DESC LIMIT 1", strings.TrimSpace(flowCode))
	return scanTemplateCatalogItem(row)
}

// GetBySourceTemplateID 按目标模板 ID 读取规则，用于同步任务幂等更新后的回读。
func (r *TemplateCatalogRepository) GetBySourceTemplateID(ctx context.Context, templateID string) (model.TemplateRuleCatalogItem, bool, error) {
	row := r.db.QueryRowContext(ctx, templateCatalogSelect+" WHERE source_template_id = ?", strings.TrimSpace(templateID))
	return scanTemplateCatalogItem(row)
}

// MarkStale 原子标记目录规则待更新，并把关联计划的已保存表单值保留为需处理状态。
func (r *TemplateCatalogRepository) MarkStale(ctx context.Context, templateID string) error {
	templateID = strings.TrimSpace(templateID)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "UPDATE test_template_rule_catalog SET stale = 1 WHERE source_template_id = ?", templateID)
	if err != nil {
		return err
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr != nil {
		return affectedErr
	} else if affected == 0 {
		return repository.ErrTemplateCatalogNotFound
	}
	// 模板过期只改变本地准备状态，完整 values 必须保留；目录刷新成功也不能冒充旧值已经重新复验。
	_, err = tx.ExecContext(ctx, `UPDATE test_execution_path_configs AS config
JOIN test_execution_paths AS path ON path.id = config.path_id
JOIN test_plans AS plan ON plan.id = path.plan_id
SET config.form_status = 'affected', config.data_status = 'needs_attention', config.form_validated = 0,
    config.updated_at = ?
WHERE plan.flow_source = 'new' AND plan.target_object_id = ?`, time.Now().UTC(), templateID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// List 按更新时间分页返回规则目录轻量列表，规则正文仍完整返回给设置页按需查看。
func (r *TemplateCatalogRepository) List(ctx context.Context, query string, offset, limit int) ([]model.TemplateRuleCatalogItem, int, error) {
	if offset < 0 {
		offset = 0
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	pattern := "%" + strings.TrimSpace(query) + "%"
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_template_rule_catalog WHERE flow_code LIKE ? OR flow_name LIKE ?", pattern, pattern).Scan(&total); err != nil {
		return nil, 0, err
	}
	// 先在窄字段子查询中完成排序和分页，再回表读取规则 JSON；否则 MySQL 会把大段规则正文放入排序缓冲并触发 1038。
	rows, err := r.db.QueryContext(ctx, templateCatalogPageSelect, pattern, pattern, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]model.TemplateRuleCatalogItem, 0, limit)
	for rows.Next() {
		item, _, scanErr := scanTemplateCatalogItem(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

// Summary 汇总渲染类型、状态和组件覆盖数，避免设置页扫描规则正文。
func (r *TemplateCatalogRepository) Summary(ctx context.Context) (model.TemplateRuleCatalogSummary, error) {
	result := model.TemplateRuleCatalogSummary{Components: map[string]int{}}
	rows, err := r.db.QueryContext(ctx, `SELECT render_type, status, stale, COUNT(*) FROM test_template_rule_catalog GROUP BY render_type, status, stale`)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var renderType, status string
		var stale bool
		var count int
		if err := rows.Scan(&renderType, &status, &stale, &count); err != nil {
			return result, err
		}
		result.CatalogTotal += count
		switch model.TemplateRuleRenderType(renderType) {
		case model.TemplateRuleRenderFormMaking:
			result.FormMaking += count
		case model.TemplateRuleRenderVueCustom:
			result.VueCustom += count
		case model.TemplateRuleRenderUnknown:
			result.Unknown += count
		}
		switch status {
		case "complete":
			result.Complete += count
		case "needs_attention":
			result.NeedsAttention += count
		case "blocked":
			result.Blocked += count
		case "failed":
			result.Failed += count
		}
		if stale {
			result.Stale += count
		}
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	var updated sql.NullTime
	if err := r.db.QueryRowContext(ctx, "SELECT MAX(updated_at) FROM test_template_rule_catalog").Scan(&updated); err != nil {
		return result, err
	}
	if updated.Valid {
		value := updated.Time.UTC()
		result.UpdatedAt = &value
	}
	componentRows, err := r.db.QueryContext(ctx, "SELECT coverage FROM test_template_rule_catalog")
	if err != nil {
		return result, err
	}
	defer componentRows.Close()
	for componentRows.Next() {
		var raw string
		if err := componentRows.Scan(&raw); err != nil {
			return result, err
		}
		var coverage map[string]any
		if err := json.Unmarshal([]byte(raw), &coverage); err != nil {
			return result, err
		}
		for name, count := range templateCatalogComponentCounts(coverage["components"]) {
			result.Components[name] += count
		}
		for name := range templateCatalogComponentNames(coverage["customComponents"]) {
			result.Components["custom:"+name]++
		}
	}
	if err := componentRows.Err(); err != nil {
		return result, err
	}
	return result, nil
}

// CreateJob 创建同一账号唯一活动分析任务，避免全量同步并发写入目录。
func (r *TemplateCatalogRepository) CreateJob(ctx context.Context, job model.TemplateRuleAnalysisJob) (model.TemplateRuleAnalysisJob, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.TemplateRuleAnalysisJob{}, err
	}
	defer tx.Rollback()
	var activeID string
	err = tx.QueryRowContext(ctx, "SELECT id FROM test_template_rule_analysis_jobs WHERE account = ? AND state IN ('queued', 'running') ORDER BY created_at DESC LIMIT 1 FOR UPDATE", job.Account).Scan(&activeID)
	if err == nil {
		return model.TemplateRuleAnalysisJob{}, repository.ErrTemplateCatalogActive
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return model.TemplateRuleAnalysisJob{}, err
	}
	failures, err := encodeTemplateCatalogJSON(job.Failures, []model.TemplateRuleAnalysisFailure{})
	if err != nil {
		return model.TemplateRuleAnalysisJob{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO test_template_rule_analysis_jobs
(id, mode, account, state, outcome, total_count, listed_count, accounted_count, complete_count, needs_attention_count,
 blocked_count, failed_count, unlisted_count, pagination_complete, failures, message, created_at, updated_at, finished_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.Mode, job.Account, job.State, job.Outcome, job.Total, job.Listed, job.Accounted, job.Complete,
		job.NeedsAttention, job.Blocked, job.Failed, job.Unlisted, job.PaginationComplete, failures, job.Message,
		job.CreatedAt.UTC(), job.UpdatedAt.UTC(), job.FinishedAt)
	if err != nil {
		return model.TemplateRuleAnalysisJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.TemplateRuleAnalysisJob{}, err
	}
	return job, nil
}

// GetJob 读取规则分析任务的真实进度。
func (r *TemplateCatalogRepository) GetJob(ctx context.Context, jobID string) (model.TemplateRuleAnalysisJob, error) {
	job, found, err := scanTemplateRuleAnalysisJob(r.db.QueryRowContext(ctx, templateRuleAnalysisJobSelect+" WHERE id = ?", strings.TrimSpace(jobID)))
	if err != nil {
		return model.TemplateRuleAnalysisJob{}, err
	}
	if !found {
		return model.TemplateRuleAnalysisJob{}, repository.ErrTemplateCatalogNotFound
	}
	return job, nil
}

// LatestJob 返回账号最近一次分析任务，供设置页刷新后恢复进度。
func (r *TemplateCatalogRepository) LatestJob(ctx context.Context, account string) (model.TemplateRuleAnalysisJob, bool, error) {
	return scanTemplateRuleAnalysisJob(r.db.QueryRowContext(ctx, templateRuleAnalysisJobSelect+" WHERE account = ? ORDER BY created_at DESC LIMIT 1", strings.TrimSpace(account)))
}

// UpdateJob 原子替换任务进度和终态字段。
func (r *TemplateCatalogRepository) UpdateJob(ctx context.Context, job model.TemplateRuleAnalysisJob) error {
	failures, encodeErr := encodeTemplateCatalogJSON(job.Failures, []model.TemplateRuleAnalysisFailure{})
	if encodeErr != nil {
		return encodeErr
	}
	_, err := r.db.ExecContext(ctx, `UPDATE test_template_rule_analysis_jobs SET state = ?, outcome = ?, total_count = ?, listed_count = ?, accounted_count = ?, complete_count = ?, needs_attention_count = ?, blocked_count = ?, failed_count = ?, unlisted_count = ?, pagination_complete = ?, failures = ?, message = ?, updated_at = ?, finished_at = ? WHERE id = ?`,
		job.State, job.Outcome, job.Total, job.Listed, job.Accounted, job.Complete, job.NeedsAttention, job.Blocked,
		job.Failed, job.Unlisted, job.PaginationComplete, failures, job.Message, job.UpdatedAt.UTC(), job.FinishedAt, job.ID)
	return err
}

// MarkInterruptedJobs 将进程重启时遗留的活动任务收敛为可重试失败态，禁止新的同步被永久阻塞。
func (r *TemplateCatalogRepository) MarkInterruptedJobs(ctx context.Context, message string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `UPDATE test_template_rule_analysis_jobs
SET state = 'finished', outcome = 'failed', unlisted_count = GREATEST(total_count - complete_count - needs_attention_count - blocked_count - failed_count, 0),
	accounted_count = total_count,
	failures = JSON_ARRAY_APPEND(failures, '$', JSON_OBJECT('stage', 'service_recovery', 'reason', ?)),
	message = ?, updated_at = ?, finished_at = ?
WHERE state IN ('queued', 'running')`, strings.TrimSpace(message), strings.TrimSpace(message), now, now)
	return err
}

// templateCatalogComponentCounts 兼容 JSON 解码后的组件计数对象，异常数据不进入汇总。
func templateCatalogComponentCounts(raw any) map[string]int {
	result := map[string]int{}
	values, ok := raw.(map[string]any)
	if !ok {
		return result
	}
	for name, value := range values {
		switch count := value.(type) {
		case float64:
			if count > 0 && count == float64(int(count)) {
				result[name] = int(count)
			}
		case int:
			if count > 0 {
				result[name] = count
			}
		}
	}
	return result
}

// templateCatalogComponentNames 兼容 JSON 解码后的自定义组件映射，只保留可公开的组件名。
func templateCatalogComponentNames(raw any) map[string]struct{} {
	result := map[string]struct{}{}
	values, ok := raw.(map[string]any)
	if !ok {
		return result
	}
	for name := range values {
		if strings.TrimSpace(name) != "" {
			result[name] = struct{}{}
		}
	}
	return result
}

const templateCatalogSelect = `SELECT id, source_template_id, flow_code, flow_name, template_type, form_exist, render_type,
source_account, source_version, target_digest, formmaking_digest, vue_source_digest, java_source_digest, component_digest,
source_fingerprint, analyzer_version, status, stale, rule_data, coverage, issues, analyzed_at, created_at, updated_at
FROM test_template_rule_catalog`

const templateCatalogPageSelect = `SELECT catalog.id, catalog.source_template_id, catalog.flow_code, catalog.flow_name,
catalog.template_type, catalog.form_exist, catalog.render_type, catalog.source_account, catalog.source_version,
catalog.target_digest, catalog.formmaking_digest, catalog.vue_source_digest, catalog.java_source_digest, catalog.component_digest,
catalog.source_fingerprint, catalog.analyzer_version, catalog.status, catalog.stale, catalog.rule_data, catalog.coverage, catalog.issues,
catalog.analyzed_at, catalog.created_at, catalog.updated_at
FROM test_template_rule_catalog AS catalog
INNER JOIN (
  SELECT id, stale, updated_at FROM test_template_rule_catalog
  WHERE flow_code LIKE ? OR flow_name LIKE ?
  ORDER BY stale DESC, updated_at DESC, id DESC LIMIT ? OFFSET ?
) AS page ON page.id = catalog.id
ORDER BY page.stale DESC, page.updated_at DESC, page.id DESC`

type templateCatalogScanner interface {
	Scan(...any) error
}

// scanTemplateCatalogItem 解码规则 JSON 并统一处理空数组，禁止 null 进入业务层。
func scanTemplateCatalogItem(row templateCatalogScanner) (model.TemplateRuleCatalogItem, bool, error) {
	var item model.TemplateRuleCatalogItem
	var renderType, ruleData, coverage, issues string
	if err := row.Scan(&item.ID, &item.SourceTemplateID, &item.FlowCode, &item.FlowName, &item.TemplateType, &item.FormExist, &renderType,
		&item.SourceAccount, &item.SourceVersion, &item.TargetDigest, &item.FormMakingDigest, &item.VueSourceDigest,
		&item.JavaSourceDigest, &item.ComponentDigest, &item.SourceFingerprint, &item.AnalyzerVersion, &item.Status, &item.Stale,
		&ruleData, &coverage, &issues, &item.AnalyzedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.TemplateRuleCatalogItem{}, false, nil
		}
		return model.TemplateRuleCatalogItem{}, false, err
	}
	item.RenderType = model.TemplateRuleRenderType(renderType)
	if err := json.Unmarshal([]byte(ruleData), &item.RuleData); err != nil {
		return model.TemplateRuleCatalogItem{}, false, err
	}
	if err := json.Unmarshal([]byte(coverage), &item.Coverage); err != nil {
		return model.TemplateRuleCatalogItem{}, false, err
	}
	if err := json.Unmarshal([]byte(issues), &item.Issues); err != nil {
		return model.TemplateRuleCatalogItem{}, false, err
	}
	if item.RuleData == nil {
		item.RuleData = map[string]any{}
	}
	if item.Coverage == nil {
		item.Coverage = map[string]any{}
	}
	if item.Issues == nil {
		item.Issues = []string{}
	}
	return item, true, nil
}

const templateRuleAnalysisJobSelect = `SELECT id, mode, account, state, outcome, total_count, listed_count, accounted_count, complete_count,
needs_attention_count, blocked_count, failed_count, unlisted_count, pagination_complete, failures, message, created_at, updated_at, finished_at FROM test_template_rule_analysis_jobs`

// scanTemplateRuleAnalysisJob 解析规则分析任务并区分不存在与数据库错误。
func scanTemplateRuleAnalysisJob(row templateCatalogScanner) (model.TemplateRuleAnalysisJob, bool, error) {
	var job model.TemplateRuleAnalysisJob
	var failures string
	if err := row.Scan(&job.ID, &job.Mode, &job.Account, &job.State, &job.Outcome, &job.Total, &job.Listed, &job.Accounted, &job.Complete, &job.NeedsAttention, &job.Blocked, &job.Failed, &job.Unlisted, &job.PaginationComplete, &failures, &job.Message, &job.CreatedAt, &job.UpdatedAt, &job.FinishedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.TemplateRuleAnalysisJob{}, false, nil
		}
		return model.TemplateRuleAnalysisJob{}, false, err
	}
	if err := json.Unmarshal([]byte(failures), &job.Failures); err != nil {
		return model.TemplateRuleAnalysisJob{}, false, err
	}
	if job.Failures == nil {
		job.Failures = []model.TemplateRuleAnalysisFailure{}
	}
	return job, true, nil
}

// encodeTemplateCatalogJSON 将 nil 规则集合编码为稳定 JSON 对象或数组。
func encodeTemplateCatalogJSON(value any, fallback any) (string, error) {
	if value == nil {
		value = fallback
	}
	data, err := json.Marshal(value)
	return string(data), err
}
