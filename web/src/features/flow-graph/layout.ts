import { graphlib, layout as dagreLayout } from '@dagrejs/dagre'
import { MarkerType, Position, type Edge, type Node } from '@vue-flow/core'

import type { FlowGraph, FlowNodeData } from './types'

export const flowNodeWidth = 180
export const flowNodeHeight = 72

export interface LaidOutFlowGraph {
  nodes: Node<FlowNodeData>[]
  edges: Edge[]
}

export function shouldFitInitialGraph(ready: boolean, fittedPlanId: string, planId: string): boolean {
  return ready && fittedPlanId !== planId
}

export function layoutFlowGraph(graph: FlowGraph): LaidOutFlowGraph {
  const dagreGraph = new graphlib.Graph().setDefaultEdgeLabel(() => ({}))
  dagreGraph.setGraph({ rankdir: 'TB', ranksep: 76, nodesep: 42, edgesep: 22, marginx: 24, marginy: 24 })

  for (const node of graph.nodes) {
    dagreGraph.setNode(node.id, { width: flowNodeWidth, height: flowNodeHeight })
  }
  for (const edge of graph.edges) {
    dagreGraph.setEdge(edge.source, edge.target)
  }
  dagreLayout(dagreGraph)

  return {
    nodes: graph.nodes.map((node) => {
      const positioned = dagreGraph.node(node.id)
      return {
        id: node.id,
        type: 'flowNode',
        position: { x: positioned.x - flowNodeWidth / 2, y: positioned.y - flowNodeHeight / 2 },
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
      type: 'smoothstep',
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
