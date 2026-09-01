import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const root = new URL('../../../..', import.meta.url)
const read = (relative) => fs.readFileSync(new URL(relative, root), 'utf8')

// TestHistoryReplayTaskEntrypoint 验证计划详情已经通过历史回放 API 驱动任务，而不是旧批量准备 API。
test('计划详情使用历史回放任务入口', () => {
  const view = read('web/src/views/PlanPathsView.vue')
  assert.match(view, /createHistoryReplay\(/)
  assert.match(view, /fetchActiveHistoryReplay\(/)
  assert.match(view, /开始历史回放/)
  assert.match(view, /历史回放进度/)
  assert.doesNotMatch(view, /from ['"]\.\.\/features\/path-preparation\/api['"]|createPathPreparation\(|fetchActivePathPreparation\(/)
})

// TestHistoryReplayApiUsesRawPathSelection 验证回放创建请求只携带路径 ID 和修订号，不包装表单正文。
test('历史回放 API 只发送路径选择与修订', () => {
  const api = read('web/src/features/history-replay/api.ts')
  assert.match(api, /history-replays/)
  assert.match(api, /JSON\.stringify\(\{ pathIds, revision \}\)/)
  assert.doesNotMatch(api, /HistoricalDataPayload|rawFormData|fieldMapping|renderAdapter/)
})
