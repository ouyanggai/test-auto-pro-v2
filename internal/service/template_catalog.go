package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/formdata"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

const templateCatalogAnalyzerVersion = "f010r-v2"
const templateCatalogSourceAccount = "欧阳改"

// TemplateCatalogService 提供本地规则目录的查询和可恢复分析任务。
type TemplateCatalogService struct {
	target     TemplateCatalogTargetReader
	repository repository.TemplateCatalogRepository
	pages      *vuePageRuleScanner
	now        func() time.Time
	mu         sync.Mutex
	running    map[string]context.CancelFunc
}

// TemplateCatalogTargetReader 是同步阶段唯一允许访问目标平台的只读能力。
type TemplateCatalogTargetReader interface {
	Templates(context.Context, string, string, int, int) (target.Page[target.FlowTemplate], error)
	TemplateConfiguration(context.Context, string, string) (target.PathConfigurationSnapshot, error)
}

// NewTemplateCatalogService 创建本地规则目录服务；计划配置阶段不依赖目标平台分析。
func NewTemplateCatalogService(targetReader TemplateCatalogTargetReader, repository repository.TemplateCatalogRepository, workspaceRoot string) *TemplateCatalogService {
	return &TemplateCatalogService{target: targetReader, repository: repository, pages: newVuePageRuleScanner(workspaceRoot), now: time.Now, running: map[string]context.CancelFunc{}}
}

// AnalyzeVueCustomPageRule 读取本地宿主源码并返回单个 Vue 业务页面的规则投影，供目录同步和回归测试共用。
func AnalyzeVueCustomPageRule(workspaceRoot, flowCode, formExist string) target.VueCustomPageRule {
	return newVuePageRuleScanner(workspaceRoot).Rule(flowCode, formExist)
}

// Summary 返回设置页的规则覆盖汇总。
func (s *TemplateCatalogService) Summary(ctx context.Context) (model.TemplateRuleCatalogSummary, error) {
	return s.repository.Summary(ctx)
}

// List 返回规则目录的有界分页列表。
func (s *TemplateCatalogService) List(ctx context.Context, query string, page, pageSize int) ([]model.TemplateRuleCatalogItem, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}
	return s.repository.List(ctx, query, (page-1)*pageSize, pageSize)
}

// GetByFlowCode 返回路径生成器按流程编码读取的本地规则。
func (s *TemplateCatalogService) GetByFlowCode(ctx context.Context, flowCode string) (model.TemplateRuleCatalogItem, bool, error) {
	return s.repository.GetByFlowCode(ctx, strings.TrimSpace(flowCode))
}

// Recover 将服务中断遗留的目录任务释放为可重试失败态，防止活动锁永久阻塞新的全量分析。
func (s *TemplateCatalogService) Recover(ctx context.Context) error {
	if err := s.repository.MarkInterruptedJobs(ctx, "规则目录分析因服务重启中断，请重试"); err != nil {
		return &TemplateCatalogError{Kind: TemplateCatalogErrorStorage, Message: "模板规则目录任务恢复失败，请重试"}
	}
	return nil
}

// CreateJob 创建增量、全量或失败重试分析任务，并立即交给后台 Worker。
func (s *TemplateCatalogService) CreateJob(ctx context.Context, account, mode string) (model.TemplateRuleAnalysisJob, error) {
	account = strings.TrimSpace(account)
	mode = strings.TrimSpace(mode)
	if account != templateCatalogSourceAccount || !validTemplateCatalogMode(mode) {
		return model.TemplateRuleAnalysisJob{}, &TemplateCatalogError{Kind: TemplateCatalogErrorInvalid, Message: "模板规则分析参数不正确"}
	}
	now := s.now().UTC()
	job := model.TemplateRuleAnalysisJob{ID: newTemplateCatalogJobID(now), Mode: mode, Account: account, Status: "queued", CreatedAt: now, UpdatedAt: now}
	created, err := s.repository.CreateJob(ctx, job)
	if err != nil {
		if errors.Is(err, repository.ErrTemplateCatalogActive) {
			return model.TemplateRuleAnalysisJob{}, &TemplateCatalogError{Kind: TemplateCatalogErrorActive, Message: "模板规则分析正在进行，请等待当前任务完成"}
		}
		return model.TemplateRuleAnalysisJob{}, &TemplateCatalogError{Kind: TemplateCatalogErrorStorage, Message: "模板规则分析任务暂时无法创建，请重试"}
	}
	s.launch(created)
	return created, nil
}

// GetJob 返回单个分析任务当前真实进度。
func (s *TemplateCatalogService) GetJob(ctx context.Context, jobID string) (model.TemplateRuleAnalysisJob, error) {
	job, err := s.repository.GetJob(ctx, strings.TrimSpace(jobID))
	if err != nil {
		return model.TemplateRuleAnalysisJob{}, mapTemplateCatalogRepositoryError(err)
	}
	return job, nil
}

// LatestJob 返回账号最近一次分析任务，页面刷新后可继续显示状态。
func (s *TemplateCatalogService) LatestJob(ctx context.Context, account string) (model.TemplateRuleAnalysisJob, bool, error) {
	job, found, err := s.repository.LatestJob(ctx, strings.TrimSpace(account))
	if err != nil {
		return model.TemplateRuleAnalysisJob{}, false, mapTemplateCatalogRepositoryError(err)
	}
	return job, found, nil
}

// launch 保证同一账号只有一个目录 Worker，进程退出前任务状态仍可从数据库恢复查看。
func (s *TemplateCatalogService) launch(job model.TemplateRuleAnalysisJob) {
	s.mu.Lock()
	if _, exists := s.running[job.Account]; exists {
		s.mu.Unlock()
		return
	}
	workerContext, cancel := context.WithCancel(context.Background())
	s.running[job.Account] = cancel
	s.mu.Unlock()
	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.running, job.Account)
			s.mu.Unlock()
		}()
		s.run(workerContext, job)
	}()
}

// run 分页读取账号可见模板，单条失败只写入该条 needs_attention，不影响其他模板。
func (s *TemplateCatalogService) run(ctx context.Context, job model.TemplateRuleAnalysisJob) {
	job.Status, job.Message, job.UpdatedAt = "running", "正在分析模板规则", s.now().UTC()
	if err := s.repository.UpdateJob(ctx, job); err != nil {
		return
	}
	page, pageSize := 1, 25
	processed := 0
	seen := map[string]bool{}
	for {
		if ctx.Err() != nil {
			return
		}
		result, err := s.readTemplatePage(ctx, job.Account, page, pageSize)
		if err != nil {
			job.Status, job.Message = "failed", "读取目标平台模板列表失败，请重试"
			job.UpdatedAt = s.now().UTC()
			job.FinishedAt = &job.UpdatedAt
			_ = s.repository.UpdateJob(context.Background(), job)
			return
		}
		if page == 1 {
			job.Total = result.Total
		}
		for _, template := range result.Items {
			if ctx.Err() != nil {
				return
			}
			templateID := strings.TrimSpace(template.ID)
			if templateID == "" || seen[templateID] {
				continue
			}
			seen[templateID] = true
			job.Listed = len(seen)
			item, skipped := s.analyzeTemplate(ctx, job.Account, job.Mode, template)
			var itemErr error
			if !skipped {
				_, itemErr = s.repository.Upsert(ctx, item)
			}
			if itemErr != nil {
				job.Failed++
			} else {
				countTemplateCatalogJobItem(&job, item.Status)
			}
			processed++
			job.Processed = processed
			job.UpdatedAt = s.now().UTC()
			_ = s.repository.UpdateJob(ctx, job)
		}
		if !result.HasMore {
			job.PaginationComplete = true
			break
		}
		if len(result.Items) == 0 {
			job.Status, job.Message = "failed", "目标模板分页未完整读取，请重试"
			job.UpdatedAt = s.now().UTC()
			job.FinishedAt = &job.UpdatedAt
			_ = s.repository.UpdateJob(context.Background(), job)
			return
		}
		page++
	}
	now := s.now().UTC()
	if !job.PaginationComplete || job.Listed != job.Total || job.Processed != job.Total {
		job.Status, job.Message, job.UpdatedAt, job.FinishedAt = "failed", "目标模板分页数量与分析计数不一致，请重试", now, &now
		_ = s.repository.UpdateJob(context.Background(), job)
		return
	}
	job.Status, job.Message, job.UpdatedAt, job.FinishedAt = "completed", "模板规则分析已完成", now, &now
	_ = s.repository.UpdateJob(context.Background(), job)
}

