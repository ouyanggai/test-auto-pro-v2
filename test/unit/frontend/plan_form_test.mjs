import assert from 'node:assert/strict'
import test from 'node:test'

import { getTargetFlowOptions } from '../../../web/src/features/plans/mock.ts'

test('前置条件不完整时不提供目标流程选项', () => {
  assert.deepEqual(getTargetFlowOptions(null, 'new'), [])
  assert.deepEqual(getTargetFlowOptions('zhangmin', null), [])
})

test('目标流程选项同时由真实账号与唯一流程来源决定', () => {
  assert.deepEqual(getTargetFlowOptions('zhangmin', 'new').map((option) => option.value), ['purchase-apply', 'contract-seal'])
  assert.deepEqual(getTargetFlowOptions('liwei', 'new').map((option) => option.value), ['project-create'])
  assert.notDeepEqual(
    getTargetFlowOptions('zhangmin', 'sent').map((option) => option.value),
    getTargetFlowOptions('zhangmin', 'new').map((option) => option.value),
  )
})
