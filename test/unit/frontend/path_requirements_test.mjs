import assert from 'node:assert/strict'
import test from 'node:test'

import {
  defaultRequirementPath,
  requirementStatusType,
  shouldApplyRequirementResponse,
} from '../../../web/src/features/plans/requirements.ts'

test('默认选择稳定序号最小的已保存路径且不修改原列表', () => {
  const paths = [
    { id: '3', sequenceNo: 3, name: '第三条', choices: [], updatedAt: '' },
    { id: '1', sequenceNo: 1, name: '第一条', choices: [], updatedAt: '' },
    { id: '2', sequenceNo: 2, name: '第二条', choices: [], updatedAt: '' },
  ]
  assert.equal(defaultRequirementPath(paths)?.id, '1')
  assert.deepEqual(paths.map((path) => path.id), ['3', '1', '2'])
  assert.equal(defaultRequirementPath([]), null)
})

test('路径切换只接受当前路径且未取消的最新响应', () => {
  assert.equal(shouldApplyRequirementResponse({
    requestedPathId: '2', activePathId: '2', requestVersion: 4, currentVersion: 4, aborted: false,
  }), true)
  assert.equal(shouldApplyRequirementResponse({
    requestedPathId: '1', activePathId: '2', requestVersion: 3, currentVersion: 4, aborted: false,
  }), false)
  assert.equal(shouldApplyRequirementResponse({
    requestedPathId: '2', activePathId: '2', requestVersion: 3, currentVersion: 4, aborted: false,
  }), false)
  assert.equal(shouldApplyRequirementResponse({
    requestedPathId: '2', activePathId: '2', requestVersion: 4, currentVersion: 4, aborted: true,
  }), false)
})

test('四类中文状态具有稳定且克制的标签类型', () => {
  assert.equal(requirementStatusType('待配置'), 'warning')
  assert.equal(requirementStatusType('目标平台自动确定'), 'success')
  assert.equal(requirementStatusType('运行时确定'), 'info')
  assert.equal(requirementStatusType('需要人工核对'), 'error')
})
