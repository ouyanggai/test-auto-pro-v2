package target

import (
	"context"

	"test-auto-pro-v2/internal/logging"
)

// 请求级元数据的上下文键：请求分类（read/write）与关联用 trace_id。
// 由唯一请求出口在发请求前写入，传输层日志读取后落盘，
// 使运行事实里的尝试记录与 network.log/curl.log 能按同一 trace_id 双向可达。
type requestClassKey struct{}

type requestTraceIDKey struct{}

// RequestClassFromContext 读取请求分类；未标记的请求一律按只读处理。
func RequestClassFromContext(ctx context.Context) string {
	if value, ok := ctx.Value(requestClassKey{}).(string); ok && value != "" {
		return value
	}
	return "read"
}

// requestTraceIDFromContext 读取调用方预生成的 trace_id；不存在时由传输层现场生成。
func requestTraceIDFromContext(ctx context.Context) string {
	if value, ok := ctx.Value(requestTraceIDKey{}).(string); ok && value != "" {
		return value
	}
	return logging.NewTraceID()
}

// withRequestMetadata 把请求分类与 trace_id 挂到请求上下文。
func withRequestMetadata(ctx context.Context, class, traceID string) context.Context {
	ctx = context.WithValue(ctx, requestClassKey{}, class)
	return context.WithValue(ctx, requestTraceIDKey{}, traceID)
}
