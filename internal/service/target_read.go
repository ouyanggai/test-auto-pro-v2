package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/formdata"
	"test-auto-pro-v2/internal/session"
)

const templateCoverageMaxPageSize = 50
const templateCoverageWorkers = 2
const templateCoverageDetailAttempts = 3
const templateCoveragePageAttempts = 3
const recentFormSampleSuccessTTL = 5 * time.Minute
const recentFormSampleFailureTTL = 15 * time.Second
const recentFormSampleMaxItems = 5

type templateCoverageDetailResult struct {
	item    target.FlowTemplate
	input   formdata.TemplateCoverageInput
	failure string
}

var (
	ErrTargetFlowNotFound        = errors.New("目标流程当前不可读取")
	ErrTargetFlowStructureEmpty  = errors.New("目标流程暂未配置节点")
	ErrTargetFlowNotConfigurable = errors.New("目标流程当前不能配置执行路径")
)

type TargetReadService struct {
	configMissing []string
	client        *target.Client
	sessions      *session.Manager
	sampleMu      sync.Mutex
	sampleCache   map[string]recentFormSampleCache
	sampleFlights map[string]*recentFormSampleFlight
}

type recentFormSampleCache struct {
	expiresAt time.Time
	values    []map[string]any
	err       error
}

type recentFormSampleFlight struct {
	done chan struct{}
}

// NewTargetReadService 从后端运行配置创建只读目标客户端和会话管理器。
func NewTargetReadService(cfg config.TargetConfig) *TargetReadService {
	missing := cfg.MissingRequired()
	if len(missing) > 0 {
		return &TargetReadService{configMissing: missing}
	}
	client, err := target.NewClient(target.ClientConfig{
		BaseURL:               cfg.APIGateway,
		LoginPassword:         cfg.LoginPassword,
		LoginAESKey:           cfg.LoginAESKey,
		LoginCode:             cfg.LoginCode,
		PlatformCode:          cfg.PlatformCode,
		TemplatePlatformCodes: cfg.TemplatePlatformCodes,
		CustomerCode:          cfg.CustomerCode,
		Timeout:               cfg.HTTPTimeout,
	})
	if err != nil {
		return &TargetReadService{configMissing: []string{"TARGET_API_GATEWAY"}}
	}
	return &TargetReadService{
		client:        client,
		sessions:      session.NewManager(client, cfg.SessionTTL),
		sampleCache:   make(map[string]recentFormSampleCache),
		sampleFlights: make(map[string]*recentFormSampleFlight),
	}
}

// NewTargetReadServiceWithClient 为假目标集成测试注入客户端和会话有效期。
func NewTargetReadServiceWithClient(client *target.Client, ttl time.Duration) *TargetReadService {
	return &TargetReadService{
		client: client, sessions: session.NewManager(client, ttl),
		sampleCache: make(map[string]recentFormSampleCache), sampleFlights: make(map[string]*recentFormSampleFlight),
	}
}

// Verify 验证账号并只返回非敏感摘要。
func (s *TargetReadService) Verify(ctx context.Context, account string) (target.AccountSummary, error) {
	if err := s.ready(); err != nil {
		return target.AccountSummary{}, err
	}
	return s.sessions.Verify(ctx, account)
}

// Templates 分页读取账号可见的流程模板。
func (s *TargetReadService) Templates(ctx context.Context, account, query string, page, pageSize int) (target.Page[target.FlowTemplate], error) {
	if err := s.ready(); err != nil {
		return target.Page[target.FlowTemplate]{}, err
	}
	var result target.Page[target.FlowTemplate]
	err := s.sessions.DoRead(ctx, account, func(callContext context.Context, active target.Session) error {
		pageResult, err := s.client.ListTemplates(callContext, active, query, page, pageSize)
		if err == nil {
			result = pageResult
		}
		return err
	})
	return result, err
}

