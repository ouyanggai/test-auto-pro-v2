package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
)

// ComponentCandidateCache 为组件候选提供内存缓存，按规则版本+账号+流程编码隔离。
type ComponentCandidateCache struct {
	provider target.ComponentCandidateProvider
	mu       sync.RWMutex
	entries  map[string]*candidateCacheEntry
	maxSize  int
	ttl      time.Duration
}

type candidateCacheEntry struct {
	set       target.ComponentCandidateSet
	expiresAt time.Time
	accessAt  time.Time
}

// NewComponentCandidateCache 创建组件候选缓存服务。
func NewComponentCandidateCache(provider target.ComponentCandidateProvider, maxSize int, ttl time.Duration) *ComponentCandidateCache {
	if maxSize <= 0 {
		maxSize = 1000
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return &ComponentCandidateCache{
		provider: provider,
		entries:  make(map[string]*candidateCacheEntry),
		maxSize:  maxSize,
		ttl:      ttl,
	}
}

// GetCandidateSet 获取或加载一个流程的完整组件候选集合。
func (c *ComponentCandidateCache) GetCandidateSet(ctx context.Context, account, flowCode, ruleVersion string) (target.ComponentCandidateSet, error) {
	key := candidateCacheKey(account, flowCode, ruleVersion)

	// 快速路径：读锁检查缓存
	c.mu.RLock()
	if entry, exists := c.entries[key]; exists && time.Now().Before(entry.expiresAt) {
		entry.accessAt = time.Now()
		set := entry.set
		c.mu.RUnlock()
		return set, nil
	}
	c.mu.RUnlock()

	// 慢速路径：写锁加载
	c.mu.Lock()
	defer c.mu.Unlock()

	// 双重检查：可能其他 goroutine 已加载
	if entry, exists := c.entries[key]; exists && time.Now().Before(entry.expiresAt) {
		entry.accessAt = time.Now()
		return entry.set, nil
	}

	// 执行实际加载
	set, err := c.loadCandidateSet(ctx, account, flowCode, ruleVersion)
	if err != nil {
		return target.ComponentCandidateSet{}, err
	}

	// 驱逐过期条目
	c.evictExpired()

	// LRU 驱逐
	if len(c.entries) >= c.maxSize {
		c.evictLRU()
	}

	// 存入缓存
	now := time.Now()
	c.entries[key] = &candidateCacheEntry{
		set:       set,
		expiresAt: now.Add(c.ttl),
		accessAt:  now,
	}

	return set, nil
}

// GetFieldCandidates 按字段路径和组件类型获取候选项。
func (c *ComponentCandidateCache) GetFieldCandidates(ctx context.Context, account, flowCode, ruleVersion, fieldPath, componentType string) ([]any, error) {
	set, err := c.GetCandidateSet(ctx, account, flowCode, ruleVersion)
	if err != nil {
		return nil, err
	}

	return extractFieldCandidates(set, fieldPath, componentType), nil
}

// Invalidate 使指定账号+流程的缓存失效。
func (c *ComponentCandidateCache) Invalidate(account, flowCode, ruleVersion string) {
	key := candidateCacheKey(account, flowCode, ruleVersion)
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}

// InvalidateAccount 使指定账号的全部缓存失效。
func (c *ComponentCandidateCache) InvalidateAccount(account string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key := range c.entries {
		if matchesCacheKeyAccount(key, account) {
			delete(c.entries, key)
		}
	}
}

// Stats 返回缓存统计信息。
func (c *ComponentCandidateCache) Stats() ComponentCandidateCacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	expired := 0
	now := time.Now()
	for _, entry := range c.entries {
		if now.After(entry.expiresAt) {
			expired++
		}
	}

	return ComponentCandidateCacheStats{
		TotalEntries:   len(c.entries),
		ExpiredEntries: expired,
		MaxSize:        c.maxSize,
		TTL:            c.ttl,
	}
}

// ComponentCandidateCacheStats 是缓存统计信息。
type ComponentCandidateCacheStats struct {
	TotalEntries   int           `json:"totalEntries"`
	ExpiredEntries int           `json:"expiredEntries"`
	MaxSize        int           `json:"maxSize"`
	TTL            time.Duration `json:"ttl"`
}

