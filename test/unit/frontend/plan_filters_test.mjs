import assert from 'node:assert/strict'
import test from 'node:test'

import { filterPlans } from '../../../web/src/features/plans/logic.ts'
import { mockPlans } from '../../../web/src/features/plans/mock.ts'

test('计划名称筛选忽略首尾空格并支持包含匹配', () => {
  const result = filterPlans(mockPlans, { name: '  合同用印  ', status: null })
  assert.deepEqual(result.map((plan) => plan.id), ['plan-002'])
})

test('计划状态筛选返回对应本地 mock 行', () => {
  const result = filterPlans(mockPlans, { name: '', status: 'running' })
  assert.deepEqual(result.map((plan) => plan.id), ['plan-003'])
})

test('名称与状态组合筛选且清空后恢复全部行', () => {
  assert.equal(filterPlans(mockPlans, { name: '项目', status: 'completed' }).length, 1)
  assert.equal(filterPlans(mockPlans, { name: '', status: null }).length, mockPlans.length)
  assert.equal(filterPlans(mockPlans, { name: '不存在', status: null }).length, 0)
})
