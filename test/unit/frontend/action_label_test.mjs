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

test('后端可信中文名优先于键映射', () => {
  assert.equal(actionLabel('目标跳过', 'approve'), '同意')
  assert.equal(actionLabel('目标跳过', undefined), '目标跳过')
})
