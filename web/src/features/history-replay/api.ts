import type { HistoryCandidatePage, HistoryDataSource, HistorySourceMode } from './types'

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
    throw new HistoryDataApiError('历史数据服务暂不可用，请重试', 'HISTORY_STORAGE_UNAVAILABLE', true)
  }
  let envelope: ApiSuccess<T> | ApiFailure
  try {
    envelope = await response.json() as ApiSuccess<T> | ApiFailure
  }
  catch {
    throw new HistoryDataApiError('历史数据响应格式异常，请重试', 'HISTORY_STORAGE_UNAVAILABLE', true)
  }
  if (!response.ok || !envelope.success) {
    const failure = envelope as ApiFailure
    throw new HistoryDataApiError(
      failure.error?.message || '历史数据操作失败，请重试',
      failure.error?.code,
      failure.error?.retryable,
    )
  }
  return envelope.data
}
