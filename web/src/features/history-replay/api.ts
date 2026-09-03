import type { HistoryCandidatePage, HistoryDataSource, HistoryReplayItemPage, HistoryReplayJob, HistorySourceMode } from './types'

interface ApiSuccess<T> {
  success: true
  data: T
}
interface ApiFailure {
  success: false
  error: {
    code: string
    message: string
    retryable: boolean
  }
}

export class HistoryDataApiError extends Error {
  readonly code: string
  readonly retryable: boolean

  constructor(message: string, code = 'HISTORY_STORAGE_UNAVAILABLE', retryable = false) {
    super(message)
    this.name = 'HistoryDataApiError'
    this.code = code
    this.retryable = retryable
  }
}

// fetchHistoryCandidates 读取同计划原始历史摘要和当前来源上下文。
export function fetchHistoryCandidates(planId: string, options: {
  pathId?: string
  query?: string
  page?: number
  pageSize?: number
  signal?: AbortSignal
} = {}): Promise<HistoryCandidatePage> {
  const search = new URLSearchParams()
  if (options.pathId) search.set('pathId', options.pathId)
  if (options.query?.trim()) search.set('query', options.query.trim())
  search.set('page', String(options.page ?? 1))
  search.set('pageSize', String(options.pageSize ?? 20))
  return request<HistoryCandidatePage>(
    `/api/plans/${encodeURIComponent(planId)}/history-data/candidates?${search}`,
    { method: 'GET' },
    options.signal,
  )
}

// saveDefaultHistorySource 创建或替换计划默认历史快照。
export function saveDefaultHistorySource(planId: string, candidateKey: string, revision: number, idempotencyKey: string): Promise<HistoryDataSource> {
  return request<HistoryDataSource>(`/api/plans/${encodeURIComponent(planId)}/history-data/default`, {
    method: 'PUT',
    headers: { 'Idempotency-Key': idempotencyKey },
    body: JSON.stringify({ candidateKey, revision }),
  })
}

// savePathHistorySource 设置路径动态继承默认来源或使用独立历史快照。
export function savePathHistorySource(
  planId: string,
  pathId: string,
  mode: Extract<HistorySourceMode, 'default' | 'override'>,
  candidateKey: string,
  revision: number,
  idempotencyKey: string,
): Promise<HistoryDataSource> {
  return request<HistoryDataSource>(`/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(pathId)}/configuration/data/source`, {
    method: 'PUT',
    headers: { 'Idempotency-Key': idempotencyKey },
    body: JSON.stringify({ mode, candidateKey: mode === 'override' ? candidateKey : undefined, revision }),
  })
}

// createHistoryReplay 只提交用户明确勾选的路径 ID 和来源修订，不提交目标正文或派生状态。
export function createHistoryReplay(planId: string, pathIds: number[], revision: number, idempotencyKey: string): Promise<HistoryReplayJob> {
  return requestReplay<HistoryReplayJob>(`/api/plans/${encodeURIComponent(planId)}/history-replays`, {
    method: 'POST',
    headers: { 'Idempotency-Key': idempotencyKey },
    body: JSON.stringify({ pathIds, revision }),
  })
}

// fetchActiveHistoryReplay 恢复刷新页面前仍处于排队或运行中的唯一任务。
export function fetchActiveHistoryReplay(planId: string, signal?: AbortSignal): Promise<HistoryReplayJob | null> {
  return requestReplay<HistoryReplayJob | null>(`/api/plans/${encodeURIComponent(planId)}/history-replays/active`, { method: 'GET' }, signal)
}

// fetchHistoryReplay 读取任务真实聚合状态，供轮询和取消后显示检查点。
export function fetchHistoryReplay(planId: string, jobId: string, signal?: AbortSignal): Promise<HistoryReplayJob> {
  return requestReplay<HistoryReplayJob>(`/api/plans/${encodeURIComponent(planId)}/history-replays/${encodeURIComponent(jobId)}`, { method: 'GET' }, signal)
}

