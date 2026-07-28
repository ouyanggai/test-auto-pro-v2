import { MarkerType, Position, type Edge, type Node } from '@vue-flow/core'

import type { FlowGraph, FlowGraphEdge, FlowGraphNode, FlowNodeData, FlowTreeEdgeData } from './types'

export const flowNodeWidth = 180
export const flowNodeHeight = 72
export const flowRoutingHubSize = 8
export const flowTreeHorizontalGap = 104
export const flowTreeVerticalGap = 88
export const flowTreeForkOffset = 48
export const flowTreeRailToNodeGap = 48
export const flowTreeMargin = 48
export const initialFlowZoom = 0.9
export const flowStructureErrorMessage = '目标流程结构异常'

const routingNodeTypes = new Set(['condition', 'manual', 'parallel'])

interface Placement {
  x: number
  y: number
}

interface RouteRail {
  forkEdgeIds: string[]
  forkRailY: number
  mergeEdgeIds: string[]
  mergeRailY?: number
}

interface SegmentBlock {
  positions: Map<string, Placement>
  rails: RouteRail[]
  exits: string[]
  width: number
  height: number
}

interface EdgeRail {
  role: FlowTreeEdgeData['role']
  railY?: number
}

export interface LaidOutFlowGraph {
  nodes: Node<FlowNodeData>[]
  edges: Edge<FlowTreeEdgeData>[]
}

export interface SafeFlowLayoutResult {
  layout: LaidOutFlowGraph | null
  error: string
}

export interface FlowViewport {
  x: number
  y: number
  zoom: number
}

export function isRoutingNodeType(type: FlowNodeData['type']): boolean {
  return routingNodeTypes.has(type)
}

export function shouldSetInitialViewport(ready: boolean, positionedPlanId: string, planId: string): boolean {
  return ready && positionedPlanId !== planId
}

export function initialViewportForGraph(
  nodes: Node<FlowNodeData>[],
  viewportWidth: number,
  zoom = initialFlowZoom,
  topOffset = 52,
): FlowViewport | null {
  if (!nodes.length || viewportWidth <= 0) return null
  const root = nodes
    .filter((node) => node.data !== undefined && !isRoutingNodeType(node.data.type))
    .sort((left, right) => left.position.y - right.position.y || left.position.x - right.position.x)[0]
  if (!root) return null
  return {
    x: viewportWidth / 2 - (root.position.x + flowNodeWidth / 2) * zoom,
    y: topOffset - root.position.y * zoom,
    zoom,
  }
}

class FlowTreeLayout {
  private readonly graph: FlowGraph
  private readonly nodeById = new Map<string, FlowGraphNode>()
  private readonly outgoingById = new Map<string, FlowGraphEdge[]>()
  private readonly edgeByPair = new Map<string, FlowGraphEdge>()

  constructor(graph: FlowGraph) {
    this.graph = graph
    for (const node of graph.nodes) this.nodeById.set(node.id, node)
    for (const edge of graph.edges) {
      const outgoing = this.outgoingById.get(edge.source) ?? []
      outgoing.push(edge)
      this.outgoingById.set(edge.source, outgoing)
      this.edgeByPair.set(`${edge.source}\u0000${edge.target}`, edge)
    }
  }

  layout(): LaidOutFlowGraph {
    const rootId = this.findRootId()
    const block = this.layoutSegment(rootId, '', new Set())
    if (block.positions.size !== this.graph.nodes.length) {
      throw new Error('流程图包含无法从真实根节点到达的节点')
    }

    const positions = new Map<string, Placement>()
    for (const [id, position] of block.positions) {
      positions.set(id, { x: position.x + flowTreeMargin, y: position.y + flowTreeMargin })
    }
    const rails = block.rails.map((rail) => ({
      ...rail,
      forkRailY: rail.forkRailY + flowTreeMargin,
      mergeRailY: rail.mergeRailY === undefined ? undefined : rail.mergeRailY + flowTreeMargin,
    }))

    return {
      nodes: this.graph.nodes.map((node) => this.toVueFlowNode(node, positions)),
      edges: this.graph.edges.map((edge) => this.toVueFlowEdge(edge, positions, rails)),
    }
  }

  private findRootId(): string {
    const incoming = new Set(this.graph.edges.map((edge) => edge.target))
    const roots = this.graph.nodes.filter((node) => !incoming.has(node.id))
    if (roots.length !== 1) throw new Error('流程图必须只有一个真实根节点')
    return roots[0].id
  }

