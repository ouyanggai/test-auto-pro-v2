package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// 运行准备与成功断言的稳定错误分类，供 API 层映射状态码与中文文案。
const (
	RunReadinessErrorNotFound = "not_found"
	RunReadinessErrorInvalid  = "invalid"
	RunReadinessErrorConflict = "conflict"
	RunReadinessErrorStorage  = "storage"
	RunReadinessErrorTarget   = "target"
)

// RunReadinessError 是本切片对外的稳定错误，Message 直接作为界面提示，与日志同源。
type RunReadinessError struct {
	Kind    string
	Message string
}

// Error 返回中文提示本身，便于日志与响应体保持同一句话。
func (e *RunReadinessError) Error() string { return e.Message }

// IsRunReadinessErrorKind 判断错误是否属于指定分类。
func IsRunReadinessErrorKind(err error, kind string) bool {
	var typed *RunReadinessError
	return errors.As(err, &typed) && typed.Kind == kind
}

// runReadinessGraphReader 是读取当前真实流程结构的最小读取面。
type runReadinessGraphReader interface {
	Get(context.Context, uint64) (model.FlowGraph, error)
}

// runReadinessPathAnalyzer 是把路径选择投影为真实可达线路的最小分析面。
type runReadinessPathAnalyzer interface {
	Analyze(model.FlowGraph, []model.ExecutionPathChoice) (model.ExecutionPathAnalysis, error)
}

// RunReadinessService 组装成功断言工作区与计划运行准备结论。
// 它只读目标与数据库既有事实，不发写请求，也不自动修正任何已保存配置。
type RunReadinessService struct {
	plans    *PlanService
	paths    repository.ExecutionPathRepository
	graphs   runReadinessGraphReader
	configs  repository.HistoryPathConfigStore
	analyzer runReadinessPathAnalyzer
	now      func() time.Time
}

// NewRunReadinessService 组装计划、路径、真实结构、断言与路径配置的只读边界。
func NewRunReadinessService(plans *PlanService, paths repository.ExecutionPathRepository, graphs runReadinessGraphReader,
	configs repository.HistoryPathConfigStore,
	pathAnalyzer runReadinessPathAnalyzer, now func() time.Time) *RunReadinessService {
	if now == nil {
		now = time.Now
	}
	return &RunReadinessService{plans: plans, paths: paths, graphs: graphs,
		configs: configs, analyzer: pathAnalyzer, now: now}
}

