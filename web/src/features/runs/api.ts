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
  releaseGroup?: string
  releaseRequired?: boolean
  action: string
  actionName: string
  nodeKey: string
  // nodeId 是当前步节点的图上标识：画布据此平移与高亮当前步。
  nodeId?: string
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
  // requests/requestSummary 是 F-030/T04 的真实目标请求明细与汇总（来自 network.log 传输层计时）。
  requests?: RunRequestItem[]
  requestSummary?: { totalMs: number, writeMs: number, count: number, writeCount: number, allDurationsKnown: boolean }
  // isReplay 是只读历史事实：用户侧重放已于 2026-09-06 移除，新运行的尝试恒为 false。
  isReplay?: boolean
}

// RunRequestItem 是一条真实目标接口请求的安全摘要。
export interface RunRequestItem {
  phase?: string
  requestClass: 'read' | 'write'
  endpoint: string
  durationMs: number
  durationKnown?: boolean
  statusCode: number
  result: string
  // resultSummary/blocking 是 F-034/T05 的一句话业务结果摘要与阻塞标记（后端受控生成）。
  resultSummary?: string
  blocking?: boolean
  retryAttempt: number
  traceId?: string
  at?: string
}

export interface RunStep {
  stepNo: number
  actionName: string
  // action 是稳定动作键：actionLabel 据此映射中文，不从中文名反推动作类型。
  action?: string
  releaseGroup?: string
  releaseRequired?: boolean
  nodeKey: string
  // nodeId 是节点在图上的真实标识：画布与侧栏按它取状态（nodeKey 是配置令牌键，另一套键空间）。
  nodeId?: string
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

// RunNodePlanAction 是某个节点上的一条已配置计划动作（后端由编译场景归组，全部中文）。
export interface RunNodePlanAction {
  sequence: number
  releaseGroup?: string
  releaseRequired?: boolean
  actionName: string
  // action 是稳定动作键：actionLabel 据此映射中文，不从中文名反推动作类型。
  action?: string
  sourceName: string
  scopeName: string
  precondition?: string
  expectedEffect?: string
  stopOnFailure?: string
  recoveryPolicy?: string
  reloadRequired: boolean
  parameterCount: number
}

export interface PathRunDetail {
  runId: number
  runNo: number
  startedAt?: string
  modeName: string
  runStatusName: string
  pathRunId: number
  pathRunStatus: string
  pathRunStatusName: string
  resultName?: string
  failureClassName?: string
  // stopKind=blocked 表示目标在写入前明确拒绝（前置条件未满足），区别于普通失败与结果待确认。
  stopKind?: string
  stopKindNote?: string
  // stopStepNo 是触发阻塞/停止的步骤号：只在展示该步骤详情时使用路径级阻塞信息。
  stopStepNo?: number
  finalTarget?: unknown
  planId: number
  planName: string
  pathId: number
  pathName: string
  // logDir 是这次运行的路径运行日志目录（相对日志根），由页面身份（runId/pathRunId）直接得出。
  logDir?: string
  // 目标实例身份：instanceName 只作业务信息展示，不参与日志目录定位；
  // instanceNameAvailable 为 false 时显示「实例名称不可用」并用 instanceNameNote 说明原因。
  instanceId?: string
  instanceName?: string
  instanceNameAvailable?: boolean
  instanceNameNote?: string
  steps: RunStep[]
  currentPreview?: RunPreview
  nodeStates: Record<string, RunNodeState>
  // graphError 原样携带后端结构读取失败的底层错误文案；为空表示结构读取正常。
  graphError?: string
  // nodePlans 按图节点 ID 索引本次运行在该节点上的已配置计划（侧栏配置页签的唯一来源）。
  nodePlans?: Record<string, RunNodePlanAction[]>
  pollIntervalMs: number
  staleAfterMs: number
  // 控制现场（F-017）：版本、当前步、生效断点、为什么停在这里、可用命令。
  controlVersion: number
  currentStepNo: number
  breakpoints: RunBreakpoint[]
  stopReason?: string
  commands: RunCommand[]
  loopRunning: boolean
  stepInFlight: boolean
  stopRequested: boolean
  pauseRequested: boolean
  // pathChoices 是这条路径已保存的分支选择（分支节点 ID + 分支 ID），画布遍历分析的输入。
  pathChoices?: Array<{ routeNodeId: string; branchId: string }>
  // 运行级信息（F-020）：调度方式、并发说明与全部路径运行摘要。
  runScheduleName?: string
  runConcurrencyLabel?: string
  paths?: RunPathSummary[]
  // currentPhase/currentPhaseNote 是当前步实时阶段与中文补充；currentPhaseSince 是进入时刻。
  currentPhase?: string
  currentPhaseNote?: string
  currentPhaseSince?: string
  // sceneLost 表示执行现场已丢失（服务重启或执行结果无法确认），运行无法安全继续；
  // sceneLostNote 是配套的大白话说明与下一步引导。页面只展示只读记录，不给任何重试或登记入口。
  sceneLost?: boolean
  sceneLostNote?: string
  // interruptedNodeId/InterruptedNote 定位中断时正在执行、尚未落账的那一步。
  interruptedNodeId?: string
  interruptedNote?: string
  // retryable 表示服务端判定这条路径运行可以重试失败动作（F-028）：
  // 只有「步骤执行中确定失败（无目标副作用）」的运行可重试；前端只按它决定是否渲染重试按钮。
  retryable?: boolean
  // 模式切换（2026-09-06）：modeSwitchPending 表示已收到切换请求、将在本步完成后生效；
  // pendingModeName 是目标模式的中文显示名。
  modeSwitchPending?: boolean
  pendingModeName?: string
}

export interface RunSummary {
  runId: number
  runNo: number
  modeName: string
  statusName: string
  resultName?: string
  startedAt?: string
  finishedAt?: string
  // 计划身份：一行只对应一次计划运行，列表按计划与运行号共同展示。
  planId: number
  planName?: string
  pathRunId: number
  pathRunStatusName: string
  // 运行级摘要（F-020）：调度方式与路径状态中文汇总。
  scheduleName?: string
  pathsSummary?: string
  pathRunCount?: number
}

// RunPathProgress 是二级路径页里一条执行路径的进度事实（后端由落库步骤与冻结总步骤计算）。
export interface RunPathProgress {
  pathRunId: number
  pathId: number
  pathName: string
  statusName: string
  resultName?: string
  failureClassName?: string
  currentNodeName?: string
  doneSteps: number
  totalSteps: number
  progressPercent: number
  startedAt?: string
  finishedAt?: string
  mainInstanceRef?: string
}

// RunPathsView 是二级「本次运行的执行路径」页的数据主体。
export interface RunPathsView {
  runId: number
  runNo: number
  modeName: string
  runStatusName: string
  resultName?: string
  scheduleName?: string
  concurrencyLabel?: string
  planId: number
  planName: string
  startedAt?: string
  finishedAt?: string
  paths: RunPathProgress[]
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

// fetchAllRuns 跨计划列出运行（最新在前）：一行只对应一次计划运行，可按状态筛选。
export function fetchAllRuns(status = ''): Promise<RunSummary[]> {
  const query = status ? `?status=${encodeURIComponent(status)}` : ''
  return requestOnce<RunSummary[]>(`/api/runs${query}`, { method: 'GET' })
}

// fetchRunPaths 读取一次运行的二级路径页数据（每条路径的准确进度）。
export function fetchRunPaths(runId: string): Promise<RunPathsView> {
  return requestOnce<RunPathsView>(`/api/runs/${encodeURIComponent(runId)}/paths`, { method: 'GET' })
}

// deleteRun 删除整次工具侧运行及其全部子记录；运行中的记录必须先停止再删除（后端守卫）。
export function deleteRun(runId: string): Promise<void> {
  return requestOnce<void>(`/api/runs/${encodeURIComponent(runId)}`, { method: 'DELETE' })
}

// RunEventItem 是一条运行事件的公开形态（F-021 事件流时间线）。
export interface RunEventItem {
  id: number
  pathRunId?: number
  kind: string
  label: string
  createdAt: string
}

// fetchRunEvents 增量读取事件流：afterEventId 为游标，只返回其后的事件。
export function fetchRunEvents(runId: string, afterEventId: number, pathRunId?: number): Promise<RunEventItem[]> {
  const params = new URLSearchParams({ afterEventId: String(afterEventId) })
  if (pathRunId) params.set('pathRunId', String(pathRunId))
  return requestOnce<RunEventItem[]>(`/api/runs/${encodeURIComponent(runId)}/events?${params.toString()}`, { method: 'GET' })
}

// fetchRunDetail 读取路径运行详情。
export function fetchRunDetail(runId: string, signal?: AbortSignal, pathRunId?: number): Promise<PathRunDetail> {
  const query = pathRunId ? `?pathRunId=${pathRunId}` : ''
  return requestOnce<PathRunDetail>(`/api/runs/${encodeURIComponent(runId)}${query}`, { method: 'GET' }, signal)
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
  // nodeKey 是节点断点的挂载键：删除断点必须原样带回。
  nodeKey?: string
  stepNo?: number
  action?: string
}

// BreakpointInput 是启动预置/增删断点的请求体。
export interface BreakpointInput {
  type: string
  stepNo?: number
  nodeKey?: string
  action?: string
  // nodeName 是服务端翻译好的业务名称（仅展示用，不参与增删匹配）。
  nodeName?: string
}

// RunPathSummary 是一条路径运行在运行级视图里的摘要。
export interface RunPathSummary {
  pathRunId: number
  pathId: number
  pathName: string
  statusName: string
  resultName?: string
  mainInstanceRef?: string
}

// RunStartResult 是多路径启动的公开结果（F-020）。
export interface RunStartResult {
  runId: number
  runNo: number
  modeName: string
  scheduleName: string
  concurrencyLabel: string
  paths: RunPathSummary[]
}

// startRun 按勾选路径集合启动一次运行（F-020 多路径；F-017 模式三选一，默认单步由后端兜底）。
// idempotencyKey 由调用方生成：同键重试返回同一次运行，绝不创建第二个运行。
export function startRun(
  planId: string,
  pathIds: string[],
  mode = 'auto',
  breakpoints: BreakpointInput[] = [],
  idempotencyKey = '',
  pathDispatch: 'serial' | 'parallel' = 'serial',
  pathMaxConcurrency?: number,
): Promise<RunStartResult> {
  return requestOnce<RunStartResult>(`/api/plans/${encodeURIComponent(planId)}/runs`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      planId: Number(planId), pathIds: pathIds.map(Number), mode, breakpoints, idempotencyKey,
      pathDispatch, pathMaxConcurrency: pathDispatch === 'parallel' ? (pathMaxConcurrency ?? 2) : null,
    }),
  })
}

