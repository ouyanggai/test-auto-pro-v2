package backend

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"test-auto-pro-v2/internal/service"
)

type recordingCandidateProvider struct {
	mu      sync.Mutex
	calls   []string
	started chan string
	release <-chan struct{}
	fail    map[string]error
}

// ComponentCandidates 记录候选权限维度，并可阻塞以验证单飞和锁边界。
func (p *recordingCandidateProvider) ComponentCandidates(ctx context.Context, account, flowCode, componentType string) ([]any, error) {
	key := strings.Join([]string{account, flowCode, componentType}, "/")
	p.mu.Lock()
	p.calls = append(p.calls, key)
	p.mu.Unlock()
	if p.started != nil {
		p.started <- key
	}
	if p.release != nil {
		select {
		case <-p.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if err := p.fail[componentType]; err != nil {
		return nil, err
	}
	return []any{map[string]any{"id": key, "name": componentType}}, nil
}

// snapshotCalls 返回并发安全的调用快照。
func (p *recordingCandidateProvider) snapshotCalls() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.calls...)
}

// TestComponentCandidateCacheLoadsOnlyRequestedTypes 验证只加载模板实际组件，并覆盖五个缓存隔离维度。
func TestComponentCandidateCacheLoadsOnlyRequestedTypes(t *testing.T) {
	provider := &recordingCandidateProvider{}
	cache := service.NewComponentCandidateCache(provider, 100, time.Minute)
	types := []string{"custome-select-project", "custome-select-project"}
	first, err := cache.GetCandidateSet(context.Background(), "account-a", "flow-a", "template-a", "rule-a", types)
	if err != nil || len(first.ByComponent["custome-select-project"]) != 1 {
		t.Fatalf("首次按需候选失败：set=%+v err=%v", first, err)
	}
	if _, err := cache.GetCandidateSet(context.Background(), "account-a", "flow-a", "template-a", "rule-a", types); err != nil {
		t.Fatalf("同键缓存读取失败：%v", err)
	}
	for _, dimensions := range [][4]string{
		{"account-b", "flow-a", "template-a", "rule-a"},
		{"account-a", "flow-b", "template-a", "rule-a"},
		{"account-a", "flow-a", "template-b", "rule-a"},
		{"account-a", "flow-a", "template-a", "rule-b"},
	} {
		if _, err := cache.GetCandidateSet(context.Background(), dimensions[0], dimensions[1], dimensions[2], dimensions[3], types); err != nil {
			t.Fatalf("缓存维度变化后的候选读取失败：%v", err)
		}
	}
	calls := provider.snapshotCalls()
	if len(calls) != 5 {
		t.Fatalf("缓存没有覆盖账号、流程、模板、组件和规则版本，或预取了未使用组件：%v", calls)
	}
	for _, call := range calls {
		if !strings.HasSuffix(call, "/custome-select-project") {
			t.Fatalf("加载了模板未使用的组件：%v", calls)
		}
	}
}

// TestComponentCandidateCacheSingleflightDoesNotHoldGlobalLock 验证同键单飞且不同键远端请求可并行开始。
func TestComponentCandidateCacheSingleflightDoesNotHoldGlobalLock(t *testing.T) {
	release := make(chan struct{})
	provider := &recordingCandidateProvider{started: make(chan string, 3), release: release}
	cache := service.NewComponentCandidateCache(provider, 100, time.Minute)
	var wait sync.WaitGroup
	for _, componentType := range []string{"custome-select-project", "custome-select-project", "city-select"} {
		componentType := componentType
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, _ = cache.GetCandidateSet(context.Background(), "account", "flow", "template", "rule", []string{componentType})
		}()
	}
	started := []string{<-provider.started, <-provider.started}
	sort.Strings(started)
	if started[0] == started[1] || !strings.Contains(strings.Join(started, "|"), "city-select") {
		t.Fatalf("同键没有单飞或全局锁阻塞了不同组件：%v", started)
	}
	select {
	case third := <-provider.started:
		t.Fatalf("同一组件发起了重复远端请求：%s", third)
	case <-time.After(80 * time.Millisecond):
	}
	close(release)
	wait.Wait()
	if calls := provider.snapshotCalls(); len(calls) != 2 {
		t.Fatalf("单飞调用数不正确：%v", calls)
	}
}

// TestComponentCandidateCacheKeepsSafePartialResults 验证一个候选来源失败时保留其他组件真实结果并返回降级错误。
func TestComponentCandidateCacheKeepsSafePartialResults(t *testing.T) {
	provider := &recordingCandidateProvider{fail: map[string]error{"city-select": errors.New("城市源不可用")}}
	cache := service.NewComponentCandidateCache(provider, 100, time.Minute)
	set, err := cache.GetCandidateSet(
		context.Background(), "account", "flow", "template", "rule", []string{"city-select", "custome-select-project"},
	)
	if err == nil || !strings.Contains(err.Error(), "城市源不可用") {
		t.Fatalf("候选失败没有形成可降级错误：%v", err)
	}
	if len(set.ByComponent["custome-select-project"]) != 1 || len(set.ByComponent["city-select"]) != 0 {
		t.Fatalf("局部失败丢失了安全候选或伪造了失败候选：%+v", set.ByComponent)
	}
}

// TestComponentCandidateCacheInvalidationAndLRU 验证失效和容量驱逐不跨账号删除。
func TestComponentCandidateCacheInvalidationAndLRU(t *testing.T) {
	provider := &recordingCandidateProvider{}
	cache := service.NewComponentCandidateCache(provider, 2, time.Minute)
	for _, account := range []string{"account-a", "account-b", "account-c"} {
		_, _ = cache.GetCandidateSet(context.Background(), account, "flow", "template", "rule", []string{"city-select"})
	}
	if stats := cache.Stats(); stats.TotalEntries != 2 {
		t.Fatalf("LRU 容量没有生效：%+v", stats)
	}
	cache.InvalidateAccount("account-b")
	if stats := cache.Stats(); stats.TotalEntries > 1 {
		t.Fatalf("账号失效范围错误：%+v", stats)
	}
}