// TemplateConfiguration 读取单个可见流程的完整配置快照，供本地规则目录同步一次分析后持久保存。
func (s *TargetReadService) TemplateConfiguration(ctx context.Context, account, templateID string) (target.PathConfigurationSnapshot, error) {
	if err := s.ready(); err != nil {
		return target.PathConfigurationSnapshot{}, err
	}
	var result target.PathConfigurationSnapshot
	err := s.sessions.DoRead(ctx, account, func(callContext context.Context, active target.Session) error {
		var err error
		result, err = s.client.ReadTemplateConfiguration(callContext, active, strings.TrimSpace(templateID))
		return err
	})
	return result, err
}

// ComponentCandidates 使用当前计划发起人的独立会话读取单个已验证组件候选类型。
func (s *TargetReadService) ComponentCandidates(ctx context.Context, account, _ string, componentType string) ([]any, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	var result []any
	err := s.sessions.DoRead(ctx, account, func(callContext context.Context, active target.Session) error {
		values, readErr := s.client.ListComponentCandidates(callContext, active, componentType)
		if readErr == nil {
			result = values
		}
		return readErr
	})
	return result, err
}

// TemplateCoverage 分页读取账号当前可见的全部模板并增量盘点规则，不在服务端累计完整模板正文。
func (s *TargetReadService) TemplateCoverage(ctx context.Context, account string) (formdata.TemplateCoverageReport, error) {
	if err := s.ready(); err != nil {
		return formdata.TemplateCoverageReport{}, err
	}
	report := formdata.NewTemplateCoverageReport()
	metadataPage, err := s.readTemplateCoveragePage(ctx, account, 1, 1)
	if err != nil {
		return formdata.TemplateCoverageReport{}, fmt.Errorf("模板列表总数读取失败：%w", err)
	}
	expectedTemplates := metadataPage.Total
	paginationComplete := true
	seenTemplateIDs := make(map[string]struct{})
	processedTargetItems := 0
	for {
		if expectedTemplates >= 0 && processedTargetItems >= expectedTemplates {
			break
		}
		pageSizeLimit := templateCoverageMaxPageSize
		page, pageSize := templateCoveragePagePosition(processedTargetItems, expectedTemplates, pageSizeLimit)
		pageResult, readErr := s.readTemplateCoveragePage(ctx, account, page, pageSize)
		for readErr != nil && pageSize > 1 {
			pageSizeLimit = pageSize / 2
			page, pageSize = templateCoveragePagePosition(processedTargetItems, expectedTemplates, pageSizeLimit)
			pageResult, readErr = s.readTemplateCoveragePage(ctx, account, page, pageSize)
		}
		if readErr != nil {
			// 单条列表数据自身损坏时没有安全方式取得模板 ID；记录失败槽位并继续后续偏移。
			formdata.AddTemplateCoverageFailure(&report, "", "", "目标模板列表单条无法读取")
			paginationComplete = false
			processedTargetItems++
			continue
		}
		if pageResult.Total != expectedTemplates {
			paginationComplete = false
		}
		// 目标声明仍有下一页却返回空页时立即停止，避免异常分页造成无界请求。
		if len(pageResult.Items) == 0 && pageResult.HasMore {
			paginationComplete = false
			break
		}
		processedTargetItems += len(pageResult.Items)
		pageItems := make([]target.FlowTemplate, 0, len(pageResult.Items))
		for _, item := range pageResult.Items {
			id := strings.TrimSpace(item.ID)
			if id == "" {
				paginationComplete = false
				formdata.AddTemplateCoverageFailure(&report, item.TypeName, item.Code, "模板标识缺失")
				continue
			}
			if _, exists := seenTemplateIDs[id]; exists {
				paginationComplete = false
				continue
			}
			seenTemplateIDs[id] = struct{}{}
			pageItems = append(pageItems, item)
		}
		if err := s.scanTemplateCoverageDetails(ctx, account, pageItems, &report); err != nil {
			return formdata.TemplateCoverageReport{}, err
		}
		if !pageResult.HasMore {
			break
		}
	}
	formdata.FinalizeTemplateCoverageReport(&report, expectedTemplates, paginationComplete)
	return report, nil
}

