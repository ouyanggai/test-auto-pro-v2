import type { FlowGraph } from '../flow-graph/types.ts'
import type {
  ExecutionPath,
  ExecutionPathAnalysis,
  ExecutionPathChoice,
  ExecutionPathDecisionProgress,
  ExecutionPathSummaryItem,
  ExecutionPathWorkspaceMode,
  ExecutionPathWorkspaceDisposition,
  ExecutionPathWorkspacePresentation,
} from './types.ts'

const selectableKinds = new Set(['condition', 'manual'])

export type ExecutionPathRunReadiness = 'ready' | 'configuration' | 'data'

// executionPathRunReadiness 只消费路径双状态，供未来运行预检和当前配置提示共用。
export function executionPathRunReadiness(path: ExecutionPath): ExecutionPathRunReadiness {
  if (path.configurationStatus !== 'configured') return 'configuration'
  if (path.dataStatus === 'not_required' || path.dataStatus === 'generated' || path.dataStatus === 'confirmed') return 'ready'
  return 'data'
}

// isExecutionPathRunnable 判断路径是否同时满足节点配置和数据准备要求。
export function isExecutionPathRunnable(path: ExecutionPath): boolean {
  return executionPathRunReadiness(path) === 'ready'
}

// summarizeExecutionPathConfiguration 只按路径列表携带的本地状态统计配置进度，不触发目标平台或完整分析。
export function summarizeExecutionPathConfiguration(paths: ExecutionPath[]): { total: number, configured: number, partial: number, pending: number, nextPath: ExecutionPath | null } {
  const configured = paths.filter((path) => path.configurationStatus === 'configured').length
  const partial = paths.filter((path) => path.configurationStatus === 'partial').length
  const pending = paths.filter((path) => path.configurationStatus === 'pending' || path.configurationStatus === 'affected').length
  return { total: paths.length, configured, partial, pending, nextPath: paths.find((path) => path.configurationStatus !== 'configured') ?? null }
}

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

