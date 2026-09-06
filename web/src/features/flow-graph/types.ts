export type FlowGraphNodeType = 'start' | 'empty' | 'parallel' | 'synergy' | 'common' | 'condition' | 'manual' | 'end' | 'unknown'
export type FlowGraphEdgeKind = 'sequence' | 'condition' | 'manual' | 'parallel'

export interface FlowGraphNode {
  id: string
  name: string
  type: FlowGraphNodeType
  typeName: string
  mergeTargetId?: string
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
  entryNodeIds: string[]
  nodes: FlowGraphNode[]
  edges: FlowGraphEdge[]
  warnings: string[]
}

export interface FlowNodeData extends Record<string, unknown> {
  name: string
  type: FlowGraphNodeType
  typeName: string
  mergeTargetId?: string
  configurationMode?: boolean
  configurationStatus?: 'not_required' | 'pending' | 'partial' | 'configured' | 'runtime' | 'affected'
  configurationStatusName?: string
  configurationInteractive?: boolean
  configurationSelected?: boolean
  configurationFormStatus?: string
  configurationFormStatusName?: string
  // runMode 是 F-016 运行画布的节点变体：九个中文运行态 + 当前步标记。
  runMode?: boolean
  runStatus?: string
  runStatusName?: string
  runCurrent?: boolean
  // runBusy 表示当前步此刻真的在执行（放行请求在途、连续执行中或核验中）：
  // 节点据此显示执行动画，与「停在这里等放行」区分开。
  runBusy?: boolean
  // runSelected 表示节点被用户点选（右栏打开）：卡片给明确的选中描边反馈。
  runSelected?: boolean
  // runErrorNote 是失败/结果待确认节点上的一句话错误摘要（来自已落账尝试，不猜测）。
  runErrorNote?: string
}

export interface FlowConfigurationNodeState {
  status: NonNullable<FlowNodeData['configurationStatus']>
  statusName: string
  interactive: boolean
  selected: boolean
}

export interface FlowTreeEdgeData extends Record<string, unknown> {
  path: string
  role: 'main' | 'fork' | 'merge'
  railY?: number
  labelX?: number
  labelY?: number
  routeNodeId: string
  branchId: string
  kind: FlowGraphEdgeKind
  workspaceOpen?: boolean
  branchEditing?: boolean
  selected?: boolean
  candidate?: boolean
  dimmed?: boolean
  active?: boolean
  parallelRequired?: boolean
  // taken 表示运行画布上本条边被真实走过；deviated 表示走过但偏离已配置路径（标红）。
  taken?: boolean
  deviated?: boolean
}
