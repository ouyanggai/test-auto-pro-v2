package api

import (
	"context"
	"strconv"
	"strings"

	"test-auto-pro-v2/internal/logging"
)

// LogScopeResolver 解析一次请求归属的业务对象。
// 中间件只从路由里取不可变 ID，计划名与执行路径名一律来自真实业务记录，禁止从 URL 字符串猜测。
type LogScopeResolver interface {
	ResolveLogScope(ctx context.Context, planID, pathID uint64) logging.Scope
}

// routeSubjectIDs 从请求路径里取出计划与执行路径的不可变 ID。
// 计划相关路由统一形如 /api/plans/<计划ID>[/execution-paths/<路径ID>]/...，
// 取不到时返回 0，交由调用方按无业务归属处理。
func routeSubjectIDs(path string) (uint64, uint64) {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) < 3 || segments[0] != "api" || segments[1] != "plans" {
		return 0, 0
	}
	planID := parseRouteID(segments[2])
	if planID == 0 {
		return 0, 0
	}
	if len(segments) >= 5 && segments[3] == "execution-paths" {
		return planID, parseRouteID(segments[4])
	}
	return planID, 0
}

// parseRouteID 解析路由段里的正整数 ID，非法值按缺失处理。
func parseRouteID(segment string) uint64 {
	value, err := strconv.ParseUint(strings.TrimSpace(segment), 10, 64)
	if err != nil {
		return 0
	}
	return value
}

// requestScope 组装一次请求的日志作用域：先生成请求标识，再补上业务归属。
// 只要能从路由确定计划 ID 就一定写进作用域，即使显示名查不到，日志也必须留在该计划目录，
// 不允许因为上下文没接通就把本属于某个计划的日志降级到应用程序目录。
// 归属解析每次请求只做一次，后续所有日志复用同一个作用域，不为每条日志重复查询数据库。
func requestScope(ctx context.Context, path string, resolver LogScopeResolver) logging.Scope {
	scope := logging.Scope{RequestID: logging.NewTraceID()}
	planID, pathID := routeSubjectIDs(path)
	if planID == 0 {
		return scope
	}
	scope.PlanID = strconv.FormatUint(planID, 10)
	if pathID != 0 {
		scope.ExecutionPathID = strconv.FormatUint(pathID, 10)
	}
	if resolver == nil {
		return scope
	}
	return scope.Merge(resolver.ResolveLogScope(ctx, planID, pathID))
}
