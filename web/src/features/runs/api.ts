// 运行主线（F-016）前端类型：与后端 DTO 字段一一对应，不制造第二份语义。
export interface RunNodeState {
  status: string
  statusName: string
}

export interface RunGateItem {
  key: string
  description: string
  passed: boolean
}

export interface RunPreview {
  stepNo: number
  totalSteps: number
  action: string
  actionName: string
  nodeKey: string
  nodeName: string
  actorName: string
  expectedEffect: string
  endpoint: string
  requestPreview: string
  gateAllowed: boolean
  gateReason?: string
  gateItems: RunGateItem[]
  facts: Record<string, unknown>
  blockReason?: string
}

export interface RunStepAttempt {
  attemptNo: number
  verdictName: string
  reason: string
  basis: string
  traceId: string
  durationMs: number
  logPath: string
  logLine: number
  phaseDurations?: Record<string, number>
  phaseDurationsNote?: string
  curlBlock?: string
}

export interface RunStep {
  stepNo: number
  actionName: string
  nodeKey: string
  nodeName: string
  actorName: string
  statusName: string
  startedAt: string
  finishedAt: string
  durationMs: number
  // gateSnapshot 是放行时的门禁结论快照 JSON（逐项中文条件与满足情况）。
  gateSnapshot?: string
  attempts: RunStepAttempt[]
}

export interface PathRunDetail {
  runId: number
  runNo: number
  modeName: string
  runStatusName: string
  pathRunId: number
  pathRunStatus: string
  pathRunStatusName: string
  resultName?: string
  failureClassName?: string
  finalTarget?: unknown
  planId: number
  planName: string
  pathId: number
  pathName: string
  steps: RunStep[]
  currentPreview?: RunPreview
  nodeStates: Record<string, RunNodeState>
  pollIntervalMs: number
  staleAfterMs: number
  // 控制现场（F-017）：版本、当前步、生效断点、为什么停在这里、可用命令。
  controlVersion: number
  currentStepNo: number
  breakpoints: RunBreakpoint[]
  stopReason?: string
  commands: RunCommand[]
  loopRunning: boolean
  stopRequested: boolean
  pauseRequested: boolean
  // pathChoices 是这条路径已保存的分支选择（分支节点 ID + 分支 ID），画布遍历分析的输入。
  pathChoices?: Array<{ routeNodeId: string; branchId: string }>
  // currentPhase/currentPhaseNote 是当前步实时阶段与中文补充；currentPhaseSince 是进入时刻。
  currentPhase?: string
  currentPhaseNote?: string
  currentPhaseSince?: string
}

export interface RunSummary {
  runId: number
  runNo: number
  modeName: string
  statusName: string
  resultName?: string
  startedAt?: string
  finishedAt?: string
  pathRunId: number
  pathRunStatusName: string
}

// 运行 API 错误：文案与后端同源，只在网络层失败时给前端兜底中文。
export class RunApiError extends Error {
  readonly code: string
  readonly retryable: boolean
  readonly status: number

  constructor(message: string, options: { code: string; retryable: boolean; status: number }) {
    super(message)
    this.name = 'RunApiError'
    this.code = options.code
    this.retryable = options.retryable
    this.status = options.status
  }
}

// requestOnce 是运行模块的统一请求出口：解析后端统一包络，不在前端另造提示。
async function requestOnce<T>(path: string, init?: RequestInit, signal?: AbortSignal): Promise<T> {
  let response: Response
  try {
    response = await fetch(path, { ...init, signal })
  } catch {
    throw new RunApiError('暂时无法连接后端服务，请重试', { code: 'NETWORK', retryable: true, status: 0 })
  }
  let envelope: unknown
  try {
    envelope = await response.json()
  } catch {
    throw new RunApiError('后端响应格式异常，请重试', { code: 'INVALID_RESPONSE', retryable: true, status: response.status })
  }
  const parsed = envelope as { success?: boolean; data?: T; error?: { code?: string; message?: string; retryable?: boolean } }
  if (!parsed.success) {
    throw new RunApiError(parsed.error?.message || '运行服务请求失败', {
      code: parsed.error?.code || 'RUN_FAILED',
      retryable: parsed.error?.retryable ?? false,
      status: response.status,
    })
  }
  return parsed.data as T
}