// PlanReadiness 聚合一个计划下每条路径的运行准备结论。
// 真实流程结构只读一次；断言一次取齐；路径配置按路径逐条读数据库，不读目标。
// selectedPathIDs 为空表示检查该计划下全部路径；非空时只检查勾选的那些路径。
// 运行只运行勾选路径，预检也必须只检查勾选路径，否则用户看到的阻塞与本次要跑的东西不是一回事。
func (s *RunReadinessService) PlanReadiness(ctx context.Context, planID uint64, selectedPathIDs []uint64) (model.PlanRunReadiness, error) {
	if _, err := s.plans.Get(ctx, planID); err != nil {
		return model.PlanRunReadiness{}, notFoundError("计划不存在")
	}
	summaries, err := s.paths.List(ctx, planID)
	if err != nil {
		return model.PlanRunReadiness{}, storageError("暂时无法读取执行路径，请重试")
	}
	if len(summaries) == 0 {
		return AggregatePlanReadiness(nil), nil
	}
	if len(selectedPathIDs) > 0 {
		wanted := make(map[uint64]bool, len(selectedPathIDs))
		for _, pathID := range selectedPathIDs {
			wanted[pathID] = true
		}
		filtered := make([]model.ExecutionPath, 0, len(selectedPathIDs))
		for _, path := range summaries {
			if wanted[path.ID] {
				filtered = append(filtered, path)
			}
		}
		if len(filtered) == 0 {
			return model.PlanRunReadiness{}, &RunReadinessError{
				Kind: RunReadinessErrorInvalid, Message: "勾选的执行路径不属于这个计划，请重新选择"}
		}
		summaries = filtered
	}
	pathIDs := make([]uint64, 0, len(summaries))
	for _, path := range summaries {
		pathIDs = append(pathIDs, path.ID)
	}
	// 摘要查询带节点配置与数据状态但不带分支选择，批量详情查询带选择但不带状态列，
	// 两边必须按路径 ID 合并；只用其中一边会让状态或拓扑判断整体失真。
	details, err := s.paths.GetMany(ctx, planID, pathIDs)
	if err != nil {
		return model.PlanRunReadiness{}, storageError("暂时无法读取执行路径详情，请重试")
	}
	choicesByPath := make(map[uint64][]model.ExecutionPathChoice, len(details))
	for _, detail := range details {
		choicesByPath[detail.ID] = detail.Choices
	}
	paths := make([]model.ExecutionPath, 0, len(summaries))
	for _, summary := range summaries {
		summary.Choices = choicesByPath[summary.ID]
		paths = append(paths, summary)
	}
	// 真实结构读失败不掩盖：整份结论直接给目标错误，不允许悄悄退化成"没有拓扑问题"。
	graph, err := s.graphs.Get(ctx, planID)
	if err != nil {
		return model.PlanRunReadiness{}, &RunReadinessError{
			Kind: RunReadinessErrorTarget, Message: "暂时无法读取目标流程结构，运行准备结论不完整，请重试"}
	}
	results := make([]model.PathRunReadiness, 0, len(paths))
	for _, path := range paths {
		results = append(results, EvaluatePathReadiness(s.pathInput(ctx, graph, path)))
	}
	return AggregatePlanReadiness(results), nil
}

// pathInput 把一条路径的既有事实读齐后交给纯聚合函数。
func (s *RunReadinessService) pathInput(ctx context.Context, graph model.FlowGraph, path model.ExecutionPath) PathReadinessInput {
	input := PathReadinessInput{Path: path}
	input.TopologyIssues = s.topologyIssues(graph, path)
	config, found, err := s.configs.GetPathConfig(ctx, path.ID)
	if err != nil || !found {
		// 读不到配置就按"没有配置记录"处理：节点配置与数据两项本身已由路径状态阻塞。
		return input
	}
	input.ConfigFound = true
	input.ConfigIssues, input.ConfigNotices = decodeConfigIssues(config.Issues, graphNodeNames(graph))
	input.CompiledStepCount = countJSONArray(config.CompiledSteps)
	input.ConfiguredActions = decodeConfiguredActions(config.UserActions)
	return input
}

// topologyIssues 复验路径与当前真实流程结构是否仍然一致；分析失败或选择不完整都算不一致。
func (s *RunReadinessService) topologyIssues(graph model.FlowGraph, path model.ExecutionPath) []model.PathConfigAffectedItem {
	analysis, err := s.analyzer.Analyze(graph, path.Choices)
	if err != nil {
		return []model.PathConfigAffectedItem{{
			Kind: "topology", Name: pathDisplayName(path),
			Reason: "这条路径的分支选择在当前真实流程结构里已不成立，请重新确认路径",
		}}
	}
	if !analysis.Complete {
		return []model.PathConfigAffectedItem{{
			Kind: "topology", Name: pathDisplayName(path),
			Reason: "当前真实流程结构里还有路由节点没有对应的分支选择，请重新确认路径",
		}}
	}
	return nil
}

// pathDisplayName 给出路径的界面名称，缺名时用序号兜底。
func pathDisplayName(path model.ExecutionPath) string {
	if name := strings.TrimSpace(path.Name); name != "" {
		return name
	}
	return "路径 " + uintDecimal(uint64(path.SequenceNo))
}

