import assert from 'node:assert/strict'
import test from 'node:test'

import {
  analyzeExecutionPath,
  applyExecutionPathChoice,
  canCreateAdditionalPath,
  canEnterExecutionPathSelection,
  classifyExecutionPathEdges,
  deriveExecutionPathWorkspacePresentation,
  deriveExecutionPathDecisionProgress,
  deriveExecutionPathWorkspaceDisposition,
  hasExecutionPathDraftChanges,
  nextExecutionPathRouteID,
  previewAllExecutionPaths,
  projectExecutionPathSummary,
  summarizeExecutionPathConfiguration,
  projectExecutionPathGuide,
  reconcileExecutionPathChoices,
  refreshExecutionPathDraft,
  transitionExecutionPathWorkspace,
  viewportForCandidateGroupCentered,
  viewportForPointNearest,
  viewportForPointCentered,
} from '../../../web/src/features/execution-paths/logic.ts'
import {
  createExecutionPath,
  deleteExecutionPath,
  ExecutionPathApiError,
  fetchExecutionPaths,
  generateAllExecutionPaths,
  updateExecutionPath,
} from '../../../web/src/features/execution-paths/api.ts'

const graph = {
  planId: '7',
  targetName: '采购审批',
  flowSource: 'new',
  entryNodeIds: ['start'],
  nodes: [
    { id: 'start', name: '发起', type: 'start', typeName: '发起' },
    { id: 'route-a', name: '', type: 'condition', typeName: '条件' },
    { id: 'parallel', name: '', type: 'parallel', typeName: '并行' },
    { id: 'left-route', name: '', type: 'manual', typeName: '手动' },
    { id: 'left-end', name: '左结束', type: 'end', typeName: '结束' },
    { id: 'right-end', name: '右结束', type: 'end', typeName: '结束' },
    { id: 'other-end', name: '另一结束', type: 'end', typeName: '结束' },
  ],
  edges: [
    { id: 'start-route', source: 'start', target: 'route-a', kind: 'sequence', label: '', branchId: '' },
    { id: 'route-parallel', source: 'route-a', target: 'parallel', kind: 'condition', label: '进入并行', branchId: 'branch-a' },
    { id: 'route-other', source: 'route-a', target: 'other-end', kind: 'condition', label: '直接结束', branchId: 'branch-b' },
    { id: 'parallel-left', source: 'parallel', target: 'left-route', kind: 'parallel', label: '左', branchId: 'parallel-left' },
    { id: 'parallel-right', source: 'parallel', target: 'right-end', kind: 'parallel', label: '右', branchId: 'parallel-right' },
    { id: 'left-a', source: 'left-route', target: 'left-end', kind: 'manual', label: '同意', branchId: 'manual-a' },
    { id: 'left-b', source: 'left-route', target: 'other-end', kind: 'manual', label: '转交', branchId: 'manual-b' },
  ],
  warnings: [],
}

test('路径选择只要求当前可达路由且并行分支全部纳入', () => {
  const initial = analyzeExecutionPath(graph, [])
  assert.equal(initial.complete, false)
  assert.deepEqual(initial.missingRouteNodeIds, ['route-a'])

  const enteredParallel = analyzeExecutionPath(graph, [{ routeNodeId: 'route-a', branchId: 'branch-a' }])
  assert.equal(enteredParallel.complete, false)
  assert.deepEqual(enteredParallel.missingRouteNodeIds, ['left-route'])
  assert.equal(enteredParallel.reachableEdgeIds.has('parallel-left'), true)
  assert.equal(enteredParallel.reachableEdgeIds.has('parallel-right'), true)

  const complete = analyzeExecutionPath(graph, [
    { routeNodeId: 'route-a', branchId: 'branch-a' },
    { routeNodeId: 'left-route', branchId: 'manual-a' },
  ])
  assert.equal(complete.complete, true)
  assert.equal(complete.invalid, false)
})

