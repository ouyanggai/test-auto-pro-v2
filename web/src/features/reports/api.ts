// F-042 报告中心前端类型与请求：与后端 report_center.go DTO 字段一一对应，不制造第二份语义。
// 全部只读聚合，无写操作；错误文案与后端同源，只在网络层失败时给中文兜底。
import { formatTime } from '../runs/api'

export { formatTime }

// LatencyStats 是一组耗时样本的分位摘要（毫秒）；sampleCount=0 表示暂无足够样本。
export interface LatencyStats {
  sampleCount: number
  p50: number
  p90: number
  p95: number
  mean: number
}

// ReportMetrics 是一个范围（全局/计划/路径）的专业测试指标聚合。
export interface ReportMetrics {
  finishedPathRuns: number
  succeededPathRuns: number
  successRate: number
  firstPassRuns: number
  firstPassRate: number
  totalPaths: number
  coveredPaths: number
  pathCoverage: number
  stepLatencyMs: LatencyStats
  interfaceLatencyMs: LatencyStats
  interfaceKnown: boolean
  executorLatencyMs: LatencyStats
  interfaceWriteLatencyMs: LatencyStats
  resultDistribution: Record<string, number>
  failureClasses: Record<string, number>
  interfaceCalls: number
  interfaceReadCalls: number
  interfaceWriteCalls: number
  retryAttempts: number
  retriedPathRuns: number
  sampleCapped: boolean
}

export interface ReportCounts {
  plansTotal: number
  plansWithRuns: number
  runsTotal: number
  runsRunning: number
  pathRunsTotal: number
}

export interface ReportPlanCard {
  planId: number
  planName: string
  runCount: number
  pathRunCount: number
  succeededPathRuns: number
  finishedPathRuns: number
  successRate: number
  resultDistribution: Record<string, number>
  coveredPaths: number
  totalPaths: number
  pathCoverage: number
  lastRunNo?: number
  lastRunResultName?: string
  lastRunFinishedAt?: string
}

export interface ReportOverview {
  counts: ReportCounts
  plans: ReportPlanCard[]
  metrics: ReportMetrics
}

export interface ReportPathCard {
  executionPathId: number
  sequenceNo: number
  name: string
  pathRunCount: number
  succeededPathRuns: number
  finishedPathRuns: number
  successRate: number
  avgSteps: number
  stepLatencyMs: LatencyStats
  interfaceLatencyMs: LatencyStats
  lastResultName: string
}

export interface ReportPlanDetail {
  planId: number
  planName: string
  account: string
  flowSource: string
  targetObjectName: string
  totalPaths: number
  runCount: number
  metrics: ReportMetrics
  paths: ReportPathCard[]
}

export interface ReportRunInstance {
  runId: number
  runNo: number
  pathRunId: number
  statusName: string
  resultName: string
  stepCount: number
  totalDurationMs: number
  interfaceCalls: number
  mainInstanceRef: string
  startedAt?: string
  interfaceDurationMs: number
  interfaceWriteDurationMs: number
  interfaceDurationKnown: boolean
  executorDurationMs: number
  executorDurationKnown: boolean
  orchestrationDurationMs: number
  orchestrationKnown: boolean
}

export interface ReportPathDetail {
  planId: number
  executionPathId: number
  sequenceNo: number
  name: string
  configurationStatus: string
  runCount: number
  metrics: ReportMetrics
  runs: ReportRunInstance[]
}

// ReportApiError 是报告接口的网络层兜底错误：文案与后端同源，网络失败时给中文提示。
export class ReportApiError extends Error {
  readonly code: string
  readonly retryable: boolean
  readonly status: number

  constructor(message: string, options: { code?: string, retryable?: boolean, status?: number } = {}) {
    super(message)
    this.name = 'ReportApiError'
    this.code = options.code || 'REPORT_FAILED'
    this.retryable = options.retryable ?? false
    this.status = options.status ?? 0
  }
}

// requestReport 发起只读报告请求并解包统一响应信封；失败时抛 ReportApiError。
async function requestReport<T>(path: string, signal?: AbortSignal): Promise<T> {
  let response: Response
  try {
    response = await fetch(path, { method: 'GET', signal })
  }
  catch (error) {
    if (signal?.aborted) throw error
    throw new ReportApiError('暂时无法连接后端服务，请重试', { code: 'NETWORK', retryable: true, status: 0 })
  }
  let envelope: unknown
  try {
    envelope = await response.json()
  }
  catch {
    throw new ReportApiError('后端响应格式异常，请重试', { code: 'INVALID_RESPONSE', retryable: true, status: response.status })
  }
  const parsed = envelope as { success?: boolean, data?: T, error?: { code?: string, message?: string, retryable?: boolean } }
  if (!response.ok || !parsed.success) {
    throw new ReportApiError(parsed.error?.message || '报告数据读取失败，请重试', {
      code: parsed.error?.code,
      retryable: parsed.error?.retryable ?? true,
      status: response.status,
    })
  }
  return parsed.data as T
}

// fetchReportOverview 读取 L0 首页总览聚合。
export function fetchReportOverview(signal?: AbortSignal): Promise<ReportOverview> {
  return requestReport<ReportOverview>('/api/reports/overview', signal)
}

// fetchPlanReport 读取 L1 计划详情聚合。
export function fetchPlanReport(planId: string, signal?: AbortSignal): Promise<ReportPlanDetail> {
  return requestReport<ReportPlanDetail>(`/api/reports/plans/${encodeURIComponent(planId)}`, signal)
}

// fetchPathReport 读取 L2 路径详情聚合。
export function fetchPathReport(planId: string, pathId: string, signal?: AbortSignal): Promise<ReportPathDetail> {
  return requestReport<ReportPathDetail>(`/api/reports/plans/${encodeURIComponent(planId)}/paths/${encodeURIComponent(pathId)}`, signal)
}

// formatPercent 把 0-1 比率格式化为百分比文本；样本不足时上层显示「暂无足够样本」。
export function formatPercent(rate: number): string {
  return `${(rate * 100).toFixed(1)}%`
}

// formatMs 把毫秒格式化为易读文本；超过 1 秒进位到秒，保留一位小数。
export function formatMs(ms: number): string {
  if (ms >= 1000) return `${(ms / 1000).toFixed(2)} s`
  return `${Math.round(ms)} ms`
}

// latencyText 汇总一组分位为一行文本；无样本返回「暂无足够样本」。
export function latencyText(stats: LatencyStats): string {
  if (!stats || stats.sampleCount === 0) return '暂无足够样本'
  return `P50 ${formatMs(stats.p50)} · P90 ${formatMs(stats.p90)} · P95 ${formatMs(stats.p95)} · 均值 ${formatMs(stats.mean)}`
}
