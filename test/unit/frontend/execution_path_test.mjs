import assert from 'node:assert/strict'
import test from 'node:test'

import {
  analyzeExecutionPath,
  applyExecutionPathChoice,
  canCreateAdditionalPath,
  canEnterExecutionPathSelection,
  classifyExecutionPathEdges,
  nextExecutionPathRouteID,
  projectExecutionPathSummary,
  reconcileExecutionPathChoices,
  refreshExecutionPathDraft,
  viewportForPointNearest,
} from '../../../web/src/features/execution-paths/logic.ts'
import {
  createExecutionPath,
  deleteExecutionPath,
  ExecutionPathApiError,
  fetchExecutionPaths,
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

test('下一待选点只做保持缩放的最小视口平移', () => {
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
})

test('路径 API 只提交 choices、创建键和归属路径地址', async () => {
  const originalFetch = globalThis.fetch
  const calls = []
  globalThis.fetch = async (url, init) => {
    calls.push({ url: String(url), init })
    if (init.method === 'DELETE') return new Response(null, { status: 204 })
    if (init.method === 'GET') return Response.json({ success: true, data: { items: [] } })
    return Response.json({ success: true, data: { id: '31', sequenceNo: 1, choices: [], updatedAt: '2026-07-28T00:00:00Z' } })
  }
  try {
    const signal = new AbortController().signal
    await fetchExecutionPaths('7', signal)
    await createExecutionPath('7', [], '123e4567-e89b-12d3-a456-426614174301')
    await updateExecutionPath('7', '31', [])
    await deleteExecutionPath('7', '31')
    assert.deepEqual(calls.map((call) => [call.init.method, call.url]), [
      ['GET', '/api/plans/7/execution-paths'],
      ['POST', '/api/plans/7/execution-paths'],
      ['PUT', '/api/plans/7/execution-paths/31'],
      ['DELETE', '/api/plans/7/execution-paths/31'],
    ])
    assert.equal(calls[1].init.headers['Idempotency-Key'], '123e4567-e89b-12d3-a456-426614174301')
    assert.equal(calls[1].init.body, '{"choices":[]}')
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
      () => createExecutionPath('7', draft, '123e4567-e89b-12d3-a456-426614174301'),
      (error) => error instanceof ExecutionPathApiError && error.code === 'EXECUTION_PATH_INVALID',
    )
    assert.deepEqual(draft, [{ routeNodeId: 'route-a', branchId: 'branch-a' }])
  }
  finally {
    globalThis.fetch = originalFetch
  }
})
