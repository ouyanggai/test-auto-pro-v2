package model

import "time"

// TemplateRuleRenderType 是本地规则目录对目标表单入口的统一分类。
type TemplateRuleRenderType string

const (
	TemplateRuleRenderFormMaking TemplateRuleRenderType = "formmaking"
	TemplateRuleRenderVueCustom  TemplateRuleRenderType = "vue_custom"
	TemplateRuleRenderUnknown    TemplateRuleRenderType = "unknown"
)

// TemplateRuleCatalogItem 是一次模板规则分析的持久快照，不保存目标平台原始源码。
type TemplateRuleCatalogItem struct {
	ID                uint64                 `json:"id"`
	SourceTemplateID  string                 `json:"sourceTemplateId"`
	FlowCode          string                 `json:"flowCode"`
	FlowName          string                 `json:"flowName"`
	TemplateType      string                 `json:"templateType"`
	FormExist         string                 `json:"formExist"`
	RenderType        TemplateRuleRenderType `json:"renderType"`
	SourceAccount     string                 `json:"sourceAccount"`
	SourceVersion     string                 `json:"sourceVersion"`
	TargetDigest      string                 `json:"targetDigest"`
	FormMakingDigest  string                 `json:"formmakingDigest"`
	VueSourceDigest   string                 `json:"vueSourceDigest"`
	JavaSourceDigest  string                 `json:"javaSourceDigest"`
	ComponentDigest   string                 `json:"componentDigest"`
	SourceFingerprint string                 `json:"sourceFingerprint"`
	AnalyzerVersion   string                 `json:"analyzerVersion"`
	Status            string                 `json:"status"`
	RuleData          map[string]any         `json:"ruleData"`
	Coverage          map[string]any         `json:"coverage"`
	Issues            []string               `json:"issues"`
	AnalyzedAt        *time.Time             `json:"analyzedAt,omitempty"`
	CreatedAt         time.Time              `json:"createdAt"`
	UpdatedAt         time.Time              `json:"updatedAt"`
}

// TemplateRuleCatalogSummary 是设置页使用的轻量覆盖汇总，避免加载全部规则正文。
type TemplateRuleCatalogSummary struct {
	CatalogTotal         int            `json:"catalogTotal"`
	FormMaking           int            `json:"formmaking"`
	VueCustom            int            `json:"vueCustom"`
	Unknown              int            `json:"unknown"`
	Complete             int            `json:"complete"`
	NeedsAttention       int            `json:"needsAttention"`
	Blocked              int            `json:"blocked"`
	Failed               int            `json:"failed"`
	Components           map[string]int `json:"components"`
	RegisteredComponents int            `json:"registeredComponents"`
	UpdatedAt            *time.Time     `json:"updatedAt,omitempty"`
}

// TemplateRuleCatalogPublicItem 是设置页可见的规则摘要，不包含源模板 ID、摘要、规则正文或覆盖原文。
type TemplateRuleCatalogPublicItem struct {
	FlowCode     string                 `json:"flowCode"`
	FlowName     string                 `json:"flowName"`
	TemplateType string                 `json:"templateType"`
	RenderType   TemplateRuleRenderType `json:"renderType"`
	Status       string                 `json:"status"`
	FieldCount   int                    `json:"fieldCount"`
	Components   []string               `json:"components"`
	Issues       []string               `json:"issues"`
	AnalyzedAt   *time.Time             `json:"analyzedAt,omitempty"`
}

// TemplateRuleAnalysisFailure 记录任务失败页或阶段的公开原因，不保存目标响应、源码或凭证。
type TemplateRuleAnalysisFailure struct {
	Page   int    `json:"page,omitempty"`
	Stage  string `json:"stage"`
	Reason string `json:"reason"`
}

// TemplateRuleAnalysisJob 是规则目录增量、全量或失败重试的后台任务快照。
type TemplateRuleAnalysisJob struct {
	ID                 string                        `json:"id"`
	Mode               string                        `json:"mode"`
	Account            string                        `json:"account"`
	State              string                        `json:"state"`
	Outcome            string                        `json:"outcome,omitempty"`
	Total              int                           `json:"total"`
	Listed             int                           `json:"listed"`
	Accounted          int                           `json:"accounted"`
	Complete           int                           `json:"complete"`
	NeedsAttention     int                           `json:"needsAttention"`
	Blocked            int                           `json:"blocked"`
	Failed             int                           `json:"failed"`
	Unlisted           int                           `json:"unlisted"`
	PaginationComplete bool                          `json:"paginationComplete"`
	Failures           []TemplateRuleAnalysisFailure `json:"failures"`
	Message            string                        `json:"message,omitempty"`
	CreatedAt          time.Time                     `json:"createdAt"`
	UpdatedAt          time.Time                     `json:"updatedAt"`
	FinishedAt         *time.Time                    `json:"finishedAt,omitempty"`
}
