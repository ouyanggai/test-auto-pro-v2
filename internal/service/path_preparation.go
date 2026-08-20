package service

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"strings"
	"sync"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/formdata"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

const pathPreparationBatchSize = 25

// PathPreparationErrorKind 是批量准备 API 的稳定错误分类。
type PathPreparationErrorKind string

const (
	PathPreparationErrorInvalid  PathPreparationErrorKind = "invalid"
	PathPreparationErrorNotFound PathPreparationErrorKind = "not_found"
	PathPreparationErrorState    PathPreparationErrorKind = "state"
	PathPreparationErrorStorage  PathPreparationErrorKind = "storage"
)

// PathPreparationError 是批量准备任务的用户可读业务错误。
type PathPreparationError struct {
	Kind    PathPreparationErrorKind
	Message string
}

// Error 返回批量准备业务错误说明。
func (e *PathPreparationError) Error() string { return e.Message }

// IsPathPreparationErrorKind 判断错误是否属于指定批量准备分类。
func IsPathPreparationErrorKind(err error, kind PathPreparationErrorKind) bool {
	var target *PathPreparationError
	return errors.As(err, &target) && target.Kind == kind
}

// PathPreparationService 组织持久任务、单次目标快照和有界路径批处理。
type PathPreparationService struct {
	config     *PathConfigService
	repository repository.PathPreparationRepository
	now        func() time.Time
	semaphore  chan struct{}
	mu         sync.Mutex
	cancel     map[string]context.CancelFunc
}

// NewPathPreparationService 创建独立批量路径准备服务。
func NewPathPreparationService(config *PathConfigService, repository repository.PathPreparationRepository) *PathPreparationService {
	return &PathPreparationService{config: config, repository: repository, now: time.Now, semaphore: make(chan struct{}, 2), cancel: map[string]context.CancelFunc{}}
}

// Create 按幂等键建立当前勾选路径任务，并异步执行同一持久检查点。
func (s *PathPreparationService) Create(ctx context.Context, planID uint64, createKey string) (model.PathPreparationJob, error) {
	createKey = strings.TrimSpace(createKey)
	if planID == 0 || !validUUID(createKey) {
		return model.PathPreparationJob{}, &PathPreparationError{Kind: PathPreparationErrorInvalid, Message: "批量准备请求标识不正确"}
	}
	if err := s.config.validateConfigMutablePlan(ctx, planID); err != nil {
		return model.PathPreparationJob{}, err
	}
	job, _, err := s.repository.Create(ctx, planID, createKey, s.now().UTC())
	if err != nil {
		return model.PathPreparationJob{}, mapPathPreparationRepositoryError(err)
	}
	if job.Status == "queued" || job.Status == "running" {
		s.launch(job)
	}
	return job, nil
}

// Get 返回任务真实计数。
func (s *PathPreparationService) Get(ctx context.Context, planID uint64, jobID string) (model.PathPreparationJob, error) {
	job, err := s.repository.Get(ctx, planID, strings.TrimSpace(jobID))
	if err != nil {
		return model.PathPreparationJob{}, mapPathPreparationRepositoryError(err)
	}
	return job, nil
}

// Active 返回刷新页面后仍应展示的同计划活动任务。
func (s *PathPreparationService) Active(ctx context.Context, planID uint64) (model.PathPreparationJob, bool, error) {
	job, found, err := s.repository.FindActive(ctx, planID)
	if err != nil {
		return model.PathPreparationJob{}, false, mapPathPreparationRepositoryError(err)
	}
	if found {
		s.launch(job)
	}
	return job, found, nil
}

// Cancel 取消活动 Worker，并把未完成明细留在可恢复检查点。
func (s *PathPreparationService) Cancel(ctx context.Context, planID uint64, jobID string) (model.PathPreparationJob, error) {
	s.mu.Lock()
	if cancel := s.cancel[jobID]; cancel != nil {
		cancel()
	}
	s.mu.Unlock()
	job, err := s.repository.Cancel(ctx, planID, jobID, s.now().UTC())
	if err != nil {
		return model.PathPreparationJob{}, mapPathPreparationRepositoryError(err)
	}
	return job, nil
}

