export interface ExecutionPathChoice {
  routeNodeId: string
  branchId: string
}

export interface ExecutionPath {
  id: string
  sequenceNo: number
  name: string
  configurationStatus: 'pending' | 'partial' | 'configured' | 'affected'
  configurationDetail: string
  included: boolean
  choices: ExecutionPathChoice[]
  updatedAt: string
}

export interface ExecutionPathBatchResult {
  totalCount: number
  existingCount: number
  createdCount: number
  items: ExecutionPath[]
}

export interface ExecutionPathGenerationPreview {
  totalCount: number
  existingCount: number
  pendingCount: number
  exceeded: boolean
}

export type ExecutionPathWorkspaceMode = 'view' | 'edit' | 'new' | 'copy' | null

export interface ExecutionPathWorkspacePresentation {
  title: '路径详情' | '编辑路径' | '新建路径' | '复制路径'
  branchEditing: boolean
  dirty: boolean
  showNameInput: boolean
  showSave: boolean
  hint: string
}

export interface ExecutionPathDecisionProgress {
  selected: number
  pending: number
  total: number
}

export type ExecutionPathWorkspaceDisposition = 'confirm' | 'reset' | 'preserve'

export interface ExecutionPathAnalysis {
  complete: boolean
  invalid: boolean
  missingRouteNodeIds: string[]
  reachableNodeIds: Set<string>
  reachableEdgeIds: Set<string>
}

export interface ExecutionPathSummaryItem {
  id: string
  kind: 'node' | 'choice' | 'parallel' | 'next' | 'pending'
  label: string
  detail: string
}
