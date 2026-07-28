import assert from 'node:assert/strict'
import test from 'node:test'

import { fetchFlowGraph, FlowGraphApiError } from '../../../web/src/features/flow-graph/api.ts'
import { layoutFlowGraph, shouldFitInitialGraph } from '../../../web/src/features/flow-graph/layout.ts'

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
})

test('每个计划只在图首次就绪时触发适配', () => {
  assert.equal(shouldFitInitialGraph(false, '', '41'), false)
  assert.equal(shouldFitInitialGraph(true, '', '41'), true)
  assert.equal(shouldFitInitialGraph(true, '41', '41'), false)
  assert.equal(shouldFitInitialGraph(true, '41', '42'), true)
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
