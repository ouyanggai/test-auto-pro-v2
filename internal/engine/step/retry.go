package step

import (
	"context"
	"errors"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
)

// RetryPolicy 保存只读重试预算和写请求连接阶段的独立重试预算，全部来自配置不写死。
type RetryPolicy struct {
	Attempts              int
	BaseDelay             time.Duration
	MaxDelay              time.Duration
	WriteConnectAttempts  int
	WriteConnectBaseDelay time.Duration
	WriteConnectMaxDelay  time.Duration
	Now                   func() time.Time
	Sleep                 func(d time.Duration)
}

// retryableTargetError 判断只读阶段遇到的错误是否值得重试。
// 只有会话失效或未收到完整响应的网络/格式错误可重试；完整业务响应必须原样暴露。
func retryableTargetError(err error) bool {
	return target.IsRetryableReadError(err)
}

// backoff 计算第 attempt 次失败后的指数退避间隔（从 1 计），封顶 MaxDelay。
func (p RetryPolicy) backoff(attempt int) time.Duration {
	delay := p.BaseDelay
	for i := 1; i < attempt && delay < p.MaxDelay; i++ {
		delay *= 2
	}
	if delay > p.MaxDelay {
		delay = p.MaxDelay
	}
	return delay
}

// writeConnectBackoff 计算写请求连接阶段失败后的退避间隔。
func (p RetryPolicy) writeConnectBackoff(attempt int) time.Duration {
	delay := p.WriteConnectBaseDelay
	if delay <= 0 {
		return 0
	}
	maxDelay := p.WriteConnectMaxDelay
	if maxDelay <= 0 {
		maxDelay = delay
	}
	for i := 1; i < attempt && delay < maxDelay; i++ {
		delay *= 2
	}
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

// RunWithWriteConnectRetry 只重试明确未写出请求的连接错误。
// 返回值最后一项表示是否已经尝试过目标写请求；连接阶段全部失败时为 false，
// 响应丢失、业务拒绝和会话失效都为 true，调用方据此决定零写入或不确定结果。
func RunWithWriteConnectRetry[R any](ctx context.Context, policy RetryPolicy, call func(context.Context) (R, target.WriteResponse, string, error), onRetry func(attempt int, nextDelay time.Duration)) (R, target.WriteResponse, string, error, bool) {
	var zero R
	attempts := policy.WriteConnectAttempts
	if attempts < 1 {
		attempts = 1
	}
	var last R
	var response target.WriteResponse
	var traceID string
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		value, currentResponse, currentTraceID, err := call(target.WithRetryAttempt(ctx, attempt > 1, attempt))
		last, response, traceID, lastErr = value, currentResponse, currentTraceID, err
		if err == nil {
			return value, currentResponse, currentTraceID, nil, true
		}
		if !target.IsRetryableWriteConnectError(err) || attempt >= attempts {
			return value, currentResponse, currentTraceID, err, target.TransportOf(err) != target.TransportConnectFailed
		}
		delay := policy.writeConnectBackoff(attempt)
		if onRetry != nil {
			onRetry(attempt, delay)
		}
		if delay <= 0 {
			continue
		}
		if policy.Sleep != nil {
			policy.Sleep(delay)
			continue
		}
		select {
		case <-ctx.Done():
			return zero, response, traceID, ctx.Err(), false
		case <-time.After(delay):
		}
	}
	return last, response, traceID, lastErr, false
}

// RunWithConnectRetry 只对登录/刷新会话时尚未建立连接的错误重试，响应丢失不得循环登录。
func RunWithConnectRetry[T any](ctx context.Context, policy RetryPolicy, call func(context.Context) (T, error), onRetry func(attempt int, nextDelay time.Duration)) (T, error) {
	var zero T
	attempts := policy.Attempts
	if attempts < 1 {
		attempts = 1
	}
	for attempt := 1; attempt <= attempts; attempt++ {
		value, err := call(target.WithRetryAttempt(ctx, attempt > 1, attempt))
		if err == nil {
			return value, nil
		}
		if !target.IsRetryableWriteConnectError(err) || attempt >= attempts {
			return value, err
		}
		delay := policy.backoff(attempt)
		if onRetry != nil {
			onRetry(attempt, delay)
		}
		if policy.Sleep != nil {
			policy.Sleep(delay)
			continue
		}
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(delay):
		}
	}
	return zero, errors.New("会话连接重试未执行")
}

// RunWithRetry 执行一次只读操作，失败且可重试时按预算退避重试。
// onRetry 会在每次决定重试时被调用（次数与下次间隔），供调用方把重试如实写进 step.log，
// 不允许出现“看起来只调了一次”的日志。
func RunWithRetry[T any](ctx context.Context, policy RetryPolicy, operation string, call func() (T, error), onRetry func(attempt int, nextDelay time.Duration)) (T, error) {
	var zero T
	if policy.Attempts < 1 {
		policy.Attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= policy.Attempts; attempt++ {
		result, err := call()
		if err == nil {
			return result, nil
		}
		lastErr = err
		if attempt >= policy.Attempts || !retryableTargetError(err) {
			return zero, err
		}
		nextDelay := policy.backoff(attempt)
		if onRetry != nil {
			onRetry(attempt, nextDelay)
		}
		if policy.Sleep != nil {
			policy.Sleep(nextDelay)
		} else {
			select {
			case <-ctx.Done():
				return zero, ctx.Err()
			case <-time.After(nextDelay):
			}
		}
	}
	return zero, lastErr
}
