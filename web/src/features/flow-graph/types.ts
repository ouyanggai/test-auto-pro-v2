export type FlowGraphNodeType = 'start' | 'empty' | 'parallel' | 'synergy' | 'common' | 'condition' | 'manual' | 'end' | 'unknown'
export type FlowGraphEdgeKind = 'sequence' | 'condition' | 'manual' | 'parallel'

export interface FlowGraphNode {
  id: string
  name: string
  type: FlowGraphNodeType
  typeName: string
}

export interface FlowGraphEdge {
  id: string
  source: string
  target: string
  kind: FlowGraphEdgeKind
  label: string
  branchId: string
}

export interface FlowGraph {
  planId: string
  targetName: string
  flowSource: 'new' | 'started' | 'pending'
  nodes: FlowGraphNode[]
  edges: FlowGraphEdge[]
  warnings: string[]
}

export interface FlowNodeData extends Record<string, unknown> {
  name: string
  type: FlowGraphNodeType
  typeName: string
}