test('空入口与下一待选点顺序保持后端一致', () => {
  const emptyEntry = analyzeExecutionPath({ ...graph, entryNodeIds: [] }, [])
  assert.equal(emptyEntry.complete, false)
  assert.equal(emptyEntry.invalid, true)

  const initial = analyzeExecutionPath(graph, [])
  assert.equal(nextExecutionPathRouteID(initial), 'route-a')
  const enteredParallel = analyzeExecutionPath(graph, [{ routeNodeId: 'route-a', branchId: 'branch-a' }])
  assert.equal(nextExecutionPathRouteID(enteredParallel), 'left-route')
})

test('改选上游分支会清理已不可达选择且不会猜测新分支', () => {
  const original = [
    { routeNodeId: 'route-a', branchId: 'branch-a' },
    { routeNodeId: 'left-route', branchId: 'manual-a' },
  ]
  const replaced = applyExecutionPathChoice(graph, original, 'route-a', 'branch-b')
  assert.deepEqual(replaced, [{ routeNodeId: 'route-a', branchId: 'branch-b' }])
  assert.equal(analyzeExecutionPath(graph, replaced).complete, true)
})

test('真实图变化时只保留仍可对应且可达的选择', () => {
  const result = reconcileExecutionPathChoices(graph, [
    { routeNodeId: 'route-a', branchId: 'branch-a' },
    { routeNodeId: 'left-route', branchId: 'removed' },
    { routeNodeId: 'foreign-route', branchId: 'foreign-branch' },
  ])
  assert.equal(result.changed, true)
  assert.deepEqual(result.choices, [{ routeNodeId: 'route-a', branchId: 'branch-a' }])
})

test('失效保存刷新图时协调有效选择，刷新失败仍保留原草稿', async () => {
  const draft = [
    { routeNodeId: 'route-a', branchId: 'branch-a' },
    { routeNodeId: 'left-route', branchId: 'removed' },
  ]
  const refreshed = await refreshExecutionPathDraft(draft, async () => graph)
  assert.equal(refreshed.error, null)
  assert.equal(refreshed.changed, true)
  assert.deepEqual(refreshed.choices, [{ routeNodeId: 'route-a', branchId: 'branch-a' }])

  const failed = await refreshExecutionPathDraft(draft, async () => { throw new Error('unavailable') })
  assert.equal(failed.graph, null)
  assert.equal(failed.changed, true)
  assert.deepEqual(failed.choices, draft)
  assert.notEqual(failed.choices, draft)
})

test('实时路径摘要只投影可达节点、选择、并行必经和下一待选点', () => {
  const choices = [{ routeNodeId: 'route-a', branchId: 'branch-a' }]
  const analysis = analyzeExecutionPath(graph, choices)
  const summary = projectExecutionPathSummary(graph, analysis, choices)
  assert.ok(summary.some((item) => item.kind === 'node' && item.label === '发起'))
  assert.ok(summary.some((item) => item.kind === 'choice' && item.label === '进入并行'))
  assert.ok(summary.some((item) => item.kind === 'parallel' && item.detail.includes('并行必经')))
  assert.ok(summary.some((item) => item.kind === 'next' && item.id === 'left-route'))
  assert.equal(summary.some((item) => item.id === 'other-end'), false)
})

test('并行待选路由只突出一个下一步，其他路由稳定显示为后续待选', () => {
  const parallelGraph = {
    ...graph,
    entryNodeIds: ['parallel'],
    nodes: [
      { id: 'parallel', name: '并行开始', type: 'parallel', typeName: '并行' },
      { id: 'left-route', name: '左侧审批', type: 'condition', typeName: '条件' },
      { id: 'right-route', name: '右侧审批', type: 'manual', typeName: '手动' },
      { id: 'left-end', name: '左结束', type: 'end', typeName: '结束' },
      { id: 'right-end', name: '右结束', type: 'end', typeName: '结束' },
    ],
    edges: [
      { id: 'parallel-left', source: 'parallel', target: 'left-route', kind: 'parallel', label: '左支线', branchId: 'parallel-left' },
      { id: 'parallel-right', source: 'parallel', target: 'right-route', kind: 'parallel', label: '右支线', branchId: 'parallel-right' },
      { id: 'left-approve', source: 'left-route', target: 'left-end', kind: 'condition', label: '同意', branchId: 'left-approve' },
      { id: 'right-approve', source: 'right-route', target: 'right-end', kind: 'manual', label: '通过', branchId: 'right-approve' },
    ],
  }
  const analysis = analyzeExecutionPath(parallelGraph, [])
  const summary = projectExecutionPathSummary(parallelGraph, analysis, [])

  assert.deepEqual(analysis.missingRouteNodeIds, ['left-route', 'right-route'])
  assert.equal(nextExecutionPathRouteID(analysis), 'left-route')
  assert.deepEqual(summary.filter((item) => item.kind === 'next').map((item) => item.id), ['left-route'])
  assert.deepEqual(summary.filter((item) => item.kind === 'pending').map((item) => item.id), ['right-route'])
})

