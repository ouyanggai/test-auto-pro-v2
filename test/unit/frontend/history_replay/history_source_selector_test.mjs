import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const component = readFileSync('web/src/features/history-replay/HistorySourceSelector.vue', 'utf8')
const api = readFileSync('web/src/features/history-replay/api.ts', 'utf8')
const types = readFileSync('web/src/features/history-replay/types.ts', 'utf8')
const planPathsView = readFileSync('web/src/views/PlanPathsView.vue', 'utf8')
const pathConfigurationView = readFileSync('web/src/views/PlanPathConfigurationView.vue', 'utf8')

test('历史候选弹窗展示默认/覆盖、完整性和明确空态', () => {
  assert.match(component, /计划默认历史来源/)
  assert.match(component, /继承计划默认来源/)
  assert.match(component, /使用独立历史来源/)
  assert.match(component, /candidate\.integrityNotice/)
  assert.match(component, /candidate\.completeness/)
  assert.match(component, /candidate\.initiator/)
  assert.match(component, /candidate\.companyName/)
  assert.match(component, /candidate\.createdAt/)
  assert.match(component, /请先在目标平台发起一次该流程并填写业务数据，再回来刷新/)
})

test('来源请求只回传不透明键、模式、修订号和幂等键', () => {
  assert.match(api, /history-data\/candidates/)
  assert.match(api, /history-data\/default/)
  assert.match(api, /configuration\/data\/source/)
  assert.match(api, /Idempotency-Key/)
  assert.match(api, /JSON\.stringify\(\{ candidateKey, revision \}\)/)
  assert.match(api, /JSON\.stringify\(\{ mode, candidateKey: mode === 'override' \? candidateKey : undefined, revision \}\)/)
  for (const forbidden of ['targetInstanceId', 'flowProxyId', 'formProxyId', 'snapshotId', 'rawFormData', 'sourceAccount', 'HistoricalDataPayload']) {
    assert.equal(api.includes(forbidden), false, `来源 API 不应包含 ${forbidden}`)
    assert.equal(types.includes(forbidden), false, `浏览器类型不应包含 ${forbidden}`)
  }
})

test('计划和路径工作区都接入同一来源选择组件且没有智能生成入口', () => {
  assert.match(planPathsView, /HistorySourceSelector/)
  assert.match(planPathsView, /scope="default"/)
  assert.match(pathConfigurationView, /HistorySourceSelector/)
  assert.match(pathConfigurationView, /scope="path"/)
  for (const forbidden of ['智能生成', '换一组', '随机种子', '示例值']) {
    assert.equal(component.includes(forbidden), false, `历史来源组件不应出现 ${forbidden}`)
  }
})
