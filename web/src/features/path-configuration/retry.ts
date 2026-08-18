const defaultRetryDelays = [500, 1200] as const

// isRetryablePathLoadError 只识别 API 明确标记的瞬时失败，业务数据错误不得被静默重试。
export function isRetryablePathLoadError(error: unknown): boolean {
  return typeof error === 'object' && error !== null && 'retryable' in error
    && (error as { retryable?: unknown }).retryable === true
}

// waitForPathLoadRetry 在重试等待期间监听取消信号，避免路由切换后留下定时器或继续发起请求。
function waitForPathLoadRetry(delayMs: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal.aborted) {
      reject(signal.reason ?? new DOMException('请求已取消', 'AbortError'))
      return
    }
    const timer = globalThis.setTimeout(() => {
      signal.removeEventListener('abort', abort)
      resolve()
    }, delayMs)
    const abort = () => {
      globalThis.clearTimeout(timer)
      signal.removeEventListener('abort', abort)
      reject(signal.reason ?? new DOMException('请求已取消', 'AbortError'))
    }
    signal.addEventListener('abort', abort, { once: true })
  })
}

// retryPathLoad 保持初始化 loading 直到成功或耗尽瞬时失败重试次数。
export async function retryPathLoad<T>(
  operation: (signal: AbortSignal) => Promise<T>,
  signal: AbortSignal,
  retryDelays: readonly number[] = defaultRetryDelays,
): Promise<T> {
  let retryIndex = 0
  while (true) {
    try {
      return await operation(signal)
    }
    catch (error) {
      if (signal.aborted || !isRetryablePathLoadError(error) || retryIndex >= retryDelays.length) throw error
      await waitForPathLoadRetry(retryDelays[retryIndex], signal)
      retryIndex += 1
    }
  }
}
