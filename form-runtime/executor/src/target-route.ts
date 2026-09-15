// T006 目标请求路由：page.route('**/*') 全量拦截，按路径前缀区分目标请求与 runtime 自身资源。
// 纪律（docs/s/01-执行引擎设计.md 5.3）：
//   - 目标 /web、/flowInstanceApi 与文件请求一律转发 Go，runtime 绝不直接访问目标平台；
//   - 未登记的外部请求直接中止，不能绕过 Go；
//   - 请求头、正文、响应头、响应正文原样转发，不脱敏；二进制（上传）按 base64 保真。
import type { Page, Route } from 'playwright'
import { RequestTraceRecorder, type TraceEntry } from './request-trace.ts'

// TARGET_PATH_PREFIXES 与 Go 侧 internal/runtimeexec（forward.go）镜像维护：
// 命中任一前缀的请求都视为目标平台请求，必须经过 Go 转发。
export const TARGET_PATH_PREFIXES = ['/web/', '/flowInstanceApi']

// TARGET_PREFIXES 匹配规则：路径前缀即目标请求，与来源主机无关——目标网关主机可能随部署变化。
export function isTargetRequest (url: string): boolean {
  let pathname: string
  try {
    pathname = new URL(url).pathname
  } catch {
    return false
  }
  return TARGET_PATH_PREFIXES.some(prefix => pathname.startsWith(prefix))
}

export interface ForwardOutcome {
  httpStatus: number
  headers: Record<string, string>
  bodyEncoding: 'utf8' | 'base64'
  body?: string
}

// TargetRouteOptions.forward 是到 Go 的转发回调（服务层实现，T007 接 WebSocket）；
// 返回即 Go 发回的目标原始响应；抛错表示转发失败（网络断开、Go 拒绝），调用方必须中止请求。
export interface TargetRouteOptions {
  runtimeOrigins: string[]
  forward: (payload: {
    method: string
    url: string
    headers: Record<string, string>
    body?: string
    bodyEncoding: 'utf8' | 'base64'
    action: string
  }) => Promise<ForwardOutcome>
}

export interface TargetRouteHandle {
  trace: RequestTraceRecorder
  uninstall: () => Promise<void>
}

// installTargetRoute 在页面上安装全量路由拦截；返回轨迹与卸载句柄。
// 拦截器贯穿页面整个生命周期，会话关闭时随 Context 一起释放（uninstall 供提前解除）。
export async function installTargetRoute (page: Page, options: TargetRouteOptions): Promise<TargetRouteHandle> {
  const trace = new RequestTraceRecorder()
  const ownOrigins = new Set(options.runtimeOrigins)

  async function handle (route: Route): Promise<void> {
    const request = route.request()
    const isTarget = isTargetRequest(request.url())
    if (!isTarget) {
      let ownOrigin = false
      try {
        ownOrigin = ownOrigins.has(new URL(request.url()).origin)
      } catch {
        ownOrigin = false
      }
      if (!ownOrigin) {
        // 未登记的外部请求：本地直接拒绝，不允许绕过 Go 访问任何外部主机。
        const entry = trace.begin({
          method: request.method(),
          url: request.url(),
          headers: await safeHeaders(request),
          action: ''
        })
        trace.markAborted(entry, 'blocked', '未登记的外部请求被本地拒绝')
        await route.abort('blockedbyclient')
        return
      }
      // runtime 自身脚本、样式与静态资源：正常放行。
      await route.continue()
      return
    }

    // 目标请求：取出正文（可能是上传二进制）后转发 Go。
    const postBuffer = request.postDataBuffer()
    const bodyEncoding: 'utf8' | 'base64' = 'base64'
    const body = postBuffer ? postBuffer.toString('base64') : undefined
    const entry: TraceEntry = trace.begin({
      method: request.method(),
      url: request.url(),
      headers: await safeHeaders(request),
      action: '',
      body,
      bodyEncoding: body ? bodyEncoding : undefined
    })
    try {
      const outcome = await options.forward({
        method: entry.method,
        url: entry.url,
        headers: entry.headers,
        body: entry.body,
        bodyEncoding: entry.bodyEncoding ?? 'utf8',
        action: entry.action
      })
      trace.markFulfilled(entry, {
        httpStatus: outcome.httpStatus,
        headers: outcome.headers,
        bodyEncoding: outcome.bodyEncoding,
        body: outcome.body,
        completedAt: new Date().toISOString(),
        durationMs: Date.now() - Date.parse(entry.requestAt)
      })
      const contentType = outcome.headers['content-type'] ?? outcome.headers['Content-Type']
      const fulfillBody = outcome.body === undefined
        ? undefined
        : outcome.bodyEncoding === 'base64' ? Buffer.from(outcome.body, 'base64') : outcome.body
      await route.fulfill({
        status: outcome.httpStatus,
        headers: outcome.headers,
        ...(contentType ? {} : { contentType: 'application/octet-stream' }),
        ...(fulfillBody === undefined ? {} : { body: fulfillBody })
      })
    } catch (error) {
      // 转发失败（Go 不可达/拒绝）：请求绝不能落到目标平台，本地中止并记录原因。
      trace.markAborted(entry, 'aborted', String((error as Error)?.message ?? error))
      await route.abort('blockedbyclient')
    }
  }

  await page.route('**/*', handle)
  return {
    trace,
    uninstall: async () => {
      await page.unroute('**/*', handle).catch(() => undefined)
    }
  }
}

// safeHeaders 收集请求头并统一小写键名，转发与回放共用同一形状。
async function safeHeaders (request: import('playwright').Request): Promise<Record<string, string>> {
  try {
    const headers = await request.allHeaders()
    const normalized: Record<string, string> = {}
    for (const [name, value] of Object.entries(headers)) {
      normalized[name.toLowerCase()] = value
    }
    return normalized
  } catch {
    return {}
  }
}
