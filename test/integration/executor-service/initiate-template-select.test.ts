// 模板精确选择计划（S2 权威实现 planTemplateSelection）的单元测试：
// 判据只认 /web/flowTemplateApi/list 的真实响应集合，空列表/无响应/不含绑定 ID 一律阻塞。
// 「查询空列表直接装载」的旧断言已被证伪删除（施工令 §2/§8.5）。
import assert from 'node:assert/strict'
import test from 'node:test'

import { planTemplateSelection } from '../../../executor-service/src/native-initiate.ts'

test('候选为 null（列表请求失败或无响应）必须阻塞', () => {
  const plan = planTemplateSelection(null, 'bound-1')
  assert.equal(plan.action, 'blocked')
})

test('候选集合为空：目标报告无流程权限，必须阻塞', () => {
  const plan = planTemplateSelection([], 'bound-1')
  assert.equal(plan.action, 'blocked')
})

test('绑定模板不在候选集合中必须阻塞且带集合信息', () => {
  const plan = planTemplateSelection(['other-1', 'other-2'], 'bound-2')
  assert.equal(plan.action, 'blocked')
  assert.match(plan.reason, /bound-2/)
})

test('唯一候选即绑定模板：等待原生自行装载（single）', () => {
  const plan = planTemplateSelection(['bound-1'], 'bound-1')
  assert.deepEqual(plan, { action: 'single', boundId: 'bound-1' })
})

test('多候选含绑定模板：走原生 confirm 选择', () => {
  const plan = planTemplateSelection(['bound-1', 'other-1', 'other-2'], 'bound-1')
  assert.deepEqual(plan, { action: 'confirm', boundId: 'bound-1' })
})

test('缺少计划绑定模板 ID 必须阻塞', () => {
  const plan = planTemplateSelection(['bound-1', 'other-1'], '')
  assert.equal(plan.action, 'blocked')
})