test('线路边明确区分已选、待选和弱化且弱化分支仍保持活动', () => {
  const initialAnalysis = analyzeExecutionPath(graph, [])
  const initialStates = classifyExecutionPathEdges(graph, initialAnalysis, [])
  assert.equal(initialStates.get('route-parallel').candidate, true)
  assert.equal(initialStates.get('route-other').candidate, true)
  assert.equal(initialStates.get('parallel-left').dimmed, true)

  const choices = [{ routeNodeId: 'route-a', branchId: 'branch-a' }]
  const selectedAnalysis = analyzeExecutionPath(graph, choices)
  const selectedStates = classifyExecutionPathEdges(graph, selectedAnalysis, choices)
  assert.equal(selectedStates.get('route-parallel').selected, true)
  assert.equal(selectedStates.get('route-other').dimmed, true)
  assert.equal(selectedStates.get('route-other').active, true)
  assert.equal(selectedStates.get('route-other').candidate, false)
  assert.equal(selectedStates.get('parallel-left').selected, true)
  assert.equal(selectedStates.get('parallel-right').selected, true)
  assert.equal(selectedStates.get('left-a').candidate, true)
})

test('新发起允许多路径而已发待发最多一条', () => {
  assert.equal(canCreateAdditionalPath('new', 3), true)
  assert.equal(canCreateAdditionalPath('started', 0), true)
  assert.equal(canCreateAdditionalPath('started', 1), false)
  assert.equal(canCreateAdditionalPath('pending', 1), false)
})

test('路径列表未完成或失败时不能进入选择模式', () => {
  const ready = { graphReady: true, pathsLoaded: true, pathsFailed: false, hasDraft: false, canCreate: true }
  assert.equal(canEnterExecutionPathSelection(ready), true)
  assert.equal(canEnterExecutionPathSelection({ ...ready, pathsLoaded: false }), false)
  assert.equal(canEnterExecutionPathSelection({ ...ready, pathsFailed: true }), false)
  assert.equal(canEnterExecutionPathSelection({ ...ready, graphReady: false }), false)
})

test('路径准备摘要只依据本地配置状态并稳定选择下一条', () => {
  const paths = [
    { id: '1', sequenceNo: 1, name: '一', configurationStatus: 'configured', choices: [], updatedAt: '' },
    { id: '2', sequenceNo: 2, name: '二', configurationStatus: 'pending', choices: [], updatedAt: '' },
    { id: '3', sequenceNo: 3, name: '三', configurationStatus: 'pending', choices: [], updatedAt: '' },
  ]
  const summary = summarizeExecutionPathConfiguration(paths)
  assert.deepEqual({ total: summary.total, configured: summary.configured, pending: summary.pending }, { total: 3, configured: 1, pending: 2 })
  assert.equal(summary.nextPath.id, '2')
})

