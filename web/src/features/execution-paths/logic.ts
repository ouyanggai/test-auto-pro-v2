import type { FlowGraph } from '../flow-graph/types.ts'
import type { ExecutionPathAnalysis, ExecutionPathChoice, ExecutionPathSummaryItem } from './types.ts'

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

  // 后端把空入口视为不可配置；前端必须同样拒绝，不能把空遍历误判成零选择完整路径。
  if (graph.entryNodeIds.length === 0) {
    return { complete: false, invalid: true, missingRouteNodeIds, reachableNodeIds, reachableEdgeIds }
  }
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

export async function refreshExecutionPathDraft(
  choices: ExecutionPathChoice[],
  readGraph: () => Promise<FlowGraph>,
) {
  // 刷新失败也必须保留独立副本，避免网络异常把用户尚未保存的选择清空。
  const preserved = choices.map((choice) => ({ ...choice }))
  try {
    const graph = await readGraph()
    const reconciled = reconcileExecutionPathChoices(graph, preserved)
    return { graph, choices: reconciled.choices, changed: true, error: null }
  }
  catch (error) {
    return { graph: null, choices: preserved, changed: true, error }
  }
}

export function nextExecutionPathRouteID(analysis: ExecutionPathAnalysis): string | null {
  // 分析器按真实入口和后端边顺序广度遍历，首个待选点因此是稳定的左到右下一步。
  return analysis.missingRouteNodeIds[0] ?? null
}

export function projectExecutionPathSummary(
  graph: FlowGraph,
  analysis: ExecutionPathAnalysis,
  choices: ExecutionPathChoice[],
): ExecutionPathSummaryItem[] {
  const selectedByRoute = new Map(choices.map((choice) => [choice.routeNodeId, choice.branchId]))
  const missing = new Set(analysis.missingRouteNodeIds)
  const nextRouteID = nextExecutionPathRouteID(analysis)
  const outgoing = new Map<string, typeof graph.edges>()
  for (const edge of graph.edges) {
    const items = outgoing.get(edge.source) ?? []
    items.push(edge)
    outgoing.set(edge.source, items)
  }

  // 摘要只投影分析器已经确认的可达节点，不自行推演第二份路径业务模型。
  return graph.nodes.flatMap((node): ExecutionPathSummaryItem[] => {
    if (!analysis.reachableNodeIds.has(node.id)) return []
    const label = node.name || node.typeName || '流程节点'
    if (node.type === 'condition' || node.type === 'manual') {
      if (missing.has(node.id)) {
        // 并行会同时暴露多个待选路由，但面板只能指向一个当前动作，避免用户误以为可以越过左侧分支。
        if (node.id === nextRouteID) {
          return [{ id: node.id, kind: 'next', label: label || '请选择分支', detail: '下一待选点' }]
        }
        return [{ id: node.id, kind: 'pending', label: label || '请选择分支', detail: '后续待选' }]
      }
      const branchID = selectedByRoute.get(node.id)
      const edge = (outgoing.get(node.id) ?? []).find((item) => item.branchId === branchID)
      return [{
        id: node.id,
        kind: 'choice',
        label: edge?.label || '已选分支',
        detail: node.type === 'condition' ? '条件分支' : '手动分支',
      }]
    }
    if (node.type === 'parallel') {
      const branches = (outgoing.get(node.id) ?? [])
        .filter((edge) => edge.kind === 'parallel')
        .map((edge) => edge.label)
        .filter(Boolean)
      return [{
        id: node.id,
        kind: 'parallel',
        label: label || '并行分支',
        detail: branches.length > 0 ? `并行必经：${branches.join('、')}` : '并行必经',
      }]
    }
    return [{ id: node.id, kind: 'node', label, detail: node.typeName || '流程节点' }]
  })
}

export function classifyExecutionPathEdges(
  graph: FlowGraph,
  analysis: ExecutionPathAnalysis,
  choices: ExecutionPathChoice[],
) {
  const selectedByRoute = new Map(choices.map((choice) => [choice.routeNodeId, choice.branchId]))
  return new Map(graph.edges.map((edge) => {
    const selectable = edge.kind === 'condition' || edge.kind === 'manual'
    const selectedBranch = selectedByRoute.get(edge.source)
    const routeReachable = analysis.reachableNodeIds.has(edge.source)
    const selected = analysis.reachableEdgeIds.has(edge.id)
    const candidate = routeReachable && selectable && !selectedBranch
    // 选定一支后，同一路由其他标签仍属于可操作候选，只弱化而不能隐藏或禁用。
    const active = selected || candidate || (routeReachable && selectable)
    const dimmed = !selected && !candidate
    return [edge.id, { selected, candidate, dimmed, active }] as const
  }))
}

export function canCreateAdditionalPath(source: FlowGraph['flowSource'], savedCount: number): boolean {
  return source === 'new' || savedCount === 0
}

export function canEnterExecutionPathSelection(options: {
  graphReady: boolean
  pathsLoaded: boolean
  pathsFailed: boolean
  hasDraft: boolean
  canCreate: boolean
}): boolean {
  // 未完成或失败的路径列表不能按空数组处理，否则已发/待发可能错误开放第二条路径。
  return options.graphReady
    && options.pathsLoaded
    && !options.pathsFailed
    && (options.hasDraft || options.canCreate)
}

export function viewportForPointNearest(
  viewport: { x: number, y: number, zoom: number },
  point: { x: number, y: number },
  container: { width: number, height: number },
  margin = 72,
  reservedRight = 0,
) {
  const safeWidth = container.width - Math.max(0, reservedRight)
  if (safeWidth <= margin * 2 || container.height <= margin * 2 || viewport.zoom <= 0) return viewport
  const screenX = point.x * viewport.zoom + viewport.x
  const screenY = point.y * viewport.zoom + viewport.y
  let x = viewport.x
  let y = viewport.y
  if (screenX < margin) x += margin - screenX
  else if (screenX > safeWidth - margin) x -= screenX - (safeWidth - margin)
  if (screenY < margin) y += margin - screenY
  else if (screenY > container.height - margin) y -= screenY - (container.height - margin)
  return { x, y, zoom: viewport.zoom }
}
