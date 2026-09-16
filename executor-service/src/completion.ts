// T010 动作完成检测：以请求轨迹、页面异常、通道状态与稳定空闲窗判定动作终局。
// 纪律（docs/s2/01-执行服务设计.md 第 7 节）：
//   - 流程接口成功不能代替全部业务成功（后置请求失败 → business_failed）；
//   - 写响应丢失/连接断开且已写出 → uncertain；
//   - 目标明确拒绝 → target_rejected；未发出写请求即失败 → failed；
//   - 硬超时 → timeout，且超时后不再放行任何新写请求。
import { RequestTraceRecorder, type TraceEntry } from './request-trace.ts'

// FLOW_WRITE_PATHS 是流程主写端点：它们的成败决定主实例是否推进（转发的辅助实例创建同属主写）。
const FLOW_WRITE_PATHS = [
  '/web/flowInstanceApi/submit',
  '/flowInstanceApi/audit',
  '/web/flowInstanceApi/transpond',
  '/web/flowInstanceApi/temporaryStorage',
  '/web/flowInstanceApi/reEdit'
]

export type CompletionVerdict = 'success' | 'target_rejected' | 'uncertain' | 'business_failed' | 'failed' | 'timeout'

export interface CompletionInput {
  action: string
  trace: RequestTraceRecorder
  baselineSeq: number
  businessDone: () => Promise<boolean>
}

export interface CompletionResult {
  status: CompletionVerdict
  message?: string
  requestTrace: TraceSummary[]
}

export interface TraceSummary {
  seq: number
  method: string
  url: string
  state: string
  abortReason?: string
  httpStatus?: number
  responseSeq?: number
  durationMs?: number
}

export interface FlowRejectionInput {
  url: string
  state: string
  response?: { body?: string, bodyEncoding?: string, httpStatus: number }
}

export interface ForwardStepEntry {
  state: string
  url: string
  writeSequence?: number
}

export interface WriteLookup {
  method: string
  url: string
}

export interface AfterFlowLookup {
  seq: number
  url: string
}

// WriteGate 在超时或写结果未知后锁死后续流程写请求，禁止重发。
export class WriteGate {
  private locked = false
  private reason = ''

  lock (reason: string): void {
    this.locked = true
    this.reason = reason
  }

  get isLocked (): boolean {
    return this.locked
  }

  assertOpen (): void {
    if (this.locked) throw new Error(`后续写请求已被拒绝：${this.reason}`)
  }
}

// canSendForwardStep 转发第一步未确认时禁止附言；同一步不得重复发出。
export function canSendForwardStep (entries: ForwardStepEntry[], writeSequence: number): boolean {
  if (writeSequence !== 1 && writeSequence !== 2) return false
  const sameStep = entries.filter(entry => entry.writeSequence === writeSequence || (writeSequence === 1 && entry.url.includes('/web/flowInstanceApi/transpond')))
  if (writeSequence === 1) return sameStep.length === 0
  const first = entries.find(entry => entry.writeSequence === 1 || entry.url.includes('/web/flowInstanceApi/transpond'))
  if (!first || first.state !== 'fulfilled' || detectFlowRejection(first) !== null) return false
  return !entries.some(entry => entry.writeSequence === 2 || entry.url.includes('/web/flowInstancePostscript/save'))
}

// detectFlowRejection 解析一笔流程写请求的响应事实：目标明确拒绝返回拒绝信息，其余返回 null。
export function detectFlowRejection (entry: FlowRejectionInput): string | null {
  const isFlowWrite = FLOW_WRITE_PATHS.some(path => entry.url.includes(path))
  if (!isFlowWrite) return null
  if (entry.state !== 'fulfilled' || !entry.response) return null
  if (entry.response.httpStatus === 401) return 'SID已失效'
  if (entry.response.httpStatus !== 200) return `HTTP ${entry.response.httpStatus}`

  if (!entry.response.body || entry.response.bodyEncoding !== 'base64') return null
  try {
    const parsed = JSON.parse(Buffer.from(entry.response.body, 'base64').toString('utf8'))
    if (parsed && (parsed.code === 'RESP401' || parsed.code === 'AUTH_401' || parsed.message === 'SID已失效')) {
      return 'SID已失效'
    }
    if (parsed && parsed.isSuccess === false) {
      const errorType = parsed.data && parsed.data.errorType
      if (errorType === 'custom_choose' || errorType === 'parallel_choose' || errorType === 'run_node_choose') {
        return `选择要求未闭环：${parsed.message ?? errorType}`
      }
      return parsed.message ?? '目标拒绝'
    }
  } catch {
    return '响应正文无法解析'
  }
  return null
}

