export interface RuntimeSourceState {
  repository: string
  branch: string
  head: string
  dirty: boolean
  changedFiles: Array<{ path: string, status: string }>
  inspectedAt: string
}

export interface RuntimeSyncJob {
  id: number
  status: 'PENDING' | 'RUNNING' | 'SUCCEEDED' | 'FAILED'
  stage: 'QUEUED' | 'INSPECT' | 'SYNC' | 'SYNC_CHECK' | 'BUILD' | 'RESTART' | 'VERIFY' | 'COMPLETED'
  source: RuntimeSourceState
  failureReason?: string
  candidateVersion?: string
  previousVersion?: string
  recoveryStatus: 'NOT_REQUIRED' | 'SUCCEEDED' | 'FAILED' | 'UNKNOWN'
  recoveryMessage?: string
  attemptCount: number
  createdAt: string
  updatedAt: string
  completedAt?: string
}

export interface RuntimeSyncLog {
  content: string
  truncated: boolean
}

interface SuccessEnvelope<T> { success: true, data: T }
interface FailureEnvelope { success: false, error: { code: string, message: string } }

// maintenanceRequest 统一解析维护 API，并保留后端稳定错误文案。
async function maintenanceRequest<T>(path: string, init: RequestInit, signal?: AbortSignal): Promise<T> {
  const response = await fetch(path, { ...init, signal, headers: { 'Content-Type': 'application/json', ...init.headers } })
  const envelope = await response.json() as SuccessEnvelope<T> | FailureEnvelope
  if (!response.ok || !envelope.success) throw new Error((envelope as FailureEnvelope).error?.message || '表单运行时维护服务不可用')
  return envelope.data
}

// fetchRuntimeSource 读取固定来源状态。
export function fetchRuntimeSource(signal?: AbortSignal) {
  return maintenanceRequest<RuntimeSourceState>('/api/form-runtime-maintenance/source', { method: 'GET' }, signal)
}

// createRuntimeSyncJob 只触发固定的一键任务，不接受任何来源或命令参数。
export function createRuntimeSyncJob() {
  return maintenanceRequest<RuntimeSyncJob>('/api/form-runtime-maintenance/jobs', { method: 'POST' })
}

// fetchLatestRuntimeSyncJob 读取最近任务。
export function fetchLatestRuntimeSyncJob(signal?: AbortSignal) {
  return maintenanceRequest<RuntimeSyncJob>('/api/form-runtime-maintenance/jobs/latest', { method: 'GET' }, signal)
}

// fetchRuntimeSyncJob 轮询指定任务状态。
export function fetchRuntimeSyncJob(id: number, signal?: AbortSignal) {
  return maintenanceRequest<RuntimeSyncJob>(`/api/form-runtime-maintenance/jobs/${id}`, { method: 'GET' }, signal)
}

// fetchRuntimeSyncLog 读取有界任务日志。
export function fetchRuntimeSyncLog(id: number, signal?: AbortSignal) {
  return maintenanceRequest<RuntimeSyncLog>(`/api/form-runtime-maintenance/jobs/${id}/log`, { method: 'GET' }, signal)
}
