import type {
  PathConfigActionValue,
  PathConfiguration,
  PathConfigFieldValue,
  PathConfigSaveResult,
} from './types.ts'

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
    details?: Array<{ kind: string, name: string, reason: string }>
  }
}

export class PathConfigApiError extends Error {
  readonly code: string
  readonly retryable: boolean
  readonly details: Array<{ kind: string, name: string, reason: string }>

  constructor(message: string, code = 'PLAN_STORAGE_UNAVAILABLE', retryable = false, details: Array<{ kind: string, name: string, reason: string }> = []) {
    super(message)
    this.name = 'PathConfigApiError'
    this.code = code
    this.retryable = retryable
    this.details = details
  }
}

export async function fetchPathConfiguration(planId: string, pathId: string, signal: AbortSignal): Promise<PathConfiguration> {
  const result = await request<PathConfiguration>(`/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(pathId)}/configuration`, { method: 'GET' }, signal)
  return result
}

export function savePathConfiguration(
  planId: string,
  pathId: string,
  revision: number,
  fields: PathConfigFieldValue[],
  actions: PathConfigActionValue[],
  idempotencyKey: string,
): Promise<PathConfigSaveResult> {
  return request<PathConfigSaveResult>(`/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(pathId)}/configuration`, {
    method: 'PUT',
    headers: { 'Idempotency-Key': idempotencyKey },
    body: JSON.stringify({ revision, fields, actions }),
  })
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
    throw new PathConfigApiError('路径配置服务暂不可用，请重试', 'PLAN_STORAGE_UNAVAILABLE', true)
  }
  let envelope: ApiSuccess<T> | ApiFailure
  try {
    envelope = await response.json() as ApiSuccess<T> | ApiFailure
  }
  catch {
    throw new PathConfigApiError('路径配置数据格式异常，请重试', 'PLAN_STORAGE_UNAVAILABLE', true)
  }
  if (!response.ok || !envelope.success) {
    const failure = envelope as ApiFailure
    throw new PathConfigApiError(
      failure.error?.message || '路径配置操作失败，请重试',
      failure.error?.code,
      failure.error?.retryable,
      failure.error?.details || [],
    )
  }
  return envelope.data
}