test('路径切换只在名称或线路真实变化时保护草稿', () => {
  const saved = {
    id: '31', sequenceNo: 3, name: '重点路径', updatedAt: '',
    choices: [{ routeNodeId: 'route-a', branchId: 'branch-a' }],
  }
  assert.equal(hasExecutionPathDraftChanges('edit', '重点路径', [...saved.choices], saved), false)
  assert.equal(hasExecutionPathDraftChanges('edit', '改名路径', [...saved.choices], saved), true)
  assert.equal(hasExecutionPathDraftChanges('edit', '重点路径', [{ routeNodeId: 'route-a', branchId: 'branch-b' }], saved), true)
  assert.equal(hasExecutionPathDraftChanges('new', '', [], null), false)
  assert.equal(hasExecutionPathDraftChanges('new', '新草稿', [], null), true)
  assert.equal(hasExecutionPathDraftChanges('copy', '', [...saved.choices], null), true)
  assert.equal(hasExecutionPathDraftChanges('view', '误改名称', [], saved), false)
})

test('已保存路径加载和保存后统一进入查看态', () => {
  assert.equal(transitionExecutionPathWorkspace(null, 'select-saved'), 'view')
  assert.equal(transitionExecutionPathWorkspace('new', 'select-saved'), 'view')
  assert.equal(transitionExecutionPathWorkspace('copy', 'select-saved'), 'view')
  assert.equal(transitionExecutionPathWorkspace('view', 'edit'), 'edit')
  assert.equal(transitionExecutionPathWorkspace('new', 'edit'), 'new')
})

test('决策进度只统计当前可达的条件和手动分支', () => {
  const choices = [{ routeNodeId: 'route-a', branchId: 'branch-a' }]
  const analysis = analyzeExecutionPath(graph, choices)
  const progress = deriveExecutionPathDecisionProgress(graph, analysis, choices)
  assert.deepEqual(progress, { selected: 1, pending: 1, total: 2 })
  assert.equal(progress.selected + progress.pending, progress.total)
  assert.equal(graph.nodes.some((node) => node.type === 'parallel'), true)
})

test('取消退出和保存结果统一决定草稿复位边界', () => {
  assert.equal(deriveExecutionPathWorkspaceDisposition('cancel', false), 'reset')
  assert.equal(deriveExecutionPathWorkspaceDisposition('fullscreen-exit', true), 'confirm')
  assert.equal(deriveExecutionPathWorkspaceDisposition('save-success', true), 'reset')
  assert.equal(deriveExecutionPathWorkspaceDisposition('save-failure', true), 'preserve')
})

test('路径工作区只在真实编辑变化时显示保存语义', () => {
  const view = deriveExecutionPathWorkspacePresentation({
    mode: 'view', dirty: false, remainingChoices: 0, invalid: false, changedByGraph: false,
  })
  assert.deepEqual(view, {
    title: '路径详情',
    branchEditing: false,
    dirty: false,
    showNameInput: false,
    showSave: false,
    hint: '已保存',
  })

  const unchangedEdit = deriveExecutionPathWorkspacePresentation({
    mode: 'edit', dirty: false, remainingChoices: 0, invalid: false, changedByGraph: false,
  })
  assert.equal(unchangedEdit.title, '编辑路径')
  assert.equal(unchangedEdit.branchEditing, true)
  assert.equal(unchangedEdit.showNameInput, true)
  assert.equal(unchangedEdit.showSave, false)
  assert.equal(unchangedEdit.hint, '已保存')

  const renamed = deriveExecutionPathWorkspacePresentation({
    mode: 'edit', dirty: true, remainingChoices: 0, invalid: false, changedByGraph: false,
  })
  assert.equal(renamed.showSave, true)
  assert.equal(renamed.hint, '修改未保存')

  const changedLine = deriveExecutionPathWorkspacePresentation({
    mode: 'edit', dirty: true, remainingChoices: 2, invalid: false, changedByGraph: false,
  })
  assert.equal(changedLine.showSave, true)
  assert.equal(changedLine.hint, '还需选择 2 处')

  const savedAgain = deriveExecutionPathWorkspacePresentation({
    mode: transitionExecutionPathWorkspace('edit', 'select-saved'),
    dirty: false,
    remainingChoices: 0,
    invalid: false,
    changedByGraph: false,
  })
  assert.equal(savedAgain.showSave, false)
  assert.equal(savedAgain.hint, '已保存')
})

