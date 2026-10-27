// 目标请求路由：page.route('**/*') 全量拦截。必须在 page.goto 之前安装。
// 目标 /web、/flowInstanceApi、文件请求一律转发 Go；未登记外部请求本地拒绝。

import type { Page, Request, Route } from 'playwright'
import { canSendForwardStep } from './completion.ts'
import { RequestTraceRecorder, type TraceEntry } from './request-trace.ts'
import type { ExecutionBaselineRecorder } from './execution-baseline.ts'


// TARGET_PATH_PREFIXES 与 Go 侧 internal/runtimeexec（forward.go）镜像维护：
// 命中任一前缀的请求都视为目标平台请求，必须经过 Go 转发。
export const TARGET_PATH_PREFIXES = ['/web/', '/flowInstanceApi', '/newWorkflowCenter/', '/api/', '/api']

// TARGET_PREFIXES 匹配规则：路径包含目标段即目标请求，与来源主机无关——
// 目标网关主机可能随部署变化，且构建产物可能把绝对网关地址（含 /api 前缀）烘焙进页面，
// 因此匹配按“路径中出现 /web/ 或 /flowInstanceApi”判定，runtime 自身静态资源不含这些段。
export function isTargetRequest (url: string): boolean {
  let pathname: string
  try {
    pathname = new URL(url).pathname
  } catch {
    return false
  }
  return TARGET_PATH_PREFIXES.some(prefix => pathname.includes(prefix))
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
  // baseline 只记录既有转发/回填边界，不增加目标请求，不参与主写或附言的放行判断。
  baseline?: ExecutionBaselineRecorder
  forward: (payload: {
    method: string
    url: string
    requestSeq: number
    headers: Record<string, string>
    body?: string
    bodyEncoding: 'utf8' | 'base64'
    action: string
    writeSequence?: number
  }) => Promise<ForwardOutcome>
  // toolFlowName 非空时，主实例 submit 请求（保存草稿/正式提交）的 data.name 在发出前恢复为工具流程名称，
  // 目标页面脚本拼装的名称不得覆盖工具值（设计 5.2）；其他业务字段保留目标脚本结果。
  toolFlowName?: string
}

export interface TargetRouteHandle {
  trace: RequestTraceRecorder
  uninstall: () => Promise<void>
  // setToolFlowName 在装载后登记工具流程名称；为空串时禁用名称恢复。
  setToolFlowName: (name: string) => void
}