  private layoutSegment(startId: string, stopId: string, ancestors: Set<string>): SegmentBlock {
    if (!startId || startId === stopId || ancestors.has(startId)) {
      throw new Error('流程图分段存在空入口或循环')
    }
    const node = this.nodeById.get(startId)
    if (!node) throw new Error(`流程图缺少节点 ${startId}`)
    const nextAncestors = new Set(ancestors)
    nextAncestors.add(startId)
    const outgoing = this.outgoingById.get(startId) ?? []
    const branchEdges = outgoing.filter((edge) => edge.kind !== 'sequence')

    if (isRoutingNodeType(node.type) && branchEdges.length > 0) {
      return this.layoutRoute(node, branchEdges, stopId, nextAncestors)
    }
    if (branchEdges.length > 0 || outgoing.length > 1) {
      throw new Error(`节点 ${startId} 不符合结构化主干约束`)
    }

    const nodeWidth = this.nodeWidth(node)
    const nodeHeight = this.nodeHeight(node)
    if (outgoing.length === 0 || outgoing[0].target === stopId) {
      return {
        positions: new Map([[startId, { x: nodeWidth / 2, y: 0 }]]),
        rails: [],
        exits: [startId],
        width: nodeWidth,
        height: nodeHeight,
      }
    }

    const child = this.layoutSegment(outgoing[0].target, stopId, nextAncestors)
    const width = Math.max(nodeWidth, child.width)
    const childY = nodeHeight + flowTreeVerticalGap
    const positions = new Map<string, Placement>([[startId, { x: width / 2, y: 0 }]])
    this.appendBlock(positions, child, (width - child.width) / 2, childY)
    return {
      positions,
      rails: this.shiftRails(child.rails, childY),
      exits: child.exits,
      width,
      height: childY + child.height,
    }
  }

  private layoutRoute(node: FlowGraphNode, branchEdges: FlowGraphEdge[], stopId: string, ancestors: Set<string>): SegmentBlock {
    const mergeTargetId = node.mergeTargetId ?? ''
    const branches = branchEdges.map((edge) => ({
      edge,
      block: this.layoutSegment(edge.target, mergeTargetId, ancestors),
    }))
    const branchGroupWidth = branches.reduce((total, branch) => total + branch.block.width, 0)
      + flowTreeHorizontalGap * Math.max(0, branches.length - 1)
    const continuation = mergeTargetId && mergeTargetId !== stopId
      ? this.layoutSegment(mergeTargetId, stopId, ancestors)
      : null
    const width = Math.max(flowRoutingHubSize, branchGroupWidth, continuation?.width ?? 0)
    const forkRailY = flowRoutingHubSize + flowTreeForkOffset
    const branchTop = forkRailY + flowTreeRailToNodeGap
    const maxBranchHeight = Math.max(...branches.map((branch) => branch.block.height))
    const positions = new Map<string, Placement>([[node.id, { x: width / 2, y: 0 }]])
    const rails: RouteRail[] = []
    const branchGroupX = (width - branchGroupWidth) / 2
    const branchExits: string[] = []
    let branchX = branchGroupX

    for (const branch of branches) {
      this.appendBlock(positions, branch.block, branchX, branchTop)
      rails.push(...this.shiftRails(branch.block.rails, branchTop))
      branchExits.push(...branch.block.exits)
      branchX += branch.block.width + flowTreeHorizontalGap
    }

    const routeRail: RouteRail = {
      forkEdgeIds: branchEdges.map((edge) => edge.id),
      forkRailY,
      mergeEdgeIds: [],
    }
    let height = branchTop + maxBranchHeight
    let exits = this.unique(branchExits)

    if (mergeTargetId) {
      const mergeRailY = height + flowTreeRailToNodeGap
      routeRail.mergeRailY = mergeRailY
      routeRail.mergeEdgeIds = exits.map((source) => {
        const edge = this.edgeByPair.get(`${source}\u0000${mergeTargetId}`)
        if (!edge) throw new Error(`路由 ${node.id} 缺少到真实汇合点的边`)
        return edge.id
      })
      height = mergeRailY
      if (continuation) {
        const continuationTop = mergeRailY + flowTreeRailToNodeGap
        this.appendBlock(positions, continuation, (width - continuation.width) / 2, continuationTop)
        rails.push(...this.shiftRails(continuation.rails, continuationTop))
        height = continuationTop + continuation.height
        exits = continuation.exits
      }
    }

    rails.push(routeRail)
    return { positions, rails, exits, width, height }
  }

