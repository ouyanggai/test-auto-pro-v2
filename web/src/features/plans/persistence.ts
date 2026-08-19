import type {
  FlowCandidate,
  PlanFilters,
  PlanFormValue,
  PlanRow,
  PlanStatus,
  PersistedPlan,
  VerifiedTargetAccount,
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
  }
}

type ApiEnvelope<T> = ApiSuccess<T> | ApiFailure

interface PlanListResponse {
  items: PersistedPlan[]
}

export interface CreatePlanRequest {
  name: string
  account: string
  accountDisplayName: string
  flowSource: PlanFormValue['flowSource']
  targetObjectId: string
  targetObjectName: string
  runMode: PlanFormValue['runMode']
  maxConcurrency: number | null
  scheduledAt: string | null
}

export class PlanApiError extends Error {
  readonly code?: string
  readonly retryable: boolean
  readonly status: number

  constructor(message: string, options: { code?: string, retryable?: boolean, status?: number } = {}) {
    super(message)
    this.name = 'PlanApiError'
    this.code = options.code
    this.retryable = options.retryable ?? false
    this.status = options.status ?? 0
  }
}

const knownPlanStatuses = new Set<PlanStatus>(['not_started', 'running', 'completed'])

export function buildCreatePlanRequest(
  form: PlanFormValue,
  account: VerifiedTargetAccount,
  candidate: FlowCandidate,
): CreatePlanRequest {
  const targetObject = candidate.kind === 'template'
    ? { id: candidate.templateId, name: candidate.flowName }
    : candidate.kind === 'submitted'
      ? { id: candidate.id, name: candidate.name }
      : { id: candidate.flowInstanceId, name: candidate.flowInstanceName }
  return {
    name: form.name.trim(),
    account: account.account,
    accountDisplayName: account.displayName,
    flowSource: form.flowSource,
    targetObjectId: targetObject.id,
    targetObjectName: targetObject.name,
    runMode: form.runMode,
    maxConcurrency: form.runMode === 'parallel' ? form.maxConcurrency : null,
    scheduledAt: form.scheduleEnabled && form.scheduledAt !== null
      ? new Date(form.scheduledAt).toISOString()
      : null,
  }
}

export function toPlanRow(plan: PersistedPlan): PlanRow {
  if (!knownPlanStatuses.has(plan.status)) throw new PlanApiError('计划数据异常，请联系维护人员', { code: 'PLAN_DATA_INVALID' })
  const identity = plan.accountDisplayName
    ? `${plan.accountDisplayName}（${plan.account}）`
    : plan.account
  return {
    id: plan.id,
    name: plan.name,
    flowName: plan.targetObjectName,
    accountName: identity,
    pathCount: plan.pathCount,
    runMode: plan.runMode,
    scheduledAt: plan.scheduledAt,
    status: plan.status,
    lastRunResult: plan.lastRunResult || '暂无运行记录',
  }
}

export async function createPlan(payload: CreatePlanRequest, idempotencyKey: string, signal: AbortSignal): Promise<PersistedPlan> {
  return request<PersistedPlan>('/api/plans', {
    method: 'POST',
    headers: { 'Idempotency-Key': idempotencyKey },
    body: JSON.stringify(payload),
  }, signal)
}

export async function fetchPlans(filters: PlanFilters, signal: AbortSignal): Promise<PlanRow[]> {
  const search = new URLSearchParams()
  const name = filters.name.trim()
  if (name) search.set('name', name)
  if (filters.status) search.set('status', filters.status)
  const suffix = search.size > 0 ? `?${search}` : ''
  const response = await request<PlanListResponse>(`/api/plans${suffix}`, { method: 'GET' }, signal)
  return response.items.map(toPlanRow)
}

export async function fetchPlan(id: string, signal: AbortSignal): Promise<PersistedPlan> {
	return request<PersistedPlan>(`/api/plans/${encodeURIComponent(id)}`, { method: 'GET' }, signal)
}

// deletePlan 删除本系统计划及其路径配置，不触发目标平台请求。
export async function deletePlan(id: string): Promise<void> {
	await request<{ deleted: boolean }>(`/api/plans/${encodeURIComponent(id)}`, { method: 'DELETE' }, new AbortController().signal)
}

async function request<T>(path: string, init: RequestInit, signal: AbortSignal): Promise<T> {
  let response: Response
  try {
    response = await fetch(path, {
      ...init,
      signal,
      headers: { 'Content-Type': 'application/json', ...init.headers },
    })
  }
  catch (error) {
    if (signal.aborted) throw error
    throw new PlanApiError('服务暂不可用，请稍后重试', { code: 'PLAN_STORAGE_UNAVAILABLE', retryable: true })
  }

  let envelope: ApiEnvelope<T>
  try {
    envelope = await response.json() as ApiEnvelope<T>
  }
  catch {
    throw new PlanApiError('计划数据异常，请联系维护人员', { code: 'PLAN_DATA_INVALID', status: response.status })
  }
  if (!response.ok || !envelope.success) {
    const failure = envelope as ApiFailure
    throw new PlanApiError(failure.error?.message || '请求失败，请重试', {
      code: failure.error?.code,
      retryable: failure.error?.retryable,
      status: response.status,
    })
  }
  return envelope.data
}
