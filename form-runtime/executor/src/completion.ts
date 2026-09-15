// T011 动作完成检测：以请求轨迹、页面异常、通道状态与稳定空闲窗判定动作终局。
// 纪律（docs/s/01-执行引擎设计.md 第 6 节）：
//   - 流程接口成功不能代替全部业务成功（后置请求失败 → business_failed）；
//   - 写响应丢失/连接断开且已写出 → uncertain；
//   - 目标明确拒绝 → target_rejected；未发出写请求即失败 → failed；
//   - 硬超时 → timeout，且超时后不再放行任何新写请求。
import { RequestTraceRecorder } from './request-trace.ts'

// FLOW_WRITE_PATHS 是流程主写端点：它们的成败决定主实例是否推进（转发的辅助实例创建同属主写）。
const FLOW_WRITE_PATHS = ['/web/flowInstanceApi/submit', '/flowInstanceApi/audit', '/web/flowInstanceApi/reSubmit', '/web/flowInstanceApi/transpond']

// CompletionVerdict 与协议结果七值对齐（本检测器产出其中五种；unsupported 在分派层产生）。
export type CompletionVerdict = 'success' | 'target_rejected' | 'uncertain' | 'business_failed' | 'failed' | 'timeout'

export interface CompletionInput {
  action: string
  trace: RequestTraceRecorder
  // 基线：检测从该序号之后开始统计（之前的请求属于装载或上一个动作）。
  baselineSeq: number
  // 业务完成事件：每轮轮询时从页面读取宿主的完成标志（FlowDialog success / opinion postExamine / 转发弹窗关闭）。
  businessDone: () => Promise<boolean>
  hardTimeoutMs: number
  idleWindowMs: number
  // 选择闭环门（可选）：返回 false 表示目标正在等待选择，完成检测必须暂停落定，
  // 直到继续命令把门打开（设计 3.8：闭环期间动作未结束，检测不得以“被拒绝”提前收口）。
  gate?: () => Promise<boolean>
}

// detectFlowVerdict 解析一笔流程写请求的响应事实：目标明确拒绝返回拒绝信息，其余返回 null。
export function detectFlowRejection (entry: { url: string; state: string; response?: { body?: string; bodyEncoding?: string; httpStatus: number } }): string | null {
  if (!entry.url.includes('/web/flowInstanceApi/submit') && !entry.url.includes('/flowInstanceApi/audit') && !entry.url.includes('/web/flowInstanceApi/reSubmit')) return null
  if (entry.state !== 'fulfilled' || !entry.response) return null
  if (entry.response.httpStatus !== 200) return `HTTP ${entry.response.httpStatus}`
  if (!entry.response.body || entry.response.bodyEncoding !== 'base64') return null
  try {
    const parsed = JSON.parse(Buffer.from(entry.response.body, 'base64').toString('utf8'))
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

// settleCompletion 等待动作落定并给出终局；轮询实现，稳定窗内无在途请求即收敛。
export async function settleCompletion (input: CompletionInput): Promise<{ status: CompletionVerdict; message?: string; requestTrace: Array<Record<string, unknown>> }> {
  const startedAt = Date.now()
  let stableSince: number | null = null
  let lastChangeAt = Date.now()
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
      lastChangeAt = Date.now()
      stableSince = null
    }
    const inFlight = entries.some(entry => entry.state === 'forwarding')
    if (!inFlight) {
      if (stableSince === null) stableSince = Date.now()
      if (Date.now() - stableSince >= input.idleWindowMs) {
        let businessDone = false
        try { businessDone = await input.businessDone() } catch { businessDone = false }
        return finalize(entries, businessDone)
      }
    } else {
      stableSince = null
    }
    if (Date.now() - startedAt >= input.hardTimeoutMs) {
      return { status: 'timeout', message: '动作硬超时，后续写请求已被拒绝', requestTrace: summarize(entries) }
    }
    await new Promise(resolve => setTimeout(resolve, 100))
  }

  // finalize 在网络空闲稳定窗后一次性判定；规则顺序即优先级：
  //   写响应丢失 → uncertain；流程写被目标明确拒绝 → target_rejected；
  //   流程成功但后置请求失败 → business_failed；无写请求且无异常 → success。
  async function finalize (entries: ReturnType<RequestTraceRecorder['list']>, businessDone: boolean) {
    const requestTrace = summarize(entries)
    const writeEntries = entries.filter(entry => isWrite(entry))
    const flowEntries = entries.filter(entry => FLOW_WRITE_PATHS.some(path => entry.url.includes(path)))
    const postEntries = entries.filter(entry => {
      if (!isAfterFlow(entry)) return false
      return true
    })
    // 写请求转发失败（响应丢失/Go 不可达）：写出事实不明，只能待确认。
    const lostWrite = writeEntries.find(entry => entry.state === 'aborted')
    if (lostWrite) return { status: 'uncertain' as CompletionVerdict, message: `写响应丢失：${lostWrite.url}`, requestTrace }
    // 流程写被目标明确拒绝：主实例未推进，标记目标拒绝。
    for (const entry of flowEntries) {
      const rejection = detectFlowRejection(entry)
      if (rejection) return { status: 'target_rejected' as CompletionVerdict, message: rejection, requestTrace }
    }
    // 流程写成功后，后置业务请求失败：business_failed（流程已推进但业务未完成）。
    const lostPost = postEntries.find(entry => entry.state === 'aborted' || entry.state === 'blocked')
    if (lostPost) return { status: 'business_failed' as CompletionVerdict, message: `后置业务请求未完成：${lostPost.url}`, requestTrace }
    // 流程写成功但业务完成事件缺失：宿主链路没有走到自然完成点。
    const hasFlowSuccess = flowEntries.some(entry => entry.state === 'fulfilled' && detectFlowRejection(entry) === null)
    if (hasFlowSuccess && !businessDone) {
      return { status: 'business_failed' as CompletionVerdict, message: '流程写请求已受理，但业务完成事件未到达', requestTrace }
    }
    if (writeEntries.length === 0 && !businessDone) {
      return { status: 'failed' as CompletionVerdict, message: '动作没有发出任何写请求', requestTrace }
    }
    return { status: 'success' as CompletionVerdict, requestTrace }
  }

  // isWrite 判定主写请求：仅流程主写端点（提交/审批/重提/转发主请求）。
  // 附言与业务/附件保存属于后置请求，丢失走 business_failed 而不是 uncertain。
  function isWrite (entry: { method: string; url: string }): boolean {
    if (entry.method !== 'POST') return false
    return FLOW_WRITE_PATHS.some(path => entry.url.includes(path))
  }

  // isAfterFlow 判定后置请求：流程主写 fulfilled 之后登记的请求（附件绑定、业务保存等）。
  function isAfterFlow (entry: { seq: number; url: string }): boolean {
    const flowSeqs = input.trace.list()
      .filter(item => item.seq > input.baselineSeq && FLOW_WRITE_PATHS.some(path => item.url.includes(path)) && item.state === 'fulfilled')
      .map(item => item.seq)
    if (flowSeqs.length === 0) return false
    const firstFlow = Math.min(...flowSeqs)
    return entry.seq > firstFlow
  }
}

// summarize 输出每笔请求的可回放摘要（供协议结果与运行详情）。
function summarize (entries: ReturnType<RequestTraceRecorder['list']>): Array<Record<string, unknown>> {
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