test('下一待选点保持缩放并移动到扣除面板后的操作区中央', () => {
  const viewport = { x: 0, y: 0, zoom: 0.9 }
  assert.deepEqual(viewportForPointNearest(viewport, { x: 200, y: 200 }, { width: 1000, height: 560 }), viewport)
  const moved = viewportForPointNearest(viewport, { x: 1800, y: 900 }, { width: 1000, height: 560 })
  assert.equal(moved.zoom, viewport.zoom)
  assert.ok(moved.x < viewport.x)
  assert.ok(moved.y < viewport.y)

  const panelSafe = viewportForPointNearest(viewport, { x: 820, y: 200 }, { width: 1000, height: 560 }, 72, 336)
  assert.ok(panelSafe.x < viewport.x)
  assert.equal(panelSafe.y, viewport.y)
  assert.equal(panelSafe.zoom, viewport.zoom)

  const centered = viewportForPointCentered(viewport, { x: 820, y: 200 }, { width: 1000, height: 560 }, 336)
  assert.equal(centered.zoom, viewport.zoom)
  assert.equal(820 * centered.zoom + centered.x, (1000 - 336) / 2)
  assert.equal(200 * centered.zoom + centered.y, 560 / 2)
})

test('多候选引导按整体中心定位并为三个可见分支生成三个箭头目标', () => {
  const viewport = { x: -120, y: -40, zoom: 0.9 }
  const candidates = [
    { id: 'left', x: 200, y: 320 },
    { id: 'middle', x: 500, y: 320 },
    { id: 'right', x: 800, y: 320 },
  ]
  const centered = viewportForCandidateGroupCentered(viewport, candidates, { width: 1200, height: 680 }, 336)
  assert.equal(centered.zoom, viewport.zoom)
  const projection = projectExecutionPathGuide(candidates, centered, { width: 1200, height: 680 }, 336)
  assert.equal(projection.visibleCandidates.length, 3)
  assert.deepEqual(projection.visibleCandidates.map((candidate) => candidate.id), ['left', 'middle', 'right'])
  assert.equal(projection.hiddenLeftCount, 0)
  assert.equal(projection.hiddenRightCount, 0)
  assert.equal((projection.visibleCandidates[0].x + projection.visibleCandidates[2].x) / 2, (1200 - 336) / 2)
})

test('候选组过宽时保持缩放并分别统计左右屏外数量', () => {
  const candidates = [
    { id: 'far-left', x: -500, y: 300 },
    { id: 'left', x: 100, y: 300 },
    { id: 'right', x: 900, y: 300 },
    { id: 'far-right', x: 1500, y: 300 },
  ]
  const viewport = viewportForCandidateGroupCentered({ x: 0, y: 0, zoom: 1 }, candidates, { width: 900, height: 600 }, 320)
  assert.equal(viewport.zoom, 1)
  const projection = projectExecutionPathGuide(candidates, viewport, { width: 900, height: 600 }, 320)
  assert.equal(projection.hiddenLeftCount, 2)
  assert.equal(projection.hiddenRightCount, 2)
  assert.equal(projection.visibleCandidates.length, 0)
})

test('画布平移缩放后引导目标使用最新视口坐标', () => {
  const candidates = [{ id: 'branch', x: 300, y: 240 }]
  const first = projectExecutionPathGuide(candidates, { x: 0, y: 0, zoom: 1 }, { width: 900, height: 600 }, 320)
  const moved = projectExecutionPathGuide(candidates, { x: 30, y: -20, zoom: 1.2 }, { width: 900, height: 600 }, 320)
  assert.deepEqual(first.visibleCandidates[0], { id: 'branch', x: 300, y: 240 })
  assert.deepEqual(moved.visibleCandidates[0], { id: 'branch', x: 390, y: 268 })
})

