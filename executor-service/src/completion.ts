// T010 动作完成检测：以动作期望写端点的响应与稳定空闲窗判定动作终局。
// 纪律（docs/s2/01-执行服务设计.md 第 7 节）：
//   - 页面完成信号与页面异常只作证据，不参与终局；
//   - 写响应丢失/连接断开且已写出 → uncertain；
//   - 目标明确拒绝 → target_rejected；未发出写请求即失败 → failed；
//   - 硬超时 → timeout，且超时后不再放行任何新写请求。
import { RequestTraceRecorder, type TraceEntry } from './request-trace.ts'
import { CHOICE_ERRORS } from './native-actions.ts'

// FLOW_WRITE_PATHS 是流程主写端点：它们的成败决定主实例是否推进（转发的辅助实例创建同属主写）。
// 端点真名来自 flow-source/src/api/index.js（全过程 test 8aaaf2a 回退后的事实）：
//   提交与保存草稿都用 saveFlowInstance=/web/flowInstanceApi/submit；
//   重提/审批用 newWorkflowCenter 的两个端点；
//   transpondFlow=/web/flowInstanceApi/transpond、storageFormData=/web/flowInstanceApi/storageFormData。
// 曾被登记的 newWorkflowCenter/flowInstance/submit 已被目标源码回退删除，不再保留。
export const FLOW_WRITE_PATHS = [
  '/web/flowInstanceApi/submit',
  '/newWorkflowCenter/flowInstance/reSubmit',
  '/newWorkflowCenter/flowInstance/flowAudit',
  '/flowInstanceApi/audit',
  '/web/flowInstanceApi/transpond',
  '/web/flowInstanceApi/storageFormData',
  '/web/flowInstanceApi/reSubmit'
]

const ACTION_WRITE_PATH: Record<string, string> = {
  save_draft: '/web/flowInstanceApi/submit',
  submit: '/web/flowInstanceApi/submit',
  storage_form_data: '/web/flowInstanceApi/storageFormData',
  approve: '/newWorkflowCenter/flowInstance/flowAudit',
  reject: '/newWorkflowCenter/flowInstance/flowAudit',
  resubmit: '/newWorkflowCenter/flowInstance/reSubmit',
  forward: '/web/flowInstanceApi/transpond'
}

const FORWARD_POSTSCRIPT_PATH = '/web/flowInstancePostscript/save'

// BUSINESS_WRITE_PATHS 是表单业务组件在流程主写之前使用的真实写端点。
// 这些请求同样有目标副作用，响应失败或丢失时不能再发主流程请求。
export const BUSINESS_WRITE_PATHS = [
  '/web/measuring/api/contractReview/increase',
  '/web/measuring/api/contractReview/modify',
  '/web/measuring/api/contractReview/saveContractRefFile',
  '/web/file/api/relationFile/saveBatch',
  '/web/file/api/relationFile/deleteByRelationIdAndFileIds'
]

// KNOWN_WRITE_PATHS 供服务端调试证据统一筛选，避免业务写被误当成读取。
export const KNOWN_WRITE_PATHS = [...new Set([...FLOW_WRITE_PATHS, ...BUSINESS_WRITE_PATHS, FORWARD_POSTSCRIPT_PATH])]

export type CompletionVerdict = 'success' | 'target_rejected' | 'uncertain' | 'business_failed' | 'failed' | 'timeout'

