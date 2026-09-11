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
type requestRetryKey struct{}
type requestRetryAttemptKey struct{}

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

// WithRetryAttempt 标记目标请求是否为网络重试及其尝试序号，供传输日志准确记录。
func WithRetryAttempt(ctx context.Context, retry bool, attempt int) context.Context {
	ctx = context.WithValue(ctx, requestRetryKey{}, retry)
	return context.WithValue(ctx, requestRetryAttemptKey{}, attempt)
}

// RetryFromContext 返回请求是否由受控网络重试发起。
func RetryFromContext(ctx context.Context) bool {
	value, _ := ctx.Value(requestRetryKey{}).(bool)
	return value
}

// RetryAttemptFromContext 返回请求尝试序号；未标记时按首次请求处理。
func RetryAttemptFromContext(ctx context.Context) int {
	value, ok := ctx.Value(requestRetryAttemptKey{}).(int)
	if !ok || value < 1 {
		return 1
	}
	return value
}
