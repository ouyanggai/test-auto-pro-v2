package service

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"test-auto-pro-v2/internal/logging"
	"test-auto-pro-v2/internal/model"
)

// logScopePlanReader 是解析计划显示名所需的最小读取面。
type logScopePlanReader interface {
	Get(context.Context, uint64) (model.Plan, error)
}

// logScopePathReader 是解析执行路径显示名所需的最小读取面；按计划 ID 读取本身就是归属校验。
type logScopePathReader interface {
	Get(context.Context, uint64, uint64) (model.ExecutionPath, error)
}

// logScopeResolveTimeout 限制归属解析的耗时，日志上下文绝不拖慢业务请求。
const logScopeResolveTimeout = 2 * time.Second

// logScopeCacheLimit 限制缓存条目数，超过后整体清空，避免长期运行后无界增长。
const logScopeCacheLimit = 512

// LogScopeService 解析一次请求归属的业务对象：按不可变 ID 从真实业务记录取计划与执行路径显示名。
// 显示名带短期缓存，一次界面操作的多个请求不会重复查询数据库；解析失败时只留 ID，
// 日志仍然落在该计划目录下，绝不因为名称没查到就降级到应用程序目录。
type LogScopeService struct {
	plans logScopePlanReader
	paths logScopePathReader
	ttl   time.Duration
	now   func() time.Time
	mu    sync.Mutex
	cache map[string]cachedLogName
}

// cachedLogName 是一条缓存的显示名及其过期时间。
type cachedLogName struct {
	name      string
	expiresAt time.Time
}

// NewLogScopeService 基于计划与执行路径的只读仓储创建归属解析器；now 可注入以固定测试时间。
func NewLogScopeService(plans logScopePlanReader, paths logScopePathReader, now func() time.Time) *LogScopeService {
	if now == nil {
		now = time.Now
	}
	return &LogScopeService{plans: plans, paths: paths, ttl: 30 * time.Second, now: now, cache: map[string]cachedLogName{}}
}

// ResolveLogScope 返回该计划与执行路径对应的日志作用域；ID 始终写上，显示名取不到时留空。
func (s *LogScopeService) ResolveLogScope(ctx context.Context, planID, pathID uint64) logging.Scope {
	if s == nil || planID == 0 {
		return logging.Scope{}
	}
	scope := logging.Scope{PlanID: strconv.FormatUint(planID, 10)}
	resolveContext, cancel := context.WithTimeout(ctx, logScopeResolveTimeout)
	defer cancel()
	scope.PlanName = s.cachedName("plan:"+scope.PlanID, func() string {
		if s.plans == nil {
			return ""
		}
		plan, err := s.plans.Get(resolveContext, planID)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(plan.Name)
	})
	if pathID == 0 {
		return scope
	}
	scope.ExecutionPathID = strconv.FormatUint(pathID, 10)
	scope.ExecutionPathName = s.cachedName("path:"+scope.PlanID+":"+scope.ExecutionPathID, func() string {
		if s.paths == nil {
			return ""
		}
		// 按计划 ID 读取执行路径，路径不属于该计划时直接返回错误，等于顺带完成了归属校验。
		path, err := s.paths.Get(resolveContext, planID, pathID)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(path.Name)
	})
	return scope
}

// cachedName 读缓存或回源一次；只缓存成功取到的名称，失败不缓存以便下次请求重试。
func (s *LogScopeService) cachedName(key string, load func() string) string {
	now := s.now()
	s.mu.Lock()
	if cached, ok := s.cache[key]; ok && cached.expiresAt.After(now) {
		s.mu.Unlock()
		return cached.name
	}
	s.mu.Unlock()
	name := load()
	if name == "" {
		return ""
	}
	s.mu.Lock()
	if len(s.cache) >= logScopeCacheLimit {
		s.cache = map[string]cachedLogName{}
	}
	s.cache[key] = cachedLogName{name: name, expiresAt: now.Add(s.ttl)}
	s.mu.Unlock()
	return name
}

// backgroundLogScope 为后台任务构造日志作用域：以任务自己的计划 ID 兜底，请求作用域优先，
// 计划名缺失时再从计划记录补一次。后台任务用 context.Background() 起协程，
// 不这样显式传递就会丢掉计划归属，让本属于某个计划的目标请求日志掉进应用程序目录。
func backgroundLogScope(ctx context.Context, plans *PlanService, planID uint64) logging.Scope {
	if planID == 0 {
		return logging.ScopeFrom(ctx)
	}
	scope := logging.Scope{PlanID: strconv.FormatUint(planID, 10)}.Merge(logging.ScopeFrom(ctx))
	if strings.TrimSpace(scope.PlanName) != "" || plans == nil {
		return scope
	}
	if plan, err := plans.Get(ctx, planID); err == nil {
		scope.PlanName = strings.TrimSpace(plan.Name)
	}
	return scope
}