// templateCoveragePagePosition 把已处理数量转换为目标页码，并按当前上限缩页而不改变真实偏移。
func templateCoveragePagePosition(processed, total, limit int) (int, int) {
	pageSize := limit
	if pageSize < 1 || pageSize > templateCoverageMaxPageSize {
		pageSize = templateCoverageMaxPageSize
	}
	if total >= 0 {
		remaining := total - processed
		if remaining < 1 {
			return 1, 1
		}
		if remaining < pageSize {
			pageSize = remaining
		}
	}
	// 目标协议只有页码而没有 offset；页大小必须整除已处理数量才能精确从下一个模板继续。
	for pageSize > 1 && processed%pageSize != 0 {
		pageSize--
	}
	return processed/pageSize + 1, pageSize
}

// readTemplateCoveragePage 对单个模板列表页执行有界瞬时故障重试，不会从第一页重新扫描。
func (s *TargetReadService) readTemplateCoveragePage(ctx context.Context, account string, page, pageSize int) (target.Page[target.FlowTemplate], error) {
	var result target.Page[target.FlowTemplate]
	var lastErr error
	for attempt := 1; attempt <= templateCoveragePageAttempts; attempt++ {
		lastErr = s.sessions.DoRead(ctx, account, func(callContext context.Context, active target.Session) error {
			pageResult, err := s.client.ListTemplates(callContext, active, "", page, pageSize)
			if err == nil {
				result = pageResult
			}
			return err
		})
		if lastErr == nil || !templateCoverageRetryable(lastErr) || attempt == templateCoveragePageAttempts {
			break
		}
		if err := waitTemplateCoverageRetry(ctx, attempt, time.Second); err != nil {
			return target.Page[target.FlowTemplate]{}, err
		}
	}
	return result, lastErr
}

// scanTemplateCoverageDetails 使用固定并发读取单页详情，并在消费结果后立即释放模板正文。
func (s *TargetReadService) scanTemplateCoverageDetails(ctx context.Context, account string, items []target.FlowTemplate, report *formdata.TemplateCoverageReport) error {
	if len(items) == 0 {
		return nil
	}
	workerCount := templateCoverageWorkers
	if len(items) < workerCount {
		workerCount = len(items)
	}
	jobs := make(chan target.FlowTemplate)
	results := make(chan templateCoverageDetailResult, workerCount)
	var workers sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for item := range jobs {
				results <- s.readTemplateCoverageDetail(ctx, account, item)
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, item := range items {
			jobs <- item
		}
	}()
	go func() {
		workers.Wait()
		close(results)
	}()
	for result := range results {
		if result.failure != "" {
			formdata.AddTemplateCoverageFailure(report, result.item.TypeName, result.item.Code, result.failure)
			continue
		}
		formdata.AddTemplateCoverage(report, result.input)
	}
	return ctx.Err()
}

// readTemplateCoverageDetail 对单个模板详情执行有界重试，并把目标原文及时转换为轻量规则输入。
func (s *TargetReadService) readTemplateCoverageDetail(ctx context.Context, account string, item target.FlowTemplate) templateCoverageDetailResult {
	var tree *target.FlowNodeTemplate
	var forms []target.FormRuntimeTemplate
	var lastErr error
	for attempt := 1; attempt <= templateCoverageDetailAttempts; attempt++ {
		lastErr = s.sessions.DoRead(ctx, account, func(callContext context.Context, active target.Session) error {
			var err error
			tree, forms, err = s.client.ReadTemplateRuleSource(callContext, active, item.ID)
			return err
		})
		if lastErr == nil || !templateCoverageRetryable(lastErr) || attempt == templateCoverageDetailAttempts {
			break
		}
		if err := waitTemplateCoverageRetry(ctx, attempt, 250*time.Millisecond); err != nil {
			lastErr = err
			break
		}
	}
	if lastErr != nil {
		return templateCoverageDetailResult{item: item, failure: templateCoverageFailureReason(lastErr)}
	}
	template, mergeIssues := runtimeTemplate(forms)
	operators, logic, fieldComparisons, conditionIssues := templateConditionRuleCoverage(tree)
	return templateCoverageDetailResult{item: item, input: formdata.TemplateCoverageInput{
		TemplateType: item.TypeName, FlowCode: item.Code, Template: template, MergeIssues: mergeIssues,
		ConditionOperators: operators, ConditionLogic: logic, FieldComparisonCount: fieldComparisons, ConditionIssues: conditionIssues,
	}}
}