function lockIfNeeded (input: CompletionInput, result: CompletionResult): CompletionResult {
  if (result.status === 'timeout' || result.status === 'uncertain') {
    input.writeGate?.lock(result.message ?? result.status)
  }
  return result
}

function isWrite (entry: WriteLookup): boolean {
  if (entry.method !== 'POST') return false
  return FLOW_WRITE_PATHS.some(path => entry.url.includes(path))
}

function isAfterFlow (entries: TraceEntry[], baselineSeq: number, entry: AfterFlowLookup): boolean {
  const flowSeqs = entries
    .filter(item => item.seq > baselineSeq && FLOW_WRITE_PATHS.some(path => item.url.includes(path)) && item.state === 'fulfilled')
    .map(item => item.seq)
  if (flowSeqs.length === 0) return false
  return entry.seq > Math.min(...flowSeqs)
}

function finalize (input: CompletionInput, entries: TraceEntry[], businessDone: boolean): CompletionResult {
  const requestTrace = summarize(entries)
  const writeEntries = entries.filter(entry => isWrite(entry))
  const flowEntries = entries.filter(entry => FLOW_WRITE_PATHS.some(path => entry.url.includes(path)))
  const postEntries = entries.filter(entry => isAfterFlow(input.trace.list(), input.baselineSeq, entry))
  const lostWrite = writeEntries.find(entry => entry.state === 'aborted')
  if (lostWrite) return { status: 'uncertain', message: `写响应丢失：${lostWrite.url}`, requestTrace }
  for (const entry of flowEntries) {
    const rejection = detectFlowRejection(entry)
    if (rejection) return { status: 'target_rejected', message: rejection, requestTrace }
  }
  const lostPost = postEntries.find(entry => entry.state === 'aborted' || entry.state === 'blocked')
  if (lostPost) return { status: 'business_failed', message: `后置业务请求未完成：${lostPost.url}`, requestTrace }
  const hasFlowSuccess = flowEntries.some(entry => entry.state === 'fulfilled' && detectFlowRejection(entry) === null)
  if (hasFlowSuccess && !businessDone) {
    return { status: 'business_failed', message: '流程写请求已受理，但业务完成事件未到达', requestTrace }
  }
  if (writeEntries.length === 0 && !businessDone) {
    return { status: 'failed', message: '动作没有发出任何写请求', requestTrace }
  }
  return { status: 'success', requestTrace }
}

// settleCompletion 等待动作落定并给出终局；轮询实现，稳定窗内无在途请求即收敛。
export async function settleCompletion (input: CompletionInput): Promise<CompletionResult> {
  const startedAt = Date.now()
  let stableSince: number | null = null
  let lastCount = -1
  for (;;) {
    if (input.gate) {
      let open = true
      try { open = await input.gate() } catch { open = true }
      if (!open) {
        stableSince = null
        await new Promise(resolve => setTimeout(resolve, 100))
        continue
      }
    }
    const entries = input.trace.list().filter(entry => entry.seq > input.baselineSeq)
    if (entries.length !== lastCount) {
      lastCount = entries.length
      stableSince = null
    }
    const inFlight = entries.some(entry => entry.state === 'forwarding')
    if (!inFlight) {
      if (stableSince === null) stableSince = Date.now()
      if (Date.now() - stableSince >= input.idleWindowMs) {
        let businessDone = false
        try { businessDone = await input.businessDone() } catch { businessDone = false }
        return lockIfNeeded(input, finalize(input, entries, businessDone))
      }
    } else {
      stableSince = null
    }
    if (Date.now() - startedAt >= input.hardTimeoutMs) {
      return lockIfNeeded(input, { status: 'timeout', message: '动作硬超时，后续写请求已被拒绝', requestTrace: summarize(entries) })
    }
    await new Promise(resolve => setTimeout(resolve, 100))
  }
}

function summarize (entries: TraceEntry[]): TraceSummary[] {
  return entries.map(entry => ({
    seq: entry.seq,
    method: entry.method,
    url: entry.url,
    state: entry.state,
    abortReason: entry.abortReason,
    httpStatus: entry.response?.httpStatus,
    responseSeq: entry.response?.responseSeq,
    durationMs: entry.response?.durationMs
  }))
}
