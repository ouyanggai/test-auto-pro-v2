import type { FlowGraph } from './types'

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

export class FlowGraphApiError extends Error {
  readonly code: string
  readonly retryable: boolean
  readonly status: number

  constructor(message: string, options: { code?: string, retryable?: boolean, status?: number } = {}) {
    super(message)
    this.name = 'FlowGraphApiError'
    this.code = options.code ?? 'TARGET_UNAVAILABLE'
    this.retryable = options.retryable ?? false
    this.status = options.status ?? 0
  }
}

export async function fetchFlowGraph(planId: string, signal: AbortSignal): Promise<FlowGraph> {
  let response: Response
  try {
    response = await fetch(`/api/plans/${encodeURIComponent(planId)}/flow-graph`, { method: 'GET', signal })
  }
  catch (error) {
    if (signal.aborted) throw error
    throw new FlowGraphApiError('暂时无法读取流程，请重试', { retryable: true })
  }

  let envelope: ApiSuccess<FlowGraph> | ApiFailure
  try {
    envelope = await response.json() as ApiSuccess<FlowGraph> | ApiFailure
  }
  catch {
    throw new FlowGraphApiError('流程数据格式异常', { code: 'TARGET_RESPONSE_INVALID', retryable: true, status: response.status })
  }
  if (!response.ok || !envelope.success) {
    const failure = envelope as ApiFailure
    throw new FlowGraphApiError(failure.error?.message || '暂时无法读取流程，请重试', {
      code: failure.error?.code,
      retryable: failure.error?.retryable,
      status: response.status,
    })
  }
  return envelope.data
}
