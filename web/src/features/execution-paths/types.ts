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
  dataStatus: 'not_required' | 'not_generated' | 'generated' | 'confirmed' | 'needs_attention'
  dataDetail: string
  included: boolean
	configurationRevision: number
  choices: ExecutionPathChoice[]
  updatedAt: string
}

export interface PathGenerationJob {
  id: string
  status: 'queued' | 'running' | 'completed' | 'cancelled' | 'failed'
  total: number
  completed: number
  created: number
  error?: string
  updatedAt: string
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
