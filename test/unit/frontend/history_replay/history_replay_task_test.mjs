import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'
import { createHistoryReplay } from '../../../../web/src/features/history-replay/api.ts'

const root = new URL('../../../..', import.meta.url)
const read = (relative) => fs.readFileSync(new URL(relative, root), 'utf8')

// TestPreparationTaskEntrypoint 验证计划详情的一键配置由服务端批量准备任务驱动，而不是浏览器自行造数。
test('计划详情一键配置走服务端批量准备任务', () => {
  const view = read('web/src/views/PlanPathsView.vue')
  assert.match(view, /createHistoryReplay\(/)
  assert.match(view, /fetchActiveHistoryReplay\(/)
  assert.match(view, /一键配置/)
  assert.match(view, /批量准备进度/)
  assert.match(view, /路径准备与运行选择/)
  assert.doesNotMatch(view, /from ['"]\.\.\/features\/path-preparation\/api['"]|createPathPreparation\(|fetchActivePathPreparation\(/)
})

// TestPreparationApiUsesRawPathSelection 验证真实创建请求携带路径、修订与动作方案，不包装表单正文。
test('批量准备 API 发送明确的路径选择、修订和动作方案', async () => {
  const api = read('web/src/features/history-replay/api.ts')
  assert.match(api, /history-replays/)
  const fetchBefore = globalThis.fetch
  let request
  globalThis.fetch = async (url, init) => {
    request = { url, init }
    return { ok: true, json: async () => ({ success: true, data: { id: 'job-1', actionMode: 'approve_only' } }) }
  }
  try {
    const result = await createHistoryReplay('91', [101, 102], 0, '123e4567-e89b-12d3-a456-426614175050', 'approve_only')
    assert.equal(request.url, '/api/plans/91/history-replays')
    assert.equal(request.init.method, 'POST')
    assert.deepEqual(JSON.parse(request.init.body), { pathIds: [101, 102], revision: 0, actionMode: 'approve_only' })
    assert.equal(result.actionMode, 'approve_only')
  } finally { globalThis.fetch = fetchBefore }
  assert.doesNotMatch(api, /HistoricalDataPayload|rawFormData|fieldMapping|renderAdapter/)
})
