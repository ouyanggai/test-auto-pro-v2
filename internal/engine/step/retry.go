package step

import (
	"context"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
)

// RetryPolicy 是只读阶段的有界重试预算，全部来自配置不写死（纲领第 4.4.1 节：
// 坏窗口可长达十几分钟，只读重试按分钟级设计；submit 阶段的重试被彻底禁止，与本机制无关）。
type RetryPolicy struct {
	Attempts  int
	BaseDelay time.Duration
	MaxDelay  time.Duration
	Now       func() time.Time
	Sleep     func(d time.Duration)
}

// retryableTargetError 判断只读阶段遇到的错误是否值得重试：
// 目标抖动（超时、暂时不可用）、会话失效与响应异常都可重试；
// 登录被拒与权限拒绝是确定性失败，重试只会拖慢失败结论。
func retryableTargetError(err error) bool {
	switch {
	case target.IsKind(err, target.ErrorTimeout),
		target.IsKind(err, target.ErrorUnavailable),
		target.IsKind(err, target.ErrorSessionExpired),
		target.IsKind(err, target.ErrorResponseInvalid):
		return true
	default:
		return false
	}
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

// withRetry 执行一次只读操作，失败且可重试时按预算退避重试。
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
