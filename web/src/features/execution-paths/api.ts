import type { ExecutionPath, ExecutionPathChoice } from './types.ts'

interface ApiSuccess<T> {
  success: true
  data: T
}

interface ApiFailure {
  success: false
  error: { code: string, message: string, retryable: boolean }
}

interface ExecutionPathListResponse {
  items: ExecutionPath[]
}

export class ExecutionPathApiError extends Error {
  readonly code: string
  readonly retryable: boolean

  constructor(message: string, code = 'PLAN_STORAGE_UNAVAILABLE', retryable = false) {
    super(message)
    this.name = 'ExecutionPathApiError'
    this.code = code
    this.retryable = retryable
  }
}

export async function fetchExecutionPaths(planId: string, signal: AbortSignal): Promise<ExecutionPath[]> {
  const result = await request<ExecutionPathListResponse>(`/api/plans/${encodeURIComponent(planId)}/execution-paths`, { method: 'GET' }, signal)
  return result.items
}

export function createExecutionPath(planId: string, choices: ExecutionPathChoice[], idempotencyKey: string): Promise<ExecutionPath> {
  return request<ExecutionPath>(`/api/plans/${encodeURIComponent(planId)}/execution-paths`, {
    method: 'POST',
    headers: { 'Idempotency-Key': idempotencyKey },
    body: JSON.stringify({ choices }),
  })
}

export function updateExecutionPath(planId: string, pathId: string, choices: ExecutionPathChoice[]): Promise<ExecutionPath> {
  return request<ExecutionPath>(`/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(pathId)}`, {
    method: 'PUT', body: JSON.stringify({ choices }),
  })
}

export function deleteExecutionPath(planId: string, pathId: string): Promise<void> {
  return request<void>(`/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(pathId)}`, { method: 'DELETE' })
}

async function request<T>(path: string, init: RequestInit, signal?: AbortSignal): Promise<T> {
  let response: Response
  try {
    response = await fetch(path, {
      ...init,
      signal,
      headers: { 'Content-Type': 'application/json', ...init.headers },
    })
  }
  catch (error) {
    if (signal?.aborted) throw error
    throw new ExecutionPathApiError('路径服务暂不可用，请重试', 'PLAN_STORAGE_UNAVAILABLE', true)
  }
  if (response.status === 204) return undefined as T
  let envelope: ApiSuccess<T> | ApiFailure
  try {
    envelope = await response.json() as ApiSuccess<T> | ApiFailure
  }
  catch {
    throw new ExecutionPathApiError('路径数据格式异常，请重试', 'PLAN_STORAGE_UNAVAILABLE', true)
  }
  if (!response.ok || !envelope.success) {
    const failure = envelope as ApiFailure
    throw new ExecutionPathApiError(
      failure.error?.message || '路径操作失败，请重试',
      failure.error?.code,
      failure.error?.retryable,
    )
  }
  return envelope.data
}
