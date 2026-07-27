import assert from 'node:assert/strict'
import test from 'node:test'

import { hasPlanFormErrors, shouldShowMaxConcurrency, validatePlanForm } from '../../../web/src/features/plans/logic.ts'
import { getTargetFlowOptions } from '../../../web/src/features/plans/mock.ts'

const validForm = {
  name: '采购申请主流程回归',
  accountId: 'zhangmin',
  flowSource: 'new',
  flowId: 'purchase-apply',
  runMode: 'serial',
  maxConcurrency: null,
  scheduledAt: null,
}

test('串行模式不显示并发字段且完整表单通过校验', () => {
  assert.equal(shouldShowMaxConcurrency('serial'), false)
  assert.equal(hasPlanFormErrors(validatePlanForm(validForm)), false)
})

test('并行模式显示并发字段并校验 2 至 20 的边界', () => {
  assert.equal(shouldShowMaxConcurrency('parallel'), true)
  assert.equal(validatePlanForm({ ...validForm, runMode: 'parallel', maxConcurrency: 1 }).maxConcurrency, '并行最大并发数应为 2 至 20')
  assert.equal(validatePlanForm({ ...validForm, runMode: 'parallel', maxConcurrency: 21 }).maxConcurrency, '并行最大并发数应为 2 至 20')
  assert.equal(hasPlanFormErrors(validatePlanForm({ ...validForm, runMode: 'parallel', maxConcurrency: 2 })), false)
})

test('必填字段缺失时返回对应人话错误', () => {
  const errors = validatePlanForm({
    ...validForm,
    name: ' ',
    accountId: null,
    flowSource: null,
    flowId: null,
  })
  assert.deepEqual(Object.keys(errors).sort(), ['accountId', 'flowId', 'flowSource', 'name'])
})

test('目标流程选项同时依赖账号与唯一流程来源', () => {
  assert.deepEqual(getTargetFlowOptions(null, 'new'), [])
  assert.deepEqual(getTargetFlowOptions('zhangmin', null), [])
  assert.deepEqual(getTargetFlowOptions('zhangmin', 'new').map((option) => option.value), ['purchase-apply', 'contract-seal'])
  assert.deepEqual(getTargetFlowOptions('liwei', 'new').map((option) => option.value), ['project-create'])
})