// Resume 把已取消或失败任务恢复到原检查点继续处理。
func (s *PathPreparationService) Resume(ctx context.Context, planID uint64, jobID string) (model.PathPreparationJob, error) {
	if err := s.config.validateConfigMutablePlan(ctx, planID); err != nil {
		return model.PathPreparationJob{}, err
	}
	job, err := s.repository.Resume(ctx, planID, jobID, s.now().UTC())
	if err != nil {
		return model.PathPreparationJob{}, mapPathPreparationRepositoryError(err)
	}
	s.launch(job)
	return job, nil
}

// ListItems 返回任务明细的游标分页。
func (s *PathPreparationService) ListItems(ctx context.Context, planID uint64, jobID string, cursor uint64, limit int) (model.PathPreparationItemPage, error) {
	page, err := s.repository.ListItems(ctx, planID, jobID, cursor, limit)
	if err != nil {
		return model.PathPreparationItemPage{}, mapPathPreparationRepositoryError(err)
	}
	if page.Items == nil {
		page.Items = []model.PathPreparationItem{}
	}
	return page, nil
}

// Recover 恢复服务重启前仍处于排队或运行状态的持久任务。
func (s *PathPreparationService) Recover(ctx context.Context) error {
	jobs, err := s.repository.ListRecoverable(ctx)
	if err != nil {
		return mapPathPreparationRepositoryError(err)
	}
	for _, job := range jobs {
		s.launch(job)
	}
	return nil
}

// launch 保证同一进程内每个任务只有一个 Worker，并限制全局并发。
func (s *PathPreparationService) launch(job model.PathPreparationJob) {
	s.mu.Lock()
	if _, running := s.cancel[job.ID]; running {
		s.mu.Unlock()
		return
	}
	workerCtx, cancel := context.WithCancel(context.Background())
	s.cancel[job.ID] = cancel
	s.mu.Unlock()
	go func() {
		s.semaphore <- struct{}{}
		defer func() {
			<-s.semaphore
			s.mu.Lock()
			delete(s.cancel, job.ID)
			s.mu.Unlock()
		}()
		s.run(workerCtx, job)
	}()
}

// run 在单次快照和共享样本上下文下分批处理任务，单条失败不终止其他路径。
func (s *PathPreparationService) run(ctx context.Context, job model.PathPreparationJob) {
	if err := s.repository.Start(ctx, job.PlanID, job.ID, s.now().UTC()); err != nil {
		return
	}
	assets, err := s.loadPathPreparationAssets(ctx, job.PlanID)
	if err != nil {
		_ = s.repository.Fail(context.Background(), job.PlanID, job.ID, "读取当前流程准备信息失败，请恢复后重试", s.now().UTC())
		return
	}
	coverage := map[string]int{}
	for {
		if ctx.Err() != nil {
			return
		}
		items, claimErr := s.repository.ClaimBatch(ctx, job.PlanID, job.ID, pathPreparationBatchSize, s.now().UTC())
		if claimErr != nil {
			_ = s.repository.Fail(context.Background(), job.PlanID, job.ID, "读取任务检查点失败，请恢复后重试", s.now().UTC())
			return
		}
		if len(items) == 0 {
			_, _ = s.repository.Finish(context.Background(), job.PlanID, job.ID, s.now().UTC())
			return
		}
		pathIDs := make([]uint64, 0, len(items))
		for _, item := range items {
			pathIDs = append(pathIDs, item.PathID)
		}
		paths, pathErr := s.config.pathRepository.GetMany(ctx, job.PlanID, pathIDs)
		configs, configErr := s.config.configRepository.FindByPaths(ctx, pathIDs)
		if pathErr != nil || configErr != nil {
			for _, item := range items {
				if currentErr := s.repository.SetCurrent(ctx, job.PlanID, job.ID, item, s.now().UTC()); currentErr != nil {
					return
				}
				_ = s.repository.CompleteItem(context.Background(), job.PlanID, job.ID, item.ID, model.PathPreparationItemResult{Status: "failed", Reason: "路径本地配置暂时无法读取"}, s.now().UTC())
			}
			continue
		}
		pathsByID := make(map[uint64]model.ExecutionPath, len(paths))
		for _, path := range paths {
			pathsByID[path.ID] = path
		}
		for _, item := range items {
			if ctx.Err() != nil {
				return
			}
			if err := s.repository.SetCurrent(ctx, job.PlanID, job.ID, item, s.now().UTC()); err != nil {
				return
			}
			path, exists := pathsByID[item.PathID]
			outcome := model.PathPreparationItemResult{Status: "needs_attention", Reason: "当前路径已不存在，请重新勾选", NeedsAttention: true}
			if exists {
				outcome = s.preparePath(ctx, assets, path, configs[item.PathID], coverage)
			}
			if err := s.repository.CompleteItem(ctx, job.PlanID, job.ID, item.ID, outcome, s.now().UTC()); err != nil {
				return
			}
		}
	}
}

