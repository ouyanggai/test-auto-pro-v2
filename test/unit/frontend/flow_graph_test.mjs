import assert from 'node:assert/strict'
import test from 'node:test'

import { fetchFlowGraph, FlowGraphApiError } from '../../../web/src/features/flow-graph/api.ts'
import {
  flowNodeHeight,
  flowNodeWidth,
  flowRoutingHubSize,
  flowTreeHorizontalGap,
  initialFlowZoom,
  initialViewportForGraph,
  layoutFlowGraph,
  shouldSetInitialViewport,
} from '../../../web/src/features/flow-graph/layout.ts'

const node = (id, type = 'common', mergeTargetId) => ({
  id,
  name: id,
  type,
  typeName: type,
  ...(mergeTargetId ? { mergeTargetId } : {}),
})
const edge = (id, source, target, kind = 'sequence', label = '') => ({
  id,
  source,
  target,
  kind,
  label,
  branchId: kind === 'sequence' ? '' : id,
})
const centerX = (value) => value.position.x + (value.type === 'routingHub' ? flowRoutingHubSize : flowNodeWidth) / 2

function byId(layout) {
  return Object.fromEntries(layout.nodes.map((value) => [value.id, value]))
}

function assertNoNodeOverlap(layout) {
  const rectangles = layout.nodes.map((value) => ({
    id: value.id,
    left: value.position.x,
    top: value.position.y,
    right: value.position.x + (value.type === 'routingHub' ? flowRoutingHubSize : flowNodeWidth),
    bottom: value.position.y + (value.type === 'routingHub' ? flowRoutingHubSize : flowNodeHeight),
  }))
  for (let leftIndex = 0; leftIndex < rectangles.length; leftIndex++) {
    for (let rightIndex = leftIndex + 1; rightIndex < rectangles.length; rightIndex++) {
      const left = rectangles[leftIndex]
      const right = rectangles[rightIndex]
      const overlaps = left.left < right.right && left.right > right.left && left.top < right.bottom && left.bottom > right.top
      assert.equal(overlaps, false, `${left.id} 与 ${right.id} 不应重叠`)
    }
  }
}

const straightFixture = {
  planId: '41',
  targetName: '采购流程',
  flowSource: 'new',
  nodes: [node('start', 'start'), node('approval'), node('end', 'end')],
  edges: [edge('start-approval', 'start', 'approval'), edge('approval-end', 'approval', 'end')],
  warnings: [],
}

function branchedFixture() {
  return {
    ...straightFixture,
    nodes: [
      node('start', 'start'), node('route', 'condition', 'merge'),
      node('a'), node('b1'), node('b2'), node('c1'), node('c2'), node('c3'),
      node('merge'), node('end', 'end'),
    ],
    edges: [
      edge('start-route', 'start', 'route'),
      edge('route-a', 'route', 'a', 'condition', 'A'),
      edge('route-b', 'route', 'b1', 'condition', 'B'),
      edge('route-c', 'route', 'c1', 'condition', 'C'),
      edge('a-merge', 'a', 'merge'),
      edge('b1-b2', 'b1', 'b2'), edge('b2-merge', 'b2', 'merge'),
      edge('c1-c2', 'c1', 'c2'), edge('c2-c3', 'c2', 'c3'), edge('c3-merge', 'c3', 'merge'),
      edge('merge-end', 'merge', 'end'),
    ],
  }
}

function nestedFixture() {
  return {
    ...straightFixture,
    nodes: [
      node('start', 'start'), node('outer', 'condition', 'merge'), node('a'),
      node('inner', 'manual', 'inner-merge'), node('d'), node('e'), node('inner-merge'), node('inner-tail'),
      node('c'), node('merge'), node('end', 'end'),
    ],
    edges: [
      edge('start-outer', 'start', 'outer'),
      edge('outer-a', 'outer', 'a', 'condition', 'A'),
      edge('outer-inner', 'outer', 'inner', 'condition', 'B'),
      edge('outer-c', 'outer', 'c', 'condition', 'C'),
      edge('a-merge', 'a', 'merge'), edge('c-merge', 'c', 'merge'),
      edge('inner-d', 'inner', 'd', 'manual', 'D'), edge('inner-e', 'inner', 'e', 'manual', 'E'),
      edge('d-inner-merge', 'd', 'inner-merge'), edge('e-inner-merge', 'e', 'inner-merge'),
      edge('inner-merge-tail', 'inner-merge', 'inner-tail'), edge('inner-tail-merge', 'inner-tail', 'merge'),
      edge('merge-end', 'merge', 'end'),
    ],
  }
}

test('直线流程保持同一主干中心且只生成确定垂直路径', () => {
  const first = layoutFlowGraph(straightFixture)
  const second = layoutFlowGraph(straightFixture)
  assert.deepEqual(first, second)
  const nodes = byId(first)
  assert.equal(centerX(nodes.start), centerX(nodes.approval))
  assert.equal(centerX(nodes.approval), centerX(nodes.end))
  assert.ok(nodes.start.position.y < nodes.approval.position.y)
  assert.ok(nodes.approval.position.y < nodes.end.position.y)
  for (const value of first.edges) {
    assert.equal(value.type, 'treeEdge')
    assert.equal(value.data.role, 'main')
    assert.match(value.data.path, /^M [\d.]+ [\d.]+ V [\d.]+$/)
    assert.doesNotMatch(value.data.path, / H /)
  }
  assert.equal(first.nodes[0].draggable, false)
  assert.equal(first.nodes[0].selectable, false)
  assert.equal(first.nodes[0].connectable, false)
})