  private appendBlock(target: Map<string, Placement>, block: SegmentBlock, offsetX: number, offsetY: number) {
    for (const [id, position] of block.positions) {
      if (target.has(id)) throw new Error(`流程节点 ${id} 在结构化树中重复出现`)
      target.set(id, { x: position.x + offsetX, y: position.y + offsetY })
    }
  }

  private shiftRails(rails: RouteRail[], offsetY: number): RouteRail[] {
    return rails.map((rail) => ({
      ...rail,
      forkRailY: rail.forkRailY + offsetY,
      mergeRailY: rail.mergeRailY === undefined ? undefined : rail.mergeRailY + offsetY,
    }))
  }

  private unique(values: string[]): string[] {
    return [...new Set(values)]
  }

  private nodeWidth(node: FlowGraphNode): number {
    return isRoutingNodeType(node.type) ? flowRoutingHubSize : flowNodeWidth
  }

  private nodeHeight(node: FlowGraphNode): number {
    return isRoutingNodeType(node.type) ? flowRoutingHubSize : flowNodeHeight
  }

  private toVueFlowNode(node: FlowGraphNode, positions: Map<string, Placement>): Node<FlowNodeData> {
    const placement = positions.get(node.id)
    if (!placement) throw new Error(`流程节点 ${node.id} 尚未布局`)
    const routingHub = isRoutingNodeType(node.type)
    const width = this.nodeWidth(node)
    return {
      id: node.id,
      type: routingHub ? 'routingHub' : 'flowNode',
      position: { x: placement.x - width / 2, y: placement.y },
      sourcePosition: Position.Bottom,
      targetPosition: Position.Top,
      draggable: false,
      selectable: false,
      connectable: false,
      data: { name: node.name, type: node.type, typeName: node.typeName, mergeTargetId: node.mergeTargetId },
    }
  }

  private toVueFlowEdge(edge: FlowGraphEdge, positions: Map<string, Placement>, rails: RouteRail[]): Edge<FlowTreeEdgeData> {
    const sourceNode = this.nodeById.get(edge.source)
    const targetNode = this.nodeById.get(edge.target)
    const source = positions.get(edge.source)
    const target = positions.get(edge.target)
    if (!sourceNode || !targetNode || !source || !target) throw new Error(`流程边 ${edge.id} 缺少已布局节点`)

    const edgeRails = new Map<string, EdgeRail>()
    for (const rail of rails) {
      for (const edgeId of rail.forkEdgeIds) edgeRails.set(edgeId, { role: 'fork', railY: rail.forkRailY })
      for (const edgeId of rail.mergeEdgeIds) edgeRails.set(edgeId, { role: 'merge', railY: rail.mergeRailY })
    }
    const edgeRail = edgeRails.get(edge.id) ?? { role: 'main' as const }
    const sourceX = source.x
    const sourceY = source.y + this.nodeHeight(sourceNode)
    const targetX = target.x
    const targetY = target.y
    let path: string
    if (edgeRail.railY !== undefined) {
      path = `M ${sourceX} ${sourceY} V ${edgeRail.railY} H ${targetX} V ${targetY}`
    }
    else {
      if (sourceX !== targetX) throw new Error(`主干边 ${edge.id} 未保持垂直`)
      path = `M ${sourceX} ${sourceY} V ${targetY}`
    }

    return {
      id: edge.id,
      source: edge.source,
      target: edge.target,
      type: 'treeEdge',
      label: edge.label || undefined,
      selectable: false,
      animated: false,
      markerEnd: MarkerType.ArrowClosed,
      class: `flow-edge flow-edge--${edge.kind}`,
      labelStyle: { fill: 'var(--flow-label-color)', fontSize: 12 },
      labelBgStyle: { fill: 'var(--flow-surface-color)', fillOpacity: 0.94 },
      labelBgPadding: [5, 3],
      labelBgBorderRadius: 2,
      data: {
        path,
        role: edgeRail.role,
        railY: edgeRail.railY,
        labelX: edge.label ? targetX : undefined,
        labelY: edge.label && edgeRail.railY !== undefined ? edgeRail.railY - 14 : undefined,
      },
    }
  }
}

export function layoutFlowGraph(graph: FlowGraph): LaidOutFlowGraph {
  return new FlowTreeLayout(graph).layout()
}

export function safeLayoutFlowGraph(graph: FlowGraph): SafeFlowLayoutResult {
  try {
    return { layout: layoutFlowGraph(graph), error: '' }
  }
  catch {
    return { layout: null, error: flowStructureErrorMessage }
  }
}