// fetchRunDispatchDefault 读取计划上次启动的路径调度选择（记住上次选择）：无历史时 dispatch 为空。
export function fetchRunDispatchDefault(planId: string): Promise<{ pathDispatch: string, maxConcurrency: number | null }> {
  return requestOnce<{ pathDispatch: string, maxConcurrency: number | null }>(
    `/api/plans/${encodeURIComponent(planId)}/run-dispatch-default`,
  )
}

// pathRunQuery 把可选的路径运行身份拼成查询串（多路径运行的控制寻址）。
function pathRunQuery(pathRunId?: number): string {
  return pathRunId ? `?pathRunId=${pathRunId}` : ''
}

// approveRun 按命令放行：命令携带步游标与控制版本（条件写、幂等：重复点击只产生一次效果）。
export function approveRun(runId: string, command = 'step', cursor = 0, controlVersion = 0, pathRunId?: number): Promise<PathRunDetail> {
  return requestOnce<PathRunDetail>(`/api/runs/${encodeURIComponent(runId)}/approve${pathRunQuery(pathRunId)}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ command, cursor, controlVersion }),
  })
}

// setBreakpoint / removeBreakpoint 运行中增删断点，即时生效并即时可见。
export function setBreakpoint(runId: string, bp: BreakpointInput, pathRunId?: number): Promise<BreakpointInput[]> {
  return requestOnce<BreakpointInput[]>(`/api/runs/${encodeURIComponent(runId)}/breakpoints${pathRunQuery(pathRunId)}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(bp),
  })
}

