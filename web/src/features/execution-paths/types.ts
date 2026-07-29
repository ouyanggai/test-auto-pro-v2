export interface ExecutionPathChoice {
  routeNodeId: string
  branchId: string
}

export interface ExecutionPath {
  id: string
  sequenceNo: number
  name: string
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
