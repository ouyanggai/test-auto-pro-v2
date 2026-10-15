import type { RunRequestItem, RunStep } from './api'

// ActionRequestItem 只属于当前动作的一次尝试；网络记录和页面轨迹共用一行展示。
export interface ActionRequestItem extends RunRequestItem {
  key: string
  attemptNo: number
  evidence: 'network' | 'trace'
  evidenceNote?: string
}

// endpointKey 消除两种出口的固定网关前缀差异；查询参数不参与关联，也不进入界面。
function endpointKey(path: string): string {
  return path.split('?')[0]!.replace(/^\/?(?:api\/)?(?:web\/)?/, '')
}

// mergeActionRequests 用尝试内的序号、方法和端点对应同一请求，绝不按相似耗时或相邻时间猜测。
// 一次 SID 刷新会有两笔物理传输，均保留；页面轨迹只补缺失事实，不能代替网络传输计时。
export function mergeActionRequests(step: RunStep | null): ActionRequestItem[] {
  if (!step) return []
  return [...step.attempts].sort((a, b) => a.attemptNo - b.attemptNo).flatMap(attempt => {
    const network = attempt.requests ?? []
    const rows: ActionRequestItem[] = network.map((request, index) => ({
      ...request,
      key: `${attempt.attemptNo}-network-${index}`,
      attemptNo: attempt.attemptNo,
      evidence: 'network',
    }))
    for (const request of attempt.runtimeDetail?.requests ?? []) {
      const recorded = network.some(item => item.requestSeq === request.seq
        && item.method === request.method && endpointKey(item.endpoint) === endpointKey(request.path))
      if (recorded) continue
      const blocked = request.state === 'blocked'
      const evidenceNote = blocked ? '本地拦截，未发送到目标平台'
        : request.state === 'fulfilled' ? '请求已发生，暂无真实请求记录'
          : request.state === 'aborted' ? '转发中止，暂无真实请求记录，无法确认目标是否收到'
            : '转发已开始，暂无真实请求记录'
      rows.push({
        key: `${attempt.attemptNo}-trace-${request.seq}`,
        attemptNo: attempt.attemptNo,
        evidence: 'trace',
        evidenceNote,
        method: request.method,
        requestSeq: request.seq,
        requestClass: request.requestClass,
        endpoint: request.path,
        durationMs: 0,
        durationKnown: false,
        statusCode: request.httpStatus ?? 0,
        result: blocked || request.state === 'aborted' ? 'failure' : '',
        resultSummary: request.abortReason || evidenceNote,
        retryAttempt: 1,
        at: request.requestAt,
      })
    }
    return rows.sort((a, b) => {
      const left = Date.parse(a.at ?? '')
      const right = Date.parse(b.at ?? '')
      if (Number.isFinite(left) && Number.isFinite(right)) {
        // 页面时间只有毫秒精度，同一毫秒内还要用真实请求序号避免补缺条目被排到末尾。
        return left - right || (a.requestSeq && b.requestSeq ? a.requestSeq - b.requestSeq || a.retryAttempt - b.retryAttempt : 0)
      }
      // 缺少时间时仍可按页面记录的真实序号排序，每次实际重试独立显示。
      if (!Number.isFinite(left) && !Number.isFinite(right) && a.requestSeq && b.requestSeq) return a.requestSeq - b.requestSeq || a.retryAttempt - b.retryAttempt
      return Number.isFinite(left) ? -1 : Number.isFinite(right) ? 1 : 0
    })
  })
}

// actionRequestEmptyMessage 只在证据完整且动作没有请求时使用“未发出”，缺失记录必须明说。
export function actionRequestEmptyMessage(step: RunStep | null, logAvailable: boolean): string {
  if (!step) return ''
  if (logAvailable) return '这一步没有发出接口请求'
  return '暂无真实请求记录：日志缺失、无法完整读取或缺少动作归属，无法判断这一步是否发出了请求'
}