// templateCoverageRetryable 仅重试目标暂不可用和超时，响应结构错误必须直接进入人工核对。
func templateCoverageRetryable(err error) bool {
	return target.IsKind(err, target.ErrorUnavailable) || target.IsKind(err, target.ErrorTimeout)
}

// waitTemplateCoverageRetry 使用有界退避保护目标平台，并允许调用方取消长时间全量盘点。
func waitTemplateCoverageRetry(ctx context.Context, attempt int, base time.Duration) error {
	timer := time.NewTimer(time.Duration(attempt) * base)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// templateConditionRuleCoverage 递归提取流程条件能力，只有求解器已验证的比较和连接方式计入覆盖。
func templateConditionRuleCoverage(tree *target.FlowNodeTemplate) ([]string, []string, int, []string) {
	operators := make([]string, 0)
	logic := make([]string, 0)
	fieldComparisons := 0
	issues := make([]string, 0)
	visitTargetTree(tree, map[string]bool{}, func(node *target.FlowNodeTemplate) {
		branches := append(append([]target.FlowBranchTemplate{}, node.ConditionNodes...), node.ParallelNodes...)
		for _, branch := range branches {
			for _, condition := range branch.Conditions {
				operator := normalizeConditionJudge(condition.Judge)
				switch operator {
				case "eq", "neq", "gt", "gte", "lt", "lte", "contains", "in":
					operators = append(operators, operator)
				default:
					issues = append(issues, "未知流程条件比较方式")
				}
				if strings.TrimSpace(condition.FieldB) != "" {
					fieldComparisons++
				}
				conditionLogic := strings.ToLower(strings.TrimSpace(condition.ConditionType))
				if conditionLogic == "" {
					continue
				}
				switch conditionLogic {
				case "and", "or":
					logic = append(logic, conditionLogic)
				default:
					issues = append(issues, "未知流程条件连接方式")
				}
			}
		}
	})
	return operators, logic, fieldComparisons, uniquePublicStrings(issues)
}

// templateCoverageFailureReason 把单模板详情故障收敛为稳定中文类别，不泄露目标原始响应。
func templateCoverageFailureReason(err error) string {
	switch {
	case target.IsKind(err, target.ErrorResponseInvalid):
		return "模板详情结构无法识别"
	case target.IsKind(err, target.ErrorTimeout):
		return "模板详情读取超时"
	default:
		return "模板详情暂时无法读取"
	}
}

// Submitted 分页读取账号已发流程实例。
func (s *TargetReadService) Submitted(ctx context.Context, account, query string, page, pageSize int) (target.Page[target.SubmittedFlow], error) {
	if err := s.ready(); err != nil {
		return target.Page[target.SubmittedFlow]{}, err
	}
	var result target.Page[target.SubmittedFlow]
	err := s.sessions.DoRead(ctx, account, func(callContext context.Context, active target.Session) error {
		pageResult, err := s.client.ListSubmitted(callContext, active, query, page, pageSize)
		if err == nil {
			result = pageResult
		}
		return err
	})
	return result, err
}

// Due 分页读取账号待发流程实例。
func (s *TargetReadService) Due(ctx context.Context, account, query string, page, pageSize int) (target.Page[target.DueFlow], error) {
	if err := s.ready(); err != nil {
		return target.Page[target.DueFlow]{}, err
	}
	var result target.Page[target.DueFlow]
	err := s.sessions.DoRead(ctx, account, func(callContext context.Context, active target.Session) error {
		pageResult, err := s.client.ListDue(callContext, active, query, page, pageSize)
		if err == nil {
			result = pageResult
		}
		return err
	})
	return result, err
}

// FlowTree 兼容只读图调用，仅返回树结构而不公开运行态入口。
func (s *TargetReadService) FlowTree(ctx context.Context, account, source, targetObjectID string) (*target.FlowNodeTemplate, error) {
	snapshot, err := s.FlowTreeSnapshot(ctx, account, source, targetObjectID)
	if err != nil {
		return nil, err
	}
	return snapshot.Tree, nil
}

// FlowTreeSnapshot 按来源精确核对保存目标，并把真实代理树与当前入口绑定读取。
func (s *TargetReadService) FlowTreeSnapshot(ctx context.Context, account, source, targetObjectID string) (target.FlowTreeSnapshot, error) {
	if err := s.ready(); err != nil {
		return target.FlowTreeSnapshot{}, err
	}
	var result target.FlowTreeSnapshot
	err := s.sessions.DoRead(ctx, account, func(callContext context.Context, active target.Session) error {
		var tree *target.FlowNodeTemplate
		var entryNodeIDs []string
		var err error
		switch strings.TrimSpace(source) {
		case "new":
			visible, findErr := s.client.FindVisibleTemplate(callContext, active, targetObjectID)
			if findErr != nil {
				return findErr
			}
			if !visible {
				return ErrTargetFlowNotFound
			}
			tree, err = s.client.ReadTemplateTree(callContext, active, targetObjectID)
		case "started":
			proxyID, entries, status, _, found, findErr := s.client.FindSubmittedFlow(callContext, active, targetObjectID)
			if findErr != nil {
				return findErr
			}
			if !found {
				return ErrTargetFlowNotFound
			}
			// 已结束或未知状态不能靠仍存在的 end 节点伪装成零选择完整路径。
			if !submittedFlowConfigurable(status) {
				return ErrTargetFlowNotConfigurable
			}
			entryNodeIDs = entries
			tree, err = s.client.ReadProxyTree(callContext, active, proxyID)
		case "pending":
			// waiting_send 任务本身是可继续证据；不存在任务时不会退回实例名称猜测入口。
			proxyID, entries, _, found, findErr := s.client.FindDueFlow(callContext, active, targetObjectID)
			if findErr != nil {
				return findErr
			}
			if !found {
				return ErrTargetFlowNotFound
			}
			entryNodeIDs = entries
			tree, err = s.client.ReadProxyTree(callContext, active, proxyID)
		default:
			return ErrTargetFlowNotFound
		}
		if err != nil {
			return err
		}
		if tree == nil {
			return ErrTargetFlowStructureEmpty
		}
		if strings.TrimSpace(source) == "new" {
			entryNodeIDs = []string{strings.TrimSpace(tree.ID)}
		}
		result = target.FlowTreeSnapshot{Tree: tree, EntryNodeIDs: entryNodeIDs}
		return nil
	})
	return result, err
}

// FlowRequirementSnapshot 按计划保存的来源读取同一流程树、当前入口和对应表单字段元数据。
func (s *TargetReadService) FlowRequirementSnapshot(ctx context.Context, account, source, targetObjectID string) (target.FlowRequirementSnapshot, error) {
	if err := s.ready(); err != nil {
		return target.FlowRequirementSnapshot{}, err
	}
	var result target.FlowRequirementSnapshot
	err := s.sessions.DoRead(ctx, account, func(callContext context.Context, active target.Session) error {
		var tree *target.FlowNodeTemplate
		var fields []target.FormFieldMetadata
		var entries []string
		var err error
		switch strings.TrimSpace(source) {
		case "new":
			visible, findErr := s.client.FindVisibleTemplate(callContext, active, targetObjectID)
			if findErr != nil {
				return findErr
			}
			if !visible {
				return ErrTargetFlowNotFound
			}
			tree, fields, err = s.client.ReadTemplateRequirements(callContext, active, targetObjectID)
		case "started":
			var proxyID, status string
			var formProxyIDs []string
			var found bool
			proxyID, entries, status, formProxyIDs, found, err = s.client.FindSubmittedFlow(callContext, active, targetObjectID)
			if err != nil {
				return err
			}
			if !found {
				return ErrTargetFlowNotFound
			}
			if !submittedFlowConfigurable(status) {
				return ErrTargetFlowNotConfigurable
			}
			tree, fields, err = s.client.ReadProxyRequirements(callContext, active, proxyID, formProxyIDs)
		case "pending":
			var proxyID string
			var formProxyIDs []string
			var found bool
			proxyID, entries, formProxyIDs, found, err = s.client.FindDueFlow(callContext, active, targetObjectID)
			if err != nil {
				return err
			}
			if !found {
				return ErrTargetFlowNotFound
			}
			tree, fields, err = s.client.ReadProxyRequirements(callContext, active, proxyID, formProxyIDs)
		default:
			return ErrTargetFlowNotFound
		}
		if err != nil {
			return err
		}
		if tree == nil {
			return ErrTargetFlowStructureEmpty
		}
		if strings.TrimSpace(source) == "new" {
			entries = []string{strings.TrimSpace(tree.ID)}
		}
		result = target.FlowRequirementSnapshot{Tree: tree, EntryNodeIDs: entries, FormFields: fields}
		return nil
	})
	return result, err
}

// PathConfigurationSnapshot 按计划保存的来源读取流程树、字段详情和实例现值，供配置工作台使用。
func (s *TargetReadService) PathConfigurationSnapshot(ctx context.Context, account, source, targetObjectID string) (target.PathConfigurationSnapshot, error) {
	if err := s.ready(); err != nil {
		return target.PathConfigurationSnapshot{}, err
	}
	var result target.PathConfigurationSnapshot
	err := s.sessions.DoRead(ctx, account, func(callContext context.Context, active target.Session) error {
		var tree *target.FlowNodeTemplate
		var fields []target.FormFieldDetail
		var forms []target.FormRuntimeTemplate
		var values map[string]any
		var flowCode string
		var flowName string
		var auditWay string
		var renderType target.FormRenderType
		var vuePage *target.VueCustomPageRule
		var entries []string
		var err error
		switch strings.TrimSpace(source) {
		case "new":
			visible, findErr := s.client.FindVisibleTemplate(callContext, active, targetObjectID)
			if findErr != nil {
				return findErr
			}
			if !visible {
				return ErrTargetFlowNotFound
			}
			var snapshot target.PathConfigurationSnapshot
			snapshot, err = s.client.ReadTemplateConfiguration(callContext, active, targetObjectID)
			tree, fields, forms, flowCode, flowName, auditWay, renderType, vuePage = snapshot.Tree, snapshot.FormFields, snapshot.Forms, snapshot.FlowCode, snapshot.FlowName, snapshot.AuditWay, snapshot.RenderType, snapshot.VuePage
		case "started":
			var proxyID, status string
			var formProxyIDs []string
			var found bool
			proxyID, entries, status, formProxyIDs, found, err = s.client.FindSubmittedFlow(callContext, active, targetObjectID)
			if err != nil {
				return err
			}
			if !found {
				return ErrTargetFlowNotFound
			}
			if !submittedFlowConfigurable(status) {
				return ErrTargetFlowNotConfigurable
			}
			var snapshot target.PathConfigurationSnapshot
			snapshot, err = s.client.ReadProxyConfiguration(callContext, active, proxyID, formProxyIDs, targetObjectID)
			tree, fields, forms, values, flowCode, flowName, auditWay, renderType, vuePage = snapshot.Tree, snapshot.FormFields, snapshot.Forms, snapshot.InstanceValues, snapshot.FlowCode, snapshot.FlowName, snapshot.AuditWay, snapshot.RenderType, snapshot.VuePage
		case "pending":
			var proxyID string
			var formProxyIDs []string
			var found bool
			proxyID, entries, formProxyIDs, found, err = s.client.FindDueFlow(callContext, active, targetObjectID)
			if err != nil {
				return err
			}
			if !found {
				return ErrTargetFlowNotFound
			}
			var snapshot target.PathConfigurationSnapshot
			snapshot, err = s.client.ReadProxyConfiguration(callContext, active, proxyID, formProxyIDs, targetObjectID)
			tree, fields, forms, values, flowCode, flowName, auditWay, renderType, vuePage = snapshot.Tree, snapshot.FormFields, snapshot.Forms, snapshot.InstanceValues, snapshot.FlowCode, snapshot.FlowName, snapshot.AuditWay, snapshot.RenderType, snapshot.VuePage
		default:
			return ErrTargetFlowNotFound
		}
		if err != nil {
			return err
		}
		if tree == nil {
			return ErrTargetFlowStructureEmpty
		}
		if strings.TrimSpace(source) == "new" {
			entries = []string{strings.TrimSpace(tree.ID)}
		}
		result = target.PathConfigurationSnapshot{Tree: tree, EntryNodeIDs: entries, FlowCode: flowCode, FlowName: flowName, AuditWay: auditWay, RenderType: renderType, VuePage: vuePage, FormFields: fields, Forms: forms, InstanceValues: values}
		return nil
	})
	return result, err
}

// FormRuntimeSession 从现有账号验证缓存建立短期 iframe 上下文，不额外持久化或复制 SID。
func (s *TargetReadService) FormRuntimeSession(ctx context.Context, account string) (target.FormRuntimeSession, error) {
	if err := s.ready(); err != nil {
		return target.FormRuntimeSession{}, err
	}
	active, err := s.sessions.Current(ctx, account)
	if err != nil {
		return target.FormRuntimeSession{}, err
	}
	identity, identityErr := s.client.FormIdentityContext(ctx, active)
	if identityErr != nil {
		return target.FormRuntimeSession{}, identityErr
	}
	companyName := strings.TrimSpace(identity.Company.Name)
	if companyName == "" {
		companyName = strings.TrimSpace(active.Summary.CompanyName)
	}
	departmentID := strings.TrimSpace(identity.Department.ID)
	if departmentID == "" {
		departmentID = strings.TrimSpace(active.DepartmentID)
	}
	departmentName := strings.TrimSpace(identity.Department.Name)
	return target.FormRuntimeSession{
		SID: active.SID, BaseURL: s.client.BaseURL(), AccountName: active.Summary.DisplayName,
		UserID: active.UserID, CompanyID: active.CompanyID, CustomerCode: active.CustomerCode, CompanyName: companyName,
		DepartmentID: departmentID, DepartmentName: departmentName,
	}, nil
}

// FormIdentityContext 只使用同一次已验证运行时会话解析公司、部门与本人目录，禁止重新按账号登录或混用其他发起人权限。
func (s *TargetReadService) FormIdentityContext(ctx context.Context, active target.Session) (target.FormIdentityContext, error) {
	if err := s.ready(); err != nil {
		return target.FormIdentityContext{}, err
	}
	if strings.TrimSpace(active.SID) == "" || strings.TrimSpace(active.UserID) == "" || strings.TrimSpace(active.CompanyID) == "" {
		return target.FormIdentityContext{}, errors.New("表单身份会话信息不完整")
	}
	return s.client.FormIdentityContext(ctx, active)
}

// RecentFormSamples 只读取同一真实流程编码的近期样本，保留旧调用边界并使用空规则维度。
func (s *TargetReadService) RecentFormSamples(ctx context.Context, account, flowCode string, limit int) ([]map[string]any, error) {
	return s.recentFormSamplesForRule(ctx, account, flowCode, "", "", "", limit)
}

// RecentFormSamplesForRule 按账号、流程、模板、组件和规则版本隔离近期样本，避免对象值跨权限或跨规则复用。
func (s *TargetReadService) RecentFormSamplesForRule(ctx context.Context, account, flowCode, templateID, componentID, ruleVersion string, limit int) ([]map[string]any, error) {
	return s.recentFormSamplesForRule(ctx, account, flowCode, templateID, componentID, ruleVersion, limit)
}

// recentFormSamplesForRule 读取并缓存同一规则快照的有限样本，目标端分页和实例读取始终沿用单账号会话。
func (s *TargetReadService) recentFormSamplesForRule(ctx context.Context, account, flowCode, templateID, componentID, ruleVersion string, limit int) ([]map[string]any, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if limit < 1 {
		limit = 1
	}
	if limit > recentFormSampleMaxItems {
		limit = recentFormSampleMaxItems
	}
	flowCode = strings.TrimSpace(flowCode)
	if flowCode == "" {
		return []map[string]any{}, nil
	}
	cacheKey := recentFormSampleCacheKey(account, flowCode, templateID, componentID, ruleVersion)
	for {
		now := time.Now()
		s.sampleMu.Lock()
		cached, found := s.sampleCache[cacheKey]
		if found && !now.Before(cached.expiresAt) {
			delete(s.sampleCache, cacheKey)
			found = false
		}
		if found {
			s.sampleMu.Unlock()
			return limitRecentSamples(cached.values, limit), cached.err
		}
		if flight, running := s.sampleFlights[cacheKey]; running {
			s.sampleMu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-flight.done:
				// 发起读取的调用方取消时不写负缓存，其他等待者重新竞争并发起自己的读取。
				continue
			}
		}
		flight := &recentFormSampleFlight{done: make(chan struct{})}
		s.sampleFlights[cacheKey] = flight
		s.sampleMu.Unlock()

		// 远程读取必须在锁外完成，其他账号或规则维度不能被一个慢目标请求阻塞。
		values, readErr := s.readRecentFormSamples(ctx, account, flowCode)
		cacheResult := readErr == nil || !errors.Is(readErr, context.Canceled)
		s.sampleMu.Lock()
		if cacheResult {
			ttl := recentFormSampleSuccessTTL
			if readErr != nil {
				ttl = recentFormSampleFailureTTL
			}
			s.sampleCache[cacheKey] = recentFormSampleCache{
				expiresAt: time.Now().Add(ttl), values: cloneRecentSamples(values), err: readErr,
			}
		}
		delete(s.sampleFlights, cacheKey)
		close(flight.done)
		s.sampleMu.Unlock()
		return limitRecentSamples(values, limit), readErr
	}
}

