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
