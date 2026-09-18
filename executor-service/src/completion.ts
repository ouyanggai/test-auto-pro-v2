// T010 动作完成检测：以动作期望写端点的响应与稳定空闲窗判定动作终局。
// 纪律（docs/s2/01-执行服务设计.md 第 7 节）：
//   - 页面完成信号与页面异常只作证据，不参与终局；
//   - 写响应丢失/连接断开且已写出 → uncertain；
//   - 目标明确拒绝 → target_rejected；未发出写请求即失败 → failed；
//   - 硬超时 → timeout，且超时后不再放行任何新写请求。
import { RequestTraceRecorder, type TraceEntry } from './request-trace.ts'

// FLOW_WRITE_PATHS 是流程主写端点：它们的成败决定主实例是否推进（转发的辅助实例创建同属主写）。
// 端点真名来自 flow-source/src/api/index.js（P1-7 修正）：
//   saveFlowInstance=/web/flowInstanceApi/submit、submitTask=/flowInstanceApi/audit（无 /web 前缀）、
//   transpondFlow=/web/flowInstanceApi/transpond、storageFormData=/web/flowInstanceApi/storageFormData、
//   saveFlowInstanceAgain=/web/flowInstanceApi/reSubmit。
// 曾被错写的 temporaryStorage/reEdit 端点在目标源码中不存在，已删除。
const FLOW_WRITE_PATHS = [
  '/web/flowInstanceApi/submit',
  '/flowInstanceApi/audit',
  '/web/flowInstanceApi/transpond',
  '/web/flowInstanceApi/storageFormData',
  '/web/flowInstanceApi/reSubmit'
]

const ACTION_WRITE_PATH: Record<string, string> = {
  save_draft: '/web/flowInstanceApi/submit',
  submit: '/web/flowInstanceApi/submit',
  storage_form_data: '/web/flowInstanceApi/storageFormData',
  approve: '/flowInstanceApi/audit',
  reject: '/flowInstanceApi/audit',
  resubmit: '/web/flowInstanceApi/reSubmit',
  forward: '/web/flowInstanceApi/transpond'
}

const FORWARD_POSTSCRIPT_PATH = '/web/flowInstancePostscript/save'

export type CompletionVerdict = 'success' | 'target_rejected' | 'uncertain' | 'business_failed' | 'failed' | 'timeout'

export interface CompletionInput {
  action: string
  trace: RequestTraceRecorder
  baselineSeq: number
  hardTimeoutMs: number
  idleWindowMs: number
  forwardPostscriptRequired?: boolean
  gate?: () => Promise<boolean>
  writeGate?: WriteGate
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