// readTemplatePage 对目标模板分页执行有界重试，连续失败后才终止整次目录任务。
func (s *TemplateCatalogService) readTemplatePage(ctx context.Context, account string, page, pageSize int) (target.Page[target.FlowTemplate], error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		result, err := s.target.Templates(ctx, account, "", page, pageSize)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if attempt >= 2 {
			break
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 20 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return target.Page[target.FlowTemplate]{}, ctx.Err()
		case <-timer.C:
		}
	}
	return target.Page[target.FlowTemplate]{}, lastErr
}

// countTemplateCatalogJobItem 按规则条目的真实终态累计任务计数，跳过未变化模板时也不得改写状态语义。
func countTemplateCatalogJobItem(job *model.TemplateRuleAnalysisJob, status string) {
	switch strings.TrimSpace(status) {
	case "complete":
		job.Completed++
	case "failed":
		job.Failed++
	default:
		job.NeedsAttention++
	}
}

// analyzeTemplate 将目标模板和宿主 Vue 页面规则转成不含原始源码的本地快照。
func (s *TemplateCatalogService) analyzeTemplate(ctx context.Context, account, mode string, template target.FlowTemplate) (model.TemplateRuleCatalogItem, bool) {
	existing, existingFound, _ := s.repository.GetBySourceTemplateID(ctx, template.ID)
	if mode == "retry" && existingFound && existing.Status == "complete" {
		return existing, true
	}
	now := s.now().UTC()
	item := model.TemplateRuleCatalogItem{
		SourceTemplateID: template.ID, FlowCode: firstCatalogText(template.Code, template.ID), FlowName: template.FlowName,
		TemplateType: template.TypeName, FormExist: template.FormExist, SourceAccount: account,
		SourceVersion: firstCatalogText(template.UpdateDate, template.CreateDate), AnalyzerVersion: templateCatalogAnalyzerVersion,
		Status: "failed", RuleData: map[string]any{}, Coverage: map[string]any{}, Issues: []string{}, CreatedAt: now, UpdatedAt: now,
	}
	snapshot, err := s.target.TemplateConfiguration(ctx, account, template.ID)
	if err != nil {
		item.Issues = []string{"模板详情读取失败，请重试"}
		return item, false
	}
	item.FlowCode = firstCatalogText(snapshot.FlowCode, item.FlowCode)
	item.RenderType = templateRuleRenderType(snapshot.RenderType)
	item.TargetDigest = catalogDigest(struct {
		FlowCode   string
		RenderType target.FormRenderType
		Tree       *target.FlowNodeTemplate
		Fields     []target.FormFieldDetail
	}{snapshot.FlowCode, snapshot.RenderType, snapshot.Tree, snapshot.FormFields})
	item.FormMakingDigest = catalogDigest(snapshot.Forms)
	item.VueSourceDigest = s.pages.vueSourceDigest
	item.JavaSourceDigest = s.pages.javaSourceDigest
	item.ComponentDigest = s.pages.componentDigest
	if mode == "incremental" && existingFound && sameTemplateCatalogSources(existing, item) {
		return existing, true
	}
	ruleData := map[string]any{
		"flowCode": item.FlowCode, "renderType": item.RenderType,
		// 流程条件与组件能力作为同一份本地快照持久化，计划页面不能在用户操作时重新分析宿主源码。
		"flow":       catalogFlowRules(snapshot.Tree),
		"components": formdata.CustomComponentCapabilities(),
	}
	issues := []string{}
	coverage := map[string]any{}
	if item.RenderType == model.TemplateRuleRenderFormMaking {
		forms, mergeIssues := mergeCatalogForms(snapshot.Forms)
		inventory := formdata.InventoryTemplateRules(forms)
		ruleData["template"] = forms
		ruleData["fields"] = catalogFields(inventory.Fields)
		ruleData["dataSources"] = inventory.DataSources
		coverage["components"] = inventory.ComponentTypes
		coverage["scripts"] = inventory.ScriptCapabilities
		coverage["dataSources"] = len(inventory.DataSources)
		issues = append(issues, mergeIssues...)
		issues = append(issues, inventory.NeedsAttention...)
	} else if item.RenderType == model.TemplateRuleRenderVueCustom {
		pageRule := s.pages.Rule(item.FlowCode, template.FormExist)
		ruleData["page"] = pageRule
		issues = append(issues, pageRule.Issues...)
		coverage["pageComponent"] = pageRule.ComponentName
		coverage["fieldCount"] = len(pageRule.Fields)
		coverage["customComponents"] = s.pages.Components()
	} else {
		issues = append(issues, "表单渲染协议尚未识别")
	}
	item.RuleData, item.Coverage, item.Issues = ruleData, coverage, uniqueCatalogStrings(issues)

	// 分级规则问题并确定就绪状态
	completeness := model.ClassifyRuleIssues(item.Issues)
	item.Status = "complete"
	if completeness.Readiness == model.RuleReadinessBlocked {
		item.Status = "blocked"
	} else if len(item.Issues) > 0 {
		item.Status = "needs_attention"
	}

	item.SourceFingerprint = catalogFingerprint(item)
	analyzed := now
	item.AnalyzedAt = &analyzed
	return item, false
}

// catalogFlowRules 将流程节点、分支排序和条件能力转为不含目标内部标识的规则快照。
func catalogFlowRules(tree *target.FlowNodeTemplate) map[string]any {
	result := map[string]any{"nodes": 0, "branches": 0, "operators": map[string]int{}, "logic": map[string]int{}, "fieldComparisons": 0}
	if tree == nil {
		return result
	}
	operators := result["operators"].(map[string]int)
	logic := result["logic"].(map[string]int)
	var visit func(*target.FlowNodeTemplate)
	visit = func(node *target.FlowNodeTemplate) {
		if node == nil {
			return
		}
		result["nodes"] = result["nodes"].(int) + 1
		for _, branches := range [][]target.FlowBranchTemplate{node.ConditionNodes, node.ParallelNodes} {
			for _, branch := range branches {
				result["branches"] = result["branches"].(int) + 1
				for _, condition := range branch.Conditions {
					operator := strings.ToLower(strings.TrimSpace(condition.Judge))
					if operator != "" {
						operators[operator]++
					}
					kind := strings.ToLower(strings.TrimSpace(condition.ConditionType))
					if kind != "" {
						logic[kind]++
					}
					if strings.TrimSpace(condition.FieldB) != "" {
						result["fieldComparisons"] = result["fieldComparisons"].(int) + 1
					}
				}
				visit(branch.Child)
			}
		}
		visit(node.Child)
	}
	visit(tree)
	return result
}

// mergeCatalogForms 合并多个 FormMaking 正文并保留可公开的模板结构，不持久化目标返回元数据。
func mergeCatalogForms(forms []target.FormRuntimeTemplate) (map[string]any, []string) {
	result := map[string]any{"list": []any{}}
	issues := []string{}
	list := result["list"].([]any)
	for _, form := range forms {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(form.TemplateData), &parsed); err != nil {
			issues = append(issues, "FormMaking 模板正文无法解析")
			continue
		}
		if children, ok := parsed["list"].([]any); ok {
			list = append(list, children...)
		}
	}
	result["list"] = list
	return result, uniqueCatalogStrings(issues)
}

// catalogFields 只保留字段规则中的公开形状，防止规则目录暴露目标内部运行键。
func catalogFields(fields []formdata.Field) []map[string]any {
	result := make([]map[string]any, 0, len(fields))
	for _, field := range fields {
		result = append(result, map[string]any{"path": field.Path, "name": field.Name, "type": field.Type, "required": field.Required, "manual": field.ManualOnly, "component": field.El})
	}
	return result
}

// templateRuleRenderType 将目标适配层渲染类型投影为规则目录协议。
func templateRuleRenderType(value target.FormRenderType) model.TemplateRuleRenderType {
	switch value {
	case target.FormRenderTypeFormMaking:
		return model.TemplateRuleRenderFormMaking
	case target.FormRenderTypeVueCustom:
		return model.TemplateRuleRenderVueCustom
	default:
		return model.TemplateRuleRenderUnknown
	}
}