export interface CompletionInput {
  action: string
  trace: RequestTraceRecorder
  baselineSeq: number
  hardTimeoutMs: number
  idleWindowMs: number
  formValidationError?: string
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
  requestAt: string
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
  response?: { body?: string, bodyEncoding?: string, httpStatus: number }
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

// parseForwardJSON 只解析完整 JSON 对象，缺正文、数组或非法正文都不能冒充成功证据。
function parseForwardJSON (body?: string, bodyEncoding?: string): Record<string, unknown> | null {
  if (!body) return null
  try {
    const value = JSON.parse(bodyEncoding === 'base64' ? Buffer.from(body, 'base64').toString('utf8') : body)
    return value && typeof value === 'object' && !Array.isArray(value) ? value : null
  } catch { return null }
}

// choiceErrorTypeFromResponse 识别目标要求继续选择的明确错误；新工作流端点可能省略 isSuccess，
// 但只要 errorType 属于目标协议的选择错误，就必须进入既有的自动续办闭环，不能当成成功或普通完成。
export function choiceErrorTypeFromResponse (url: string, response: { httpStatus?: number, body?: string, bodyEncoding?: string }): string | null {
  if (!FLOW_WRITE_PATHS.some(path => url.includes(path))) return null
  if (response.httpStatus !== undefined && response.httpStatus !== 200) return null
  const parsed = parseForwardJSON(response.body, response.bodyEncoding)
  if (!parsed || parsed.isSuccess === true) return null
  const data = parsed.data as { errorType?: unknown } | undefined
  const errorType = data?.errorType ?? parsed.errorType
  return typeof errorType === 'string' && (CHOICE_ERRORS as readonly string[]).includes(errorType) ? errorType : null
}

// hasExplicitSuccess 只认可目标明确给出的成功标记；HTTP 200 或未知响应包络都不能单独证明业务写成功。
function hasExplicitSuccess (entry: TraceEntry): boolean {
  if (entry.state !== 'fulfilled' || entry.response?.httpStatus !== 200) return false
  return parseForwardJSON(entry.response.body, entry.response.bodyEncoding)?.isSuccess === true
}

// isBusinessWriteEntry 判定表单业务前置写，必须同时满足 POST 和业务端点白名单。
export function isBusinessWriteEntry (entry: { method?: string, url: string }): boolean {
  if (entry.method !== undefined && entry.method !== 'POST') return false
  return BUSINESS_WRITE_PATHS.some(path => entry.url.includes(path))
}

// responseFailureMessage 保留目标返回的 code 与中文原话，避免执行器再压成泛化错误。
function responseFailureMessage (parsed: Record<string, unknown>, fallback: string): string {
  const code = typeof parsed.code === 'string' ? parsed.code.trim() : ''
  const message = typeof parsed.message === 'string' && parsed.message.trim() ? parsed.message : fallback
  return code ? `${code}：${message}` : message
}

// detectBusinessRejection 解析业务写的明确拒绝；未明确拒绝的异常响应交给 uncertain 判定。
export function detectBusinessRejection (entry: FlowRejectionInput): string | null {
  if (!isBusinessWriteEntry(entry)) return null
  if (entry.state !== 'fulfilled' || !entry.response) return null
  if (entry.response.httpStatus === 401) return 'SID已失效'
  if (entry.response.httpStatus !== 200) return `HTTP ${entry.response.httpStatus}`
  const parsed = parseForwardJSON(entry.response.body, entry.response.bodyEncoding)
  if (!parsed) return null
  if (parsed.code === 'RESP401' || parsed.code === 'AUTH_401' || parsed.message === 'SID已失效') return 'SID已失效'
  if (parsed.isSuccess === false) return responseFailureMessage(parsed, '目标业务拒绝')
  return null
}

// forwardInstanceID 只从明确成功的转发响应取新实例 ID；HTTP 200 本身不能证明成功。
export function forwardInstanceID (entry: ForwardStepEntry | undefined): string {
  if (!entry || entry.state !== 'fulfilled' || entry.response?.httpStatus !== 200) return ''
  const value = parseForwardJSON(entry.response.body, entry.response.bodyEncoding)
  const data = value?.data as { id?: unknown } | undefined
  return value?.isSuccess === true && typeof data?.id === 'string' ? data.id.trim() : ''
}

// postscriptMatchesInstance 确认附言属于这次新实例，允许空文本，但不接受空 ID 或错误容器。
function postscriptMatchesInstance (request: { body?: string, bodyEncoding?: string } | undefined, instanceId: string): boolean {
  if (!request || !instanceId) return false
  const value = parseForwardJSON(request.body, request.bodyEncoding)
  const data = value?.data as { flowInstanceId?: unknown, id?: unknown, text?: unknown } | undefined
  return data?.flowInstanceId === instanceId && typeof data.id === 'string' && data.id.trim() !== '' && typeof data.text === 'string'
}

// canSendForwardStep 两笔写各放行一次；第二笔必须晚于明确成功且指向刚创建的新实例。
export function canSendForwardStep (entries: ForwardStepEntry[], writeSequence: number, request?: { body?: string, bodyEncoding?: string }): boolean {
  if (writeSequence !== 1 && writeSequence !== 2) return false
  const sameStep = entries.filter(entry => entry.writeSequence === writeSequence || (writeSequence === 1 && entry.url.includes('/web/flowInstanceApi/transpond')))
  if (writeSequence === 1) return sameStep.length === 0
  const first = entries.find(entry => entry.writeSequence === 1 || entry.url.includes('/web/flowInstanceApi/transpond'))
  if (!postscriptMatchesInstance(request, forwardInstanceID(first))) return false
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
    const choiceErrorType = choiceErrorTypeFromResponse(entry.url, entry.response)
    if (choiceErrorType) {
      return `选择要求未闭环：${parsed?.message ?? choiceErrorType}`
    }
    if (parsed && parsed.isSuccess === false) {
      return responseFailureMessage(parsed, '目标拒绝')
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
  // 转发一旦收口失败也禁止迟到的回调继续写，尤其不能在“附言未发出”之后又补发。
  if (result.status === 'timeout' || result.status === 'uncertain' || (input.action === 'forward' && result.status !== 'success')) {
    input.writeGate?.lock(result.message ?? result.status)
  }
  return result
}

// finalizeForward 只用两笔完整响应判定转发，不把原主实例未推进当成没有副作用。
function finalizeForward (entries: TraceEntry[], mainEntries: TraceEntry[], requestTrace: TraceSummary[]): CompletionResult {
  const result = (status: CompletionVerdict, message: string): CompletionResult => ({ status, message, requestTrace })
  const main = mainEntries[0]!
  if (main.state === 'blocked') return result('failed', `转发主请求被本地阻止：${main.abortReason ?? main.url}`)
  if (main.state !== 'fulfilled' || !main.response) return result('uncertain', '转发主请求响应丢失，结果待确认；禁止重发主请求')
  const mainEnvelope = parseForwardJSON(main.response.body, main.response.bodyEncoding)
  const rejection = detectFlowRejection(main)
  if (main.response.httpStatus === 401 || mainEnvelope?.isSuccess === false || mainEnvelope?.code === 'RESP401' || mainEnvelope?.code === 'AUTH_401') {
    return result('target_rejected', rejection ?? String(mainEnvelope?.message ?? '目标拒绝转发'))
  }
  if (main.response.httpStatus !== 200 || mainEnvelope?.isSuccess !== true) {
    return result('uncertain', '转发响应没有完整、明确的成功标记，结果待确认；禁止重发主请求')
  }
  const instanceId = forwardInstanceID(main)
  if (!instanceId) return result('business_failed', '转发主请求已成功，但没有返回新实例 ID，不能安全保存附言')
  const postscript = entries.filter(entry => entry.method === 'POST' && entry.url.includes(FORWARD_POSTSCRIPT_PATH))
  // fuyan 控制历史数据；目标源码无论勾选与否，都保存本次附言（包括空文本）。
  if (postscript.length === 0) return result('business_failed', '转发主请求已成功，但附言请求没有发出；禁止重发主请求')
  if (mainEntries.length !== 1 || postscript.length !== 1 || postscript[0]!.seq <= main.seq || !postscriptMatchesInstance(postscript[0], instanceId)) {
    return result('business_failed', '转发主请求已成功，但附言实例或请求顺序不正确；禁止重发主请求')
  }
  const saved = postscript[0]!
  if (saved.state === 'blocked') return result('business_failed', `转发主请求已成功，但附言被本地阻止：${saved.abortReason ?? saved.url}`)
  if (saved.state !== 'fulfilled' || !saved.response) return result('uncertain', '转发主请求已成功，附言响应丢失，保存结果待确认；禁止重发主请求')
  const savedEnvelope = parseForwardJSON(saved.response.body, saved.response.bodyEncoding)
  if (saved.response.httpStatus === 401 || savedEnvelope?.isSuccess === false || savedEnvelope?.code === 'RESP401' || savedEnvelope?.code === 'AUTH_401') {
    return result('business_failed', `转发主请求已成功，但附言保存失败：${String(savedEnvelope?.message ?? `HTTP ${saved.response.httpStatus}`)}；禁止重发主请求`)
  }
  if (saved.response.httpStatus !== 200 || savedEnvelope?.isSuccess !== true) return result('uncertain', '转发主请求已成功，附言缺少完整、明确的成功响应，保存结果待确认；禁止重发主请求')
  return { status: 'success', requestTrace }
}

// isFlowWriteEntry 判定一笔请求是否为真实流程主写：端点命中且不是权限校验型 submit。
function isFlowWriteEntry (entry: { method?: string, url: string, body?: string, bodyEncoding?: string }): boolean {
  if (entry.method !== undefined && entry.method !== 'POST') return false
  if (!FLOW_WRITE_PATHS.some(path => entry.url.includes(path))) return false
  return !isPermissionCheckSubmit(entry)
}

// isKnownWriteEntry 汇总流程、业务和转发附言写，供完成检测避免把业务副作用丢掉。
function isKnownWriteEntry (entry: { method?: string, url: string, body?: string, bodyEncoding?: string }): boolean {
  return isFlowWriteEntry(entry) || isBusinessWriteEntry(entry) || entry.url.includes(FORWARD_POSTSCRIPT_PATH)
}

function finalize (input: CompletionInput, entries: TraceEntry[]): CompletionResult {
  const requestTrace = summarize(entries)
  const expectedPath = ACTION_WRITE_PATH[input.action]
  const allWriteEntries = entries.filter(entry => entry.method === 'POST' && isKnownWriteEntry(entry))
  const businessEntries = entries.filter(entry => entry.method === 'POST' && isBusinessWriteEntry(entry))
  const expectedEntries = expectedPath
    ? entries.filter(entry => entry.method === 'POST' && entry.url.includes(expectedPath) && !isPermissionCheckSubmit(entry))
    : []
  if (!expectedPath) return { status: 'failed', message: `动作 ${input.action} 没有登记期望写端点`, requestTrace }
  // 前置业务事实优先于流程主写事实：即使迟到的主写已经出现，也不能覆盖合同保存失败。
  const businessRejected = businessEntries.map(detectBusinessRejection).find(Boolean)
  if (businessRejected) {
    return { status: 'business_failed', message: `表单业务处理失败：${businessRejected}`, requestTrace }
  }
  const businessLost = businessEntries.find(entry => entry.state === 'aborted')
  if (businessLost) {
    return { status: 'uncertain', message: `表单业务写响应丢失：${businessLost.url}；主流程禁止继续或重发`, requestTrace }
  }
  const businessBlocked = businessEntries.find(entry => entry.state === 'blocked')
  if (businessBlocked) {
    return { status: 'failed', message: `表单业务写请求被本地阻止：${businessBlocked.abortReason ?? businessBlocked.url}`, requestTrace }
  }
  const businessUnknown = businessEntries.find(entry => !hasExplicitSuccess(entry))
  if (businessUnknown) {
    return { status: 'uncertain', message: `表单业务写响应缺少明确成功标记：${businessUnknown.url}；主流程禁止继续或重发`, requestTrace }
  }
  if (expectedEntries.length === 0) {
    if (input.formValidationError) {
      return { status: 'failed', message: `表单数据缺失无法发起请求，请检查表单数据（${input.formValidationError}）`, requestTrace }
    }
    if (allWriteEntries.length > 0) {
      if (businessEntries.length > 0) {
        return { status: 'business_failed', message: `表单业务已完成，但未发出期望流程写端点 ${expectedPath}，已阻止流程继续`, requestTrace }
      }
      return { status: 'failed', message: `动作未调用期望写端点 ${expectedPath}，实际调用了 ${allWriteEntries.map(entry => entry.url).join('、')}`, requestTrace }
    }
    return { status: 'failed', message: `动作没有发出期望写请求 ${expectedPath}`, requestTrace }
  }
  if (input.action === 'forward') return finalizeForward(entries, expectedEntries, requestTrace)
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
      expectedEntries.some(later => later.seq > entry.seq && hasExplicitSuccess(later))
    if (superseded) continue
    return { status: 'target_rejected', message: rejection, requestTrace }
  }
  const hasExpectedSuccess = expectedEntries.some(hasExplicitSuccess)
  if (!hasExpectedSuccess) {
    return { status: 'failed', message: `期望写端点 ${expectedPath} 没有成功响应`, requestTrace }
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
    requestAt: entry.requestAt,
    state: entry.state,
    abortReason: entry.abortReason,
    httpStatus: entry.response?.httpStatus,
    responseSeq: entry.response?.responseSeq,
    durationMs: entry.response?.durationMs
  }))
}