export function removeBreakpoint(runId: string, bp: BreakpointInput, pathRunId?: number): Promise<BreakpointInput[]> {
  return requestOnce<BreakpointInput[]>(`/api/runs/${encodeURIComponent(runId)}/breakpoints${pathRunQuery(pathRunId)}`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(bp),
  })
}

// switchRunMode 运行中切换自动/单步：请求携带控制版本（条件写幂等），
// 切换在安全步骤边界生效；响应里的 modeSwitchPending 表示将在本步完成后生效。
export function switchRunMode(runId: string, mode: 'auto' | 'manual_control', controlVersion: number, pathRunId?: number): Promise<PathRunDetail> {
  return requestOnce<PathRunDetail>(`/api/runs/${encodeURIComponent(runId)}/mode${pathRunQuery(pathRunId)}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ mode, controlVersion }),
  })
}

// requestPause 提交暂停请求（本步走完核验与落账后生效）。
export function requestPause(runId: string, pathRunId?: number): Promise<unknown> {
  return requestOnce<unknown>(`/api/runs/${encodeURIComponent(runId)}/pause${pathRunQuery(pathRunId)}`, { method: 'POST' })
}



// stopRun 停止路径运行。
export function stopRun(runId: string, pathRunId?: number): Promise<PathRunDetail> {
  return requestOnce<PathRunDetail>(`/api/runs/${encodeURIComponent(runId)}/stop${pathRunQuery(pathRunId)}`, { method: 'POST' })
}

// retryFailedAction 重试失败动作（F-028）：把确定失败的路径运行从失败步骤重新装填。
// 重试本身不发任何写请求；装填后按原运行模式继续（人工控制等放行，单步等执行一步）。
// 重复点击时第一次已把状态装填为运行中，后续请求得到稳定的中文冲突提示。
export function retryFailedAction(runId: string, pathRunId?: number): Promise<PathRunDetail> {
  return requestOnce<PathRunDetail>(`/api/runs/${encodeURIComponent(runId)}/retry${pathRunQuery(pathRunId)}`, { method: 'POST' })
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