type pathPreparationAssets struct {
	plan                model.Plan
	snapshot            target.PathConfigurationSnapshot
	graph               model.FlowGraph
	template            map[string]any
	unsupported         []string
	samples             []map[string]any
	initiator           string
	identity            formdata.IdentityContext
	componentCandidates map[string][]any
}

// loadPathPreparationAssets 一次读取计划、真实流程快照、模板、身份和有限近期样本。
func (s *PathPreparationService) loadPathPreparationAssets(ctx context.Context, planID uint64) (pathPreparationAssets, error) {
	plan, err := s.config.plans.Get(ctx, planID)
	if err != nil {
		return pathPreparationAssets{}, err
	}
	// 批量准备必须与单条页面使用同一份已持久化规则目录，不能绕过目录重新扫描宿主源码或逐路径读取目标表单。
	snapshot, err := s.config.readVerifiedSnapshot(ctx, planID)
	if err != nil {
		return pathPreparationAssets{}, err
	}
	nodes, edges, warnings, err := s.config.flowAnalyzer.Analyze(snapshot.Tree)
	if err != nil {
		return pathPreparationAssets{}, err
	}
	entries, err := validateEntryNodeIDs(snapshot.EntryNodeIDs, nodes)
	if err != nil {
		return pathPreparationAssets{}, err
	}
	template, unsupported := runtimeTemplate(snapshot.Forms)
	if snapshot.RenderType == target.FormRenderTypeVueCustom {
		if snapshot.VuePage == nil || len(snapshot.VuePage.Issues) > 0 {
			return pathPreparationAssets{}, &PathPreparationError{Kind: PathPreparationErrorState, Message: "Vue 业务页面规则尚未完成分析，请先在系统设置重试分析"}
		}
		template = vueCustomTemplate(snapshot.VuePage)
	}
	assets := pathPreparationAssets{
		plan: plan, snapshot: snapshot, template: template, unsupported: unsupported,
		graph:               model.FlowGraph{PlanID: plan.ID, TargetName: plan.TargetObjectName, FlowSource: plan.FlowSource, EntryNodeIDs: entries, Nodes: nodes, Edges: edges, Warnings: warnings},
		initiator:           plan.Account,
		samples:             []map[string]any{},
		componentCandidates: make(map[string][]any),
	}
	if reader, ok := s.config.target.(pathFormRuleSampleReader); ok {
		componentID := "formmaking"
		if snapshot.VuePage != nil {
			componentID = strings.TrimSpace(snapshot.VuePage.ComponentName)
		}
		if samples, readErr := reader.RecentFormSamplesForRule(ctx, plan.Account, snapshot.FlowCode, snapshot.TemplateID, componentID, snapshot.RuleVersion, 5); readErr == nil {
			assets.samples = samples
		}
	} else if reader, ok := s.config.target.(pathFormSampleReader); ok {
		if samples, readErr := reader.RecentFormSamples(ctx, plan.Account, snapshot.FlowCode, 5); readErr == nil {
			assets.samples = samples
		}
	}
	if reader, ok := s.config.target.(pathFormIdentityReader); ok {
		// 批量任务只读取一次可信身份目录，避免运行时会话和身份投影重复访问目标平台。
		if identity, readErr := reader.FormIdentityContext(ctx, plan.Account); readErr == nil {
			assets.identity = formdataIdentityContext(identity)
			if strings.TrimSpace(identity.User.Name) != "" {
				assets.initiator = identity.User.Name
			}
		}
	}
	// 预加载组件候选池，供批量任务共享
	if s.config.candidateCache != nil {
		if candidateSet, err := s.config.candidateCache.GetCandidateSet(ctx, plan.Account, snapshot.FlowCode, snapshot.RuleVersion); err == nil {
			assets.componentCandidates = buildComponentCandidatesMap(template, candidateSet)
		}
	}
	return assets, nil
}