test('三分支按 A B C 左到右顶对齐并共享唯一分叉轨和汇合轨', () => {
  const layout = layoutFlowGraph(branchedFixture())
  const nodes = byId(layout)
  assert.ok(nodes.a.position.x < nodes.b1.position.x && nodes.b1.position.x < nodes.c1.position.x)
  assert.equal(nodes.a.position.y, nodes.b1.position.y)
  assert.equal(nodes.b1.position.y, nodes.c1.position.y)
  assert.equal(centerX(nodes.start), centerX(nodes.route))
  assert.equal(centerX(nodes.route), centerX(nodes.merge))
  assert.equal(centerX(nodes.merge), centerX(nodes.end))

  const forkEdges = layout.edges.filter((value) => value.data.role === 'fork')
  const mergeEdges = layout.edges.filter((value) => value.data.role === 'merge')
  assert.equal(new Set(forkEdges.map((value) => value.data.railY)).size, 1)
  assert.equal(new Set(mergeEdges.map((value) => value.data.railY)).size, 1)
  assert.ok(forkEdges.every((value) => value.data.path.includes(` V ${value.data.railY} H `)))
  assert.ok(mergeEdges.every((value) => value.data.path.includes(` V ${value.data.railY} H `)))
  assert.deepEqual(forkEdges.map((value) => value.label), ['A', 'B', 'C'])
  assert.ok(mergeEdges.every((value) => value.data.railY > nodes.c3.position.y + flowNodeHeight))
  assert.ok(nodes.merge.position.y > mergeEdges[0].data.railY)
  assertNoNodeOverlap(layout)
})

test('两级嵌套路由在父分支块内展开并推开相邻分支', () => {
  const layout = layoutFlowGraph(nestedFixture())
  const nodes = byId(layout)
  assert.equal(nodes.outer.type, 'routingHub')
  assert.equal(nodes.inner.type, 'routingHub')
  assert.ok(nodes.a.position.x < nodes.inner.position.x && nodes.inner.position.x < nodes.c.position.x)
  assert.ok(nodes.d.position.x < nodes.e.position.x)
  assert.equal(nodes.d.position.y, nodes.e.position.y)
  assert.ok(nodes.c.position.x - nodes.a.position.x > (flowNodeWidth + flowTreeHorizontalGap) * 2)
  assert.equal(centerX(nodes.outer), centerX(nodes.merge))
  assert.equal(centerX(nodes.inner), centerX(nodes['inner-merge']))
  const forkRailYs = layout.edges.filter((value) => value.data.role === 'fork').map((value) => value.data.railY)
  const mergeRailYs = layout.edges.filter((value) => value.data.role === 'merge').map((value) => value.data.railY)
  assert.equal(new Set(forkRailYs).size, 2)
  assert.equal(new Set(mergeRailYs).size, 2)
  assertNoNodeOverlap(layout)
})

test('结构化布局拒绝循环和重复分支节点而不回退通用图布局', () => {
  const cycle = {
    ...straightFixture,
    nodes: [node('a'), node('b')],
    edges: [edge('a-b', 'a', 'b'), edge('b-a', 'b', 'a')],
  }
  assert.throws(() => layoutFlowGraph(cycle), /真实根节点/)

  const duplicate = {
    ...straightFixture,
    nodes: [node('route', 'parallel'), node('shared')],
    edges: [edge('route-a', 'route', 'shared', 'parallel'), edge('route-b', 'route', 'shared', 'parallel')],
  }
  assert.throws(() => layoutFlowGraph(duplicate), /重复出现/)
})

test('每个计划只设置一次可读初始视口并把根业务节点放在上方中央', () => {
  assert.equal(shouldSetInitialViewport(false, '', '41'), false)
  assert.equal(shouldSetInitialViewport(true, '', '41'), true)
  assert.equal(shouldSetInitialViewport(true, '41', '41'), false)
  assert.equal(shouldSetInitialViewport(true, '41', '42'), true)

  const laidOut = layoutFlowGraph(straightFixture)
  const viewport = initialViewportForGraph(laidOut.nodes, 1000)
  const root = laidOut.nodes.find((value) => value.id === 'start')
  assert.ok(viewport)
  assert.equal(viewport.zoom, initialFlowZoom)
  assert.equal((root.position.x + flowNodeWidth / 2) * viewport.zoom + viewport.x, 500)
  assert.equal(root.position.y * viewport.zoom + viewport.y, 52)
})

test('流程图 API 保留稳定错误与取消边界', async (t) => {
  const originalFetch = globalThis.fetch
  t.after(() => { globalThis.fetch = originalFetch })

  globalThis.fetch = async (path, init) => {
    assert.equal(path, '/api/plans/41/flow-graph')
    assert.equal(init.method, 'GET')
    return new Response(JSON.stringify({
      success: false,
      error: { code: 'TARGET_FLOW_NOT_FOUND', message: '目标流程当前不可读取', retryable: false },
    }), { status: 404, headers: { 'Content-Type': 'application/json' } })
  }
  await assert.rejects(
    fetchFlowGraph('41', new AbortController().signal),
    (error) => error instanceof FlowGraphApiError && error.code === 'TARGET_FLOW_NOT_FOUND' && error.retryable === false,
  )

  const controller = new AbortController()
  globalThis.fetch = async (_path, init) => new Promise((_resolve, reject) => {
    init.signal.addEventListener('abort', () => reject(init.signal.reason), { once: true })
  })
  const pending = fetchFlowGraph('42', controller.signal)
  controller.abort(new DOMException('aborted', 'AbortError'))
  await assert.rejects(pending, (error) => error.name === 'AbortError')
})
