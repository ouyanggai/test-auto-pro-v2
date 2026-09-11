package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
)

type FlowTreeReader interface {
	FlowTreeSnapshot(context.Context, string, string, string) (target.FlowTreeSnapshot, error)
}

type FlowAnalyzer interface {
	Analyze(*target.FlowNodeTemplate) ([]model.FlowGraphNode, []model.FlowGraphEdge, []string, error)
}

type FlowGraphService struct {
	plans    *PlanService
	target   FlowTreeReader
	analyzer FlowAnalyzer
	// graphCache 是按计划缓存的短时效结构投影：运行详情页按秒轮询，若每次都读目标，
	// 高频只读请求会与执行器的会话刷新互相触发重登（目标平台同账号会话互踢），
	// 反而破坏运行现场。结构以分钟级变化，短缓存不影响路径偏离等运行期硬门禁
	//（那些判定用执行器自身的实时读取，不走本缓存）。
	cacheMu    sync.Mutex
	graphCache map[uint64]cachedGraph
	cacheTTL   time.Duration
	now        func() time.Time
}

// cachedGraph 是一次结构投影的缓存行。
type cachedGraph struct {
	graph     model.FlowGraph
	expiresAt time.Time
}

// NewFlowGraphService 组装持久化计划身份、目标树读取和安全图分析。
func NewFlowGraphService(plans *PlanService, targetReader FlowTreeReader, flowAnalyzer FlowAnalyzer) *FlowGraphService {
	return &FlowGraphService{
		plans: plans, target: targetReader, analyzer: flowAnalyzer,
		graphCache: map[uint64]cachedGraph{}, cacheTTL: 15 * time.Second, now: time.Now,
	}
}

// Get 按计划保存的身份重新读取真实图，并附加本次运行态入口集合。
// 根因修复：目标平台的 flowTemplateApi/list 会间歇性返回 500，运行详情每秒轮询，
// 若读取失败就降级会让用户在目标抖动期间反复看到结构降级提示。结构在运行期间稳定，
// 且图投影仅用于画布展示（运行期硬门禁不走本接口），因此读取失败时回退到最近一次
// 成功的投影（即使已过期）；只有从未成功读取过才真正降级。
func (s *FlowGraphService) Get(ctx context.Context, planID uint64) (model.FlowGraph, error) {
	graph, err := s.readGraph(ctx, planID)
	if err == nil {
		s.storeGraph(planID, graph)
		return graph, nil
	}
	if stale, ok := s.lastGoodGraph(planID); ok {
		return stale, nil
	}
	return model.FlowGraph{}, err
}

// cachedGraph 返回未过期的缓存投影；缓存关闭读取失败时的自愈：过期即重读。
func (s *FlowGraphService) cachedGraph(planID uint64) (model.FlowGraph, bool) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	entry, ok := s.graphCache[planID]
	if !ok || s.now().After(entry.expiresAt) {
		return model.FlowGraph{}, false
	}
	return entry.graph, true
}

// lastGoodGraph 返回最近一次成功的投影（不论是否过期），供目标抖动时回退展示。
func (s *FlowGraphService) lastGoodGraph(planID uint64) (model.FlowGraph, bool) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	entry, ok := s.graphCache[planID]
	if !ok {
		return model.FlowGraph{}, false
	}
	return entry.graph, true
}

// storeGraph 记录一次成功读取的投影。
func (s *FlowGraphService) storeGraph(planID uint64, graph model.FlowGraph) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.graphCache[planID] = cachedGraph{graph: graph, expiresAt: s.now().Add(s.cacheTTL)}
}

// readGraph 真实读取目标结构并分析成图投影。
func (s *FlowGraphService) readGraph(ctx context.Context, planID uint64) (model.FlowGraph, error) {
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return model.FlowGraph{}, err
	}
	snapshot, err := s.target.FlowTreeSnapshot(ctx, plan.Account, plan.FlowSource, plan.TargetObjectID)
	if err != nil {
		return model.FlowGraph{}, err
	}
	nodes, edges, warnings, err := s.analyzer.Analyze(snapshot.Tree)
	if err != nil {
		if errors.Is(err, analyzer.ErrFlowStructureInvalid) {
			return model.FlowGraph{}, analyzer.ErrFlowStructureInvalid
		}
		return model.FlowGraph{}, err
	}
	entries, err := validateEntryNodeIDs(snapshot.EntryNodeIDs, nodes)
	if err != nil {
		return model.FlowGraph{}, err
	}
	return model.FlowGraph{
		PlanID: plan.ID, TargetName: plan.TargetObjectName, FlowSource: plan.FlowSource,
		EntryNodeIDs: entries, Nodes: nodes, Edges: edges, Warnings: warnings,
	}, nil
}

// validateEntryNodeIDs 去重入口并确认每个入口都属于本次分析出的真实代理树。
func validateEntryNodeIDs(entryNodeIDs []string, nodes []model.FlowGraphNode) ([]string, error) {
	known := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		known[node.ID] = struct{}{}
	}
	entries := make([]string, 0, len(entryNodeIDs))
	seen := make(map[string]struct{}, len(entryNodeIDs))
	for _, rawID := range entryNodeIDs {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if _, exists := known[id]; !exists {
			return nil, ErrTargetFlowNotConfigurable
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		entries = append(entries, id)
	}
	if len(entries) == 0 {
		return nil, ErrTargetFlowNotConfigurable
	}
	return entries, nil
}
