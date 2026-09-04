import type { PathSuccessAssertion, PlanRunReadiness, SuccessAssertionWorkspace } from './types'

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

// fetchSuccessAssertion 读取单条路径的断言工作区：真实候选、目标真实状态与已保存断言。
export function fetchSuccessAssertion(planId: string, pathId: string, signal?: AbortSignal): Promise<SuccessAssertionWorkspace> {
  return request<SuccessAssertionWorkspace>(
    `/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(pathId)}/success-assertion`,
    { method: 'GET' },
    signal,
  )
}

// saveSuccessAssertion 保存单条路径的成功断言；revision 用于并发保存冲突检测，首次保存传 0。
export function saveSuccessAssertion(
  planId: string,
  pathId: string,
  payload: { endNodeKey: string, expectedStatus: string, arrivalOrdinal: number, revision: number },
  idempotencyKey: string,
): Promise<PathSuccessAssertion> {
  return request<PathSuccessAssertion>(
    `/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(pathId)}/success-assertion`,
    { method: 'PUT', headers: { 'Idempotency-Key': idempotencyKey }, body: JSON.stringify(payload) },
  )
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
