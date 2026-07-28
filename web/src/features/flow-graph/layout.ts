import { graphlib, layout as dagreLayout } from '@dagrejs/dagre'
import { MarkerType, Position, type Edge, type Node } from '@vue-flow/core'

import type { FlowGraph, FlowNodeData } from './types'

export const flowNodeWidth = 180
export const flowNodeHeight = 72
export const flowRoutingHubSize = 8
export const flowNodeHorizontalGap = 112
export const flowNodeVerticalGap = 124
export const initialFlowZoom = 0.9

const routingNodeTypes = new Set(['condition', 'manual', 'parallel'])

export interface LaidOutFlowGraph {
  nodes: Node<FlowNodeData>[]
  edges: Edge[]
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

export function layoutFlowGraph(graph: FlowGraph): LaidOutFlowGraph {
  const dagreGraph = new graphlib.Graph({ multigraph: true }).setDefaultEdgeLabel(() => ({}))
  dagreGraph.setGraph({
    rankdir: 'TB',
    ranksep: flowNodeVerticalGap,
    nodesep: flowNodeHorizontalGap,
    edgesep: 42,
    marginx: 36,
    marginy: 36,
  })

  for (const node of graph.nodes) {
    const routingHub = isRoutingNodeType(node.type)
    dagreGraph.setNode(node.id, {
      width: routingHub ? flowRoutingHubSize : flowNodeWidth,
      height: routingHub ? flowRoutingHubSize : flowNodeHeight,
    })
  }
  // Dagre 的 TB 排序会把同源边按插入顺序反向排布；逆序输入可稳定恢复后端边的左到右语义。
  for (const edge of [...graph.edges].reverse()) {
    dagreGraph.setEdge(edge.source, edge.target, {}, edge.id)
  }
  dagreLayout(dagreGraph)

  return {
    nodes: graph.nodes.map((node) => {
      const positioned = dagreGraph.node(node.id)
      const routingHub = isRoutingNodeType(node.type)
      const width = routingHub ? flowRoutingHubSize : flowNodeWidth
      const height = routingHub ? flowRoutingHubSize : flowNodeHeight
      return {
        id: node.id,
        type: routingHub ? 'routingHub' : 'flowNode',
        position: { x: positioned.x - width / 2, y: positioned.y - height / 2 },
        sourcePosition: Position.Bottom,
        targetPosition: Position.Top,
        draggable: false,
        selectable: false,
        connectable: false,
        data: { name: node.name, type: node.type, typeName: node.typeName },
      }
    }),
    edges: graph.edges.map((edge) => ({
      id: edge.id,
      source: edge.source,
      target: edge.target,
      type: 'step',
      label: edge.label || undefined,
      selectable: false,
      animated: false,
      markerEnd: MarkerType.ArrowClosed,
      class: `flow-edge flow-edge--${edge.kind}`,
      labelStyle: { fill: 'var(--flow-label-color)', fontSize: 12 },
      labelBgStyle: { fill: 'var(--flow-surface-color)', fillOpacity: 0.94 },
      labelBgPadding: [5, 3],
      labelBgBorderRadius: 2,
    })),
  }
}