// loadCandidateSet 从目标平台加载完整候选集合，调用方必须持有写锁。
func (c *ComponentCandidateCache) loadCandidateSet(ctx context.Context, account, flowCode, ruleVersion string) (target.ComponentCandidateSet, error) {
	if c.provider == nil {
		return target.ComponentCandidateSet{
			FlowCode:    flowCode,
			Account:     account,
			RuleVersion: ruleVersion,
		}, nil
	}

	set := target.ComponentCandidateSet{
		FlowCode:    flowCode,
		Account:     account,
		RuleVersion: ruleVersion,
		Materials:   make(map[string][]target.MaterialCandidate),
	}

	// 并行加载各类候选
	type loadResult struct {
		name string
		err  error
	}
	results := make(chan loadResult, 7)

	// 材料（出库）
	go func() {
		materials, err := c.provider.GetMaterialCandidates(ctx, account, flowCode, "out")
		if err == nil {
			set.Materials["out"] = materials
		}
		results <- loadResult{name: "materials_out", err: err}
	}()

	// 材料（入库）
	go func() {
		materials, err := c.provider.GetMaterialCandidates(ctx, account, flowCode, "in")
		if err == nil {
			set.Materials["in"] = materials
		}
		results <- loadResult{name: "materials_in", err: err}
	}()

	// 项目
	go func() {
		projects, err := c.provider.GetProjectCandidates(ctx, account, flowCode)
		if err == nil {
			set.Projects = projects
		}
		results <- loadResult{name: "projects", err: err}
	}()

	// 订单
	go func() {
		orders, err := c.provider.GetOrderCandidates(ctx, account, flowCode)
		if err == nil {
			set.Orders = orders
		}
		results <- loadResult{name: "orders", err: err}
	}()

	// 流程列表
	go func() {
		flowLists, err := c.provider.GetFlowListCandidates(ctx, account, flowCode)
		if err == nil {
			set.FlowLists = flowLists
		}
		results <- loadResult{name: "flow_lists", err: err}
	}()

	// 费用预算类型
	go func() {
		budgetTypes, err := c.provider.GetExpenseBudgetTypes(ctx, account)
		if err == nil {
			set.ExpenseBudgetTypes = budgetTypes
		}
		results <- loadResult{name: "expense_budget_types", err: err}
	}()

	// 城市
	go func() {
		cities, err := c.provider.GetCityCandidates(ctx, account)
		if err == nil {
			set.Cities = cities
		}
		results <- loadResult{name: "cities", err: err}
	}()

	// 等待全部完成，记录错误但不终止
	var firstError error
	for i := 0; i < 7; i++ {
		result := <-results
		if result.err != nil && firstError == nil {
			firstError = result.err
		}
	}

	// 即使部分失败也返回已加载的候选
	return set, nil
}

// evictExpired 驱逐已过期条目，调用方必须持有写锁。
func (c *ComponentCandidateCache) evictExpired() {
	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
}

// evictLRU 驱逐最久未访问的条目，调用方必须持有写锁。
func (c *ComponentCandidateCache) evictLRU() {
	if len(c.entries) == 0 {
		return
	}

	var oldestKey string
	var oldestTime time.Time
	first := true

	for key, entry := range c.entries {
		if first || entry.accessAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.accessAt
			first = false
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// candidateCacheKey 生成缓存键。
func candidateCacheKey(account, flowCode, ruleVersion string) string {
	return fmt.Sprintf("%s:%s:%s", account, flowCode, ruleVersion)
}

// matchesCacheKeyAccount 判断缓存键是否属于指定账号。
func matchesCacheKeyAccount(key, account string) bool {
	prefix := account + ":"
	return len(key) > len(prefix) && key[:len(prefix)] == prefix
}

// extractFieldCandidates 从候选集合中提取特定字段的候选项。
func extractFieldCandidates(set target.ComponentCandidateSet, fieldPath, componentType string) []any {
	switch componentType {
	case "out-bound-material-select":
		return toAnySlice(set.Materials["out"])
	case "in-bound-material-select":
		return toAnySlice(set.Materials["in"])
	case "custome-select-project":
		return toAnySlice(set.Projects)
	case "travel-order-management":
		return toAnySlice(set.Orders)
	case "general-flow-list-mulSelect", "flow-list-mul-select":
		return toAnySlice(set.FlowLists)
	case "custome-expense-budgetType":
		return toAnySlice(set.ExpenseBudgetTypes)
	case "city-select":
		return toAnySlice(set.Cities)
	case "travel-route-planning":
		return toAnySlice(set.TravelRoutes)
	default:
		return []any{}
	}
}

// toAnySlice 将类型化切片转换为 []any。
func toAnySlice[T any](items []T) []any {
	result := make([]any, len(items))
	for i, item := range items {
		result[i] = item
	}
	return result
}

// SerializeCandidateForComponent 将候选对象序列化为组件要求的 JSON 字符串。
func SerializeCandidateForComponent(candidate any, componentType string) (any, error) {
	// 大部分自定义组件使用 JSON 字符串序列化
	switch componentType {
	case "custom-weather":
		// custom-weather 使用纯字符串
		if str, ok := candidate.(string); ok {
			return str, nil
		}
		return fmt.Sprint(candidate), nil
	default:
		// 其他组件使用 JSON 字符串
		data, err := json.Marshal(candidate)
		if err != nil {
			return nil, err
		}
		return string(data), nil
	}
}
