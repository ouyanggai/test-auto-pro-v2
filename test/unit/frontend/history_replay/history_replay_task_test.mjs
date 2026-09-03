import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

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

// TestPreparationApiUsesRawPathSelection 验证批量准备创建请求只携带路径 ID 和修订号，不包装表单正文。
test('批量准备 API 只发送路径选择与修订', () => {
  const api = read('web/src/features/history-replay/api.ts')
  assert.match(api, /history-replays/)
  assert.match(api, /JSON\.stringify\(\{ pathIds, revision \}\)/)
  assert.doesNotMatch(api, /HistoricalDataPayload|rawFormData|fieldMapping|renderAdapter/)
})
