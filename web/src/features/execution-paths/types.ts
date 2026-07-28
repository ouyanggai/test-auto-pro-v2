export interface ExecutionPathChoice {
  routeNodeId: string
  branchId: string
}

export interface ExecutionPath {
  id: string
  sequenceNo: number
  choices: ExecutionPathChoice[]
  updatedAt: string
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