// preparePath 为一条路径保留人工事实、补齐安全节点默认值并生成或复验表单数据。
func (s *PathPreparationService) preparePath(ctx context.Context, assets pathPreparationAssets, path model.ExecutionPath, stored model.StoredPathConfig, coverage map[string]int) model.PathPreparationItemResult {
	snapshot := assets.snapshot
	analysis, err := s.config.pathAnalyzer.Analyze(assets.graph, path.Choices)
	if err != nil || !analysis.Complete {
		return model.PathPreparationItemResult{Status: "needs_attention", Reason: "当前路径与真实流程不一致，请先编辑路径", NeedsAttention: true}
	}
	found := stored.PathID != 0
	if !found {
		stored = model.StoredPathConfig{
			PathID: path.ID, FieldValues: map[string]map[string]string{}, ActionValues: map[string]string{},
			FormStatus: initialStoredFormStatus(snapshot), DataStatus: initialStoredDataStatus(snapshot),
		}
	}
	configuration, validation, err := s.config.configAnalyzer.Analyze(assets.graph, snapshot.Tree, snapshot.FormFields, path, analysis, snapshot.InstanceValues, stored.FieldValues, stored.ActionValues, found)
	if err != nil {
		return model.PathPreparationItemResult{Status: "needs_attention", Reason: "当前节点配置无法安全投影", NeedsAttention: true}
	}
	applyConfirmedNodeState(&configuration, stored.ConfirmedNodeKeys)
	nodeChanged, nodeNeedsAttention := preparePathNodes(&stored, configuration, validation, coverage)
	applyConfirmedNodeState(&configuration, stored.ConfirmedNodeKeys)
	stored.Status = derivePathConfigurationStatus(configuration)
	dataChanged, dataGenerated, preservedManual, dataReason := preparePathFormData(assets, path, analysis, &stored)
	if nodeChanged || dataChanged || !found {
		expected := stored.Revision
		stored.Revision++
		if nodeChanged {
			stored.NodeRevision++
		}
		if dataChanged {
			stored.FormRevision++
		}
		stored.ConfigVersion = currentPathConfigVersion
		stored.IdempotencyKey = ""
		if _, saveErr := s.config.configRepository.Save(ctx, stored, expected, s.now().UTC()); saveErr != nil {
			return model.PathPreparationItemResult{Status: "failed", Reason: "保存路径准备结果失败，请稍后恢复任务"}
		}
	}
	nodeConfigured := stored.Status == "configured"
	needsAttention := nodeNeedsAttention || stored.DataStatus == "needs_attention"
	reasons := make([]string, 0, 2)
	if nodeNeedsAttention {
		reasons = append(reasons, "部分节点需要人工配置")
	}
	if dataReason != "" {
		reasons = append(reasons, dataReason)
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "节点与表单数据已完成安全准备")
	}
	status := "completed"
	if needsAttention {
		status = "needs_attention"
	}
	return model.PathPreparationItemResult{
		Status: status, Reason: strings.Join(reasons, "；"), NodeConfigured: nodeConfigured,
		DataGenerated: dataGenerated, NeedsAttention: needsAttention, PreservedManual: preservedManual,
	}
}

// preparePathNodes 只补未人工确认节点，保存过的人员、动作和循环全部保持原样。
func preparePathNodes(stored *model.StoredPathConfig, configuration model.PathConfiguration, validation analyzer.PathConfigValidation, coverage map[string]int) (bool, bool) {
	confirmed := make(map[string]bool, len(stored.ConfirmedNodeKeys))
	for _, key := range stored.ConfirmedNodeKeys {
		confirmed[key] = true
	}
	changed, needsAttention := false, false
	for _, group := range configuration.Groups {
		for _, node := range group.Nodes {
			if confirmed[node.Key] || node.Status == "not_required" || node.Status == "runtime" {
				continue
			}
			targetNode, exists := validation.NodeTokens[node.Key]
			if !exists || len(targetNode.Blockers) > 0 {
				needsAttention = true
				continue
			}
			input, ok := defaultPathNodeInput(node, coverage)
			if !ok {
				needsAttention = true
				continue
			}
			encoded, err := validatePathConfigNodeSubmission(targetNode, input)
			if err != nil {
				needsAttention = true
				continue
			}
			for key, value := range encoded {
				stored.ActionValues[key] = value
			}
			stored.ConfirmedNodeKeys = appendUnique(stored.ConfirmedNodeKeys, node.Key)
			changed = true
		}
	}
	return changed, needsAttention
}

