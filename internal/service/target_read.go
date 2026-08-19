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
}

type recentFormSampleCache struct {
	expiresAt time.Time
	values    []map[string]any
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
		client:      client,
		sessions:    session.NewManager(client, cfg.SessionTTL),
		sampleCache: make(map[string]recentFormSampleCache),
	}
}

// NewTargetReadServiceWithClient 为假目标集成测试注入客户端和会话有效期。
func NewTargetReadServiceWithClient(client *target.Client, ttl time.Duration) *TargetReadService {
	return &TargetReadService{client: client, sessions: session.NewManager(client, ttl), sampleCache: make(map[string]recentFormSampleCache)}
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
			tree, fields, forms, flowCode, renderType, vuePage = snapshot.Tree, snapshot.FormFields, snapshot.Forms, snapshot.FlowCode, snapshot.RenderType, snapshot.VuePage
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
			tree, fields, forms, values, flowCode, renderType, vuePage = snapshot.Tree, snapshot.FormFields, snapshot.Forms, snapshot.InstanceValues, snapshot.FlowCode, snapshot.RenderType, snapshot.VuePage
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
			tree, fields, forms, values, flowCode, renderType, vuePage = snapshot.Tree, snapshot.FormFields, snapshot.Forms, snapshot.InstanceValues, snapshot.FlowCode, snapshot.RenderType, snapshot.VuePage
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
		result = target.PathConfigurationSnapshot{Tree: tree, EntryNodeIDs: entries, FlowCode: flowCode, RenderType: renderType, VuePage: vuePage, FormFields: fields, Forms: forms, InstanceValues: values}
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

// FormIdentityContext 从已验证账号缓存解析公司目录中的公司、部门与本人节点，供表单选人/选公司组件自动填充。
func (s *TargetReadService) FormIdentityContext(ctx context.Context, account string) (target.FormIdentityContext, error) {
	if err := s.ready(); err != nil {
		return target.FormIdentityContext{}, err
	}
	active, err := s.sessions.Current(ctx, account)
	if err != nil {
		return target.FormIdentityContext{}, err
	}
	return s.client.FormIdentityContext(ctx, active)
}

// RecentFormSamples 只读取同一真实流程编码的近期样本，缓存键必须隔离账号和流程避免跨模板复用。
func (s *TargetReadService) RecentFormSamples(ctx context.Context, account, flowCode string, limit int) ([]map[string]any, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 5 {
		limit = 5
	}
	flowCode = strings.TrimSpace(flowCode)
	if flowCode == "" {
		return []map[string]any{}, nil
	}
	cacheKey := strings.TrimSpace(account) + "|" + flowCode
	s.sampleMu.Lock()
	cached, found := s.sampleCache[cacheKey]
	s.sampleMu.Unlock()
	if found && time.Now().Before(cached.expiresAt) {
		return cloneRecentSamples(cached.values), nil
	}
	result := make([]map[string]any, 0, limit)
	err := s.sessions.DoRead(ctx, account, func(callContext context.Context, active target.Session) error {
		pageNumber := 1
		for len(result) < limit {
			// 列表查询不把流程编码当作实例名称，避免目标端按名称误筛；逐页按真实 flowCode 精确过滤。
			page, pageErr := s.client.ListSubmitted(callContext, active, "", pageNumber, 20)
			if pageErr != nil {
				return pageErr
			}
			// 并发度固定为一，避免智能生成同时放大目标实例与表单读取压力。
			for _, item := range page.Items {
				if len(result) >= limit || strings.TrimSpace(item.FlowCode) != flowCode {
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
			if !page.HasMore || len(page.Items) == 0 {
				break
			}
			pageNumber++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.sampleMu.Lock()
	s.sampleCache[cacheKey] = recentFormSampleCache{expiresAt: time.Now().Add(30 * time.Second), values: cloneRecentSamples(result)}
	s.sampleMu.Unlock()
	return cloneRecentSamples(result), nil
}

// cloneRecentSamples 深复制近期样本，生成器修改 values 时不会污染跨请求缓存。
func cloneRecentSamples(values []map[string]any) []map[string]any {
	data, _ := json.Marshal(values)
	result := make([]map[string]any, 0)
	_ = json.Unmarshal(data, &result)
	return result
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