// catalogFingerprint 为同一模板、来源版本与规则内容生成稳定指纹，支持增量同步判断。
func catalogFingerprint(item model.TemplateRuleCatalogItem) string {
	payload, _ := json.Marshal(struct {
		ID, FlowCode, SourceVersion, TargetDigest, FormMakingDigest, VueSourceDigest, JavaSourceDigest, ComponentDigest, AnalyzerVersion string
		RuleData                                                                                                                         map[string]any
	}{item.SourceTemplateID, item.FlowCode, item.SourceVersion, item.TargetDigest, item.FormMakingDigest, item.VueSourceDigest, item.JavaSourceDigest, item.ComponentDigest, item.AnalyzerVersion, item.RuleData})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// catalogDigest 对可证明的来源对象生成稳定摘要，原始源码和目标响应不会进入目录统计输出。
func catalogDigest(value any) string {
	payload, _ := json.Marshal(value)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// sameTemplateCatalogSources 只有六类来源摘要和分析器版本全部一致时才允许增量跳过。
func sameTemplateCatalogSources(existing, current model.TemplateRuleCatalogItem) bool {
	return existing.SourceVersion == current.SourceVersion && existing.TargetDigest == current.TargetDigest &&
		existing.FormMakingDigest == current.FormMakingDigest && existing.VueSourceDigest == current.VueSourceDigest &&
		existing.JavaSourceDigest == current.JavaSourceDigest && existing.ComponentDigest == current.ComponentDigest &&
		existing.AnalyzerVersion == current.AnalyzerVersion
}

// newTemplateCatalogJobID 生成可排序且不暴露账号信息的分析任务 ID。
func newTemplateCatalogJobID(now time.Time) string {
	return fmt.Sprintf("f010-%d", now.UnixNano())
}

// validTemplateCatalogMode 限制设置页只能发起批准范围内的三类分析任务。
func validTemplateCatalogMode(mode string) bool {
	switch mode {
	case "incremental", "full", "retry":
		return true
	default:
		return false
	}
}

// firstCatalogText 返回第一个非空文本，避免目录出现空流程编码。
func firstCatalogText(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return "unknown"
}

// uniqueCatalogStrings 稳定去重中文规则问题，确保同一组件不会重复提示。
func uniqueCatalogStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

// mapTemplateCatalogRepositoryError 将规则仓储故障收敛为稳定设置页错误。
func mapTemplateCatalogRepositoryError(err error) error {
	if errors.Is(err, repository.ErrTemplateCatalogNotFound) {
		return &TemplateCatalogError{Kind: TemplateCatalogErrorNotFound, Message: "模板规则或分析任务不存在"}
	}
	return &TemplateCatalogError{Kind: TemplateCatalogErrorStorage, Message: "模板规则目录暂不可用，请重试"}
}

// TemplateCatalogError 是规则目录 API 的用户可读错误边界。
type TemplateCatalogError struct {
	Kind    string
	Message string
}

// Error 返回设置页可安全展示的中文错误。
func (e *TemplateCatalogError) Error() string { return e.Message }

const (
	TemplateCatalogErrorInvalid  = "invalid"
	TemplateCatalogErrorActive   = "active"
	TemplateCatalogErrorNotFound = "not_found"
	TemplateCatalogErrorStorage  = "storage"
)

// vuePageRuleScanner 从宿主实际运行时代码中提取页面入口、配置字段和自定义组件注册表。
type vuePageRuleScanner struct {
	root             string
	components       map[string]string
	componentFiles   map[string]string
	pageByCode       map[string]vuePageEntry
	apiPaths         map[string]string
	javaSources      []javaRuleSource
	vueSourceDigest  string
	javaSourceDigest string
	componentDigest  string
}

type vuePageEntry struct {
	name      string
	component string
	isShow    bool
}

// javaRuleSource 是 Java 控制器的只读分析输入，只在进程内保存，绝不写入规则目录。
type javaRuleSource struct {
	module  string
	path    string
	content string
}

// newVuePageRuleScanner 创建可在分析任务中复用的只读源码扫描器。
func newVuePageRuleScanner(root string) *vuePageRuleScanner {
	scanner := &vuePageRuleScanner{root: root, components: map[string]string{}, componentFiles: map[string]string{}, pageByCode: map[string]vuePageEntry{}, apiPaths: map[string]string{}}
	scanner.load()
	return scanner
}

// load 扫描 settings.js 和 runtime main.js；文件缺失时由页面规则标记分析问题。
func (s *vuePageRuleScanner) load() {
	main := s.read("form-runtime/runtime-source/src/main.js")
	componentPattern := regexp.MustCompile(`name:\s*['"]([^'"]+)['"]\s*,\s*component:\s*([A-Za-z0-9_]+)`)
	for _, match := range componentPattern.FindAllStringSubmatch(main, -1) {
		s.components[match[1]] = match[2]
	}
	settings := s.read("form-runtime/runtime-source/src/store/modules/settings.js")
	entryPattern := regexp.MustCompile(`(?ms)^[ \t]{4}([A-Za-z0-9_]+)\s*:\s*\{(.*?)^[ \t]{4}\},?`)
	namePattern := regexp.MustCompile(`name\s*:\s*['"]([^'"]+)['"]`)
	pageComponentPattern := regexp.MustCompile(`component\s*:\s*['"]?([A-Za-z0-9_]+)['"]?`)
	isShowPattern := regexp.MustCompile(`isShow\s*:\s*true`)
	for _, match := range entryPattern.FindAllStringSubmatch(settings, -1) {
		nameMatch := namePattern.FindStringSubmatch(match[2])
		if len(nameMatch) < 2 {
			continue
		}
		componentMatch := pageComponentPattern.FindStringSubmatch(match[2])
		component := ""
		if len(componentMatch) > 1 {
			component = componentMatch[1]
		}
		entry := s.pageByCode[match[1]]
		entry.name = nameMatch[1]
		// settings.js 中存在同编码的补充展示项；补充项没有 component 时不能覆盖前面已经确认的真实页面入口。
		if component != "" {
			entry.component = component
		}
		entry.isShow = entry.isShow || isShowPattern.MatchString(match[2])
		s.pageByCode[match[1]] = entry
	}
	contractSection := regexp.MustCompile(`(?s)contractPagesName\s*:\s*\{(.*?)\n\s*\}`).FindStringSubmatch(settings)
	if len(contractSection) > 1 {
		contractEntryPattern := regexp.MustCompile(`([A-Za-z0-9_]+)\s*:\s*['"]([^'"]+)['"]`)
		for _, match := range contractEntryPattern.FindAllStringSubmatch(contractSection[1], -1) {
			entry := s.pageByCode[match[1]]
			if strings.TrimSpace(entry.name) == "" {
				entry.name = match[2]
			}
			s.pageByCode[match[1]] = entry
		}
	}
	s.loadHostVuePageFiles()
	s.loadAPIPaths()
	s.loadJavaRuleSources()
	s.vueSourceDigest = digestCatalogDirectory(filepath.Join(s.root, "form-runtime", "runtime-source", "src"), map[string]bool{".js": true, ".vue": true, ".json": true})
	s.javaSourceDigest = digestCatalogDirectory(filepath.Join(s.root, "参考代码", "java-serve"), map[string]bool{".java": true})
	s.componentDigest = catalogDigest(struct {
		Registry     map[string]string
		Capabilities map[string]map[string]string
		Source       string
	}{s.components, formdata.CustomComponentCapabilities(), digestCatalogDirectory(filepath.Join(s.root, "form-runtime", "runtime-source", "src", "components", "Custom"), map[string]bool{".js": true, ".vue": true})})
}

// loadAPIPaths 从宿主真实 API 常量表提取命名端点，静态分析只记录声明路径而不发起请求。
func (s *vuePageRuleScanner) loadAPIPaths() {
	content := s.read("form-runtime/runtime-source/src/api/index.js")
	type apiObject struct {
		name   string
		indent int
	}
	objects := make([]apiObject, 0)
	objectPattern := regexp.MustCompile(`^(\s*)([A-Za-z0-9_]+)\s*:\s*\{`)
	propertyPattern := regexp.MustCompile(`^(\s*)([A-Za-z0-9_]+)\s*:\s*['"]([^'"]+)['"]`)
	closePattern := regexp.MustCompile(`^(\s*)\}`)
	for _, line := range strings.Split(content, "\n") {
		if match := closePattern.FindStringSubmatch(line); len(match) > 1 {
			indent := len(match[1])
			for len(objects) > 0 && objects[len(objects)-1].indent >= indent {
				objects = objects[:len(objects)-1]
			}
			continue
		}
		if match := objectPattern.FindStringSubmatch(line); len(match) > 2 {
			indent := len(match[1])
			for len(objects) > 0 && objects[len(objects)-1].indent >= indent {
				objects = objects[:len(objects)-1]
			}
			objects = append(objects, apiObject{name: match[2], indent: indent})
			continue
		}
		if match := propertyPattern.FindStringSubmatch(line); len(match) > 3 && len(objects) > 0 {
			parts := make([]string, 0, len(objects)+1)
			for _, object := range objects {
				parts = append(parts, object.name)
			}
			parts = append(parts, match[2])
			s.apiPaths[strings.Join(parts, ".")] = match[3]
		}
	}
}

// loadJavaRuleSources 只读取带 Spring 路由注解的控制器，供页面协议与 Java 端点交叉验证。
func (s *vuePageRuleScanner) loadJavaRuleSources() {
	root := filepath.Join(s.root, "参考代码", "java-serve")
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || filepath.Ext(path) != ".java" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil || !strings.Contains(string(data), "RequestMapping") {
			return nil
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(relative), "/")
		module := "java-serve"
		if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
			module = parts[0]
		}
		s.javaSources = append(s.javaSources, javaRuleSource{module: module, path: filepath.ToSlash(relative), content: string(data)})
		return nil
	})
	sort.Slice(s.javaSources, func(left, right int) bool { return s.javaSources[left].path < s.javaSources[right].path })
}