// fetchPlanRuns 列出计划下的运行（最新在前）。
export function fetchPlanRuns(planId: string, signal?: AbortSignal): Promise<RunSummary[]> {
  return requestOnce<RunSummary[]>(`/api/plans/${encodeURIComponent(planId)}/runs`, { method: 'GET' }, signal)
}

// fetchRunDetail 读取路径运行详情。
export function fetchRunDetail(runId: string, signal?: AbortSignal): Promise<PathRunDetail> {
  return requestOnce<PathRunDetail>(`/api/runs/${encodeURIComponent(runId)}`, { method: 'GET' }, signal)
}

// ReconcileView 是只读对账的结论：三值、唯一动作与逐维度依据。
export interface ReconcileView {
  verdict: string
  verdictName: string
  action: string
  headline: string
  reasons: string[]
  replaysUsed: number
  replaysMax: number
}

// RunCommand 是后端给出的可用命令（含中文停止条件说明）。
export interface RunCommand {
  command: string
  label: string
}

// RunBreakpoint 是生效断点的公开形态。
export interface RunBreakpoint {
  type: string
  typeName: string
  nodeName?: string
  stepNo?: number
  action?: string
}

// BreakpointInput 是启动预置/增删断点的请求体。
export interface BreakpointInput {
  type: string
  stepNo?: number
  nodeKey?: string
  action?: string
}

// startRun 按模式与预置断点启动一次运行（F-017：模式三选一，默认单步由后端兜底）。
export function startRun(planId: string, executionPathId: string, mode = 'single_step', breakpoints: BreakpointInput[] = []): Promise<PathRunDetail> {
  return requestOnce<PathRunDetail>(`/api/plans/${encodeURIComponent(planId)}/runs`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ planId: Number(planId), executionPathId: Number(executionPathId), mode, breakpoints }),
  })
}

// approveRun 按命令放行：命令携带步游标与控制版本（条件写、幂等：重复点击只产生一次效果）。
export function approveRun(runId: string, command = 'step', cursor = 0, controlVersion = 0): Promise<PathRunDetail> {
  return requestOnce<PathRunDetail>(`/api/runs/${encodeURIComponent(runId)}/approve`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ command, cursor, controlVersion }),
  })
}

// setBreakpoint / removeBreakpoint 运行中增删断点，即时生效并即时可见。
export function setBreakpoint(runId: string, bp: BreakpointInput): Promise<BreakpointInput[]> {
  return requestOnce<BreakpointInput[]>(`/api/runs/${encodeURIComponent(runId)}/breakpoints`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(bp),
  })
}

export function removeBreakpoint(runId: string, bp: BreakpointInput): Promise<BreakpointInput[]> {
  return requestOnce<BreakpointInput[]>(`/api/runs/${encodeURIComponent(runId)}/breakpoints`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(bp),
  })
}

// reconcileNow 触发只读对账（可重复调用，安全）。
export function reconcileNow(runId: string): Promise<ReconcileView> {
  return requestOnce<ReconcileView>(`/api/runs/${encodeURIComponent(runId)}/reconcile`, { method: 'POST' })
}

// recoveryAction 执行对账给出的唯一合法动作。
export function recoveryAction(runId: string, action: string, manual?: { instanceStatus: string; currentNode: string; note: string; reporter: string }): Promise<PathRunDetail> {
  return requestOnce<PathRunDetail>(`/api/runs/${encodeURIComponent(runId)}/recovery`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ action, ...(manual || {}) }),
  })
}

// requestPause 提交暂停请求（本步走完核验与落账后生效）。
export function requestPause(runId: string): Promise<unknown> {
  return requestOnce<unknown>(`/api/runs/${encodeURIComponent(runId)}/pause`, { method: 'POST' })
}



// stopRun 停止路径运行。
export function stopRun(runId: string): Promise<PathRunDetail> {
  return requestOnce<PathRunDetail>(`/api/runs/${encodeURIComponent(runId)}/stop`, { method: 'POST' })
}

// formatElapsed 把毫秒格式化为中文可读时长。
export function formatElapsed(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) return '—'
  if (ms < 1000) return `${Math.round(ms)} 毫秒`
  const seconds = ms / 1000
  if (seconds < 60) return `${seconds.toFixed(1)} 秒`
  const minutes = Math.floor(seconds / 60)
  const rest = Math.round(seconds % 60)
  return `${minutes} 分 ${rest} 秒`
}

// formatTime 把服务端时间格式化为本地中文时间。
export function formatTime(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '时间异常'
  return date.toLocaleString('zh-CN', { hour12: false })
}