// defaultPathNodeInput 为人员采用目标默认或稳定随机，并尽量覆盖可用额外动作。
func defaultPathNodeInput(node model.PathConfigNode, coverage map[string]int) (model.PathNodeSaveInput, bool) {
	input := model.PathNodeSaveInput{Persons: []model.PathConfigPersonStrategyInput{}, Actions: []model.PathConfigConfiguredActionInput{}}
	for _, person := range node.Persons {
		if !person.Editable {
			continue
		}
		selected := model.PathConfigPersonStrategyInput{Key: person.Key, Seed: person.StrategySeed}
		for _, strategy := range person.Strategies {
			if strategy.Value == "target_default" && len(person.DefaultSelected) > 0 {
				selected.Strategy, selected.Selected = "target_default", append([]string(nil), person.DefaultSelected...)
				break
			}
			if selected.Strategy == "" && strategy.Value == "random" {
				selected.Strategy = "random"
			}
		}
		if selected.Strategy == "" {
			return model.PathNodeSaveInput{}, false
		}
		input.Persons = append(input.Persons, selected)
	}
	catalog := pathPreparationActionCatalog(node.ActionConfiguration.Catalog)
	if len(catalog) == 0 {
		return input, true
	}
	action, ok := choosePathPreparationAction(catalog, coverage, node.Key)
	if !ok {
		return model.PathNodeSaveInput{}, false
	}
	input.Actions = append(input.Actions, model.PathConfigConfiguredActionInput{Key: "batch-" + node.Key, Kind: action.Kind, Count: 1})
	if action.RequiresPerson {
		if action.Person == nil {
			return model.PathNodeSaveInput{}, false
		}
		for _, strategy := range action.Person.Strategies {
			if strategy.Value == "random" {
				input.Actions[0].Person = &model.PathConfigPersonStrategyInput{Key: action.Person.Key, Strategy: "random", Seed: action.Person.StrategySeed}
				break
			}
		}
		if input.Actions[0].Person == nil {
			return model.PathNodeSaveInput{}, false
		}
	}
	coverage[action.Kind]++
	return input, true
}

