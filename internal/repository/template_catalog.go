package repository

import (
	"context"
	"errors"

	"test-auto-pro-v2/internal/model"
)

var (
	// ErrTemplateCatalogNotFound 表示规则或分析任务不存在。
	ErrTemplateCatalogNotFound = errors.New("模板规则不存在")
	// ErrTemplateCatalogActive 表示同一账号已有活动分析任务。
	ErrTemplateCatalogActive = errors.New("模板规则分析任务正在运行")
)

// TemplateCatalogRepository 持久化全模板规则快照和分析任务检查点。
type TemplateCatalogRepository interface {
	Upsert(context.Context, model.TemplateRuleCatalogItem) (model.TemplateRuleCatalogItem, error)
	GetByFlowCode(context.Context, string) (model.TemplateRuleCatalogItem, bool, error)
	GetBySourceTemplateID(context.Context, string) (model.TemplateRuleCatalogItem, bool, error)
	MarkStale(context.Context, string) error
	List(context.Context, string, int, int) ([]model.TemplateRuleCatalogItem, int, error)
	Summary(context.Context) (model.TemplateRuleCatalogSummary, error)
	CreateJob(context.Context, model.TemplateRuleAnalysisJob) (model.TemplateRuleAnalysisJob, error)
	GetJob(context.Context, string) (model.TemplateRuleAnalysisJob, error)
	LatestJob(context.Context, string) (model.TemplateRuleAnalysisJob, bool, error)
	UpdateJob(context.Context, model.TemplateRuleAnalysisJob) error
	MarkInterruptedJobs(context.Context, string) error
}
