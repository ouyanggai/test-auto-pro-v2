import type { FlowGraph } from '../flow-graph/types.ts'
import type { ExecutionPathAnalysis, ExecutionPathChoice } from './types.ts'

const selectableKinds = new Set(['condition', 'manual'])

export function analyzeExecutionPath(graph: FlowGraph, choices: ExecutionPathChoice[]): ExecutionPathAnalysis {
  const nodes = new Map(graph.nodes.map((node) => [node.id, node]))
  const outgoing = new Map<string, typeof graph.edges>()
  for (const edge of graph.edges) {
    const items = outgoing.get(edge.source) ?? []
    items.push(edge)
    outgoing.set(edge.source, items)
  }
  const choiceByRoute = new Map<string, string>()
  let invalid = false
  for (const choice of choices) {
    if (!choice.routeNodeId || !choice.branchId || choiceByRoute.has(choice.routeNodeId)) invalid = true
    choiceByRoute.set(choice.routeNodeId, choice.branchId)
  }
  const reachableNodeIds = new Set<string>()
  const reachableEdgeIds = new Set<string>()
  const missingRouteNodeIds: string[] = []
  const usedChoices = new Set<string>()
  const queue = [...graph.entryNodeIds]
  while (queue.length > 0 && !invalid) {
    const nodeId = queue.shift()!
    if (reachableNodeIds.has(nodeId)) continue
    const node = nodes.get(nodeId)
    if (!node) {
      invalid = true
      break
    }
    reachableNodeIds.add(nodeId)
    const edges = outgoing.get(nodeId) ?? []
    if (selectableKinds.has(node.type)) {
      const selectedBranch = choiceByRoute.get(nodeId)
      if (!selectedBranch) {
        missingRouteNodeIds.push(nodeId)
        continue
      }
      const selectedEdge = edges.find((edge) => edge.kind === node.type && edge.branchId === selectedBranch)
      if (!selectedEdge) {
        invalid = true
        break
      }
      usedChoices.add(nodeId)
      reachableEdgeIds.add(selectedEdge.id)
      queue.push(selectedEdge.target)
      continue
    }
    if (node.type === 'parallel') {
      const parallelEdges = edges.filter((edge) => edge.kind === 'parallel')
      if (parallelEdges.length !== edges.length || parallelEdges.length === 0) {
        invalid = true
        break
      }
      for (const edge of parallelEdges) {
        reachableEdgeIds.add(edge.id)
        queue.push(edge.target)
      }
      continue
    }
    if (edges.length > 1 || edges.some((edge) => edge.kind !== 'sequence')) {
      invalid = true
      break
    }
    if (edges[0]) {
      reachableEdgeIds.add(edges[0].id)
      queue.push(edges[0].target)
    }
  }
  if (usedChoices.size !== choiceByRoute.size) invalid = true
  return {
    complete: !invalid && missingRouteNodeIds.length === 0,
    invalid,
    missingRouteNodeIds,
    reachableNodeIds,
    reachableEdgeIds,
  }
}

export function replaceExecutionPathChoice(
  choices: ExecutionPathChoice[],
  routeNodeId: string,
  branchId: string,
): ExecutionPathChoice[] {
  return [
    ...choices.filter((choice) => choice.routeNodeId !== routeNodeId),
    { routeNodeId, branchId },
  ]
}

export function applyExecutionPathChoice(
  graph: FlowGraph,
  choices: ExecutionPathChoice[],
  routeNodeId: string,
  branchId: string,
): ExecutionPathChoice[] {
  const replaced = replaceExecutionPathChoice(choices, routeNodeId, branchId)
  const firstPass = analyzeExecutionPath(graph, replaced)
  return replaced.filter((choice) => firstPass.reachableNodeIds.has(choice.routeNodeId))
}

export function reconcileExecutionPathChoices(graph: FlowGraph, choices: ExecutionPathChoice[]) {
  const branchPairs = new Set(
    graph.edges
      .filter((edge) => edge.kind === 'condition' || edge.kind === 'manual')
      .map((edge) => `${edge.source}\u0000${edge.branchId}`),
  )
  const unique = new Map<string, ExecutionPathChoice>()
  let changed = false
  for (const choice of choices) {
    const key = `${choice.routeNodeId}\u0000${choice.branchId}`
    if (!branchPairs.has(key) || unique.has(choice.routeNodeId)) {
      changed = true
      continue
    }
    unique.set(choice.routeNodeId, choice)
  }
  const validPairs = [...unique.values()]
  const analysis = analyzeExecutionPath(graph, validPairs)
  const reachable = validPairs.filter((choice) => analysis.reachableNodeIds.has(choice.routeNodeId))
  if (reachable.length !== choices.length) changed = true
  return { choices: reachable, changed }
}

export function canCreateAdditionalPath(source: FlowGraph['flowSource'], savedCount: number): boolean {
  return source === 'new' || savedCount === 0
}

export function viewportForPointNearest(
  viewport: { x: number, y: number, zoom: number },
  point: { x: number, y: number },
  container: { width: number, height: number },
  margin = 72,
) {
  if (container.width <= margin * 2 || container.height <= margin * 2 || viewport.zoom <= 0) return viewport
  const screenX = point.x * viewport.zoom + viewport.x
  const screenY = point.y * viewport.zoom + viewport.y
  let x = viewport.x
  let y = viewport.y
  if (screenX < margin) x += margin - screenX
  else if (screenX > container.width - margin) x -= screenX - (container.width - margin)
  if (screenY < margin) y += margin - screenY
  else if (screenY > container.height - margin) y -= screenY - (container.height - margin)
  return { x, y, zoom: viewport.zoom }
}