// preparePathFormData 保留人工确认值；其余路径使用同一任务资产生成并完整复验。
func preparePathFormData(assets pathPreparationAssets, path model.ExecutionPath, analysis model.ExecutionPathAnalysis, stored *model.StoredPathConfig) (bool, bool, bool, string) {
	snapshot := assets.snapshot
	if snapshot.RenderType == target.FormRenderTypeUnknown {
		changed := stored.DataStatus != "needs_attention" || stored.FormStatus != "affected"
		stored.DataStatus, stored.FormStatus, stored.FormValidated = "needs_attention", "affected", false
		return changed, false, false, "当前流程页面规则尚未完成分析"
	}
	if snapshot.RenderType == target.FormRenderTypeFormMaking && len(snapshot.Forms) == 0 {
		changed := stored.DataStatus != "not_required"
		stored.DataStatus, stored.FormStatus = "not_required", "valid"
		return changed, false, false, "当前路径无需准备表单数据"
	}
	if stored.DataStatus == "confirmed" {
		reasons := validateTargetPathSelection(snapshot.Tree, path.Choices, stored.FormValues)
		if len(reasons) == 0 {
			return false, false, true, "已保留人工确认的表单数据"
		}
		stored.DataStatus = "needs_attention"
		return true, false, true, "人工表单数据与当前路径冲突，已保留原值"
	}
	conditions := buildPathFormConditionProjection(snapshot.Tree, path.Choices, assets.template, stored.FormValues)
	seed := int64(path.ID*1000003 + uint64(len(path.Choices))*97 + 1)
	permissions := formPermissions(snapshot.Tree, formPermissionNodeIDs(assets.plan.FlowSource, snapshot, analysis.ReachableNodeIDs))
	if snapshot.RenderType == target.FormRenderTypeVueCustom {
		permissions = vueCustomFormPermissions(snapshot.VuePage)
	}
	bindings := buildPathDateRangeBindings(snapshot.Tree, path.Choices, assets.template)
	generated := formdata.Generate(formdata.GenerateInput{
		Template: assets.template, Base: stored.FormValues, Samples: assets.samples, Seed: seed, Initiator: assets.initiator,
		Constraints: conditions.Constraints, DateRangeBindings: bindings, ManualOverridePaths: stored.ManualOverridePaths,
		ProtectedPaths: conditions.ProtectedPaths, EditablePaths: editableFormPathsFromPermissions(permissions), Identity: assets.identity,
		ComponentCandidates: assets.componentCandidates,
	})
	solved := solveTargetPathValues(snapshot.Tree, path.Choices, assets.template, generated.Values, seed)
	generated.Values = solved.values
	formdata.SynchronizeDateRangeBindings(generated.Values, bindings, stored.ManualOverridePaths)
	reasons := append(validateTargetPathSelection(snapshot.Tree, path.Choices, generated.Values), formdata.ValidateDateRangeBindings(generated.Values, bindings)...)
	// 已识别自定义组件没有真实候选时只将本路径标为需处理，不能虚构对象引用，也不能让其他路径回滚。
	blocking := len(conditions.Reviews) > 0 || len(assets.unsupported) > 0 || generated.Pending > 0 || len(solved.issues) > 0 || len(reasons) > 0
	stored.FormValues = generated.Values
	stored.FormSeed = seed
	stored.GeneratedFieldPaths = generated.GeneratedFieldPaths
	stored.SampleSummary = model.PathFormSampleSummary{Defaults: generated.Defaults, Recent: generated.Recent, Fallback: generated.Fallback, Identity: generated.Identity}
	stored.FormTemplateVersion = formdata.TemplateVersion(assets.template)
	if blocking {
		stored.DataStatus, stored.FormStatus, stored.FormValidated = "needs_attention", "draft", false
		return true, false, false, "表单条件需要人工处理"
	}
	stored.DataStatus, stored.FormStatus, stored.FormValidated = "generated", "valid", true
	return true, true, false, "表单数据已生成并命中当前路径"
}

// mapPathPreparationRepositoryError 把仓储错误收敛为稳定 API 语义。
func mapPathPreparationRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrPathPreparationNotFound):
		return &PathPreparationError{Kind: PathPreparationErrorNotFound, Message: "批量准备任务不存在"}
	case errors.Is(err, repository.ErrPathPreparationEmpty):
		return &PathPreparationError{Kind: PathPreparationErrorInvalid, Message: "请先勾选需要准备的路径"}
	case errors.Is(err, repository.ErrPathPreparationState):
		return &PathPreparationError{Kind: PathPreparationErrorState, Message: "当前任务状态不能执行此操作"}
	default:
		return &PathPreparationError{Kind: PathPreparationErrorStorage, Message: "批量准备存储暂不可用，请重试"}
	}
}

// pathPreparationActionCatalog 只保留后台任务可以安全自动选择的额外动作。
func pathPreparationActionCatalog(catalog []model.PathConfigActionCatalogItem) []model.PathConfigActionCatalogItem {
	result := make([]model.PathConfigActionCatalogItem, 0, len(catalog))
	for _, item := range catalog {
		if item.Enabled && (!item.RequiresPerson || item.Person != nil) {
			result = append(result, item)
		}
	}
	return result
}

// choosePathPreparationAction 在覆盖次数最低的动作中稳定抽取一个，保证同一任务结果可复现。
func choosePathPreparationAction(catalog []model.PathConfigActionCatalogItem, coverage map[string]int, seed string) (model.PathConfigActionCatalogItem, bool) {
	if len(catalog) == 0 {
		return model.PathConfigActionCatalogItem{}, false
	}
	minimum := -1
	for _, item := range catalog {
		count := coverage[item.Kind]
		if minimum < 0 || count < minimum {
			minimum = count
		}
	}
	candidates := make([]model.PathConfigActionCatalogItem, 0, len(catalog))
	for _, item := range catalog {
		if coverage[item.Kind] == minimum {
			candidates = append(candidates, item)
		}
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(seed)))
	index := binary.BigEndian.Uint64(sum[:8]) % uint64(len(candidates))
	return candidates[index], true
}
