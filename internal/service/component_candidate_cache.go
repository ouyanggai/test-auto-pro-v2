package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
)

// ComponentCandidateCache 按账号、流程、模板、组件和规则版本隔离真实候选。
type ComponentCandidateCache struct {
	provider target.ComponentCandidateProvider
	mu       sync.Mutex
	entries  map[string]candidateCacheEntry
	flights  map[string]*candidateCacheFlight
	maxSize  int
	ttl      time.Duration
}

type candidateCacheEntry struct {
	values    []any
	expiresAt time.Time
	accessAt  time.Time
}

type candidateCacheFlight struct {
	done   chan struct{}
	values []any
	err    error
}

type candidateLoadResult struct {
	componentType string
	values        []any
	err           error
}

// NewComponentCandidateCache 创建有界候选缓存；远端读取始终在全局锁外执行。
func NewComponentCandidateCache(provider target.ComponentCandidateProvider, maxSize int, ttl time.Duration) *ComponentCandidateCache {
	if maxSize <= 0 {
		maxSize = 1000
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return &ComponentCandidateCache{
		provider: provider, entries: make(map[string]candidateCacheEntry), flights: make(map[string]*candidateCacheFlight),
		maxSize: maxSize, ttl: ttl,
	}
}

// GetCandidateSet 只并发加载当前模板实际使用且有真实入口的组件类型，单路径失败不丢弃其他安全候选。
func (c *ComponentCandidateCache) GetCandidateSet(ctx context.Context, account, flowCode, templateID, ruleVersion string, componentTypes []string) (target.ComponentCandidateSet, error) {
	types := target.SortedComponentCandidateTypes(componentTypes)
	set := target.ComponentCandidateSet{
		Account: strings.TrimSpace(account), FlowCode: strings.TrimSpace(flowCode), TemplateID: strings.TrimSpace(templateID),
		RuleVersion: strings.TrimSpace(ruleVersion), ByComponent: make(map[string][]any, len(types)), Errors: make(map[string]error, len(types)),
	}
	if len(types) == 0 {
		return set, nil
	}
	results := make(chan candidateLoadResult, len(types))
	for _, componentType := range types {
		componentType := componentType
		go func() {
			values, err := c.getComponent(ctx, account, flowCode, templateID, ruleVersion, componentType)
			results <- candidateLoadResult{componentType: componentType, values: values, err: err}
		}()
	}
	var joined error
	for range types {
		result := <-results
		if len(result.values) > 0 {
			set.ByComponent[result.componentType] = cloneAnyCandidates(result.values)
		}
		if result.err != nil {
			set.Errors[result.componentType] = result.err
			joined = errors.Join(joined, fmt.Errorf("%s：%w", result.componentType, result.err))
		}
	}
	return set, joined
}

// RefreshCandidateSet 使当前规则范围的旧缓存失效后重新读取候选，供运行前预检核对外部对象仍然可用。
func (c *ComponentCandidateCache) RefreshCandidateSet(ctx context.Context, account, flowCode, templateID, ruleVersion string, componentTypes []string) (target.ComponentCandidateSet, error) {
	c.Invalidate(account, flowCode, templateID, ruleVersion)
	return c.GetCandidateSet(ctx, account, flowCode, templateID, ruleVersion, componentTypes)
}

// getComponent 以组件级键实现单飞；等待者支持自己的 context 取消。
func (c *ComponentCandidateCache) getComponent(ctx context.Context, account, flowCode, templateID, ruleVersion, componentType string) ([]any, error) {
	key := candidateCacheKey(account, flowCode, templateID, componentType, ruleVersion)
	now := time.Now()
	c.mu.Lock()
	if entry, exists := c.entries[key]; exists && now.Before(entry.expiresAt) {
		entry.accessAt = now
		c.entries[key] = entry
		values := cloneAnyCandidates(entry.values)
		c.mu.Unlock()
		return values, nil
	}
	if flight, exists := c.flights[key]; exists {
		c.mu.Unlock()
		select {
		case <-flight.done:
			return cloneAnyCandidates(flight.values), flight.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	flight := &candidateCacheFlight{done: make(chan struct{})}
	c.flights[key] = flight
	c.mu.Unlock()

	values, err := c.loadComponent(ctx, account, flowCode, componentType)
	c.mu.Lock()
	flight.values = cloneAnyCandidates(values)
	flight.err = err
	if err == nil {
		c.evictExpiredLocked(now)
		if len(c.entries) >= c.maxSize {
			c.evictLRULocked()
		}
		c.entries[key] = candidateCacheEntry{values: cloneAnyCandidates(values), expiresAt: now.Add(c.ttl), accessAt: now}
	}
	delete(c.flights, key)
	close(flight.done)
	c.mu.Unlock()
	return values, err
}

// loadComponent 在互斥锁外调用目标平台；没有提供者时不能伪造空候选成功。
func (c *ComponentCandidateCache) loadComponent(ctx context.Context, account, flowCode, componentType string) ([]any, error) {
	if c.provider == nil {
		return nil, target.ErrComponentCandidatesUnsupported
	}
	return c.provider.ComponentCandidates(ctx, account, flowCode, componentType)
}

// Invalidate 使指定账号、流程、模板和规则版本的全部组件缓存失效。
func (c *ComponentCandidateCache) Invalidate(account, flowCode, templateID, ruleVersion string) {
	prefix := candidateCacheScopePrefix(account, flowCode, templateID)
	suffix := ":" + strings.TrimSpace(ruleVersion)
	c.mu.Lock()
	for key := range c.entries {
		if strings.HasPrefix(key, prefix) && strings.HasSuffix(key, suffix) {
			delete(c.entries, key)
		}
	}
	c.mu.Unlock()
}

// InvalidateAccount 使指定账号的全部候选缓存失效。
func (c *ComponentCandidateCache) InvalidateAccount(account string) {
	prefix := strings.TrimSpace(account) + ":"
	c.mu.Lock()
	for key := range c.entries {
		if strings.HasPrefix(key, prefix) {
			delete(c.entries, key)
		}
	}
	c.mu.Unlock()
}

// Stats 返回不触发远端读取的缓存统计。
func (c *ComponentCandidateCache) Stats() ComponentCandidateCacheStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	expired := 0
	now := time.Now()
	for _, entry := range c.entries {
		if !now.Before(entry.expiresAt) {
			expired++
		}
	}
	return ComponentCandidateCacheStats{TotalEntries: len(c.entries), ExpiredEntries: expired, InFlight: len(c.flights), MaxSize: c.maxSize, TTL: c.ttl}
}

// ComponentCandidateCacheStats 是候选缓存的非敏感统计。
type ComponentCandidateCacheStats struct {
	TotalEntries   int           `json:"totalEntries"`
	ExpiredEntries int           `json:"expiredEntries"`
	InFlight       int           `json:"inFlight"`
	MaxSize        int           `json:"maxSize"`
	TTL            time.Duration `json:"ttl"`
}

// evictExpiredLocked 删除过期缓存；调用方必须持有互斥锁。
func (c *ComponentCandidateCache) evictExpiredLocked(now time.Time) {
	for key, entry := range c.entries {
		if !now.Before(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
}

// evictLRULocked 删除最久未访问条目；调用方必须持有互斥锁。
func (c *ComponentCandidateCache) evictLRULocked() {
	var oldestKey string
	var oldestTime time.Time
	for key, entry := range c.entries {
		if oldestKey == "" || entry.accessAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.accessAt
		}
	}
	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// candidateCacheKey 生成包含全部权限和规则维度的组件级缓存键。
func candidateCacheKey(account, flowCode, templateID, componentType, ruleVersion string) string {
	return strings.Join([]string{
		strings.TrimSpace(account), strings.TrimSpace(flowCode), strings.TrimSpace(templateID), strings.TrimSpace(componentType), strings.TrimSpace(ruleVersion),
	}, ":")
}

// candidateCacheScopePrefix 返回账号、流程和模板的稳定键前缀。
func candidateCacheScopePrefix(account, flowCode, templateID string) string {
	return strings.Join([]string{strings.TrimSpace(account), strings.TrimSpace(flowCode), strings.TrimSpace(templateID)}, ":") + ":"
}

// cloneAnyCandidates 复制候选切片，避免调用方修改缓存中的集合边界。
func cloneAnyCandidates(values []any) []any {
	return append([]any(nil), values...)
}
