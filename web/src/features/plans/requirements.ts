import type { ExecutionPath } from '../execution-paths/types.ts'

export type RequirementStatus = '待配置' | '目标平台自动确定' | '运行时确定' | '需要人工核对'

export interface RequirementCount {
  status: RequirementStatus
  count: number
}

export interface RequirementItem {
  category: '条件' | '人员' | '动作' | '约束'
  title: string
  detail: string
  status: RequirementStatus
}

export interface RequirementNode {
  name: string
  typeName: string
  items: RequirementItem[]
}

export interface RequirementGroup {
  title: string
  kind: 'main' | 'parallel'
  nodes: RequirementNode[]
}

export interface PathRequirements {
  path: { sequenceNo: number, name: string }
  summary: RequirementCount[]
  groups: RequirementGroup[]
}

interface ApiSuccess<T> {
  success: true
  data: T
}

interface ApiFailure {
  success: false
  error: { code: string, message: string, retryable: boolean }
}

export class PathRequirementApiError extends Error {
  readonly code: string
  readonly retryable: boolean

  constructor(message: string, code = 'TARGET_UNAVAILABLE', retryable = false) {
    super(message)
    this.name = 'PathRequirementApiError'
    this.code = code
    this.retryable = retryable
  }
}

export async function fetchPathRequirements(planId: string, pathId: string, signal: AbortSignal): Promise<PathRequirements> {
  let response: Response
  try {
    response = await fetch(`/api/plans/${encodeURIComponent(planId)}/execution-paths/${encodeURIComponent(pathId)}/requirements`, {
      method: 'GET', signal, headers: { Accept: 'application/json' },
    })
  }
  catch (error) {
    if (signal.aborted) throw error
    throw new PathRequirementApiError('暂时无法读取路径要求，请重试', 'TARGET_UNAVAILABLE', true)
  }
  let envelope: ApiSuccess<PathRequirements> | ApiFailure
  try {
    envelope = await response.json() as ApiSuccess<PathRequirements> | ApiFailure
  }
  catch {
    throw new PathRequirementApiError('路径要求数据格式异常，请重试', 'TARGET_RESPONSE_INVALID', true)
  }
  if (!response.ok || !envelope.success) {
    const failure = envelope as ApiFailure
    const message = failure.error?.code === 'EXECUTION_PATH_INVALID'
      ? '当前路径已不符合最新流程，请返回重新选择'
      : failure.error?.message || '暂时无法读取路径要求，请重试'
    throw new PathRequirementApiError(message, failure.error?.code, failure.error?.retryable)
  }
  return envelope.data
}

export function defaultRequirementPath(paths: ExecutionPath[]): ExecutionPath | null {
  if (paths.length === 0) return null
  return [...paths].sort((left, right) => left.sequenceNo - right.sequenceNo)[0] ?? null
}

export function shouldApplyRequirementResponse(input: {
  requestedPathId: string
  activePathId: string | null
  requestVersion: number
  currentVersion: number
  aborted: boolean
}): boolean {
  // 路径切换可能在旧请求返回前发生，路径标识和版本必须同时匹配才能覆盖当前页面。
  return !input.aborted
    && input.requestedPathId === input.activePathId
    && input.requestVersion === input.currentVersion
}

export function requirementStatusType(status: RequirementStatus): 'warning' | 'success' | 'info' | 'error' {
  switch (status) {
    case '待配置': return 'warning'
    case '目标平台自动确定': return 'success'
    case '运行时确定': return 'info'
    case '需要人工核对': return 'error'
  }
}