// decodeConfigIssues 解析路径配置里已记录的问题。
// 该列是异构对象数组：F-012 的路径问题与动作问题字段名不同（reason 与 message、name 与 action），
// 因此按几种已知键取值，并且**只保留有可读中文原因的条目**——
// 没有原因的条目在界面上就是一条空白阻塞，比不显示更糟。解析不了就当没有，不编造问题。
func decodeConfigIssues(payload []byte, nodeNames map[string]string) (blocking, notices []model.PathConfigAffectedItem) {
	if len(payload) == 0 {
		return nil, nil
	}
	var objects []map[string]any
	if err := json.Unmarshal(payload, &objects); err != nil {
		return nil, nil
	}
	blocking = make([]model.PathConfigAffectedItem, 0, len(objects))
	notices = make([]model.PathConfigAffectedItem, 0, len(objects))
	for _, object := range objects {
		reason := firstStringValue(object, "reason", "message", "detail")
		if reason == "" {
			continue
		}
		name := firstStringValue(object, "name", "nodeName")
		if name == "" {
			// 问题里带的是目标结构节点键，界面不显示这类标识：能查到中文节点名就用它，
			// 查不到就留空，由中文原因承载信息。产品规则要求界面只出现业务语言。
			name = nodeNames[firstStringValue(object, "path", "nodeKey")]
		}
		item := model.PathConfigAffectedItem{
			Kind:   firstStringValue(object, "kind", "code"),
			Name:   name,
			Reason: reason,
		}
		// F-012 把说明性提示和真正的阻塞写在同一列，只有 blocking=true 才是用户必须先处理的事。
		// 把说明当阻塞显示，用户看不懂也无从下手，这是人工验收明确指出的问题。
		if isBlockingIssue(object) {
			blocking = append(blocking, item)
			continue
		}
		notices = append(notices, item)
	}
	return blocking, notices
}

// isBlockingIssue 读取问题的 blocking 标记；没有该字段的条目按阻塞处理，宁严不宽。
func isBlockingIssue(object map[string]any) bool {
	value, present := object["blocking"]
	if !present {
		return true
	}
	flag, ok := value.(bool)
	return !ok || flag
}

// graphNodeNames 把真实结构里的节点键映射为中文节点名，供问题条目显示业务语言。
func graphNodeNames(graph model.FlowGraph) map[string]string {
	names := make(map[string]string, len(graph.Nodes))
	for _, node := range graph.Nodes {
		if name := strings.TrimSpace(node.Name); name != "" {
			names[node.ID] = name
		}
	}
	return names
}

// firstStringValue 按给定键顺序取第一个非空字符串值。
func firstStringValue(object map[string]any, keys ...string) string {
	for _, key := range keys {
		if text, ok := object[key].(string); ok {
			if trimmed := strings.TrimSpace(text); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

// decodeConfiguredActions 取出这条路径已配置的动作标识，供与已验证子集比对。
func decodeConfiguredActions(payload []byte) []model.ActionKey {
	if len(payload) == 0 {
		return nil
	}
	var actions []model.ConfiguredAction
	if err := json.Unmarshal(payload, &actions); err != nil {
		return nil
	}
	keys := make([]model.ActionKey, 0, len(actions))
	for _, action := range actions {
		keys = append(keys, action.Action)
	}
	return keys
}

// countJSONArray 统计 JSON 数组的元素个数，只用来判断编译场景是否为空。
func countJSONArray(payload []byte) int {
	if len(payload) == 0 {
		return 0
	}
	var items []json.RawMessage
	if err := json.Unmarshal(payload, &items); err != nil {
		return 0
	}
	return len(items)
}

// uintDecimal 是不引入额外依赖的最小无符号十进制转换。
func uintDecimal(value uint64) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 20)
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}

// notFoundError 与 storageError 统一构造稳定错误，避免各处自造文案。
func notFoundError(message string) error {
	return &RunReadinessError{Kind: RunReadinessErrorNotFound, Message: message}
}

func storageError(message string) error {
	return &RunReadinessError{Kind: RunReadinessErrorStorage, Message: message}
}