// digestCatalogDirectory 按相对路径和文件内容生成目录摘要，缺失目录以稳定空摘要参与增量判断。
func digestCatalogDirectory(root string, extensions map[string]bool) string {
	type sourceFile struct{ Path, Digest string }
	files := make([]sourceFile, 0)
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || !extensions[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		sum := sha256.Sum256(data)
		relative, _ := filepath.Rel(root, path)
		files = append(files, sourceFile{Path: filepath.ToSlash(relative), Digest: hex.EncodeToString(sum[:])})
		return nil
	})
	sort.Slice(files, func(left, right int) bool { return files[left].Path < files[right].Path })
	return catalogDigest(files)
}

// loadHostVuePageFiles 读取当前运行时实际使用的宿主组件注册表，把公开组件名解析到真实 Vue 文件。
func (s *vuePageRuleScanner) loadHostVuePageFiles() {
	registry := s.read("form-runtime/src/runtime/hostVuePages.js")
	imports := map[string]string{}
	importPattern := regexp.MustCompile(`(?m)^import\s+([A-Za-z0-9_]+)\s+from\s+['"]@runtime/([^'"]+\.vue)['"]`)
	for _, match := range importPattern.FindAllStringSubmatch(registry, -1) {
		imports[match[1]] = filepath.Join(s.root, "form-runtime", "runtime-source", "src", filepath.FromSlash(match[2]))
	}
	section := regexp.MustCompile(`(?s)HOST_VUE_PAGES\s*=\s*\{(.*?)\n\}`).FindStringSubmatch(registry)
	if len(section) < 2 {
		return
	}
	entryPattern := regexp.MustCompile(`(?m)^\s*([A-Za-z0-9_]+)(?:\s*:\s*([A-Za-z0-9_]+))?\s*,?\s*$`)
	for _, match := range entryPattern.FindAllStringSubmatch(section[1], -1) {
		variable := match[1]
		if len(match) > 2 && strings.TrimSpace(match[2]) != "" {
			variable = match[2]
		}
		if path := imports[variable]; path != "" {
			s.componentFiles[match[1]] = path
		}
	}
}

// Rule 生成一个流程编码对应的 Vue 页面规则；未知入口必须显式进入需处理状态。
func (s *vuePageRuleScanner) Rule(flowCode, formExist string) target.VueCustomPageRule {
	entry, exists := s.pageByCode[strings.TrimSpace(flowCode)]
	page := target.VueCustomPageRule{
		PageKey: flowCode, PageName: entry.name, ComponentName: entry.component, Route: flowCode,
		InitialState: map[string]any{}, Fields: []target.VueCustomFieldRule{}, Dependencies: []target.VueCustomDependencyRule{},
		ReadRequests: []target.VueCustomRequestRule{}, Issues: []string{},
	}
	if !exists {
		page.PageName = firstCatalogText(flowCode)
		page.Issues = append(page.Issues, "宿主 Vue 页面入口尚未识别")
		return page
	}
	fields, configComponent, found := s.configFields(flowCode)
	if found && strings.TrimSpace(page.ComponentName) == "" {
		page.ComponentName = configComponent
	}
	if !found && entry.component != "" {
		fields, found = s.componentFields(entry.component)
	}
	page.Fields = fields
	// 配置式页面的字段结构与组件页面的生命周期逻辑分开存放，合并分析但不执行任何宿主脚本。
	source := s.configSource(flowCode)
	if componentSource := s.componentSource(page.ComponentName); componentSource != "" {
		source += "\n" + componentSource
	}
	page.InitialState = vueInitialState(page.Fields, source)
	page.Dependencies = vueFieldDependencies(page.Fields, source)
	requests, requestIssues := s.vueReadRequests(source)
	page.ReadRequests = requests
	page.Issues = append(page.Issues, requestIssues...)
	page.Submit = s.vueSubmitRule(flowCode)
	page.Identity = vueIdentityRule(source)
	page.Java = s.javaPageRule(page.ReadRequests, page.Submit, page.Identity)
	if page.Submit != nil {
		page.Issues = append(page.Issues, page.Submit.Issues...)
	}
	if page.Java != nil {
		page.Issues = append(page.Issues, page.Java.Issues...)
	}
	if !found {
		page.Issues = append(page.Issues, "宿主 Vue 页面字段规则尚未完整识别")
	}
	if strings.TrimSpace(formExist) == "" {
		page.Issues = append(page.Issues, "宿主页面缺少渲染标记")
	}
	page.Issues = uniqueCatalogStrings(page.Issues)
	return page
}

// Components 返回已注册自定义组件能力名称，供所有模板覆盖报告共享。
func (s *vuePageRuleScanner) Components() map[string]string {
	result := make(map[string]string, len(s.components))
	for key, value := range s.components {
		result[key] = value
	}
	return result
}

// configFields 从 NoFormFLow 配置式页面提取 prop、类型和 required 规则。
func (s *vuePageRuleScanner) configFields(flowCode string) ([]target.VueCustomFieldRule, string, bool) {
	content := s.configSource(flowCode)
	if content == "" {
		return nil, "", false
	}
	fields := parseVueConfigFields(content)
	component := strings.TrimSuffix(filepath.Base(s.configPath(flowCode)), filepath.Ext(s.configPath(flowCode)))
	component = strings.TrimSuffix(component, "Config")
	return fields, component, len(fields) > 0
}

// configPath 按流程编码查找真实 NoFormFlow 配置文件，不为某个合同或页面维护专用表。
func (s *vuePageRuleScanner) configPath(flowCode string) string {
	configRoot := filepath.Join(s.root, "form-runtime", "runtime-source", "src", "components", "NoFormFLow", "config")
	var path string
	_ = filepath.Walk(configRoot, func(candidate string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || !strings.HasSuffix(candidate, ".js") || path != "" {
			return nil
		}
		base := strings.TrimSuffix(filepath.Base(candidate), filepath.Ext(candidate))
		base = strings.TrimSuffix(base, "Config")
		if normalizeVueRuleKey(base) == normalizeVueRuleKey(flowCode) {
			path = candidate
		}
		return nil
	})
	return path
}

// configSource 读取匹配配置文件的文本，仅供静态规则提取且不把原文带出服务边界。
func (s *vuePageRuleScanner) configSource(flowCode string) string {
	path := s.configPath(flowCode)
	if path == "" {
		return ""
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(content)
}

// normalizeVueRuleKey 将流程编码和配置文件名归一化后比较，避免为每个流程维护专用映射表。
func normalizeVueRuleKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("_", "", "-", "", " ", "").Replace(value)
	return value
}