test('全路径预览按完整组合过滤已保存线路并在第129条停止', () => {
  const saved = [{ id: '1', sequenceNo: 1, name: '路径 1', choices: [{ routeNodeId: 'route-a', branchId: 'branch-b' }], updatedAt: '' }]
  const preview = previewAllExecutionPaths(graph, saved)
  assert.deepEqual(preview, { totalCount: 3, existingCount: 1, pendingCount: 2, exceeded: false })

  const wideGraph = { planId: '8', targetName: '', flowSource: 'new', entryNodeIds: ['route-0'], nodes: [], edges: [], warnings: [] }
  for (let index = 0; index < 8; index++) {
    const source = `route-${index}`
    const target = index === 7 ? 'end' : `route-${index + 1}`
    wideGraph.nodes.push({ id: source, name: source, type: 'condition', typeName: '条件' })
    wideGraph.edges.push(
      { id: `${source}-a`, source, target, kind: 'condition', label: 'A', branchId: 'a' },
      { id: `${source}-b`, source, target, kind: 'condition', label: 'B', branchId: 'b' },
    )
  }
  wideGraph.nodes.push({ id: 'end', name: '结束', type: 'end', typeName: '结束' })
  assert.equal(previewAllExecutionPaths(wideGraph, []).exceeded, true)
})

test('路径 API 只提交 choices、创建键和归属路径地址', async () => {
  const originalFetch = globalThis.fetch
  const calls = []
  globalThis.fetch = async (url, init) => {
    calls.push({ url: String(url), init })
    if (init.method === 'DELETE') return new Response(null, { status: 204 })
    if (init.method === 'GET') return Response.json({ success: true, data: { items: [] } })
    if (String(url).endsWith('/generate-all')) return Response.json({ success: true, data: { totalCount: 2, existingCount: 1, createdCount: 1, items: [] } })
    return Response.json({ success: true, data: { id: '31', sequenceNo: 1, name: '路径 1', choices: [], updatedAt: '2026-07-28T00:00:00Z' } })
  }
  try {
    const signal = new AbortController().signal
    await fetchExecutionPaths('7', signal)
    await createExecutionPath('7', '重点路径', [], '123e4567-e89b-12d3-a456-426614174301')
    await updateExecutionPath('7', '31', '改名路径', [])
    await generateAllExecutionPaths('7', '123e4567-e89b-12d3-a456-426614174302')
    await deleteExecutionPath('7', '31')
    assert.deepEqual(calls.map((call) => [call.init.method, call.url]), [
      ['GET', '/api/plans/7/execution-paths'],
      ['POST', '/api/plans/7/execution-paths'],
      ['PUT', '/api/plans/7/execution-paths/31'],
      ['POST', '/api/plans/7/execution-paths/generate-all'],
      ['DELETE', '/api/plans/7/execution-paths/31'],
    ])
    assert.equal(calls[1].init.headers['Idempotency-Key'], '123e4567-e89b-12d3-a456-426614174301')
    assert.equal(calls[1].init.body, '{"name":"重点路径","choices":[]}')
    assert.equal(calls[2].init.body, '{"name":"改名路径","choices":[]}')
    assert.equal(calls[3].init.headers['Idempotency-Key'], '123e4567-e89b-12d3-a456-426614174302')
  }
  finally {
    globalThis.fetch = originalFetch
  }
})

test('保存失败返回稳定错误且不修改调用方草稿', async () => {
  const originalFetch = globalThis.fetch
  const draft = [{ routeNodeId: 'route-a', branchId: 'branch-a' }]
  globalThis.fetch = async () => Response.json({
    success: false,
    error: { code: 'EXECUTION_PATH_INVALID', message: '执行路径选择不完整或已失效', retryable: false },
  }, { status: 409 })
  try {
    await assert.rejects(
      () => createExecutionPath('7', '失败保留', draft, '123e4567-e89b-12d3-a456-426614174301'),
      (error) => error instanceof ExecutionPathApiError && error.code === 'EXECUTION_PATH_INVALID',
    )
    assert.deepEqual(draft, [{ routeNodeId: 'route-a', branchId: 'branch-a' }])
  }
  finally {
    globalThis.fetch = originalFetch
  }
})