// fetchHistoryReplayItems 按明细 ID 游标分页读取路径终态和结构化问题。
export function fetchHistoryReplayItems(planId: string, jobId: string, cursor = 0, limit = 20, signal?: AbortSignal): Promise<HistoryReplayItemPage> {
  const query = new URLSearchParams({ cursor: String(cursor), limit: String(limit) })
  return requestReplay<HistoryReplayItemPage>(`/api/plans/${encodeURIComponent(planId)}/history-replays/${encodeURIComponent(jobId)}/items?${query}`, { method: 'GET' }, signal)
}

// cancelHistoryReplay 取消任务并保留已经完成的路径检查点。
export function cancelHistoryReplay(planId: string, jobId: string): Promise<HistoryReplayJob> {
  return requestReplay<HistoryReplayJob>(`/api/plans/${encodeURIComponent(planId)}/history-replays/${encodeURIComponent(jobId)}/cancel`, { method: 'POST' })
}

// resumeHistoryReplay 从取消或失败任务的未完成检查点继续回放。
export function resumeHistoryReplay(planId: string, jobId: string): Promise<HistoryReplayJob> {
  return requestReplay<HistoryReplayJob>(`/api/plans/${encodeURIComponent(planId)}/history-replays/${encodeURIComponent(jobId)}/resume`, { method: 'POST' })
}

// request 统一解析历史数据接口错误，不记录响应正文或候选键。
async function request<T>(path: string, init: RequestInit, signal?: AbortSignal): Promise<T> {
  let response: Response
  try {
    response = await fetch(path, {
      ...init,
      signal,
      headers: { 'Content-Type': 'application/json', ...init.headers },
    })
  }
  catch (caught) {
    if (signal?.aborted) throw caught
    throw new HistoryDataApiError('业务数据服务暂不可用，请重试', 'HISTORY_STORAGE_UNAVAILABLE', true)
  }
  let envelope: ApiSuccess<T> | ApiFailure
  try {
    envelope = await response.json() as ApiSuccess<T> | ApiFailure
  }
  catch {
    throw new HistoryDataApiError('业务数据响应格式异常，请重试', 'HISTORY_STORAGE_UNAVAILABLE', true)
  }
  if (!response.ok || !envelope.success) {
    const failure = envelope as ApiFailure
    throw new HistoryDataApiError(
      failure.error?.message || '业务数据操作失败，请重试',
      failure.error?.code,
      failure.error?.retryable,
    )
  }
  return envelope.data
}

export class HistoryReplayApiError extends Error {
  readonly code: string
  readonly retryable: boolean

  constructor(message: string, code = 'HISTORY_REPLAY_STORAGE_UNAVAILABLE', retryable = false) {
    super(message)
    this.name = 'HistoryReplayApiError'
    this.code = code
    this.retryable = retryable
  }
}

// requestReplay 解析回放任务错误并保留后端的稳定错误码，避免把任务失败伪装成路径保存失败。
async function requestReplay<T>(path: string, init: RequestInit, signal?: AbortSignal): Promise<T> {
  let response: Response
  try {
    response = await fetch(path, { ...init, signal, headers: { 'Content-Type': 'application/json', ...init.headers } })
  }
  catch (caught) {
    if (signal?.aborted) throw caught
    throw new HistoryReplayApiError('批量准备服务暂不可用，请重试', 'HISTORY_REPLAY_STORAGE_UNAVAILABLE', true)
  }
  let envelope: ApiSuccess<T> | ApiFailure
  try {
    envelope = await response.json() as ApiSuccess<T> | ApiFailure
  }
  catch {
    throw new HistoryReplayApiError('批量准备响应格式异常，请重试', 'HISTORY_REPLAY_STORAGE_UNAVAILABLE', true)
  }
  if (!response.ok || !envelope.success) {
    const failure = envelope as ApiFailure
    throw new HistoryReplayApiError(failure.error?.message || '批量准备操作失败，请重试', failure.error?.code, failure.error?.retryable)
  }
  return envelope.data
}
