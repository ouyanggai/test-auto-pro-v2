package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// TemplateCatalogService 提供设置页的模板规则目录和后台分析任务。
type TemplateCatalogService interface {
	Summary(context.Context) (model.TemplateRuleCatalogSummary, error)
	List(context.Context, string, int, int) ([]model.TemplateRuleCatalogItem, int, error)
	CreateJob(context.Context, string, string) (model.TemplateRuleAnalysisJob, error)
	GetJob(context.Context, string) (model.TemplateRuleAnalysisJob, error)
	LatestJob(context.Context, string) (model.TemplateRuleAnalysisJob, bool, error)
}

type templateCatalogJobInput struct {
	Account string `json:"account"`
	Mode    string `json:"mode"`
}

type templateCatalogListResponse struct {
	Items []model.TemplateRuleCatalogPublicItem `json:"items"`
	Page  int                                   `json:"page"`
	Size  int                                   `json:"size"`
	Total int                                   `json:"total"`
}

// registerTemplateCatalogRoutes 注册规则目录只读查询与受控同步任务入口。
func registerTemplateCatalogRoutes(mux *http.ServeMux, catalog TemplateCatalogService) {
	mux.HandleFunc("GET /api/settings/template-rules/summary", handleTemplateCatalogSummary(catalog))
	mux.HandleFunc("GET /api/settings/template-rules", handleTemplateCatalogList(catalog))
	mux.HandleFunc("POST /api/settings/template-rules/jobs", handleCreateTemplateCatalogJob(catalog))
	mux.HandleFunc("GET /api/settings/template-rules/jobs/latest", handleLatestTemplateCatalogJob(catalog))
	mux.HandleFunc("GET /api/settings/template-rules/jobs/{jobId}", handleTemplateCatalogJob(catalog))
}

// handleTemplateCatalogSummary 返回已持久化规则的轻量覆盖统计，不触发目标平台读取。
func handleTemplateCatalogSummary(catalog TemplateCatalogService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		summary, err := catalog.Summary(request.Context())
		if err != nil {
			writeTemplateCatalogError(response, err)
			return
		}
		writeSuccess(response, summary)
	}
}

// handleTemplateCatalogList 返回设置页按需查看的规则目录分页。
func handleTemplateCatalogList(catalog TemplateCatalogService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		page, size, ok := parsePagination(response, request)
		if !ok {
			return
		}
		items, total, err := catalog.List(request.Context(), request.URL.Query().Get("query"), page, size)
		if err != nil {
			writeTemplateCatalogError(response, err)
			return
		}
		publicItems := make([]model.TemplateRuleCatalogPublicItem, 0, len(items))
		for _, item := range items {
			publicItems = append(publicItems, publicTemplateRuleCatalogItem(item))
		}
		writeSuccess(response, templateCatalogListResponse{Items: publicItems, Page: page, Size: size, Total: total})
	}
}

// publicTemplateRuleCatalogItem 构造设置页专用摘要，禁止源模板 ID、摘要、版本、规则正文和覆盖原文进入响应。
func publicTemplateRuleCatalogItem(item model.TemplateRuleCatalogItem) model.TemplateRuleCatalogPublicItem {
	components := append(templateRulePublicComponents(item.Coverage["components"], ""), templateRulePublicComponents(item.Coverage["customComponents"], "custom:")...)
	sort.Strings(components)
	return model.TemplateRuleCatalogPublicItem{
		FlowCode: item.FlowCode, FlowName: item.FlowName, TemplateType: item.TemplateType, RenderType: item.RenderType,
		Status: item.Status, Stale: item.Stale, FieldCount: templateRulePublicCount(item.Coverage["fieldCount"]), Components: components,
		Issues: append([]string(nil), item.Issues...), AnalyzedAt: item.AnalyzedAt,
	}
}

// templateRulePublicComponents 从仓储 JSON 或内存分析结果中提取当前模板实际使用的组件名。
func templateRulePublicComponents(raw any, prefix string) []string {
	result := make([]string, 0)
	switch values := raw.(type) {
	case map[string]any:
		for name, count := range values {
			if templateRulePublicCount(count) > 0 && strings.TrimSpace(name) != "" {
				result = append(result, prefix+strings.TrimSpace(name))
			}
		}
	case map[string]int:
		for name, count := range values {
			if count > 0 && strings.TrimSpace(name) != "" {
				result = append(result, prefix+strings.TrimSpace(name))
			}
		}
	}
	return result
}

// templateRulePublicCount 只接受 JSON 解码后的非负整数，异常覆盖数据不进入公开摘要。
func templateRulePublicCount(value any) int {
	switch count := value.(type) {
	case int:
		if count >= 0 {
			return count
		}
	case float64:
		if count >= 0 && count == float64(int(count)) {
			return int(count)
		}
	}
	return 0
}

