// 执行基线只旁观已有调用。函数返回、HTTP 正文到达和内部业务完成是不同事实，不能互相代替。
export interface BaselineSpan {
  spanId: number
  name: string
  startedAt: number | null
}

export interface BaselineRecord {
  type: 'execution_baseline'
  version: 1
  sequence: number
  at: string
  context: Record<string, unknown>
  name: string
  phase: 'begin' | 'end' | 'checkpoint'
  spanId?: number
  durationMs: number | null
  outcome?: 'returned' | 'error'
  data?: unknown
  pending?: Array<{ spanId: number, name: string }>
  unknowns: string[]
}

export interface BaselineOptions {
  enabled?: boolean
  sink?: (record: BaselineRecord) => void
  now?: () => number
  maxActiveSpans?: number
}

// 这些边界当前没有新增读取或目标内部埋点；记录到返回也不能宣称它们已完成。
const UNKNOWN_BOUNDARIES = [
  'form_async_completion', 'business_callback_completion',
  'target_fact_verification', 'deployed_source_version'
]

// describeBaselineError 不读取对象属性或转换钩子；原业务日志/响应负责详细错误，诊断不能改写它。
export function describeBaselineError (error: unknown): string {
  if (error !== null && (typeof error === 'object' || typeof error === 'function')) {
    return '业务错误对象（内容未额外读取，详情见原业务响应/日志）'
  }
  return String(error)
}

// ExecutionBaselineRecorder 保存有界的未结束步骤，把诊断故障与业务返回/异常完全分开。
export class ExecutionBaselineRecorder {
  private context: Record<string, unknown>
  private readonly enabled: boolean
  private readonly sink: (record: BaselineRecord) => void
  private readonly now: () => number
  private readonly maxActiveSpans: number
  private readonly active = new Map<number, { span: BaselineSpan, context: Record<string, unknown> }>()
  private nextSpanId = 1
  private sequence = 0
  private sinkFailures = 0
  private droppedSpans = 0

  constructor (context: Record<string, unknown>, options: BaselineOptions = {}) {
    this.context = { ...context }
    this.enabled = options.enabled ?? false
    this.sink = options.sink ?? (record => { console.log(JSON.stringify(record)) })
    this.now = options.now ?? (() => performance.now())
    const limit = options.maxActiveSpans ?? 256
    this.maxActiveSpans = Number.isFinite(limit) ? Math.max(1, Math.floor(limit)) : 256
  }

  // setContext 只保存调用者已经持有的身份/实例信息，不为补齐归属发任何请求。
  setContext (values: Record<string, unknown>): void {
    if (!this.enabled) return
    try { this.context = { ...this.context, ...values } } catch { this.sinkFailures++ }
  }

  // begin 记录可见调用开始；超过上限只报告丢弃，绝不拒绝或延迟原业务。
  begin (name: string, data?: unknown): BaselineSpan | null {
    if (!this.enabled) return null
    if (this.active.size >= this.maxActiveSpans) {
      this.droppedSpans++
      return null
    }
    const span = { spanId: this.nextSpanId++, name, startedAt: this.time() }
    // 请求可能晚于下一条命令返回，结束记录必须沿用开始时身份，不能跟随当前上下文串台。
    this.active.set(span.spanId, { span, context: { ...this.context } })
    this.emit({ name, phase: 'begin', spanId: span.spanId, durationMs: null, data })
    return span
  }

  // end 只记当前边界返回/抛错，不能推断原生脚本、后置业务或目标数据已经核对完。
  end (span: BaselineSpan | null, outcome: 'returned' | 'error', data?: unknown): void {
    if (!span) return
    const started = this.active.get(span.spanId)
    if (!started || started.span !== span) return
    this.active.delete(span.spanId)
    const endedAt = this.time()
    const durationMs = span.startedAt === null || endedAt === null || endedAt < span.startedAt
      ? null : endedAt - span.startedAt
    this.emit({ name: span.name, phase: 'end', spanId: span.spanId, durationMs, outcome, data, context: started.context })
  }

  // invoke 同步透传已有调用；不 await、不接 then/finally，保持原 Promise 对象和微任务顺序。
  invoke<T> (name: string, call: () => T, data?: unknown): T {
    if (!this.enabled) return call()
    const span = this.begin(name, data)
    try {
      const returned = call()
      this.end(span, 'returned', { boundary: 'function_return_only' })
      return returned
    } catch (error) {
      this.end(span, 'error', { error: describeBaselineError(error), boundary: 'function_throw' })
      throw error
    }
  }

  // checkpoint 如实列出仍未结束的步骤；没有计时器，也不取消、补发或改变放行。
  checkpoint (name: string, data?: unknown): void {
    if (!this.enabled) return
    this.emit({
      name, phase: 'checkpoint', durationMs: null, data,
      pending: [...this.active.values()].map(({ span: { spanId, name } }) => ({ spanId, name }))
    })
  }

  // stats 返回记录是否完整的计数，不能把丢记录或写日志失败解释为业务成功。
  stats (): { records: number, sinkFailures: number, droppedSpans: number, activeSpans: number } {
    return { records: this.sequence, sinkFailures: this.sinkFailures, droppedSpans: this.droppedSpans, activeSpans: this.active.size }
  }

  // time 使用单调时钟；时钟失败或值无效时用未知，不伪造零耗时。
  private time (): number | null {
    try {
      const value = this.now()
      return Number.isFinite(value) ? value : null
    } catch { return null }
  }

  // emit 的编码/输出失败只影响诊断完整性；不穿透业务调用，也不新增业务事件。
  private emit (fields: Pick<BaselineRecord, 'name' | 'phase' | 'durationMs'> & Partial<BaselineRecord>): void {
    try {
      this.sink({
        type: 'execution_baseline', version: 1, ...fields,
        sequence: ++this.sequence, at: new Date().toISOString(),
        context: { ...(fields.context ?? this.context) }, unknowns: [...UNKNOWN_BOUNDARIES]
      })
    } catch { this.sinkFailures++ }
  }
}