  if (!entry.response.body) return null
  try {
    const text = entry.response.bodyEncoding === 'base64'
      ? Buffer.from(entry.response.body, 'base64').toString('utf8')
      : entry.response.body
    const parsed = JSON.parse(text)
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

// isPermissionCheckSubmit 识别权限校验型 POST /web/flowInstanceApi/submit：
// 重提链的 checkFlowPermission（EnterpriseExamineOpinion checkFlowPermission，data.checkPermissions='first'）
// 与 FlowDialog 打开时的前置校验都走 submit 端点，但它们不是流程写；只看 URL+方法会把
// 「权限校验」误判成「流程已写」（施工令 §3 附加项）。判定必须精确到 data 顶层的
// checkPermissions 键：FormMaking 表单正文（formDataMongoVo）可能携带含同名子串的
// 历史数据值（2026-09-19 夜间调休单实测），裸子串匹配会把真实草稿写误判成权限探测，
// 导致「零写失败」误判而目标上实际已创建草稿实例。
export function isPermissionCheckSubmit (entry: { url: string, body?: string, bodyEncoding?: string }): boolean {
  if (!entry.url.includes('/web/flowInstanceApi/submit')) return false
  let text = entry.body ?? ''
  if (entry.bodyEncoding === 'base64') text = Buffer.from(text, 'base64').toString('utf8')
  if (text.trim() === '') return false
  try {
    const parsed = JSON.parse(text) as { data?: Record<string, unknown> }
    const data = parsed?.data
    return !!data && typeof data === 'object' && Object.prototype.hasOwnProperty.call(data, 'checkPermissions')
  } catch {
    // 非 JSON 正文不属于页面构造的权限探测形状，按真实写对待（宁可靠拢到写侧保守处理）。
    return false
  }
}

function lockIfNeeded (input: CompletionInput, result: CompletionResult): CompletionResult {
  if (result.status === 'timeout' || result.status === 'uncertain') {
    input.writeGate?.lock(result.message ?? result.status)
  }
  return result
}

// isFlowWriteEntry 判定一笔请求是否为真实流程主写：端点命中且不是权限校验型 submit。
function isFlowWriteEntry (entry: { method?: string, url: string, body?: string, bodyEncoding?: string }): boolean {
  if (entry.method !== undefined && entry.method !== 'POST') return false
  if (!FLOW_WRITE_PATHS.some(path => entry.url.includes(path))) return false
  return !isPermissionCheckSubmit(entry)
}

function finalize (input: CompletionInput, entries: TraceEntry[]): CompletionResult {
  const requestTrace = summarize(entries)
  const expectedPath = ACTION_WRITE_PATH[input.action]
  const allWriteEntries = entries.filter(entry => entry.method === 'POST' && (isFlowWriteEntry(entry) || entry.url.includes(FORWARD_POSTSCRIPT_PATH)))
  const expectedEntries = expectedPath
    ? entries.filter(entry => entry.method === 'POST' && entry.url.includes(expectedPath) && !isPermissionCheckSubmit(entry))
    : []
  if (!expectedPath) return { status: 'failed', message: `动作 ${input.action} 没有登记期望写端点`, requestTrace }
  if (expectedEntries.length === 0) {
    if (allWriteEntries.length > 0) {
      return { status: 'failed', message: `动作未调用期望写端点 ${expectedPath}，实际调用了 ${allWriteEntries.map(entry => entry.url).join('、')}`, requestTrace }
    }
    return { status: 'failed', message: `动作没有发出期望写请求 ${expectedPath}`, requestTrace }
  }
  const lostWrite = expectedEntries.find(entry => entry.state === 'aborted')
  if (lostWrite) return { status: 'uncertain', message: `写响应丢失：${lostWrite.url}`, requestTrace }
  const blockedWrite = expectedEntries.find(entry => entry.state === 'blocked')
  if (blockedWrite) return { status: 'failed', message: `期望写请求被本地阻止：${blockedWrite.abortReason ?? blockedWrite.url}`, requestTrace }
  for (const entry of expectedEntries) {
    const rejection = detectFlowRejection(entry)
    if (!rejection) continue
    // 选择要求类拒绝可以被「继续一次」后的后续成功写覆盖（S6 闭环：目标以
    // isSuccess=false 要求选择，选择继续后重新提交成功）。此时早先的选择要求
    // 不是终局；其余拒绝（SID 失效、HTTP 错误、业务拒绝）仍然是终局。
    const superseded = rejection.startsWith('选择要求未闭环') &&
      expectedEntries.some(later => later.seq > entry.seq && later.state === 'fulfilled' && later.response?.httpStatus === 200 && detectFlowRejection(later) === null)
    if (superseded) continue
    return { status: 'target_rejected', message: rejection, requestTrace }
  }
  const hasExpectedSuccess = expectedEntries.some(entry => entry.state === 'fulfilled' && entry.response?.httpStatus === 200 && detectFlowRejection(entry) === null)
  if (!hasExpectedSuccess) {
    return { status: 'failed', message: `期望写端点 ${expectedPath} 没有成功响应`, requestTrace }
  }
  if (input.action === 'forward') {
    const postscript = entries.filter(entry => entry.method === 'POST' && entry.url.includes(FORWARD_POSTSCRIPT_PATH))
    if (input.forwardPostscriptRequired && postscript.length === 0) {
      return { status: 'business_failed', message: '转发主请求已成功，但附言请求没有发出', requestTrace }
    }
    const failedPostscript = postscript.find(entry => entry.state !== 'fulfilled' || entry.response?.httpStatus !== 200 || detectEnvelopeRejection(entry) !== null)
    if (failedPostscript) return { status: 'business_failed', message: `转发主请求已成功，但附言请求未完成：${failedPostscript.url}`, requestTrace }
  }
  return { status: 'success', requestTrace }
}

// detectEnvelopeRejection 解析非流程端点的统一响应信封，供转发附言判定使用。
function detectEnvelopeRejection (entry: TraceEntry): string | null {
  if (entry.state !== 'fulfilled' || !entry.response) return '响应未完成'
  if (entry.response.httpStatus !== 200) return `HTTP ${entry.response.httpStatus}`
  if (!entry.response.body) return null
  try {
    const text = entry.response.bodyEncoding === 'base64' ? Buffer.from(entry.response.body, 'base64').toString('utf8') : entry.response.body
    const parsed = JSON.parse(text)
    return parsed?.isSuccess === false ? String(parsed.message ?? '目标拒绝') : null
  } catch { return '响应正文无法解析' }
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
        return lockIfNeeded(input, finalize(input, entries))
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