// parseVueConfigFields 从配置对象中按 prop 边界提取字段，支持 type 与 prop 的任意书写顺序。
func parseVueConfigFields(content string) []target.VueCustomFieldRule {
	propPattern := regexp.MustCompile(`prop\s*:\s*['"]([^'"]+)['"]`)
	typePattern := regexp.MustCompile(`(?:nodeType|type)\s*:\s*['"]([^'"]+)['"]`)
	titlePattern := regexp.MustCompile(`(?:title|label)\s*:\s*['"]([^'"]+)['"]`)
	defaultPattern := regexp.MustCompile(`value\s*:\s*['"]([^'"]*)['"]`)
	optionPattern := regexp.MustCompile(`(?s)(?:value\s*:\s*['"]([^'"]*)['"]\s*,\s*label\s*:\s*['"]([^'"]*)['"]|label\s*:\s*['"]([^'"]*)['"]\s*,\s*value\s*:\s*['"]([^'"]*)['"])`)
	fields, seen := make([]target.VueCustomFieldRule, 0), map[string]bool{}
	matches := propPattern.FindAllStringSubmatchIndex(content, -1)
	spans := vueObjectSpans(content)
	for _, match := range matches {
		path := strings.TrimSpace(content[match[2]:match[3]])
		if path == "" || seen[path] {
			continue
		}
		window, found := vueObjectWindow(content, spans, match[0])
		if !found {
			continue
		}
		seen[path] = true
		typeName, title, defaultValue := "runtime", path, ""
		if found := typePattern.FindStringSubmatch(window); len(found) > 1 {
			typeName = found[1]
		}
		if previousTitle := vuePreviousTitle(content, titlePattern, match[0]); previousTitle != "" {
			title = previousTitle
		}
		if found := defaultPattern.FindStringSubmatch(window); len(found) > 1 {
			defaultValue = found[1]
		}
		options := make([]target.VueCustomFieldOption, 0)
		for _, option := range optionPattern.FindAllStringSubmatch(window, -1) {
			value, label := option[1], option[2]
			if value == "" && len(option) > 4 {
				label, value = option[3], option[4]
			}
			if value != "" || label != "" {
				options = append(options, target.VueCustomFieldOption{Value: value, Label: label})
			}
		}
		candidateKind := vueCandidateKind(typeName, window, len(options))
		field := target.VueCustomFieldRule{
			Path: path, Name: title, ValueType: normalizeVueFieldType(typeName), Required: strings.Contains(window, "required: true") || strings.Contains(window, "isRequire: true"),
			ReadOnly: strings.Contains(window, "disabled: true") || strings.Contains(window, "readonly: true"), Disabled: strings.Contains(window, "disabled: true"), Hidden: strings.Contains(window, "hidden: true") || strings.Contains(window, "isShow: false"), DefaultValue: defaultValue,
			CandidateKind: candidateKind, DataSource: vueFieldDataSource(candidateKind, window), Options: options,
		}
		fields = append(fields, applyVueFieldFacts(field, window))
	}
	return fields
}

// vueFieldDataSource 只记录可证明的候选来源类别，不把源码表达式或目标接口地址暴露到规则目录页面。
func vueFieldDataSource(candidateKind, window string) string {
	if candidateKind == "static" {
		return "static_options"
	}
	if strings.Contains(window, "Api.") || strings.Contains(window, "$http") || strings.Contains(window, "axios") {
		return "target_readonly_api"
	}
	if candidateKind == "runtime_source" {
		return "host_runtime"
	}
	return ""
}

type vueObjectSpan struct{ start, end int }

// vueObjectSpans 以括号深度提取 JavaScript 对象边界，避免跨字段正则把相邻控件混为一条规则。
func vueObjectSpans(content string) []vueObjectSpan {
	stack := make([]int, 0)
	spans := make([]vueObjectSpan, 0)
	for index, character := range content {
		switch character {
		case '{':
			stack = append(stack, index)
		case '}':
			if len(stack) > 0 {
				start := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				spans = append(spans, vueObjectSpan{start: start, end: index + 1})
			}
		}
	}
	sort.Slice(spans, func(left, right int) bool {
		return spans[left].end-spans[left].start < spans[right].end-spans[right].start
	})
	return spans
}

// vueObjectWindow 返回包住当前 prop 的最小对象，字段规则不应跨越同级组件。
func vueObjectWindow(content string, spans []vueObjectSpan, offset int) (string, bool) {
	for _, span := range spans {
		if span.start <= offset && offset < span.end {
			return content[span.start:span.end], true
		}
	}
	return "", false
}

// vuePreviousTitle 从字段前最近的标签对象取得展示名称，找不到时保留字段路径而不猜测业务含义。
func vuePreviousTitle(content string, titlePattern *regexp.Regexp, offset int) string {
	start := offset - 360
	if start < 0 {
		start = 0
	}
	matches := titlePattern.FindAllStringSubmatch(content[start:offset], -1)
	if len(matches) == 0 || len(matches[len(matches)-1]) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[len(matches)-1][1])
}

// normalizeVueFieldType 把宿主配置式控件类型映射为统一生成器可校验的公开类型。
func normalizeVueFieldType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "inputnum", "number", "amount":
		return "number"
	case "date", "datetime", "daterange", "date-picker", "time":
		return "date"
	case "select", "radio":
		return "select"
	case "checkbox", "multiple":
		return "checkbox"
	case "textarea", "input", "text":
		return "input"
	case "upload", "file", "fileupload":
		return "file"
	default:
		return "runtime"
	}
}

// vueCandidateKind 标记字段候选来源，动态接口不会被误判为可随机构造。
func vueCandidateKind(typeName, window string, optionCount int) string {
	if optionCount > 0 {
		return "static"
	}
	if normalizeVueFieldType(typeName) == "file" {
		return "external"
	}
	switch normalizeVueFieldType(typeName) {
	case "select", "checkbox":
		return "runtime_source"
	default:
		return "generated"
	}
}

// componentSource 按宿主运行时注册表读取真实 Vue 组件，找不到时才以同名文件作为受限回退。
func (s *vuePageRuleScanner) componentSource(component string) string {
	var content string
	if registeredPath := s.componentFiles[component]; registeredPath != "" {
		if data, err := os.ReadFile(registeredPath); err == nil {
			content = string(data)
		}
	}
	if content == "" {
		_ = filepath.Walk(filepath.Join(s.root, "form-runtime", "runtime-source", "src"), func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() || content != "" || !strings.HasSuffix(path, ".vue") {
				return nil
			}
			if strings.EqualFold(strings.TrimSuffix(filepath.Base(path), ".vue"), component) {
				data, readErr := os.ReadFile(path)
				if readErr == nil {
					content = string(data)
				}
			}
			return nil
		})
	}
	return content
}

// componentFields 从宿主 Vue 组件提取 v-model 与 data 字段，覆盖非配置式页面的真实状态入口。
func (s *vuePageRuleScanner) componentFields(component string) ([]target.VueCustomFieldRule, bool) {
	content := s.componentSource(component)
	if content == "" {
		return nil, false
	}
	pattern := regexp.MustCompile(`v-model(?:\.\w+)?\s*=\s*['"]([^'"]+)['"]`)
	fields := append(parseVueTemplateFields(content), parseVueConfigFields(content)...)
	if len(fields) == 0 {
		fields = parseVueStateFields(content)
	}
	seen := make(map[string]bool, len(fields))
	for _, field := range fields {
		seen[field.Path] = true
	}
	for _, match := range pattern.FindAllStringSubmatch(content, -1) {
		path := strings.TrimSpace(match[1])
		if path == "" || strings.HasSuffix(path, "[") || strings.HasPrefix(path, "scope.") || strings.HasPrefix(path, "val.") || seen[path] {
			continue
		}
		seen[path] = true
		fields = append(fields, applyVueFieldFacts(target.VueCustomFieldRule{Path: path, Name: path, ValueType: "runtime", CandidateKind: "runtime"}, content))
	}
	return fields, len(fields) > 0
}

