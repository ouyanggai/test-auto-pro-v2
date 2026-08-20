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
	item.Status = "complete"
	if len(item.Issues) > 0 {
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
	vueSourceDigest  string
	javaSourceDigest string
	componentDigest  string
}

type vuePageEntry struct {
	name      string
	component string
	isShow    bool
}

// newVuePageRuleScanner 创建可在分析任务中复用的只读源码扫描器。
func newVuePageRuleScanner(root string) *vuePageRuleScanner {
	scanner := &vuePageRuleScanner{root: root, components: map[string]string{}, componentFiles: map[string]string{}, pageByCode: map[string]vuePageEntry{}}
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
	s.vueSourceDigest = digestCatalogDirectory(filepath.Join(s.root, "form-runtime", "runtime-source", "src"), map[string]bool{".js": true, ".vue": true, ".json": true})
	s.javaSourceDigest = digestCatalogDirectory(filepath.Join(s.root, "参考代码", "java-serve"), map[string]bool{".java": true})
	s.componentDigest = catalogDigest(struct {
		Registry     map[string]string
		Capabilities map[string]map[string]string
		Source       string
	}{s.components, formdata.CustomComponentCapabilities(), digestCatalogDirectory(filepath.Join(s.root, "form-runtime", "runtime-source", "src", "components", "Custom"), map[string]bool{".js": true, ".vue": true})})
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
	page := target.VueCustomPageRule{PageKey: flowCode, PageName: entry.name, ComponentName: entry.component, Route: flowCode, Fields: []target.VueCustomFieldRule{}, Issues: []string{}}
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
	if !found {
		page.Issues = append(page.Issues, "宿主 Vue 页面字段规则尚未完整识别")
	}
	if strings.TrimSpace(formExist) == "" {
		page.Issues = append(page.Issues, "宿主页面缺少渲染标记")
	}
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
	if path == "" {
		return nil, "", false
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, "", false
	}
	fields := parseVueConfigFields(string(content))
	component := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	component = strings.TrimSuffix(component, "Config")
	return fields, component, len(fields) > 0
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
		fields = append(fields, target.VueCustomFieldRule{
			Path: path, Name: title, ValueType: normalizeVueFieldType(typeName), Required: strings.Contains(window, "required: true") || strings.Contains(window, "isRequire: true"),
			ReadOnly: strings.Contains(window, "disabled: true"), DefaultValue: defaultValue,
			CandidateKind: candidateKind, DataSource: vueFieldDataSource(candidateKind, window), Options: options,
		})
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

// componentFields 从宿主 Vue 组件提取 v-model 与 data 字段，覆盖非配置式页面的真实状态入口。
func (s *vuePageRuleScanner) componentFields(component string) ([]target.VueCustomFieldRule, bool) {
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
		if path == "" || strings.Contains(path, "[") || strings.HasPrefix(path, "scope.") || strings.HasPrefix(path, "val.") || seen[path] {
			continue
		}
		seen[path] = true
		fields = append(fields, target.VueCustomFieldRule{Path: path, Name: path, ValueType: "runtime", CandidateKind: "runtime"})
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
		fields = append(fields, target.VueCustomFieldRule{
			Path: path, Name: path, ValueType: "runtime", Required: requiredPattern.MatchString(content), ReadOnly: true,
			CandidateKind: "runtime_source", DataSource: dataSource,
		})
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
		if path == "" || strings.Contains(path, "[") || strings.HasPrefix(path, "scope.") || strings.HasPrefix(path, "val.") || seen[path] {
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
		fields = append(fields, target.VueCustomFieldRule{
			Path: path, Name: name, ValueType: normalizeVueFieldType(typeName), Required: vueTemplateFieldRequired(content, attributes, prop),
			ReadOnly: strings.Contains(body, "disabled") || strings.Contains(body, ":disabled"), CandidateKind: candidateKind,
			DataSource: vueFieldDataSource(candidateKind, body), Options: options,
		})
	}
	return fields
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

// read 读取项目内参考运行时文件，缺失只返回空文本并由上层标记规则问题。
func (s *vuePageRuleScanner) read(relative string) string {
	data, err := os.ReadFile(filepath.Join(s.root, filepath.FromSlash(relative)))
	if err != nil {
		return ""
	}
	return string(data)
}
