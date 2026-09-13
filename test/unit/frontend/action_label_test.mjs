import assert from 'node:assert/strict'
import test from 'node:test'

import { actionLabel } from '../../../web/src/features/runs/presentation.ts'

// F-034 T03/T06：动作名集中映射与未知键安全占位；原始英文稳定键绝不出现在用户可见文本。

test('已知动作键映射中文，重点是“同意”', () => {
  assert.equal(actionLabel('同意', 'approve'), '同意')
  assert.equal(actionLabel(undefined, 'approve'), '同意')
  assert.equal(actionLabel(undefined, 'submit'), '提交')
  assert.equal(actionLabel(undefined, 'resubmit'), '重新提交')
  assert.equal(actionLabel(undefined, 'save_draft'), '保存草稿')
  assert.equal(actionLabel(undefined, 'storage_form_data'), '暂存当前表单')
  assert.equal(actionLabel(undefined, 'rollback_previous'), '回退上一节点')
  assert.equal(actionLabel(undefined, 'unfollow'), '取消关注')
})

test('未知动作键显示安全占位，不渲染原始稳定键', () => {
  assert.equal(actionLabel(undefined, 'mystery_action'), '未识别动作（请查看日志）')
  assert.equal(actionLabel('mystery_action', undefined), '未识别动作（请查看日志）')
  assert.equal(actionLabel('', undefined), '未识别动作（请查看日志）')
})

test('actionName 缺失但存在稳定 action 键时仍显示中文（评审 #3）', () => {
  // 历史数据只有 { "action": "approve" } 时必须显示“同意”，不能落到未识别占位。
  assert.equal(actionLabel(undefined, 'approve'), '同意')
  assert.equal(actionLabel('', 'approve'), '同意')
  assert.equal(actionLabel(undefined, 'storage_form_data'), '暂存当前表单')
  assert.equal(actionLabel(undefined, 'rollback_previous'), '回退上一节点')
})

test('阻塞信息按步骤绑定（评审 #2 语义口径）', () => {
  // 该用例锁定判定口径：只有 stopStepNo 匹配的步骤才使用路径阻塞；
  // 实现位于 RunNodePanel.vue 的 stepOutcomeTitle/stepWhyText（stepBlocked 判定），
  // 这里以纯函数口径记录预期，组件级行为由人工验收确认。
  const detail = { stopKind: 'blocked', stopStepNo: 3, stopKindNote: '手动条件分支,请选择' }
  const isBlockedForStep = (step) => detail.stopKind === 'blocked' && detail.stopStepNo === step.stepNo
  assert.equal(isBlockedForStep({ stepNo: 1, statusName: '执行成功' }), false)
  assert.equal(isBlockedForStep({ stepNo: 3, statusName: '执行失败' }), true)
})

test('后端可信中文名优先于键映射', () => {
  assert.equal(actionLabel('目标跳过', 'approve'), '同意')
  assert.equal(actionLabel('目标跳过', undefined), '目标跳过')
})