// parseVueStateFields 识别只读 Vue 业务页通过 detail/rawData 回显并由 getValues 返回的状态字段。
func parseVueStateFields(content string) []target.VueCustomFieldRule {
	if !strings.Contains(content, "getValues") {
		return nil
	}
	fieldPattern := regexp.MustCompile(`(?:detail|rawData)\.([A-Za-z0-9_]+)`)
	fields, seen := make([]target.VueCustomFieldRule, 0), map[string]bool{}
	dataSource := "host_runtime"
	if strings.Contains(content, "$axios") || strings.Contains(content, "Api.") {
		dataSource = "target_readonly_api"
	}
	for _, match := range fieldPattern.FindAllStringSubmatch(content, -1) {
		path := strings.TrimSpace(match[1])
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		requiredPattern := regexp.MustCompile(`!\s*this\.rawData\.` + regexp.QuoteMeta(path) + `\b`)
		field := target.VueCustomFieldRule{
			Path: path, Name: path, ValueType: "runtime", Required: requiredPattern.MatchString(content), ReadOnly: true,
			CandidateKind: "runtime_source", DataSource: dataSource,
		}
		fields = append(fields, applyVueFieldFacts(field, content))
	}
	return fields
}

// parseVueTemplateFields 从 Element 表单项读取实际 v-model、标签、控件类型和 required 规则。
func parseVueTemplateFields(content string) []target.VueCustomFieldRule {
	itemPattern := regexp.MustCompile(`(?s)<el-form-item\b([^>]*)>(.*?)</el-form-item>`)
	labelPattern := regexp.MustCompile(`\blabel\s*=\s*['"]([^'"]+)['"]`)
	propPattern := regexp.MustCompile(`\bprop\s*=\s*['"]([^'"]+)['"]`)
	modelPattern := regexp.MustCompile(`v-model(?:\.\w+)?\s*=\s*['"]([^'"]+)['"]`)
	fields, seen := make([]target.VueCustomFieldRule, 0), map[string]bool{}
	for _, match := range itemPattern.FindAllStringSubmatch(content, -1) {
		attributes, body := match[1], match[2]
		model := modelPattern.FindStringSubmatch(body)
		if len(model) < 2 {
			continue
		}
		path := strings.TrimSpace(model[1])
		if path == "" || strings.HasSuffix(path, "[") || strings.HasPrefix(path, "scope.") || strings.HasPrefix(path, "val.") || seen[path] {
			continue
		}
		seen[path] = true
		name, prop := path, ""
		if label := labelPattern.FindStringSubmatch(attributes); len(label) > 1 {
			name = strings.TrimSpace(label[1])
		}
		if property := propPattern.FindStringSubmatch(attributes); len(property) > 1 {
			prop = strings.TrimSpace(property[1])
		}
		typeName := vueTemplateControlType(body)
		options := vueTemplateOptions(body)
		candidateKind := vueCandidateKind(typeName, body, len(options))
		field := target.VueCustomFieldRule{
			Path: path, Name: name, ValueType: normalizeVueFieldType(typeName), Required: vueTemplateFieldRequired(content, attributes, prop),
			ReadOnly: strings.Contains(body, "disabled") || strings.Contains(body, ":disabled") || strings.Contains(body, "readonly") || strings.Contains(body, ":readonly"), Disabled: strings.Contains(body, "disabled") || strings.Contains(body, ":disabled"), Hidden: strings.Contains(attributes, "v-if=\"false\"") || strings.Contains(attributes, "v-show=\"false\""), CandidateKind: candidateKind,
			DataSource: vueFieldDataSource(candidateKind, body), Options: options,
		}
		fields = append(fields, applyVueFieldFacts(field, body))
	}
	return fields
}

// applyVueFieldFacts 为已识别字段补充路径、格式和验证事实；动态表达式只标记为规则，绝不执行或推断其值。
func applyVueFieldFacts(field target.VueCustomFieldRule, source string) target.VueCustomFieldRule {
	path := strings.TrimSpace(field.Path)
	field.Nested = strings.Contains(path, ".")
	field.Collection = strings.Contains(path, "[") || strings.Contains(path, "[]")
	validation := make([]string, 0, 3)
	if field.Required {
		validation = append(validation, "required")
	}
	if strings.Contains(source, "pattern") {
		validation = append(validation, "pattern")
	}
	if strings.Contains(source, "validator") {
		validation = append(validation, "custom_validator")
	}
	if field.ValueType == "date" {
		field.Format = "date"
	} else if strings.Contains(source, "type=\"email\"") || strings.Contains(source, "type: 'email'") || strings.Contains(source, "type: \"email\"") {
		field.Format = "email"
	} else if strings.Contains(source, "pattern") {
		field.Format = "pattern"
	}
	field.Validation = validation
	return field
}

// vueTemplateControlType 按宿主模板实际控件标签识别统一字段类型，未知控件保持 runtime 交给真实页面。
func vueTemplateControlType(body string) string {
	for _, candidate := range []struct{ marker, value string }{
		{"<el-input-number", "number"}, {"<el-date-picker", "date"}, {"<el-radio-group", "radio"},
		{"<el-checkbox-group", "checkbox"}, {"<el-select", "select"}, {"<el-upload", "upload"}, {"<el-input", "input"},
	} {
		if strings.Contains(body, candidate.marker) {
			return candidate.value
		}
	}
	return "runtime"
}

// vueTemplateOptions 只提取模板中直接声明的静态 el-option，不执行动态表达式。
func vueTemplateOptions(body string) []target.VueCustomFieldOption {
	optionPattern := regexp.MustCompile(`<el-option\b[^>]*\slabel\s*=\s*['"]([^'"]*)['"][^>]*\svalue\s*=\s*['"]([^'"]*)['"][^>]*>`)
	result := make([]target.VueCustomFieldOption, 0)
	for _, match := range optionPattern.FindAllStringSubmatch(body, -1) {
		result = append(result, target.VueCustomFieldOption{Label: match[1], Value: match[2]})
	}
	return result
}

// vueTemplateFieldRequired 同时识别行内规则和 data 中按 prop 声明的 Element required 规则。
func vueTemplateFieldRequired(content, attributes, prop string) bool {
	if strings.Contains(attributes, "required") || strings.Contains(attributes, ":rules") {
		return true
	}
	if prop == "" {
		return false
	}
	pattern := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(prop) + `\s*:\s*\[[^\]]*required\s*:\s*true`)
	return pattern.MatchString(content)
}

// vueInitialState 汇总字段声明中可证明的默认值；未声明初值的字段保持缺席，禁止以空值猜测宿主状态。
func vueInitialState(fields []target.VueCustomFieldRule, _ string) map[string]any {
	result := make(map[string]any)
	for _, field := range fields {
		if strings.TrimSpace(field.Path) == "" || field.DefaultValue == nil {
			continue
		}
		if value, ok := field.DefaultValue.(string); ok && value == "" {
			continue
		}
		result[field.Path] = field.DefaultValue
	}
	return result
}

// vueFieldDependencies 从实际赋值语句提取字段联动边，无法归属到已识别字段的表达式不会被猜测为业务规则。
func vueFieldDependencies(fields []target.VueCustomFieldRule, source string) []target.VueCustomDependencyRule {
	byTerminal := make(map[string]string, len(fields))
	for _, field := range fields {
		path := strings.TrimSpace(field.Path)
		if path == "" {
			continue
		}
		parts := strings.FieldsFunc(path, func(r rune) bool { return r == '.' || r == '[' || r == ']' })
		if len(parts) > 0 {
			byTerminal[parts[len(parts)-1]] = path
		}
	}
	pattern := regexp.MustCompile(`this\.([A-Za-z0-9_]+)\s*=\s*[^\n;]*this\.([A-Za-z0-9_]+)`)
	result := make([]target.VueCustomDependencyRule, 0)
	for _, match := range pattern.FindAllStringSubmatch(source, -1) {
		field, fieldOK := byTerminal[match[1]]
		depends, dependsOK := byTerminal[match[2]]
		if !fieldOK || !dependsOK || field == depends {
			continue
		}
		result = append(result, target.VueCustomDependencyRule{Field: field, Depends: []string{depends}, Kind: "assignment", Source: "vue_component"})
	}
	return uniqueVueDependencies(result)
}

// uniqueVueDependencies 去重页面字段联动，保持持久化快照在重复模板片段下稳定。
func uniqueVueDependencies(values []target.VueCustomDependencyRule) []target.VueCustomDependencyRule {
	seen := make(map[string]bool, len(values))
	result := make([]target.VueCustomDependencyRule, 0, len(values))
	for _, value := range values {
		key := value.Field + "\x00" + strings.Join(value.Depends, "\x00") + "\x00" + value.Kind
		if value.Field == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, value)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Field < result[right].Field })
	return result
}

