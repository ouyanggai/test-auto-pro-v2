import type { PlanRunReadiness } from './types'

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

// RunReadinessApiError 保留后端稳定错误码与中文文案，界面直接显示这句话，与日志同源。
export class RunReadinessApiError extends Error {
  readonly code: string
  readonly retryable: boolean

  constructor(message: string, code = 'RUN_READINESS_STORAGE_UNAVAILABLE', retryable = false) {
    super(message)
    this.name = 'RunReadinessApiError'
    this.code = code
    this.retryable = retryable
  }
}

// request 统一解析统一响应包络，失败一律抛出带中文文案的错误，不在前端另造提示。
async function request<T>(url: string, init: RequestInit, signal?: AbortSignal): Promise<T> {
  let response: Response
  try {
    response = await fetch(url, { ...init, signal, headers: { 'Content-Type': 'application/json', ...(init.headers ?? {}) } })
  }
  catch {
    throw new RunReadinessApiError('暂时无法连接后端服务，请重试', 'RUN_READINESS_STORAGE_UNAVAILABLE', true)
  }
  let envelope: ApiSuccess<T> | ApiFailure
  try {
    envelope = await response.json() as ApiSuccess<T> | ApiFailure
  }
  catch {
    throw new RunReadinessApiError('运行准备响应格式异常，请重试', 'RUN_READINESS_STORAGE_UNAVAILABLE', true)
  }
  if (!response.ok || !envelope.success) {
    const failure = envelope as ApiFailure
    throw new RunReadinessApiError(failure.error?.message || '运行准备操作失败，请重试', failure.error?.code, failure.error?.retryable)
  }
  return envelope.data
}

// fetchPlanRunReadiness 读取运行前检查结论，只读，不启动任何运行。
// pathIds 为空表示检查全部路径；传了就只检查这些勾选路径，与"运行只运行勾选路径"保持同一范围。
export function fetchPlanRunReadiness(planId: string, pathIds: string[] = [], signal?: AbortSignal): Promise<PlanRunReadiness> {
  const search = pathIds.length ? `?pathIds=${encodeURIComponent(pathIds.join(','))}` : ''
  return request<PlanRunReadiness>(
    `/api/plans/${encodeURIComponent(planId)}/run-readiness${search}`,
    { method: 'GET' },
    signal,
  )
}
