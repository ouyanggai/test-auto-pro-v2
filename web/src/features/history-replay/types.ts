export type HistoryRuntimeType = 'formmaking' | 'vue_custom' | 'unknown'
export type HistorySourceMode = 'none' | 'default' | 'override'
export type HistoryDataStatus = 'empty' | 'needs_input' | 'ready' | 'affected'

// HistoryCandidate 只包含目标返回的可见摘要和后端不透明键，不包含目标实例 ID。
export interface HistoryCandidate {
  candidateKey: string
  flowCode: string
  formName: string
  flowName: string
  runtimeType: HistoryRuntimeType
  instanceTitle: string
  businessSummary: string
  initiator: string
  companyName: string
  createdAt: string
  status: string
  statusName: string
  completeness: 'complete' | 'partial'
  integrityNotice: string
  snapshotAvailable: boolean
}

export interface HistorySnapshotSummary {
  candidateKey: string
  flowCode: string
  formName: string
  flowName: string
  instanceTitle: string
  businessSummary: string
  initiator: string
  companyName: string
  createdAt: string
  status: string
  statusName: string
  runtimeType: HistoryRuntimeType
}

export interface HistoryDataIssue {
  code: string
  path?: string
  message: string
  blocking: boolean
}

export interface HistoryDataSource {
  mode: HistorySourceMode
  summary?: HistorySnapshotSummary
  dataStatus: HistoryDataStatus
  issues: HistoryDataIssue[]
  revision: number
}

export interface HistoryCandidatePage {
  items: HistoryCandidate[]
  page: number
  pageSize: number
  total: number
  hasMore: boolean
  defaultSource?: HistoryDataSource
  pathSource?: HistoryDataSource
}

export type HistoryReplayStatus = 'queued' | 'running' | 'completed' | 'cancelled' | 'failed'
export type HistoryReplayItemStatus = 'pending' | 'running' | 'ready' | 'needs_input' | 'affected' | 'failed'

// HistoryReplayJob 只呈现后端真实聚合计数，租约、幂等键和 fencing 信息不进入页面状态。
export interface HistoryReplayJob {
  id: string
  status: HistoryReplayStatus
  total: number
  pending: number
  running: number
  ready: number
  needsInput: number
  affected: number
  failed: number
  cancelled: number
  createdAt: string
  updatedAt: string
  completedAt?: string
}

// HistoryReplayItem 是单路径回放检查点的公开投影，正文仍保持目标原始 map。
export interface HistoryReplayItem {
  id: number
  pathId: number
  pathRevision: number
  snapshotId?: number
  status: HistoryReplayItemStatus
  dataStatus: HistoryDataStatus
  issues: HistoryDataIssue[]
  branchPatches: Array<{ path: string, before: unknown, after: unknown, reason: string, branchKey: string }>
  effectiveFormData?: Record<string, unknown>
  updatedAt: string
  completedAt?: string
}

// HistoryReplayItemPage 以明细自增 ID 作为唯一游标，前端不向后端发送页码。
export interface HistoryReplayItemPage {
  items: HistoryReplayItem[]
  nextCursor?: number
}