export function deriveExecutionPathDecisionProgress(
  graph: FlowGraph,
  analysis: ExecutionPathAnalysis,
  choices: ExecutionPathChoice[],
): ExecutionPathDecisionProgress {
  const selectableNodeIDs = new Set(
    graph.nodes
      .filter((node) => selectableKinds.has(node.type))
      .map((node) => node.id),
  )
  // 只统计分析器已证明可达的人工决策点；并行路由由系统自动纳入，不能抬高用户需要完成的数量。
  const selected = new Set(
    choices
      .map((choice) => choice.routeNodeId)
      .filter((routeNodeID) => selectableNodeIDs.has(routeNodeID) && analysis.reachableNodeIds.has(routeNodeID)),
  ).size
  const pending = new Set(
    analysis.missingRouteNodeIds.filter((routeNodeID) => selectableNodeIDs.has(routeNodeID)),
  ).size
  return { selected, pending, total: selected + pending }
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

function executionPathChoiceSignature(choices: ExecutionPathChoice[]): string {
  return [...choices]
    .sort((left, right) => left.routeNodeId.localeCompare(right.routeNodeId) || left.branchId.localeCompare(right.branchId))
    .map((choice) => `${choice.routeNodeId.length}:${choice.routeNodeId}${choice.branchId.length}:${choice.branchId}`)
    .join(';')
}

export function hasExecutionPathDraftChanges(
  mode: ExecutionPathWorkspaceMode,
  name: string,
  choices: ExecutionPathChoice[],
  savedPath: ExecutionPath | null,
): boolean {
  if (!mode) return false
  if (mode === 'view') return false
  if (mode === 'new' || mode === 'copy') {
    return name.trim() !== '' || choices.length > 0
  }
  if (!savedPath) return name.trim() !== '' || choices.length > 0
  const savedName = savedPath.name?.trim() || `路径 ${savedPath.sequenceNo}`
  // 分支选择的传输顺序不是业务事实；切换保护只比较规范化集合，避免同一线路因数组顺序变化被误报为未保存。
  return name.trim() !== savedName
    || executionPathChoiceSignature(choices) !== executionPathChoiceSignature(savedPath.choices)
}

export function deriveExecutionPathWorkspacePresentation(options: {
  mode: ExecutionPathWorkspaceMode
  dirty: boolean
  remainingChoices: number
  invalid: boolean
  changedByGraph: boolean
}): ExecutionPathWorkspacePresentation {
  const branchEditing = options.mode === 'edit' || options.mode === 'new' || options.mode === 'copy'
  const title = options.mode === 'edit'
    ? '编辑路径'
    : options.mode === 'new'
      ? '新建路径'
      : options.mode === 'copy'
        ? '复制路径'
        : '路径详情'
  let hint = '已保存'
  if (branchEditing) {
    if (options.changedByGraph || options.invalid) hint = '当前选择已失效，需要重新选择'
    else if (options.remainingChoices > 0) hint = `还需选择 ${options.remainingChoices} 处`
    else if (options.dirty || options.mode === 'new' || options.mode === 'copy') hint = '线路选择已完成，保存后生效'
  }
  // 编辑已保存路径时，没有真实变化就保持安静的已保存状态，也不暴露无意义的保存操作。
  if (options.mode === 'edit' && options.dirty && options.remainingChoices === 0 && !options.invalid && !options.changedByGraph) {
    hint = '修改未保存'
  }
  return {
    title,
    branchEditing,
    dirty: options.dirty,
    showNameInput: branchEditing,
    showSave: branchEditing && (options.mode === 'new' || options.mode === 'copy' || options.dirty),
    hint,
  }
}

export function transitionExecutionPathWorkspace(
  current: ExecutionPathWorkspaceMode,
  action: 'select-saved' | 'edit' | 'new' | 'copy',
): ExecutionPathWorkspaceMode {
  // 已保存路径无论来自首次加载、切换还是保存响应，都统一落到查看态，避免成功后残留编辑和保存提示。
  if (action === 'select-saved') return 'view'
  if (action === 'edit') return current === 'view' ? 'edit' : current
  return action
}

export function deriveExecutionPathWorkspaceDisposition(
  action: 'cancel' | 'fullscreen-exit' | 'save-success' | 'save-failure',
  dirty: boolean,
): ExecutionPathWorkspaceDisposition {
  // 网络或目标图失败必须保留同一草稿和幂等键；只有成功或用户明确放弃才能清空本地编辑状态。
  if (action === 'save-failure') return 'preserve'
  if (action === 'save-success') return 'reset'
  return dirty ? 'confirm' : 'reset'
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

export function viewportForPointCentered(
  viewport: { x: number, y: number, zoom: number },
  point: { x: number, y: number },
  container: { width: number, height: number },
  reservedRight = 0,
) {
  const safeWidth = container.width - Math.max(0, reservedRight)
  if (safeWidth <= 0 || container.height <= 0 || viewport.zoom <= 0) return viewport
  // 下一步始终落在扣除侧栏后的操作区正中央，用户不需要再根据气泡自行寻找目标。
  return {
    x: safeWidth / 2 - point.x * viewport.zoom,
    y: container.height / 2 - point.y * viewport.zoom,
    zoom: viewport.zoom,
  }
}

export interface ExecutionPathGuideCandidate {
  id: string
  x: number
  y: number
}

export interface ExecutionPathGuideProjection {
  bubble: { x: number, y: number }
  visibleCandidates: ExecutionPathGuideCandidate[]
  hiddenLeftCount: number
  hiddenRightCount: number
}

export function viewportForCandidateGroupCentered(
  viewport: { x: number, y: number, zoom: number },
  candidates: ExecutionPathGuideCandidate[],
  container: { width: number, height: number },
  reservedRight = 0,
) {
  const finiteCandidates = candidates.filter((candidate) => Number.isFinite(candidate.x) && Number.isFinite(candidate.y))
  const safeWidth = container.width - Math.max(0, reservedRight)
  if (finiteCandidates.length === 0 || safeWidth <= 0 || container.height <= 0 || viewport.zoom <= 0) return viewport
  const xs = finiteCandidates.map((candidate) => candidate.x)
  const ys = finiteCandidates.map((candidate) => candidate.y)
  // 候选可能横跨多个分支列；用整体包围盒中心而不是首个标签，才能让三支及更多分支获得对称的可操作空间。
  const center = {
    x: (Math.min(...xs) + Math.max(...xs)) / 2,
    y: (Math.min(...ys) + Math.max(...ys)) / 2,
  }
  return {
    x: safeWidth / 2 - center.x * viewport.zoom,
    y: container.height / 2 - center.y * viewport.zoom,
    zoom: viewport.zoom,
  }
}

export function projectExecutionPathGuide(
  candidates: ExecutionPathGuideCandidate[],
  viewport: { x: number, y: number, zoom: number },
  container: { width: number, height: number },
  reservedRight = 0,
  horizontalMargin = 44,
): ExecutionPathGuideProjection {
  const safeWidth = Math.max(0, container.width - Math.max(0, reservedRight))
  const projected = candidates
    .filter((candidate) => Number.isFinite(candidate.x) && Number.isFinite(candidate.y))
    .map((candidate) => ({
      id: candidate.id,
      x: candidate.x * viewport.zoom + viewport.x,
      y: candidate.y * viewport.zoom + viewport.y,
    }))
  const hiddenLeftCount = projected.filter((candidate) => candidate.x < horizontalMargin).length
  const hiddenRightCount = projected.filter((candidate) => candidate.x > safeWidth - horizontalMargin).length
  const visibleCandidates = projected.filter((candidate) => (
    candidate.x >= horizontalMargin
    && candidate.x <= safeWidth - horizontalMargin
    && candidate.y >= 24
    && candidate.y <= container.height - 24
  ))
  const visibleXs = visibleCandidates.map((candidate) => candidate.x)
  const visibleYs = visibleCandidates.map((candidate) => candidate.y)
  const fallbackX = safeWidth / 2
  const fallbackY = Math.max(58, container.height / 2 - 76)
  return {
    bubble: {
      x: visibleXs.length > 0 ? (Math.min(...visibleXs) + Math.max(...visibleXs)) / 2 : fallbackX,
      y: visibleYs.length > 0 ? Math.max(58, Math.min(...visibleYs) - 76) : fallbackY,
    },
    visibleCandidates,
    hiddenLeftCount,
    hiddenRightCount,
  }
}