// handleCreateTemplateCatalogJob 创建全量、增量或失败重试任务；请求不能携带目标地址或任意脚本。
func handleCreateTemplateCatalogJob(catalog TemplateCatalogService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		var input templateCatalogJobInput
		decoder := json.NewDecoder(io.LimitReader(request.Body, maxAPIRequestBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || ensureJSONEnd(decoder) != nil {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "模板规则分析请求格式不正确", false)
			return
		}
		job, err := catalog.CreateJob(request.Context(), strings.TrimSpace(input.Account), strings.TrimSpace(input.Mode))
		if err != nil {
			writeTemplateCatalogError(response, err)
			return
		}
		writeJSON(response, http.StatusAccepted, apiSuccess{Success: true, Data: job})
	}
}

// handleLatestTemplateCatalogJob 返回指定账号最近一次分析状态，刷新页面后可继续观察任务。
func handleLatestTemplateCatalogJob(catalog TemplateCatalogService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		account := strings.TrimSpace(request.URL.Query().Get("account"))
		if account == "" {
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", "缺少规则目录账号", false)
			return
		}
		job, found, err := catalog.LatestJob(request.Context(), account)
		if err != nil {
			writeTemplateCatalogError(response, err)
			return
		}
		if !found {
			writeSuccess(response, nil)
			return
		}
		writeSuccess(response, job)
	}
}

// handleTemplateCatalogJob 返回指定任务的真实持久进度。
func handleTemplateCatalogJob(catalog TemplateCatalogService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		job, err := catalog.GetJob(request.Context(), strings.TrimSpace(request.PathValue("jobId")))
		if err != nil {
			writeTemplateCatalogError(response, err)
			return
		}
		writeSuccess(response, job)
	}
}

// writeTemplateCatalogError 映射规则目录稳定错误，不泄露目标平台接口或源码路径。
func writeTemplateCatalogError(response http.ResponseWriter, err error) {
	var catalogError *service.TemplateCatalogError
	if errors.As(err, &catalogError) {
		switch catalogError.Kind {
		case service.TemplateCatalogErrorInvalid:
			writeFailure(response, http.StatusBadRequest, "INVALID_ARGUMENT", catalogError.Message, false)
		case service.TemplateCatalogErrorActive:
			writeFailure(response, http.StatusConflict, "TEMPLATE_RULE_ANALYSIS_ACTIVE", catalogError.Message, false)
		case service.TemplateCatalogErrorNotFound:
			writeFailure(response, http.StatusNotFound, "TEMPLATE_RULE_NOT_FOUND", catalogError.Message, false)
		default:
			writeFailure(response, http.StatusServiceUnavailable, "TEMPLATE_RULE_CATALOG_UNAVAILABLE", catalogError.Message, true)
		}
		return
	}
	writeFailure(response, http.StatusServiceUnavailable, "TEMPLATE_RULE_CATALOG_UNAVAILABLE", "模板规则目录暂不可用，请重试", true)
}

type unavailableTemplateCatalogService struct{}

// Summary 在默认处理器未注入规则目录时返回稳定不可用错误。
func (unavailableTemplateCatalogService) Summary(context.Context) (model.TemplateRuleCatalogSummary, error) {
	return model.TemplateRuleCatalogSummary{}, &service.TemplateCatalogError{Kind: service.TemplateCatalogErrorStorage, Message: "模板规则目录暂不可用"}
}

// List 在默认处理器未注入规则目录时返回稳定不可用错误。
func (unavailableTemplateCatalogService) List(context.Context, string, int, int) ([]model.TemplateRuleCatalogItem, int, error) {
	return nil, 0, &service.TemplateCatalogError{Kind: service.TemplateCatalogErrorStorage, Message: "模板规则目录暂不可用"}
}

// CreateJob 在默认处理器未注入规则目录时返回稳定不可用错误。
func (unavailableTemplateCatalogService) CreateJob(context.Context, string, string) (model.TemplateRuleAnalysisJob, error) {
	return model.TemplateRuleAnalysisJob{}, &service.TemplateCatalogError{Kind: service.TemplateCatalogErrorStorage, Message: "模板规则目录暂不可用"}
}

// GetJob 在默认处理器未注入规则目录时返回稳定不可用错误。
func (unavailableTemplateCatalogService) GetJob(context.Context, string) (model.TemplateRuleAnalysisJob, error) {
	return model.TemplateRuleAnalysisJob{}, &service.TemplateCatalogError{Kind: service.TemplateCatalogErrorStorage, Message: "模板规则目录暂不可用"}
}

// LatestJob 在默认处理器未注入规则目录时返回稳定不可用错误。
func (unavailableTemplateCatalogService) LatestJob(context.Context, string) (model.TemplateRuleAnalysisJob, bool, error) {
	return model.TemplateRuleAnalysisJob{}, false, &service.TemplateCatalogError{Kind: service.TemplateCatalogErrorStorage, Message: "模板规则目录暂不可用"}
}