// vueReadRequests 从组件中已声明的 Api 常量提取只读初始化和联动请求，写请求始终留在宿主隔离边界。
func (s *vuePageRuleScanner) vueReadRequests(source string) ([]target.VueCustomRequestRule, []string) {
	pattern := regexp.MustCompile(`Api\.([A-Za-z0-9_]+(?:\.[A-Za-z0-9_]+)+)`)
	requests := make([]target.VueCustomRequestRule, 0)
	issues := make([]string, 0)
	seen := map[string]bool{}
	for _, match := range pattern.FindAllStringSubmatchIndex(source, -1) {
		if len(match) < 4 {
			continue
		}
		name := source[match[2]:match[3]]
		path, found := s.apiPaths[name]
		if !found {
			issues = append(issues, "Vue 请求常量「"+name+"」未在宿主 API 表中识别")
			continue
		}
		if !vueEndpointReadOnly(path) {
			continue
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		requests = append(requests, target.VueCustomRequestRule{
			Name: name, Method: "POST", Path: path, Phase: vueRequestPhase(source, match[0]),
			Response: vueRequestResponse(source, match[1]), ReadOnly: true, Issues: []string{},
		})
	}
	sort.Slice(requests, func(left, right int) bool { return requests[left].Name < requests[right].Name })
	return requests, uniqueCatalogStrings(issues)
}

// vueEndpointReadOnly 只按端点最后动作词判定只读；无法证明只读的请求不会被规则分析器消费。
func vueEndpointReadOnly(path string) bool {
	last := strings.ToLower(strings.TrimSpace(filepath.Base(strings.TrimSuffix(path, "/"))))
	for _, marker := range []string{"save", "update", "delete", "create", "modify", "increase", "submit", "upload"} {
		if strings.Contains(last, marker) {
			return false
		}
	}
	return last != ""
}

// vueRequestPhase 根据调用所在生命周期片段归类请求阶段，方法内调用保守标记为交互而非初始化。
func vueRequestPhase(source string, offset int) string {
	prefix := source[:offset]
	lastMounted := strings.LastIndex(prefix, "mounted()")
	lastCreated := strings.LastIndex(prefix, "created()")
	lastMethods := strings.LastIndex(prefix, "methods:")
	if (lastMounted >= 0 || lastCreated >= 0) && lastMethods < maxInt(lastMounted, lastCreated) {
		return "initial"
	}
	return "interaction"
}

// maxInt 返回两个偏移量中的较大值，用于生命周期片段的保守比较。
func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

// vueRequestResponse 仅在调用附近有真实 isSuccess 分支时声明成功判定，未证明时保留空字符串。
func vueRequestResponse(source string, offset int) string {
	end := offset + 600
	if end > len(source) {
		end = len(source)
	}
	if strings.Contains(source[offset:end], "isSuccess") {
		return "isSuccess"
	}
	return ""
}

// vueSubmitRule 从 NoFormFlow 共用 mixin 的精确流程映射提取保存协议，不用组件名或合同名称推断端点。
func (s *vuePageRuleScanner) vueSubmitRule(flowCode string) *target.VueCustomSubmitRule {
	mixin := s.read("form-runtime/runtime-source/src/components/NoFormFLow/mixin/mixin.js")
	pattern := regexp.MustCompile(`['"]` + regexp.QuoteMeta(strings.TrimSpace(flowCode)) + `['"]\s*:\s*\{\s*save\s*:\s*Api\.([A-Za-z0-9_]+)\.([A-Za-z0-9_]+)`)
	match := pattern.FindStringSubmatch(mixin)
	if len(match) < 3 {
		return nil
	}
	name := match[1] + "." + match[2]
	path, found := s.apiPaths[name]
	if !found {
		return &target.VueCustomSubmitRule{Method: "POST", Payload: []string{"data"}, SuccessChecks: []string{"isSuccess"}, Blocked: true, Issues: []string{"宿主保存请求常量「" + name + "」未在 API 表中识别"}}
	}

	// 识别 Vue 错误回显逻辑
	configSource := s.configSource(flowCode)
	componentSource := s.componentSource(s.pageByCode[flowCode].component)
	fullSource := configSource + "\n" + componentSource + "\n" + mixin

	fieldErrorPaths := s.extractVueFieldErrorPaths(fullSource)
	issues := []string{}
	if len(fieldErrorPaths) == 0 {
		issues = append(issues, "宿主保存响应未证明字段级错误路径")
	}

	// 工作区只回显协议并交给本地保存接口复验，任何宿主业务写操作都不能通过此规则发起。
	return &target.VueCustomSubmitRule{
		Method: "POST", Path: path, Payload: []string{"data"},
		SuccessChecks: []string{"isSuccess"}, FieldErrorPaths: fieldErrorPaths,
		Blocked: true, Issues: issues,
	}
}

// vueIdentityRule 识别页面实际读取的登录上下文键，并按用户、部门、公司投影到本地安全会话。
func vueIdentityRule(source string) *target.VueCustomIdentityRule {
	localPattern := regexp.MustCompile(`localstorageGet\(\s*['"]([^'"]+)['"]\s*\)`)
	storePattern := regexp.MustCompile(`\$store\.state\.user\.([A-Za-z0-9_]+)`)
	user, department, company := make([]string, 0), make([]string, 0), make([]string, 0)
	for _, pattern := range []*regexp.Regexp{localPattern, storePattern} {
		for _, match := range pattern.FindAllStringSubmatch(source, -1) {
			if len(match) < 2 {
				continue
			}
			key := match[1]
			lower := strings.ToLower(key)
			switch {
			case strings.Contains(lower, "department") || strings.Contains(lower, "dept"):
				department = append(department, key)
			case strings.Contains(lower, "company") || strings.Contains(lower, "group"):
				company = append(company, key)
			case strings.Contains(lower, "user") || strings.Contains(lower, "initiator"):
				user = append(user, key)
			}
		}
	}
	user, department, company = uniqueCatalogStrings(user), uniqueCatalogStrings(department), uniqueCatalogStrings(company)
	if len(user) == 0 && len(department) == 0 && len(company) == 0 {
		return nil
	}
	return &target.VueCustomIdentityRule{UserKeys: user, DepartmentKeys: department, CompanyKeys: company, Source: "host_session"}
}

// javaPageRule 用本地 Java 控制器注解复验 Vue 已声明端点，未匹配的请求必须作为具体分析问题保留。
func (s *vuePageRuleScanner) javaPageRule(requests []target.VueCustomRequestRule, submit *target.VueCustomSubmitRule, identity *target.VueCustomIdentityRule) *target.JavaPageRule {
	endpoints := make([]struct{ path, method string }, 0, len(requests)+1)
	for _, request := range requests {
		endpoints = append(endpoints, struct{ path, method string }{path: request.Path, method: request.Method})
	}
	if submit != nil && strings.TrimSpace(submit.Path) != "" {
		endpoints = append(endpoints, struct{ path, method string }{path: submit.Path, method: submit.Method})
	}
	if len(endpoints) == 0 {
		return nil
	}
	result := &target.JavaPageRule{Routes: []target.JavaRouteRule{}, RequestDTO: []string{}, Response: []string{}, SuccessChecks: []string{}, FieldErrors: []string{}, DataSources: []string{}, IdentityReads: []string{}, Issues: []string{}}
	seen := map[string]bool{}
	for _, endpoint := range endpoints {
		if endpoint.path == "" || seen[endpoint.method+"\x00"+endpoint.path] {
			continue
		}
		seen[endpoint.method+"\x00"+endpoint.path] = true
		route, module, controller, found := s.javaRoute(endpoint.method, endpoint.path)
		if !found {
			result.Issues = append(result.Issues, "Java 控制器未识别端点「"+endpoint.path+"」")
			continue
		}
		// 保存端点是页面提交协议的权威后端，优先作为页面 Java 摘要；只读候选端点仍完整保留在 Routes。
		if result.Module == "" || (submit != nil && endpoint.path == submit.Path && endpoint.method == submit.Method) {
			result.Module, result.Controller = module, controller
		}
		result.Routes = append(result.Routes, route)
		if route.Request != "" {
			result.RequestDTO = append(result.RequestDTO, route.Request)
		}
		if route.Response != "" {
			result.Response = append(result.Response, route.Response)
			if route.Response == "BaseResponseProtocol" {
				result.SuccessChecks = append(result.SuccessChecks, "isSuccess")
			}
			// 识别字段级错误路径
			if submit != nil && endpoint.path == submit.Path {
				fieldErrors := s.extractJavaFieldErrorPaths(route, endpoint)
				result.FieldErrors = append(result.FieldErrors, fieldErrors...)
			}
		}
	}
	for _, request := range requests {
		if request.ReadOnly {
			result.DataSources = append(result.DataSources, request.Name)
		}
	}
	if identity != nil {
		result.IdentityReads = append(result.IdentityReads, identity.UserKeys...)
		result.IdentityReads = append(result.IdentityReads, identity.DepartmentKeys...)
		result.IdentityReads = append(result.IdentityReads, identity.CompanyKeys...)
	}
	// 只有在未识别到字段错误路径时才标记问题
	if submit != nil && len(result.FieldErrors) == 0 {
		result.Issues = append(result.Issues, "Java 保存接口未证明字段级错误返回路径")
	}
	result.RequestDTO = uniqueCatalogStrings(result.RequestDTO)
	result.Response = uniqueCatalogStrings(result.Response)
	result.SuccessChecks = uniqueCatalogStrings(result.SuccessChecks)
	result.FieldErrors = uniqueCatalogStrings(result.FieldErrors)
	result.DataSources = uniqueCatalogStrings(result.DataSources)
	result.IdentityReads = uniqueCatalogStrings(result.IdentityReads)
	result.Issues = uniqueCatalogStrings(result.Issues)
	return result
}

// javaRoute 在 Spring Controller 的类级和方法级映射中查找一个已声明端点，返回 DTO 和响应形状摘要。
func (s *vuePageRuleScanner) javaRoute(method, endpoint string) (target.JavaRouteRule, string, string, bool) {
	classPattern := regexp.MustCompile(`@RequestMapping\s*\(\s*(?:value\s*=\s*)?['"]([^'"]+)['"]`)
	signaturePattern := regexp.MustCompile(`public\s+([A-Za-z0-9_<>?, \t]+)\s+([A-Za-z0-9_]+)\s*\(([^)]*)\)`)
	requestPattern := regexp.MustCompile(`@RequestBody\s+([A-Za-z0-9_]+)`)
	for _, source := range s.javaSources {
		classMatch := classPattern.FindStringSubmatch(source.content)
		if len(classMatch) < 2 {
			continue
		}
		base := strings.TrimSuffix(classMatch[1], "/")
		if endpoint != base && !strings.HasPrefix(endpoint, base+"/") {
			continue
		}
		suffix := strings.TrimPrefix(strings.TrimPrefix(endpoint, base), "/")
		for _, marker := range javaMappingMarkers(method, suffix) {
			offset := strings.Index(source.content, marker)
			if offset < 0 {
				continue
			}
			end := offset + 1400
			if end > len(source.content) {
				end = len(source.content)
			}
			window := source.content[offset:end]
			signature := signaturePattern.FindStringSubmatch(window)
			if len(signature) < 4 {
				continue
			}
			request := ""
			if match := requestPattern.FindStringSubmatch(signature[3]); len(match) > 1 {
				request = match[1]
			}
			controller := strings.TrimSuffix(filepath.Base(source.path), ".java")
			return target.JavaRouteRule{Method: strings.ToUpper(method), Path: endpoint, Handler: signature[2], Request: request, Response: strings.TrimSpace(signature[1])}, source.module, controller, true
		}
	}
	return target.JavaRouteRule{}, "", "", false
}

// javaMappingMarkers 返回 Spring 常用映射注解的精确文本，避免用端点名称跨控制器猜测方法。
func javaMappingMarkers(method, suffix string) []string {
	name := strings.Title(strings.ToLower(strings.TrimSpace(method))) + "Mapping"
	quoted := []string{"\"" + suffix + "\"", "\"/" + suffix + "\""}
	markers := make([]string, 0, len(quoted)*2)
	for _, value := range quoted {
		markers = append(markers, "@"+name+"("+value+")", "@"+name+"(value = "+value+")")
	}
	return markers
}

// extractJavaFieldErrorPaths 从 Java 方法体中识别字段级错误返回路径。
func (s *vuePageRuleScanner) extractJavaFieldErrorPaths(route target.JavaRouteRule, endpoint struct{ path, method string }) []string {
	paths := make([]string, 0)

	// 查找包含该方法的源码文件
	for _, source := range s.javaSources {
		if !strings.Contains(source.content, route.Handler+"(") {
			continue
		}

		// 查找方法体
		handlerPattern := regexp.MustCompile(`public\s+[A-Za-z0-9_<>?, \t]+\s+` + regexp.QuoteMeta(route.Handler) + `\s*\([^)]*\)\s*\{`)
		handlerMatch := handlerPattern.FindStringIndex(source.content)
		if handlerMatch == nil {
			continue
		}

		// 提取方法体（简单括号匹配）
		start := handlerMatch[1]
		braceCount := 1
		end := start
		for i := start; i < len(source.content) && braceCount > 0; i++ {
			if source.content[i] == '{' {
				braceCount++
			} else if source.content[i] == '}' {
				braceCount--
			}
			end = i
		}

		methodBody := source.content[start:end]

		// 识别常见的字段错误模式
		fieldErrorPatterns := []struct {
			pattern *regexp.Regexp
			path    string
		}{
			{regexp.MustCompile(`\.setErrors\s*\(`), "data.errors"},
			{regexp.MustCompile(`\.setFieldErrors\s*\(`), "data.fieldErrors"},
			{regexp.MustCompile(`\.setValidationErrors\s*\(`), "data.validationErrors"},
			{regexp.MustCompile(`new\s+FieldError\s*\(`), "data.errors[].field"},
			{regexp.MustCompile(`\.addError\s*\(\s*['"]([^'"]+)['"]`), "data.errors"},
			{regexp.MustCompile(`fieldError\.setField\s*\(`), "data.errors[].field"},
			{regexp.MustCompile(`error\.put\s*\(\s*['"]field['"]`), "data.errors[].field"},
		}

		for _, pe := range fieldErrorPatterns {
			if pe.pattern.MatchString(methodBody) {
				paths = append(paths, pe.path)
			}
		}
	}

	return uniqueCatalogStrings(paths)
}

// extractVueFieldErrorPaths 从 Vue 组件源码中识别字段级错误回显逻辑。
func (s *vuePageRuleScanner) extractVueFieldErrorPaths(source string) []string {
	paths := make([]string, 0)

	// 识别常见的 Vue 错误回显模式
	errorPatterns := []struct {
		pattern *regexp.Regexp
		path    string
	}{
		// Element UI form validation
		{regexp.MustCompile(`\$refs\.form\.setFieldError\s*\(`), "errors[].field"},
		{regexp.MustCompile(`this\.errors\s*\[\s*['"]([^'"]+)['"]\s*\]`), "errors[field]"},
		{regexp.MustCompile(`this\.fieldErrors\s*=`), "fieldErrors"},
		{regexp.MustCompile(`response\.data\.errors`), "data.errors"},
		{regexp.MustCompile(`response\.data\.fieldErrors`), "data.fieldErrors"},
		{regexp.MustCompile(`response\.data\.validationErrors`), "data.validationErrors"},
		{regexp.MustCompile(`res\.data\.errors\.forEach`), "data.errors[].field"},
		{regexp.MustCompile(`error\.field\s*&&`), "data.errors[].field"},
		{regexp.MustCompile(`\.setError\s*\(\s*['"]([^'"]+)['"]`), "errors[field]"},
		// 通用错误处理
		{regexp.MustCompile(`this\.\$message\.error\s*\(\s*res\.data\.message`), "data.message"},
	}

	for _, ep := range errorPatterns {
		if ep.pattern.MatchString(source) {
			paths = append(paths, ep.path)
		}
	}

	return uniqueCatalogStrings(paths)
}

// read 读取项目内参考运行时文件，缺失只返回空文本并由上层标记规则问题。
func (s *vuePageRuleScanner) read(relative string) string {
	data, err := os.ReadFile(filepath.Join(s.root, filepath.FromSlash(relative)))
	if err != nil {
		return ""
	}
	return string(data)
}
