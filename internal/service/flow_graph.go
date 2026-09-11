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

// backgroundRefreshTimeout 限制后台刷新单次目标读的时长；超时保留旧投影，不影响任何请求。
const backgroundRefreshTimeout = 30 * time.Second

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
	// refreshing 标记后台刷新中的计划，防止单飞重复读目标。
	refreshing map[uint64]bool
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
		graphCache: map[uint64]cachedGraph{}, refreshing: map[uint64]bool{}, cacheTTL: 15 * time.Second, now: time.Now,
	}
}

// Get 返回计划的结构投影：缓存未过期直接返回；过期时立即返回旧投影并在后台单飞刷新，
// 从未读取过才同步读一次。
// 为什么必须这样：运行详情页每 4 秒轮询一次，若每次都同步读目标（list+findById 约 1s+），
// 既让轮询请求本身动辄十几秒，又会长时间占用账号使用锁，与执行器的 prepare 读、写前探活
// 互斥，直接拖慢每个动作的执行节奏——结构在运行期稳定，轮询绝不该同步等目标。
func (s *FlowGraphService) Get(ctx context.Context, planID uint64) (model.FlowGraph, error) {
	if graph, ok := s.cachedGraph(planID); ok {
		return graph, nil
	}
	if stale, ok := s.lastGoodGraph(planID); ok {
		// 旧投影已过期：先返回旧值，后台单飞刷新，避免轮询风暴同时读目标。
		s.refreshInBackground(planID)
		return stale, nil
	}
	graph, err := s.readGraph(ctx, planID)
	if err != nil {
		return model.FlowGraph{}, err
	}
	s.storeGraph(planID, graph)
	return graph, nil
}

// refreshInBackground 在后台刷新投影；refreshing 标志保证同一计划同时只跑一个刷新，
// 避免高频轮询打出并发目标读。刷新失败时保留旧投影，下次过期再试。
func (s *FlowGraphService) refreshInBackground(planID uint64) {
	s.cacheMu.Lock()
	if s.refreshing[planID] {
		s.cacheMu.Unlock()
		return
	}
	s.refreshing[planID] = true
	s.cacheMu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), backgroundRefreshTimeout)
		defer cancel()
		graph, err := s.readGraph(ctx, planID)
		s.cacheMu.Lock()
		delete(s.refreshing, planID)
		s.cacheMu.Unlock()
		if err != nil {
			return
		}
		s.storeGraph(planID, graph)
	}()
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
