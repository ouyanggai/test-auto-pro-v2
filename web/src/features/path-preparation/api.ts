import type { PathPreparationItemPage, PathPreparationJob } from './types.ts'

interface ApiSuccess<T> {
  success: true
  data: T
}

interface ApiFailure {
  success: false
  error: { code: string, message: string, retryable: boolean }
}

export class PathPreparationApiError extends Error {
  readonly code: string
  readonly retryable: boolean

  constructor(message: string, code = 'PLAN_STORAGE_UNAVAILABLE', retryable = false) {
    super(message)
    this.name = 'PathPreparationApiError'
    this.code = code
    this.retryable = retryable
  }
}

// createPathPreparation 只创建当前计划已勾选路径的持久后台任务。
export function createPathPreparation(planId: string, idempotencyKey: string): Promise<PathPreparationJob> {
  return request<PathPreparationJob>(`/api/plans/${encodeURIComponent(planId)}/path-preparations`, {
    method: 'POST', headers: { 'Idempotency-Key': idempotencyKey },
  })
}

// fetchActivePathPreparation 读取刷新后仍在排队或运行的任务。
export function fetchActivePathPreparation(planId: string, signal?: AbortSignal): Promise<PathPreparationJob | null> {
  return request<PathPreparationJob | null>(`/api/plans/${encodeURIComponent(planId)}/path-preparations/active`, { method: 'GET' }, signal)
}

// fetchPathPreparation 读取任务真实聚合计数。
export function fetchPathPreparation(planId: string, jobId: string, signal?: AbortSignal): Promise<PathPreparationJob> {
  return request<PathPreparationJob>(`/api/plans/${encodeURIComponent(planId)}/path-preparations/${encodeURIComponent(jobId)}`, { method: 'GET' }, signal)
}

// fetchPathPreparationItems 按明细 ID 游标分页读取单路径结果。
export function fetchPathPreparationItems(planId: string, jobId: string, cursor = 0, limit = 20, signal?: AbortSignal): Promise<PathPreparationItemPage> {
  const query = new URLSearchParams({ cursor: String(cursor), limit: String(limit) })
  return request<PathPreparationItemPage>(`/api/plans/${encodeURIComponent(planId)}/path-preparations/${encodeURIComponent(jobId)}/items?${query}`, { method: 'GET' }, signal)
}

// cancelPathPreparation 取消任务并保留检查点。
export function cancelPathPreparation(planId: string, jobId: string): Promise<PathPreparationJob> {
  return request<PathPreparationJob>(`/api/plans/${encodeURIComponent(planId)}/path-preparations/${encodeURIComponent(jobId)}/cancel`, { method: 'POST' })
}

// resumePathPreparation 从未完成检查点恢复任务。
export function resumePathPreparation(planId: string, jobId: string): Promise<PathPreparationJob> {
  return request<PathPreparationJob>(`/api/plans/${encodeURIComponent(planId)}/path-preparations/${encodeURIComponent(jobId)}/resume`, { method: 'POST' })
}

// request 统一解析批量准备业务响应并隐藏技术错误。
async function request<T>(path: string, init: RequestInit, signal?: AbortSignal): Promise<T> {
  let response: Response
  try {
    response = await fetch(path, { ...init, signal, headers: { 'Content-Type': 'application/json', ...init.headers } })
  }
  catch (error) {
    if (signal?.aborted) throw error
    throw new PathPreparationApiError('批量准备服务暂不可用，请重试', 'PLAN_STORAGE_UNAVAILABLE', true)
  }
  const envelope = await response.json() as ApiSuccess<T> | ApiFailure
  if (!response.ok || !envelope.success) {
    const failure = envelope as ApiFailure
    throw new PathPreparationApiError(failure.error?.message || '批量准备失败，请重试', failure.error?.code, failure.error?.retryable)
  }
  return envelope.data
}
