import assert from 'node:assert/strict'
import test from 'node:test'

import { fetchFlowGraph, FlowGraphApiError } from '../../../web/src/features/flow-graph/api.ts'
import {
  flowNodeHorizontalGap,
  flowNodeVerticalGap,
  initialFlowZoom,
  initialViewportForGraph,
  layoutFlowGraph,
  shouldSetInitialViewport,
} from '../../../web/src/features/flow-graph/layout.ts'

const fixture = {
  planId: '41',
  targetName: '采购流程',
  flowSource: 'new',
  nodes: [
    { id: 'start', name: '发起', type: 'start', typeName: '发起' },
    { id: 'condition', name: '条件', type: 'condition', typeName: '条件' },
    { id: 'end', name: '结束', type: 'end', typeName: '结束' },
  ],
  edges: [
    { id: 'a', source: 'start', target: 'condition', kind: 'sequence', label: '', branchId: '' },
    { id: 'b', source: 'condition', target: 'end', kind: 'condition', label: '金额较小', branchId: 'strategy-a' },
  ],
  warnings: [],
}

test('dagre 以 TB 方向生成确定位置并保留真实分支名称', () => {
  const first = layoutFlowGraph(fixture)
  const second = layoutFlowGraph(fixture)
  assert.deepEqual(first, second)
  assert.equal(first.nodes.length, 3)
  assert.ok(first.nodes[0].position.y < first.nodes[1].position.y)
  assert.ok(first.nodes[1].position.y < first.nodes[2].position.y)
  assert.equal(first.edges[1].label, '金额较小')
  assert.equal(first.nodes[0].draggable, false)
  assert.equal(first.nodes[0].selectable, false)
  assert.equal(first.nodes[0].connectable, false)
  assert.equal(first.nodes.find((node) => node.id === 'condition').type, 'routingHub')
  assert.equal(first.edges[1].type, 'step')
})

test('嵌套路由的每组分支都严格按后端边顺序从左到右', () => {
  const graph = {
    ...fixture,
    nodes: [
      { id: 'start', name: '发起', type: 'start', typeName: '发起' },
      { id: 'route-1', name: '条件', type: 'condition', typeName: '条件路由' },
      { id: 'a', name: '分支 A', type: 'common', typeName: '审批' },
      { id: 'b', name: '手动', type: 'manual', typeName: '手动路由' },
      { id: 'c', name: '分支 C', type: 'common', typeName: '审批' },
      { id: 'd', name: '分支 D', type: 'common', typeName: '审批' },
      { id: 'e', name: '分支 E', type: 'common', typeName: '审批' },
      { id: 'f', name: '分支 F', type: 'common', typeName: '审批' },
    ],
    edges: [
      { id: 's-r1', source: 'start', target: 'route-1', kind: 'sequence', label: '', branchId: '' },
      { id: 'r1-a', source: 'route-1', target: 'a', kind: 'condition', label: 'A', branchId: 'a' },
      { id: 'r1-b', source: 'route-1', target: 'b', kind: 'condition', label: 'B', branchId: 'b' },
      { id: 'r1-c', source: 'route-1', target: 'c', kind: 'condition', label: 'C', branchId: 'c' },
      { id: 'b-d', source: 'b', target: 'd', kind: 'manual', label: 'D', branchId: 'd' },
      { id: 'b-e', source: 'b', target: 'e', kind: 'manual', label: 'E', branchId: 'e' },
      { id: 'b-f', source: 'b', target: 'f', kind: 'manual', label: 'F', branchId: 'f' },
    ],
  }
  const laidOut = layoutFlowGraph(graph)
  const x = Object.fromEntries(laidOut.nodes.map((node) => [node.id, node.position.x]))
  assert.ok(x.a < x.b && x.b < x.c, '第一层 A、B、C 应从左到右')
  assert.ok(x.d < x.e && x.e < x.f, '嵌套层 D、E、F 应从左到右')
  assert.equal(laidOut.nodes.find((node) => node.id === 'route-1').type, 'routingHub')
  assert.equal(laidOut.nodes.find((node) => node.id === 'b').type, 'routingHub')
  assert.ok(flowNodeHorizontalGap >= 100)
  assert.ok(flowNodeVerticalGap >= 110)
})

test('每个计划只设置一次可读初始视口并把根业务节点放在上方中央', () => {
  assert.equal(shouldSetInitialViewport(false, '', '41'), false)
  assert.equal(shouldSetInitialViewport(true, '', '41'), true)
  assert.equal(shouldSetInitialViewport(true, '41', '41'), false)
  assert.equal(shouldSetInitialViewport(true, '41', '42'), true)

  const laidOut = layoutFlowGraph(fixture)
  const viewport = initialViewportForGraph(laidOut.nodes, 1000)
  const root = laidOut.nodes.find((node) => node.id === 'start')
  assert.ok(viewport)
  assert.equal(viewport.zoom, initialFlowZoom)
  assert.equal((root.position.x + 90) * viewport.zoom + viewport.x, 500)
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