// installTargetRoute 在页面上安装全量路由拦截；返回轨迹与卸载句柄。
// 拦截器贯穿页面整个生命周期，会话关闭时随 Context 一起释放（uninstall 供提前解除）。
export async function installTargetRoute (page: Page, options: TargetRouteOptions): Promise<TargetRouteHandle> {
  const trace = new RequestTraceRecorder()
  const ownOrigins = new Set(options.runtimeOrigins)
  // toolFlowName 由装载链路登记（闭包可变，路由处理器每次发出前读取最新值）。
  let toolFlowName = String(options.toolFlowName ?? '')

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
        console.error('[executor] 拒绝未登记外部请求:', request.url())
        await route.abort('blockedbyclient')
        return
      }
      // runtime 自身脚本、样式与静态资源：正常放行。
      await route.continue()
      return
    }

    // 目标请求：取出正文（可能是上传二进制）后转发 Go。
    const postBuffer = request.postDataBuffer()
    let body = postBuffer ? postBuffer.toString('base64') : undefined
    if (
      body &&
      toolFlowName &&
      request.method() === 'POST' &&
      /\/(web\/flowInstanceApi\/submit|newWorkflowCenter\/flowInstance\/submit)$/.test(new URL(request.url()).pathname)
    ) {
      // 工具流程名称保护：只改写主实例 submit 的 data.name，其余字段原样保留。
      try {
        const parsed = JSON.parse(Buffer.from(body, 'base64').toString('utf8'))
        if (parsed && parsed.data && typeof parsed.data === 'object' && typeof parsed.data.name === 'string') {
          // 只“恢复”页面已写入的名称字段，不给未携带名称的请求（如权限校验）注入新字段。
          parsed.data.name = toolFlowName
          body = Buffer.from(JSON.stringify(parsed), 'utf8').toString('base64')
        }
      } catch {
        // 正文不是 JSON（异常形态）时原样转发，不伪造改写事实。
      }
    }
    const bodyEncoding: 'utf8' | 'base64' = 'base64'
    const headers = await safeHeaders(request)
    const writeSequence = request.method() === 'POST' && request.url().includes('/web/flowInstanceApi/transpond')
      ? 1
      : request.method() === 'POST' && request.url().includes('/web/flowInstancePostscript/save') ? 2 : undefined
    // 检查与登记之间不能 await：并发进入的两笔请求必须看到彼此的占位，防止双写。
    if (writeSequence !== undefined && !canSendForwardStep(trace.list(), writeSequence, { body, bodyEncoding })) {
      const entry = trace.begin({
        method: request.method(),
        url: request.url(),
        headers,
        action: '',
        body,
        bodyEncoding: body ? bodyEncoding : undefined,
        writeSequence
      })
      trace.markAborted(entry, 'blocked', writeSequence === 1
        ? '转发主请求已登记，禁止重复发送'
        : '转发主请求尚未明确成功、附言实例不匹配或附言已登记，禁止发送附言')
      await route.abort('blockedbyclient')
      return
    }
    const entry: TraceEntry = trace.begin({
      method: request.method(),
      url: request.url(),
      headers,
      action: '',
      body,
      bodyEncoding: body ? bodyEncoding : undefined,
      writeSequence
    })
    const responseSpan = options.baseline?.begin('target_response', {
      traceSeq: entry.seq,
      request: { method: entry.method, url: entry.url, headers: entry.headers, body: entry.body, bodyEncoding: entry.bodyEncoding, writeSequence: entry.writeSequence }
    }) ?? null
    let fulfillSpan = null as ReturnType<ExecutionBaselineRecorder['begin']>
    try {
      const payload = {
        method: entry.method,
        url: entry.url,
        requestSeq: entry.seq,
        headers: entry.headers,
        body: entry.body,
        bodyEncoding: entry.bodyEncoding ?? 'utf8',
        action: entry.action,
        writeSequence: entry.writeSequence
      }
      // 同步透传 forward 的原 Promise；计时在原 await 两侧，不额外挂任何异步回调。
      const outcome = await (options.baseline
        ? options.baseline.invoke('forward_invocation', () => options.forward(payload))
        : options.forward(payload))
      trace.markFulfilled(entry, {
        httpStatus: outcome.httpStatus,
        headers: outcome.headers,
        bodyEncoding: outcome.bodyEncoding,
        body: outcome.body,
        completedAt: new Date().toISOString(),
        durationMs: Date.now() - Date.parse(entry.requestAt)
      })
      options.baseline?.end(responseSpan, 'returned', { traceSeq: entry.seq, response: entry.response, boundary: 'full_response_available' })
      const contentType = outcome.headers['content-type'] ?? outcome.headers['Content-Type']
      const fulfillBody = outcome.body === undefined
        ? undefined
        : outcome.bodyEncoding === 'base64' ? Buffer.from(outcome.body, 'base64') : outcome.body
      // 逐跳/长度类响应头必须剔除：Go 出口返回的正文是解压后的原文字节，
      // 若把 content-length/transfer-encoding/content-encoding 原样带给 fulfill，
      // 长度与正文不符会让页面 XHR 直接抛 Network Error，业务回调整条中断
      // （2026-09-16 验收实测：getFormDetail 已 200 但 fmDone 永不触发的根因）。
      const unsafeHeader = (name: string): boolean => {
        const normalized = name.toLowerCase()
        return ['content-length', 'transfer-encoding', 'content-encoding', 'keep-alive', 'connection', 'te', 'trailer', 'upgrade']
          .includes(normalized)
      }
      const fulfillHeaders = Object.fromEntries(
        Object.entries(outcome.headers ?? {}).filter(([name]) => !unsafeHeader(name))
      )
      fulfillSpan = options.baseline?.begin('page_fulfill', { traceSeq: entry.seq }) ?? null
      await route.fulfill({
        status: outcome.httpStatus,
        headers: fulfillHeaders,
        ...(contentType ? {} : { contentType: 'application/octet-stream' }),
        ...(fulfillBody === undefined ? {} : { body: fulfillBody })
      })
      options.baseline?.end(fulfillSpan, 'returned', { traceSeq: entry.seq, boundary: 'route_fulfill_returned' })
    } catch (error) {
      // 转发失败（Go 不可达/拒绝）：请求绝不能落到目标平台，本地中止并记录原因。
      trace.markAborted(entry, 'aborted', String((error as Error)?.message ?? error))
      options.baseline?.end(responseSpan, 'error', { traceSeq: entry.seq, error: entry.abortReason, targetWriteResult: 'unknown' })
      options.baseline?.end(fulfillSpan, 'error', { traceSeq: entry.seq, error: entry.abortReason })
      console.error('[executor] 转发失败中止:', request.url(), '|', String((error as Error)?.message ?? error))
      await route.abort('blockedbyclient')
    }
  }

  await page.route('**/*', handle)
  return {
    trace,
    uninstall: async () => {
      await page.unroute('**/*', handle).catch(() => undefined)
    },
    setToolFlowName: (name: string) => { toolFlowName = String(name ?? '') }
  }
}

// safeHeaders 收集请求头并统一小写键名，转发与回放共用同一形状。
async function safeHeaders (request: Request): Promise<Record<string, string>> {

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
