package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/logging"
	"test-auto-pro-v2/internal/repository"
	"test-auto-pro-v2/internal/session"
)

var (
	ErrTargetFlowNotFound        = errors.New("目标流程当前不可读取")
	ErrTargetFlowStructureEmpty  = errors.New("目标流程暂未配置节点")
	ErrTargetFlowNotConfigurable = errors.New("目标流程当前不能配置执行路径")
)

type TargetReadService struct {
	configMissing []string
	client        *target.Client
	sessions      *session.Manager
	candidates    repository.TargetHistoryCandidateStore
	snapshotMu    sync.Mutex
	snapshotCache map[string]cachedPathConfigurationSnapshot
}

// cachedPathConfigurationSnapshot 是一次流程读取结果的短期缓存。
// 一次页面加载会连续请求流程图、路径配置、编译场景和数据工作区，每个都要完整读一遍目标流程；
// 同一账号的读取在会话层串行，叠加起来足以让后面的请求撞上目标网关超时。
// 这里只缓存"投影用"的流程结构，有效期很短；真正的动作执行仍然必须按 F-012 规则重读事实。
type cachedPathConfigurationSnapshot struct {
	snapshot target.PathConfigurationSnapshot
	expires  time.Time
}

const pathConfigurationSnapshotTTL = 15 * time.Second

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
		client:   client,
		sessions: session.NewManager(client, cfg.SessionTTL),
	}
}

// NewTargetReadServiceWithClient 为假目标集成测试注入客户端和会话有效期。
func NewTargetReadServiceWithClient(client *target.Client, ttl time.Duration) *TargetReadService {
	return &TargetReadService{
		client: client, sessions: session.NewManager(client, ttl),
	}
}

// SetHistoryCandidateStore 注入目标业务库只读候选来源；未注入时候选回落到目标只读 API。
func (s *TargetReadService) SetHistoryCandidateStore(store repository.TargetHistoryCandidateStore) {
	s.candidates = store
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
	cacheKey := strings.Join([]string{strings.TrimSpace(account), strings.TrimSpace(source), strings.TrimSpace(targetObjectID)}, "\x00")
	if cached, ok := s.cachedSnapshot(cacheKey); ok {
		return cached, nil
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
	if err != nil {
		return target.PathConfigurationSnapshot{}, err
	}
	s.storeSnapshot(cacheKey, result)
	return result, nil
}

// cachedSnapshot 返回仍在有效期内的流程结构缓存。
func (s *TargetReadService) cachedSnapshot(key string) (target.PathConfigurationSnapshot, bool) {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	cached, exists := s.snapshotCache[key]
	if !exists || time.Now().After(cached.expires) {
		return target.PathConfigurationSnapshot{}, false
	}
	return cached.snapshot, true
}

// storeSnapshot 写入短期流程结构缓存，并顺带清掉已过期条目，避免长时间运行后无界增长。
func (s *TargetReadService) storeSnapshot(key string, snapshot target.PathConfigurationSnapshot) {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	if s.snapshotCache == nil {
		s.snapshotCache = make(map[string]cachedPathConfigurationSnapshot, 4)
	}
	now := time.Now()
	for existing, cached := range s.snapshotCache {
		if now.After(cached.expires) {
			delete(s.snapshotCache, existing)
		}
	}
	s.snapshotCache[key] = cachedPathConfigurationSnapshot{snapshot: snapshot, expires: now.Add(pathConfigurationSnapshotTTL)}
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

// SetNetworkLogger 在唯一目标出口接入请求日志；未注入时目标客户端行为完全不变。
func (s *TargetReadService) SetNetworkLogger(logger *logging.Logger) {
	if s == nil || s.client == nil || logger == nil {
		return
	}
	s.client.SetNetworkLogger(logger)
}

// ReadProxyConfigurationForInstance 按历史实例绑定的流程/表单代理读取该实例当时的宿主配置
//（流程树、表单模板原文与实例当前数据），供数据工作区按实例版本回显，避免当前已发布模板
// 与历史数据键不一致导致整表对不上。只读取，不写目标。
func (s *TargetReadService) ReadProxyConfigurationForInstance(ctx context.Context, account, flowProxyID string, formProxyIDs []string, instanceID string) (target.PathConfigurationSnapshot, error) {
	if err := s.ready(); err != nil {
		return target.PathConfigurationSnapshot{}, err
	}
	var result target.PathConfigurationSnapshot
	err := s.sessions.DoRead(ctx, account, func(callContext context.Context, active target.Session) error {
		snapshot, err := s.client.ReadProxyConfiguration(callContext, active, flowProxyID, formProxyIDs, instanceID)
		if err != nil {
			return err
		}
		result = snapshot
		return nil
	})
	return result, err
}

// ResolveHistoryInstanceForSnapshot 按候选键重新解析历史实例身份，供旧快照补齐实例绑定版本
// 重读所需的实例/流程代理/表单代理标识；匹配规则与快照采集完全一致。
func (s *TargetReadService) ResolveHistoryInstanceForSnapshot(ctx context.Context, account, flowCode, formName, flowName, candidateKey string) (target.HistoryInstance, error) {
	if err := s.ready(); err != nil {
		return target.HistoryInstance{}, err
	}
	var result target.HistoryInstance
	err := s.sessions.DoRead(ctx, account, func(callContext context.Context, active target.Session) error {
		selected, err := s.findHistoryInstanceByKey(callContext, active, account, flowCode, formName, flowName, candidateKey)
		if err != nil {
			return err
		}
		result = *selected
		return nil
	})
	return result, err
}