// readRecentFormSamples 从目标精确流程第一页读取最多五条样本，供同键单飞的拥有者调用。
func (s *TargetReadService) readRecentFormSamples(ctx context.Context, account, flowCode string) ([]map[string]any, error) {
	result := make([]map[string]any, 0, recentFormSampleMaxItems)
	err := s.sessions.DoRead(ctx, account, func(callContext context.Context, active target.Session) error {
		// 固定读取五条后由调用方按 limit 截取，避免不同 limit 绕过同一权限维度的缓存和单飞。
		page, pageErr := s.client.ListSubmittedByFlowCode(callContext, active, flowCode, 1, recentFormSampleMaxItems)
		if pageErr != nil {
			return pageErr
		}
		// 实例详情保持固定单并发；本地再次核对 flowCode，防御目标端返回越界记录。
		for _, item := range page.Items {
			if len(result) >= recentFormSampleMaxItems || strings.TrimSpace(item.FlowCode) != flowCode {
				continue
			}
			values, readErr := s.client.ReadInstanceCurrentData(callContext, active, item.ID)
			if readErr != nil {
				return readErr
			}
			if len(values) > 0 {
				result = append(result, values)
			}
		}
		return nil
	})
	return result, err
}

// recentFormSampleCacheKey 显式包含所有规则隔离维度，任何一项变化都不会命中旧样本。
func recentFormSampleCacheKey(account, flowCode, templateID, componentID, ruleVersion string) string {
	return strings.Join([]string{strings.TrimSpace(account), strings.TrimSpace(flowCode), strings.TrimSpace(templateID), strings.TrimSpace(componentID), strings.TrimSpace(ruleVersion)}, "|")
}

// cloneRecentSamples 深复制近期样本，生成器修改 values 时不会污染跨请求缓存。
func cloneRecentSamples(values []map[string]any) []map[string]any {
	data, _ := json.Marshal(values)
	result := make([]map[string]any, 0)
	_ = json.Unmarshal(data, &result)
	return result
}

// limitRecentSamples 深复制并截取调用方请求数量，缓存始终保存同一流程最多五条完整样本。
func limitRecentSamples(values []map[string]any, limit int) []map[string]any {
	if limit > len(values) {
		limit = len(values)
	}
	return cloneRecentSamples(values[:limit])
}

// submittedFlowConfigurable 只允许参考页面明确展示且仍可继续的已发状态。
func submittedFlowConfigurable(status string) bool {
	switch strings.TrimSpace(status) {
	case "run", "await_sent":
		return true
	default:
		return false
	}
}

// ready 在目标配置缺失时保持健康接口可用，并让业务接口返回稳定配置错误。
func (s *TargetReadService) ready() error {
	if len(s.configMissing) > 0 || s.client == nil || s.sessions == nil {
		return &config.MissingTargetConfigError{Names: append([]string(nil), s.configMissing...)}
	}
	return nil
}
